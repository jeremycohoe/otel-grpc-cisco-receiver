package ciscotelemetryreceiver

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
)

// createValidTestConfig returns a Config suitable for unit tests. The listen
// address uses port 0 so the OS picks a free port automatically, preventing
// port conflicts when tests run in parallel.
func createValidTestConfig() *Config {
	return &Config{
		ListenAddress:        "127.0.0.1:0",
		TLS:                  nil, // plaintext for tests
		MaxRecvMsgSizeMiB:    4,
		MaxConcurrentStreams: 100,
		KeepAlive: KeepAliveConfig{
			Time:                      30 * time.Second,
			Timeout:                   10 * time.Second,
			EnforcementMinTime:        10 * time.Second,
			EnforcementPermitNoStream: true,
		},
		YANG: YANGConfig{
			EnableRFCParser: true,
			CacheModules:    true,
			MaxModules:      1000,
		},
	}
}

// createTestSettings returns a receiver.Settings wired to no-op telemetry,
// which is safe for all unit test scenarios.
func createTestSettings() receiver.Settings {
	typeStr := component.MustNewType(TypeStr)
	return receiver.Settings{
		ID: component.NewID(typeStr),
		TelemetrySettings: component.TelemetrySettings{
			Logger:        zap.NewNop(),
			MeterProvider: noop.NewMeterProvider(),
		},
		BuildInfo: component.NewDefaultBuildInfo(),
	}
}

// withQuickTimeout runs fn with a per-test deadline so that stuck tests
// do not block the entire suite.
func withQuickTimeout(t *testing.T, fn func(t *testing.T)) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(t)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("test timed out after 10 s")
	}
}
