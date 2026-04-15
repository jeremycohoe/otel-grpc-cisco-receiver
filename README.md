# Cisco Telemetry Receiver for OpenTelemetry Collector

> **A native OpenTelemetry Collector receiver for Cisco IOS XE Model-Driven Telemetry — get production switch metrics into Splunk, Prometheus, Datadog, or any OTel backend.**

[![OpenTelemetry](https://img.shields.io/badge/OpenTelemetry-Contribution%20Ready-brightgreen)](https://opentelemetry.io/)
[![Coverage](https://img.shields.io/badge/coverage-83.6%25-brightgreen)](#testing)
[![Go Report Card](https://goreportcard.com/badge/github.com/jeremycohoe/otel-grpc-cisco-receiver)](https://goreportcard.com/report/github.com/jeremycohoe/otel-grpc-cisco-receiver)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![OTel Issue](https://img.shields.io/badge/OTel%20Issue-%2343840-orange)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43840)

---

## The Problem

Cisco IOS XE switches send telemetry over **gRPC dial-out with kvGPB encoding** — a Cisco-specific protocol that the OpenTelemetry Collector does not natively support. Without a native receiver, Cisco switch telemetry requires a separate agent with its own config format and failure domain alongside an otherwise unified OTel Collector deployment.

## The Solution

The `cisco_telemetry` receiver is a native OTel Collector component that brings Cisco IOS XE MDT into a standard OTel pipeline. Cisco switches connect directly to the OTel Collector using the same MDT subscription configuration — no separate agent required.

```
Cisco IOS XE Switch  ──gRPC dial-out (kvGPB)──▶  OTel Collector  ──▶  Any Backend
```

One collector. One config format. One operational model. Every OTel Collector processor and exporter available to your network metrics.

---

## Key Features

- **gRPC Dial-Out**: Implements Cisco's `GRPCMdtDialout.MdtDialout` bidirectional streaming service
- **kvGPB Decoding**: Recursive parser for Cisco key-value Google Protocol Buffer payloads
- **Universal YANG Support**: Every Cisco IOS XE YANG path is processed — no xpath is rejected or dropped. All numeric fields become OTel metrics regardless of whether the module is known. The RFC 6020/7950 parser provides precise key and type metadata for any module when its `.yang` file is available; for completely unknown paths, the receiver infers types from the kvGPB wire encoding and still produces metrics.
- **Pre-Tuned Built-In Metadata**: Common IOS XE paths — interfaces, CPU, memory, environment, BGP, OSPF, PoE, TCAM, TrustSec, and more — ship with hardcoded key fields and metric type classifications for highest accuracy and performance out of the box; optionally extend coverage by loading the full Cisco YANG model set from the repository
- **Two-Pass Key Propagation**: YANG list keys (interface name, process name, sensor ID, etc.) are automatically attached as attributes on sibling numeric metrics so you can `GROUP BY interface_name` in any backend — no post-processing required
- **TLS / mTLS**: Standard OTel `configtls.ServerConfig` — certificate management integrates with existing OTel workflows; supports auto-reload on rotation
- **8 Internal Metrics**: Monitor the telemetry pipeline itself — messages received, bytes, active connections, YANG cache hits, processing latency — at the collector's self-metrics endpoint
- **7 Splunk Dashboards**: Ready-to-import Dashboard Studio panels covering infrastructure, network, routing, power, security, and telemetry health

---

## Quick Start

### Prerequisites

| Tool | Version |
|------|---------|
| Go | 1.23+ |
| OTel Collector Builder (`builder`) | v0.138.0 |
| Docker | 20.10+ (for Splunk stack) |
| Cisco IOS XE | 17.x+ |

### 1. Build the Custom Collector

```bash
go install go.opentelemetry.io/collector/cmd/builder@v0.138.0

git clone https://github.com/jeremycohoe/otel-grpc-cisco-receiver.git
cd otel-grpc-cisco-receiver
go mod tidy
builder --config=builder-config.yaml
```

The binary is written to `./build/cisco-otelcol`.

### 2. Download YANG Models (Optional)

The receiver processes **any** Cisco IOS XE YANG path — data is never dropped for unknown modules. However, YANG model files unlock two critical capabilities that significantly improve data quality:

**1. List key attribute propagation (affects all `GROUP BY` queries)**

Cisco kvGPB payloads store the identifier for a list entry (e.g., interface name, process name) as a sibling field alongside the counters. Without knowing which fields are YANG list keys, the receiver cannot reliably attach the identifier as an attribute on sibling metrics. This means queries like `BY interface_name` or `GROUP BY process_name` in Splunk/Prometheus will not work correctly for unfamiliar paths.

**2. Counter vs. Gauge classification (affects rate calculations)**

YANG distinguishes between counters (`counter64` — monotonically increasing, reset on wrap) and gauges (point-in-time values). OTel and Splunk use this to decide whether to calculate rates automatically. Without it, everything defaults to `Gauge`, so counters like `in-octets` won't produce correct bits/sec calculations.

The built-in metadata already covers these for the most common IOS XE telemetry paths. Download the full model set if you are subscribing to paths beyond what ships built-in, or want to guarantee correct behavior for any future subscription path you add:

```bash
./scripts/fetch-yang-models.sh           # defaults to IOS XE 17.18.1
./scripts/fetch-yang-models.sh 17151     # specific IOS XE release
```

Match the version to your switch's IOS XE release. Then point the receiver at the downloaded files:

```yaml
receivers:
  cisco_telemetry:
    yang:
      models_dir: ./yang-models
```

Downloaded files are parsed once at startup and cached to `yang-models/yang-cache.json` — subsequent starts load the cache instantly with no re-parse overhead.

### 3. Configure the Collector

Minimal setup (lab, no TLS):

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [cisco_telemetry]
      exporters: [debug]
```

Production with mTLS + Splunk HEC:

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
    max_recv_msg_size_mib: 8
    max_concurrent_streams: 256
    keepalive:
      time: 20s
      timeout: 5s
    tls:
      cert_file: /etc/otel/certs/server.crt
      key_file: /etc/otel/certs/server.key
      client_ca_file: /etc/otel/certs/ca.crt
      min_version: "1.2"
      reload_interval: 24h

exporters:
  splunk_hec:
    endpoint: "https://splunk.corp.example.com:8088"
    token: "${env:SPLUNK_HEC_TOKEN}"
    index: "metrics"

service:
  pipelines:
    metrics:
      receivers: [cisco_telemetry]
      exporters: [splunk_hec]
```

### 4. Configure the Cisco Switch

```cisco
telemetry ietf subscription 101
 encoding encode-kvgpb
 filter xpath /interfaces-ios-xe-oper:interfaces/interface/statistics
 source-address 10.0.1.10
 stream yang-push
 update-policy periodic 30000
 receiver ip address <COLLECTOR_IP> 57500 protocol grpc-tcp
```

Use `protocol grpc-tls` when TLS is enabled on the receiver. A complete set of 49 subscriptions covering the most common IOS XE telemetry paths is in [`c9300x-mdt-subscriptions.cfg`](c9300x-mdt-subscriptions.cfg).

Alternatively, push subscriptions to the switch programmatically via SSH using `configure-mdt.py` (requires [Netmiko](https://github.com/ktbyers/netmiko)):

```bash
pip install netmiko
python3 configure-mdt.py --host <SWITCH_IP> --collector <COLLECTOR_IP>
```

### 5. Start Splunk + Import Dashboards (Optional)

```bash
./start-splunk.sh          # Splunk Enterprise in Docker on port 8000
./start-otel.sh            # Start the collector
./scripts/import-dashboards.sh   # Import all 7 dashboards
```

Or use Docker Compose for the full stack:

```bash
docker compose up -d
```

Docker Compose starts:
- **otel-collector** on port `57500` (gRPC dial-out) and `8888` (self-metrics)
- **Splunk Enterprise** on port `8000` (Web UI) and `8088` (HEC)

Credentials: `admin` / `Cisco123` — HEC token: `cisco-mdt-token` — metrics index: `cisco_mdt`.

See [docker-compose.yaml](docker-compose.yaml) and [docker-collector-config.yaml](docker-collector-config.yaml).

---

## Verification

Confirm the switch is sending telemetry:

```cisco
! On the switch:
show telemetry ietf subscription all
show telemetry ietf subscription 101 detail
show telemetry ietf subscription 101 receiver
```

Confirm metrics are arriving in Splunk:

```bash
curl -sk -u admin:Cisco123 \
  'https://localhost:8089/services/search/jobs' \
  -d 'search=| mcatalog values(metric_name) WHERE index=cisco_mdt | stats count' \
  -d output_mode=json -d exec_mode=oneshot
```

Confirm the collector is receiving data via its self-metrics endpoint:

```bash
curl -s http://localhost:8888/metrics | grep cisco_telemetry
```

---

## Architecture

```
Cisco IOS XE  ──gRPC dial-out──▶  OTel Collector  ──exporter──▶  Backend
                                  ┌────────────────────┐
  kvGPB payload ────────────────▶ │  cisco_telemetry   │
  MdtDialoutArgs                  │    receiver        │
                                  │                    │
                                  │  ● Decode kvGPB    │
                                  │  ● Two-pass keys   │
                                  │  ● YANG type aware  │
                                  │  → OTel Metrics     │
                                  └──────────┬─────────┘
                                             │
                                      batch processor
                                             │
                              ┌─────────────┼─────────────┐
                              ▼             ▼             ▼
                          Splunk HEC   Prometheus    OTLP / Any
```

---

## How YANG Processing Works

Cisco kvGPB telemetry payloads are flat trees of key-value fields. To turn them into useful metrics, the receiver needs to know three things about each field: **is it a list key or a data value?**, **what numeric type is it?**, and **is it a counter (monotonically increasing) or a gauge (point-in-time)?** The YANG models answer all three questions.

### Two Layers of YANG Knowledge

**Layer 1 — Built-in metadata**

For the most common IOS XE telemetry paths, the receiver ships with pre-compiled Go structs that encode:
- Which fields are YANG list keys (e.g., `name` is the key for `/interfaces/interface`)
- The data type of each leaf (`uint64`, `string`, `enumeration`, etc.)
- Whether each counter is a `Sum` (monotonically increasing) or `Gauge` (point-in-time)
- Units metadata (`bytes`, `packets`, `percent`, `seconds`)

This layer requires no files on disk and has zero parse overhead at startup.

**Layer 2 — RFC 6020/7950 dynamic parser**

For any path not covered by the built-in set, the receiver parses raw `.yang` files at startup using a full RFC 6020/7950 compliant parser. It resolves `typedef` chains, `grouping`/`uses` expansions, `import` references, and `identity`/`identityref` inheritance to determine the same key/type/counter information dynamically. Results are cached to `yang-models/yang-cache.json` so subsequent startups are instant.

If neither layer has information for a path, the receiver falls back to safe defaults: `uint64` for unsigned integers, `Gauge` semantics, and no key attributes.

### Two-Pass Key Propagation

This is the core problem the receiver solves. In a kvGPB payload for a YANG list like `/interfaces/interface`, each list entry contains both **key fields** (the interface name) and **data fields** (in-octets, out-octets, errors) as siblings at the same level. A naive parser produces separate data points for each, losing the association between a counter value and which interface it belongs to.

The receiver uses a two-pass approach per list entry:

```
kvGPB list entry (one interface):
  ├── name: "GigabitEthernet1/0/1"    ← key field
  ├── in-octets: 1234567              ← data field
  ├── out-octets: 891234              ← data field
  └── in-errors: 0                   ← data field
```

**Pass 1** — scan all immediate children, identify fields whose names match the known YANG list keys for this path, and collect their values into a key map: `{"name": "GigabitEthernet1/0/1"}`.

**Pass 2** — walk all children again, create an OTel metric for each numeric data field, and stamp every data point with the key map as attributes.

Result in the backend:
```
cisco.interfaces.in-octets{name="GigabitEthernet1/0/1"} 1234567
cisco.interfaces.out-octets{name="GigabitEthernet1/0/1"} 891234
```

Without this, `GROUP BY interface_name` or `BY name` in Splunk/Prometheus would be impossible because the interface name would be a separate orphaned data point with no link to its sibling counters.

### When to Download the Full YANG Model Set

**Any xpath you configure on the switch will produce metrics** — this is unconditional. The receiver never drops data because a YANG module is unrecognized.

What the YANG model set improves is *metadata quality* for paths outside the built-in set:

| Scenario | Metrics produced? | List key attributes? | Counter/Gauge classification? |
|----------|:-----------------:|:--------------------:|:------------------------------:|
| Built-in metadata | ✓ | ✓ | ✓ |
| RFC parser + `.yang` file present | ✓ | ✓ | ✓ |
| Unknown path, no `.yang` file | ✓ | — (best-effort) | Gauge (conservative default) |

In the worst case — a completely unknown path with no `.yang` file — all numeric fields still become metrics with correct values. The only thing missing is YANG list key attributes (e.g., `interface_name` won't be stamped on sibling counters), which affects `GROUP BY` queries in backends. All other data is preserved.

Run `./scripts/fetch-yang-models.sh` to download the full Cisco IOS XE YANG model set from the [YangModels/yang](https://github.com/YangModels/yang) repository. The receiver parses them once at startup and caches the result.

---

## Performance

Benchmarked on a Cisco C9300X in a production network:

| Metric | Result |
|--------|--------|
| Throughput | >1,000 messages/second |
| Processing latency p99 | <10 ms |
| Memory per message | ~14 KB |
| Max concurrent streams | 256 (configurable) |

---

## Testing

```bash
# Run all tests with race detector
go test ./receiver/ciscotelemetryreceiver/ -count=1 -race

# Coverage report
go test -coverprofile=coverage.out ./receiver/ciscotelemetryreceiver/
go tool cover -html=coverage.out

# Benchmarks
go test -bench=. -benchmem ./receiver/ciscotelemetryreceiver/
```

| Metric | Result |
|--------|--------|
| Coverage | **83.6%** |
| Test cases | **80+** |
| Full suite runtime | **<5 seconds** |

---

## Splunk Dashboards

Seven Dashboard Studio dashboards, importable with one command:

| Dashboard | Key Panels |
|-----------|-----------|
| **Overview** | Device identity, CPU/Memory/PoE gauges, links to all category dashboards |
| **Infrastructure** | CPU, per-process memory, DRAM, temperature, fans, PSU, stack, HA |
| **Network** | Interface throughput/errors/rates, VLANs, STP, ARP, MAC, CDP, LLDP |
| **Routing** | BGP/OSPF state, prefix counts, adjacencies, DHCP, NTP |
| **Power & PoE** | PoE budget, per-port consumption, PSU readings, fan RPM |
| **Security** | 802.1X, TrustSec, ACLs, MACsec, TCAM utilization |
| **Telemetry Health** | MDT connections, subscription state, data volume, processing metrics |

---

## Configuration Reference

See [docs/CONFIG.md](docs/CONFIG.md) for the full field reference including keep-alive tuning, YANG parser options, and all TLS settings.

Key fields:

| Field | Default | Description |
|-------|---------|-------------|
| `listen_address` | `0.0.0.0:57500` | Host:port for gRPC dial-out connections |
| `max_recv_msg_size_mib` | `4` | Max inbound message size (MiB) |
| `max_concurrent_streams` | `128` | Max concurrent gRPC streams |
| `yang.enable_rfc_parser` | `true` | RFC 6020/7950 YANG type inference |
| `yang.cache_modules` | `true` | Cache parsed YANG modules in memory |
| `tls` | `nil` (plaintext) | Standard OTel `configtls.ServerConfig` |

## Security

See [docs/SECURITY.md](docs/SECURITY.md) for TLS/mTLS setup, Cisco trustpoint configuration, certificate auto-reload, and network hardening recommendations.

---

## Project Structure

```
├── receiver/ciscotelemetryreceiver/   # Core receiver implementation
│   ├── config.go                      # Configuration types and validation
│   ├── factory.go                     # OTel receiver factory
│   ├── receiver.go                    # Lifecycle (Start / Shutdown)
│   ├── grpc_service.go                # MdtDialout handler and metric conversion
│   ├── telemetry.go                   # Internal observability (8 metrics)
│   ├── yang_parser.go                 # Built-in YANG module metadata
│   ├── rfc_yang_parser.go             # RFC 6020/7950 dynamic parser
│   ├── yang_loader.go                 # YANG model file loader and cache
│   └── metadata.yaml                  # OTel component metadata
├── proto/                             # Cisco .proto files + generated Go bindings
├── docs/
│   ├── CONFIG.md                      # Full configuration field reference
│   ├── SECURITY.md                    # TLS / mTLS setup guide
│   └── TELEMETRY-CAPTURE.md           # Capture and validation workflow
├── splunk-dashboards/                 # Dashboard Studio JSON (7 dashboards)
│   ├── cisco_mdt_overview.json
│   ├── cisco_mdt_infrastructure.json
│   ├── cisco_mdt_network.json
│   ├── cisco_mdt_routing.json
│   ├── cisco_mdt_power.json
│   ├── cisco_mdt_security.json
│   └── cisco_mdt_telemetry.json
├── scripts/
│   ├── fetch-yang-models.sh           # Download all 848 Cisco IOS XE YANG modules
│   ├── generate-proto.sh              # Regenerate Go protobuf bindings
│   └── import-dashboards.sh           # Import all 7 dashboards into Splunk
├── c9300x-mdt-subscriptions.cfg       # 49 ready-to-apply IOS XE telemetry subscriptions
├── capture-config.yaml                # Per-subscription file capture (for validation)
├── parse-capture.py                   # Parse captured telemetry into readable formats
├── configure-mdt.py                   # Push MDT subscriptions to switches via SSH
├── start-splunk.sh                    # One-command Splunk Enterprise in Docker
├── start-otel.sh                      # Start the OTel collector
├── builder-config.yaml                # OTel Collector Builder manifest
├── collector-config.yaml              # Collector pipeline (Splunk HEC + debug)
├── docker-compose.yaml                # Full Splunk HEC stack
└── Dockerfile                         # Multi-stage collector build
```

---

## OpenTelemetry Contribution

This receiver is being prepared for contribution to [opentelemetry-collector-contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib), aligned with [Issue #43840](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43840) requesting a YANG gRPC receiver. Until then, it is available as a custom collector build via OTel Collector Builder.

---

## Development

```bash
# Generate protobuf Go code
./scripts/generate-proto.sh

# Run tests
go test ./receiver/ciscotelemetryreceiver/ -count=1
```

### Telemetry Capture and Validation

Before connecting a production backend, capture raw telemetry from a live switch to files for inspection:

```bash
# Run the collector in capture mode — writes one JSONL file per subscription
./build/cisco-otelcol --config capture-config.yaml

# Parse captured files into human-readable formats (data.txt, flat.jsonl, pretty JSON, markdown)
python3 parse-capture.py --rename --collect

# Review a specific subscription's output
cat telemetry-capture/_all-data-txt/1005-environment-sensors.txt
```

See [docs/TELEMETRY-CAPTURE.md](docs/TELEMETRY-CAPTURE.md) for the full capture and validation workflow.

---

## License

Apache License 2.0 — see [LICENSE](LICENSE).

---

## References

| Resource | Link |
|----------|------|
| Cisco MDT Configuration Guide | [cisco.com](https://www.cisco.com/c/en/us/td/docs/ios-xml/ios/prog/configuration/1718/b-1718-programmability-cg/model-driven-telemetry.html) |
| OpenTelemetry Collector | [opentelemetry.io/docs/collector](https://opentelemetry.io/docs/collector/) |
| OTel Collector Builder | [opentelemetry.io/docs/collector/custom-collector](https://opentelemetry.io/docs/collector/custom-collector/) |
| OTel contrib Issue #43840 | [github.com/open-telemetry/opentelemetry-collector-contrib/issues/43840](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43840) |
| Cisco Proto Definitions | [github.com/cisco-ie/cisco-proto](https://github.com/cisco-ie/cisco-proto) |
