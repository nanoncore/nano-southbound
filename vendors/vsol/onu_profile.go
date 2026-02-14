package vsol

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/nanoncore/nano-southbound/types"
)

var (
	reProfileID          = regexp.MustCompile(`^\s*Id:\s*(\d+)`)
	reProfileName        = regexp.MustCompile(`^\s*Name:\s*(.+)$`)
	reProfileDescription = regexp.MustCompile(`^\s*Description:\s*(.+)$`)
	reMaxTcont           = regexp.MustCompile(`^\s*Max tcont:\s*(\d+)`)
	reMaxGemport         = regexp.MustCompile(`^\s*Max gemport:\s*(\d+)`)
	reMaxSwitchPerSlot   = regexp.MustCompile(`^\s*Max switch per slot:\s*(\d+)`)
	reMaxEth             = regexp.MustCompile(`^\s*Max eth:\s*(\d+)`)
	reMaxPots            = regexp.MustCompile(`^\s*Max pots:\s*(\d+)`)
	reMaxIPHost          = regexp.MustCompile(`^\s*Max iphost:\s*(\d+)`)
	reMaxIPv6Host        = regexp.MustCompile(`^\s*Max ipv6host:\s*(\d+)`)
	reMaxVeip            = regexp.MustCompile(`^\s*Max veip:\s*(\d+)`)
	reServiceAbilityN1   = regexp.MustCompile(`^\s*Service ability N:1:\s*(\d+)`)
	reExOMCI             = regexp.MustCompile(`^\s*Ex-OMCI:\s*(\S+)`)
	reWifiMngViaNonOMCI  = regexp.MustCompile(`^\s*Wifi mgmt via non OMCI:\s*(\S+)`)
	reOmciSendMode       = regexp.MustCompile(`^\s*Omci send mode:\s*(\S+)`)
	reDefaultMulticast   = regexp.MustCompile(`^\s*Default multicast range:\s*(\S+)`)
	reCommitStatus       = regexp.MustCompile(`^\s*commit:\s*(\S+)`)
	reANSI               = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
)

// ListONUProfiles lists ONU hardware profiles on the OLT.
func (a *Adapter) ListONUProfiles(ctx context.Context) ([]*types.ONUHardwareProfile, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}

	commands := []string{
		"configure terminal",
		"show profile onu",
		"exit",
	}
	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return nil, fmt.Errorf("failed to list ONU profiles: %w", err)
	}

	showOutput := cliOutputAt(outputs, 1)
	profiles, err := parseONUProfiles(showOutput)
	if err != nil {
		return nil, err
	}
	return profiles, nil
}

// GetONUProfile retrieves a specific ONU hardware profile by name.
func (a *Adapter) GetONUProfile(ctx context.Context, name string) (*types.ONUHardwareProfile, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}
	if name == "" {
		return nil, fmt.Errorf("profile name is required")
	}

	commands := []string{
		"configure terminal",
		fmt.Sprintf("show profile onu name %s", name),
		"exit",
	}
	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return nil, fmt.Errorf("failed to get ONU profile: %w", err)
	}
	showOutput := cliOutputAt(outputs, 1)
	profiles, err := parseONUProfiles(showOutput)
	if err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("profile name %s not found", name)
	}
	return profiles[0], nil
}

// CreateONUProfile creates an ONU hardware profile using CLI commands.
func (a *Adapter) CreateONUProfile(ctx context.Context, profile *types.ONUHardwareProfile) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}
	if err := profile.Validate(); err != nil {
		return err
	}
	if err := validateProfileName(profile.Name); err != nil {
		return err
	}

	commands := buildONUProfileCreateCommands(profile)
	if _, err := a.cliExecutor.ExecCommands(ctx, commands); err != nil {
		return fmt.Errorf("failed to create ONU profile: %w", err)
	}
	return nil
}

// DeleteONUProfile deletes an ONU hardware profile by name.
func (a *Adapter) DeleteONUProfile(ctx context.Context, name string) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}
	if name == "" {
		return fmt.Errorf("profile name is required")
	}

	commands := []string{
		"configure terminal",
		fmt.Sprintf("no profile onu name %s", name),
		"exit",
	}
	if _, err := a.cliExecutor.ExecCommands(ctx, commands); err != nil {
		return fmt.Errorf("failed to delete ONU profile: %w", err)
	}
	return nil
}

func buildONUProfileCreateCommands(profile *types.ONUHardwareProfile) []string {
	commands := []string{
		"configure terminal",
		fmt.Sprintf("profile onu name %s", profile.Name),
	}

	if profile.Ports != nil {
		if profile.Ports.Eth != nil {
			commands = append(commands, fmt.Sprintf("port-num eth %d", *profile.Ports.Eth))
		}
		if profile.Ports.Pots != nil {
			commands = append(commands, fmt.Sprintf("port-num pots %d", *profile.Ports.Pots))
		}
		if profile.Ports.IPHost != nil {
			commands = append(commands, fmt.Sprintf("port-num iphost %d", *profile.Ports.IPHost))
		}
		if profile.Ports.IPv6Host != nil {
			commands = append(commands, fmt.Sprintf("port-num ipv6host %d", *profile.Ports.IPv6Host))
		}
		if profile.Ports.Veip != nil {
			commands = append(commands, fmt.Sprintf("port-num veip %d", *profile.Ports.Veip))
		}
	}

	if profile.TcontNum != nil && profile.GemportNum != nil {
		commands = append(commands, fmt.Sprintf("tcont-num %d gemport-num %d", *profile.TcontNum, *profile.GemportNum))
	}

	if profile.SwitchNum != nil {
		commands = append(commands, fmt.Sprintf("switch-num %d", *profile.SwitchNum))
	}
	if profile.ServiceAbility != nil {
		commands = append(commands, fmt.Sprintf("service-ability %s", *profile.ServiceAbility))
	}
	if profile.OmciSendMode != nil {
		commands = append(commands, fmt.Sprintf("omci-send-mode %s", *profile.OmciSendMode))
	}
	if profile.ExOMCI != nil {
		if *profile.ExOMCI {
			commands = append(commands, "ex-omci enable")
		} else {
			commands = append(commands, "ex-omci disable")
		}
	}
	if profile.WifiMngViaNonOMCI != nil {
		if *profile.WifiMngViaNonOMCI {
			commands = append(commands, "wifi-mng-via-non-omci enable")
		} else {
			commands = append(commands, "wifi-mng-via-non-omci disable")
		}
	}
	if profile.DefaultMulticastRange != nil {
		commands = append(commands, fmt.Sprintf("default-multicast-range %s", *profile.DefaultMulticastRange))
	}
	if profile.Description != nil {
		commands = append(commands, fmt.Sprintf("description %q", *profile.Description))
	}

	commands = append(commands, "commit", "exit", "exit")
	return commands
}

func parseONUProfiles(output string) ([]*types.ONUHardwareProfile, error) {
	clean := sanitizeProfileOutput(output)
	if strings.Contains(clean, "not found") {
		return nil, nil
	}

	lines := strings.Split(clean, "\n")
	var profiles []*types.ONUHardwareProfile
	var current *types.ONUHardwareProfile

	flush := func() {
		if current == nil {
			return
		}
		if current.Name != "" || current.ID != nil {
			profiles = append(profiles, current)
		}
		current = nil
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		if match := reProfileID.FindStringSubmatch(line); len(match) == 2 {
			flush()
			id, _ := strconv.Atoi(match[1])
			current = &types.ONUHardwareProfile{ID: &id}
			continue
		}
		if current == nil {
			continue
		}

		switch {
		case reProfileName.MatchString(line):
			current.Name = strings.TrimSpace(reProfileName.FindStringSubmatch(line)[1])
		case reProfileDescription.MatchString(line):
			desc := strings.TrimSpace(reProfileDescription.FindStringSubmatch(line)[1])
			current.Description = &desc
		case reMaxTcont.MatchString(line):
			val, err := parseProfileInt("max tcont", reMaxTcont.FindStringSubmatch(line)[1])
			if err != nil {
				return nil, err
			}
			current.TcontNum = &val
		case reMaxGemport.MatchString(line):
			val, err := parseProfileInt("max gemport", reMaxGemport.FindStringSubmatch(line)[1])
			if err != nil {
				return nil, err
			}
			current.GemportNum = &val
		case reMaxSwitchPerSlot.MatchString(line):
			val, err := parseProfileInt("max switch", reMaxSwitchPerSlot.FindStringSubmatch(line)[1])
			if err != nil {
				return nil, err
			}
			current.SwitchNum = &val
		case reMaxEth.MatchString(line):
			val, err := parseProfileInt("max eth", reMaxEth.FindStringSubmatch(line)[1])
			if err != nil {
				return nil, err
			}
			ensurePorts(current)
			current.Ports.Eth = &val
		case reMaxPots.MatchString(line):
			val, err := parseProfileInt("max pots", reMaxPots.FindStringSubmatch(line)[1])
			if err != nil {
				return nil, err
			}
			ensurePorts(current)
			current.Ports.Pots = &val
		case reMaxIPHost.MatchString(line):
			val, err := parseProfileInt("max iphost", reMaxIPHost.FindStringSubmatch(line)[1])
			if err != nil {
				return nil, err
			}
			ensurePorts(current)
			current.Ports.IPHost = &val
		case reMaxIPv6Host.MatchString(line):
			val, err := parseProfileInt("max ipv6host", reMaxIPv6Host.FindStringSubmatch(line)[1])
			if err != nil {
				return nil, err
			}
			ensurePorts(current)
			current.Ports.IPv6Host = &val
		case reMaxVeip.MatchString(line):
			val, err := parseProfileInt("max veip", reMaxVeip.FindStringSubmatch(line)[1])
			if err != nil {
				return nil, err
			}
			ensurePorts(current)
			current.Ports.Veip = &val
		case reServiceAbilityN1.MatchString(line):
			val := strings.TrimSpace(reServiceAbilityN1.FindStringSubmatch(line)[1])
			if val == "1" {
				ability := "n:1"
				current.ServiceAbility = &ability
			}
		case reExOMCI.MatchString(line):
			val := strings.TrimSpace(reExOMCI.FindStringSubmatch(line)[1])
			enabled := strings.EqualFold(val, "enable")
			current.ExOMCI = &enabled
		case reWifiMngViaNonOMCI.MatchString(line):
			val := strings.TrimSpace(reWifiMngViaNonOMCI.FindStringSubmatch(line)[1])
			enabled := strings.EqualFold(val, "enable")
			current.WifiMngViaNonOMCI = &enabled
		case reOmciSendMode.MatchString(line):
			val := strings.TrimSpace(reOmciSendMode.FindStringSubmatch(line)[1])
			current.OmciSendMode = &val
		case reDefaultMulticast.MatchString(line):
			val := strings.TrimSpace(reDefaultMulticast.FindStringSubmatch(line)[1])
			current.DefaultMulticastRange = &val
		case reCommitStatus.MatchString(line):
			val := strings.TrimSpace(reCommitStatus.FindStringSubmatch(line)[1])
			committed := strings.EqualFold(val, "yes")
			current.Committed = &committed
		}
	}

	flush()
	return profiles, nil
}

func ensurePorts(profile *types.ONUHardwareProfile) {
	if profile.Ports == nil {
		profile.Ports = &types.ONUProfilePorts{}
	}
}

func sanitizeProfileOutput(output string) string {
	clean := strings.ReplaceAll(output, "\r", "")
	clean = strings.ReplaceAll(clean, "\x08", "")
	clean = reANSI.ReplaceAllString(clean, "")
	lines := strings.Split(clean, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, "--More--") {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

func parseProfileInt(field, value string) (int, error) {
	val, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", field, value, err)
	}
	return val, nil
}

func cliOutputAt(outputs []string, index int) string {
	if len(outputs) > index {
		return outputs[index]
	}
	return ""
}

// reValidProfileName matches names that are safe for CLI interpolation:
// alphanumeric, underscores, hyphens, and dots only.
var reValidProfileName = regexp.MustCompile(`^[a-zA-Z0-9_.\-]+$`)

// validateProfileName ensures a profile name is safe to interpolate into CLI commands.
func validateProfileName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name is required")
	}
	if len(name) > 64 {
		return fmt.Errorf("profile name too long (max 64 characters)")
	}
	if !reValidProfileName.MatchString(name) {
		return fmt.Errorf("profile name %q contains invalid characters (only alphanumeric, underscore, hyphen, dot allowed)", name)
	}
	return nil
}
