package huawei

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nanoncore/nano-southbound/drivers/snmp"
	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
	"github.com/nanoncore/nano-southbound/vendors/common"
)

// Adapter wraps a base driver with Huawei-specific logic
// Huawei OLTs (MA5600T/MA5800-X series) require BOTH protocols:
// - CLI for configuration (provisioning, deletion, updates)
// - SNMP for monitoring (ONU listing, power readings, stats)
type Adapter struct {
	baseDriver      types.Driver
	secondaryDriver types.Driver // SNMP driver when primary is CLI
	cliExecutor     types.CLIExecutor
	snmpExecutor    types.SNMPExecutor
	config          *types.EquipmentConfig
}

// NewAdapter creates a new Huawei adapter
// If the base driver is CLI, it automatically creates an SNMP driver for monitoring
func NewAdapter(baseDriver types.Driver, config *types.EquipmentConfig) types.Driver {
	adapter := &Adapter{
		baseDriver: baseDriver,
		config:     config,
	}

	// Extract executors from base driver
	if executor, ok := baseDriver.(types.CLIExecutor); ok {
		adapter.cliExecutor = executor
	}
	if executor, ok := baseDriver.(types.SNMPExecutor); ok {
		adapter.snmpExecutor = executor
	}

	// Create secondary SNMP driver if base is CLI and SNMP not available
	if adapter.cliExecutor != nil && adapter.snmpExecutor == nil {
		adapter.createSNMPDriver()
	}

	return adapter
}

// createSNMPDriver creates an SNMP driver for monitoring operations
func (a *Adapter) createSNMPDriver() {
	snmpConfig := *a.config
	snmpConfig.Protocol = types.ProtocolSNMP

	// Use secondary port or SNMP default (161)
	if a.config.SecondaryPort > 0 {
		snmpConfig.Port = a.config.SecondaryPort
	} else {
		snmpConfig.Port = 161
	}

	// Set SNMP metadata
	if snmpConfig.Metadata == nil {
		snmpConfig.Metadata = make(map[string]string)
	}
	community := a.config.SNMPCommunity
	if community == "" {
		community = "public"
	}
	snmpConfig.Metadata["snmp_community"] = community

	version := a.config.SNMPVersion
	if version == "" {
		version = "2c"
	}
	snmpConfig.Metadata["snmp_version"] = version

	snmpDriver, err := snmp.NewDriver(&snmpConfig)
	if err != nil {
		return // SNMP creation failed, continue without it
	}

	a.secondaryDriver = snmpDriver
	if executor, ok := snmpDriver.(types.SNMPExecutor); ok {
		a.snmpExecutor = executor
	}
}

func (a *Adapter) Connect(ctx context.Context, config *types.EquipmentConfig) error {
	// Connect primary driver
	if err := a.baseDriver.Connect(ctx, config); err != nil {
		return fmt.Errorf("primary driver connect failed: %w", err)
	}

	// Connect secondary driver if present (uses its own config set during creation)
	if a.secondaryDriver != nil {
		// Create SNMP-specific config for secondary driver
		snmpConfig := *a.config
		snmpConfig.Protocol = types.ProtocolSNMP
		if a.config.SecondaryPort > 0 {
			snmpConfig.Port = a.config.SecondaryPort
		} else {
			snmpConfig.Port = 161
		}
		if err := a.secondaryDriver.Connect(ctx, &snmpConfig); err != nil {
			// Log but don't fail - secondary is optional for some operations
			// SNMP may not be required if only doing CLI operations
			_ = err // Explicitly ignore error - secondary driver is optional
		}
	}

	return nil
}

func (a *Adapter) Disconnect(ctx context.Context) error {
	// Disconnect secondary driver first (if present)
	if a.secondaryDriver != nil {
		_ = a.secondaryDriver.Disconnect(ctx)
	}

	// Disconnect primary driver
	return a.baseDriver.Disconnect(ctx)
}

func (a *Adapter) IsConnected() bool {
	return a.baseDriver.IsConnected()
}

// CreateSubscriber provisions an ONT on the Huawei OLT
func (a *Adapter) CreateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) (*types.SubscriberResult, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - Huawei requires CLI driver")
	}

	// Parse subscriber info
	frame, slot, port := a.parseFSP(subscriber)
	ontID := a.getONTID(subscriber)
	serial := subscriber.Spec.ONUSerial
	vlan := subscriber.Spec.VLAN

	// Get profile IDs
	lineProfileID := a.getLineProfileID(tier)
	srvProfileID := a.getServiceProfileID(tier)

	// Huawei MA5800 CLI command sequence
	commands := a.buildProvisioningCommands(frame, slot, port, ontID, serial, vlan, lineProfileID, srvProfileID, tier)

	// Execute commands
	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return nil, fmt.Errorf("Huawei provisioning failed: %w", err)
	}

	// Build result
	result := &types.SubscriberResult{
		SubscriberID:  subscriber.Name,
		SessionID:     fmt.Sprintf("ont-%d/%d/%d-%d", frame, slot, port, ontID),
		AssignedIP:    subscriber.Spec.IPAddress,
		AssignedIPv6:  subscriber.Spec.IPv6Address,
		InterfaceName: fmt.Sprintf("gpon %d/%d/%d ont %d", frame, slot, port, ontID),
		VLAN:          vlan,
		Metadata: map[string]interface{}{
			"vendor":          "huawei",
			"model":           a.detectModel(),
			"frame":           frame,
			"slot":            slot,
			"port":            port,
			"ont_id":          ontID,
			"serial":          serial,
			"line_profile_id": lineProfileID,
			"srv_profile_id":  srvProfileID,
			"cli_outputs":     outputs,
		},
	}

	return result, nil
}

// buildProvisioningCommands builds Huawei GPON CLI commands
func (a *Adapter) buildProvisioningCommands(frame, slot, port, ontID int, serial string, vlan int, lineProfileID, srvProfileID int, tier *model.ServiceTier) []string {
	// Huawei MA5800/MA5600T GPON CLI reference
	// Based on Huawei SmartAX MA5800-X series CLI documentation

	commands := []string{
		// Enter privileged exec mode first (required before config)
		"enable",

		// Enter global config mode
		"config",

		// Navigate to GPON interface
		fmt.Sprintf("interface gpon %d/%d", frame, slot),

		// Add ONT with serial number authentication
		// ont add <port> <ont-id> sn-auth <serial> omci ont-lineprofile-id <id> ont-srvprofile-id <id> desc <description>
		fmt.Sprintf("ont add %d %d sn-auth %s omci ont-lineprofile-id %d ont-srvprofile-id %d desc nanoncore",
			port, ontID, serial, lineProfileID, srvProfileID),

		// Configure native VLAN on ONT ETH port
		// ont port native-vlan <port> <ont-id> eth <eth-port> vlan <vlan> priority <0-7>
		fmt.Sprintf("ont port native-vlan %d %d eth 1 vlan %d priority 0", port, ontID, vlan),

		// Exit GPON interface
		"quit",

		// Configure service port for traffic
		// service-port <id> vlan <vlan> gpon <frame>/<slot>/<port> ont <ont-id> gemport <gemport> multi-service user-vlan <vlan> tag-transform translate
		fmt.Sprintf("service-port vlan %d gpon %d/%d/%d ont %d gemport 1 multi-service user-vlan %d tag-transform translate",
			vlan, frame, slot, port, ontID, vlan),

		// Apply configuration
		"quit",
	}

	// Add traffic profile commands if bandwidth is specified
	if tier.Spec.BandwidthDown > 0 || tier.Spec.BandwidthUp > 0 {
		trafficCommands := a.buildTrafficProfileCommands(frame, slot, port, ontID, tier)
		commands = append(commands, trafficCommands...)
	}

	return commands
}

// buildTrafficProfileCommands builds Huawei traffic/QoS commands
func (a *Adapter) buildTrafficProfileCommands(frame, slot, port, ontID int, tier *model.ServiceTier) []string {
	// Huawei uses traffic tables and profiles for QoS
	// CAR (Committed Access Rate) for rate limiting

	trafficTableID := a.getTrafficTableID(tier)

	commands := []string{
		"config",
		fmt.Sprintf("interface gpon %d/%d", frame, slot),

		// Bind traffic table to ONT
		// ont traffic-policy <port> <ont-id> profile-id <id>
		fmt.Sprintf("ont traffic-policy %d %d profile-id %d", port, ontID, trafficTableID),

		"quit",
	}

	return commands
}

func (a *Adapter) UpdateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	frame, slot, port := a.parseFSP(subscriber)
	ontID := a.getONTID(subscriber)
	vlan := subscriber.Spec.VLAN
	lineProfileID := a.getLineProfileID(tier)
	srvProfileID := a.getServiceProfileID(tier)
	trafficTableID := a.getTrafficTableID(tier)

	commands := []string{
		"config",
		fmt.Sprintf("interface gpon %d/%d", frame, slot),

		// Modify ONT profiles
		fmt.Sprintf("ont modify %d %d ont-lineprofile-id %d ont-srvprofile-id %d",
			port, ontID, lineProfileID, srvProfileID),

		// Update VLAN if changed
		fmt.Sprintf("ont port native-vlan %d %d eth 1 vlan %d priority 0", port, ontID, vlan),

		// Update traffic policy
		fmt.Sprintf("ont traffic-policy %d %d profile-id %d", port, ontID, trafficTableID),

		"quit",
		"quit",
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	return err
}

func (a *Adapter) DeleteSubscriber(ctx context.Context, subscriberID string) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	frame, slot, port, ontID := a.parseSubscriberID(subscriberID)

	commands := []string{
		"enable",
		"config",
		fmt.Sprintf("interface gpon %d/%d", frame, slot),

		// Delete ONT (this also removes associated service ports)
		fmt.Sprintf("ont delete %d %d", port, ontID),

		"quit",
		"quit",
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	return err
}

func (a *Adapter) SuspendSubscriber(ctx context.Context, subscriberID string) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	frame, slot, port, ontID := a.parseSubscriberID(subscriberID)

	commands := []string{
		"enable",
		"config",
		fmt.Sprintf("interface gpon %d/%d", frame, slot),

		// Deactivate ONT (keeps config, disables traffic)
		fmt.Sprintf("ont deactivate %d %d", port, ontID),

		"quit",
		"quit",
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	return err
}

func (a *Adapter) ResumeSubscriber(ctx context.Context, subscriberID string) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	frame, slot, port, ontID := a.parseSubscriberID(subscriberID)

	commands := []string{
		"enable",
		"config",
		fmt.Sprintf("interface gpon %d/%d", frame, slot),

		// Activate ONT
		fmt.Sprintf("ont activate %d %d", port, ontID),

		"quit",
		"quit",
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	return err
}

func (a *Adapter) GetSubscriberStatus(ctx context.Context, subscriberID string) (*types.SubscriberStatus, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	frame, slot, port, ontID := a.parseSubscriberID(subscriberID)

	// Get ONT info
	cmd := fmt.Sprintf("display ont info %d/%d %d %d", frame, slot, port, ontID)
	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get ONT status: %w", err)
	}

	// Parse status
	status := a.parseONTStatus(output, subscriberID)

	// Get optical info for additional details
	optCmd := fmt.Sprintf("display ont optical-info %d/%d %d %d", frame, slot, port, ontID)
	optOutput, _ := a.cliExecutor.ExecCommand(ctx, optCmd)
	if optOutput != "" {
		a.parseOpticalInfo(optOutput, status)
	}

	return status, nil
}

func (a *Adapter) GetSubscriberStats(ctx context.Context, subscriberID string) (*types.SubscriberStats, error) {
	// Prefer SNMP for stats collection (faster, more reliable)
	if a.snmpExecutor != nil {
		return a.getSubscriberStatsSNMP(ctx, subscriberID)
	}

	// Fallback to CLI
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("no executor available (need CLI or SNMP)")
	}

	frame, slot, port, ontID := a.parseSubscriberID(subscriberID)

	// Get ONT traffic statistics via CLI
	cmd := fmt.Sprintf("display ont traffic %d/%d %d %d", frame, slot, port, ontID)
	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get ONT stats: %w", err)
	}

	// Parse statistics
	stats := a.parseONTStats(output)

	return stats, nil
}

// getSubscriberStatsSNMP retrieves stats using SNMP (preferred method)
// Based on legacy production code with Huawei OIDs
func (a *Adapter) getSubscriberStatsSNMP(ctx context.Context, subscriberID string) (*types.SubscriberStats, error) {
	frame, slot, port, ontID := a.parseSubscriberID(subscriberID)

	// Build SNMP index for this ONT
	// Huawei index format: <portIndex>.<onuIndex>
	// portIndex = (frame * 65536) + (slot * 256) + port (varies by model)
	portIndex := (frame << 16) | (slot << 8) | port
	snmpIndex := fmt.Sprintf("%d.%d", portIndex, ontID)

	stats := &types.SubscriberStats{
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Query traffic counters
	upBytesOID := fmt.Sprintf("%s.%s", OIDOnuUpBytes, snmpIndex)
	downBytesOID := fmt.Sprintf("%s.%s", OIDOnuDownBytes, snmpIndex)

	// Query optical parameters for metadata
	rxPowerOID := fmt.Sprintf("%s.%s", OIDOnuRxPower, snmpIndex)
	txPowerOID := fmt.Sprintf("%s.%s", OIDOnuTxPower, snmpIndex)
	temperatureOID := fmt.Sprintf("%s.%s", OIDOnuTemperature, snmpIndex)
	voltageOID := fmt.Sprintf("%s.%s", OIDOnuVoltage, snmpIndex)

	// Bulk get all values
	oids := []string{upBytesOID, downBytesOID, rxPowerOID, txPowerOID, temperatureOID, voltageOID}
	results, err := a.snmpExecutor.BulkGetSNMP(ctx, oids)
	if err != nil {
		return nil, fmt.Errorf("SNMP query failed: %w", err)
	}

	// Parse traffic counters
	if val, ok := results[upBytesOID]; ok {
		if v, ok := val.(uint64); ok {
			stats.BytesUp = v
		}
	}
	if val, ok := results[downBytesOID]; ok {
		if v, ok := val.(uint64); ok {
			stats.BytesDown = v
		}
	}

	// Parse optical parameters (add to metadata)
	if val, ok := results[rxPowerOID]; ok {
		if v, ok := val.(int64); ok {
			if IsOnuOnline(v) {
				stats.Metadata["rx_power_dbm"] = ConvertOpticalPower(v)
			}
		}
	}
	if val, ok := results[txPowerOID]; ok {
		if v, ok := val.(int64); ok {
			if v != common.SNMPInvalidValue {
				stats.Metadata["tx_power_dbm"] = ConvertOpticalPower(v)
			}
		}
	}
	if val, ok := results[temperatureOID]; ok {
		if v, ok := val.(int64); ok {
			if v != common.SNMPInvalidValue {
				stats.Metadata["temperature_c"] = v
			}
		}
	}
	if val, ok := results[voltageOID]; ok {
		if v, ok := val.(int64); ok {
			if v != common.SNMPInvalidValue {
				stats.Metadata["voltage_v"] = ConvertVoltage(v)
			}
		}
	}

	stats.Metadata["source"] = "snmp"
	stats.Metadata["snmp_index"] = snmpIndex

	return stats, nil
}

func (a *Adapter) HealthCheck(ctx context.Context) error {
	if a.cliExecutor == nil {
		return a.baseDriver.HealthCheck(ctx)
	}

	// Huawei health check: display version
	_, err := a.cliExecutor.ExecCommand(ctx, "display version")
	return err
}

// DiscoverONTs discovers unprovisioned ONTs on all GPON ports
func (a *Adapter) DiscoverONTs(ctx context.Context) ([]ONTDiscovery, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	// Huawei CLI command to show autofind ONTs
	cmd := "display ont autofind all"
	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to discover ONTs: %w", err)
	}

	// Parse autofind output
	return a.parseAutofindOutput(output), nil
}

// ONTDiscovery represents a discovered ONT
type ONTDiscovery struct {
	Frame     int       `json:"frame"`
	Slot      int       `json:"slot"`
	Port      int       `json:"port"`
	Serial    string    `json:"serial"`
	EquipID   string    `json:"equip_id"`   // Equipment identifier (ONT model)
	LOID      string    `json:"loid"`       // Logical ONU ID (if LOID auth used)
	Distance  int       `json:"distance_m"` // Distance in meters
	RxPower   float64   `json:"rx_power_dbm"`
	Timestamp time.Time `json:"discovered_at"`
}

// ONTStats represents ONT statistics from SNMP bulk scan
type ONTStats struct {
	Index       string  `json:"index"`
	Serial      string  `json:"serial"`
	Frame       int     `json:"frame"`
	Slot        int     `json:"slot"`
	Port        int     `json:"port"`
	ONUID       int     `json:"onu_id"`
	IsOnline    bool    `json:"is_online"`
	RxPower     float64 `json:"rx_power_dbm"`
	TxPower     float64 `json:"tx_power_dbm"`
	Temperature float64 `json:"temperature_c"`
	Voltage     float64 `json:"voltage_v"`
	Distance    int     `json:"distance_m"`
	BiasCurrent float64 `json:"bias_current_ma"`
	BytesUp     uint64  `json:"bytes_up"`
	BytesDown   uint64  `json:"bytes_down"`
}

// BulkScanONUsSNMP performs SNMP walk to get all ONUs (like legacy PHP code)
// This is much faster than querying each ONU individually
func (a *Adapter) BulkScanONUsSNMP(ctx context.Context) ([]ONTStats, error) {
	if a.snmpExecutor == nil {
		return nil, fmt.Errorf("SNMP executor not available")
	}

	// Walk serial numbers to get all ONUs
	serials, err := a.snmpExecutor.WalkSNMP(ctx, OIDOnuSerialNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to walk serial numbers: %w", err)
	}

	// Walk Rx power to determine online status
	rxPowers, err := a.snmpExecutor.WalkSNMP(ctx, OIDOnuRxPower)
	if err != nil {
		// Non-fatal, continue without power data
		rxPowers = make(map[string]interface{})
	}

	// Walk Tx power
	txPowers, err := a.snmpExecutor.WalkSNMP(ctx, OIDOnuTxPower)
	if err != nil {
		txPowers = make(map[string]interface{})
	}

	// Walk temperature
	temperatures, err := a.snmpExecutor.WalkSNMP(ctx, OIDOnuTemperature)
	if err != nil {
		temperatures = make(map[string]interface{})
	}

	// Walk voltage
	voltages, err := a.snmpExecutor.WalkSNMP(ctx, OIDOnuVoltage)
	if err != nil {
		voltages = make(map[string]interface{})
	}

	// Walk distance
	distances, err := a.snmpExecutor.WalkSNMP(ctx, OIDOnuDistance)
	if err != nil {
		distances = make(map[string]interface{})
	}

	// Walk bias current
	biasCurrents, err := a.snmpExecutor.WalkSNMP(ctx, OIDOnuCurrent)
	if err != nil {
		biasCurrents = make(map[string]interface{})
	}

	// Walk traffic counters
	upBytes, err := a.snmpExecutor.WalkSNMP(ctx, OIDOnuUpBytes)
	if err != nil {
		upBytes = make(map[string]interface{})
	}

	downBytes, err := a.snmpExecutor.WalkSNMP(ctx, OIDOnuDownBytes)
	if err != nil {
		downBytes = make(map[string]interface{})
	}

	// Build results
	results := make([]ONTStats, 0, len(serials))

	for index, serialVal := range serials {
		hexSerial, ok := serialVal.(string)
		if !ok {
			continue
		}

		// Decode serial number (hex to readable)
		serial := DecodeHexSerial(hexSerial)

		// Parse index to get frame/slot/port/onuID
		frame, slot, port, onuID, err := ParseONUIndex(index)
		if err != nil {
			slog.Debug("failed to parse ONU index", "index", index, "error", err)
			continue
		}

		onu := ONTStats{
			Index:  index,
			Serial: serial,
			Frame:  frame,
			Slot:   slot,
			Port:   port,
			ONUID:  onuID,
		}

		// Check Rx power (determines online status)
		if rxVal, ok := rxPowers[index]; ok {
			if rxRaw, ok := rxVal.(int64); ok {
				onu.IsOnline = IsOnuOnline(rxRaw)
				if onu.IsOnline {
					onu.RxPower = ConvertOpticalPower(rxRaw)
				}
			}
		}

		// Tx power
		if txVal, ok := txPowers[index]; ok {
			if txRaw, ok := txVal.(int64); ok {
				if txRaw != common.SNMPInvalidValue {
					onu.TxPower = ConvertOpticalPower(txRaw)
				}
			}
		}

		// Temperature (convert from raw to Celsius)
		if tempVal, ok := temperatures[index]; ok {
			if tempRaw, ok := tempVal.(int64); ok {
				if tempRaw != common.SNMPInvalidValue {
					onu.Temperature = ConvertTemperature(tempRaw)
				}
			}
		}

		// Voltage
		if voltVal, ok := voltages[index]; ok {
			if voltRaw, ok := voltVal.(int64); ok {
				if voltRaw != common.SNMPInvalidValue {
					onu.Voltage = ConvertVoltage(voltRaw)
				}
			}
		}

		// Distance
		if distVal, ok := distances[index]; ok {
			if distRaw, ok := distVal.(int64); ok {
				onu.Distance = int(distRaw)
			}
		}

		// Bias current (convert from µA to mA)
		if biasVal, ok := biasCurrents[index]; ok {
			if biasRaw, ok := biasVal.(int64); ok {
				if biasRaw != common.SNMPInvalidValue {
					onu.BiasCurrent = float64(biasRaw) / 1000.0
				}
			}
		}

		// Traffic counters
		if upVal, ok := upBytes[index]; ok {
			if upRaw, ok := upVal.(uint64); ok {
				onu.BytesUp = upRaw
			}
		}

		if downVal, ok := downBytes[index]; ok {
			if downRaw, ok := downVal.(uint64); ok {
				onu.BytesDown = downRaw
			}
		}

		results = append(results, onu)
	}

	return results, nil
}

// parseAutofindOutput parses Huawei autofind CLI output
func (a *Adapter) parseAutofindOutput(output string) []ONTDiscovery {
	discoveries := []ONTDiscovery{}

	// Huawei autofind output format:
	// F/S/P   ONT         SN                  VendorID   EquipmentID     Time
	// 0/1/0   1           485754430A2C4F13    HWTC       HG8245Q2        2024-01-15 10:30:00
	// 0/1/1   2           5053534E00000001    ZTEG       F670L           2024-01-15 10:31:00

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "F/S/P") || strings.HasPrefix(line, "-") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 5 {
			// Parse F/S/P
			fspParts := strings.Split(fields[0], "/")
			if len(fspParts) != 3 {
				continue
			}

			frame, err := strconv.Atoi(fspParts[0])
			if err != nil {
				continue // skip entries with invalid F/S/P data
			}
			slot, err := strconv.Atoi(fspParts[1])
			if err != nil {
				continue
			}
			port, err := strconv.Atoi(fspParts[2])
			if err != nil {
				continue
			}

			discovery := ONTDiscovery{
				Frame:     frame,
				Slot:      slot,
				Port:      port,
				Serial:    fields[2],
				Timestamp: time.Now(),
			}

			if len(fields) >= 5 {
				discovery.EquipID = fields[4]
			}

			discoveries = append(discoveries, discovery)
		}
	}

	return discoveries
}

// parseONTStatus parses Huawei ONT status CLI output
func (a *Adapter) parseONTStatus(output string, subscriberID string) *types.SubscriberStatus {
	status := &types.SubscriberStatus{
		SubscriberID: subscriberID,
		State:        "unknown",
		IsOnline:     false,
		LastActivity: time.Now(),
		Metadata:     make(map[string]interface{}),
	}

	// Parse state from output
	outputLower := strings.ToLower(output)

	// Huawei uses "Run state" field
	if strings.Contains(outputLower, "run state") {
		if strings.Contains(outputLower, "online") {
			status.State = "online"
			status.IsOnline = true
		} else if strings.Contains(outputLower, "offline") {
			status.State = "offline"
			status.IsOnline = false
		}
	}

	// Check config state
	if strings.Contains(outputLower, "config state") {
		if strings.Contains(outputLower, "normal") {
			status.Metadata["config_state"] = "normal"
		} else if strings.Contains(outputLower, "deactivate") {
			status.State = "suspended"
			status.IsOnline = false
		}
	}

	// Parse uptime
	uptimeRe := regexp.MustCompile(`online\s+duration[:\s]+(\d+)\s*day[s]?\s*(\d+):(\d+):(\d+)`)
	if match := uptimeRe.FindStringSubmatch(outputLower); len(match) == 5 {
		days, _ := strconv.ParseInt(match[1], 10, 64)
		hours, _ := strconv.ParseInt(match[2], 10, 64)
		minutes, _ := strconv.ParseInt(match[3], 10, 64)
		seconds, _ := strconv.ParseInt(match[4], 10, 64)
		status.UptimeSeconds = days*86400 + hours*3600 + minutes*60 + seconds
	}

	// Parse IP if assigned
	ipRe := regexp.MustCompile(`ip\s+address[:\s]+(\d+\.\d+\.\d+\.\d+)`)
	if match := ipRe.FindStringSubmatch(outputLower); len(match) > 1 {
		status.IPv4Address = match[1]
	}

	status.Metadata["cli_output"] = output

	return status
}

// parseOpticalInfo adds optical info to status
func (a *Adapter) parseOpticalInfo(output string, status *types.SubscriberStatus) {
	outputLower := strings.ToLower(output)

	// Parse Rx power
	rxPowerRe := regexp.MustCompile(`rx\s*optical\s*power[:\s]+(-?\d+\.?\d*)\s*(?:dbm)?`)
	if match := rxPowerRe.FindStringSubmatch(outputLower); len(match) > 1 {
		status.Metadata["rx_power_dbm"] = match[1]
	}

	// Parse Tx power
	txPowerRe := regexp.MustCompile(`tx\s*optical\s*power[:\s]+(-?\d+\.?\d*)\s*(?:dbm)?`)
	if match := txPowerRe.FindStringSubmatch(outputLower); len(match) > 1 {
		status.Metadata["tx_power_dbm"] = match[1]
	}

	// Parse OLT Rx power (from ONT)
	oltRxRe := regexp.MustCompile(`olt\s*rx\s*ont\s*optical\s*power[:\s]+(-?\d+\.?\d*)\s*(?:dbm)?`)
	if match := oltRxRe.FindStringSubmatch(outputLower); len(match) > 1 {
		status.Metadata["olt_rx_power_dbm"] = match[1]
	}

	// Parse temperature
	tempRe := regexp.MustCompile(`temperature[:\s]+(-?\d+\.?\d*)\s*(?:c)?`)
	if match := tempRe.FindStringSubmatch(outputLower); len(match) > 1 {
		status.Metadata["temperature_c"] = match[1]
	}
}

// parseONTStats parses Huawei ONT statistics CLI output
func (a *Adapter) parseONTStats(output string) *types.SubscriberStats {
	stats := &types.SubscriberStats{
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Parse traffic counters from output
	// Huawei format:
	// Upstream traffic   : xxxx bytes
	// Downstream traffic : yyyy bytes

	rxBytesRe := regexp.MustCompile(`downstream\s*(?:traffic)?[:\s]+(\d+)\s*bytes`)
	if match := rxBytesRe.FindStringSubmatch(strings.ToLower(output)); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.BytesDown = val
		}
	}

	txBytesRe := regexp.MustCompile(`upstream\s*(?:traffic)?[:\s]+(\d+)\s*bytes`)
	if match := txBytesRe.FindStringSubmatch(strings.ToLower(output)); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.BytesUp = val
		}
	}

	// Parse packet counters if available
	rxPacketsRe := regexp.MustCompile(`downstream\s*packets[:\s]+(\d+)`)
	if match := rxPacketsRe.FindStringSubmatch(strings.ToLower(output)); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.PacketsDown = val
		}
	}

	txPacketsRe := regexp.MustCompile(`upstream\s*packets[:\s]+(\d+)`)
	if match := txPacketsRe.FindStringSubmatch(strings.ToLower(output)); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.PacketsUp = val
		}
	}

	// Parse errors
	errorsRe := regexp.MustCompile(`(?:error|discard)[s]?[:\s]+(\d+)`)
	if match := errorsRe.FindStringSubmatch(strings.ToLower(output)); len(match) > 1 {
		if val, err := strconv.ParseUint(match[1], 10, 64); err == nil {
			stats.ErrorsDown = val
		}
	}

	stats.Metadata["cli_output"] = output

	return stats
}

// Helper methods

// detectModel determines the Huawei OLT model
func (a *Adapter) detectModel() string {
	if model, ok := a.config.Metadata["model"]; ok {
		return model
	}
	return "ma5800"
}

// parseFSP extracts Frame/Slot/Port from subscriber metadata
func (a *Adapter) parseFSP(subscriber *model.Subscriber) (frame, slot, port int) {
	// Check for FSP in annotations (support both annotation formats)
	// Format: "0/0/1" or "0/1/0" (frame/slot/port)
	fsp, _ := common.GetAnnotationString(subscriber.Annotations, "nanoncore.com/gpon-fsp", "nano.io/pon-port")

	if fsp != "" {
		parts := strings.Split(fsp, "/")
		if len(parts) == 3 {
			frame, _ = strconv.Atoi(parts[0])
			slot, _ = strconv.Atoi(parts[1])
			port, _ = strconv.Atoi(parts[2])
		}
	}

	return
}

// getONTID extracts or generates ONT ID
func (a *Adapter) getONTID(subscriber *model.Subscriber) int {
	// Check subscriber annotations (support both annotation formats)
	if id, ok := common.GetAnnotationInt(subscriber.Annotations, "nanoncore.com/ont-id", "nano.io/onu-id"); ok {
		return id
	}
	// Generate from VLAN as fallback (max 128 ONTs per port)
	return subscriber.Spec.VLAN % 128
}

// getLineProfileID returns the line profile ID for a service tier
func (a *Adapter) getLineProfileID(tier *model.ServiceTier) int {
	if tier == nil {
		return 1 // default profile ID
	}
	return common.GetAnnotationIntWithDefault(tier.Annotations, 1, "nanoncore.com/line-profile-id")
}

// getServiceProfileID returns the service profile ID for a service tier
func (a *Adapter) getServiceProfileID(tier *model.ServiceTier) int {
	if tier == nil {
		return 1 // default service profile ID
	}
	return common.GetAnnotationIntWithDefault(tier.Annotations, 1, "nanoncore.com/srv-profile-id")
}

// getTrafficTableID returns the traffic table ID for a service tier
func (a *Adapter) getTrafficTableID(tier *model.ServiceTier) int {
	if tier == nil {
		return 1 // default traffic table ID
	}
	if id, ok := common.GetAnnotationInt(tier.Annotations, "nanoncore.com/traffic-table-id"); ok {
		return id
	}
	// Generate based on bandwidth (use bandwidth as table ID)
	// This assumes traffic tables are pre-configured with matching IDs
	return tier.Spec.BandwidthDown
}

// parseSubscriberID parses a subscriber ID to extract Frame/Slot/Port and ONT ID
func (a *Adapter) parseSubscriberID(subscriberID string) (frame, slot, port, ontID int) {
	// Expected format: "ont-0/1/0-5" or just subscriber name
	re := regexp.MustCompile(`ont-(\d+)/(\d+)/(\d+)-(\d+)`)
	if match := re.FindStringSubmatch(subscriberID); len(match) == 5 {
		frame, _ = strconv.Atoi(match[1])
		slot, _ = strconv.Atoi(match[2])
		port, _ = strconv.Atoi(match[3])
		ontID, _ = strconv.Atoi(match[4])
		return
	}

	// Fallback: use defaults and hash of ID
	hash := 0
	for _, c := range subscriberID {
		hash = (hash*31 + int(c)) % 128
	}
	return 0, 1, 0, hash
}

// ============================================================================
// DriverV2 Interface Implementation
// ============================================================================
// These methods adapt the existing Driver v1 implementation to the DriverV2
// interface expected by nano-agent CLI commands.

// GetONUList returns all provisioned ONUs matching the filter.
// Adapts the existing BulkScanONUsSNMP() method to DriverV2 format.
func (a *Adapter) GetONUList(ctx context.Context, filter *types.ONUFilter) ([]types.ONUInfo, error) {
	if a.snmpExecutor == nil {
		return nil, fmt.Errorf("SNMP executor not available - Huawei requires SNMP for ONU listing")
	}

	// Use existing bulk scan method
	onts, err := a.BulkScanONUsSNMP(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to scan ONUs: %w", err)
	}

	// Convert to DriverV2 format
	results := make([]types.ONUInfo, 0, len(onts))
	for _, ont := range onts {
		// Build PON port identifier (frame/slot/port format)
		ponPort := fmt.Sprintf("%d/%d/%d", ont.Frame, ont.Slot, ont.Port)

		// Map operational state
		operState := "offline"
		if ont.IsOnline {
			operState = "online"
		}

		info := types.ONUInfo{
			PONPort:     ponPort,
			ONUID:       ont.ONUID,
			Serial:      ont.Serial,
			AdminState:  "enabled", // Assume enabled if provisioned
			OperState:   operState,
			IsOnline:    ont.IsOnline,
			RxPowerDBm:  ont.RxPower,
			TxPowerDBm:  ont.TxPower,
			DistanceM:   ont.Distance,
			VLAN:        0, // Not available from bulk scan
			Vendor:      "huawei",
			Temperature: ont.Temperature,
			Voltage:     ont.Voltage,
			BiasCurrent: ont.BiasCurrent,
			BytesUp:     ont.BytesUp,
			BytesDown:   ont.BytesDown,
			Metadata: map[string]interface{}{
				"frame":      ont.Frame,
				"slot":       ont.Slot,
				"port":       ont.Port,
				"snmp_index": ont.Index,
			},
		}

		// Apply filters
		if filter != nil {
			if filter.PONPort != "" && filter.PONPort != ponPort {
				continue
			}
			if filter.Status != "" {
				if filter.Status == "online" && !ont.IsOnline {
					continue
				}
				if filter.Status == "offline" && ont.IsOnline {
					continue
				}
			}
			if filter.Serial != "" && !strings.Contains(ont.Serial, filter.Serial) {
				continue
			}
		}

		results = append(results, info)
	}

	return results, nil
}

// GetONUBySerial finds a specific ONU by serial number.
func (a *Adapter) GetONUBySerial(ctx context.Context, serial string) (*types.ONUInfo, error) {
	filter := &types.ONUFilter{Serial: serial}
	onus, err := a.GetONUList(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(onus) == 0 {
		return nil, nil // Not found
	}

	return &onus[0], nil
}

// DiscoverONUs finds unprovisioned ONUs on the OLT.
// Adapts the existing DiscoverONTs() method to new DriverV2 signature.
func (a *Adapter) DiscoverONUs(ctx context.Context, ponPorts []string) ([]types.ONUDiscovery, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - Huawei requires CLI for discovery")
	}

	// Use existing discovery method
	discoveries, err := a.DiscoverONTs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover ONTs: %w", err)
	}

	// Convert to DriverV2 format
	results := make([]types.ONUDiscovery, 0, len(discoveries))
	for _, disc := range discoveries {
		ponPort := fmt.Sprintf("%d/%d/%d", disc.Frame, disc.Slot, disc.Port)

		// Filter by PON ports if specified
		if len(ponPorts) > 0 {
			found := false
			for _, filterPort := range ponPorts {
				if filterPort == ponPort {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		discovery := types.ONUDiscovery{
			PONPort:      ponPort,
			Serial:       disc.Serial,
			Model:        disc.EquipID,
			DistanceM:    disc.Distance,
			RxPowerDBm:   disc.RxPower,
			DiscoveredAt: disc.Timestamp,
			Metadata: map[string]interface{}{
				"frame":    disc.Frame,
				"slot":     disc.Slot,
				"port":     disc.Port,
				"loid":     disc.LOID,
				"equip_id": disc.EquipID,
			},
		}

		results = append(results, discovery)
	}

	return results, nil
}

// RunDiagnostics performs comprehensive diagnostics on an ONU.
// Combines GetSubscriberStatus() and GetSubscriberStats() data.
func (a *Adapter) RunDiagnostics(ctx context.Context, ponPort string, onuID int) (*types.ONUDiagnostics, error) {
	// Build subscriber ID from PON port and ONU ID
	subscriberID := fmt.Sprintf("ont-%s-%d", ponPort, onuID)

	// Get status information
	status, err := a.GetSubscriberStatus(ctx, subscriberID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ONU status: %w", err)
	}

	// Get statistics
	stats, err := a.GetSubscriberStats(ctx, subscriberID)
	if err != nil {
		// Non-fatal, continue with status only
		stats = &types.SubscriberStats{}
	}

	// Build diagnostics result
	diag := &types.ONUDiagnostics{
		Serial:         "", // Not available without additional lookup
		PONPort:        ponPort,
		ONUID:          onuID,
		AdminState:     "enabled",
		OperState:      status.State,
		BytesUp:        stats.BytesUp,
		BytesDown:      stats.BytesDown,
		Errors:         stats.ErrorsDown + stats.ErrorsUp,
		Drops:          stats.Drops,
		LineProfile:    "",
		ServiceProfile: "",
		VLAN:           0,
		Timestamp:      time.Now(),
		VendorData:     status.Metadata,
	}

	// Extract optical power if available
	if rxPower, ok := status.Metadata["rx_power_dbm"]; ok {
		if rxStr, ok := rxPower.(string); ok {
			if rxVal, err := strconv.ParseFloat(rxStr, 64); err == nil {
				diag.Power = &types.ONUPowerReading{
					PONPort:    ponPort,
					ONUID:      onuID,
					RxPowerDBm: rxVal,
					Timestamp:  time.Now(),
				}
			}
		}
	}

	if txPower, ok := status.Metadata["tx_power_dbm"]; ok {
		if txStr, ok := txPower.(string); ok {
			if txVal, err := strconv.ParseFloat(txStr, 64); err == nil {
				if diag.Power == nil {
					diag.Power = &types.ONUPowerReading{
						PONPort:   ponPort,
						ONUID:     onuID,
						Timestamp: time.Now(),
					}
				}
				diag.Power.TxPowerDBm = txVal
			}
		}
	}

	return diag, nil
}

// GetOLTStatus returns comprehensive OLT status.
func (a *Adapter) GetOLTStatus(ctx context.Context) (*types.OLTStatus, error) {
	status := &types.OLTStatus{
		OLTID:       a.config.Name,
		Vendor:      "huawei",
		Model:       a.detectModel(),
		IsReachable: a.baseDriver.IsConnected(),
		IsHealthy:   a.baseDriver.IsConnected(),
		LastPoll:    time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	// Get ONU counts via SNMP
	if a.snmpExecutor != nil {
		onts, err := a.BulkScanONUsSNMP(ctx)
		if err == nil {
			status.TotalONUs = len(onts)
			activeCount := 0
			for _, ont := range onts {
				if ont.IsOnline {
					activeCount++
				}
			}
			status.ActiveONUs = activeCount
		}
	}

	// Try to get system information via SNMP
	if a.snmpExecutor != nil {
		// Query standard MIB-II system OIDs and Huawei SmartAX telemetry OIDs
		systemOIDs := []string{
			OIDSysDescr,
			OIDSysUpTime,
			OIDSysName,
			OIDSmartAXCPU,
			OIDSmartAXMemory,
			OIDSmartAXTemperature,
		}

		results, err := a.snmpExecutor.BulkGetSNMP(ctx, systemOIDs)
		if err == nil {
			// Parse uptime (in hundredths of seconds)
			if uptime, ok := GetSNMPResult(results, OIDSysUpTime); ok {
				if uptimeVal, ok := ParseNumericSNMPValue(uptime); ok {
					status.UptimeSeconds = int64(uptimeVal / 100)
				}
			}

			// Parse firmware from sysDescr
			if descr, ok := GetSNMPResult(results, OIDSysDescr); ok {
				if descrStr, ok := descr.(string); ok {
					status.Metadata["sys_descr"] = descrStr
					// Try to extract version from description
					versionRe := regexp.MustCompile(`V(\d+R\d+C\d+)`)
					if match := versionRe.FindStringSubmatch(descrStr); len(match) > 1 {
						status.Firmware = match[1]
					}
				}
			}

			// Parse CPU utilization
			if cpu, ok := GetSNMPResult(results, OIDSmartAXCPU); ok {
				if cpuVal, ok := ParseNumericSNMPValue(cpu); ok {
					status.CPUPercent = cpuVal
				}
			}

			// Parse memory utilization
			if mem, ok := GetSNMPResult(results, OIDSmartAXMemory); ok {
				if memVal, ok := ParseNumericSNMPValue(mem); ok {
					status.MemoryPercent = memVal
				}
			}

			// Parse board temperature
			if temp, ok := GetSNMPResult(results, OIDSmartAXTemperature); ok {
				if tempVal, ok := ParseNumericSNMPValue(temp); ok {
					status.Temperature = tempVal
				}
			}
		}
	}

	// TODO: Query PON port status if needed
	// This would require walking GPON interface table via SNMP

	return status, nil
}

// GetPONPower returns optical power readings for a PON port.
func (a *Adapter) GetPONPower(ctx context.Context, ponPort string) (*types.PONPowerReading, error) {
	if a.snmpExecutor == nil {
		return nil, fmt.Errorf("SNMP executor not available - Huawei requires SNMP for PON power query")
	}

	// Parse PON port (format: frame/slot/port)
	parts := strings.Split(ponPort, "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid PON port format: %s (expected frame/slot/port)", ponPort)
	}

	frame, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid frame number: %s", parts[0])
	}
	slot, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid slot number: %s", parts[1])
	}
	port, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid port number: %s", parts[2])
	}

	// Build SNMP index (portIndex = (frame << 16) | (slot << 8) | port)
	portIndex := (frame << 16) | (slot << 8) | port

	// Query OLT PON port optical parameters
	oids := []string{
		fmt.Sprintf("%s.%d", OIDOltPonTxPower, portIndex),
		fmt.Sprintf("%s.%d", OIDOltPonTemperature, portIndex),
		fmt.Sprintf("%s.%d", OIDOltPonVoltage, portIndex),
	}

	results, err := a.snmpExecutor.BulkGetSNMP(ctx, oids)
	if err != nil {
		return nil, fmt.Errorf("failed to get PON power: %w", err)
	}

	reading := &types.PONPowerReading{
		PONPort:   ponPort,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Parse Tx power (value * 0.01 dBm)
	if val, ok := GetSNMPResult(results, oids[0]); ok {
		if v, ok := ParseNumericSNMPValue(val); ok {
			reading.TxPowerDBm = ConvertOpticalPower(int64(v))
		}
	}

	// Parse temperature (value / 256 = °C)
	if val, ok := GetSNMPResult(results, oids[1]); ok {
		if v, ok := ParseNumericSNMPValue(val); ok {
			reading.Temperature = ConvertTemperature(int64(v))
		}
	}

	// Parse voltage (value * 0.001 V)
	if val, ok := GetSNMPResult(results, oids[2]); ok {
		if v, ok := ParseNumericSNMPValue(val); ok {
			reading.Metadata["voltage_v"] = ConvertVoltage(int64(v))
		}
	}

	reading.Metadata["port_index"] = portIndex
	reading.Metadata["source"] = "snmp"

	return reading, nil
}

// GetONUPower returns optical power readings for a specific ONU.
func (a *Adapter) GetONUPower(ctx context.Context, ponPort string, onuID int) (*types.ONUPowerReading, error) {
	subscriberID := fmt.Sprintf("ont-%s-%d", ponPort, onuID)

	// Get statistics which includes optical power
	stats, err := a.GetSubscriberStats(ctx, subscriberID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ONU power: %w", err)
	}

	reading := &types.ONUPowerReading{
		PONPort:   ponPort,
		ONUID:     onuID,
		Timestamp: time.Now(),
	}

	// Extract optical power from metadata
	if rxPower, ok := stats.Metadata["rx_power_dbm"]; ok {
		if rxVal, ok := rxPower.(float64); ok {
			reading.RxPowerDBm = rxVal
		}
	}

	if txPower, ok := stats.Metadata["tx_power_dbm"]; ok {
		if txVal, ok := txPower.(float64); ok {
			reading.TxPowerDBm = txVal
		}
	}

	// Check if readings are within spec
	reading.IsWithinSpec = types.IsPowerWithinSpec(reading.RxPowerDBm, reading.TxPowerDBm)

	return reading, nil
}

// GetONUDistance returns estimated fiber distance to ONU in meters.
func (a *Adapter) GetONUDistance(ctx context.Context, ponPort string, onuID int) (int, error) {
	if a.snmpExecutor == nil {
		return -1, fmt.Errorf("SNMP executor not available - Huawei requires SNMP for distance query")
	}

	// Parse PON port (format: frame/slot/port)
	parts := strings.Split(ponPort, "/")
	if len(parts) != 3 {
		return -1, fmt.Errorf("invalid PON port format: %s (expected frame/slot/port)", ponPort)
	}

	frame, err := strconv.Atoi(parts[0])
	if err != nil {
		return -1, fmt.Errorf("invalid frame number: %s", parts[0])
	}
	slot, err := strconv.Atoi(parts[1])
	if err != nil {
		return -1, fmt.Errorf("invalid slot number: %s", parts[1])
	}
	port, err := strconv.Atoi(parts[2])
	if err != nil {
		return -1, fmt.Errorf("invalid port number: %s", parts[2])
	}

	// Build SNMP index (Huawei format: portIndex.onuID)
	// portIndex = (frame << 16) | (slot << 8) | port
	portIndex := (frame << 16) | (slot << 8) | port
	snmpIndex := fmt.Sprintf("%d.%d", portIndex, onuID)

	oid := fmt.Sprintf("%s.%s", OIDOnuDistance, snmpIndex)
	result, err := a.snmpExecutor.GetSNMP(ctx, oid)
	if err != nil {
		return -1, fmt.Errorf("failed to get ONU distance: %w", err)
	}

	// Parse result - distance is in meters
	if dist, ok := result.(int64); ok {
		return int(dist), nil
	}
	if dist, ok := result.(int); ok {
		return dist, nil
	}
	if dist, ok := result.(uint64); ok {
		return int(dist), nil
	}

	return -1, nil
}

// RestartONU triggers a reboot of the specified ONU.
func (a *Adapter) RestartONU(ctx context.Context, ponPort string, onuID int) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	// Parse PON port (format: frame/slot/port, e.g., "0/0/1")
	parts := strings.Split(ponPort, "/")
	if len(parts) != 3 {
		return fmt.Errorf("invalid PON port format: %s (expected frame/slot/port)", ponPort)
	}

	frame, _ := strconv.Atoi(parts[0])
	slot, _ := strconv.Atoi(parts[1])
	port, _ := strconv.Atoi(parts[2])

	commands := []string{
		"enable",
		"config",
		fmt.Sprintf("interface gpon %d/%d", frame, slot),
		fmt.Sprintf("ont reset %d %d", port, onuID),
		"quit",
		"quit",
		"quit",
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	return err
}

// ApplyProfile applies a bandwidth/service profile to an ONU.
func (a *Adapter) ApplyProfile(ctx context.Context, ponPort string, onuID int, profile *types.ONUProfile) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available - Huawei requires CLI for profile management")
	}

	if profile == nil {
		return fmt.Errorf("profile cannot be nil")
	}

	// Parse PON port (format: frame/slot/port)
	parts := strings.Split(ponPort, "/")
	if len(parts) != 3 {
		return fmt.Errorf("invalid PON port format: %s (expected frame/slot/port)", ponPort)
	}

	frame, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid frame number: %s", parts[0])
	}
	slot, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid slot number: %s", parts[1])
	}
	port, err := strconv.Atoi(parts[2])
	if err != nil {
		return fmt.Errorf("invalid port number: %s", parts[2])
	}

	commands := []string{
		"enable",
		"config",
		fmt.Sprintf("interface gpon %d/%d", frame, slot),
	}

	// Update line profile if specified
	if profile.LineProfile != "" {
		// Try to use profile name directly (ont modify supports both name and ID)
		commands = append(commands,
			fmt.Sprintf("ont modify %d %d ont-lineprofile-name %s", port, onuID, profile.LineProfile))
	}

	// Update service profile if specified
	if profile.ServiceProfile != "" {
		commands = append(commands,
			fmt.Sprintf("ont modify %d %d ont-srvprofile-name %s", port, onuID, profile.ServiceProfile))
	}

	// Update VLAN if specified
	if profile.VLAN > 0 {
		priority := profile.Priority
		if priority < 0 || priority > 7 {
			priority = 0
		}
		commands = append(commands,
			fmt.Sprintf("ont port native-vlan %d %d eth 1 vlan %d priority %d", port, onuID, profile.VLAN, priority))
	}

	commands = append(commands, "quit")

	// Apply bandwidth/traffic profile if specified
	if profile.BandwidthDown > 0 || profile.BandwidthUp > 0 {
		// Look up traffic table ID from profile metadata or use bandwidth-based ID
		trafficTableID := 1
		if profile.Metadata != nil {
			if id, ok := profile.Metadata["traffic_table_id"].(int); ok {
				trafficTableID = id
			}
		}
		// Fallback: use bandwidth in Mbps as table ID (common Huawei convention)
		if trafficTableID == 1 && profile.BandwidthDown > 0 {
			trafficTableID = profile.BandwidthDown / 1000 // Convert kbps to Mbps
			if trafficTableID == 0 {
				trafficTableID = 1
			}
		}

		commands = append(commands,
			fmt.Sprintf("interface gpon %d/%d", frame, slot),
			fmt.Sprintf("ont traffic-policy %d %d profile-id %d", port, onuID, trafficTableID),
			"quit")
	}

	commands = append(commands, "quit")

	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		output := strings.Join(outputs, "\n")
		// Check for common errors
		if strings.Contains(output, "does not exist") || strings.Contains(err.Error(), "does not exist") {
			return &types.HumanError{
				Code:    types.ErrCodeONUNotFound,
				Message: fmt.Sprintf("ONT %d on port %s not found", onuID, ponPort),
				Vendor:  "huawei",
				Raw:     output,
			}
		}
		if strings.Contains(output, "profile") && strings.Contains(output, "not exist") {
			return &types.HumanError{
				Code:    types.ErrCodeProfileNotFound,
				Message: fmt.Sprintf("Profile not found: %s / %s", profile.LineProfile, profile.ServiceProfile),
				Vendor:  "huawei",
				Raw:     output,
			}
		}
		return fmt.Errorf("failed to apply profile: %w", err)
	}

	return nil
}

// BulkProvision provisions multiple ONUs in a single session.
func (a *Adapter) BulkProvision(ctx context.Context, operations []types.BulkProvisionOp) (*types.BulkResult, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - Huawei requires CLI for provisioning")
	}

	result := &types.BulkResult{
		Results: make([]types.BulkOpResult, len(operations)),
	}

	for i, op := range operations {
		opResult := types.BulkOpResult{
			Serial:   op.Serial,
			PONPort:  op.PONPort,
			ONUID:    op.ONUID,
			Metadata: make(map[string]interface{}),
		}

		// Build subscriber from operation
		subscriber := &model.Subscriber{
			Name:        fmt.Sprintf("bulk-%s", op.Serial),
			Annotations: make(map[string]string),
			Spec: model.SubscriberSpec{
				ONUSerial: op.Serial,
			},
		}

		// Set PON port annotation
		if op.PONPort != "" {
			subscriber.Annotations["nanoncore.com/gpon-fsp"] = op.PONPort
		}

		// Set ONU ID annotation
		if op.ONUID > 0 {
			subscriber.Annotations["nanoncore.com/ont-id"] = strconv.Itoa(op.ONUID)
		}

		// Build tier from profile
		var tier *model.ServiceTier
		if op.Profile != nil {
			subscriber.Spec.VLAN = op.Profile.VLAN

			tier = &model.ServiceTier{
				Name:        fmt.Sprintf("bulk-tier-%s", op.Serial),
				Annotations: make(map[string]string),
				Spec: model.ServiceTierSpec{
					BandwidthDown: op.Profile.BandwidthDown / 1000, // Convert kbps to Mbps
					BandwidthUp:   op.Profile.BandwidthUp / 1000,
				},
			}

			// Set profile annotations if specified
			if op.Profile.LineProfile != "" {
				tier.Annotations["nanoncore.com/line-profile-name"] = op.Profile.LineProfile
			}
			if op.Profile.ServiceProfile != "" {
				tier.Annotations["nanoncore.com/srv-profile-name"] = op.Profile.ServiceProfile
			}
		} else {
			// Default tier with no bandwidth limits
			tier = &model.ServiceTier{
				Name: fmt.Sprintf("bulk-tier-%s", op.Serial),
				Spec: model.ServiceTierSpec{
					BandwidthDown: 100, // Default 100 Mbps
					BandwidthUp:   50,  // Default 50 Mbps
				},
			}
		}

		// Call CreateSubscriber
		subResult, err := a.CreateSubscriber(ctx, subscriber, tier)
		if err != nil {
			opResult.Success = false
			opResult.Error = err.Error()

			// Map error to code
			switch {
			case strings.Contains(err.Error(), "exists") || strings.Contains(err.Error(), "already"):
				opResult.ErrorCode = types.ErrCodeONUExists
			case strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist"):
				opResult.ErrorCode = types.ErrCodeONUNotFound
			case strings.Contains(err.Error(), "timeout"):
				opResult.ErrorCode = types.ErrCodeTimeout
			case strings.Contains(err.Error(), "serial"):
				opResult.ErrorCode = types.ErrCodeInvalidSerial
			default:
				opResult.ErrorCode = types.ErrCodeUnknown
			}

			result.Failed++
		} else {
			opResult.Success = true
			result.Succeeded++

			// Extract ONU ID from result metadata
			if subResult != nil && subResult.Metadata != nil {
				if ontID, ok := subResult.Metadata["ont_id"].(int); ok {
					opResult.ONUID = ontID
				}
				opResult.Metadata["session_id"] = subResult.SessionID
				opResult.Metadata["interface"] = subResult.InterfaceName
			}
		}

		result.Results[i] = opResult
	}

	return result, nil
}

// GetAlarms returns active alarms from the OLT.
func (a *Adapter) GetAlarms(ctx context.Context) ([]types.OLTAlarm, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - Huawei requires CLI for alarm query")
	}

	output, err := a.cliExecutor.ExecCommand(ctx, "display alarm active all")
	if err != nil {
		return nil, fmt.Errorf("failed to get alarms: %w", err)
	}

	return a.parseAlarms(output), nil
}

// parseAlarms parses Huawei CLI output for active alarms.
// Huawei alarm format varies by model, but typically:
// Alarm ID   Severity   Type         Source            Time                    Description
// 12345      Critical   LOS          0/0/1:5           2024-01-15 10:30:00    Loss of signal
func (a *Adapter) parseAlarms(output string) []types.OLTAlarm {
	alarms := []types.OLTAlarm{}

	lines := strings.Split(output, "\n")
	inTable := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and detect table start
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "-") {
			inTable = true
			continue
		}
		// Skip header lines
		if strings.HasPrefix(line, "Alarm") || strings.HasPrefix(line, "alarm") ||
			strings.Contains(line, "Severity") || strings.Contains(line, "Total") ||
			strings.Contains(line, "No alarm") {
			continue
		}

		if !inTable {
			continue
		}

		// Parse alarm line
		// Try to extract fields - format varies by Huawei model
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		alarm := types.OLTAlarm{
			ID:       fields[0],
			Metadata: make(map[string]interface{}),
		}

		// Map severity
		if len(fields) >= 2 {
			severity := strings.ToLower(fields[1])
			switch {
			case strings.Contains(severity, "critical"):
				alarm.Severity = "critical"
			case strings.Contains(severity, "major"):
				alarm.Severity = "major"
			case strings.Contains(severity, "minor"):
				alarm.Severity = "minor"
			case strings.Contains(severity, "warning"):
				alarm.Severity = "warning"
			default:
				alarm.Severity = "unknown"
			}
		}

		// Map alarm type
		if len(fields) >= 3 {
			alarmType := strings.ToLower(fields[2])
			switch {
			case strings.Contains(alarmType, "los"):
				alarm.Type = "los"
				alarm.Source = "onu"
			case strings.Contains(alarmType, "power"):
				alarm.Type = "power"
				alarm.Source = "onu"
			case strings.Contains(alarmType, "dying"):
				alarm.Type = "dying_gasp"
				alarm.Source = "onu"
			case strings.Contains(alarmType, "config"):
				alarm.Type = "config"
				alarm.Source = "system"
			case strings.Contains(alarmType, "link"):
				alarm.Type = "link"
				alarm.Source = "port"
			default:
				alarm.Type = alarmType
				alarm.Source = "unknown"
			}
		}

		// Extract source ID (e.g., PON port or ONU identifier)
		if len(fields) >= 4 {
			alarm.SourceID = fields[3]
			// Try to determine source type from ID format
			if strings.Contains(alarm.SourceID, ":") {
				// Format: 0/0/1:5 suggests ONU
				alarm.Source = "onu"
			} else if strings.Contains(alarm.SourceID, "/") {
				// Format: 0/0/1 suggests port
				alarm.Source = "port"
			}
		}

		// Try to parse timestamp (format: YYYY-MM-DD HH:MM:SS)
		for i := 4; i < len(fields)-1; i++ {
			if len(fields[i]) == 10 && strings.Contains(fields[i], "-") {
				// Found date part
				if i+1 < len(fields) && strings.Contains(fields[i+1], ":") {
					timeStr := fields[i] + " " + fields[i+1]
					if t, err := time.Parse("2006-01-02 15:04:05", timeStr); err == nil {
						alarm.RaisedAt = t
					}
					// Collect remaining fields as message
					if i+2 < len(fields) {
						alarm.Message = strings.Join(fields[i+2:], " ")
					}
					break
				}
			}
		}

		// If no timestamp found, use current time and build message from remaining fields
		if alarm.RaisedAt.IsZero() {
			alarm.RaisedAt = time.Now()
			if len(fields) >= 5 {
				alarm.Message = strings.Join(fields[4:], " ")
			}
		}

		alarm.Metadata["raw_line"] = line

		alarms = append(alarms, alarm)
	}

	return alarms
}

// ListPorts returns status for all PON ports on the OLT.
// Uses SNMP to query interface status and counts ONUs per port.
func (a *Adapter) ListPorts(ctx context.Context) ([]*types.PONPortStatus, error) {
	if a.snmpExecutor == nil {
		return nil, fmt.Errorf("SNMP executor not available - Huawei requires SNMP for port listing")
	}

	// Walk interface descriptions to identify PON/GPON ports
	descrResults, err := a.snmpExecutor.WalkSNMP(ctx, OIDIfDescr)
	if err != nil {
		return nil, fmt.Errorf("failed to walk interface descriptions: %w", err)
	}

	// Walk admin status
	adminResults, err := a.snmpExecutor.WalkSNMP(ctx, OIDIfAdminStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to walk admin status: %w", err)
	}

	// Walk oper status
	operResults, err := a.snmpExecutor.WalkSNMP(ctx, OIDIfOperStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to walk oper status: %w", err)
	}

	// Walk interface aliases (port descriptions)
	aliasResults, err := a.snmpExecutor.WalkSNMP(ctx, OIDIfAlias)
	if err != nil {
		// Non-fatal, continue without aliases
		aliasResults = make(map[string]interface{})
	}

	// Get ONU list to count ONUs per port
	onus, err := a.GetONUList(ctx, nil)
	if err != nil {
		// Non-fatal, continue without ONU counts
		onus = []types.ONUInfo{}
	}

	// Count ONUs per port
	onuCountByPort := make(map[string]int)
	for _, onu := range onus {
		onuCountByPort[onu.PONPort]++
	}

	// Build port list
	ports := make([]*types.PONPortStatus, 0)

	for index, descrVal := range descrResults {
		descr, ok := descrVal.(string)
		if !ok {
			continue
		}

		// Filter to PON/GPON ports only
		descrLower := strings.ToLower(descr)
		if !strings.Contains(descrLower, "gpon") && !strings.Contains(descrLower, "pon") {
			continue
		}

		// Parse port identifier from description (e.g., "GPON 0/0/1")
		portID := a.parsePortFromDescr(descr)
		if portID == "" {
			portID = descr // Use description as-is if parsing fails
		}

		port := &types.PONPortStatus{
			Port:       portID,
			AdminState: "unknown",
			OperState:  "unknown",
			ONUCount:   onuCountByPort[portID],
			MaxONUs:    128, // Typical GPON limit
		}

		// Get admin status (1=up, 2=down, 3=testing)
		if adminVal, ok := adminResults[index]; ok {
			if adminInt := toInt(adminVal); adminInt >= 0 {
				switch adminInt {
				case 1:
					port.AdminState = "enabled"
				case 2:
					port.AdminState = "disabled"
				default:
					port.AdminState = "testing"
				}
			}
		}

		// Get oper status (1=up, 2=down, etc.)
		if operVal, ok := operResults[index]; ok {
			if operInt := toInt(operVal); operInt >= 0 {
				switch operInt {
				case 1:
					port.OperState = "up"
				case 2:
					port.OperState = "down"
				default:
					port.OperState = "unknown"
				}
			}
		}

		// Get port description/alias
		if aliasVal, ok := aliasResults[index]; ok {
			if aliasStr, ok := aliasVal.(string); ok {
				port.Description = aliasStr
			}
		}

		ports = append(ports, port)
	}

	return ports, nil
}

// parsePortFromDescr extracts port identifier from interface description.
// Example: "GPON 0/0/1" -> "0/0/1"
func (a *Adapter) parsePortFromDescr(descr string) string {
	// Try to extract frame/slot/port pattern
	re := regexp.MustCompile(`(\d+)/(\d+)/(\d+)`)
	if match := re.FindStringSubmatch(descr); len(match) == 4 {
		return fmt.Sprintf("%s/%s/%s", match[1], match[2], match[3])
	}
	return ""
}

// SetPortState enables or disables a PON port administratively.
// Uses CLI to execute shutdown/undo shutdown commands.
func (a *Adapter) SetPortState(ctx context.Context, port string, enabled bool) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available - Huawei requires CLI for port management")
	}

	// Parse PON port (format: frame/slot/port, e.g., "0/0/1")
	parts := strings.Split(port, "/")
	if len(parts) != 3 {
		return fmt.Errorf("invalid PON port format: %s (expected frame/slot/port)", port)
	}

	frame, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid frame number: %s", parts[0])
	}
	slot, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid slot number: %s", parts[1])
	}
	portNum, err := strconv.Atoi(parts[2])
	if err != nil {
		return fmt.Errorf("invalid port number: %s", parts[2])
	}

	// Build CLI commands
	var portCmd string
	if enabled {
		portCmd = fmt.Sprintf("undo port %d shutdown", portNum)
	} else {
		portCmd = fmt.Sprintf("port %d shutdown", portNum)
	}

	commands := []string{
		"enable",
		"config",
		fmt.Sprintf("interface gpon %d/%d", frame, slot),
		portCmd,
		"quit",
		"quit",
		"quit",
	}

	_, err = a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		action := "enable"
		if !enabled {
			action = "disable"
		}
		return fmt.Errorf("failed to %s port %s: %w", action, port, err)
	}

	return nil
}

// toInt converts various integer types to int. Returns -1 if conversion fails.
// Handles int, int64, uint, uint64 from SNMP responses.
func toInt(val interface{}) int {
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint64:
		return int(v)
	default:
		return -1
	}
}

// ==================== VLAN Management Methods ====================

// ListVLANs returns all configured VLANs on the OLT.
func (a *Adapter) ListVLANs(ctx context.Context) ([]types.VLANInfo, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available - Huawei requires CLI for VLAN listing")
	}

	cmd := "display vlan all"
	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list VLANs: %w", err)
	}

	return a.parseVLANList(output), nil
}

// parseVLANList parses Huawei CLI output for VLAN list.
func (a *Adapter) parseVLANList(output string) []types.VLANInfo {
	vlans := []types.VLANInfo{}

	lines := strings.Split(output, "\n")
	inTable := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and headers
		if line == "" || strings.HasPrefix(line, "-") {
			if strings.HasPrefix(line, "-") {
				inTable = true
			}
			continue
		}
		if strings.HasPrefix(line, "VLAN ID") || strings.HasPrefix(line, "Total") {
			continue
		}

		if !inTable {
			continue
		}

		// Parse VLAN line: "100       Customer_VLAN_100          smart     5               Customer traffic"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		vlanID, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		vlan := types.VLANInfo{
			ID:   vlanID,
			Name: fields[1],
			Type: "smart",
		}

		if len(fields) >= 3 {
			vlan.Type = fields[2]
		}
		if len(fields) >= 4 {
			vlan.ServicePortCount, _ = strconv.Atoi(fields[3])
		}
		if len(fields) >= 5 {
			vlan.Description = strings.Join(fields[4:], " ")
		}

		vlans = append(vlans, vlan)
	}

	return vlans
}

// GetVLAN retrieves a specific VLAN by ID.
func (a *Adapter) GetVLAN(ctx context.Context, vlanID int) (*types.VLANInfo, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	cmd := fmt.Sprintf("display vlan %d", vlanID)
	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get VLAN %d: %w", vlanID, err)
	}

	// Check if VLAN doesn't exist
	if strings.Contains(output, "does not exist") || strings.Contains(output, "Error") {
		return nil, nil
	}

	// Parse single VLAN output
	vlan := &types.VLANInfo{
		ID:   vlanID,
		Type: "smart",
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				vlan.Name = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "Description") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				vlan.Description = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "Service Port Count") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				vlan.ServicePortCount, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			}
		} else if strings.HasPrefix(line, "Type") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				vlan.Type = strings.TrimSpace(parts[1])
			}
		}
	}

	return vlan, nil
}

// CreateVLAN creates a new VLAN on the OLT.
func (a *Adapter) CreateVLAN(ctx context.Context, req *types.CreateVLANRequest) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	// Validate VLAN ID range
	if req.ID < 1 || req.ID > 4094 {
		return &types.HumanError{
			Code:    types.ErrCodeInvalidVLANID,
			Message: fmt.Sprintf("VLAN ID %d is out of range (1-4094)", req.ID),
			Vendor:  "huawei",
		}
	}

	// Build commands
	commands := []string{
		"enable",
		"config",
		fmt.Sprintf("vlan %d smart", req.ID),
	}

	if req.Name != "" {
		commands = append(commands, fmt.Sprintf("name %s", req.Name))
	}
	if req.Description != "" {
		commands = append(commands, fmt.Sprintf("description %s", req.Description))
	}

	commands = append(commands, "quit", "quit")

	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	output := strings.Join(outputs, "\n")
	if err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(output, "already exists") {
			return &types.HumanError{
				Code:    types.ErrCodeVLANExists,
				Message: fmt.Sprintf("VLAN %d already exists", req.ID),
				Vendor:  "huawei",
			}
		}
		return fmt.Errorf("failed to create VLAN: %w", err)
	}

	// Check output for errors
	if strings.Contains(output, "Error") || strings.Contains(output, "already exists") {
		return &types.HumanError{
			Code:    types.ErrCodeVLANExists,
			Message: fmt.Sprintf("VLAN %d already exists", req.ID),
			Vendor:  "huawei",
			Raw:     output,
		}
	}

	return nil
}

// DeleteVLAN removes a VLAN from the OLT.
func (a *Adapter) DeleteVLAN(ctx context.Context, vlanID int, force bool) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	// Check if VLAN exists and has service ports
	vlan, err := a.GetVLAN(ctx, vlanID)
	if err != nil {
		return err
	}
	if vlan == nil {
		return &types.HumanError{
			Code:    types.ErrCodeVLANNotFound,
			Message: fmt.Sprintf("VLAN %d not found", vlanID),
			Vendor:  "huawei",
		}
	}

	if vlan.ServicePortCount > 0 && !force {
		return &types.HumanError{
			Code:    types.ErrCodeVLANHasServicePorts,
			Message: fmt.Sprintf("VLAN %d has %d service port(s) configured", vlanID, vlan.ServicePortCount),
			Action:  "Use --force to delete anyway, or remove service ports first",
			Vendor:  "huawei",
		}
	}

	commands := []string{
		"enable",
		"config",
		fmt.Sprintf("undo vlan %d", vlanID),
		"quit",
	}

	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	output := strings.Join(outputs, "\n")
	if err != nil {
		return fmt.Errorf("failed to delete VLAN: %w", err)
	}

	// Check for warning about service ports
	if strings.Contains(output, "service port") && !force {
		return &types.HumanError{
			Code:    types.ErrCodeVLANHasServicePorts,
			Message: fmt.Sprintf("VLAN %d has service ports configured", vlanID),
			Action:  "Use --force to delete anyway",
			Vendor:  "huawei",
			Raw:     output,
		}
	}

	return nil
}

// ==================== Service Port Management Methods ====================

// ListServicePorts returns all service port configurations.
func (a *Adapter) ListServicePorts(ctx context.Context) ([]types.ServicePort, error) {
	if a.cliExecutor == nil {
		return nil, fmt.Errorf("CLI executor not available")
	}

	cmd := "display service-port all"
	output, err := a.cliExecutor.ExecCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list service ports: %w", err)
	}

	return a.parseServicePortList(output), nil
}

// parseServicePortList parses Huawei CLI output for service port list.
func (a *Adapter) parseServicePortList(output string) []types.ServicePort {
	servicePorts := []types.ServicePort{}

	lines := strings.Split(output, "\n")
	inTable := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and headers
		if line == "" || strings.HasPrefix(line, "-") {
			if strings.HasPrefix(line, "-") {
				inTable = true
			}
			continue
		}
		if strings.HasPrefix(line, "Index") || strings.HasPrefix(line, "Total") || strings.Contains(line, "No service") {
			continue
		}

		if !inTable {
			continue
		}

		// Parse line: "1       100     0/0/1           101     1         100          translate"
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		index, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		vlan, _ := strconv.Atoi(fields[1])
		ontID, _ := strconv.Atoi(fields[3])
		gemport, _ := strconv.Atoi(fields[4])

		sp := types.ServicePort{
			Index:     index,
			VLAN:      vlan,
			Interface: fields[2],
			ONTID:     ontID,
			GemPort:   gemport,
		}

		if len(fields) >= 6 {
			sp.UserVLAN, _ = strconv.Atoi(fields[5])
		}
		if len(fields) >= 7 {
			sp.TagTransform = fields[6]
		}

		servicePorts = append(servicePorts, sp)
	}

	return servicePorts
}

// AddServicePort creates a service port mapping.
func (a *Adapter) AddServicePort(ctx context.Context, req *types.AddServicePortRequest) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	// Default values
	gemPort := req.GemPort
	if gemPort == 0 {
		gemPort = 1
	}
	userVLAN := req.UserVLAN
	if userVLAN == 0 {
		userVLAN = req.VLAN
	}

	// Build command
	cmd := fmt.Sprintf(
		"service-port vlan %d gpon %s ont %d gemport %d multi-service user-vlan %d",
		req.VLAN, req.PONPort, req.ONTID, gemPort, userVLAN,
	)

	commands := []string{
		"enable",
		"config",
		cmd,
		"quit",
	}

	outputs, err := a.cliExecutor.ExecCommands(ctx, commands)
	output := strings.Join(outputs, "\n")
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return &types.HumanError{
				Code:    types.ErrCodeONUNotFound,
				Message: fmt.Sprintf("ONT %d on port %s not found", req.ONTID, req.PONPort),
				Vendor:  "huawei",
			}
		}
		if strings.Contains(err.Error(), "VLAN") && strings.Contains(err.Error(), "not exist") {
			return &types.HumanError{
				Code:    types.ErrCodeVLANNotFound,
				Message: fmt.Sprintf("VLAN %d does not exist", req.VLAN),
				Vendor:  "huawei",
			}
		}
		return fmt.Errorf("failed to add service port: %w", err)
	}

	// Check output for errors
	if strings.Contains(output, "Error") || strings.Contains(output, "does not exist") {
		if strings.Contains(output, "ONT") || strings.Contains(output, "ont") {
			return &types.HumanError{
				Code:    types.ErrCodeONUNotFound,
				Message: fmt.Sprintf("ONT %d on port %s not found", req.ONTID, req.PONPort),
				Vendor:  "huawei",
				Raw:     output,
			}
		}
		if strings.Contains(output, "VLAN") || strings.Contains(output, "vlan") {
			return &types.HumanError{
				Code:    types.ErrCodeVLANNotFound,
				Message: fmt.Sprintf("VLAN %d does not exist", req.VLAN),
				Vendor:  "huawei",
				Raw:     output,
			}
		}
	}

	return nil
}

// DeleteServicePort removes a service port mapping.
func (a *Adapter) DeleteServicePort(ctx context.Context, ponPort string, ontID int) error {
	if a.cliExecutor == nil {
		return fmt.Errorf("CLI executor not available")
	}

	cmd := fmt.Sprintf("undo service-port port %s ont %d", ponPort, ontID)

	commands := []string{
		"enable",
		"config",
		cmd,
		"quit",
	}

	_, err := a.cliExecutor.ExecCommands(ctx, commands)
	if err != nil {
		return fmt.Errorf("failed to delete service port: %w", err)
	}

	return nil
}
