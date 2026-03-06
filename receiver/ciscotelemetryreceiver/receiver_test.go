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
	cfg := createValidTestConfig()
	consumer := consumertest.NewNop()
	settings := createTestSettings()

	rcv, err := createMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err)
	require.NotNil(t, rcv)

	ctx := context.Background()
	host := &mockHost{}

	err = rcv.Start(ctx, host)
	require.NoError(t, err, "receiver should start")

	time.Sleep(50 * time.Millisecond)

	err = rcv.Shutdown(ctx)
	require.NoError(t, err, "receiver should shutdown")

	// Idempotent shutdown.
	err = rcv.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestCiscoTelemetryReceiver_ShutdownBeforeStart(t *testing.T) {
	cfg := createValidTestConfig()
	consumer := &consumertest.MetricsSink{}
	settings := createTestSettings()

	rcv, err := newCiscoTelemetryReceiver(cfg, settings, consumer)
	require.NoError(t, err)

	// Shutdown before start should not panic.
	err = rcv.Shutdown(context.Background())
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

func TestCiscoTelemetryReceiver_MultipleInstances(t *testing.T) {
	receivers := make([]receiver.Metrics, 3)

	for i := 0; i < 3; i++ {
		cfg := createValidTestConfig()
		consumer := consumertest.NewNop()
		settings := createTestSettings()

		rcv, err := createMetricsReceiver(context.Background(), settings, cfg, consumer)
		require.NoError(t, err)
		receivers[i] = rcv
	}

	ctx := context.Background()
	host := &mockHost{}

	for i, rcv := range receivers {
		if err := rcv.Start(ctx, host); err != nil {
			t.Logf("receiver %d start failed (ok): %v", i, err)
		}
	}
	for _, rcv := range receivers {
		_ = rcv.Shutdown(ctx)
	}
}

// mockHost implements component.Host for testing.
type mockHost struct{}

func (h *mockHost) ReportFatalError(err error)                                          {}
func (h *mockHost) GetFactory(component.Kind, component.Type) component.Factory         { return nil }
func (h *mockHost) GetExtensions() map[component.ID]component.Component                 { return nil }
func (h *mockHost) GetExporters() map[component.ID]component.Component                  { return nil }
