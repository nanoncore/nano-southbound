package vsol

import (
	"testing"

	"github.com/nanoncore/nano-southbound/types"
)

// =============================================================================
// parseONUState Tests
// =============================================================================

func TestParseONUState_SlotFrameFormat(t *testing.T) {
	// Standard V1600 format: slot/frame/port:onuid
	output := `OnuIndex    Admin State    OMCC State    Phase State    Channel
---------------------------------------------------------------
1/1/1:1     enable         enable        working        1(GPON)
1/1/1:2     enable         enable        syncMib        1(GPON)
1/1/2:1     enable         disable       los            1(GPON)`

	adapter := &Adapter{}
	states := adapter.parseONUState(output)

	if len(states) != 3 {
		t.Fatalf("expected 3 states, got %d", len(states))
	}

	// First ONU: 1/1/1:1 -> ponPort 0/1, onuID 1, working=online
	assertONUState(t, states[0], "0/1", 1, "enable", "working", true)

	// Second ONU: 1/1/1:2 -> ponPort 0/1, onuID 2, syncMib=offline
	assertONUState(t, states[1], "0/1", 2, "enable", "syncmib", false)

	// Third ONU: 1/1/2:1 -> ponPort 0/2, onuID 1, los=offline
	assertONUState(t, states[2], "0/2", 1, "enable", "los", false)
}

func TestParseONUState_GPONFormat(t *testing.T) {
	// Alternate firmware format: GPON0/1:1
	output := `OnuIndex    Admin State    OMCC State    Phase State    Channel
---------------------------------------------------------------
GPON0/1:1   enable         enable        working        1(GPON)
GPON0/1:2   enable         enable        working        1(GPON)
GPON0/2:1   disable        disable       los            1(GPON)`

	adapter := &Adapter{}
	states := adapter.parseONUState(output)

	if len(states) != 3 {
		t.Fatalf("expected 3 states, got %d", len(states))
	}

	assertONUState(t, states[0], "0/1", 1, "enable", "working", true)
	assertONUState(t, states[1], "0/1", 2, "enable", "working", true)
	assertONUState(t, states[2], "0/2", 1, "disable", "los", false)
}

func TestParseONUState_BareFormat(t *testing.T) {
	// Format without GPON prefix: 0/1:1
	output := `OnuIndex    Admin State    OMCC State    Phase State    Channel
---------------------------------------------------------------
0/1:1       enable         enable        working        1(GPON)
0/2:3       enable         enable        dying_gasp     1(GPON)`

	adapter := &Adapter{}
	states := adapter.parseONUState(output)

	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d", len(states))
	}

	assertONUState(t, states[0], "0/1", 1, "enable", "working", true)
	assertONUState(t, states[1], "0/2", 3, "enable", "dying_gasp", false)
}

func TestParseONUState_MixedFormats(t *testing.T) {
	// Some firmware might mix formats in the same output
	output := `OnuIndex    Admin State    OMCC State    Phase State    Channel
---------------------------------------------------------------
1/1/1:1     enable         enable        working        1(GPON)
GPON0/2:1   enable         enable        working        1(GPON)`

	adapter := &Adapter{}
	states := adapter.parseONUState(output)

	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d", len(states))
	}

	assertONUState(t, states[0], "0/1", 1, "enable", "working", true)
	assertONUState(t, states[1], "0/2", 1, "enable", "working", true)
}

func TestParseONUState_Empty(t *testing.T) {
	adapter := &Adapter{}

	tests := []struct {
		name   string
		output string
	}{
		{"empty string", ""},
		{"header only", "OnuIndex    Admin State    OMCC State    Phase State    Channel\n---------------------------------------------------------------"},
		{"error message", "Error: command not found"},
		{"ONU Number header", "ONU Number: 0"},
		{"percent prefix", "% No matching ONU found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			states := adapter.parseONUState(tt.output)
			if len(states) != 0 {
				t.Errorf("expected 0 states, got %d", len(states))
			}
		})
	}
}

func TestParseONUState_PhaseStates(t *testing.T) {
	adapter := &Adapter{}

	tests := []struct {
		name       string
		phaseState string
		wantOnline bool
	}{
		{"working", "working", true},
		{"syncMib", "syncMib", false},
		{"los", "los", false},
		{"dying_gasp", "dying_gasp", false},
		{"initial", "initial", false},
		{"standby", "standby", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := "1/1/1:1     enable         enable        " + tt.phaseState + "        1(GPON)"
			states := adapter.parseONUState(output)
			if len(states) != 1 {
				t.Fatalf("expected 1 state, got %d", len(states))
			}
			if states[0].IsOnline != tt.wantOnline {
				t.Errorf("phaseState %q: expected IsOnline=%v, got %v", tt.phaseState, tt.wantOnline, states[0].IsOnline)
			}
		})
	}
}

func TestParseONUState_SkipsUnparseableLines(t *testing.T) {
	output := `1/1/1:1     enable         enable        working        1(GPON)
garbage_line_without_proper_format
INVALID     enable         enable        working        1(GPON)
1/1/1:2     enable         enable        los            1(GPON)`

	adapter := &Adapter{}
	states := adapter.parseONUState(output)

	// Should only parse the two valid lines
	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d", len(states))
	}
	if states[0].ONUID != 1 {
		t.Errorf("expected first ONU ID 1, got %d", states[0].ONUID)
	}
	if states[1].ONUID != 2 {
		t.Errorf("expected second ONU ID 2, got %d", states[1].ONUID)
	}
}

// =============================================================================
// parseV1600ONUList Tests
// =============================================================================

func TestParseV1600ONUList_StandardOutput(t *testing.T) {
	output := `Onuindex   Model                Profile                Mode    AuthInfo
----------------------------------------------------------------------------
GPON0/1:1  unknown              AN5506-04-F1           sn      FHTT5929E410
GPON0/1:2  HG6143D              AN5506-04-F1           sn      FHTT59CB8310
GPON0/2:1  unknown              default                sn      GPON00929978`

	adapter := &Adapter{}
	onus := adapter.parseV1600ONUList(output, "")

	if len(onus) != 3 {
		t.Fatalf("expected 3 ONUs, got %d", len(onus))
	}

	// First ONU
	assertONUInfo(t, onus[0], "0/1", 1, "FHTT5929E410", "unknown", "AN5506-04-F1")
	if onus[0].Vendor != "FiberHome" {
		t.Errorf("expected vendor FiberHome, got %q", onus[0].Vendor)
	}

	// Second ONU
	assertONUInfo(t, onus[1], "0/1", 2, "FHTT59CB8310", "HG6143D", "AN5506-04-F1")

	// Third ONU on different port
	assertONUInfo(t, onus[2], "0/2", 1, "GPON00929978", "unknown", "default")
}

func TestParseV1600ONUList_DefaultFields(t *testing.T) {
	output := `Onuindex   Model                Profile                Mode    AuthInfo
----------------------------------------------------------------------------
GPON0/1:1  HG8245H              default-onu            sn      HWTC12345678`

	adapter := &Adapter{}
	onus := adapter.parseV1600ONUList(output, "")

	if len(onus) != 1 {
		t.Fatalf("expected 1 ONU, got %d", len(onus))
	}

	onu := onus[0]
	if !onu.IsOnline {
		t.Error("expected default IsOnline=true")
	}
	if onu.AdminState != "enabled" {
		t.Errorf("expected default AdminState=enabled, got %q", onu.AdminState)
	}
	if onu.OperState != "unknown" {
		t.Errorf("expected default OperState=unknown, got %q", onu.OperState)
	}
	if onu.Metadata == nil || onu.Metadata["auth_mode"] != "serial" {
		t.Error("expected auth_mode=serial in metadata for sn mode")
	}
}

func TestParseV1600ONUList_SkipsHeaders(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{"empty", ""},
		{"header only", "Onuindex   Model                Profile                Mode    AuthInfo\n----------------------------------------------------------------------------"},
		{"error message", "Error: this command is not supported"},
		{"error with colon space", "Error : failed to get ONU list"},
		{"command not found", "command not supported for this interface"},
		{"not supported", "Feature not supported"},
	}

	adapter := &Adapter{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			onus := adapter.parseV1600ONUList(tt.output, "")
			if len(onus) != 0 {
				t.Errorf("expected 0 ONUs, got %d", len(onus))
			}
		})
	}
}

func TestParseV1600ONUList_SkipsInvalidSerial(t *testing.T) {
	output := `Onuindex   Model                Profile                Mode    AuthInfo
----------------------------------------------------------------------------
GPON0/1:1  unknown              default                sn      for
GPON0/1:2  unknown              default                sn      FHTT59CB8310`

	adapter := &Adapter{}
	onus := adapter.parseV1600ONUList(output, "")

	// Should skip the "for" serial but keep the valid one
	if len(onus) != 1 {
		t.Fatalf("expected 1 ONU (skipping invalid serial), got %d", len(onus))
	}
	if onus[0].Serial != "FHTT59CB8310" {
		t.Errorf("expected serial FHTT59CB8310, got %q", onus[0].Serial)
	}
}

// =============================================================================
// mergeONUState Tests
// =============================================================================

func TestMergeONUState_MatchByKey(t *testing.T) {
	onus := []types.ONUInfo{
		{PONPort: "0/1", ONUID: 1, Serial: "FHTT00000001", IsOnline: true},
		{PONPort: "0/1", ONUID: 2, Serial: "FHTT00000002", IsOnline: true},
		{PONPort: "0/2", ONUID: 1, Serial: "GPON00000001", IsOnline: true},
	}

	states := []ONUStateInfo{
		{PONPort: "0/1", ONUID: 1, AdminState: "enable", PhaseState: "working", IsOnline: true},
		{PONPort: "0/1", ONUID: 2, AdminState: "enable", PhaseState: "los", IsOnline: false},
		{PONPort: "0/2", ONUID: 1, AdminState: "disable", PhaseState: "syncmib", IsOnline: false},
	}

	adapter := &Adapter{}
	adapter.mergeONUState(onus, states)

	// ONU 0/1:1 — working → online
	if !onus[0].IsOnline {
		t.Error("ONU 0/1:1 should be online (working)")
	}
	if onus[0].OperState != "online" {
		t.Errorf("ONU 0/1:1 expected OperState=online, got %q", onus[0].OperState)
	}

	// ONU 0/1:2 — los → offline
	if onus[1].IsOnline {
		t.Error("ONU 0/1:2 should be offline (los)")
	}
	if onus[1].OperState != "los" {
		t.Errorf("ONU 0/1:2 expected OperState=los, got %q", onus[1].OperState)
	}

	// ONU 0/2:1 — syncmib with admin disabled → offline
	if onus[2].IsOnline {
		t.Error("ONU 0/2:1 should be offline (syncmib)")
	}
	if onus[2].AdminState != "disable" {
		t.Errorf("ONU 0/2:1 expected AdminState=disable, got %q", onus[2].AdminState)
	}
}

func TestMergeONUState_NoMatchLeavesDefaults(t *testing.T) {
	onus := []types.ONUInfo{
		{PONPort: "0/1", ONUID: 1, Serial: "FHTT00000001", IsOnline: true, OperState: "unknown"},
	}

	// State has a different port — no match
	states := []ONUStateInfo{
		{PONPort: "0/3", ONUID: 1, AdminState: "enable", PhaseState: "working", IsOnline: true},
	}

	adapter := &Adapter{}
	adapter.mergeONUState(onus, states)

	// ONU should retain its defaults since there's no matching state
	if !onus[0].IsOnline {
		t.Error("ONU should retain default IsOnline=true when no state match")
	}
	if onus[0].OperState != "unknown" {
		t.Errorf("expected OperState=unknown (unchanged), got %q", onus[0].OperState)
	}
}

func TestMergeONUState_PhaseStateMapping(t *testing.T) {
	tests := []struct {
		name          string
		phaseState    string
		stateIsOnline bool
		wantOnline    bool
		wantOperState string
	}{
		{"working via IsOnline", "working", true, true, "online"},
		{"working via phaseState", "working", false, true, "online"},
		{"los", "los", false, false, "los"},
		{"dying_gasp", "dying_gasp", false, false, "dying_gasp"},
		{"syncmib", "syncmib", false, false, "offline"},
		{"initial", "initial", false, false, "offline"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			onus := []types.ONUInfo{
				{PONPort: "0/1", ONUID: 1},
			}
			states := []ONUStateInfo{
				{PONPort: "0/1", ONUID: 1, PhaseState: tt.phaseState, IsOnline: tt.stateIsOnline},
			}

			adapter := &Adapter{}
			adapter.mergeONUState(onus, states)

			if onus[0].IsOnline != tt.wantOnline {
				t.Errorf("expected IsOnline=%v, got %v", tt.wantOnline, onus[0].IsOnline)
			}
			if onus[0].OperState != tt.wantOperState {
				t.Errorf("expected OperState=%q, got %q", tt.wantOperState, onus[0].OperState)
			}
		})
	}
}

func TestMergeONUState_EmptyStates(t *testing.T) {
	onus := []types.ONUInfo{
		{PONPort: "0/1", ONUID: 1, IsOnline: true},
	}

	adapter := &Adapter{}
	adapter.mergeONUState(onus, nil)

	// Should not crash, ONU retains defaults
	if !onus[0].IsOnline {
		t.Error("ONU should retain IsOnline=true with nil states")
	}
}

// =============================================================================
// parseONUOpticalInfo Tests
// =============================================================================

func TestParseONUOpticalInfo_FullOutput(t *testing.T) {
	output := `Rx optical level:             -28.530(dBm)
Tx optical level:             2.520(dBm)
Temperature:                  48.430(C)
Power feed voltage:           3.28(V)
Laser bias current:           6.220(mA)`

	adapter := &Adapter{}
	info := adapter.parseONUOpticalInfo(output)

	assertFloat(t, "RxPowerDBm", -28.53, info.RxPowerDBm)
	assertFloat(t, "TxPowerDBm", 2.52, info.TxPowerDBm)
	assertFloat(t, "Temperature", 48.43, info.Temperature)
	assertFloat(t, "Voltage", 3.28, info.Voltage)
	assertFloat(t, "BiasCurrent", 6.22, info.BiasCurrent)
}

func TestParseONUOpticalInfo_PartialOutput(t *testing.T) {
	// Only rx and tx power
	output := `Rx optical level:   -22.1(dBm)
Tx optical level:   3.0(dBm)`

	adapter := &Adapter{}
	info := adapter.parseONUOpticalInfo(output)

	assertFloat(t, "RxPowerDBm", -22.1, info.RxPowerDBm)
	assertFloat(t, "TxPowerDBm", 3.0, info.TxPowerDBm)
	assertFloat(t, "Temperature", 0, info.Temperature)
	assertFloat(t, "Voltage", 0, info.Voltage)
	assertFloat(t, "BiasCurrent", 0, info.BiasCurrent)
}

func TestParseONUOpticalInfo_Empty(t *testing.T) {
	adapter := &Adapter{}
	info := adapter.parseONUOpticalInfo("")

	if info.RxPowerDBm != 0 || info.TxPowerDBm != 0 {
		t.Error("expected zero values for empty output")
	}
}

func TestParseONUOpticalInfo_CaseInsensitive(t *testing.T) {
	output := `RX OPTICAL LEVEL:  -25.0(dBm)
TX OPTICAL LEVEL:  2.0(dBm)
TEMPERATURE:       40.0(C)
VOLTAGE:           3.30(V)
BIAS CURRENT:      10.0(mA)`

	adapter := &Adapter{}
	info := adapter.parseONUOpticalInfo(output)

	assertFloat(t, "RxPowerDBm", -25.0, info.RxPowerDBm)
	assertFloat(t, "TxPowerDBm", 2.0, info.TxPowerDBm)
	assertFloat(t, "Temperature", 40.0, info.Temperature)
	assertFloat(t, "Voltage", 3.30, info.Voltage)
	assertFloat(t, "BiasCurrent", 10.0, info.BiasCurrent)
}

// =============================================================================
// parseONUStatistics Tests
// =============================================================================

func TestParseONUStatistics_FullOutput(t *testing.T) {
	output := `Input rate(Bps):              1500
Output rate(Bps):             25000
Input bytes:                  18830
Output bytes:                 1144072
Input packets:                500
Output packets:               17484`

	adapter := &Adapter{}
	stats := adapter.parseONUStatistics(output)

	assertUint64(t, "InputRateBps", 1500, stats.InputRateBps)
	assertUint64(t, "OutputRateBps", 25000, stats.OutputRateBps)
	assertUint64(t, "InputBytes", 18830, stats.InputBytes)
	assertUint64(t, "OutputBytes", 1144072, stats.OutputBytes)
	assertUint64(t, "InputPackets", 500, stats.InputPackets)
	assertUint64(t, "OutputPackets", 17484, stats.OutputPackets)
}

func TestParseONUStatistics_ZeroValues(t *testing.T) {
	output := `Input rate(Bps):              0
Output rate(Bps):             0
Input bytes:                  0
Output bytes:                 0
Input packets:                0
Output packets:               0`

	adapter := &Adapter{}
	stats := adapter.parseONUStatistics(output)

	if stats.InputRateBps != 0 || stats.OutputRateBps != 0 ||
		stats.InputBytes != 0 || stats.OutputBytes != 0 ||
		stats.InputPackets != 0 || stats.OutputPackets != 0 {
		t.Error("expected all zero values")
	}
}

func TestParseONUStatistics_Empty(t *testing.T) {
	adapter := &Adapter{}
	stats := adapter.parseONUStatistics("")

	if stats.InputRateBps != 0 || stats.OutputRateBps != 0 {
		t.Error("expected zero values for empty output")
	}
}

func TestParseONUStatistics_LargeValues(t *testing.T) {
	output := `Input rate(Bps):              999999999
Output rate(Bps):             1000000000
Input bytes:                  18446744073709551615
Output bytes:                 9999999999999
Input packets:                4294967295
Output packets:               1234567890123`

	adapter := &Adapter{}
	stats := adapter.parseONUStatistics(output)

	assertUint64(t, "InputRateBps", 999999999, stats.InputRateBps)
	assertUint64(t, "OutputRateBps", 1000000000, stats.OutputRateBps)
	// uint64 max: 18446744073709551615
	assertUint64(t, "InputBytes", 18446744073709551615, stats.InputBytes)
	assertUint64(t, "OutputBytes", 9999999999999, stats.OutputBytes)
	assertUint64(t, "InputPackets", 4294967295, stats.InputPackets)
	assertUint64(t, "OutputPackets", 1234567890123, stats.OutputPackets)
}

// =============================================================================
// parseONURunningConfigVLAN Tests
// =============================================================================

func TestParseONURunningConfigVLAN_ServicePort(t *testing.T) {
	output := `interface GPON 0/1
  onu 1 service-port 1 gemport 1 uservlan 702 vlan 702 new_cos 0`

	adapter := &Adapter{}
	vlan := adapter.parseONURunningConfigVLAN(output)

	if vlan != 702 {
		t.Errorf("expected VLAN 702, got %d", vlan)
	}
}

func TestParseONURunningConfigVLAN_ServiceLine(t *testing.T) {
	// Fallback format when service-port line not present
	output := `interface GPON 0/1
  onu 1 service INTERNET gemport 1 vlan 500 cos 0-7`

	adapter := &Adapter{}
	vlan := adapter.parseONURunningConfigVLAN(output)

	if vlan != 500 {
		t.Errorf("expected VLAN 500, got %d", vlan)
	}
}

func TestParseONURunningConfigVLAN_ServicePortTakesPriority(t *testing.T) {
	// Both formats present — service-port should win
	output := `interface GPON 0/1
  onu 1 service-port 1 gemport 1 uservlan 100 vlan 100 new_cos 0
  onu 1 service INTERNET gemport 1 vlan 200 cos 0-7`

	adapter := &Adapter{}
	vlan := adapter.parseONURunningConfigVLAN(output)

	if vlan != 100 {
		t.Errorf("expected VLAN 100 (service-port priority), got %d", vlan)
	}
}

func TestParseONURunningConfigVLAN_NoVLAN(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{"empty", ""},
		{"no vlan line", "interface GPON 0/1\n  onu 1 profile default"},
		{"vlan zero", "onu 1 service-port 1 gemport 1 uservlan 0 vlan 0"},
	}

	adapter := &Adapter{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vlan := adapter.parseONURunningConfigVLAN(tt.output)
			if vlan != 0 {
				t.Errorf("expected VLAN 0, got %d", vlan)
			}
		})
	}
}

// =============================================================================
// parseVLANList Tests
// =============================================================================

func TestParseVLANList_TableFormat(t *testing.T) {
	output := `VLAN  Name             Type     Ports  Description
-------------------------------------------------
100   CustomerVLAN     static   2      Customer traffic
200   ManagementVLAN   smart    0      Management
1     default          static   5      Default VLAN`

	adapter := &Adapter{}
	vlans := adapter.parseVLANList(output)

	if len(vlans) != 3 {
		t.Fatalf("expected 3 VLANs, got %d", len(vlans))
	}

	if vlans[0].ID != 100 || vlans[0].Name != "CustomerVLAN" {
		t.Errorf("VLAN 0: expected ID=100 Name=CustomerVLAN, got ID=%d Name=%q", vlans[0].ID, vlans[0].Name)
	}
	if vlans[0].Type != "static" {
		t.Errorf("VLAN 0: expected Type=static, got %q", vlans[0].Type)
	}
	if vlans[0].ServicePortCount != 2 {
		t.Errorf("VLAN 0: expected ServicePortCount=2, got %d", vlans[0].ServicePortCount)
	}
	if vlans[0].Description != "Customer traffic" {
		t.Errorf("VLAN 0: expected Description=%q, got %q", "Customer traffic", vlans[0].Description)
	}

	if vlans[1].ID != 200 || vlans[1].Type != "smart" {
		t.Errorf("VLAN 1: expected ID=200 Type=smart, got ID=%d Type=%q", vlans[1].ID, vlans[1].Type)
	}
}

func TestParseVLANList_CreatedVLANsFormat(t *testing.T) {
	output := `Created VLANs
1 100 200 500`

	adapter := &Adapter{}
	vlans := adapter.parseVLANList(output)

	if len(vlans) != 4 {
		t.Fatalf("expected 4 VLANs, got %d", len(vlans))
	}

	expectedIDs := []int{1, 100, 200, 500}
	for i, expected := range expectedIDs {
		if vlans[i].ID != expected {
			t.Errorf("VLAN %d: expected ID=%d, got %d", i, expected, vlans[i].ID)
		}
		if vlans[i].Type != "static" {
			t.Errorf("VLAN %d: expected Type=static, got %q", i, vlans[i].Type)
		}
	}
}

func TestParseVLANList_Empty(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{"empty", ""},
		{"total only", "Total VLANs: 0"},
		{"header and separator only", "VLAN  Name  Type  Ports  Description\n-------------------------------------------------"},
	}

	adapter := &Adapter{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vlans := adapter.parseVLANList(tt.output)
			if len(vlans) != 0 {
				t.Errorf("expected 0 VLANs, got %d", len(vlans))
			}
		})
	}
}

func TestParseVLANList_MinimalFields(t *testing.T) {
	output := `-------------------------------------------------
100   CustomerVLAN`

	adapter := &Adapter{}
	vlans := adapter.parseVLANList(output)

	if len(vlans) != 1 {
		t.Fatalf("expected 1 VLAN, got %d", len(vlans))
	}
	if vlans[0].ID != 100 {
		t.Errorf("expected ID=100, got %d", vlans[0].ID)
	}
	if vlans[0].Name != "CustomerVLAN" {
		t.Errorf("expected Name=CustomerVLAN, got %q", vlans[0].Name)
	}
	// Type defaults to "static" when only 2 fields
	if vlans[0].Type != "static" {
		t.Errorf("expected Type=static, got %q", vlans[0].Type)
	}
}

// =============================================================================
// parseServicePortList Tests
// =============================================================================

func TestParseServicePortList_StandardOutput(t *testing.T) {
	output := `Index   VLAN    Interface     ONU     GemPort   UserVLAN
---------------------------------------------------------
1       100     0/1           5       1         100
2       200     0/1           3       2         200
3       300     0/2           1       1         0`

	adapter := &Adapter{}
	sps := adapter.parseServicePortList(output)

	if len(sps) != 3 {
		t.Fatalf("expected 3 service ports, got %d", len(sps))
	}

	// First service port
	if sps[0].Index != 1 || sps[0].VLAN != 100 || sps[0].Interface != "0/1" || sps[0].ONTID != 5 {
		t.Errorf("SP 0: got Index=%d VLAN=%d Interface=%q ONTID=%d", sps[0].Index, sps[0].VLAN, sps[0].Interface, sps[0].ONTID)
	}
	if sps[0].GemPort != 1 || sps[0].UserVLAN != 100 {
		t.Errorf("SP 0: got GemPort=%d UserVLAN=%d", sps[0].GemPort, sps[0].UserVLAN)
	}

	// Third service port — UserVLAN=0
	if sps[2].Interface != "0/2" || sps[2].ONTID != 1 {
		t.Errorf("SP 2: got Interface=%q ONTID=%d", sps[2].Interface, sps[2].ONTID)
	}
}

func TestParseServicePortList_WithTagTransform(t *testing.T) {
	output := `Index   VLAN    Interface     ONU     GemPort   UserVLAN   TagTransform
----------------------------------------------------------------------
1       100     0/1           5       1         100        translate`

	adapter := &Adapter{}
	sps := adapter.parseServicePortList(output)

	if len(sps) != 1 {
		t.Fatalf("expected 1 service port, got %d", len(sps))
	}
	if sps[0].TagTransform != "translate" {
		t.Errorf("expected TagTransform=translate, got %q", sps[0].TagTransform)
	}
}

func TestParseServicePortList_Empty(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{"empty", ""},
		{"no service ports", "No service port found"},
		{"header only", "Index   VLAN    Interface     ONU     GemPort   UserVLAN\n---------------------------------------------------------"},
		{"total only", "Total service ports: 0"},
	}

	adapter := &Adapter{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sps := adapter.parseServicePortList(tt.output)
			if len(sps) != 0 {
				t.Errorf("expected 0 service ports, got %d", len(sps))
			}
		})
	}
}

func TestParseServicePortList_MinimalFields(t *testing.T) {
	output := `---------------------------------------------------------
1       100     0/1           5`

	adapter := &Adapter{}
	sps := adapter.parseServicePortList(output)

	if len(sps) != 1 {
		t.Fatalf("expected 1 service port, got %d", len(sps))
	}
	if sps[0].Index != 1 || sps[0].VLAN != 100 || sps[0].ONTID != 5 {
		t.Errorf("got Index=%d VLAN=%d ONTID=%d", sps[0].Index, sps[0].VLAN, sps[0].ONTID)
	}
	if sps[0].GemPort != 0 || sps[0].UserVLAN != 0 {
		t.Errorf("expected zero defaults, got GemPort=%d UserVLAN=%d", sps[0].GemPort, sps[0].UserVLAN)
	}
}

// =============================================================================
// parseAlarms Tests
// =============================================================================

func TestParseAlarms_TableFormat(t *testing.T) {
	output := `ID      Severity  Type    Source     Message              Time
1       Critical  LOS     PON        Loss of signal       2024-01-15 10:30:00
2       Warning   Power   ONU        Rx power low         2024-01-15 10:35:00`

	adapter := &Adapter{}
	alarms := adapter.parseAlarms(output)

	if len(alarms) != 2 {
		t.Fatalf("expected 2 alarms, got %d", len(alarms))
	}

	if alarms[0].ID != "1" || alarms[0].Severity != "critical" || alarms[0].Type != "los" {
		t.Errorf("alarm 0: ID=%q Severity=%q Type=%q", alarms[0].ID, alarms[0].Severity, alarms[0].Type)
	}
	if alarms[0].Message != "Loss of signal" {
		t.Errorf("alarm 0: expected message %q, got %q", "Loss of signal", alarms[0].Message)
	}

	if alarms[1].ID != "2" || alarms[1].Severity != "warning" {
		t.Errorf("alarm 1: ID=%q Severity=%q", alarms[1].ID, alarms[1].Severity)
	}
}

func TestParseAlarms_OamlogFormat(t *testing.T) {
	output := `2024/01/15 10:30:00  Warning  ONU_LOS           GPON0/1:1 los detected
2024/01/15 10:35:00  Critical  PON_FAILURE       PON port 0/1 failure`

	adapter := &Adapter{}
	alarms := adapter.parseAlarms(output)

	if len(alarms) != 2 {
		t.Fatalf("expected 2 alarms, got %d", len(alarms))
	}

	// OAMlog format parses differently via parseVSolOamlog
	if alarms[0].Severity != "warning" {
		t.Errorf("expected severity=warning, got %q", alarms[0].Severity)
	}
	if alarms[0].Type != "onu_los" {
		t.Errorf("expected type=onu_los, got %q", alarms[0].Type)
	}
}

func TestParseAlarms_Empty(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{"empty", ""},
		{"header only", "ID      Severity  Type    Source     Message              Time\n-----------------------------------------------------------------"},
	}

	adapter := &Adapter{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alarms := adapter.parseAlarms(tt.output)
			if len(alarms) != 0 {
				t.Errorf("expected 0 alarms, got %d", len(alarms))
			}
		})
	}
}

// =============================================================================
// parseVSolOamlog Tests
// =============================================================================

func TestParseVSolOamlog_ThreeColumns(t *testing.T) {
	output := `2024/01/15 10:30:00  Warning  ONU_LOS           GPON0/1:1 los detected
2024/01/15 10:35:00  Critical  PON_DOWN          PON port 0/1 down`

	adapter := &Adapter{}
	alarms := adapter.parseVSolOamlog(output)

	if len(alarms) != 2 {
		t.Fatalf("expected 2 alarms, got %d", len(alarms))
	}

	if alarms[0].Severity != "warning" {
		t.Errorf("expected severity=warning, got %q", alarms[0].Severity)
	}
	if alarms[0].Type != "onu_los" {
		t.Errorf("expected type=onu_los, got %q", alarms[0].Type)
	}
	if alarms[0].RaisedAt.Year() != 2024 || alarms[0].RaisedAt.Month() != 1 || alarms[0].RaisedAt.Day() != 15 {
		t.Errorf("unexpected timestamp: %v", alarms[0].RaisedAt)
	}
	if alarms[0].Metadata == nil || alarms[0].Metadata["raw_line"] == nil {
		t.Error("expected raw_line in metadata")
	}

	if alarms[1].Severity != "critical" || alarms[1].Type != "pon_down" {
		t.Errorf("alarm 1: severity=%q type=%q", alarms[1].Severity, alarms[1].Type)
	}
}

func TestParseVSolOamlog_TwoColumns(t *testing.T) {
	output := `2024/06/01 08:00:00  system_startup         System started successfully`

	adapter := &Adapter{}
	alarms := adapter.parseVSolOamlog(output)

	if len(alarms) != 1 {
		t.Fatalf("expected 1 alarm, got %d", len(alarms))
	}

	// With 2 columns: eventName + message, severity defaults to unknown
	if alarms[0].Severity != "unknown" {
		t.Errorf("expected severity=unknown, got %q", alarms[0].Severity)
	}
}

func TestParseVSolOamlog_SingleColumn(t *testing.T) {
	output := `2024/06/01 08:00:00  reboot`

	adapter := &Adapter{}
	alarms := adapter.parseVSolOamlog(output)

	if len(alarms) != 1 {
		t.Fatalf("expected 1 alarm, got %d", len(alarms))
	}

	if alarms[0].Message != "reboot" {
		t.Errorf("expected message=reboot, got %q", alarms[0].Message)
	}
}

func TestParseVSolOamlog_SkipsInvalidTimestamp(t *testing.T) {
	output := `not-a-date 10:30:00  Warning  ONU_LOS  los detected
2024/01/15 10:35:00  Critical  PON_DOWN  down`

	adapter := &Adapter{}
	alarms := adapter.parseVSolOamlog(output)

	// First line has invalid date, should be skipped
	if len(alarms) != 1 {
		t.Fatalf("expected 1 alarm (skip invalid timestamp), got %d", len(alarms))
	}
	if alarms[0].Severity != "critical" {
		t.Errorf("expected severity=critical, got %q", alarms[0].Severity)
	}
}

func TestParseVSolOamlog_Empty(t *testing.T) {
	adapter := &Adapter{}
	alarms := adapter.parseVSolOamlog("")

	if len(alarms) != 0 {
		t.Errorf("expected 0 alarms, got %d", len(alarms))
	}
}

// =============================================================================
// detectONUVendor Tests
// =============================================================================

func TestDetectONUVendor(t *testing.T) {
	tests := []struct {
		serial   string
		expected string
	}{
		{"FHTT59CB8310", "FiberHome"},
		{"HWTC12345678", "Huawei"},
		{"GPON00929978", "Generic"}, // GPON prefix maps to Generic
		{"FH", ""},                  // too short
		{"", ""},                    // empty
	}

	for _, tt := range tests {
		t.Run(tt.serial, func(t *testing.T) {
			got := detectONUVendor(tt.serial)
			if got != tt.expected {
				t.Errorf("detectONUVendor(%q) = %q, want %q", tt.serial, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// Test Helpers
// =============================================================================

func assertONUState(t *testing.T, state ONUStateInfo, wantPort string, wantID int, wantAdmin, wantPhase string, wantOnline bool) {
	t.Helper()
	if state.PONPort != wantPort {
		t.Errorf("PONPort: expected %q, got %q", wantPort, state.PONPort)
	}
	if state.ONUID != wantID {
		t.Errorf("ONUID: expected %d, got %d", wantID, state.ONUID)
	}
	if state.AdminState != wantAdmin {
		t.Errorf("AdminState: expected %q, got %q", wantAdmin, state.AdminState)
	}
	if state.PhaseState != wantPhase {
		t.Errorf("PhaseState: expected %q, got %q", wantPhase, state.PhaseState)
	}
	if state.IsOnline != wantOnline {
		t.Errorf("IsOnline: expected %v, got %v", wantOnline, state.IsOnline)
	}
}

func assertONUInfo(t *testing.T, onu types.ONUInfo, wantPort string, wantID int, wantSerial, wantModel, wantProfile string) {
	t.Helper()
	if onu.PONPort != wantPort {
		t.Errorf("PONPort: expected %q, got %q", wantPort, onu.PONPort)
	}
	if onu.ONUID != wantID {
		t.Errorf("ONUID: expected %d, got %d", wantID, onu.ONUID)
	}
	if onu.Serial != wantSerial {
		t.Errorf("Serial: expected %q, got %q", wantSerial, onu.Serial)
	}
	if onu.Model != wantModel {
		t.Errorf("Model: expected %q, got %q", wantModel, onu.Model)
	}
	if onu.ONUProfile != wantProfile {
		t.Errorf("ONUProfile: expected %q, got %q", wantProfile, onu.ONUProfile)
	}
}

func assertFloat(t *testing.T, name string, expected, got float64) {
	t.Helper()
	diff := expected - got
	if diff < -0.001 || diff > 0.001 {
		t.Errorf("%s: expected %v, got %v", name, expected, got)
	}
}

func assertUint64(t *testing.T, name string, expected, got uint64) {
	t.Helper()
	if expected != got {
		t.Errorf("%s: expected %d, got %d", name, expected, got)
	}
}
