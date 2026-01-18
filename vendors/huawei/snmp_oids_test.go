package huawei

import "testing"

func TestParseONUIndex(t *testing.T) {
	tests := []struct {
		name        string
		index       string
		wantFrame   int
		wantSlot    int
		wantPort    int
		wantOnuID   int
		wantErr     bool
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
