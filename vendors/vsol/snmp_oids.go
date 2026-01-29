package vsol

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nanoncore/nano-southbound/vendors/common"
)

// V-SOL GPON OLT SNMP OIDs
// Based on olt-simulator OID templates (olt-simulator/devices/tier3_vendors/vsol/v2.1.6r/oids_templates.yaml)
// Enterprise OID: 1.3.6.1.4.1.37950

const (
	// Enterprise OID prefix for V-SOL
	OIDVSOLEnterprise = "1.3.6.1.4.1.37950"

	// Standard MIB-II System OIDs (RFC 1213)
	OIDSysDescr   = "1.3.6.1.2.1.1.1.0" // System description
	OIDSysUpTime  = "1.3.6.1.2.1.1.3.0" // System uptime in hundredths of seconds
	OIDSysName    = "1.3.6.1.2.1.1.5.0" // System name / hostname
	OIDIfNumber   = "1.3.6.1.2.1.2.1.0" // Number of interfaces

	// Standard MIB-II Interface Table OIDs (RFC 1213) - for fallback port listing
	OIDIfDescr       = "1.3.6.1.2.1.2.2.1.2"  // Interface description
	OIDIfAdminStatus = "1.3.6.1.2.1.2.2.1.7"  // Admin status (1=up, 2=down, 3=testing)
	OIDIfOperStatus  = "1.3.6.1.2.1.2.2.1.8"  // Operational status (1=up, 2=down, etc.)

	// V-SOL System Info OIDs (1.3.6.1.4.1.37950.1.1.5.10.12.5)
	OIDVSOLHostname         = "1.3.6.1.4.1.37950.1.1.5.10.12.5.1.0" // OLT hostname
	OIDVSOLVersion          = "1.3.6.1.4.1.37950.1.1.5.10.12.5.2.0" // Software version
	OIDVSOLSerialNumber     = "1.3.6.1.4.1.37950.1.1.5.10.12.5.3.0" // OLT serial number
	OIDVSOLSoftwareCreated  = "1.3.6.1.4.1.37950.1.1.5.10.12.5.4.0" // Software build timestamp
	OIDVSOLHardwareVersion  = "1.3.6.1.4.1.37950.1.1.5.10.12.5.5.0" // Hardware version
	OIDVSOLTemperature      = "1.3.6.1.4.1.37950.1.1.5.10.12.7.1.0" // System temperature (Celsius)
	OIDVSOLFanState         = "1.3.6.1.4.1.37950.1.1.5.10.12.7.2.0" // Fan state

	// PON Port Table OIDs (1.3.6.1.4.1.37950.1.1.6.1.2.1)
	// Format: .2.1.{attr}.{pon_idx} where pon_idx = 1-8
	OIDPONPortName           = "1.3.6.1.4.1.37950.1.1.6.1.2.1.1" // PON port name
	OIDPONPortAdminStatus    = "1.3.6.1.4.1.37950.1.1.6.1.2.1.2" // Admin status (1=enabled, 2=disabled)
	OIDPONPortOperStatus     = "1.3.6.1.4.1.37950.1.1.6.1.2.1.3" // Oper status (1=up, 2=down)
	OIDPONPortMaxONUs        = "1.3.6.1.4.1.37950.1.1.6.1.2.1.4" // Max ONUs supported
	OIDPONPortRegisteredONUs = "1.3.6.1.4.1.37950.1.1.6.1.2.1.5" // Registered ONU count
	OIDPONPortInputRate      = "1.3.6.1.4.1.37950.1.1.6.1.2.1.6" // Input rate (bps)
	OIDPONPortOutputRate     = "1.3.6.1.4.1.37950.1.1.6.1.2.1.7" // Output rate (bps)
	OIDPONPortInOctets       = "1.3.6.1.4.1.37950.1.1.6.1.2.1.8" // Input octets
	OIDPONPortOutOctets      = "1.3.6.1.4.1.37950.1.1.6.1.2.1.9" // Output octets

	// ONU Basic Info Table OIDs (1.3.6.1.4.1.37950.1.1.6.1.1.2.1)
	// Format: .2.1.{attr}.{pon_idx}.{onu_idx}
	// pon_idx = 1-8 (maps to ports 0/1 through 0/8)
	// onu_idx = 1-128 (ONU ID)
	OIDONUAdminState     = "1.3.6.1.4.1.37950.1.1.6.1.1.2.1.1"  // Admin state (1=enable, 2=disable)
	OIDONUOMCCState      = "1.3.6.1.4.1.37950.1.1.6.1.1.2.1.2"  // OMCC state
	OIDONUProfile        = "1.3.6.1.4.1.37950.1.1.6.1.1.2.1.3"  // Profile name
	OIDONUAuthMode       = "1.3.6.1.4.1.37950.1.1.6.1.1.2.1.4"  // Auth mode (sn/password)
	OIDONUSerialNumber   = "1.3.6.1.4.1.37950.1.1.6.1.1.2.1.5"  // Serial number
	OIDONUModel          = "1.3.6.1.4.1.37950.1.1.6.1.1.2.1.6"  // Model name
	OIDONUVendorID       = "1.3.6.1.4.1.37950.1.1.6.1.1.2.1.7"  // Vendor ID (4-char)
	OIDONUEquipmentID    = "1.3.6.1.4.1.37950.1.1.6.1.1.2.1.8"  // Equipment ID
	OIDONUFirmware       = "1.3.6.1.4.1.37950.1.1.6.1.1.2.1.9"  // Firmware version
	OIDONUPhaseState     = "1.3.6.1.4.1.37950.1.1.6.1.1.2.1.10" // Phase state (syncMib/working)
	OIDONUChannel        = "1.3.6.1.4.1.37950.1.1.6.1.1.2.1.11" // Channel info
	OIDONUUptime         = "1.3.6.1.4.1.37950.1.1.6.1.1.2.1.12" // Uptime (seconds)

	// ONU Optical Info Table OIDs (1.3.6.1.4.1.37950.1.1.6.1.1.3.1)
	// Format: .3.1.{attr}.{pon_idx}.{onu_idx}
	// Based on real V-SOL V2.1.6R device - returns STRING values with units
	// e.g. "-28.530(dBm)", "47.957(C)", "3.30(V)", "6.220(mA)"
	OIDONUTemperature = "1.3.6.1.4.1.37950.1.1.6.1.1.3.1.3" // Temperature STRING "47.957(C)"
	OIDONUVoltage     = "1.3.6.1.4.1.37950.1.1.6.1.1.3.1.4" // Voltage STRING "3.30(V)"
	OIDONUBiasCurrent = "1.3.6.1.4.1.37950.1.1.6.1.1.3.1.5" // Bias current STRING "6.220(mA)"
	OIDONUTxPower     = "1.3.6.1.4.1.37950.1.1.6.1.1.3.1.6" // TX power STRING "2.520(dBm)"
	OIDONURxPower     = "1.3.6.1.4.1.37950.1.1.6.1.1.3.1.7" // RX power STRING "-28.530(dBm)"
	OIDONUDistance    = "1.3.6.1.4.1.37950.1.1.6.1.1.3.1.8" // Distance in meters (Integer32)

	// ONU Capabilities Table OIDs (1.3.6.1.4.1.37950.1.1.6.1.1.4.1)
	// Format: .4.1.{attr}.{pon_idx}.{onu_idx}
	OIDONUTContNumber    = "1.3.6.1.4.1.37950.1.1.6.1.1.4.1.1" // T-CONT number
	OIDONUGEMPortNumber  = "1.3.6.1.4.1.37950.1.1.6.1.1.4.1.2" // GEM port number
	OIDONUEthUNINumber   = "1.3.6.1.4.1.37950.1.1.6.1.1.4.1.3" // Ethernet UNI ports
	OIDONUPOTSUNINumber  = "1.3.6.1.4.1.37950.1.1.6.1.1.4.1.4" // POTS UNI ports
	OIDONUWiFiUNINumber  = "1.3.6.1.4.1.37950.1.1.6.1.1.4.1.5" // WiFi UNI interfaces
	OIDONUVEIPNumber     = "1.3.6.1.4.1.37950.1.1.6.1.1.4.1.6" // VEIP number

	// ONU Statistics Table OIDs (1.3.6.1.4.1.37950.1.1.6.1.1.5.1)
	// Format: .5.1.{attr}.{pon_idx}.{onu_idx}
	OIDONUUpstreamBytes   = "1.3.6.1.4.1.37950.1.1.6.1.1.5.1.1" // Upstream bytes (Counter64)
	OIDONUDownstreamBytes = "1.3.6.1.4.1.37950.1.1.6.1.1.5.1.2" // Downstream bytes (Counter64)

	// Auto-Find Table OIDs (1.3.6.1.4.1.37950.1.1.6.1.1.6.1)
	// Format: .6.1.{attr}.{pon_idx}.{autofind_idx}
	OIDAutoFindSerial = "1.3.6.1.4.1.37950.1.1.6.1.1.6.1.1" // Auto-discovered ONU serial
	OIDAutoFindState  = "1.3.6.1.4.1.37950.1.1.6.1.1.6.1.2" // Auto-discovered ONU state

	// PON Port Statistics Table OIDs (1.3.6.1.4.1.37950.1.1.6.1.1.18.1)
	// Format: .18.1.{attr}.{pon_idx}
	// Provides aggregate ONU counts per PON port
	OIDPONProvisionedONUs = "1.3.6.1.4.1.37950.1.1.6.1.1.18.1.2" // Provisioned ONU count per PON
	OIDPONOnlineONUs      = "1.3.6.1.4.1.37950.1.1.6.1.1.18.1.3" // Online ONU count per PON

	// PON Port GBIC/SFP Optical OIDs (1.3.6.1.4.1.37950.1.1.5.10.13.1.1)
	// Format: .13.1.1.{attr}.{pon_idx}
	// Values returned as STRING (e.g., "37.016", "6.733")
	OIDGBICTemperature = "1.3.6.1.4.1.37950.1.1.5.10.13.1.1.2" // GBIC temp STRING "37.016"
	OIDGBICTxPower     = "1.3.6.1.4.1.37950.1.1.5.10.13.1.1.5" // GBIC TX power STRING "6.733"

)

// ConvertOpticalPower converts raw SNMP value to dBm
// V-SOL format: value / 1000.0
func ConvertOpticalPower(rawValue int64) float64 {
	if rawValue == common.SNMPInvalidValue || rawValue == 0 {
		return -100.0 // Return very low value for offline
	}
	return float64(rawValue) / 1000.0
}

// ConvertVoltage converts raw SNMP value to Volts
// V-SOL format: value / 100.0
func ConvertVoltage(rawValue int64) float64 {
	if rawValue == common.SNMPInvalidValue || rawValue == 0 {
		return 0.0
	}
	return float64(rawValue) / 100.0
}

// ConvertTemperature converts raw SNMP value to Celsius
// V-SOL format: value / 1000.0
func ConvertTemperature(rawValue int64) float64 {
	if rawValue == common.SNMPInvalidValue || rawValue == 0 {
		return 0.0
	}
	return float64(rawValue) / 1000.0
}

// ConvertBiasCurrent converts raw SNMP value to mA
// V-SOL format: value / 1000.0
func ConvertBiasCurrent(rawValue int64) float64 {
	if rawValue == common.SNMPInvalidValue || rawValue == 0 {
		return 0.0
	}
	return float64(rawValue) / 1000.0
}

// IsOnuOnline checks if ONU is online based on Rx power value
// Returns true if power reading is valid (not invalid value or zero)
func IsOnuOnline(rxPowerRaw int64) bool {
	return common.IsValidSNMPValue(rxPowerRaw)
}

// ParseOpticalString parses V-SOL optical string values like "-28.530(dBm)" or "47.957(C)"
// Returns the numeric value and true if parsing succeeded
func ParseOpticalString(value string) (float64, bool) {
	if value == "" {
		return 0, false
	}
	// Find the opening parenthesis to extract just the number part
	parenIdx := strings.Index(value, "(")
	if parenIdx > 0 {
		value = value[:parenIdx]
	}
	// Parse the numeric value
	f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// ParseOpticalValue parses SNMP optical value - handles both string format (real device)
// and integer format (legacy/simulator). Returns the value in proper units (dBm, C, V, mA).
func ParseOpticalValue(value interface{}, divisor float64) (float64, bool) {
	switch v := value.(type) {
	case string:
		// Real V-SOL device format: "-28.530(dBm)", "47.957(C)", "3.30(V)", "6.220(mA)"
		return ParseOpticalString(v)
	case []byte:
		// gosnmp may return []byte for strings
		return ParseOpticalString(string(v))
	case int, int32, int64, uint, uint32, uint64:
		// Legacy integer format (value * divisor)
		raw, ok := common.ParseIntSNMPValue(value)
		if !ok {
			return 0, false
		}
		if raw == common.SNMPInvalidValue || raw == 0 {
			return -100.0, true // Return low value for offline
		}
		return float64(raw) / divisor, true
	default:
		return 0, false
	}
}

// ParseRxPower parses RX power from SNMP value (string or integer)
func ParseRxPower(value interface{}) (float64, bool) {
	return ParseOpticalValue(value, 1000.0)
}

// ParseTxPower parses TX power from SNMP value (string or integer)
func ParseTxPower(value interface{}) (float64, bool) {
	return ParseOpticalValue(value, 1000.0)
}

// ParseTemperature parses temperature from SNMP value (string or integer)
func ParseTemperature(value interface{}) (float64, bool) {
	return ParseOpticalValue(value, 1000.0)
}

// ParseVoltage parses voltage from SNMP value (string or integer)
func ParseVoltage(value interface{}) (float64, bool) {
	return ParseOpticalValue(value, 100.0)
}

// ParseBiasCurrent parses bias current from SNMP value (string or integer)
func ParseBiasCurrent(value interface{}) (float64, bool) {
	return ParseOpticalValue(value, 1000.0)
}

// ParseDistance parses distance from SNMP value (integer in meters)
func ParseDistance(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	default:
		return 0, false
	}
}

// ParseONUIndex extracts PON port index and ONU ID from SNMP index
// V-SOL format: {pon_idx}.{onu_idx}
// pon_idx: 1-8 maps to ports "0/1" through "0/8"
// onu_idx: 1-128 is the ONU ID
func ParseONUIndex(index string) (ponIdx, onuIdx int, err error) {
	// Strip leading dot if present (gosnmp returns OIDs with leading dots)
	if len(index) > 0 && index[0] == '.' {
		index = index[1:]
	}

	parts := strings.Split(index, ".")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid V-SOL ONU index format (expected 2 components): %s", index)
	}

	ponIdx, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid PON index in %s: %w", index, err)
	}

	onuIdx, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid ONU index in %s: %w", index, err)
	}

	return ponIdx, onuIdx, nil
}

// PONIndexToPort converts SNMP PON port index (1-8) to port string ("0/1" through "0/8")
func PONIndexToPort(ponIdx int) string {
	return fmt.Sprintf("0/%d", ponIdx)
}

// PortToPONIndex converts port string ("0/1" through "0/8") to SNMP PON index (1-8)
func PortToPONIndex(port string) (int, error) {
	// Handle "0/X" format
	if strings.HasPrefix(port, "0/") {
		idxStr := strings.TrimPrefix(port, "0/")
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			return 0, fmt.Errorf("invalid port format: %s", port)
		}
		return idx, nil
	}
	// Try parsing as direct number
	return strconv.Atoi(port)
}

// Aliases for common SNMP helpers (for backward compatibility)
var (
	GetSNMPResult        = common.GetSNMPResult
	ParseNumericSNMPValue = common.ParseIntSNMPValue
	ParseUint64SNMPValue  = common.ParseUint64SNMPValue
	ParseStringSNMPValue  = common.ParseStringSNMPValue
)
