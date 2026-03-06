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

	assert.Equal(t, TypeStr, factory.Type().String())

	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)
	assert.IsType(t, &Config{}, cfg)

	rcv, err := factory.CreateMetrics(
		context.Background(),
		createTestSettings(),
		cfg,
		&consumertest.MetricsSink{},
	)
	assert.NoError(t, err)
	assert.NotNil(t, rcv)
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	require.NotNil(t, cfg)

	config, ok := cfg.(*Config)
	require.True(t, ok)

	assert.Equal(t, "0.0.0.0:57500", config.ListenAddress)
	assert.Nil(t, config.TLS)
	assert.Equal(t, 4, config.MaxRecvMsgSizeMiB)
	assert.Equal(t, uint32(128), config.MaxConcurrentStreams)
	assert.Equal(t, 30*time.Second, config.KeepAlive.Time)
	assert.Equal(t, 10*time.Second, config.KeepAlive.Timeout)
	assert.True(t, config.YANG.EnableRFCParser)
	assert.True(t, config.YANG.CacheModules)
	assert.Equal(t, 1000, config.YANG.MaxModules)

	assert.NoError(t, config.Validate())
}

func TestCreateMetricsReceiver_ValidConfig(t *testing.T) {
	cfg := createValidTestConfig()
	consumer := consumertest.NewNop()
	settings := createTestSettings()

	rcv, err := createMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err)
	require.NotNil(t, rcv)

	ciscoReceiver, ok := rcv.(*ciscoTelemetryReceiver)
	require.True(t, ok)
	assert.NotNil(t, ciscoReceiver.config)
	assert.NotNil(t, ciscoReceiver.consumer)
}

func TestCreateMetricsReceiver_InvalidConfig(t *testing.T) {
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
			name: "empty_listen_address",
			config: &Config{
				ListenAddress:        "",
				MaxConcurrentStreams: 128,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rcv, err := createMetricsReceiver(context.Background(), settings, tt.config, consumer)
			assert.Error(t, err)
			assert.Nil(t, rcv)
		})
	}
}
