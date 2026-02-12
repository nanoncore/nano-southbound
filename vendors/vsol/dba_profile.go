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
	reDBAProfileRow = regexp.MustCompile(`^\s*(\d+)\s+(\S+)\s+(\d)\s+(.+)$`)
	reDBABWFixed    = regexp.MustCompile(`Fixed:\s*(\d+)`)
	reDBABWAssured  = regexp.MustCompile(`Assured:\s*(\d+)`)
	reDBABWMax      = regexp.MustCompile(`Max:\s*(\d+)`)
)

// ListDBAProfiles lists DBA profiles on the OLT.
func (a *Adapter) ListDBAProfiles(ctx context.Context) ([]types.DBAProfile, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}

	commands := []string{
		"configure terminal",
		"show profile dba all",
		"exit",
	}
	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return nil, fmt.Errorf("failed to list DBA profiles: %w", err)
	}

	showOutput := cliOutputAt(outputs, 1)
	return parseDBAProfiles(showOutput)
}

// GetDBAProfile retrieves a specific DBA profile by name.
func (a *Adapter) GetDBAProfile(ctx context.Context, name string) (*types.DBAProfile, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}
	if name == "" {
		return nil, fmt.Errorf("profile name is required")
	}

	profiles, err := a.ListDBAProfiles(ctx)
	if err != nil {
		return nil, err
	}
	for i := range profiles {
		if profiles[i].Name == name {
			return &profiles[i], nil
		}
	}
	return nil, fmt.Errorf("DBA profile %q not found", name)
}

// CreateDBAProfile creates a DBA profile using CLI commands.
func (a *Adapter) CreateDBAProfile(ctx context.Context, profile types.DBAProfile) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}
	if profile.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if profile.Type < 1 || profile.Type > 5 {
		return fmt.Errorf("profile type must be between 1 and 5")
	}

	// Auto-assign ID: find max existing ID and use max+1
	existing, err := a.ListDBAProfiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to list existing profiles: %w", err)
	}
	nextID := 2 // Start at 2 (1 is typically "default")
	for _, p := range existing {
		if p.ID >= nextID {
			nextID = p.ID + 1
		}
	}

	commands := buildDBAProfileCreateCommands(nextID, profile)
	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return fmt.Errorf("failed to create DBA profile: %w", err)
	}
	if err := detectProfileCLIErrors(outputs); err != nil {
		return fmt.Errorf("failed to create DBA profile: %w", err)
	}
	return nil
}

// DeleteDBAProfile deletes a DBA profile by name.
func (a *Adapter) DeleteDBAProfile(ctx context.Context, name string) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}
	if name == "" {
		return fmt.Errorf("profile name is required")
	}

	// Look up profile ID by name
	profile, err := a.GetDBAProfile(ctx, name)
	if err != nil {
		return err
	}

	commands := []string{
		"configure terminal",
		fmt.Sprintf("no profile dba id %d", profile.ID),
		"exit",
	}
	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return fmt.Errorf("failed to delete DBA profile: %w", err)
	}
	if err := detectProfileCLIErrors(outputs); err != nil {
		return fmt.Errorf("failed to delete DBA profile: %w", err)
	}
	return nil
}

func buildDBAProfileCreateCommands(id int, profile types.DBAProfile) []string {
	commands := []string{
		"configure terminal",
		fmt.Sprintf("profile dba id %d", id),
		fmt.Sprintf("name %s", profile.Name),
	}

	// Build type + bandwidth command based on DBA type
	switch profile.Type {
	case 1: // fixed
		commands = append(commands, fmt.Sprintf("type 1 bandwidth %d", profile.FixedBW))
	case 2: // assured
		commands = append(commands, fmt.Sprintf("type 2 bandwidth %d", profile.AssuredBW))
	case 3: // assured + max
		commands = append(commands, fmt.Sprintf("type 3 bandwidth %d bandwidth %d", profile.AssuredBW, profile.MaxBW))
	case 4: // maximum
		commands = append(commands, fmt.Sprintf("type 4 bandwidth %d", profile.MaxBW))
	case 5: // fixed + assured + max
		commands = append(commands, fmt.Sprintf("type 5 bandwidth %d bandwidth %d bandwidth %d", profile.FixedBW, profile.AssuredBW, profile.MaxBW))
	}

	commands = append(commands, "commit", "exit", "exit")
	return commands
}

// parseDBAProfiles parses the output of "show profile dba all".
// Expected format:
//
//	------+---------------------+------+-----------------------
//	  Id    Name                 Type   Bindwidth(kbps)
//	------+---------------------+------+-----------------------
//	  1     default              4      Max: 1024000
//	  3     test_dba_50M         4      Max: 50000
//	------+---------------------+------+-----------------------
func parseDBAProfiles(output string) ([]types.DBAProfile, error) {
	clean := sanitizeProfileOutput(output)
	lines := strings.Split(clean, "\n")

	var profiles []types.DBAProfile
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "---") || strings.Contains(line, "Bindwidth") || strings.Contains(line, "Bandwidth") {
			continue
		}

		match := reDBAProfileRow.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		id, _ := strconv.Atoi(match[1])
		dbaType, _ := strconv.Atoi(match[3])
		bwPart := match[4]

		profile := types.DBAProfile{
			ID:   id,
			Name: match[2],
			Type: dbaType,
		}

		if m := reDBABWFixed.FindStringSubmatch(bwPart); len(m) == 2 {
			profile.FixedBW, _ = strconv.Atoi(m[1])
		}
		if m := reDBABWAssured.FindStringSubmatch(bwPart); len(m) == 2 {
			profile.AssuredBW, _ = strconv.Atoi(m[1])
		}
		if m := reDBABWMax.FindStringSubmatch(bwPart); len(m) == 2 {
			profile.MaxBW, _ = strconv.Atoi(m[1])
		}

		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// detectProfileCLIErrors checks CLI outputs for common error patterns.
func detectProfileCLIErrors(outputs []string) error {
	for _, output := range outputs {
		lower := strings.ToLower(output)
		if strings.Contains(lower, "can't delete profile") {
			return fmt.Errorf("profile is in use: %s", strings.TrimSpace(output))
		}
		if strings.Contains(lower, "already existed") {
			return fmt.Errorf("profile already exists: %s", strings.TrimSpace(output))
		}
		if strings.Contains(lower, "isn't existed") {
			return fmt.Errorf("profile does not exist: %s", strings.TrimSpace(output))
		}
	}
	return nil
}
