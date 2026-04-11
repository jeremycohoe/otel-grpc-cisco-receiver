#!/bin/bash
# Start Splunk Enterprise in Docker with HEC pre-configured
# Web UI: http://<host>:8000 (admin / Cisco123)
# HEC:    https://<host>:8088

set -e

CONTAINER_NAME="splunk"
SPLUNK_PASSWORD="Cisco123"
HEC_TOKEN="cisco-mdt-token"

# Remove existing container if present
docker rm -f "$CONTAINER_NAME" 2>/dev/null || true

docker run -d --name "$CONTAINER_NAME" \
  -p 8000:8000 \
  -p 8088:8088 \
  -p 8089:8089 \
  -e SPLUNK_GENERAL_TERMS="--accept-sgt-current-at-splunk-com" \
  -e SPLUNK_START_ARGS="--accept-license" \
  -e SPLUNK_PASSWORD="$SPLUNK_PASSWORD" \
  -e SPLUNK_HEC_TOKEN="$HEC_TOKEN" \
  splunk/splunk:latest

echo "Splunk starting... waiting for health check"
echo "Web UI will be at http://$(hostname -I | awk '{print $1}'):8000"
echo "Credentials: admin / $SPLUNK_PASSWORD"

# Wait for Splunk to become healthy
for i in $(seq 1 60); do
  if curl -sf -k https://localhost:8089/services/server/health >/dev/null 2>&1; then
    echo "Splunk is healthy!"

    # Create cisco_mdt index
    echo "Creating cisco_mdt index..."
    curl -sf -k -u admin:"$SPLUNK_PASSWORD" \
      https://localhost:8089/services/data/indexes \
      -d name=cisco_mdt \
      -d datatype=event >/dev/null 2>&1 && echo "Index cisco_mdt created" || echo "Index may already exist"

    echo "Splunk ready."
    exit 0
  fi
  printf "."
  sleep 5
done

echo ""
echo "WARNING: Splunk did not become healthy within 5 minutes"
exit 1
