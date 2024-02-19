package stores

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	socketPath        = "/var/lib/kubelet/pod-resources/kubelet.sock"
	connectionTimeout = 10 * time.Second
)

var (
	instance *PodResourcesStore
	once     sync.Once
)

type PodResourcesStore struct {
}

func init() {
	once.Do(func() {
		instance = &PodResourcesStore{}
	})
}

func (p *PodResourcesStore) GetPodResources() (*podresourcesapi.ListPodResourcesResponse, error) {
	_, err := os.Stat(socketPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("socket path does not exist: %s", socketPath)
	} else if err != nil {
		return nil, fmt.Errorf("failed to check socket path: %v", err)
	}

	conn, cleanup, err := p.connectToServer(socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %v", err)
	}
	defer cleanup()

	return p.listPods(conn)
}

func (p *PodResourcesStore) connectToServer(socket string) (*grpc.ClientConn, func(), error) {
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
		return nil, func() {}, fmt.Errorf("failure connecting to '%s': %v", socket, err)
	}

	return conn, func() { conn.Close() }, nil
}

func (p *PodResourcesStore) listPods(conn *grpc.ClientConn) (*podresourcesapi.ListPodResourcesResponse, error) {
	client := podresourcesapi.NewPodResourcesListerClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	resp, err := client.List(ctx, &podresourcesapi.ListPodResourcesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failure getting pod resources: %v", err)
	}

	return resp, nil
}
