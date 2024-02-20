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
	v1 "k8s.io/kubelet/pkg/apis/podresources/v1"
)

var (
	expectedContainerInfoToResourcesMap = map[ContainerInfo][]ResourceInfo{
		{ // ContainerInfo
			podName:       "test-pod",
			containerName: "test-container",
			namespace:     "test-namespace",
		}: {
			{ // ResourceInfo
				resourceName: "test-resource",
				deviceID:     "device-id-1",
			},
			{ // ResourceInfo
				resourceName: "test-resource",
				deviceID:     "device-id-2",
			},
		},
	}

	expectedResourceToPodContainerMap = map[ResourceInfo]ContainerInfo{
		{ // ResourceInfo
			resourceName: "test-resource",
			deviceID:     "device-id-1",
		}: { // ContainerInfo
			podName:       "test-pod",
			containerName: "test-container",
			namespace:     "test-namespace",
		},
		{ // ResourceInfo
			resourceName: "test-resource",
			deviceID:     "device-id-2",
		}: { // ContainerInfo
			podName:       "test-pod",
			containerName: "test-container",
			namespace:     "test-namespace",
		},
	}
)

// MockPodResourcesClient is a mock implementation of PodResourcesClient
type MockPodResourcesClient struct {
}

// ListPods mocks the ListPods method of PodResourcesClient
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
	// Test initialization of PodResourcesStore
	logger := zap.NewNop()
	store := NewPodResourcesStore(logger)
	assert.NotNil(t, store, "PodResourcesStore should not be nil")
	assert.NotNil(t, store.ctx, "Context should not be nil")
	assert.NotNil(t, store.cancel, "Cancel function should not be nil")
}

func TestRefreshTick(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create a PodResourcesStore instance with the mocked client and logger
	store := &PodResourcesStore{
		containerInfoToResourcesMap: make(map[ContainerInfo][]ResourceInfo),
		resourceToPodContainerMap:   make(map[ResourceInfo]ContainerInfo),
		lastRefreshed:               time.Now(),
		ctx:                         context.Background(),
		cancel:                      func() {},
		logger:                      logger,
		podResourcesClient:          &MockPodResourcesClient{},
	}

	// Set the lastRefreshed time to an hour ago
	store.lastRefreshed = time.Now().Add(-time.Hour)

	// Call refreshTick
	store.refreshTick()

	// Check if lastRefreshed has been updated
	assert.True(t, store.lastRefreshed.After(time.Now().Add(-time.Hour)), "lastRefreshed should have been updated")
}

func TestUpdateMaps(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create a PodResourcesStore instance with the mocked client and logger
	store := &PodResourcesStore{
		containerInfoToResourcesMap: make(map[ContainerInfo][]ResourceInfo),
		resourceToPodContainerMap:   make(map[ResourceInfo]ContainerInfo),
		lastRefreshed:               time.Now(),
		ctx:                         context.Background(),
		cancel:                      func() {},
		logger:                      logger,
		podResourcesClient:          &MockPodResourcesClient{},
	}

	// Call the updateMaps method
	store.updateMaps()

	// Assert that the maps are updated correctly
	assert.NotNil(t, store.containerInfoToResourcesMap)
	assert.NotNil(t, store.resourceToPodContainerMap)

	// Print containerInfoToResourcesMap
	fmt.Println("containerInfoToResourcesMap:")
	for key, value := range store.containerInfoToResourcesMap {
		fmt.Printf("Key: %+v, Value: %+v\n", key, value)
	}

	// Print resourceToPodContainerMap
	fmt.Println("\nresourceToPodContainerMap:")
	for key, value := range store.resourceToPodContainerMap {
		fmt.Printf("Key: %+v, Value: %+v\n", key, value)
	}

	assert.Equal(t, len(expectedContainerInfoToResourcesMap), len(store.containerInfoToResourcesMap))
	assert.Equal(t, len(expectedResourceToPodContainerMap), len(store.resourceToPodContainerMap))
	assert.Equal(t, expectedContainerInfoToResourcesMap, store.containerInfoToResourcesMap)
	assert.Equal(t, expectedResourceToPodContainerMap, store.resourceToPodContainerMap)
}
