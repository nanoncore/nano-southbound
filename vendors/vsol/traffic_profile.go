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
	reTrafficId   = regexp.MustCompile(`(?i)^\s*Id:\s*(\d+)`)
	reTrafficName = regexp.MustCompile(`(?i)^\s*Name:\s*(\S+)`)
	reTrafficSIR  = regexp.MustCompile(`(?i)^\s*sir:\s*(\d+)`)
	reTrafficPIR  = regexp.MustCompile(`(?i)^\s*pir:\s*(\d+)`)
)

// ListTrafficProfiles lists traffic profiles on the OLT.
func (a *Adapter) ListTrafficProfiles(ctx context.Context) ([]types.TrafficProfile, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}

	commands := []string{
		"configure terminal",
		"show profile traffic",
		"exit",
	}
	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return nil, fmt.Errorf("failed to list traffic profiles: %w", err)
	}

	showOutput := cliOutputAt(outputs, 1)
	return parseTrafficProfiles(showOutput)
}

// GetTrafficProfile retrieves a specific traffic profile by name.
func (a *Adapter) GetTrafficProfile(ctx context.Context, name string) (*types.TrafficProfile, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}
	if err := validateProfileName(name); err != nil {
		return nil, err
	}

	profiles, err := a.ListTrafficProfiles(ctx)
	if err != nil {
		return nil, err
	}
	for i := range profiles {
		if profiles[i].Name == name {
			return &profiles[i], nil
		}
	}
	return nil, fmt.Errorf("traffic profile %q not found", name)
}

// CreateTrafficProfile creates a traffic profile using CLI commands.
func (a *Adapter) CreateTrafficProfile(ctx context.Context, profile types.TrafficProfile) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}
	if err := validateProfileName(profile.Name); err != nil {
		return err
	}
	if profile.PIR <= 0 {
		return fmt.Errorf("PIR must be greater than 0")
	}

	// Auto-assign ID: find max existing ID and use max+1
	existing, err := a.ListTrafficProfiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to list existing profiles: %w", err)
	}
	nextID := 2 // Start at 2 (1 is typically "default")
	for _, p := range existing {
		if p.ID >= nextID {
			nextID = p.ID + 1
		}
	}

	commands := buildTrafficProfileCreateCommands(nextID, profile)
	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return fmt.Errorf("failed to create traffic profile: %w", err)
	}
	if err := detectProfileCLIErrors(outputs); err != nil {
		return fmt.Errorf("failed to create traffic profile: %w", err)
	}
	return nil
}

// DeleteTrafficProfile deletes a traffic profile by name.
func (a *Adapter) DeleteTrafficProfile(ctx context.Context, name string) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}
	if err := validateProfileName(name); err != nil {
		return err
	}

	// Look up profile ID by name
	profile, err := a.GetTrafficProfile(ctx, name)
	if err != nil {
		return err
	}

	commands := []string{
		"configure terminal",
		fmt.Sprintf("no profile traffic id %d", profile.ID),
		"exit",
	}
	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return fmt.Errorf("failed to delete traffic profile: %w", err)
	}
	if err := detectProfileCLIErrors(outputs); err != nil {
		return fmt.Errorf("failed to delete traffic profile: %w", err)
	}
	return nil
}

func buildTrafficProfileCreateCommands(id int, profile types.TrafficProfile) []string {
	return []string{
		"configure terminal",
		fmt.Sprintf("profile traffic id %d name %s", id, profile.Name),
		fmt.Sprintf("sir %d pir %d", profile.SIR, profile.PIR),
		"commit",
		"exit",
		"exit",
	}
}

// parseTrafficProfiles parses the output of "show profile traffic".
// Expected DETAILED format:
//
//	###############TRAFFIC PROFILE###########
//	*****************************
//	Id:   1
//	Name: default
//	sir:  0 Kbps
//	pir:  1024000 Kbps
//
//	*****************************
//	Id:   3
//	Name: test_traffic_50M
//	sir:  0 Kbps
//	pir:  50000 Kbps
func parseTrafficProfiles(output string) ([]types.TrafficProfile, error) {
	clean := sanitizeProfileOutput(output)
	lines := strings.Split(clean, "\n")

	var profiles []types.TrafficProfile
	var current *types.TrafficProfile

	for _, raw := range lines {
		line := strings.TrimSpace(raw)

		if strings.HasPrefix(line, "*****") {
			if current != nil && current.Name != "" {
				profiles = append(profiles, *current)
			}
			current = &types.TrafficProfile{}
			continue
		}

		if current == nil {
			continue
		}

		if m := reTrafficId.FindStringSubmatch(line); len(m) == 2 {
			current.ID, _ = strconv.Atoi(m[1])
		} else if m := reTrafficName.FindStringSubmatch(line); len(m) == 2 {
			current.Name = m[1]
		} else if m := reTrafficSIR.FindStringSubmatch(line); len(m) == 2 {
			current.SIR, _ = strconv.Atoi(m[1])
		} else if m := reTrafficPIR.FindStringSubmatch(line); len(m) == 2 {
			current.PIR, _ = strconv.Atoi(m[1])
		}
	}

	if current != nil && current.Name != "" {
		profiles = append(profiles, *current)
	}

	return profiles, nil
}
