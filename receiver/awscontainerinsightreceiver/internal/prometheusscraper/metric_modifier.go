package prometheusscraper

import (
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	neuronCoreAttributeKey      = "neuroncore"
	neuronDeviceAttributeKey    = "neuron_device_index"
	neuronCoreResourceName      = "aws.amazon.com/neuroncore"
	neuronDeviceResourceName    = "aws.amazon.com/neurondevice"
	neuronDeviceResourceNameAlt = "aws.amazon.com/neuron"
)

type MetricModifier struct {
	logger            *zap.Logger
	podResourcesStore *stores.PodResourcesStore // replace with podResourcesApi
}

func NewMetricModifier(logger *zap.Logger, podResourcesStore *stores.PodResourcesStore) *MetricModifier {
	d := &MetricModifier{
		logger:            logger,
		podResourcesStore: podResourcesStore,
	}
	return d
}

func (d *MetricModifier) AddPodCorrelationAttributes(metricDatapoints pmetric.NumberDataPointSlice, neuronCoresPerDevice int64) {
	for i := 0; i < metricDatapoints.Len(); i++ {
		attributes := metricDatapoints.At(i).Attributes()
		neuronCoreIndex, neuronCoreIndexPresent := attributes.Get(neuronCoreAttributeKey)
		if neuronCoreIndexPresent {
			neuronDeviceIndex := neuronCoreIndex.Int() / neuronCoresPerDevice
			neuronDeviceIndexString := strconv.FormatInt(neuronDeviceIndex, 10)
			neuronCoreIndexString := strconv.FormatInt(neuronCoreIndex.Int(), 10)

			containerInfo := d.podResourcesStore.GetContainerInfo(neuronCoreIndexString, neuronCoreResourceName)
			if containerInfo == nil {
				containerInfo = d.podResourcesStore.GetContainerInfo(neuronDeviceIndexString, neuronDeviceResourceName)
				if containerInfo == nil {
					// Alt resource name is to support backward compatibility in neuron monitor : https://awsdocs-neuron.readthedocs-hosted.com/en/latest/containers/tutorials/k8s-setup.html
					containerInfo = d.podResourcesStore.GetContainerInfo(neuronDeviceIndexString, neuronDeviceResourceNameAlt)
				}
			}
			attributes.PutStr(neuronDeviceAttributeKey, strconv.FormatInt(neuronDeviceIndex, 10))

			if containerInfo != nil {
				attributes.PutStr("ContainerName", containerInfo.ContainerName)
				attributes.PutStr("PodName", containerInfo.PodName)
				attributes.PutStr("Namespace", containerInfo.Namespace)
				attributes.PutStr("FullPodname", containerInfo.PodName+"."+containerInfo.Namespace)
			}
		} else {
			neuronDeviceIndex, neuronDeviceIndexPresent := attributes.Get(neuronDeviceAttributeKey)
			neuronDeviceIndexString := strconv.FormatInt(neuronDeviceIndex.Int(), 10)
			if neuronDeviceIndexPresent {
				containerInfo := d.podResourcesStore.GetContainerInfo(neuronDeviceIndexString, neuronDeviceResourceName)
				if containerInfo == nil {
					// Alt resource name is to support backward compatibility in neuron monitor : https://awsdocs-neuron.readthedocs-hosted.com/en/latest/containers/tutorials/k8s-setup.html
					containerInfo = d.podResourcesStore.GetContainerInfo(neuronDeviceIndexString, neuronDeviceResourceNameAlt)
				}

				if containerInfo != nil {
					attributes.PutStr("ContainerName", containerInfo.ContainerName)
					attributes.PutStr("PodName", containerInfo.PodName)
					attributes.PutStr("Namespace", containerInfo.Namespace)
					attributes.PutStr("FullPodname", containerInfo.PodName+"."+containerInfo.Namespace)
				}
			}
		}

	}
}
