package mapper

import (
	"testing"

	"thermia_exporter/internal/types"
)

func ptr(f float64) *float64 {
	return &f
}

func TestExtractTemperatures(t *testing.T) {
	status := &types.InstallationStatus{
		IndoorTemperature: ptr(22.5),
		SupplyLine:        ptr(35.2),
	}

	grp := []types.GroupItem{
		{
			RegisterName:  RegOutdoorTemperature,
			RegisterValue: ptr(-2.3),
		},
		{
			RegisterName:  RegDesiredSupplyLineTemp,
			RegisterValue: ptr(40.0),
		},
	}

	temps := ExtractTemperatures(status, grp)

	if temps.Indoor == nil || *temps.Indoor != 22.5 {
		t.Errorf("Indoor = %v, want 22.5", temps.Indoor)
	}
	if temps.SupplyLine == nil || *temps.SupplyLine != 35.2 {
		t.Errorf("SupplyLine = %v, want 35.2", temps.SupplyLine)
	}
	if temps.DesiredSupplyLine == nil || *temps.DesiredSupplyLine != 40.0 {
		t.Errorf("DesiredSupplyLine = %v, want 40.0", temps.DesiredSupplyLine)
	}
}

func TestTemperaturesToMap(t *testing.T) {
	temps := types.TemperatureData{
		Indoor:     ptr(22.5),
		Outdoor:    ptr(-2.3),
		SupplyLine: ptr(35.234),
	}

	m := TemperaturesToMap(temps)

	if m["indoor"] != 22.5 {
		t.Errorf("indoor = %v, want 22.5", m["indoor"])
	}
	if m["outdoor"] != -2.2 {
		t.Errorf("outdoor = %v, want -2.2 (rounded)", m["outdoor"])
	}
	// Should be rounded to 1 decimal
	if m["supply_line"] != 35.2 {
		t.Errorf("supply_line = %v, want 35.2 (rounded)", m["supply_line"])
	}
}

func TestTemperaturesToMap_FilterInvalidIndoor(t *testing.T) {
	temps := types.TemperatureData{
		Indoor: ptr(150.0), // Invalid - too high
	}

	m := TemperaturesToMap(temps)

	if _, exists := m["indoor"]; exists {
		t.Error("indoor should be filtered out when >= 100°C")
	}
}

func TestExtractOperationMode(t *testing.T) {
	items := []types.GroupItem{
		{
			RegisterName:  RegOperationMode,
			RegisterValue: ptr(0),
			IsReadOnly:    false,
			ValueNames: []types.ValueEntry{
				{Name: "REG_VALUE_OPERATION_MODE_AUTO", Value: 0, Visible: true},
				{Name: "REG_VALUE_OPERATION_MODE_MANUAL", Value: 1, Visible: true},
				{Name: "REG_VALUE_HIDDEN", Value: 2, Visible: false},
			},
		},
	}

	modeData := ExtractOperationMode(items)

	if modeData.Current != "AUTO" {
		t.Errorf("Current = %v, want AUTO", modeData.Current)
	}
	if len(modeData.Available) != 2 {
		t.Errorf("Available length = %d, want 2", len(modeData.Available))
	}
	if modeData.ReadOnly {
		t.Error("ReadOnly should be false")
	}
}

func TestExtractBitmaskStatuses(t *testing.T) {
	items := []types.GroupItem{
		{
			RegisterName:  CompStatus,
			RegisterValue: ptr(5), // Binary 101 = bits 0 and 2
			ValueNames: []types.ValueEntry{
				{Name: "REG_VALUE_STATUS_A", Value: 1, Visible: true},  // Bit 0
				{Name: "REG_VALUE_STATUS_B", Value: 2, Visible: true},  // Bit 1
				{Name: "REG_VALUE_STATUS_C", Value: 4, Visible: true},  // Bit 2
				{Name: "REG_VALUE_HIDDEN", Value: 8, Visible: false},
			},
		},
	}

	statusData := ExtractBitmaskStatuses(items, []string{CompStatus})

	if len(statusData.Running) != 2 {
		t.Errorf("Running length = %d, want 2", len(statusData.Running))
	}
	if len(statusData.Available) != 3 {
		t.Errorf("Available length = %d, want 3", len(statusData.Available))
	}

	// Check that STATUS_A and STATUS_C are running (bits 0 and 2)
	found := make(map[string]bool)
	for _, s := range statusData.Running {
		found[s] = true
	}
	if !found["STATUS_A"] {
		t.Error("STATUS_A should be running")
	}
	if found["STATUS_B"] {
		t.Error("STATUS_B should not be running")
	}
	if !found["STATUS_C"] {
		t.Error("STATUS_C should be running")
	}
}

func TestExtractHotWaterSwitches(t *testing.T) {
	items := []types.GroupItem{
		{
			RegisterName:  RegHotWaterStatus,
			RegisterValue: ptr(1),
		},
		{
			RegisterName:  RegHotWaterBoost,
			RegisterValue: ptr(0),
		},
	}

	switchState, boostState := ExtractHotWaterSwitches(items)

	if switchState == nil || *switchState != 1 {
		t.Errorf("switchState = %v, want 1", switchState)
	}
	if boostState == nil || *boostState != 0 {
		t.Errorf("boostState = %v, want 0", boostState)
	}
}

func TestExtractOperationalTime(t *testing.T) {
	items := []types.GroupItem{
		{
			RegisterName:  RegOperTimeCompressor,
			RegisterValue: ptr(1234.5),
		},
		{
			RegisterName:  RegOperTimeHeating,
			RegisterValue: ptr(567.8),
		},
	}

	opTime := ExtractOperationalTime(items)

	if opTime[RegOperTimeCompressor] != 1234 {
		t.Errorf("Compressor hours = %d, want 1234", opTime[RegOperTimeCompressor])
	}
	if opTime[RegOperTimeHeating] != 567 {
		t.Errorf("Heating hours = %d, want 567", opTime[RegOperTimeHeating])
	}
}

func TestParseTimeToUnix(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{"empty", "", 0},
		{"ISO8601", "2024-01-15T10:30:00.000Z", 1705314600},
		{"invalid", "not-a-date", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTimeToUnix(tt.input)
			if got != tt.want {
				t.Errorf("ParseTimeToUnix(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSafe(t *testing.T) {
	tests := []struct {
		value    string
		fallback string
		want     string
	}{
		{"value", "fallback", "value"},
		{"  value  ", "fallback", "value"},
		{"", "fallback", "fallback"},
		{"  ", "fallback", "fallback"},
	}

	for _, tt := range tests {
		got := Safe(tt.value, tt.fallback)
		if got != tt.want {
			t.Errorf("Safe(%q, %q) = %q, want %q", tt.value, tt.fallback, got, tt.want)
		}
	}
}
