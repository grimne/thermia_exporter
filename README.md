# Thermia Exporter

A Prometheus exporter written in Go, with help from ChatGPT, for **Thermia Online** heat pumps.
It logs in using the same flow as the Thermia web app and exposes heat pump metrics at `:9808/metrics`.

---

## Features

- **Temperatures** (indoor, outdoor, supply, desired supply, return, buffer tank, hot water, brine in/out, pool, cooling tank/supply).
- **Online status** + last online timestamp.
- **Operation modes** (current and available).
- **Operational statuses** (current and available bit flags).
- **Power statuses** (current and available bit flags).
- **Hot water switch and boost state**.
- **Operational time counters** (compressor, heating, hot water, aux heaters).
- **Alert counts** (active vs archived/history).
- Low-cardinality metrics suitable for Prometheus/Grafana.

---

## Installation

```bash
git clone https://github.com/yourorg/thermia_exporter.git
cd thermia_exporter

go build ./cmd/thermia-exporter

# Run locally
export THERMIA_USERNAME="you@example.com"
export THERMIA_PASSWORD="your_password"

./thermia-exporter
```

## Usage
Environment variables
`THERMIA_USERNAME` – Thermia Online username (email).
`THERMIA_PASSWORD` – Thermia Online password.
`THERMIA_ADDR` – (optional) listen address, default :9808.
`THERMIA_SCRAPE_INTERVAL` – (optional) scrape interval in seconds, default 60.

## Example metrics
```
# HELP thermia_online Online (1) / Offline (0)
# TYPE thermia_online gauge
thermia_online{heatpump_id="1234567",heatpump_name="HeatPumpAtHome",model="Thermia Model"} 1

# HELP thermia_hot_water_temperature_celsius Hot water temperature (°C)
# TYPE thermia_hot_water_temperature_celsius gauge
thermia_hot_water_temperature_celsius{heatpump_id="1234567",heatpump_name="HeatPumpAtHome",model="Thermia Model"} 44

# HELP thermia_active_alerts Number of active alerts
# TYPE thermia_active_alerts gauge
thermia_active_alerts{heatpump_id="1234567",heatpump_name="HeatPumpAtHome",model="Thermia Model"} 2
```

## Metrics overview
### Temperatures
- `thermia_indoor_temperature_celsius`
- `thermia_outdoor_temperature_celsius`
- `thermia_supply_line_temperature_celsius`
- `thermia_desired_supply_line_temperature_celsius`
- `thermia_return_line_temperature_celsius`
- `thermia_buffer_tank_temperature_celsius`
- `thermia_hot_water_temperature_celsius`
- `thermia_brine_out_temperature_celsius`
- `thermia_brine_in_temperature_celsius`
- `thermia_pool_temperature_celsius`
- `thermia_cooling_tank_temperature_celsius`
- `thermia_cooling_supply_temperature_celsius`

### Status
- `thermia_online (1/0)`
- `thermia_last_online_unix`

### Operation & power
- `thermia_operation_mode{mode=...} = 1`
- `thermia_operation_mode_available{mode=...} = 1`
- `thermia_operational_status_running{status=...} = 1`
- `thermia_operational_status_available{status=...} = 1`
- `thermia_power_status_running{status=...} = 1`
- `thermia_power_status_available{status=...} = 1`

### Hot water
- `thermia_hot_water_switch_state (0/1)`
- `thermia_hot_water_boost_state (0/1)`

Operational time
- `thermia_oper_time_compressor_hours`
- `thermia_oper_time_heating_hours`
- `thermia_oper_time_hot_water_hours`
- `thermia_oper_time_imm1_hours`
- `thermia_oper_time_imm2_hours`
- `thermia_oper_time_imm3_hours`

### Alerts
- `thermia_active_alerts`
- `thermia_archived_alerts`
