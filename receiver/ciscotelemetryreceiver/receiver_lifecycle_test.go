package ciscotelemetryreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestReceiverLifecycle_StartShutdown(t *testing.T) {
	withQuickTimeout(t, func(t *testing.T) {
		config := createValidTestConfig()
		consumer := &consumertest.MetricsSink{}
		settings := createTestSettings()

		rcv, err := newCiscoTelemetryReceiver(config, settings, consumer)
		require.NoError(t, err)

		ctx := context.Background()
		err = rcv.Start(ctx, nil)
		assert.NoError(t, err)

		err = rcv.Shutdown(ctx)
		assert.NoError(t, err)
	})
}

func TestReceiverLifecycle_ShutdownBeforeStart(t *testing.T) {
	config := createValidTestConfig()
	consumer := &consumertest.MetricsSink{}
	settings := createTestSettings()

	rcv, err := newCiscoTelemetryReceiver(config, settings, consumer)
	require.NoError(t, err)

	err = rcv.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestReceiverLifecycle_StartWithInvalidAddress(t *testing.T) {
	config := createValidTestConfig()
	config.ListenAddress = "localhost:-1"
	consumer := &consumertest.MetricsSink{}
	settings := createTestSettings()

	rcv, err := newCiscoTelemetryReceiver(config, settings, consumer)
	require.NoError(t, err)

	err = rcv.Start(context.Background(), nil)
	assert.Error(t, err)
}

func TestReceiverLifecycle_GracefulShutdownTimeout(t *testing.T) {
	withQuickTimeout(t, func(t *testing.T) {
		config := createValidTestConfig()
		consumer := &consumertest.MetricsSink{}
		settings := createTestSettings()

		rcv, err := newCiscoTelemetryReceiver(config, settings, consumer)
		require.NoError(t, err)

		ctx := context.Background()
		err = rcv.Start(ctx, nil)
		require.NoError(t, err)

		// Shutdown with a very short deadline to exercise the force-stop path.
		shutdownCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
		defer cancel()

		err = rcv.Shutdown(shutdownCtx)
		assert.NoError(t, err)
	})
}
