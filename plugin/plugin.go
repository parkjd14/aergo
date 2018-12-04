package plugin

import (
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/aergoio/aergo-lib/log"
	"github.com/aergoio/aergo/plugin/shared"
	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
)

var logger *log.Logger
var pluginInstances map[string]*shared.GRPCClient

func init() {
	logger = log.NewLogger("plugin")
	pluginInstances = make(map[string]*shared.GRPCClient)
}

func loadAndServePlugin(pluginPath string, grpcServerPort int) {
	logger.Info().Str("path", pluginPath).Msg("Loading plugin")

	// We're a host. Start by launching the plugin process.
	pluginLogger := hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Output: os.Stdout,
		Level:  hclog.Debug,
	})
	pluginName := path.Base(pluginPath) // TODO: document or find better way to pass plugin name
	var Handshake = plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "BASIC_PLUGIN",
		MagicCookieValue: pluginName,
	}
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  Handshake,
		Plugins:          shared.PluginMap,
		Cmd:              exec.Command("sh", "-c", pluginPath),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           pluginLogger,
	})
	defer client.Kill()

	pluginLogger.Debug("test")

	// Connect to client
	rpcClient, err := client.Client()
	if err != nil {
		logger.Error().Err(err).Msg("Could not connect to plugin")
		return
	}

	// Request the plugin
	raw, err := rpcClient.Dispense("aergosvr")
	if err != nil {
		logger.Error().Err(err).Msg("Could not dispense plugin")
		return
	}

	// Call Init function of plugin
	pluginInstance := raw.(*shared.GRPCClient)
	err = pluginInstance.Init()
	if err != nil {
		logger.Error().Err(err).Msg("Could not initialize plugin")
		return
	}

	// Call Start function of plugin with aergo grpc port
	err = pluginInstance.Start(uint32(grpcServerPort))
	if err != nil {
		logger.Error().Err(err).Msg("Could not start plugin")
		return
	}

	logger.Info().Str("path", pluginPath).Msg("Plugin is ready")
	pluginInstances[pluginName] = pluginInstance

	for {
		time.Sleep(time.Minute)
	}
}

// LoadAndServePlugins loads the plugins specified by paths
func LoadAndServePlugins(pluginPaths []string, grpcServerPort int) {
	for _, pluginPath := range pluginPaths {
		go loadAndServePlugin(pluginPath, grpcServerPort)
	}
}

func StopPlugins() {
	for pluginName, pluginInstance := range pluginInstances {
		err := pluginInstance.Stop()
		if err != nil {
			logger.Error().Err(err).Str("plugin", pluginName).Msg("Failed to stop plugin")
		}
	}
}
