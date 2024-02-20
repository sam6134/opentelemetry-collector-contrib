// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeletutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores/kubeletutil"

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	socketPath        = "/var/lib/kubelet/pod-resources/kubelet.sock"
	connectionTimeout = 10 * time.Second
)

type PodResourcesClient struct {
	delegateClient podresourcesapi.PodResourcesListerClient
}

func NewPodResourcesClient() (*PodResourcesClient, error) {
	podResourcesClient := &PodResourcesClient{}

	conn, cleanup, err := podResourcesClient.connectToServer(socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer cleanup()

	podResourcesClient.delegateClient = podresourcesapi.NewPodResourcesListerClient(conn)

	return podResourcesClient, nil
}

func (p *PodResourcesClient) connectToServer(socket string) (*grpc.ClientConn, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx,
		socket,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "unix", addr)
		}),
	)

	if err != nil {
		return nil, func() {}, fmt.Errorf("failure connecting to '%s': %w", socket, err)
	}

	return conn, func() { conn.Close() }, nil
}

func (p *PodResourcesClient) ListPods() (*podresourcesapi.ListPodResourcesResponse, error) {
	_, err := os.Stat(socketPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("socket path does not exist: %s", socketPath)
	} else if err != nil {
		return nil, fmt.Errorf("failed to check socket path: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	resp, err := p.delegateClient.List(ctx, &podresourcesapi.ListPodResourcesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failure getting pod resources: %w", err)
	}

	return resp, nil
}
