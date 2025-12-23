package netconf

// Broadband Forum (BBF) YANG Models and Paths
// Reference: TR-385 (xPON YANG), TR-383 (Common Fixed Access YANG)
// Used by Nokia Lightspan, Adtran SDX, and other BBF-compliant OLTs

// BBF YANG Namespaces (TR-385 xPON)
const (
	// Core xPON namespaces
	NSBBFXpon          = "urn:bbf:yang:bbf-xpon"
	NSBBFXponTypes     = "urn:bbf:yang:bbf-xpon-types"
	NSBBFXponIfType    = "urn:bbf:yang:bbf-xpon-if-type"
	NSBBFXponONUState  = "urn:bbf:yang:bbf-xpon-onu-state"
	NSBBFXponONUStates = "urn:bbf:yang:bbf-xpon-onu-states"
	NSBBFFiberTypes    = "urn:bbf:yang:bbf-fiber-types"

	// PON technology-specific namespaces
	NSBBFGpon    = "urn:bbf:yang:bbf-gpon"
	NSBBFXgsPon  = "urn:bbf:yang:bbf-xgs-pon"
	NSBBF25gPon  = "urn:bbf:yang:bbf-25gs-pon"
	NSBBFTwdmPon = "urn:bbf:yang:bbf-ngpon2"
	NSBBF10gEpon = "urn:bbf:yang:bbf-epon"

	// Hardware/inventory namespaces
	NSBBFHardware      = "urn:bbf:yang:bbf-hardware"
	NSBBFHardwareTypes = "urn:bbf:yang:bbf-hardware-types"

	// QoS namespaces
	NSBBFQos               = "urn:bbf:yang:bbf-qos-policies"
	NSBBFQosTypes          = "urn:bbf:yang:bbf-qos-types"
	NSBBFQosTrafficShaping = "urn:bbf:yang:bbf-qos-traffic-shaping"

	// Subscriber/service namespaces (TR-383)
	NSBBFSubIf        = "urn:bbf:yang:bbf-sub-interfaces"
	NSBBFSubIfTagging = "urn:bbf:yang:bbf-sub-interface-tagging"
	NSBBFFrameClass   = "urn:bbf:yang:bbf-frame-classification"
	NSBBFForwarder    = "urn:bbf:yang:bbf-forwarder"
	NSBBFL2Forwarder  = "urn:bbf:yang:bbf-l2-forwarder"

	// IETF standard namespaces used with BBF
	NSIETFInterfaces = "urn:ietf:params:xml:ns:yang:ietf-interfaces"
	NSIETFHardware   = "urn:ietf:params:xml:ns:yang:ietf-hardware"
	NSIETFSystem     = "urn:ietf:params:xml:ns:yang:ietf-system"
)

// BBF xPON Configuration Paths (TR-385)
const (
	// Channel group paths (physical PON port grouping)
	PathChannelGroups = "/bbf-xpon:xpon/channel-groups"
	PathChannelGroup  = "/bbf-xpon:xpon/channel-groups/channel-group[name='%s']"

	// Channel partition paths (wavelength partitioning)
	PathChannelPartitions = "/bbf-xpon:xpon/channel-partitions"
	PathChannelPartition  = "/bbf-xpon:xpon/channel-partitions/channel-partition[name='%s']"

	// Channel pair paths (upstream/downstream wavelength pair)
	PathChannelPairs = "/bbf-xpon:xpon/channel-pairs"
	PathChannelPair  = "/bbf-xpon:xpon/channel-pairs/channel-pair[name='%s']"

	// Channel termination paths (OLT-side PON interface)
	PathChannelTerminations = "/bbf-xpon:xpon/channel-terminations"
	PathChannelTermination  = "/bbf-xpon:xpon/channel-terminations/channel-termination[name='%s']"

	// ONU paths
	PathONUs        = "/bbf-xpon:xpon/onus"
	PathONU         = "/bbf-xpon:xpon/onus/onu[name='%s']"
	PathONUBySerial = "/bbf-xpon:xpon/onus/onu[serial-number='%s']"

	// V-ANI (Virtual Access Network Interface) paths
	PathVANIs = "/bbf-xpon:xpon/v-anis"
	PathVANI  = "/bbf-xpon:xpon/v-anis/v-ani[name='%s']"

	// ONU state paths
	PathONUStates = "/bbf-xpon-onu-states:xpon-onu-states"
	PathONUState  = "/bbf-xpon-onu-states:xpon-onu-states/onu-state[onu-ref='%s']"
)

// BBF Interface Configuration Paths
const (
	// Standard IETF interfaces augmented by BBF
	PathInterfaces = "/ietf-interfaces:interfaces"
	PathInterface  = "/ietf-interfaces:interfaces/interface[name='%s']"

	// PON interface types (channel-termination, ani, v-ani, onu-v-vrefpoint)
	PathPONInterface = "/ietf-interfaces:interfaces/interface[name='%s']/bbf-xpon:port"

	// Sub-interface paths (TR-383)
	PathSubInterfaces = "/ietf-interfaces:interfaces/interface[name='%s']/bbf-sub-if:sub-interfaces"
	PathSubInterface  = "/ietf-interfaces:interfaces/interface[name='%s']/bbf-sub-if:sub-interfaces/sub-interface[name='%s']"
)

// BBF QoS Configuration Paths
const (
	// Traffic shaping profiles
	PathQoSProfiles = "/bbf-qos:qos-policies"
	PathQoSProfile  = "/bbf-qos:qos-policies/qos-policy[name='%s']"

	// Bandwidth profiles
	PathBWProfiles = "/bbf-qos-traffic-shaping:traffic-shaping-profiles"
	PathBWProfile  = "/bbf-qos-traffic-shaping:traffic-shaping-profiles/traffic-shaping-profile[name='%s']"

	// Schedulers
	PathSchedulers = "/bbf-qos:schedulers"
	PathScheduler  = "/bbf-qos:schedulers/scheduler[name='%s']"
)

// BBF Forwarding Configuration Paths (TR-383)
const (
	// L2 forwarder (VLAN bridge/switch)
	PathForwarders = "/bbf-forwarder:forwarders"
	PathForwarder  = "/bbf-forwarder:forwarders/forwarder[name='%s']"

	// Forwarding database
	PathForwarderPorts = "/bbf-forwarder:forwarders/forwarder[name='%s']/ports"
	PathForwarderPort  = "/bbf-forwarder:forwarders/forwarder[name='%s']/ports/port[name='%s']"
)

// BBF State/Telemetry Paths
const (
	// ONU operational state
	PathStateONUPresence = "/bbf-xpon-onu-states:xpon-onu-states/onu-state[onu-ref='%s']/onu-presence-state"
	PathStateONUOptical  = "/bbf-xpon-onu-states:xpon-onu-states/onu-state[onu-ref='%s']/optical-info"
	PathStateONUCounters = "/bbf-xpon-onu-states:xpon-onu-states/onu-state[onu-ref='%s']/counters"

	// Channel termination statistics
	PathStateCTCounters = "/ietf-interfaces:interfaces-state/interface[name='%s']/bbf-xpon:statistics"
	PathStateCTONUs     = "/ietf-interfaces:interfaces-state/interface[name='%s']/bbf-xpon:onus"

	// Interface statistics
	PathStateIfCounters = "/ietf-interfaces:interfaces-state/interface[name='%s']/statistics"
)

// XML Templates for BBF xPON Operations

// ONUConfigXML provisions an ONU using BBF TR-385 model
const ONUConfigXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon">
  <onus>
    <onu>
      <name>%s</name>
      <serial-number>%s</serial-number>
      <channel-partition-ref>%s</channel-partition-ref>
      <expected-registration-id>%s</expected-registration-id>
      <admin-state>%s</admin-state>
    </onu>
  </onus>
</xpon>`

// VANIConfigXML creates a Virtual ANI for an ONU
const VANIConfigXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon">
  <v-anis>
    <v-ani>
      <name>%s</name>
      <onu-ref>%s</onu-ref>
      <olt-v-ont-ani-ref>%s</olt-v-ont-ani-ref>
      <management-gemport-aes-indicator>%t</management-gemport-aes-indicator>
    </v-ani>
  </v-anis>
</xpon>`

// ChannelTerminationConfigXML configures a PON port
const ChannelTerminationConfigXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon">
  <channel-terminations>
    <channel-termination>
      <name>%s</name>
      <channel-pair-ref>%s</channel-pair-ref>
      <admin-state>%s</admin-state>
      <location>%s</location>
    </channel-termination>
  </channel-terminations>
</xpon>`

// BWProfileConfigXML creates a bandwidth profile
const BWProfileConfigXML = `
<traffic-shaping-profiles xmlns="urn:bbf:yang:bbf-qos-traffic-shaping">
  <traffic-shaping-profile>
    <name>%s</name>
    <cir>%d</cir>
    <pir>%d</pir>
    <cbs>%d</cbs>
    <pbs>%d</pbs>
  </traffic-shaping-profile>
</traffic-shaping-profiles>`

// SubInterfaceConfigXML creates a subscriber sub-interface
const SubInterfaceConfigXML = `
<interfaces xmlns="urn:ietf:params:xml:ns:yang:ietf-interfaces">
  <interface>
    <name>%s</name>
    <type xmlns:bbf-xpon-if-type="urn:bbf:yang:bbf-xpon-if-type">bbf-xpon-if-type:v-ani</type>
    <enabled>%t</enabled>
    <sub-interfaces xmlns="urn:bbf:yang:bbf-sub-interfaces">
      <sub-interface>
        <name>%s</name>
        <ingress-qos-policy-profile>%s</ingress-qos-policy-profile>
        <egress-qos-policy-profile>%s</egress-qos-policy-profile>
        <inline-frame-processing>
          <ingress-rule>
            <priority>1</priority>
            <match-any/>
          </ingress-rule>
        </inline-frame-processing>
      </sub-interface>
    </sub-interfaces>
  </interface>
</interfaces>`

// ForwarderConfigXML creates an L2 forwarder for VLAN bridging
const ForwarderConfigXML = `
<forwarders xmlns="urn:bbf:yang:bbf-forwarder">
  <forwarder>
    <name>%s</name>
    <ports>
      <port>
        <name>%s</name>
        <sub-interface>%s</sub-interface>
      </port>
      <port>
        <name>%s</name>
        <sub-interface>%s</sub-interface>
      </port>
    </ports>
  </forwarder>
</forwarders>`

// DeleteONUXML removes an ONU configuration
const DeleteONUXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0">
  <onus>
    <onu nc:operation="delete">
      <name>%s</name>
    </onu>
  </onus>
</xpon>`

// GetONUStateFilterXML is the filter for getting ONU operational state
const GetONUStateFilterXML = `
<xpon-onu-states xmlns="urn:bbf:yang:bbf-xpon-onu-states">
  <onu-state>
    <onu-ref>%s</onu-ref>
  </onu-state>
</xpon-onu-states>`

// GetCTStatsFilterXML is the filter for channel termination statistics
const GetCTStatsFilterXML = `
<interfaces-state xmlns="urn:ietf:params:xml:ns:yang:ietf-interfaces">
  <interface>
    <name>%s</name>
    <statistics xmlns="urn:bbf:yang:bbf-xpon"/>
  </interface>
</interfaces-state>`

// BBF ONU Presence States (from TR-385)
const (
	ONUPresenceStateOnuNotPresent               = "onu-not-present"
	ONUPresenceStateOnuPresentAndOnExpectedCT   = "onu-present-and-on-expected-channel-termination"
	ONUPresenceStateOnuPresentAndOnUnexpectedCT = "onu-present-and-on-unexpected-channel-termination"
	ONUPresenceStateOnuPresentAndEmergencyStop  = "onu-present-and-in-emergency-stop-state"
	ONUPresenceStateOnuPresentAndDyingGasp      = "onu-present-and-in-dying-gasp-state"
	ONUPresenceStateOnuNotPresentNoVANI         = "onu-not-present-with-v-ani"
	ONUPresenceStateOnuPresentNoVANI            = "onu-present-without-v-ani"
)

// BBF Admin States
const (
	AdminStateLocked       = "locked"
	AdminStateUnlocked     = "unlocked"
	AdminStateShuttingDown = "shutting-down"
)

// Helper types for parsing BBF responses

// BBFONUState represents parsed ONU state from BBF model
type BBFONUState struct {
	Name             string
	SerialNumber     string
	ChannelPartition string
	PresenceState    string
	AdminState       string
	OperState        string
	RxPowerDbm       float64
	TxPowerDbm       float64
	LaserBiasCurrent float64
	Temperature      float64
	Voltage          float64
	ONUDistance      int
}

// BBFONUCounters represents ONU traffic counters
type BBFONUCounters struct {
	TotalBytesReceived   uint64
	TotalBytesSent       uint64
	TotalFramesReceived  uint64
	TotalFramesSent      uint64
	BroadcastFramesRx    uint64
	BroadcastFramesTx    uint64
	MulticastFramesRx    uint64
	MulticastFramesTx    uint64
	FramesDroppedRx      uint64
	FramesDroppedTx      uint64
	FECCorrectedErrors   uint64
	FECUncorrectedErrors uint64
	BIPErrors            uint64
}

// BBFChannelTerminationStats represents PON port statistics
type BBFChannelTerminationStats struct {
	TotalBytesRx    uint64
	TotalBytesTx    uint64
	TotalFramesRx   uint64
	TotalFramesTx   uint64
	GEMFramesRx     uint64
	GEMFramesTx     uint64
	PLOAMsRx        uint64
	PLOAMsTx        uint64
	ActivatedONUs   int
	DeactivatedONUs int
}

// BBFBandwidthProfile represents a QoS bandwidth profile
type BBFBandwidthProfile struct {
	Name        string
	CIR         uint64 // Committed Information Rate (kbps)
	PIR         uint64 // Peak Information Rate (kbps)
	CBS         uint64 // Committed Burst Size (bytes)
	PBS         uint64 // Peak Burst Size (bytes)
	Description string
}
