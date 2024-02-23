// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	podresourcesv1 "k8s.io/kubelet/pkg/apis/podresources/v1"
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

	expectedContainerInfo = ContainerInfo{
		podName:       "test-pod",
		containerName: "test-container",
		namespace:     "test-namespace",
	}

	expectedResourceInfo = []ResourceInfo{
		{
			resourceName: "test-resource",
			deviceID:     "device-id-1",
		},
		{
			resourceName: "test-resource",
			deviceID:     "device-id-2",
		},
	}

	listPodResourcesResponse = &podresourcesv1.ListPodResourcesResponse{
		PodResources: []*podresourcesv1.PodResources{
			{
				Name:      "test-pod",
				Namespace: "test-namespace",
				Containers: []*podresourcesv1.ContainerResources{
					{
						Name: "test-container",
						Devices: []*podresourcesv1.ContainerDevices{
							{
								ResourceName: "test-resource",
								DeviceIds:    []string{"device-id-1", "device-id-2"},
							},
						},
					},
				},
			},
			{
				Name:      "test-pod-no-device",
				Namespace: "test-namespace-no-device",
				Containers: []*podresourcesv1.ContainerResources{
					{
						Name:    "test-container-no-device",
						Devices: []*podresourcesv1.ContainerDevices{},
					},
				},
			},
		},
	}

	listPodResourcesResponseEmptyPodResources = &podresourcesv1.ListPodResourcesResponse{
		PodResources: []*podresourcesv1.PodResources{},
	}

	listPodResourcesResponseEmptyResponse = &podresourcesv1.ListPodResourcesResponse{}
)

type MockPodResourcesClient struct {
	response       *podresourcesv1.ListPodResourcesResponse
	err            error
	shutdownCalled bool
}

func (m *MockPodResourcesClient) ListPods() (*podresourcesv1.ListPodResourcesResponse, error) {
	return m.response, m.err
}

func (m *MockPodResourcesClient) Shutdown() {
	m.shutdownCalled = true
}

func TestNewPodResourcesStore(t *testing.T) {
	logger := zap.NewNop()
	store := NewPodResourcesStore(logger)
	assert.NotNil(t, store, "PodResourcesStore should not be nil")
	assert.NotNil(t, store.ctx, "Context should not be nil")
	assert.NotNil(t, store.cancel, "Cancel function should not be nil")
}

func TestRefreshTick(t *testing.T) {
	store := constructPodResourcesStore(make(map[ContainerInfo][]ResourceInfo), make(map[ResourceInfo]ContainerInfo), listPodResourcesResponse, nil)

	store.lastRefreshed = time.Now().Add(-time.Hour)

	store.refreshTick()

	assert.True(t, store.lastRefreshed.After(time.Now().Add(-time.Hour)), "lastRefreshed should have been updated")
}

func TestShutdown(t *testing.T) {
	store := constructPodResourcesStore(make(map[ContainerInfo][]ResourceInfo), make(map[ResourceInfo]ContainerInfo), listPodResourcesResponse, nil)

	mockClient := &MockPodResourcesClient{listPodResourcesResponse, nil, false}
	store.podResourcesClient = mockClient

	store.Shutdown()

	assert.True(t, mockClient.shutdownCalled, "Shutdown method of the client should have been called")
}

func TestUpdateMaps(t *testing.T) {
	store := constructPodResourcesStore(make(map[ContainerInfo][]ResourceInfo), make(map[ResourceInfo]ContainerInfo), listPodResourcesResponse, nil)
	store.updateMaps()

	assert.NotNil(t, store.containerInfoToResourcesMap)
	assert.NotNil(t, store.resourceToPodContainerMap)
	assert.Equal(t, len(expectedContainerInfoToResourcesMap), len(store.containerInfoToResourcesMap))
	assert.Equal(t, len(expectedResourceToPodContainerMap), len(store.resourceToPodContainerMap))
	assert.Equal(t, expectedContainerInfoToResourcesMap, store.containerInfoToResourcesMap)
	assert.Equal(t, expectedResourceToPodContainerMap, store.resourceToPodContainerMap)
}

func TestGets(t *testing.T) {
	store := constructPodResourcesStore(make(map[ContainerInfo][]ResourceInfo), make(map[ResourceInfo]ContainerInfo), listPodResourcesResponse, nil)
	store.updateMaps()

	assert.Equal(t, expectedContainerInfo, *store.GetContainerInfo("device-id-1", "test-resource"))
	assert.Equal(t, expectedResourceInfo, *store.GetResourcesInfo("test-pod", "test-container", "test-namespace"))

	actualResourceInfo := store.GetResourcesInfo("test-pod-no-device", "test-container-no-device", "test-namespace-no-device")
	if actualResourceInfo != nil {
		t.Errorf("Expected GetResourcesInfo to return nil for an unexpected key, but got %v", actualResourceInfo)
	}
}

func TestGetsWhenThereAreNoPods(t *testing.T) {
	store := constructPodResourcesStore(make(map[ContainerInfo][]ResourceInfo), make(map[ResourceInfo]ContainerInfo), listPodResourcesResponseEmptyPodResources, nil)
	store.updateMaps()

	assert.Equal(t, 0, len(store.containerInfoToResourcesMap))
	assert.Equal(t, 0, len(store.resourceToPodContainerMap))

	actualContainerInfo := store.GetContainerInfo("device-id-1", "test-resource")
	if actualContainerInfo != nil {
		t.Errorf("Expected GetContainerInfo to return nil for an unexpected key, but got %v", actualContainerInfo)
	}

	actualResourceInfo := store.GetResourcesInfo("test-pod", "test-container", "test-namespace")
	if actualResourceInfo != nil {
		t.Errorf("Expected GetResourcesInfo to return nil for an unexpected key, but got %v", actualResourceInfo)
	}
}

func TestGetsWhenPodReourcesResponseIsEmpty(t *testing.T) {
	store := constructPodResourcesStore(make(map[ContainerInfo][]ResourceInfo), make(map[ResourceInfo]ContainerInfo), listPodResourcesResponseEmptyResponse, nil)
	store.updateMaps()

	assert.Equal(t, 0, len(store.containerInfoToResourcesMap))
	assert.Equal(t, 0, len(store.resourceToPodContainerMap))

	actualContainerInfo := store.GetContainerInfo("device-id-1", "test-resource")
	if actualContainerInfo != nil {
		t.Errorf("Expected GetContainerInfo to return nil for an unexpected key, but got %v", actualContainerInfo)
	}

	actualResourceInfo := store.GetResourcesInfo("test-pod", "test-container", "test-namespace")
	if actualResourceInfo != nil {
		t.Errorf("Expected GetResourcesInfo to return nil for an unexpected key, but got %v", actualResourceInfo)
	}
}

func TestGetsWhenPodReourcesThrowsError(t *testing.T) {
	store := constructPodResourcesStore(make(map[ContainerInfo][]ResourceInfo), make(map[ResourceInfo]ContainerInfo), listPodResourcesResponseEmptyResponse, fmt.Errorf("mocked behavior"))
	store.updateMaps()

	assert.Equal(t, 0, len(store.containerInfoToResourcesMap))
	assert.Equal(t, 0, len(store.resourceToPodContainerMap))

	actualContainerInfo := store.GetContainerInfo("device-id-1", "test-resource")
	if actualContainerInfo != nil {
		t.Errorf("Expected GetContainerInfo to return nil for an unexpected key, but got %v", actualContainerInfo)
	}

	actualResourceInfo := store.GetResourcesInfo("test-pod", "test-container", "test-namespace")
	if actualResourceInfo != nil {
		t.Errorf("Expected GetResourcesInfo to return nil for an unexpected key, but got %v", actualResourceInfo)
	}
}

func constructPodResourcesStore(containerToDevices map[ContainerInfo][]ResourceInfo, deviceToContainer map[ResourceInfo]ContainerInfo, podResourcesResponse *podresourcesv1.ListPodResourcesResponse, podResourcesError error) *PodResourcesStore {
	logger, _ := zap.NewDevelopment()
	return &PodResourcesStore{
		containerInfoToResourcesMap: containerToDevices,
		resourceToPodContainerMap:   deviceToContainer,
		lastRefreshed:               time.Now(),
		ctx:                         context.Background(),
		cancel:                      func() {},
		logger:                      logger,
		podResourcesClient:          &MockPodResourcesClient{podResourcesResponse, podResourcesError, false},
	}
}
