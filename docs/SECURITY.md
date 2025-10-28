# Security Configuration Guide

This document provides comprehensive security configuration and best practices for the Cisco Telemetry Receiver in production environments.

## Table of Contents

- [Security Overview](#security-overview)
- [TLS/mTLS Configuration](#tlsmtls-configuration)
- [Certificate Management](#certificate-management)
- [Access Control](#access-control)
- [Rate Limiting](#rate-limiting)
- [Security Monitoring](#security-monitoring)
- [Security Hardening](#security-hardening)
- [Troubleshooting](#troubleshooting)

## Security Overview

The Cisco Telemetry Receiver implements multiple layers of security:

```
┌─────────────────────────────────────────────────────────┐
│                    Security Layers                      │
├─────────────────────────────────────────────────────────┤
│ 1. Network Access Control (IP Allowlisting)            │
│ 2. TLS/mTLS Authentication & Encryption                 │
│ 3. Certificate-based Client Authentication              │
│ 4. Rate Limiting & DoS Protection                      │
│ 5. Resource Limits & Connection Controls               │
│ 6. Security Monitoring & Alerting                      │
└─────────────────────────────────────────────────────────┘
```

### Security Features

- **TLS 1.2/1.3**: Modern encryption standards
- **mTLS**: Mutual certificate authentication
- **Rate Limiting**: Per-client DoS protection
- **IP Allowlisting**: Network-level access control
- **Security Metrics**: Built-in monitoring
- **Certificate Rotation**: Automatic cert reload

## TLS/mTLS Configuration

### Basic TLS (Server Authentication)

```yaml
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"
    tls:
      enabled: true
      cert_file: "/etc/ssl/certs/server.crt"
      key_file: "/etc/ssl/private/server.key"
      min_version: "1.2"
      max_version: "1.3"
```

### Mutual TLS (Client + Server Authentication)

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
      min_version: "1.3"  # TLS 1.3 only for maximum security
```

### Client Authentication Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| `NoClientCert` | No client certificate required | Development, basic TLS |
| `RequestClientCert` | Request cert but don't require | Optional client auth |
| `RequireAnyClientCert` | Require cert, don't verify | Trust any certificate |
| `VerifyClientCertIfGiven` | Verify if provided | Flexible authentication |
| `RequireAndVerifyClientCert` | Full mTLS | Production environments |

### Cipher Suite Configuration

```yaml
tls:
  enabled: true
  # ... certificate config ...
  cipher_suites:
    # TLS 1.2 cipher suites (secure options)
    - "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
    - "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
    - "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"
    - "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
  curve_preferences:
    - "X25519"     # Modern, fast curve
    - "CurveP256"  # NIST P-256
    - "CurveP384"  # NIST P-384
```

## Certificate Management

### Certificate Requirements

#### Server Certificate
- **Format**: X.509 PEM
- **Key Size**: RSA 2048+ or ECDSA P-256+
- **Subject Alternative Names**: Include all server IPs/hostnames
- **Validity**: Recommend 1 year maximum
- **Key Usage**: Digital Signature, Key Encipherment

#### Client Certificate (for Cisco devices)
- **Format**: X.509 PEM  
- **Key Size**: RSA 2048+ or ECDSA P-256+
- **Extended Key Usage**: Client Authentication
- **Subject**: Unique per device (CN=device-hostname)

#### CA Certificate
- **Purpose**: Sign client certificates
- **Key Size**: RSA 4096+ or ECDSA P-384+
- **Validity**: 5-10 years
- **Basic Constraints**: CA=true

### Certificate Generation Example

#### 1. Create CA Private Key & Certificate

```bash
# Generate CA private key
openssl genpkey -algorithm RSA -out ca-key.pem -pkcs8 -aes256 -pass pass:ca-password

# Generate CA certificate
openssl req -new -x509 -key ca-key.pem -out ca.crt -days 3650 -config ca.conf
```

`ca.conf`:
```ini
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_ca

[req_distinguished_name]
CN = Cisco Telemetry CA

[v3_ca]
basicConstraints = CA:TRUE
keyUsage = keyCertSign, cRLSign
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
```

#### 2. Create Server Certificate

```bash
# Generate server private key
openssl genpkey -algorithm RSA -out server-key.pem -pkcs8

# Generate certificate signing request
openssl req -new -key server-key.pem -out server.csr -config server.conf

# Sign server certificate
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca-key.pem -out server.crt -days 365 -extensions v3_server -extfile server.conf
```

`server.conf`:
```ini
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_server

[req_distinguished_name]
CN = telemetry.example.com

[v3_server]
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = telemetry.example.com
DNS.2 = telemetry-server
IP.1 = 10.1.1.100
IP.2 = 192.168.1.100
```

#### 3. Create Client Certificate (for Cisco device)

```bash
# Generate client private key
openssl genpkey -algorithm RSA -out cisco-client-key.pem -pkcs8

# Generate client CSR
openssl req -new -key cisco-client-key.pem -out cisco-client.csr -subj "/CN=cisco-switch-01"

# Sign client certificate
openssl x509 -req -in cisco-client.csr -CA ca.crt -CAkey ca-key.pem -out cisco-client.crt -days 365 -extensions v3_client -extfile client.conf
```

`client.conf`:
```ini
[v3_client]
basicConstraints = CA:FALSE
keyUsage = digitalSignature
extendedKeyUsage = clientAuth
```

### Certificate Deployment

#### Server-side (OpenTelemetry Receiver)
```bash
# Copy certificates to secure location
sudo cp server.crt /etc/ssl/certs/telemetry-server.crt
sudo cp server-key.pem /etc/ssl/private/telemetry-server.key
sudo cp ca.crt /etc/ssl/certs/cisco-ca.crt

# Set secure permissions
sudo chmod 644 /etc/ssl/certs/telemetry-server.crt
sudo chmod 600 /etc/ssl/private/telemetry-server.key
sudo chmod 644 /etc/ssl/certs/cisco-ca.crt

# Change ownership to otel user
sudo chown otel:otel /etc/ssl/private/telemetry-server.key
```

#### Client-side (Cisco IOS XE)
```cisco
! Copy client certificate to device
crypto pki import client-cert pem terminal
! Paste cisco-client.crt content

! Copy client private key
crypto pki import client-key pem terminal  
! Paste cisco-client-key.pem content

! Copy CA certificate
crypto pki trustpoint TELEMETRY-CA
 enrollment terminal pem
 revocation-check none
crypto pki authenticate TELEMETRY-CA
! Paste ca.crt content
```

### Automatic Certificate Rotation

```yaml
tls:
  enabled: true
  cert_file: "/etc/ssl/certs/server.crt"
  key_file: "/etc/ssl/private/server.key"
  reload_interval: 5m  # Check for updates every 5 minutes
```

The receiver automatically reloads certificates when files change, enabling zero-downtime certificate rotation.

## Access Control

### IP Allowlisting

```yaml
security:
  allowed_clients:
    # Specific devices
    - "10.1.1.50"     # Core router
    - "10.1.1.51"     # Distribution switch
    
    # Network ranges  
    - "10.0.0.0/8"    # Private network
    - "192.168.100.0/24"  # Management network
    
    # Avoid public networks
    # - "0.0.0.0/0"   # NEVER allow all IPs in production!
```

### Connection Limits

```yaml
security:
  max_connections: 100        # Total concurrent connections
  connection_timeout: 30s     # New connection timeout
```

### Best Practices

1. **Principle of Least Privilege**: Only allow required IP ranges
2. **Network Segmentation**: Use dedicated management networks
3. **Monitoring**: Track connection sources and patterns
4. **Regular Audits**: Review and update allowlists regularly

## Rate Limiting

### Per-Client Rate Limiting

```yaml
security:
  rate_limiting:
    enabled: true
    requests_per_second: 100.0  # Max requests per client per second
    burst_size: 10             # Burst allowance
    cleanup_interval: 1m       # Memory cleanup frequency
```

### Rate Limiting Strategy

| Environment | RPS | Burst | Reasoning |
|-------------|-----|-------|-----------|
| Development | 1000+ | 50+ | Permissive for testing |
| Staging | 500 | 20 | Moderate limits |
| Production | 100 | 10 | Conservative, DoS protection |
| High-throughput | 1000 | 100 | Performance-focused |

### DDoS Protection

```yaml
security:
  rate_limiting:
    enabled: true
    requests_per_second: 10.0   # Very conservative
    burst_size: 2              # Small burst
  max_connections: 50          # Limit total connections
  connection_timeout: 10s      # Fast timeout
```

## Security Monitoring

### Built-in Security Metrics

The receiver exposes security-related metrics:

```
# Connection metrics
cisco_telemetry_active_connections{node_id="switch-01"}
cisco_telemetry_grpc_errors_total{error_type="rate_limited"}
cisco_telemetry_grpc_errors_total{error_type="unauthorized"}

# Rate limiting metrics  
cisco_telemetry_messages_dropped_total{reason="rate_limited"}
cisco_telemetry_messages_dropped_total{reason="ip_blocked"}

# TLS metrics
cisco_telemetry_tls_handshake_errors_total
cisco_telemetry_certificate_expiry_seconds
```

### Alerting Rules (Prometheus)

```yaml
groups:
- name: cisco_telemetry_security
  rules:
  
  # High rate of authentication failures
  - alert: CiscoTelemetryAuthFailures
    expr: rate(cisco_telemetry_grpc_errors_total{error_type="unauthorized"}[5m]) > 0.1
    for: 1m
    labels:
      severity: warning
    annotations:
      summary: "High rate of telemetry authentication failures"
      
  # Rate limiting active
  - alert: CiscoTelemetryRateLimited  
    expr: rate(cisco_telemetry_messages_dropped_total{reason="rate_limited"}[5m]) > 0
    for: 30s
    labels:
      severity: info
    annotations:
      summary: "Telemetry clients being rate limited"
      
  # Certificate expiring soon
  - alert: CiscoTelemetryCertExpiring
    expr: cisco_telemetry_certificate_expiry_seconds < 604800  # 7 days
    for: 0s
    labels:
      severity: critical
    annotations:
      summary: "Telemetry server certificate expiring soon"
```

### Log Analysis

Monitor logs for security events:

```bash
# Authentication failures
grep "authentication failed" /var/log/otel/collector.log

# Rate limiting events  
grep "rate limit exceeded" /var/log/otel/collector.log

# TLS errors
grep "tls handshake" /var/log/otel/collector.log
```

## Security Hardening

### Operating System Level

```bash
# Create dedicated user
sudo useradd -r -s /bin/false otel-cisco-telemetry

# Secure file permissions
sudo chown -R otel-cisco-telemetry:otel-cisco-telemetry /opt/otel-collector
sudo chmod 750 /opt/otel-collector
sudo chmod 600 /opt/otel-collector/config.yaml

# Network security
sudo ufw allow from 10.0.0.0/8 to any port 57500
sudo ufw deny 57500  # Deny all other access
```

### Container Security (Docker)

```dockerfile
FROM alpine:latest

# Create non-root user
RUN adduser -D -s /bin/false otel

# Copy application
COPY --chown=otel:otel cisco-telemetry-receiver /usr/local/bin/
COPY --chown=otel:otel config.yaml /etc/otel/

# Security settings
RUN chmod 755 /usr/local/bin/cisco-telemetry-receiver
RUN chmod 600 /etc/otel/config.yaml

# Run as non-root
USER otel
EXPOSE 57500

CMD ["/usr/local/bin/cisco-telemetry-receiver"]
```

### Kubernetes Security

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cisco-telemetry-receiver
spec:
  template:
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      containers:
      - name: receiver
        image: cisco-telemetry-receiver:latest
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop:
            - ALL
        resources:
          limits:
            memory: "512Mi"
            cpu: "500m"
          requests:
            memory: "256Mi"  
            cpu: "100m"
```

### Network Policies

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: cisco-telemetry-policy
spec:
  podSelector:
    matchLabels:
      app: cisco-telemetry-receiver
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: management
    ports:
    - protocol: TCP
      port: 57500
  egress:
  - to: []  # Allow egress to exporters
    ports:
    - protocol: TCP
      port: 4317  # OTEL gRPC
```

## Troubleshooting

### Common Security Issues

#### TLS Handshake Failures

**Symptoms**: Connection refused, handshake errors
**Causes**: 
- Mismatched TLS versions
- Invalid certificates
- Clock skew between client/server

**Solutions**:
```bash
# Check certificate validity
openssl x509 -in server.crt -text -noout

# Test TLS connection
openssl s_client -connect localhost:57500 -cert client.crt -key client.key

# Verify certificate chain
openssl verify -CAfile ca.crt server.crt
```

#### Client Authentication Failures

**Symptoms**: "unauthorized" errors in logs
**Causes**:
- Client certificate not trusted by CA
- Client certificate expired
- Wrong client authentication mode

**Solutions**:
```yaml
# Enable debug logging
logging:
  level: debug

# Temporarily reduce auth requirements
tls:
  client_auth_type: "VerifyClientCertIfGiven"  # Instead of RequireAndVerifyClientCert
```

#### Rate Limiting Issues

**Symptoms**: Intermittent connection failures, "rate limit exceeded"
**Causes**:
- Too aggressive rate limits
- Burst traffic patterns
- Multiple devices from same IP

**Solutions**:
```yaml
# Increase rate limits temporarily
security:
  rate_limiting:
    requests_per_second: 1000.0  # Increase from 100
    burst_size: 50               # Increase from 10
```

### Security Monitoring Commands

```bash
# Check active connections
ss -tulnp | grep :57500

# Monitor certificate expiry
openssl x509 -in /etc/ssl/certs/server.crt -checkend 604800

# Check for failed connections
journalctl -u otel-collector -g "authentication failed"

# Monitor rate limiting
curl http://localhost:8888/metrics | grep cisco_telemetry_messages_dropped
```

### Emergency Procedures

#### Disable Security (Emergency Only)

```yaml
# EMERGENCY: Disable all security temporarily
receivers:
  cisco_telemetry:
    listen_address: "127.0.0.1:57500"  # Localhost only!
    tls:
      enabled: false
    security:
      rate_limiting:
        enabled: false
      allowed_clients: []  # Allow all (dangerous!)
```

#### Certificate Rollback

```bash
# Backup current certificates
sudo cp /etc/ssl/certs/server.crt /etc/ssl/certs/server.crt.backup

# Restore previous certificates  
sudo cp /etc/ssl/certs/server.crt.previous /etc/ssl/certs/server.crt

# Restart receiver (certificates auto-reload in 5 minutes)
sudo systemctl restart otel-collector
```

## Security Checklist

### Pre-Production

- [ ] TLS 1.2+ enabled with strong cipher suites
- [ ] mTLS configured with proper client certificate validation
- [ ] IP allowlisting configured for management networks only
- [ ] Rate limiting enabled with appropriate limits
- [ ] Security metrics monitoring configured
- [ ] Certificate expiry monitoring set up
- [ ] Log aggregation and alerting configured
- [ ] Network segmentation implemented
- [ ] Regular certificate rotation scheduled

### Production Monitoring

- [ ] Security metrics dashboards created
- [ ] Alerting rules for authentication failures
- [ ] Certificate expiry notifications
- [ ] Rate limiting alerts configured
- [ ] Connection monitoring enabled
- [ ] Security log analysis automated
- [ ] Regular security audits scheduled
- [ ] Incident response procedures documented

### Compliance Considerations

- **PCI DSS**: Strong encryption, access controls, monitoring
- **HIPAA**: Encryption in transit, audit logging, access controls  
- **SOX**: Change management, access reviews, monitoring
- **GDPR**: Data encryption, access logs, data minimization

For compliance requirements, ensure:
- All data encrypted in transit (TLS 1.2+)
- Strong authentication mechanisms (mTLS)
- Comprehensive audit logging enabled
- Regular access reviews conducted
- Security monitoring and alerting active