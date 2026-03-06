package ciscotelemetryreceiver

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

// TypeStr defines the receiver type identifier.
const TypeStr = "cisco_telemetry"

// NewFactory creates a factory for the Cisco Telemetry receiver.
func NewFactory() receiver.Factory {
	typeStr := component.MustNewType(TypeStr)
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ListenAddress: "0.0.0.0:57500",

		// No TLS by default – set to non-nil configtls.ServerConfig in
		// collector-config.yaml to enable mTLS.
		TLS: nil,

		MaxRecvMsgSizeMiB:    4, // 4 MiB
		MaxConcurrentStreams: 128,

		KeepAlive: KeepAliveConfig{
			Time:                      30 * time.Second,
			Timeout:                   10 * time.Second,
			EnforcementMinTime:        30 * time.Second,
			EnforcementPermitNoStream: true,
		},

		YANG: YANGConfig{
			EnableRFCParser: true,
			CacheModules:    true,
			MaxModules:      1000,
		},
	}
}

func createMetricsReceiver(
	ctx context.Context,
	params receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: %T", cfg)
	}

	return newCiscoTelemetryReceiver(config, params, consumer)
}
