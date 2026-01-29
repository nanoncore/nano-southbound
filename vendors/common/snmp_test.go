package common

import "testing"

func TestGetSNMPResult(t *testing.T) {
	tests := []struct {
		name      string
		results   map[string]interface{}
		oid       string
		wantValue interface{}
		wantFound bool
	}{
		{
			name:      "nil results",
			results:   nil,
			oid:       "1.3.6.1",
			wantValue: nil,
			wantFound: false,
		},
		{
			name:      "empty results",
			results:   map[string]interface{}{},
			oid:       "1.3.6.1",
			wantValue: nil,
			wantFound: false,
		},
		{
			name:      "exact match without dot",
			results:   map[string]interface{}{"1.3.6.1": "value"},
			oid:       "1.3.6.1",
			wantValue: "value",
			wantFound: true,
		},
		{
			name:      "exact match with dot",
			results:   map[string]interface{}{".1.3.6.1": "value"},
			oid:       ".1.3.6.1",
			wantValue: "value",
			wantFound: true,
		},
		{
			name:      "result has dot, oid without",
			results:   map[string]interface{}{".1.3.6.1": "value"},
			oid:       "1.3.6.1",
			wantValue: "value",
			wantFound: true,
		},
		{
			name:      "result without dot, oid has dot",
			results:   map[string]interface{}{"1.3.6.1": "value"},
			oid:       ".1.3.6.1",
			wantValue: "value",
			wantFound: true,
		},
		{
			name:      "not found",
			results:   map[string]interface{}{"1.3.6.1": "value"},
			oid:       "1.3.6.2",
			wantValue: nil,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotFound := GetSNMPResult(tt.results, tt.oid)
			if gotValue != tt.wantValue {
				t.Errorf("GetSNMPResult() value = %v, want %v", gotValue, tt.wantValue)
			}
			if gotFound != tt.wantFound {
				t.Errorf("GetSNMPResult() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

func TestParseNumericSNMPValue(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		wantValue float64
		wantOK    bool
	}{
		{name: "nil", value: nil, wantValue: 0, wantOK: false},
		{name: "int", value: int(42), wantValue: 42, wantOK: true},
		{name: "int64", value: int64(123), wantValue: 123, wantOK: true},
		{name: "uint", value: uint(100), wantValue: 100, wantOK: true},
		{name: "uint64", value: uint64(999), wantValue: 999, wantOK: true},
		{name: "float32", value: float32(3.14), wantValue: float64(float32(3.14)), wantOK: true},
		{name: "float64", value: float64(2.718), wantValue: 2.718, wantOK: true},
		{name: "string", value: "not a number", wantValue: 0, wantOK: false},
		{name: "negative int", value: int(-5), wantValue: -5, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOK := ParseNumericSNMPValue(tt.value)
			if gotOK != tt.wantOK {
				t.Errorf("ParseNumericSNMPValue() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotOK && gotValue != tt.wantValue {
				t.Errorf("ParseNumericSNMPValue() value = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestParseIntSNMPValue(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		wantValue int64
		wantOK    bool
	}{
		{name: "nil", value: nil, wantValue: 0, wantOK: false},
		{name: "int", value: int(42), wantValue: 42, wantOK: true},
		{name: "int64", value: int64(123), wantValue: 123, wantOK: true},
		{name: "uint32", value: uint32(100), wantValue: 100, wantOK: true},
		{name: "float64", value: float64(99.9), wantValue: 99, wantOK: true},
		{name: "string", value: "invalid", wantValue: 0, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOK := ParseIntSNMPValue(tt.value)
			if gotOK != tt.wantOK {
				t.Errorf("ParseIntSNMPValue() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotOK && gotValue != tt.wantValue {
				t.Errorf("ParseIntSNMPValue() value = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestParseUint64SNMPValue(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		wantValue uint64
		wantOK    bool
	}{
		{name: "nil", value: nil, wantValue: 0, wantOK: false},
		{name: "uint64", value: uint64(12345), wantValue: 12345, wantOK: true},
		{name: "uint32", value: uint32(999), wantValue: 999, wantOK: true},
		{name: "int positive", value: int(50), wantValue: 50, wantOK: true},
		{name: "int negative", value: int(-5), wantValue: 0, wantOK: false},
		{name: "int64 negative", value: int64(-100), wantValue: 0, wantOK: false},
		{name: "float64 positive", value: float64(123.45), wantValue: 123, wantOK: true},
		{name: "float64 negative", value: float64(-1.5), wantValue: 0, wantOK: false},
		{name: "string", value: "invalid", wantValue: 0, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOK := ParseUint64SNMPValue(tt.value)
			if gotOK != tt.wantOK {
				t.Errorf("ParseUint64SNMPValue() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotOK && gotValue != tt.wantValue {
				t.Errorf("ParseUint64SNMPValue() value = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestParseStringSNMPValue(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		wantValue string
		wantOK    bool
	}{
		{name: "nil", value: nil, wantValue: "", wantOK: false},
		{name: "string", value: "hello", wantValue: "hello", wantOK: true},
		{name: "empty string", value: "", wantValue: "", wantOK: true},
		{name: "byte slice", value: []byte("world"), wantValue: "world", wantOK: true},
		{name: "int", value: int(123), wantValue: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOK := ParseStringSNMPValue(tt.value)
			if gotOK != tt.wantOK {
				t.Errorf("ParseStringSNMPValue() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotValue != tt.wantValue {
				t.Errorf("ParseStringSNMPValue() value = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestIsValidSNMPValue(t *testing.T) {
	tests := []struct {
		name  string
		value int64
		want  bool
	}{
		{name: "valid positive", value: 100, want: true},
		{name: "valid negative", value: -28, want: true},
		{name: "zero is invalid", value: 0, want: false},
		{name: "invalid marker", value: SNMPInvalidValue, want: false},
		{name: "large valid", value: 2147483646, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidSNMPValue(tt.value)
			if got != tt.want {
				t.Errorf("IsValidSNMPValue(%d) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
