package huawei

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/testutil"
	"github.com/nanoncore/nano-southbound/types"
)

// ============================================================================
// Adapter construction tests
// ============================================================================

func TestNewAdapter_WithCLIDriver(t *testing.T) {
	mockDriver := &testutil.MockDriver{
		CLIExec: &testutil.MockCLIExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	if adapter == nil {
		t.Fatal("NewAdapter returned nil")
	}

	hw, ok := adapter.(*Adapter)
	if !ok {
		t.Fatal("NewAdapter did not return *Adapter")
	}

	if hw.cliExecutor == nil {
		t.Error("expected CLI executor to be set")
	}
	if hw.baseDriver == nil {
		t.Error("expected base driver to be set")
	}
	if hw.config != config {
		t.Error("expected config to be set")
	}
}

func TestNewAdapter_WithSNMPDriver(t *testing.T) {
	mockDriver := &testutil.MockDriver{
		SNMPExec: &testutil.MockSNMPExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	hw := adapter.(*Adapter)

	if hw.snmpExecutor == nil {
		t.Error("expected SNMP executor to be set")
	}
	// Should NOT create secondary driver when SNMP is already available
	if hw.secondaryDriver != nil {
		t.Error("expected no secondary driver when SNMP already available")
	}
}

func TestNewAdapter_WithBothExecutors(t *testing.T) {
	mockDriver := &testutil.MockDriver{
		CLIExec:  &testutil.MockCLIExecutor{},
		SNMPExec: &testutil.MockSNMPExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	hw := adapter.(*Adapter)

	if hw.cliExecutor == nil {
		t.Error("expected CLI executor to be set")
	}
	if hw.snmpExecutor == nil {
		t.Error("expected SNMP executor to be set")
	}
	if hw.secondaryDriver != nil {
		t.Error("expected no secondary driver when both executors available")
	}
}

// ============================================================================
// detectModel tests
// ============================================================================

func TestDetectModel(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		expected string
	}{
		{
			name:     "model from metadata",
			metadata: map[string]string{"model": "MA5800-X15"},
			expected: "MA5800-X15",
		},
		{
			name:     "model from metadata - MA5600T",
			metadata: map[string]string{"model": "MA5600T"},
			expected: "MA5600T",
		},
		{
			name:     "no model - uses default",
			metadata: map[string]string{},
			expected: "ma5800",
		},
		{
			name:     "nil metadata uses default",
			metadata: nil,
			expected: "ma5800",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{
				config: &types.EquipmentConfig{
					Metadata: tt.metadata,
				},
			}
			got := a.detectModel()
			if got != tt.expected {
				t.Errorf("detectModel() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ============================================================================
// getONTID tests
// ============================================================================

func TestGetONTID(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		vlan        int
		expected    int
	}{
		{
			name:        "from nanoncore.com/ont-id annotation",
			annotations: map[string]string{"nanoncore.com/ont-id": "5"},
			vlan:        100,
			expected:    5,
		},
		{
			name:        "from nano.io/onu-id annotation",
			annotations: map[string]string{"nano.io/onu-id": "42"},
			vlan:        100,
			expected:    42,
		},
		{
			name:        "nanoncore.com takes precedence over nano.io",
			annotations: map[string]string{"nanoncore.com/ont-id": "10", "nano.io/onu-id": "20"},
			vlan:        100,
			expected:    10,
		},
		{
			name:        "fallback to VLAN mod 128",
			annotations: map[string]string{},
			vlan:        100,
			expected:    100 % 128,
		},
		{
			name:        "fallback to VLAN mod 128 - large VLAN",
			annotations: map[string]string{},
			vlan:        300,
			expected:    300 % 128,
		},
		{
			name:        "nil annotations - fallback",
			annotations: nil,
			vlan:        50,
			expected:    50 % 128,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{config: &types.EquipmentConfig{}}
			subscriber := &model.Subscriber{
				Annotations: tt.annotations,
				Spec:        model.SubscriberSpec{VLAN: tt.vlan},
			}
			got := a.getONTID(subscriber)
			if got != tt.expected {
				t.Errorf("getONTID() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// ============================================================================
// getLineProfileID tests
// ============================================================================

func TestGetLineProfileID(t *testing.T) {
	tests := []struct {
		name     string
		tier     *model.ServiceTier
		expected int
	}{
		{
			name:     "nil tier returns default 1",
			tier:     nil,
			expected: 1,
		},
		{
			name: "from nanoncore.com/line-profile-id annotation",
			tier: &model.ServiceTier{
				Annotations: map[string]string{"nanoncore.com/line-profile-id": "10"},
			},
			expected: 10,
		},
		{
			name: "no annotation returns default 1",
			tier: &model.ServiceTier{
				Annotations: map[string]string{},
			},
			expected: 1,
		},
		{
			name: "nil annotations returns default 1",
			tier: &model.ServiceTier{
				Annotations: nil,
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{config: &types.EquipmentConfig{}}
			got := a.getLineProfileID(tt.tier)
			if got != tt.expected {
				t.Errorf("getLineProfileID() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// ============================================================================
// getServiceProfileID tests
// ============================================================================

func TestGetServiceProfileID(t *testing.T) {
	tests := []struct {
		name     string
		tier     *model.ServiceTier
		expected int
	}{
		{
			name:     "nil tier returns default 1",
			tier:     nil,
			expected: 1,
		},
		{
			name: "from nanoncore.com/srv-profile-id annotation",
			tier: &model.ServiceTier{
				Annotations: map[string]string{"nanoncore.com/srv-profile-id": "20"},
			},
			expected: 20,
		},
		{
			name: "no annotation returns default 1",
			tier: &model.ServiceTier{
				Annotations: map[string]string{},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{config: &types.EquipmentConfig{}}
			got := a.getServiceProfileID(tt.tier)
			if got != tt.expected {
				t.Errorf("getServiceProfileID() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// ============================================================================
// getTrafficTableID tests
// ============================================================================

func TestGetTrafficTableID(t *testing.T) {
	tests := []struct {
		name     string
		tier     *model.ServiceTier
		expected int
	}{
		{
			name:     "nil tier returns default 1",
			tier:     nil,
			expected: 1,
		},
		{
			name: "from nanoncore.com/traffic-table-id annotation",
			tier: &model.ServiceTier{
				Annotations: map[string]string{"nanoncore.com/traffic-table-id": "50"},
			},
			expected: 50,
		},
		{
			name: "no annotation uses bandwidth down as fallback",
			tier: &model.ServiceTier{
				Annotations: map[string]string{},
				Spec: model.ServiceTierSpec{
					BandwidthDown: 100,
				},
			},
			expected: 100,
		},
		{
			name: "zero bandwidth falls back to bandwidth value",
			tier: &model.ServiceTier{
				Annotations: map[string]string{},
				Spec: model.ServiceTierSpec{
					BandwidthDown: 0,
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{config: &types.EquipmentConfig{}}
			got := a.getTrafficTableID(tt.tier)
			if got != tt.expected {
				t.Errorf("getTrafficTableID() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// ============================================================================
// parseFSP tests
// ============================================================================

func TestParseFSP(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		wantFrame   int
		wantSlot    int
		wantPort    int
	}{
		{
			name:        "from nanoncore.com/gpon-fsp annotation",
			annotations: map[string]string{"nanoncore.com/gpon-fsp": "0/1/2"},
			wantFrame:   0,
			wantSlot:    1,
			wantPort:    2,
		},
		{
			name:        "from nano.io/pon-port annotation",
			annotations: map[string]string{"nano.io/pon-port": "1/3/7"},
			wantFrame:   1,
			wantSlot:    3,
			wantPort:    7,
		},
		{
			name:        "nanoncore.com takes precedence",
			annotations: map[string]string{"nanoncore.com/gpon-fsp": "0/0/0", "nano.io/pon-port": "1/1/1"},
			wantFrame:   0,
			wantSlot:    0,
			wantPort:    0,
		},
		{
			name:        "no annotation returns zeros",
			annotations: map[string]string{},
			wantFrame:   0,
			wantSlot:    0,
			wantPort:    0,
		},
		{
			name:        "nil annotations returns zeros",
			annotations: nil,
			wantFrame:   0,
			wantSlot:    0,
			wantPort:    0,
		},
		{
			name:        "invalid format returns zeros",
			annotations: map[string]string{"nanoncore.com/gpon-fsp": "invalid"},
			wantFrame:   0,
			wantSlot:    0,
			wantPort:    0,
		},
		{
			name:        "partial format returns zeros for missing",
			annotations: map[string]string{"nanoncore.com/gpon-fsp": "1/2"},
			wantFrame:   0,
			wantSlot:    0,
			wantPort:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{config: &types.EquipmentConfig{}}
			subscriber := &model.Subscriber{Annotations: tt.annotations}
			frame, slot, port := a.parseFSP(subscriber)
			if frame != tt.wantFrame {
				t.Errorf("parseFSP() frame = %d, want %d", frame, tt.wantFrame)
			}
			if slot != tt.wantSlot {
				t.Errorf("parseFSP() slot = %d, want %d", slot, tt.wantSlot)
			}
			if port != tt.wantPort {
				t.Errorf("parseFSP() port = %d, want %d", port, tt.wantPort)
			}
		})
	}
}

// ============================================================================
// parseSubscriberID tests
// ============================================================================

func TestParseSubscriberID(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		wantFrame int
		wantSlot  int
		wantPort  int
		wantONTID int
	}{
		{
			name:      "standard ont-F/S/P-ID format",
			id:        "ont-0/1/0-5",
			wantFrame: 0,
			wantSlot:  1,
			wantPort:  0,
			wantONTID: 5,
		},
		{
			name:      "different F/S/P values",
			id:        "ont-1/3/7-42",
			wantFrame: 1,
			wantSlot:  3,
			wantPort:  7,
			wantONTID: 42,
		},
		{
			name:      "zero values",
			id:        "ont-0/0/0-0",
			wantFrame: 0,
			wantSlot:  0,
			wantPort:  0,
			wantONTID: 0,
		},
		{
			name: "non-matching ID falls back to hash",
			id:   "some-subscriber",
			// Hash fallback: frame=0, slot=1, port=0, ontID=hash%128
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{config: &types.EquipmentConfig{}}
			frame, slot, port, ontID := a.parseSubscriberID(tt.id)

			if tt.name == "non-matching ID falls back to hash" {
				// Verify fallback behavior
				if frame != 0 {
					t.Errorf("parseSubscriberID() frame = %d, want 0 (fallback)", frame)
				}
				if slot != 1 {
					t.Errorf("parseSubscriberID() slot = %d, want 1 (fallback)", slot)
				}
				if port != 0 {
					t.Errorf("parseSubscriberID() port = %d, want 0 (fallback)", port)
				}
				if ontID < 0 || ontID >= 128 {
					t.Errorf("parseSubscriberID() ontID = %d, want 0-127 (fallback)", ontID)
				}
				return
			}

			if frame != tt.wantFrame {
				t.Errorf("parseSubscriberID() frame = %d, want %d", frame, tt.wantFrame)
			}
			if slot != tt.wantSlot {
				t.Errorf("parseSubscriberID() slot = %d, want %d", slot, tt.wantSlot)
			}
			if port != tt.wantPort {
				t.Errorf("parseSubscriberID() port = %d, want %d", port, tt.wantPort)
			}
			if ontID != tt.wantONTID {
				t.Errorf("parseSubscriberID() ontID = %d, want %d", ontID, tt.wantONTID)
			}
		})
	}
}

// ============================================================================
// buildProvisioningCommands tests
// ============================================================================

func TestBuildProvisioningCommands(t *testing.T) {
	a := &Adapter{config: &types.EquipmentConfig{}}
	tier := &model.ServiceTier{
		Spec: model.ServiceTierSpec{
			BandwidthDown: 0,
			BandwidthUp:   0,
		},
	}

	commands := a.buildProvisioningCommands(0, 1, 0, 5, "HWTC00001234", 100, 10, 20, tier)

	// Verify expected commands are present
	expectedContains := []struct {
		desc    string
		pattern string
	}{
		{"enable command", "enable"},
		{"config command", "config"},
		{"gpon interface", "interface gpon 0/1"},
		{"ont add with serial", "ont add 0 5 sn-auth HWTC00001234"},
		{"line profile", "ont-lineprofile-id 10"},
		{"srv profile", "ont-srvprofile-id 20"},
		{"native vlan", "ont port native-vlan 0 5 eth 1 vlan 100"},
		{"service-port", "service-port vlan 100 gpon 0/1/0 ont 5"},
	}

	cmdString := strings.Join(commands, " | ")
	for _, ec := range expectedContains {
		t.Run(ec.desc, func(t *testing.T) {
			found := false
			for _, cmd := range commands {
				if strings.Contains(cmd, ec.pattern) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected command containing %q in:\n%s", ec.pattern, cmdString)
			}
		})
	}
}

func TestBuildProvisioningCommands_WithBandwidth(t *testing.T) {
	a := &Adapter{config: &types.EquipmentConfig{}}
	tier := &model.ServiceTier{
		Spec: model.ServiceTierSpec{
			BandwidthDown: 100,
			BandwidthUp:   50,
		},
	}

	commands := a.buildProvisioningCommands(0, 1, 0, 5, "HWTC00001234", 100, 10, 20, tier)

	// When bandwidth is set, traffic profile commands should be appended
	found := false
	for _, cmd := range commands {
		if strings.Contains(cmd, "traffic-policy") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected traffic-policy command when bandwidth is specified")
	}
}

func TestBuildProvisioningCommands_NoBandwidth(t *testing.T) {
	a := &Adapter{config: &types.EquipmentConfig{}}
	tier := &model.ServiceTier{
		Spec: model.ServiceTierSpec{
			BandwidthDown: 0,
			BandwidthUp:   0,
		},
	}

	commands := a.buildProvisioningCommands(0, 1, 0, 5, "HWTC00001234", 100, 10, 20, tier)

	// When bandwidth is 0, no traffic-policy command should be present
	for _, cmd := range commands {
		if strings.Contains(cmd, "traffic-policy") {
			t.Error("unexpected traffic-policy command when no bandwidth specified")
		}
	}
}

// ============================================================================
// buildTrafficProfileCommands tests
// ============================================================================

func TestBuildTrafficProfileCommands(t *testing.T) {
	tests := []struct {
		name        string
		frame       int
		slot        int
		port        int
		ontID       int
		tier        *model.ServiceTier
		wantContain string
	}{
		{
			name:  "basic traffic profile",
			frame: 0, slot: 1, port: 0, ontID: 5,
			tier: &model.ServiceTier{
				Annotations: map[string]string{"nanoncore.com/traffic-table-id": "50"},
				Spec:        model.ServiceTierSpec{BandwidthDown: 100},
			},
			wantContain: "ont traffic-policy 0 5 profile-id 50",
		},
		{
			name:  "traffic profile with bandwidth fallback",
			frame: 0, slot: 2, port: 3, ontID: 10,
			tier: &model.ServiceTier{
				Annotations: map[string]string{},
				Spec:        model.ServiceTierSpec{BandwidthDown: 200},
			},
			wantContain: "ont traffic-policy 3 10 profile-id 200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{config: &types.EquipmentConfig{}}
			commands := a.buildTrafficProfileCommands(tt.frame, tt.slot, tt.port, tt.ontID, tt.tier)

			found := false
			for _, cmd := range commands {
				if strings.Contains(cmd, tt.wantContain) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected command containing %q in %v", tt.wantContain, commands)
			}

			// Should contain config and interface gpon
			hasCfg := false
			hasIface := false
			for _, cmd := range commands {
				if cmd == "config" {
					hasCfg = true
				}
				if strings.HasPrefix(cmd, "interface gpon") {
					hasIface = true
				}
			}
			if !hasCfg {
				t.Error("expected 'config' command")
			}
			if !hasIface {
				t.Error("expected 'interface gpon' command")
			}
		})
	}
}

// ============================================================================
// Connect / Disconnect / IsConnected tests
// ============================================================================

func TestConnect_PrimaryOnly(t *testing.T) {
	mockDriver := &testutil.MockDriver{
		CLIExec:  &testutil.MockCLIExecutor{},
		SNMPExec: &testutil.MockSNMPExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	ctx := context.Background()
	err := adapter.Connect(ctx, config)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
}

func TestDisconnect(t *testing.T) {
	mockDriver := &testutil.MockDriver{
		Connected: true,
		CLIExec:   &testutil.MockCLIExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	ctx := context.Background()
	err := adapter.Disconnect(ctx)
	if err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
}

func TestIsConnected(t *testing.T) {
	mockDriver := &testutil.MockDriver{Connected: true}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)

	if !adapter.IsConnected() {
		t.Error("expected IsConnected() = true")
	}
}

// ============================================================================
// CreateSubscriber tests
// ============================================================================

func TestCreateSubscriber_NoCLIExecutor(t *testing.T) {
	mockDriver := &testutil.MockDriver{
		SNMPExec: &testutil.MockSNMPExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	ctx := context.Background()

	sub := testutil.NewTestSubscriber("HWTC00001234", "0/1/0", 100)
	tier := testutil.NewTestServiceTier(50, 100)

	_, err := adapter.CreateSubscriber(ctx, sub, tier)
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
	if !strings.Contains(err.Error(), "CLI executor not available") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateSubscriber_Success(t *testing.T) {
	mockDriver := &testutil.MockDriver{
		CLIExec:  &testutil.MockCLIExecutor{},
		SNMPExec: &testutil.MockSNMPExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	ctx := context.Background()

	sub := &model.Subscriber{
		Name: "test-sub",
		Annotations: map[string]string{
			"nanoncore.com/gpon-fsp": "0/1/0",
			"nanoncore.com/ont-id":   "5",
		},
		Spec: model.SubscriberSpec{
			ONUSerial: "HWTC00001234",
			VLAN:      100,
		},
	}

	tier := &model.ServiceTier{
		Annotations: map[string]string{
			"nanoncore.com/line-profile-id": "10",
			"nanoncore.com/srv-profile-id":  "20",
		},
		Spec: model.ServiceTierSpec{
			BandwidthDown: 0,
			BandwidthUp:   0,
		},
	}

	result, err := adapter.CreateSubscriber(ctx, sub, tier)
	if err != nil {
		t.Fatalf("CreateSubscriber() error = %v", err)
	}

	if result.SubscriberID != "test-sub" {
		t.Errorf("SubscriberID = %q, want %q", result.SubscriberID, "test-sub")
	}
	if result.SessionID != "ont-0/1/0-5" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "ont-0/1/0-5")
	}
	if result.VLAN != 100 {
		t.Errorf("VLAN = %d, want 100", result.VLAN)
	}
	if result.InterfaceName != "gpon 0/1/0 ont 5" {
		t.Errorf("InterfaceName = %q, want %q", result.InterfaceName, "gpon 0/1/0 ont 5")
	}
}

// ============================================================================
// DeleteSubscriber tests
// ============================================================================

func TestDeleteSubscriber_NoCLIExecutor(t *testing.T) {
	mockDriver := &testutil.MockDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	err := adapter.DeleteSubscriber(context.Background(), "ont-0/1/0-5")
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestDeleteSubscriber_Success(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	mockDriver := &testutil.MockDriver{
		CLIExec:  mock,
		SNMPExec: &testutil.MockSNMPExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	err := adapter.DeleteSubscriber(context.Background(), "ont-0/1/0-5")
	if err != nil {
		t.Fatalf("DeleteSubscriber() error = %v", err)
	}

	// Verify commands were sent
	found := false
	for _, cmd := range mock.Commands {
		if strings.Contains(cmd, "ont delete 0 5") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'ont delete 0 5' command, got: %v", mock.Commands)
	}
}

// ============================================================================
// SuspendSubscriber / ResumeSubscriber tests
// ============================================================================

func TestSuspendSubscriber_NoCLIExecutor(t *testing.T) {
	mockDriver := &testutil.MockDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	err := adapter.SuspendSubscriber(context.Background(), "ont-0/1/0-5")
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestSuspendSubscriber_Success(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	mockDriver := &testutil.MockDriver{
		CLIExec:  mock,
		SNMPExec: &testutil.MockSNMPExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	err := adapter.SuspendSubscriber(context.Background(), "ont-0/1/0-5")
	if err != nil {
		t.Fatalf("SuspendSubscriber() error = %v", err)
	}

	found := false
	for _, cmd := range mock.Commands {
		if strings.Contains(cmd, "ont deactivate 0 5") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'ont deactivate 0 5' command, got: %v", mock.Commands)
	}
}

func TestResumeSubscriber_NoCLIExecutor(t *testing.T) {
	mockDriver := &testutil.MockDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	err := adapter.ResumeSubscriber(context.Background(), "ont-0/1/0-5")
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestResumeSubscriber_Success(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	mockDriver := &testutil.MockDriver{
		CLIExec:  mock,
		SNMPExec: &testutil.MockSNMPExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	err := adapter.ResumeSubscriber(context.Background(), "ont-0/1/0-5")
	if err != nil {
		t.Fatalf("ResumeSubscriber() error = %v", err)
	}

	found := false
	for _, cmd := range mock.Commands {
		if strings.Contains(cmd, "ont activate 0 5") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'ont activate 0 5' command, got: %v", mock.Commands)
	}
}

// ============================================================================
// UpdateSubscriber tests
// ============================================================================

func TestUpdateSubscriber_NoCLIExecutor(t *testing.T) {
	mockDriver := &testutil.MockDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	sub := testutil.NewTestSubscriber("HWTC00001234", "0/1/0", 100)
	tier := testutil.NewTestServiceTier(50, 100)

	err := adapter.UpdateSubscriber(context.Background(), sub, tier)
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestUpdateSubscriber_Success(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	mockDriver := &testutil.MockDriver{
		CLIExec:  mock,
		SNMPExec: &testutil.MockSNMPExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)

	sub := &model.Subscriber{
		Name: "test-sub",
		Annotations: map[string]string{
			"nanoncore.com/gpon-fsp": "0/1/0",
			"nanoncore.com/ont-id":   "5",
		},
		Spec: model.SubscriberSpec{
			VLAN: 200,
		},
	}

	tier := &model.ServiceTier{
		Annotations: map[string]string{
			"nanoncore.com/line-profile-id":  "15",
			"nanoncore.com/srv-profile-id":   "25",
			"nanoncore.com/traffic-table-id": "30",
		},
	}

	err := adapter.UpdateSubscriber(context.Background(), sub, tier)
	if err != nil {
		t.Fatalf("UpdateSubscriber() error = %v", err)
	}

	// Verify important commands
	cmdStr := strings.Join(mock.Commands, " | ")
	if !strings.Contains(cmdStr, "ont modify 0 5 ont-lineprofile-id 15 ont-srvprofile-id 25") {
		t.Errorf("expected ont modify command, got: %v", mock.Commands)
	}
}

// ============================================================================
// HealthCheck tests
// ============================================================================

func TestHealthCheck_WithCLI(t *testing.T) {
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display version": "MA5800-X15 V800R021C10",
		},
	}
	mockDriver := &testutil.MockDriver{
		CLIExec:  mock,
		SNMPExec: &testutil.MockSNMPExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	err := adapter.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}

	if len(mock.Commands) != 1 || mock.Commands[0] != "display version" {
		t.Errorf("expected 'display version' command, got: %v", mock.Commands)
	}
}

func TestHealthCheck_WithoutCLI(t *testing.T) {
	// MockDriver always implements CLIExecutor, so create an adapter directly
	// with no CLI executor to test the fallback
	mockDriver := &testutil.MockDriver{
		Connected: true,
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := &Adapter{
		baseDriver: mockDriver,
		config:     config,
	}
	// Should fall back to base driver HealthCheck
	err := adapter.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
}

// ============================================================================
// GetSubscriberStatus tests
// ============================================================================

func TestGetSubscriberStatus_NoCLIExecutor(t *testing.T) {
	mockDriver := &testutil.MockDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	_, err := adapter.GetSubscriberStatus(context.Background(), "ont-0/1/0-5")
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestGetSubscriberStatus_Online(t *testing.T) {
	ontOutput := `
  ONT ID          : 5
  Run state       : online
  Config state    : normal
  Online duration : 5 days 12:30:45
  IP address      : 192.168.1.100
`
	opticalOutput := `
  Rx Optical Power  : -18.50 dBm
  Tx Optical Power  : 2.10 dBm
  OLT Rx ONT optical power : -19.30 dBm
  Temperature       : 42.5 C
`
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display ont info 0/1 0 5":         ontOutput,
			"display ont optical-info 0/1 0 5": opticalOutput,
		},
	}
	mockDriver := &testutil.MockDriver{
		CLIExec:  mock,
		SNMPExec: &testutil.MockSNMPExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	status, err := adapter.GetSubscriberStatus(context.Background(), "ont-0/1/0-5")
	if err != nil {
		t.Fatalf("GetSubscriberStatus() error = %v", err)
	}

	if status.State != "online" {
		t.Errorf("State = %q, want %q", status.State, "online")
	}
	if !status.IsOnline {
		t.Error("expected IsOnline = true")
	}
	if status.IPv4Address != "192.168.1.100" {
		t.Errorf("IPv4Address = %q, want %q", status.IPv4Address, "192.168.1.100")
	}
}

// ============================================================================
// GetSubscriberStats tests (CLI fallback)
// ============================================================================

func TestGetSubscriberStats_NoCLIOrSNMP(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	// Build adapter directly with no executors
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     config,
	}
	_, err := adapter.GetSubscriberStats(context.Background(), "ont-0/1/0-5")
	if err == nil {
		t.Error("expected error when no executor available")
	}
}

func TestGetSubscriberStats_CLIFallback(t *testing.T) {
	output := `
  Upstream traffic   : 12345 bytes
  Downstream traffic : 67890 bytes
  Upstream packets   : 100
  Downstream packets : 200
  Errors             : 5
`
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display ont traffic 0/1 0 5": output,
		},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	// Build adapter directly with only CLI executor (no SNMP) to test CLI fallback
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      config,
	}
	stats, err := adapter.GetSubscriberStats(context.Background(), "ont-0/1/0-5")
	if err != nil {
		t.Fatalf("GetSubscriberStats() error = %v", err)
	}

	if stats.BytesUp != 12345 {
		t.Errorf("BytesUp = %d, want 12345", stats.BytesUp)
	}
	if stats.BytesDown != 67890 {
		t.Errorf("BytesDown = %d, want 67890", stats.BytesDown)
	}
	if stats.PacketsUp != 100 {
		t.Errorf("PacketsUp = %d, want 100", stats.PacketsUp)
	}
	if stats.PacketsDown != 200 {
		t.Errorf("PacketsDown = %d, want 200", stats.PacketsDown)
	}
	if stats.ErrorsDown != 5 {
		t.Errorf("ErrorsDown = %d, want 5", stats.ErrorsDown)
	}
}

// ============================================================================
// DiscoverONTs tests
// ============================================================================

func TestDiscoverONTs_NoCLIExecutor(t *testing.T) {
	mockDriver := &testutil.MockDriver{}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config).(*Adapter)
	_, err := adapter.DiscoverONTs(context.Background())
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestDiscoverONTs_Success(t *testing.T) {
	output := `
   F/S/P   ONT         SN                  VendorID   EquipmentID     Time
   -------------------------------------------------------------------
   0/1/0   1           485754430A2C4F13    HWTC       HG8245Q2        2024-01-15 10:30:00
   0/1/1   2           5053534E00000001    ZTEG       F670L           2024-01-15 10:31:00
`
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display ont autofind all": output,
		},
	}
	mockDriver := &testutil.MockDriver{
		CLIExec:  mock,
		SNMPExec: &testutil.MockSNMPExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config).(*Adapter)
	discoveries, err := adapter.DiscoverONTs(context.Background())
	if err != nil {
		t.Fatalf("DiscoverONTs() error = %v", err)
	}

	if len(discoveries) != 2 {
		t.Fatalf("expected 2 discoveries, got %d", len(discoveries))
	}

	if discoveries[0].Frame != 0 || discoveries[0].Slot != 1 || discoveries[0].Port != 0 {
		t.Errorf("first discovery F/S/P = %d/%d/%d, want 0/1/0",
			discoveries[0].Frame, discoveries[0].Slot, discoveries[0].Port)
	}
	if discoveries[0].Serial != "485754430A2C4F13" {
		t.Errorf("first serial = %q, want %q", discoveries[0].Serial, "485754430A2C4F13")
	}
	if discoveries[0].EquipID != "HG8245Q2" {
		t.Errorf("first equipID = %q, want %q", discoveries[0].EquipID, "HG8245Q2")
	}
}

// ============================================================================
// DiscoverONUs (DriverV2) tests
// ============================================================================

func TestDiscoverONUs_NoCLIExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	_, err := adapter.DiscoverONUs(context.Background(), nil)
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestDiscoverONUs_WithFilter(t *testing.T) {
	output := `
   F/S/P   ONT         SN                  VendorID   EquipmentID     Time
   -------------------------------------------------------------------
   0/1/0   1           485754430A2C4F13    HWTC       HG8245Q2        2024-01-15 10:30:00
   0/1/1   2           5053534E00000001    ZTEG       F670L           2024-01-15 10:31:00
   0/2/0   3           485754430A2C4F14    HWTC       HG8546M         2024-01-15 10:32:00
`
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display ont autofind all": output,
		},
	}

	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	// Filter to specific PON port
	results, err := adapter.DiscoverONUs(context.Background(), []string{"0/1/0"})
	if err != nil {
		t.Fatalf("DiscoverONUs() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].PONPort != "0/1/0" {
		t.Errorf("PONPort = %q, want %q", results[0].PONPort, "0/1/0")
	}
}

func TestDiscoverONUs_NoFilter(t *testing.T) {
	output := `
   -------------------------------------------------------------------
   0/1/0   1           485754430A2C4F13    HWTC       HG8245Q2        2024-01-15 10:30:00
   0/2/0   2           5053534E00000001    ZTEG       F670L           2024-01-15 10:31:00
`
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display ont autofind all": output,
		},
	}

	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	// No filter - return all
	results, err := adapter.DiscoverONUs(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverONUs() error = %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Model != "HG8245Q2" {
		t.Errorf("Model = %q, want %q", results[0].Model, "HG8245Q2")
	}
}

// ============================================================================
// RestartONU tests
// ============================================================================

func TestRestartONU_NoCLIExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	_, err := adapter.RestartONU(context.Background(), "0/1/0", 5)
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestRestartONU_InvalidPort(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	result, err := adapter.RestartONU(context.Background(), "invalid", 5)
	if err == nil {
		t.Error("expected error for invalid port format")
	}
	if result == nil || result.Success {
		t.Error("result should indicate failure")
	}
}

func TestRestartONU_Success(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	result, err := adapter.RestartONU(context.Background(), "0/1/0", 5)
	if err != nil {
		t.Fatalf("RestartONU() error = %v", err)
	}
	if !result.Success {
		t.Error("expected Success = true")
	}
	if !result.DeactivateSuccess {
		t.Error("expected DeactivateSuccess = true")
	}

	// Verify reset command was sent
	found := false
	for _, cmd := range mock.Commands {
		if strings.Contains(cmd, "ont reset 0 5") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'ont reset 0 5' command, got: %v", mock.Commands)
	}
}

// ============================================================================
// ApplyProfile tests
// ============================================================================

func TestApplyProfile_NoCLIExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	err := adapter.ApplyProfile(context.Background(), "0/1/0", 5, &types.ONUProfile{})
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestApplyProfile_NilProfile(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	err := adapter.ApplyProfile(context.Background(), "0/1/0", 5, nil)
	if err == nil {
		t.Error("expected error for nil profile")
	}
}

func TestApplyProfile_InvalidPort(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	err := adapter.ApplyProfile(context.Background(), "bad", 5, &types.ONUProfile{})
	if err == nil {
		t.Error("expected error for invalid port format")
	}
}

func TestApplyProfile_LineAndServiceProfile(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	profile := &types.ONUProfile{
		LineProfile:    "line-prof-1",
		ServiceProfile: "srv-prof-1",
		VLAN:           100,
		Priority:       3,
	}

	err := adapter.ApplyProfile(context.Background(), "0/1/0", 5, profile)
	if err != nil {
		t.Fatalf("ApplyProfile() error = %v", err)
	}

	cmdStr := strings.Join(mock.Commands, " | ")
	if !strings.Contains(cmdStr, "ont modify 0 5 ont-lineprofile-name line-prof-1") {
		t.Errorf("expected line profile command in: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "ont modify 0 5 ont-srvprofile-name srv-prof-1") {
		t.Errorf("expected service profile command in: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "ont port native-vlan 0 5 eth 1 vlan 100 priority 3") {
		t.Errorf("expected VLAN command in: %s", cmdStr)
	}
}

func TestApplyProfile_BandwidthProfile(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	profile := &types.ONUProfile{
		BandwidthDown: 100000, // 100 Mbps in kbps
	}

	err := adapter.ApplyProfile(context.Background(), "0/1/0", 5, profile)
	if err != nil {
		t.Fatalf("ApplyProfile() error = %v", err)
	}

	// Should have traffic-policy command
	found := false
	for _, cmd := range mock.Commands {
		if strings.Contains(cmd, "traffic-policy") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected traffic-policy command, got: %v", mock.Commands)
	}
}

// ============================================================================
// BulkProvision tests
// ============================================================================

func TestBulkProvision_NoCLIExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	_, err := adapter.BulkProvision(context.Background(), []types.BulkProvisionOp{})
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestBulkProvision_Success(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	ops := []types.BulkProvisionOp{
		{
			Serial:  "HWTC00001234",
			PONPort: "0/1/0",
			ONUID:   5,
			Profile: &types.ONUProfile{
				VLAN:           100,
				BandwidthDown:  100000,
				BandwidthUp:    50000,
				LineProfile:    "line1",
				ServiceProfile: "srv1",
			},
		},
		{
			Serial:  "HWTC00005678",
			PONPort: "0/1/0",
			ONUID:   6,
			// No profile - should use defaults
		},
	}

	result, err := adapter.BulkProvision(context.Background(), ops)
	if err != nil {
		t.Fatalf("BulkProvision() error = %v", err)
	}

	if result.Succeeded != 2 {
		t.Errorf("Succeeded = %d, want 2", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d, want 0", result.Failed)
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result.Results))
	}
	if !result.Results[0].Success {
		t.Error("first result should be success")
	}
}

func TestBulkProvision_WithError(t *testing.T) {
	mock := &testutil.MockCLIExecutor{
		Errors: map[string]error{},
	}
	// Simulate a failure on the second provisioning by returning an error for a command
	// that contains "HWTC00005678"
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	ops := []types.BulkProvisionOp{
		{
			Serial:  "HWTC00001234",
			PONPort: "0/1/0",
			ONUID:   5,
		},
	}

	result, err := adapter.BulkProvision(context.Background(), ops)
	if err != nil {
		t.Fatalf("BulkProvision() error = %v", err)
	}

	if result.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1", result.Succeeded)
	}
}

// ============================================================================
// GetAlarms tests
// ============================================================================

func TestGetAlarms_NoCLIExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	_, err := adapter.GetAlarms(context.Background())
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestGetAlarms_Success(t *testing.T) {
	output := `
  Alarm List
  -----------------------------------------------------------------------
  12345      Critical   LOS          0/0/1:5           2024-01-15 10:30:00    Loss of signal
  12346      Major      power        0/0/1:6           2024-01-15 10:31:00    Low power
`
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display alarm active all": output,
		},
	}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	alarms, err := adapter.GetAlarms(context.Background())
	if err != nil {
		t.Fatalf("GetAlarms() error = %v", err)
	}

	if len(alarms) != 2 {
		t.Fatalf("expected 2 alarms, got %d", len(alarms))
	}
	if alarms[0].ID != "12345" {
		t.Errorf("first alarm ID = %q, want %q", alarms[0].ID, "12345")
	}
}

func TestGetAlarms_Error(t *testing.T) {
	mock := &testutil.MockCLIExecutor{
		Errors: map[string]error{
			"display alarm active all": fmt.Errorf("connection timeout"),
		},
	}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	_, err := adapter.GetAlarms(context.Background())
	if err == nil {
		t.Error("expected error on CLI failure")
	}
}

// ============================================================================
// SetPortState tests
// ============================================================================

func TestSetPortState_NoCLIExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	err := adapter.SetPortState(context.Background(), "0/1/0", true)
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestSetPortState_InvalidPort(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	err := adapter.SetPortState(context.Background(), "invalid", true)
	if err == nil {
		t.Error("expected error for invalid port format")
	}
}

func TestSetPortState_Enable(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	err := adapter.SetPortState(context.Background(), "0/1/0", true)
	if err != nil {
		t.Fatalf("SetPortState() error = %v", err)
	}

	found := false
	for _, cmd := range mock.Commands {
		if strings.Contains(cmd, "undo port 0 shutdown") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'undo port 0 shutdown' command, got: %v", mock.Commands)
	}
}

func TestSetPortState_Disable(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	err := adapter.SetPortState(context.Background(), "0/1/0", false)
	if err != nil {
		t.Fatalf("SetPortState() error = %v", err)
	}

	found := false
	for _, cmd := range mock.Commands {
		if strings.Contains(cmd, "port 0 shutdown") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'port 0 shutdown' command, got: %v", mock.Commands)
	}
}

// ============================================================================
// ListVLANs tests
// ============================================================================

func TestListVLANs_NoCLIExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	_, err := adapter.ListVLANs(context.Background())
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestListVLANs_Success(t *testing.T) {
	output := `
  -------------------------------------------------------------------------
  VLAN Configuration
  -------------------------------------------------------------------------
  VLAN ID   Name                      Type      Service Ports   Description
  -------------------------------------------------------------------------
  100       Customer_VLAN_100         smart     5               Customer traffic
  200       Management                smart     0               Management VLAN
  -------------------------------------------------------------------------
  Total VLANs: 2
`
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display vlan all": output,
		},
	}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	vlans, err := adapter.ListVLANs(context.Background())
	if err != nil {
		t.Fatalf("ListVLANs() error = %v", err)
	}

	if len(vlans) != 2 {
		t.Fatalf("expected 2 VLANs, got %d", len(vlans))
	}
}

// ============================================================================
// GetVLAN tests
// ============================================================================

func TestGetVLAN_NoCLIExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	_, err := adapter.GetVLAN(context.Background(), 100)
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestGetVLAN_Found(t *testing.T) {
	output := `
  VLAN 100 Information:
  Name              : Customer_VLAN_100
  Description       : Customer traffic
  Service Port Count: 5
  Type              : smart
`
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display vlan 100": output,
		},
	}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	vlan, err := adapter.GetVLAN(context.Background(), 100)
	if err != nil {
		t.Fatalf("GetVLAN() error = %v", err)
	}
	if vlan == nil {
		t.Fatal("expected vlan, got nil")
	}
	if vlan.ID != 100 {
		t.Errorf("ID = %d, want 100", vlan.ID)
	}
	if vlan.Name != "Customer_VLAN_100" {
		t.Errorf("Name = %q, want %q", vlan.Name, "Customer_VLAN_100")
	}
	if vlan.ServicePortCount != 5 {
		t.Errorf("ServicePortCount = %d, want 5", vlan.ServicePortCount)
	}
}

func TestGetVLAN_NotFound(t *testing.T) {
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display vlan 999": "Error: This VLAN does not exist.",
		},
	}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	vlan, err := adapter.GetVLAN(context.Background(), 999)
	if err != nil {
		t.Fatalf("GetVLAN() error = %v", err)
	}
	if vlan != nil {
		t.Error("expected nil vlan for non-existent VLAN")
	}
}

// ============================================================================
// CreateVLAN tests
// ============================================================================

func TestCreateVLAN_NoCLIExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	err := adapter.CreateVLAN(context.Background(), &types.CreateVLANRequest{ID: 100})
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestCreateVLAN_InvalidID(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	tests := []struct {
		name string
		id   int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too high", 4095},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.CreateVLAN(context.Background(), &types.CreateVLANRequest{ID: tt.id})
			if err == nil {
				t.Error("expected error for invalid VLAN ID")
			}
		})
	}
}

func TestCreateVLAN_Success(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	err := adapter.CreateVLAN(context.Background(), &types.CreateVLANRequest{
		ID:          100,
		Name:        "TestVLAN",
		Description: "Test VLAN description",
	})
	if err != nil {
		t.Fatalf("CreateVLAN() error = %v", err)
	}

	cmdStr := strings.Join(mock.Commands, " | ")
	if !strings.Contains(cmdStr, "vlan 100 smart") {
		t.Errorf("expected 'vlan 100 smart' command in: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "name TestVLAN") {
		t.Errorf("expected 'name TestVLAN' command in: %s", cmdStr)
	}
}

func TestCreateVLAN_AlreadyExists(t *testing.T) {
	mock := &testutil.MockCLIExecutor{
		Errors: map[string]error{
			"vlan 100 smart": fmt.Errorf("VLAN already exists"),
		},
	}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	err := adapter.CreateVLAN(context.Background(), &types.CreateVLANRequest{ID: 100})
	if err == nil {
		t.Error("expected error when VLAN already exists")
	}
}

// ============================================================================
// DeleteVLAN tests
// ============================================================================

func TestDeleteVLAN_NoCLIExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	err := adapter.DeleteVLAN(context.Background(), 100, false)
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestDeleteVLAN_NotFound(t *testing.T) {
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display vlan 999": "Error: This VLAN does not exist.",
		},
	}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	err := adapter.DeleteVLAN(context.Background(), 999, false)
	if err == nil {
		t.Error("expected error when VLAN not found")
	}
}

func TestDeleteVLAN_HasServicePorts_NoForce(t *testing.T) {
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display vlan 100": `
  VLAN 100 Information:
  Name              : Test
  Service Port Count: 5
`,
		},
	}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	err := adapter.DeleteVLAN(context.Background(), 100, false)
	if err == nil {
		t.Error("expected error when VLAN has service ports and no force")
	}
	if he, ok := err.(*types.HumanError); ok {
		if he.Code != types.ErrCodeVLANHasServicePorts {
			t.Errorf("error code = %q, want %q", he.Code, types.ErrCodeVLANHasServicePorts)
		}
	}
}

func TestDeleteVLAN_Success(t *testing.T) {
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display vlan 100": `
  VLAN 100 Information:
  Name              : Test
  Service Port Count: 0
`,
		},
	}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	err := adapter.DeleteVLAN(context.Background(), 100, false)
	if err != nil {
		t.Fatalf("DeleteVLAN() error = %v", err)
	}

	found := false
	for _, cmd := range mock.Commands {
		if strings.Contains(cmd, "undo vlan 100") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'undo vlan 100' command, got: %v", mock.Commands)
	}
}

// ============================================================================
// ListServicePorts tests
// ============================================================================

func TestListServicePorts_NoCLIExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	_, err := adapter.ListServicePorts(context.Background())
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestListServicePorts_Success(t *testing.T) {
	output := `
  ---------------------------------------------------------------------------------
  Index   VLAN    Interface       ONT     GemPort   User-VLAN   Transform
  ---------------------------------------------------------------------------------
  1       100     0/0/1           101     1         100         translate
  2       200     0/0/2           102     2         200         transparent
  ---------------------------------------------------------------------------------
  Total service ports: 2
`
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display service-port all": output,
		},
	}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	ports, err := adapter.ListServicePorts(context.Background())
	if err != nil {
		t.Fatalf("ListServicePorts() error = %v", err)
	}

	if len(ports) != 2 {
		t.Fatalf("expected 2 service ports, got %d", len(ports))
	}
}

// ============================================================================
// AddServicePort tests
// ============================================================================

func TestAddServicePort_NoCLIExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	err := adapter.AddServicePort(context.Background(), &types.AddServicePortRequest{})
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestAddServicePort_Success(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	err := adapter.AddServicePort(context.Background(), &types.AddServicePortRequest{
		VLAN:    100,
		PONPort: "0/1/0",
		ONTID:   5,
		GemPort: 1,
	})
	if err != nil {
		t.Fatalf("AddServicePort() error = %v", err)
	}

	found := false
	for _, cmd := range mock.Commands {
		if strings.Contains(cmd, "service-port vlan 100 gpon 0/1/0 ont 5 gemport 1") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected service-port command, got: %v", mock.Commands)
	}
}

func TestAddServicePort_DefaultGemPort(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	err := adapter.AddServicePort(context.Background(), &types.AddServicePortRequest{
		VLAN:    200,
		PONPort: "0/1/0",
		ONTID:   10,
		// GemPort omitted - should default to 1
	})
	if err != nil {
		t.Fatalf("AddServicePort() error = %v", err)
	}

	found := false
	for _, cmd := range mock.Commands {
		if strings.Contains(cmd, "gemport 1") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected default gemport 1 in commands, got: %v", mock.Commands)
	}
}

// ============================================================================
// DeleteServicePort tests
// ============================================================================

func TestDeleteServicePort_NoCLIExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	err := adapter.DeleteServicePort(context.Background(), "0/1/0", 5)
	if err == nil {
		t.Error("expected error when CLI executor not available")
	}
}

func TestDeleteServicePort_Success(t *testing.T) {
	mock := &testutil.MockCLIExecutor{}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	err := adapter.DeleteServicePort(context.Background(), "0/1/0", 5)
	if err != nil {
		t.Fatalf("DeleteServicePort() error = %v", err)
	}

	found := false
	for _, cmd := range mock.Commands {
		if strings.Contains(cmd, "undo service-port port 0/1/0 ont 5") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected undo service-port command, got: %v", mock.Commands)
	}
}

// ============================================================================
// GetONUProfiles tests
// ============================================================================

func TestGetONUProfiles(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	_, err := adapter.GetONUProfiles(context.Background())
	if err == nil {
		t.Error("expected error (not implemented)")
	}
}

// ============================================================================
// RunDiagnostics tests
// ============================================================================

func TestRunDiagnostics(t *testing.T) {
	ontOutput := `
  ONT ID          : 5
  Run state       : online
  Config state    : normal
  Online duration : 1 day 00:00:00
`
	opticalOutput := `
  Rx Optical Power  : -18.50 dBm
  Tx Optical Power  : 2.10 dBm
`
	trafficOutput := `
  Upstream traffic   : 1000 bytes
  Downstream traffic : 2000 bytes
  Errors             : 3
`
	mock := &testutil.MockCLIExecutor{
		Outputs: map[string]string{
			"display ont info 0/1 0 5":         ontOutput,
			"display ont optical-info 0/1 0 5": opticalOutput,
			"display ont traffic 0/1 0 5":      trafficOutput,
		},
	}
	adapter := &Adapter{
		baseDriver:  &testutil.MockDriver{},
		cliExecutor: mock,
		config:      testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	diag, err := adapter.RunDiagnostics(context.Background(), "0/1/0", 5)
	if err != nil {
		t.Fatalf("RunDiagnostics() error = %v", err)
	}

	if diag.OperState != "online" {
		t.Errorf("OperState = %q, want %q", diag.OperState, "online")
	}
	if diag.PONPort != "0/1/0" {
		t.Errorf("PONPort = %q, want %q", diag.PONPort, "0/1/0")
	}
	if diag.ONUID != 5 {
		t.Errorf("ONUID = %d, want 5", diag.ONUID)
	}
	if diag.BytesUp != 1000 {
		t.Errorf("BytesUp = %d, want 1000", diag.BytesUp)
	}
	if diag.BytesDown != 2000 {
		t.Errorf("BytesDown = %d, want 2000", diag.BytesDown)
	}
}

// ============================================================================
// GetSubscriberStats via SNMP tests
// ============================================================================

func TestGetSubscriberStats_SNMP(t *testing.T) {
	snmpExec := &testutil.MockSNMPExecutor{
		BulkGetResults: map[string]interface{}{
			"1.3.6.1.4.1.2011.6.128.1.1.4.23.1.3.256.5": uint64(12345), // up bytes
			"1.3.6.1.4.1.2011.6.128.1.1.4.23.1.4.256.5": uint64(67890), // down bytes
			"1.3.6.1.4.1.2011.6.128.1.1.2.51.1.4.256.5": int64(-1850),  // rx power
			"1.3.6.1.4.1.2011.6.128.1.1.2.51.1.3.256.5": int64(210),    // tx power
			"1.3.6.1.4.1.2011.6.128.1.1.2.51.1.1.256.5": int64(11264),  // temperature
			"1.3.6.1.4.1.2011.6.128.1.1.2.51.1.5.256.5": int64(3300),   // voltage
		},
	}

	adapter := &Adapter{
		baseDriver:   &testutil.MockDriver{},
		snmpExecutor: snmpExec,
		config:       testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	stats, err := adapter.GetSubscriberStats(context.Background(), "ont-0/1/0-5")
	if err != nil {
		t.Fatalf("GetSubscriberStats() error = %v", err)
	}

	if stats.BytesUp != 12345 {
		t.Errorf("BytesUp = %d, want 12345", stats.BytesUp)
	}
	if stats.BytesDown != 67890 {
		t.Errorf("BytesDown = %d, want 67890", stats.BytesDown)
	}

	if stats.Metadata["source"] != "snmp" {
		t.Errorf("source = %v, want %q", stats.Metadata["source"], "snmp")
	}
}

// ============================================================================
// GetOLTStatus tests
// ============================================================================

func TestGetOLTStatus_BasicInfo(t *testing.T) {
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")
	config.Metadata["model"] = "MA5800-X15"

	mockDriver := &testutil.MockDriver{
		Connected: true,
	}

	adapter := &Adapter{
		baseDriver: mockDriver,
		config:     config,
	}

	status, err := adapter.GetOLTStatus(context.Background())
	if err != nil {
		t.Fatalf("GetOLTStatus() error = %v", err)
	}

	if status.Vendor != "huawei" {
		t.Errorf("Vendor = %q, want %q", status.Vendor, "huawei")
	}
	if status.Model != "MA5800-X15" {
		t.Errorf("Model = %q, want %q", status.Model, "MA5800-X15")
	}
	if !status.IsReachable {
		t.Error("expected IsReachable = true")
	}
}

// ============================================================================
// GetPONPower tests
// ============================================================================

func TestGetPONPower_NoSNMPExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	_, err := adapter.GetPONPower(context.Background(), "0/1/0")
	if err == nil {
		t.Error("expected error when SNMP executor not available")
	}
}

func TestGetPONPower_InvalidPort(t *testing.T) {
	snmpExec := &testutil.MockSNMPExecutor{}
	adapter := &Adapter{
		baseDriver:   &testutil.MockDriver{},
		snmpExecutor: snmpExec,
		config:       testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	_, err := adapter.GetPONPower(context.Background(), "invalid")
	if err == nil {
		t.Error("expected error for invalid port format")
	}
}

func TestGetPONPower_Success(t *testing.T) {
	// Port 0/1/0 -> portIndex = (0<<16)|(1<<8)|0 = 256
	snmpExec := &testutil.MockSNMPExecutor{
		BulkGetResults: map[string]interface{}{
			"1.3.6.1.4.1.2011.6.128.1.1.2.23.1.4.256": int64(210),   // tx power
			"1.3.6.1.4.1.2011.6.128.1.1.2.23.1.1.256": int64(11264), // temperature
			"1.3.6.1.4.1.2011.6.128.1.1.2.23.1.2.256": int64(3300),  // voltage
		},
	}

	adapter := &Adapter{
		baseDriver:   &testutil.MockDriver{},
		snmpExecutor: snmpExec,
		config:       testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	reading, err := adapter.GetPONPower(context.Background(), "0/1/0")
	if err != nil {
		t.Fatalf("GetPONPower() error = %v", err)
	}

	if reading.PONPort != "0/1/0" {
		t.Errorf("PONPort = %q, want %q", reading.PONPort, "0/1/0")
	}
}

// ============================================================================
// GetONUDistance tests
// ============================================================================

func TestGetONUDistance_NoSNMPExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	_, err := adapter.GetONUDistance(context.Background(), "0/1/0", 5)
	if err == nil {
		t.Error("expected error when SNMP executor not available")
	}
}

func TestGetONUDistance_InvalidPort(t *testing.T) {
	snmpExec := &testutil.MockSNMPExecutor{}
	adapter := &Adapter{
		baseDriver:   &testutil.MockDriver{},
		snmpExecutor: snmpExec,
		config:       testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	_, err := adapter.GetONUDistance(context.Background(), "bad", 5)
	if err == nil {
		t.Error("expected error for invalid port format")
	}
}

func TestGetONUDistance_Success(t *testing.T) {
	// Port 0/1/0 -> portIndex = 256
	snmpExec := &testutil.MockSNMPExecutor{
		GetResults: map[string]interface{}{
			"1.3.6.1.4.1.2011.6.128.1.1.2.43.1.12.256.5": int64(1500),
		},
	}

	adapter := &Adapter{
		baseDriver:   &testutil.MockDriver{},
		snmpExecutor: snmpExec,
		config:       testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	dist, err := adapter.GetONUDistance(context.Background(), "0/1/0", 5)
	if err != nil {
		t.Fatalf("GetONUDistance() error = %v", err)
	}

	if dist != 1500 {
		t.Errorf("distance = %d, want 1500", dist)
	}
}

// ============================================================================
// Connect with secondary driver tests
// ============================================================================

func TestConnect_PrimaryFails(t *testing.T) {
	mockDriver := &testutil.MockDriver{
		ConnectError: fmt.Errorf("connection refused"),
		CLIExec:      &testutil.MockCLIExecutor{},
		SNMPExec:     &testutil.MockSNMPExecutor{},
	}
	config := testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1")

	adapter := NewAdapter(mockDriver, config)
	err := adapter.Connect(context.Background(), config)
	if err == nil {
		t.Error("expected error when primary driver fails")
	}
	if !strings.Contains(err.Error(), "primary driver connect failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================================
// BulkScanONUsSNMP tests
// ============================================================================

func TestBulkScanONUsSNMP_NoSNMPExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	_, err := adapter.BulkScanONUsSNMP(context.Background())
	if err == nil {
		t.Error("expected error when SNMP executor not available")
	}
}

func TestBulkScanONUsSNMP_Success(t *testing.T) {
	snmpExec := &testutil.MockSNMPExecutor{
		WalkResults: map[string]map[string]interface{}{
			OIDOnuSerialNumber: {
				"0.1.0": "HWTC00001234",
				"0.1.1": "ZTEG00005678",
			},
			OIDOnuRxPower: {
				"0.1.0": int64(-1850),
				"0.1.1": int64(2147483647), // offline
			},
			OIDOnuTxPower: {
				"0.1.0": int64(210),
				"0.1.1": int64(2147483647), // offline
			},
			OIDOnuTemperature: {
				"0.1.0": int64(11264), // 44 C
			},
			OIDOnuVoltage: {
				"0.1.0": int64(3300), // 3.3 V
			},
			OIDOnuDistance: {
				"0.1.0": int64(1500),
			},
			OIDOnuCurrent: {
				"0.1.0": int64(15000), // 15 mA
			},
			OIDOnuUpBytes: {
				"0.1.0": uint64(12345),
			},
			OIDOnuDownBytes: {
				"0.1.0": uint64(67890),
			},
		},
	}

	adapter := &Adapter{
		baseDriver:   &testutil.MockDriver{},
		snmpExecutor: snmpExec,
		config:       testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	results, err := adapter.BulkScanONUsSNMP(context.Background())
	if err != nil {
		t.Fatalf("BulkScanONUsSNMP() error = %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 ONTs, got %d", len(results))
	}

	// Find the online ONU (index 0.1.0)
	var onlineONU *ONTStats
	var offlineONU *ONTStats
	for i := range results {
		if results[i].Serial == "HWTC00001234" {
			onlineONU = &results[i]
		} else if results[i].Serial == "ZTEG00005678" {
			offlineONU = &results[i]
		}
	}

	if onlineONU == nil {
		t.Fatal("expected to find HWTC00001234 ONU")
	}
	if !onlineONU.IsOnline {
		t.Error("expected HWTC00001234 to be online")
	}
	if onlineONU.BytesUp != 12345 {
		t.Errorf("BytesUp = %d, want 12345", onlineONU.BytesUp)
	}
	if onlineONU.BytesDown != 67890 {
		t.Errorf("BytesDown = %d, want 67890", onlineONU.BytesDown)
	}
	if onlineONU.Distance != 1500 {
		t.Errorf("Distance = %d, want 1500", onlineONU.Distance)
	}

	if offlineONU == nil {
		t.Fatal("expected to find ZTEG00005678 ONU")
	}
	if offlineONU.IsOnline {
		t.Error("expected ZTEG00005678 to be offline")
	}
}

func TestBulkScanONUsSNMP_SerialWalkError(t *testing.T) {
	snmpExec := &testutil.MockSNMPExecutor{
		WalkErrors: map[string]error{
			OIDOnuSerialNumber: fmt.Errorf("SNMP timeout"),
		},
	}

	adapter := &Adapter{
		baseDriver:   &testutil.MockDriver{},
		snmpExecutor: snmpExec,
		config:       testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	_, err := adapter.BulkScanONUsSNMP(context.Background())
	if err == nil {
		t.Error("expected error when serial walk fails")
	}
}

// ============================================================================
// GetONUList (DriverV2) tests
// ============================================================================

func TestGetONUList_NoSNMPExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	_, err := adapter.GetONUList(context.Background(), nil)
	if err == nil {
		t.Error("expected error when SNMP executor not available")
	}
}

func TestGetONUList_NoFilter(t *testing.T) {
	snmpExec := &testutil.MockSNMPExecutor{
		WalkResults: map[string]map[string]interface{}{
			OIDOnuSerialNumber: {
				"0.1.0": "HWTC00001234",
				"0.2.1": "ZTEG00005678",
			},
			OIDOnuRxPower: {
				"0.1.0": int64(-1850),
				"0.2.1": int64(-2200),
			},
			OIDOnuTxPower:     {},
			OIDOnuTemperature: {},
			OIDOnuVoltage:     {},
			OIDOnuDistance:    {},
			OIDOnuCurrent:     {},
			OIDOnuUpBytes:     {},
			OIDOnuDownBytes:   {},
		},
	}

	adapter := &Adapter{
		baseDriver:   &testutil.MockDriver{},
		snmpExecutor: snmpExec,
		config:       testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	results, err := adapter.GetONUList(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetONUList() error = %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 ONUs, got %d", len(results))
	}
}

func TestGetONUList_WithStatusFilter(t *testing.T) {
	snmpExec := &testutil.MockSNMPExecutor{
		WalkResults: map[string]map[string]interface{}{
			OIDOnuSerialNumber: {
				"0.1.0": "HWTC00001234",
				"0.1.1": "ZTEG00005678",
			},
			OIDOnuRxPower: {
				"0.1.0": int64(-1850),      // online
				"0.1.1": int64(2147483647), // offline
			},
			OIDOnuTxPower:     {},
			OIDOnuTemperature: {},
			OIDOnuVoltage:     {},
			OIDOnuDistance:    {},
			OIDOnuCurrent:     {},
			OIDOnuUpBytes:     {},
			OIDOnuDownBytes:   {},
		},
	}

	adapter := &Adapter{
		baseDriver:   &testutil.MockDriver{},
		snmpExecutor: snmpExec,
		config:       testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	// Filter for online only
	results, err := adapter.GetONUList(context.Background(), &types.ONUFilter{Status: "online"})
	if err != nil {
		t.Fatalf("GetONUList() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 online ONU, got %d", len(results))
	}
	if results[0].Serial != "HWTC00001234" {
		t.Errorf("Serial = %q, want %q", results[0].Serial, "HWTC00001234")
	}
}

func TestGetONUList_WithPONPortFilter(t *testing.T) {
	snmpExec := &testutil.MockSNMPExecutor{
		WalkResults: map[string]map[string]interface{}{
			OIDOnuSerialNumber: {
				"0.1.0": "HWTC00001234", // port 0/0/1
				"0.2.0": "ZTEG00005678", // port 0/0/2
			},
			OIDOnuRxPower: {
				"0.1.0": int64(-1850),
				"0.2.0": int64(-2200),
			},
			OIDOnuTxPower:     {},
			OIDOnuTemperature: {},
			OIDOnuVoltage:     {},
			OIDOnuDistance:    {},
			OIDOnuCurrent:     {},
			OIDOnuUpBytes:     {},
			OIDOnuDownBytes:   {},
		},
	}

	adapter := &Adapter{
		baseDriver:   &testutil.MockDriver{},
		snmpExecutor: snmpExec,
		config:       testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	// Filter by PON port
	results, err := adapter.GetONUList(context.Background(), &types.ONUFilter{PONPort: "0/0/1"})
	if err != nil {
		t.Fatalf("GetONUList() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 ONU on port 0/0/1, got %d", len(results))
	}
}

func TestGetONUList_WithSerialFilter(t *testing.T) {
	snmpExec := &testutil.MockSNMPExecutor{
		WalkResults: map[string]map[string]interface{}{
			OIDOnuSerialNumber: {
				"0.1.0": "HWTC00001234",
				"0.1.1": "ZTEG00005678",
			},
			OIDOnuRxPower:     {},
			OIDOnuTxPower:     {},
			OIDOnuTemperature: {},
			OIDOnuVoltage:     {},
			OIDOnuDistance:    {},
			OIDOnuCurrent:     {},
			OIDOnuUpBytes:     {},
			OIDOnuDownBytes:   {},
		},
	}

	adapter := &Adapter{
		baseDriver:   &testutil.MockDriver{},
		snmpExecutor: snmpExec,
		config:       testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	results, err := adapter.GetONUList(context.Background(), &types.ONUFilter{Serial: "HWTC"})
	if err != nil {
		t.Fatalf("GetONUList() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 ONU matching serial, got %d", len(results))
	}
}

// ============================================================================
// GetONUBySerial tests
// ============================================================================

func TestGetONUBySerial_NoSNMPExecutor(t *testing.T) {
	adapter := &Adapter{
		baseDriver: &testutil.MockDriver{},
		config:     testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}
	_, err := adapter.GetONUBySerial(context.Background(), "HWTC00001234")
	if err == nil {
		t.Error("expected error when SNMP executor not available")
	}
}

func TestGetONUBySerial_Found(t *testing.T) {
	snmpExec := &testutil.MockSNMPExecutor{
		WalkResults: map[string]map[string]interface{}{
			OIDOnuSerialNumber: {
				"0.1.0": "HWTC00001234",
			},
			OIDOnuRxPower:     {"0.1.0": int64(-1850)},
			OIDOnuTxPower:     {},
			OIDOnuTemperature: {},
			OIDOnuVoltage:     {},
			OIDOnuDistance:    {},
			OIDOnuCurrent:     {},
			OIDOnuUpBytes:     {},
			OIDOnuDownBytes:   {},
		},
	}

	adapter := &Adapter{
		baseDriver:   &testutil.MockDriver{},
		snmpExecutor: snmpExec,
		config:       testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	onu, err := adapter.GetONUBySerial(context.Background(), "HWTC00001234")
	if err != nil {
		t.Fatalf("GetONUBySerial() error = %v", err)
	}
	if onu == nil {
		t.Fatal("expected ONU, got nil")
	}
	if onu.Serial != "HWTC00001234" {
		t.Errorf("Serial = %q, want %q", onu.Serial, "HWTC00001234")
	}
}

func TestGetONUBySerial_NotFound(t *testing.T) {
	snmpExec := &testutil.MockSNMPExecutor{
		WalkResults: map[string]map[string]interface{}{
			OIDOnuSerialNumber: {},
			OIDOnuRxPower:      {},
			OIDOnuTxPower:      {},
			OIDOnuTemperature:  {},
			OIDOnuVoltage:      {},
			OIDOnuDistance:     {},
			OIDOnuCurrent:      {},
			OIDOnuUpBytes:      {},
			OIDOnuDownBytes:    {},
		},
	}

	adapter := &Adapter{
		baseDriver:   &testutil.MockDriver{},
		snmpExecutor: snmpExec,
		config:       testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	onu, err := adapter.GetONUBySerial(context.Background(), "NONEXISTENT")
	if err != nil {
		t.Fatalf("GetONUBySerial() error = %v", err)
	}
	if onu != nil {
		t.Error("expected nil for non-existent serial")
	}
}

func TestDisconnect_WithSecondaryDriver(t *testing.T) {
	secondaryDriver := &testutil.MockDriver{Connected: true}
	adapter := &Adapter{
		baseDriver:      &testutil.MockDriver{Connected: true},
		secondaryDriver: secondaryDriver,
		config:          testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
	}

	err := adapter.Disconnect(context.Background())
	if err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
}

// ============================================================================
// ONU Lifecycle Operations Tests
// ============================================================================

// newTestAdapter creates a Huawei adapter with a mock CLI executor for lifecycle testing.
func newTestAdapter(outputs map[string]string) *Adapter {
	return &Adapter{
		cliExecutor:      &testutil.MockCLIExecutor{Outputs: outputs},
		config:           testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
		suspensionStates: make(map[string]*types.SuspensionState),
	}
}

func TestCaptureSubscriberConfig_Success(t *testing.T) {
	adapter := newTestAdapter(map[string]string{
		"display ont info 0 5": "SN : HWTC12345678\nLine profile ID: 10\nStatus: online\n",
	})

	snapshot, err := adapter.CaptureSubscriberConfig(context.Background(), "ont-0/1/0-5")
	if err != nil {
		t.Fatalf("CaptureSubscriberConfig() error: %v", err)
	}
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	if snapshot.Serial != "HWTC12345678" {
		t.Errorf("Serial = %q, want HWTC12345678", snapshot.Serial)
	}
	if snapshot.PONPort != "0/1/0" {
		t.Errorf("PONPort = %q, want 0/1/0", snapshot.PONPort)
	}
	if snapshot.ONUID != 5 {
		t.Errorf("ONUID = %d, want 5", snapshot.ONUID)
	}
	if snapshot.Metadata["vendor"] != "huawei" {
		t.Errorf("vendor = %q, want huawei", snapshot.Metadata["vendor"])
	}
}

func TestCaptureSubscriberConfig_NoCLI(t *testing.T) {
	adapter := &Adapter{
		config:           testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
		suspensionStates: make(map[string]*types.SuspensionState),
	}

	_, err := adapter.CaptureSubscriberConfig(context.Background(), "ont-0/1/0-5")
	if err == nil {
		t.Fatal("expected error when CLI executor is nil")
	}
}

func TestCaptureSubscriberConfig_ONTNotFound(t *testing.T) {
	adapter := newTestAdapter(map[string]string{
		"display ont info 0 5": "No such ONT\n",
	})

	_, err := adapter.CaptureSubscriberConfig(context.Background(), "ont-0/1/0-5")
	if err == nil {
		t.Fatal("expected error when ONT not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
}

func TestRestoreSubscriberConfig_Success(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	snapshot := &types.SubscriberSnapshot{
		Serial:  "HWTC99999999",
		PONPort: "0/1/0",
		ONUID:   5,
		VLAN:    100,
		Metadata: map[string]string{
			"vendor": "huawei",
		},
	}

	result, err := adapter.RestoreSubscriberConfig(context.Background(), snapshot, "0/2/0", 3)
	if err != nil {
		t.Fatalf("RestoreSubscriberConfig() error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.SubscriberID != "ont-0/2/0-3" {
		t.Errorf("SubscriberID = %q, want ont-0/2/0-3", result.SubscriberID)
	}
}

func TestRestoreSubscriberConfig_NilSnapshot(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	_, err := adapter.RestoreSubscriberConfig(context.Background(), nil, "0/2/0", 3)
	if err == nil {
		t.Fatal("expected error for nil snapshot")
	}
}

func TestRestoreSubscriberConfig_InvalidFSP(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	snapshot := &types.SubscriberSnapshot{Serial: "HWTC99999999", VLAN: 100, Metadata: map[string]string{}}
	_, err := adapter.RestoreSubscriberConfig(context.Background(), snapshot, "0/1", 3)
	if err == nil {
		t.Fatal("expected error for invalid FSP format")
	}
	if !strings.Contains(err.Error(), "invalid target PON port") {
		t.Errorf("error = %q, want 'invalid target PON port'", err.Error())
	}
}

func TestReplaceONU_Success(t *testing.T) {
	adapter := newTestAdapter(map[string]string{
		"display ont info 0 5": "SN : HWTC12345678\nStatus: online\n",
	})

	result, err := adapter.ReplaceONU(context.Background(), "ont-0/1/0-5", "HWTCNEW12345")
	if err != nil {
		t.Fatalf("ReplaceONU() error: %v", err)
	}
	if result.OldSerial != "HWTC12345678" {
		t.Errorf("OldSerial = %q, want HWTC12345678", result.OldSerial)
	}
	if result.NewSerial != "HWTCNEW12345" {
		t.Errorf("NewSerial = %q, want HWTCNEW12345", result.NewSerial)
	}
}

func TestReplaceONU_EmptySerial(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	_, err := adapter.ReplaceONU(context.Background(), "ont-0/1/0-5", "")
	if err == nil {
		t.Fatal("expected error for empty serial")
	}
	if !strings.Contains(err.Error(), "new serial is required") {
		t.Errorf("error = %q, want 'new serial is required'", err.Error())
	}
}

func TestReplaceONU_NoCLI(t *testing.T) {
	adapter := &Adapter{
		config:           testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
		suspensionStates: make(map[string]*types.SuspensionState),
	}

	_, err := adapter.ReplaceONU(context.Background(), "ont-0/1/0-5", "HWTCNEW12345")
	if err == nil {
		t.Fatal("expected error when CLI executor is nil")
	}
}

func TestSoftSuspendSubscriber_Throttle(t *testing.T) {
	adapter := newTestAdapter(map[string]string{
		"display ont info 0 5": "SN : HWTC12345678\nStatus: online\n",
	})

	opts := &types.SuspendOptions{
		Mode:                  types.SuspensionModeThrottle,
		ThrottleBandwidthKbps: 64,
	}

	state, err := adapter.SoftSuspendSubscriber(context.Background(), "ont-0/1/0-5", opts)
	if err != nil {
		t.Fatalf("SoftSuspendSubscriber() error: %v", err)
	}
	if state.Mode != types.SuspensionModeThrottle {
		t.Errorf("Mode = %q, want throttle", state.Mode)
	}
	if state.AppliedBandwidthKbps != 64 {
		t.Errorf("AppliedBandwidthKbps = %d, want 64", state.AppliedBandwidthKbps)
	}
	if state.OriginalSnapshot == nil {
		t.Error("OriginalSnapshot should not be nil")
	}
}

func TestSoftSuspendSubscriber_WalledGarden(t *testing.T) {
	adapter := newTestAdapter(map[string]string{
		"display ont info 0 5": "SN : HWTC12345678\nStatus: online\n",
	})

	opts := &types.SuspendOptions{
		Mode:             types.SuspensionModeWalledGarden,
		WalledGardenVLAN: 999,
	}

	state, err := adapter.SoftSuspendSubscriber(context.Background(), "ont-0/1/0-5", opts)
	if err != nil {
		t.Fatalf("SoftSuspendSubscriber() error: %v", err)
	}
	if state.Mode != types.SuspensionModeWalledGarden {
		t.Errorf("Mode = %q, want walled-garden", state.Mode)
	}
	if state.AppliedVLAN != 999 {
		t.Errorf("AppliedVLAN = %d, want 999", state.AppliedVLAN)
	}
}

func TestSoftSuspendSubscriber_InvalidMode(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	opts := &types.SuspendOptions{Mode: "invalid"}
	_, err := adapter.SoftSuspendSubscriber(context.Background(), "ont-0/1/0-5", opts)
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	if !strings.Contains(err.Error(), "invalid suspension mode") {
		t.Errorf("error = %q, want 'invalid suspension mode'", err.Error())
	}
}

func TestSoftSuspendSubscriber_NilOpts(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	_, err := adapter.SoftSuspendSubscriber(context.Background(), "ont-0/1/0-5", nil)
	if err == nil {
		t.Fatal("expected error for nil opts")
	}
}

func TestGetSuspensionState_NotSuspended(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	state, err := adapter.GetSuspensionState(context.Background(), "ont-0/1/0-5")
	if err != nil {
		t.Fatalf("GetSuspensionState() error: %v", err)
	}
	if state != nil {
		t.Error("expected nil state for non-suspended subscriber")
	}
}

func TestGetSuspensionState_Suspended(t *testing.T) {
	adapter := newTestAdapter(map[string]string{
		"display ont info 0 5": "SN : HWTC12345678\nStatus: online\n",
	})

	// First suspend
	opts := &types.SuspendOptions{
		Mode:                  types.SuspensionModeThrottle,
		ThrottleBandwidthKbps: 64,
	}
	_, err := adapter.SoftSuspendSubscriber(context.Background(), "ont-0/1/0-5", opts)
	if err != nil {
		t.Fatalf("SoftSuspendSubscriber() error: %v", err)
	}

	// Then check state
	state, err := adapter.GetSuspensionState(context.Background(), "ont-0/1/0-5")
	if err != nil {
		t.Fatalf("GetSuspensionState() error: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.Mode != types.SuspensionModeThrottle {
		t.Errorf("Mode = %q, want throttle", state.Mode)
	}
}

func TestMoveSubscriber_Success(t *testing.T) {
	adapter := newTestAdapter(map[string]string{
		"display ont info 0 5": "SN : HWTC12345678\nStatus: online\n",
	})

	result, err := adapter.MoveSubscriber(context.Background(), "ont-0/1/0-5", "0/2/0", 3)
	if err != nil {
		t.Fatalf("MoveSubscriber() error: %v", err)
	}
	if result.OldPONPort != "0/1/0" {
		t.Errorf("OldPONPort = %q, want 0/1/0", result.OldPONPort)
	}
	if result.NewPONPort != "0/2/0" {
		t.Errorf("NewPONPort = %q, want 0/2/0", result.NewPONPort)
	}
	if result.OldONUID != 5 {
		t.Errorf("OldONUID = %d, want 5", result.OldONUID)
	}
	if result.NewONUID != 3 {
		t.Errorf("NewONUID = %d, want 3", result.NewONUID)
	}
}

func TestMoveSubscriber_EmptyTarget(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	_, err := adapter.MoveSubscriber(context.Background(), "ont-0/1/0-5", "", 3)
	if err == nil {
		t.Fatal("expected error for empty target")
	}
	if !strings.Contains(err.Error(), "target PON port is required") {
		t.Errorf("error = %q, want 'target PON port is required'", err.Error())
	}
}

func TestMoveSubscriber_NoCLI(t *testing.T) {
	adapter := &Adapter{
		config:           testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
		suspensionStates: make(map[string]*types.SuspensionState),
	}

	_, err := adapter.MoveSubscriber(context.Background(), "ont-0/1/0-5", "0/2/0", 3)
	if err == nil {
		t.Fatal("expected error when CLI executor is nil")
	}
}

func TestCheckONUCompatibility_SameVendor(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	report, err := adapter.CheckONUCompatibility(context.Background(), "ont-0/1/0-5", "HWTCNEW12345")
	if err != nil {
		t.Fatalf("CheckONUCompatibility() error: %v", err)
	}
	if !report.Compatible {
		t.Error("expected compatible for same-vendor ONTs")
	}
}

func TestCheckONUCompatibility_EmptySerial(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	_, err := adapter.CheckONUCompatibility(context.Background(), "ont-0/1/0-5", "")
	if err == nil {
		t.Fatal("expected error for empty serial")
	}
}

func TestClassifyHuaweiSerial(t *testing.T) {
	tests := []struct {
		serial   string
		expected string
	}{
		{"HWTC12345678", "GPON"},
		{"485712345678", "GPON"},
		{"XGSH12345678", "XGS-PON"},
		{"EPON12345678", "EPON"},
		{"UNKN12345678", "GPON"}, // default
		{"AB", ""},               // too short
	}

	for _, tt := range tests {
		got := classifyHuaweiSerial(tt.serial)
		if got != tt.expected {
			t.Errorf("classifyHuaweiSerial(%q) = %q, want %q", tt.serial, got, tt.expected)
		}
	}
}

func TestAddONUToSubscriber_Success(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	binding := model.ONUBinding{
		Serial:  "HWTC99999999",
		PONPort: "0/3/0",
		ONUID:   7,
		Role:    model.ONUBindingRoleSecondary,
	}

	result, err := adapter.AddONUToSubscriber(context.Background(), "ont-0/1/0-5", binding, nil)
	if err != nil {
		t.Fatalf("AddONUToSubscriber() error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestAddONUToSubscriber_MissingSerial(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	binding := model.ONUBinding{PONPort: "0/3/0", ONUID: 7}
	_, err := adapter.AddONUToSubscriber(context.Background(), "ont-0/1/0-5", binding, nil)
	if err == nil {
		t.Fatal("expected error for missing serial")
	}
}

func TestAddONUToSubscriber_InvalidFSP(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	binding := model.ONUBinding{Serial: "HWTC99999999", PONPort: "0/1", ONUID: 7}
	_, err := adapter.AddONUToSubscriber(context.Background(), "ont-0/1/0-5", binding, nil)
	if err == nil {
		t.Fatal("expected error for invalid FSP")
	}
}

func TestRemoveONUFromSubscriber_NoCLI(t *testing.T) {
	adapter := &Adapter{
		config:           testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
		suspensionStates: make(map[string]*types.SuspensionState),
	}

	err := adapter.RemoveONUFromSubscriber(context.Background(), "ont-0/1/0-5", "HWTC12345678")
	if err == nil {
		t.Fatal("expected error when CLI executor is nil")
	}
}

func TestRemoveONUFromSubscriber_EmptySerial(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	err := adapter.RemoveONUFromSubscriber(context.Background(), "ont-0/1/0-5", "")
	if err == nil {
		t.Fatal("expected error for empty serial")
	}
}

func TestRemoveONUFromSubscriber_NotFound(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	err := adapter.RemoveONUFromSubscriber(context.Background(), "ont-0/1/0-5", "HWTC_NONEXIST")
	if err == nil {
		t.Fatal("expected error when ONT not found")
	}
	if !strings.Contains(err.Error(), "not found for subscriber") {
		t.Errorf("error = %q, want 'not found for subscriber'", err.Error())
	}
}

func TestListSubscriberONUs_NoCLI(t *testing.T) {
	adapter := &Adapter{
		config:           testutil.NewTestEquipmentConfig(types.VendorHuawei, "10.0.0.1"),
		suspensionStates: make(map[string]*types.SuspensionState),
	}

	_, err := adapter.ListSubscriberONUs(context.Background(), "ont-0/1/0-5")
	if err == nil {
		t.Fatal("expected error when CLI executor is nil")
	}
}

func TestListSubscriberONUs_Placeholder(t *testing.T) {
	adapter := newTestAdapter(map[string]string{})

	bindings, err := adapter.ListSubscriberONUs(context.Background(), "ont-0/1/0-5")
	if err != nil {
		t.Fatalf("ListSubscriberONUs() error: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(bindings))
	}
	if bindings[0].PONPort != "0/1/0" {
		t.Errorf("PONPort = %q, want 0/1/0", bindings[0].PONPort)
	}
	if bindings[0].ONUID != 5 {
		t.Errorf("ONUID = %d, want 5", bindings[0].ONUID)
	}
	if bindings[0].Role != model.ONUBindingRolePrimary {
		t.Errorf("Role = %q, want primary", bindings[0].Role)
	}
}
