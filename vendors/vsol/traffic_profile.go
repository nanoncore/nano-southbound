package vsol

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/nanoncore/nano-southbound/types"
)

var reTrafficProfileRow = regexp.MustCompile(`^\s*(\d+)\s+(\S+)\s+(\d+)\s+(\d+)\s*$`)

// ListTrafficProfiles lists traffic profiles on the OLT.
func (a *Adapter) ListTrafficProfiles(ctx context.Context) ([]types.TrafficProfile, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}

	commands := []string{
		"configure terminal",
		"show profile traffic all",
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
	if name == "" {
		return nil, fmt.Errorf("profile name is required")
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
	if profile.Name == "" {
		return fmt.Errorf("profile name is required")
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
	if name == "" {
		return fmt.Errorf("profile name is required")
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
		fmt.Sprintf("profile traffic id %d", id),
		fmt.Sprintf("name %s", profile.Name),
		fmt.Sprintf("sir %d", profile.SIR),
		fmt.Sprintf("pir %d", profile.PIR),
		"commit",
		"exit",
		"exit",
	}
}

// parseTrafficProfiles parses the output of "show profile traffic all".
// Expected format:
//
//	------+---------------------+---------------+---------------
//	  Id    Name                 SIR(kbps)       PIR(kbps)
//	------+---------------------+---------------+---------------
//	  1     default              0               1024000
//	  3     test_traffic_50M     0               50000
//	------+---------------------+---------------+---------------
func parseTrafficProfiles(output string) ([]types.TrafficProfile, error) {
	clean := sanitizeProfileOutput(output)
	lines := strings.Split(clean, "\n")

	var profiles []types.TrafficProfile
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "---") || strings.Contains(line, "SIR") || strings.Contains(line, "PIR") {
			continue
		}

		match := reTrafficProfileRow.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		id, _ := strconv.Atoi(match[1])
		sir, _ := strconv.Atoi(match[3])
		pir, _ := strconv.Atoi(match[4])

		profiles = append(profiles, types.TrafficProfile{
			ID:   id,
			Name: match[2],
			SIR:  sir,
			PIR:  pir,
		})
	}

	return profiles, nil
}
