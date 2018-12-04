// Package contains logic to translate from and to GRPC requests
package shared

import (
	"context"
	"fmt"

	"github.com/aergoio/aergo/types"
	"google.golang.org/grpc"
)

// GRPCClient wraps the Impl so that the consumer does not have to deal with the GRPC details
type GRPCClient struct {
	client types.AergoPluginRPCServiceClient
}

func (m *GRPCClient) Init() error {
	_, err := m.client.Init(context.Background(), &types.Empty{})
	return err
}

func (m *GRPCClient) Start(grpcServerPort uint32) error {
	_, err := m.client.Start(context.Background(), &types.PluginStartRequest{
		GrpcServerPort: grpcServerPort,
	})
	return err
}

func (m *GRPCClient) Stop() error {
	_, err := m.client.Stop(context.Background(), &types.Empty{})
	return err
}

func (m *GRPCClient) Receive(value []byte) ([]byte, error) {
	resp, err := m.client.Receive(context.Background(), &types.SingleBytes{Value: value})
	if err != nil {
		return nil, err
	}

	return resp.Value, nil
}

// GRPCServer wraps the Impl so that the consumer does not have to deal with the GRPC details
type GRPCServer struct {
	Impl   AergosvrInterface
	server *grpc.Server
}

func (m *GRPCServer) Init(
	ctx context.Context,
	empty *types.Empty) (*types.Empty, error) {
	return &types.Empty{}, m.Impl.Init()
}

func (m *GRPCServer) Start(
	ctx context.Context,
	pluginStartRequest *types.PluginStartRequest) (*types.Empty, error) {
	serverAddr := fmt.Sprintf("%s:%d", "127.0.0.1", pluginStartRequest.GrpcServerPort)
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil || conn == nil {
		return &types.Empty{}, m.Impl.Start(nil)
	}
	client := types.NewAergoRPCServiceClient(conn)
	return &types.Empty{}, m.Impl.Start(client)
}

func (m *GRPCServer) Stop(
	ctx context.Context,
	empty *types.Empty) (*types.Empty, error) {
	m.server.Stop()
	return &types.Empty{}, m.Impl.Stop()
}

func (m *GRPCServer) Receive(
	ctx context.Context,
	input *types.SingleBytes) (*types.SingleBytes, error) {
	v, err := m.Impl.Receive(input.Value)
	return &types.SingleBytes{Value: v}, err
}
