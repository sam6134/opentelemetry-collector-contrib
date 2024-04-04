// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package neuron

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	statusType     = "status_type"
	errorType      = "error_type"
	memoryLocation = "memory_location"
	percentile     = "percentile"
)

var attributeConfig = map[string][]string{
	NeuronExecutionStatus:                       {statusType},
	NeuronExecutionErrors:                       {errorType},
	NeuronRuntimeMemoryUsage:                    {memoryLocation},
	NeuronExecutionLatency:                      {percentile},
	NeuronCoreUtilization:                       {neuronCoreAttributeKey, neuronDeviceAttributeKey},
	NeuronCoreMemoryUtilizationConstants:        {neuronCoreAttributeKey, neuronDeviceAttributeKey},
	NeuronCoreMemoryUtilizationModelCode:        {neuronCoreAttributeKey, neuronDeviceAttributeKey},
	NeuronCoreMemoryUtilizationSharedScratchpad: {neuronCoreAttributeKey, neuronDeviceAttributeKey},
	NeuronCoreMemoryUtilizationRuntimeMemory:    {neuronCoreAttributeKey, neuronDeviceAttributeKey},
	NeuronCoreMemoryUtilizationTensors:          {neuronCoreAttributeKey, neuronDeviceAttributeKey},
}

var nonCoreAttributeValues = map[string]string{
	statusType:     "completed",
	errorType:      "generic",
	memoryLocation: "neuron_device",
	percentile:     "p50",
}

// The decorator is used to add metric with zero dataPoint values, if not present
// This allows non-sparse metrics in cases when neuron monitor is not running
type EmptyMetricDecorator struct {
	NextConsumer consumer.Metrics
	Logger       *zap.Logger
}

func (ed *EmptyMetricDecorator) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (ed *EmptyMetricDecorator) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	ed.logMd(md, "EmptyMetricDecorator: before adding empty metrics")
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		ilms := rs.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()

			neuronHardwareInfo, neuronHardwareInfoFound := findNeuronHardwareInfo(metrics)
			if neuronHardwareInfoFound {
				ed.addEmptyMetrics(neuronHardwareInfo, metrics)
				ed.Logger.Info(fmt.Sprintf("Hardware Info found with attributes : %v at %s", neuronHardwareInfo.Sum().DataPoints().At(0).Attributes().AsRaw(), neuronHardwareInfo.Sum().DataPoints().At(0).Timestamp().String()))
			}
		}
	}
	ed.logMd(md, "EmptyMetricDecorator: after adding empty metrics")
	return ed.NextConsumer.ConsumeMetrics(ctx, md)
}

func (ed *EmptyMetricDecorator) addEmptyMetrics(hardwareInfo pmetric.Metric, metrics pmetric.MetricSlice) {
	var metricFoundMap = make(map[string]bool)
	for k := range attributeConfig {
		metricFoundMap[k] = false
	}

	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		if _, ok := metricFoundMap[m.Name()]; ok {
			if m.Name() == "execution_latency_seconds" {
				var logMessage strings.Builder
				logMessage.WriteString("execution_latency_seconds \n")
				logMessage.WriteString("type: " + m.Type().String() + "\n")
				logMessage.WriteString(fmt.Sprintf("datapoints len: %d \n", m.Gauge().DataPoints().Len()))
				datapoints := m.Gauge().DataPoints()
				if datapoints.Len() > 0 {
					for i := 0; i < datapoints.Len(); i++ {
						logMessage.WriteString(fmt.Sprintf("datapoint %d: %v %f \n", i, datapoints.At(i).Attributes().AsRaw(), datapoints.At(i).DoubleValue()))
					}
				}
				ed.Logger.Info(logMessage.String())
				logMessage.Reset()
			}
			metricFoundMap[m.Name()] = true
		}
	}
	ed.Logger.Info(fmt.Sprintf("metrics found map : %v", metricFoundMap))
	for k, v := range metricFoundMap {
		if v {
			continue
		}
		if strings.Contains(k, "core") {
			populateCoreMetrics(metrics, k, hardwareInfo)
		} else {
			populateNonCoreMetrics(metrics, k, attributeConfig[k], hardwareInfo)
		}
	}
}

// method populates per non-core metrics, thus empty metrics are added only once per node
func populateNonCoreMetrics(metrics pmetric.MetricSlice, metricName string, attributesToAdd []string, hardwareInfo pmetric.Metric) {
	metricToAdd := createNewMetricFromHardwareInfo(hardwareInfo, metricName)
	metricBody := metricToAdd.Gauge().DataPoints().At(0)

	for _, attribute := range attributesToAdd {
		metricBody.Attributes().PutStr(attribute, nonCoreAttributeValues[attribute])
	}

	metricToAdd.CopyTo(metrics.AppendEmpty())
}

// method populates per core metrics, thus empty metrics are added per core
func populateCoreMetrics(metrics pmetric.MetricSlice, metricName string, hardwareInfo pmetric.Metric) {
	neuronCoresPerDevice, foundCoresPerDevice := getNeuronCoresPerDevice(hardwareInfo)
	neuronDeviceCount, foundDeviceCount := getNeuronDeviceCount(hardwareInfo)
	if !(foundCoresPerDevice && foundDeviceCount) {
		return
	}

	for coreIndex := 0; coreIndex < neuronCoresPerDevice*neuronDeviceCount; coreIndex++ {
		metricToAdd := createNewMetricFromHardwareInfo(hardwareInfo, metricName)
		metricBody := metricToAdd.Gauge().DataPoints().At(0)

		metricBody.Attributes().PutStr(neuronCoreAttributeKey, strconv.Itoa(coreIndex))
		metricBody.Attributes().PutStr(neuronDeviceAttributeKey, strconv.Itoa(coreIndex/neuronCoresPerDevice))
		metricToAdd.CopyTo(metrics.AppendEmpty())
	}

}

// returns the device count for neuron from the hardwareInfo metric
// https://awsdocs-neuron.readthedocs-hosted.com/en/latest/tools/neuron-sys-tools/neuron-monitor-user-guide.html#neuron-hw-counters
func getNeuronDeviceCount(hardwareInfo pmetric.Metric) (int, bool) {
	neuronCoreHardwareInfoDatapoints := hardwareInfo.Sum().DataPoints()
	if neuronCoreHardwareInfoDatapoints.Len() > 0 {
		neuronDeviceCountValue, found := neuronCoreHardwareInfoDatapoints.At(0).Attributes().Get(neuronDeviceCountAttributeKey)
		if found {
			neuronDeviceCount, _ := strconv.Atoi(neuronDeviceCountValue.AsString())
			return neuronDeviceCount, true
		}
	}
	return -1, false
}

// returns a empty gauge metric with all attributes of hardwareInfo metric copied
func createNewMetricFromHardwareInfo(hardwareInfo pmetric.Metric, metricName string) pmetric.Metric {
	metricToAdd := pmetric.NewMetric()
	metricToAdd.SetEmptyGauge()
	hardwareInfo.Sum().DataPoints().CopyTo(metricToAdd.Gauge().DataPoints())

	metricToAdd.SetName(metricName)
	metricBody := metricToAdd.Gauge().DataPoints().At(0)
	metricBody.SetDoubleValue(0)
	metricBody.Attributes().PutStr("runtime_tag", "default")

	return metricToAdd
}
func (d *EmptyMetricDecorator) logMd(md pmetric.Metrics, name string) {
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

				var datapoints pmetric.NumberDataPointSlice
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					datapoints = m.Gauge().DataPoints()
				case pmetric.MetricTypeSum:
					datapoints = m.Sum().DataPoints()
				default:
					datapoints = pmetric.NewNumberDataPointSlice()
				}

				logMessage.WriteString("\t\t\t\t\"datapoints\": [\n")
				for yu := 0; yu < datapoints.Len(); yu++ {
					logMessage.WriteString("\t\t\t\t\t{\n")
					logMessage.WriteString(fmt.Sprintf("\t\t\t\t\t\t\"attributes\": \"%v\",\n", datapoints.At(yu).Attributes().AsRaw()))
					logMessage.WriteString(fmt.Sprintf("\t\t\t\t\t\t\"value\": %v,\n", datapoints.At(yu).DoubleValue()))
					logMessage.WriteString(fmt.Sprintf("\t\t\t\t\t\t\"timestamp\": %s,\n", datapoints.At(yu).Timestamp().String()))
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

	d.Logger.Info(logMessage.String())
}
