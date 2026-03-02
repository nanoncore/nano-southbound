package nokia

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/testutil"
	"github.com/nanoncore/nano-southbound/types"
)

// bareDriver implements only types.Driver -- no NETCONF methods.
// This is used to verify that NewAdapter correctly detects the absence
// of a NETCONFExecutor interface on the base driver.
type bareDriver struct {
	connected      bool
	healthCheckErr error
	calls          []string
}

func (d *bareDriver) Connect(_ context.Context, _ *types.EquipmentConfig) error {
	d.calls = append(d.calls, "Connect")
	d.connected = true
	return nil
}
func (d *bareDriver) Disconnect(_ context.Context) error {
	d.calls = append(d.calls, "Disconnect")
	d.connected = false
	return nil
}
func (d *bareDriver) IsConnected() bool { return d.connected }
func (d *bareDriver) CreateSubscriber(_ context.Context, _ *model.Subscriber, _ *model.ServiceTier) (*types.SubscriberResult, error) {
	return nil, fmt.Errorf("not implemented")
}
func (d *bareDriver) UpdateSubscriber(_ context.Context, _ *model.Subscriber, _ *model.ServiceTier) error {
	return fmt.Errorf("not implemented")
}
func (d *bareDriver) DeleteSubscriber(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}
func (d *bareDriver) SuspendSubscriber(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}
func (d *bareDriver) ResumeSubscriber(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}
func (d *bareDriver) GetSubscriberStatus(_ context.Context, _ string) (*types.SubscriberStatus, error) {
	return nil, fmt.Errorf("not implemented")
}
func (d *bareDriver) GetSubscriberStats(_ context.Context, _ string) (*types.SubscriberStats, error) {
	return nil, fmt.Errorf("not implemented")
}
func (d *bareDriver) HealthCheck(_ context.Context) error {
	d.calls = append(d.calls, "HealthCheck")
	return d.healthCheckErr
}

// ============================================================================
// Adapter construction tests
// ============================================================================

func TestNewAdapter_WithNETCONFDriver(t *testing.T) {
	mockDriver := &testutil.MockDriver{
		NETCONFExec: &testutil.MockNETCONFExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	if adapter == nil {
		t.Fatal("NewAdapter returned nil")
	}

	nk, ok := adapter.(*Adapter)
	if !ok {
		t.Fatal("NewAdapter did not return *Adapter")
	}

	if nk.netconfExecutor == nil {
		t.Error("expected NETCONF executor to be set")
	}
	if nk.baseDriver == nil {
		t.Error("expected base driver to be set")
	}
	if nk.config != config {
		t.Error("expected config to be set")
	}
}

func TestNewAdapter_WithoutNETCONF(t *testing.T) {
	base := &bareDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1")

	adapter := NewAdapter(base, config)
	nk, ok := adapter.(*Adapter)
	if !ok {
		t.Fatal("NewAdapter did not return *Adapter")
	}

	if nk.netconfExecutor != nil {
		t.Error("expected NETCONF executor to be nil when base driver has no NETCONFExec")
	}
	if nk.baseDriver == nil {
		t.Error("expected base driver to still be set")
	}
}

// ============================================================================
// Parser tests: parseSubscriberState
// ============================================================================

func TestParseSubscriberState_ValidXML(t *testing.T) {
	adapter := &Adapter{
		config: testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1"),
	}

	xml := []byte(`<subscriber><subscriber-id>test-sub</subscriber-id><admin-state>enable</admin-state><oper-state>up</oper-state><mac-address>AA:BB:CC:DD:EE:FF</mac-address><ipv4-address>10.0.0.1</ipv4-address><ipv6-address>::1</ipv6-address><sub-profile>nanoncore-100M</sub-profile><sla-profile>nanoncore-sla-100M</sla-profile><up-time>86400</up-time></subscriber>`)

	state := adapter.parseSubscriberState(xml)

	if state.SubscriberID != "test-sub" {
		t.Errorf("SubscriberID = %q, want %q", state.SubscriberID, "test-sub")
	}
	if state.AdminState != "enable" {
		t.Errorf("AdminState = %q, want %q", state.AdminState, "enable")
	}
	if state.OperState != "up" {
		t.Errorf("OperState = %q, want %q", state.OperState, "up")
	}
	if state.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("MAC = %q, want %q", state.MAC, "AA:BB:CC:DD:EE:FF")
	}
	if state.IPv4Address != "10.0.0.1" {
		t.Errorf("IPv4Address = %q, want %q", state.IPv4Address, "10.0.0.1")
	}
	if state.IPv6Address != "::1" {
		t.Errorf("IPv6Address = %q, want %q", state.IPv6Address, "::1")
	}
	if state.SubProfile != "nanoncore-100M" {
		t.Errorf("SubProfile = %q, want %q", state.SubProfile, "nanoncore-100M")
	}
	if state.SLAProfile != "nanoncore-sla-100M" {
		t.Errorf("SLAProfile = %q, want %q", state.SLAProfile, "nanoncore-sla-100M")
	}
	if state.UptimeSecs != 86400 {
		t.Errorf("UptimeSecs = %d, want %d", state.UptimeSecs, 86400)
	}
}

func TestParseSubscriberState_Empty(t *testing.T) {
	adapter := &Adapter{
		config: testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1"),
	}

	state := adapter.parseSubscriberState([]byte{})

	if state.SubscriberID != "" {
		t.Errorf("expected empty SubscriberID, got %q", state.SubscriberID)
	}
	if state.AdminState != "" {
		t.Errorf("expected empty AdminState, got %q", state.AdminState)
	}
	if state.UptimeSecs != 0 {
		t.Errorf("expected UptimeSecs = 0, got %d", state.UptimeSecs)
	}
}

// ============================================================================
// Parser tests: parseSubscriberStats
// ============================================================================

func TestParseSubscriberStats_ValidXML(t *testing.T) {
	adapter := &Adapter{
		config: testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1"),
	}

	xml := []byte(`<statistics><ingress-octets>1000</ingress-octets><egress-octets>2000</egress-octets><ingress-packets>10</ingress-packets><egress-packets>20</egress-packets><ingress-drops>1</ingress-drops><egress-drops>2</egress-drops></statistics>`)

	stats := adapter.parseSubscriberStats(xml)

	if stats.IngressOctets != 1000 {
		t.Errorf("IngressOctets = %d, want %d", stats.IngressOctets, 1000)
	}
	if stats.EgressOctets != 2000 {
		t.Errorf("EgressOctets = %d, want %d", stats.EgressOctets, 2000)
	}
	if stats.IngressPackets != 10 {
		t.Errorf("IngressPackets = %d, want %d", stats.IngressPackets, 10)
	}
	if stats.EgressPackets != 20 {
		t.Errorf("EgressPackets = %d, want %d", stats.EgressPackets, 20)
	}
	if stats.IngressDrops != 1 {
		t.Errorf("IngressDrops = %d, want %d", stats.IngressDrops, 1)
	}
	if stats.EgressDrops != 2 {
		t.Errorf("EgressDrops = %d, want %d", stats.EgressDrops, 2)
	}
}

func TestParseSubscriberStats_Empty(t *testing.T) {
	adapter := &Adapter{
		config: testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1"),
	}

	stats := adapter.parseSubscriberStats([]byte{})

	if stats.IngressOctets != 0 {
		t.Errorf("expected IngressOctets = 0, got %d", stats.IngressOctets)
	}
	if stats.EgressOctets != 0 {
		t.Errorf("expected EgressOctets = 0, got %d", stats.EgressOctets)
	}
}

// ============================================================================
// Parser tests: parseUptime
// ============================================================================

func TestParseUptime_Seconds(t *testing.T) {
	result := parseUptime("86400")
	if result != 86400 {
		t.Errorf("parseUptime(%q) = %d, want %d", "86400", result, 86400)
	}
}

func TestParseUptime_DurationFormat(t *testing.T) {
	// 1d2h30m45s = 86400 + 7200 + 1800 + 45 = 95445
	result := parseUptime("1d2h30m45s")
	expected := int64(1*86400 + 2*3600 + 30*60 + 45)
	if result != expected {
		t.Errorf("parseUptime(%q) = %d, want %d", "1d2h30m45s", result, expected)
	}
}

func TestParseUptime_Empty(t *testing.T) {
	result := parseUptime("")
	if result != 0 {
		t.Errorf("parseUptime(%q) = %d, want %d", "", result, 0)
	}
}

func TestParseUptime_DaysOnly(t *testing.T) {
	result := parseUptime("3d")
	if result != 3*86400 {
		t.Errorf("parseUptime(%q) = %d, want %d", "3d", result, 3*86400)
	}
}

func TestParseUptime_HoursMinutes(t *testing.T) {
	result := parseUptime("12h30m")
	expected := int64(12*3600 + 30*60)
	if result != expected {
		t.Errorf("parseUptime(%q) = %d, want %d", "12h30m", result, expected)
	}
}

// ============================================================================
// Parser tests: parseSystemInfo
// ============================================================================

func TestParseSystemInfo_ValidXML(t *testing.T) {
	adapter := &Adapter{
		config: testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1"),
	}

	xml := []byte(`<system><information><system-name>nokia-bng</system-name><chassis-type>SR-7750</chassis-type><software-version>22.10.R1</software-version><up-time>86400</up-time></information></system>`)

	info := adapter.parseSystemInfo(xml)

	if info.Name != "nokia-bng" {
		t.Errorf("Name = %q, want %q", info.Name, "nokia-bng")
	}
	if info.Type != "SR-7750" {
		t.Errorf("Type = %q, want %q", info.Type, "SR-7750")
	}
	if info.Version != "22.10.R1" {
		t.Errorf("Version = %q, want %q", info.Version, "22.10.R1")
	}
	if info.UptimeSecs != 86400 {
		t.Errorf("UptimeSecs = %d, want %d", info.UptimeSecs, 86400)
	}
}

func TestParseSystemInfo_Empty(t *testing.T) {
	adapter := &Adapter{
		config: testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1"),
	}

	info := adapter.parseSystemInfo([]byte{})

	if info.Name != "" {
		t.Errorf("expected empty Name, got %q", info.Name)
	}
	if info.Type != "" {
		t.Errorf("expected empty Type, got %q", info.Type)
	}
	if info.Version != "" {
		t.Errorf("expected empty Version, got %q", info.Version)
	}
	if info.UptimeSecs != 0 {
		t.Errorf("expected UptimeSecs = 0, got %d", info.UptimeSecs)
	}
}

// ============================================================================
// Helper tests: extractSubscriberParams
// ============================================================================

func TestExtractSubscriberParams_WithAnnotations(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1")
	config.Metadata["vprn"] = "customer-vprn"

	adapter := &Adapter{config: config}

	sub := testutil.NewTestSubscriber("ABCD12345678", "0/1", 100)
	sub.Annotations["nanoncore.com/sub-interface"] = "sub-customer"
	sub.Annotations["nanoncore.com/group-interface"] = "grp-customer"
	sub.Annotations["nanoncore.com/sap-id"] = "lag-1:100"
	sub.Spec.MACAddress = "AA:BB:CC:DD:EE:FF"
	sub.Spec.IPAddress = "10.0.0.100"
	sub.Spec.IPv6Address = "2001:db8::1"

	tier := testutil.NewTestServiceTier(50, 100)
	tier.Annotations = map[string]string{
		"nanoncore.com/sub-profile":      "custom-sub-profile",
		"nanoncore.com/sla-profile":      "custom-sla-profile",
		"nanoncore.com/sub-ident-policy": "custom-ident",
	}

	params := adapter.extractSubscriberParams(sub, tier)

	if params.VPRN != "customer-vprn" {
		t.Errorf("VPRN = %q, want %q", params.VPRN, "customer-vprn")
	}
	if params.SubInterface != "sub-customer" {
		t.Errorf("SubInterface = %q, want %q", params.SubInterface, "sub-customer")
	}
	if params.GroupInterface != "grp-customer" {
		t.Errorf("GroupInterface = %q, want %q", params.GroupInterface, "grp-customer")
	}
	if params.SapID != "lag-1:100" {
		t.Errorf("SapID = %q, want %q", params.SapID, "lag-1:100")
	}
	if params.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("MAC = %q, want %q", params.MAC, "AA:BB:CC:DD:EE:FF")
	}
	if params.IPv4Address != "10.0.0.100" {
		t.Errorf("IPv4Address = %q, want %q", params.IPv4Address, "10.0.0.100")
	}
	if params.IPv6Address != "2001:db8::1" {
		t.Errorf("IPv6Address = %q, want %q", params.IPv6Address, "2001:db8::1")
	}
	if params.SubProfile != "custom-sub-profile" {
		t.Errorf("SubProfile = %q, want %q", params.SubProfile, "custom-sub-profile")
	}
	if params.SLAProfile != "custom-sla-profile" {
		t.Errorf("SLAProfile = %q, want %q", params.SLAProfile, "custom-sla-profile")
	}
	if params.SubIdentPolicy != "custom-ident" {
		t.Errorf("SubIdentPolicy = %q, want %q", params.SubIdentPolicy, "custom-ident")
	}
	if params.BandwidthUp != 50 {
		t.Errorf("BandwidthUp = %d, want %d", params.BandwidthUp, 50)
	}
	if params.BandwidthDown != 100 {
		t.Errorf("BandwidthDown = %d, want %d", params.BandwidthDown, 100)
	}
}

func TestExtractSubscriberParams_Defaults(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1")
	// No vprn in metadata, no uplink_port

	adapter := &Adapter{config: config}

	sub := testutil.NewTestSubscriber("ABCD12345678", "0/1", 200)

	tier := testutil.NewTestServiceTier(10, 50)

	params := adapter.extractSubscriberParams(sub, tier)

	// VPRN should default to "internet"
	if params.VPRN != "internet" {
		t.Errorf("VPRN = %q, want %q", params.VPRN, "internet")
	}
	// SubInterface should default to sub-{vlan}
	if params.SubInterface != "sub-200" {
		t.Errorf("SubInterface = %q, want %q", params.SubInterface, "sub-200")
	}
	// GroupInterface should default to grp-{vlan}
	if params.GroupInterface != "grp-200" {
		t.Errorf("GroupInterface = %q, want %q", params.GroupInterface, "grp-200")
	}
	// SapID should default to 1/1/1:{vlan}
	if params.SapID != "1/1/1:200" {
		t.Errorf("SapID = %q, want %q", params.SapID, "1/1/1:200")
	}
	// SubProfile should default based on bandwidth
	if params.SubProfile != "nanoncore-50M" {
		t.Errorf("SubProfile = %q, want %q", params.SubProfile, "nanoncore-50M")
	}
	// SLAProfile should default based on bandwidth
	if params.SLAProfile != "nanoncore-sla-50M" {
		t.Errorf("SLAProfile = %q, want %q", params.SLAProfile, "nanoncore-sla-50M")
	}
	// SubIdentPolicy should default
	if params.SubIdentPolicy != "nanoncore-sub-ident" {
		t.Errorf("SubIdentPolicy = %q, want %q", params.SubIdentPolicy, "nanoncore-sub-ident")
	}
}

func TestExtractSubscriberParams_NilTier(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1")
	adapter := &Adapter{config: config}

	sub := testutil.NewTestSubscriber("ABCD12345678", "0/1", 100)

	params := adapter.extractSubscriberParams(sub, nil)

	// Bandwidth should be 0 when no tier
	if params.BandwidthUp != 0 {
		t.Errorf("BandwidthUp = %d, want %d", params.BandwidthUp, 0)
	}
	if params.BandwidthDown != 0 {
		t.Errorf("BandwidthDown = %d, want %d", params.BandwidthDown, 0)
	}
	// SubProfile should use 0M when no tier
	if params.SubProfile != "nanoncore-0M" {
		t.Errorf("SubProfile = %q, want %q", params.SubProfile, "nanoncore-0M")
	}
	if params.SLAProfile != "nanoncore-sla-0M" {
		t.Errorf("SLAProfile = %q, want %q", params.SLAProfile, "nanoncore-sla-0M")
	}
}

// ============================================================================
// Helper tests: detectPlatform
// ============================================================================

func TestDetectPlatform_FromConfigMetadata(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1")
	config.Metadata["platform"] = "srlinux"

	adapter := &Adapter{
		config: config,
	}

	platform := adapter.detectPlatform()
	if platform != "srlinux" {
		t.Errorf("detectPlatform() = %q, want %q", platform, "srlinux")
	}
}

func TestDetectPlatform_FromCapabilities_SRLinux(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1")

	mockNETCONF := &testutil.MockNETCONFExecutor{
		Capabilities: []string{
			"urn:ietf:params:netconf:base:1.0",
			"urn:nokia:sr-linux:yang:platform",
		},
	}

	adapter := &Adapter{
		config:          config,
		netconfExecutor: mockNETCONF,
	}

	platform := adapter.detectPlatform()
	if platform != "srlinux" {
		t.Errorf("detectPlatform() = %q, want %q", platform, "srlinux")
	}
}

func TestDetectPlatform_FromCapabilities_SROS(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1")

	mockNETCONF := &testutil.MockNETCONFExecutor{
		Capabilities: []string{
			"urn:ietf:params:netconf:base:1.0",
			"urn:nokia.com:sros:ns:yang:sr:conf",
		},
	}

	adapter := &Adapter{
		config:          config,
		netconfExecutor: mockNETCONF,
	}

	platform := adapter.detectPlatform()
	if platform != "sros" {
		t.Errorf("detectPlatform() = %q, want %q", platform, "sros")
	}
}

func TestDetectPlatform_Default(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1")

	adapter := &Adapter{
		config:          config,
		netconfExecutor: nil,
	}

	platform := adapter.detectPlatform()
	if platform != "sros" {
		t.Errorf("detectPlatform() = %q, want default %q", platform, "sros")
	}
}

// ============================================================================
// Helper tests: parseSubscriberID
// ============================================================================

func TestParseSubscriberID_StructuredFormat(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1")
	config.Metadata["vprn"] = "my-vprn"

	adapter := &Adapter{config: config}

	params := adapter.parseSubscriberID("sub-iface:sub-100/grp:grp-100/sap:lag-1:100")

	if params.VPRN != "my-vprn" {
		t.Errorf("VPRN = %q, want %q", params.VPRN, "my-vprn")
	}
	if params.SubInterface != "sub-100" {
		t.Errorf("SubInterface = %q, want %q", params.SubInterface, "sub-100")
	}
	if params.GroupInterface != "grp-100" {
		t.Errorf("GroupInterface = %q, want %q", params.GroupInterface, "grp-100")
	}
	if params.SapID != "lag-1:100" {
		t.Errorf("SapID = %q, want %q", params.SapID, "lag-1:100")
	}
	if params.HostID != "sub-iface:sub-100/grp:grp-100/sap:lag-1:100" {
		t.Errorf("HostID = %q, want original subscriberID", params.HostID)
	}
}

func TestParseSubscriberID_Fallback(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1")

	adapter := &Adapter{config: config}

	params := adapter.parseSubscriberID("simple-subscriber-id")

	if params.VPRN != "internet" {
		t.Errorf("VPRN = %q, want default %q", params.VPRN, "internet")
	}
	if params.SubInterface != "sub-default" {
		t.Errorf("SubInterface = %q, want %q", params.SubInterface, "sub-default")
	}
	if params.GroupInterface != "grp-default" {
		t.Errorf("GroupInterface = %q, want %q", params.GroupInterface, "grp-default")
	}
	if params.SapID != "1/1/1:100" {
		t.Errorf("SapID = %q, want %q", params.SapID, "1/1/1:100")
	}
	if params.HostID != "simple-subscriber-id" {
		t.Errorf("HostID = %q, want %q", params.HostID, "simple-subscriber-id")
	}
}

// ============================================================================
// Driver method tests with MockNETCONFExecutor
// ============================================================================

func newTestAdapter(t *testing.T, withNETCONF bool) (*Adapter, *testutil.MockDriver, *testutil.MockNETCONFExecutor) {
	t.Helper()

	var mockNC *testutil.MockNETCONFExecutor
	mockDriver := &testutil.MockDriver{Connected: true}
	if withNETCONF {
		mockNC = &testutil.MockNETCONFExecutor{}
		mockDriver.NETCONFExec = mockNC
	}

	config := testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1")

	driver := NewAdapter(mockDriver, config)
	adapter, ok := driver.(*Adapter)
	if !ok {
		t.Fatal("NewAdapter did not return *Adapter")
	}

	return adapter, mockDriver, mockNC
}

// --- CreateSubscriber ---

func TestCreateSubscriber_Success(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)

	sub := testutil.NewTestSubscriber("ABCD12345678", "0/1", 100)
	sub.Spec.MACAddress = "AA:BB:CC:DD:EE:FF"
	sub.Spec.IPAddress = "10.0.0.50"
	sub.Spec.IPv6Address = "2001:db8::50"

	tier := testutil.NewTestServiceTier(50, 100)

	ctx := context.Background()
	result, err := adapter.CreateSubscriber(ctx, sub, tier)
	if err != nil {
		t.Fatalf("CreateSubscriber returned error: %v", err)
	}

	if result.SubscriberID != sub.Name {
		t.Errorf("SubscriberID = %q, want %q", result.SubscriberID, sub.Name)
	}
	if !strings.HasPrefix(result.SessionID, "nokia-") {
		t.Errorf("SessionID = %q, want prefix 'nokia-'", result.SessionID)
	}
	if result.AssignedIP != "10.0.0.50" {
		t.Errorf("AssignedIP = %q, want %q", result.AssignedIP, "10.0.0.50")
	}
	if result.AssignedIPv6 != "2001:db8::50" {
		t.Errorf("AssignedIPv6 = %q, want %q", result.AssignedIPv6, "2001:db8::50")
	}
	if result.VLAN != 100 {
		t.Errorf("VLAN = %d, want %d", result.VLAN, 100)
	}

	// Verify InterfaceName contains structured format
	if !strings.HasPrefix(result.InterfaceName, "sub-iface:") {
		t.Errorf("InterfaceName = %q, want prefix 'sub-iface:'", result.InterfaceName)
	}

	// Verify metadata
	if result.Metadata["vendor"] != "nokia" {
		t.Errorf("metadata vendor = %v, want %q", result.Metadata["vendor"], "nokia")
	}
	if result.Metadata["vprn"] != "internet" {
		t.Errorf("metadata vprn = %v, want %q", result.Metadata["vprn"], "internet")
	}

	// Verify EditConfig was called
	hasEditConfig := false
	for _, call := range mockNC.Calls {
		if call == "EditConfig" {
			hasEditConfig = true
			break
		}
	}
	if !hasEditConfig {
		t.Error("expected EditConfig to be called")
	}
}

func TestCreateSubscriber_NoNETCONF(t *testing.T) {
	adapter, _, _ := newTestAdapter(t, false)

	sub := testutil.NewTestSubscriber("ABCD12345678", "0/1", 100)
	tier := testutil.NewTestServiceTier(50, 100)

	ctx := context.Background()
	_, err := adapter.CreateSubscriber(ctx, sub, tier)
	if err == nil {
		t.Fatal("expected error when NETCONF not available")
	}
	if !strings.Contains(err.Error(), "NETCONF") {
		t.Errorf("error = %q, want message containing 'NETCONF'", err.Error())
	}
}

func TestCreateSubscriber_EditConfigFails(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)
	mockNC.EditConfigError = fmt.Errorf("NETCONF edit-config failed: connection timeout")

	sub := testutil.NewTestSubscriber("ABCD12345678", "0/1", 100)
	tier := testutil.NewTestServiceTier(50, 100)

	ctx := context.Background()
	_, err := adapter.CreateSubscriber(ctx, sub, tier)
	if err == nil {
		t.Fatal("expected error when EditConfig fails")
	}
	if !strings.Contains(err.Error(), "provisioning failed") {
		t.Errorf("error = %q, want message containing 'provisioning failed'", err.Error())
	}
}

// --- UpdateSubscriber ---

func TestUpdateSubscriber_Success(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)

	sub := testutil.NewTestSubscriber("ABCD12345678", "0/1", 100)
	tier := testutil.NewTestServiceTier(100, 200)

	ctx := context.Background()
	err := adapter.UpdateSubscriber(ctx, sub, tier)
	if err != nil {
		t.Fatalf("UpdateSubscriber returned error: %v", err)
	}

	hasEditConfig := false
	for _, call := range mockNC.Calls {
		if call == "EditConfig" {
			hasEditConfig = true
			break
		}
	}
	if !hasEditConfig {
		t.Error("expected EditConfig to be called for update")
	}
}

func TestUpdateSubscriber_NoNETCONF(t *testing.T) {
	adapter, _, _ := newTestAdapter(t, false)

	sub := testutil.NewTestSubscriber("ABCD12345678", "0/1", 100)
	tier := testutil.NewTestServiceTier(50, 100)

	ctx := context.Background()
	err := adapter.UpdateSubscriber(ctx, sub, tier)
	if err == nil {
		t.Fatal("expected error when NETCONF not available")
	}
}

// --- DeleteSubscriber ---

func TestDeleteSubscriber_Success(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)

	ctx := context.Background()
	err := adapter.DeleteSubscriber(ctx, "sub-iface:sub-100/grp:grp-100/sap:1/1/1:100")
	if err != nil {
		t.Fatalf("DeleteSubscriber returned error: %v", err)
	}

	hasEditConfig := false
	for _, call := range mockNC.Calls {
		if call == "EditConfig" {
			hasEditConfig = true
			break
		}
	}
	if !hasEditConfig {
		t.Error("expected EditConfig to be called for delete")
	}
}

func TestDeleteSubscriber_NoNETCONF(t *testing.T) {
	adapter, _, _ := newTestAdapter(t, false)

	ctx := context.Background()
	err := adapter.DeleteSubscriber(ctx, "sub-id")
	if err == nil {
		t.Fatal("expected error when NETCONF not available")
	}
}

func TestDeleteSubscriber_EditConfigFails(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)
	mockNC.EditConfigError = fmt.Errorf("delete failed")

	ctx := context.Background()
	err := adapter.DeleteSubscriber(ctx, "sub-id")
	if err == nil {
		t.Fatal("expected error when EditConfig fails")
	}
}

// --- SuspendSubscriber ---

func TestSuspendSubscriber_Success(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)

	ctx := context.Background()
	err := adapter.SuspendSubscriber(ctx, "sub-iface:sub-100/grp:grp-100/sap:1/1/1:100")
	if err != nil {
		t.Fatalf("SuspendSubscriber returned error: %v", err)
	}

	hasEditConfig := false
	for _, call := range mockNC.Calls {
		if call == "EditConfig" {
			hasEditConfig = true
			break
		}
	}
	if !hasEditConfig {
		t.Error("expected EditConfig to be called for suspend")
	}
}

func TestSuspendSubscriber_NoNETCONF(t *testing.T) {
	adapter, _, _ := newTestAdapter(t, false)

	ctx := context.Background()
	err := adapter.SuspendSubscriber(ctx, "sub-id")
	if err == nil {
		t.Fatal("expected error when NETCONF not available")
	}
}

// --- ResumeSubscriber ---

func TestResumeSubscriber_Success(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)

	ctx := context.Background()
	err := adapter.ResumeSubscriber(ctx, "sub-iface:sub-100/grp:grp-100/sap:1/1/1:100")
	if err != nil {
		t.Fatalf("ResumeSubscriber returned error: %v", err)
	}

	hasEditConfig := false
	for _, call := range mockNC.Calls {
		if call == "EditConfig" {
			hasEditConfig = true
			break
		}
	}
	if !hasEditConfig {
		t.Error("expected EditConfig to be called for resume")
	}
}

func TestResumeSubscriber_NoNETCONF(t *testing.T) {
	adapter, _, _ := newTestAdapter(t, false)

	ctx := context.Background()
	err := adapter.ResumeSubscriber(ctx, "sub-id")
	if err == nil {
		t.Fatal("expected error when NETCONF not available")
	}
}

// --- GetSubscriberStatus ---

func TestGetSubscriberStatus_Success(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)

	// Set up a response for the Get call
	subFilter := fmt.Sprintf(GetSubscriberFilterXML, "test-sub")
	mockNC.GetResponses = map[string][]byte{
		subFilter: []byte(`<subscriber><subscriber-id>test-sub</subscriber-id><admin-state>enable</admin-state><oper-state>up</oper-state><mac-address>AA:BB:CC:DD:EE:FF</mac-address><ipv4-address>10.0.0.1</ipv4-address><ipv6-address>::1</ipv6-address><sub-profile>nanoncore-100M</sub-profile><sla-profile>nanoncore-sla-100M</sla-profile><up-time>86400</up-time></subscriber>`),
	}

	ctx := context.Background()
	status, err := adapter.GetSubscriberStatus(ctx, "test-sub")
	if err != nil {
		t.Fatalf("GetSubscriberStatus returned error: %v", err)
	}

	if status.SubscriberID != "test-sub" {
		t.Errorf("SubscriberID = %q, want %q", status.SubscriberID, "test-sub")
	}
	if status.State != "up" {
		t.Errorf("State = %q, want %q", status.State, "up")
	}
	if status.IPv4Address != "10.0.0.1" {
		t.Errorf("IPv4Address = %q, want %q", status.IPv4Address, "10.0.0.1")
	}
	if status.IPv6Address != "::1" {
		t.Errorf("IPv6Address = %q, want %q", status.IPv6Address, "::1")
	}
	if status.UptimeSeconds != 86400 {
		t.Errorf("UptimeSeconds = %d, want %d", status.UptimeSeconds, 86400)
	}
	if !status.IsOnline {
		t.Error("expected IsOnline = true when oper-state is 'up'")
	}
	if status.SessionID != "nokia-test-sub" {
		t.Errorf("SessionID = %q, want %q", status.SessionID, "nokia-test-sub")
	}

	// Verify metadata
	if status.Metadata["vendor"] != "nokia" {
		t.Errorf("metadata vendor = %v, want %q", status.Metadata["vendor"], "nokia")
	}
	if status.Metadata["admin_state"] != "enable" {
		t.Errorf("metadata admin_state = %v, want %q", status.Metadata["admin_state"], "enable")
	}
	if status.Metadata["mac"] != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("metadata mac = %v, want %q", status.Metadata["mac"], "AA:BB:CC:DD:EE:FF")
	}
}

func TestGetSubscriberStatus_NoNETCONF(t *testing.T) {
	adapter, _, _ := newTestAdapter(t, false)

	ctx := context.Background()
	_, err := adapter.GetSubscriberStatus(ctx, "test-sub")
	if err == nil {
		t.Fatal("expected error when NETCONF not available")
	}
}

func TestGetSubscriberStatus_GetFails(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)

	subFilter := fmt.Sprintf(GetSubscriberFilterXML, "test-sub")
	mockNC.GetErrors = map[string]error{
		subFilter: fmt.Errorf("NETCONF get failed"),
	}

	ctx := context.Background()
	_, err := adapter.GetSubscriberStatus(ctx, "test-sub")
	if err == nil {
		t.Fatal("expected error when Get fails")
	}
	if !strings.Contains(err.Error(), "failed to get subscriber status") {
		t.Errorf("error = %q, want message containing 'failed to get subscriber status'", err.Error())
	}
}

// --- GetSubscriberStats ---

func TestGetSubscriberStats_Success(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)

	statsFilter := fmt.Sprintf(GetSubscriberStatsFilterXML, "test-sub")
	mockNC.GetResponses = map[string][]byte{
		statsFilter: []byte(`<statistics><ingress-octets>1000</ingress-octets><egress-octets>2000</egress-octets><ingress-packets>10</ingress-packets><egress-packets>20</egress-packets><ingress-drops>1</ingress-drops><egress-drops>2</egress-drops></statistics>`),
	}

	ctx := context.Background()
	stats, err := adapter.GetSubscriberStats(ctx, "test-sub")
	if err != nil {
		t.Fatalf("GetSubscriberStats returned error: %v", err)
	}

	if stats.BytesUp != 1000 {
		t.Errorf("BytesUp = %d, want %d", stats.BytesUp, 1000)
	}
	if stats.BytesDown != 2000 {
		t.Errorf("BytesDown = %d, want %d", stats.BytesDown, 2000)
	}
	if stats.PacketsUp != 10 {
		t.Errorf("PacketsUp = %d, want %d", stats.PacketsUp, 10)
	}
	if stats.PacketsDown != 20 {
		t.Errorf("PacketsDown = %d, want %d", stats.PacketsDown, 20)
	}
	if stats.Drops != 3 {
		t.Errorf("Drops = %d, want %d (ingress_drops + egress_drops)", stats.Drops, 3)
	}

	// Verify metadata
	if stats.Metadata["vendor"] != "nokia" {
		t.Errorf("metadata vendor = %v, want %q", stats.Metadata["vendor"], "nokia")
	}
	if stats.Metadata["source"] != "netconf" {
		t.Errorf("metadata source = %v, want %q", stats.Metadata["source"], "netconf")
	}
	if stats.Metadata["ingress_drops"] != uint64(1) {
		t.Errorf("metadata ingress_drops = %v, want %d", stats.Metadata["ingress_drops"], 1)
	}
	if stats.Metadata["egress_drops"] != uint64(2) {
		t.Errorf("metadata egress_drops = %v, want %d", stats.Metadata["egress_drops"], 2)
	}
}

func TestGetSubscriberStats_NoNETCONF(t *testing.T) {
	adapter, _, _ := newTestAdapter(t, false)

	ctx := context.Background()
	_, err := adapter.GetSubscriberStats(ctx, "test-sub")
	if err == nil {
		t.Fatal("expected error when NETCONF not available")
	}
}

func TestGetSubscriberStats_GetFails(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)

	statsFilter := fmt.Sprintf(GetSubscriberStatsFilterXML, "test-sub")
	mockNC.GetErrors = map[string]error{
		statsFilter: fmt.Errorf("NETCONF get failed"),
	}

	ctx := context.Background()
	_, err := adapter.GetSubscriberStats(ctx, "test-sub")
	if err == nil {
		t.Fatal("expected error when Get fails")
	}
	if !strings.Contains(err.Error(), "failed to get subscriber stats") {
		t.Errorf("error = %q, want message containing 'failed to get subscriber stats'", err.Error())
	}
}

// --- HealthCheck ---

func TestHealthCheck_WithNETCONF(t *testing.T) {
	adapter, _, _ := newTestAdapter(t, true)

	ctx := context.Background()
	err := adapter.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("HealthCheck returned error: %v", err)
	}
}

func TestHealthCheck_WithoutNETCONF_DelegatesToBase(t *testing.T) {
	base := &bareDriver{connected: true}
	config := testutil.NewTestEquipmentConfig(types.VendorNokia, "10.0.0.1")

	driver := NewAdapter(base, config)
	adapter, ok := driver.(*Adapter)
	if !ok {
		t.Fatal("NewAdapter did not return *Adapter")
	}

	ctx := context.Background()
	err := adapter.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("HealthCheck returned error: %v", err)
	}

	// Verify it delegated to base driver
	hasHealthCheck := false
	for _, call := range base.calls {
		if call == "HealthCheck" {
			hasHealthCheck = true
			break
		}
	}
	if !hasHealthCheck {
		t.Error("expected HealthCheck to delegate to base driver when NETCONF unavailable")
	}
}

func TestHealthCheck_NETCONFGetFails(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)

	mockNC.GetErrors = map[string]error{
		GetSystemInfoFilterXML: fmt.Errorf("connection lost"),
	}

	ctx := context.Background()
	err := adapter.HealthCheck(ctx)
	if err == nil {
		t.Fatal("expected error when NETCONF get fails")
	}
}

// --- CreateQoSProfiles ---

func TestCreateQoSProfiles_Success(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)

	tier := testutil.NewTestServiceTier(50, 100)

	ctx := context.Background()
	err := adapter.CreateQoSProfiles(ctx, tier)
	if err != nil {
		t.Fatalf("CreateQoSProfiles returned error: %v", err)
	}

	hasEditConfig := false
	for _, call := range mockNC.Calls {
		if call == "EditConfig" {
			hasEditConfig = true
			break
		}
	}
	if !hasEditConfig {
		t.Error("expected EditConfig to be called for QoS profile creation")
	}
}

func TestCreateQoSProfiles_NoNETCONF(t *testing.T) {
	adapter, _, _ := newTestAdapter(t, false)

	tier := testutil.NewTestServiceTier(50, 100)

	ctx := context.Background()
	err := adapter.CreateQoSProfiles(ctx, tier)
	if err == nil {
		t.Fatal("expected error when NETCONF not available")
	}
}

// --- CreateSubscriberProfile ---

func TestCreateSubscriberProfile_Success(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)

	tier := testutil.NewTestServiceTier(50, 100)

	ctx := context.Background()
	err := adapter.CreateSubscriberProfile(ctx, tier)
	if err != nil {
		t.Fatalf("CreateSubscriberProfile returned error: %v", err)
	}

	hasEditConfig := false
	for _, call := range mockNC.Calls {
		if call == "EditConfig" {
			hasEditConfig = true
			break
		}
	}
	if !hasEditConfig {
		t.Error("expected EditConfig to be called for subscriber profile creation")
	}
}

func TestCreateSubscriberProfile_NoNETCONF(t *testing.T) {
	adapter, _, _ := newTestAdapter(t, false)

	tier := testutil.NewTestServiceTier(50, 100)

	ctx := context.Background()
	err := adapter.CreateSubscriberProfile(ctx, tier)
	if err == nil {
		t.Fatal("expected error when NETCONF not available")
	}
}

// --- GetSystemInfo ---

func TestGetSystemInfo_Success(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)

	mockNC.GetResponses = map[string][]byte{
		GetSystemInfoFilterXML: []byte(`<system><information><system-name>nokia-bng</system-name><chassis-type>SR-7750</chassis-type><software-version>22.10.R1</software-version><up-time>86400</up-time></information></system>`),
	}

	ctx := context.Background()
	info, err := adapter.GetSystemInfo(ctx)
	if err != nil {
		t.Fatalf("GetSystemInfo returned error: %v", err)
	}

	if info.Name != "nokia-bng" {
		t.Errorf("Name = %q, want %q", info.Name, "nokia-bng")
	}
	if info.Type != "SR-7750" {
		t.Errorf("Type = %q, want %q", info.Type, "SR-7750")
	}
	if info.Version != "22.10.R1" {
		t.Errorf("Version = %q, want %q", info.Version, "22.10.R1")
	}
	if info.UptimeSecs != 86400 {
		t.Errorf("UptimeSecs = %d, want %d", info.UptimeSecs, 86400)
	}
}

func TestGetSystemInfo_NoNETCONF(t *testing.T) {
	adapter, _, _ := newTestAdapter(t, false)

	ctx := context.Background()
	_, err := adapter.GetSystemInfo(ctx)
	if err == nil {
		t.Fatal("expected error when NETCONF not available")
	}
}

func TestGetSystemInfo_GetFails(t *testing.T) {
	adapter, _, mockNC := newTestAdapter(t, true)

	mockNC.GetErrors = map[string]error{
		GetSystemInfoFilterXML: fmt.Errorf("NETCONF connection timeout"),
	}

	ctx := context.Background()
	_, err := adapter.GetSystemInfo(ctx)
	if err == nil {
		t.Fatal("expected error when Get fails")
	}
}
