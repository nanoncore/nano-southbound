package vsol

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nanoncore/nano-southbound/types"
)

// ListLineProfiles lists line profiles on the OLT.
func (a *Adapter) ListLineProfiles(ctx context.Context) ([]*types.LineProfile, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}

	commands := []string{
		"configure terminal",
		"show profile line",
		"exit",
	}
	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return nil, fmt.Errorf("failed to list line profiles: %w", err)
	}

	showOutput := cliOutputAt(outputs, 1)
	profiles, err := parseLineProfiles(showOutput)
	if err != nil {
		return nil, err
	}
	return profiles, nil
}

// GetLineProfile retrieves a specific line profile by name.
func (a *Adapter) GetLineProfile(ctx context.Context, name string) (*types.LineProfile, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}
	if name == "" {
		return nil, fmt.Errorf("profile name is required")
	}

	commands := []string{
		"configure terminal",
		fmt.Sprintf("show profile line name %s", name),
		"exit",
	}
	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return nil, fmt.Errorf("failed to get line profile: %w", err)
	}
	showOutput := cliOutputAt(outputs, 1)
	profiles, err := parseLineProfiles(showOutput)
	if err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("profile name %s not found", name)
	}
	return profiles[0], nil
}

// CreateLineProfile creates a line profile using CLI commands.
func (a *Adapter) CreateLineProfile(ctx context.Context, profile *types.LineProfile) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}
	if err := profile.Validate(); err != nil {
		return err
	}
	if err := validateProfileName(profile.Name); err != nil {
		return err
	}

	commands := buildLineProfileCreateCommands(profile)
	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return fmt.Errorf("failed to create line profile: %w", err)
	}
	if err := detectLineProfileCLIErrors(commands, outputs); err != nil {
		return err
	}
	return nil
}

// DeleteLineProfile deletes a line profile by name.
func (a *Adapter) DeleteLineProfile(ctx context.Context, name string) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}
	if name == "" {
		return fmt.Errorf("profile name is required")
	}

	commands := []string{
		"configure terminal",
		fmt.Sprintf("no profile line name %s", name),
		"exit",
	}
	if _, err := a.cliExecutor.ExecCommands(ctx, commands); err != nil {
		return fmt.Errorf("failed to delete line profile: %w", err)
	}
	return nil
}

func buildLineProfileCreateCommands(profile *types.LineProfile) []string {
	commands := []string{
		"configure terminal",
		fmt.Sprintf("profile line name %s", profile.Name),
	}

	for _, tcont := range profile.Tconts {
		if tcont == nil {
			continue
		}
		tcontCmd := fmt.Sprintf("tcont %d", tcont.ID)
		if tcont.Name != "" {
			tcontCmd += fmt.Sprintf(" name %s", tcont.Name)
		}
		if tcont.DBA != "" {
			tcontCmd += fmt.Sprintf(" dba %s", tcont.DBA)
		}
		commands = append(commands, tcontCmd)

		for _, gem := range tcont.Gemports {
			if gem == nil {
				continue
			}
			gemCmd := fmt.Sprintf("gemport %d", gem.ID)
			if gem.TcontID != 0 {
				gemCmd += fmt.Sprintf(" tcont %d", gem.TcontID)
			} else {
				gemCmd += fmt.Sprintf(" tcont %d", tcont.ID)
			}
			commands = append(commands, gemCmd)

			// Traffic-limit must be a separate command on V-SOL OLTs.
			// NOTE: The input syntax uses "upstream"/"downstream" (no hyphens),
			// while the show output displays "up-stream"/"down-stream" (with hyphens).
			if gem.TrafficLimitUp != "" || gem.TrafficLimitDn != "" {
				up := gem.TrafficLimitUp
				if up == "" {
					up = "default"
				}
				dn := gem.TrafficLimitDn
				if dn == "" {
					dn = "default"
				}
				commands = append(commands, fmt.Sprintf("gemport %d traffic-limit upstream %s downstream %s", gem.ID, up, dn))
			}

			for _, service := range gem.Services {
				if service == nil {
					continue
				}
				serviceCmd := fmt.Sprintf("service %s", service.Name)
				if service.GemportID != 0 {
					serviceCmd += fmt.Sprintf(" gemport %d", service.GemportID)
				} else {
					serviceCmd += fmt.Sprintf(" gemport %d", gem.ID)
				}
				if service.VLAN != 0 {
					serviceCmd += fmt.Sprintf(" vlan %d", service.VLAN)
				}
				if service.COS != "" {
					serviceCmd += fmt.Sprintf(" cos %s", service.COS)
				}
				commands = append(commands, serviceCmd)
			}

			for _, sp := range gem.ServicePorts {
				if sp == nil {
					continue
				}
				spCmd := fmt.Sprintf("service-port %d", sp.ID)
				if sp.GemportID != 0 {
					spCmd += fmt.Sprintf(" gemport %d", sp.GemportID)
				} else {
					spCmd += fmt.Sprintf(" gemport %d", gem.ID)
				}
				if sp.UserVLAN != 0 {
					spCmd += fmt.Sprintf(" uservlan %d", sp.UserVLAN)
				}
				if sp.VLAN != 0 {
					spCmd += fmt.Sprintf(" vlan %d", sp.VLAN)
				}
				commands = append(commands, spCmd)
			}
		}
	}

	if profile.Mvlan != nil {
		mvlanCmd := buildMvlanCommand(profile.Mvlan)
		if mvlanCmd != "" {
			commands = append(commands, mvlanCmd)
		}
	}

	commands = append(commands, "commit", "exit", "exit")
	return commands
}

func detectLineProfileCLIErrors(commands, outputs []string) error {
	errorPatterns := []string{
		"unknown command",
		"unknown gemport",
		"error:",
		"fail",
		"invalid",
		"command incomplete",
		"not found",
		"already existed",
		"isn't existed",
		"is not exist",
	}
	for i, output := range outputs {
		lower := strings.ToLower(output)
		for _, pattern := range errorPatterns {
			if !strings.Contains(lower, pattern) {
				continue
			}
			cmd := ""
			if i < len(commands) {
				cmd = commands[i]
			}
			if pattern == "unknown gemport" {
				return fmt.Errorf("OLT rejected gemport reference for %q: %s", cmd, strings.TrimSpace(output))
			}
			return fmt.Errorf("OLT rejected command %q: %s", cmd, strings.TrimSpace(output))
		}
	}
	return nil
}

func buildMvlanCommand(mvlan *types.LineProfileMvlan) string {
	if mvlan == nil {
		return ""
	}
	if mvlan.Raw != "" {
		return fmt.Sprintf("mvlan %s", mvlan.Raw)
	}
	if len(mvlan.VLANs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(mvlan.VLANs))
	for _, vlan := range mvlan.VLANs {
		parts = append(parts, strconv.Itoa(vlan))
	}
	return fmt.Sprintf("mvlan %s", strings.Join(parts, ","))
}

func parseLineProfiles(output string) ([]*types.LineProfile, error) {
	clean := sanitizeProfileOutput(output)
	if strings.Contains(strings.ToLower(clean), "not found") {
		return nil, nil
	}

	lines := strings.Split(clean, "\n")
	var profiles []*types.LineProfile
	var current *types.LineProfile
	var currentTcont *types.LineProfileTcont
	var currentGemport *types.LineProfileGemport

	flush := func() {
		if current == nil {
			return
		}
		if current.Name != "" || current.ID != nil {
			profiles = append(profiles, current)
		}
		current = nil
		currentTcont = nil
		currentGemport = nil
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		if match := reProfileID.FindStringSubmatch(line); len(match) == 2 {
			flush()
			id, _ := strconv.Atoi(match[1])
			current = &types.LineProfile{ID: &id}
			continue
		}
		if current == nil {
			continue
		}
		if reProfileName.MatchString(line) {
			current.Name = strings.TrimSpace(reProfileName.FindStringSubmatch(line)[1])
			continue
		}
		if reCommitStatus.MatchString(line) {
			val := strings.TrimSpace(reCommitStatus.FindStringSubmatch(line)[1])
			committed := strings.EqualFold(val, "yes")
			current.Committed = &committed
			continue
		}

		if strings.HasPrefix(line, "tcont ") {
			tcont, err := parseLineProfileTcont(line)
			if err != nil {
				return nil, err
			}
			current.Tconts = append(current.Tconts, tcont)
			currentTcont = tcont
			currentGemport = nil
			continue
		}
		if strings.HasPrefix(line, "gemport ") {
			gem, err := parseLineProfileGemport(line)
			if err != nil {
				return nil, err
			}
			if currentTcont != nil && gem.TcontID == 0 {
				gem.TcontID = currentTcont.ID
			}
			if currentTcont != nil {
				currentTcont.Gemports = append(currentTcont.Gemports, gem)
			}
			currentGemport = gem
			continue
		}
		if strings.HasPrefix(line, "service-port ") {
			sp, err := parseLineProfileServicePort(line)
			if err != nil {
				return nil, err
			}
			if currentGemport != nil && sp.GemportID == 0 {
				sp.GemportID = currentGemport.ID
			}
			if currentGemport != nil {
				currentGemport.ServicePorts = append(currentGemport.ServicePorts, sp)
			}
			continue
		}
		if strings.HasPrefix(line, "service ") {
			service, err := parseLineProfileService(line)
			if err != nil {
				return nil, err
			}
			if currentGemport != nil && service.GemportID == 0 {
				service.GemportID = currentGemport.ID
			}
			if currentGemport != nil {
				currentGemport.Services = append(currentGemport.Services, service)
			}
			continue
		}
		if strings.HasPrefix(line, "mvlan ") {
			mvlan, err := parseLineProfileMvlan(line)
			if err != nil {
				return nil, err
			}
			current.Mvlan = mvlan
		}
	}

	flush()
	return profiles, nil
}

func parseLineProfileTcont(line string) (*types.LineProfileTcont, error) {
	tokens := strings.Fields(line)
	if len(tokens) < 2 {
		return nil, fmt.Errorf("invalid tcont line: %s", line)
	}
	id, err := strconv.Atoi(tokens[1])
	if err != nil {
		return nil, fmt.Errorf("invalid tcont id %q", tokens[1])
	}
	tcont := &types.LineProfileTcont{ID: id}
	for i := 2; i < len(tokens); i++ {
		switch tokens[i] {
		case "name":
			if i+1 < len(tokens) {
				tcont.Name = tokens[i+1]
				i++
			}
		case "dba":
			if i+1 < len(tokens) {
				tcont.DBA = tokens[i+1]
				i++
			}
		}
	}
	return tcont, nil
}

func parseLineProfileGemport(line string) (*types.LineProfileGemport, error) {
	tokens := strings.Fields(line)
	if len(tokens) < 2 {
		return nil, fmt.Errorf("invalid gemport line: %s", line)
	}
	id, err := strconv.Atoi(tokens[1])
	if err != nil {
		return nil, fmt.Errorf("invalid gemport id %q", tokens[1])
	}
	gem := &types.LineProfileGemport{ID: id}
	for i := 2; i < len(tokens); i++ {
		switch tokens[i] {
		case "name":
			if i+1 < len(tokens) {
				gem.Name = tokens[i+1]
				i++
			}
		case "tcont":
			if i+1 < len(tokens) {
				val, err := strconv.Atoi(tokens[i+1])
				if err == nil {
					gem.TcontID = val
				}
				i++
			}
		case "traffic-limit":
			j := i + 1
			for j+1 < len(tokens) {
				switch tokens[j] {
				case "up-stream":
					gem.TrafficLimitUp = tokens[j+1]
					j += 2
				case "down-stream":
					gem.TrafficLimitDn = tokens[j+1]
					j += 2
				default:
					i = j - 1
					j = len(tokens)
				}
			}
			if j == len(tokens) {
				i = j - 1
			}
		case "encrypt":
			if i+1 < len(tokens) {
				val := strings.EqualFold(tokens[i+1], "enable")
				gem.Encrypt = &val
				i++
			}
		case "state":
			if i+1 < len(tokens) {
				gem.State = tokens[i+1]
				i++
			}
		case "down-queue-map-id":
			if i+1 < len(tokens) {
				val, err := strconv.Atoi(tokens[i+1])
				if err == nil {
					gem.DownQueueMapID = &val
				}
				i++
			}
		}
	}
	return gem, nil
}

func parseLineProfileService(line string) (*types.LineProfileService, error) {
	tokens := strings.Fields(line)
	if len(tokens) < 2 {
		return nil, fmt.Errorf("invalid service line: %s", line)
	}
	service := &types.LineProfileService{Name: tokens[1]}
	for i := 2; i < len(tokens); i++ {
		switch tokens[i] {
		case "gemport":
			if i+1 < len(tokens) {
				val, err := strconv.Atoi(tokens[i+1])
				if err == nil {
					service.GemportID = val
				}
				i++
			}
		case "vlan":
			if i+1 < len(tokens) {
				val, err := strconv.Atoi(tokens[i+1])
				if err == nil {
					service.VLAN = val
				}
				i++
			}
		case "cos":
			if i+1 < len(tokens) {
				service.COS = tokens[i+1]
				i++
			}
		}
	}
	return service, nil
}

func parseLineProfileServicePort(line string) (*types.LineProfileServicePort, error) {
	tokens := strings.Fields(line)
	if len(tokens) < 2 {
		return nil, fmt.Errorf("invalid service-port line: %s", line)
	}
	id, err := strconv.Atoi(tokens[1])
	if err != nil {
		return nil, fmt.Errorf("invalid service-port id %q", tokens[1])
	}
	sp := &types.LineProfileServicePort{ID: id}
	for i := 2; i < len(tokens); i++ {
		switch tokens[i] {
		case "gemport":
			if i+1 < len(tokens) {
				val, err := strconv.Atoi(tokens[i+1])
				if err == nil {
					sp.GemportID = val
				}
				i++
			}
		case "uservlan":
			if i+1 < len(tokens) {
				val, err := strconv.Atoi(tokens[i+1])
				if err == nil {
					sp.UserVLAN = val
				}
				i++
			}
		case "vlan":
			if i+1 < len(tokens) {
				val, err := strconv.Atoi(tokens[i+1])
				if err == nil {
					sp.VLAN = val
				}
				i++
			}
		case "admin-status":
			if i+1 < len(tokens) {
				sp.AdminStatus = tokens[i+1]
				i++
			}
		case "description":
			if i+1 < len(tokens) {
				sp.Description = strings.Join(tokens[i+1:], " ")
				i = len(tokens)
			}
		}
	}
	return sp, nil
}

func parseLineProfileMvlan(line string) (*types.LineProfileMvlan, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid mvlan line: %s", line)
	}
	raw := strings.Join(parts[1:], " ")
	mvlan := &types.LineProfileMvlan{Raw: raw}
	if err := mvlan.Validate(); err != nil {
		return nil, err
	}
	return mvlan, nil
}
