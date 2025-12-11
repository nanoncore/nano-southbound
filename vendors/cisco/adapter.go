package cisco

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

// Adapter wraps a base driver with Cisco-specific logic
// Cisco uses NETCONF/YANG (IOS-XR/XE) and gNMI for telemetry
type Adapter struct {
	baseDriver      types.Driver
	netconfExecutor netconf.NETCONFExecutor
	config          *types.EquipmentConfig
}

// NewAdapter creates a new Cisco adapter
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

// CreateSubscriber provisions a subscriber using Cisco IOS-XR YANG models
func (a *Adapter) CreateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) (*types.SubscriberResult, error) {
	if a.netconfExecutor == nil {
		return nil, fmt.Errorf("NETCONF executor not available - Cisco requires NETCONF driver")
	}

	// Extract subscriber parameters
	params := a.extractSubscriberParams(subscriber, tier)

	// Build the interface and subscriber configuration
	config := a.buildSubscriberConfig(params)

	// Apply configuration via NETCONF edit-config
	err := a.netconfExecutor.EditConfig(ctx, "", config,
		netconf.WithMerge(),
		netconf.WithRollbackOnError(),
	)
	if err != nil {
		return nil, fmt.Errorf("Cisco subscriber provisioning failed: %w", err)
	}

	// Build result
	result := &types.SubscriberResult{
		SubscriberID:  subscriber.Name,
		SessionID:     fmt.Sprintf("cisco-%s-%d", subscriber.Name, subscriber.Spec.VLAN),
		AssignedIP:    subscriber.Spec.IPAddress,
		AssignedIPv6:  subscriber.Spec.IPv6Address,
		InterfaceName: params.InterfaceName,
		VLAN:          subscriber.Spec.VLAN,
		Metadata: map[string]interface{}{
			"vendor":          "cisco",
			"os":              a.detectOS(),
			"interface":       params.InterfaceName,
			"parent_iface":    params.ParentInterface,
			"template":        params.DynamicTemplate,
			"policy_input":    params.PolicyInput,
			"policy_output":   params.PolicyOutput,
			"node":            params.NodeName,
		},
	}

	return result, nil
}

// subscriberParams holds parsed subscriber parameters for Cisco
type subscriberParams struct {
	NodeName         string
	ParentInterface  string
	InterfaceName    string
	VLAN             int
	MAC              string
	IPv4Address      string
	IPv6Address      string
	DynamicTemplate  string
	PolicyInput      string
	PolicyOutput     string
	BandwidthUp      int
	BandwidthDown    int
	UnnumberedIface  string
}

// extractSubscriberParams extracts parameters from Subscriber and ServiceTier
func (a *Adapter) extractSubscriberParams(subscriber *model.Subscriber, tier *model.ServiceTier) *subscriberParams {
	params := &subscriberParams{
		VLAN:        subscriber.Spec.VLAN,
		MAC:         subscriber.Spec.MACAddress,
		IPv4Address: subscriber.Spec.IPAddress,
		IPv6Address: subscriber.Spec.IPv6Address,
	}

	// Get node name from metadata or use default
	if nodeName, ok := a.config.Metadata["node_name"]; ok {
		params.NodeName = nodeName
	} else {
		params.NodeName = "0/0/CPU0"
	}

	// Get parent interface from metadata or annotations
	if parentIface, ok := a.config.Metadata["uplink_interface"]; ok {
		params.ParentInterface = parentIface
	} else {
		params.ParentInterface = "Bundle-Ether1"
	}

	if subscriber.Annotations != nil {
		if iface, ok := subscriber.Annotations["nanoncore.com/interface"]; ok {
			params.ParentInterface = iface
		}
	}

	// Build sub-interface name
	params.InterfaceName = fmt.Sprintf("%s.%d", params.ParentInterface, params.VLAN)

	// Get unnumbered interface for IP assignment
	if unnumbered, ok := a.config.Metadata["unnumbered_interface"]; ok {
		params.UnnumberedIface = unnumbered
	} else {
		params.UnnumberedIface = "Loopback0"
	}

	// Get profile names from tier or use defaults
	if tier != nil {
		params.BandwidthUp = tier.Spec.BandwidthUp
		params.BandwidthDown = tier.Spec.BandwidthDown

		if tier.Annotations != nil {
			if template, ok := tier.Annotations["nanoncore.com/dynamic-template"]; ok {
				params.DynamicTemplate = template
			}
			if policyIn, ok := tier.Annotations["nanoncore.com/policy-input"]; ok {
				params.PolicyInput = policyIn
			}
			if policyOut, ok := tier.Annotations["nanoncore.com/policy-output"]; ok {
				params.PolicyOutput = policyOut
			}
		}
	}

	// Default policies based on bandwidth
	if params.DynamicTemplate == "" {
		params.DynamicTemplate = fmt.Sprintf("nanoncore-ipsub-%dM", params.BandwidthDown)
	}
	if params.PolicyInput == "" {
		params.PolicyInput = fmt.Sprintf("nanoncore-ingress-%dM", params.BandwidthDown)
	}
	if params.PolicyOutput == "" {
		params.PolicyOutput = fmt.Sprintf("nanoncore-egress-%dM", params.BandwidthDown)
	}

	return params
}

// buildSubscriberConfig builds Cisco IOS-XR YANG XML for subscriber provisioning
func (a *Adapter) buildSubscriberConfig(params *subscriberParams) string {
	// Build sub-interface configuration with IPoE subscriber attachment
	return fmt.Sprintf(`
<interface-configurations xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ifmgr-cfg">
  <interface-configuration>
    <active>act</active>
    <interface-name>%s</interface-name>
    <interface-mode-non-physical>l2-transport</interface-mode-non-physical>
    <description>Nanoncore subscriber VLAN %d</description>
    <vlan-sub-configuration xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-l2-eth-infra-cfg">
      <vlan-identifier>
        <vlan-type>vlan-type-dot1q</vlan-type>
        <first-tag>%d</first-tag>
      </vlan-identifier>
    </vlan-sub-configuration>
    <ipsub xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-subscriber-ipsub-cfg">
      <subscriber>
        <ipv4>
          <l2-connected>
            <initiator>
              <dhcp/>
            </initiator>
          </l2-connected>
        </ipv4>
      </subscriber>
    </ipsub>
    <qos xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-qos-ma-cfg">
      <input>
        <service-policy>
          <service-policy-name>%s</service-policy-name>
        </service-policy>
      </input>
      <output>
        <service-policy>
          <service-policy-name>%s</service-policy-name>
        </service-policy>
      </output>
    </qos>
  </interface-configuration>
</interface-configurations>`,
		params.InterfaceName,
		params.VLAN,
		params.VLAN,
		params.PolicyInput,
		params.PolicyOutput,
	)
}

// UpdateSubscriber updates subscriber configuration
func (a *Adapter) UpdateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	// For Cisco, update is same as create with merge operation
	params := a.extractSubscriberParams(subscriber, tier)
	config := a.buildSubscriberConfig(params)

	return a.netconfExecutor.EditConfig(ctx, "", config,
		netconf.WithMerge(),
		netconf.WithRollbackOnError(),
	)
}

// DeleteSubscriber removes a subscriber by deleting the sub-interface
func (a *Adapter) DeleteSubscriber(ctx context.Context, subscriberID string) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	// Parse subscriberID to get interface name
	interfaceName := a.parseSubscriberInterface(subscriberID)

	// Build delete configuration
	config := fmt.Sprintf(DeleteInterfaceXML, interfaceName)

	fullConfig := fmt.Sprintf(`
<interface-configurations xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ifmgr-cfg" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0">
  %s
</interface-configurations>`, config)

	return a.netconfExecutor.EditConfig(ctx, "", fullConfig, netconf.WithRollbackOnError())
}

// SuspendSubscriber suspends a subscriber by shutting down the interface
func (a *Adapter) SuspendSubscriber(ctx context.Context, subscriberID string) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	interfaceName := a.parseSubscriberInterface(subscriberID)

	config := fmt.Sprintf(`
<interface-configurations xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ifmgr-cfg">
  <interface-configuration>
    <active>act</active>
    <interface-name>%s</interface-name>
    <shutdown/>
  </interface-configuration>
</interface-configurations>`, interfaceName)

	return a.netconfExecutor.EditConfig(ctx, "", config, netconf.WithMerge())
}

// ResumeSubscriber resumes a suspended subscriber
func (a *Adapter) ResumeSubscriber(ctx context.Context, subscriberID string) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	interfaceName := a.parseSubscriberInterface(subscriberID)

	config := fmt.Sprintf(`
<interface-configurations xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ifmgr-cfg" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0">
  <interface-configuration>
    <active>act</active>
    <interface-name>%s</interface-name>
    <shutdown nc:operation="delete"/>
  </interface-configuration>
</interface-configurations>`, interfaceName)

	return a.netconfExecutor.EditConfig(ctx, "", config, netconf.WithMerge())
}

// GetSubscriberStatus retrieves subscriber status
func (a *Adapter) GetSubscriberStatus(ctx context.Context, subscriberID string) (*types.SubscriberStatus, error) {
	if a.netconfExecutor == nil {
		return nil, fmt.Errorf("NETCONF executor not available")
	}

	// Get node name and session ID
	nodeName := a.getNodeName()
	interfaceName := a.parseSubscriberInterface(subscriberID)

	// Query subscriber session
	filter := fmt.Sprintf(GetSubscriberSessionFilterXML, nodeName, subscriberID)

	response, err := a.netconfExecutor.Get(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber status: %w", err)
	}

	// Parse response
	session := a.parseSubscriberSession(response)

	status := &types.SubscriberStatus{
		SubscriberID:  subscriberID,
		State:         session.State,
		SessionID:     session.SessionID,
		IPv4Address:   session.IPv4Address,
		IPv6Address:   session.IPv6Address,
		UptimeSeconds: session.UptimeSecs,
		IsOnline:      session.State == "activated" || session.State == "connected",
		LastActivity:  time.Now(),
		Metadata: map[string]interface{}{
			"vendor":        "cisco",
			"interface":     interfaceName,
			"mac":           session.MACAddress,
			"service_type":  session.ServiceType,
			"accounting_id": session.AccountingID,
		},
	}

	return status, nil
}

// GetSubscriberStats retrieves subscriber statistics
func (a *Adapter) GetSubscriberStats(ctx context.Context, subscriberID string) (*types.SubscriberStats, error) {
	if a.netconfExecutor == nil {
		return nil, fmt.Errorf("NETCONF executor not available")
	}

	interfaceName := a.parseSubscriberInterface(subscriberID)

	// Query interface statistics
	filter := fmt.Sprintf(GetInterfaceStatsFilterXML, interfaceName)

	response, err := a.netconfExecutor.Get(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber stats: %w", err)
	}

	// Parse response
	ifaceStats := a.parseInterfaceStats(response)

	stats := &types.SubscriberStats{
		BytesUp:     ifaceStats.BytesReceived,
		BytesDown:   ifaceStats.BytesSent,
		PacketsUp:   ifaceStats.PacketsReceived,
		PacketsDown: ifaceStats.PacketsSent,
		ErrorsUp:    ifaceStats.InputErrors,
		ErrorsDown:  ifaceStats.OutputErrors,
		Drops:       ifaceStats.InputDrops + ifaceStats.OutputDrops,
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"vendor":       "cisco",
			"source":       "netconf",
			"interface":    interfaceName,
			"input_drops":  ifaceStats.InputDrops,
			"output_drops": ifaceStats.OutputDrops,
			"crc_errors":   ifaceStats.InputCRCErrors,
		},
	}

	return stats, nil
}

// HealthCheck performs a health check
func (a *Adapter) HealthCheck(ctx context.Context) error {
	if a.netconfExecutor == nil {
		return a.baseDriver.HealthCheck(ctx)
	}

	// Query system monitoring as health check
	_, err := a.netconfExecutor.Get(ctx, GetSystemInfoFilterXML)
	return err
}

// parseSubscriberInterface extracts interface name from subscriber ID
func (a *Adapter) parseSubscriberInterface(subscriberID string) string {
	// Try to parse structured ID format from CreateSubscriber result
	// Format: cisco-<name>-<vlan>
	re := regexp.MustCompile(`cisco-(.+)-(\d+)$`)
	if match := re.FindStringSubmatch(subscriberID); len(match) == 3 {
		vlan, _ := strconv.Atoi(match[2])
		parentIface := a.config.Metadata["uplink_interface"]
		if parentIface == "" {
			parentIface = "Bundle-Ether1"
		}
		return fmt.Sprintf("%s.%d", parentIface, vlan)
	}

	// Check if it's already an interface name
	if strings.Contains(subscriberID, ".") {
		return subscriberID
	}

	// Fallback: assume it's a VLAN number
	parentIface := a.config.Metadata["uplink_interface"]
	if parentIface == "" {
		parentIface = "Bundle-Ether1"
	}
	return fmt.Sprintf("%s.%s", parentIface, subscriberID)
}

// getNodeName returns the router node name
func (a *Adapter) getNodeName() string {
	if nodeName, ok := a.config.Metadata["node_name"]; ok {
		return nodeName
	}
	return "0/0/CPU0"
}

// parseSubscriberSession parses subscriber session from NETCONF response
func (a *Adapter) parseSubscriberSession(data []byte) *SubscriberSession {
	session := &SubscriberSession{}

	// Parse XML response
	type SessionXML struct {
		XMLName       xml.Name `xml:"session-id"`
		SessionID     string   `xml:"session-id"`
		SubLabel      string   `xml:"subscriber-label"`
		State         string   `xml:"state"`
		MAC           string   `xml:"mac-address"`
		IPv4          string   `xml:"ipv4-address"`
		IPv6          string   `xml:"ipv6-address"`
		Interface     string   `xml:"interface-name"`
		VLAN          int      `xml:"outer-vlan"`
		Uptime        string   `xml:"up-time"`
		AccountingID  string   `xml:"accounting-session-id"`
		ServiceType   string   `xml:"session-type"`
	}

	var s SessionXML
	if err := xml.Unmarshal(data, &s); err == nil {
		session.SessionID = s.SessionID
		session.SubscriberLabel = s.SubLabel
		session.State = s.State
		session.MACAddress = s.MAC
		session.IPv4Address = s.IPv4
		session.IPv6Address = s.IPv6
		session.Interface = s.Interface
		session.VLAN = s.VLAN
		session.AccountingID = s.AccountingID
		session.ServiceType = s.ServiceType
		session.UptimeSecs = parseUptime(s.Uptime)
	}

	return session
}

// parseInterfaceStats parses interface statistics from NETCONF response
func (a *Adapter) parseInterfaceStats(data []byte) *InterfaceStats {
	stats := &InterfaceStats{}

	type StatsXML struct {
		XMLName         xml.Name `xml:"generic-counters"`
		BytesReceived   uint64   `xml:"bytes-received"`
		BytesSent       uint64   `xml:"bytes-sent"`
		PacketsReceived uint64   `xml:"packets-received"`
		PacketsSent     uint64   `xml:"packets-sent"`
		InputErrors     uint64   `xml:"input-errors"`
		OutputErrors    uint64   `xml:"output-errors"`
		InputDrops      uint64   `xml:"input-drops"`
		OutputDrops     uint64   `xml:"output-drops"`
		CRCErrors       uint64   `xml:"crc-errors"`
		OutputBufferFails uint64 `xml:"output-buffer-failures"`
	}

	var s StatsXML
	if err := xml.Unmarshal(data, &s); err == nil {
		stats.BytesReceived = s.BytesReceived
		stats.BytesSent = s.BytesSent
		stats.PacketsReceived = s.PacketsReceived
		stats.PacketsSent = s.PacketsSent
		stats.InputErrors = s.InputErrors
		stats.OutputErrors = s.OutputErrors
		stats.InputDrops = s.InputDrops
		stats.OutputDrops = s.OutputDrops
		stats.InputCRCErrors = s.CRCErrors
		stats.OutputBufferFails = s.OutputBufferFails
	}

	return stats
}

// parseUptime parses uptime string to seconds
func parseUptime(uptime string) int64 {
	// Handle formats like "1d 2h 30m 45s", "12345" (seconds), or "1:02:30:45"
	if secs, err := strconv.ParseInt(uptime, 10, 64); err == nil {
		return secs
	}

	// Try d:h:m:s format
	re := regexp.MustCompile(`(\d+):(\d+):(\d+):(\d+)`)
	if match := re.FindStringSubmatch(uptime); len(match) == 5 {
		days, _ := strconv.ParseInt(match[1], 10, 64)
		hours, _ := strconv.ParseInt(match[2], 10, 64)
		minutes, _ := strconv.ParseInt(match[3], 10, 64)
		seconds, _ := strconv.ParseInt(match[4], 10, 64)
		return days*86400 + hours*3600 + minutes*60 + seconds
	}

	// Try "Xd Xh Xm Xs" format
	var total int64
	re2 := regexp.MustCompile(`(\d+)([dhms])`)
	matches := re2.FindAllStringSubmatch(uptime, -1)
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

// detectOS returns the detected OS type
func (a *Adapter) detectOS() string {
	if os, ok := a.config.Metadata["os"]; ok {
		return os
	}

	// Try to detect from NETCONF capabilities
	if a.netconfExecutor != nil {
		caps := a.netconfExecutor.GetCapabilities()
		for _, cap := range caps {
			if strings.Contains(cap, "IOS-XR") {
				return "ios-xr"
			}
			if strings.Contains(cap, "IOS-XE") {
				return "ios-xe"
			}
		}
	}

	return "ios-xr" // Default assumption for BNG
}

// Cisco-specific additional methods

// CreateDynamicTemplate creates a dynamic template for subscriber services
func (a *Adapter) CreateDynamicTemplate(ctx context.Context, name string, tier *model.ServiceTier) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	unnumbered := a.config.Metadata["unnumbered_interface"]
	if unnumbered == "" {
		unnumbered = "Loopback0"
	}

	policyIn := fmt.Sprintf("nanoncore-ingress-%dM", tier.Spec.BandwidthDown)
	policyOut := fmt.Sprintf("nanoncore-egress-%dM", tier.Spec.BandwidthDown)

	config := fmt.Sprintf(`
<dynamic-template xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-subscriber-infra-tmplmgr-cfg">
  <ip-subscribers>
    <ip-subscriber>
      <template-name>%s</template-name>
      <ipv4-network xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ipv4-ma-subscriber-cfg">
        <unnumbered>%s</unnumbered>
      </ipv4-network>
      <ipv6-network xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ipv6-ma-subscriber-cfg">
        <addresses>
          <auto-configuration>
            <enable/>
          </auto-configuration>
        </addresses>
      </ipv6-network>
      <qos xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-qos-ma-cfg">
        <service-policy>
          <input>
            <policy-name>%s</policy-name>
          </input>
          <output>
            <policy-name>%s</policy-name>
          </output>
        </service-policy>
      </qos>
    </ip-subscriber>
  </ip-subscribers>
</dynamic-template>`, name, unnumbered, policyIn, policyOut)

	return a.netconfExecutor.EditConfig(ctx, "", config, netconf.WithMerge())
}

// CreateQoSPolicy creates a QoS policy map for subscriber rate limiting
func (a *Adapter) CreateQoSPolicy(ctx context.Context, tier *model.ServiceTier) error {
	if a.netconfExecutor == nil {
		return fmt.Errorf("NETCONF executor not available")
	}

	// CIR = 80% of PIR, burst = 128KB
	cirUp := tier.Spec.BandwidthUp * 800     // kbps
	pirUp := tier.Spec.BandwidthUp * 1000    // kbps
	cirDown := tier.Spec.BandwidthDown * 800 // kbps
	pirDown := tier.Spec.BandwidthDown * 1000 // kbps
	burstKB := 128

	// Create ingress policy
	ingressPolicy := fmt.Sprintf(ServicePolicyMapXML,
		fmt.Sprintf("nanoncore-ingress-%dM", tier.Spec.BandwidthDown),
		cirUp, pirUp, burstKB, burstKB,
	)

	// Create egress policy
	egressPolicy := fmt.Sprintf(ServicePolicyMapXML,
		fmt.Sprintf("nanoncore-egress-%dM", tier.Spec.BandwidthDown),
		cirDown, pirDown, burstKB, burstKB,
	)

	config := fmt.Sprintf(`
<qos xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-qos-ma-cfg">
  <policy-maps>
    %s
    %s
  </policy-maps>
</qos>`, ingressPolicy, egressPolicy)

	return a.netconfExecutor.EditConfig(ctx, "", config, netconf.WithMerge())
}

// GetSubscriberSummary retrieves subscriber count summary
func (a *Adapter) GetSubscriberSummary(ctx context.Context) (*SubscriberSummary, error) {
	if a.netconfExecutor == nil {
		return nil, fmt.Errorf("NETCONF executor not available")
	}

	nodeName := a.getNodeName()
	filter := fmt.Sprintf(GetSubscriberSummaryFilterXML, nodeName)

	response, err := a.netconfExecutor.Get(ctx, filter)
	if err != nil {
		return nil, err
	}

	return a.parseSubscriberSummary(response), nil
}

// parseSubscriberSummary parses subscriber summary from NETCONF response
func (a *Adapter) parseSubscriberSummary(data []byte) *SubscriberSummary {
	summary := &SubscriberSummary{}

	type SummaryXML struct {
		XMLName     xml.Name `xml:"summary"`
		Total       int      `xml:"total-sessions"`
		PPPoE       int      `xml:"pppoe-sessions"`
		IPoE        int      `xml:"ipoe-sessions"`
		Activated   int      `xml:"activated-sessions"`
		Initiating  int      `xml:"initiating-sessions"`
	}

	var s SummaryXML
	if err := xml.Unmarshal(data, &s); err == nil {
		summary.TotalSessions = s.Total
		summary.PPPoESessions = s.PPPoE
		summary.IPoESessions = s.IPoE
		summary.ActiveSessions = s.Activated
		summary.InitiatingSessions = s.Initiating
	}

	return summary
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

	type CPUInfo struct {
		XMLName    xml.Name `xml:"cpu-utilization"`
		TotalCPU   float64  `xml:"total-cpu-fifteen-minute"`
	}

	var cpu CPUInfo
	if err := xml.Unmarshal(data, &cpu); err == nil {
		info.CPUPercent = cpu.TotalCPU
	}

	return info
}
