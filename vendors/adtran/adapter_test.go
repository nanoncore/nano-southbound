package adtran

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/testutil"
	"github.com/nanoncore/nano-southbound/types"
)

// --- NewAdapter tests ---

func TestNewAdapter_WithNETCONFExecutor(t *testing.T) {
	mock := &testutil.MockDriver{
		Connected:   true,
		NETCONFExec: &testutil.MockNETCONFExecutor{},
	}
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	adapter := NewAdapter(mock, cfg)
	if adapter == nil {
		t.Fatal("NewAdapter returned nil")
	}
	a, ok := adapter.(*Adapter)
	if !ok {
		t.Fatal("expected *Adapter type")
	}
	if a.netconfExecutor == nil {
		t.Fatal("expected netconfExecutor to be set when driver implements NETCONFExecutor")
	}
}

func TestNewAdapter_WithoutNETCONF(t *testing.T) {
	// MockDriver without NETCONFExec still implements NETCONFExecutor interface
	// because MockDriver always has the methods. However, when NETCONFExec is nil,
	// the adapter still gets the executor set (since the mock implements the interface).
	// We test the case where the base driver does NOT implement NETCONFExecutor.
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	// Use a plain MockDriver -- it still implements NETCONFExecutor through delegation
	mock := &testutil.MockDriver{Connected: true}
	adapter := NewAdapter(mock, cfg)
	if adapter == nil {
		t.Fatal("NewAdapter returned nil")
	}
	// The adapter should still have an executor because MockDriver implements the interface
	a := adapter.(*Adapter)
	if a.netconfExecutor == nil {
		t.Fatal("expected netconfExecutor even with plain MockDriver (it implements NETCONFExecutor)")
	}
}

// --- parseONTState tests ---

func TestParseONTState_ValidXML(t *testing.T) {
	xml := []byte(`<ont-state><serial-number>ADTN12345678</serial-number><ont-id>1</ont-id><pon-port>gpon-0/0/1</pon-port><admin-state>enabled</admin-state><operational-status>online</operational-status><description>test</description><optical-info><rx-power>-18.5</rx-power><tx-power>2.3</tx-power><temperature>42.5</temperature><voltage>3.3</voltage></optical-info><distance>1234</distance><uptime>86400</uptime></ont-state>`)

	a := &Adapter{}
	state := a.parseONTState(xml)

	if state.SerialNumber != "ADTN12345678" {
		t.Fatalf("expected serial ADTN12345678, got %s", state.SerialNumber)
	}
	if state.ONTID != 1 {
		t.Fatalf("expected ONTID 1, got %d", state.ONTID)
	}
	if state.PONPort != "gpon-0/0/1" {
		t.Fatalf("expected pon-port gpon-0/0/1, got %s", state.PONPort)
	}
	if state.AdminState != "enabled" {
		t.Fatalf("expected admin-state enabled, got %s", state.AdminState)
	}
	if state.OperState != "online" {
		t.Fatalf("expected oper-state online, got %s", state.OperState)
	}
	if state.Description != "test" {
		t.Fatalf("expected description test, got %s", state.Description)
	}
	if state.RxPower != -18.5 {
		t.Fatalf("expected rx-power -18.5, got %f", state.RxPower)
	}
	if state.TxPower != 2.3 {
		t.Fatalf("expected tx-power 2.3, got %f", state.TxPower)
	}
	if state.Temperature != 42.5 {
		t.Fatalf("expected temperature 42.5, got %f", state.Temperature)
	}
	if state.Voltage != 3.3 {
		t.Fatalf("expected voltage 3.3, got %f", state.Voltage)
	}
	if state.Distance != 1234 {
		t.Fatalf("expected distance 1234, got %d", state.Distance)
	}
	if state.UptimeSecs != 86400 {
		t.Fatalf("expected uptime 86400, got %d", state.UptimeSecs)
	}
}

func TestParseONTState_MinimalXML(t *testing.T) {
	xml := []byte(`<ont-state><serial-number>ADTN00000001</serial-number></ont-state>`)

	a := &Adapter{}
	state := a.parseONTState(xml)

	if state.SerialNumber != "ADTN00000001" {
		t.Fatalf("expected serial ADTN00000001, got %s", state.SerialNumber)
	}
	if state.ONTID != 0 {
		t.Fatalf("expected ONTID 0, got %d", state.ONTID)
	}
	if state.RxPower != 0 {
		t.Fatalf("expected rx-power 0, got %f", state.RxPower)
	}
}

func TestParseONTState_MalformedXML(t *testing.T) {
	xml := []byte(`<not-valid>garbage</>`)

	a := &Adapter{}
	state := a.parseONTState(xml)

	// Should return empty state without panic
	if state == nil {
		t.Fatal("expected non-nil state even for malformed XML")
	}
	if state.SerialNumber != "" {
		t.Fatalf("expected empty serial for malformed XML, got %s", state.SerialNumber)
	}
}

// --- parseServiceStats tests ---

func TestParseServiceStats_ValidXML(t *testing.T) {
	xml := []byte(`<service-state><service-id>svc-test</service-id><state>active</state><statistics><bytes-upstream>1000</bytes-upstream><bytes-downstream>2000</bytes-downstream><packets-upstream>10</packets-upstream><packets-downstream>20</packets-downstream></statistics></service-state>`)

	a := &Adapter{}
	stats := a.parseServiceStats(xml)

	if stats.ServiceID != "svc-test" {
		t.Fatalf("expected service-id svc-test, got %s", stats.ServiceID)
	}
	if stats.State != "active" {
		t.Fatalf("expected state active, got %s", stats.State)
	}
	if stats.BytesUp != 1000 {
		t.Fatalf("expected bytes-up 1000, got %d", stats.BytesUp)
	}
	if stats.BytesDown != 2000 {
		t.Fatalf("expected bytes-down 2000, got %d", stats.BytesDown)
	}
	if stats.PacketsUp != 10 {
		t.Fatalf("expected packets-up 10, got %d", stats.PacketsUp)
	}
	if stats.PacketsDown != 20 {
		t.Fatalf("expected packets-down 20, got %d", stats.PacketsDown)
	}
}

func TestParseServiceStats_EmptyXML(t *testing.T) {
	xml := []byte(`<service-state></service-state>`)

	a := &Adapter{}
	stats := a.parseServiceStats(xml)

	if stats.ServiceID != "" {
		t.Fatalf("expected empty service-id, got %s", stats.ServiceID)
	}
	if stats.BytesUp != 0 || stats.BytesDown != 0 {
		t.Fatal("expected zero counters for empty XML")
	}
}

// --- parseUptime tests ---

func TestParseUptime_Seconds(t *testing.T) {
	result := parseUptime("86400")
	if result != 86400 {
		t.Fatalf("expected 86400, got %d", result)
	}
}

func TestParseUptime_DurationFormat(t *testing.T) {
	result := parseUptime("1d2h30m45s")
	// 1*86400 + 2*3600 + 30*60 + 45 = 86400 + 7200 + 1800 + 45 = 95445
	expected := int64(95445)
	if result != expected {
		t.Fatalf("expected %d, got %d", expected, result)
	}
}

func TestParseUptime_Empty(t *testing.T) {
	result := parseUptime("")
	if result != 0 {
		t.Fatalf("expected 0 for empty, got %d", result)
	}
}

func TestParseUptime_OnlyHours(t *testing.T) {
	result := parseUptime("3h")
	if result != 10800 {
		t.Fatalf("expected 10800, got %d", result)
	}
}

// --- parseDiscoveredONTs tests ---

func TestParseDiscoveredONTs_MultiONT(t *testing.T) {
	xml := []byte(`<discovered-onts><ont><serial-number>ADTN11111111</serial-number><distance>500</distance><rx-power>-20.0</rx-power><discovery-time>2024-01-01</discovery-time></ont><ont><serial-number>ADTN22222222</serial-number><distance>750</distance><rx-power>-22.5</rx-power><discovery-time>2024-01-02</discovery-time></ont></discovered-onts>`)

	a := &Adapter{}
	onts := a.parseDiscoveredONTs(xml)

	if len(onts) != 2 {
		t.Fatalf("expected 2 ONTs, got %d", len(onts))
	}
	if onts[0].SerialNumber != "ADTN11111111" {
		t.Fatalf("expected serial ADTN11111111, got %s", onts[0].SerialNumber)
	}
	if onts[0].Distance != 500 {
		t.Fatalf("expected distance 500, got %d", onts[0].Distance)
	}
	if onts[0].RxPower != -20.0 {
		t.Fatalf("expected rx-power -20.0, got %f", onts[0].RxPower)
	}
	if onts[0].OperState != "discovered" {
		t.Fatalf("expected oper-state discovered, got %s", onts[0].OperState)
	}
	if onts[1].SerialNumber != "ADTN22222222" {
		t.Fatalf("expected serial ADTN22222222, got %s", onts[1].SerialNumber)
	}
	if onts[1].Distance != 750 {
		t.Fatalf("expected distance 750, got %d", onts[1].Distance)
	}
}

func TestParseDiscoveredONTs_Empty(t *testing.T) {
	xml := []byte(`<discovered-onts></discovered-onts>`)

	a := &Adapter{}
	onts := a.parseDiscoveredONTs(xml)

	if len(onts) != 0 {
		t.Fatalf("expected 0 ONTs for empty response, got %d", len(onts))
	}
}

// --- parseSystemInfo tests ---

func TestParseSystemInfo_ValidXML(t *testing.T) {
	xml := []byte(`<system-state><information><hostname>adtran-olt</hostname><model>SDX-6324</model><serial-number>ABC123</serial-number><software-version>1.2.3</software-version><uptime>86400</uptime></information><cpu-utilization><percent>45.5</percent></cpu-utilization><memory-utilization><percent>60.2</percent></memory-utilization></system-state>`)

	a := &Adapter{}
	info := a.parseSystemInfo(xml)

	if info.Hostname != "adtran-olt" {
		t.Fatalf("expected hostname adtran-olt, got %s", info.Hostname)
	}
	if info.Model != "SDX-6324" {
		t.Fatalf("expected model SDX-6324, got %s", info.Model)
	}
	if info.SerialNumber != "ABC123" {
		t.Fatalf("expected serial ABC123, got %s", info.SerialNumber)
	}
	if info.SoftwareVer != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %s", info.SoftwareVer)
	}
	if info.UptimeSecs != 86400 {
		t.Fatalf("expected uptime 86400, got %d", info.UptimeSecs)
	}
	if info.CPUPercent != 45.5 {
		t.Fatalf("expected CPU 45.5, got %f", info.CPUPercent)
	}
	if info.MemoryPercent != 60.2 {
		t.Fatalf("expected memory 60.2, got %f", info.MemoryPercent)
	}
}

func TestParseSystemInfo_EmptyXML(t *testing.T) {
	xml := []byte(`<system-state></system-state>`)

	a := &Adapter{}
	info := a.parseSystemInfo(xml)

	if info.Hostname != "" {
		t.Fatalf("expected empty hostname, got %s", info.Hostname)
	}
	if info.CPUPercent != 0 {
		t.Fatalf("expected zero CPU, got %f", info.CPUPercent)
	}
}

// --- extractSubscriberParams tests ---

func TestExtractSubscriberParams_WithAnnotations(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{config: cfg}

	sub := &model.Subscriber{
		Name: "test-sub",
		Annotations: map[string]string{
			"nanoncore.com/pon-port": "gpon-0/1/3",
			"nanoncore.com/ont-id":   "42",
		},
		Spec: model.SubscriberSpec{
			ONUSerial: "ADTN99999999",
			VLAN:      200,
		},
	}

	tier := &model.ServiceTier{
		Name: "premium",
		Annotations: map[string]string{
			"nanoncore.com/ont-profile":       "custom-ont",
			"nanoncore.com/service-profile":   "custom-svc",
			"nanoncore.com/bandwidth-profile": "custom-bw",
		},
		Spec: model.ServiceTierSpec{
			BandwidthUp:   100,
			BandwidthDown: 500,
		},
	}

	params := a.extractSubscriberParams(sub, tier)

	if params.SerialNumber != "ADTN99999999" {
		t.Fatalf("expected serial ADTN99999999, got %s", params.SerialNumber)
	}
	if params.PONPort != "gpon-0/1/3" {
		t.Fatalf("expected pon-port gpon-0/1/3, got %s", params.PONPort)
	}
	if params.ONTID != 42 {
		t.Fatalf("expected ont-id 42, got %d", params.ONTID)
	}
	if params.VLAN != 200 {
		t.Fatalf("expected VLAN 200, got %d", params.VLAN)
	}
	if params.BandwidthUp != 100 {
		t.Fatalf("expected bandwidth-up 100, got %d", params.BandwidthUp)
	}
	if params.BandwidthDown != 500 {
		t.Fatalf("expected bandwidth-down 500, got %d", params.BandwidthDown)
	}
	if params.ONTProfile != "custom-ont" {
		t.Fatalf("expected ont-profile custom-ont, got %s", params.ONTProfile)
	}
	if params.ServiceProfile != "custom-svc" {
		t.Fatalf("expected service-profile custom-svc, got %s", params.ServiceProfile)
	}
	if params.BandwidthProfile != "custom-bw" {
		t.Fatalf("expected bandwidth-profile custom-bw, got %s", params.BandwidthProfile)
	}
}

func TestExtractSubscriberParams_Defaults(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{config: cfg}

	sub := &model.Subscriber{
		Name: "test-sub",
		Spec: model.SubscriberSpec{
			ONUSerial: "ADTN00001234",
			VLAN:      300,
		},
	}

	tier := &model.ServiceTier{
		Name: "basic",
		Spec: model.ServiceTierSpec{
			BandwidthUp:   25,
			BandwidthDown: 100,
		},
	}

	params := a.extractSubscriberParams(sub, tier)

	// Default PON port from config (empty metadata) falls back to gpon-0/0/1
	if params.PONPort != "gpon-0/0/1" {
		t.Fatalf("expected default pon-port gpon-0/0/1, got %s", params.PONPort)
	}
	// ONT ID should be VLAN % 128 = 300 % 128 = 44
	if params.ONTID != 44 {
		t.Fatalf("expected ont-id 44 (300 mod 128), got %d", params.ONTID)
	}
	if params.ONTProfile != "nanoncore-ont-default" {
		t.Fatalf("expected default ont-profile, got %s", params.ONTProfile)
	}
	if params.ServiceProfile != "nanoncore-svc-100M" {
		t.Fatalf("expected default service-profile nanoncore-svc-100M, got %s", params.ServiceProfile)
	}
	if params.BandwidthProfile != "nanoncore-bw-100M" {
		t.Fatalf("expected default bandwidth-profile nanoncore-bw-100M, got %s", params.BandwidthProfile)
	}
}

func TestExtractSubscriberParams_NilTier(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{config: cfg}

	sub := &model.Subscriber{
		Name: "test-sub",
		Spec: model.SubscriberSpec{
			ONUSerial: "ADTN00005678",
			VLAN:      100,
		},
	}

	params := a.extractSubscriberParams(sub, nil)

	if params.BandwidthUp != 0 {
		t.Fatalf("expected bandwidth-up 0 with nil tier, got %d", params.BandwidthUp)
	}
	if params.ONTProfile != "nanoncore-ont-default" {
		t.Fatalf("expected default ont-profile, got %s", params.ONTProfile)
	}
	// With 0 BandwidthDown, profile names include 0M
	if params.ServiceProfile != "nanoncore-svc-0M" {
		t.Fatalf("expected nanoncore-svc-0M, got %s", params.ServiceProfile)
	}
}

func TestExtractSubscriberParams_PONPortFromConfig(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	cfg.Metadata["pon_port"] = "gpon-0/2/0"
	a := &Adapter{config: cfg}

	sub := &model.Subscriber{
		Name: "test-sub",
		Spec: model.SubscriberSpec{
			ONUSerial: "ADTN00001111",
			VLAN:      50,
		},
	}

	params := a.extractSubscriberParams(sub, nil)

	if params.PONPort != "gpon-0/2/0" {
		t.Fatalf("expected pon-port from config metadata, got %s", params.PONPort)
	}
}

// --- parseSubscriberSerial tests ---

func TestParseSubscriberSerial_AdtranPrefix(t *testing.T) {
	a := &Adapter{}
	result := a.parseSubscriberSerial("adtran-ADTN12345678")
	if result != "ADTN12345678" {
		t.Fatalf("expected ADTN12345678, got %s", result)
	}
}

func TestParseSubscriberSerial_BareSerial(t *testing.T) {
	a := &Adapter{}
	result := a.parseSubscriberSerial("ADTN12345678")
	if result != "ADTN12345678" {
		t.Fatalf("expected ADTN12345678, got %s", result)
	}
}

func TestParseSubscriberSerial_Fallback(t *testing.T) {
	a := &Adapter{}
	result := a.parseSubscriberSerial("abc/def")
	if result != "abc/def" {
		t.Fatalf("expected abc/def as fallback, got %s", result)
	}
}

func TestParseSubscriberSerial_ShortString(t *testing.T) {
	a := &Adapter{}
	result := a.parseSubscriberSerial("short")
	// Less than 8 chars, not matching adtran- prefix, fallback
	if result != "short" {
		t.Fatalf("expected short as fallback, got %s", result)
	}
}

// --- detectModel tests ---

func TestDetectModel_FromConfig(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	cfg.Metadata["model"] = "SDX-6320-16"
	a := &Adapter{config: cfg}

	if model := a.detectModel(); model != "SDX-6320-16" {
		t.Fatalf("expected SDX-6320-16, got %s", model)
	}
}

func TestDetectModel_Default(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{config: cfg}

	if model := a.detectModel(); model != "sdx-600" {
		t.Fatalf("expected sdx-600 default, got %s", model)
	}
}

// --- Driver method tests with MockNETCONFExecutor ---

func newTestAdapter() (*Adapter, *testutil.MockDriver, *testutil.MockNETCONFExecutor) {
	nc := &testutil.MockNETCONFExecutor{}
	mock := &testutil.MockDriver{
		Connected:   true,
		NETCONFExec: nc,
	}
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	driver := NewAdapter(mock, cfg)
	return driver.(*Adapter), mock, nc
}

func TestCreateSubscriber_Success(t *testing.T) {
	a, _, nc := newTestAdapter()
	ctx := context.Background()

	sub := testutil.NewTestSubscriber("ADTN12345678", "0/1", 100)
	tier := testutil.NewTestServiceTier(50, 200)

	result, err := a.CreateSubscriber(ctx, sub, tier)
	if err != nil {
		t.Fatalf("CreateSubscriber failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.SubscriberID != sub.Name {
		t.Fatalf("expected subscriber ID %s, got %s", sub.Name, result.SubscriberID)
	}
	if result.SessionID != "adtran-ADTN12345678" {
		t.Fatalf("expected session ID adtran-ADTN12345678, got %s", result.SessionID)
	}
	if result.VLAN != 100 {
		t.Fatalf("expected VLAN 100, got %d", result.VLAN)
	}
	if result.Metadata["vendor"] != "adtran" {
		t.Fatalf("expected vendor=adtran in metadata, got %v", result.Metadata["vendor"])
	}

	// Verify EditConfig was called
	found := false
	for _, call := range nc.Calls {
		if call == "EditConfig" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected EditConfig call, got %v", nc.Calls)
	}
}

func TestCreateSubscriber_NoNETCONF(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{
		baseDriver:      &testutil.MockDriver{Connected: true},
		netconfExecutor: nil,
		config:          cfg,
	}

	sub := testutil.NewTestSubscriber("ADTN12345678", "0/1", 100)
	tier := testutil.NewTestServiceTier(50, 200)

	_, err := a.CreateSubscriber(context.Background(), sub, tier)
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
	if !strings.Contains(err.Error(), "NETCONF executor not available") {
		t.Fatalf("expected NETCONF error message, got: %s", err.Error())
	}
}

func TestCreateSubscriber_EditConfigFails(t *testing.T) {
	a, _, nc := newTestAdapter()
	nc.EditConfigError = fmt.Errorf("config commit failed")

	sub := testutil.NewTestSubscriber("ADTN12345678", "0/1", 100)
	tier := testutil.NewTestServiceTier(50, 200)

	_, err := a.CreateSubscriber(context.Background(), sub, tier)
	if err == nil {
		t.Fatal("expected error when EditConfig fails")
	}
	if !strings.Contains(err.Error(), "ONT provisioning failed") {
		t.Fatalf("expected provisioning error, got: %s", err.Error())
	}
}

func TestUpdateSubscriber_Success(t *testing.T) {
	a, _, nc := newTestAdapter()

	sub := testutil.NewTestSubscriber("ADTN12345678", "0/1", 100)
	tier := testutil.NewTestServiceTier(50, 200)

	err := a.UpdateSubscriber(context.Background(), sub, tier)
	if err != nil {
		t.Fatalf("UpdateSubscriber failed: %v", err)
	}

	found := false
	for _, call := range nc.Calls {
		if call == "EditConfig" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected EditConfig call for UpdateSubscriber")
	}
}

func TestUpdateSubscriber_NoNETCONF(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{
		baseDriver: &testutil.MockDriver{Connected: true},
		config:     cfg,
	}

	sub := testutil.NewTestSubscriber("ADTN12345678", "0/1", 100)
	tier := testutil.NewTestServiceTier(50, 200)

	err := a.UpdateSubscriber(context.Background(), sub, tier)
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestDeleteSubscriber_Success(t *testing.T) {
	a, _, nc := newTestAdapter()

	err := a.DeleteSubscriber(context.Background(), "adtran-ADTN12345678")
	if err != nil {
		t.Fatalf("DeleteSubscriber failed: %v", err)
	}

	found := false
	for _, call := range nc.Calls {
		if call == "EditConfig" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected EditConfig call for DeleteSubscriber")
	}
}

func TestDeleteSubscriber_NoNETCONF(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{
		baseDriver: &testutil.MockDriver{Connected: true},
		config:     cfg,
	}

	err := a.DeleteSubscriber(context.Background(), "adtran-ADTN12345678")
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestSuspendSubscriber_Success(t *testing.T) {
	a, _, nc := newTestAdapter()

	err := a.SuspendSubscriber(context.Background(), "adtran-ADTN12345678")
	if err != nil {
		t.Fatalf("SuspendSubscriber failed: %v", err)
	}

	found := false
	for _, call := range nc.Calls {
		if call == "EditConfig" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected EditConfig call for SuspendSubscriber")
	}
}

func TestSuspendSubscriber_NoNETCONF(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{
		baseDriver: &testutil.MockDriver{Connected: true},
		config:     cfg,
	}

	err := a.SuspendSubscriber(context.Background(), "sub-1")
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestResumeSubscriber_Success(t *testing.T) {
	a, _, nc := newTestAdapter()

	err := a.ResumeSubscriber(context.Background(), "adtran-ADTN12345678")
	if err != nil {
		t.Fatalf("ResumeSubscriber failed: %v", err)
	}

	found := false
	for _, call := range nc.Calls {
		if call == "EditConfig" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected EditConfig call for ResumeSubscriber")
	}
}

func TestResumeSubscriber_NoNETCONF(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{
		baseDriver: &testutil.MockDriver{Connected: true},
		config:     cfg,
	}

	err := a.ResumeSubscriber(context.Background(), "sub-1")
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestGetSubscriberStatus_Success(t *testing.T) {
	a, _, nc := newTestAdapter()

	ontStateXML := []byte(`<ont-state><serial-number>ADTN12345678</serial-number><ont-id>1</ont-id><pon-port>gpon-0/0/1</pon-port><admin-state>enabled</admin-state><operational-status>online</operational-status><description>test</description><optical-info><rx-power>-18.5</rx-power><tx-power>2.3</tx-power><temperature>42.5</temperature><voltage>3.3</voltage></optical-info><distance>1234</distance><uptime>86400</uptime></ont-state>`)

	filter := fmt.Sprintf(GetONTStateFilterXML, "ADTN12345678")
	nc.GetResponses = map[string][]byte{
		filter: ontStateXML,
	}

	status, err := a.GetSubscriberStatus(context.Background(), "adtran-ADTN12345678")
	if err != nil {
		t.Fatalf("GetSubscriberStatus failed: %v", err)
	}
	if status.State != "online" {
		t.Fatalf("expected state online, got %s", status.State)
	}
	if !status.IsOnline {
		t.Fatal("expected IsOnline to be true for 'online' state")
	}
	if status.UptimeSeconds != 86400 {
		t.Fatalf("expected uptime 86400, got %d", status.UptimeSeconds)
	}
	if status.SessionID != "adtran-ADTN12345678" {
		t.Fatalf("expected session ID adtran-ADTN12345678, got %s", status.SessionID)
	}
	if status.Metadata["rx_power_dbm"] != -18.5 {
		t.Fatalf("expected rx_power_dbm -18.5, got %v", status.Metadata["rx_power_dbm"])
	}
}

func TestGetSubscriberStatus_NoNETCONF(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{
		baseDriver: &testutil.MockDriver{Connected: true},
		config:     cfg,
	}

	_, err := a.GetSubscriberStatus(context.Background(), "sub-1")
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestGetSubscriberStats_Success(t *testing.T) {
	a, _, nc := newTestAdapter()

	svcStatsXML := []byte(`<service-state><service-id>svc-ADTN12345678</service-id><state>active</state><statistics><bytes-upstream>1000</bytes-upstream><bytes-downstream>2000</bytes-downstream><packets-upstream>10</packets-upstream><packets-downstream>20</packets-downstream></statistics></service-state>`)

	filter := fmt.Sprintf(GetServiceStatsFilterXML, "svc-ADTN12345678")
	nc.GetResponses = map[string][]byte{
		filter: svcStatsXML,
	}

	stats, err := a.GetSubscriberStats(context.Background(), "adtran-ADTN12345678")
	if err != nil {
		t.Fatalf("GetSubscriberStats failed: %v", err)
	}
	if stats.BytesUp != 1000 {
		t.Fatalf("expected bytes-up 1000, got %d", stats.BytesUp)
	}
	if stats.BytesDown != 2000 {
		t.Fatalf("expected bytes-down 2000, got %d", stats.BytesDown)
	}
	if stats.PacketsUp != 10 {
		t.Fatalf("expected packets-up 10, got %d", stats.PacketsUp)
	}
	if stats.PacketsDown != 20 {
		t.Fatalf("expected packets-down 20, got %d", stats.PacketsDown)
	}
	if stats.Metadata["vendor"] != "adtran" {
		t.Fatalf("expected vendor=adtran in metadata, got %v", stats.Metadata["vendor"])
	}
}

func TestGetSubscriberStats_NoNETCONF(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{
		baseDriver: &testutil.MockDriver{Connected: true},
		config:     cfg,
	}

	_, err := a.GetSubscriberStats(context.Background(), "sub-1")
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestHealthCheck_WithNETCONF(t *testing.T) {
	a, _, nc := newTestAdapter()

	nc.GetResponses = map[string][]byte{
		GetSystemInfoFilterXML: []byte(`<system-state/>`),
	}

	err := a.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	found := false
	for _, call := range nc.Calls {
		if strings.HasPrefix(call, "Get:") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected Get call for HealthCheck with NETCONF")
	}
}

func TestHealthCheck_WithoutNETCONF(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	mockBase := &testutil.MockDriver{Connected: true}
	a := &Adapter{
		baseDriver: mockBase,
		config:     cfg,
	}

	err := a.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck without NETCONF failed: %v", err)
	}

	found := false
	for _, call := range mockBase.Calls {
		if call == "HealthCheck" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected base driver HealthCheck to be called when no NETCONF executor")
	}
}

func TestHealthCheck_NETCONFFails(t *testing.T) {
	a, _, nc := newTestAdapter()

	nc.GetErrors = map[string]error{
		GetSystemInfoFilterXML: fmt.Errorf("connection timeout"),
	}

	err := a.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("expected error when NETCONF Get fails")
	}
}

func TestCreateBandwidthProfile_Success(t *testing.T) {
	a, _, nc := newTestAdapter()

	tier := testutil.NewTestServiceTier(50, 200)

	err := a.CreateBandwidthProfile(context.Background(), tier)
	if err != nil {
		t.Fatalf("CreateBandwidthProfile failed: %v", err)
	}

	// Should have 2 EditConfig calls (upstream + downstream)
	editCount := 0
	for _, call := range nc.Calls {
		if call == "EditConfig" {
			editCount++
		}
	}
	if editCount != 2 {
		t.Fatalf("expected 2 EditConfig calls (up+down), got %d", editCount)
	}
}

func TestCreateBandwidthProfile_NoNETCONF(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{
		baseDriver: &testutil.MockDriver{Connected: true},
		config:     cfg,
	}

	tier := testutil.NewTestServiceTier(50, 200)
	err := a.CreateBandwidthProfile(context.Background(), tier)
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestCreateBandwidthProfile_EditConfigFails(t *testing.T) {
	a, _, nc := newTestAdapter()
	nc.EditConfigError = fmt.Errorf("commit failed")

	tier := testutil.NewTestServiceTier(50, 200)
	err := a.CreateBandwidthProfile(context.Background(), tier)
	if err == nil {
		t.Fatal("expected error when EditConfig fails")
	}
}

func TestCreateServiceProfile_Success(t *testing.T) {
	a, _, nc := newTestAdapter()

	err := a.CreateServiceProfile(context.Background(), "test-profile", 100, 200, 5, "bw-100M")
	if err != nil {
		t.Fatalf("CreateServiceProfile failed: %v", err)
	}

	found := false
	for _, call := range nc.Calls {
		if call == "EditConfig" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected EditConfig call for CreateServiceProfile")
	}
}

func TestCreateServiceProfile_NoNETCONF(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{
		baseDriver: &testutil.MockDriver{Connected: true},
		config:     cfg,
	}

	err := a.CreateServiceProfile(context.Background(), "test", 100, 200, 5, "bw")
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestDiscoverONTs_Success(t *testing.T) {
	a, _, nc := newTestAdapter()

	discoveredXML := []byte(`<discovered-onts><ont><serial-number>ADTN11111111</serial-number><distance>500</distance><rx-power>-20.0</rx-power><discovery-time>2024-01-01</discovery-time></ont></discovered-onts>`)

	ponPort := "gpon-0/0/1"
	filter := fmt.Sprintf(`
<gpon-state xmlns="http://www.adtran.com/ns/yang/adtran-gpon">
  <port>
    <port-id>%s</port-id>
    <discovered-onts/>
  </port>
</gpon-state>`, ponPort)

	nc.GetResponses = map[string][]byte{
		filter: discoveredXML,
	}

	onts, err := a.DiscoverONTs(context.Background(), ponPort)
	if err != nil {
		t.Fatalf("DiscoverONTs failed: %v", err)
	}
	if len(onts) != 1 {
		t.Fatalf("expected 1 ONT, got %d", len(onts))
	}
	if onts[0].SerialNumber != "ADTN11111111" {
		t.Fatalf("expected serial ADTN11111111, got %s", onts[0].SerialNumber)
	}
}

func TestDiscoverONTs_NoNETCONF(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{
		baseDriver: &testutil.MockDriver{Connected: true},
		config:     cfg,
	}

	_, err := a.DiscoverONTs(context.Background(), "gpon-0/0/1")
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestGetSystemInfo_Success(t *testing.T) {
	a, _, nc := newTestAdapter()

	sysInfoXML := []byte(`<system-state><information><hostname>adtran-olt</hostname><model>SDX-6324</model><serial-number>ABC123</serial-number><software-version>1.2.3</software-version><uptime>86400</uptime></information><cpu-utilization><percent>45.5</percent></cpu-utilization><memory-utilization><percent>60.2</percent></memory-utilization></system-state>`)

	nc.GetResponses = map[string][]byte{
		GetSystemInfoFilterXML: sysInfoXML,
	}

	info, err := a.GetSystemInfo(context.Background())
	if err != nil {
		t.Fatalf("GetSystemInfo failed: %v", err)
	}
	if info.Hostname != "adtran-olt" {
		t.Fatalf("expected hostname adtran-olt, got %s", info.Hostname)
	}
	if info.Model != "SDX-6324" {
		t.Fatalf("expected model SDX-6324, got %s", info.Model)
	}
	if info.CPUPercent != 45.5 {
		t.Fatalf("expected CPU 45.5, got %f", info.CPUPercent)
	}
}

func TestGetSystemInfo_NoNETCONF(t *testing.T) {
	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	a := &Adapter{
		baseDriver: &testutil.MockDriver{Connected: true},
		config:     cfg,
	}

	_, err := a.GetSystemInfo(context.Background())
	if err == nil {
		t.Fatal("expected error when NETCONF executor is nil")
	}
}

func TestGetSystemInfo_GetFails(t *testing.T) {
	a, _, nc := newTestAdapter()

	nc.GetErrors = map[string]error{
		GetSystemInfoFilterXML: fmt.Errorf("transport error"),
	}

	_, err := a.GetSystemInfo(context.Background())
	if err == nil {
		t.Fatal("expected error when NETCONF Get fails")
	}
}

// --- Delegation tests ---

func TestConnect_DelegatesToBaseDriver(t *testing.T) {
	a, mock, _ := newTestAdapter()

	cfg := testutil.NewTestEquipmentConfig(types.VendorAdtran, "10.0.0.1")
	err := a.Connect(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	found := false
	for _, call := range mock.Calls {
		if call == "Connect" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected Connect to delegate to base driver")
	}
}

func TestDisconnect_DelegatesToBaseDriver(t *testing.T) {
	a, mock, _ := newTestAdapter()

	err := a.Disconnect(context.Background())
	if err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	found := false
	for _, call := range mock.Calls {
		if call == "Disconnect" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected Disconnect to delegate to base driver")
	}
}

func TestIsConnected_DelegatesToBaseDriver(t *testing.T) {
	a, mock, _ := newTestAdapter()

	if !a.IsConnected() {
		t.Fatal("expected IsConnected to return true")
	}
	mock.Connected = false
	if a.IsConnected() {
		t.Fatal("expected IsConnected to return false after disconnecting")
	}
}

// --- Interface compliance ---

var _ types.Driver = (*Adapter)(nil)
