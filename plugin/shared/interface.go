// Package shared contains shared data between the host and plugins.
package shared

import (
	"context"

	"google.golang.org/grpc"

	"github.com/aergoio/aergo/types"
	plugin "github.com/hashicorp/go-plugin"
)

// ServePlugin is a shortcut for a plugin to serve its implementation of the AergosvrGrpc protocol
func ServePlugin(impl interface{}) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"aergoscan": &AergosvrGrpcPlugin{Impl: impl.(AergosvrInterface)},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}

// Handshake is a common handshake that is shared by plugin and host.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "BASIC_PLUGIN",
	MagicCookieValue: "hello",
}

// PluginMap is the map of plugins we can dispense.
var PluginMap = map[string]plugin.Plugin{
	"aergosvr": &AergosvrGrpcPlugin{},
}

// AergosvrInterface is the interface that we're exposing as a plugin.
type AergosvrInterface interface {
	Init() error
	Start(grpcServerPort uint32) error
	Stop() error
	Receive(input []byte) ([]byte, error)
}

// AergosvrGrpcPlugin is the implementation of plugin.GRPCPlugin so we can serve/consume this.
type AergosvrGrpcPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	Impl AergosvrInterface
}

// GRPCServer returns the server to use for this plugin
func (p *AergosvrGrpcPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	types.RegisterAergoPluginRPCServiceServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

// GRPCClient returns the client to use for this plugin
func (p *AergosvrGrpcPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{client: types.NewAergoPluginRPCServiceClient(c)}, nil
}
