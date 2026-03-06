# Cisco IOS XE MDT Telemetry Receiver for OpenTelemetry Collector

A native OpenTelemetry Collector receiver for **Cisco IOS XE Model-Driven Telemetry (MDT)** over **gRPC dial-out** with **kvGPB** encoding.

This receiver replaces Telegraf's `cisco_telemetry_mdt` plugin with a first-class OTel component that can export to **any** backend supported by the OpenTelemetry Collector — Splunk HEC, Prometheus, Datadog, Grafana Mimir, OTLP, etc.

## Features

- **gRPC Dial-Out**: Cisco switch initiates the connection (`gRPCMdtDialout.MdtDialout` bidirectional streaming)
- **kvGPB Decoding**: Decodes key-value Google Protocol Buffer telemetry payloads
- **YANG Intelligence**: RFC 6020/7950 compliant parser + 4 built-in modules (CPU, interfaces, BGP, OSPF) for semantic type inference
- **mTLS / TLS**: Standard OTel `configtls.ServerConfig` — cert files, client CA, min TLS version, reload interval
- **Internal Observability**: 8 OTel SDK metrics (messages received, bytes, errors, active connections, processing time, YANG cache hits/misses, metrics produced)
- **Backend Agnostic**: Works with any OTel Collector exporter

## Quick Start

### Prerequisites

| Tool | Version |
|------|---------|
| Go | 1.23+ |
| OTel Collector Builder (`builder`) | v0.138.0 |
| Cisco IOS XE | 16.9+ |

### 1. Build the Custom Collector

```bash
# Install the OTel Collector Builder
go install go.opentelemetry.io/collector/cmd/builder@v0.138.0

# Clone and build
git clone https://github.com/jcohoe/otel-grpc-cisco-receiver.git
cd otel-grpc-cisco-receiver
go mod tidy
builder --config=builder-config.yaml
```

The binary is written to `./build/cisco-otelcol`.

### 2. Configure the Collector

Edit `collector-config.yaml` (or use the provided example):

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
    max_recv_msg_size_mib: 4
    max_concurrent_streams: 128
    keepalive:
      time: 30s
      timeout: 10s
    yang:
      enable_rfc_parser: true
      cache_modules: true
      max_modules: 1000
    # Uncomment for mTLS:
    # tls:
    #   cert_file: /etc/otel/certs/server.crt
    #   key_file:  /etc/otel/certs/server.key
    #   client_ca_file: /etc/otel/certs/ca.crt

processors:
  batch:
    timeout: 1s
    send_batch_size: 1024

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [cisco_telemetry]
      processors: [batch]
      exporters: [debug]
```

### 3. Run

```bash
./build/cisco-otelcol --config collector-config.yaml
```

### 4. Configure the Cisco Switch

```cisco
telemetry ietf subscription 101
 encoding encode-kvgpb
 filter xpath /process-cpu-ios-xe-oper:cpu-usage/cpu-utilization
 source-address 10.1.1.1
 stream yang-push
 update-policy periodic 30000
 receiver ip address 10.1.1.100 57500 protocol grpc-tcp
```

Replace `10.1.1.100` with the host running the collector.

## Docker Compose (with Splunk HEC)

A full stack is provided: **Cisco switch → OTel Collector → Splunk Enterprise**.

```bash
docker compose up -d
```

This starts:
- **otel-collector** on port 57500 (gRPC) and 8888 (self-metrics)
- **Splunk Enterprise** on port 8000 (Web UI) and 8088 (HEC)

Default Splunk credentials: `admin` / `changeme123`

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

The receiver uses the standard OpenTelemetry `configtls.ServerConfig`:

```yaml
tls:
  cert_file: /path/to/server.crt
  key_file: /path/to/server.key
  client_ca_file: /path/to/ca.crt    # enables mutual TLS
  min_version: "1.2"                  # optional, default: 1.2
  reload_interval: 24h                # auto-reload certs
```

When `tls` is omitted (or null) the server runs plaintext — useful for lab environments.

## Project Structure

```
├── receiver/ciscotelemetryreceiver/   # Core receiver implementation
│   ├── config.go                      # Configuration types & validation
│   ├── factory.go                     # OTel receiver factory
│   ├── receiver.go                    # Lifecycle (Start / Shutdown)
│   ├── grpc_service.go                # MdtDialout handler, metric conversion
│   ├── telemetry.go                   # Internal observability (8 metrics)
│   ├── yang_parser.go                 # Built-in YANG modules
│   ├── rfc_yang_parser.go             # RFC 6020/7950 parser
│   └── metadata.yaml                  # OTel component metadata
├── proto/                             # Cisco .proto files + generated Go
├── examples/                          # Config examples
├── docker-compose.yaml                # Full Splunk HEC stack
├── Dockerfile                         # Multi-stage build
├── builder-config.yaml                # OTel Collector Builder manifest
└── collector-config.yaml              # Default collector config
```

## Data Flow

```
Cisco IOS XE  ──gRPC dial-out──▶  OTel Collector  ──exporter──▶  Backend
                                  ┌──────────────┐
  kvGPB payload ──────────────▶   │ cisco_telemetry │
  MdtDialoutArgs                  │   receiver      │
                                  │                 │
                                  │  ● Decode kvGPB │
                                  │  ● YANG parser  │
                                  │  ● → OTel Metrics│
                                  └────────┬────────┘
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

## Common Cisco IOS XE Subscriptions

```cisco
! CPU utilization (every 30 seconds)
telemetry ietf subscription 101
 encoding encode-kvgpb
 filter xpath /process-cpu-ios-xe-oper:cpu-usage/cpu-utilization
 stream yang-push
 update-policy periodic 30000
 receiver ip address <COLLECTOR_IP> 57500 protocol grpc-tcp

! Interface statistics (every 10 seconds)
telemetry ietf subscription 102
 encoding encode-kvgpb
 filter xpath /interfaces-ios-xe-oper:interfaces/interface/statistics
 stream yang-push
 update-policy periodic 10000
 receiver ip address <COLLECTOR_IP> 57500 protocol grpc-tcp

! Memory usage (every 30 seconds)
telemetry ietf subscription 103
 encoding encode-kvgpb
 filter xpath /memory-ios-xe-oper:memory-statistics/memory-statistic
 stream yang-push
 update-policy periodic 30000
 receiver ip address <COLLECTOR_IP> 57500 protocol grpc-tcp
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).

## References

- [Cisco Model-Driven Telemetry Guide](https://www.cisco.com/c/en/us/td/docs/ios-xml/ios/prog/configuration/1718/b-1718-programmability-cg/model-driven-telemetry.html)
- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
- [Cisco Proto Definitions](https://github.com/cisco-ie/cisco-proto)
- [OTel Collector Builder](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder)