#!/bin/bash
# Start the OTEL Collector with Cisco MDT receiver
# Listens on 0.0.0.0:57500 for gRPC dial-out from C9300X
# Exports to Splunk HEC at https://127.0.0.1:8088

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
COLLECTOR="$SCRIPT_DIR/build/cisco-otelcol"
CONFIG="$SCRIPT_DIR/collector-config.yaml"

# Kill any stale collector process to free ports (8888, 57500)
if pkill -f cisco-otelcol 2>/dev/null; then
  echo "Stopped previous collector instance"
  sleep 1
fi

if [[ ! -x "$COLLECTOR" ]]; then
  echo "ERROR: Collector binary not found at $COLLECTOR"
  echo "Run: builder --config=builder-config.yaml"
  exit 1
fi

if ! curl -sf -k https://localhost:8088/services/collector/health >/dev/null 2>&1; then
  echo "WARNING: Splunk HEC not reachable at https://localhost:8088"
  echo "Make sure Splunk is running: ./start-splunk.sh"
fi

echo "Starting OTEL Collector..."
echo "  gRPC listen:  0.0.0.0:57500"
echo "  Splunk HEC:   https://127.0.0.1:8088"
echo "  Self-metrics: http://0.0.0.0:8888"
echo "  Health check: http://0.0.0.0:13133"
echo ""

exec "$COLLECTOR" --config "$CONFIG"
