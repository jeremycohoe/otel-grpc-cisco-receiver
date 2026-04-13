#!/bin/bash
# Import all Cisco MDT Dashboard Studio dashboards into Splunk
# Usage: ./scripts/import-dashboards.sh [SPLUNK_URL] [USERNAME] [PASSWORD]

set -e

SPLUNK_URL="${1:-https://localhost:8089}"
USERNAME="${2:-admin}"
PASSWORD="${3:-Cisco123}"
DASHBOARD_DIR="$(cd "$(dirname "$0")/../splunk-dashboards" && pwd)"

# Dashboard files and their Splunk view names
DASHBOARDS=(
  "cisco_mdt_overview"
  "cisco_mdt_infrastructure"
  "cisco_mdt_network"
  "cisco_mdt_routing"
  "cisco_mdt_power"
  "cisco_mdt_security"
  "cisco_mdt_telemetry"
)

echo "Importing Cisco MDT dashboards into Splunk..."
echo "  Splunk API: $SPLUNK_URL"
echo "  Dashboard dir: $DASHBOARD_DIR"
echo ""

imported=0
failed=0

for name in "${DASHBOARDS[@]}"; do
  json_file="${DASHBOARD_DIR}/${name}.json"

  if [[ ! -f "$json_file" ]]; then
    echo "SKIP: $json_file not found"
    ((failed++))
    continue
  fi

  # Read JSON content and wrap in Dashboard Studio XML envelope
  json_content=$(cat "$json_file")
  xml_payload="<dashboard version=\"2\" theme=\"dark\"><label>$(echo "$json_content" | python3 -c "import sys,json; print(json.load(sys.stdin).get('title','Untitled'))")</label><description>$(echo "$json_content" | python3 -c "import sys,json; print(json.load(sys.stdin).get('description',''))")</description></dashboard>"

  # Create or update the dashboard view
  http_code=$(curl -sk -o /dev/null -w "%{http_code}" \
    -u "${USERNAME}:${PASSWORD}" \
    "${SPLUNK_URL}/servicesNS/admin/search/data/ui/views/${name}" \
    -X POST \
    -d "eai:data=${xml_payload}" \
    -d "eai:type=views" 2>/dev/null)

  if [[ "$http_code" == "200" || "$http_code" == "201" || "$http_code" == "409" ]]; then
    # Upload the JSON payload as Dashboard Studio content
    http_code=$(curl -sk -o /dev/null -w "%{http_code}" \
      -u "${USERNAME}:${PASSWORD}" \
      "${SPLUNK_URL}/servicesNS/admin/search/data/ui/views/${name}" \
      -X POST \
      --data-urlencode "eai:data=${xml_payload}" 2>/dev/null)
  fi

  # Now set the Dashboard Studio JSON payload via the dashboards endpoint
  http_code_ds=$(curl -sk -o /dev/null -w "%{http_code}" \
    -u "${USERNAME}:${PASSWORD}" \
    "${SPLUNK_URL}/servicesNS/admin/search/data/ui/views/${name}" \
    -X POST \
    -d "eai:type=views" \
    --data-urlencode "eai:data@${json_file}" 2>/dev/null)

  if [[ "$http_code_ds" == "200" || "$http_code_ds" == "201" ]]; then
    echo "  OK: ${name}"
    ((imported++))
  else
    echo "  FAIL (HTTP ${http_code_ds}): ${name}"
    ((failed++))
  fi
done

echo ""
echo "Done: ${imported} imported, ${failed} failed"
echo ""
if [[ $imported -gt 0 ]]; then
  host=$(echo "$SPLUNK_URL" | sed 's|https*://||;s|:.*||')
  echo "Dashboards available at:"
  for name in "${DASHBOARDS[@]}"; do
    echo "  http://${host}:8000/app/search/${name}"
  done
fi
