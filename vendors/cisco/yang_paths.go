package cisco

// Cisco IOS-XR YANG Paths and XML Templates
// Reference: Cisco IOS-XR YANG models for BNG subscriber management
// Supports: IOS-XR 6.x, 7.x on ASR 9000, NCS 5500, 8000 series

// YANG Namespaces
const (
	// Cisco IOS-XR namespaces
	NSCiscoSubscriber    = "http://cisco.com/ns/yang/Cisco-IOS-XR-subscriber-infra-tmplmgr-cfg"
	NSCiscoSubSession    = "http://cisco.com/ns/yang/Cisco-IOS-XR-subscriber-session-mon-oper"
	NSCiscoSubPPPoE      = "http://cisco.com/ns/yang/Cisco-IOS-XR-subscriber-pppoe-ma-gbl-cfg"
	NSCiscoSubIPSub      = "http://cisco.com/ns/yang/Cisco-IOS-XR-subscriber-ipsub-cfg"
	NSCiscoQoS           = "http://cisco.com/ns/yang/Cisco-IOS-XR-qos-ma-cfg"
	NSCiscoInfra         = "http://cisco.com/ns/yang/Cisco-IOS-XR-infra-infra-cfg"
	NSCiscoInterface     = "http://cisco.com/ns/yang/Cisco-IOS-XR-ifmgr-cfg"
	NSCiscoAAA           = "http://cisco.com/ns/yang/Cisco-IOS-XR-aaa-lib-cfg"
	NSCiscoRADIUS        = "http://cisco.com/ns/yang/Cisco-IOS-XR-aaa-protocol-radius-cfg"
	NSCiscoIPv4          = "http://cisco.com/ns/yang/Cisco-IOS-XR-ipv4-io-cfg"
	NSCiscoIPv6          = "http://cisco.com/ns/yang/Cisco-IOS-XR-ipv6-ma-cfg"

	// Standard namespaces
	NSNetconfBase = "urn:ietf:params:xml:ns:netconf:base:1.0"
)

// Configuration Paths - Dynamic Templates
const (
	// Dynamic template paths (BNG subscriber templates)
	PathDynamicTemplate      = "/dynamic-template"
	PathPPPoETemplate        = "/dynamic-template/ppps/ppp[template-name='%s']"
	PathIPSubTemplate        = "/dynamic-template/ip-subscribers/ip-subscriber[template-name='%s']"
	PathServiceTemplate      = "/dynamic-template/service[template-name='%s']"

	// Subscriber configuration paths
	PathSubscriberInfra      = "/subscriber/manager"
	PathSubscriberAccess     = "/subscriber/manager/nodes/node[node-name='%s']"
	PathSubsRedundancy       = "/subscriber/redundancy"

	// Interface paths
	PathInterface            = "/interface-configurations/interface-configuration[active='act'][interface-name='%s']"
	PathSubInterface         = "/interface-configurations/interface-configuration[active='act'][interface-name='%s.%d']"
	PathBundleInterface      = "/interface-configurations/interface-configuration[active='act'][interface-name='Bundle-Ether%d']"

	// QoS paths
	PathQoSPolicyMap         = "/qos/policy-maps/policy-map[name='%s']"
	PathQoSClassMap          = "/qos/class-maps/class-map[name='%s']"
	PathQoSServicePolicy     = "/interface-configurations/interface-configuration[active='act'][interface-name='%s']/qos/input/service-policy[service-policy-name='%s']"

	// AAA/RADIUS paths
	PathAAA                  = "/aaa"
	PathRADIUSServer         = "/aaa/radius/hosts/host[ordering-index='%d'][ip-address='%s'][auth-port-number='%d'][acct-port-number='%d']"
	PathRADIUSAttribute      = "/aaa/radius/attributes"
	PathRADIUSDeadCriteria   = "/aaa/radius/dead-criteria"

	// DHCP paths
	PathDHCPIPv4             = "/ipv4-dhcpd"
	PathDHCPIPv4Profile      = "/ipv4-dhcpd/profiles/profile[profile-name='%s']"
	PathDHCPIPv4Relay        = "/ipv4-dhcpd/interfaces/interface[interface-name='%s']"
	PathDHCPv6               = "/dhcp/ipv6"
)

// State/Telemetry Paths
const (
	// Subscriber session state
	PathStateSubscriberSessions = "/subscriber-session-mon"
	PathStateSubscriberSession  = "/subscriber-session-mon/nodes/node[node-name='%s']/session-ids/session-id[session-id='%s']"
	PathStateSubSessionSummary  = "/subscriber-session-mon/nodes/node[node-name='%s']/summary"

	// Subscriber accounting
	PathStateSubAccounting      = "/subscriber-accounting/nodes/node[node-name='%s']"

	// Interface statistics
	PathStateInterfaceStats     = "/infra-statistics/interfaces/interface[interface-name='%s']/latest/generic-counters"

	// QoS statistics
	PathStateQoSStats           = "/qos/nodes/node[node-name='%s']/policy-map/interface-table/interface[interface-name='%s']"

	// System state
	PathStateSystem             = "/system-monitoring"
	PathStateCPU                = "/system-monitoring/cpu-utilization"
	PathStateMemory             = "/system-monitoring/memory-statistics"
)

// gNMI Telemetry Paths
const (
	// Subscriber telemetry
	GNMISubscriberSession   = "/Cisco-IOS-XR-subscriber-session-mon-oper:subscriber-session-mon/nodes/node/session-ids/session-id"
	GNMISubscriberSummary   = "/Cisco-IOS-XR-subscriber-session-mon-oper:subscriber-session-mon/nodes/node/summary"
	GNMISubscriberAcct      = "/Cisco-IOS-XR-subscriber-accounting-oper:subscriber-accounting"

	// Interface telemetry
	GNMIInterfaceStats      = "/Cisco-IOS-XR-infra-statsd-oper:infra-statistics/interfaces/interface/latest/generic-counters"
	GNMIInterfaceDataRates  = "/Cisco-IOS-XR-infra-statsd-oper:infra-statistics/interfaces/interface/data-rate"

	// QoS telemetry
	GNMIQoSInterfaceStats   = "/Cisco-IOS-XR-qos-ma-oper:qos/interface-table/interface/input/service-policy-names/service-policy-instance/statistics"

	// System telemetry
	GNMICPUUtilization      = "/Cisco-IOS-XR-wdsysmon-fd-oper:system-monitoring/cpu-utilization"
	GNMIMemoryStats         = "/Cisco-IOS-XR-nto-misc-shmem-oper:memory-summary/nodes/node/summary"
)

// XML Templates for common operations

// DynamicTemplatePPPoEXML creates a PPPoE dynamic template
const DynamicTemplatePPPoEXML = `
<ppp xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-subscriber-pppoe-ma-gbl-cfg">
  <template-name>%s</template-name>
  <ppp xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ppp-ma-gbl-cfg">
    <pap>
      <send-user-info/>
    </pap>
    <chap>
      <host-name>%s</host-name>
    </chap>
  </ppp>
  <ipv4-network xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ipv4-ma-subscriber-cfg">
    <unnumbered>%s</unnumbered>
  </ipv4-network>
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
</ppp>`

// DynamicTemplateIPSubXML creates an IPoE dynamic template
const DynamicTemplateIPSubXML = `
<ip-subscriber xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-subscriber-ipsub-cfg">
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
</ip-subscriber>`

// ServicePolicyMapXML creates a QoS policy map for subscriber rate limiting
const ServicePolicyMapXML = `
<policy-map xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-qos-ma-cfg">
  <name>%s</name>
  <policy-map-type>qos</policy-map-type>
  <policy-map-rule>
    <class-name>class-default</class-name>
    <class-type>qos</class-type>
    <police-rate>
      <rate>%d</rate>
      <rate-unit>kbps</rate-unit>
      <peak-rate>%d</peak-rate>
      <peak-rate-unit>kbps</peak-rate-unit>
      <burst>
        <value>%d</value>
        <units>kbytes</units>
      </burst>
      <peak-burst>
        <value>%d</value>
        <units>kbytes</units>
      </peak-burst>
      <conform-action>
        <transmit/>
      </conform-action>
      <exceed-action>
        <drop/>
      </exceed-action>
    </police-rate>
  </policy-map-rule>
</policy-map>`

// InterfaceConfigXML creates a subscriber-facing interface configuration
const InterfaceConfigXML = `
<interface-configuration xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ifmgr-cfg">
  <active>act</active>
  <interface-name>%s</interface-name>
  <description>%s</description>
  <shutdown nc:operation="delete" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0"/>
  <ipv4-io xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ipv4-io-cfg">
    <addresses>
      <primary>
        <address>%s</address>
        <netmask>%s</netmask>
      </primary>
    </addresses>
  </ipv4-io>
</interface-configuration>`

// SubInterfaceConfigXML creates a VLAN sub-interface for a subscriber
const SubInterfaceConfigXML = `
<interface-configuration xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ifmgr-cfg">
  <active>act</active>
  <interface-name>%s.%d</interface-name>
  <interface-mode-non-physical>l2-transport</interface-mode-non-physical>
  <description>Subscriber %s VLAN %d</description>
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
</interface-configuration>`

// RADIUSServerXML configures a RADIUS server
const RADIUSServerXML = `
<host xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-aaa-protocol-radius-cfg">
  <ordering-index>%d</ordering-index>
  <ip-address>%s</ip-address>
  <auth-port-number>%d</auth-port-number>
  <acct-port-number>%d</acct-port-number>
  <host-key>%s</host-key>
  <host-timeout>%d</host-timeout>
  <host-retransmit>%d</host-retransmit>
</host>`

// DeleteInterfaceXML removes an interface configuration
const DeleteInterfaceXML = `
<interface-configuration xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ifmgr-cfg" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0" nc:operation="delete">
  <active>act</active>
  <interface-name>%s</interface-name>
</interface-configuration>`

// ShutdownInterfaceXML shuts down an interface (suspend)
const ShutdownInterfaceXML = `
<interface-configuration xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ifmgr-cfg">
  <active>act</active>
  <interface-name>%s</interface-name>
  <shutdown/>
</interface-configuration>`

// NoShutdownInterfaceXML brings up an interface (resume)
const NoShutdownInterfaceXML = `
<interface-configuration xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ifmgr-cfg" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0">
  <active>act</active>
  <interface-name>%s</interface-name>
  <shutdown nc:operation="delete"/>
</interface-configuration>`

// GetSubscriberSessionFilterXML is the filter for getting subscriber sessions
const GetSubscriberSessionFilterXML = `
<subscriber-session-mon xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-subscriber-session-mon-oper">
  <nodes>
    <node>
      <node-name>%s</node-name>
      <session-ids>
        <session-id>
          <session-id>%s</session-id>
        </session-id>
      </session-ids>
    </node>
  </nodes>
</subscriber-session-mon>`

// GetSubscriberSummaryFilterXML is the filter for subscriber summary
const GetSubscriberSummaryFilterXML = `
<subscriber-session-mon xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-subscriber-session-mon-oper">
  <nodes>
    <node>
      <node-name>%s</node-name>
      <summary/>
    </node>
  </nodes>
</subscriber-session-mon>`

// GetInterfaceStatsFilterXML is the filter for interface statistics
const GetInterfaceStatsFilterXML = `
<infra-statistics xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-infra-statsd-oper">
  <interfaces>
    <interface>
      <interface-name>%s</interface-name>
      <latest>
        <generic-counters/>
      </latest>
    </interface>
  </interfaces>
</infra-statistics>`

// GetSystemInfoFilterXML is the filter for system information
const GetSystemInfoFilterXML = `
<system-monitoring xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-wdsysmon-fd-oper">
  <cpu-utilization/>
</system-monitoring>`

// Helper types for parsing Cisco responses

// SubscriberSession represents a parsed subscriber session
type SubscriberSession struct {
	SessionID       string
	SubscriberLabel string
	State           string
	MACAddress      string
	IPv4Address     string
	IPv6Address     string
	Interface       string
	VLAN            int
	UptimeSecs      int64
	AccountingID    string
	ServiceType     string // pppoe, ipoe, etc.
}

// SubscriberStats represents subscriber traffic statistics
type SubscriberStats struct {
	BytesIn       uint64
	BytesOut      uint64
	PacketsIn     uint64
	PacketsOut    uint64
	DropsIn       uint64
	DropsOut      uint64
}

// InterfaceStats represents interface statistics
type InterfaceStats struct {
	BytesReceived      uint64
	BytesSent          uint64
	PacketsReceived    uint64
	PacketsSent        uint64
	InputErrors        uint64
	OutputErrors       uint64
	InputDrops         uint64
	OutputDrops        uint64
	InputCRCErrors     uint64
	OutputBufferFails  uint64
}

// SystemInfo represents system information
type SystemInfo struct {
	Hostname      string
	Version       string
	Platform      string
	UptimeSecs    int64
	CPUPercent    float64
	MemoryUsed    uint64
	MemoryFree    uint64
}

// SubscriberSummary represents subscriber count summary
type SubscriberSummary struct {
	TotalSessions     int
	PPPoESessions     int
	IPoESessions      int
	ActiveSessions    int
	InitiatingSessions int
}
