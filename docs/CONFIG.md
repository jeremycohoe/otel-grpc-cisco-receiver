# Configuration Reference

This document provides comprehensive configuration options for the Cisco Telemetry Receiver.

## Table of Contents

- [Basic Configuration](#basic-configuration)
- [Network Settings](#network-settings)
- [TLS/Security Configuration](#tlssecurity-configuration)
- [Performance Tuning](#performance-tuning)
- [YANG Parser Settings](#yang-parser-settings)
- [Legacy Configuration](#legacy-configuration)
- [Configuration Examples](#configuration-examples)

## Basic Configuration

### Minimal Configuration

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
```

### Production Configuration

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
    
    # TLS/mTLS Configuration
    tls:
      enabled: true
      cert_file: "/etc/otel/certs/server.crt"
      key_file: "/etc/otel/certs/server.key"
      ca_file: "/etc/otel/certs/ca.crt"
      client_auth_type: "RequireAndVerifyClientCert"
      min_version: "1.2"
      max_version: "1.3"
      reload_interval: 5m
    
    # Security & Rate Limiting
    security:
      rate_limiting:
        enabled: true
        requests_per_second: 100.0
        burst_size: 10
        cleanup_interval: 1m
      allowed_clients: 
        - "10.0.0.0/8"
        - "192.168.0.0/16"
      max_connections: 1000
      connection_timeout: 30s
      enable_metrics: true
    
    # Performance Settings
    max_message_size: 4194304  # 4MB
    max_concurrent_streams: 100
    
    # Keep-Alive Configuration
    keep_alive:
      time: 30s
      timeout: 10s
    
    # YANG Parser Configuration
    yang:
      enable_rfc_parser: true
      cache_modules: true
      max_modules: 1000
```

## Network Settings

### `listen_address`
- **Type**: `string`
- **Default**: `"0.0.0.0:57500"`
- **Description**: IP address and port to bind the gRPC server
- **Examples**:
  ```yaml
  listen_address: "0.0.0.0:57500"        # Listen on all interfaces
  listen_address: "127.0.0.1:57500"      # Localhost only
  listen_address: "10.1.1.100:9999"      # Specific IP and port
  ```

### `max_message_size`
- **Type**: `int`
- **Default**: `4194304` (4MB)
- **Description**: Maximum size of gRPC messages in bytes
- **Range**: `1024` to `268435456` (256MB)

### `max_concurrent_streams`
- **Type**: `uint32`
- **Default**: `100`
- **Description**: Maximum number of concurrent gRPC streams
- **Range**: `1` to `1000000`

## TLS/Security Configuration

### TLS Configuration (`tls`)

#### `enabled`
- **Type**: `bool`
- **Default**: `false`
- **Description**: Enable TLS/mTLS for gRPC connections

#### `cert_file`
- **Type**: `string`
- **Required**: When TLS is enabled
- **Description**: Path to server certificate file (PEM format)

#### `key_file`
- **Type**: `string`
- **Required**: When TLS is enabled
- **Description**: Path to server private key file (PEM format)

#### `ca_file`
- **Type**: `string`
- **Required**: For client certificate validation
- **Description**: Path to Certificate Authority file for client verification

#### `client_auth_type`
- **Type**: `string`
- **Default**: `"NoClientCert"`
- **Values**:
  - `"NoClientCert"`: No client certificate required
  - `"RequestClientCert"`: Request client cert but don't require
  - `"RequireAnyClientCert"`: Require client cert (don't verify)
  - `"VerifyClientCertIfGiven"`: Verify client cert if provided
  - `"RequireAndVerifyClientCert"`: Require and verify client cert (mTLS)

#### `min_version` / `max_version`
- **Type**: `string`
- **Default**: `"1.2"` / `"1.3"`
- **Values**: `"1.0"`, `"1.1"`, `"1.2"`, `"1.3"`
- **Description**: Minimum and maximum TLS versions

#### `cipher_suites`
- **Type**: `[]string`
- **Default**: Go defaults
- **Description**: Allowed cipher suites for TLS 1.2 and below
- **Examples**:
  ```yaml
  cipher_suites:
    - "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
    - "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
  ```

#### `curve_preferences`
- **Type**: `[]string`
- **Default**: Go defaults
- **Description**: Elliptic curves for ECDHE handshakes
- **Examples**:
  ```yaml
  curve_preferences:
    - "CurveP256"
    - "CurveP384"
    - "X25519"
  ```

#### `reload_interval`
- **Type**: `duration`
- **Default**: `5m`
- **Description**: How often to check for certificate updates

### Security Configuration (`security`)

#### Rate Limiting (`rate_limiting`)

##### `enabled`
- **Type**: `bool`
- **Default**: `false`
- **Description**: Enable per-client rate limiting

##### `requests_per_second`
- **Type**: `float64`
- **Default**: `100.0`
- **Description**: Maximum requests per second per client

##### `burst_size`
- **Type**: `int`
- **Default**: `10`
- **Description**: Burst allowance for rate limiting

##### `cleanup_interval`
- **Type**: `duration`
- **Default**: `1m`
- **Description**: How often to clean up rate limiter entries

#### Access Control

##### `allowed_clients`
- **Type**: `[]string`
- **Default**: `[]` (allow all)
- **Description**: IP addresses or CIDR blocks allowed to connect
- **Examples**:
  ```yaml
  allowed_clients:
    - "10.1.1.100"           # Specific IP
    - "192.168.0.0/16"       # CIDR block
    - "10.0.0.0/8"           # Large network
  ```

##### `max_connections`
- **Type**: `int`
- **Default**: `1000`
- **Description**: Maximum concurrent connections

##### `connection_timeout`
- **Type**: `duration`
- **Default**: `30s`
- **Description**: Timeout for new connections

##### `enable_metrics`
- **Type**: `bool`
- **Default**: `true`
- **Description**: Enable security-related metrics collection

## Performance Tuning

### Keep-Alive Configuration (`keep_alive`)

#### `time`
- **Type**: `duration`
- **Default**: `30s`
- **Description**: Time between keep-alive pings

#### `timeout`
- **Type**: `duration`
- **Default**: `10s`
- **Description**: Timeout waiting for keep-alive response

### Resource Limits

```yaml
# High-throughput configuration
max_message_size: 8388608        # 8MB
max_concurrent_streams: 500      # More concurrent streams
keep_alive:
  time: 15s                      # More frequent pings
  timeout: 5s                    # Faster timeout detection

# Conservative configuration  
max_message_size: 1048576        # 1MB
max_concurrent_streams: 50       # Fewer streams
keep_alive:
  time: 60s                      # Less frequent pings
  timeout: 20s                   # More patience
```

## YANG Parser Settings

### YANG Configuration (`yang`)

#### `enable_rfc_parser`
- **Type**: `bool`
- **Default**: `true`
- **Description**: Enable RFC 6020/7950 compliant YANG parser

#### `cache_modules`
- **Type**: `bool`
- **Default**: `true`
- **Description**: Cache discovered YANG modules

#### `max_modules`
- **Type**: `int`
- **Default**: `1000`
- **Description**: Maximum number of YANG modules to cache

## Legacy Configuration

For backward compatibility, the receiver supports legacy configuration format:

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
    
    # Legacy TLS settings (automatically migrated)
    tls_enabled: true
    tls_cert_file: "/path/to/cert.pem"
    tls_key_file: "/path/to/key.pem"
    tls_client_ca_file: "/path/to/ca.pem"
    
    # Legacy keep-alive setting
    keep_alive_timeout: 60s
```

**Note**: Legacy settings are automatically migrated to the new format at startup.

## Configuration Examples

### Development Environment

```yaml
receivers:
  cisco_telemetry:
    listen_address: "127.0.0.1:57500"
    tls:
      enabled: false  # Disable TLS for development
    yang:
      enable_rfc_parser: true
      cache_modules: false  # Don't cache in development
```

### High-Security Production

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
    
    tls:
      enabled: true
      cert_file: "/etc/ssl/certs/telemetry-server.crt"
      key_file: "/etc/ssl/private/telemetry-server.key"
      ca_file: "/etc/ssl/certs/cisco-ca.crt"
      client_auth_type: "RequireAndVerifyClientCert"
      min_version: "1.3"
      max_version: "1.3"  # TLS 1.3 only
    
    security:
      rate_limiting:
        enabled: true
        requests_per_second: 50.0  # Conservative rate
        burst_size: 5
      allowed_clients:
        - "10.100.0.0/16"  # Management network only
      max_connections: 100
    
    yang:
      enable_rfc_parser: true
      cache_modules: true
```

### High-Throughput Environment

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
    
    max_message_size: 16777216      # 16MB messages
    max_concurrent_streams: 1000    # Many concurrent streams
    
    keep_alive:
      time: 10s      # Frequent keep-alives
      timeout: 3s    # Fast failure detection
    
    security:
      rate_limiting:
        enabled: true
        requests_per_second: 1000.0  # High rate limit
        burst_size: 100
      max_connections: 5000          # Many connections
    
    yang:
      enable_rfc_parser: true
      cache_modules: true
      max_modules: 5000              # Cache many modules
```

## Validation

The receiver validates all configuration at startup. Common validation errors:

- **Missing required TLS files**: Ensure cert_file and key_file exist when TLS is enabled
- **Invalid TLS versions**: min_version must be ≤ max_version  
- **Invalid rate limiting**: requests_per_second must be > 0
- **Invalid client auth**: ca_file required for mTLS modes

## Environment Variables

Configuration values can be overridden using environment variables:

- `CISCO_TELEMETRY_LISTEN_ADDRESS`: Override listen_address
- `CISCO_TELEMETRY_TLS_CERT`: Override tls.cert_file
- `CISCO_TELEMETRY_TLS_KEY`: Override tls.key_file
- `CISCO_TELEMETRY_TLS_CA`: Override tls.ca_file

Example:
```bash
export CISCO_TELEMETRY_LISTEN_ADDRESS="10.1.1.100:9999"
export CISCO_TELEMETRY_TLS_CERT="/custom/path/cert.pem"
```

## Configuration Hot Reload

The receiver supports hot reload for certain configuration changes:

- **TLS Certificates**: Automatically reloaded based on `reload_interval`
- **Rate Limits**: Updated dynamically without restart
- **Security Settings**: Applied to new connections immediately

**Note**: Network settings (listen_address, ports) require a restart.