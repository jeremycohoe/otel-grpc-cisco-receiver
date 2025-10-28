package ciscotelemetryreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)

	// Verify factory type matches expected type string
	assert.Equal(t, TypeStr, factory.Type().String())

	// Verify factory can create default config
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)
	assert.IsType(t, &Config{}, cfg)

	// Verify factory has metrics receiver creation capability
	receiver, err := factory.CreateMetrics(
		context.Background(),
		createTestSettings(),
		cfg,
		&consumertest.MetricsSink{},
	)
	assert.NoError(t, err)
	assert.NotNil(t, receiver)
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	require.NotNil(t, cfg)

	config, ok := cfg.(*Config)
	require.True(t, ok)

	// Verify default values
	assert.Equal(t, "0.0.0.0:57500", config.ListenAddress)
	assert.False(t, config.TLS.Enabled)
	assert.Equal(t, 4*1024*1024, config.MaxMessageSize)
	assert.Equal(t, uint32(100), config.MaxConcurrentStreams)
	assert.Equal(t, 30*time.Second, config.KeepAlive.Time)
	assert.Equal(t, 10*time.Second, config.KeepAlive.Timeout)
	assert.True(t, config.YANG.EnableRFCParser)
	assert.True(t, config.YANG.CacheModules)
	assert.Equal(t, 1000, config.YANG.MaxModules)

	// Validate the default config
	assert.NoError(t, config.Validate())
}

func TestCreateMetricsReceiver_ValidConfig(t *testing.T) {
	ctx := context.Background()
	cfg := createValidTestConfig()
	consumer := consumertest.NewNop()

	// Create receiver settings (simplified)
	settings := createTestSettings()

	receiver, err := createMetricsReceiver(ctx, settings, cfg, consumer)

	require.NoError(t, err)
	require.NotNil(t, receiver)

	// Test that receiver can be cast to the expected type
	ciscoReceiver, ok := receiver.(*ciscoTelemetryReceiver)
	require.True(t, ok)
	assert.NotNil(t, ciscoReceiver.config)
	assert.NotNil(t, ciscoReceiver.consumer)
}

func TestCreateMetricsReceiver_InvalidConfig(t *testing.T) {
	ctx := context.Background()
	consumer := consumertest.NewNop()
	settings := createTestSettings()

	tests := []struct {
		name   string
		config component.Config
	}{
		{
			name:   "wrong_config_type",
			config: &struct{}{},
		},
		{
			name: "invalid_config_values",
			config: &Config{
				ListenAddress:        "",  // Invalid: empty
				MaxConcurrentStreams: 100, // Still need this for other validation to pass
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver, err := createMetricsReceiver(ctx, settings, tt.config, consumer)

			assert.Error(t, err)
			assert.Nil(t, receiver)
		})
	}
}

func TestFactory_ConfigValidation(t *testing.T) {
	ctx := context.Background()
	consumer := consumertest.NewNop()
	settings := createTestSettings()

	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid_default",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "custom_listen_address",
			modify: func(c *Config) {
				c.ListenAddress = "127.0.0.1:8080"
			},
			wantErr: false,
		},
		{
			name: "enable_tls",
			modify: func(c *Config) {
				c.TLS.Enabled = true
				c.TLS.CertFile = "cert.pem"
				c.TLS.KeyFile = "key.pem"
			},
			wantErr: false,
		},
		{
			name: "invalid_listen_address",
			modify: func(c *Config) {
				c.ListenAddress = ""
			},
			wantErr: true,
		},
		{
			name: "tls_without_cert",
			modify: func(c *Config) {
				c.TLS.Enabled = true
				c.TLS.KeyFile = "key.pem"
				// Missing CertFile
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createValidTestConfig()
			tt.modify(cfg)

			receiver, err := createMetricsReceiver(ctx, settings, cfg, consumer)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, receiver)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, receiver)
			}
		})
	}
}
