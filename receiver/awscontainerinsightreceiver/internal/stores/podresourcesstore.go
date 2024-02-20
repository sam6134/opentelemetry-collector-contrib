// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores/kubeletutil"
	"go.uber.org/zap"
	podresourcesv1 "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	socketPath        = "/var/lib/kubelet/pod-resources/kubelet.sock"
	connectionTimeout = 10 * time.Second
	taskTimeout       = 10 * time.Second
)

var (
	instance *PodResourcesStore
	once     sync.Once
)

type ContainerInfo struct {
	podName       string
	containerName string
	namespace     string
}

type ResourceInfo struct {
	resourceName string
	deviceID     string
}

type PodResourcesClientInterface interface {
	ListPods() (*podresourcesv1.ListPodResourcesResponse, error)
}

type PodResourcesStore struct {
	containerInfoToResourcesMap map[ContainerInfo][]ResourceInfo
	resourceToPodContainerMap   map[ResourceInfo]ContainerInfo
	lastRefreshed               time.Time
	ctx                         context.Context
	cancel                      context.CancelFunc
	logger                      *zap.Logger
	podResourcesClient          PodResourcesClientInterface
}

func NewPodResourcesStore(logger *zap.Logger) *PodResourcesStore {
	once.Do(func() {
		podResourcesClient, _ := kubeletutil.NewPodResourcesClient()
		ctx, cancel := context.WithCancel(context.Background())
		instance = &PodResourcesStore{
			containerInfoToResourcesMap: make(map[ContainerInfo][]ResourceInfo),
			resourceToPodContainerMap:   make(map[ResourceInfo]ContainerInfo),
			lastRefreshed:               time.Now(),
			ctx:                         ctx,
			cancel:                      cancel,
			logger:                      logger,
			podResourcesClient:          podResourcesClient,
		}

		go func() {
			refreshTicker := time.NewTicker(time.Second)
			for {
				select {
				case <-refreshTicker.C:
					instance.refreshTick()
				case <-instance.ctx.Done():
					refreshTicker.Stop()
					return
				}
			}
		}()
	})
	return instance
}

func (p *PodResourcesStore) refreshTick() {
	now := time.Now()
	if now.Sub(p.lastRefreshed) >= taskTimeout {
		p.refresh()
		p.lastRefreshed = now
	}
}

func (p *PodResourcesStore) refresh() {
	doRefresh := func() {
		p.updateMaps()
	}

	refreshWithTimeout(p.ctx, doRefresh, taskTimeout)
}

func (p *PodResourcesStore) updateMaps() {
	p.containerInfoToResourcesMap = make(map[ContainerInfo][]ResourceInfo)
	p.resourceToPodContainerMap = make(map[ResourceInfo]ContainerInfo)

	devicePods, err := p.podResourcesClient.ListPods()

	if err != nil {
		p.logger.Error(fmt.Sprintf("Error getting pod resources: %v", err))
		return
	}

	for _, pod := range devicePods.GetPodResources() {
		for _, container := range pod.GetContainers() {
			for _, device := range container.GetDevices() {

				containerInfo := ContainerInfo{
					podName:       pod.GetName(),
					namespace:     pod.GetNamespace(),
					containerName: container.GetName(),
				}

				for _, deviceID := range device.GetDeviceIds() {
					resourceInfo := ResourceInfo{
						resourceName: device.GetResourceName(),
						deviceID:     deviceID,
					}
					p.containerInfoToResourcesMap[containerInfo] = append(p.containerInfoToResourcesMap[containerInfo], resourceInfo)
					p.resourceToPodContainerMap[resourceInfo] = containerInfo
				}
			}
		}
	}
}
