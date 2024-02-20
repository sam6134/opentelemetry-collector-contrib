// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	podresourcesv1 "k8s.io/kubelet/pkg/apis/podresources/v1"
	v1 "k8s.io/kubelet/pkg/apis/podresources/v1"
)

var (
	expectedContainerInfoToResourcesMap = map[ContainerInfo][]ResourceInfo{
		{
			podName:       "test-pod",
			containerName: "test-container",
			namespace:     "test-namespace",
		}: {
			{
				resourceName: "test-resource",
				deviceID:     "device-id-1",
			},
			{
				resourceName: "test-resource",
				deviceID:     "device-id-2",
			},
		},
	}

	expectedResourceToPodContainerMap = map[ResourceInfo]ContainerInfo{
		{
			resourceName: "test-resource",
			deviceID:     "device-id-1",
		}: {
			podName:       "test-pod",
			containerName: "test-container",
			namespace:     "test-namespace",
		},
		{
			resourceName: "test-resource",
			deviceID:     "device-id-2",
		}: {
			podName:       "test-pod",
			containerName: "test-container",
			namespace:     "test-namespace",
		},
	}
)

type MockPodResourcesClient struct {
}

func (m *MockPodResourcesClient) ListPods() (*podresourcesv1.ListPodResourcesResponse, error) {
	mockResp := &podresourcesv1.ListPodResourcesResponse{
		PodResources: []*v1.PodResources{
			{
				Name:      "test-pod",
				Namespace: "test-namespace",
				Containers: []*v1.ContainerResources{
					{
						Name: "test-container",
						Devices: []*v1.ContainerDevices{
							{
								ResourceName: "test-resource",
								DeviceIds:    []string{"device-id-1", "device-id-2"},
							},
						},
					},
				},
			},
		},
	}
	return mockResp, nil
}

func TestNewPodResourcesStore(t *testing.T) {
	logger := zap.NewNop()
	store := NewPodResourcesStore(logger)
	assert.NotNil(t, store, "PodResourcesStore should not be nil")
	assert.NotNil(t, store.ctx, "Context should not be nil")
	assert.NotNil(t, store.cancel, "Cancel function should not be nil")
}

func TestRefreshTick(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	store := &PodResourcesStore{
		containerInfoToResourcesMap: make(map[ContainerInfo][]ResourceInfo),
		resourceToPodContainerMap:   make(map[ResourceInfo]ContainerInfo),
		lastRefreshed:               time.Now(),
		ctx:                         context.Background(),
		cancel:                      func() {},
		logger:                      logger,
		podResourcesClient:          &MockPodResourcesClient{},
	}

	store.lastRefreshed = time.Now().Add(-time.Hour)

	store.refreshTick()

	assert.True(t, store.lastRefreshed.After(time.Now().Add(-time.Hour)), "lastRefreshed should have been updated")
}

func TestUpdateMaps(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	store := &PodResourcesStore{
		containerInfoToResourcesMap: make(map[ContainerInfo][]ResourceInfo),
		resourceToPodContainerMap:   make(map[ResourceInfo]ContainerInfo),
		lastRefreshed:               time.Now(),
		ctx:                         context.Background(),
		cancel:                      func() {},
		logger:                      logger,
		podResourcesClient:          &MockPodResourcesClient{},
	}

	store.updateMaps()

	assert.NotNil(t, store.containerInfoToResourcesMap)
	assert.NotNil(t, store.resourceToPodContainerMap)
	assert.Equal(t, len(expectedContainerInfoToResourcesMap), len(store.containerInfoToResourcesMap))
	assert.Equal(t, len(expectedResourceToPodContainerMap), len(store.resourceToPodContainerMap))
	assert.Equal(t, expectedContainerInfoToResourcesMap, store.containerInfoToResourcesMap)
	assert.Equal(t, expectedResourceToPodContainerMap, store.resourceToPodContainerMap)
}
