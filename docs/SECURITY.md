# Security Guide

## TLS / mTLS

The `cisco_telemetry` receiver uses the standard OpenTelemetry `configtls.ServerConfig` for TLS. This is the same configuration structure used by all OTel Collector components, so it plugs into existing certificate management workflows.

### Server-side TLS (encrypt only)

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
    tls:
      cert_file: /etc/otel/certs/server.crt
      key_file: /etc/otel/certs/server.key
```

The switch connects over TLS but does not present a client certificate.

### Mutual TLS (mTLS)

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
    tls:
      cert_file: /etc/otel/certs/server.crt
      key_file: /etc/otel/certs/server.key
      client_ca_file: /etc/otel/certs/ca.crt
```

When `client_ca_file` is set, the server requires and verifies a client certificate from the Cisco switch. This is the **recommended** approach for production deployments — it ensures only authorised switches can stream telemetry.

### Cisco IOS XE Switch TLS Configuration

On the switch side, configure the receiver with TLS:

```cisco
! Create a trustpoint for the CA and server certificate
crypto pki trustpoint OTEL-CA
 enrollment terminal
 revocation-check none

! Import the CA certificate
crypto pki authenticate OTEL-CA

! For mTLS, also create a client certificate trustpoint
crypto pki trustpoint OTEL-CLIENT
 enrollment terminal
 revocation-check none

! Configure the telemetry receiver with TLS
telemetry ietf subscription 101
 encoding encode-kvgpb
 filter xpath /process-cpu-ios-xe-oper:cpu-usage/cpu-utilization
 stream yang-push
 update-policy periodic 30000
 receiver ip address <COLLECTOR_IP> 57500 protocol grpc-tls profile OTEL-CA
```

Note: Use `protocol grpc-tls` (not `grpc-tcp`) when TLS is enabled.

### Certificate Auto-Reload

Set `reload_interval` to automatically pick up renewed certificates without restarting the collector:

```yaml
tls:
  cert_file: /etc/otel/certs/server.crt
  key_file: /etc/otel/certs/server.key
  client_ca_file: /etc/otel/certs/ca.crt
  reload_interval: 24h
```

### Plaintext Mode

Omit the `tls` block entirely to run without encryption (lab/dev only):

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
```

## Network Hardening

- Bind to a specific interface instead of `0.0.0.0` when the collector is multi-homed.
- Use `max_concurrent_streams` to limit resource consumption from misbehaving clients.
- Place the collector behind a firewall that only allows the management subnet of your switches.

## Internal Metrics

The receiver emits 8 OTel SDK metrics under the scope `github.com/jcohoe/otel-grpc-cisco-receiver`:

| Metric | Type | Description |
|--------|------|-------------|
| `cisco_telemetry_receiver_messages_received` | counter | Total messages received |
| `cisco_telemetry_receiver_messages_processed` | counter | Total messages processed successfully |
| `cisco_telemetry_receiver_messages_dropped` | counter | Total messages dropped |
| `cisco_telemetry_receiver_bytes_received` | counter | Total bytes received |
| `cisco_telemetry_receiver_connections_active` | gauge | Current active gRPC streams |
| `cisco_telemetry_receiver_yang_modules_discovered` | counter | YANG modules discovered |
| `cisco_telemetry_receiver_processing_duration` | histogram | Per-message processing latency |
| `cisco_telemetry_receiver_grpc_errors` | counter | Total gRPC errors |

These are available at the collector's self-metrics endpoint (default `:8888`).
