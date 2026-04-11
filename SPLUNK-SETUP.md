# Splunk Enterprise Setup Reference

Everything needed to run Splunk Enterprise with the Cisco MDT telemetry pipeline.

## Docker Container

| Setting | Value |
|---------|-------|
| Image | `splunk/splunk:latest` |
| Container name | `splunk` |
| Start command | `./start-splunk.sh` |

### Ports

| Port | Protocol | Service |
|------|----------|---------|
| 8000 | HTTP | Splunk Web UI |
| 8088 | HTTPS | HTTP Event Collector (HEC) |
| 8089 | HTTPS | Splunk Management / REST API |

### Docker Run Command

```bash
docker run -d --name splunk \
  -p 8000:8000 \
  -p 8088:8088 \
  -p 8089:8089 \
  -e SPLUNK_GENERAL_TERMS="--accept-sgt-current-at-splunk-com" \
  -e SPLUNK_START_ARGS="--accept-license" \
  -e SPLUNK_PASSWORD="Cisco123" \
  -e SPLUNK_HEC_TOKEN="cisco-mdt-token" \
  splunk/splunk:latest
```

## Login Credentials

| Interface | URL | Username | Password |
|-----------|-----|----------|----------|
| Web UI | http://\<host\>:8000 | admin | Cisco123 |
| REST API | https://\<host\>:8089 | admin | Cisco123 |

## HTTP Event Collector (HEC)

| Setting | Value |
|---------|-------|
| Endpoint | `https://<host>:8088/services/collector` |
| Token | `cisco-mdt-token` |
| TLS | Self-signed cert (use `insecure_skip_verify: true`) |
| Source | `cisco:mdt` |
| Sourcetype | `cisco:mdt:grpc` |
| Index | `cisco_mdt` |

### Test HEC connectivity

```bash
curl -sk https://localhost:8088/services/collector/health
```

### Send a test event

```bash
curl -sk https://localhost:8088/services/collector/event \
  -H "Authorization: Splunk cisco-mdt-token" \
  -d '{"event": "test", "index": "cisco_mdt"}'
```

## REST API Examples

### Health check

```bash
curl -sk https://localhost:8089/services/server/health
```

### Create the cisco_mdt metrics index

```bash
curl -sk -u admin:Cisco123 \
  https://localhost:8089/services/data/indexes \
  -d name=cisco_mdt \
  -d datatype=metric
```

### List all indexes

```bash
curl -sk -u admin:Cisco123 \
  'https://localhost:8089/services/data/indexes?output_mode=json' \
  | python3 -m json.tool
```

### Count metric names in cisco_mdt

```bash
curl -sk -u admin:Cisco123 \
  'https://localhost:8089/services/search/jobs' \
  -d 'search=| mcatalog values(metric_name) WHERE index=cisco_mdt | stats count' \
  -d output_mode=json \
  -d exec_mode=oneshot
```

### Run an mstats query

```bash
curl -sk -u admin:Cisco123 \
  'https://localhost:8089/services/search/jobs' \
  -d 'search=| mstats latest("cisco.content.five-seconds") WHERE index=cisco_mdt BY cisco.node_id' \
  -d output_mode=json \
  -d exec_mode=oneshot
```

### Import the dashboard

```bash
curl -sk -u admin:Cisco123 \
  'https://localhost:8089/servicesNS/admin/search/data/ui/views/cisco_mdt_overview' \
  -X POST \
  --data-urlencode "eai:data@splunk-dashboards/cisco_mdt_overview.xml"
```

### Export a dashboard

```bash
curl -sk -u admin:Cisco123 \
  'https://localhost:8089/servicesNS/admin/search/data/ui/views/cisco_mdt_overview' \
  -d output_mode=json
```

## OTEL Collector → Splunk HEC Config

This is the exporter block in `collector-config.yaml`:

```yaml
exporters:
  splunk_hec:
    endpoint: "https://127.0.0.1:8088/services/collector"
    token: "cisco-mdt-token"
    source: "cisco:mdt"
    sourcetype: "cisco:mdt:grpc"
    index: "cisco_mdt"
    tls:
      insecure_skip_verify: true
```

## Useful SPL Queries

### List all metric names

```
| mcatalog values(metric_name) WHERE index=cisco_mdt
```

### List all switches reporting

```
| mstats count("cisco.content.five-seconds") WHERE index=cisco_mdt BY cisco.node_id
```

### CPU utilization by switch

```
| mstats avg("cisco.content.five-seconds") WHERE index=cisco_mdt BY cisco.node_id span=5m
```

### List all encoding paths (YANG models sending data)

```
| mcatalog values(cisco.encoding_path) WHERE index=cisco_mdt
```

### Interface throughput

```
| mstats latest("cisco.content.statistics.rx-kbps") as rx latest("cisco.content.statistics.tx-kbps") as tx WHERE index=cisco_mdt BY cisco.node_id name span=5m
```

### System memory (DRAM)

```
| mstats latest("cisco.content.memory-stats.used-number") as used latest("cisco.content.memory-stats.total") as total WHERE index=cisco_mdt BY cisco.node_id
| eval used_gb=round(used/1048576, 2), total_gb=round(total/1048576, 2)
```

## Container Management

```bash
# Stop Splunk
docker stop splunk

# Start Splunk
docker start splunk

# Restart Splunk
docker restart splunk

# View logs
docker logs splunk --tail 50

# Shell into container
docker exec -it splunk bash

# Remove and recreate (data lost)
docker rm -f splunk
./start-splunk.sh
```

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Web UI not loading | Wait 2-3 min after container start; check `docker logs splunk` |
| HEC returns 403 | Verify token `cisco-mdt-token`; check HEC is enabled |
| No data in cisco_mdt | Verify index exists; check collector logs at `/tmp/otel.log` |
| "Login failed" | Password is `Cisco123` (case-sensitive) |
| Dashboard blank | Wait one telemetry cycle (5 min); verify switch subscriptions |
