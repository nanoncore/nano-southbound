package huawei

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

// Adapter wraps a base driver with Huawei-specific logic
// Huawei OLTs (MA5600T/MA5800-X series) primarily use CLI + SNMP
type Adapter struct {
	baseDriver   types.Driver
	cliExecutor  types.CLIExecutor
	snmpExecutor types.SNMPExecutor
	config       *types.EquipmentConfig
}

// NewAdapter creates a new Huawei adapter
func NewAdapter(baseDriver types.Driver, config *types.EquipmentConfig) types.Driver {
	adapter := &Adapter{
		baseDriver: baseDriver,
		config:     config,
	}

	// Check if base driver supports CLI execution
	if executor, ok := baseDriver.(types.CLIExecutor); ok {
		adapter.cliExecutor = executor
	}

	// Check if base driver supports SNMP execution
	if executor, ok := baseDriver.(types.SNMPExecutor); ok {
		adapter.snmpExecutor = executor
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
			if v != SNMPInvalidValue {
				stats.Metadata["tx_power_dbm"] = ConvertOpticalPower(v)
			}
		}
	}
	if val, ok := results[temperatureOID]; ok {
		if v, ok := val.(int64); ok {
			if v != SNMPInvalidValue {
				stats.Metadata["temperature_c"] = v
			}
		}
	}
	if val, ok := results[voltageOID]; ok {
		if v, ok := val.(int64); ok {
			if v != SNMPInvalidValue {
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
	Temperature int64   `json:"temperature_c"`
	Voltage     float64 `json:"voltage_v"`
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
		slot, port, onuID := ParseONUIndex(index)

		onu := ONTStats{
			Index:  index,
			Serial: serial,
			Frame:  0, // Usually 0 for single-frame OLTs
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
				if txRaw != SNMPInvalidValue {
					onu.TxPower = ConvertOpticalPower(txRaw)
				}
			}
		}

		// Temperature
		if tempVal, ok := temperatures[index]; ok {
			if tempRaw, ok := tempVal.(int64); ok {
				if tempRaw != SNMPInvalidValue {
					onu.Temperature = tempRaw
				}
			}
		}

		// Voltage
		if voltVal, ok := voltages[index]; ok {
			if voltRaw, ok := voltVal.(int64); ok {
				if voltRaw != SNMPInvalidValue {
					onu.Voltage = ConvertVoltage(voltRaw)
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

			frame, _ := strconv.Atoi(fspParts[0])
			slot, _ := strconv.Atoi(fspParts[1])
			port, _ := strconv.Atoi(fspParts[2])

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
	// Default values
	frame = 0
	slot = 1
	port = 0

	// Check subscriber annotations
	if subscriber.Annotations == nil {
		return
	}

	// nanoncore.com/gpon-fsp: "0/1/0"
	if fsp, ok := subscriber.Annotations["nanoncore.com/gpon-fsp"]; ok {
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
	// Check subscriber annotations
	if subscriber.Annotations != nil {
		if idStr, ok := subscriber.Annotations["nanoncore.com/ont-id"]; ok {
			if id, err := strconv.Atoi(idStr); err == nil {
				return id
			}
		}
	}
	// Generate from VLAN as fallback (max 128 ONTs per port)
	return subscriber.Spec.VLAN % 128
}

// getLineProfileID returns the line profile ID for a service tier
func (a *Adapter) getLineProfileID(tier *model.ServiceTier) int {
	// Check tier annotations for custom profile ID
	if tier.Annotations != nil {
		if idStr, ok := tier.Annotations["nanoncore.com/line-profile-id"]; ok {
			if id, err := strconv.Atoi(idStr); err == nil {
				return id
			}
		}
	}
	// Default profile ID (1 is typically the default/basic profile)
	return 1
}

// getServiceProfileID returns the service profile ID for a service tier
func (a *Adapter) getServiceProfileID(tier *model.ServiceTier) int {
	// Check tier annotations for custom profile ID
	if tier.Annotations != nil {
		if idStr, ok := tier.Annotations["nanoncore.com/srv-profile-id"]; ok {
			if id, err := strconv.Atoi(idStr); err == nil {
				return id
			}
		}
	}
	// Default service profile ID
	return 1
}

// getTrafficTableID returns the traffic table ID for a service tier
func (a *Adapter) getTrafficTableID(tier *model.ServiceTier) int {
	// Check tier annotations for custom traffic table ID
	if tier.Annotations != nil {
		if idStr, ok := tier.Annotations["nanoncore.com/traffic-table-id"]; ok {
			if id, err := strconv.Atoi(idStr); err == nil {
				return id
			}
		}
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
