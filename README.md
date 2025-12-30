# Thermia Exporter

A Prometheus exporter for **Thermia Online** heat pumps, written in Go.

It authenticates with the Thermia Online API using OAuth2 and exposes comprehensive heat pump metrics at `:9808/metrics`.

---

## Features

- Metrics coverage: Temperatures, operation modes, statuses, hot water, operational time, alerts
- Native support for mounted Kubernetes secrets, health endpoint, handles SIGTERM/SIGINT for clean and graceful shutdown
- Scrapes data on-demand when Prometheus requests `/metrics`, not on a fixed interval with low-cardinality labels
- Authenticates once and reuses the token until expiry (~1h), minimizing login attempts
- Uses `slog` for JSON/text logging with contextual fields
- Continues with partial data if some API calls fail
- Reuses HTTP connections for better performance

### Metrics Exported

- **12 temperature sensors** (indoor, outdoor, supply/return lines, hot water, brine, buffer tank, pool, cooling)
- **Online status** with last-seen timestamp
- **Operation modes** (current and available)
- **Operational statuses** (heat, cool, hot water, standby, etc.)
- **Power statuses** (compressor, aux heaters)
- **Hot water controls** (switch state, boost mode)
- **Operational time counters** (hours for compressor, heating, hot water, aux heaters)
- **Alert counts** (active and archived)
- **Scrape metrics** (errors, duration)

---

## Quick Start

### Local Development

```bash
git clone https://github.com/grimne/thermia_exporter.git
cd thermia_exporter

# Build
go build -o thermia-exporter ./cmd/thermia-exporter

# Run with environment variables
export THERMIA_USERNAME="you@example.com"
export THERMIA_PASSWORD="your_password"
./thermia-exporter
```

### Docker
Container repo: https://github.com/grimne/thermia_exporter/pkgs/container/thermia_exporter

```bash
docker build -t thermia-exporter .
docker run -p 9808:9808 \
  -e THERMIA_USERNAME="you@example.com" \
  -e THERMIA_PASSWORD="your_password" \
  thermia-exporter
```

### Kubernetes

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: thermia-credentials
type: Opaque
stringData:
  username: "you@example.com"
  password: "your_password"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: thermia-exporter
spec:
  replicas: 1
  selector:
    matchLabels:
      app: thermia-exporter
  template:
    metadata:
      labels:
        app: thermia-exporter
    spec:
      containers:
      - name: exporter
        image: ghcr.io/grimne/thermia_exporter:latest
        ports:
        - containerPort: 9808
          name: metrics
        volumeMounts:
        - name: credentials
          mountPath: /var/run/secrets/thermia
          readOnly: true
        env:
        - name: THERMIA_LOG_LEVEL
          value: "info"
        - name: THERMIA_LOG_FORMAT
          value: "json"
        livenessProbe:
          httpGet:
            path: /health
            port: 9808
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 9808
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: credentials
        secret:
          secretName: thermia-credentials
---
apiVersion: v1
kind: Service
metadata:
  name: thermia-exporter
  labels:
    app: thermia-exporter
spec:
  ports:
  - port: 9808
    targetPort: 9808
    name: metrics
  selector:
    app: thermia-exporter
```

---

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `THERMIA_USERNAME` | Yes* | - | Thermia Online username (email) |
| `THERMIA_PASSWORD` | Yes* | - | Thermia Online password |
| `THERMIA_ADDR` | No | `:9808` | HTTP listen address |
| `THERMIA_LOG_LEVEL` | No | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `THERMIA_LOG_FORMAT` | No | `text` | Log format: `text`, `json` |
| `THERMIA_REQUEST_TIMEOUT` | No | `120` | API request timeout in seconds |
| `THERMIA_SECRETS_PATH` | No | `/var/run/secrets/thermia` | Path to mounted Kubernetes secrets |

\* Not required if using Kubernetes secrets

### Kubernetes Secrets

The exporter automatically reads credentials from mounted secret files:
- `/var/run/secrets/thermia/username`
- `/var/run/secrets/thermia/password`

**Kubernetes secrets take precedence over environment variables**

---

## Endpoints

- `/metrics` - Prometheus metrics
- `/health` - Health check endpoint

---

## Example Metrics

```prometheus
# HELP thermia_online Online (1) / Offline (0)
# TYPE thermia_online gauge
thermia_online{heatpump_id="1234567",heatpump_name="MyHeatPump",model="Thermia"} 1

# HELP thermia_indoor_temperature_celsius Indoor temperature (°C)
# TYPE thermia_indoor_temperature_celsius gauge
thermia_indoor_temperature_celsius{heatpump_id="1234567",heatpump_name="MyHeatPump",model="Thermia"} 22.3

# HELP thermia_supply_line_temperature_celsius Supply line temperature (°C)
# TYPE thermia_supply_line_temperature_celsius gauge
thermia_supply_line_temperature_celsius{heatpump_id="1234567",heatpump_name="MyHeatPump",model="Thermia"} 35.2

# HELP thermia_operation_mode Current operation mode (1 for current)
# TYPE thermia_operation_mode gauge
thermia_operation_mode{heatpump_id="1234567",heatpump_name="MyHeatPump",model="Thermia",mode="AUTO"} 1

# HELP thermia_operational_status_running Operational status one-hot (1 for current, 0 for others)
# TYPE thermia_operational_status_running gauge
thermia_operational_status_running{heatpump_id="1234567",heatpump_name="MyHeatPump",model="Thermia",status="STATUS_HEAT"} 1

# HELP thermia_scrape_duration_seconds Time spent scraping Thermia API
# TYPE thermia_scrape_duration_seconds histogram
thermia_scrape_duration_seconds_bucket{le="1"} 0
thermia_scrape_duration_seconds_bucket{le="5"} 45
thermia_scrape_duration_seconds_bucket{le="10"} 98
thermia_scrape_duration_seconds_bucket{le="+Inf"} 100
thermia_scrape_duration_seconds_sum 456.7
thermia_scrape_duration_seconds_count 100
```

---

## Troubleshooting

### Authentication Issues

Enable debug logging to see the OAuth2 flow:
```bash
export THERMIA_LOG_LEVEL=debug
./thermia-exporter
```

Check logs for warnings like:
```
level=WARN msg="Failed to get temperature registers" id=1234567 error="status 404"
```

### Prometheus Scrape Timeouts

If scrapes are timing out, increase the scrape timeout in Prometheus:
```yaml
scrape_configs:
  - job_name: 'thermia'
    scrape_interval: 300s
    scrape_timeout: 30s  # Increase if needed
    static_configs:
      - targets: ['thermia-exporter:9808']
```
Or using `vmagent`
```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMPodScrape
metadata:
  name: thermia-exporter
  namespace: mynamespace
spec:
  selector:
    matchLabels:
      app: thermia-exporter
  namespaceSelector:
    matchNames: ["mynamespace"]
  podMetricsEndpoints:
    - port: metrics
      path: /metrics
      interval: 300s
      scrapeTimeout: 30s
      scheme: http
      honorLabels: true
```

---

## License

This project is provided as-is for personal and educational use.
