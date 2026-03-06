# Configuration Reference

Complete configuration options for the `cisco_telemetry` receiver.

## Minimal Configuration

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
```

## Full Configuration

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
    max_recv_msg_size_mib: 4
    max_concurrent_streams: 128
    keepalive:
      time: 30s
      timeout: 10s
      enforcement_min_time: 30s
      enforcement_permit_no_stream: true
    yang:
      enable_rfc_parser: true
      cache_modules: true
      max_modules: 1000
    tls:
      cert_file: /etc/otel/certs/server.crt
      key_file: /etc/otel/certs/server.key
      client_ca_file: /etc/otel/certs/ca.crt
      min_version: "1.2"
      reload_interval: 24h
```

---

## Field Reference

### Network

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `listen_address` | string | `0.0.0.0:57500` | Host:port to bind for incoming gRPC dial-out connections. |
| `max_recv_msg_size_mib` | int | `4` | Maximum inbound gRPC message size in MiB. Set higher if your switch sends large payloads. |
| `max_concurrent_streams` | uint32 | `128` | Maximum concurrent gRPC streams the server will accept. |

### Keep-Alive (`keepalive`)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `time` | duration | `30s` | Interval between server-side keep-alive pings. Set `0` to disable. |
| `timeout` | duration | `10s` | How long the server waits for a keep-alive ack before closing the connection. |
| `enforcement_min_time` | duration | `30s` | Minimum interval the server enforces between client pings. |
| `enforcement_permit_no_stream` | bool | `true` | Allow client keep-alive pings when there are no active streams. |

### YANG Parser (`yang`)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enable_rfc_parser` | bool | `true` | Enable the RFC 6020/7950 compliant YANG parser for semantic type inference. |
| `cache_modules` | bool | `true` | Cache discovered YANG modules in memory. |
| `max_modules` | int | `1000` | Maximum number of YANG modules to cache. |

### TLS / mTLS (`tls`)

The `tls` block uses the standard OpenTelemetry [`configtls.ServerConfig`](https://pkg.go.dev/go.opentelemetry.io/collector/config/configtls#ServerConfig). When omitted (nil), the server runs in plaintext mode.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `cert_file` | string | — | Path to server certificate (PEM). |
| `key_file` | string | — | Path to server private key (PEM). |
| `client_ca_file` | string | — | Path to CA certificate for verifying client certs (enables mTLS). |
| `min_version` | string | `"1.2"` | Minimum TLS version (`"1.0"` – `"1.3"`). |
| `max_version` | string | `"1.3"` | Maximum TLS version. |
| `reload_interval` | duration | `0` | How often to reload certificates from disk. `0` = no auto-reload. |

See the [configtls docs](https://pkg.go.dev/go.opentelemetry.io/collector/config/configtls) for all supported fields (cipher suites, curve preferences, etc.).

---

## Example: Lab / Development

No TLS, debug exporter only.

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

## Example: Production with mTLS + Splunk HEC

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
    yang:
      enable_rfc_parser: true
      cache_modules: true
      max_modules: 2000

processors:
  batch:
    timeout: 2s
    send_batch_size: 2048

exporters:
  splunk_hec:
    endpoint: "https://splunk.corp.example.com:8088/services/collector"
    token: "${SPLUNK_HEC_TOKEN}"
    source: "cisco:mdt"
    sourcetype: "cisco:mdt:grpc"
    index: "cisco_metrics"

service:
  pipelines:
    metrics:
      receivers: [cisco_telemetry]
      processors: [batch]
      exporters: [splunk_hec]
```

## Validation

The receiver validates the config at startup. Errors include:

- `listen_address must not be empty`
- `max_concurrent_streams must be greater than 0`
- `max_recv_msg_size_mib must be non-negative`
- `yang.max_modules must be non-negative`
- TLS certificate load failures (wrong path, bad PEM, etc.)
