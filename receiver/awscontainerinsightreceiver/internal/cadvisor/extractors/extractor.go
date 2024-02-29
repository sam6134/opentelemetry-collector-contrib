// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"

import (
	"fmt"
	"time"

	cinfo "github.com/google/cadvisor/info/v1"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	awsmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
)

func GetStats(info *cinfo.ContainerInfo) *cinfo.ContainerStats {
	if len(info.Stats) == 0 {
		return nil
	}
	// When there is more than one stats point, always use the last one
	return info.Stats[len(info.Stats)-1]
}

type CPUMemInfoProvider interface {
	GetNumCores() int64
	GetMemoryCapacity() int64
}

type MetricExtractor interface {
	HasValue(*cinfo.ContainerInfo) bool
	GetValue(info *cinfo.ContainerInfo, mInfo CPUMemInfoProvider, containerType string) []*RawContainerInsightsMetric
	Shutdown() error
}

type RawContainerInsightsMetric struct {
	// source of the metric for debugging merge conflict
	ContainerName string
	// key/value pairs that are typed and contain the metric (numerical) data
	Fields map[string]any
	// key/value string pairs that are used to identify the metrics
	Tags map[string]string

	Logger *zap.Logger
}

var _ stores.CIMetric = (*RawContainerInsightsMetric)(nil)

func NewRawContainerInsightsMetric(mType string, logger *zap.Logger) *RawContainerInsightsMetric {
	metric := &RawContainerInsightsMetric{
		Fields: make(map[string]any),
		Tags:   make(map[string]string),
		Logger: logger,
	}
	metric.Tags[ci.MetricType] = mType
	return metric
}

func (c *RawContainerInsightsMetric) GetTags() map[string]string {
	return c.Tags
}

func (c *RawContainerInsightsMetric) GetFields() map[string]any {
	return c.Fields
}

func (c *RawContainerInsightsMetric) GetMetricType() string {
	return c.Tags[ci.MetricType]
}

func (c *RawContainerInsightsMetric) AddTags(tags map[string]string) {
	for k, v := range tags {
		c.Tags[k] = v
	}
}

func (c *RawContainerInsightsMetric) HasField(key string) bool {
	return c.Fields[key] != nil
}

func (c *RawContainerInsightsMetric) AddField(key string, val any) {
	c.Fields[key] = val
}

func (c *RawContainerInsightsMetric) GetField(key string) any {
	return c.Fields[key]
}

func (c *RawContainerInsightsMetric) HasTag(key string) bool {
	return c.Tags[key] != ""
}

func (c *RawContainerInsightsMetric) AddTag(key, val string) {
	c.Tags[key] = val
}

func (c *RawContainerInsightsMetric) GetTag(key string) string {
	return c.Tags[key]
}

func (c *RawContainerInsightsMetric) RemoveTag(key string) {
	delete(c.Tags, key)
}

func (c *RawContainerInsightsMetric) Merge(src *RawContainerInsightsMetric) {
	// If there is any conflict, keep the Fields with earlier timestamp
	for k, v := range src.Fields {
		if _, ok := c.Fields[k]; ok {
			c.Logger.Debug(fmt.Sprintf("metric being merged has conflict in Fields, src: %v, dest: %v \n", *src, *c))
			c.Logger.Debug("metric being merged has conflict in Fields", zap.String("src", src.ContainerName), zap.String("dest", c.ContainerName))
			if c.Tags[ci.Timestamp] < src.Tags[ci.Timestamp] {
				continue
			}
		}
		c.Fields[k] = v
	}
}

func newFloat64RateCalculator() awsmetrics.MetricCalculator {
	return awsmetrics.NewMetricCalculator(func(prev *awsmetrics.MetricValue, val any, timestamp time.Time) (any, bool) {
		if prev != nil {
			deltaNs := timestamp.Sub(prev.Timestamp)
			deltaValue := val.(float64) - prev.RawValue.(float64)
			if deltaNs > ci.MinTimeDiff && deltaValue >= 0 {
				return deltaValue / float64(deltaNs), true
			}
		}
		return float64(0), false
	})
}

func assignRateValueToField(rateCalculator *awsmetrics.MetricCalculator, fields map[string]any, metricName string,
	cinfoName string, curVal any, curTime time.Time, multiplier float64) {
	mKey := awsmetrics.NewKey(cinfoName+metricName, nil)
	if val, ok := rateCalculator.Calculate(mKey, curVal, curTime); ok {
		fields[metricName] = val.(float64) * multiplier
	}
}

// MergeMetrics merges an array of cadvisor metrics based on common metric keys
func MergeMetrics(metrics []*RawContainerInsightsMetric) []*RawContainerInsightsMetric {
	result := make([]*RawContainerInsightsMetric, 0, len(metrics))
	metricMap := make(map[string]*RawContainerInsightsMetric)
	for _, metric := range metrics {
		if metricKey := getMetricKey(metric); metricKey != "" {
			if mergedMetric, ok := metricMap[metricKey]; ok {
				mergedMetric.Merge(metric)
			} else {
				metricMap[metricKey] = metric
			}
		} else {
			// this metric cannot be merged
			result = append(result, metric)
		}
	}
	for _, metric := range metricMap {
		result = append(result, metric)
	}
	return result
}

// return MetricKey for merge-able metrics
func getMetricKey(metric *RawContainerInsightsMetric) string {
	metricType := metric.GetMetricType()
	var metricKey string
	switch metricType {
	case ci.TypeInstance:
		// merge cpu, memory, net metric for type Instance
		metricKey = fmt.Sprintf("metricType:%s", ci.TypeInstance)
	case ci.TypeNode:
		// merge cpu, memory, net metric for type Node
		metricKey = fmt.Sprintf("metricType:%s", ci.TypeNode)
	case ci.TypePod:
		// merge cpu, memory, net metric for type Pod
		metricKey = fmt.Sprintf("metricType:%s,podId:%s", ci.TypePod, metric.GetTags()[ci.PodIDKey])
	case ci.TypeContainer:
		// merge cpu, memory metric for type Container
		metricKey = fmt.Sprintf("metricType:%s,podId:%s,ContainerName:%s", ci.TypeContainer, metric.GetTags()[ci.PodIDKey], metric.GetTags()[ci.ContainerNamekey])
	case ci.TypeInstanceDiskIO:
		// merge io_serviced, io_service_bytes for type InstanceDiskIO
		metricKey = fmt.Sprintf("metricType:%s,device:%s", ci.TypeInstanceDiskIO, metric.GetTags()[ci.DiskDev])
	case ci.TypeNodeDiskIO:
		// merge io_serviced, io_service_bytes for type NodeDiskIO
		metricKey = fmt.Sprintf("metricType:%s,device:%s", ci.TypeNodeDiskIO, metric.GetTags()[ci.DiskDev])
	default:
		metricKey = ""
	}
	return metricKey
}
