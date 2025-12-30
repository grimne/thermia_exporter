package mapper

import (
	"thermia_exporter/internal/types"
)

// ExtractTemperatures extracts temperature data from installation status and register groups.
// It tries multiple fallback register names for each temperature to handle different heat pump models.
func ExtractTemperatures(status *types.InstallationStatus, grp []types.GroupItem) types.TemperatureData {
	data := types.TemperatureData{
		Indoor:     status.IndoorTemperature,
		HotWater:   status.HotWaterTemperature,
		SupplyLine: status.SupplyLine,
		DesiredSupplyLine: firstNonNil(
			status.DesiredSupplyLineTemperature,
			findValue(grp, RegDesiredSupplyLineTemp),
			findValue(grp, RegDesiredSupplyLine),
			findValue(grp, RegDesiredSysSupplyLineTemp),
		),
		BufferTank: status.BufferTankTemperature,
		ReturnLine: firstNonNil(
			status.ReturnLineTemperature,
			findValue(grp, RegReturnLine),
			findValue(grp, RegOperDataReturn),
		),
		BrineOut:      status.BrineOutTemperature,
		BrineIn:       status.BrineInTemperature,
		Pool:          status.PoolTemperature,
		CoolingTank:   status.CoolingTankTemperature,
		CoolingSupply: status.CoolingSupplyLineTemperature,
	}

	// Fallback to register groups if status doesn't have the value
	if data.Indoor == nil {
		data.Indoor = findValue(grp, RegIndoorTemperature)
	}
	if data.SupplyLine == nil {
		data.SupplyLine = findValue(grp, RegSupplyLine)
	}
	if data.BufferTank == nil {
		data.BufferTank = findValue(grp, RegOperDataBufferTank)
	}
	if data.BrineOut == nil {
		data.BrineOut = findValue(grp, RegBrineOut)
	}
	if data.BrineIn == nil {
		data.BrineIn = findValue(grp, RegBrineIn)
	}
	if data.Pool == nil {
		data.Pool = findValue(grp, RegActualPoolTemp)
	}
	if data.CoolingTank == nil {
		data.CoolingTank = findValue(grp, RegCoolSensorTank)
	}
	if data.CoolingSupply == nil {
		data.CoolingSupply = findValue(grp, RegCoolSensorSupply)
	}

	return data
}

// TemperaturesToMap converts temperature data to a map of metric names to values.
// It excludes nil values and rounds to 1 decimal place.
// It also filters out invalid indoor temperatures (>= 100°C).
func TemperaturesToMap(t types.TemperatureData) map[string]float64 {
	result := make(map[string]float64)

	if t.Indoor != nil && *t.Indoor < 100 {
		result["indoor"] = round1(*t.Indoor)
	}
	if t.Outdoor != nil {
		result["outdoor"] = round1(*t.Outdoor)
	}
	if t.SupplyLine != nil {
		result["supply_line"] = round1(*t.SupplyLine)
	}
	if t.DesiredSupplyLine != nil {
		result["desired_supply_line"] = round1(*t.DesiredSupplyLine)
	}
	if t.ReturnLine != nil {
		result["return_line"] = round1(*t.ReturnLine)
	}
	if t.BufferTank != nil {
		result["buffer_tank"] = round1(*t.BufferTank)
	}
	if t.HotWater != nil {
		result["hot_water"] = round1(*t.HotWater)
	}
	if t.BrineOut != nil {
		result["brine_out"] = round1(*t.BrineOut)
	}
	if t.BrineIn != nil {
		result["brine_in"] = round1(*t.BrineIn)
	}
	if t.Pool != nil {
		result["pool"] = round1(*t.Pool)
	}
	if t.CoolingTank != nil {
		result["cooling_tank"] = round1(*t.CoolingTank)
	}
	if t.CoolingSupply != nil {
		result["cooling_supply"] = round1(*t.CoolingSupply)
	}

	return result
}

// findValue searches for a register by name and returns its value if found.
func findValue(items []types.GroupItem, registerName string) *float64 {
	for _, it := range items {
		if it.RegisterName == registerName && it.RegisterValue != nil {
			return it.RegisterValue
		}
	}
	return nil
}

// FindValue is the public version of findValue for use by other packages.
func FindValue(items []types.GroupItem, registerName string) *float64 {
	return findValue(items, registerName)
}

// firstNonNil returns the first non-nil value from the provided pointers.
func firstNonNil(vals ...*float64) *float64 {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

// round1 rounds a float to 1 decimal place.
func round1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10.0
}
