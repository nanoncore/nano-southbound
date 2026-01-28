package vsol

import (
	"testing"
	"time"
)

func TestExtractPONPortFromIndex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"standard_port_1", "1/1/1:1", "0/1"},
		{"standard_port_8", "1/1/8:2", "0/8"},
		{"standard_port_4", "1/1/4:10", "0/4"},
		{"invalid_format", "invalid", ""},
		{"empty_string", "", ""},
		{"no_colon", "1/1/1", "0/1"},
		{"single_slash", "1/1", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPONPortFromIndex(tt.input)
			if got != tt.expected {
				t.Errorf("extractPONPortFromIndex(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseAutofindOutput(t *testing.T) {
	// Create adapter for testing (no actual connections needed)
	adapter := &Adapter{}

	tests := []struct {
		name           string
		input          string
		expectedCount  int
		expectedFirst  struct {
			ponPort string
			serial  string
			state   string
		}
	}{
		{
			name: "single_ONU",
			input: `OnuIndex                 Sn                       State
---------------------------------------------------------
1/1/1:1                  FHTT99990001             unknow`,
			expectedCount: 1,
			expectedFirst: struct {
				ponPort string
				serial  string
				state   string
			}{
				ponPort: "0/1",
				serial:  "FHTT99990001",
				state:   "unknow",
			},
		},
		{
			name: "multiple_ONUs",
			input: `OnuIndex                 Sn                       State
---------------------------------------------------------
1/1/1:1                  FHTT99990001             unknow
1/1/1:2                  FHTT99990002             unknow`,
			expectedCount: 2,
			expectedFirst: struct {
				ponPort string
				serial  string
				state   string
			}{
				ponPort: "0/1",
				serial:  "FHTT99990001",
				state:   "unknow",
			},
		},
		{
			name: "empty_list_with_error",
			input: `OnuIndex                 Sn                       State
---------------------------------------------------------

Error: No related information to show. 62310`,
			expectedCount: 0,
		},
		{
			name: "different_port",
			input: `OnuIndex                 Sn                       State
---------------------------------------------------------
1/1/8:3                  ZTEG12345678             unknow`,
			expectedCount: 1,
			expectedFirst: struct {
				ponPort string
				serial  string
				state   string
			}{
				ponPort: "0/8",
				serial:  "ZTEG12345678",
				state:   "unknow",
			},
		},
		{
			name: "mixed_ports",
			input: `OnuIndex                 Sn                       State
---------------------------------------------------------
1/1/1:1                  FHTT00000001             unknow
1/1/2:1                  FHTT00000002             unknow
1/1/3:1                  FHTT00000003             unknow`,
			expectedCount: 3,
			expectedFirst: struct {
				ponPort string
				serial  string
				state   string
			}{
				ponPort: "0/1",
				serial:  "FHTT00000001",
				state:   "unknow",
			},
		},
		{
			name:          "empty_output",
			input:         "",
			expectedCount: 0,
		},
		{
			name: "header_only",
			input: `OnuIndex                 Sn                       State
---------------------------------------------------------`,
			expectedCount: 0,
		},
		{
			name: "without_state_column",
			input: `OnuIndex                 Sn
---------------------------------------------------------
1/1/1:1                  FHTT99990001`,
			expectedCount: 1,
			expectedFirst: struct {
				ponPort string
				serial  string
				state   string
			}{
				ponPort: "0/1",
				serial:  "FHTT99990001",
				state:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discoveries := adapter.parseAutofindOutput(tt.input)

			if len(discoveries) != tt.expectedCount {
				t.Errorf("parseAutofindOutput() returned %d discoveries, want %d", len(discoveries), tt.expectedCount)
				return
			}

			if tt.expectedCount > 0 {
				first := discoveries[0]
				if first.PONPort != tt.expectedFirst.ponPort {
					t.Errorf("first discovery PONPort = %q, want %q", first.PONPort, tt.expectedFirst.ponPort)
				}
				if first.Serial != tt.expectedFirst.serial {
					t.Errorf("first discovery Serial = %q, want %q", first.Serial, tt.expectedFirst.serial)
				}
				if first.State != tt.expectedFirst.state {
					t.Errorf("first discovery State = %q, want %q", first.State, tt.expectedFirst.state)
				}
				// Verify DiscoveredAt is set
				if first.DiscoveredAt.IsZero() {
					t.Error("first discovery DiscoveredAt should not be zero")
				}
				// Verify it's recent (within last second)
				if time.Since(first.DiscoveredAt) > time.Second {
					t.Error("first discovery DiscoveredAt should be recent")
				}
			}
		})
	}
}

func TestParseAutofindOutput_MultipleONUsVerifyAll(t *testing.T) {
	adapter := &Adapter{}

	input := `OnuIndex                 Sn                       State
---------------------------------------------------------
1/1/1:1                  FHTT99990001             unknow
1/1/1:2                  FHTT99990002             unknow
1/1/8:1                  ZTEG12345678             unknow`

	discoveries := adapter.parseAutofindOutput(input)

	if len(discoveries) != 3 {
		t.Fatalf("expected 3 discoveries, got %d", len(discoveries))
	}

	expected := []struct {
		ponPort string
		serial  string
		state   string
	}{
		{"0/1", "FHTT99990001", "unknow"},
		{"0/1", "FHTT99990002", "unknow"},
		{"0/8", "ZTEG12345678", "unknow"},
	}

	for i, exp := range expected {
		got := discoveries[i]
		if got.PONPort != exp.ponPort {
			t.Errorf("discovery[%d] PONPort = %q, want %q", i, got.PONPort, exp.ponPort)
		}
		if got.Serial != exp.serial {
			t.Errorf("discovery[%d] Serial = %q, want %q", i, got.Serial, exp.serial)
		}
		if got.State != exp.state {
			t.Errorf("discovery[%d] State = %q, want %q", i, got.State, exp.state)
		}
	}
}
