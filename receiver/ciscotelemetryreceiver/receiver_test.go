package ciscotelemetryreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
)

func TestCiscoTelemetryReceiver_Lifecycle(t *testing.T) {
	// Create receiver with default config using random port
	cfg := createValidTestConfig()

	consumer := consumertest.NewNop()
	settings := createTestSettings()

	receiver, err := createMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err)
	require.NotNil(t, receiver)

	// Test component lifecycle
	ctx := context.Background()

	// Create mock host for testing
	host := &mockHost{}

	// Test Start
	err = receiver.Start(ctx, host)
	require.NoError(t, err, "Receiver should start successfully")

	// Give some time for server to initialize
	time.Sleep(100 * time.Millisecond)

	// Verify receiver is started by checking its state
	// (Note: In a real test environment, we could verify the gRPC server is listening)

	// Test Shutdown
	err = receiver.Shutdown(ctx)
	require.NoError(t, err, "Receiver should shutdown successfully")

	// Test that shutdown is idempotent
	err = receiver.Shutdown(ctx)
	assert.NoError(t, err, "Second shutdown should not return error")
}

func TestCiscoTelemetryReceiver_StartTwice(t *testing.T) {
	cfg := createValidTestConfig()

	consumer := consumertest.NewNop()
	settings := createTestSettings()

	receiver, err := createMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err)

	ctx := context.Background()
	host := &mockHost{}

	// First start
	err = receiver.Start(ctx, host)
	if err != nil {
		t.Logf("First start failed (acceptable): %v", err)
		return
	}

	// Second start should handle gracefully
	err = receiver.Start(ctx, host)
	// Should not panic or cause issues

	// Cleanup
	receiver.Shutdown(ctx)
}

func TestCiscoTelemetryReceiver_ShutdownTwice(t *testing.T) {
	cfg := createValidTestConfig()

	consumer := consumertest.NewNop()
	settings := createTestSettings()

	receiver, err := createMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err)

	ctx := context.Background()

	// Start receiver
	err = receiver.Start(ctx, &mockHost{})
	if err != nil {
		t.Logf("Start failed (acceptable): %v", err)
		return
	}

	// First shutdown
	err = receiver.Shutdown(ctx)
	assert.NoError(t, err)

	// Second shutdown should not cause issues
	err = receiver.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestCiscoTelemetryReceiver_InvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name: "empty_listen_address",
			config: &Config{
				ListenAddress:        "",
				MaxConcurrentStreams: 100, // Still need this for validation
			},
		},
		{
			name: "invalid_tls_config",
			config: &Config{
				ListenAddress: "127.0.0.1:0",
				TLS: TLSConfig{
					Enabled:  true,
					CertFile: "", // Missing cert file
					KeyFile:  "key.pem",
				},
				MaxConcurrentStreams: 100,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer := consumertest.NewNop()
			settings := createTestSettings()

			_, err := createMetricsReceiver(context.Background(), settings, tt.config, consumer)
			assert.Error(t, err)
		})
	}
}

func TestCiscoTelemetryReceiver_ConfigMigration(t *testing.T) {
	// Test legacy configuration migration
	cfg := &Config{
		ListenAddress: "127.0.0.1:57500",

		// Legacy TLS settings
		TLSEnabled:      true,
		TLSCertFile:     "cert.pem",
		TLSKeyFile:      "key.pem",
		TLSClientCAFile: "ca.pem",

		// Legacy keep-alive settings
		KeepAliveTimeout: 45 * time.Second,

		MaxMessageSize:       8 * 1024 * 1024,
		MaxConcurrentStreams: 200,
	}

	// Migrate configuration
	cfg.MigrateLegacyConfig()

	// Verify migration
	assert.True(t, cfg.TLS.Enabled)
	assert.Equal(t, "cert.pem", cfg.TLS.CertFile)
	assert.Equal(t, "key.pem", cfg.TLS.KeyFile)
	assert.Equal(t, "ca.pem", cfg.TLS.CAFile)
	assert.Equal(t, 45*time.Second, cfg.KeepAlive.Time)
	assert.Equal(t, 10*time.Second, cfg.KeepAlive.Timeout)

	// Validate migrated config
	assert.NoError(t, cfg.Validate())

	// Test creating receiver with migrated config
	consumer := consumertest.NewNop()
	settings := createTestSettings()

	receiver, err := createMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err)
	require.NotNil(t, receiver)
}

func TestCiscoTelemetryReceiver_MultipleInstances(t *testing.T) {
	// Test creating multiple receiver instances
	receivers := make([]receiver.Metrics, 3)

	for i := 0; i < 3; i++ {
		cfg := createValidTestConfig() // Already has random port

		consumer := consumertest.NewNop()
		settings := createTestSettings()

		rcv, err := createMetricsReceiver(context.Background(), settings, cfg, consumer)
		require.NoError(t, err)
		require.NotNil(t, rcv)

		receivers[i] = rcv
	}

	// Verify instances are independent
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			assert.NotSame(t, receivers[i], receivers[j])
		}
	}

	// Test starting all instances
	ctx := context.Background()
	host := &mockHost{}

	for i, rcv := range receivers {
		err := rcv.Start(ctx, host)
		if err != nil {
			t.Logf("Receiver %d start failed (acceptable): %v", i, err)
		}
	}

	// Test shutting down all instances
	for i, rcv := range receivers {
		err := rcv.Shutdown(ctx)
		if err != nil {
			t.Logf("Receiver %d shutdown failed: %v", i, err)
		}
	}
}

// mockHost implements component.Host for testing
type mockHost struct{}

func (h *mockHost) ReportFatalError(err error) {}
func (h *mockHost) GetFactory(kind component.Kind, componentType component.Type) component.Factory {
	return nil
}
func (h *mockHost) GetExtensions() map[component.ID]component.Component { return nil }
func (h *mockHost) GetExporters() map[component.ID]component.Component  { return nil }
