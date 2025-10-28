# Troubleshooting Guide

This guide helps diagnose and resolve common issues with the Cisco Telemetry Receiver.

## Table of Contents

- [Quick Diagnostics](#quick-diagnostics)
- [Connection Issues](#connection-issues)
- [Authentication Problems](#authentication-problems)
- [Performance Issues](#performance-issues)
- [Data Processing Issues](#data-processing-issues)
- [Configuration Problems](#configuration-problems)
- [Monitoring & Debugging](#monitoring--debugging)
- [Common Error Messages](#common-error-messages)

## Quick Diagnostics

### Health Check Commands

```bash
# Check if receiver is running
ps aux | grep cisco-telemetry-receiver

# Check listening ports
ss -tulnp | grep :57500

# Test basic connectivity
telnet localhost 57500

# Check recent logs
journalctl -u otel-collector -n 50

# Check metrics endpoint
curl http://localhost:8888/metrics | grep cisco_telemetry
```

### Diagnostic Information Collection

```bash
#!/bin/bash
# collect-diagnostics.sh

echo "=== Cisco Telemetry Receiver Diagnostics ==="
echo "Timestamp: $(date)"
echo

echo "=== Process Status ==="
ps aux | grep -E "(otel|cisco)"

echo "=== Network Status ==="
ss -tulnp | grep :57500

echo "=== System Resources ==="
free -h
df -h
uptime

echo "=== Recent Logs ==="
journalctl -u otel-collector -n 100 --no-pager

echo "=== Configuration ==="
cat /etc/otel-collector/config.yaml 2>/dev/null || echo "Config file not found"

echo "=== Metrics Sample ==="
curl -s http://localhost:8888/metrics | grep cisco_telemetry | head -20
```

## Connection Issues

### Problem: Cannot Connect to Receiver

#### Symptoms
- Cisco devices report connection failures
- "Connection refused" errors
- Telnet to port fails

#### Diagnosis
```bash
# Check if service is running
systemctl status otel-collector

# Verify listening address
ss -tulnp | grep :57500

# Check firewall rules
sudo ufw status
sudo iptables -L | grep 57500

# Test local connectivity
nc -zv localhost 57500
```

#### Solutions

1. **Service Not Running**
```bash
# Start the service
sudo systemctl start otel-collector
sudo systemctl enable otel-collector

# Check for startup errors
journalctl -u otel-collector -f
```

2. **Wrong Listening Address**
```yaml
# Fix configuration
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"  # Not 127.0.0.1
```

3. **Firewall Blocking**
```bash
# Allow port through firewall
sudo ufw allow 57500
# Or specific source
sudo ufw allow from 10.0.0.0/8 to any port 57500
```

4. **Port Already in Use**
```bash
# Find process using port
sudo lsof -i :57500
sudo kill <PID>
```

### Problem: Intermittent Connection Drops

#### Symptoms
- Connections work initially but drop periodically
- "Connection reset by peer" errors
- Cisco devices show reconnection attempts

#### Diagnosis
```bash
# Monitor connections in real-time
watch "ss -t | grep :57500"

# Check connection metrics
curl http://localhost:8888/metrics | grep active_connections

# Monitor logs for patterns
journalctl -u otel-collector -f | grep -i "disconnect\|drop\|reset"
```

#### Solutions

1. **Keep-Alive Issues**
```yaml
receivers:
  cisco_telemetry:
    keep_alive:
      time: 30s      # Increase keep-alive frequency
      timeout: 10s   # Reasonable timeout
```

2. **Network MTU Issues**
```bash
# Check MTU sizes
ip link show

# Test with larger messages
ping -M do -s 1472 <cisco-device-ip>
```

3. **Resource Exhaustion**
```yaml
security:
  max_connections: 1000        # Increase if needed
  connection_timeout: 60s      # Longer timeout
```

### Problem: High Connection Latency

#### Symptoms
- Slow connection establishment
- High initial handshake times
- Delayed first messages

#### Diagnosis
```bash
# Time connection establishment
time telnet <receiver-ip> 57500

# Check DNS resolution times
time nslookup <receiver-hostname>

# Monitor handshake metrics
curl http://localhost:8888/metrics | grep handshake
```

#### Solutions

1. **DNS Issues**
```bash
# Use IP addresses instead of hostnames
# Configure proper DNS resolution
echo "10.1.1.100 telemetry-server" >> /etc/hosts
```

2. **TLS Handshake Optimization**
```yaml
tls:
  enabled: true
  # Use faster curves
  curve_preferences:
    - "X25519"      # Fast modern curve
    - "CurveP256"
```

## Authentication Problems

### Problem: TLS Handshake Failures

#### Symptoms
- "TLS handshake failed" errors
- "Certificate verification failed"
- "Protocol version mismatch"

#### Diagnosis
```bash
# Test TLS connection manually
openssl s_client -connect <receiver>:57500 -cert client.crt -key client.key

# Check certificate validity
openssl x509 -in server.crt -text -noout

# Verify certificate chain
openssl verify -CAfile ca.crt server.crt

# Check supported TLS versions
nmap --script ssl-enum-ciphers -p 57500 <receiver>
```

#### Solutions

1. **Certificate Issues**
```bash
# Regenerate certificates with proper SANs
openssl req -new -x509 -key server.key -out server.crt -days 365 \
    -config server.conf

# Verify certificate includes correct hostnames/IPs
openssl x509 -in server.crt -text | grep -A5 "Subject Alternative Name"
```

2. **TLS Version Mismatch**
```yaml
tls:
  min_version: "1.2"    # Compatible with most Cisco devices
  max_version: "1.3"    # Allow modern clients
```

3. **Clock Skew Issues**
```bash
# Synchronize clocks
sudo ntpdate -s time.nist.gov

# Check certificate validity periods
openssl x509 -in server.crt -dates -noout
```

### Problem: Client Certificate Rejection

#### Symptoms
- "Client certificate required" errors
- "Unknown certificate authority"
- "Certificate verification failed"

#### Diagnosis
```bash
# Check client certificate validity
openssl x509 -in client.crt -text -noout

# Verify client cert is signed by CA
openssl verify -CAfile ca.crt client.crt

# Check certificate purposes
openssl x509 -in client.crt -purpose -noout
```

#### Solutions

1. **Missing Client Certificate**
```cisco
! On Cisco device, install client certificate
crypto pki import client pem terminal
! Paste certificate content
```

2. **CA Trust Issues**
```yaml
tls:
  ca_file: "/etc/ssl/certs/cisco-ca.crt"  # Ensure path is correct
  client_auth_type: "RequireAndVerifyClientCert"
```

3. **Certificate Purpose Issues**
```bash
# Regenerate client cert with proper extensions
openssl req -new -key client.key -out client.csr \
    -config client.conf
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key \
    -out client.crt -extensions v3_client -extfile client.conf
```

## Performance Issues

### Problem: High CPU Usage

#### Symptoms
- CPU usage >80% constantly
- Slow message processing
- High system load

#### Diagnosis
```bash
# Monitor CPU usage by process
top -p $(pgrep cisco-telemetry)

# Profile CPU usage
go tool pprof http://localhost:8888/debug/pprof/profile?seconds=30

# Check message processing rates
curl http://localhost:8888/metrics | grep -E "(received|processed)_total"
```

#### Solutions

1. **Optimize Configuration**
```yaml
receivers:
  cisco_telemetry:
    max_concurrent_streams: 100    # Reduce if too high
    yang:
      enable_rfc_parser: false     # Disable if not needed
    security:
      rate_limiting:
        enabled: false             # Disable if not needed
```

2. **Scale Resources**
```bash
# Increase CPU allocation (containers)
docker update --cpus="2.0" <container-id>

# Set CPU affinity
taskset -c 0-3 $(pgrep cisco-telemetry)
```

### Problem: High Memory Usage

#### Symptoms
- Memory usage growing over time
- Out of memory errors
- Frequent garbage collection

#### Diagnosis
```bash
# Monitor memory usage
ps -o pid,vsz,rss,comm -p $(pgrep cisco-telemetry)

# Memory profiling
go tool pprof http://localhost:8888/debug/pprof/heap

# Check for goroutine leaks
curl http://localhost:8888/debug/pprof/goroutine?debug=1
```

#### Solutions

1. **Reduce Memory Footprint**
```yaml
receivers:
  cisco_telemetry:
    max_message_size: 2097152     # Reduce to 2MB
    max_concurrent_streams: 50    # Lower concurrency
    yang:
      max_modules: 500           # Smaller cache
```

2. **Tune Garbage Collector**
```bash
# Set memory limit
export GOMEMLIMIT=2GB

# Adjust GC frequency
export GOGC=80    # More aggressive GC
```

### Problem: High Processing Latency

#### Symptoms
- p95 latency >100ms
- Messages queuing up
- Slow response times

#### Diagnosis
```bash
# Check latency metrics
curl http://localhost:8888/metrics | grep processing_duration

# Monitor message queue depth
curl http://localhost:8888/metrics | grep messages_received | tail -10
curl http://localhost:8888/metrics | grep messages_processed | tail -10

# Profile application
go tool pprof http://localhost:8888/debug/pprof/profile
```

#### Solutions

1. **Optimize Processing Pipeline**
```yaml
receivers:
  cisco_telemetry:
    # Reduce processing complexity
    yang:
      enable_rfc_parser: false    # Skip YANG processing if not needed
    # Increase parallelism  
    max_concurrent_streams: 200
```

2. **Network Optimization**
```bash
# Increase network buffers
echo 'net.core.rmem_max = 134217728' >> /etc/sysctl.conf
sysctl -p
```

## Data Processing Issues

### Problem: Missing or Incomplete Metrics

#### Symptoms
- Some telemetry data not appearing in metrics
- Partial data in downstream systems
- Data gaps in time series

#### Diagnosis
```bash
# Check message drop metrics
curl http://localhost:8888/metrics | grep messages_dropped

# Monitor processing errors
curl http://localhost:8888/metrics | grep grpc_errors

# Check logs for parsing errors
journalctl -u otel-collector | grep -i "error\|failed\|drop"
```

#### Solutions

1. **Increase Message Size Limits**
```yaml
receivers:
  cisco_telemetry:
    max_message_size: 8388608    # Increase to 8MB if messages are large
```

2. **Fix YANG Parsing Issues**
```yaml
yang:
  enable_rfc_parser: true        # Enable for better parsing
  cache_modules: true            # Cache discovered modules
```

3. **Debug Message Content**
```yaml
# Enable debug logging temporarily
service:
  telemetry:
    logs:
      level: debug
```

### Problem: Incorrect Metric Values

#### Symptoms
- Metric values don't match Cisco CLI output
- Unexpected data types (strings vs numbers)
- Scale factors incorrect

#### Diagnosis
```bash
# Compare with Cisco CLI
# On Cisco device:
show interfaces GigabitEthernet0/0/1 | include packets

# Check raw telemetry data
# Enable debug mode to see raw messages
```

#### Solutions

1. **Verify YANG Model Interpretation**
```cisco
! On Cisco device, verify YANG path
show telemetry ietf subscription 100 detail
```

2. **Check Data Type Mapping**
```yaml
# The receiver should handle type conversion automatically
# If issues persist, check YANG parser configuration
yang:
  enable_rfc_parser: true    # Better type inference
```

## Configuration Problems

### Problem: Configuration Validation Errors

#### Symptoms
- Receiver fails to start
- "Configuration invalid" errors
- Validation error messages in logs

#### Diagnosis
```bash
# Test configuration syntax
/usr/local/bin/cisco-telemetry-receiver --config-check

# Check YAML syntax
yamllint /etc/otel-collector/config.yaml

# Validate against schema
# (if available)
```

#### Solutions

1. **Fix YAML Syntax**
```bash
# Common issues:
# - Incorrect indentation (use spaces, not tabs)
# - Missing colons after keys
# - Incorrect list syntax

# Use online YAML validator or:
python -c "import yaml; yaml.safe_load(open('config.yaml'))"
```

2. **Correct Configuration Values**
```yaml
# Common fixes:
receivers:
  cisco_telemetry:
    listen_address: "0.0.0.0:57500"    # String, not number
    max_concurrent_streams: 100         # Number, not string
    tls:
      enabled: true                     # Boolean, not string
```

### Problem: Legacy Configuration Migration

#### Symptoms
- Deprecated configuration warnings
- Old parameters not working
- Migration errors in logs

#### Solutions

```yaml
# Old format (deprecated)
receivers:
  cisco_telemetry:
    tls_enabled: true
    tls_cert_file: "/path/to/cert"

# New format (preferred)
receivers:
  cisco_telemetry:
    tls:
      enabled: true
      cert_file: "/path/to/cert"
```

## Monitoring & Debugging

### Debug Logging Configuration

```yaml
service:
  telemetry:
    logs:
      level: debug    # Enable debug logging
      development: true
      output_paths: ["/var/log/otel/cisco-telemetry.log"]
```

### Useful Debug Commands

```bash
# Real-time log monitoring
tail -f /var/log/otel/cisco-telemetry.log | grep -E "(ERROR|WARN|DEBUG)"

# Connection monitoring
watch "ss -t | grep :57500 | wc -l"

# Metrics monitoring
watch "curl -s http://localhost:8888/metrics | grep cisco_telemetry_messages"

# Process monitoring
watch "ps -o pid,ppid,%cpu,%mem,cmd -p $(pgrep cisco-telemetry)"
```

### Performance Profiling

```bash
# CPU profiling (30 seconds)
go tool pprof http://localhost:8888/debug/pprof/profile?seconds=30

# Memory profiling
go tool pprof http://localhost:8888/debug/pprof/heap

# Goroutine analysis
go tool pprof http://localhost:8888/debug/pprof/goroutine

# Trace analysis (5 seconds)
wget http://localhost:8888/debug/pprof/trace?seconds=5 -O trace.out
go tool trace trace.out
```

## Common Error Messages

### Connection Errors

| Error Message | Likely Cause | Solution |
|--------------|--------------|----------|
| `connection refused` | Service not running or wrong port | Check service status and configuration |
| `no route to host` | Network/firewall issue | Check network connectivity and firewall rules |
| `connection reset by peer` | Keep-alive or resource limits | Adjust keep-alive settings and resource limits |
| `context deadline exceeded` | Timeout issues | Increase timeout values |

### TLS/Security Errors

| Error Message | Likely Cause | Solution |
|--------------|--------------|----------|
| `certificate verify failed` | Invalid certificate or CA | Check certificate validity and CA configuration |
| `protocol version not supported` | TLS version mismatch | Adjust TLS version settings |
| `handshake failure` | Certificate or cipher issues | Check certificate configuration and cipher suites |
| `client certificate required` | mTLS misconfiguration | Configure client certificates properly |

### Processing Errors

| Error Message | Likely Cause | Solution |
|--------------|--------------|----------|
| `message too large` | Message size exceeds limit | Increase max_message_size |
| `too many connections` | Connection limit exceeded | Increase max_connections or add load balancing |
| `rate limit exceeded` | Rate limiting active | Adjust rate limiting settings or investigate traffic |
| `YANG parsing failed` | Invalid YANG data or configuration | Check YANG parser configuration and data format |

### Configuration Errors

| Error Message | Likely Cause | Solution |
|--------------|--------------|----------|
| `invalid listen address` | Malformed address format | Use correct "host:port" format |
| `file not found` | Missing certificate files | Check file paths and permissions |
| `permission denied` | Insufficient file permissions | Fix file ownership and permissions |
| `invalid configuration` | YAML syntax or validation error | Check YAML syntax and configuration values |

## Emergency Procedures

### Quick Recovery Steps

1. **Service Recovery**
```bash
# Stop service
sudo systemctl stop otel-collector

# Check for hung processes
sudo pkill -f cisco-telemetry

# Clear any locked files
sudo rm -f /var/lock/otel-collector.lock

# Start service
sudo systemctl start otel-collector
```

2. **Configuration Rollback**
```bash
# Backup current config
sudo cp /etc/otel-collector/config.yaml /etc/otel-collector/config.yaml.backup

# Restore previous working config
sudo cp /etc/otel-collector/config.yaml.previous /etc/otel-collector/config.yaml

# Restart service
sudo systemctl restart otel-collector
```

3. **Network Reset**
```bash
# Reset network connections
sudo systemctl restart networking

# Flush iptables (if safe)
sudo iptables -F

# Restart firewall
sudo ufw --force reset
sudo ufw enable
```

### Escalation Checklist

When escalating issues, collect:

- [ ] Full configuration file
- [ ] Recent logs (last 1000 lines)
- [ ] System resource usage (CPU, memory, disk)
- [ ] Network configuration and status
- [ ] Current metrics output
- [ ] Steps to reproduce the issue
- [ ] Expected vs actual behavior
- [ ] Timeline of when issue started
- [ ] Any recent changes to system or configuration

### Support Information

For additional support:

1. **Documentation**: Check [CONFIG.md](CONFIG.md), [SECURITY.md](SECURITY.md), [PERFORMANCE.md](PERFORMANCE.md)
2. **Issues**: Open GitHub issue with diagnostic information
3. **Community**: Join OpenTelemetry Slack community
4. **Professional Support**: Contact your organization's infrastructure team

Remember to sanitize any sensitive information (credentials, internal IPs, etc.) before sharing diagnostic information externally.