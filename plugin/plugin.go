package plugin

import (
	"os"
	"os/exec"
	"time"

	"github.com/aergoio/aergo-lib/log"
	"github.com/aergoio/aergo/plugin/shared"
	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
)

var logger *log.Logger

func init() {
	logger = log.NewLogger("plugin")
}

func loadAndServePlugin(pluginPath string, grpcServerPort int) {
	logger.Info().Str("path", pluginPath).Msg("Loading plugin")

	// We're a host. Start by launching the plugin process.
	pluginLogger := hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Output: os.Stdout,
		Level:  hclog.Debug,
	})
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  shared.Handshake,
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

	err = pluginInstance.Start(uint32(grpcServerPort))
	if err != nil {
		logger.Error().Err(err).Msg("Could not start plugin")
		return
	}

	result, err := pluginInstance.Receive([]byte{1})
	if err != nil {
		logger.Error().Err(err).Msg("Returned error")
		return
	}
	logger.Info().Int("result", int(result[0])).Msg("Plugin returned result")

	logger.Info().Str("path", pluginPath).Msg("Plugin is ready")

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
