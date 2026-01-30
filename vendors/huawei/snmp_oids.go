package huawei

import (
	"fmt"
	"strings"

	"github.com/nanoncore/nano-southbound/vendors/common"
)

// Huawei GPON MIB OIDs
// Based on legacy production code and Huawei documentation
// Reference: https://ixnfo.com/oid-i-mib-dlya-huawei-olt-i-onu.html

const (
	// Enterprise OID prefix for Huawei
	OIDHuaweiEnterprise = "1.3.6.1.4.1.2011"

	// Standard MIB-II System OIDs (RFC 1213)
	OIDSysDescr  = "1.3.6.1.2.1.1.1.0" // System description
	OIDSysUpTime = "1.3.6.1.2.1.1.3.0" // System uptime in hundredths of seconds
	OIDSysName   = "1.3.6.1.2.1.1.5.0" // System name

	// Huawei SmartAX System Telemetry OIDs
	// These are the OIDs used by SmartAX MA5600T/MA5800-X series for system metrics
	OIDSmartAXCPU         = "1.3.6.1.4.1.2011.6.128.1.1.2.98.1.1.1.1" // CPU utilization %
	OIDSmartAXMemory      = "1.3.6.1.4.1.2011.6.128.1.1.2.98.1.2.1.1" // Memory utilization %
	OIDSmartAXTemperature = "1.3.6.1.4.1.2011.2.6.7.1.1.2.1.10"       // Board temperature in Celsius

	// ONU Serial Number - returns hex-encoded serial (e.g., "485754430011D168" = HWTC0011D168)
	// Index: <portIndex>.<onuIndex>
	OIDOnuSerialNumber = "1.3.6.1.4.1.2011.6.128.1.1.2.43.1.3"

	// ONU Optical Parameters (1.3.6.1.4.1.2011.6.128.1.1.2.51.1.x)
	// Index: <portIndex>.<onuIndex>
	// Note: Value 2147483647 (0x7FFFFFFF) indicates offline/invalid
	OIDOnuTemperature = "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.1" // Temperature (value / 256 = °C)
	OIDOnuCurrent     = "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.2" // Bias current in uA
	OIDOnuTxPower     = "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.3" // Tx power (value * 0.01 dBm)
	OIDOnuRxPower     = "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.4" // Rx power (value * 0.01 dBm)
	OIDOnuVoltage     = "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.5" // Voltage (value * 0.001 V)
	OIDOltRxPower     = "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.6" // OLT Rx from ONU ((value-10000)*0.01 dBm)
	OIDOnuCatvRx      = "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.7" // CATV Rx power

	// ONU Distance (from ONU info table 1.3.6.1.4.1.2011.6.128.1.1.2.43.1.x)
	OIDOnuDistance = "1.3.6.1.4.1.2011.6.128.1.1.2.43.1.12" // Distance in meters

	// OLT PON Port Optical Parameters (1.3.6.1.4.1.2011.6.128.1.1.2.23.1.x)
	OIDOltPonTemperature = "1.3.6.1.4.1.2011.6.128.1.1.2.23.1.1"
	OIDOltPonVoltage     = "1.3.6.1.4.1.2011.6.128.1.1.2.23.1.2"
	OIDOltPonCurrent     = "1.3.6.1.4.1.2011.6.128.1.1.2.23.1.3"
	OIDOltPonTxPower     = "1.3.6.1.4.1.2011.6.128.1.1.2.23.1.4"

	// ONU Traffic Statistics (Counter64)
	// Index: <portIndex>.<onuIndex>
	OIDOnuUpBytes   = "1.3.6.1.4.1.2011.6.128.1.1.4.23.1.3" // Upstream bytes
	OIDOnuDownBytes = "1.3.6.1.4.1.2011.6.128.1.1.4.23.1.4" // Downstream bytes

	// OLT Card Resources
	OIDCardCPU    = "1.3.6.1.4.1.2011.2.6.7.1.1.2.1.5" // CPU utilization %
	OIDCardMemory = "1.3.6.1.4.1.2011.2.6.7.1.1.2.1.6" // Memory utilization %

	// Standard MIB-II Interface Counters
	OIDIfDescr       = "1.3.6.1.2.1.2.2.1.2"     // Interface description
	OIDIfAdminStatus = "1.3.6.1.2.1.2.2.1.7"     // Admin status (1=up, 2=down, 3=testing)
	OIDIfOperStatus  = "1.3.6.1.2.1.2.2.1.8"     // Oper status (1=up, 2=down, 3=testing, etc.)
	OIDIfInOctets    = "1.3.6.1.2.1.2.2.1.10"    // 32-bit input bytes
	OIDIfOutOctets   = "1.3.6.1.2.1.2.2.1.16"    // 32-bit output bytes
	OIDIfHCInOctets  = "1.3.6.1.2.1.31.1.1.1.6"  // 64-bit input bytes
	OIDIfHCOutOctets = "1.3.6.1.2.1.31.1.1.1.10" // 64-bit output bytes
	OIDIfAlias       = "1.3.6.1.2.1.31.1.1.1.1"  // Interface alias (PON port description)

)

// ConvertOpticalPower converts raw SNMP value to dBm
// Formula: value * 0.01
func ConvertOpticalPower(rawValue int64) float64 {
	if rawValue == common.SNMPInvalidValue {
		return -100.0 // Return very low value for offline
	}
	return float64(rawValue) * 0.01
}

// ConvertOltRxPower converts OLT Rx power value to dBm
// Formula: (value - 10000) * 0.01
func ConvertOltRxPower(rawValue int64) float64 {
	if rawValue == common.SNMPInvalidValue {
		return -100.0
	}
	return float64(rawValue-10000) * 0.01
}

// ConvertVoltage converts raw SNMP value to Volts
// Formula: value * 0.001
func ConvertVoltage(rawValue int64) float64 {
	if rawValue == common.SNMPInvalidValue {
		return 0.0
	}
	return float64(rawValue) * 0.001
}

// ConvertTemperature converts raw SNMP value to Celsius
// Formula: value / 256
func ConvertTemperature(rawValue int64) float64 {
	if rawValue == common.SNMPInvalidValue || rawValue == 0 {
		return 0.0
	}
	return float64(rawValue) / 256.0
}

// IsOnuOnline checks if ONU is online based on Rx power value
// Huawei returns 2147483647 when ONU is offline
func IsOnuOnline(rxPowerRaw int64) bool {
	return rxPowerRaw != common.SNMPInvalidValue
}

// DecodeHexSerial converts hex serial number to readable format
// Handles both hex-encoded serials (e.g., "485754430011D168" -> "HWTC0011D168")
// and already-ASCII serials (e.g., "HWTC00000101" -> "HWTC00000101")
func DecodeHexSerial(hexSerial string) string {
	if len(hexSerial) < 8 {
		return hexSerial
	}

	// Check if serial is already in ASCII format (starts with letters like "HWTC", "ZTEG", etc.)
	// ASCII ONU serials typically have 4 letter prefix followed by numbers
	if isASCIISerial(hexSerial) {
		return hexSerial
	}

	// First 8 chars are vendor ID in hex (ASCII)
	vendorHex := hexSerial[:8]
	vendorID := ""
	for i := 0; i < 8; i += 2 {
		if i+2 <= len(vendorHex) {
			b := hexToByte(vendorHex[i : i+2])
			if b >= 32 && b <= 126 { // Printable ASCII
				vendorID += string(rune(b))
			}
		}
	}

	// Rest is the serial number
	serialPart := hexSerial[8:]

	return vendorID + serialPart
}

// isASCIISerial checks if the serial is already in ASCII format (not hex-encoded)
func isASCIISerial(serial string) bool {
	if len(serial) < 4 {
		return false
	}
	// Check if first 4 chars are uppercase letters (typical ONU vendor ID)
	for i := 0; i < 4; i++ {
		c := serial[i]
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

func hexToByte(hex string) byte {
	var b byte
	for _, c := range hex {
		b <<= 4
		switch {
		case c >= '0' && c <= '9':
			b |= byte(c - '0')
		case c >= 'A' && c <= 'F':
			b |= byte(c - 'A' + 10)
		case c >= 'a' && c <= 'f':
			b |= byte(c - 'a' + 10)
		}
	}
	return b
}

// ParseONUIndex extracts frame, slot, port, and ONU ID from SNMP index
// Huawei uses portIndex.onuIndex format where portIndex encodes slot/port
// Supports both 2-component (portIndex.onuIndex) and 3-component (frame.portIndex.onuIndex) formats
func ParseONUIndex(index string) (frame, slot, port, onuID int, err error) {
	// Strip leading dot if present (gosnmp returns OIDs with leading dots)
	if len(index) > 0 && index[0] == '.' {
		index = index[1:]
	}

	parts := strings.Split(index, ".")

	switch len(parts) {
	case 3:
		// "frame.portIndex.onuIndex" format from simulator
		var portIndex int
		if _, err := fmt.Sscanf(index, "%d.%d.%d", &frame, &portIndex, &onuID); err != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid 3-component ONU index format: %s", index)
		}
		// Decode portIndex to get slot and port
		slot = (portIndex >> 8) & 0xFF
		port = portIndex & 0xFF

	case 2:
		// "portIndex.onuIndex" format (existing logic for real OLTs)
		var portIndex int
		if _, err := fmt.Sscanf(index, "%d.%d", &portIndex, &onuID); err != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid 2-component ONU index format: %s", index)
		}
		// Decode portIndex to frame/slot/port
		frame = (portIndex >> 16) & 0xFF
		slot = (portIndex >> 8) & 0xFF
		port = portIndex & 0xFF

	default:
		return 0, 0, 0, 0, fmt.Errorf("invalid ONU index format (expected 2 or 3 components): %s", index)
	}

	return frame, slot, port, onuID, nil
}

// Aliases for common SNMP helpers (for backward compatibility)
var (
	GetSNMPResult         = common.GetSNMPResult
	ParseNumericSNMPValue = common.ParseNumericSNMPValue
)
