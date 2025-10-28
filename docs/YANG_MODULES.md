# YANG Modules and Data Model Guide

This guide explains how the Cisco Telemetry Receiver handles YANG data models and provides information about supported Cisco IOS XE YANG modules.

## Table of Contents

- [Overview](#overview)
- [Supported YANG Modules](#supported-yang-modules)
- [YANG Parser Configuration](#yang-parser-configuration)
- [Data Type Mapping](#data-type-mapping)
- [Common YANG Paths](#common-yang-paths)
- [Troubleshooting YANG Issues](#troubleshooting-yang-issues)
- [Custom YANG Module Support](#custom-yang-module-support)
- [YANG Tools and Utilities](#yang-tools-and-utilities)

## Overview

The Cisco Telemetry Receiver includes an advanced YANG parser that can automatically discover and process YANG data models used by Cisco IOS XE devices. This enables automatic data type inference, path validation, and structured metric generation.

### YANG Parser Features

- **Automatic Module Discovery**: Detects YANG modules from telemetry data
- **RFC-Compliant Parsing**: Supports standard YANG 1.0 and 1.1 syntax
- **Type Inference**: Automatically maps YANG types to OpenTelemetry metric types
- **Path Validation**: Validates telemetry paths against YANG schema
- **Caching**: Improves performance by caching parsed modules
- **Error Recovery**: Graceful handling of invalid or incomplete YANG data

### Architecture

```
Cisco Device → gRPC Stream → Telemetry Message → YANG Parser → OpenTelemetry Metrics
                                      ↓
                               YANG Module Cache
```

## Supported YANG Modules

The receiver supports the following categories of YANG modules commonly used in Cisco IOS XE:

### Standard IETF Modules

| Module | Description | Supported Paths |
|--------|-------------|----------------|
| `ietf-interfaces` | Interface statistics and configuration | `/interfaces/interface/statistics/*` |
| `ietf-ip` | IP configuration and statistics | `/ip/ipv4/`, `/ip/ipv6/` |
| `ietf-routing` | Routing table information | `/routing/routing-instance/` |
| `ietf-yang-library` | YANG module information | `/yang-library/module-set/` |

### Cisco Native Modules

| Module | Description | Supported Paths |
|--------|-------------|----------------|
| `Cisco-IOS-XE-interfaces-oper` | Enhanced interface operational data | `/interfaces-ios-xe-oper:interfaces/` |
| `Cisco-IOS-XE-platform-oper` | Platform and hardware information | `/platform-ios-xe-oper:components/` |
| `Cisco-IOS-XE-memory-oper` | Memory utilization statistics | `/memory-ios-xe-oper:memory-statistics/` |
| `Cisco-IOS-XE-process-cpu-oper` | CPU utilization per process | `/process-cpu-ios-xe-oper:cpu-usage/` |
| `Cisco-IOS-XE-bgp-oper` | BGP operational data | `/bgp-ios-xe-oper:bgp-state/` |
| `Cisco-IOS-XE-ospf-oper` | OSPF routing protocol data | `/ospf-ios-xe-oper:ospf-oper-data/` |

### OpenConfig Modules

| Module | Description | Supported Paths |
|--------|-------------|----------------|
| `openconfig-interfaces` | Standard interface model | `/interfaces/interface/` |
| `openconfig-network-instance` | Network instance configuration | `/network-instances/network-instance/` |
| `openconfig-bgp` | BGP protocol configuration | `/network-instances/network-instance/protocols/protocol/bgp/` |
| `openconfig-system` | System configuration and state | `/system/` |

## YANG Parser Configuration

### Basic Configuration

```yaml
receivers:
  cisco_telemetry:
    yang:
      enabled: true                    # Enable YANG parsing
      enable_rfc_parser: true         # Use RFC-compliant parser
      cache_modules: true             # Enable module caching
      max_modules: 1000              # Maximum cached modules
      parser_timeout: 30s            # Timeout for parsing operations
```

### Advanced Configuration

```yaml
receivers:
  cisco_telemetry:
    yang:
      enabled: true
      enable_rfc_parser: true
      
      # Module discovery settings
      discovery:
        auto_discover: true           # Automatically discover modules
        module_paths:                 # Additional module search paths
          - "/usr/share/yang/modules"
          - "/opt/cisco/yang"
        
      # Caching configuration
      cache:
        enabled: true
        max_modules: 1000
        ttl: 24h                     # Cache TTL
        persistence: true            # Persist cache to disk
        cache_dir: "/var/cache/yang"
        
      # Parser behavior
      parser:
        strict_mode: false           # Allow non-compliant modules
        ignore_errors: true          # Continue on parse errors
        max_depth: 10               # Maximum parsing depth
        timeout: 30s
        
      # Type mapping customization
      type_mapping:
        enable_custom_types: true    # Allow custom type definitions
        default_numeric_type: "gauge" # Default for unknown numeric types
        preserve_strings: true       # Keep string types as labels
```

### Debugging Configuration

```yaml
receivers:
  cisco_telemetry:
    yang:
      debug:
        enabled: true                # Enable debug logging
        log_parsed_modules: true     # Log successful module parsing
        log_failed_parsing: true     # Log parsing failures
        dump_raw_yang: false        # Dump raw YANG content (verbose)
        trace_type_inference: true   # Trace type mapping decisions
```

## Data Type Mapping

The YANG parser automatically maps YANG data types to appropriate OpenTelemetry metric types:

### Numeric Types

| YANG Type | OpenTelemetry Type | Example Value | Notes |
|-----------|-------------------|---------------|-------|
| `uint8`, `uint16`, `uint32`, `uint64` | Gauge | `42` | Unsigned integers |
| `int8`, `int16`, `int32`, `int64` | Gauge | `-42` | Signed integers |
| `decimal64` | Gauge | `3.14159` | Fixed-point decimal |
| `counter32`, `counter64` | Counter | `12345678` | Monotonic counters |

### String and Enumeration Types

| YANG Type | OpenTelemetry Type | Example Value | Notes |
|-----------|-------------------|---------------|-------|
| `string` | Label | `"GigabitEthernet0/0/1"` | Added as metric label |
| `enumeration` | Label | `"up"` | Enum values as strings |
| `identityref` | Label | `"ethernet"` | Identity references |
| `leafref` | Label | Reference to other leaf value | Follow references |

### Complex Types

| YANG Type | Handling | Example | Notes |
|-----------|----------|---------|-------|
| `container` | Nested structure | `/interfaces/interface/` | Creates metric hierarchy |
| `list` | Multiple instances | `/interface[name="eth0"]/` | Keyed by list keys |
| `leaf-list` | Multiple values | `["vlan100", "vlan200"]` | Multiple labels/metrics |
| `choice`/`case` | Conditional structure | Varies by case | Handles alternatives |

### Boolean and Binary Types

| YANG Type | OpenTelemetry Type | Example Value | Notes |
|-----------|-------------------|---------------|-------|
| `boolean` | Gauge | `1` (true), `0` (false) | Boolean as numeric |
| `binary` | Label | Base64 encoded | Binary data as string |
| `empty` | Gauge | `1` (present), `0` (absent) | Presence indicator |

## Common YANG Paths

### Interface Statistics

```yang
# Standard IETF interface stats
/ietf-interfaces:interfaces/interface[name="GigabitEthernet0/0/1"]/statistics/
  ├── in-octets          → counter (bytes received)
  ├── in-unicast-pkts    → counter (unicast packets in)
  ├── in-broadcast-pkts  → counter (broadcast packets in)
  ├── in-multicast-pkts  → counter (multicast packets in)
  ├── in-discards        → counter (input discards)
  ├── in-errors          → counter (input errors)
  ├── out-octets         → counter (bytes transmitted)
  ├── out-unicast-pkts   → counter (unicast packets out)
  ├── out-broadcast-pkts → counter (broadcast packets out)
  ├── out-multicast-pkts → counter (multicast packets out)
  ├── out-discards       → counter (output discards)
  └── out-errors         → counter (output errors)

# Cisco enhanced interface stats
/Cisco-IOS-XE-interfaces-oper:interfaces/interface[name="GigabitEthernet0/0/1"]/
  ├── admin-status       → label (up/down/testing)
  ├── oper-status        → label (up/down/unknown/dormant/not-present/lower-layer-down)
  ├── last-change        → gauge (timestamp)
  ├── if-index           → gauge (interface index)
  ├── phys-address       → label (MAC address)
  ├── speed              → gauge (interface speed in bps)
  └── statistics/
      ├── discontinuity-time     → gauge
      ├── in-octets-64           → counter
      ├── in-unicast-pkts-64     → counter
      ├── out-octets-64          → counter
      └── out-unicast-pkts-64    → counter
```

### System Resources

```yang
# CPU utilization
/Cisco-IOS-XE-process-cpu-oper:cpu-usage/cpu-utilization/
  ├── cpu-usage-processes/
  │   └── cpu-usage-process[pid]/
  │       ├── pid                → gauge
  │       ├── name               → label
  │       ├── tty                → gauge
  │       ├── total-run-time     → counter (milliseconds)
  │       └── invoked-count      → counter
  ├── five-seconds              → gauge (5-second average CPU %)
  ├── one-minute                → gauge (1-minute average CPU %)
  └── five-minutes              → gauge (5-minute average CPU %)

# Memory statistics
/Cisco-IOS-XE-memory-oper:memory-statistics/memory-statistic[name]/
  ├── name                      → label (memory pool name)
  ├── total-memory             → gauge (total memory in bytes)
  ├── used-memory              → gauge (used memory in bytes)
  ├── free-memory              → gauge (free memory in bytes)
  ├── available-memory         → gauge (available memory in bytes)
  └── platform-memory          → container
```

### BGP Operational Data

```yang
# BGP neighbor information
/Cisco-IOS-XE-bgp-oper:bgp-state/neighbors/neighbor[afi-safi][vrf][neighbor-id]/
  ├── afi-safi                  → label (address family)
  ├── vrf                       → label (VRF name)
  ├── neighbor-id               → label (neighbor IP)
  ├── description               → label
  ├── bgp-version               → gauge
  ├── router-id                 → label
  ├── negotiated-keepalive-time → gauge (seconds)
  ├── negotiated-holdtime       → gauge (seconds)
  ├── connection/
  │   ├── state                 → label (Idle/Connect/Active/OpenSent/OpenConfirm/Established)
  │   ├── total-established     → counter
  │   └── total-dropped         → counter
  ├── transport/
  │   ├── local-port           → gauge
  │   ├── remote-port          → gauge
  │   └── mss                  → gauge
  └── prefix-activity/
      ├── received/
      │   ├── current-prefixes → gauge
      │   └── total-prefixes   → counter
      └── sent/
          ├── current-prefixes → gauge
          └── total-prefixes   → counter
```

### Platform Information

```yang
# Platform components
/Cisco-IOS-XE-platform-oper:components/component[name]/
  ├── name                      → label (component name)
  ├── config/
  │   └── name                  → label
  └── state/
      ├── type                  → label (component type)
      ├── id                    → label (component ID)
      ├── location              → label (physical location)
      ├── description           → label
      ├── parent                → label (parent component)
      ├── part-no               → label (part number)
      ├── serial-no             → label (serial number)
      ├── software-version      → label
      ├── firmware-version      → label
      ├── temperature/
      │   ├── instant           → gauge (current temperature in Celsius)
      │   ├── avg               → gauge (average temperature)
      │   ├── min               → gauge (minimum temperature)
      │   ├── max               → gauge (maximum temperature)
      │   └── alarm-status      → label (ok/minor/major/critical)
      └── memory/
          ├── total             → gauge (total memory in bytes)
          ├── available         → gauge (available memory in bytes)
          └── utilized          → gauge (utilized memory in bytes)
```

## Troubleshooting YANG Issues

### Common Problems and Solutions

#### 1. Module Not Found

**Problem**: Parser cannot find YANG module referenced in telemetry data.

**Symptoms**:
```
WARN [yang_parser] Module 'Cisco-IOS-XE-foo-oper' not found
ERROR [yang_parser] Failed to resolve path '/foo-oper:data/item'
```

**Solutions**:

1. **Enable Auto-Discovery**:
```yaml
yang:
  discovery:
    auto_discover: true
```

2. **Add Module Search Paths**:
```yaml
yang:
  discovery:
    module_paths:
      - "/usr/share/yang/cisco"
      - "/opt/yang-modules"
```

3. **Download Missing Modules**:
```bash
# Download from Cisco DevNet
wget https://github.com/YangModels/yang/raw/master/vendor/cisco/xe/17.3.1/Cisco-IOS-XE-foo-oper.yang
sudo cp Cisco-IOS-XE-foo-oper.yang /usr/share/yang/modules/
```

#### 2. Type Inference Failures

**Problem**: Parser cannot determine correct data type for metrics.

**Symptoms**:
```
WARN [yang_parser] Unknown type for path '/interfaces/interface/mtu', defaulting to string
ERROR [type_mapper] Failed to convert value '1500' to numeric type
```

**Solutions**:

1. **Enable RFC Parser**:
```yaml
yang:
  enable_rfc_parser: true
```

2. **Configure Type Mapping**:
```yaml
yang:
  type_mapping:
    default_numeric_type: "gauge"
    preserve_strings: false
```

3. **Add Custom Type Rules**:
```yaml
yang:
  type_mapping:
    custom_rules:
      - path_pattern: "*/mtu"
        target_type: "gauge"
      - path_pattern: "*/admin-status"
        target_type: "label"
```

#### 3. Performance Issues

**Problem**: YANG parsing is slow or consuming too much memory.

**Symptoms**:
- High CPU usage during parsing
- Memory growth over time
- Slow telemetry processing

**Solutions**:

1. **Optimize Caching**:
```yaml
yang:
  cache:
    max_modules: 500        # Reduce cache size
    ttl: 1h                # Shorter TTL
```

2. **Disable Complex Features**:
```yaml
yang:
  enable_rfc_parser: false  # Use simpler parser
  parser:
    max_depth: 5           # Limit parsing depth
```

3. **Filter Modules**:
```yaml
yang:
  discovery:
    module_filter:
      include:
        - "ietf-interfaces"
        - "Cisco-IOS-XE-interfaces-oper"
      exclude:
        - "*-deviation"
        - "*-augments"
```

### Debug Techniques

#### Enable Debug Logging

```yaml
yang:
  debug:
    enabled: true
    log_parsed_modules: true
    trace_type_inference: true
```

#### Inspect Raw YANG Data

```bash
# Capture raw telemetry messages
tcpdump -i any -w telemetry.pcap port 57500

# Extract YANG content
wireshark telemetry.pcap
# Filter: grpc and protobuf
```

#### Validate YANG Modules

```bash
# Use pyang to validate YANG files
pip install pyang
pyang --strict Cisco-IOS-XE-interfaces-oper.yang

# Check module dependencies
pyang --print-yang-structure Cisco-IOS-XE-interfaces-oper.yang
```

## Custom YANG Module Support

### Adding Custom Modules

1. **Place YANG Files**:
```bash
sudo mkdir -p /usr/share/yang/custom
sudo cp my-custom-module.yang /usr/share/yang/custom/
```

2. **Configure Search Path**:
```yaml
yang:
  discovery:
    module_paths:
      - "/usr/share/yang/custom"
```

3. **Register Module**:
```yaml
yang:
  modules:
    custom:
      - name: "my-custom-module"
        revision: "2024-01-15"
        namespace: "http://example.com/yang/my-module"
```

### Custom Type Mappings

```yaml
yang:
  type_mapping:
    custom_rules:
      # Map custom types to OpenTelemetry types
      - path_pattern: "*/custom-counter-*"
        target_type: "counter"
        description: "Custom counter metrics"
        
      - path_pattern: "*/temperature-*"
        target_type: "gauge"
        unit: "celsius"
        description: "Temperature readings"
        
      - path_pattern: "*/status-*"
        target_type: "label"
        description: "Status enumeration values"
```

### Module Development Guidelines

When developing custom YANG modules for telemetry:

1. **Use Standard Types**: Prefer standard YANG types over custom types
2. **Clear Naming**: Use descriptive names for leaves and containers  
3. **Proper Units**: Include units in descriptions or use YANG units statement
4. **Consistent Structure**: Follow established patterns from standard modules
5. **Documentation**: Include comprehensive descriptions and examples

Example custom module:
```yang
module example-telemetry {
  namespace "http://example.com/yang/telemetry";
  prefix "ex-tel";
  
  revision "2024-01-15" {
    description "Example telemetry module";
  }
  
  container system-metrics {
    description "System performance metrics";
    
    leaf cpu-utilization {
      type uint8 {
        range "0..100";
      }
      units "percent";
      description "Overall CPU utilization percentage";
    }
    
    leaf memory-used {
      type uint64;
      units "bytes";
      description "Total memory usage in bytes";
    }
    
    leaf-list active-processes {
      type string;
      description "List of active process names";
    }
  }
}
```

## YANG Tools and Utilities

### Recommended Tools

1. **pyang**: YANG module validator and converter
```bash
pip install pyang
pyang --version
```

2. **yanglint**: libyang-based YANG validator
```bash
# Ubuntu/Debian
sudo apt-get install libyang-tools

# CentOS/RHEL
sudo yum install libyang-tools
```

3. **YANG Explorer**: Web-based YANG browser
```bash
git clone https://github.com/CiscoDevNet/yang-explorer.git
cd yang-explorer
python setup.py install
```

### Useful Commands

```bash
# Validate YANG file syntax
pyang --strict module.yang

# Convert YANG to tree format
pyang -f tree module.yang

# Generate documentation
pyang -f html module.yang -o module.html

# Check dependencies
pyang --print-yang-structure module.yang

# Validate instance data
yanglint -s module.yang data.xml

# Convert between formats
pyang -f json module.yang -o module.json
```

### Integration Scripts

Create helper scripts for YANG operations:

```bash
#!/bin/bash
# validate-yang.sh - Validate all YANG modules

YANG_DIR="/usr/share/yang/modules"

echo "Validating YANG modules in $YANG_DIR"

for yang_file in "$YANG_DIR"/*.yang; do
    if [ -f "$yang_file" ]; then
        echo -n "Checking $(basename "$yang_file")... "
        if pyang --strict "$yang_file" >/dev/null 2>&1; then
            echo "OK"
        else
            echo "FAILED"
            pyang --strict "$yang_file" 2>&1 | head -5
        fi
    fi
done
```

```bash
#!/bin/bash
# extract-yang-paths.sh - Extract all paths from YANG module

if [ $# -ne 1 ]; then
    echo "Usage: $0 <yang-file>"
    exit 1
fi

YANG_FILE="$1"

echo "Extracting paths from $YANG_FILE"
pyang -f tree "$YANG_FILE" | grep -E '^\s*[+x-]' | sed 's/^[[:space:]]*//' | sed 's/^[+x-]//'
```

This comprehensive guide should help users understand and work effectively with YANG modules in the Cisco Telemetry Receiver. The combination of automatic discovery, flexible configuration, and debugging tools provides a robust foundation for handling diverse YANG-based telemetry data.