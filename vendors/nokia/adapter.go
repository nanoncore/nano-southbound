package nokia

import (
	"context"
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/drivers/netconf"
	"github.com/nanoncore/nano-southbound/types"
)

// Adapter wraps a base driver with Nokia-specific logic
// Nokia uses NETCONF/YANG for configuration and gNMI for telemetry (SR OS / SR Linux)
type Adapter struct {
	baseDriver      types.Driver
	netconfExecutor netconf.NETCONFExecutor
	config          *types.EquipmentConfig
}

// NewAdapter creates a new Nokia adapter
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

// Connect delegates to base driver
func (a *Adapter) Connect(ctx context.Context, config *types.EquipmentConfig) error {
	return a.baseDriver.Connect(ctx, config)
}

// Disconnect delegates to base driver
func (a *Adapter) Disconnect(ctx context.Context) error {
	return a.baseDriver.Disconnect(ctx)
}

// IsConnected delegates to base driver
func (a *Adapter) IsConnected() bool {
	return a.baseDriver.IsConnected()
}

// CreateSubscriber provisions a subscriber with Nokia-specific YANG configuration
func (a *Adapter) CreateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) (*types.SubscriberResult, error) {
	if a.netconfExecutor == nil {
		return nil, fmt.Errorf("NETCONF executor not available - Nokia requires NETCONF driver")
	}

	// Extract subscriber parameters
	params := a.extractSubscriberParams(subscriber, tier)

	// Build the subscriber configuration XML
	config := a.buildSubscriberConfig(params)

	// Apply configuration via NETCONF edit-config
	err := a.netconfExecutor.EditConfig(ctx, "", config,
		netconf.WithMerge(),
		netconf.WithRollbackOnError(),
	)
	if err != nil {
		return nil, fmt.Errorf("Nokia subscriber provisioning failed: %w", err)
	}

	// Build result
	result := &types.SubscriberResult{
		SubscriberID:  subscriber.Name,
		SessionID:     fmt.Sprintf("nokia-%s", params.HostID),
		AssignedIP:    subscriber.Spec.IPAddress,
		AssignedIPv6:  subscriber.Spec.IPv6Address,
		InterfaceName: fmt.Sprintf("sub-iface:%s/grp:%s/sap:%s", params.SubInterface, params.GroupInterface, params.SapID),
		VLAN:          subscriber.Spec.VLAN,
		Metadata: map[string]interface{}{
			"vendor":          "nokia",
			"platform":        a.detectPlatform(),
			"vprn":            params.VPRN,
			"sub_interface":   params.SubInterface,
			"group_interface": params.GroupInterface,
			"sap_id":          params.SapID,
			"host_id":         params.HostID,
			"sub_profile":     params.SubProfile,
			"sla_profile":     params.SLAProfile,
		},
	}

	return result, nil
}

// subscriberParams holds parsed subscriber parameters for Nokia
type subscriberParams struct {
	VPRN           string
	SubInterface   string
	GroupInterface string
	SapID          string
	HostID         string
	MAC            string
	IPv4Address    string
	IPv6Address    string
	SubProfile     string
	SLAProfile     string
	SubIdentPolicy string
	BandwidthUp    int
	BandwidthDown  int
}

// extractSubscriberParams extracts parameters from Subscriber and ServiceTier
func (a *Adapter) extractSubscriberParams(subscriber *model.Subscriber, tier *model.ServiceTier) *subscriberParams {
	params := &subscriberParams{
		MAC:         subscriber.Spec.MACAddress,
		IPv4Address: subscriber.Spec.IPAddress,
		IPv6Address: subscriber.Spec.IPv6Address,
		HostID:      subscriber.Name,
	}

	// Get VPRN name from metadata or use default
	if vprn, ok := a.config.Metadata["vprn"]; ok {
		params.VPRN = vprn
	} else {
		params.VPRN = "internet"
	}

	// Get interface names from annotations or generate defaults
	if subscriber.Annotations != nil {
		if subIface, ok := subscriber.Annotations["nanoncore.com/sub-interface"]; ok {
			params.SubInterface = subIface
		}
		if grpIface, ok := subscriber.Annotations["nanoncore.com/group-interface"]; ok {
			params.GroupInterface = grpIface
		}
		if sapID, ok := subscriber.Annotations["nanoncore.com/sap-id"]; ok {
			params.SapID = sapID
		}
	}

	// Generate defaults if not specified
	if params.SubInterface == "" {
		params.SubInterface = fmt.Sprintf("sub-%d", subscriber.Spec.VLAN)
	}
	if params.GroupInterface == "" {
		params.GroupInterface = fmt.Sprintf("grp-%d", subscriber.Spec.VLAN)
	}
	if params.SapID == "" {
		// SAP ID format: port:vlan or lag:vlan
		port := a.config.Metadata["uplink_port"]
		if port == "" {
			port = "1/1/1"
		}
		params.SapID = fmt.Sprintf("%s:%d", port, subscriber.Spec.VLAN)
	}

	// Get profile names from tier or use defaults
	if tier != nil {
		params.BandwidthUp = tier.Spec.BandwidthUp
		params.BandwidthDown = tier.Spec.BandwidthDown

		if tier.Annotations != nil {
			if subProfile, ok := tier.Annotations["nanoncore.com/sub-profile"]; ok {
				params.SubProfile = subProfile
			}
			if slaProfile, ok := tier.Annotations["nanoncore.com/sla-profile"]; ok {
				params.SLAProfile = slaProfile
			}
			if subIdentPolicy, ok := tier.Annotations["nanoncore.com/sub-ident-policy"]; ok {
				params.SubIdentPolicy = subIdentPolicy
			}
		}
	}

	// Default profiles based on tier bandwidth
	if params.SubProfile == "" {
		params.SubProfile = fmt.Sprintf("nanoncore-%dM", params.BandwidthDown)
	}
	if params.SLAProfile == "" {
		params.SLAProfile = fmt.Sprintf("nanoncore-sla-%dM", params.BandwidthDown)
	}
	if params.SubIdentPolicy == "" {
		params.SubIdentPolicy = "nanoncore-sub-ident"
	}

	return params
}

// buildSubscriberConfig builds Nokia YANG XML for subscriber provisioning
func (a *Adapter) buildSubscriberConfig(params *subscriberParams) string {
	// Build the static subscriber host configuration
	return fmt.Sprintf(`
<configure xmlns="urn:nokia.com:sros:ns:yang:sr:conf">
  <service>
    <vprn>
      <service-name>%s</service-name>
      <subscriber-interface>
        <interface-name>%s</interface-name>
        <admin-state>enable</admin-state>
        <group-interface>
          <group-interface-name>%s</group-interface-name>
          <admin-state>enable</admin-state>
          <sap>
            <sap-id>%s</sap-id>
            <admin-state>enable</admin-state>
            <sub-sla-mgmt>
              <sub-ident-policy>%s</sub-ident-policy>
              <single-sub-parameters>
                <sub-profile>%s</sub-profile>
                <sla-profile>%s</sla-profile>
              </single-sub-parameters>
            </sub-sla-mgmt>
            <static-host>
              <static-host-id>%s</static-host-id>
              <admin-state>enable</admin-state>
              <mac>%s</mac>
              <ip-address>%s</ip-address>
              <sub-profile>%s</sub-profile>
              <sla-profile>%s</sla-profile>
            </static-host>
          </sap>
        </group-interface>
      </subscriber-interface>
    </vprn>
  </service>
</configure>`,
		params.VPRN,
		params.SubInterface,
		params.GroupInterface,
		params.SapID,
		params.SubIdentPolicy,
		params.SubProfile,
		params.SLAProfile,
		params.HostID,
		params.MAC,
		params.IPv4Address,
		params.SubProfile,
		params.SLAProfile,
	)
}

// UpdateSubscriber updates subscriber configuration
func (a *Adapter) UpdateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	// For Nokia, update is same as create with merge operation
	params := a.extractSubscriberParams(subscriber, tier)
	config := a.buildSubscriberConfig(params)

	return a.netconfExecutor.EditConfig(ctx, "", config,
		netconf.WithMerge(),
		netconf.WithRollbackOnError(),
	)
}

// DeleteSubscriber removes a subscriber
func (a *Adapter) DeleteSubscriber(ctx context.Context, subscriberID string) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	// Parse subscriberID to get interface/SAP info
	// Expected format from CreateSubscriber metadata or annotation
	params := a.parseSubscriberID(subscriberID)

	// Build delete configuration
	config := fmt.Sprintf(`
<configure xmlns="urn:nokia.com:sros:ns:yang:sr:conf" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0">
  <service>
    <vprn>
      <service-name>%s</service-name>
      <subscriber-interface>
        <interface-name>%s</interface-name>
        <group-interface>
          <group-interface-name>%s</group-interface-name>
          <sap>
            <sap-id>%s</sap-id>
            <static-host nc:operation="delete">
              <static-host-id>%s</static-host-id>
            </static-host>
          </sap>
        </group-interface>
      </subscriber-interface>
    </vprn>
  </service>
</configure>`,
		params.VPRN,
		params.SubInterface,
		params.GroupInterface,
		params.SapID,
		subscriberID,
	)

	return a.netconfExecutor.EditConfig(ctx, "", config, netconf.WithRollbackOnError())
}

// SuspendSubscriber suspends a subscriber by setting admin-state to disable
func (a *Adapter) SuspendSubscriber(ctx context.Context, subscriberID string) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	params := a.parseSubscriberID(subscriberID)

	config := fmt.Sprintf(`
<configure xmlns="urn:nokia.com:sros:ns:yang:sr:conf">
  <service>
    <vprn>
      <service-name>%s</service-name>
      <subscriber-interface>
        <interface-name>%s</interface-name>
        <group-interface>
          <group-interface-name>%s</group-interface-name>
          <sap>
            <sap-id>%s</sap-id>
            <static-host>
              <static-host-id>%s</static-host-id>
              <admin-state>disable</admin-state>
            </static-host>
          </sap>
        </group-interface>
      </subscriber-interface>
    </vprn>
  </service>
</configure>`,
		params.VPRN,
		params.SubInterface,
		params.GroupInterface,
		params.SapID,
		subscriberID,
	)

	return a.netconfExecutor.EditConfig(ctx, "", config, netconf.WithMerge())
}

// ResumeSubscriber resumes a suspended subscriber
func (a *Adapter) ResumeSubscriber(ctx context.Context, subscriberID string) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	params := a.parseSubscriberID(subscriberID)

	config := fmt.Sprintf(`
<configure xmlns="urn:nokia.com:sros:ns:yang:sr:conf">
  <service>
    <vprn>
      <service-name>%s</service-name>
      <subscriber-interface>
        <interface-name>%s</interface-name>
        <group-interface>
          <group-interface-name>%s</group-interface-name>
          <sap>
            <sap-id>%s</sap-id>
            <static-host>
              <static-host-id>%s</static-host-id>
              <admin-state>enable</admin-state>
            </static-host>
          </sap>
        </group-interface>
      </subscriber-interface>
    </vprn>
  </service>
</configure>`,
		params.VPRN,
		params.SubInterface,
		params.GroupInterface,
		params.SapID,
		subscriberID,
	)

	return a.netconfExecutor.EditConfig(ctx, "", config, netconf.WithMerge())
}

// GetSubscriberStatus retrieves subscriber status with Nokia-specific info
func (a *Adapter) GetSubscriberStatus(ctx context.Context, subscriberID string) (*types.SubscriberStatus, error) {
	if a.netconfExecutor == nil {
		return nil, fmt.Errorf("NETCONF executor not available")
	}

	// Build filter for subscriber state
	filter := fmt.Sprintf(GetSubscriberFilterXML, subscriberID)

	// Query state via NETCONF get
	response, err := a.netconfExecutor.Get(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber status: %w", err)
	}

	// Parse response
	subState := a.parseSubscriberState(response)

	status := &types.SubscriberStatus{
		SubscriberID:  subscriberID,
		State:         subState.OperState,
		SessionID:     fmt.Sprintf("nokia-%s", subscriberID),
		IPv4Address:   subState.IPv4Address,
		IPv6Address:   subState.IPv6Address,
		UptimeSeconds: subState.UptimeSecs,
		IsOnline:      subState.OperState == "up" || subState.AdminState == "enable",
		LastActivity:  time.Now(),
		Metadata: map[string]interface{}{
			"vendor":      "nokia",
			"admin_state": subState.AdminState,
			"oper_state":  subState.OperState,
			"mac":         subState.MAC,
			"sub_profile": subState.SubProfile,
			"sla_profile": subState.SLAProfile,
		},
	}

	return status, nil
}

// GetSubscriberStats retrieves subscriber statistics
func (a *Adapter) GetSubscriberStats(ctx context.Context, subscriberID string) (*types.SubscriberStats, error) {
	if a.netconfExecutor == nil {
		return nil, fmt.Errorf("NETCONF executor not available")
	}

	// Build filter for subscriber statistics
	filter := fmt.Sprintf(GetSubscriberStatsFilterXML, subscriberID)

	// Query state via NETCONF get
	response, err := a.netconfExecutor.Get(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber stats: %w", err)
	}

	// Parse response
	subStats := a.parseSubscriberStats(response)

	stats := &types.SubscriberStats{
		BytesUp:     subStats.IngressOctets,
		BytesDown:   subStats.EgressOctets,
		PacketsUp:   subStats.IngressPackets,
		PacketsDown: subStats.EgressPackets,
		Drops:       subStats.IngressDrops + subStats.EgressDrops,
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"vendor":         "nokia",
			"source":         "netconf",
			"ingress_drops":  subStats.IngressDrops,
			"egress_drops":   subStats.EgressDrops,
		},
	}

	return stats, nil
}

// HealthCheck performs a health check by querying system info
func (a *Adapter) HealthCheck(ctx context.Context) error {
	if a.netconfExecutor == nil {
		return a.baseDriver.HealthCheck(ctx)
	}

	// Query system information as health check
	_, err := a.netconfExecutor.Get(ctx, GetSystemInfoFilterXML)
	return err
}

// parseSubscriberID extracts interface parameters from subscriber ID
func (a *Adapter) parseSubscriberID(subscriberID string) *subscriberParams {
	params := &subscriberParams{
		HostID: subscriberID,
	}

	// Get defaults from config
	if vprn, ok := a.config.Metadata["vprn"]; ok {
		params.VPRN = vprn
	} else {
		params.VPRN = "internet"
	}

	// Try to parse structured ID format: sub-iface:X/grp:Y/sap:Z
	// This is the format returned by CreateSubscriber
	re := regexp.MustCompile(`sub-iface:([^/]+)/grp:([^/]+)/sap:(.+)`)
	if match := re.FindStringSubmatch(subscriberID); len(match) == 4 {
		params.SubInterface = match[1]
		params.GroupInterface = match[2]
		params.SapID = match[3]
		return params
	}

	// Fallback: use defaults
	params.SubInterface = "sub-default"
	params.GroupInterface = "grp-default"
	params.SapID = "1/1/1:100"

	return params
}

// parseSubscriberState parses subscriber state from NETCONF response
func (a *Adapter) parseSubscriberState(data []byte) *SubscriberState {
	state := &SubscriberState{}

	// Parse XML response
	type SubscriberXML struct {
		XMLName      xml.Name `xml:"subscriber"`
		SubscriberID string   `xml:"subscriber-id"`
		AdminState   string   `xml:"admin-state"`
		OperState    string   `xml:"oper-state"`
		MAC          string   `xml:"mac-address"`
		IPv4Address  string   `xml:"ipv4-address"`
		IPv6Address  string   `xml:"ipv6-address"`
		SubProfile   string   `xml:"sub-profile"`
		SLAProfile   string   `xml:"sla-profile"`
		Uptime       string   `xml:"up-time"`
	}

	var sub SubscriberXML
	if err := xml.Unmarshal(data, &sub); err == nil {
		state.SubscriberID = sub.SubscriberID
		state.AdminState = sub.AdminState
		state.OperState = sub.OperState
		state.MAC = sub.MAC
		state.IPv4Address = sub.IPv4Address
		state.IPv6Address = sub.IPv6Address
		state.SubProfile = sub.SubProfile
		state.SLAProfile = sub.SLAProfile

		// Parse uptime (format varies)
		state.UptimeSecs = parseUptime(sub.Uptime)
	}

	return state
}

// parseSubscriberStats parses subscriber statistics from NETCONF response
func (a *Adapter) parseSubscriberStats(data []byte) *SubscriberStats {
	stats := &SubscriberStats{}

	// Parse XML response for statistics
	type StatsXML struct {
		XMLName        xml.Name `xml:"statistics"`
		IngressOctets  uint64   `xml:"ingress-octets"`
		EgressOctets   uint64   `xml:"egress-octets"`
		IngressPackets uint64   `xml:"ingress-packets"`
		EgressPackets  uint64   `xml:"egress-packets"`
		IngressDrops   uint64   `xml:"ingress-drops"`
		EgressDrops    uint64   `xml:"egress-drops"`
	}

	var s StatsXML
	if err := xml.Unmarshal(data, &s); err == nil {
		stats.IngressOctets = s.IngressOctets
		stats.EgressOctets = s.EgressOctets
		stats.IngressPackets = s.IngressPackets
		stats.EgressPackets = s.EgressPackets
		stats.IngressDrops = s.IngressDrops
		stats.EgressDrops = s.EgressDrops
	}

	return stats
}

// parseUptime parses uptime string to seconds
func parseUptime(uptime string) int64 {
	// Handle formats like "1d 2h 30m 45s" or "12345" (seconds)
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

// detectPlatform determines if this is SR OS or SR Linux
func (a *Adapter) detectPlatform() string {
	if platform, ok := a.config.Metadata["platform"]; ok {
		return platform
	}

	// Try to detect from NETCONF capabilities
	if a.netconfExecutor != nil {
		caps := a.netconfExecutor.GetCapabilities()
		for _, cap := range caps {
			if strings.Contains(cap, "sr-linux") {
				return "srlinux"
			}
			if strings.Contains(cap, "nokia.com:sros") {
				return "sros"
			}
		}
	}

	return "sros" // Default assumption
}

// Nokia-specific additional methods

// CreateQoSProfiles creates QoS profiles for a service tier
func (a *Adapter) CreateQoSProfiles(ctx context.Context, tier *model.ServiceTier) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	profileName := fmt.Sprintf("nanoncore-%dM", tier.Spec.BandwidthDown)

	// CIR = 80% of PIR, burst = 128KB
	cirUp := tier.Spec.BandwidthUp * 800     // kbps (80% of Mbps * 1000)
	pirUp := tier.Spec.BandwidthUp * 1000    // kbps
	cirDown := tier.Spec.BandwidthDown * 800 // kbps
	pirDown := tier.Spec.BandwidthDown * 1000 // kbps
	mbs := 131072 // 128KB burst size

	// Create SAP ingress policy
	ingressPolicy := fmt.Sprintf(SapIngressPolicyXML,
		profileName+"-ingress",
		fmt.Sprintf("Nanoncore QoS profile for %dM down / %dM up", tier.Spec.BandwidthDown, tier.Spec.BandwidthUp),
		cirUp, pirUp, mbs, cirUp, pirUp,
	)

	// Create SAP egress policy
	egressPolicy := fmt.Sprintf(SapEgressPolicyXML,
		profileName+"-egress",
		fmt.Sprintf("Nanoncore QoS profile for %dM down / %dM up", tier.Spec.BandwidthDown, tier.Spec.BandwidthUp),
		cirDown, pirDown,
	)

	// Apply both policies
	config := fmt.Sprintf(`
<configure xmlns="urn:nokia.com:sros:ns:yang:sr:conf">
  <qos>
    %s
    %s
  </qos>
</configure>`, ingressPolicy, egressPolicy)

	return a.netconfExecutor.EditConfig(ctx, "", config, netconf.WithMerge())
}

// CreateSubscriberProfile creates a subscriber profile for a service tier
func (a *Adapter) CreateSubscriberProfile(ctx context.Context, tier *model.ServiceTier) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	profileName := fmt.Sprintf("nanoncore-%dM", tier.Spec.BandwidthDown)
	slaProfile := fmt.Sprintf("nanoncore-sla-%dM", tier.Spec.BandwidthDown)
	subIdentPolicy := "nanoncore-sub-ident"

	// CIR = 80% of PIR
	cirUp := tier.Spec.BandwidthUp * 800
	pirUp := tier.Spec.BandwidthUp * 1000
	cirDown := tier.Spec.BandwidthDown * 800
	pirDown := tier.Spec.BandwidthDown * 1000

	config := fmt.Sprintf(SubscriberProfileXML,
		profileName,
		fmt.Sprintf("Nanoncore tier: %dM down / %dM up", tier.Spec.BandwidthDown, tier.Spec.BandwidthUp),
		slaProfile,
		subIdentPolicy,
		cirUp, pirUp, // ingress
		cirDown, pirDown, // egress
	)

	fullConfig := fmt.Sprintf(`
<configure xmlns="urn:nokia.com:sros:ns:yang:sr:conf">
  <subscriber-mgmt>
    %s
  </subscriber-mgmt>
</configure>`, config)

	return a.netconfExecutor.EditConfig(ctx, "", fullConfig, netconf.WithMerge())
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

	type SystemXML struct {
		XMLName xml.Name `xml:"system"`
		Info    struct {
			Name    string `xml:"system-name"`
			Type    string `xml:"chassis-type"`
			Version string `xml:"software-version"`
			Uptime  string `xml:"up-time"`
		} `xml:"information"`
	}

	var sys SystemXML
	if err := xml.Unmarshal(data, &sys); err == nil {
		info.Name = sys.Info.Name
		info.Type = sys.Info.Type
		info.Version = sys.Info.Version
		info.UptimeSecs = parseUptime(sys.Info.Uptime)
	}

	return info
}
