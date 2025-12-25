package adtran

import (
	"context"
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nanoncore/nano-southbound/drivers/netconf"
	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
)

// Adapter wraps a base driver with Adtran-specific logic
// Adtran SDX series uses NETCONF/YANG for OLT management
type Adapter struct {
	baseDriver      types.Driver
	netconfExecutor netconf.NETCONFExecutor
	config          *types.EquipmentConfig
}

// NewAdapter creates a new Adtran adapter
func NewAdapter(baseDriver types.Driver, config *types.EquipmentConfig) types.Driver {
	adapter := &Adapter{
		baseDriver: baseDriver,
		config:     config,
	}

	// Check if base driver supports NETCONF operations
	if executor, ok := baseDriver.(netconf.NETCONFExecutor); ok {
		adapter.netconfExecutor = executor
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

// CreateSubscriber provisions an ONT on the Adtran OLT
func (a *Adapter) CreateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) (*types.SubscriberResult, error) {
	if a.netconfExecutor == nil {
		return nil, fmt.Errorf("NETCONF executor not available - Adtran requires NETCONF driver")
	}

	// Extract subscriber parameters
	params := a.extractSubscriberParams(subscriber, tier)

	// Build ONT provisioning configuration
	config := a.buildONTConfig(params)

	// Apply configuration via NETCONF edit-config
	err := a.netconfExecutor.EditConfig(ctx, "", config,
		netconf.WithMerge(),
		netconf.WithRollbackOnError(),
	)
	if err != nil {
		return nil, fmt.Errorf("Adtran ONT provisioning failed: %w", err)
	}

	// Build result
	result := &types.SubscriberResult{
		SubscriberID:  subscriber.Name,
		SessionID:     fmt.Sprintf("adtran-%s", params.SerialNumber),
		AssignedIP:    subscriber.Spec.IPAddress,
		AssignedIPv6:  subscriber.Spec.IPv6Address,
		InterfaceName: fmt.Sprintf("pon-%s/ont-%d", params.PONPort, params.ONTID),
		VLAN:          subscriber.Spec.VLAN,
		Metadata: map[string]interface{}{
			"vendor":            "adtran",
			"model":             a.detectModel(),
			"pon_port":          params.PONPort,
			"ont_id":            params.ONTID,
			"serial_number":     params.SerialNumber,
			"ont_profile":       params.ONTProfile,
			"service_profile":   params.ServiceProfile,
			"bandwidth_profile": params.BandwidthProfile,
		},
	}

	return result, nil
}

// subscriberParams holds parsed subscriber parameters for Adtran
type subscriberParams struct {
	SerialNumber     string
	ONTID            int
	PONPort          string
	VLAN             int
	Description      string
	ONTProfile       string
	ServiceProfile   string
	BandwidthProfile string
	BandwidthUp      int
	BandwidthDown    int
}

// extractSubscriberParams extracts parameters from Subscriber and ServiceTier
func (a *Adapter) extractSubscriberParams(subscriber *model.Subscriber, tier *model.ServiceTier) *subscriberParams {
	params := &subscriberParams{
		SerialNumber: subscriber.Spec.ONUSerial,
		VLAN:         subscriber.Spec.VLAN,
		Description:  fmt.Sprintf("Nanoncore subscriber %s", subscriber.Name),
	}

	// Get PON port from metadata or annotations
	if ponPort, ok := a.config.Metadata["pon_port"]; ok {
		params.PONPort = ponPort
	} else {
		params.PONPort = "gpon-0/0/1"
	}

	if subscriber.Annotations != nil {
		if ponPort, ok := subscriber.Annotations["nanoncore.com/pon-port"]; ok {
			params.PONPort = ponPort
		}
		if ontID, ok := subscriber.Annotations["nanoncore.com/ont-id"]; ok {
			if id, err := strconv.Atoi(ontID); err == nil {
				params.ONTID = id
			}
		}
	}

	// Generate ONT ID from VLAN if not specified
	if params.ONTID == 0 {
		params.ONTID = subscriber.Spec.VLAN % 128
	}

	// Get profile names from tier or use defaults
	if tier != nil {
		params.BandwidthUp = tier.Spec.BandwidthUp
		params.BandwidthDown = tier.Spec.BandwidthDown

		if tier.Annotations != nil {
			if ontProfile, ok := tier.Annotations["nanoncore.com/ont-profile"]; ok {
				params.ONTProfile = ontProfile
			}
			if svcProfile, ok := tier.Annotations["nanoncore.com/service-profile"]; ok {
				params.ServiceProfile = svcProfile
			}
			if bwProfile, ok := tier.Annotations["nanoncore.com/bandwidth-profile"]; ok {
				params.BandwidthProfile = bwProfile
			}
		}
	}

	// Default profiles based on bandwidth
	if params.ONTProfile == "" {
		params.ONTProfile = "nanoncore-ont-default"
	}
	if params.ServiceProfile == "" {
		params.ServiceProfile = fmt.Sprintf("nanoncore-svc-%dM", params.BandwidthDown)
	}
	if params.BandwidthProfile == "" {
		params.BandwidthProfile = fmt.Sprintf("nanoncore-bw-%dM", params.BandwidthDown)
	}

	return params
}

// buildONTConfig builds Adtran YANG XML for ONT provisioning
func (a *Adapter) buildONTConfig(params *subscriberParams) string {
	return fmt.Sprintf(`
<config xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <ont xmlns="http://www.adtran.com/ns/yang/adtran-ont">
    <serial-number>%s</serial-number>
    <ont-id>%d</ont-id>
    <pon-port>%s</pon-port>
    <admin-state>enabled</admin-state>
    <description>%s</description>
    <ont-profile>%s</ont-profile>
    <service-profile>%s</service-profile>
  </ont>
</config>`,
		params.SerialNumber,
		params.ONTID,
		params.PONPort,
		params.Description,
		params.ONTProfile,
		params.ServiceProfile,
	)
}

// UpdateSubscriber updates subscriber configuration
func (a *Adapter) UpdateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	// For Adtran, update is same as create with merge operation
	params := a.extractSubscriberParams(subscriber, tier)
	config := a.buildONTConfig(params)

	return a.netconfExecutor.EditConfig(ctx, "", config,
		netconf.WithMerge(),
		netconf.WithRollbackOnError(),
	)
}

// DeleteSubscriber removes an ONT from the OLT
func (a *Adapter) DeleteSubscriber(ctx context.Context, subscriberID string) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	// Parse subscriberID to get serial number
	serialNumber := a.parseSubscriberSerial(subscriberID)

	config := fmt.Sprintf(`
<config xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <ont xmlns="http://www.adtran.com/ns/yang/adtran-ont" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0" nc:operation="delete">
    <serial-number>%s</serial-number>
  </ont>
</config>`, serialNumber)

	return a.netconfExecutor.EditConfig(ctx, "", config, netconf.WithRollbackOnError())
}

// SuspendSubscriber disables an ONT
func (a *Adapter) SuspendSubscriber(ctx context.Context, subscriberID string) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	serialNumber := a.parseSubscriberSerial(subscriberID)

	config := fmt.Sprintf(`
<config xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <ont xmlns="http://www.adtran.com/ns/yang/adtran-ont">
    <serial-number>%s</serial-number>
    <admin-state>disabled</admin-state>
  </ont>
</config>`, serialNumber)

	return a.netconfExecutor.EditConfig(ctx, "", config, netconf.WithMerge())
}

// ResumeSubscriber enables an ONT
func (a *Adapter) ResumeSubscriber(ctx context.Context, subscriberID string) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	serialNumber := a.parseSubscriberSerial(subscriberID)

	config := fmt.Sprintf(`
<config xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <ont xmlns="http://www.adtran.com/ns/yang/adtran-ont">
    <serial-number>%s</serial-number>
    <admin-state>enabled</admin-state>
  </ont>
</config>`, serialNumber)

	return a.netconfExecutor.EditConfig(ctx, "", config, netconf.WithMerge())
}

// GetSubscriberStatus retrieves ONT status
func (a *Adapter) GetSubscriberStatus(ctx context.Context, subscriberID string) (*types.SubscriberStatus, error) {
	if a.netconfExecutor == nil {
		return nil, fmt.Errorf("NETCONF executor not available")
	}

	serialNumber := a.parseSubscriberSerial(subscriberID)

	// Query ONT state
	filter := fmt.Sprintf(GetONTStateFilterXML, serialNumber)

	response, err := a.netconfExecutor.Get(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get ONT status: %w", err)
	}

	// Parse response
	ontState := a.parseONTState(response)

	status := &types.SubscriberStatus{
		SubscriberID:  subscriberID,
		State:         ontState.OperState,
		SessionID:     fmt.Sprintf("adtran-%s", serialNumber),
		UptimeSeconds: ontState.UptimeSecs,
		IsOnline:      ontState.OperState == "online" || ontState.OperState == "active",
		LastActivity:  time.Now(),
		Metadata: map[string]interface{}{
			"vendor":       "adtran",
			"serial":       serialNumber,
			"ont_id":       ontState.ONTID,
			"pon_port":     ontState.PONPort,
			"admin_state":  ontState.AdminState,
			"oper_state":   ontState.OperState,
			"rx_power_dbm": ontState.RxPower,
			"tx_power_dbm": ontState.TxPower,
			"temperature":  ontState.Temperature,
			"distance_m":   ontState.Distance,
		},
	}

	return status, nil
}

// GetSubscriberStats retrieves ONT traffic statistics
func (a *Adapter) GetSubscriberStats(ctx context.Context, subscriberID string) (*types.SubscriberStats, error) {
	if a.netconfExecutor == nil {
		return nil, fmt.Errorf("NETCONF executor not available")
	}

	serialNumber := a.parseSubscriberSerial(subscriberID)

	// Query service statistics (linked to ONT)
	serviceID := fmt.Sprintf("svc-%s", serialNumber)
	filter := fmt.Sprintf(GetServiceStatsFilterXML, serviceID)

	response, err := a.netconfExecutor.Get(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get ONT stats: %w", err)
	}

	// Parse response
	svcStats := a.parseServiceStats(response)

	stats := &types.SubscriberStats{
		BytesUp:     svcStats.BytesUp,
		BytesDown:   svcStats.BytesDown,
		PacketsUp:   svcStats.PacketsUp,
		PacketsDown: svcStats.PacketsDown,
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"vendor":     "adtran",
			"source":     "netconf",
			"service_id": serviceID,
		},
	}

	return stats, nil
}

// HealthCheck performs a health check
func (a *Adapter) HealthCheck(ctx context.Context) error {
	if a.netconfExecutor == nil {
		return a.baseDriver.HealthCheck(ctx)
	}

	// Query system info as health check
	_, err := a.netconfExecutor.Get(ctx, GetSystemInfoFilterXML)
	return err
}

// parseSubscriberSerial extracts serial number from subscriber ID
func (a *Adapter) parseSubscriberSerial(subscriberID string) string {
	// Try to parse structured ID format: adtran-<serial>
	re := regexp.MustCompile(`adtran-(.+)$`)
	if match := re.FindStringSubmatch(subscriberID); len(match) == 2 {
		return match[1]
	}

	// Check if it looks like a serial number already
	if len(subscriberID) >= 8 && !strings.Contains(subscriberID, "/") {
		return subscriberID
	}

	// Fallback: use as-is
	return subscriberID
}

// parseONTState parses ONT state from NETCONF response
func (a *Adapter) parseONTState(data []byte) *ONTState {
	state := &ONTState{}

	type ONTStateXML struct {
		XMLName      xml.Name `xml:"ont-state"`
		SerialNumber string   `xml:"serial-number"`
		ONTID        int      `xml:"ont-id"`
		PONPort      string   `xml:"pon-port"`
		AdminState   string   `xml:"admin-state"`
		OperState    string   `xml:"operational-status"`
		Description  string   `xml:"description"`
		OpticalInfo  struct {
			RxPower     float64 `xml:"rx-power"`
			TxPower     float64 `xml:"tx-power"`
			Temperature float64 `xml:"temperature"`
			Voltage     float64 `xml:"voltage"`
		} `xml:"optical-info"`
		Distance    int    `xml:"distance"`
		LastOnline  string `xml:"last-online-time"`
		LastOffline string `xml:"last-offline-time"`
		Uptime      string `xml:"uptime"`
	}

	var s ONTStateXML
	if err := xml.Unmarshal(data, &s); err == nil {
		state.SerialNumber = s.SerialNumber
		state.ONTID = s.ONTID
		state.PONPort = s.PONPort
		state.AdminState = s.AdminState
		state.OperState = s.OperState
		state.Description = s.Description
		state.RxPower = s.OpticalInfo.RxPower
		state.TxPower = s.OpticalInfo.TxPower
		state.Temperature = s.OpticalInfo.Temperature
		state.Voltage = s.OpticalInfo.Voltage
		state.Distance = s.Distance
		state.LastOnline = s.LastOnline
		state.LastOffline = s.LastOffline
		state.UptimeSecs = parseUptime(s.Uptime)
	}

	return state
}

// parseServiceStats parses service statistics from NETCONF response
func (a *Adapter) parseServiceStats(data []byte) *ServiceStats {
	stats := &ServiceStats{}

	type StatsXML struct {
		XMLName   xml.Name `xml:"service-state"`
		ServiceID string   `xml:"service-id"`
		State     string   `xml:"state"`
		Stats     struct {
			BytesUp     uint64 `xml:"bytes-upstream"`
			BytesDown   uint64 `xml:"bytes-downstream"`
			PacketsUp   uint64 `xml:"packets-upstream"`
			PacketsDown uint64 `xml:"packets-downstream"`
		} `xml:"statistics"`
	}

	var s StatsXML
	if err := xml.Unmarshal(data, &s); err == nil {
		stats.ServiceID = s.ServiceID
		stats.State = s.State
		stats.BytesUp = s.Stats.BytesUp
		stats.BytesDown = s.Stats.BytesDown
		stats.PacketsUp = s.Stats.PacketsUp
		stats.PacketsDown = s.Stats.PacketsDown
	}

	return stats
}

// parseUptime parses uptime string to seconds
func parseUptime(uptime string) int64 {
	if secs, err := strconv.ParseInt(uptime, 10, 64); err == nil {
		return secs
	}

	var total int64
	re := regexp.MustCompile(`(\d+)([dhms])`)
	matches := re.FindAllStringSubmatch(uptime, -1)
	for _, match := range matches {
		val, _ := strconv.ParseInt(match[1], 10, 64)
		switch match[2] {
		case "d":
			total += val * 86400
		case "h":
			total += val * 3600
		case "m":
			total += val * 60
		case "s":
			total += val
		}
	}
	return total
}

// detectModel returns the detected OLT model
func (a *Adapter) detectModel() string {
	if model, ok := a.config.Metadata["model"]; ok {
		return model
	}
	return "sdx-600"
}

// Adtran-specific additional methods

// CreateBandwidthProfile creates a bandwidth profile for rate limiting
func (a *Adapter) CreateBandwidthProfile(ctx context.Context, tier *model.ServiceTier) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	profileName := fmt.Sprintf("nanoncore-bw-%dM", tier.Spec.BandwidthDown)

	// CIR = 80% of PIR, burst = 128KB
	cirUp := tier.Spec.BandwidthUp * 800      // kbps
	pirUp := tier.Spec.BandwidthUp * 1000     // kbps
	cirDown := tier.Spec.BandwidthDown * 800  // kbps
	pirDown := tier.Spec.BandwidthDown * 1000 // kbps
	burstBytes := 131072                      // 128KB

	// Create upstream bandwidth profile
	configUp := fmt.Sprintf(`
<config xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <bandwidth-profile xmlns="http://www.adtran.com/ns/yang/adtran-service">
    <profile-name>%s-up</profile-name>
    <description>Nanoncore upstream %dM up</description>
    <cir>%d</cir>
    <pir>%d</pir>
    <cbs>%d</cbs>
    <pbs>%d</pbs>
  </bandwidth-profile>
</config>`, profileName, tier.Spec.BandwidthUp, cirUp, pirUp, burstBytes, burstBytes*2)

	if err := a.netconfExecutor.EditConfig(ctx, "", configUp, netconf.WithMerge()); err != nil {
		return err
	}

	// Create downstream bandwidth profile
	configDown := fmt.Sprintf(`
<config xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <bandwidth-profile xmlns="http://www.adtran.com/ns/yang/adtran-service">
    <profile-name>%s-down</profile-name>
    <description>Nanoncore downstream %dM down</description>
    <cir>%d</cir>
    <pir>%d</pir>
    <cbs>%d</cbs>
    <pbs>%d</pbs>
  </bandwidth-profile>
</config>`, profileName, tier.Spec.BandwidthDown, cirDown, pirDown, burstBytes, burstBytes*2)

	return a.netconfExecutor.EditConfig(ctx, "", configDown, netconf.WithMerge())
}

// CreateServiceProfile creates a service profile for VLAN mapping
func (a *Adapter) CreateServiceProfile(ctx context.Context, name string, userVLAN, networkVLAN, cos int, bwProfile string) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	config := fmt.Sprintf(`
<config xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <service-profile xmlns="http://www.adtran.com/ns/yang/adtran-service">
    <profile-name>%s</profile-name>
    <description>Nanoncore service profile</description>
    <vlan-translation>
      <user-vlan>%d</user-vlan>
      <network-vlan>%d</network-vlan>
      <cos>%d</cos>
    </vlan-translation>
    <bandwidth-profile>%s</bandwidth-profile>
  </service-profile>
</config>`, name, userVLAN, networkVLAN, cos, bwProfile)

	return a.netconfExecutor.EditConfig(ctx, "", config, netconf.WithMerge())
}

// DiscoverONTs retrieves list of unconfigured ONTs
func (a *Adapter) DiscoverONTs(ctx context.Context, ponPort string) ([]ONTState, error) {
	if a.netconfExecutor == nil {
		return nil, fmt.Errorf("NETCONF executor not available")
	}

	filter := fmt.Sprintf(`
<gpon-state xmlns="http://www.adtran.com/ns/yang/adtran-gpon">
  <port>
    <port-id>%s</port-id>
    <discovered-onts/>
  </port>
</gpon-state>`, ponPort)

	response, err := a.netconfExecutor.Get(ctx, filter)
	if err != nil {
		return nil, err
	}

	return a.parseDiscoveredONTs(response), nil
}

// parseDiscoveredONTs parses list of discovered ONTs
func (a *Adapter) parseDiscoveredONTs(data []byte) []ONTState {
	var onts []ONTState

	type DiscoveredONT struct {
		SerialNumber string  `xml:"serial-number"`
		Distance     int     `xml:"distance"`
		RxPower      float64 `xml:"rx-power"`
		DiscoverTime string  `xml:"discovery-time"`
	}

	type DiscoveryXML struct {
		XMLName xml.Name        `xml:"discovered-onts"`
		ONTs    []DiscoveredONT `xml:"ont"`
	}

	var d DiscoveryXML
	if err := xml.Unmarshal(data, &d); err == nil {
		for _, o := range d.ONTs {
			onts = append(onts, ONTState{
				SerialNumber: o.SerialNumber,
				Distance:     o.Distance,
				RxPower:      o.RxPower,
				OperState:    "discovered",
			})
		}
	}

	return onts
}

// GetSystemInfo retrieves system information
func (a *Adapter) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	if a.netconfExecutor == nil {
		return nil, fmt.Errorf("NETCONF executor not available")
	}

	response, err := a.netconfExecutor.Get(ctx, GetSystemInfoFilterXML)
	if err != nil {
		return nil, err
	}

	return a.parseSystemInfo(response), nil
}

// parseSystemInfo parses system info from NETCONF response
func (a *Adapter) parseSystemInfo(data []byte) *SystemInfo {
	info := &SystemInfo{}

	type SysInfoXML struct {
		XMLName xml.Name `xml:"system-state"`
		Info    struct {
			Hostname     string `xml:"hostname"`
			Model        string `xml:"model"`
			SerialNumber string `xml:"serial-number"`
			SoftwareVer  string `xml:"software-version"`
			Uptime       string `xml:"uptime"`
		} `xml:"information"`
		CPU struct {
			Percent float64 `xml:"percent"`
		} `xml:"cpu-utilization"`
		Memory struct {
			Percent float64 `xml:"percent"`
		} `xml:"memory-utilization"`
	}

	var s SysInfoXML
	if err := xml.Unmarshal(data, &s); err == nil {
		info.Hostname = s.Info.Hostname
		info.Model = s.Info.Model
		info.SerialNumber = s.Info.SerialNumber
		info.SoftwareVer = s.Info.SoftwareVer
		info.UptimeSecs = parseUptime(s.Info.Uptime)
		info.CPUPercent = s.CPU.Percent
		info.MemoryPercent = s.Memory.Percent
	}

	return info
}
