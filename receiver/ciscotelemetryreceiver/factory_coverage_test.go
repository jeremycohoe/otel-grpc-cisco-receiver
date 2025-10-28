package ciscotelemetryreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

// TestNewFactory_Coverage tests the NewFactory function to improve coverage
func TestNewFactory_Coverage(t *testing.T) {
	withQuickTimeout(t, func(t *testing.T) {
		// Call NewFactory to get coverage
		factory := NewFactory()

		// Verify factory is created and has correct type
		assert.NotNil(t, factory)
		assert.Equal(t, TypeStr, factory.Type().String())

		// Verify factory capabilities
		assert.Equal(t, component.StabilityLevelDevelopment, factory.MetricsStability())
	})
}

// TestCreateDefaultConfig_Coverage tests createDefaultConfig function
func TestCreateDefaultConfig_Coverage(t *testing.T) {
	withQuickTimeout(t, func(t *testing.T) {
		// Call createDefaultConfig directly to get coverage
		config := createDefaultConfig()

		// Verify config is created with expected defaults
		require.NotNil(t, config)
		ciscoConfig, ok := config.(*Config)
		require.True(t, ok)

		// Verify key defaults are set
		assert.Equal(t, "0.0.0.0:57500", ciscoConfig.ListenAddress)
		assert.False(t, ciscoConfig.TLS.Enabled)
		assert.Equal(t, 4*1024*1024, ciscoConfig.MaxMessageSize)
		assert.Equal(t, uint32(100), ciscoConfig.MaxConcurrentStreams)
		assert.True(t, ciscoConfig.YANG.EnableRFCParser)
		assert.True(t, ciscoConfig.YANG.CacheModules)
		assert.Equal(t, 1000, ciscoConfig.YANG.MaxModules)

		// Verify config validation passes
		assert.NoError(t, ciscoConfig.Validate())
	})
}
