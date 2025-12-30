package collector

import (
	"github.com/prometheus/client_golang/prometheus"

	"thermia_exporter/internal/mapper"
)

// MetricSet holds all Prometheus metric descriptors for the Thermia exporter.
type MetricSet struct {
	// Temperature metrics
	indoorTemp          *prometheus.Desc
	outdoorTemp         *prometheus.Desc
	supplyLineTemp      *prometheus.Desc
	desiredSupplyTemp   *prometheus.Desc
	returnLineTemp      *prometheus.Desc
	bufferTankTemp      *prometheus.Desc
	hotWaterTemp        *prometheus.Desc
	brineOutTemp        *prometheus.Desc
	brineInTemp         *prometheus.Desc
	poolTemp            *prometheus.Desc
	coolingTankTemp     *prometheus.Desc
	coolingSupplyTemp   *prometheus.Desc

	// Status metrics
	online         *prometheus.Desc
	lastOnlineUnix *prometheus.Desc

	// Mode/status metrics
	operationMode      *prometheus.Desc
	operationModeAvail *prometheus.Desc
	operationalStatus  *prometheus.Desc
	operationalStatusAvail *prometheus.Desc
	powerStatus        *prometheus.Desc
	powerStatusAvail   *prometheus.Desc

	// Hot water metrics
	hotWaterSwitch *prometheus.Desc
	hotWaterBoost  *prometheus.Desc

	// Operational time metrics
	operTimeCompressor *prometheus.Desc
	operTimeHeating    *prometheus.Desc
	operTimeHotWater   *prometheus.Desc
	operTimeImm1       *prometheus.Desc
	operTimeImm2       *prometheus.Desc
	operTimeImm3       *prometheus.Desc

	// Alert metrics
	activeAlerts   *prometheus.Desc
	archivedAlerts *prometheus.Desc

	// Scrape metrics
	scrapeErrors   prometheus.Counter
	scrapeDuration prometheus.Histogram
}

// newMetricSet creates all metric descriptors.
func newMetricSet() *MetricSet {
	labels := []string{mapper.LabelHeatpumpID, mapper.LabelHeatpumpName, mapper.LabelModel}
	labelsWithMode := append(labels, mapper.LabelMode)
	labelsWithStatus := append(labels, mapper.LabelStatus)

	return &MetricSet{
		// Temperature metrics
		indoorTemp: prometheus.NewDesc(
			"thermia_indoor_temperature_celsius",
			"Indoor temperature (°C)",
			labels, nil,
		),
		outdoorTemp: prometheus.NewDesc(
			"thermia_outdoor_temperature_celsius",
			"Outdoor temperature (°C)",
			labels, nil,
		),
		supplyLineTemp: prometheus.NewDesc(
			"thermia_supply_line_temperature_celsius",
			"Supply line temperature (°C)",
			labels, nil,
		),
		desiredSupplyTemp: prometheus.NewDesc(
			"thermia_desired_supply_line_temperature_celsius",
			"Desired supply line temperature (°C)",
			labels, nil,
		),
		returnLineTemp: prometheus.NewDesc(
			"thermia_return_line_temperature_celsius",
			"Return line temperature (°C)",
			labels, nil,
		),
		bufferTankTemp: prometheus.NewDesc(
			"thermia_buffer_tank_temperature_celsius",
			"Buffer tank temperature (°C)",
			labels, nil,
		),
		hotWaterTemp: prometheus.NewDesc(
			"thermia_hot_water_temperature_celsius",
			"Hot water temperature (°C)",
			labels, nil,
		),
		brineOutTemp: prometheus.NewDesc(
			"thermia_brine_out_temperature_celsius",
			"Brine out temperature (°C)",
			labels, nil,
		),
		brineInTemp: prometheus.NewDesc(
			"thermia_brine_in_temperature_celsius",
			"Brine in temperature (°C)",
			labels, nil,
		),
		poolTemp: prometheus.NewDesc(
			"thermia_pool_temperature_celsius",
			"Pool temperature (°C)",
			labels, nil,
		),
		coolingTankTemp: prometheus.NewDesc(
			"thermia_cooling_tank_temperature_celsius",
			"Cooling tank temperature (°C)",
			labels, nil,
		),
		coolingSupplyTemp: prometheus.NewDesc(
			"thermia_cooling_supply_temperature_celsius",
			"Cooling supply line temperature (°C)",
			labels, nil,
		),

		// Status metrics
		online: prometheus.NewDesc(
			"thermia_online",
			"Online (1) / Offline (0)",
			labels, nil,
		),
		lastOnlineUnix: prometheus.NewDesc(
			"thermia_last_online_unix",
			"Last online timestamp (unix seconds)",
			labels, nil,
		),

		// Mode/status metrics
		operationMode: prometheus.NewDesc(
			"thermia_operation_mode",
			"Current operation mode (1 for current)",
			labelsWithMode, nil,
		),
		operationModeAvail: prometheus.NewDesc(
			"thermia_operation_mode_available",
			"Available operation modes (1)",
			labelsWithMode, nil,
		),
		operationalStatus: prometheus.NewDesc(
			"thermia_operational_status_running",
			"Operational status one-hot (1 for current, 0 for others)",
			labelsWithStatus, nil,
		),
		operationalStatusAvail: prometheus.NewDesc(
			"thermia_operational_status_available",
			"Operational statuses available (1)",
			labelsWithStatus, nil,
		),
		powerStatus: prometheus.NewDesc(
			"thermia_power_status_running",
			"Power status bits that are running (1)",
			labelsWithStatus, nil,
		),
		powerStatusAvail: prometheus.NewDesc(
			"thermia_power_status_available",
			"Power statuses available (1)",
			labelsWithStatus, nil,
		),

		// Hot water metrics
		hotWaterSwitch: prometheus.NewDesc(
			"thermia_hot_water_switch_state",
			"Hot water switch state (0/1)",
			labels, nil,
		),
		hotWaterBoost: prometheus.NewDesc(
			"thermia_hot_water_boost_state",
			"Hot water boost state (0/1)",
			labels, nil,
		),

		// Operational time metrics
		operTimeCompressor: prometheus.NewDesc(
			"thermia_oper_time_compressor_hours",
			"Operational time - compressor (hours)",
			labels, nil,
		),
		operTimeHeating: prometheus.NewDesc(
			"thermia_oper_time_heating_hours",
			"Operational time - heating (hours)",
			labels, nil,
		),
		operTimeHotWater: prometheus.NewDesc(
			"thermia_oper_time_hot_water_hours",
			"Operational time - hot water (hours)",
			labels, nil,
		),
		operTimeImm1: prometheus.NewDesc(
			"thermia_oper_time_imm1_hours",
			"Operational time - aux heater 1 (hours)",
			labels, nil,
		),
		operTimeImm2: prometheus.NewDesc(
			"thermia_oper_time_imm2_hours",
			"Operational time - aux heater 2 (hours)",
			labels, nil,
		),
		operTimeImm3: prometheus.NewDesc(
			"thermia_oper_time_imm3_hours",
			"Operational time - aux heater 3 (hours)",
			labels, nil,
		),

		// Alert metrics
		activeAlerts: prometheus.NewDesc(
			"thermia_active_alerts",
			"Number of active alerts",
			labels, nil,
		),
		archivedAlerts: prometheus.NewDesc(
			"thermia_archived_alerts",
			"Number of archived alerts (history minus active)",
			labels, nil,
		),

		// Scrape metrics
		scrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "thermia_scrape_errors_total",
			Help: "Total number of scrape errors",
		}),
		scrapeDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "thermia_scrape_duration_seconds",
			Help:    "Time spent scraping Thermia API",
			Buckets: []float64{1, 5, 10, 30, 60, 120},
		}),
	}
}
