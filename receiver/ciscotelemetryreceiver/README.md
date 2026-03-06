# Cisco Telemetry Receiver

| Status    |               |
|-----------|---------------|
| Stability | [development] |

## Description

OpenTelemetry Collector receiver for **Cisco IOS XE gRPC Dial-Out Model-Driven Telemetry (MDT)** using kvGPB encoding. Replaces Telegraf's `cisco_telemetry_mdt` plugin with native OTel support.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `listen_address` | string | `0.0.0.0:57500` | Address and port for incoming gRPC connections |
| `tls` | `configtls.ServerConfig` | *nil* (plaintext) | TLS / mTLS settings — see [SECURITY.md](../../docs/SECURITY.md) |
| `max_recv_msg_size_mib` | int | `4` | Maximum inbound gRPC message size in MiB |
| `max_concurrent_streams` | uint32 | `128` | Maximum concurrent gRPC streams |
| `keepalive.time` | duration | `30s` | Server keepalive ping interval |
| `keepalive.timeout` | duration | `10s` | Keepalive ping timeout |
| `yang.enable_rfc_parser` | bool | `true` | Enable RFC 6020/7950 YANG parser |
| `yang.cache_modules` | bool | `true` | Cache parsed YANG modules |
| `yang.max_modules` | int | `1000` | Maximum cached YANG modules |

### Minimal Example

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
```

### mTLS Example

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
    tls:
      cert_file: /etc/otel/certs/server.crt
      key_file: /etc/otel/certs/server.key
      client_ca_file: /etc/otel/certs/ca.crt
```

See [docs/CONFIG.md](../../docs/CONFIG.md) for the full configuration reference.

## Cisco IOS XE Device Configuration

```cisco
telemetry ietf subscription 101
 encoding encode-kvgpb
 filter xpath /interfaces-ios-xe-oper:interfaces/interface/statistics
 source-address 10.0.0.1
 stream yang-push
 update-policy periodic 30000
 receiver ip address <COLLECTOR_IP> 57500 protocol grpc-tcp
```

Use `protocol grpc-tls` when TLS is enabled on the collector.

## Emitted Metrics

Numeric telemetry fields → gauge metrics named `cisco.<field>`.
String telemetry fields → gauge metrics named `cisco.<field>_info` (value = 1, string in attribute).

### Resource Attributes

| Attribute | Description |
|-----------|-------------|
| `cisco.node_id` | Device hostname / node ID |
| `cisco.subscription_id` | Subscription identifier |
| `cisco.encoding_path` | YANG encoding path |

## Limitations

- Only kvGPB encoding is supported (GPB table format is not implemented).
- Only gRPC dial-out (device-initiated) connections are supported.