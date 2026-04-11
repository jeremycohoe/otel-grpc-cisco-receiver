<!-- Use this file to provide workspace-specific custom instructions to Copilot. For more details, visit https://code.visualstudio.com/docs/copilot/copilot-customization#_use-a-githubcopilotinstructionsmd-file -->

# OpenTelemetry gRPC Receiver for Cisco IOS XE Telemetry

This project creates a custom OpenTelemetry collector component that can receive gRPC dial-out telemetry from Cisco IOS XE switches using kvGPB encoding.

## Project Context
- **Primary Goal**: Replace Telegraf cisco_telemetry_mdt plugin with native OTEL gRPC receiver
- **Protocol**: gRPC dial-out with kvGPB encoding
- **Target**: Cisco IOS XE switches sending telemetry to OTEL collector
- **Protobuf Schemas**: Cisco mdt_grpc_dialout.proto and telemetry.proto
- **Builder**: OTel Collector Builder v0.138.0; receiver uses `import:` field in builder-config.yaml
- **Downstream**: Splunk HEC (metrics index), Prometheus, any OTel-compatible backend

## Critical Issue: Key-Value Correlation
Numeric metrics (in-octets, holding-memory, etc.) and their YANG list keys (interface name, process name) are stored as SEPARATE OTel data points. The receiver must propagate list key values as attributes on sibling numeric metrics so backends can group BY entity. See `.github/instructions/engineering-plan.instructions.md` for the fix plan.

## Development Guidelines
- Use Go as primary language for OTEL collector components
- Follow OpenTelemetry collector component development patterns
- Implement proper protobuf handling for Cisco telemetry messages
- Support TLS/mTLS for secure gRPC connections
- Include configuration examples for Cisco switch integration
- Run `go test ./receiver/ciscotelemetryreceiver/... -v` after changes
- Build with `builder --config=builder-config.yaml` (requires Go 1.23+)
