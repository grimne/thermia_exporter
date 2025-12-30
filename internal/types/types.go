// Package types contains shared type definitions used across the thermia_exporter packages.
package types

// ThermiaSummary is the public summary returned to the exporter containing all heat pump metrics.
type ThermiaSummary struct {
	HeatpumpID                 int64              `json:"heatpump_id"`
	HeatpumpName               string             `json:"heatpump_name"`
	HeatpumpModel              string             `json:"heatpump_model"`
	Online                     bool               `json:"online"`
	LastOnline                 string             `json:"last_online"`
	LastOnlineUnix             int64              `json:"last_online_unix"`
	Temperatures               map[string]float64 `json:"temperatures"`
	OperationModesAvailable    []string           `json:"operation_modes_available"`
	OperationMode              string             `json:"operation_mode"`
	OperationalStatusAvailable []string           `json:"operational_status_available"`
	OperationalStatusRunning   []string           `json:"operational_status_running"`
	PowerStatusAvailable       []string           `json:"power_status_available"`
	PowerStatusRunning         []string           `json:"power_status_running"`
	HotWaterSwitch             *int               `json:"hot_water_switch"`
	HotWaterBoost              *int               `json:"hot_water_boost"`
	OperationalTimeHours       map[string]int     `json:"operational_time_h"`
	ActiveAlerts               []string           `json:"active_alerts"`
	ArchivedAlerts             []string           `json:"archived_alerts"`
}

// Config represents the Thermia API configuration response.
type Config struct {
	APIBaseURL string `json:"apiBaseUrl"`
}

// Installation represents a heat pump installation.
type Installation struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// InstallationInfo contains detailed information about an installation.
type InstallationInfo struct {
	CreatedWhen string `json:"createdWhen"`
	IsOnline    bool   `json:"isOnline"`
	LastOnline  string `json:"lastOnline"`
	Model       string `json:"model"`
	Profile     struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"profile"`
	Name string `json:"name"`
}

// InstallationStatus contains real-time temperature readings from the heat pump.
type InstallationStatus struct {
	IndoorTemperature            *float64 `json:"indoorTemperature"`
	HotWaterTemperature          *float64 `json:"hotWaterTemperature"`
	SupplyLine                   *float64 `json:"supplyLineTemperature"`
	DesiredSupplyLineTemperature *float64 `json:"desiredSupplyLineTemperature"`
	BufferTankTemperature        *float64 `json:"bufferTankTemperature"`
	ReturnLineTemperature        *float64 `json:"returnLineTemperature"`
	BrineOutTemperature          *float64 `json:"brineOutTemperature"`
	BrineInTemperature           *float64 `json:"brineInTemperature"`
	PoolTemperature              *float64 `json:"poolTemperature"`
	CoolingTankTemperature       *float64 `json:"coolingTankTemperature"`
	CoolingSupplyLineTemperature *float64 `json:"coolingSupplyLineTemperature"`
}

// GroupItem represents a register item from a register group.
type GroupItem struct {
	RegisterName  string       `json:"registerName"`
	RegisterValue *float64     `json:"registerValue"`
	Unit          string       `json:"unit"`
	IsReadOnly    bool         `json:"isReadOnly"`
	ValueNames    []ValueEntry `json:"valueNames"`
	StringValue   *string      `json:"stringRegisterValue"`
}

// ValueEntry represents a possible value for a register.
type ValueEntry struct {
	Name     string `json:"name"`
	Value    int    `json:"value"`
	Visible  bool   `json:"visible"`
	Readonly bool   `json:"isReadonly"`
}

// Event represents an alarm or event from the heat pump.
type Event struct {
	EventTitle   string  `json:"eventTitle"`
	Severity     string  `json:"severity"`
	OccurredWhen string  `json:"occurredWhen"`
	ClearedWhen  *string `json:"clearedWhen"`
	IsActive     *bool   `json:"isActive"`
}

// TemperatureData holds all extracted temperature values.
type TemperatureData struct {
	Indoor            *float64
	Outdoor           *float64
	SupplyLine        *float64
	DesiredSupplyLine *float64
	ReturnLine        *float64
	BufferTank        *float64
	HotWater          *float64
	BrineOut          *float64
	BrineIn           *float64
	Pool              *float64
	CoolingTank       *float64
	CoolingSupply     *float64
}

// OperationModeData holds operation mode information.
type OperationModeData struct {
	Current   string
	Available []string
	ReadOnly  bool
}

// StatusData holds bitmask status information.
type StatusData struct {
	Running   []string
	Available []string
}
