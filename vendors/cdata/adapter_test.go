package cdata

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/testutil"
	"github.com/nanoncore/nano-southbound/types"
)

// Compile-time interface compliance check.
var _ types.Driver = (*Adapter)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newGPONConfig returns an EquipmentConfig with pon_type=gpon.
func newGPONConfig() *types.EquipmentConfig {
	cfg := testutil.NewTestEquipmentConfig(types.VendorCData, "10.0.0.1")
	cfg.Metadata["model"] = "fd1208s"
	cfg.Metadata["pon_type"] = "gpon"
	return cfg
}

// newEPONConfig returns an EquipmentConfig with pon_type=epon.
func newEPONConfig() *types.EquipmentConfig {
	cfg := testutil.NewTestEquipmentConfig(types.VendorCData, "10.0.0.1")
	cfg.Metadata["model"] = "fd1104s"
	cfg.Metadata["pon_type"] = "epon"
	return cfg
}

// newSubscriber builds a subscriber with C-Data-specific annotations.
func newSubscriber(serial, ponPort string, vlan int, onuID string, onuType string) *model.Subscriber {
	sub := testutil.NewTestSubscriber(serial, ponPort, vlan)
	// Override annotations to use the C-Data annotation keys.
	sub.Annotations = map[string]string{
		"nanoncore.com/pon-port": ponPort,
		"nanoncore.com/onu-id":   onuID,
		"nanoncore.com/onu-type": onuType,
	}
	return sub
}

// newTier builds a service tier with optional custom profile annotations.
func newTier(up, down int, lineProfile, serviceProfile string) *model.ServiceTier {
	tier := testutil.NewTestServiceTier(up, down)
	if lineProfile != "" || serviceProfile != "" {
		tier.Annotations = map[string]string{}
		if lineProfile != "" {
			tier.Annotations["nanoncore.com/line-profile"] = lineProfile
		}
		if serviceProfile != "" {
			tier.Annotations["nanoncore.com/service-profile"] = serviceProfile
		}
	}
	return tier
}

// cliMockDriver returns a MockDriver with a working CLIExecutor.
func cliMockDriver(outputs map[string]string) *testutil.MockDriver {
	return &testutil.MockDriver{
		Connected: true,
		CLIExec: &testutil.MockCLIExecutor{
			Outputs: outputs,
		},
	}
}

// ---------------------------------------------------------------------------
// NewAdapter
// ---------------------------------------------------------------------------

func TestNewAdapter_WithCLIExecutor(t *testing.T) {
	mock := cliMockDriver(nil)
	cfg := newGPONConfig()

	adapter := NewAdapter(mock, cfg)
	if adapter == nil {
		t.Fatal("NewAdapter returned nil")
	}

	a, ok := adapter.(*Adapter)
	if !ok {
		t.Fatal("expected *Adapter")
	}
	if a.cliExecutor == nil {
		t.Fatal("expected cliExecutor to be set when base driver implements CLIExecutor")
	}
}

func TestNewAdapter_WithoutCLIExecutor(t *testing.T) {
	// simpleDriver has no CLIExecutor implementation.
	base := &simpleDriver{}
	cfg := newGPONConfig()

	adapter := NewAdapter(base, cfg)
	if adapter == nil {
		t.Fatal("NewAdapter returned nil")
	}
	a := adapter.(*Adapter)
	if a.cliExecutor != nil {
		t.Fatal("expected cliExecutor to be nil when base driver has no CLIExecutor")
	}
}

// simpleDriver is a minimal Driver with no CLIExecutor capability.
type simpleDriver struct{ connected bool }

func (d *simpleDriver) Connect(context.Context, *types.EquipmentConfig) error             { d.connected = true; return nil }
func (d *simpleDriver) Disconnect(context.Context) error                                   { d.connected = false; return nil }
func (d *simpleDriver) IsConnected() bool                                                  { return d.connected }
func (d *simpleDriver) CreateSubscriber(context.Context, *model.Subscriber, *model.ServiceTier) (*types.SubscriberResult, error) { return nil, nil }
func (d *simpleDriver) UpdateSubscriber(context.Context, *model.Subscriber, *model.ServiceTier) error { return nil }
func (d *simpleDriver) DeleteSubscriber(context.Context, string) error                     { return nil }
func (d *simpleDriver) SuspendSubscriber(context.Context, string) error                    { return nil }
func (d *simpleDriver) ResumeSubscriber(context.Context, string) error                     { return nil }
func (d *simpleDriver) GetSubscriberStatus(context.Context, string) (*types.SubscriberStatus, error) { return nil, nil }
func (d *simpleDriver) GetSubscriberStats(context.Context, string) (*types.SubscriberStats, error)   { return nil, nil }
func (d *simpleDriver) HealthCheck(context.Context) error                                  { return nil }

// ---------------------------------------------------------------------------
// Helper methods (table-driven)
// ---------------------------------------------------------------------------

func TestDetectModel(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		want     string
	}{
		{"explicit model", map[string]string{"model": "fd1616s"}, "fd1616s"},
		{"default", map[string]string{}, "fd1104s"},
		{"nil-safe", nil, "fd1104s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testutil.NewTestEquipmentConfig(types.VendorCData, "10.0.0.1")
			if tt.metadata != nil {
				cfg.Metadata = tt.metadata
			} else {
				cfg.Metadata = map[string]string{}
			}
			a := &Adapter{config: cfg}
			if got := a.detectModel(); got != tt.want {
				t.Errorf("detectModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectPONType(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		want     string
	}{
		{"gpon explicit", map[string]string{"pon_type": "gpon"}, "gpon"},
		{"epon explicit", map[string]string{"pon_type": "epon"}, "epon"},
		{"default is gpon", map[string]string{}, "gpon"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testutil.NewTestEquipmentConfig(types.VendorCData, "10.0.0.1")
			cfg.Metadata = tt.metadata
			a := &Adapter{config: cfg}
			if got := a.detectPONType(); got != tt.want {
				t.Errorf("detectPONType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetPONPort(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        string
	}{
		{"from annotation", map[string]string{"nanoncore.com/pon-port": "1/2/3"}, "1/2/3"},
		{"default", map[string]string{}, "1/1/1"},
		{"nil annotations", nil, "1/1/1"},
	}

	a := &Adapter{config: newGPONConfig()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &model.Subscriber{Name: "test", Annotations: tt.annotations}
			if got := a.getPONPort(sub); got != tt.want {
				t.Errorf("getPONPort() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetONUID(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		vlan        int
		want        int
	}{
		{"from annotation", map[string]string{"nanoncore.com/onu-id": "42"}, 100, 42},
		{"invalid annotation falls back", map[string]string{"nanoncore.com/onu-id": "abc"}, 200, 200 % 128},
		{"default from vlan", map[string]string{}, 300, 300 % 128},
		{"nil annotations", nil, 500, 500 % 128},
	}

	a := &Adapter{config: newGPONConfig()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &model.Subscriber{
				Name:        "test",
				Annotations: tt.annotations,
				Spec:        model.SubscriberSpec{VLAN: tt.vlan},
			}
			if got := a.getONUID(sub); got != tt.want {
				t.Errorf("getONUID() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetONUType(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        string
	}{
		{"custom type", map[string]string{"nanoncore.com/onu-type": "bridge"}, "bridge"},
		{"default", map[string]string{}, "router"},
		{"nil annotations", nil, "router"},
	}

	a := &Adapter{config: newGPONConfig()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &model.Subscriber{Name: "test", Annotations: tt.annotations}
			if got := a.getONUType(sub); got != tt.want {
				t.Errorf("getONUType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetLineProfile(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		bwDown      int
		bwUp        int
		want        string
	}{
		{"custom profile", map[string]string{"nanoncore.com/line-profile": "my_line"}, 100, 50, "my_line"},
		{"generated", nil, 100, 50, "line_100M_50M"},
		{"generated small", nil, 10, 5, "line_10M_5M"},
	}

	a := &Adapter{config: newGPONConfig()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := &model.ServiceTier{
				Name:        "t",
				Annotations: tt.annotations,
				Spec:        model.ServiceTierSpec{BandwidthDown: tt.bwDown, BandwidthUp: tt.bwUp},
			}
			if got := a.getLineProfile(tier); got != tt.want {
				t.Errorf("getLineProfile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetServiceProfile(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        string
	}{
		{"custom", map[string]string{"nanoncore.com/service-profile": "svc_voip"}, "svc_voip"},
		{"default", nil, "service_internet"},
	}

	a := &Adapter{config: newGPONConfig()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := &model.ServiceTier{Name: "t", Annotations: tt.annotations}
			if got := a.getServiceProfile(tier); got != tt.want {
				t.Errorf("getServiceProfile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSubscriberID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		wantPort string
		wantONU  int
	}{
		{"standard format", "onu-1/1/1-5", "1/1/1", 5},
		{"different port", "onu-1/2/3-42", "1/2/3", 42},
		{"fallback hashes", "some-subscriber", "1/1/1", func() int {
			h := 0
			for _, c := range "some-subscriber" {
				h = (h*31 + int(c)) % 128
			}
			return h
		}()},
	}

	a := &Adapter{config: newGPONConfig()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, onu := a.parseSubscriberID(tt.id)
			if port != tt.wantPort {
				t.Errorf("port = %q, want %q", port, tt.wantPort)
			}
			if onu != tt.wantONU {
				t.Errorf("onuID = %d, want %d", onu, tt.wantONU)
			}
		})
	}
}

func TestExtractPortFromInterface(t *testing.T) {
	tests := []struct {
		iface string
		want  string
	}{
		{"gpon-olt_1/1/1", "1/1/1"},
		{"epon-olt_1/2/4", "1/2/4"},
		{"unknown-format", "unknown-format"},
		{"gpon-olt_0/0/0", "0/0/0"},
	}

	a := &Adapter{config: newGPONConfig()}
	for _, tt := range tests {
		t.Run(tt.iface, func(t *testing.T) {
			if got := a.extractPortFromInterface(tt.iface); got != tt.want {
				t.Errorf("extractPortFromInterface(%q) = %q, want %q", tt.iface, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Command building
// ---------------------------------------------------------------------------

func TestBuildGPONCommands(t *testing.T) {
	a := &Adapter{config: newGPONConfig()}
	sub := newSubscriber("CDAT12345678", "1/1/2", 100, "5", "router")
	tier := newTier(50, 100, "line_100M_50M", "service_internet")

	cmds := a.buildGPONCommands("1/1/2", 5, "CDAT12345678", 100, 100, 50, sub, tier)

	expected := []string{
		"configure terminal",
		"interface gpon-olt_1/1/2",
		"onu-set 5 type router sn CDAT12345678",
		"onu-profile 5 line line_100M_50M service service_internet",
		"onu-vlan 5 mode translate user-vlan 100 svlan 100",
		"onu-ratelimit 5 upstream 50000 downstream 100000",
		"onu-activate 5",
		"exit",
		"commit",
		"end",
	}

	if len(cmds) != len(expected) {
		t.Fatalf("got %d commands, want %d", len(cmds), len(expected))
	}
	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("cmd[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

func TestBuildEPONCommands(t *testing.T) {
	a := &Adapter{config: newEPONConfig()}
	sub := newSubscriber("AA:BB:CC:DD:EE:FF", "1/1/3", 200, "10", "bridge")
	tier := newTier(25, 50, "", "") // no custom profiles

	cmds := a.buildEPONCommands("1/1/3", 10, "AA:BB:CC:DD:EE:FF", 200, 50, 25, sub, tier)

	expected := []string{
		"configure terminal",
		"interface epon-olt_1/1/3",
		"onu-set 10 mac AA:BB:CC:DD:EE:FF",
		"onu-profile 10 line line_50M_25M service service_internet",
		"onu-vlan 10 mode translate user-vlan 200 svlan 200",
		"onu-ratelimit 10 upstream 25000 downstream 50000",
		"onu-activate 10",
		"exit",
		"commit",
		"end",
	}

	if len(cmds) != len(expected) {
		t.Fatalf("got %d commands, want %d", len(cmds), len(expected))
	}
	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("cmd[%d] = %q, want %q", i, cmds[i], want)
		}
	}
}

// ---------------------------------------------------------------------------
// Parsers
// ---------------------------------------------------------------------------

func TestParseONUStatus(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantState string
		wantOnline bool
	}{
		{
			"online",
			"ONU Status: Online\nUptime: 86400\nRx_Power: -18.5\nTx_Power: 2.3",
			"online", true,
		},
		{
			"working",
			"State: Working\nuptime: 3600",
			"online", true,
		},
		{
			"offline",
			"ONU Status: Offline (LOS)\nRx_Power: N/A",
			"offline", false,
		},
		{
			"los",
			"State: LOS detected",
			"offline", false,
		},
		{
			"deactivated",
			"ONU is Deactivated by admin",
			"suspended", false,
		},
		{
			"disabled",
			"State: Disabled",
			"suspended", false,
		},
		{
			"unknown state",
			"Some random output with no recognizable state",
			"unknown", false,
		},
	}

	a := &Adapter{config: newGPONConfig()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := a.parseONUStatus(tt.output, "sub-1")

			if status.SubscriberID != "sub-1" {
				t.Errorf("SubscriberID = %q, want %q", status.SubscriberID, "sub-1")
			}
			if status.State != tt.wantState {
				t.Errorf("State = %q, want %q", status.State, tt.wantState)
			}
			if status.IsOnline != tt.wantOnline {
				t.Errorf("IsOnline = %v, want %v", status.IsOnline, tt.wantOnline)
			}
			if status.Metadata["cli_output"] != tt.output {
				t.Error("expected cli_output in metadata")
			}
		})
	}
}

func TestParseONUStatus_Uptime(t *testing.T) {
	a := &Adapter{config: newGPONConfig()}
	// The uptime regex matches on the raw output (not lowercased),
	// so use lowercase "uptime" to match the pattern `uptime[:\s]+(\d+)`.
	output := "ONU Status: Online\nuptime: 86400"
	status := a.parseONUStatus(output, "sub-2")
	if status.UptimeSeconds != 86400 {
		t.Errorf("UptimeSeconds = %d, want 86400", status.UptimeSeconds)
	}
}

func TestParseONUStatus_OpticalPower(t *testing.T) {
	a := &Adapter{config: newGPONConfig()}
	output := "ONU Status: Online\nrx_power: -18.5\ntx_power: 2.3"
	status := a.parseONUStatus(output, "sub-3")
	if status.Metadata["rx_power_dbm"] != "-18.5" {
		t.Errorf("rx_power_dbm = %v, want -18.5", status.Metadata["rx_power_dbm"])
	}
	if status.Metadata["tx_power_dbm"] != "2.3" {
		t.Errorf("tx_power_dbm = %v, want 2.3", status.Metadata["tx_power_dbm"])
	}
}

func TestParseONUStats(t *testing.T) {
	a := &Adapter{config: newGPONConfig()}
	output := `ONU Statistics:
Rx_Bytes: 123456789
Tx_Bytes: 987654321
Rx_Packets: 100000
Tx_Packets: 200000
Errors: 42
Drops: 7`

	stats := a.parseONUStats(output)
	if stats.BytesDown != 123456789 {
		t.Errorf("BytesDown = %d, want 123456789", stats.BytesDown)
	}
	if stats.BytesUp != 987654321 {
		t.Errorf("BytesUp = %d, want 987654321", stats.BytesUp)
	}
	if stats.PacketsDown != 100000 {
		t.Errorf("PacketsDown = %d, want 100000", stats.PacketsDown)
	}
	if stats.PacketsUp != 200000 {
		t.Errorf("PacketsUp = %d, want 200000", stats.PacketsUp)
	}
	if stats.ErrorsDown != 42 {
		t.Errorf("ErrorsDown = %d, want 42", stats.ErrorsDown)
	}
	if stats.Drops != 7 {
		t.Errorf("Drops = %d, want 7", stats.Drops)
	}
	if stats.Metadata["cli_output"] != output {
		t.Error("expected cli_output in metadata")
	}
}

func TestParseONUStats_Empty(t *testing.T) {
	a := &Adapter{config: newGPONConfig()}
	stats := a.parseONUStats("")
	if stats.BytesDown != 0 || stats.BytesUp != 0 {
		t.Error("expected zero counters for empty output")
	}
}

func TestParseAutofindOutput(t *testing.T) {
	a := &Adapter{config: newGPONConfig()}
	output := `Interface       SN              Distance  RxPower
-----------------------------------------------------
gpon-olt_1/1/1  CDAT12345678    1234      -18.5
gpon-olt_1/1/2  CDAT87654321    567       -22.1`

	discoveries := a.parseAutofindOutput(output)
	if len(discoveries) != 2 {
		t.Fatalf("expected 2 discoveries, got %d", len(discoveries))
	}

	d0 := discoveries[0]
	if d0.PONPort != "1/1/1" {
		t.Errorf("d0.PONPort = %q, want %q", d0.PONPort, "1/1/1")
	}
	if d0.Serial != "CDAT12345678" {
		t.Errorf("d0.Serial = %q, want %q", d0.Serial, "CDAT12345678")
	}
	if d0.DistanceM != 1234 {
		t.Errorf("d0.DistanceM = %d, want 1234", d0.DistanceM)
	}
	if d0.RxPowerDBm != -18.5 {
		t.Errorf("d0.RxPowerDBm = %f, want -18.5", d0.RxPowerDBm)
	}

	d1 := discoveries[1]
	if d1.PONPort != "1/1/2" {
		t.Errorf("d1.PONPort = %q, want %q", d1.PONPort, "1/1/2")
	}
	if d1.Serial != "CDAT87654321" {
		t.Errorf("d1.Serial = %q", d1.Serial)
	}
	if d1.DistanceM != 567 {
		t.Errorf("d1.DistanceM = %d, want 567", d1.DistanceM)
	}
	if d1.RxPowerDBm != -22.1 {
		t.Errorf("d1.RxPowerDBm = %f, want -22.1", d1.RxPowerDBm)
	}
}

func TestParseAutofindOutput_Empty(t *testing.T) {
	a := &Adapter{config: newGPONConfig()}
	discoveries := a.parseAutofindOutput("")
	if len(discoveries) != 0 {
		t.Fatalf("expected 0 discoveries, got %d", len(discoveries))
	}
}

func TestParseAutofindOutput_HeaderOnly(t *testing.T) {
	a := &Adapter{config: newGPONConfig()}
	output := "Interface       SN              Distance  RxPower\n-----"
	discoveries := a.parseAutofindOutput(output)
	if len(discoveries) != 0 {
		t.Fatalf("expected 0 discoveries for header-only output, got %d", len(discoveries))
	}
}

func TestParseAutofindOutput_MinimalFields(t *testing.T) {
	a := &Adapter{config: newGPONConfig()}
	output := "gpon-olt_1/1/4  CDATABCD1234"
	discoveries := a.parseAutofindOutput(output)
	if len(discoveries) != 1 {
		t.Fatalf("expected 1 discovery, got %d", len(discoveries))
	}
	if discoveries[0].DistanceM != 0 {
		t.Errorf("expected 0 distance for minimal output, got %d", discoveries[0].DistanceM)
	}
}

// ---------------------------------------------------------------------------
// Driver methods with MockCLIExecutor
// ---------------------------------------------------------------------------

func TestCreateSubscriber_GPON_Success(t *testing.T) {
	cfg := newGPONConfig()
	verifyCmd := "show gpon onu-info gpon-olt_1/1/2 5"

	mock := cliMockDriver(map[string]string{
		verifyCmd: "ONU 5: Online, SN: CDAT12345678",
	})

	adapter := NewAdapter(mock, cfg)
	sub := newSubscriber("CDAT12345678", "1/1/2", 100, "5", "router")
	tier := newTier(50, 100, "line_100M_50M", "service_internet")

	result, err := adapter.CreateSubscriber(context.Background(), sub, tier)
	if err != nil {
		t.Fatalf("CreateSubscriber failed: %v", err)
	}
	if result.SubscriberID != sub.Name {
		t.Errorf("SubscriberID = %q, want %q", result.SubscriberID, sub.Name)
	}
	if result.SessionID != "onu-1/1/2-5" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "onu-1/1/2-5")
	}
	if result.VLAN != 100 {
		t.Errorf("VLAN = %d, want 100", result.VLAN)
	}
	if result.Metadata["vendor"] != "cdata" {
		t.Errorf("vendor = %v, want cdata", result.Metadata["vendor"])
	}
	if result.Metadata["pon_type"] != "gpon" {
		t.Errorf("pon_type = %v, want gpon", result.Metadata["pon_type"])
	}
	if result.Metadata["model"] != "fd1208s" {
		t.Errorf("model = %v, want fd1208s", result.Metadata["model"])
	}

	// Verify the correct commands were sent.
	cmds := mock.CLIExec.Commands
	if len(cmds) == 0 {
		t.Fatal("no commands were executed")
	}
	// First batch: the GPON provisioning commands (10) + verify command (1).
	foundConfigTerminal := false
	foundOnuSet := false
	for _, c := range cmds {
		if c == "configure terminal" {
			foundConfigTerminal = true
		}
		if strings.Contains(c, "onu-set 5 type router sn CDAT12345678") {
			foundOnuSet = true
		}
	}
	if !foundConfigTerminal {
		t.Error("missing 'configure terminal' command")
	}
	if !foundOnuSet {
		t.Error("missing 'onu-set' command")
	}
}

func TestCreateSubscriber_EPON_Success(t *testing.T) {
	cfg := newEPONConfig()
	verifyCmd := "show epon onu-info epon-olt_1/1/3 10"

	mock := cliMockDriver(map[string]string{
		verifyCmd: "ONU 10: Registered, MAC: AA:BB:CC:DD:EE:FF",
	})

	adapter := NewAdapter(mock, cfg)
	sub := newSubscriber("AA:BB:CC:DD:EE:FF", "1/1/3", 200, "10", "bridge")
	tier := newTier(25, 50, "", "")

	result, err := adapter.CreateSubscriber(context.Background(), sub, tier)
	if err != nil {
		t.Fatalf("CreateSubscriber EPON failed: %v", err)
	}
	if result.Metadata["pon_type"] != "epon" {
		t.Errorf("pon_type = %v, want epon", result.Metadata["pon_type"])
	}

	// Verify EPON-specific commands were used.
	foundEPONInterface := false
	for _, c := range mock.CLIExec.Commands {
		if strings.Contains(c, "interface epon-olt_1/1/3") {
			foundEPONInterface = true
		}
	}
	if !foundEPONInterface {
		t.Error("missing EPON interface command")
	}
}

func TestCreateSubscriber_NoCLI(t *testing.T) {
	base := &simpleDriver{}
	cfg := newGPONConfig()
	adapter := NewAdapter(base, cfg)

	sub := newSubscriber("CDAT12345678", "1/1/1", 100, "1", "router")
	tier := newTier(50, 100, "", "")

	_, err := adapter.CreateSubscriber(context.Background(), sub, tier)
	if err == nil {
		t.Fatal("expected error when CLI executor is not available")
	}
	if !strings.Contains(err.Error(), "CLI executor not available") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateSubscriber_VerifyFails(t *testing.T) {
	cfg := newGPONConfig()
	verifyCmd := "show gpon onu-info gpon-olt_1/1/1 1"

	mock := cliMockDriver(map[string]string{
		verifyCmd: "Error: ONU not found at this location",
	})

	adapter := NewAdapter(mock, cfg)
	sub := newSubscriber("CDAT12345678", "1/1/1", 100, "1", "router")
	tier := newTier(50, 100, "", "")

	_, err := adapter.CreateSubscriber(context.Background(), sub, tier)
	if err == nil {
		t.Fatal("expected error when ONU verification fails")
	}
	if !strings.Contains(err.Error(), "verification failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCreateSubscriber_CLIError(t *testing.T) {
	cfg := newGPONConfig()
	mock := &testutil.MockDriver{
		Connected: true,
		CLIExec: &testutil.MockCLIExecutor{
			Errors: map[string]error{
				"configure terminal": fmt.Errorf("timeout"),
			},
		},
	}

	adapter := NewAdapter(mock, cfg)
	sub := newSubscriber("CDAT12345678", "1/1/1", 100, "1", "router")
	tier := newTier(50, 100, "", "")

	_, err := adapter.CreateSubscriber(context.Background(), sub, tier)
	if err == nil {
		t.Fatal("expected error on CLI failure")
	}
	// Should be a TranslatedError.
	te, ok := err.(*TranslatedError)
	if !ok {
		t.Fatalf("expected *TranslatedError, got %T", err)
	}
	if te.Code != ErrTimeout {
		t.Errorf("error code = %q, want %q", te.Code, ErrTimeout)
	}
}

func TestDeleteSubscriber_GPON(t *testing.T) {
	cfg := newGPONConfig()
	mock := cliMockDriver(nil)
	adapter := NewAdapter(mock, cfg)

	err := adapter.DeleteSubscriber(context.Background(), "onu-1/1/2-5")
	if err != nil {
		t.Fatalf("DeleteSubscriber failed: %v", err)
	}

	foundNoOnuSet := false
	for _, c := range mock.CLIExec.Commands {
		if c == "no onu-set 5" {
			foundNoOnuSet = true
		}
	}
	if !foundNoOnuSet {
		t.Error("missing 'no onu-set 5' command")
	}
}

func TestDeleteSubscriber_EPON(t *testing.T) {
	cfg := newEPONConfig()
	mock := cliMockDriver(nil)
	adapter := NewAdapter(mock, cfg)

	err := adapter.DeleteSubscriber(context.Background(), "onu-1/1/3-10")
	if err != nil {
		t.Fatalf("DeleteSubscriber EPON failed: %v", err)
	}

	foundEPON := false
	for _, c := range mock.CLIExec.Commands {
		if strings.Contains(c, "epon-olt_1/1/3") {
			foundEPON = true
		}
	}
	if !foundEPON {
		t.Error("missing EPON interface command in delete")
	}
}

func TestDeleteSubscriber_NoCLI(t *testing.T) {
	base := &simpleDriver{}
	adapter := NewAdapter(base, newGPONConfig())

	err := adapter.DeleteSubscriber(context.Background(), "onu-1/1/1-1")
	if err == nil {
		t.Fatal("expected error when CLI executor is not available")
	}
}

func TestSuspendSubscriber_GPON(t *testing.T) {
	cfg := newGPONConfig()
	mock := cliMockDriver(nil)
	adapter := NewAdapter(mock, cfg)

	err := adapter.SuspendSubscriber(context.Background(), "onu-1/1/1-3")
	if err != nil {
		t.Fatalf("SuspendSubscriber failed: %v", err)
	}

	foundDeactivate := false
	for _, c := range mock.CLIExec.Commands {
		if c == "onu-deactivate 3" {
			foundDeactivate = true
		}
	}
	if !foundDeactivate {
		t.Error("missing 'onu-deactivate 3' command")
	}
}

func TestSuspendSubscriber_NoCLI(t *testing.T) {
	base := &simpleDriver{}
	adapter := NewAdapter(base, newGPONConfig())

	err := adapter.SuspendSubscriber(context.Background(), "onu-1/1/1-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResumeSubscriber_GPON(t *testing.T) {
	cfg := newGPONConfig()
	mock := cliMockDriver(nil)
	adapter := NewAdapter(mock, cfg)

	err := adapter.ResumeSubscriber(context.Background(), "onu-1/1/1-3")
	if err != nil {
		t.Fatalf("ResumeSubscriber failed: %v", err)
	}

	foundActivate := false
	for _, c := range mock.CLIExec.Commands {
		if c == "onu-activate 3" {
			foundActivate = true
		}
	}
	if !foundActivate {
		t.Error("missing 'onu-activate 3' command")
	}
}

func TestResumeSubscriber_NoCLI(t *testing.T) {
	base := &simpleDriver{}
	adapter := NewAdapter(base, newGPONConfig())

	err := adapter.ResumeSubscriber(context.Background(), "onu-1/1/1-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetSubscriberStatus_Success(t *testing.T) {
	cfg := newGPONConfig()
	showCmd := "show gpon onu-info gpon-olt_1/1/2 5"

	mock := cliMockDriver(map[string]string{
		showCmd: "ONU Status: Online\nuptime: 7200\nrx_power: -19.2\ntx_power: 2.1",
	})

	adapter := NewAdapter(mock, cfg)
	status, err := adapter.GetSubscriberStatus(context.Background(), "onu-1/1/2-5")
	if err != nil {
		t.Fatalf("GetSubscriberStatus failed: %v", err)
	}
	if status.State != "online" {
		t.Errorf("State = %q, want online", status.State)
	}
	if !status.IsOnline {
		t.Error("expected IsOnline = true")
	}
	if status.UptimeSeconds != 7200 {
		t.Errorf("UptimeSeconds = %d, want 7200", status.UptimeSeconds)
	}
}

func TestGetSubscriberStatus_NoCLI(t *testing.T) {
	base := &simpleDriver{}
	adapter := NewAdapter(base, newGPONConfig())

	_, err := adapter.GetSubscriberStatus(context.Background(), "onu-1/1/1-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetSubscriberStats_Success(t *testing.T) {
	cfg := newGPONConfig()
	showCmd := "show gpon onu-statistics gpon-olt_1/1/2 5"

	mock := cliMockDriver(map[string]string{
		showCmd: "Rx_Bytes: 5000\nTx_Bytes: 3000\nRx_Packets: 100\nTx_Packets: 50\nErrors: 2\nDrops: 1",
	})

	adapter := NewAdapter(mock, cfg)
	stats, err := adapter.GetSubscriberStats(context.Background(), "onu-1/1/2-5")
	if err != nil {
		t.Fatalf("GetSubscriberStats failed: %v", err)
	}
	if stats.BytesDown != 5000 {
		t.Errorf("BytesDown = %d, want 5000", stats.BytesDown)
	}
	if stats.BytesUp != 3000 {
		t.Errorf("BytesUp = %d, want 3000", stats.BytesUp)
	}
	if stats.PacketsDown != 100 {
		t.Errorf("PacketsDown = %d, want 100", stats.PacketsDown)
	}
	if stats.PacketsUp != 50 {
		t.Errorf("PacketsUp = %d, want 50", stats.PacketsUp)
	}
}

func TestGetSubscriberStats_NoCLI(t *testing.T) {
	base := &simpleDriver{}
	adapter := NewAdapter(base, newGPONConfig())

	_, err := adapter.GetSubscriberStats(context.Background(), "onu-1/1/1-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHealthCheck_WithCLI(t *testing.T) {
	cfg := newGPONConfig()
	mock := cliMockDriver(map[string]string{
		"show system": "System: FD1208S\nVersion: 3.2.1\nUptime: 864000",
	})

	adapter := NewAdapter(mock, cfg)
	err := adapter.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	foundShowSystem := false
	for _, c := range mock.CLIExec.Commands {
		if c == "show system" {
			foundShowSystem = true
		}
	}
	if !foundShowSystem {
		t.Error("missing 'show system' command")
	}
}

func TestHealthCheck_WithoutCLI_DelegatesToBase(t *testing.T) {
	base := &simpleDriver{connected: true}
	adapter := NewAdapter(base, newGPONConfig())

	// simpleDriver.HealthCheck always returns nil.
	err := adapter.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck should delegate to base: %v", err)
	}
}

func TestHealthCheck_CLIError(t *testing.T) {
	cfg := newGPONConfig()
	mock := &testutil.MockDriver{
		Connected: true,
		CLIExec: &testutil.MockCLIExecutor{
			Errors: map[string]error{
				"show system": fmt.Errorf("connection reset"),
			},
		},
	}

	adapter := NewAdapter(mock, cfg)
	err := adapter.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("expected error on CLI failure")
	}
}

func TestDiscoverONUs_Success(t *testing.T) {
	cfg := newGPONConfig()
	mock := cliMockDriver(map[string]string{
		"show gpon onu autofind": `Interface       SN              Distance  RxPower
-----------------------------------------------------
gpon-olt_1/1/1  CDAT11111111    100       -17.0
gpon-olt_1/1/2  CDAT22222222    200       -20.0
gpon-olt_1/1/1  CDAT33333333    150       -19.5`,
	})

	adapter := NewAdapter(mock, cfg).(*Adapter)

	// Discover all ports.
	discoveries, err := adapter.DiscoverONUs(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverONUs failed: %v", err)
	}
	if len(discoveries) != 3 {
		t.Fatalf("expected 3 discoveries, got %d", len(discoveries))
	}
}

func TestDiscoverONUs_FilterByPort(t *testing.T) {
	cfg := newGPONConfig()
	mock := cliMockDriver(map[string]string{
		"show gpon onu autofind": `Interface       SN              Distance  RxPower
-----------------------------------------------------
gpon-olt_1/1/1  CDAT11111111    100       -17.0
gpon-olt_1/1/2  CDAT22222222    200       -20.0
gpon-olt_1/1/1  CDAT33333333    150       -19.5`,
	})

	adapter := NewAdapter(mock, cfg).(*Adapter)

	// Filter to only port 1/1/2.
	discoveries, err := adapter.DiscoverONUs(context.Background(), []string{"1/1/2"})
	if err != nil {
		t.Fatalf("DiscoverONUs failed: %v", err)
	}
	if len(discoveries) != 1 {
		t.Fatalf("expected 1 filtered discovery, got %d", len(discoveries))
	}
	if discoveries[0].Serial != "CDAT22222222" {
		t.Errorf("expected CDAT22222222, got %s", discoveries[0].Serial)
	}
}

func TestDiscoverONUs_EPON(t *testing.T) {
	cfg := newEPONConfig()
	mock := cliMockDriver(map[string]string{
		"show epon onu autofind": "epon-olt_1/1/1  AABB11223344  50  -15.0",
	})

	adapter := NewAdapter(mock, cfg).(*Adapter)

	discoveries, err := adapter.DiscoverONUs(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverONUs EPON failed: %v", err)
	}
	if len(discoveries) != 1 {
		t.Fatalf("expected 1 discovery, got %d", len(discoveries))
	}
}

func TestDiscoverONUs_NoCLI(t *testing.T) {
	base := &simpleDriver{}
	adapter := NewAdapter(base, newGPONConfig()).(*Adapter)

	_, err := adapter.DiscoverONUs(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when CLI executor is not available")
	}
}

// ---------------------------------------------------------------------------
// Delegation tests
// ---------------------------------------------------------------------------

func TestConnect_Delegates(t *testing.T) {
	mock := &testutil.MockDriver{}
	cfg := newGPONConfig()
	adapter := NewAdapter(mock, cfg)

	if err := adapter.Connect(context.Background(), cfg); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if len(mock.Calls) == 0 || mock.Calls[0] != "Connect" {
		t.Fatalf("expected Connect call, got %v", mock.Calls)
	}
}

func TestDisconnect_Delegates(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	adapter := NewAdapter(mock, newGPONConfig())

	if err := adapter.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}
	if len(mock.Calls) == 0 || mock.Calls[0] != "Disconnect" {
		t.Fatalf("expected Disconnect call, got %v", mock.Calls)
	}
}

func TestIsConnected_Delegates(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	adapter := NewAdapter(mock, newGPONConfig())

	if !adapter.IsConnected() {
		t.Fatal("expected IsConnected true")
	}
	mock.Connected = false
	if adapter.IsConnected() {
		t.Fatal("expected IsConnected false")
	}
}

// ---------------------------------------------------------------------------
// UpdateSubscriber
// ---------------------------------------------------------------------------

func TestUpdateSubscriber_GPON(t *testing.T) {
	cfg := newGPONConfig()
	mock := cliMockDriver(nil)
	adapter := NewAdapter(mock, cfg)

	sub := newSubscriber("CDAT12345678", "1/1/2", 100, "5", "router")
	tier := newTier(50, 100, "line_100M_50M", "service_internet")

	err := adapter.UpdateSubscriber(context.Background(), sub, tier)
	if err != nil {
		t.Fatalf("UpdateSubscriber failed: %v", err)
	}

	cmds := mock.CLIExec.Commands
	foundProfile := false
	foundRateLimit := false
	for _, c := range cmds {
		if strings.Contains(c, "onu-profile 5 line line_100M_50M service service_internet") {
			foundProfile = true
		}
		if strings.Contains(c, "onu-ratelimit 5 upstream 50000 downstream 100000") {
			foundRateLimit = true
		}
	}
	if !foundProfile {
		t.Error("missing onu-profile update command")
	}
	if !foundRateLimit {
		t.Error("missing onu-ratelimit update command")
	}
}

func TestUpdateSubscriber_NoCLI(t *testing.T) {
	base := &simpleDriver{}
	adapter := NewAdapter(base, newGPONConfig())
	sub := newSubscriber("CDAT12345678", "1/1/1", 100, "1", "router")
	tier := newTier(50, 100, "", "")

	err := adapter.UpdateSubscriber(context.Background(), sub, tier)
	if err == nil {
		t.Fatal("expected error")
	}
}
