package vsol

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
)

// Adapter wraps a base driver with V-SOL-specific logic
// V-SOL OLTs (V1600G series) use CLI + SNMP, with optional EMS REST API
type Adapter struct {
	baseDriver  types.Driver
	cliExecutor types.CLIExecutor
	config      *types.EquipmentConfig
}

// NewAdapter creates a new V-SOL adapter
func NewAdapter(baseDriver types.Driver, config *types.EquipmentConfig) types.Driver {
	adapter := &Adapter{baseDriver: baseDriver, config: config}

	// Check if base driver supports CLI execution
	if executor, ok := baseDriver.(types.CLIExecutor); ok {
		adapter.cliExecutor = executor
	}

	return adapter
}

func (a *Adapter) Connect(ctx context.Context, config *types.EquipmentConfig) error {
	return a.baseDriver.Connect(ctx, config)
}

func (a *Adapter) Disconnect(ctx context.Context) error {
	return a.baseDriver.Disconnect(ctx)
}

func (a *Adapter) IsConnected() bool {
	return a.baseDriver.IsConnected()
}

// CreateSubscriber provisions an ONU on the V-SOL OLT
func (a *Adapter) CreateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) (*types.SubscriberResult, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - V-SOL requires CLI driver")
	}

	// Parse subscriber info
	ponPort := a.getPONPort(subscriber)
	onuID := a.getONUID(subscriber)
	serial := subscriber.Spec.ONUSerial
	vlan := subscriber.Spec.VLAN

	// Get bandwidth rates in kbps
	bandwidthDown := tier.Spec.BandwidthDown // Mbps
	bandwidthUp := tier.Spec.BandwidthUp     // Mbps

	// V-SOL CLI command sequence for GPON ONU provisioning
	var commands []string

	if a.detectPONType() == "gpon" {
		commands = a.buildGPONCommands(ponPort, onuID, serial, vlan, bandwidthDown, bandwidthUp, subscriber, tier)
	} else {
		commands = a.buildEPONCommands(ponPort, onuID, serial, vlan, bandwidthDown, bandwidthUp, subscriber, tier)
	}

	// Execute commands
	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return nil, fmt.Errorf("V-SOL provisioning failed: %w", err)
	}

	// Build result
	result := &types.SubscriberResult{
		SubscriberID:  subscriber.Name,
		SessionID:     fmt.Sprintf("onu-%s-%d", ponPort, onuID),
		AssignedIP:    subscriber.Spec.IPAddress,
		AssignedIPv6:  subscriber.Spec.IPv6Address,
		InterfaceName: fmt.Sprintf("gpon %s onu %d", ponPort, onuID),
		VLAN:          vlan,
		Metadata: map[string]interface{}{
			"vendor":      "vsol",
			"model":       a.detectModel(),
			"pon_type":    a.detectPONType(),
			"pon_port":    ponPort,
			"onu_id":      onuID,
			"serial":      serial,
			"cli_outputs": outputs,
		},
	}

	return result, nil
}

// buildGPONCommands builds V-SOL GPON CLI commands
func (a *Adapter) buildGPONCommands(ponPort string, onuID int, serial string, vlan int, bwDown, bwUp int, subscriber *model.Subscriber, tier *model.ServiceTier) []string {
	// V-SOL V1600G GPON CLI reference
	// Based on V-SOL CLI User Manual

	commands := []string{
		// Enter config mode
		"config",

		// Select GPON interface
		fmt.Sprintf("interface gpon %s", ponPort),

		// Register ONU with serial number
		// onu <onu-id> type <type> sn <serial>
		// Types: router, bridge, hgu, sfu
		fmt.Sprintf("onu %d type router sn %s", onuID, serial),

		// Assign line profile (T-CONT + GEM port mapping)
		// onu profile <onu-id> line-profile <name> service-profile <name>
		fmt.Sprintf("onu profile %d line-profile %s service-profile %s",
			onuID,
			a.getLineProfile(tier),
			a.getServiceProfile(tier)),

		// Configure VLAN for the ONU
		// onu vlan <onu-id> user-vlan <vlan> priority <0-7>
		fmt.Sprintf("onu vlan %d user-vlan %d priority 0", onuID, vlan),

		// Configure bandwidth (ingress = upstream, egress = downstream)
		// onu flowctrl <onu-id> ingress <kbps> egress <kbps>
		fmt.Sprintf("onu flowctrl %d ingress %d egress %d",
			onuID,
			bwUp*1000,    // Convert Mbps to kbps
			bwDown*1000), // Convert Mbps to kbps

		// Enable the ONU
		fmt.Sprintf("no onu disable %d", onuID),

		// Exit interface config
		"exit",

		// Apply changes
		"commit",
		"end",
	}

	return commands
}

// buildEPONCommands builds V-SOL EPON CLI commands
func (a *Adapter) buildEPONCommands(ponPort string, onuID int, mac string, vlan int, bwDown, bwUp int, subscriber *model.Subscriber, tier *model.ServiceTier) []string {
	// V-SOL EPON CLI reference

	commands := []string{
		"config",
		fmt.Sprintf("interface epon %s", ponPort),

		// Register LLID with MAC address
		// llid <llid-id> mac <mac-address>
		fmt.Sprintf("llid %d mac %s", onuID, mac),

		// Assign profiles
		fmt.Sprintf("llid profile %d line-profile %s service-profile %s",
			onuID,
			a.getLineProfile(tier),
			a.getServiceProfile(tier)),

		// Configure VLAN
		fmt.Sprintf("llid vlan %d user-vlan %d", onuID, vlan),

		// Configure bandwidth
		fmt.Sprintf("llid flowctrl %d ingress %d egress %d",
			onuID,
			bwUp*1000,
			bwDown*1000),

		"exit",
		"commit",
		"end",
	}

	return commands
}

func (a *Adapter) UpdateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	ponPort := a.getPONPort(subscriber)
	onuID := a.getONUID(subscriber)
	vlan := subscriber.Spec.VLAN
	bwDown := tier.Spec.BandwidthDown
	bwUp := tier.Spec.BandwidthUp

	var commands []string

	if a.detectPONType() == "gpon" {
		commands = []string{
			"config",
			fmt.Sprintf("interface gpon %s", ponPort),
			// Update profiles
			fmt.Sprintf("onu profile %d line-profile %s service-profile %s",
				onuID, a.getLineProfile(tier), a.getServiceProfile(tier)),
			// Update VLAN
			fmt.Sprintf("onu vlan %d user-vlan %d priority 0", onuID, vlan),
			// Update bandwidth
			fmt.Sprintf("onu flowctrl %d ingress %d egress %d", onuID, bwUp*1000, bwDown*1000),
			"exit",
			"commit",
			"end",
		}
	} else {
		commands = []string{
			"config",
			fmt.Sprintf("interface epon %s", ponPort),
			fmt.Sprintf("llid profile %d line-profile %s service-profile %s",
				onuID, a.getLineProfile(tier), a.getServiceProfile(tier)),
			fmt.Sprintf("llid vlan %d user-vlan %d", onuID, vlan),
			fmt.Sprintf("llid flowctrl %d ingress %d egress %d", onuID, bwUp*1000, bwDown*1000),
			"exit",
			"commit",
			"end",
		}
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	return err
}

func (a *Adapter) DeleteSubscriber(ctx context.Context, subscriberID string) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	// Parse subscriberID to get PON port and ONU ID
	ponPort, onuID := a.parseSubscriberID(subscriberID)

	var commands []string

	if a.detectPONType() == "gpon" {
		commands = []string{
			"config",
			fmt.Sprintf("interface gpon %s", ponPort),
			fmt.Sprintf("no onu %d", onuID),
			"exit",
			"commit",
			"end",
		}
	} else {
		commands = []string{
			"config",
			fmt.Sprintf("interface epon %s", ponPort),
			fmt.Sprintf("no llid %d", onuID),
			"exit",
			"commit",
			"end",
		}
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	return err
}

func (a *Adapter) SuspendSubscriber(ctx context.Context, subscriberID string) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	ponPort, onuID := a.parseSubscriberID(subscriberID)

	var commands []string

	if a.detectPONType() == "gpon" {
		commands = []string{
			"config",
			fmt.Sprintf("interface gpon %s", ponPort),
			fmt.Sprintf("onu disable %d", onuID),
			"exit",
			"commit",
			"end",
		}
	} else {
		commands = []string{
			"config",
			fmt.Sprintf("interface epon %s", ponPort),
			fmt.Sprintf("llid disable %d", onuID),
			"exit",
			"commit",
			"end",
		}
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	return err
}

func (a *Adapter) ResumeSubscriber(ctx context.Context, subscriberID string) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	ponPort, onuID := a.parseSubscriberID(subscriberID)

	var commands []string

	if a.detectPONType() == "gpon" {
		commands = []string{
			"config",
			fmt.Sprintf("interface gpon %s", ponPort),
			fmt.Sprintf("no onu disable %d", onuID),
			"exit",
			"commit",
			"end",
		}
	} else {
		commands = []string{
			"config",
			fmt.Sprintf("interface epon %s", ponPort),
			fmt.Sprintf("no llid disable %d", onuID),
			"exit",
			"commit",
			"end",
		}
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	return err
}

func (a *Adapter) GetSubscriberStatus(ctx context.Context, subscriberID string) (*types.SubscriberStatus, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	ponPort, onuID := a.parseSubscriberID(subscriberID)

	// V-SOL CLI command to get ONU info
	var cmd string
	if a.detectPONType() == "gpon" {
		cmd = fmt.Sprintf("show onu-info gpon %s %d", ponPort, onuID)
	} else {
		cmd = fmt.Sprintf("show llid-info epon %s %d", ponPort, onuID)
	}

	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get ONU status: %w", err)
	}

	// Parse CLI output
	status := a.parseONUStatus(output, subscriberID)

	return status, nil
}

func (a *Adapter) GetSubscriberStats(ctx context.Context, subscriberID string) (*types.SubscriberStats, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	ponPort, onuID := a.parseSubscriberID(subscriberID)

	// V-SOL CLI command to get ONU statistics
	var cmd string
	if a.detectPONType() == "gpon" {
		cmd = fmt.Sprintf("show onu statistics gpon %s %d", ponPort, onuID)
	} else {
		cmd = fmt.Sprintf("show llid statistics epon %s %d", ponPort, onuID)
	}

	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get ONU stats: %w", err)
	}

	// Parse CLI output
	stats := a.parseONUStats(output)

	return stats, nil
}

func (a *Adapter) HealthCheck(ctx context.Context) error {
	if a.cliExecutor == nil {
		return a.baseDriver.HealthCheck(ctx)
	}

	// V-SOL health check: show system info
	_, err := a.cliExecutor.ExecCommand(ctx, "show system")
	return err
}

// ============================================================================
// DriverV2 Interface Implementation
// ============================================================================

// DiscoverONUs discovers unprovisioned ONUs on specified PON ports (DriverV2)
// Note: V1600 series does not support autofind commands. Instead, we return all
// currently registered ONUs - the caller can filter out already-provisioned ones.
func (a *Adapter) DiscoverONUs(ctx context.Context, ponPorts []string) ([]types.ONUDiscovery, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	var discoveries []types.ONUDiscovery

	// V1600 series doesn't have autofind - get registered ONUs instead
	// Use the same V1600-style command sequence as GetONUList
	if a.detectPONType() == "gpon" {
		// Determine which ports to scan
		portsToScan := ponPorts
		if len(portsToScan) == 0 {
			portsToScan = a.getPONPortList()
		}

		for _, ponPort := range portsToScan {
			commands := []string{
				"configure terminal",
				fmt.Sprintf("interface gpon %s", ponPort),
				"show onu info",
				"exit",
				"exit",
			}

			outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
			if err != nil {
				// Try legacy autofind command as fallback (for older V-SOL models)
				cmd := "show onu autofind all"
				output, legacyErr := a.cliExecutor.ExecCommand(ctx, cmd)
				if legacyErr != nil {
					// If both fail, return empty list (no discovery available on this model)
					return []types.ONUDiscovery{}, nil
				}
				discoveries = append(discoveries, a.parseAutofindOutput(output)...)
				break // Legacy command gets all ports at once
			}

			// Parse the "show onu info" output (index 2 in the commands)
			if len(outputs) > 2 {
				onus := a.parseV1600ONUList(outputs[2], ponPort)
				for _, onu := range onus {
					discovery := types.ONUDiscovery{
						PONPort:      onu.PONPort,
						Serial:       onu.Serial,
						Model:        onu.Model,
						RxPowerDBm:   onu.RxPowerDBm,
						DistanceM:    onu.DistanceM,
						DiscoveredAt: time.Now(),
					}
					discoveries = append(discoveries, discovery)
				}
			}
		}
	} else {
		// EPON: try autofind first, fall back to registered LLIDs
		cmd := "show llid autofind all"
		output, err := a.cliExecutor.ExecCommand(ctx, cmd)
		if err != nil {
			// Try getting all registered LLIDs
			cmd = "show llid all"
			output, err = a.cliExecutor.ExecCommand(ctx, cmd)
			if err != nil {
				return []types.ONUDiscovery{}, nil
			}
			// Parse registered LLIDs as discoveries
			onus := a.parseONUList(output)
			for _, onu := range onus {
				discovery := types.ONUDiscovery{
					PONPort:      onu.PONPort,
					Serial:       onu.Serial,
					MAC:          onu.MAC,
					Model:        onu.Model,
					RxPowerDBm:   onu.RxPowerDBm,
					DistanceM:    onu.DistanceM,
					DiscoveredAt: time.Now(),
				}
				discoveries = append(discoveries, discovery)
			}
		} else {
			discoveries = a.parseAutofindOutput(output)
		}
	}

	// Filter by requested PON ports if specified
	if len(ponPorts) > 0 {
		portSet := make(map[string]bool)
		for _, p := range ponPorts {
			portSet[p] = true
		}

		var filtered []types.ONUDiscovery
		for _, d := range discoveries {
			if portSet[d.PONPort] {
				filtered = append(filtered, d)
			}
		}
		return filtered, nil
	}

	return discoveries, nil
}

// GetONUList returns all provisioned ONUs matching the filter (DriverV2)
func (a *Adapter) GetONUList(ctx context.Context, filter *types.ONUFilter) ([]types.ONUInfo, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	var allOnus []types.ONUInfo

	// V-SOL V1600 series requires entering config mode and iterating PON ports
	// Command sequence: configure terminal -> interface gpon X/Y -> show onu info
	if a.detectPONType() == "gpon" {
		// Try V1600 style first (configure terminal -> interface gpon -> show onu info)
		ponPorts := a.getPONPortList()
		for _, ponPort := range ponPorts {
			commands := []string{
				"configure terminal",
				fmt.Sprintf("interface gpon %s", ponPort),
				"show onu info",
				"exit",
				"exit",
			}

			outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
			if err != nil {
				// If V1600 style fails, try legacy command
				cmd := "show onu all"
				output, legacyErr := a.cliExecutor.ExecCommand(ctx, cmd)
				if legacyErr != nil {
					return nil, fmt.Errorf("failed to get ONU list: %w", legacyErr)
				}
				onus := a.parseONUList(output)
				if filter != nil {
					onus = a.filterONUList(onus, filter)
				}
				return onus, nil
			}

			// Parse the "show onu info" output (index 2 in the commands)
			if len(outputs) > 2 {
				onus := a.parseV1600ONUList(outputs[2], ponPort)
				allOnus = append(allOnus, onus...)
			}
		}
	} else {
		// EPON: use legacy command
		cmd := "show llid all"
		output, err := a.cliExecutor.ExecCommand(ctx, cmd)
		if err != nil {
			return nil, fmt.Errorf("failed to get ONU list: %w", err)
		}
		allOnus = a.parseONUList(output)
	}

	// Apply filters
	if filter != nil {
		allOnus = a.filterONUList(allOnus, filter)
	}

	return allOnus, nil
}

// getPONPortList returns the list of PON ports to scan
func (a *Adapter) getPONPortList() []string {
	// Default PON ports for V1600 series (8 or 16 ports typically)
	// Start with common ports, can be expanded based on model detection
	return []string{"0/1", "0/2", "0/3", "0/4", "0/5", "0/6", "0/7", "0/8"}
}

// parseV1600ONUList parses the V1600 series "show onu info" output format
// Example output:
// Onuindex   Model                Profile                Mode    AuthInfo
// ----------------------------------------------------------------------------
// GPON0/1:1  unknown              AN5506-04-F1           sn      FHTT5929E410
// GPON0/1:2  HG6143D              AN5506-04-F1           sn      FHTT59CB8310
func (a *Adapter) parseV1600ONUList(output string, ponPort string) []types.ONUInfo {
	onus := []types.ONUInfo{}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Onuindex") || strings.HasPrefix(line, "-") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 5 {
			// Parse Onuindex field (e.g., "GPON0/1:1")
			onuIndex := fields[0]

			// Extract PON port and ONU ID from index
			var extractedPort string
			var onuID int

			// Try to parse format like "GPON0/1:1" or "0/1:1"
			re := regexp.MustCompile(`(?:GPON)?(\d+/\d+):(\d+)`)
			if matches := re.FindStringSubmatch(onuIndex); len(matches) == 3 {
				extractedPort = matches[1]
				onuID, _ = strconv.Atoi(matches[2])
			}

			onu := types.ONUInfo{
				PONPort:     extractedPort,
				ONUID:       onuID,
				Model:       fields[1],
				LineProfile: fields[2],
				Serial:      fields[4], // AuthInfo field contains serial
				IsOnline:    true,      // If it appears in list, it's online
				AdminState:  "enabled",
				OperState:   "online",
			}

			// Mode field (fields[3]) indicates auth type (sn = serial number)
			if len(fields) >= 4 && fields[3] == "sn" {
				onu.Metadata = map[string]interface{}{
					"auth_mode": "serial",
				}
			}

			onus = append(onus, onu)
		}
	}

	return onus
}

// GetONUBySerial finds a specific ONU by serial number (DriverV2)
func (a *Adapter) GetONUBySerial(ctx context.Context, serial string) (*types.ONUInfo, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	// V-SOL CLI command to search for ONU by serial
	var cmd string
	if a.detectPONType() == "gpon" {
		cmd = fmt.Sprintf("show onu sn %s", serial)
	} else {
		cmd = fmt.Sprintf("show llid sn %s", serial)
	}

	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get ONU by serial: %w", err)
	}

	// Check if ONU was found
	if strings.Contains(strings.ToLower(output), "not found") ||
		strings.Contains(strings.ToLower(output), "no onu") {
		return nil, nil
	}

	// Parse ONU info
	onu := a.parseONUInfo(output, serial)
	return onu, nil
}

// GetPONPower returns optical power readings for a PON port (DriverV2)
func (a *Adapter) GetPONPower(ctx context.Context, ponPort string) (*types.PONPowerReading, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	// V-SOL CLI command to get PON port optical info
	var cmd string
	if a.detectPONType() == "gpon" {
		cmd = fmt.Sprintf("show pon optical gpon %s", ponPort)
	} else {
		cmd = fmt.Sprintf("show pon optical epon %s", ponPort)
	}

	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get PON power: %w", err)
	}

	reading := &types.PONPowerReading{
		PONPort:   ponPort,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Parse Tx power
	txRe := regexp.MustCompile(`tx[_\s]*power[:\s]+(-?\d+\.?\d*)`)
	if match := txRe.FindStringSubmatch(strings.ToLower(output)); len(match) > 1 {
		if tx, err := strconv.ParseFloat(match[1], 64); err == nil {
			reading.TxPowerDBm = tx
		}
	}

	// Parse Rx power (aggregate)
	rxRe := regexp.MustCompile(`rx[_\s]*power[:\s]+(-?\d+\.?\d*)`)
	if match := rxRe.FindStringSubmatch(strings.ToLower(output)); len(match) > 1 {
		if rx, err := strconv.ParseFloat(match[1], 64); err == nil {
			reading.RxPowerDBm = rx
		}
	}

	// Parse temperature
	tempRe := regexp.MustCompile(`temp[:\s]+(-?\d+\.?\d*)`)
	if match := tempRe.FindStringSubmatch(strings.ToLower(output)); len(match) > 1 {
		if temp, err := strconv.ParseFloat(match[1], 64); err == nil {
			reading.Temperature = temp
		}
	}

	reading.Metadata["cli_output"] = output

	return reading, nil
}

// GetONUPower returns optical power readings for a specific ONU (DriverV2)
func (a *Adapter) GetONUPower(ctx context.Context, ponPort string, onuID int) (*types.ONUPowerReading, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	// V-SOL CLI command to get ONU optical info
	var cmd string
	if a.detectPONType() == "gpon" {
		cmd = fmt.Sprintf("show onu optical gpon %s %d", ponPort, onuID)
	} else {
		cmd = fmt.Sprintf("show llid optical epon %s %d", ponPort, onuID)
	}

	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get ONU power: %w", err)
	}

	reading := &types.ONUPowerReading{
		PONPort:   ponPort,
		ONUID:     onuID,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	outputLower := strings.ToLower(output)

	// Parse ONU Tx power (what ONU sends)
	txRe := regexp.MustCompile(`(?:onu[_\s]*)?tx[_\s]*power[:\s]+(-?\d+\.?\d*)`)
	if match := txRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if tx, err := strconv.ParseFloat(match[1], 64); err == nil {
			reading.TxPowerDBm = tx
		}
	}

	// Parse ONU Rx power (what ONU sees from OLT)
	rxRe := regexp.MustCompile(`(?:onu[_\s]*)?rx[_\s]*power[:\s]+(-?\d+\.?\d*)`)
	if match := rxRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if rx, err := strconv.ParseFloat(match[1], 64); err == nil {
			reading.RxPowerDBm = rx
		}
	}

	// Parse OLT Rx power (what OLT receives from this ONU)
	oltRxRe := regexp.MustCompile(`olt[_\s]*rx[:\s]+(-?\d+\.?\d*)`)
	if match := oltRxRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if oltRx, err := strconv.ParseFloat(match[1], 64); err == nil {
			reading.OLTRxDBm = oltRx
		}
	}

	// Parse distance
	distRe := regexp.MustCompile(`distance[:\s]+(\d+)`)
	if match := distRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if dist, err := strconv.Atoi(match[1]); err == nil {
			reading.DistanceM = dist
		}
	}

	// Set thresholds
	reading.TxHighThreshold = types.GPONTxHighThreshold
	reading.TxLowThreshold = types.GPONTxLowThreshold
	reading.RxHighThreshold = types.GPONRxHighThreshold
	reading.RxLowThreshold = types.GPONRxLowThreshold

	// Check if within spec
	reading.IsWithinSpec = types.IsPowerWithinSpec(reading.RxPowerDBm, reading.TxPowerDBm)

	reading.Metadata["cli_output"] = output

	return reading, nil
}

// GetONUDistance returns estimated fiber distance to ONU in meters (DriverV2)
func (a *Adapter) GetONUDistance(ctx context.Context, ponPort string, onuID int) (int, error) {
	power, err := a.GetONUPower(ctx, ponPort, onuID)
	if err != nil {
		return -1, err
	}
	if power.DistanceM == 0 {
		return -1, nil
	}
	return power.DistanceM, nil
}

// RestartONU triggers a reboot of the specified ONU (DriverV2)
func (a *Adapter) RestartONU(ctx context.Context, ponPort string, onuID int) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	var commands []string
	if a.detectPONType() == "gpon" {
		commands = []string{
			"config",
			fmt.Sprintf("interface gpon %s", ponPort),
			fmt.Sprintf("onu reboot %d", onuID),
			"exit",
			"end",
		}
	} else {
		commands = []string{
			"config",
			fmt.Sprintf("interface epon %s", ponPort),
			fmt.Sprintf("llid reboot %d", onuID),
			"exit",
			"end",
		}
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	return err
}

// ApplyProfile applies a bandwidth/service profile to an ONU (DriverV2)
func (a *Adapter) ApplyProfile(ctx context.Context, ponPort string, onuID int, profile *types.ONUProfile) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	var commands []string
	if a.detectPONType() == "gpon" {
		commands = []string{
			"config",
			fmt.Sprintf("interface gpon %s", ponPort),
		}

		// Update profile if specified
		if profile.LineProfile != "" || profile.ServiceProfile != "" {
			lineProfile := profile.LineProfile
			serviceProfile := profile.ServiceProfile
			if lineProfile == "" {
				lineProfile = fmt.Sprintf("line-%d-%d", profile.BandwidthDown/1000, profile.BandwidthUp/1000)
			}
			if serviceProfile == "" {
				serviceProfile = "service-internet"
			}
			commands = append(commands, fmt.Sprintf("onu profile %d line-profile %s service-profile %s", onuID, lineProfile, serviceProfile))
		}

		// Update VLAN if specified
		if profile.VLAN > 0 {
			commands = append(commands, fmt.Sprintf("onu vlan %d user-vlan %d priority %d", onuID, profile.VLAN, profile.Priority))
		}

		// Update bandwidth
		if profile.BandwidthUp > 0 || profile.BandwidthDown > 0 {
			commands = append(commands, fmt.Sprintf("onu flowctrl %d ingress %d egress %d", onuID, profile.BandwidthUp, profile.BandwidthDown))
		}

		commands = append(commands, "exit", "commit", "end")
	} else {
		commands = []string{
			"config",
			fmt.Sprintf("interface epon %s", ponPort),
		}

		if profile.LineProfile != "" || profile.ServiceProfile != "" {
			lineProfile := profile.LineProfile
			serviceProfile := profile.ServiceProfile
			if lineProfile == "" {
				lineProfile = fmt.Sprintf("line-%d-%d", profile.BandwidthDown/1000, profile.BandwidthUp/1000)
			}
			if serviceProfile == "" {
				serviceProfile = "service-internet"
			}
			commands = append(commands, fmt.Sprintf("llid profile %d line-profile %s service-profile %s", onuID, lineProfile, serviceProfile))
		}

		if profile.VLAN > 0 {
			commands = append(commands, fmt.Sprintf("llid vlan %d user-vlan %d", onuID, profile.VLAN))
		}

		if profile.BandwidthUp > 0 || profile.BandwidthDown > 0 {
			commands = append(commands, fmt.Sprintf("llid flowctrl %d ingress %d egress %d", onuID, profile.BandwidthUp, profile.BandwidthDown))
		}

		commands = append(commands, "exit", "commit", "end")
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	return err
}

// BulkProvision provisions multiple ONUs in a single session (DriverV2)
func (a *Adapter) BulkProvision(ctx context.Context, operations []types.BulkProvisionOp) (*types.BulkResult, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	result := &types.BulkResult{
		Results: make([]types.BulkOpResult, len(operations)),
	}

	// Enter config mode once
	if _, err := a.cliExecutor.ExecCommand(ctx, "config"); err != nil {
		return nil, fmt.Errorf("failed to enter config mode: %w", err)
	}
	defer func() { _, _ = a.cliExecutor.ExecCommand(ctx, "end") }()

	for i, op := range operations {
		opResult := types.BulkOpResult{
			Serial:  op.Serial,
			PONPort: op.PONPort,
			ONUID:   op.ONUID,
		}

		// Build commands for this ONU
		var commands []string
		if a.detectPONType() == "gpon" {
			commands = []string{
				fmt.Sprintf("interface gpon %s", op.PONPort),
				fmt.Sprintf("onu %d type router sn %s", op.ONUID, op.Serial),
			}

			if op.Profile != nil {
				lineProfile := op.Profile.LineProfile
				serviceProfile := op.Profile.ServiceProfile
				if lineProfile == "" {
					lineProfile = fmt.Sprintf("line-%d-%d", op.Profile.BandwidthDown/1000, op.Profile.BandwidthUp/1000)
				}
				if serviceProfile == "" {
					serviceProfile = "service-internet"
				}
				commands = append(commands, fmt.Sprintf("onu profile %d line-profile %s service-profile %s", op.ONUID, lineProfile, serviceProfile))

				if op.Profile.VLAN > 0 {
					commands = append(commands, fmt.Sprintf("onu vlan %d user-vlan %d priority %d", op.ONUID, op.Profile.VLAN, op.Profile.Priority))
				}

				if op.Profile.BandwidthUp > 0 || op.Profile.BandwidthDown > 0 {
					commands = append(commands, fmt.Sprintf("onu flowctrl %d ingress %d egress %d", op.ONUID, op.Profile.BandwidthUp, op.Profile.BandwidthDown))
				}
			}

			commands = append(commands, fmt.Sprintf("no onu disable %d", op.ONUID), "exit")
		}

		// Execute commands for this ONU
		outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
		if err != nil {
			opResult.Success = false
			opResult.Error = err.Error()
			result.Failed++
		} else {
			// Check outputs for errors
			hasError := false
			for _, out := range outputs {
				outLower := strings.ToLower(out)
				if strings.Contains(outLower, "error") || strings.Contains(outLower, "fail") || strings.Contains(outLower, "invalid") {
					hasError = true
					opResult.Error = out
					break
				}
			}

			if hasError {
				opResult.Success = false
				result.Failed++
			} else {
				opResult.Success = true
				result.Succeeded++
			}
		}

		result.Results[i] = opResult
	}

	// Commit all changes (ignore error - some operations may have succeeded)
	_, _ = a.cliExecutor.ExecCommand(ctx, "commit")

	return result, nil
}

// RunDiagnostics performs comprehensive diagnostics on an ONU (DriverV2)
func (a *Adapter) RunDiagnostics(ctx context.Context, ponPort string, onuID int) (*types.ONUDiagnostics, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	diag := &types.ONUDiagnostics{
		PONPort:    ponPort,
		ONUID:      onuID,
		Timestamp:  time.Now(),
		VendorData: make(map[string]interface{}),
	}

	// Get optical power readings
	power, err := a.GetONUPower(ctx, ponPort, onuID)
	if err == nil {
		diag.Power = power
		diag.Serial = power.Serial
	}

	// Get ONU status
	subscriberID := fmt.Sprintf("onu-%s-%d", ponPort, onuID)
	status, err := a.GetSubscriberStatus(ctx, subscriberID)
	if err == nil {
		diag.AdminState = status.State
		diag.OperState = status.State
		if status.IsOnline {
			diag.OperState = "online"
		}
	}

	// Get ONU statistics
	stats, err := a.GetSubscriberStats(ctx, subscriberID)
	if err == nil {
		diag.BytesUp = stats.BytesUp
		diag.BytesDown = stats.BytesDown
		diag.Errors = stats.ErrorsUp + stats.ErrorsDown
		diag.Drops = stats.Drops
	}

	// Get configuration info
	var cmd string
	if a.detectPONType() == "gpon" {
		cmd = fmt.Sprintf("show onu config gpon %s %d", ponPort, onuID)
	} else {
		cmd = fmt.Sprintf("show llid config epon %s %d", ponPort, onuID)
	}

	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err == nil {
		// Parse configuration
		outputLower := strings.ToLower(output)

		// Line profile
		lineRe := regexp.MustCompile(`line[_\s]*profile[:\s]+(\S+)`)
		if match := lineRe.FindStringSubmatch(outputLower); len(match) > 1 {
			diag.LineProfile = match[1]
		}

		// Service profile
		serviceRe := regexp.MustCompile(`service[_\s]*profile[:\s]+(\S+)`)
		if match := serviceRe.FindStringSubmatch(outputLower); len(match) > 1 {
			diag.ServiceProfile = match[1]
		}

		// VLAN
		vlanRe := regexp.MustCompile(`vlan[:\s]+(\d+)`)
		if match := vlanRe.FindStringSubmatch(outputLower); len(match) > 1 {
			if vlan, err := strconv.Atoi(match[1]); err == nil {
				diag.VLAN = vlan
			}
		}

		// Bandwidth
		bwUpRe := regexp.MustCompile(`(?:upstream|ingress)[:\s]+(\d+)`)
		if match := bwUpRe.FindStringSubmatch(outputLower); len(match) > 1 {
			if bw, err := strconv.Atoi(match[1]); err == nil {
				diag.BandwidthUp = bw
			}
		}

		bwDownRe := regexp.MustCompile(`(?:downstream|egress)[:\s]+(\d+)`)
		if match := bwDownRe.FindStringSubmatch(outputLower); len(match) > 1 {
			if bw, err := strconv.Atoi(match[1]); err == nil {
				diag.BandwidthDown = bw
			}
		}

		diag.VendorData["config_output"] = output
	}

	return diag, nil
}

// GetAlarms returns active alarms from the OLT (DriverV2)
func (a *Adapter) GetAlarms(ctx context.Context) ([]types.OLTAlarm, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	output, err := a.cliExecutor.ExecCommand(ctx, "show alarm active")
	if err != nil {
		return nil, fmt.Errorf("failed to get alarms: %w", err)
	}

	return a.parseAlarms(output), nil
}

// GetOLTStatus returns comprehensive OLT status (DriverV2)
func (a *Adapter) GetOLTStatus(ctx context.Context) (*types.OLTStatus, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	status := &types.OLTStatus{
		Vendor:      "vsol",
		Model:       a.detectModel(),
		IsReachable: true,
		IsHealthy:   true,
		LastPoll:    time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	// Get version info (serial, firmware)
	versionOutput, err := a.cliExecutor.ExecCommand(ctx, "show version")
	if err != nil {
		status.IsReachable = false
		return status, nil
	}

	// Parse serial number: "Olt Serial Number:           V2104230071"
	snRe := regexp.MustCompile(`(?i)olt serial number[:\s]+(\S+)`)
	if match := snRe.FindStringSubmatch(versionOutput); len(match) > 1 {
		status.SerialNumber = match[1]
	}

	// Parse firmware version: "Software Version:            V2.1.6R"
	fwRe := regexp.MustCompile(`(?i)software version[:\s]+(\S+)`)
	if match := fwRe.FindStringSubmatch(versionOutput); len(match) > 1 {
		status.Firmware = match[1]
	}

	// Get CPU usage: "show sys cpu-usage"
	// Output has %idle column, CPU usage = 100 - idle
	cpuOutput, err := a.cliExecutor.ExecCommand(ctx, "show sys cpu-usage")
	if err != nil {
		status.Metadata["cpu_error"] = err.Error()
	} else {
		status.Metadata["cpu_output"] = cpuOutput
		// Parse the "Average:" line for %idle (last column)
		// "Average:     all    1.75    0.00    1.05    0.00    0.00   22.53    0.00    0.00   74.68"
		idleRe := regexp.MustCompile(`(?i)average:\s+all\s+[\d.]+\s+[\d.]+\s+[\d.]+\s+[\d.]+\s+[\d.]+\s+[\d.]+\s+[\d.]+\s+[\d.]+\s+([\d.]+)`)
		if match := idleRe.FindStringSubmatch(cpuOutput); len(match) > 1 {
			if idle, err := strconv.ParseFloat(match[1], 64); err == nil {
				status.CPUPercent = 100 - idle
			}
		} else {
			status.Metadata["cpu_parse_fail"] = "regex did not match"
		}
	}

	// Get memory usage: "show sys mem"
	// MemTotal and MemFree in kB
	memOutput, err := a.cliExecutor.ExecCommand(ctx, "show sys mem")
	if err != nil {
		status.Metadata["mem_error"] = err.Error()
	} else {
		status.Metadata["mem_output"] = memOutput
		var memTotal, memFree float64
		totalRe := regexp.MustCompile(`(?i)memtotal[:\s]+(\d+)`)
		freeRe := regexp.MustCompile(`(?i)memfree[:\s]+(\d+)`)
		if match := totalRe.FindStringSubmatch(memOutput); len(match) > 1 {
			memTotal, _ = strconv.ParseFloat(match[1], 64)
		}
		if match := freeRe.FindStringSubmatch(memOutput); len(match) > 1 {
			memFree, _ = strconv.ParseFloat(match[1], 64)
		}
		if memTotal > 0 {
			status.MemoryPercent = (memTotal - memFree) / memTotal * 100
		} else {
			status.Metadata["mem_parse_fail"] = "memTotal is 0"
		}
	}

	// Get uptime: "show sys running-time"
	// "Syetem Running Time:         6 Days 3 Hours 5 Minutes 31 Seconds"
	uptimeOutput, err := a.cliExecutor.ExecCommand(ctx, "show sys running-time")
	if err == nil {
		var totalSeconds int64
		daysRe := regexp.MustCompile(`(\d+)\s*days?`)
		hoursRe := regexp.MustCompile(`(\d+)\s*hours?`)
		minsRe := regexp.MustCompile(`(\d+)\s*minutes?`)
		secsRe := regexp.MustCompile(`(\d+)\s*seconds?`)
		uptimeLower := strings.ToLower(uptimeOutput)
		if match := daysRe.FindStringSubmatch(uptimeLower); len(match) > 1 {
			days, _ := strconv.ParseInt(match[1], 10, 64)
			totalSeconds += days * 86400
		}
		if match := hoursRe.FindStringSubmatch(uptimeLower); len(match) > 1 {
			hours, _ := strconv.ParseInt(match[1], 10, 64)
			totalSeconds += hours * 3600
		}
		if match := minsRe.FindStringSubmatch(uptimeLower); len(match) > 1 {
			mins, _ := strconv.ParseInt(match[1], 10, 64)
			totalSeconds += mins * 60
		}
		if match := secsRe.FindStringSubmatch(uptimeLower); len(match) > 1 {
			secs, _ := strconv.ParseInt(match[1], 10, 64)
			totalSeconds += secs
		}
		status.UptimeSeconds = totalSeconds
	}

	// Get PON port status
	portOutput, err := a.cliExecutor.ExecCommand(ctx, "show pon status all")
	if err == nil {
		status.PONPorts = a.parsePONPortStatus(portOutput)
	}

	// Count ONUs
	for _, port := range status.PONPorts {
		status.TotalONUs += port.ONUCount
		if port.OperState == "up" {
			status.ActiveONUs += port.ONUCount
		}
	}

	status.Metadata["version_output"] = versionOutput

	return status, nil
}

// parseAutofindOutput parses V-SOL autofind CLI output
func (a *Adapter) parseAutofindOutput(output string) []types.ONUDiscovery {
	discoveries := []types.ONUDiscovery{}

	// V-SOL autofind output format (example):
	// Port  Serial          MAC              Model         Distance  Rx Power
	// 0/1   VSOL12345678    AA:BB:CC:DD:EE   V2802GWT      1234m     -18.5dBm
	// 0/2   VSOL87654321    11:22:33:44:55   V2802RGW      567m      -22.1dBm

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Port") || strings.HasPrefix(line, "-") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 4 {
			discovery := types.ONUDiscovery{
				PONPort:      fields[0],
				Serial:       fields[1],
				DiscoveredAt: time.Now(),
			}

			if len(fields) >= 3 {
				discovery.MAC = fields[2]
			}
			if len(fields) >= 4 {
				discovery.Model = fields[3]
			}
			if len(fields) >= 5 {
				// Parse distance (e.g., "1234m")
				distStr := strings.TrimSuffix(fields[4], "m")
				if dist, err := strconv.Atoi(distStr); err == nil {
					discovery.DistanceM = dist
				}
			}
			if len(fields) >= 6 {
				// Parse Rx power (e.g., "-18.5dBm")
				rxStr := strings.TrimSuffix(fields[5], "dBm")
				if rx, err := strconv.ParseFloat(rxStr, 64); err == nil {
					discovery.RxPowerDBm = rx
				}
			}

			discoveries = append(discoveries, discovery)
		}
	}

	return discoveries
}

// parseONUStatus parses V-SOL ONU status CLI output
func (a *Adapter) parseONUStatus(output string, subscriberID string) *types.SubscriberStatus {
	status := &types.SubscriberStatus{
		SubscriberID: subscriberID,
		State:        "unknown",
		IsOnline:     false,
		LastActivity: time.Now(),
		Metadata:     make(map[string]interface{}),
	}

	// Parse state from output
	outputLower := strings.ToLower(output)
	if strings.Contains(outputLower, "online") || strings.Contains(outputLower, "active") {
		status.State = "online"
		status.IsOnline = true
	} else if strings.Contains(outputLower, "offline") || strings.Contains(outputLower, "inactive") {
		status.State = "offline"
		status.IsOnline = false
	} else if strings.Contains(outputLower, "disabled") {
		status.State = "suspended"
		status.IsOnline = false
	}

	// Parse uptime if present
	uptimeRe := regexp.MustCompile(`uptime[:\s]+(\d+)`)
	if match := uptimeRe.FindStringSubmatch(output); len(match) > 1 {
		if uptime, err := strconv.ParseInt(match[1], 10, 64); err == nil {
			status.UptimeSeconds = uptime
		}
	}

	// Parse optical power
	rxPowerRe := regexp.MustCompile(`rx[:\s]+(-?\d+\.?\d*).*dbm`)
	if match := rxPowerRe.FindStringSubmatch(outputLower); len(match) > 1 {
		status.Metadata["rx_power_dbm"] = match[1]
	}

	txPowerRe := regexp.MustCompile(`tx[:\s]+(-?\d+\.?\d*).*dbm`)
	if match := txPowerRe.FindStringSubmatch(outputLower); len(match) > 1 {
		status.Metadata["tx_power_dbm"] = match[1]
	}

	// Store raw output
	status.Metadata["cli_output"] = output

	return status
}

// parseONUStats parses V-SOL ONU statistics CLI output
func (a *Adapter) parseONUStats(output string) *types.SubscriberStats {
	stats := &types.SubscriberStats{
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Parse bytes/packets from output
	// V-SOL format varies, common patterns:
	// Rx Bytes: 123456789
	// Tx Bytes: 987654321

	rxBytesRe := regexp.MustCompile(`rx\s*bytes[:\s]+(\d+)`)
	if match := rxBytesRe.FindStringSubmatch(strings.ToLower(output)); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.BytesDown = val
		}
	}

	txBytesRe := regexp.MustCompile(`tx\s*bytes[:\s]+(\d+)`)
	if match := txBytesRe.FindStringSubmatch(strings.ToLower(output)); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.BytesUp = val
		}
	}

	rxPacketsRe := regexp.MustCompile(`rx\s*packets[:\s]+(\d+)`)
	if match := rxPacketsRe.FindStringSubmatch(strings.ToLower(output)); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.PacketsDown = val
		}
	}

	txPacketsRe := regexp.MustCompile(`tx\s*packets[:\s]+(\d+)`)
	if match := txPacketsRe.FindStringSubmatch(strings.ToLower(output)); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.PacketsUp = val
		}
	}

	// Parse errors
	errorsRe := regexp.MustCompile(`errors[:\s]+(\d+)`)
	if match := errorsRe.FindStringSubmatch(strings.ToLower(output)); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.ErrorsDown = val
		}
	}

	dropsRe := regexp.MustCompile(`drops[:\s]+(\d+)`)
	if match := dropsRe.FindStringSubmatch(strings.ToLower(output)); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.Drops = val
		}
	}

	stats.Metadata["cli_output"] = output

	return stats
}

// Helper methods

// detectModel determines the V-SOL OLT model
func (a *Adapter) detectModel() string {
	if model, ok := a.config.Metadata["model"]; ok {
		return model
	}
	return "v1600g"
}

// detectPONType determines if this is GPON or EPON
func (a *Adapter) detectPONType() string {
	if ponType, ok := a.config.Metadata["pon_type"]; ok {
		return ponType
	}
	return "gpon"
}

// getPONPort extracts PON port from subscriber metadata
func (a *Adapter) getPONPort(subscriber *model.Subscriber) string {
	// Check subscriber annotations for PON port
	if subscriber.Annotations != nil {
		if port, ok := subscriber.Annotations["nanoncore.com/pon-port"]; ok {
			return port
		}
	}
	// Default to first port
	return "0/1"
}

// getONUID extracts or generates ONU ID
func (a *Adapter) getONUID(subscriber *model.Subscriber) int {
	// Check subscriber annotations for ONU ID
	if subscriber.Annotations != nil {
		if idStr, ok := subscriber.Annotations["nanoncore.com/onu-id"]; ok {
			if id, err := strconv.Atoi(idStr); err == nil {
				return id
			}
		}
	}
	// Generate from VLAN as fallback
	return subscriber.Spec.VLAN % 128
}

// getLineProfile returns the line profile name for a service tier
func (a *Adapter) getLineProfile(tier *model.ServiceTier) string {
	// Check tier annotations for custom profile
	if tier.Annotations != nil {
		if profile, ok := tier.Annotations["nanoncore.com/line-profile"]; ok {
			return profile
		}
	}
	// Generate based on bandwidth
	return fmt.Sprintf("line-%dM-%dM", tier.Spec.BandwidthDown, tier.Spec.BandwidthUp)
}

// getServiceProfile returns the service profile name for a service tier
func (a *Adapter) getServiceProfile(tier *model.ServiceTier) string {
	// Check tier annotations for custom profile
	if tier.Annotations != nil {
		if profile, ok := tier.Annotations["nanoncore.com/service-profile"]; ok {
			return profile
		}
	}
	// Default service profile
	return "service-internet"
}

// parseSubscriberID parses a subscriber ID to extract PON port and ONU ID
func (a *Adapter) parseSubscriberID(subscriberID string) (string, int) {
	// Expected format: "onu-0/1-5" or just subscriber name
	// Try to parse from ID first
	re := regexp.MustCompile(`onu-(\d+/\d+)-(\d+)`)
	if match := re.FindStringSubmatch(subscriberID); len(match) == 3 {
		if onuID, err := strconv.Atoi(match[2]); err == nil {
			return match[1], onuID
		}
	}

	// Fallback: use default port and hash of ID
	hash := 0
	for _, c := range subscriberID {
		hash = (hash*31 + int(c)) % 128
	}
	return "0/1", hash
}

// parseONUList parses V-SOL "show onu all" CLI output
func (a *Adapter) parseONUList(output string) []types.ONUInfo {
	onus := []types.ONUInfo{}

	// V-SOL ONU list output format (example):
	// Port  ID   Serial          Status   Rx Power  Distance  Profile
	// 0/1   1    VSOL12345678    Online   -18.5     1234      line-100M
	// 0/1   2    VSOL87654321    Offline  -22.1     567       line-50M

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Port") || strings.HasPrefix(line, "-") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 4 {
			onu := types.ONUInfo{
				PONPort: fields[0],
			}

			if len(fields) >= 2 {
				if id, err := strconv.Atoi(fields[1]); err == nil {
					onu.ONUID = id
				}
			}
			if len(fields) >= 3 {
				onu.Serial = fields[2]
			}
			if len(fields) >= 4 {
				statusLower := strings.ToLower(fields[3])
				onu.OperState = fields[3]
				onu.IsOnline = statusLower == "online" || statusLower == "active"
				if onu.IsOnline {
					onu.AdminState = "enabled"
				} else {
					onu.AdminState = "disabled"
				}
			}
			if len(fields) >= 5 {
				if rx, err := strconv.ParseFloat(fields[4], 64); err == nil {
					onu.RxPowerDBm = rx
				}
			}
			if len(fields) >= 6 {
				if dist, err := strconv.Atoi(fields[5]); err == nil {
					onu.DistanceM = dist
				}
			}
			if len(fields) >= 7 {
				onu.LineProfile = fields[6]
			}

			onus = append(onus, onu)
		}
	}

	return onus
}

// filterONUList filters ONU list based on filter criteria
func (a *Adapter) filterONUList(onus []types.ONUInfo, filter *types.ONUFilter) []types.ONUInfo {
	if filter == nil {
		return onus
	}

	var filtered []types.ONUInfo
	for _, onu := range onus {
		// Filter by PON port
		if filter.PONPort != "" && onu.PONPort != filter.PONPort {
			continue
		}

		// Filter by status
		if filter.Status != "" && filter.Status != "all" {
			if filter.Status == "online" && !onu.IsOnline {
				continue
			}
			if filter.Status == "offline" && onu.IsOnline {
				continue
			}
		}

		// Filter by profile
		if filter.Profile != "" && onu.LineProfile != filter.Profile {
			continue
		}

		// Filter by serial (partial match)
		if filter.Serial != "" && !strings.Contains(strings.ToLower(onu.Serial), strings.ToLower(filter.Serial)) {
			continue
		}

		// Filter by VLAN
		if filter.VLAN > 0 && onu.VLAN != filter.VLAN {
			continue
		}

		filtered = append(filtered, onu)
	}

	return filtered
}

// parseONUInfo parses V-SOL ONU info CLI output for a single ONU
func (a *Adapter) parseONUInfo(output string, serial string) *types.ONUInfo {
	onu := &types.ONUInfo{
		Serial:   serial,
		Metadata: make(map[string]interface{}),
	}

	outputLower := strings.ToLower(output)

	// Parse PON port
	portRe := regexp.MustCompile(`port[:\s]+(\d+/\d+)`)
	if match := portRe.FindStringSubmatch(outputLower); len(match) > 1 {
		onu.PONPort = match[1]
	}

	// Parse ONU ID
	idRe := regexp.MustCompile(`(?:onu[_\s]*)?id[:\s]+(\d+)`)
	if match := idRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if id, err := strconv.Atoi(match[1]); err == nil {
			onu.ONUID = id
		}
	}

	// Parse status
	if strings.Contains(outputLower, "online") || strings.Contains(outputLower, "active") {
		onu.OperState = "online"
		onu.AdminState = "enabled"
		onu.IsOnline = true
	} else if strings.Contains(outputLower, "offline") {
		onu.OperState = "offline"
		onu.AdminState = "enabled"
		onu.IsOnline = false
	} else if strings.Contains(outputLower, "disabled") {
		onu.OperState = "disabled"
		onu.AdminState = "disabled"
		onu.IsOnline = false
	}

	// Parse Rx power
	rxRe := regexp.MustCompile(`rx[_\s]*power[:\s]+(-?\d+\.?\d*)`)
	if match := rxRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if rx, err := strconv.ParseFloat(match[1], 64); err == nil {
			onu.RxPowerDBm = rx
		}
	}

	// Parse Tx power
	txRe := regexp.MustCompile(`tx[_\s]*power[:\s]+(-?\d+\.?\d*)`)
	if match := txRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if tx, err := strconv.ParseFloat(match[1], 64); err == nil {
			onu.TxPowerDBm = tx
		}
	}

	// Parse distance
	distRe := regexp.MustCompile(`distance[:\s]+(\d+)`)
	if match := distRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if dist, err := strconv.Atoi(match[1]); err == nil {
			onu.DistanceM = dist
		}
	}

	// Parse line profile
	lineRe := regexp.MustCompile(`line[_\s]*profile[:\s]+(\S+)`)
	if match := lineRe.FindStringSubmatch(outputLower); len(match) > 1 {
		onu.LineProfile = match[1]
	}

	// Parse service profile
	serviceRe := regexp.MustCompile(`service[_\s]*profile[:\s]+(\S+)`)
	if match := serviceRe.FindStringSubmatch(outputLower); len(match) > 1 {
		onu.ServiceProfile = match[1]
	}

	// Parse VLAN
	vlanRe := regexp.MustCompile(`vlan[:\s]+(\d+)`)
	if match := vlanRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if vlan, err := strconv.Atoi(match[1]); err == nil {
			onu.VLAN = vlan
		}
	}

	onu.Metadata["cli_output"] = output

	return onu
}

// parseAlarms parses V-SOL alarm CLI output
func (a *Adapter) parseAlarms(output string) []types.OLTAlarm {
	alarms := []types.OLTAlarm{}

	// V-SOL alarm output format (example):
	// ID      Severity  Type    Source     Message              Time
	// 1       Critical  LOS     PON 0/1    Loss of signal       2024-01-15 10:30:00
	// 2       Warning   Power   ONU 0/1/5  Rx power low         2024-01-15 10:35:00

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "ID") || strings.HasPrefix(line, "-") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 5 {
			alarm := types.OLTAlarm{
				ID:       fields[0],
				Severity: strings.ToLower(fields[1]),
				Type:     strings.ToLower(fields[2]),
				Source:   strings.ToLower(fields[3]),
				RaisedAt: time.Now(),
				Metadata: make(map[string]interface{}),
			}

			// Reconstruct message from remaining fields (except timestamp at end)
			if len(fields) > 5 {
				// Try to find where message ends and timestamp begins
				msgParts := []string{}
				for i := 4; i < len(fields); i++ {
					// Stop at first field that looks like a date
					if strings.Contains(fields[i], "-") && len(fields[i]) >= 10 {
						// Parse timestamp
						if i+1 < len(fields) {
							dateTimeStr := fields[i] + " " + fields[i+1]
							if t, err := time.Parse("2006-01-02 15:04:05", dateTimeStr); err == nil {
								alarm.RaisedAt = t
							}
						}
						break
					}
					msgParts = append(msgParts, fields[i])
				}
				alarm.Message = strings.Join(msgParts, " ")
			}

			if alarm.Source != "" {
				alarm.SourceID = alarm.Source
			}

			alarms = append(alarms, alarm)
		}
	}

	return alarms
}

// parsePONPortStatus parses V-SOL PON port status CLI output
func (a *Adapter) parsePONPortStatus(output string) []types.PONPortStatus {
	ports := []types.PONPortStatus{}

	// V-SOL PON port status output format (example):
	// Port   Admin    Oper   ONUs   Rx Power   Tx Power
	// 0/1    enabled  up     32     -15.5      3.2
	// 0/2    enabled  down   0      -          3.1

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Port") || strings.HasPrefix(line, "-") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 4 {
			port := types.PONPortStatus{
				Port:     fields[0],
				MaxONUs:  128, // Default max ONUs per port
				Metadata: make(map[string]interface{}),
			}

			if len(fields) >= 2 {
				port.AdminState = strings.ToLower(fields[1])
			}
			if len(fields) >= 3 {
				port.OperState = strings.ToLower(fields[2])
			}
			if len(fields) >= 4 {
				if count, err := strconv.Atoi(fields[3]); err == nil {
					port.ONUCount = count
				}
			}
			if len(fields) >= 5 && fields[4] != "-" {
				if rx, err := strconv.ParseFloat(fields[4], 64); err == nil {
					port.RxPowerDBm = rx
				}
			}
			if len(fields) >= 6 {
				if tx, err := strconv.ParseFloat(fields[5], 64); err == nil {
					port.TxPowerDBm = tx
				}
			}

			ports = append(ports, port)
		}
	}

	return ports
}
