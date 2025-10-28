package ciscotelemetryreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSecurityManager_DirectMethodCalls tests security methods directly
func TestSecurityManager_DirectMethodCalls(t *testing.T) {
	withQuickTimeout(t, func(t *testing.T) {
		sm := &SecurityManager{}

		// Direct test of parseCipherSuites to ensure coverage
		suites, err := sm.parseCipherSuites([]string{"TLS_AES_128_GCM_SHA256"})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(suites))

		// Test error case
		_, err = sm.parseCipherSuites([]string{"INVALID"})
		assert.Error(t, err)

		// Direct test of parseCurvePreferences
		curves, err := sm.parseCurvePreferences([]string{"CurveP256"})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(curves))

		// Test error case
		_, err = sm.parseCurvePreferences([]string{"INVALID"})
		assert.Error(t, err)
	})
}
