package huawei

import (
	"math"
	"testing"

	"github.com/nanoncore/nano-southbound/types"
)

const floatTolerance = 1e-9

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < floatTolerance
}

// ============================================================================
// parseAutofindOutput tests
// ============================================================================

func TestParseAutofindOutput(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedCount  int
		checkFirst     func(t *testing.T, d ONTDiscovery)
	}{
		{
			name: "standard autofind output with two ONTs",
			output: `
   F/S/P   ONT         SN                  VendorID   EquipmentID     Time
   -------------------------------------------------------------------
   0/1/0   1           485754430A2C4F13    HWTC       HG8245Q2        2024-01-15 10:30:00
   0/1/1   2           5053534E00000001    ZTEG       F670L           2024-01-15 10:31:00
`,
			expectedCount: 2,
			checkFirst: func(t *testing.T, d ONTDiscovery) {
				if d.Frame != 0 {
					t.Errorf("Frame = %d, want 0", d.Frame)
				}
				if d.Slot != 1 {
					t.Errorf("Slot = %d, want 1", d.Slot)
				}
				if d.Port != 0 {
					t.Errorf("Port = %d, want 0", d.Port)
				}
				if d.Serial != "485754430A2C4F13" {
					t.Errorf("Serial = %q, want %q", d.Serial, "485754430A2C4F13")
				}
				if d.EquipID != "HG8245Q2" {
					t.Errorf("EquipID = %q, want %q", d.EquipID, "HG8245Q2")
				}
			},
		},
		{
			name: "single ONT with different frame/slot/port",
			output: `
   F/S/P   ONT         SN                  VendorID   EquipmentID
   -------------------------------------------------------------------
   1/3/7   1           HWTC12345678        HWTC       HG8546M
`,
			expectedCount: 1,
			checkFirst: func(t *testing.T, d ONTDiscovery) {
				if d.Frame != 1 {
					t.Errorf("Frame = %d, want 1", d.Frame)
				}
				if d.Slot != 3 {
					t.Errorf("Slot = %d, want 3", d.Slot)
				}
				if d.Port != 7 {
					t.Errorf("Port = %d, want 7", d.Port)
				}
			},
		},
		{
			name:          "empty output",
			output:        "",
			expectedCount: 0,
		},
		{
			name: "header only, no ONTs found",
			output: `
   F/S/P   ONT         SN                  VendorID   EquipmentID     Time
   -------------------------------------------------------------------
`,
			expectedCount: 0,
		},
		{
			name:          "whitespace only",
			output:        "   \n  \n  ",
			expectedCount: 0,
		},
		{
			name: "skip lines with invalid F/S/P",
			output: `
   -------------------------------------------------------------------
   invalid  1   485754430A2C4F13    HWTC   HG8245Q2   2024-01-15 10:30:00
   0/1/0    2   5053534E00000001    ZTEG   F670L      2024-01-15 10:31:00
`,
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{config: &types.EquipmentConfig{}}
			discoveries := a.parseAutofindOutput(tt.output)

			if len(discoveries) != tt.expectedCount {
				t.Fatalf("parseAutofindOutput() returned %d discoveries, want %d", len(discoveries), tt.expectedCount)
			}

			if tt.checkFirst != nil && len(discoveries) > 0 {
				tt.checkFirst(t, discoveries[0])
			}
		})
	}
}

// ============================================================================
// parseONTStatus tests
// ============================================================================

func TestParseONTStatus(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		subscriberID   string
		wantState      string
		wantOnline     bool
		wantUptime     int64
		wantIPv4       string
		wantConfigMeta string // expected config_state in metadata
	}{
		{
			name: "online ONT with full details",
			output: `
  ONT ID          : 5
  Run state       : online
  Config state    : normal
  Online duration : 5 days 12:30:45
  IP address      : 192.168.1.100
`,
			subscriberID:   "ont-0/1/0-5",
			wantState:      "online",
			wantOnline:     true,
			wantUptime:     5*86400 + 12*3600 + 30*60 + 45,
			wantIPv4:       "192.168.1.100",
			wantConfigMeta: "normal",
		},
		{
			name: "offline ONT",
			output: `
  ONT ID          : 5
  Run state       : offline
  Config state    : normal
`,
			subscriberID: "ont-0/1/0-5",
			wantState:    "offline",
			wantOnline:   false,
			wantUptime:   0,
		},
		{
			name: "deactivated (suspended) ONT",
			output: `
  ONT ID          : 5
  Run state       : offline
  Config state    : deactivate
`,
			subscriberID: "ont-0/1/0-5",
			wantState:    "suspended",
			wantOnline:   false,
			wantUptime:   0,
		},
		{
			name:         "empty output - unknown state",
			output:       "",
			subscriberID: "test-sub",
			wantState:    "unknown",
			wantOnline:   false,
			wantUptime:   0,
		},
		{
			name: "online with 0 days uptime",
			output: `
  Run state       : online
  Online duration : 0 day 01:15:30
`,
			subscriberID: "ont-0/0/0-0",
			wantState:    "online",
			wantOnline:   true,
			wantUptime:   1*3600 + 15*60 + 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{config: &types.EquipmentConfig{}}
			status := a.parseONTStatus(tt.output, tt.subscriberID)

			if status.SubscriberID != tt.subscriberID {
				t.Errorf("SubscriberID = %q, want %q", status.SubscriberID, tt.subscriberID)
			}
			if status.State != tt.wantState {
				t.Errorf("State = %q, want %q", status.State, tt.wantState)
			}
			if status.IsOnline != tt.wantOnline {
				t.Errorf("IsOnline = %v, want %v", status.IsOnline, tt.wantOnline)
			}
			if status.UptimeSeconds != tt.wantUptime {
				t.Errorf("UptimeSeconds = %d, want %d", status.UptimeSeconds, tt.wantUptime)
			}
			if tt.wantIPv4 != "" && status.IPv4Address != tt.wantIPv4 {
				t.Errorf("IPv4Address = %q, want %q", status.IPv4Address, tt.wantIPv4)
			}
			if tt.wantConfigMeta != "" {
				if cs, ok := status.Metadata["config_state"]; ok {
					if cs != tt.wantConfigMeta {
						t.Errorf("Metadata[config_state] = %q, want %q", cs, tt.wantConfigMeta)
					}
				} else {
					t.Error("expected config_state in metadata")
				}
			}
		})
	}
}

// ============================================================================
// parseOpticalInfo tests
// ============================================================================

func TestParseOpticalInfo(t *testing.T) {
	tests := []struct {
		name         string
		output       string
		wantRxPower  string
		wantTxPower  string
		wantOltRx    string
		wantTemp     string
	}{
		{
			name: "full optical info",
			output: `
  Rx Optical Power  : -18.50 dBm
  Tx Optical Power  : 2.10 dBm
  OLT Rx ONT optical power : -19.30 dBm
  Temperature       : 42.5 C
`,
			wantRxPower: "-18.50",
			wantTxPower: "2.10",
			wantOltRx:   "-19.30",
			wantTemp:    "42.5",
		},
		{
			name: "only rx power",
			output: `
  Rx Optical Power  : -22.30 dBm
`,
			wantRxPower: "-22.30",
		},
		{
			name:   "empty output",
			output: "",
		},
		{
			name: "negative power values",
			output: `
  Rx Optical Power  : -30.00 dBm
  Tx Optical Power  : -5.00 dBm
`,
			wantRxPower: "-30.00",
			wantTxPower: "-5.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{config: &types.EquipmentConfig{}}
			status := &types.SubscriberStatus{
				Metadata: make(map[string]interface{}),
			}

			a.parseOpticalInfo(tt.output, status)

			checkMeta := func(key, want string) {
				if want == "" {
					return
				}
				val, ok := status.Metadata[key]
				if !ok {
					t.Errorf("expected %q in metadata", key)
					return
				}
				if val != want {
					t.Errorf("Metadata[%q] = %q, want %q", key, val, want)
				}
			}

			checkMeta("rx_power_dbm", tt.wantRxPower)
			checkMeta("tx_power_dbm", tt.wantTxPower)
			checkMeta("olt_rx_power_dbm", tt.wantOltRx)
			checkMeta("temperature_c", tt.wantTemp)
		})
	}
}

// ============================================================================
// parseONTStats tests
// ============================================================================

func TestParseONTStats(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		wantBytesUp uint64
		wantBytesDn uint64
		wantPktsUp  uint64
		wantPktsDn  uint64
		wantErrors  uint64
	}{
		{
			name: "full traffic stats",
			output: `
  Upstream traffic   : 12345 bytes
  Downstream traffic : 67890 bytes
  Upstream packets   : 100
  Downstream packets : 200
  Errors             : 5
`,
			wantBytesUp: 12345,
			wantBytesDn: 67890,
			wantPktsUp:  100,
			wantPktsDn:  200,
			wantErrors:  5,
		},
		{
			name: "only byte counters",
			output: `
  Upstream traffic   : 999 bytes
  Downstream traffic : 5000 bytes
`,
			wantBytesUp: 999,
			wantBytesDn: 5000,
		},
		{
			name:   "empty output",
			output: "",
		},
		{
			name: "large counters",
			output: `
  Upstream traffic   : 18446744073709551615 bytes
  Downstream traffic : 9999999999 bytes
  Upstream packets   : 1000000
  Downstream packets : 2000000
`,
			wantBytesUp: 18446744073709551615,
			wantBytesDn: 9999999999,
			wantPktsUp:  1000000,
			wantPktsDn:  2000000,
		},
		{
			name: "with discards",
			output: `
  Discards : 42
`,
			wantErrors: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{config: &types.EquipmentConfig{}}
			stats := a.parseONTStats(tt.output)

			if stats.BytesUp != tt.wantBytesUp {
				t.Errorf("BytesUp = %d, want %d", stats.BytesUp, tt.wantBytesUp)
			}
			if stats.BytesDown != tt.wantBytesDn {
				t.Errorf("BytesDown = %d, want %d", stats.BytesDown, tt.wantBytesDn)
			}
			if stats.PacketsUp != tt.wantPktsUp {
				t.Errorf("PacketsUp = %d, want %d", stats.PacketsUp, tt.wantPktsUp)
			}
			if stats.PacketsDown != tt.wantPktsDn {
				t.Errorf("PacketsDown = %d, want %d", stats.PacketsDown, tt.wantPktsDn)
			}
			if stats.ErrorsDown != tt.wantErrors {
				t.Errorf("ErrorsDown = %d, want %d", stats.ErrorsDown, tt.wantErrors)
			}
			// Stats should always have a metadata map and timestamp
			if stats.Metadata == nil {
				t.Error("expected non-nil Metadata")
			}
			if stats.Timestamp.IsZero() {
				t.Error("expected non-zero Timestamp")
			}
		})
	}
}

// ============================================================================
// parseAlarms tests
// ============================================================================

func TestParseAlarms(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		expectedCount int
		checkFirst    func(t *testing.T, a types.OLTAlarm)
	}{
		{
			name: "standard alarm table",
			output: `
  Alarm List
  -----------------------------------------------------------------------
  12345      Critical   LOS          0/0/1:5           2024-01-15 10:30:00    Loss of signal
  12346      Major      power        0/0/1:6           2024-01-15 10:31:00    Low power
`,
			expectedCount: 2,
			checkFirst: func(t *testing.T, a types.OLTAlarm) {
				if a.ID != "12345" {
					t.Errorf("ID = %q, want %q", a.ID, "12345")
				}
				if a.Severity != "critical" {
					t.Errorf("Severity = %q, want %q", a.Severity, "critical")
				}
				if a.Type != "los" {
					t.Errorf("Type = %q, want %q", a.Type, "los")
				}
				if a.SourceID != "0/0/1:5" {
					t.Errorf("SourceID = %q, want %q", a.SourceID, "0/0/1:5")
				}
				if a.Source != "onu" {
					t.Errorf("Source = %q, want %q", a.Source, "onu")
				}
			},
		},
		{
			name: "alarm with ONU colon source",
			output: `
  -----------------------------------------------------------------------
  99999      Minor      dying        0/2/3:10          2024-06-01 08:00:00    Dying gasp
`,
			expectedCount: 1,
			checkFirst: func(t *testing.T, a types.OLTAlarm) {
				if a.Severity != "minor" {
					t.Errorf("Severity = %q, want %q", a.Severity, "minor")
				}
				if a.Type != "dying_gasp" {
					t.Errorf("Type = %q, want %q", a.Type, "dying_gasp")
				}
				if a.Source != "onu" {
					t.Errorf("Source = %q, want %q", a.Source, "onu")
				}
			},
		},
		{
			name: "alarm with port source (no colon)",
			output: `
  -----------------------------------------------------------------------
  11111      Warning    link         0/0/1             2024-02-20 12:00:00    Link down
`,
			expectedCount: 1,
			checkFirst: func(t *testing.T, a types.OLTAlarm) {
				if a.Severity != "warning" {
					t.Errorf("Severity = %q, want %q", a.Severity, "warning")
				}
				if a.Type != "link" {
					t.Errorf("Type = %q, want %q", a.Type, "link")
				}
				if a.Source != "port" {
					t.Errorf("Source = %q, want %q", a.Source, "port")
				}
			},
		},
		{
			name: "config alarm",
			output: `
  -----------------------------------------------------------------------
  22222      Major      config       system            2024-03-15 09:00:00    Config mismatch
`,
			expectedCount: 1,
			checkFirst: func(t *testing.T, a types.OLTAlarm) {
				if a.Type != "config" {
					t.Errorf("Type = %q, want %q", a.Type, "config")
				}
				if a.Source != "system" {
					t.Errorf("Source = %q, want %q", a.Source, "system")
				}
			},
		},
		{
			name:          "empty output",
			output:        "",
			expectedCount: 0,
		},
		{
			name: "no alarms message",
			output: `
  No alarm active.
`,
			expectedCount: 0,
		},
		{
			name: "alarm with timestamp parsing",
			output: `
  -----------------------------------------------------------------------
  55555      Critical   LOS          0/0/1:1           2024-06-15 14:30:00    Signal lost
`,
			expectedCount: 1,
			checkFirst: func(t *testing.T, a types.OLTAlarm) {
				if a.RaisedAt.IsZero() {
					t.Error("expected non-zero RaisedAt")
				}
				if a.RaisedAt.Year() != 2024 || a.RaisedAt.Month() != 6 || a.RaisedAt.Day() != 15 {
					t.Errorf("RaisedAt = %v, want 2024-06-15", a.RaisedAt)
				}
				if a.Message != "Signal lost" {
					t.Errorf("Message = %q, want %q", a.Message, "Signal lost")
				}
			},
		},
		{
			name: "skip header and total lines",
			output: `
  Alarm Configuration
  Alarm ID   Severity   Type         Source            Time
  -----------------------------------------------------------------------
  12345      Critical   LOS          0/0/1:5           2024-01-15 10:30:00    LOS alarm
  Total alarms: 1
`,
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{config: &types.EquipmentConfig{}}
			alarms := a.parseAlarms(tt.output)

			if len(alarms) != tt.expectedCount {
				t.Fatalf("parseAlarms() returned %d alarms, want %d", len(alarms), tt.expectedCount)
			}

			if tt.checkFirst != nil && len(alarms) > 0 {
				tt.checkFirst(t, alarms[0])
			}
		})
	}
}

// ============================================================================
// parsePortFromDescr tests
// ============================================================================

func TestParsePortFromDescr(t *testing.T) {
	tests := []struct {
		name     string
		descr    string
		expected string
	}{
		{
			name:     "GPON port description",
			descr:    "GPON 0/0/1",
			expected: "0/0/1",
		},
		{
			name:     "bare F/S/P format",
			descr:    "0/1/7",
			expected: "0/1/7",
		},
		{
			name:     "with prefix text",
			descr:    "interface gpon 1/3/2",
			expected: "1/3/2",
		},
		{
			name:     "no port pattern",
			descr:    "no port here",
			expected: "",
		},
		{
			name:     "empty description",
			descr:    "",
			expected: "",
		},
		{
			name:     "multi-digit values",
			descr:    "GPON 10/20/30",
			expected: "10/20/30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{config: &types.EquipmentConfig{}}
			got := a.parsePortFromDescr(tt.descr)
			if got != tt.expected {
				t.Errorf("parsePortFromDescr(%q) = %q, want %q", tt.descr, got, tt.expected)
			}
		})
	}
}

// ============================================================================
// SNMP conversion functions (additional tests)
// ============================================================================

func TestConvertOpticalPower(t *testing.T) {
	tests := []struct {
		name     string
		rawValue int64
		expected float64
	}{
		{
			name:     "normal positive value",
			rawValue: 210,
			expected: 2.10,
		},
		{
			name:     "normal negative value",
			rawValue: -1850,
			expected: -18.50,
		},
		{
			name:     "zero",
			rawValue: 0,
			expected: 0.0,
		},
		{
			name:     "invalid value (offline ONU)",
			rawValue: 2147483647,
			expected: -100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertOpticalPower(tt.rawValue)
			if got != tt.expected {
				t.Errorf("ConvertOpticalPower(%d) = %f, want %f", tt.rawValue, got, tt.expected)
			}
		})
	}
}

func TestConvertOltRxPower(t *testing.T) {
	tests := []struct {
		name     string
		rawValue int64
		expected float64
	}{
		{
			name:     "normal value",
			rawValue: 8150,
			expected: -18.50,
		},
		{
			name:     "value at 10000 (zero dBm)",
			rawValue: 10000,
			expected: 0.0,
		},
		{
			name:     "value above 10000 (positive dBm)",
			rawValue: 10210,
			expected: 2.10,
		},
		{
			name:     "invalid value (offline)",
			rawValue: 2147483647,
			expected: -100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertOltRxPower(tt.rawValue)
			if got != tt.expected {
				t.Errorf("ConvertOltRxPower(%d) = %f, want %f", tt.rawValue, got, tt.expected)
			}
		})
	}
}

func TestConvertVoltage(t *testing.T) {
	tests := []struct {
		name     string
		rawValue int64
		expected float64
	}{
		{
			name:     "3.3V typical ONU voltage",
			rawValue: 3300,
			expected: 3.3,
		},
		{
			name:     "zero",
			rawValue: 0,
			expected: 0.0,
		},
		{
			name:     "invalid value (offline)",
			rawValue: 2147483647,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertVoltage(tt.rawValue)
			if !almostEqual(got, tt.expected) {
				t.Errorf("ConvertVoltage(%d) = %f, want %f", tt.rawValue, got, tt.expected)
			}
		})
	}
}

func TestConvertTemperature(t *testing.T) {
	tests := []struct {
		name     string
		rawValue int64
		expected float64
	}{
		{
			name:     "normal temperature",
			rawValue: 11264, // 44 * 256
			expected: 44.0,
		},
		{
			name:     "zero value",
			rawValue: 0,
			expected: 0.0,
		},
		{
			name:     "invalid value (offline)",
			rawValue: 2147483647,
			expected: 0.0,
		},
		{
			name:     "fractional temperature",
			rawValue: 11520, // 45 * 256
			expected: 45.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertTemperature(tt.rawValue)
			if got != tt.expected {
				t.Errorf("ConvertTemperature(%d) = %f, want %f", tt.rawValue, got, tt.expected)
			}
		})
	}
}

func TestIsOnuOnline(t *testing.T) {
	tests := []struct {
		name       string
		rxPowerRaw int64
		want       bool
	}{
		{
			name:       "online ONU (valid power)",
			rxPowerRaw: -1850,
			want:       true,
		},
		{
			name:       "online ONU (zero power)",
			rxPowerRaw: 0,
			want:       true,
		},
		{
			name:       "offline ONU (invalid marker)",
			rxPowerRaw: 2147483647,
			want:       false,
		},
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

func TestIsASCIISerial(t *testing.T) {
	tests := []struct {
		name   string
		serial string
		want   bool
	}{
		{name: "ASCII serial HWTC", serial: "HWTC00001234", want: true},
		{name: "ASCII serial ZTEG", serial: "ZTEG00000001", want: true},
		{name: "hex serial", serial: "485754430011D168", want: false},
		{name: "short serial", serial: "AB", want: false},
		{name: "starts with lowercase", serial: "hwtc00001234", want: false},
		{name: "starts with numbers", serial: "1234ABCD", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isASCIISerial(tt.serial)
			if got != tt.want {
				t.Errorf("isASCIISerial(%q) = %v, want %v", tt.serial, got, tt.want)
			}
		})
	}
}

func TestHexToByte(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected byte
	}{
		{name: "zero", input: "00", expected: 0},
		{name: "uppercase FF", input: "FF", expected: 255},
		{name: "lowercase ff", input: "ff", expected: 255},
		{name: "mixed Af", input: "Af", expected: 175},
		{name: "letter H (0x48)", input: "48", expected: 72},
		{name: "letter W (0x57)", input: "57", expected: 87},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hexToByte(tt.input)
			if got != tt.expected {
				t.Errorf("hexToByte(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}
