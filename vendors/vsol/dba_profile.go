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
	reDBAId      = regexp.MustCompile(`(?i)^\s*Id:\s*(\d+)`)
	reDBAName    = regexp.MustCompile(`(?i)^\s*name:\s*(\S+)`)
	reDBAType    = regexp.MustCompile(`(?i)^\s*type:\s*(\d+)`)
	reDBAFixed   = regexp.MustCompile(`(?i)^\s*fixed:\s*(\d+)`)
	reDBAAssured = regexp.MustCompile(`(?i)^\s*assured:\s*(\d+)`)
	reDBAMaximum = regexp.MustCompile(`(?i)^\s*maximum:\s*(\d+)`)
)

// ListDBAProfiles lists DBA profiles on the OLT.
func (a *Adapter) ListDBAProfiles(ctx context.Context) ([]types.DBAProfile, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}

	commands := []string{
		"configure terminal",
		"show profile dba",
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
	if err := validateProfileName(name); err != nil {
		return nil, err
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
	if err := validateProfileName(profile.Name); err != nil {
		return err
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
	if err := validateProfileName(name); err != nil {
		return err
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
		fmt.Sprintf("profile dba id %d name %s", id, profile.Name),
	}

	switch profile.Type {
	case 1: // fixed
		commands = append(commands, fmt.Sprintf("type 1 fixed %d", profile.FixedBW))
	case 2: // assured
		commands = append(commands, fmt.Sprintf("type 2 assured %d", profile.AssuredBW))
	case 3: // assured + max
		commands = append(commands, fmt.Sprintf("type 3 assured %d maximum %d", profile.AssuredBW, profile.MaxBW))
	case 4: // maximum
		commands = append(commands, fmt.Sprintf("type 4 maximum %d", profile.MaxBW))
	case 5: // fixed + assured + max
		commands = append(commands, fmt.Sprintf("type 5 fixed %d assured %d maximum %d", profile.FixedBW, profile.AssuredBW, profile.MaxBW))
	}

	commands = append(commands, "commit", "exit", "exit")
	return commands
}

// parseDBAProfiles parses the output of "show profile dba".
// Expected DETAILED format:
//
//	###############DBA PROFILE###########
//	*****************************
//	              Id: 1
//	            name: INTERNET
//	            type: 4
//	         maximum: 512000 Kbps
//
//	*****************************
//	              Id: 511
//	            name: default1
//	            type: 3
//	         assured: 1024 Kbps
//	         maximum: 1024000 Kbps
func parseDBAProfiles(output string) ([]types.DBAProfile, error) {
	clean := sanitizeProfileOutput(output)
	lines := strings.Split(clean, "\n")

	var profiles []types.DBAProfile
	var current *types.DBAProfile

	for _, raw := range lines {
		line := strings.TrimSpace(raw)

		// Profile separator — emit current and start new
		if strings.HasPrefix(line, "*****") {
			if current != nil && current.Name != "" {
				profiles = append(profiles, *current)
			}
			current = &types.DBAProfile{}
			continue
		}

		if current == nil {
			continue
		}

		if m := reDBAId.FindStringSubmatch(line); len(m) == 2 {
			current.ID, _ = strconv.Atoi(m[1])
		} else if m := reDBAName.FindStringSubmatch(line); len(m) == 2 {
			current.Name = m[1]
		} else if m := reDBAType.FindStringSubmatch(line); len(m) == 2 {
			current.Type, _ = strconv.Atoi(m[1])
		} else if m := reDBAFixed.FindStringSubmatch(line); len(m) == 2 {
			current.FixedBW, _ = strconv.Atoi(m[1])
		} else if m := reDBAAssured.FindStringSubmatch(line); len(m) == 2 {
			current.AssuredBW, _ = strconv.Atoi(m[1])
		} else if m := reDBAMaximum.FindStringSubmatch(line); len(m) == 2 {
			current.MaxBW, _ = strconv.Atoi(m[1])
		}
	}

	// Emit last profile
	if current != nil && current.Name != "" {
		profiles = append(profiles, *current)
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
		if strings.Contains(lower, "isn't existed") || strings.Contains(lower, "is not exist") {
			return fmt.Errorf("profile does not exist: %s", strings.TrimSpace(output))
		}
	}
	return nil
}
