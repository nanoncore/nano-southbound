package cisco

import (
	"context"
	"fmt"
	"testing"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/testutil"
	"github.com/nanoncore/nano-southbound/types"
)

// ---------------------------------------------------------------------------
// NewAdapter
// ---------------------------------------------------------------------------

func TestNewAdapter_WithNETCONFExecutor(t *testing.T) {
	mock := &testutil.MockDriver{
		NETCONFExec: &testutil.MockNETCONFExecutor{},
		Connected:   true,
	}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")

	adapter := NewAdapter(mock, config)

	a, ok := adapter.(*Adapter)
	if !ok {
		t.Fatal("expected *Adapter")
	}
	if a.netconfExecutor == nil {
		t.Error("expected netconfExecutor to be set when base driver implements NETCONFExecutor")
	}
}

func TestNewAdapter_WithoutNETCONF(t *testing.T) {
	// A driver that does NOT implement NETCONFExecutor.
	plain := &plainDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")

	adapter := NewAdapter(plain, config)

	a, ok := adapter.(*Adapter)
	if !ok {
		t.Fatal("expected *Adapter")
	}
	if a.netconfExecutor != nil {
		t.Error("expected netconfExecutor to be nil when base driver lacks NETCONF")
	}
}

// plainDriver is a minimal Driver that does NOT implement NETCONFExecutor.
type plainDriver struct{}

func (p *plainDriver) Connect(context.Context, *types.EquipmentConfig) error  { return nil }
func (p *plainDriver) Disconnect(context.Context) error                       { return nil }
func (p *plainDriver) IsConnected() bool                                      { return true }
func (p *plainDriver) CreateSubscriber(context.Context, *model.Subscriber, *model.ServiceTier) (*types.SubscriberResult, error) {
	return nil, nil
}
func (p *plainDriver) UpdateSubscriber(context.Context, *model.Subscriber, *model.ServiceTier) error {
	return nil
}
func (p *plainDriver) DeleteSubscriber(context.Context, string) error               { return nil }
func (p *plainDriver) SuspendSubscriber(context.Context, string) error              { return nil }
func (p *plainDriver) ResumeSubscriber(context.Context, string) error               { return nil }
func (p *plainDriver) GetSubscriberStatus(context.Context, string) (*types.SubscriberStatus, error) {
	return nil, nil
}
func (p *plainDriver) GetSubscriberStats(context.Context, string) (*types.SubscriberStats, error) {
	return nil, nil
}
func (p *plainDriver) HealthCheck(context.Context) error { return nil }

// failingHealthDriver is a minimal Driver whose HealthCheck always fails.
type failingHealthDriver struct{ plainDriver }

func (f *failingHealthDriver) HealthCheck(context.Context) error {
	return fmt.Errorf("not connected")
}

// ---------------------------------------------------------------------------
// parseSubscriberSession
// ---------------------------------------------------------------------------

func TestParseSubscriberSession_ValidXML(t *testing.T) {
	a := &Adapter{config: testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")}

	xml := []byte(`<session-id><session-id>sess-123</session-id><subscriber-label>sub-1</subscriber-label><state>activated</state><mac-address>AA:BB:CC:DD:EE:FF</mac-address><ipv4-address>10.0.0.1</ipv4-address><ipv6-address>::1</ipv6-address><interface-name>Bundle-Ether1.100</interface-name><outer-vlan>100</outer-vlan><up-time>86400</up-time><accounting-session-id>acct-123</accounting-session-id><session-type>ipoe</session-type></session-id>`)

	session := a.parseSubscriberSession(xml)

	if session.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", session.SessionID, "sess-123")
	}
	if session.SubscriberLabel != "sub-1" {
		t.Errorf("SubscriberLabel = %q, want %q", session.SubscriberLabel, "sub-1")
	}
	if session.State != "activated" {
		t.Errorf("State = %q, want %q", session.State, "activated")
	}
	if session.MACAddress != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("MACAddress = %q, want %q", session.MACAddress, "AA:BB:CC:DD:EE:FF")
	}
	if session.IPv4Address != "10.0.0.1" {
		t.Errorf("IPv4Address = %q, want %q", session.IPv4Address, "10.0.0.1")
	}
	if session.IPv6Address != "::1" {
		t.Errorf("IPv6Address = %q, want %q", session.IPv6Address, "::1")
	}
	if session.Interface != "Bundle-Ether1.100" {
		t.Errorf("Interface = %q, want %q", session.Interface, "Bundle-Ether1.100")
	}
	if session.VLAN != 100 {
		t.Errorf("VLAN = %d, want %d", session.VLAN, 100)
	}
	if session.UptimeSecs != 86400 {
		t.Errorf("UptimeSecs = %d, want %d", session.UptimeSecs, 86400)
	}
	if session.AccountingID != "acct-123" {
		t.Errorf("AccountingID = %q, want %q", session.AccountingID, "acct-123")
	}
	if session.ServiceType != "ipoe" {
		t.Errorf("ServiceType = %q, want %q", session.ServiceType, "ipoe")
	}
}

func TestParseSubscriberSession_Empty(t *testing.T) {
	a := &Adapter{config: testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")}

	session := a.parseSubscriberSession([]byte{})

	if session.SessionID != "" {
		t.Errorf("expected empty SessionID, got %q", session.SessionID)
	}
	if session.State != "" {
		t.Errorf("expected empty State, got %q", session.State)
	}
}

// ---------------------------------------------------------------------------
// parseInterfaceStats
// ---------------------------------------------------------------------------

func TestParseInterfaceStats_ValidXML(t *testing.T) {
	a := &Adapter{config: testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")}

	xml := []byte(`<generic-counters><bytes-received>1000</bytes-received><bytes-sent>2000</bytes-sent><packets-received>10</packets-received><packets-sent>20</packets-sent><input-errors>1</input-errors><output-errors>2</output-errors><input-drops>3</input-drops><output-drops>4</output-drops><crc-errors>5</crc-errors><output-buffer-failures>6</output-buffer-failures></generic-counters>`)

	stats := a.parseInterfaceStats(xml)

	checks := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"BytesReceived", stats.BytesReceived, 1000},
		{"BytesSent", stats.BytesSent, 2000},
		{"PacketsReceived", stats.PacketsReceived, 10},
		{"PacketsSent", stats.PacketsSent, 20},
		{"InputErrors", stats.InputErrors, 1},
		{"OutputErrors", stats.OutputErrors, 2},
		{"InputDrops", stats.InputDrops, 3},
		{"OutputDrops", stats.OutputDrops, 4},
		{"InputCRCErrors", stats.InputCRCErrors, 5},
		{"OutputBufferFails", stats.OutputBufferFails, 6},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestParseInterfaceStats_Empty(t *testing.T) {
	a := &Adapter{config: testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")}

	stats := a.parseInterfaceStats([]byte{})

	if stats.BytesReceived != 0 || stats.BytesSent != 0 {
		t.Errorf("expected zero stats from empty input, got received=%d sent=%d",
			stats.BytesReceived, stats.BytesSent)
	}
}

// ---------------------------------------------------------------------------
// parseUptime
// ---------------------------------------------------------------------------

func TestParseUptime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{"plain seconds", "86400", 86400},
		{"dhms format", "1d2h30m45s", 1*86400 + 2*3600 + 30*60 + 45},
		{"colon d:h:m:s", "1:02:30:45", 1*86400 + 2*3600 + 30*60 + 45},
		{"zero", "0", 0},
		{"empty string", "", 0},
		{"only hours and seconds", "3h15s", 3*3600 + 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseUptime(tt.input)
			if got != tt.want {
				t.Errorf("parseUptime(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseSubscriberSummary
// ---------------------------------------------------------------------------

func TestParseSubscriberSummary_ValidXML(t *testing.T) {
	a := &Adapter{config: testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")}

	xml := []byte(`<summary><total-sessions>100</total-sessions><pppoe-sessions>40</pppoe-sessions><ipoe-sessions>60</ipoe-sessions><activated-sessions>95</activated-sessions><initiating-sessions>5</initiating-sessions></summary>`)

	summary := a.parseSubscriberSummary(xml)

	if summary.TotalSessions != 100 {
		t.Errorf("TotalSessions = %d, want 100", summary.TotalSessions)
	}
	if summary.PPPoESessions != 40 {
		t.Errorf("PPPoESessions = %d, want 40", summary.PPPoESessions)
	}
	if summary.IPoESessions != 60 {
		t.Errorf("IPoESessions = %d, want 60", summary.IPoESessions)
	}
	if summary.ActiveSessions != 95 {
		t.Errorf("ActiveSessions = %d, want 95", summary.ActiveSessions)
	}
	if summary.InitiatingSessions != 5 {
		t.Errorf("InitiatingSessions = %d, want 5", summary.InitiatingSessions)
	}
}

func TestParseSubscriberSummary_Empty(t *testing.T) {
	a := &Adapter{config: testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")}

	summary := a.parseSubscriberSummary([]byte{})

	if summary.TotalSessions != 0 {
		t.Errorf("expected zero TotalSessions, got %d", summary.TotalSessions)
	}
}

// ---------------------------------------------------------------------------
// parseSystemInfo
// ---------------------------------------------------------------------------

func TestParseSystemInfo_ValidXML(t *testing.T) {
	a := &Adapter{config: testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")}

	xml := []byte(`<cpu-utilization><total-cpu-fifteen-minute>45.5</total-cpu-fifteen-minute></cpu-utilization>`)

	info := a.parseSystemInfo(xml)

	if info.CPUPercent != 45.5 {
		t.Errorf("CPUPercent = %f, want 45.5", info.CPUPercent)
	}
}

func TestParseSystemInfo_Empty(t *testing.T) {
	a := &Adapter{config: testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")}

	info := a.parseSystemInfo([]byte{})

	if info.CPUPercent != 0 {
		t.Errorf("expected zero CPUPercent, got %f", info.CPUPercent)
	}
}

// ---------------------------------------------------------------------------
// extractSubscriberParams
// ---------------------------------------------------------------------------

func TestExtractSubscriberParams_WithInterfaceAnnotation(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	config.Metadata["uplink_interface"] = "TenGigE0/0/0/1"
	a := &Adapter{config: config}

	sub := testutil.NewTestSubscriber("SN001", "0/1", 200)
	sub.Annotations["nanoncore.com/interface"] = "Bundle-Ether2"
	tier := testutil.NewTestServiceTier(50, 100)

	params := a.extractSubscriberParams(sub, tier)

	// Annotation overrides uplink_interface metadata
	if params.ParentInterface != "Bundle-Ether2" {
		t.Errorf("ParentInterface = %q, want %q", params.ParentInterface, "Bundle-Ether2")
	}
	if params.InterfaceName != "Bundle-Ether2.200" {
		t.Errorf("InterfaceName = %q, want %q", params.InterfaceName, "Bundle-Ether2.200")
	}
	if params.VLAN != 200 {
		t.Errorf("VLAN = %d, want 200", params.VLAN)
	}
	if params.BandwidthUp != 50 {
		t.Errorf("BandwidthUp = %d, want 50", params.BandwidthUp)
	}
	if params.BandwidthDown != 100 {
		t.Errorf("BandwidthDown = %d, want 100", params.BandwidthDown)
	}
}

func TestExtractSubscriberParams_Defaults(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	a := &Adapter{config: config}

	sub := testutil.NewTestSubscriber("SN002", "0/1", 300)
	tier := testutil.NewTestServiceTier(10, 50)

	params := a.extractSubscriberParams(sub, tier)

	if params.NodeName != "0/0/CPU0" {
		t.Errorf("NodeName = %q, want default %q", params.NodeName, "0/0/CPU0")
	}
	if params.ParentInterface != "Bundle-Ether1" {
		t.Errorf("ParentInterface = %q, want default %q", params.ParentInterface, "Bundle-Ether1")
	}
	if params.UnnumberedIface != "Loopback0" {
		t.Errorf("UnnumberedIface = %q, want default %q", params.UnnumberedIface, "Loopback0")
	}
	// Default policies derived from bandwidth
	if params.DynamicTemplate != "nanoncore-ipsub-50M" {
		t.Errorf("DynamicTemplate = %q, want %q", params.DynamicTemplate, "nanoncore-ipsub-50M")
	}
	if params.PolicyInput != "nanoncore-ingress-50M" {
		t.Errorf("PolicyInput = %q, want %q", params.PolicyInput, "nanoncore-ingress-50M")
	}
	if params.PolicyOutput != "nanoncore-egress-50M" {
		t.Errorf("PolicyOutput = %q, want %q", params.PolicyOutput, "nanoncore-egress-50M")
	}
}

func TestExtractSubscriberParams_NilTier(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	a := &Adapter{config: config}

	sub := testutil.NewTestSubscriber("SN003", "0/1", 100)

	params := a.extractSubscriberParams(sub, nil)

	// With nil tier, bandwidth is 0 so default policies use 0M
	if params.DynamicTemplate != "nanoncore-ipsub-0M" {
		t.Errorf("DynamicTemplate = %q, want %q", params.DynamicTemplate, "nanoncore-ipsub-0M")
	}
	if params.BandwidthUp != 0 {
		t.Errorf("BandwidthUp = %d, want 0", params.BandwidthUp)
	}
}

func TestExtractSubscriberParams_TierAnnotations(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	a := &Adapter{config: config}

	sub := testutil.NewTestSubscriber("SN004", "0/1", 400)
	tier := testutil.NewTestServiceTier(100, 500)
	tier.Annotations = map[string]string{
		"nanoncore.com/dynamic-template": "custom-template",
		"nanoncore.com/policy-input":     "custom-ingress",
		"nanoncore.com/policy-output":    "custom-egress",
	}

	params := a.extractSubscriberParams(sub, tier)

	if params.DynamicTemplate != "custom-template" {
		t.Errorf("DynamicTemplate = %q, want %q", params.DynamicTemplate, "custom-template")
	}
	if params.PolicyInput != "custom-ingress" {
		t.Errorf("PolicyInput = %q, want %q", params.PolicyInput, "custom-ingress")
	}
	if params.PolicyOutput != "custom-egress" {
		t.Errorf("PolicyOutput = %q, want %q", params.PolicyOutput, "custom-egress")
	}
}

// ---------------------------------------------------------------------------
// parseSubscriberInterface
// ---------------------------------------------------------------------------

func TestParseSubscriberInterface_StructuredFormat(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	config.Metadata["uplink_interface"] = "TenGigE0/0/0/0"
	a := &Adapter{config: config}

	got := a.parseSubscriberInterface("cisco-sub1-100")

	if got != "TenGigE0/0/0/0.100" {
		t.Errorf("parseSubscriberInterface(cisco-sub1-100) = %q, want %q", got, "TenGigE0/0/0/0.100")
	}
}

func TestParseSubscriberInterface_AlreadyInterface(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	a := &Adapter{config: config}

	got := a.parseSubscriberInterface("Bundle-Ether1.100")

	if got != "Bundle-Ether1.100" {
		t.Errorf("parseSubscriberInterface(Bundle-Ether1.100) = %q, want %q", got, "Bundle-Ether1.100")
	}
}

func TestParseSubscriberInterface_BareVLANFallback(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	a := &Adapter{config: config}

	got := a.parseSubscriberInterface("200")

	// No uplink_interface in metadata => default "Bundle-Ether1"
	if got != "Bundle-Ether1.200" {
		t.Errorf("parseSubscriberInterface(200) = %q, want %q", got, "Bundle-Ether1.200")
	}
}

func TestParseSubscriberInterface_BareVLANWithUplink(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	config.Metadata["uplink_interface"] = "GigE0/0/0/5"
	a := &Adapter{config: config}

	got := a.parseSubscriberInterface("300")

	if got != "GigE0/0/0/5.300" {
		t.Errorf("parseSubscriberInterface(300) = %q, want %q", got, "GigE0/0/0/5.300")
	}
}

// ---------------------------------------------------------------------------
// getNodeName
// ---------------------------------------------------------------------------

func TestGetNodeName_FromConfig(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	config.Metadata["node_name"] = "0/1/CPU0"
	a := &Adapter{config: config}

	if got := a.getNodeName(); got != "0/1/CPU0" {
		t.Errorf("getNodeName() = %q, want %q", got, "0/1/CPU0")
	}
}

func TestGetNodeName_Default(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	a := &Adapter{config: config}

	if got := a.getNodeName(); got != "0/0/CPU0" {
		t.Errorf("getNodeName() = %q, want default %q", got, "0/0/CPU0")
	}
}

// ---------------------------------------------------------------------------
// detectOS
// ---------------------------------------------------------------------------

func TestDetectOS_FromConfig(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	config.Metadata["os"] = "ios-xe"
	a := &Adapter{config: config}

	if got := a.detectOS(); got != "ios-xe" {
		t.Errorf("detectOS() = %q, want %q", got, "ios-xe")
	}
}

func TestDetectOS_FromCapabilities_IOSXR(t *testing.T) {
	mockNE := &testutil.MockNETCONFExecutor{
		Capabilities: []string{
			"urn:ietf:params:netconf:base:1.0",
			"http://cisco.com/ns/yang/Cisco-IOS-XR-ifmgr-cfg",
		},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	a := &Adapter{config: config, netconfExecutor: mockNE}

	if got := a.detectOS(); got != "ios-xr" {
		t.Errorf("detectOS() = %q, want %q", got, "ios-xr")
	}
}

func TestDetectOS_FromCapabilities_IOSXE(t *testing.T) {
	mockNE := &testutil.MockNETCONFExecutor{
		Capabilities: []string{
			"urn:ietf:params:netconf:base:1.0",
			"http://cisco.com/ns/yang/Cisco-IOS-XE-native",
		},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	a := &Adapter{config: config, netconfExecutor: mockNE}

	if got := a.detectOS(); got != "ios-xe" {
		t.Errorf("detectOS() = %q, want %q", got, "ios-xe")
	}
}

func TestDetectOS_Default(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	a := &Adapter{config: config}

	if got := a.detectOS(); got != "ios-xr" {
		t.Errorf("detectOS() = %q, want default %q", got, "ios-xr")
	}
}

// ---------------------------------------------------------------------------
// Driver method tests using MockNETCONFExecutor
// ---------------------------------------------------------------------------

func newTestAdapter(t *testing.T) (*Adapter, *testutil.MockNETCONFExecutor, *testutil.MockDriver) {
	t.Helper()
	mockNE := &testutil.MockNETCONFExecutor{
		GetResponses: map[string][]byte{},
		GetErrors:    map[string]error{},
		Capabilities: []string{"urn:ietf:params:netconf:base:1.0"},
	}
	mockDriver := &testutil.MockDriver{
		NETCONFExec: mockNE,
		Connected:   true,
	}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	a, ok := adapter.(*Adapter)
	if !ok {
		t.Fatal("expected *Adapter")
	}
	return a, mockNE, mockDriver
}

func TestCreateSubscriber_Success(t *testing.T) {
	a, _, _ := newTestAdapter(t)
	ctx := context.Background()

	sub := testutil.NewTestSubscriber("SN100", "0/1", 500)
	sub.Spec.IPAddress = "192.168.1.10"
	sub.Spec.IPv6Address = "2001:db8::1"
	tier := testutil.NewTestServiceTier(50, 100)

	result, err := a.CreateSubscriber(ctx, sub, tier)
	if err != nil {
		t.Fatalf("CreateSubscriber() error = %v", err)
	}

	if result.SubscriberID != sub.Name {
		t.Errorf("SubscriberID = %q, want %q", result.SubscriberID, sub.Name)
	}
	wantSession := fmt.Sprintf("cisco-%s-%d", sub.Name, sub.Spec.VLAN)
	if result.SessionID != wantSession {
		t.Errorf("SessionID = %q, want %q", result.SessionID, wantSession)
	}
	if result.AssignedIP != "192.168.1.10" {
		t.Errorf("AssignedIP = %q, want %q", result.AssignedIP, "192.168.1.10")
	}
	if result.AssignedIPv6 != "2001:db8::1" {
		t.Errorf("AssignedIPv6 = %q, want %q", result.AssignedIPv6, "2001:db8::1")
	}
	if result.VLAN != 500 {
		t.Errorf("VLAN = %d, want 500", result.VLAN)
	}
	if result.InterfaceName != "Bundle-Ether1.500" {
		t.Errorf("InterfaceName = %q, want %q", result.InterfaceName, "Bundle-Ether1.500")
	}
	// Check metadata
	if v, ok := result.Metadata["vendor"].(string); !ok || v != "cisco" {
		t.Errorf("Metadata[vendor] = %v, want %q", result.Metadata["vendor"], "cisco")
	}
}

func TestCreateSubscriber_NoNETCONF(t *testing.T) {
	plain := &plainDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	adapter := NewAdapter(plain, config).(*Adapter)
	ctx := context.Background()

	sub := testutil.NewTestSubscriber("SN101", "0/1", 100)
	tier := testutil.NewTestServiceTier(10, 50)

	_, err := adapter.CreateSubscriber(ctx, sub, tier)
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
	if err.Error() != "NETCONF executor not available - Cisco requires NETCONF driver" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCreateSubscriber_EditConfigFails(t *testing.T) {
	a, mockNE, _ := newTestAdapter(t)
	mockNE.EditConfigError = fmt.Errorf("rpc-error: lock denied")
	ctx := context.Background()

	sub := testutil.NewTestSubscriber("SN102", "0/1", 100)
	tier := testutil.NewTestServiceTier(10, 50)

	_, err := a.CreateSubscriber(ctx, sub, tier)
	if err == nil {
		t.Fatal("expected error when EditConfig fails")
	}
	if err.Error() != "Cisco subscriber provisioning failed: rpc-error: lock denied" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateSubscriber_Success(t *testing.T) {
	a, _, _ := newTestAdapter(t)
	ctx := context.Background()

	sub := testutil.NewTestSubscriber("SN200", "0/1", 600)
	tier := testutil.NewTestServiceTier(100, 200)

	err := a.UpdateSubscriber(ctx, sub, tier)
	if err != nil {
		t.Fatalf("UpdateSubscriber() error = %v", err)
	}
}

func TestUpdateSubscriber_NoNETCONF(t *testing.T) {
	plain := &plainDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	adapter := NewAdapter(plain, config).(*Adapter)
	ctx := context.Background()

	sub := testutil.NewTestSubscriber("SN201", "0/1", 100)
	tier := testutil.NewTestServiceTier(10, 50)

	err := adapter.UpdateSubscriber(ctx, sub, tier)
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestDeleteSubscriber_Success(t *testing.T) {
	a, _, _ := newTestAdapter(t)
	ctx := context.Background()

	err := a.DeleteSubscriber(ctx, "cisco-sub1-100")
	if err != nil {
		t.Fatalf("DeleteSubscriber() error = %v", err)
	}
}

func TestDeleteSubscriber_NoNETCONF(t *testing.T) {
	plain := &plainDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	adapter := NewAdapter(plain, config).(*Adapter)
	ctx := context.Background()

	err := adapter.DeleteSubscriber(ctx, "cisco-sub1-100")
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestSuspendSubscriber_Success(t *testing.T) {
	a, _, _ := newTestAdapter(t)
	ctx := context.Background()

	err := a.SuspendSubscriber(ctx, "Bundle-Ether1.100")
	if err != nil {
		t.Fatalf("SuspendSubscriber() error = %v", err)
	}
}

func TestSuspendSubscriber_NoNETCONF(t *testing.T) {
	plain := &plainDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	adapter := NewAdapter(plain, config).(*Adapter)
	ctx := context.Background()

	err := adapter.SuspendSubscriber(ctx, "Bundle-Ether1.100")
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestResumeSubscriber_Success(t *testing.T) {
	a, _, _ := newTestAdapter(t)
	ctx := context.Background()

	err := a.ResumeSubscriber(ctx, "Bundle-Ether1.100")
	if err != nil {
		t.Fatalf("ResumeSubscriber() error = %v", err)
	}
}

func TestResumeSubscriber_NoNETCONF(t *testing.T) {
	plain := &plainDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	adapter := NewAdapter(plain, config).(*Adapter)
	ctx := context.Background()

	err := adapter.ResumeSubscriber(ctx, "Bundle-Ether1.100")
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestGetSubscriberStatus_Success(t *testing.T) {
	a, mockNE, _ := newTestAdapter(t)
	ctx := context.Background()

	sessionXML := []byte(`<session-id><session-id>sess-456</session-id><subscriber-label>sub-2</subscriber-label><state>activated</state><mac-address>11:22:33:44:55:66</mac-address><ipv4-address>10.1.1.1</ipv4-address><ipv6-address>2001:db8::2</ipv6-address><interface-name>Bundle-Ether1.200</interface-name><outer-vlan>200</outer-vlan><up-time>3600</up-time><accounting-session-id>acct-456</accounting-session-id><session-type>pppoe</session-type></session-id>`)

	// The filter is built with nodeName and subscriberID; we use a wildcard-like match
	// by populating every possible filter key
	for k := range mockNE.GetResponses {
		delete(mockNE.GetResponses, k)
	}
	// We need to match the exact filter. Build it the same way the adapter does.
	nodeName := a.getNodeName()
	filter := fmt.Sprintf(GetSubscriberSessionFilterXML, nodeName, "cisco-sub2-200")
	mockNE.GetResponses[filter] = sessionXML

	status, err := a.GetSubscriberStatus(ctx, "cisco-sub2-200")
	if err != nil {
		t.Fatalf("GetSubscriberStatus() error = %v", err)
	}

	if status.SubscriberID != "cisco-sub2-200" {
		t.Errorf("SubscriberID = %q, want %q", status.SubscriberID, "cisco-sub2-200")
	}
	if status.State != "activated" {
		t.Errorf("State = %q, want %q", status.State, "activated")
	}
	if status.SessionID != "sess-456" {
		t.Errorf("SessionID = %q, want %q", status.SessionID, "sess-456")
	}
	if !status.IsOnline {
		t.Error("expected IsOnline to be true for activated state")
	}
	if status.UptimeSeconds != 3600 {
		t.Errorf("UptimeSeconds = %d, want 3600", status.UptimeSeconds)
	}
}

func TestGetSubscriberStatus_NoNETCONF(t *testing.T) {
	plain := &plainDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	adapter := NewAdapter(plain, config).(*Adapter)
	ctx := context.Background()

	_, err := adapter.GetSubscriberStatus(ctx, "sub-1")
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestGetSubscriberStats_Success(t *testing.T) {
	a, mockNE, _ := newTestAdapter(t)
	ctx := context.Background()

	statsXML := []byte(`<generic-counters><bytes-received>5000</bytes-received><bytes-sent>10000</bytes-sent><packets-received>50</packets-received><packets-sent>100</packets-sent><input-errors>2</input-errors><output-errors>3</output-errors><input-drops>1</input-drops><output-drops>2</output-drops><crc-errors>0</crc-errors><output-buffer-failures>0</output-buffer-failures></generic-counters>`)

	interfaceName := "Bundle-Ether1.300"
	filter := fmt.Sprintf(GetInterfaceStatsFilterXML, interfaceName)
	mockNE.GetResponses[filter] = statsXML

	stats, err := a.GetSubscriberStats(ctx, "Bundle-Ether1.300")
	if err != nil {
		t.Fatalf("GetSubscriberStats() error = %v", err)
	}

	if stats.BytesUp != 5000 {
		t.Errorf("BytesUp = %d, want 5000", stats.BytesUp)
	}
	if stats.BytesDown != 10000 {
		t.Errorf("BytesDown = %d, want 10000", stats.BytesDown)
	}
	if stats.PacketsUp != 50 {
		t.Errorf("PacketsUp = %d, want 50", stats.PacketsUp)
	}
	if stats.PacketsDown != 100 {
		t.Errorf("PacketsDown = %d, want 100", stats.PacketsDown)
	}
	if stats.ErrorsUp != 2 {
		t.Errorf("ErrorsUp = %d, want 2", stats.ErrorsUp)
	}
	if stats.ErrorsDown != 3 {
		t.Errorf("ErrorsDown = %d, want 3", stats.ErrorsDown)
	}
	if stats.Drops != 3 { // 1 + 2
		t.Errorf("Drops = %d, want 3", stats.Drops)
	}
}

func TestGetSubscriberStats_NoNETCONF(t *testing.T) {
	plain := &plainDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	adapter := NewAdapter(plain, config).(*Adapter)
	ctx := context.Background()

	_, err := adapter.GetSubscriberStats(ctx, "sub-1")
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestHealthCheck_WithNETCONF(t *testing.T) {
	a, _, _ := newTestAdapter(t)
	ctx := context.Background()

	err := a.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
}

func TestHealthCheck_WithoutNETCONF(t *testing.T) {
	// Use plainDriver which does NOT implement NETCONFExecutor,
	// so the adapter falls back to the base driver's HealthCheck.
	plain := &plainDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	adapter := NewAdapter(plain, config).(*Adapter)
	ctx := context.Background()

	// plainDriver.HealthCheck always returns nil
	err := adapter.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
}

func TestHealthCheck_WithoutNETCONF_BaseDriverFails(t *testing.T) {
	// Use a driver that returns an error from HealthCheck
	failing := &failingHealthDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	adapter := NewAdapter(failing, config).(*Adapter)
	ctx := context.Background()

	err := adapter.HealthCheck(ctx)
	if err == nil {
		t.Fatal("expected error when base driver health check fails")
	}
}

func TestCreateDynamicTemplate_Success(t *testing.T) {
	a, _, _ := newTestAdapter(t)
	ctx := context.Background()

	tier := testutil.NewTestServiceTier(50, 100)

	err := a.CreateDynamicTemplate(ctx, "nanoncore-ipsub-100M", tier)
	if err != nil {
		t.Fatalf("CreateDynamicTemplate() error = %v", err)
	}
}

func TestCreateDynamicTemplate_NoNETCONF(t *testing.T) {
	plain := &plainDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	adapter := NewAdapter(plain, config).(*Adapter)
	ctx := context.Background()

	tier := testutil.NewTestServiceTier(50, 100)

	err := adapter.CreateDynamicTemplate(ctx, "template-1", tier)
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestCreateQoSPolicy_Success(t *testing.T) {
	a, _, _ := newTestAdapter(t)
	ctx := context.Background()

	tier := testutil.NewTestServiceTier(50, 100)

	err := a.CreateQoSPolicy(ctx, tier)
	if err != nil {
		t.Fatalf("CreateQoSPolicy() error = %v", err)
	}
}

func TestCreateQoSPolicy_NoNETCONF(t *testing.T) {
	plain := &plainDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	adapter := NewAdapter(plain, config).(*Adapter)
	ctx := context.Background()

	tier := testutil.NewTestServiceTier(50, 100)

	err := adapter.CreateQoSPolicy(ctx, tier)
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestGetSubscriberSummary_Success(t *testing.T) {
	a, mockNE, _ := newTestAdapter(t)
	ctx := context.Background()

	summaryXML := []byte(`<summary><total-sessions>200</total-sessions><pppoe-sessions>80</pppoe-sessions><ipoe-sessions>120</ipoe-sessions><activated-sessions>190</activated-sessions><initiating-sessions>10</initiating-sessions></summary>`)

	nodeName := a.getNodeName()
	filter := fmt.Sprintf(GetSubscriberSummaryFilterXML, nodeName)
	mockNE.GetResponses[filter] = summaryXML

	summary, err := a.GetSubscriberSummary(ctx)
	if err != nil {
		t.Fatalf("GetSubscriberSummary() error = %v", err)
	}

	if summary.TotalSessions != 200 {
		t.Errorf("TotalSessions = %d, want 200", summary.TotalSessions)
	}
	if summary.PPPoESessions != 80 {
		t.Errorf("PPPoESessions = %d, want 80", summary.PPPoESessions)
	}
	if summary.IPoESessions != 120 {
		t.Errorf("IPoESessions = %d, want 120", summary.IPoESessions)
	}
	if summary.ActiveSessions != 190 {
		t.Errorf("ActiveSessions = %d, want 190", summary.ActiveSessions)
	}
	if summary.InitiatingSessions != 10 {
		t.Errorf("InitiatingSessions = %d, want 10", summary.InitiatingSessions)
	}
}

func TestGetSubscriberSummary_NoNETCONF(t *testing.T) {
	plain := &plainDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	adapter := NewAdapter(plain, config).(*Adapter)
	ctx := context.Background()

	_, err := adapter.GetSubscriberSummary(ctx)
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestGetSystemInfo_Success(t *testing.T) {
	a, mockNE, _ := newTestAdapter(t)
	ctx := context.Background()

	cpuXML := []byte(`<cpu-utilization><total-cpu-fifteen-minute>72.3</total-cpu-fifteen-minute></cpu-utilization>`)
	mockNE.GetResponses[GetSystemInfoFilterXML] = cpuXML

	info, err := a.GetSystemInfo(ctx)
	if err != nil {
		t.Fatalf("GetSystemInfo() error = %v", err)
	}

	if info.CPUPercent != 72.3 {
		t.Errorf("CPUPercent = %f, want 72.3", info.CPUPercent)
	}
}

func TestGetSystemInfo_NoNETCONF(t *testing.T) {
	plain := &plainDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorCisco, "10.0.0.1")
	adapter := NewAdapter(plain, config).(*Adapter)
	ctx := context.Background()

	_, err := adapter.GetSystemInfo(ctx)
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}
