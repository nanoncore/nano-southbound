package huawei

import "testing"

func TestParseONUIndex(t *testing.T) {
	tests := []struct {
		name      string
		index     string
		wantFrame int
		wantSlot  int
		wantPort  int
		wantOnuID int
		wantErr   bool
	}{
		{
			name:      "3-component format: 0.1.0 (frame=0, portIndex=1, onuID=0)",
			index:     "0.1.0",
			wantFrame: 0,
			wantSlot:  0,
			wantPort:  1,
			wantOnuID: 0,
			wantErr:   false,
		},
		{
			name:      "3-component format: 0.2.5 (frame=0, portIndex=2, onuID=5)",
			index:     "0.2.5",
			wantFrame: 0,
			wantSlot:  0,
			wantPort:  2,
			wantOnuID: 5,
			wantErr:   false,
		},
		{
			name:      "3-component format with leading dot",
			index:     ".0.1.0",
			wantFrame: 0,
			wantSlot:  0,
			wantPort:  1,
			wantOnuID: 0,
			wantErr:   false,
		},
		{
			name:      "2-component format: 1.0 (portIndex=1, onuID=0)",
			index:     "1.0",
			wantFrame: 0,
			wantSlot:  0,
			wantPort:  1,
			wantOnuID: 0,
			wantErr:   false,
		},
		{
			name:      "2-component format: encoded portIndex (slot=1, port=2)",
			index:     "258.3", // portIndex = (1 << 8) | 2 = 258
			wantFrame: 0,
			wantSlot:  1,
			wantPort:  2,
			wantOnuID: 3,
			wantErr:   false,
		},
		{
			name:      "2-component format: encoded portIndex with frame",
			index:     "65794.7", // portIndex = (1 << 16) | (1 << 8) | 2 = 65794
			wantFrame: 1,
			wantSlot:  1,
			wantPort:  2,
			wantOnuID: 7,
			wantErr:   false,
		},
		{
			name:    "invalid: single component",
			index:   "123",
			wantErr: true,
		},
		{
			name:    "invalid: four components",
			index:   "0.1.2.3",
			wantErr: true,
		},
		{
			name:    "invalid: empty string",
			index:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, slot, port, onuID, err := ParseONUIndex(tt.index)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseONUIndex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if frame != tt.wantFrame {
				t.Errorf("ParseONUIndex() frame = %v, want %v", frame, tt.wantFrame)
			}
			if slot != tt.wantSlot {
				t.Errorf("ParseONUIndex() slot = %v, want %v", slot, tt.wantSlot)
			}
			if port != tt.wantPort {
				t.Errorf("ParseONUIndex() port = %v, want %v", port, tt.wantPort)
			}
			if onuID != tt.wantOnuID {
				t.Errorf("ParseONUIndex() onuID = %v, want %v", onuID, tt.wantOnuID)
			}
		})
	}
}

func TestGetSNMPResult(t *testing.T) {
	results := map[string]interface{}{
		".1.3.6.1.2.1.1.3.0":                       uint64(12345),
		"1.3.6.1.4.1.2011.6.128.1.1.2.98.1.1.1.1":  int64(5),
		".1.3.6.1.4.1.2011.6.128.1.1.2.98.1.2.1.1": int64(24),
	}

	tests := []struct {
		name    string
		oid     string
		wantVal interface{}
		wantOk  bool
	}{
		{name: "with leading dot in results", oid: "1.3.6.1.2.1.1.3.0", wantVal: uint64(12345), wantOk: true},
		{name: "without leading dot in results", oid: "1.3.6.1.4.1.2011.6.128.1.1.2.98.1.1.1.1", wantVal: int64(5), wantOk: true},
		{name: "query with leading dot", oid: ".1.3.6.1.4.1.2011.6.128.1.1.2.98.1.2.1.1", wantVal: int64(24), wantOk: true},
		{name: "not found", oid: "1.3.6.1.2.1.1.1.0", wantVal: nil, wantOk: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := GetSNMPResult(results, tt.oid)
			if ok != tt.wantOk {
				t.Errorf("GetSNMPResult() ok = %v, want %v", ok, tt.wantOk)
				return
			}
			if tt.wantOk && got != tt.wantVal {
				t.Errorf("GetSNMPResult() = %v, want %v", got, tt.wantVal)
			}
		})
	}
}

func TestParseNumericSNMPValue(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		want   float64
		wantOk bool
	}{
		{name: "int", input: int(42), want: 42.0, wantOk: true},
		{name: "int negative", input: int(-10), want: -10.0, wantOk: true},
		{name: "int32", input: int32(100), want: 100.0, wantOk: true},
		{name: "int64", input: int64(999999), want: 999999.0, wantOk: true},
		{name: "uint", input: uint(50), want: 50.0, wantOk: true},
		{name: "uint32", input: uint32(200), want: 200.0, wantOk: true},
		{name: "uint64", input: uint64(123456789), want: 123456789.0, wantOk: true},
		{name: "float32", input: float32(3.14), want: float64(float32(3.14)), wantOk: true},
		{name: "float64", input: float64(2.718), want: 2.718, wantOk: true},
		{name: "string", input: "not a number", want: 0, wantOk: false},
		{name: "nil", input: nil, want: 0, wantOk: false},
		{name: "bool", input: true, want: 0, wantOk: false},
		{name: "slice", input: []int{1, 2, 3}, want: 0, wantOk: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseNumericSNMPValue(tt.input)
			if ok != tt.wantOk {
				t.Errorf("ParseNumericSNMPValue(%v) ok = %v, want %v", tt.input, ok, tt.wantOk)
				return
			}
			if tt.wantOk && got != tt.want {
				t.Errorf("ParseNumericSNMPValue(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDecodeHexSerial(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "hex-encoded serial",
			input:    "485754430011D168",
			expected: "HWTC0011D168",
		},
		{
			name:     "already ASCII serial",
			input:    "HWTC00000101",
			expected: "HWTC00000101",
		},
		{
			name:     "short serial",
			input:    "HWTC",
			expected: "HWTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecodeHexSerial(tt.input)
			if result != tt.expected {
				t.Errorf("DecodeHexSerial(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
