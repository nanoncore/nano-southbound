package huawei

import "fmt"

// Huawei GPON MIB OIDs
// Based on legacy production code and Huawei documentation
// Reference: https://ixnfo.com/oid-i-mib-dlya-huawei-olt-i-onu.html

const (
	// Enterprise OID prefix for Huawei
	OIDHuaweiEnterprise = "1.3.6.1.4.1.2011"

	// ONU Serial Number - returns hex-encoded serial (e.g., "485754430011D168" = HWTC0011D168)
	// Index: <portIndex>.<onuIndex>
	OIDOnuSerialNumber = "1.3.6.1.4.1.2011.6.128.1.1.2.43.1.3"

	// ONU Optical Parameters (1.3.6.1.4.1.2011.6.128.1.1.2.51.1.x)
	// Index: <portIndex>.<onuIndex>
	// Note: Value 2147483647 (0x7FFFFFFF) indicates offline/invalid
	OIDOnuTemperature = "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.1" // Temperature in Celsius
	OIDOnuCurrent     = "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.2" // Bias current in uA
	OIDOnuTxPower     = "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.3" // Tx power (value * 0.01 dBm)
	OIDOnuRxPower     = "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.4" // Rx power (value * 0.01 dBm)
	OIDOnuVoltage     = "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.5" // Voltage (value * 0.001 V)
	OIDOltRxPower     = "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.6" // OLT Rx from ONU ((value-10000)*0.01 dBm)
	OIDOnuCatvRx      = "1.3.6.1.4.1.2011.6.128.1.1.2.51.1.7" // CATV Rx power

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
	OIDIfInOctets    = "1.3.6.1.2.1.2.2.1.10"    // 32-bit input bytes
	OIDIfOutOctets   = "1.3.6.1.2.1.2.2.1.16"    // 32-bit output bytes
	OIDIfHCInOctets  = "1.3.6.1.2.1.31.1.1.1.6"  // 64-bit input bytes
	OIDIfHCOutOctets = "1.3.6.1.2.1.31.1.1.1.10" // 64-bit output bytes
	OIDIfAlias       = "1.3.6.1.2.1.31.1.1.1.1"  // Interface alias (PON port description)

	// Magic value indicating offline/invalid reading
	SNMPInvalidValue = 2147483647
)

// ConvertOpticalPower converts raw SNMP value to dBm
// Formula: value * 0.01
func ConvertOpticalPower(rawValue int64) float64 {
	if rawValue == SNMPInvalidValue {
		return -100.0 // Return very low value for offline
	}
	return float64(rawValue) * 0.01
}

// ConvertOltRxPower converts OLT Rx power value to dBm
// Formula: (value - 10000) * 0.01
func ConvertOltRxPower(rawValue int64) float64 {
	if rawValue == SNMPInvalidValue {
		return -100.0
	}
	return float64(rawValue-10000) * 0.01
}

// ConvertVoltage converts raw SNMP value to Volts
// Formula: value * 0.001
func ConvertVoltage(rawValue int64) float64 {
	if rawValue == SNMPInvalidValue {
		return 0.0
	}
	return float64(rawValue) * 0.001
}

// IsOnuOnline checks if ONU is online based on Rx power value
// Huawei returns 2147483647 when ONU is offline
func IsOnuOnline(rxPowerRaw int64) bool {
	return rxPowerRaw != SNMPInvalidValue
}

// DecodeHexSerial converts hex serial number to readable format
// Input: "485754430011D168" -> Output: "HWTC0011D168"
func DecodeHexSerial(hexSerial string) string {
	if len(hexSerial) < 8 {
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

// ParseONUIndex extracts slot, port, and ONU ID from SNMP index
// Huawei uses portIndex.onuIndex format where portIndex encodes slot/port
func ParseONUIndex(index string) (slot, port, onuID int) {
	// Index format: <portIndex>.<onuIndex>
	// portIndex is typically calculated from frame/slot/port
	// This is vendor-specific and may vary by model

	// Simple parsing - assumes format like "portIndex.onuId"
	var portIndex int
	_, _ = fmt.Sscanf(index, "%d.%d", &portIndex, &onuID)

	// Decode portIndex to slot/port (this varies by OLT model)
	// Common formula: portIndex = (slot * 256) + port
	// Or for some models: portIndex = (frame * 65536) + (slot * 256) + port
	slot = (portIndex >> 8) & 0xFF
	port = portIndex & 0xFF

	return
}
