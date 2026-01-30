package common

import "strings"

// SNMPInvalidValue is the magic value indicating offline/invalid SNMP reading.
// Used by Huawei, V-SOL, and other vendors to indicate the ONU is offline
// or the value could not be read.
const SNMPInvalidValue int64 = 2147483647

// GetSNMPResult looks up an OID in SNMP results, handling the leading dot issue.
// gosnmp returns OIDs with a leading dot (e.g., ".1.3.6.1..."), but OID constants
// typically don't have the leading dot. This function tries both formats.
func GetSNMPResult(results map[string]interface{}, oid string) (interface{}, bool) {
	if results == nil {
		return nil, false
	}

	// Try with leading dot first (gosnmp format)
	if !strings.HasPrefix(oid, ".") {
		if val, ok := results["."+oid]; ok {
			return val, true
		}
	}

	// Try exact match
	if val, ok := results[oid]; ok {
		return val, true
	}

	// Try without leading dot
	if strings.HasPrefix(oid, ".") {
		if val, ok := results[strings.TrimPrefix(oid, ".")]; ok {
			return val, true
		}
	}

	return nil, false
}

// ParseNumericSNMPValue extracts a float64 from various numeric types
// that SNMP libraries may return (int, int64, uint, uint64, etc.)
func ParseNumericSNMPValue(value interface{}) (float64, bool) {
	if value == nil {
		return 0, false
	}

	switch v := value.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	default:
		return 0, false
	}
}

// ParseIntSNMPValue extracts an int64 from various numeric types.
func ParseIntSNMPValue(value interface{}) (int64, bool) {
	if value == nil {
		return 0, false
	}

	switch v := value.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		return int64(v), true
	case float32:
		return int64(v), true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}

// ParseUint64SNMPValue extracts a uint64 from SNMP counter values.
func ParseUint64SNMPValue(value interface{}) (uint64, bool) {
	if value == nil {
		return 0, false
	}

	switch v := value.(type) {
	case int:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int8:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int16:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int32:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case uint:
		return uint64(v), true
	case uint8:
		return uint64(v), true
	case uint16:
		return uint64(v), true
	case uint32:
		return uint64(v), true
	case uint64:
		return v, true
	case float32:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case float64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	default:
		return 0, false
	}
}

// ParseStringSNMPValue extracts a string from SNMP result.
// Handles both string and []byte types.
func ParseStringSNMPValue(value interface{}) (string, bool) {
	if value == nil {
		return "", false
	}

	switch v := value.(type) {
	case string:
		return v, true
	case []byte:
		return string(v), true
	default:
		return "", false
	}
}

// IsValidSNMPValue checks if a raw SNMP value is valid (not the invalid marker).
func IsValidSNMPValue(value int64) bool {
	return value != SNMPInvalidValue && value != 0
}
