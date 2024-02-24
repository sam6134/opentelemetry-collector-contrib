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

const (
	defaultResourceName          = "Resource-1"
	defaultPodName               = "Pod-1"
	defaultNamespace             = "Namespace-1"
	defaultContainerName         = "Container-1"
	defaultDeviceID1             = "Device-1"
	defaultDeviceID2             = "Device-2"
	defaultDeviceID3             = "Device-3"
	defaultDeviceID4             = "Device-4"
	defaultResourceNameSkipped   = "Resource-Skipped"
	defaultContainerNameNoDevice = "Container-NoDevice"
	defaultNamespaceNoDevice     = "Namespace-NoDevice"
	defaultPodNameNoDevice       = "Pod-NoDevice"
)

var (
	expectedContainerInfoToResourcesMap = map[ContainerInfo][]ResourceInfo{
		{
			podName:       defaultPodName,
			containerName: defaultContainerName,
			namespace:     defaultNamespace,
		}: {
			{
				resourceName: defaultResourceName,
				deviceID:     defaultDeviceID1,
			},
			{
				resourceName: defaultResourceName,
				deviceID:     defaultDeviceID2,
			},
		},
	}

	expectedResourceToPodContainerMap = map[ResourceInfo]ContainerInfo{
		{
			resourceName: defaultResourceName,
			deviceID:     defaultDeviceID1,
		}: {
			podName:       defaultPodName,
			containerName: defaultContainerName,
			namespace:     defaultNamespace,
		},
		{
			resourceName: defaultResourceName,
			deviceID:     defaultDeviceID2,
		}: {
			podName:       defaultPodName,
			containerName: defaultContainerName,
			namespace:     defaultNamespace,
		},
	}

	expectedContainerInfo = ContainerInfo{
		podName:       defaultPodName,
		containerName: defaultContainerName,
		namespace:     defaultNamespace,
	}

	expectedResourceInfo = []ResourceInfo{
		{
			resourceName: defaultResourceName,
			deviceID:     defaultDeviceID1,
		},
		{
			resourceName: defaultResourceName,
			deviceID:     defaultDeviceID2,
		},
	}

	listPodResourcesResponse = &podresourcesv1.ListPodResourcesResponse{
		PodResources: []*podresourcesv1.PodResources{
			{
				Name:      defaultPodName,
				Namespace: defaultNamespace,
				Containers: []*podresourcesv1.ContainerResources{
					{
						Name: defaultContainerName,
						Devices: []*podresourcesv1.ContainerDevices{
							{
								ResourceName: defaultResourceName,
								DeviceIds:    []string{defaultDeviceID1, defaultDeviceID2},
							},
							{
								ResourceName: defaultResourceNameSkipped,
								DeviceIds:    []string{defaultDeviceID3, defaultDeviceID4},
							},
						},
					},
				},
			},
			{
				Name:      defaultPodNameNoDevice,
				Namespace: defaultNamespaceNoDevice,
				Containers: []*podresourcesv1.ContainerResources{
					{
						Name:    defaultContainerNameNoDevice,
						Devices: []*podresourcesv1.ContainerDevices{},
					},
				},
			},
		},
	}

	listPodResourcesResponseWithEmptyPodResources = &podresourcesv1.ListPodResourcesResponse{
		PodResources: []*podresourcesv1.PodResources{},
	}

	listPodResourcesResponseWithEmptyResponse = &podresourcesv1.ListPodResourcesResponse{}

	resourceNameSet = map[string]struct{}{
		defaultResourceName: {},
	}
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

	assertMapsContainData(t, store)
}

func TestGetsWhenThereAreNoPods(t *testing.T) {
	store := constructPodResourcesStore(make(map[ContainerInfo][]ResourceInfo), make(map[ResourceInfo]ContainerInfo), listPodResourcesResponseWithEmptyPodResources, nil)
	store.updateMaps()

	assertMapsDontContainData(t, store)
}

func TestGetsWhenPodResourcesResponseIsEmpty(t *testing.T) {
	store := constructPodResourcesStore(make(map[ContainerInfo][]ResourceInfo), make(map[ResourceInfo]ContainerInfo), listPodResourcesResponseWithEmptyResponse, nil)
	store.updateMaps()

	assertMapsDontContainData(t, store)
}

func TestGetsWhenPodResourcesThrowsError(t *testing.T) {
	store := constructPodResourcesStore(make(map[ContainerInfo][]ResourceInfo), make(map[ResourceInfo]ContainerInfo), listPodResourcesResponseWithEmptyResponse, fmt.Errorf("mocked behavior"))
	store.updateMaps()

	assertMapsDontContainData(t, store)
}

func TestAddResourceName(t *testing.T) {
	store := constructPodResourcesStore(make(map[ContainerInfo][]ResourceInfo), make(map[ResourceInfo]ContainerInfo), listPodResourcesResponse, nil)

	store.resourceNameSet = make(map[string]struct{})
	store.updateMaps()
	assertMapsDontContainData(t, store)

	// After adding resource to map
	store.AddResourceName(defaultResourceName)
	store.updateMaps()
	assertMapsContainData(t, store)
}

func constructPodResourcesStore(containerToDevices map[ContainerInfo][]ResourceInfo, deviceToContainer map[ResourceInfo]ContainerInfo, podResourcesResponse *podresourcesv1.ListPodResourcesResponse, podResourcesError error) *PodResourcesStore {
	logger, _ := zap.NewDevelopment()
	return &PodResourcesStore{
		containerInfoToResourcesMap: containerToDevices,
		resourceToPodContainerMap:   deviceToContainer,
		resourceNameSet:             resourceNameSet,
		lastRefreshed:               time.Now(),
		ctx:                         context.Background(),
		cancel:                      func() {},
		logger:                      logger,
		podResourcesClient:          &MockPodResourcesClient{podResourcesResponse, podResourcesError, false},
	}
}

func assertMapsContainData(t *testing.T, store *PodResourcesStore) {
	assert.Equal(t, len(expectedContainerInfoToResourcesMap), len(store.containerInfoToResourcesMap))
	assert.Equal(t, len(expectedResourceToPodContainerMap), len(store.resourceToPodContainerMap))

	assert.Equal(t, expectedContainerInfo, *store.GetContainerInfo(defaultDeviceID1, defaultResourceName))
	assert.Equal(t, expectedResourceInfo, *store.GetResourcesInfo(defaultPodName, defaultContainerName, defaultNamespace))

	actualResourceInfo := store.GetResourcesInfo(defaultPodNameNoDevice, defaultContainerNameNoDevice, defaultNamespaceNoDevice)
	if actualResourceInfo != nil {
		t.Errorf("Expected GetResourcesInfo to return nil for an unexpected key, but got %v", actualResourceInfo)
	}
}

func assertMapsDontContainData(t *testing.T, store *PodResourcesStore) {
	assert.Equal(t, 0, len(store.containerInfoToResourcesMap))
	assert.Equal(t, 0, len(store.resourceToPodContainerMap))

	actualContainerInfo := store.GetContainerInfo(defaultDeviceID1, defaultResourceName)
	if actualContainerInfo != nil {
		t.Errorf("Expected GetContainerInfo to return nil for an unexpected key, but got %v", actualContainerInfo)
	}

	actualResourceInfo := store.GetResourcesInfo(defaultPodName, defaultContainerName, defaultNamespace)
	if actualResourceInfo != nil {
		t.Errorf("Expected GetResourcesInfo to return nil for an unexpected key, but got %v", actualResourceInfo)
	}
}
