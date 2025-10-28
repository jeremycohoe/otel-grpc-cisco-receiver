# Cisco Telemetry Receiver

| Status        |           |
| ------------- |-----------|
| Stability     | [development] |
| Distributions | [contrib] |
| Issues        | [![Open issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aopen%20label%3Areceiver%2Fciscotelemetry%20&label=open&color=orange&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aopen+is%3Aissue+label%3Areceiver%2Fciscotelemetry) |

## Description

The Cisco Telemetry receiver can be used to receive gRPC dial-out telemetry from Cisco IOS XE devices using the kvGPB (key-value Google Protocol Buffers) encoding format. This replaces the need for Telegraf's `cisco_telemetry_mdt` plugin by providing native OpenTelemetry support for Cisco Model Driven Telemetry (MDT).

## Configuration

The following settings are required:

- `listen_address` (default = `0.0.0.0:57500`): Address and port to bind to for incoming gRPC connections

The following settings are optional:

- `tls_enabled` (default = `false`): Enable TLS for gRPC connections
- `tls_cert_file`: Path to TLS certificate file (required if `tls_enabled` is true)
- `tls_key_file`: Path to TLS private key file (required if `tls_enabled` is true)
- `tls_client_ca_file`: Path to client CA certificate for mTLS
- `keep_alive_timeout` (default = `0s`): gRPC keep-alive timeout
- `max_message_size` (default = `0`): Maximum gRPC message size in bytes

### Example Configuration

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
    tls_enabled: false
    keep_alive_timeout: 60s
    max_message_size: 4194304  # 4MB
```

### TLS Configuration Example

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
    tls_enabled: true
    tls_cert_file: "/path/to/server.crt"
    tls_key_file: "/path/to/server.key"
    tls_client_ca_file: "/path/to/ca.crt"  # For mTLS
```

## Cisco Device Configuration

Configure your Cisco IOS XE device to send telemetry to the receiver:

```cisco
telemetry ietf subscription 101
 encoding encode-kvgpb
 filter xpath /interfaces-ios-xe-oper:interfaces/interface/statistics
 source-address 192.168.1.10
 stream yang-push
 update-policy periodic 30000
 receiver ip address 192.168.1.100 57500 protocol grpc-tcp
```

For TLS connections, use `protocol grpc-tls` instead of `protocol grpc-tcp`.

## Supported Metrics

The receiver processes kvGPB telemetry data and converts it to OpenTelemetry metrics format. All numeric fields become gauge metrics with names like `cisco.{field_path}`. String fields become info metrics with the suffix `_info`.

### Resource Attributes

The following resource attributes are automatically added:

| Name | Description | Values | Enabled |
| ---- | ----------- | ------ | ------- |
| cisco.node_id | Cisco device identifier | Any string | Yes |
| cisco.subscription_id | Subscription identifier | Any string | Yes |
| cisco.encoding_path | YANG model path for the telemetry data | Any string | Yes |

### Metric Attributes

| Name | Description | Values | Enabled |
| ---- | ----------- | ------ | ------- |
| encoding_path | YANG model path for this specific metric | Any string | Yes |
| value | Original string value (for info metrics only) | Any string | Yes |

## Protocol Details

This receiver implements the Cisco MDT gRPC dialout protocol as defined in:
- `mdt_grpc_dialout.proto`: gRPC service definition
- `telemetry.proto`: Telemetry message format

The receiver supports:
- Bidirectional streaming gRPC
- kvGPB (key-value GPB) encoding format  
- TLS/mTLS for secure connections
- Automatic acknowledgment of received data

## Limitations

- GPB table format is not currently supported (placeholder implementation exists)
- Only supports gRPC dial-out (device initiates connection)
- Requires kvGPB encoding format

## References

- [Cisco Model Driven Telemetry Configuration Guide](https://www.cisco.com/c/en/us/td/docs/ios-xml/ios/prog/configuration/1718/b-1718-programmability-cg/model-driven-telemetry.html)
- [Cisco Proto Definitions](https://github.com/cisco-ie/cisco-proto)