package vsol

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nanoncore/nano-southbound/types"
)

var _ types.WifiManager = (*Adapter)(nil)

type wifiStep struct {
	name      string
	command   string
	sensitive bool
}

type wifiCommandProfile string

const (
	wifiCommandProfileLegacy wifiCommandProfile = "legacy"
	wifiCommandProfilePri    wifiCommandProfile = "pri"
)

func (a *Adapter) GetWifiConfig(ctx context.Context, target types.WifiTarget) (*types.WifiActionResult, error) {
	ponPort, onuID, result := a.resolveWifiTarget(ctx, target)
	if result != nil {
		return result, nil
	}

	// MVP: only return observed config when readback is explicitly enabled.
	if !a.omciWifiReadbackEnabled() {
		return &types.WifiActionResult{
			OK:        false,
			ErrorCode: types.WifiErrorCodeReadbackUnavailable,
			Reason:    "OMCI readback unavailable for this ONU model or environment",
		}, nil
	}

	raw, err := a.GetONURunningConfig(ctx, ponPort, onuID)
	if err != nil {
		return &types.WifiActionResult{
			OK:        false,
			ErrorCode: classifyWifiErrCode(err, ""),
			Reason:    err.Error(),
		}, nil
	}

	// Parser TODO: wire model-specific readback parsing once golden outputs are captured.
	// Keep semantics truthful: no observedConfig unless parser is proven.
	return &types.WifiActionResult{
		OK:        false,
		ErrorCode: types.WifiErrorCodeReadbackUnavailable,
		Reason:    "running config captured but Wi-Fi readback parser not yet enabled",
		RawOutput: raw,
	}, nil
}

func (a *Adapter) SetWifiConfig(ctx context.Context, target types.WifiTarget, cfg types.WifiConfig) (*types.WifiActionResult, error) {
	if a.cliExecutor == nil {
		return &types.WifiActionResult{
			OK:        false,
			ErrorCode: types.WifiErrorCodeInternalError,
			Reason:    "CLI executor not available",
		}, nil
	}

	if strings.TrimSpace(cfg.SSID) == "" {
		return &types.WifiActionResult{
			OK:        false,
			ErrorCode: types.WifiErrorCodeInvalidValue,
			Reason:    "SSID is required",
		}, nil
	}
	if len(cfg.Password) > 0 && len(cfg.Password) < 8 {
		return &types.WifiActionResult{
			OK:        false,
			ErrorCode: types.WifiErrorCodeInvalidValue,
			Reason:    "password must be at least 8 characters",
		}, nil
	}

	ponPort, onuID, result := a.resolveWifiTarget(ctx, target)
	if result != nil {
		return result, nil
	}
	if !a.isOMCIProfileReady(ctx, ponPort, onuID) {
		return &types.WifiActionResult{
			OK:        false,
			ErrorCode: types.WifiErrorCodeProfileNotOMCIReady,
			Reason:    "profile not configured for OMCI Wi-Fi",
		}, nil
	}

	profile, err := a.resolveWifiCommandProfile(ctx, ponPort, onuID, target.OnuSerial)
	if err != nil {
		return &types.WifiActionResult{
			OK:        false,
			ErrorCode: types.WifiErrorCodeInternalError,
			Reason:    err.Error(),
		}, nil
	}

	steps := []wifiStep{
		{name: "ENTER_CONFIG", command: "configure terminal"},
		{name: "ENTER_PON_INTERFACE", command: fmt.Sprintf("interface gpon %s", ponPort)},
	}
	steps = append(steps, wifiConfigSteps(profile, onuID, cfg)...)
	steps = append(steps,
		wifiStep{name: "EXIT_INTERFACE", command: "exit"},
		wifiStep{name: "EXIT_CONFIG", command: "end"},
	)

	return a.runWifiSteps(ctx, steps), nil
}

func (a *Adapter) SetWifiEnabled(ctx context.Context, target types.WifiTarget, enabled bool) (*types.WifiActionResult, error) {
	if a.cliExecutor == nil {
		return &types.WifiActionResult{
			OK:        false,
			ErrorCode: types.WifiErrorCodeInternalError,
			Reason:    "CLI executor not available",
		}, nil
	}

	ponPort, onuID, result := a.resolveWifiTarget(ctx, target)
	if result != nil {
		return result, nil
	}
	if !a.isOMCIProfileReady(ctx, ponPort, onuID) {
		return &types.WifiActionResult{
			OK:        false,
			ErrorCode: types.WifiErrorCodeProfileNotOMCIReady,
			Reason:    "profile not configured for OMCI Wi-Fi",
		}, nil
	}

	profile, err := a.resolveWifiCommandProfile(ctx, ponPort, onuID, target.OnuSerial)
	if err != nil {
		return &types.WifiActionResult{
			OK:        false,
			ErrorCode: types.WifiErrorCodeInternalError,
			Reason:    err.Error(),
		}, nil
	}

	steps := []wifiStep{
		{name: "ENTER_CONFIG", command: "configure terminal"},
		{name: "ENTER_PON_INTERFACE", command: fmt.Sprintf("interface gpon %s", ponPort)},
		{name: "ENABLE_WIFI", command: wifiEnableCommand(profile, onuID, enabled)},
		{name: "EXIT_INTERFACE", command: "exit"},
		{name: "EXIT_CONFIG", command: "end"},
	}

	return a.runWifiSteps(ctx, steps), nil
}

func (a *Adapter) runWifiSteps(ctx context.Context, steps []wifiStep) *types.WifiActionResult {
	result := &types.WifiActionResult{
		OK:     true,
		Events: make([]types.WifiActionEvent, 0, len(steps)),
	}

	var rawBuilder strings.Builder
	successfulSteps := 0

	for _, step := range steps {
		output, err := a.cliExecutor.ExecCommand(ctx, step.command)
		sanitizedOutput := sanitizeOutput(step, output)
		if sanitizedOutput != "" {
			if rawBuilder.Len() > 0 {
				rawBuilder.WriteString("\n")
			}
			rawBuilder.WriteString(sanitizedOutput)
		}

		event := types.WifiActionEvent{
			Step:      step.name,
			OK:        err == nil && !hasCLIErrorMarker(output),
			Timestamp: time.Now().UTC(),
		}
		if !event.OK {
			event.Detail = sanitizeOutput(step, firstLine(output))
		}
		result.Events = append(result.Events, event)

		if err != nil || hasCLIErrorMarker(output) {
			result.OK = false
			result.FailedStep = step.name
			result.RawOutput = rawBuilder.String()
			result.Reason = normalizeReason(err, output)
			if successfulSteps > 0 {
				result.ErrorCode = types.WifiErrorCodePartialApply
			} else {
				result.ErrorCode = classifyWifiErrCode(err, output)
			}
			return result
		}

		successfulSteps++
	}

	result.RawOutput = rawBuilder.String()
	return result
}

func (a *Adapter) resolveWifiTarget(ctx context.Context, target types.WifiTarget) (string, int, *types.WifiActionResult) {
	if target.PONPort != "" && target.ONUID > 0 {
		return target.PONPort, target.ONUID, nil
	}

	serial := strings.TrimSpace(target.OnuSerial)
	if serial == "" {
		return "", 0, &types.WifiActionResult{
			OK:        false,
			ErrorCode: types.WifiErrorCodeInvalidValue,
			Reason:    "onuSerial is required when ponPort/onuId are not provided",
		}
	}

	onu, err := a.GetONUBySerial(ctx, serial)
	if err != nil {
		return "", 0, &types.WifiActionResult{
			OK:        false,
			ErrorCode: classifyWifiErrCode(err, ""),
			Reason:    err.Error(),
		}
	}
	if onu == nil {
		return "", 0, &types.WifiActionResult{
			OK:        false,
			ErrorCode: types.WifiErrorCodeOnuNotFound,
			Reason:    "ONU not found in OLT inventory",
		}
	}
	if !onu.IsOnline || isOfflineState(onu.OperState) {
		return "", 0, &types.WifiActionResult{
			OK:        false,
			ErrorCode: types.WifiErrorCodeOnuOffline,
			Reason:    "ONU exists but is offline on OLT",
		}
	}
	if onu.PONPort == "" || onu.ONUID <= 0 {
		return "", 0, &types.WifiActionResult{
			OK:        false,
			ErrorCode: types.WifiErrorCodeInternalError,
			Reason:    "unable to resolve ONU coordinates from inventory",
		}
	}

	return onu.PONPort, onu.ONUID, nil
}

func (a *Adapter) isOMCIProfileReady(ctx context.Context, ponPort string, onuID int) bool {
	// Allow bypass in tests/labs where profile readback is unavailable.
	if a.config != nil && a.config.Metadata != nil {
		if strings.EqualFold(a.config.Metadata["skip_omci_profile_check"], "true") {
			return true
		}
	}

	raw, err := a.GetONURunningConfig(ctx, ponPort, onuID)
	if err != nil {
		return false
	}

	lower := strings.ToLower(raw)
	return strings.Contains(lower, "wifi-mng-via-non-omci disable") ||
		strings.Contains(lower, "wifi management omci")
}

func (a *Adapter) omciWifiReadbackEnabled() bool {
	if a.config == nil || a.config.Metadata == nil {
		return false
	}
	return strings.EqualFold(a.config.Metadata["omci_wifi_readback"], "true")
}

func classifyWifiErrCode(err error, output string) types.WifiErrorCode {
	combined := strings.ToLower(strings.TrimSpace(output))
	if err != nil {
		if combined != "" {
			combined += " "
		}
		combined += strings.ToLower(err.Error())
	}

	switch {
	case strings.Contains(combined, "timeout"):
		return types.WifiErrorCodeCommandTimeout
	case strings.Contains(combined, "permission denied"), strings.Contains(combined, "not authorized"):
		return types.WifiErrorCodePermissionDenied
	case strings.Contains(combined, "not found"), strings.Contains(combined, "no onu"):
		return types.WifiErrorCodeOnuNotFound
	case strings.Contains(combined, "offline"), strings.Contains(combined, "los"):
		return types.WifiErrorCodeOnuOffline
	case strings.Contains(combined, "invalid"), strings.Contains(combined, "out of range"):
		return types.WifiErrorCodeInvalidValue
	default:
		return types.WifiErrorCodeInternalError
	}
}

func hasCLIErrorMarker(output string) bool {
	lower := strings.ToLower(output)
	markers := []string{
		"unknown command",
		"no matched command",
		"% error",
		"error:",
		"invalid input",
		"failed",
		"permission denied",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func isOfflineState(operState string) bool {
	switch strings.ToLower(strings.TrimSpace(operState)) {
	case "offline", "los", "dying_gasp", "down":
		return true
	default:
		return false
	}
}

func quoteArg(v string) string {
	return strconv.Quote(strings.TrimSpace(v))
}

func wifiConfigSteps(profile wifiCommandProfile, onuID int, cfg types.WifiConfig) []wifiStep {
	switch profile {
	case wifiCommandProfilePri:
		return []wifiStep{
			{name: "SET_SSID", command: priSSIDCommand(onuID, cfg), sensitive: len(strings.TrimSpace(cfg.Password)) > 0},
			{name: "ENABLE_WIFI", command: wifiEnableCommand(profile, onuID, cfg.Enabled)},
		}
	default:
		steps := []wifiStep{
			{name: "SET_SSID", command: fmt.Sprintf("onu %d wifi ssid %s", onuID, quoteArg(cfg.SSID))},
		}
		if strings.TrimSpace(cfg.Password) != "" {
			steps = append(steps, wifiStep{
				name:      "SET_PASSWORD",
				command:   fmt.Sprintf("onu %d wifi password %s", onuID, quoteArg(cfg.Password)),
				sensitive: true,
			})
		}
		steps = append(steps, wifiStep{name: "ENABLE_WIFI", command: wifiEnableCommand(profile, onuID, cfg.Enabled)})
		return steps
	}
}

func wifiEnableCommand(profile wifiCommandProfile, onuID int, enabled bool) string {
	switch profile {
	case wifiCommandProfilePri:
		if enabled {
			// V1600G PRI path from lab: country/channel/standard/power required.
			return fmt.Sprintf("onu %d pri wifi_switch 1 enable global auto 80211acanac 10", onuID)
		}
		return fmt.Sprintf("onu %d pri wifi_switch 1 disable", onuID)
	default:
		if enabled {
			return fmt.Sprintf("onu %d wifi enable", onuID)
		}
		return fmt.Sprintf("onu %d wifi disable", onuID)
	}
}

func priSSIDCommand(onuID int, cfg types.WifiConfig) string {
	ssid := strings.TrimSpace(cfg.SSID)
	password := strings.TrimSpace(cfg.Password)
	if password == "" {
		return fmt.Sprintf(
			"onu %d pri wifi_ssid 1 name %s hide disable auth_mode open encrypt_type none",
			onuID,
			quoteArg(ssid),
		)
	}

	return fmt.Sprintf(
		"onu %d pri wifi_ssid 1 name %s hide disable auth_mode wpa2psk encrypt_type aes shared_key %s rekey_interval 3600",
		onuID,
		quoteArg(ssid),
		quoteArg(password),
	)
}

func (a *Adapter) resolveWifiCommandProfile(ctx context.Context, ponPort string, onuID int, serial string) (wifiCommandProfile, error) {
	if forced := a.forcedWifiCommandProfile(); forced != "" {
		return forced, nil
	}
	if hinted := a.hintWifiCommandProfile(ctx, serial); hinted != "" {
		return hinted, nil
	}
	probed, err := a.probeWifiCommandProfile(ctx, ponPort, onuID)
	if err != nil {
		return "", err
	}
	if probed != "" {
		return probed, nil
	}
	return wifiCommandProfileLegacy, nil
}

func (a *Adapter) forcedWifiCommandProfile() wifiCommandProfile {
	if a.config == nil || a.config.Metadata == nil {
		return ""
	}
	return normalizeWifiCommandProfile(a.config.Metadata["wifi_command_profile"])
}

func (a *Adapter) hintWifiCommandProfile(ctx context.Context, serial string) wifiCommandProfile {
	model := ""
	firmware := ""
	if a.config != nil && a.config.Metadata != nil {
		model = strings.ToLower(strings.TrimSpace(a.config.Metadata["model"]))
		firmware = strings.ToLower(strings.TrimSpace(a.config.Metadata["firmware"]))
	}
	if model == "" && strings.TrimSpace(serial) != "" {
		if onu, err := a.GetONUBySerial(ctx, serial); err == nil && onu != nil {
			model = strings.ToLower(strings.TrimSpace(onu.Model))
		}
	}

	// Known path: V1600G/V2.1.6R exposes Wi-Fi on `onu <id> pri ...`.
	if strings.Contains(model, "v1600g") || strings.Contains(firmware, "v2.1.6r") {
		return wifiCommandProfilePri
	}
	return ""
}

func (a *Adapter) probeWifiCommandProfile(ctx context.Context, ponPort string, onuID int) (wifiCommandProfile, error) {
	if a.cliExecutor == nil {
		return "", fmt.Errorf("CLI executor not available")
	}
	priProbe := []string{
		"configure terminal",
		fmt.Sprintf("interface gpon %s", ponPort),
		fmt.Sprintf("onu %d pri ?", onuID),
		"exit",
		"end",
	}
	if outputs, err := a.cliExecutor.ExecCommands(ctx, priProbe); err == nil {
		joined := strings.ToLower(strings.Join(outputs, "\n"))
		if strings.Contains(joined, "wifi_ssid") && strings.Contains(joined, "wifi_switch") {
			return wifiCommandProfilePri, nil
		}
	}

	legacyProbe := []string{
		"configure terminal",
		fmt.Sprintf("interface gpon %s", ponPort),
		fmt.Sprintf("onu %d wifi ?", onuID),
		"exit",
		"end",
	}
	if outputs, err := a.cliExecutor.ExecCommands(ctx, legacyProbe); err == nil {
		joined := strings.ToLower(strings.Join(outputs, "\n"))
		if strings.Contains(joined, "ssid") &&
			!strings.Contains(joined, "unknown command") &&
			!strings.Contains(joined, "no matched command") {
			return wifiCommandProfileLegacy, nil
		}
	}
	return "", nil
}

func sanitizeOutput(step wifiStep, output string) string {
	if output == "" {
		return ""
	}
	if !step.sensitive {
		return output
	}
	// Strip the full password command argument from both command echo and errors.
	return redactPassword(output)
}

func redactPassword(text string) string {
	if text == "" {
		return text
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return text
	}

	for i := 0; i < len(fields)-1; i++ {
		token := strings.ToLower(fields[i])
		if token == "shared_key" {
			fields[i+1] = "<redacted>"
		}
		if token == "password" && i >= 3 && strings.ToLower(fields[i-1]) == "wifi" {
			fields[i+1] = "<redacted>"
		}
	}
	return strings.Join(fields, " ")
}

func firstLine(v string) string {
	lines := strings.Split(strings.TrimSpace(v), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

func normalizeReason(err error, output string) string {
	if err != nil {
		return err.Error()
	}
	line := firstLine(output)
	if line != "" {
		return line
	}
	return "CLI command failed"
}

func normalizeWifiCommandProfile(v string) wifiCommandProfile {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case string(wifiCommandProfileLegacy):
		return wifiCommandProfileLegacy
	case string(wifiCommandProfilePri):
		return wifiCommandProfilePri
	default:
		return ""
	}
}
