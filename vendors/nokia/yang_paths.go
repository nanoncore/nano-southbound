package nokia

// Nokia SR OS YANG Paths and XML Templates
// Reference: Nokia SR OS YANG models for subscriber management
// Supports: SR OS 19.x, 20.x, 21.x, 22.x, 23.x

// YANG Namespaces
const (
	// Nokia SR OS namespaces
	NSNokiaConf  = "urn:nokia.com:sros:ns:yang:sr:conf"
	NSNokiaState = "urn:nokia.com:sros:ns:yang:sr:state"
	NSNokiaTypes = "urn:nokia.com:sros:ns:yang:sr:types-sros"

	// Standard namespaces
	NSNetconfBase = "urn:ietf:params:xml:ns:netconf:base:1.0"
	NSYANG        = "urn:ietf:params:xml:ns:yang:1"
)

// Configuration Paths - Subscriber Management
const (
	// VPRN Service paths
	PathVPRN                = "/configure/service/vprn"
	PathVPRNSubscriberIface = "/configure/service/vprn[service-name='%s']/subscriber-interface[interface-name='%s']"
	PathVPRNGroupIface      = "/configure/service/vprn[service-name='%s']/subscriber-interface[interface-name='%s']/group-interface[group-interface-name='%s']"

	// Subscriber Management paths
	PathSubMgmt        = "/configure/subscriber-mgmt"
	PathSubProfile     = "/configure/subscriber-mgmt/sub-profile[sub-profile-name='%s']"
	PathSLAProfile     = "/configure/subscriber-mgmt/sla-profile[sla-profile-name='%s']"
	PathSubIdentPolicy = "/configure/subscriber-mgmt/sub-ident-policy[sub-ident-policy-name='%s']"
	PathMSAPPolicy     = "/configure/subscriber-mgmt/msap-policy[msap-policy-name='%s']"

	// QoS paths
	PathQoS          = "/configure/qos"
	PathSapIngress   = "/configure/qos/sap-ingress[sap-ingress-policy-name='%s']"
	PathSapEgress    = "/configure/qos/sap-egress[sap-egress-policy-name='%s']"
	PathSchedulerPol = "/configure/qos/scheduler-policy[scheduler-policy-name='%s']"
	PathQueuePol     = "/configure/qos/queue-policy[queue-policy-name='%s']"
	PathPolicer      = "/configure/qos/sap-ingress[sap-ingress-policy-name='%s']/policer[policer-id='%d']"

	// Port/Interface paths
	PathPort     = "/configure/port[port-id='%s']"
	PathEthernet = "/configure/port[port-id='%s']/ethernet"
	PathLag      = "/configure/lag[lag-name='%s']"

	// Router paths
	PathRouter    = "/configure/router[router-name='%s']"
	PathInterface = "/configure/router[router-name='%s']/interface[interface-name='%s']"

	// RADIUS paths
	PathRADIUS       = "/configure/aaa/radius"
	PathRADIUSServer = "/configure/aaa/radius/server[name='%s']"
	PathRADIUSPolicy = "/configure/aaa/radius/policy[name='%s']"

	// DHCP paths
	PathDHCPServer   = "/configure/service/vprn[service-name='%s']/dhcp/local-dhcp-server[server-name='%s']"
	PathDHCPPool     = "/configure/service/vprn[service-name='%s']/dhcp/local-dhcp-server[server-name='%s']/pool[pool-name='%s']"
	PathDHCPv6Server = "/configure/service/vprn[service-name='%s']/dhcp6/local-dhcp-server[server-name='%s']"
)

// State Paths - Telemetry and Monitoring
const (
	// Subscriber session state
	PathStateSubscribers = "/state/subscriber-mgmt/subscriber"
	PathStateSubscriber  = "/state/subscriber-mgmt/subscriber[subscriber-id='%s']"
	PathStateSubSession  = "/state/service/vprn[service-name='%s']/subscriber-interface/group-interface/sap/sub-sla-mgmt/sub-ident[subscriber-id='%s']"

	// Subscriber statistics
	PathStateSubStats    = "/state/subscriber-mgmt/subscriber[subscriber-id='%s']/statistics"
	PathStateSubSapStats = "/state/service/vprn[service-name='%s']/subscriber-interface[interface-name='%s']/group-interface[group-interface-name='%s']/sap[sap-id='%s']/statistics"

	// Port/Interface statistics
	PathStatePort          = "/state/port[port-id='%s']"
	PathStatePortStats     = "/state/port[port-id='%s']/statistics"
	PathStateEthernetStats = "/state/port[port-id='%s']/ethernet/statistics"

	// System state
	PathStateSystem     = "/state/system"
	PathStateSystemInfo = "/state/system/information"
	PathStateCPU        = "/state/system/cpu"
	PathStateMemory     = "/state/system/memory-pools"

	// VPRN state
	PathStateVPRN         = "/state/service/vprn[service-name='%s']"
	PathStateVPRNSubIface = "/state/service/vprn[service-name='%s']/subscriber-interface[interface-name='%s']"
)

// gNMI Paths for streaming telemetry
const (
	// Subscriber telemetry (gNMI paths use same structure, different encoding)
	GNMISubSession = "/nokia-state:state/subscriber-mgmt/subscriber"
	GNMISubStats   = "/nokia-state:state/subscriber-mgmt/subscriber[subscriber-id=%s]/statistics"

	// Interface telemetry
	GNMIPortStats     = "/nokia-state:state/port[port-id=%s]/statistics"
	GNMIEthernetStats = "/nokia-state:state/port[port-id=%s]/ethernet/statistics"

	// System telemetry
	GNMISystemInfo  = "/nokia-state:state/system/information"
	GNMICPUStats    = "/nokia-state:state/system/cpu[sample-period=1]"
	GNMIMemoryStats = "/nokia-state:state/system/memory-pools"

	// QoS telemetry
	GNMISapIngressStats = "/nokia-state:state/service/vprn[service-name=%s]/interface[interface-name=%s]/sap[sap-id=%s]/ingress/qos/sap-ingress/queue[queue-id=%d]/statistics"
	GNMISapEgressStats  = "/nokia-state:state/service/vprn[service-name=%s]/interface[interface-name=%s]/sap[sap-id=%s]/egress/qos/sap-egress/queue[queue-id=%d]/statistics"
)

// XML Templates for common operations

// SubscriberProfileXML creates a subscriber profile configuration
const SubscriberProfileXML = `
<sub-profile xmlns="urn:nokia.com:sros:ns:yang:sr:conf">
  <sub-profile-name>%s</sub-profile-name>
  <description>%s</description>
  <sla-profile-string>%s</sla-profile-string>
  <sub-ident-policy>%s</sub-ident-policy>
  <ingress>
    <policer>
      <policer-id>1</policer-id>
      <rate>
        <cir>%d</cir>
        <pir>%d</pir>
      </rate>
    </policer>
  </ingress>
  <egress>
    <policer>
      <policer-id>1</policer-id>
      <rate>
        <cir>%d</cir>
        <pir>%d</pir>
      </rate>
    </policer>
  </egress>
</sub-profile>`

// SLAProfileXML creates an SLA profile configuration
const SLAProfileXML = `
<sla-profile xmlns="urn:nokia.com:sros:ns:yang:sr:conf">
  <sla-profile-name>%s</sla-profile-name>
  <description>%s</description>
  <ingress>
    <qos>
      <sap-ingress>
        <policy-name>%s</policy-name>
      </sap-ingress>
    </qos>
  </ingress>
  <egress>
    <qos>
      <sap-egress>
        <policy-name>%s</policy-name>
      </sap-egress>
    </qos>
  </egress>
</sla-profile>`

// SubscriberInterfaceXML creates a subscriber interface configuration
const SubscriberInterfaceXML = `
<subscriber-interface xmlns="urn:nokia.com:sros:ns:yang:sr:conf">
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
    </sap>
  </group-interface>
</subscriber-interface>`

// StaticSubscriberXML creates a static subscriber host entry
const StaticSubscriberXML = `
<subscriber-interface xmlns="urn:nokia.com:sros:ns:yang:sr:conf">
  <interface-name>%s</interface-name>
  <group-interface>
    <group-interface-name>%s</group-interface-name>
    <sap>
      <sap-id>%s</sap-id>
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
</subscriber-interface>`

// SuspendSubscriberXML sets subscriber admin-state to disable
const SuspendSubscriberXML = `
<subscriber-interface xmlns="urn:nokia.com:sros:ns:yang:sr:conf" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0">
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
</subscriber-interface>`

// ResumeSubscriberXML sets subscriber admin-state to enable
const ResumeSubscriberXML = `
<subscriber-interface xmlns="urn:nokia.com:sros:ns:yang:sr:conf" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0">
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
</subscriber-interface>`

// DeleteSubscriberXML removes a static subscriber host entry
const DeleteSubscriberXML = `
<subscriber-interface xmlns="urn:nokia.com:sros:ns:yang:sr:conf" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0">
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
</subscriber-interface>`

// SapIngressPolicyXML creates a SAP ingress QoS policy
const SapIngressPolicyXML = `
<sap-ingress xmlns="urn:nokia.com:sros:ns:yang:sr:conf">
  <sap-ingress-policy-name>%s</sap-ingress-policy-name>
  <description>%s</description>
  <policer>
    <policer-id>1</policer-id>
    <rate>
      <cir>%d</cir>
      <pir>%d</pir>
    </rate>
    <mbs>%d</mbs>
  </policer>
  <queue>
    <queue-id>1</queue-id>
    <queue-type>expedite</queue-type>
    <rate>
      <cir>%d</cir>
      <pir>%d</pir>
    </rate>
  </queue>
</sap-ingress>`

// SapEgressPolicyXML creates a SAP egress QoS policy
const SapEgressPolicyXML = `
<sap-egress xmlns="urn:nokia.com:sros:ns:yang:sr:conf">
  <sap-egress-policy-name>%s</sap-egress-policy-name>
  <description>%s</description>
  <queue>
    <queue-id>1</queue-id>
    <rate>
      <cir>%d</cir>
      <pir>%d</pir>
    </rate>
  </queue>
</sap-egress>`

// GetSubscriberFilterXML is the filter for getting subscriber state
const GetSubscriberFilterXML = `
<state xmlns="urn:nokia.com:sros:ns:yang:sr:state">
  <subscriber-mgmt>
    <subscriber>
      <subscriber-id>%s</subscriber-id>
    </subscriber>
  </subscriber-mgmt>
</state>`

// GetSubscriberStatsFilterXML is the filter for subscriber statistics
const GetSubscriberStatsFilterXML = `
<state xmlns="urn:nokia.com:sros:ns:yang:sr:state">
  <subscriber-mgmt>
    <subscriber>
      <subscriber-id>%s</subscriber-id>
      <statistics/>
    </subscriber>
  </subscriber-mgmt>
</state>`

// GetSystemInfoFilterXML is the filter for system information
const GetSystemInfoFilterXML = `
<state xmlns="urn:nokia.com:sros:ns:yang:sr:state">
  <system>
    <information/>
  </system>
</state>`

// Helper types for parsing Nokia responses

// SubscriberState represents the parsed subscriber state from Nokia
type SubscriberState struct {
	SubscriberID string
	AdminState   string
	OperState    string
	MAC          string
	IPv4Address  string
	IPv6Address  string
	SubProfile   string
	SLAProfile   string
	UptimeSecs   int64
}

// SubscriberStats represents subscriber traffic statistics
type SubscriberStats struct {
	IngressOctets  uint64
	EgressOctets   uint64
	IngressPackets uint64
	EgressPackets  uint64
	IngressDrops   uint64
	EgressDrops    uint64
}

// PortStats represents port statistics
type PortStats struct {
	InOctets    uint64
	OutOctets   uint64
	InPackets   uint64
	OutPackets  uint64
	InErrors    uint64
	OutErrors   uint64
	InDiscards  uint64
	OutDiscards uint64
}

// SystemInfo represents system information
type SystemInfo struct {
	Name        string
	Type        string
	Version     string
	UptimeSecs  int64
	CPUPercent  float64
	MemoryUsed  uint64
	MemoryTotal uint64
}
