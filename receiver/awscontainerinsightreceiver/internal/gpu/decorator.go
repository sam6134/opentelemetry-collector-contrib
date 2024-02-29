// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gpu

import (
	"context"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	gpuUtil        = "DCGM_FI_DEV_GPU_UTIL"
	gpuMemUtil     = "DCGM_FI_DEV_FB_USED_PERCENT"
	gpuMemUsed     = "DCGM_FI_DEV_FB_USED"
	gpuMemTotal    = "DCGM_FI_DEV_FB_TOTAL"
	gpuTemperature = "DCGM_FI_DEV_GPU_TEMP"
	gpuPowerDraw   = "DCGM_FI_DEV_POWER_USAGE"
)

var metricToUnit = map[string]string{
	gpuUtil:        "Percent",
	gpuMemUtil:     "Percent",
	gpuMemUsed:     "Bytes",
	gpuMemTotal:    "Bytes",
	gpuTemperature: "None",
	gpuPowerDraw:   "None",
}

// GPU decorator acts as an interceptor of metrics before the scraper sends them to the next designated consumer
type decorateConsumer struct {
	containerOrchestrator string
	nextConsumer          consumer.Metrics
	k8sDecorator          Decorator
	logger                *zap.Logger
}

func (dc *decorateConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (dc *decorateConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	resourceTags := make(map[string]string)
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		// get resource attributes
		ras := rms.At(i).Resource().Attributes()
		ras.Range(func(k string, v pcommon.Value) bool {
			resourceTags[k] = v.AsString()
			return true
		})
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ms := ilms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				fields, tags := ci.ConvertToFieldsAndTags(m, dc.logger)
				// copy down resource metrics only when it's missing at datapoint
				for rtk, rtv := range resourceTags {
					if _, ok := tags[rtk]; !ok {
						tags[rtk] = rtv
					}
				}
				cim := extractors.NewRawContainerInsightsMetric(ci.TypeGpuContainer, dc.logger)
				cim.Fields = fields
				cim.Tags = tags
				if !cim.HasTag(ci.MetricType) {
					// force type to be Container to decorate with container level labels
					cim.AddTag(ci.MetricType, ci.TypeGpuContainer)
				}
				dc.decorateMetrics([]*extractors.RawContainerInsightsMetric{cim})
				dc.updateAttributes(m, cim)
				if unit, ok := metricToUnit[m.Name()]; ok {
					m.SetUnit(unit)
				}
			}
		}
	}
	return dc.nextConsumer.ConsumeMetrics(ctx, md)
}

type Decorator interface {
	Decorate(stores.CIMetric) stores.CIMetric
	Shutdown() error
}

func (dc *decorateConsumer) decorateMetrics(metrics []*extractors.RawContainerInsightsMetric) []*extractors.RawContainerInsightsMetric {
	var result []*extractors.RawContainerInsightsMetric
	for _, m := range metrics {
		// add tags for EKS
		if dc.containerOrchestrator == ci.EKS {
			out := dc.k8sDecorator.Decorate(m)
			if out != nil {
				result = append(result, out.(*extractors.RawContainerInsightsMetric))
			}
		}
	}
	return result
}

func (dc *decorateConsumer) updateAttributes(m pmetric.Metric, cim *extractors.RawContainerInsightsMetric) {
	if len(cim.Tags) == 0 {
		return
	}
	var dps pmetric.NumberDataPointSlice
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps = m.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dps = m.Sum().DataPoints()
	default:
		dc.logger.Warn("Unsupported metric type", zap.String("metric", m.Name()), zap.String("type", m.Type().String()))
	}

	if dps.Len() == 0 {
		return
	}
	attrs := dps.At(0).Attributes()
	for tk, tv := range cim.Tags {
		// type gets set with metrictransformer while duplicating metrics at different resource levels
		if tk == ci.MetricType {
			continue
		}
		attrs.PutStr(tk, tv)
	}
}

func (dc *decorateConsumer) Shutdown() error {
	if dc.k8sDecorator != nil {
		return dc.k8sDecorator.Shutdown()
	}
	return nil
}
