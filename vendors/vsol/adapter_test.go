package vsol

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
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
		name          string
		input         string
		expectedCount int
		expectedFirst struct {
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

func TestParseVSOLConfirmOnuID(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected int
		ok       bool
	}{
		{
			name:     "simple_confirm",
			output:   "Note: Register pon 1 onu 5 OK.",
			expected: 5,
			ok:       true,
		},
		{
			name:     "register_format",
			output:   "Register pon 1 onu 12 OK",
			expected: 12,
			ok:       true,
		},
		{
			name:     "no_match",
			output:   "Error: No related information to show. 62310",
			expected: 0,
			ok:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := parseVSOLConfirmOnuID(tt.output)
			if ok != tt.ok {
				t.Fatalf("expected ok=%v, got %v", tt.ok, ok)
			}
			if id != tt.expected {
				t.Fatalf("expected id=%d, got %d", tt.expected, id)
			}
		})
	}
}

func TestProvisionGPONWithConfirm(t *testing.T) {
	exec := &mockCLIExecutor{
		outputs: map[string]string{
			"configure terminal":              "",
			"interface gpon 0/1":              "",
			"onu confirm":                     "Register pon 1 onu 7 OK",
			"onu 7 profile line name line999": "Note: pon 1 onu 7 set profile line name line999 OK.",
			"exit":                            "",
			"end":                             "",
		},
	}

	adapter := &Adapter{cliExecutor: exec}
	subscriber := &model.Subscriber{
		Annotations: map[string]string{
			"nano.io/line-profile": "line999",
		},
		Spec: model.SubscriberSpec{
			ONUSerial: "FHTT99990001",
			VLAN:      999,
		},
	}

	assignedID, outputs, err := adapter.provisionGPONWithConfirm(
		context.Background(),
		"0/1",
		subscriber.Spec.ONUSerial,
		subscriber.Spec.VLAN,
		subscriber,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assignedID != 7 {
		t.Fatalf("expected assigned ID 7, got %d", assignedID)
	}
	if len(outputs) == 0 {
		t.Fatalf("expected outputs to be recorded")
	}

	expected := []string{
		"configure terminal",
		"interface gpon 0/1",
		"onu confirm",
		"onu 7 profile line name line999",
		"exit",
		"end",
	}
	if !equalStringSlices(exec.commands, expected) {
		t.Fatalf("unexpected command sequence: %v", exec.commands)
	}
}

type mockCLIExecutor struct {
	outputs  map[string]string
	commands []string
}

func (m *mockCLIExecutor) ExecCommand(_ context.Context, command string) (string, error) {
	m.commands = append(m.commands, command)
	if out, ok := m.outputs[command]; ok {
		return out, nil
	}
	return "", nil
}

func (m *mockCLIExecutor) ExecCommands(ctx context.Context, commands []string) ([]string, error) {
	results := make([]string, 0, len(commands))
	for _, cmd := range commands {
		out, err := m.ExecCommand(ctx, cmd)
		if err != nil {
			return results, err
		}
		results = append(results, out)
	}
	return results, nil
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- findOrCreateBandwidthProfiles tests ---

// profileMockCLI is a mock CLI that simulates show profile dba/traffic output
// and tracks profile creation commands.
type profileMockCLI struct {
	dbaOutput     string
	trafficOutput string
	commands      []string
}

func (m *profileMockCLI) ExecCommand(_ context.Context, command string) (string, error) {
	m.commands = append(m.commands, command)
	return "", nil
}

func (m *profileMockCLI) ExecCommands(_ context.Context, commands []string) ([]string, error) {
	results := make([]string, len(commands))
	for i, cmd := range commands {
		m.commands = append(m.commands, cmd)
		if cmd == "show profile dba" {
			results[i] = m.dbaOutput
		} else if cmd == "show profile traffic" {
			results[i] = m.trafficOutput
		} else if strings.HasPrefix(cmd, "profile dba id") {
			results[i] = "profile_id:99 create success"
		} else if strings.HasPrefix(cmd, "profile traffic id") {
			results[i] = "profile_id:99 create success"
		}
	}
	return results, nil
}

func TestFindOrCreateBandwidthProfiles_ExistingProfiles(t *testing.T) {
	mock := &profileMockCLI{
		dbaOutput: `
###############DBA PROFILE###########
*****************************
              Id: 1
            name: default
            type: 4
         maximum: 1024000 Kbps

*****************************
              Id: 5
            name: nano_dba_50000
            type: 4
         maximum: 50000 Kbps
`,
		trafficOutput: `
###############TRAFFIC PROFILE###########
*****************************
Id:   1
Name: default
sir:  0 Kbps
pir:  1024000 Kbps

*****************************
Id:   5
Name: nano_traffic_50000
sir:  0 Kbps
pir:  50000 Kbps

*****************************
Id:   6
Name: nano_traffic_100000
sir:  0 Kbps
pir:  100000 Kbps
`,
	}

	adapter := &Adapter{cliExecutor: mock}
	ctx := context.Background()

	result, err := adapter.findOrCreateBandwidthProfiles(ctx, 50000, 100000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.DBAName != "nano_dba_50000" {
		t.Errorf("DBAName: got %q, want nano_dba_50000", result.DBAName)
	}
	if result.TrafficUpName != "nano_traffic_50000" {
		t.Errorf("TrafficUpName: got %q, want nano_traffic_50000", result.TrafficUpName)
	}
	if result.TrafficDnName != "nano_traffic_100000" {
		t.Errorf("TrafficDnName: got %q, want nano_traffic_100000", result.TrafficDnName)
	}

	// Should NOT have any create commands since all profiles already exist
	for _, cmd := range mock.commands {
		if strings.HasPrefix(cmd, "profile dba id") || strings.HasPrefix(cmd, "profile traffic id") {
			t.Errorf("unexpected create command: %s", cmd)
		}
	}
}

func TestFindOrCreateBandwidthProfiles_CreatesNewProfiles(t *testing.T) {
	mock := &profileMockCLI{
		dbaOutput: `
###############DBA PROFILE###########
*****************************
              Id: 1
            name: default
            type: 4
         maximum: 1024000 Kbps
`,
		trafficOutput: `
###############TRAFFIC PROFILE###########
*****************************
Id:   1
Name: default
sir:  0 Kbps
pir:  1024000 Kbps
`,
	}

	adapter := &Adapter{cliExecutor: mock}
	ctx := context.Background()

	result, err := adapter.findOrCreateBandwidthProfiles(ctx, 50000, 100000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DBAName != "nano_dba_50000" {
		t.Errorf("DBAName: got %q, want nano_dba_50000", result.DBAName)
	}
	if result.TrafficUpName != "nano_traffic_50000" {
		t.Errorf("TrafficUpName: got %q, want nano_traffic_50000", result.TrafficUpName)
	}
	if result.TrafficDnName != "nano_traffic_100000" {
		t.Errorf("TrafficDnName: got %q, want nano_traffic_100000", result.TrafficDnName)
	}

	// Should have create commands for all 3 profiles
	createCount := 0
	for _, cmd := range mock.commands {
		if strings.HasPrefix(cmd, "profile dba id") || strings.HasPrefix(cmd, "profile traffic id") {
			createCount++
		}
	}
	if createCount != 3 {
		t.Errorf("expected 3 create commands, got %d (commands: %v)", createCount, mock.commands)
	}
}

func TestFindOrCreateBandwidthProfiles_ZeroBandwidth(t *testing.T) {
	adapter := &Adapter{}
	ctx := context.Background()

	result, err := adapter.findOrCreateBandwidthProfiles(ctx, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for zero bandwidth, got %+v", result)
	}
}

func TestFindOrCreateBandwidthProfiles_CachedAvoidsDuplicateLists(t *testing.T) {
	mock := &profileMockCLI{
		dbaOutput: `
###############DBA PROFILE###########
*****************************
              Id: 1
            name: default
            type: 4
         maximum: 1024000 Kbps
`,
		trafficOutput: `
###############TRAFFIC PROFILE###########
*****************************
Id:   1
Name: default
sir:  0 Kbps
pir:  1024000 Kbps
`,
	}

	adapter := &Adapter{cliExecutor: mock}
	ctx := context.Background()

	cache, err := adapter.newProfileCache(ctx)
	if err != nil {
		t.Fatalf("newProfileCache: %v", err)
	}

	// Reset commands to track only the findOrCreate calls
	mock.commands = nil

	// First call creates profiles — internally CreateDBAProfile/CreateTrafficProfile
	// still list to auto-assign IDs, so some "show profile" commands are expected.
	_, err = adapter.findOrCreateBandwidthProfilesCached(ctx, 50000, 100000, cache)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	firstCallCmds := len(mock.commands)

	// Second call with same bandwidth should find profiles in cache — zero CLI commands
	mock.commands = nil
	_, err = adapter.findOrCreateBandwidthProfilesCached(ctx, 50000, 100000, cache)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if len(mock.commands) != 0 {
		t.Errorf("second call should issue 0 commands (all cached), got %d: %v", len(mock.commands), mock.commands)
	}
	if firstCallCmds == 0 {
		t.Errorf("first call should have issued commands to create profiles")
	}
}

func TestBuildBandwidthONUCommands(t *testing.T) {
	bw := &bandwidthProfiles{
		DBAName:       "nano_dba_50000",
		TrafficUpName: "nano_traffic_50000",
		TrafficDnName: "nano_traffic_100000",
	}

	cmds := buildBandwidthONUCommands(7, bw)
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d: %v", len(cmds), cmds)
	}
	if cmds[0] != "onu 7 tcont 1 dba nano_dba_50000" {
		t.Errorf("cmd[0]: got %q", cmds[0])
	}
	if cmds[1] != "onu 7 gemport 1 traffic-limit upstream nano_traffic_50000 downstream nano_traffic_100000" {
		t.Errorf("cmd[1]: got %q", cmds[1])
	}
}

func TestBuildBandwidthONUCommandsDefaults(t *testing.T) {
	bw := &bandwidthProfiles{
		DBAName: "test_dba",
		// TrafficUpName and TrafficDnName are empty — should default to "default"
	}

	cmds := buildBandwidthONUCommands(3, bw)
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d: %v", len(cmds), cmds)
	}
	if cmds[1] != "onu 3 gemport 1 traffic-limit upstream default downstream default" {
		t.Errorf("expected defaults, got %q", cmds[1])
	}
}

func TestBuildBandwidthCommands(t *testing.T) {
	bw := &bandwidthProfiles{
		DBAName:       "nano_dba_50000",
		TrafficUpName: "nano_traffic_50000",
		TrafficDnName: "nano_traffic_100000",
	}

	cmds := buildBandwidthCommands("0/1", 5, bw)
	expected := []string{
		"configure terminal",
		"interface gpon 0/1",
		"onu 5 tcont 1 dba nano_dba_50000",
		"onu 5 gemport 1 traffic-limit upstream nano_traffic_50000 downstream nano_traffic_100000",
		"exit",
		"end",
	}
	if !equalStringSlices(cmds, expected) {
		t.Errorf("got %v, want %v", cmds, expected)
	}
}

// Ensure unused imports are used
var _ = types.DBAProfile{}
var _ = strings.Contains
