package ciscotelemetryreceiver

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
)

// createValidTestConfig creates a config suitable for testing
func createValidTestConfig() *Config {
	return &Config{
		ListenAddress: "127.0.0.1:0", // Random port
		TLS: TLSConfig{
			Enabled: false,
		},
		MaxMessageSize:       4 * 1024 * 1024,
		MaxConcurrentStreams: 100,
		KeepAlive: KeepAliveConfig{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		},
		YANG: YANGConfig{
			EnableRFCParser: true,
			CacheModules:    true,
			MaxModules:      1000,
		},
	}
}

// createTestReceiver creates a receiver with valid test configuration
func createTestReceiver() (*ciscoTelemetryReceiver, error) {
	config := createValidTestConfig()

	// Create proper receiver settings
	settings := receiver.Settings{
		ID:                component.NewID(component.MustNewType(TypeStr)),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         component.NewDefaultBuildInfo(),
	}

	// Create a test metrics consumer
	consumer := consumertest.NewNop()

	receiver, err := newCiscoTelemetryReceiver(config, settings, consumer)
	return receiver, err
}

// createTestSettings creates proper receiver settings for testing
func createTestSettings() receiver.Settings {
	return receiver.Settings{
		ID:                component.NewID(component.MustNewType(TypeStr)),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         component.NewDefaultBuildInfo(),
	}
}
