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
			Metadata: map[string]string{
				"skip_omci_profile_check": "true",
				"wifi_command_profile":    "legacy",
			},
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
	if len(result.Events) != 8 {
		t.Fatalf("expected 8 events, got %d", len(result.Events))
	}
	if result.Events[0].Step != "PROFILE_OMCI_PRECHECK" || !result.Events[0].OK {
		t.Fatalf("expected successful PROFILE_OMCI_PRECHECK event, got %+v", result.Events[0])
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
	if result.FailedStep != "PROFILE_OMCI_PRECHECK" {
		t.Fatalf("expected failed step PROFILE_OMCI_PRECHECK, got %s", result.FailedStep)
	}
}

func TestSetWifiConfig_ProfileReadyFromProfileTable(t *testing.T) {
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal":                     "ok",
			"interface gpon 0/1":                     "ok",
			"show running-config onu 7":              "onu 7 profile onu default\nonu 7 service INTERNET gemport 1 vlan 100",
			"show profile onu":                       "Name: default\nWifi mgmt via non OMCI: disable\n",
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
			Metadata: map[string]string{
				"wifi_command_profile": "legacy",
			},
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
			Metadata: map[string]string{
				"skip_omci_profile_check": "true",
				"wifi_command_profile":    "legacy",
			},
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

func TestSetWifiConfig_PartialApplyOnPasswordStep(t *testing.T) {
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal":                     "ok",
			"interface gpon 0/1":                     "ok",
			"onu 7 wifi ssid \"Nanoncore\"":          "ok",
			"onu 7 wifi password \"SuperSecret123\"": "% Unknown command.",
		},
		errByCommand: map[string]error{},
	}

	adapter := &Adapter{
		cliExecutor: mock,
		config: &types.EquipmentConfig{
			Metadata: map[string]string{
				"skip_omci_profile_check": "true",
				"wifi_command_profile":    "legacy",
			},
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
	if result.FailedStep != "SET_PASSWORD" {
		t.Fatalf("expected failed step SET_PASSWORD, got %s", result.FailedStep)
	}
}

func TestSetWifiConfig_PartialApplyOnEnableStep(t *testing.T) {
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal":                     "ok",
			"interface gpon 0/1":                     "ok",
			"onu 7 wifi ssid \"Nanoncore\"":          "ok",
			"onu 7 wifi password \"SuperSecret123\"": "ok",
			"onu 7 wifi enable":                      "% Error: command failed",
		},
		errByCommand: map[string]error{},
	}

	adapter := &Adapter{
		cliExecutor: mock,
		config: &types.EquipmentConfig{
			Metadata: map[string]string{
				"skip_omci_profile_check": "true",
				"wifi_command_profile":    "legacy",
			},
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
	if result.FailedStep != "SET_ENABLED" {
		t.Fatalf("expected failed step SET_ENABLED, got %s", result.FailedStep)
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
			Metadata: map[string]string{
				"skip_omci_profile_check": "true",
				"wifi_command_profile":    "legacy",
			},
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
	if result.ErrorCode != types.WifiErrorCodeCommandTimeout {
		t.Fatalf("expected COMMAND_TIMEOUT, got %s", result.ErrorCode)
	}
	if result.FailedStep != "ENTER_PON_INTERFACE" {
		t.Fatalf("expected failed step ENTER_PON_INTERFACE, got %s", result.FailedStep)
	}
}

func TestSetWifiConfig_CommandTimeoutOnSSIDStep(t *testing.T) {
	timeoutErr := fmt.Errorf("timeout waiting for prompt")
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal": "ok",
			"interface gpon 0/1": "ok",
		},
		errByCommand: map[string]error{
			"onu 7 wifi ssid \"Nanoncore\"": timeoutErr,
		},
	}

	adapter := &Adapter{
		cliExecutor: mock,
		config: &types.EquipmentConfig{
			Metadata: map[string]string{
				"skip_omci_profile_check": "true",
				"wifi_command_profile":    "legacy",
			},
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
	if result.ErrorCode != types.WifiErrorCodeCommandTimeout {
		t.Fatalf("expected COMMAND_TIMEOUT, got %s", result.ErrorCode)
	}
	if result.FailedStep != "SET_SSID" {
		t.Fatalf("expected failed step SET_SSID, got %s", result.FailedStep)
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

func TestSetWifiConfig_PriProfile_Success(t *testing.T) {
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal": "ok",
			"interface gpon 0/2": "ok",
			"onu 9 pri wifi_ssid 1 name \"LabSSID\" hide disable auth_mode wpa2psk encrypt_type aes shared_key \"StrongPass1\" rekey_interval 3600": "ok",
			"onu 9 pri wifi_switch 1 enable global auto 80211acanac 10":                                                                             "ok",
			"exit": "ok",
			"end":  "ok",
		},
		errByCommand: map[string]error{},
	}

	adapter := &Adapter{
		cliExecutor: mock,
		config: &types.EquipmentConfig{
			Metadata: map[string]string{
				"skip_omci_profile_check": "true",
				"wifi_command_profile":    "pri",
			},
		},
	}

	result, err := adapter.SetWifiConfig(context.Background(), types.WifiTarget{PONPort: "0/2", ONUID: 9}, types.WifiConfig{
		SSID:     "LabSSID",
		Password: "StrongPass1",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("SetWifiConfig returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected success, got errorCode=%s reason=%s", result.ErrorCode, result.Reason)
	}
	if strings.Contains(result.RawOutput, "StrongPass1") {
		t.Fatalf("raw output must redact shared_key")
	}
	for _, event := range result.Events {
		if strings.Contains(event.Detail, "StrongPass1") {
			t.Fatalf("event detail must redact shared_key")
		}
	}
}

func TestSetWifiConfig_AutoProfileByModelHint_UsesPri(t *testing.T) {
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal": "ok",
			"interface gpon 0/2": "ok",
			"onu 9 pri wifi_ssid 1 name \"LabSSID\" hide disable auth_mode open encrypt_type none": "ok",
			"onu 9 pri wifi_switch 1 disable": "ok",
			"exit":                            "ok",
			"end":                             "ok",
		},
		errByCommand: map[string]error{},
	}

	adapter := &Adapter{
		cliExecutor: mock,
		config: &types.EquipmentConfig{
			Metadata: map[string]string{
				"skip_omci_profile_check": "true",
				"model":                   "V1600G1",
			},
		},
	}

	result, err := adapter.SetWifiConfig(context.Background(), types.WifiTarget{PONPort: "0/2", ONUID: 9}, types.WifiConfig{
		SSID:    "LabSSID",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("SetWifiConfig returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected success, got errorCode=%s reason=%s", result.ErrorCode, result.Reason)
	}

	if !containsString(mock.commands, "onu 9 pri wifi_ssid 1 name \"LabSSID\" hide disable auth_mode open encrypt_type none") {
		t.Fatalf("expected PRI SSID command, got sequence: %+v", mock.commands)
	}
	if !containsString(mock.commands, "onu 9 pri wifi_switch 1 disable") {
		t.Fatalf("expected PRI switch command, got sequence: %+v", mock.commands)
	}
}

func TestSetWifiConfig_UnresolvedProfileFailsClosed(t *testing.T) {
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal": "ok",
			"interface gpon 0/1": "ok",
			"onu 9 pri ?":        "",
			"onu 9 wifi ?":       "",
			"exit":               "ok",
			"end":                "ok",
		},
		errByCommand: map[string]error{},
	}

	adapter := &Adapter{
		cliExecutor: mock,
		config: &types.EquipmentConfig{
			Metadata: map[string]string{
				"skip_omci_profile_check": "true",
			},
		},
	}

	result, err := adapter.SetWifiConfig(context.Background(), types.WifiTarget{PONPort: "0/1", ONUID: 9}, types.WifiConfig{
		SSID:     "LabSSID",
		Password: "StrongPass1",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("SetWifiConfig returned error: %v", err)
	}
	if result.OK {
		t.Fatalf("expected failure when profile cannot be resolved")
	}
	if result.ErrorCode != types.WifiErrorCodeUnsupportedOperation {
		t.Fatalf("expected UNSUPPORTED_OPERATION, got %s", result.ErrorCode)
	}
}

func TestSetWifiConfig_PriProfile_UsesMetadataDefaults(t *testing.T) {
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal": "ok",
			"interface gpon 0/2": "ok",
			"onu 9 pri wifi_ssid 1 name \"LabSSID\" hide disable auth_mode open encrypt_type none": "ok",
			"onu 9 pri wifi_switch 1 enable fcc chl_36 80211acanac 7 20/40":                        "ok",
			"exit": "ok",
			"end":  "ok",
		},
		errByCommand: map[string]error{},
	}

	adapter := &Adapter{
		cliExecutor: mock,
		config: &types.EquipmentConfig{
			Metadata: map[string]string{
				"skip_omci_profile_check": "true",
				"wifi_command_profile":    "pri",
				"wifi_pri_country":        "fcc",
				"wifi_pri_channel":        "chl_36",
				"wifi_pri_standard":       "80211acanac",
				"wifi_pri_power":          "7",
				"wifi_pri_width":          "20/40",
			},
		},
	}

	result, err := adapter.SetWifiConfig(context.Background(), types.WifiTarget{PONPort: "0/2", ONUID: 9}, types.WifiConfig{
		SSID:    "LabSSID",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("SetWifiConfig returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected success, got errorCode=%s reason=%s", result.ErrorCode, result.Reason)
	}
	if !containsString(mock.commands, "onu 9 pri wifi_switch 1 enable fcc chl_36 80211acanac 7 20/40") {
		t.Fatalf("expected metadata-driven PRI switch command, got sequence: %+v", mock.commands)
	}
}

func TestResolveWifiTarget_SerialOnline(t *testing.T) {
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal": "ok",
			"interface gpon 0/1": "ok",
			"show onu info all":  "GPON0/1:3 HG6143D default sn FHTT59CB8310",
			"show onu state":     "1/1/1:3 enable enable working 1(GPON)",
			"exit":               "ok",
		},
		errByCommand: map[string]error{},
	}

	adapter := &Adapter{
		cliExecutor: mock,
		config:      &types.EquipmentConfig{},
	}

	ponPort, onuID, result := adapter.resolveWifiTarget(context.Background(), types.WifiTarget{OnuSerial: "FHTT59CB8310"})
	if result != nil {
		t.Fatalf("expected successful resolution, got %+v", result)
	}
	if ponPort != "0/1" || onuID != 3 {
		t.Fatalf("expected 0/1:3, got %s:%d", ponPort, onuID)
	}
}

func TestResolveWifiTarget_SerialOffline(t *testing.T) {
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal": "ok",
			"interface gpon 0/1": "ok",
			"show onu info all":  "GPON0/1:2 HG6143D default sn HWTCA23A8DF2",
			"show onu state":     "1/1/1:2 enable enable OffLine 1(GPON)",
			"exit":               "ok",
		},
		errByCommand: map[string]error{},
	}

	adapter := &Adapter{
		cliExecutor: mock,
		config:      &types.EquipmentConfig{},
	}

	_, _, result := adapter.resolveWifiTarget(context.Background(), types.WifiTarget{OnuSerial: "HWTCA23A8DF2"})
	if result == nil {
		t.Fatalf("expected offline resolution error")
	}
	if result.ErrorCode != types.WifiErrorCodeOnuOffline {
		t.Fatalf("expected ONU_OFFLINE, got %s", result.ErrorCode)
	}
}

func TestResolveWifiTarget_SerialNotFound(t *testing.T) {
	mock := &wifiMockCLI{
		outputByCommand: map[string]string{
			"configure terminal": "ok",
			"exit":               "ok",
		},
		errByCommand: map[string]error{},
	}

	adapter := &Adapter{
		cliExecutor: mock,
		config:      &types.EquipmentConfig{},
	}

	_, _, result := adapter.resolveWifiTarget(context.Background(), types.WifiTarget{OnuSerial: "FAKESERIAL9999"})
	if result == nil {
		t.Fatalf("expected not found resolution error")
	}
	if result.ErrorCode != types.WifiErrorCodeOnuNotFound {
		t.Fatalf("expected ONU_NOT_FOUND, got %s", result.ErrorCode)
	}
}

func TestParseProfileOMCIReadiness(t *testing.T) {
	output := `
Name: default
Wifi mgmt via non OMCI: disable

Name: AN5506-04-F1
Wifi mgmt via non OMCI: enable
`
	if !parseProfileOMCIReadiness(output, "default") {
		t.Fatalf("expected default to be OMCI-ready")
	}
	if parseProfileOMCIReadiness(output, "AN5506-04-F1") {
		t.Fatalf("expected AN5506-04-F1 to be non-OMCI-ready")
	}
	if parseProfileOMCIReadiness(output, "missing") {
		t.Fatalf("expected missing profile to be non-OMCI-ready")
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
