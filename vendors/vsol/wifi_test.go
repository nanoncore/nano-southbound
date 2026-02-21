package vsol

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/nanoncore/nano-southbound/types"
)

type wifiMockCLI struct {
	outputByCommand map[string]string
	errByCommand    map[string]error
	commands        []string
}

func (m *wifiMockCLI) ExecCommand(_ context.Context, command string) (string, error) {
	m.commands = append(m.commands, command)
	if err := m.errByCommand[command]; err != nil {
		return m.outputByCommand[command], err
	}
	return m.outputByCommand[command], nil
}

func (m *wifiMockCLI) ExecCommands(ctx context.Context, commands []string) ([]string, error) {
	out := make([]string, 0, len(commands))
	for _, cmd := range commands {
		res, err := m.ExecCommand(ctx, cmd)
		if err != nil {
			return out, err
		}
		out = append(out, res)
	}
	return out, nil
}

func TestSetWifiConfig_Success(t *testing.T) {
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal":                     "ok",
			"interface gpon 0/1":                     "ok",
			"onu 7 wifi ssid \"Nanoncore\"":          "ok",
			"onu 7 wifi password \"SuperSecret123\"": "ok",
			"onu 7 wifi enable":                      "ok",
			"exit":                                   "ok",
			"end":                                    "ok",
		},
		errByCommand: map[string]error{},
	}

	adapter := &Adapter{
		cliExecutor: mock,
		config: &types.EquipmentConfig{
			Metadata: map[string]string{"skip_omci_profile_check": "true"},
		},
	}

	result, err := adapter.SetWifiConfig(context.Background(), types.WifiTarget{PONPort: "0/1", ONUID: 7}, types.WifiConfig{
		SSID:     "Nanoncore",
		Password: "SuperSecret123",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("SetWifiConfig returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected success, got errorCode=%s reason=%s", result.ErrorCode, result.Reason)
	}
	if result.FailedStep != "" {
		t.Fatalf("expected no failed step, got %s", result.FailedStep)
	}
	if len(result.Events) != 7 {
		t.Fatalf("expected 7 events, got %d", len(result.Events))
	}
	if strings.Contains(result.RawOutput, "SuperSecret123") {
		t.Fatalf("raw output must redact password")
	}

	expected := []string{
		"configure terminal",
		"interface gpon 0/1",
		"onu 7 wifi ssid \"Nanoncore\"",
		"onu 7 wifi password \"SuperSecret123\"",
		"onu 7 wifi enable",
		"exit",
		"end",
	}
	if !equalStringSlices(mock.commands, expected) {
		t.Fatalf("unexpected command sequence: %+v", mock.commands)
	}
}

func TestSetWifiConfig_ProfileNotReady(t *testing.T) {
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal":        "ok",
			"interface gpon 0/1":        "ok",
			"show running-config onu 7": "onu 7 service INTERNET gemport 1 vlan 100",
			"exit":                      "ok",
			"end":                       "ok",
			"terminal length 0":         "ok",
			"show running-config":       "no wifi-mng-via-non-omci disable line",
		},
		errByCommand: map[string]error{},
	}

	adapter := &Adapter{
		cliExecutor: mock,
		config:      &types.EquipmentConfig{Metadata: map[string]string{}},
	}

	result, err := adapter.SetWifiConfig(context.Background(), types.WifiTarget{PONPort: "0/1", ONUID: 7}, types.WifiConfig{
		SSID:     "Nanoncore",
		Password: "SuperSecret123",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("SetWifiConfig returned error: %v", err)
	}
	if result.OK {
		t.Fatalf("expected failure")
	}
	if result.ErrorCode != types.WifiErrorCodeProfileNotOMCIReady {
		t.Fatalf("expected PROFILE_NOT_OMCI_READY, got %s", result.ErrorCode)
	}
}

func TestSetWifiConfig_PartialApply(t *testing.T) {
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal":            "ok",
			"interface gpon 0/1":            "ok",
			"onu 7 wifi ssid \"Nanoncore\"": "% Unknown command.",
		},
		errByCommand: map[string]error{},
	}

	adapter := &Adapter{
		cliExecutor: mock,
		config: &types.EquipmentConfig{
			Metadata: map[string]string{"skip_omci_profile_check": "true"},
		},
	}

	result, err := adapter.SetWifiConfig(context.Background(), types.WifiTarget{PONPort: "0/1", ONUID: 7}, types.WifiConfig{
		SSID:     "Nanoncore",
		Password: "SuperSecret123",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("SetWifiConfig returned error: %v", err)
	}
	if result.OK {
		t.Fatalf("expected failure")
	}
	if result.ErrorCode != types.WifiErrorCodePartialApply {
		t.Fatalf("expected PARTIAL_APPLY, got %s", result.ErrorCode)
	}
	if result.FailedStep != "SET_SSID" {
		t.Fatalf("expected failed step SET_SSID, got %s", result.FailedStep)
	}
}

func TestSetWifiConfig_CommandTimeout(t *testing.T) {
	timeoutErr := fmt.Errorf("timeout waiting for prompt")
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal": "ok",
		},
		errByCommand: map[string]error{
			"interface gpon 0/1": timeoutErr,
		},
	}

	adapter := &Adapter{
		cliExecutor: mock,
		config: &types.EquipmentConfig{
			Metadata: map[string]string{"skip_omci_profile_check": "true"},
		},
	}

	result, err := adapter.SetWifiConfig(context.Background(), types.WifiTarget{PONPort: "0/1", ONUID: 7}, types.WifiConfig{
		SSID:     "Nanoncore",
		Password: "SuperSecret123",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("SetWifiConfig returned error: %v", err)
	}
	if result.OK {
		t.Fatalf("expected failure")
	}
	if result.ErrorCode != types.WifiErrorCodePartialApply {
		t.Fatalf("expected PARTIAL_APPLY (post-first-step timeout), got %s", result.ErrorCode)
	}
}

func TestGetWifiConfig_ReadbackUnavailable(t *testing.T) {
	adapter := &Adapter{
		config: &types.EquipmentConfig{
			Metadata: map[string]string{},
		},
	}

	result, err := adapter.GetWifiConfig(context.Background(), types.WifiTarget{PONPort: "0/1", ONUID: 7})
	if err != nil {
		t.Fatalf("GetWifiConfig returned error: %v", err)
	}
	if result.OK {
		t.Fatalf("expected readback unavailable result")
	}
	if result.ErrorCode != types.WifiErrorCodeReadbackUnavailable {
		t.Fatalf("expected READBACK_UNAVAILABLE, got %s", result.ErrorCode)
	}
}
