package stores

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

func TestGetPodResources_Success(t *testing.T) {
	instance = &PodResourcesStore{}

	osStatOrig := osStat
	osStat = func(name string) (os.FileInfo, error) {
		return nil, nil
	}
	defer func() { osStat = osStatOrig }()

	connectToServerOrig := instance.connectToServer
	instance.connectToServer = func(socket string) (*grpc.ClientConn, func(), error) {
		mockClientConn := &grpc.ClientConn{}
		mockCleanup := func() {}
		return mockClientConn, mockCleanup, nil
	}
	defer func() { instance.connectToServer = connectToServerOrig }()

	listPodsOrig := instance.listPods
	mockResponse := &podresourcesapi.ListPodResourcesResponse{}
	instance.listPods = func(conn *grpc.ClientConn) (*podresourcesapi.ListPodResourcesResponse, error) {
		return mockResponse, nil
	}
	defer func() { instance.listPods = listPodsOrig }()

	resp, err := instance.GetPodResources()

	assert.NoError(t, err)
	assert.Equal(t, mockResponse, resp)
}

func TestGetPodResources_Error(t *testing.T) {
	instance = &PodResourcesStore{}

	osStatOrig := osStat
	osStat = func(name string) (os.FileInfo, error) {
		return nil, assert.AnError
	}
	defer func() { osStat = osStatOrig }()

	_, err := instance.GetPodResources()

	assert.Error(t, err)
}

func TestConnectToServer_Error(t *testing.T) {
	instance = &PodResourcesStore{}

	grpcDialContextOrig := grpcDialContext
	grpcDialContext = func(ctx context.Context, target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		return nil, assert.AnError
	}
	defer func() { grpcDialContext = grpcDialContextOrig }()

	_, _, err := instance.connectToServer("dummy-socket")

	assert.Error(t, err)
}

func TestListPods_Error(t *testing.T) {
	instance = &PodResourcesStore{}

	listOrig := clientList
	clientList = func(ctx context.Context, in *podresourcesapi.ListPodResourcesRequest, opts ...grpc.CallOption) (*podresourcesapi.ListPodResourcesResponse, error) {
		return nil, assert.AnError
	}
	defer func() { clientList = listOrig }()

	_, err := instance.listPods(&grpc.ClientConn{})

	assert.Error(t, err)
}
