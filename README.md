# Cisco IOS XE MDT Telemetry Receiver for OpenTelemetry Collector

A native OpenTelemetry Collector receiver for **Cisco IOS XE Model-Driven Telemetry (MDT)** over **gRPC dial-out** with **kvGPB** encoding.

This receiver replaces Telegraf's `cisco_telemetry_mdt` plugin with a first-class OTel component that can export to **any** backend supported by the OpenTelemetry Collector — Splunk HEC, Prometheus, Datadog, Grafana Mimir, OTLP, etc.

## Architecture

```
Cisco IOS XE Switch ──gRPC dial-out (kvGPB)──▶ OTEL Collector (cisco-otelcol) ──Splunk HEC──▶ Splunk Enterprise
```

## Features

- **gRPC Dial-Out**: Cisco switch initiates the connection (`gRPCMdtDialout.MdtDialout` bidirectional streaming)
- **kvGPB Decoding**: Decodes key-value Google Protocol Buffer telemetry payloads
- **YANG Intelligence**: RFC 6020/7950 compliant parser with **27 built-in Cisco IOS XE modules** for semantic type inference, key-value correlation, and counter/gauge classification
- **Two-Pass Key Propagation**: YANG list keys (interface name, process name, sensor ID, etc.) are automatically attached as attributes on all sibling metrics, enabling `BY interface_name` grouping in Splunk/Prometheus
- **mTLS / TLS**: Standard OTel `configtls.ServerConfig` — cert files, client CA, min TLS version, reload interval
- **Internal Observability**: 8 OTel SDK metrics (messages received, bytes, errors, active connections, processing time, YANG cache hits/misses, metrics produced)
- **Splunk Dashboard**: Pre-built 30-panel Splunk dashboard with multi-switch support
- **Backend Agnostic**: Works with any OTel Collector exporter

## Supported YANG Modules (27)

| Module | Subscription XPath | Data |
|--------|-------------------|------|
| Cisco-IOS-XE-interfaces-oper | `/interfaces-ios-xe-oper:interfaces/interface` | Interface stats, rates, errors |
| Cisco-IOS-XE-process-cpu-oper | `/process-cpu-ios-xe-oper:cpu-usage/cpu-utilization` | CPU utilization (5s/1m/5m) |
| Cisco-IOS-XE-process-memory-oper | `/process-memory-ios-xe-oper:memory-usage-processes` | Per-process memory usage |
| Cisco-IOS-XE-environment-oper | `/environment-ios-xe-oper:environment-sensors` | Temperature, fan, power sensors |
| Cisco-IOS-XE-arp-oper | `/arp-ios-xe-oper:arp-data` | ARP table entries |
| Cisco-IOS-XE-cdp-oper | `/cdp-ios-xe-oper:cdp-neighbor-details` | CDP neighbor discovery |
| Cisco-IOS-XE-lldp-oper | `/lldp-ios-xe-oper:lldp-entries` | LLDP neighbor entries |
| Cisco-IOS-XE-matm-oper | `/matm-ios-xe-oper:matm-oper-data` | MAC address table |
| Cisco-IOS-XE-mdt-oper | `/mdt-oper:mdt-oper-data/mdt-subscriptions` | Telemetry subscription health |
| Cisco-IOS-XE-platform-oper | `/platform-ios-xe-oper:components` | Platform components, temperature |
| Cisco-IOS-XE-platform-software-oper | `/platform-sw-ios-xe-oper:cisco-platform-software/control-processes` | System DRAM memory |
| Cisco-IOS-XE-poe-oper | `/poe-ios-xe-oper:poe-oper-data` | PoE per-port power usage |
| Cisco-IOS-XE-spanning-tree-oper | `/stp-ios-xe-oper:stp-details` | STP instance/port state |
| Cisco-IOS-XE-stack-oper | `/stack-ios-xe-oper:stack-oper-data` | Stack member health |
| Cisco-IOS-XE-identity-oper | `/identity-ios-xe-oper:identity-oper-data` | 802.1X / identity sessions |
| Cisco-IOS-XE-switchport-oper | `/switchport-ios-xe-oper:switchport-oper-data` | Switchport mode/VLAN |
| Cisco-IOS-XE-udld-oper | `/udld-ios-xe-oper:udld-oper-data` | UDLD neighbor status |
| Cisco-IOS-XE-transceiver-oper | `/xcvr-ios-xe-oper:transceiver-oper-data` | Optics/transceiver info |
| Cisco-IOS-XE-vlan-oper | `/vlan-ios-xe-oper:vlans` | VLAN operational data |
| Cisco-IOS-XE-install-oper | `/install-ios-xe-oper:install-oper-data` | Software install packages |
| Cisco-IOS-XE-device-hardware-oper | `/device-hardware-xe-oper:device-hardware-data/device-hardware` | Uptime, SW version, HW inventory |
| Cisco-IOS-XE-bgp-oper | `/bgp-ios-xe-oper:bgp-state-data` | BGP neighbor state |
| Cisco-IOS-XE-ospf-oper | `/ospf-ios-xe-oper:ospf-oper-data` | OSPF instance/area data |
| Cisco-IOS-XE-tcam-oper | `/tcam-ios-xe-oper:tcam-details` | TCAM utilization |
| Cisco-IOS-XE-dhcp-oper | `/dhcp-ios-xe-oper:dhcp-oper-data` | DHCP pool stats |
| Cisco-IOS-XE-ha-oper | `/ha-ios-xe-oper:ha-oper-data` | High availability state |
| Cisco-IOS-XE-linecard-oper | `/linecard-ios-xe-oper:linecard-oper-data` | Linecard status |
| Cisco-IOS-XE-trustsec-oper | `/trustsec-ios-xe-oper:trustsec-state` | TrustSec SGT/SXP |

## Quick Start

### Prerequisites

| Tool | Version |
|------|---------|
| Go | 1.23+ |
| OTel Collector Builder (`builder`) | v0.138.0 |
| Docker | 20.10+ (for Splunk) |
| Cisco IOS XE | 17.x+ |

### 1. Build the Custom Collector

```bash
# Install the OTel Collector Builder
go install go.opentelemetry.io/collector/cmd/builder@v0.138.0

# Clone and build
git clone https://github.com/jeremycohoe/otel-grpc-cisco-receiver.git
cd otel-grpc-cisco-receiver
go mod tidy
builder --config=builder-config.yaml
```

The binary is written to `./build/cisco-otelcol`.

### 2. Start Splunk

```bash
./start-splunk.sh
```

This starts Splunk Enterprise in Docker with:
- **Web UI**: http://\<host\>:8000 (admin / Cisco123)
- **HEC endpoint**: https://\<host\>:8088 (token: `cisco-mdt-token`)
- **cisco_mdt** metrics index auto-created

### 3. Start the Collector

```bash
./start-otel.sh
```

Or run directly:

```bash
./build/cisco-otelcol --config collector-config.yaml
```

The collector listens on `0.0.0.0:57500` for gRPC dial-out connections.

### 4. Configure the Cisco Switch

Apply the telemetry subscriptions — replace `<COLLECTOR_IP>` with your collector host:

```cisco
telemetry ietf subscription 101
 encoding encode-kvgpb
 filter xpath /arp-ios-xe-oper:arp-data
 stream yang-push
 update-policy periodic 30000
 receiver ip address <COLLECTOR_IP> 57500 protocol grpc-tcp

telemetry ietf subscription 109
 encoding encode-kvgpb
 filter xpath /process-cpu-ios-xe-oper:cpu-usage/cpu-utilization
 stream yang-push
 update-policy periodic 30000
 receiver ip address <COLLECTOR_IP> 57500 protocol grpc-tcp
```

A complete set of 21 subscriptions is in [`c9300x-mdt-subscriptions.cfg`](c9300x-mdt-subscriptions.cfg). Update the receiver IP address and apply to your switch.

### 5. Import the Splunk Dashboard

```bash
curl -sk -u admin:Cisco123 \
  'https://localhost:8089/servicesNS/admin/search/data/ui/views/cisco_mdt_overview' \
  -X POST \
  --data-urlencode "eai:data@splunk-dashboards/cisco_mdt_overview.xml"
```

The dashboard is at http://\<host\>:8000 → **Cisco IOS XE - Model Driven Telemetry**.

### Verification

```cisco
! On the switch:
show telemetry ietf subscription all
show telemetry ietf subscription 101 detail
show telemetry ietf subscription 101 receiver
```

```bash
# On the collector host — check for metrics arriving:
curl -sk -u admin:Cisco123 \
  'https://localhost:8089/services/search/jobs' \
  -d 'search=| mcatalog values(metric_name) WHERE index=cisco_mdt | stats count' \
  -d output_mode=json -d exec_mode=oneshot
```

## Docker Compose

A full stack is also available via Docker Compose:

```bash
docker compose up -d
```

This starts:
- **otel-collector** on port 57500 (gRPC) and 8888 (self-metrics)
- **Splunk Enterprise** on port 8000 (Web UI) and 8088 (HEC)

See [docker-compose.yaml](docker-compose.yaml) and [docker-collector-config.yaml](docker-collector-config.yaml).

## Configuration Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `listen_address` | string | `0.0.0.0:57500` | Host:port for gRPC dial-out |
| `max_recv_msg_size_mib` | int | `4` | Max inbound message size (MiB) |
| `max_concurrent_streams` | uint32 | `128` | Max concurrent gRPC streams |
| `keepalive.time` | duration | `30s` | Server-side keep-alive ping interval |
| `keepalive.timeout` | duration | `10s` | Keep-alive response timeout |
| `yang.enable_rfc_parser` | bool | `true` | Use RFC 6020/7950 YANG parser |
| `yang.cache_modules` | bool | `true` | Cache parsed YANG modules |
| `yang.max_modules` | int | `1000` | Max YANG modules to cache |
| `tls` | configtls.ServerConfig | *nil* | Standard OTel TLS config (nil = plaintext) |

### TLS / mTLS

```yaml
tls:
  cert_file: /path/to/server.crt
  key_file: /path/to/server.key
  client_ca_file: /path/to/ca.crt    # enables mutual TLS
  min_version: "1.2"
  reload_interval: 24h
```

When `tls` is omitted the server runs plaintext — useful for lab environments.

## Project Structure

```
├── receiver/ciscotelemetryreceiver/   # Core receiver implementation
│   ├── config.go                      # Configuration types & validation
│   ├── factory.go                     # OTel receiver factory
│   ├── receiver.go                    # Lifecycle (Start / Shutdown)
│   ├── grpc_service.go                # MdtDialout handler, metric conversion
│   ├── telemetry.go                   # Internal observability (8 metrics)
│   ├── yang_parser.go                 # 27 built-in YANG modules
│   ├── rfc_yang_parser.go             # RFC 6020/7950 parser
│   └── metadata.yaml                  # OTel component metadata
├── proto/                             # Cisco .proto files + generated Go
├── splunk-dashboards/                 # Pre-built Splunk dashboard XML
│   └── cisco_mdt_overview.xml         # 30-panel dashboard with multi-switch support
├── c9300x-mdt-subscriptions.cfg       # 21 IOS XE telemetry subscriptions
├── start-splunk.sh                    # One-command Splunk Enterprise setup
├── start-otel.sh                      # Start the collector
├── docker-compose.yaml                # Full Splunk HEC stack
├── Dockerfile                         # Multi-stage build
├── builder-config.yaml                # OTel Collector Builder manifest
└── collector-config.yaml              # Collector pipeline config
```

## Splunk Dashboard

The included dashboard provides 30 panels across 18 rows:

| Row | Panels |
|-----|--------|
| Device Overview | Software version, boot time, reboot reason, hardware inventory |
| CPU | Utilization over time (5s/1m/5m), current gauge |
| Process Memory | Top processes by allocated memory, allocated vs freed |
| Environment | Sensor readings (temperature, fan, power) |
| Interfaces | Throughput (Rx/Tx Kbps), packet rates |
| ARP + CDP | ARP entry count, CDP neighbors |
| MATM + LLDP | MAC address table, LLDP neighbors |
| Platform | Component temperature readings |
| MDT Health | Subscription state and update counts |
| PoE | Per-port power consumption |
| Per-Process CPU | Individual process CPU usage |
| System DRAM | Total/used/free memory in GB |
| STP | Spanning tree instance and port state |
| Stack | Stack member roles and health |
| VLANs + 802.1X | VLAN list, EAPOL statistics |
| Switchport + Transceiver | Port mode/VLAN, optics status |
| Software Install | Installed packages by switch |

## Data Flow

```
Cisco IOS XE  ──gRPC dial-out──▶  OTel Collector  ──exporter──▶  Backend
                                  ┌──────────────────┐
  kvGPB payload ──────────────▶   │ cisco_telemetry   │
  MdtDialoutArgs                  │   receiver        │
                                  │                   │
                                  │ ● Decode kvGPB    │
                                  │ ● Two-pass keys   │
                                  │ ● YANG type aware  │
                                  │ ● → OTel Metrics   │
                                  └────────┬──────────┘
                                           │
                                    batch processor
                                           │
                             ┌─────────────┼─────────────┐
                             ▼             ▼             ▼
                         Splunk HEC   Prometheus    Debug/OTLP
```

## Development

```bash
# Run all tests
go test ./receiver/ciscotelemetryreceiver/ -count=1 -race

# Benchmarks
go test -bench=. -benchmem ./receiver/ciscotelemetryreceiver/

# Coverage report
go test -coverprofile=coverage.out ./receiver/ciscotelemetryreceiver/
go tool cover -html=coverage.out
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).

## References

- [Cisco Model-Driven Telemetry Guide](https://www.cisco.com/c/en/us/td/docs/ios-xml/ios/prog/configuration/1718/b-1718-programmability-cg/model-driven-telemetry.html)
- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
- [Cisco Proto Definitions](https://github.com/cisco-ie/cisco-proto)
- [OTel Collector Builder](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder)