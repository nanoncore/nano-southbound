package vsol

import (
	"context"
	"fmt"
	"regexp"
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

var (
	reWifiPasswordValue = regexp.MustCompile(`(?i)(wifi\s+password\s+)(".*?"|\S+)`)
	reSharedKeyValue    = regexp.MustCompile(`(?i)(shared_key\s+)(".*?"|\S+)`)
)

type priWifiDefaults struct {
	Country  string
	Channel  string
	Standard string
	Power    int
	Width    string
}

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
			FailedStep: "READBACK",
			Events: []types.WifiActionEvent{
				{
					Step:      "READBACK",
					OK:        false,
					Timestamp: time.Now().UTC(),
					Detail:    "readback not supported",
				},
			},
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
		FailedStep: "READBACK",
		Events: []types.WifiActionEvent{
			{
				Step:      "READBACK",
				OK:        false,
				Timestamp: time.Now().UTC(),
				Detail:    "parser unavailable",
			},
		},
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
			FailedStep: "PROFILE_OMCI_PRECHECK",
			Events: []types.WifiActionEvent{
				{
					Step:      "PROFILE_OMCI_PRECHECK",
					OK:        false,
					Timestamp: time.Now().UTC(),
				},
			},
		}, nil
	}

	profile, err := a.resolveWifiCommandProfile(ctx, ponPort, onuID, target.OnuSerial)
	if err != nil {
		return &types.WifiActionResult{
			OK:        false,
			ErrorCode: classifyWifiErrCode(err, ""),
			Reason:    err.Error(),
		}, nil
	}

	steps := []wifiStep{
		{name: "ENTER_CONFIG", command: "configure terminal"},
		{name: "ENTER_PON_INTERFACE", command: fmt.Sprintf("interface gpon %s", ponPort)},
	}
	steps = append(steps, wifiConfigSteps(profile, onuID, cfg, a.priDefaults())...)
	steps = append(steps,
		wifiStep{name: "EXIT_INTERFACE", command: "exit"},
		wifiStep{name: "COMMIT", command: "end"},
	)

	return a.runWifiSteps(ctx, steps, true), nil
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
			FailedStep: "PROFILE_OMCI_PRECHECK",
			Events: []types.WifiActionEvent{
				{
					Step:      "PROFILE_OMCI_PRECHECK",
					OK:        false,
					Timestamp: time.Now().UTC(),
				},
			},
		}, nil
	}

	profile, err := a.resolveWifiCommandProfile(ctx, ponPort, onuID, target.OnuSerial)
	if err != nil {
		return &types.WifiActionResult{
			OK:        false,
			ErrorCode: classifyWifiErrCode(err, ""),
			Reason:    err.Error(),
		}, nil
	}

	steps := []wifiStep{
		{name: "ENTER_CONFIG", command: "configure terminal"},
		{name: "ENTER_PON_INTERFACE", command: fmt.Sprintf("interface gpon %s", ponPort)},
		{name: "SET_ENABLED", command: wifiEnableCommand(profile, onuID, enabled, a.priDefaults())},
		{name: "EXIT_INTERFACE", command: "exit"},
		{name: "COMMIT", command: "end"},
	}

	return a.runWifiSteps(ctx, steps, true), nil
}

func (a *Adapter) runWifiSteps(ctx context.Context, steps []wifiStep, includePrecheckEvent bool) *types.WifiActionResult {
	result := &types.WifiActionResult{
		OK:     true,
		Events: make([]types.WifiActionEvent, 0, len(steps)+1),
	}
	if includePrecheckEvent {
		result.Events = append(result.Events, types.WifiActionEvent{
			Step:      "PROFILE_OMCI_PRECHECK",
			OK:        true,
			Timestamp: time.Now().UTC(),
		})
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
			if classifyWifiErrCode(err, output) == types.WifiErrorCodeCommandTimeout {
				result.ErrorCode = types.WifiErrorCodeCommandTimeout
			} else if successfulSteps > 0 {
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
	case strings.Contains(combined, "unsupported operation"):
		return types.WifiErrorCodeUnsupportedOperation
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

func wifiConfigSteps(profile wifiCommandProfile, onuID int, cfg types.WifiConfig, priDefaults priWifiDefaults) []wifiStep {
	switch profile {
	case wifiCommandProfilePri:
		return []wifiStep{
			{name: "SET_SSID", command: priSSIDCommand(onuID, cfg), sensitive: len(strings.TrimSpace(cfg.Password)) > 0},
			{name: "SET_ENABLED", command: wifiEnableCommand(profile, onuID, cfg.Enabled, priDefaults)},
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
		steps = append(steps, wifiStep{name: "SET_ENABLED", command: wifiEnableCommand(profile, onuID, cfg.Enabled, priDefaults)})
		return steps
	}
}

func wifiEnableCommand(profile wifiCommandProfile, onuID int, enabled bool, defaults priWifiDefaults) string {
	switch profile {
	case wifiCommandProfilePri:
		if enabled {
			// PRI path is firmware-specific and therefore metadata-driven.
			if strings.TrimSpace(defaults.Width) != "" {
				return fmt.Sprintf(
					"onu %d pri wifi_switch 1 enable %s %s %s %d %s",
					onuID,
					defaults.Country,
					defaults.Channel,
					defaults.Standard,
					defaults.Power,
					defaults.Width,
				)
			}
			return fmt.Sprintf(
				"onu %d pri wifi_switch 1 enable %s %s %s %d",
				onuID,
				defaults.Country,
				defaults.Channel,
				defaults.Standard,
				defaults.Power,
			)
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

func (a *Adapter) priDefaults() priWifiDefaults {
	defaults := priWifiDefaults{
		Country:  "global",
		Channel:  "auto",
		Standard: "80211acanac",
		Power:    10,
		Width:    "",
	}
	if a.config == nil || a.config.Metadata == nil {
		return defaults
	}
	if country := strings.TrimSpace(a.config.Metadata["wifi_pri_country"]); country != "" {
		defaults.Country = country
	}
	if channel := strings.TrimSpace(a.config.Metadata["wifi_pri_channel"]); channel != "" {
		defaults.Channel = channel
	}
	if standard := strings.TrimSpace(a.config.Metadata["wifi_pri_standard"]); standard != "" {
		defaults.Standard = standard
	}
	if width := strings.TrimSpace(a.config.Metadata["wifi_pri_width"]); width != "" {
		defaults.Width = width
	}
	if powerRaw := strings.TrimSpace(a.config.Metadata["wifi_pri_power"]); powerRaw != "" {
		if power, err := strconv.Atoi(powerRaw); err == nil && power >= 0 && power <= 20 {
			defaults.Power = power
		}
	}
	return defaults
}

func (a *Adapter) resolveWifiCommandProfile(ctx context.Context, ponPort string, onuID int, serial string) (wifiCommandProfile, error) {
	if forced, err := a.forcedWifiCommandProfile(); err != nil {
		return "", err
	} else if forced != "" {
		return forced, nil
	}

	cacheKey := a.wifiCommandProfileCacheKey(ctx, serial, ponPort, onuID)
	if cached, ok := a.getCachedWifiCommandProfile(cacheKey); ok {
		return cached, nil
	}

	if hinted := a.hintWifiCommandProfile(ctx, serial); hinted != "" {
		a.setCachedWifiCommandProfile(cacheKey, hinted)
		return hinted, nil
	}
	probed, err := a.probeWifiCommandProfile(ctx, ponPort, onuID)
	if err != nil {
		return "", err
	}
	if probed != "" {
		a.setCachedWifiCommandProfile(cacheKey, probed)
		return probed, nil
	}
	return "", fmt.Errorf(
		"unsupported operation: unresolved Wi-Fi command profile for onu %d on gpon %s (set metadata wifi_command_profile)",
		onuID,
		ponPort,
	)
}

func (a *Adapter) forcedWifiCommandProfile() (wifiCommandProfile, error) {
	if a.config == nil || a.config.Metadata == nil {
		return "", nil
	}
	raw := strings.TrimSpace(a.config.Metadata["wifi_command_profile"])
	if raw == "" {
		return "", nil
	}
	normalized := normalizeWifiCommandProfile(raw)
	if normalized == "" {
		return "", fmt.Errorf("unsupported operation: invalid wifi_command_profile %q", raw)
	}
	return normalized, nil
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

func (a *Adapter) wifiCommandProfileCacheKey(ctx context.Context, serial, ponPort string, onuID int) string {
	model := ""
	firmware := ""
	onuModel := ""
	if a.config != nil && a.config.Metadata != nil {
		model = strings.ToLower(strings.TrimSpace(a.config.Metadata["model"]))
		firmware = strings.ToLower(strings.TrimSpace(a.config.Metadata["firmware"]))
	}
	if model == "" {
		model = strings.ToLower(strings.TrimSpace(a.detectModel()))
	}
	if strings.TrimSpace(serial) != "" {
		if onu, err := a.GetONUBySerial(ctx, serial); err == nil && onu != nil {
			onuModel = strings.ToLower(strings.TrimSpace(onu.Model))
		}
	}
	if onuModel == "" {
		onuModel = fmt.Sprintf("%s:%d", ponPort, onuID)
	}
	return strings.Join([]string{string(types.VendorVSOL), model, firmware, onuModel}, "|")
}

func (a *Adapter) getCachedWifiCommandProfile(key string) (wifiCommandProfile, bool) {
	a.wifiProfileMu.RLock()
	defer a.wifiProfileMu.RUnlock()
	if a.wifiProfileCache == nil {
		return "", false
	}
	raw, ok := a.wifiProfileCache[key]
	if !ok {
		return "", false
	}
	normalized := normalizeWifiCommandProfile(raw)
	return normalized, normalized != ""
}

func (a *Adapter) setCachedWifiCommandProfile(key string, profile wifiCommandProfile) {
	a.wifiProfileMu.Lock()
	defer a.wifiProfileMu.Unlock()
	if a.wifiProfileCache == nil {
		a.wifiProfileCache = make(map[string]string)
	}
	a.wifiProfileCache[key] = string(profile)
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
	text = reWifiPasswordValue.ReplaceAllString(text, `${1}<redacted>`)
	text = reSharedKeyValue.ReplaceAllString(text, `${1}<redacted>`)
	return text
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
