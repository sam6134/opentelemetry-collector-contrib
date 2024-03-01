// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"

import (
	"time"

	cinfo "github.com/google/cadvisor/info/v1"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	awsmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
)

type MemMetricExtractor struct {
	logger         *zap.Logger
	rateCalculator awsmetrics.MetricCalculator
}

func (m *MemMetricExtractor) HasValue(info *cinfo.ContainerInfo) bool {
	return info.Spec.HasMemory
}

func (m *MemMetricExtractor) GetValue(info *cinfo.ContainerInfo, mInfo CPUMemInfoProvider, containerType string) []*stores.RawContainerInsightsMetric {
	var metrics []*stores.RawContainerInsightsMetric
	if containerType == ci.TypeInfraContainer {
		return metrics
	}

	metric := stores.NewRawContainerInsightsMetric(containerType, m.logger)
	metric.ContainerName = info.Name
	curStats := GetStats(info)

	metric.Fields[ci.MetricName(containerType, ci.MemUsage)] = curStats.Memory.Usage
	metric.Fields[ci.MetricName(containerType, ci.MemCache)] = curStats.Memory.Cache
	metric.Fields[ci.MetricName(containerType, ci.MemRss)] = curStats.Memory.RSS
	metric.Fields[ci.MetricName(containerType, ci.MemMaxusage)] = curStats.Memory.MaxUsage
	metric.Fields[ci.MetricName(containerType, ci.MemSwap)] = curStats.Memory.Swap
	metric.Fields[ci.MetricName(containerType, ci.MemFailcnt)] = curStats.Memory.Failcnt
	metric.Fields[ci.MetricName(containerType, ci.MemMappedfile)] = curStats.Memory.MappedFile
	metric.Fields[ci.MetricName(containerType, ci.MemWorkingset)] = curStats.Memory.WorkingSet

	multiplier := float64(time.Second)
	assignRateValueToField(&m.rateCalculator, metric.Fields, ci.MetricName(containerType, ci.MemPgfault), info.Name,
		float64(curStats.Memory.ContainerData.Pgfault), curStats.Timestamp, multiplier)
	assignRateValueToField(&m.rateCalculator, metric.Fields, ci.MetricName(containerType, ci.MemPgmajfault), info.Name,
		float64(curStats.Memory.ContainerData.Pgmajfault), curStats.Timestamp, multiplier)
	assignRateValueToField(&m.rateCalculator, metric.Fields, ci.MetricName(containerType, ci.MemHierarchicalPgfault), info.Name,
		float64(curStats.Memory.HierarchicalData.Pgfault), curStats.Timestamp, multiplier)
	assignRateValueToField(&m.rateCalculator, metric.Fields, ci.MetricName(containerType, ci.MemHierarchicalPgmajfault), info.Name,
		float64(curStats.Memory.HierarchicalData.Pgmajfault), curStats.Timestamp, multiplier)
	memoryFailuresTotal := curStats.Memory.ContainerData.Pgfault + curStats.Memory.ContainerData.Pgmajfault
	assignRateValueToField(&m.rateCalculator, metric.Fields, ci.MetricName(containerType, ci.MemFailuresTotal), info.Name,
		float64(memoryFailuresTotal), curStats.Timestamp, multiplier)

	memoryCapacity := mInfo.GetMemoryCapacity()
	if metric.Fields[ci.MetricName(containerType, ci.MemWorkingset)] != nil && memoryCapacity != 0 {
		metric.Fields[ci.MetricName(containerType, ci.MemUtilization)] = float64(metric.Fields[ci.MetricName(containerType, ci.MemWorkingset)].(uint64)) / float64(memoryCapacity) * 100
	}

	if containerType == ci.TypeNode || containerType == ci.TypeInstance {
		metric.Fields[ci.MetricName(containerType, ci.MemLimit)] = memoryCapacity
	}

	metrics = append(metrics, metric)
	return metrics
}

func (m *MemMetricExtractor) Shutdown() error {
	return m.rateCalculator.Shutdown()
}

func NewMemMetricExtractor(logger *zap.Logger) *MemMetricExtractor {
	return &MemMetricExtractor{
		logger:         logger,
		rateCalculator: newFloat64RateCalculator(),
	}
}
