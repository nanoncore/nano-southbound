package vsol

import (
	"math"
	"testing"

	"github.com/nanoncore/nano-southbound/vendors/common"
)

// =============================================================================
// ConvertOpticalPower Tests
// =============================================================================

func TestConvertOpticalPower(t *testing.T) {
	tests := []struct {
		name     string
		rawValue int64
		want     float64
	}{
		{"normal negative value", -28530, -28.530},
		{"SNMPInvalidValue returns -100", common.SNMPInvalidValue, -100.0},
		{"zero returns -100", 0, -100.0},
		{"positive value", 2520, 2.520},
		{"small negative", -500, -0.5},
		{"large negative", -50000, -50.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertOpticalPower(tt.rawValue)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("ConvertOpticalPower(%d) = %v, want %v", tt.rawValue, got, tt.want)
			}
		})
	}
}

// =============================================================================
// ConvertVoltage Tests
// =============================================================================

func TestConvertVoltage(t *testing.T) {
	tests := []struct {
		name     string
		rawValue int64
		want     float64
	}{
		{"normal value 330 -> 3.3V", 330, 3.3},
		{"SNMPInvalidValue returns 0", common.SNMPInvalidValue, 0.0},
		{"zero returns 0", 0, 0.0},
		{"small value", 100, 1.0},
		{"large value", 500, 5.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertVoltage(tt.rawValue)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("ConvertVoltage(%d) = %v, want %v", tt.rawValue, got, tt.want)
			}
		})
	}
}

// =============================================================================
// ConvertTemperature Tests
// =============================================================================

func TestConvertTemperature(t *testing.T) {
	tests := []struct {
		name     string
		rawValue int64
		want     float64
	}{
		{"normal value 47957 -> 47.957C", 47957, 47.957},
		{"SNMPInvalidValue returns 0", common.SNMPInvalidValue, 0.0},
		{"zero returns 0", 0, 0.0},
		{"small value", 1000, 1.0},
		{"large value", 85000, 85.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertTemperature(tt.rawValue)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("ConvertTemperature(%d) = %v, want %v", tt.rawValue, got, tt.want)
			}
		})
	}
}

// =============================================================================
// ConvertBiasCurrent Tests
// =============================================================================

func TestConvertBiasCurrent(t *testing.T) {
	tests := []struct {
		name     string
		rawValue int64
		want     float64
	}{
		{"normal value 6220 -> 6.22mA", 6220, 6.22},
		{"SNMPInvalidValue returns 0", common.SNMPInvalidValue, 0.0},
		{"zero returns 0", 0, 0.0},
		{"small value", 100, 0.1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertBiasCurrent(tt.rawValue)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("ConvertBiasCurrent(%d) = %v, want %v", tt.rawValue, got, tt.want)
			}
		})
	}
}

// =============================================================================
// IsOnuOnline Tests
// =============================================================================

func TestIsOnuOnline(t *testing.T) {
	tests := []struct {
		name       string
		rxPowerRaw int64
		want       bool
	}{
		{"valid power reading", -28530, true},
		{"zero is offline", 0, false},
		{"SNMPInvalidValue is offline", common.SNMPInvalidValue, false},
		{"positive value is online", 2520, true},
		{"negative value is online", -1000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsOnuOnline(tt.rxPowerRaw)
			if got != tt.want {
				t.Errorf("IsOnuOnline(%d) = %v, want %v", tt.rxPowerRaw, got, tt.want)
			}
		})
	}
}

// =============================================================================
// ParseOpticalString Tests
// =============================================================================

func TestParseOpticalString(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantValue float64
		wantOK    bool
	}{
		{"temperature with unit", "47.957(C)", 47.957, true},
		{"negative rx power", "-28.530(dBm)", -28.530, true},
		{"voltage", "3.30(V)", 3.30, true},
		{"bias current", "6.220(mA)", 6.220, true},
		{"empty string", "", 0, false},
		{"no parens - valid number", "42.5", 42.5, true},
		{"invalid non-numeric", "abc", 0, false},
		{"zero value with unit", "0.000(dBm)", 0.0, true},
		{"positive tx power", "2.520(dBm)", 2.520, true},
		{"negative with spaces", " -10.5(dBm)", -10.5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOK := ParseOpticalString(tt.value)
			if gotOK != tt.wantOK {
				t.Errorf("ParseOpticalString(%q) ok = %v, want %v", tt.value, gotOK, tt.wantOK)
				return
			}
			if gotOK && math.Abs(gotValue-tt.wantValue) > 0.001 {
				t.Errorf("ParseOpticalString(%q) = %v, want %v", tt.value, gotValue, tt.wantValue)
			}
		})
	}
}

// =============================================================================
// ParseOpticalValue Tests
// =============================================================================

func TestParseOpticalValue(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		divisor   float64
		wantValue float64
		wantOK    bool
	}{
		{"string voltage", "3.30(V)", 100.0, 3.30, true},
		{"string rx power", "-28.530(dBm)", 1000.0, -28.530, true},
		{"byte slice", []byte("47.957(C)"), 1000.0, 47.957, true},
		{"int64 normal value", int64(-28530), 1000.0, -28.530, true},
		{"int64 SNMPInvalidValue", int64(common.SNMPInvalidValue), 1000.0, -100.0, true},
		{"int64 zero", int64(0), 1000.0, -100.0, true},
		{"int value", int(5000), 1000.0, 5.0, true},
		{"nil value", nil, 1000.0, 0, false},
		{"unsupported type bool", true, 1000.0, 0, false},
		{"empty string", "", 1000.0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOK := ParseOpticalValue(tt.value, tt.divisor)
			if gotOK != tt.wantOK {
				t.Errorf("ParseOpticalValue(%v, %v) ok = %v, want %v", tt.value, tt.divisor, gotOK, tt.wantOK)
				return
			}
			if gotOK && math.Abs(gotValue-tt.wantValue) > 0.001 {
				t.Errorf("ParseOpticalValue(%v, %v) = %v, want %v", tt.value, tt.divisor, gotValue, tt.wantValue)
			}
		})
	}
}

// =============================================================================
// ParseRxPower, ParseTxPower, ParseTemperature, ParseVoltage, ParseBiasCurrent
// =============================================================================

func TestParseRxPower(t *testing.T) {
	val, ok := ParseRxPower("-28.530(dBm)")
	if !ok || math.Abs(val-(-28.530)) > 0.001 {
		t.Errorf("ParseRxPower string: got (%v, %v)", val, ok)
	}

	val, ok = ParseRxPower(int64(-28530))
	if !ok || math.Abs(val-(-28.530)) > 0.001 {
		t.Errorf("ParseRxPower int64: got (%v, %v)", val, ok)
	}

	_, ok = ParseRxPower(nil)
	if ok {
		t.Error("ParseRxPower nil should return false")
	}
}

func TestParseTxPower(t *testing.T) {
	val, ok := ParseTxPower("2.520(dBm)")
	if !ok || math.Abs(val-2.520) > 0.001 {
		t.Errorf("ParseTxPower string: got (%v, %v)", val, ok)
	}

	val, ok = ParseTxPower(int64(2520))
	if !ok || math.Abs(val-2.520) > 0.001 {
		t.Errorf("ParseTxPower int64: got (%v, %v)", val, ok)
	}
}

func TestParseTemperatureFunc(t *testing.T) {
	val, ok := ParseTemperature("47.957(C)")
	if !ok || math.Abs(val-47.957) > 0.001 {
		t.Errorf("ParseTemperature string: got (%v, %v)", val, ok)
	}

	val, ok = ParseTemperature(int64(47957))
	if !ok || math.Abs(val-47.957) > 0.001 {
		t.Errorf("ParseTemperature int64: got (%v, %v)", val, ok)
	}
}

func TestParseVoltageFunc(t *testing.T) {
	val, ok := ParseVoltage("3.30(V)")
	if !ok || math.Abs(val-3.30) > 0.001 {
		t.Errorf("ParseVoltage string: got (%v, %v)", val, ok)
	}

	// Voltage uses divisor 100
	val, ok = ParseVoltage(int64(330))
	if !ok || math.Abs(val-3.30) > 0.001 {
		t.Errorf("ParseVoltage int64: got (%v, %v)", val, ok)
	}
}

func TestParseBiasCurrentFunc(t *testing.T) {
	val, ok := ParseBiasCurrent("6.220(mA)")
	if !ok || math.Abs(val-6.220) > 0.001 {
		t.Errorf("ParseBiasCurrent string: got (%v, %v)", val, ok)
	}

	val, ok = ParseBiasCurrent(int64(6220))
	if !ok || math.Abs(val-6.220) > 0.001 {
		t.Errorf("ParseBiasCurrent int64: got (%v, %v)", val, ok)
	}
}

// =============================================================================
// ParseDistance Tests
// =============================================================================

func TestParseDistance(t *testing.T) {
	tests := []struct {
		name   string
		value  interface{}
		wantDist int
		wantOK   bool
	}{
		{"int value", int(1234), 1234, true},
		{"int32 value", int32(567), 567, true},
		{"int64 value", int64(890), 890, true},
		{"uint value", uint(1000), 1000, true},
		{"uint32 value", uint32(2000), 2000, true},
		{"uint64 value", uint64(3000), 3000, true},
		{"string not supported", "1234", 0, false},
		{"nil not supported", nil, 0, false},
		{"bool not supported", true, 0, false},
		{"float not supported", float64(1.5), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDist, gotOK := ParseDistance(tt.value)
			if gotOK != tt.wantOK {
				t.Errorf("ParseDistance(%v) ok = %v, want %v", tt.value, gotOK, tt.wantOK)
				return
			}
			if gotDist != tt.wantDist {
				t.Errorf("ParseDistance(%v) = %d, want %d", tt.value, gotDist, tt.wantDist)
			}
		})
	}
}

// =============================================================================
// ParseONUIndex Tests
// =============================================================================

func TestParseONUIndex(t *testing.T) {
	tests := []struct {
		name       string
		index      string
		wantPON    int
		wantONU    int
		wantErr    bool
	}{
		{"with leading dot", ".1.6", 1, 6, false},
		{"without leading dot", "1.6", 1, 6, false},
		{"port 8 onu 128", "8.128", 8, 128, false},
		{"only one component", ".1", 0, 0, true},
		{"non-numeric pon", "abc.def", 0, 0, true},
		{"non-numeric onu", "1.def", 0, 0, true},
		{"empty string", "", 0, 0, true},
		{"three components", "1.2.3", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ponIdx, onuIdx, err := ParseONUIndex(tt.index)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseONUIndex(%q) error = %v, wantErr %v", tt.index, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if ponIdx != tt.wantPON {
					t.Errorf("ParseONUIndex(%q) ponIdx = %d, want %d", tt.index, ponIdx, tt.wantPON)
				}
				if onuIdx != tt.wantONU {
					t.Errorf("ParseONUIndex(%q) onuIdx = %d, want %d", tt.index, onuIdx, tt.wantONU)
				}
			}
		})
	}
}

// =============================================================================
// ParseONUVLANIndex Tests
// =============================================================================

func TestParseONUVLANIndex(t *testing.T) {
	tests := []struct {
		name    string
		index   string
		wantPON int
		wantONU int
		wantGEM int
		wantErr bool
	}{
		{"with leading dot", ".1.6.1", 1, 6, 1, false},
		{"without leading dot", "1.6.1", 1, 6, 1, false},
		{"high values", "8.128.4", 8, 128, 4, false},
		{"only two components", ".1.6", 0, 0, 0, true},
		{"only one component", "1", 0, 0, 0, true},
		{"non-numeric", "abc.def.ghi", 0, 0, 0, true},
		{"empty string", "", 0, 0, 0, true},
		{"four components", "1.2.3.4", 0, 0, 0, true},
		{"non-numeric gem", "1.6.abc", 0, 0, 0, true},
		{"non-numeric onu", "1.abc.1", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ponIdx, onuIdx, gemIdx, err := ParseONUVLANIndex(tt.index)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseONUVLANIndex(%q) error = %v, wantErr %v", tt.index, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if ponIdx != tt.wantPON {
					t.Errorf("ParseONUVLANIndex(%q) ponIdx = %d, want %d", tt.index, ponIdx, tt.wantPON)
				}
				if onuIdx != tt.wantONU {
					t.Errorf("ParseONUVLANIndex(%q) onuIdx = %d, want %d", tt.index, onuIdx, tt.wantONU)
				}
				if gemIdx != tt.wantGEM {
					t.Errorf("ParseONUVLANIndex(%q) gemIdx = %d, want %d", tt.index, gemIdx, tt.wantGEM)
				}
			}
		})
	}
}

// =============================================================================
// PONIndexToPort Tests
// =============================================================================

func TestPONIndexToPort(t *testing.T) {
	tests := []struct {
		name    string
		ponIdx  int
		want    string
	}{
		{"port 1", 1, "0/1"},
		{"port 8", 8, "0/8"},
		{"port 4", 4, "0/4"},
		{"port 0", 0, "0/0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PONIndexToPort(tt.ponIdx)
			if got != tt.want {
				t.Errorf("PONIndexToPort(%d) = %q, want %q", tt.ponIdx, got, tt.want)
			}
		})
	}
}

// =============================================================================
// PortToPONIndex Tests
// =============================================================================

func TestPortToPONIndex(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		want    int
		wantErr bool
	}{
		{"standard port 0/1", "0/1", 1, false},
		{"standard port 0/8", "0/8", 8, false},
		{"standard port 0/4", "0/4", 4, false},
		{"bare number 3", "3", 3, false},
		{"bare number 1", "1", 1, false},
		{"invalid format", "abc", 0, true},
		{"invalid after 0/", "0/abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PortToPONIndex(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("PortToPONIndex(%q) error = %v, wantErr %v", tt.port, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("PortToPONIndex(%q) = %d, want %d", tt.port, got, tt.want)
			}
		})
	}
}
