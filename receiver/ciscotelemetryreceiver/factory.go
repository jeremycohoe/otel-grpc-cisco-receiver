package ciscotelemetryreceiver

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

// TypeStr defines the receiver type.
const TypeStr = "ciscotelemetry"

// NewFactory creates a factory for Cisco Telemetry receiver.
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
		// Network Configuration
		ListenAddress: "0.0.0.0:57500",

		// Security Configuration
		TLS: TLSConfig{
			Enabled:            false,
			CertFile:           "",
			KeyFile:            "",
			CAFile:             "",
			InsecureSkipVerify: false,
			ClientAuthType:     "NoClientCert",
			MinVersion:         "1.2",      // Default to TLS 1.2 minimum for security
			MaxVersion:         "1.3",      // Default to TLS 1.3 maximum
			CipherSuites:       []string{}, // Use Go defaults
			CurvePreferences:   []string{}, // Use Go defaults
			ReloadInterval:     5 * time.Minute,
		},

		// Security Hardening Configuration
		Security: SecurityConfig{
			RateLimiting: RateLimitingConfig{
				Enabled:           false,
				RequestsPerSecond: 100.0,
				BurstSize:         10,
				CleanupInterval:   time.Minute,
			},
			AllowedClients:    []string{}, // Empty means allow all
			MaxConnections:    1000,
			ConnectionTimeout: 30 * time.Second,
			EnableMetrics:     true,
		},

		// Performance Configuration
		MaxMessageSize:       4 * 1024 * 1024, // 4MB
		MaxConcurrentStreams: 100,
		KeepAlive: KeepAliveConfig{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		},

		// YANG Configuration
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
