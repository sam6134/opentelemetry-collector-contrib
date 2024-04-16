// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package deltatosparseprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatosparseprocessor"

import (
	"context"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type deltaToSparseProcessor struct {
	*Config
	logger *zap.Logger
}

func newDeltaToSparseProcessor(config *Config, logger *zap.Logger) *deltaToSparseProcessor {
	d := &deltaToSparseProcessor{
		Config: config,
		logger: logger,
	}
	return d
}

var toConvert = map[string]struct{}{
	"node_neuron_execution_errors_generic":                {},
	"node_neuron_execution_errors_numerical":              {},
	"node_neuron_execution_errors_transient":              {},
	"node_neuron_execution_errors_model":                  {},
	"node_neuron_execution_errors_runtime":                {},
	"node_neuron_execution_errors_hardware":               {},
	"node_neuron_execution_status_completed":              {},
	"node_neuron_execution_status_timed_out":              {},
	"node_neuron_execution_status_completed_with_err":     {},
	"node_neuron_execution_status_completed_with_num_err": {},
	"node_neuron_execution_status_incorrect_input":        {},
	"node_neuron_execution_status_failed_to_queue":        {},
	//"node_neuron_execution_errors_total" : {},
	//"container_neurondevice_hw_ecc_events_total" : {},
	//"pod_neurondevice_hw_ecc_events_total" : {},
	//"node_neurondevice_hw_ecc_events_total" : {},
}

func (d *deltaToSparseProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		ilms := rs.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()

			metricsLength := metrics.Len()
			for k := 0; k < metricsLength; k++ {
				m := metrics.At(k)
				if m.Type() == pmetric.MetricTypeSum && m.Sum().IsMonotonic() && m.Sum().AggregationTemporality() == pmetric.AggregationTemporalityDelta {
					if _, exists := toConvert[m.Name()]; exists {
						m.Sum().DataPoints().RemoveIf(func(dp pmetric.NumberDataPoint) bool {
							return dp.DoubleValue() == 0
						})
					}
				}
			}
		}
	}
	return md, nil
}
