package ciscotelemetryreceiver

import (
	"testing"
	"time"
)

// testWithTimeout runs a test function with a timeout
func testWithTimeout(t *testing.T, timeout time.Duration, testFunc func(t *testing.T)) {
	t.Helper()

	done := make(chan bool, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Test panicked: %v", r)
			}
			done <- true
		}()
		testFunc(t)
	}()

	select {
	case <-done:
		// Test completed normally
	case <-time.After(timeout):
		t.Fatalf("Test timed out after %v - likely has hanging goroutines or blocking operations", timeout)
	}
}

// withTestTimeout is a test helper that applies a 30-second timeout
func withTestTimeout(t *testing.T, testFunc func(t *testing.T)) {
	testWithTimeout(t, 30*time.Second, testFunc)
}

// withQuickTimeout is for tests that should complete very quickly (5 seconds)
func withQuickTimeout(t *testing.T, testFunc func(t *testing.T)) {
	testWithTimeout(t, 5*time.Second, testFunc)
}
