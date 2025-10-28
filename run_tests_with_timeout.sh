#!/bin/bash

# Test runner with timeout to identify slow tests
# Usage: ./run_tests_with_timeout.sh [timeout_seconds]

TIMEOUT=${1:-30}  # Default 30 seconds timeout
TEST_DIR="./receiver/ciscotelemetryreceiver"

echo "Running tests with ${TIMEOUT}s timeout..."
echo "================================="

# Function to run a single test with timeout
run_test_with_timeout() {
    local test_name="$1"
    echo -n "Running $test_name... "
    
    # Use gtimeout on macOS, timeout on Linux
    if command -v gtimeout >/dev/null 2>&1; then
        TIMEOUT_CMD="gtimeout"
    elif command -v timeout >/dev/null 2>&1; then
        TIMEOUT_CMD="timeout"
    else
        echo "SKIP (no timeout command available)"
        return
    fi
    
    if $TIMEOUT_CMD ${TIMEOUT}s go test $TEST_DIR -run "^${test_name}$" -v >/dev/null 2>&1; then
        echo "PASS"
    else
        exit_code=$?
        if [ $exit_code -eq 124 ] || [ $exit_code -eq 143 ]; then
            echo "TIMEOUT (>${TIMEOUT}s) - NEEDS FIXING"
            echo "$test_name" >> slow_tests.log
        else
            echo "FAIL"
            echo "$test_name" >> failing_tests.log
        fi
    fi
}

# Get list of all test functions
TEST_FUNCTIONS=$(go test $TEST_DIR -list ".*" 2>/dev/null | grep "^Test" | head -20)

# Clear log files
> slow_tests.log
> failing_tests.log

# Run each test individually with timeout
for test in $TEST_FUNCTIONS; do
    run_test_with_timeout "$test"
done

echo "================================="
echo "Summary:"

if [ -s slow_tests.log ]; then
    echo "SLOW TESTS (need timeout fixes):"
    cat slow_tests.log
    echo ""
fi

if [ -s failing_tests.log ]; then
    echo "FAILING TESTS:"
    cat failing_tests.log
    echo ""
fi

# Run fast tests together for coverage
echo "Running all fast tests for coverage..."
FAST_TESTS=$(go test $TEST_DIR -list ".*" 2>/dev/null | grep "^Test" | grep -E "(Config|Factory|GrpcServiceHelpers|ReceiverLifecycle_StartShutdown)" | tr '\n' '|' | sed 's/|$//')

if [ -n "$FAST_TESTS" ]; then
    echo "Fast test pattern: $FAST_TESTS"
    go test $TEST_DIR -run "$FAST_TESTS" -coverprofile=coverage.out
    if [ -f coverage.out ]; then
        echo "Coverage report:"
        go tool cover -func=coverage.out | tail -1
    fi
fi