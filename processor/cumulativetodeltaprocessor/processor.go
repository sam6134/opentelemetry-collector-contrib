// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cumulativetodeltaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"

import (
	"context"
	"fmt"
	"math"
	"strings"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/tracking"
)

type cumulativeToDeltaProcessor struct {
	includeFS       filterset.FilterSet
	excludeFS       filterset.FilterSet
	logger          *zap.Logger
	deltaCalculator *tracking.MetricTracker
	cancelFunc      context.CancelFunc
}

func newCumulativeToDeltaProcessor(config *Config, logger *zap.Logger) *cumulativeToDeltaProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	p := &cumulativeToDeltaProcessor{
		logger:          logger,
		deltaCalculator: tracking.NewMetricTracker(ctx, logger, config.MaxStaleness, config.InitialValue),
		cancelFunc:      cancel,
	}
	if len(config.Include.Metrics) > 0 {
		p.includeFS, _ = filterset.CreateFilterSet(config.Include.Metrics, &config.Include.Config)
	}
	if len(config.Exclude.Metrics) > 0 {
		p.excludeFS, _ = filterset.CreateFilterSet(config.Exclude.Metrics, &config.Exclude.Config)
	}
	return p
}

// processMetrics implements the ProcessMetricsFunc type.
func (ctdp *cumulativeToDeltaProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	ctdp.logMd(md, "before c2d processor")
	md.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		rm.ScopeMetrics().RemoveIf(func(ilm pmetric.ScopeMetrics) bool {
			ilm.Metrics().RemoveIf(func(m pmetric.Metric) bool {
				if !ctdp.shouldConvertMetric(m.Name()) {
					return false
				}
				switch m.Type() {
				case pmetric.MetricTypeSum:
					ms := m.Sum()
					if ms.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						return false
					}

					// Ignore any metrics that aren't monotonic
					if !ms.IsMonotonic() {
						return false
					}

					baseIdentity := tracking.MetricIdentity{
						Resource:               rm.Resource(),
						InstrumentationLibrary: ilm.Scope(),
						MetricType:             m.Type(),
						MetricName:             m.Name(),
						MetricUnit:             m.Unit(),
						MetricIsMonotonic:      ms.IsMonotonic(),
					}
					ctdp.convertDataPoints(ms.DataPoints(), baseIdentity)
					ms.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
					ctdp.logger.Info("This metric got delta'd : " + m.Name())
					return ms.DataPoints().Len() == 0
				case pmetric.MetricTypeHistogram:
					ms := m.Histogram()
					if ms.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						return false
					}

					if ms.DataPoints().Len() == 0 {
						return false
					}

					baseIdentity := tracking.MetricIdentity{
						Resource:               rm.Resource(),
						InstrumentationLibrary: ilm.Scope(),
						MetricType:             m.Type(),
						MetricName:             m.Name(),
						MetricUnit:             m.Unit(),
						MetricIsMonotonic:      true,
						MetricValueType:        pmetric.NumberDataPointValueTypeInt,
					}

					ctdp.convertHistogramDataPoints(ms.DataPoints(), baseIdentity)

					ms.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
					return ms.DataPoints().Len() == 0
				case pmetric.MetricTypeEmpty, pmetric.MetricTypeGauge, pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeSummary:
					fallthrough
				default:
					return false
				}
			})
			return ilm.Metrics().Len() == 0
		})
		return rm.ScopeMetrics().Len() == 0
	})
	ctdp.logMd(md, "after c2d processor")
	return md, nil
}

func (ctdp *cumulativeToDeltaProcessor) shutdown(context.Context) error {
	ctdp.cancelFunc()
	return nil
}

func (ctdp *cumulativeToDeltaProcessor) shouldConvertMetric(metricName string) bool {
	return (ctdp.includeFS == nil || ctdp.includeFS.Matches(metricName)) &&
		(ctdp.excludeFS == nil || !ctdp.excludeFS.Matches(metricName))
}

func (ctdp *cumulativeToDeltaProcessor) convertDataPoints(in any, baseIdentity tracking.MetricIdentity) {
	if dps, ok := in.(pmetric.NumberDataPointSlice); ok {
		dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
			id := baseIdentity
			id.StartTimestamp = dp.StartTimestamp()
			id.Attributes = dp.Attributes()
			id.MetricValueType = dp.ValueType()
			point := tracking.ValuePoint{
				ObservedTimestamp: dp.Timestamp(),
			}

			if dp.Flags().NoRecordedValue() {
				// drop points with no value
				return true
			}
			if id.IsFloatVal() {
				// Do not attempt to transform NaN values
				if math.IsNaN(dp.DoubleValue()) {
					return false
				}
				point.FloatValue = dp.DoubleValue()
			} else {
				point.IntValue = dp.IntValue()
			}
			trackingPoint := tracking.MetricPoint{
				Identity: id,
				Value:    point,
			}
			delta, valid := ctdp.deltaCalculator.Convert(trackingPoint)
			if !valid {
				return true
			}
			dp.SetStartTimestamp(delta.StartTimestamp)
			if id.IsFloatVal() {
				dp.SetDoubleValue(delta.FloatValue)
			} else {
				dp.SetIntValue(delta.IntValue)
			}
			return false
		})
	}
}

func (ctdp *cumulativeToDeltaProcessor) convertHistogramDataPoints(in any, baseIdentity tracking.MetricIdentity) {
	if dps, ok := in.(pmetric.HistogramDataPointSlice); ok {
		dps.RemoveIf(func(dp pmetric.HistogramDataPoint) bool {
			id := baseIdentity
			id.StartTimestamp = dp.StartTimestamp()
			id.Attributes = dp.Attributes()

			if dp.Flags().NoRecordedValue() {
				// drop points with no value
				return true
			}

			point := tracking.ValuePoint{
				ObservedTimestamp: dp.Timestamp(),
				HistogramValue: &tracking.HistogramPoint{
					Count:   dp.Count(),
					Sum:     dp.Sum(),
					Buckets: dp.BucketCounts().AsRaw(),
				},
			}

			trackingPoint := tracking.MetricPoint{
				Identity: id,
				Value:    point,
			}
			delta, valid := ctdp.deltaCalculator.Convert(trackingPoint)

			if valid {
				dp.SetStartTimestamp(delta.StartTimestamp)
				dp.SetCount(delta.HistogramValue.Count)
				if dp.HasSum() && !math.IsNaN(dp.Sum()) {
					dp.SetSum(delta.HistogramValue.Sum)
				}
				dp.BucketCounts().FromRaw(delta.HistogramValue.Buckets)
				dp.RemoveMin()
				dp.RemoveMax()
				return false
			}

			return !valid
		})
	}
}

func (d *cumulativeToDeltaProcessor) logMd(md pmetric.Metrics, name string) {
	var logMessage strings.Builder

	logMessage.WriteString(fmt.Sprintf("\"%s_METRICS_MD\" : {\n", name))
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		ilms := rs.ScopeMetrics()
		logMessage.WriteString(fmt.Sprintf("\t\"ResourceMetric_%d\": {\n", i))
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()
			logMessage.WriteString(fmt.Sprintf("\t\t\"ScopeMetric_%d\": {\n", j))
			logMessage.WriteString(fmt.Sprintf("\t\t\"Metrics_%d\": [\n", j))

			for k := 0; k < metrics.Len(); k++ {
				m := metrics.At(k)
				logMessage.WriteString(fmt.Sprintf("\t\t\t\"Metric_%d\": {\n", k))
				logMessage.WriteString(fmt.Sprintf("\t\t\t\t\"name\": \"%s\",\n", m.Name()))
				logMessage.WriteString(fmt.Sprintf("\t\t\t\t\"type\": \"%s\",\n", m.Type()))

				var datapoints pmetric.NumberDataPointSlice
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					datapoints = m.Gauge().DataPoints()
				case pmetric.MetricTypeSum:

					datapoints = m.Sum().DataPoints()
					logMessage.WriteString(fmt.Sprintf("\t\t\t\t\"aggregation temporality\": \"%s\",\n", m.Sum().AggregationTemporality()))
					logMessage.WriteString(fmt.Sprintf("\t\t\t\t\"ismonotonic\": \"%v\",\n", m.Sum().IsMonotonic()))
				default:
					datapoints = pmetric.NewNumberDataPointSlice()
				}

				logMessage.WriteString("\t\t\t\t\"datapoints\": [\n")
				for yu := 0; yu < datapoints.Len(); yu++ {
					logMessage.WriteString("\t\t\t\t\t{\n")
					logMessage.WriteString(fmt.Sprintf("\t\t\t\t\t\t\"attributes\": \"%v\",\n", datapoints.At(yu).Attributes().AsRaw()))
					logMessage.WriteString(fmt.Sprintf("\t\t\t\t\t\t\"value\": %v,\n", datapoints.At(yu).DoubleValue()))
					logMessage.WriteString(fmt.Sprintf("\t\t\t\t\t\t\"timestamp\": %v,\n", datapoints.At(yu).Timestamp()))
					logMessage.WriteString(fmt.Sprintf("\t\t\t\t\t\t\"flags\": %v,\n", datapoints.At(yu).Flags()))
					logMessage.WriteString(fmt.Sprintf("\t\t\t\t\t\t\"value type\": %v,\n", datapoints.At(yu).ValueType()))
					logMessage.WriteString("\t\t\t\t\t},\n")
				}
				logMessage.WriteString("\t\t\t\t],\n")
				logMessage.WriteString("\t\t\t},\n")
			}
			logMessage.WriteString("\t\t],\n")
			logMessage.WriteString("\t\t},\n")
		}
		logMessage.WriteString("\t},\n")
	}
	logMessage.WriteString("},\n")

	d.logger.Info(logMessage.String())
}
