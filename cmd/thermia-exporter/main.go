package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"thermia_exporter/internal/thermia"
)

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	user := os.Getenv("THERMIA_USERNAME")
	pass := os.Getenv("THERMIA_PASSWORD")
	if user == "" || pass == "" {
		fmt.Println("Set THERMIA_USERNAME and THERMIA_PASSWORD")
		os.Exit(1)
	}

	addr := envOrDefault("THERMIA_ADDR", ":9808")
	intervalStr := envOrDefault("THERMIA_SCRAPE_INTERVAL", "60") // seconds
	intervalSec, _ := strconv.Atoi(intervalStr)
	if intervalSec <= 0 {
		intervalSec = 60
	}

	labels := []string{"heatpump_id", "heatpump_name", "model"}

	// === Temperature metrics ===
	gIndoor := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_indoor_temperature_celsius", Help: "Indoor temperature (°C)"}, labels)
	gOutdoor := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_outdoor_temperature_celsius", Help: "Outdoor temperature (°C)"}, labels)
	gSupply := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_supply_line_temperature_celsius", Help: "Supply line temperature (°C)"}, labels)
	gDesiredSupply := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_desired_supply_line_temperature_celsius", Help: "Desired supply line temperature (°C)"}, labels)
	gReturn := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_return_line_temperature_celsius", Help: "Return line temperature (°C)"}, labels)
	gBuffer := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_buffer_tank_temperature_celsius", Help: "Buffer tank temperature (°C)"}, labels)
	gHotWater := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_hot_water_temperature_celsius", Help: "Hot water temperature (°C)"}, labels)
	gBrineOut := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_brine_out_temperature_celsius", Help: "Brine out temperature (°C)"}, labels)
	gBrineIn := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_brine_in_temperature_celsius", Help: "Brine in temperature (°C)"}, labels)
	gPool := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_pool_temperature_celsius", Help: "Pool temperature (°C)"}, labels)
	gCoolTank := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_cooling_tank_temperature_celsius", Help: "Cooling tank temperature (°C)"}, labels)
	gCoolSupply := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_cooling_supply_temperature_celsius", Help: "Cooling supply line temperature (°C)"}, labels)

	// === Online / time ===
	gOnline := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_online", Help: "Online (1) / Offline (0)"}, labels)
	gLastOnlineUnix := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_last_online_unix", Help: "Last online timestamp (unix seconds)"}, labels)

	// === Operation modes & statuses ===
	gOperationMode := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_operation_mode", Help: "Current operation mode (1 for current)"}, append(labels, "mode"))
	gOperationModeAvailable := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_operation_mode_available", Help: "Available operation modes (1)"}, append(labels, "mode"))

	// One-hot operational status (exactly one = 1, rest = 0)
	gOperationalStatus := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_operational_status_running", Help: "Operational status one-hot (1 for current, 0 for others)"}, append(labels, "status"))
	gOperationalStatusAvail := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_operational_status_available", Help: "Operational statuses available (1)"}, append(labels, "status"))

	// Power statuses can remain bitmask-style (multiple may be 1)
	gPowerStatus := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_power_status_running", Help: "Power status bits that are running (1)"}, append(labels, "status"))
	gPowerStatusAvail := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_power_status_available", Help: "Power statuses available (1)"}, append(labels, "status"))

	// === Hot water switches ===
	gHWSwitch := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_hot_water_switch_state", Help: "Hot water switch state (0/1)"}, labels)
	gHWBoost := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_hot_water_boost_state", Help: "Hot water boost state (0/1)"}, labels)

	// === Operational time (hours) ===
	gOpComp := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_oper_time_compressor_hours", Help: "Operational time - compressor (hours)"}, labels)
	gOpHeat := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_oper_time_heating_hours", Help: "Operational time - heating (hours)"}, labels)
	gOpHW := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_oper_time_hot_water_hours", Help: "Operational time - hot water (hours)"}, labels)
	gOpImm1 := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_oper_time_imm1_hours", Help: "Operational time - aux heater 1 (hours)"}, labels)
	gOpImm2 := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_oper_time_imm2_hours", Help: "Operational time - aux heater 2 (hours)"}, labels)
	gOpImm3 := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_oper_time_imm3_hours", Help: "Operational time - aux heater 3 (hours)"}, labels)

	// === Alerts ===
	gActiveAlerts := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_active_alerts", Help: "Number of active alerts"}, labels)
	gArchivedAlerts := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "thermia_archived_alerts", Help: "Number of archived alerts (history minus active)"}, labels)

	prometheus.MustRegister(
		gIndoor, gOutdoor, gSupply, gDesiredSupply, gReturn, gBuffer, gHotWater, gBrineOut, gBrineIn, gPool, gCoolTank, gCoolSupply,
		gOnline, gLastOnlineUnix,
		gOperationMode, gOperationModeAvailable,
		gOperationalStatus, gOperationalStatusAvail,
		gPowerStatus, gPowerStatusAvail,
		gHWSwitch, gHWBoost,
		gOpComp, gOpHeat, gOpHW, gOpImm1, gOpImm2, gOpImm3,
		gActiveAlerts, gArchivedAlerts,
	)

	// Poller
	go func() {
		for {
			func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				sum, err := thermia.FetchThermiaSummary(ctx, user, pass)
				if err != nil {
					fmt.Println("fetch error:", err)
					return
				}

				lbls := prometheus.Labels{
					"heatpump_id":   fmt.Sprint(sum.HeatpumpID),
					"heatpump_name": sum.HeatpumpName,
					"model":         sum.HeatpumpModel,
				}

				// Online + last online
				if sum.Online {
					gOnline.With(lbls).Set(1)
				} else {
					gOnline.With(lbls).Set(0)
				}
				if ts := sum.LastOnlineUnix; ts > 0 {
					gLastOnlineUnix.With(lbls).Set(float64(ts))
				}

				// Temps
				if v, ok := sum.Temperatures["indoor"]; ok {
					gIndoor.With(lbls).Set(v)
				}
				if v, ok := sum.Temperatures["outdoor"]; ok {
					gOutdoor.With(lbls).Set(v)
				}
				if v, ok := sum.Temperatures["supply_line"]; ok {
					gSupply.With(lbls).Set(v)
				}
				if v, ok := sum.Temperatures["desired_supply_line"]; ok {
					gDesiredSupply.With(lbls).Set(v)
				}
				if v, ok := sum.Temperatures["return_line"]; ok {
					gReturn.With(lbls).Set(v)
				}
				if v, ok := sum.Temperatures["buffer_tank"]; ok {
					gBuffer.With(lbls).Set(v)
				}
				if v, ok := sum.Temperatures["hot_water"]; ok {
					gHotWater.With(lbls).Set(v)
				}
				if v, ok := sum.Temperatures["brine_out"]; ok {
					gBrineOut.With(lbls).Set(v)
				}
				if v, ok := sum.Temperatures["brine_in"]; ok {
					gBrineIn.With(lbls).Set(v)
				}
				if v, ok := sum.Temperatures["pool"]; ok {
					gPool.With(lbls).Set(v)
				}
				if v, ok := sum.Temperatures["cooling_tank"]; ok {
					gCoolTank.With(lbls).Set(v)
				}
				if v, ok := sum.Temperatures["cooling_supply"]; ok {
					gCoolSupply.With(lbls).Set(v)
				}

				// Operation modes
				for _, m := range sum.OperationModesAvailable {
					gOperationModeAvailable.With(merge(lbls, "mode", m)).Set(1)
				}
				if sum.OperationMode != "" {
					gOperationMode.With(merge(lbls, "mode", sum.OperationMode)).Set(1)
				}

				// Operational statuses (one-hot)
				for _, s := range sum.OperationalStatusAvailable {
					gOperationalStatusAvail.With(merge(lbls, "status", s)).Set(1)
				}
				curr := pickCurrentStatus(sum.OperationalStatusRunning, sum.OperationalStatusAvailable)
				for _, s := range sum.OperationalStatusAvailable {
					v := 0.0
					if strings.EqualFold(s, curr) {
						v = 1.0
					}
					gOperationalStatus.With(merge(lbls, "status", s)).Set(v)
				}

				// Power statuses (bitmask)
				for _, s := range sum.PowerStatusAvailable {
					gPowerStatusAvail.With(merge(lbls, "status", s)).Set(1)
				}
				// Reset previously set power running to 0 before marking current as 1? Not necessary—series will be overwritten.
				for _, s := range sum.PowerStatusRunning {
					gPowerStatus.With(merge(lbls, "status", s)).Set(1)
				}

				// Hot water
				if sum.HotWaterSwitch != nil {
					gHWSwitch.With(lbls).Set(float64(*sum.HotWaterSwitch))
				}
				if sum.HotWaterBoost != nil {
					gHWBoost.With(lbls).Set(float64(*sum.HotWaterBoost))
				}

				// Operational time
				if v, ok := sum.OperationalTimeHours["REG_OPER_TIME_COMPRESSOR"]; ok {
					gOpComp.With(lbls).Set(float64(v))
				}
				if v, ok := sum.OperationalTimeHours["REG_OPER_TIME_HEATING"]; ok {
					gOpHeat.With(lbls).Set(float64(v))
				}
				if v, ok := sum.OperationalTimeHours["REG_OPER_TIME_HOT_WATER"]; ok {
					gOpHW.With(lbls).Set(float64(v))
				}
				if v, ok := sum.OperationalTimeHours["REG_OPER_TIME_IMM1"]; ok {
					gOpImm1.With(lbls).Set(float64(v))
				}
				if v, ok := sum.OperationalTimeHours["REG_OPER_TIME_IMM2"]; ok {
					gOpImm2.With(lbls).Set(float64(v))
				}
				if v, ok := sum.OperationalTimeHours["REG_OPER_TIME_IMM3"]; ok {
					gOpImm3.With(lbls).Set(float64(v))
				}

				// Alerts
				gActiveAlerts.With(lbls).Set(float64(len(sum.ActiveAlerts)))
				gArchivedAlerts.With(lbls).Set(float64(len(sum.ArchivedAlerts)))
			}()
			time.Sleep(time.Duration(intervalSec) * time.Second)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	fmt.Println("Listening on", addr, "…")
	if err := http.ListenAndServe(addr, nil); err != nil {
		panic(err)
	}
}

func merge(m prometheus.Labels, k, v string) prometheus.Labels {
	out := prometheus.Labels{}
	for kk, vv := range m {
		out[kk] = vv
	}
	out[k] = v
	return out
}

// choose one current operational status from possibly-many running bits
func pickCurrentStatus(running, available []string) string {
	if len(running) == 0 && len(available) == 0 {
		return ""
	}
	// If both NO_DEMAND and another status are present, drop NO_DEMAND
	filtered := make([]string, 0, len(running))
	for _, s := range running {
		if strings.EqualFold(s, "STATUS_NO_DEMAND") {
			continue
		}
		filtered = append(filtered, s)
	}
	if len(filtered) == 0 {
		filtered = running
	}
	// Priority order (highest first)
	priority := []string{
		"STATUS_LEGIONELLA",
		"STATUS_HOTWATER",
		"STATUS_HEAT",
		"STATUS_COOL",
		"STATUS_PASSIVE_COOL",
		"STATUS_POOL",
		"STATUS_STANDBY",
		"STATUS_NO_DEMAND",
		"OPERATION_MODE_OFF",
	}
	have := map[string]struct{}{}
	for _, s := range filtered {
		have[strings.ToUpper(s)] = struct{}{}
	}
	for _, p := range priority {
		if _, ok := have[p]; ok {
			return p
		}
	}
	// Fallback: first running, or first available if none running
	if len(filtered) > 0 {
		return filtered[0]
	}
	if len(available) > 0 {
		return available[0]
	}
	return ""
}
