package thermia

// Public summary returned to the exporter
type ThermiaSummary struct {
	HeatpumpID      int64              `json:"heatpump_id"`
	HeatpumpName    string             `json:"heatpump_name"`
	HeatpumpModel   string             `json:"heatpump_model"`
	Online          bool               `json:"online"`
	LastOnline      string             `json:"last_online"`
	LastOnlineUnix  int64              `json:"last_online_unix"`
	Temperatures    map[string]float64 `json:"temperatures"`
	OperationModesAvailable    []string          `json:"operation_modes_available"`
	OperationMode              string            `json:"operation_mode"`
	OperationalStatusAvailable []string          `json:"operational_status_available"`
	OperationalStatusRunning   []string          `json:"operational_status_running"`
	PowerStatusAvailable       []string          `json:"power_status_available"`
	PowerStatusRunning         []string          `json:"power_status_running"`
	HotWaterSwitch             *int              `json:"hot_water_switch"`
	HotWaterBoost              *int              `json:"hot_water_boost"`
	OperationalTimeHours       map[string]int    `json:"operational_time_h"`
	ActiveAlerts               []string          `json:"active_alerts"`
	ArchivedAlerts             []string          `json:"archived_alerts"`
}

// ===== Raw API shapes (subset) =====
type Config struct {
	APIBaseURL string `json:"apiBaseUrl"`
}

type InstallationList struct{ Items []Installation `json:"items"` }
type Installation struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

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

// Generic group item
type GroupItem struct {
	RegisterName  string       `json:"registerName"`
	RegisterValue *float64     `json:"registerValue"`
	Unit          string       `json:"unit"`
	IsReadOnly    bool         `json:"isReadOnly"`
	ValueNames    []ValueEntry `json:"valueNames"`
	StringValue   *string      `json:"stringRegisterValue"`
}
type ValueEntry struct {
	Name     string `json:"name"`
	Value    int    `json:"value"`
	Visible  bool   `json:"visible"`
	Readonly bool   `json:"isReadonly"`
}

type Event struct {
	EventTitle   string  `json:"eventTitle"`
	Severity     string  `json:"severity"`
	OccurredWhen string  `json:"occurredWhen"`
	ClearedWhen  *string `json:"clearedWhen"`
	IsActive     *bool   `json:"isActive"`
}

// helpers
type tempsOut struct {
	Indoor, SupplyLine, DesiredSupplyLine, BufferTank, ReturnLine, HotWater, BrineOut, BrineIn, Pool, CoolingTank, CoolingSupply *float64
}
