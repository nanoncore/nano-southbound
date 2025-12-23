package cdata

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

// Adapter wraps a base driver with C-Data-specific logic
// C-Data OLTs (FD1104S, FD1208S, FD1616S series) use CLI + SNMP
//
// C-Data CLI quirks:
// 1. Uses "gpon-olt_1/1/1" format for PON ports (slot/frame/port)
// 2. ONU registration via "onu-set" command (not just "onu")
// 3. Config lock can occur - needs "config unlock" first
// 4. Show commands take 5-10 seconds to respond
// 5. Commit sometimes fails silently - always verify after write
// 6. Prompts can vary between firmware versions
type Adapter struct {
	baseDriver  types.Driver
	cliExecutor types.CLIExecutor
	config      *types.EquipmentConfig
}

// NewAdapter creates a new C-Data adapter
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

// CreateSubscriber provisions an ONU on the C-Data OLT
func (a *Adapter) CreateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) (*types.SubscriberResult, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - C-Data requires CLI driver")
	}

	// Parse subscriber info
	ponPort := a.getPONPort(subscriber)
	onuID := a.getONUID(subscriber)
	serial := subscriber.Spec.ONUSerial
	vlan := subscriber.Spec.VLAN

	// Get bandwidth rates
	bandwidthDown := tier.Spec.BandwidthDown // Mbps
	bandwidthUp := tier.Spec.BandwidthUp     // Mbps

	// C-Data CLI command sequence for GPON ONU provisioning
	var commands []string

	if a.detectPONType() == "gpon" {
		commands = a.buildGPONCommands(ponPort, onuID, serial, vlan, bandwidthDown, bandwidthUp, subscriber, tier)
	} else {
		commands = a.buildEPONCommands(ponPort, onuID, serial, vlan, bandwidthDown, bandwidthUp, subscriber, tier)
	}

	// Execute commands
	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return nil, a.translateError(err)
	}

	// Verify the ONU was actually created (C-Data can fail silently)
	if verifyErr := a.verifyONUExists(ctx, ponPort, onuID); verifyErr != nil {
		return nil, fmt.Errorf("C-Data provisioning verification failed: %w", verifyErr)
	}

	// Build result
	result := &types.SubscriberResult{
		SubscriberID:  subscriber.Name,
		SessionID:     fmt.Sprintf("onu-%s-%d", ponPort, onuID),
		AssignedIP:    subscriber.Spec.IPAddress,
		AssignedIPv6:  subscriber.Spec.IPv6Address,
		InterfaceName: fmt.Sprintf("gpon-olt_%s onu %d", ponPort, onuID),
		VLAN:          vlan,
		Metadata: map[string]interface{}{
			"vendor":      "cdata",
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

// buildGPONCommands builds C-Data GPON CLI commands
// Based on C-Data FD1104S/FD1208S CLI Reference Manual
func (a *Adapter) buildGPONCommands(ponPort string, onuID int, serial string, vlan int, bwDown, bwUp int, subscriber *model.Subscriber, tier *model.ServiceTier) []string {
	// C-Data FD series GPON CLI reference
	// Port format: gpon-olt_1/1/1 (slot/frame/port)

	onuType := a.getONUType(subscriber)
	lineProfile := a.getLineProfile(tier)
	serviceProfile := a.getServiceProfile(tier)

	commands := []string{
		// Enter config mode
		"configure terminal",

		// Select GPON OLT interface
		fmt.Sprintf("interface gpon-olt_%s", ponPort),

		// Register ONU with serial number
		// onu-set <onu-id> type <type> sn <serial>
		// Types: router, bridge, hgu, sfu (same as V-SOL)
		fmt.Sprintf("onu-set %d type %s sn %s", onuID, onuType, serial),

		// Assign line profile (T-CONT + GEM port mapping)
		// onu-profile <onu-id> line <name> service <name>
		fmt.Sprintf("onu-profile %d line %s service %s", onuID, lineProfile, serviceProfile),

		// Configure VLAN translation for the ONU
		// onu-vlan <onu-id> mode translate user-vlan <cvlan> svlan <svlan>
		fmt.Sprintf("onu-vlan %d mode translate user-vlan %d svlan %d", onuID, vlan, vlan),

		// Configure bandwidth rate limiting
		// onu-ratelimit <onu-id> upstream <kbps> downstream <kbps>
		fmt.Sprintf("onu-ratelimit %d upstream %d downstream %d",
			onuID,
			bwUp*1000,    // Convert Mbps to kbps
			bwDown*1000), // Convert Mbps to kbps

		// Activate the ONU
		fmt.Sprintf("onu-activate %d", onuID),

		// Exit interface config
		"exit",

		// Commit changes (required on C-Data)
		"commit",

		// Exit config mode
		"end",
	}

	return commands
}

// buildEPONCommands builds C-Data EPON CLI commands
func (a *Adapter) buildEPONCommands(ponPort string, onuID int, mac string, vlan int, bwDown, bwUp int, subscriber *model.Subscriber, tier *model.ServiceTier) []string {
	// C-Data EPON CLI reference

	lineProfile := a.getLineProfile(tier)
	serviceProfile := a.getServiceProfile(tier)

	commands := []string{
		"configure terminal",
		fmt.Sprintf("interface epon-olt_%s", ponPort),

		// Register ONU with MAC address
		// onu-set <onu-id> mac <mac-address>
		fmt.Sprintf("onu-set %d mac %s", onuID, mac),

		// Assign profiles
		fmt.Sprintf("onu-profile %d line %s service %s", onuID, lineProfile, serviceProfile),

		// Configure VLAN
		fmt.Sprintf("onu-vlan %d mode translate user-vlan %d svlan %d", onuID, vlan, vlan),

		// Configure bandwidth
		fmt.Sprintf("onu-ratelimit %d upstream %d downstream %d", onuID, bwUp*1000, bwDown*1000),

		// Activate
		fmt.Sprintf("onu-activate %d", onuID),

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
	lineProfile := a.getLineProfile(tier)
	serviceProfile := a.getServiceProfile(tier)

	var commands []string

	if a.detectPONType() == "gpon" {
		commands = []string{
			"configure terminal",
			fmt.Sprintf("interface gpon-olt_%s", ponPort),
			// Update profiles
			fmt.Sprintf("onu-profile %d line %s service %s", onuID, lineProfile, serviceProfile),
			// Update VLAN
			fmt.Sprintf("onu-vlan %d mode translate user-vlan %d svlan %d", onuID, vlan, vlan),
			// Update bandwidth
			fmt.Sprintf("onu-ratelimit %d upstream %d downstream %d", onuID, bwUp*1000, bwDown*1000),
			"exit",
			"commit",
			"end",
		}
	} else {
		commands = []string{
			"configure terminal",
			fmt.Sprintf("interface epon-olt_%s", ponPort),
			fmt.Sprintf("onu-profile %d line %s service %s", onuID, lineProfile, serviceProfile),
			fmt.Sprintf("onu-vlan %d mode translate user-vlan %d svlan %d", onuID, vlan, vlan),
			fmt.Sprintf("onu-ratelimit %d upstream %d downstream %d", onuID, bwUp*1000, bwDown*1000),
			"exit",
			"commit",
			"end",
		}
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return a.translateError(err)
	}
	return nil
}

func (a *Adapter) DeleteSubscriber(ctx context.Context, subscriberID string) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	ponPort, onuID := a.parseSubscriberID(subscriberID)

	var commands []string

	if a.detectPONType() == "gpon" {
		commands = []string{
			"configure terminal",
			fmt.Sprintf("interface gpon-olt_%s", ponPort),
			fmt.Sprintf("no onu-set %d", onuID),
			"exit",
			"commit",
			"end",
		}
	} else {
		commands = []string{
			"configure terminal",
			fmt.Sprintf("interface epon-olt_%s", ponPort),
			fmt.Sprintf("no onu-set %d", onuID),
			"exit",
			"commit",
			"end",
		}
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return a.translateError(err)
	}
	return nil
}

func (a *Adapter) SuspendSubscriber(ctx context.Context, subscriberID string) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	ponPort, onuID := a.parseSubscriberID(subscriberID)

	var commands []string

	if a.detectPONType() == "gpon" {
		commands = []string{
			"configure terminal",
			fmt.Sprintf("interface gpon-olt_%s", ponPort),
			fmt.Sprintf("onu-deactivate %d", onuID),
			"exit",
			"commit",
			"end",
		}
	} else {
		commands = []string{
			"configure terminal",
			fmt.Sprintf("interface epon-olt_%s", ponPort),
			fmt.Sprintf("onu-deactivate %d", onuID),
			"exit",
			"commit",
			"end",
		}
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return a.translateError(err)
	}
	return nil
}

func (a *Adapter) ResumeSubscriber(ctx context.Context, subscriberID string) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	ponPort, onuID := a.parseSubscriberID(subscriberID)

	var commands []string

	if a.detectPONType() == "gpon" {
		commands = []string{
			"configure terminal",
			fmt.Sprintf("interface gpon-olt_%s", ponPort),
			fmt.Sprintf("onu-activate %d", onuID),
			"exit",
			"commit",
			"end",
		}
	} else {
		commands = []string{
			"configure terminal",
			fmt.Sprintf("interface epon-olt_%s", ponPort),
			fmt.Sprintf("onu-activate %d", onuID),
			"exit",
			"commit",
			"end",
		}
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return a.translateError(err)
	}
	return nil
}

func (a *Adapter) GetSubscriberStatus(ctx context.Context, subscriberID string) (*types.SubscriberStatus, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	ponPort, onuID := a.parseSubscriberID(subscriberID)

	// C-Data CLI command to get ONU info
	var cmd string
	if a.detectPONType() == "gpon" {
		cmd = fmt.Sprintf("show gpon onu-info gpon-olt_%s %d", ponPort, onuID)
	} else {
		cmd = fmt.Sprintf("show epon onu-info epon-olt_%s %d", ponPort, onuID)
	}

	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return nil, a.translateError(err)
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

	// C-Data CLI command to get ONU statistics
	var cmd string
	if a.detectPONType() == "gpon" {
		cmd = fmt.Sprintf("show gpon onu-statistics gpon-olt_%s %d", ponPort, onuID)
	} else {
		cmd = fmt.Sprintf("show epon onu-statistics epon-olt_%s %d", ponPort, onuID)
	}

	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return nil, a.translateError(err)
	}

	// Parse CLI output
	stats := a.parseONUStats(output)

	return stats, nil
}

func (a *Adapter) HealthCheck(ctx context.Context) error {
	if a.cliExecutor == nil {
		return a.baseDriver.HealthCheck(ctx)
	}

	// C-Data health check: show system info
	_, err := a.cliExecutor.ExecCommand(ctx, "show system")
	if err != nil {
		return a.translateError(err)
	}
	return nil
}

// DiscoverONUs discovers unprovisioned ONUs on all PON ports
func (a *Adapter) DiscoverONUs(ctx context.Context, ponPorts []string) ([]types.ONUDiscovery, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	// If no specific ports requested, discover all
	var cmd string
	if a.detectPONType() == "gpon" {
		cmd = "show gpon onu autofind"
	} else {
		cmd = "show epon onu autofind"
	}

	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return nil, a.translateError(err)
	}

	// Parse autofind output
	discoveries := a.parseAutofindOutput(output)

	// Filter by requested ports if specified
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

// parseAutofindOutput parses C-Data autofind CLI output
func (a *Adapter) parseAutofindOutput(output string) []types.ONUDiscovery {
	discoveries := []types.ONUDiscovery{}

	// C-Data autofind output format (example):
	// Interface       SN              Distance  RxPower
	// gpon-olt_1/1/1  CDAT12345678    1234      -18.5
	// gpon-olt_1/1/2  CDAT87654321    567       -22.1

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Interface") || strings.HasPrefix(line, "-") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 {
			// Extract port from interface name (e.g., "gpon-olt_1/1/1" -> "1/1/1")
			ponPort := a.extractPortFromInterface(fields[0])

			discovery := types.ONUDiscovery{
				PONPort:      ponPort,
				Serial:       fields[1],
				DiscoveredAt: time.Now(),
			}

			if len(fields) >= 3 {
				// Parse distance
				if dist, err := strconv.Atoi(fields[2]); err == nil {
					discovery.DistanceM = dist
				}
			}
			if len(fields) >= 4 {
				// Parse Rx power
				if rx, err := strconv.ParseFloat(fields[3], 64); err == nil {
					discovery.RxPowerDBm = rx
				}
			}

			discoveries = append(discoveries, discovery)
		}
	}

	return discoveries
}

// parseONUStatus parses C-Data ONU status CLI output
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
	if strings.Contains(outputLower, "online") || strings.Contains(outputLower, "working") {
		status.State = "online"
		status.IsOnline = true
	} else if strings.Contains(outputLower, "offline") || strings.Contains(outputLower, "los") {
		status.State = "offline"
		status.IsOnline = false
	} else if strings.Contains(outputLower, "deactivate") || strings.Contains(outputLower, "disabled") {
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
	rxPowerRe := regexp.MustCompile(`rx[_\s]*power[:\s]+(-?\d+\.?\d*)`)
	if match := rxPowerRe.FindStringSubmatch(outputLower); len(match) > 1 {
		status.Metadata["rx_power_dbm"] = match[1]
	}

	txPowerRe := regexp.MustCompile(`tx[_\s]*power[:\s]+(-?\d+\.?\d*)`)
	if match := txPowerRe.FindStringSubmatch(outputLower); len(match) > 1 {
		status.Metadata["tx_power_dbm"] = match[1]
	}

	// Store raw output
	status.Metadata["cli_output"] = output

	return status
}

// parseONUStats parses C-Data ONU statistics CLI output
func (a *Adapter) parseONUStats(output string) *types.SubscriberStats {
	stats := &types.SubscriberStats{
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	outputLower := strings.ToLower(output)

	// Parse bytes/packets from output
	rxBytesRe := regexp.MustCompile(`rx[_\s]*bytes[:\s]+(\d+)`)
	if match := rxBytesRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.BytesDown = val
		}
	}

	txBytesRe := regexp.MustCompile(`tx[_\s]*bytes[:\s]+(\d+)`)
	if match := txBytesRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.BytesUp = val
		}
	}

	rxPacketsRe := regexp.MustCompile(`rx[_\s]*packets[:\s]+(\d+)`)
	if match := rxPacketsRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.PacketsDown = val
		}
	}

	txPacketsRe := regexp.MustCompile(`tx[_\s]*packets[:\s]+(\d+)`)
	if match := txPacketsRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.PacketsUp = val
		}
	}

	// Parse errors
	errorsRe := regexp.MustCompile(`errors[:\s]+(\d+)`)
	if match := errorsRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.ErrorsDown = val
		}
	}

	dropsRe := regexp.MustCompile(`drops[:\s]+(\d+)`)
	if match := dropsRe.FindStringSubmatch(outputLower); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.Drops = val
		}
	}

	stats.Metadata["cli_output"] = output

	return stats
}

// verifyONUExists verifies the ONU was created successfully
// C-Data can fail silently, so we always verify after provisioning
func (a *Adapter) verifyONUExists(ctx context.Context, ponPort string, onuID int) error {
	var cmd string
	if a.detectPONType() == "gpon" {
		cmd = fmt.Sprintf("show gpon onu-info gpon-olt_%s %d", ponPort, onuID)
	} else {
		cmd = fmt.Sprintf("show epon onu-info epon-olt_%s %d", ponPort, onuID)
	}

	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to verify ONU: %w", err)
	}

	// Check for indicators that ONU exists
	outputLower := strings.ToLower(output)
	if strings.Contains(outputLower, "not found") ||
		strings.Contains(outputLower, "no onu") ||
		strings.Contains(outputLower, "invalid") {
		return fmt.Errorf("ONU not found after provisioning")
	}

	return nil
}

// Helper methods

// detectModel determines the C-Data OLT model
func (a *Adapter) detectModel() string {
	if model, ok := a.config.Metadata["model"]; ok {
		return model
	}
	return "fd1104s"
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
	// Default to first port (C-Data format: slot/frame/port)
	return "1/1/1"
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

// getONUType returns the ONU type for provisioning
func (a *Adapter) getONUType(subscriber *model.Subscriber) string {
	// Check subscriber annotations for ONU type
	if subscriber.Annotations != nil {
		if onuType, ok := subscriber.Annotations["nanoncore.com/onu-type"]; ok {
			return onuType
		}
	}
	// Default to router type
	return "router"
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
	return fmt.Sprintf("line_%dM_%dM", tier.Spec.BandwidthDown, tier.Spec.BandwidthUp)
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
	return "service_internet"
}

// parseSubscriberID parses a subscriber ID to extract PON port and ONU ID
func (a *Adapter) parseSubscriberID(subscriberID string) (string, int) {
	// Expected format: "onu-1/1/1-5" or just subscriber name
	// Try to parse from ID first
	re := regexp.MustCompile(`onu-(\d+/\d+/\d+)-(\d+)`)
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
	return "1/1/1", hash
}

// extractPortFromInterface extracts port number from interface name
// e.g., "gpon-olt_1/1/1" -> "1/1/1"
func (a *Adapter) extractPortFromInterface(iface string) string {
	re := regexp.MustCompile(`(?:gpon|epon)-olt_(\d+/\d+/\d+)`)
	if match := re.FindStringSubmatch(iface); len(match) > 1 {
		return match[1]
	}
	return iface
}
