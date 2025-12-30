// Package mapper provides data extraction and transformation functions for Thermia API responses.
package mapper

// Register group names
const (
	RegGroupTemperatures         = "REG_GROUP_TEMPERATURES"
	RegGroupOperationalStatus    = "REG_GROUP_OPERATIONAL_STATUS"
	RegGroupOperationalTime      = "REG_GROUP_OPERATIONAL_TIME"
	RegGroupOperationalOperation = "REG_GROUP_OPERATIONAL_OPERATION"
	RegGroupHotWater             = "REG_GROUP_HOT_WATER"
)

// Temperature register names
const (
	RegIndoorTemperature        = "REG_INDOOR_TEMPERATURE"
	RegOutdoorTemperature       = "REG_OUTDOOR_TEMPERATURE"
	RegOperDataOutdoorTempMaSa  = "REG_OPER_DATA_OUTDOOR_TEMP_MA_SA"
	RegSupplyLine               = "REG_SUPPLY_LINE"
	RegDesiredSupplyLineTemp    = "REG_DESIRED_SUPPLY_LINE_TEMP"
	RegDesiredSupplyLine        = "REG_DESIRED_SUPPLY_LINE"
	RegDesiredSysSupplyLineTemp = "REG_DESIRED_SYS_SUPPLY_LINE_TEMP"
	RegReturnLine               = "REG_RETURN_LINE"
	RegOperDataReturn           = "REG_OPER_DATA_RETURN"
	RegOperDataBufferTank       = "REG_OPER_DATA_BUFFER_TANK"
	RegBrineOut                 = "REG_BRINE_OUT"
	RegBrineIn                  = "REG_BRINE_IN"
	RegActualPoolTemp           = "REG_ACTUAL_POOL_TEMP"
	RegCoolSensorTank           = "REG_COOL_SENSOR_TANK"
	RegCoolSensorSupply         = "REG_COOL_SENSOR_SUPPLY"
)

// Operational status register names
const (
	RegOperationalStatusPriorityBitmask = "REG_OPERATIONAL_STATUS_PRIORITY_BITMASK"
	CompStatus                          = "COMP_STATUS"
	CompStatusAtec                      = "COMP_STATUS_ATEC"
	CompStatusItec                      = "COMP_STATUS_ITEC"
	CompPowerStatus                     = "COMP_POWER_STATUS"
)

// Operation mode register name
const (
	RegOperationMode = "REG_OPERATIONMODE"
)

// Hot water register names
const (
	RegHotWaterBoost  = "REG__HOT_WATER_BOOST"
	RegHotWaterStatus = "REG_HOT_WATER_STATUS"
)

// Operational time register names
const (
	RegOperTimeCompressor = "REG_OPER_TIME_COMPRESSOR"
	RegOperTimeHeating    = "REG_OPER_TIME_HEATING"
	RegOperTimeHotWater   = "REG_OPER_TIME_HOT_WATER"
	RegOperTimeImm1       = "REG_OPER_TIME_IMM1"
	RegOperTimeImm2       = "REG_OPER_TIME_IMM2"
	RegOperTimeImm3       = "REG_OPER_TIME_IMM3"
)

// Prometheus metric label names
const (
	LabelHeatpumpID   = "heatpump_id"
	LabelHeatpumpName = "heatpump_name"
	LabelModel        = "model"
	LabelMode         = "mode"
	LabelStatus       = "status"
)

// String trimming prefixes
const (
	StatusPrefixRegValue  = "REG_VALUE_"
	StatusPrefixCompValue = "COMP_VALUE_"
	ModePrefixRegValue    = "REG_VALUE_OPERATION_MODE_"
)

// OperationalStatusCandidates lists the register names to check for operational status bitmasks.
var OperationalStatusCandidates = []string{
	RegOperationalStatusPriorityBitmask,
	CompStatus,
	CompStatusAtec,
	CompStatusItec,
}

// PowerStatusCandidates lists the register names to check for power status bitmasks.
var PowerStatusCandidates = []string{
	CompPowerStatus,
}
