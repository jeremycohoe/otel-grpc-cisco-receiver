#!/bin/bash
# Import all Cisco MDT Dashboard Studio dashboards into Splunk
# Usage: ./scripts/import-dashboards.sh [SPLUNK_URL] [USERNAME] [PASSWORD]

SPLUNK_URL="${1:-https://localhost:8089}"
USERNAME="${2:-admin}"
PASSWORD="${3:-Cisco123}"
DASHBOARD_DIR="$(cd "$(dirname "$0")/../splunk-dashboards" && pwd)"

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

# Use Python to avoid bash escaping issues with CDATA/XML
python3 - "$SPLUNK_URL" "$USERNAME" "$PASSWORD" "$DASHBOARD_DIR" "${DASHBOARDS[@]}" <<'PYEOF'
import sys, json, urllib.request, urllib.parse, ssl, base64, os

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

splunk_url = sys.argv[1]
username = sys.argv[2]
password = sys.argv[3]
dashboard_dir = sys.argv[4]
dashboards = sys.argv[5:]

auth = "Basic " + base64.b64encode(f"{username}:{password}".encode()).decode()
imported = 0
failed = 0

for name in dashboards:
    json_file = os.path.join(dashboard_dir, f"{name}.json")
    if not os.path.isfile(json_file):
        print(f"  SKIP: {json_file} not found")
        failed += 1
        continue

    with open(json_file) as f:
        data = json.load(f)

    title = data.get("title", "Untitled").replace("&", "&amp;")
    desc = data.get("description", "").replace("&", "&amp;")
    json_str = json.dumps(data)

    xml = (
        f'<dashboard version="2" theme="dark">\n'
        f'  <label>{title}</label>\n'
        f'  <description>{desc}</description>\n'
        f'  <definition><![CDATA[{json_str}]]></definition>\n'
        f'</dashboard>'
    )

    # Try update first (POST to specific view)
    params = urllib.parse.urlencode({"eai:data": xml}).encode()
    url = f"{splunk_url}/servicesNS/admin/search/data/ui/views/{name}"
    req = urllib.request.Request(url, data=params, method="POST")
    req.add_header("Authorization", auth)

    try:
        resp = urllib.request.urlopen(req, context=ctx)
        print(f"  OK: {name} (updated)")
        imported += 1
        continue
    except urllib.error.HTTPError as e:
        if e.code != 404:
            print(f"  FAIL (HTTP {e.code}): {name}")
            failed += 1
            continue

    # 404 = doesn't exist yet, create it
    params = urllib.parse.urlencode({"name": name, "eai:data": xml}).encode()
    url = f"{splunk_url}/servicesNS/admin/search/data/ui/views"
    req = urllib.request.Request(url, data=params, method="POST")
    req.add_header("Authorization", auth)

    try:
        resp = urllib.request.urlopen(req, context=ctx)
        print(f"  OK: {name} (created)")
        imported += 1
    except urllib.error.HTTPError as e:
        print(f"  FAIL (HTTP {e.code}): {name}")
        failed += 1

print(f"\nDone: {imported} imported, {failed} failed\n")
if imported > 0:
    host = splunk_url.replace("https://", "").replace("http://", "").split(":")[0]
    print("Dashboards available at:")
    for name in dashboards:
        print(f"  http://{host}:8000/app/search/{name}")
PYEOF
