# Telemetry Capture & Validation Guide

Capture per-subscription telemetry data from Cisco IOS XE switches into individual files for **validation**, **dashboard design**, and **debugging**. Each subscription gets its own folder with raw JSON, pretty-printed JSON, flat data points, and a human-readable text summary.

## Why Capture?

- **Validate subscriptions** — confirm every XPath actually produces data
- **Understand the data** — see exact metric names, types, entity keys, and sample values
- **Design dashboards** — know which fields to query and group-by before writing SPL/PromQL
- **Debug issues** — compare expected vs. actual telemetry output

## Prerequisites

| Requirement | Details |
|-------------|---------|
| Custom collector built | `builder --config=builder-config.yaml` (includes `fileexporter`) |
| Python 3.6+ | For `parse-capture.py` (stdlib only, no pip packages) |
| Switch sending telemetry | At least one Cisco IOS XE switch with MDT subscriptions pointing at the collector |

The `fileexporter` module is already included in `builder-config.yaml`. If you're using a different builder config, add this line to the `exporters` section:

```yaml
exporters:
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.138.0
```

Then rebuild: `builder --config=builder-config.yaml`

## Quick Start

```bash
# 1. Stop any running production collector (if applicable)
sudo systemctl stop cisco-otelcol.service

# 2. Run the collector in capture mode
./build/cisco-otelcol --config capture-config.yaml

# 3. Wait for telemetry data to arrive (30-60 seconds for HOT subscriptions)
#    Watch the debug output — you'll see metrics arriving per subscription

# 4. Stop the collector (Ctrl+C)

# 5. Parse the captured data into readable formats
python3 parse-capture.py

# 6. Optionally rename folders and collect summaries
python3 parse-capture.py --rename --collect
```

## Capture Configuration

The file [`capture-config.yaml`](../capture-config.yaml) configures the collector to write per-subscription files using the OTel `file` exporter with `group_by`:

```yaml
exporters:
  file/capture:
    path: ./telemetry-capture/*/metrics.jsonl
    format: json
    group_by:
      enabled: true
      resource_attribute: cisco.subscription_id
      max_open_files: 100
```

**How it works:**
- The `*` in the path is replaced by the value of the `cisco.subscription_id` resource attribute
- Each subscription gets its own directory: `telemetry-capture/1001/`, `telemetry-capture/1007/`, etc.
- Raw telemetry is written as JSON Lines (one JSON object per batch)
- The `group_by.max_open_files` setting limits concurrent open file handles

**Key settings you may want to adjust:**

| Setting | Default | Purpose |
|---------|---------|---------|
| `listen_address` | `0.0.0.0:57500` | gRPC listen port — must not conflict with production |
| `batch.timeout` | `2s` | How long to buffer before writing |
| `telemetry.metrics.port` | `8889` | Self-metrics port — use a different port than production (8888) |
| `telemetry.logs.level` | `debug` | Set to `info` to reduce console noise |

### Using a different port

If your production collector is still running on port 57500, either stop it or change the capture config:

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57501"  # different port
```

Then update the switch receiver to point to the new port.

## Parse Script

[`parse-capture.py`](../parse-capture.py) processes the raw JSONL files into four human-readable formats per subscription:

| File | Format | Purpose |
|------|--------|---------|
| `metrics.jsonl` | Raw JSON Lines | Original collector output (input) |
| `metrics-pretty.json` | Indented JSON | Easy to browse in an editor with JSON folding |
| `flat.jsonl` | One line per data point | Quick grep/jq queries for specific metrics |
| `data.txt` | Plain text table | Terminal-friendly overview grouped by metric |
| `README.md` | Markdown | Full documentation with entity keys, sample values, YANG metadata |

### Usage

```bash
# Generate all output files (README.md, metrics-pretty.json, flat.jsonl, data.txt)
python3 parse-capture.py

# Also rename folders from "1001" to "1001-cpu-usage_cpu-utilization"
python3 parse-capture.py --rename

# Rename folders AND collect all data.txt into _all-data-txt/ for consolidated review
python3 parse-capture.py --rename --collect

# Use a custom capture directory
python3 parse-capture.py --dir /path/to/my-capture
```

### Output Structure

After running with `--rename --collect`:

```
telemetry-capture/
├── _all-data-txt/                              # Consolidated data.txt copies
│   ├── 1001-cpu-usage_cpu-utilization.txt
│   ├── 1005-environment-sensors.txt
│   ├── 1007-interfaces_interface.txt
│   └── ...
├── 1001-cpu-usage_cpu-utilization/
│   ├── metrics.jsonl          # Raw capture (input)
│   ├── metrics-pretty.json    # Indented JSON
│   ├── flat.jsonl             # One line per data point
│   ├── data.txt               # Plain text summary
│   └── README.md              # Markdown documentation
├── 1005-environment-sensors/
│   └── ...
└── 1007-interfaces_interface/
    └── ...
```

### Reading data.txt

The `data.txt` format groups data points by metric name, showing values and entity attributes:

```
==========================================================================================
  Sub 1005: Environment Sensors
  Folder: 1005-environment-sensors
  202 data points from 2 telemetry messages
==========================================================================================

  environment-sensor.current-reading  (gauge, 25)
  ----------------------------------------------------------------------
              50  |  location=Switch 1, name=Inlet Temp Sensor
              51  |  location=Switch 1, name=Outlet Temp Sensor
              39  |  location=Switch 1, name=Hotspot Temp Sensor
              ...

  environment-sensor.state_info  (gauge, 25)
  ----------------------------------------------------------------------
               1  |  value=Green, name=Inlet Temp Sensor
               1  |  value=Green, name=Hotspot Temp Sensor
```

- **Numeric metrics** show the value on the left, entity attributes on the right
- **Info metrics** (suffix `_info`) have value=1 and a string `value=` attribute — these represent enums/strings from YANG
- **`(global)`** means the metric has no entity keys (single-instance data)

### Querying flat.jsonl

```bash
# Find all interface counter metrics
grep '"metric":"in-octets"' telemetry-capture/1007-*/flat.jsonl

# Get all CPU utilization readings
jq -r 'select(.metric == "five-seconds") | "\(.value)%"' telemetry-capture/1001-*/flat.jsonl

# List all unique metric names in a subscription
jq -r '.metric' telemetry-capture/1005-*/flat.jsonl | sort -u
```

## How Long to Capture

The amount of capture time needed depends on your subscription polling intervals:

| Tier | Period | Capture Time for 2 Samples |
|------|--------|---------------------------|
| HOT | 30s (3000 centiseconds) | ~60 seconds |
| WARM | 60s (6000 centiseconds) | ~2 minutes |
| COOL | 5min (30000 centiseconds) | ~10 minutes |

For a quick validation of all subscriptions, **2 minutes** is usually sufficient — HOT subscriptions will have 3-4 samples and WARM will have 1-2.

## Subscriptions With No Data

Some subscriptions may produce no telemetry even though they show "Valid" on the switch. This is normal — it means the feature isn't active:

| Feature | Requires |
|---------|----------|
| BFD Sessions | BFD configured on interfaces |
| HSRP / VRRP | First-hop redundancy configured |
| EIGRP / IS-IS | Routing protocol configured |
| IP SLA | SLA probes configured |
| Port Security | 802.1X port security enabled |
| NetFlow | Flow monitors applied to interfaces |
| LAG Aggregate | Port-channels configured |

Check the switch: `show telemetry ietf subscription <id> detail` — if `State: Valid` but no data arrives, the feature simply has no operational data to report.

## Troubleshooting

### No files created in telemetry-capture/

1. Verify the switch is sending to the correct IP and port:
   ```cisco
   show telemetry ietf subscription 1001 receiver
   ```
2. Check the collector debug output for gRPC connections:
   ```
   Connection established from <switch-ip>
   ```
3. Ensure no firewall is blocking port 57500

### Folders created but metrics.jsonl is empty

The subscription XPath may be invalid. Check the switch:
```cisco
show telemetry ietf subscription <id> detail
```
Look for `State: Valid` vs `State: Invalid`.

### parse-capture.py finds 0 directories

The script looks for folders matching `^\d+$` (numeric only) or `^\d+-.+` (already renamed). If your folders have a different naming convention, use `--dir` to point to the right location.

## Example: Full Workflow

```bash
# Build the collector (one-time)
builder --config=builder-config.yaml

# Stop production collector
sudo systemctl stop cisco-otelcol.service

# Clean any previous capture
rm -rf telemetry-capture/

# Run capture for 2 minutes
timeout 120 ./build/cisco-otelcol --config capture-config.yaml

# Parse everything
python3 parse-capture.py --rename --collect

# Browse the results
ls telemetry-capture/
cat telemetry-capture/_all-data-txt/1001-cpu-usage_cpu-utilization.txt

# Restart production when done
sudo systemctl start cisco-otelcol.service
```
