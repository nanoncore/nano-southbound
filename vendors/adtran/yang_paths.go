package adtran

// Adtran SDX 6000 Series YANG Paths and XML Templates
// Reference: Adtran SDX 6000 OLT NETCONF/YANG models + BBF TR-385
// Supports: SDX 6324 (4-port Combo PON), SDX 6020-48 (48-port GPON),
//           SDX 6320-16 (16-port 10G Combo PON), SDX 6312-4 (Remote OLT)
// Note: SDX 6xxx series does NOT support SNMP - uses NETCONF/YANG and REST API only

// YANG Namespaces - Adtran uses BBF standard models with vendor extensions
const (
	// BBF Standard namespaces (TR-385 xPON)
	NSBBFXpon         = "urn:bbf:yang:bbf-xpon"
	NSBBFXponTypes    = "urn:bbf:yang:bbf-xpon-types"
	NSBBFXponONUState = "urn:bbf:yang:bbf-xpon-onu-states"
	NSBBFQos          = "urn:bbf:yang:bbf-qos-policies"
	NSBBFSubIf        = "urn:bbf:yang:bbf-sub-interfaces"
	NSBBFForwarder    = "urn:bbf:yang:bbf-forwarder"

	// IETF Standard namespaces
	NSIETFInterfaces = "urn:ietf:params:xml:ns:yang:ietf-interfaces"
	NSIETFSystem     = "urn:ietf:params:xml:ns:yang:ietf-system"
	NSNetconfBase    = "urn:ietf:params:xml:ns:netconf:base:1.0"

	// Adtran vendor-specific extensions
	NSAdtranXpon    = "http://www.adtran.com/ns/yang/adtran-xpon"
	NSAdtranSystem  = "http://www.adtran.com/ns/yang/adtran-system"
	NSAdtranService = "http://www.adtran.com/ns/yang/adtran-service"
)

// Configuration Paths - BBF TR-385 compliant with Adtran extensions
const (
	// Channel termination (PON port) paths
	PathChannelTerminations = "/bbf-xpon:xpon/channel-terminations"
	PathChannelTermination  = "/bbf-xpon:xpon/channel-terminations/channel-termination[name='%s']"

	// Channel pair paths (wavelength configuration)
	PathChannelPairs = "/bbf-xpon:xpon/channel-pairs"
	PathChannelPair  = "/bbf-xpon:xpon/channel-pairs/channel-pair[name='%s']"

	// ONU paths (BBF standard)
	PathONUs        = "/bbf-xpon:xpon/onus"
	PathONU         = "/bbf-xpon:xpon/onus/onu[name='%s']"
	PathONUBySerial = "/bbf-xpon:xpon/onus/onu[serial-number='%s']"

	// V-ANI (Virtual ANI) paths
	PathVANIs = "/bbf-xpon:xpon/v-anis"
	PathVANI  = "/bbf-xpon:xpon/v-anis/v-ani[name='%s']"

	// Interface paths
	PathInterfaces = "/ietf-interfaces:interfaces"
	PathInterface  = "/ietf-interfaces:interfaces/interface[name='%s']"

	// QoS/Bandwidth profile paths
	PathQoSProfiles = "/bbf-qos:qos-policies"
	PathQoSProfile  = "/bbf-qos:qos-policies/qos-policy[name='%s']"

	// Adtran-specific service paths
	PathAdtranServiceProfiles = "/adtran-service:service-profiles"
	PathAdtranServiceProfile  = "/adtran-service:service-profiles/service-profile[name='%s']"
	PathAdtranBWProfiles      = "/adtran-service:bandwidth-profiles"
	PathAdtranBWProfile       = "/adtran-service:bandwidth-profiles/bandwidth-profile[name='%s']"

	// Forwarder/bridge paths (TR-383)
	PathForwarders = "/bbf-forwarder:forwarders"
	PathForwarder  = "/bbf-forwarder:forwarders/forwarder[name='%s']"

	// System paths
	PathSystem     = "/ietf-system:system"
	PathSystemInfo = "/ietf-system:system-state"
)

// State/Telemetry Paths
const (
	// ONU state (BBF TR-385)
	PathONUStates        = "/bbf-xpon-onu-states:xpon-onu-states"
	PathONUState         = "/bbf-xpon-onu-states:xpon-onu-states/onu-state[onu-ref='%s']"
	PathONUPresenceState = "/bbf-xpon-onu-states:xpon-onu-states/onu-state[onu-ref='%s']/onu-presence-state"
	PathONUOpticalInfo   = "/bbf-xpon-onu-states:xpon-onu-states/onu-state[onu-ref='%s']/optical-info"

	// Interface state
	PathInterfaceState    = "/ietf-interfaces:interfaces-state/interface[name='%s']"
	PathInterfaceCounters = "/ietf-interfaces:interfaces-state/interface[name='%s']/statistics"

	// Channel termination statistics
	PathCTStats = "/ietf-interfaces:interfaces-state/interface[name='%s']/bbf-xpon:statistics"

	// System state
	PathSystemState  = "/ietf-system:system-state"
	PathSystemCPU    = "/adtran-system:system-state/cpu-utilization"
	PathSystemMemory = "/adtran-system:system-state/memory-utilization"
)

// REST API Endpoints (Adtran SDX 6000 also supports REST)
const (
	RESTBaseURL       = "/restconf/data"
	RESTONUs          = "/restconf/data/bbf-xpon:xpon/onus"
	RESTONUStates     = "/restconf/data/bbf-xpon-onu-states:xpon-onu-states"
	RESTInterfaces    = "/restconf/data/ietf-interfaces:interfaces"
	RESTSystem        = "/restconf/data/ietf-system:system"
	RESTOperations    = "/restconf/operations"
)

// XML Templates for BBF TR-385 compliant operations

// ONUProvisionXML provisions an ONU using BBF TR-385 model
const ONUProvisionXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon">
  <onus>
    <onu>
      <name>%s</name>
      <serial-number>%s</serial-number>
      <channel-partition-ref>%s</channel-partition-ref>
      <expected-registration-id>%s</expected-registration-id>
      <admin-state>unlocked</admin-state>
    </onu>
  </onus>
</xpon>`

// VANIProvisionXML creates a Virtual ANI for an ONU
const VANIProvisionXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon">
  <v-anis>
    <v-ani>
      <name>%s</name>
      <onu-ref>%s</onu-ref>
      <expected-serial-number>%s</expected-serial-number>
      <preferred-channel-pair>%s</preferred-channel-pair>
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
      <admin-state>unlocked</admin-state>
      <meant-for-type-b-primary-role>true</meant-for-type-b-primary-role>
    </channel-termination>
  </channel-terminations>
</xpon>`

// BWProfileXML creates a bandwidth profile (Adtran extension)
const BWProfileXML = `
<bandwidth-profiles xmlns="http://www.adtran.com/ns/yang/adtran-service">
  <bandwidth-profile>
    <name>%s</name>
    <description>%s</description>
    <cir>%d</cir>
    <pir>%d</pir>
    <cbs>%d</cbs>
    <pbs>%d</pbs>
  </bandwidth-profile>
</bandwidth-profiles>`

// ServiceProfileXML creates a service profile for subscriber services
const ServiceProfileXML = `
<service-profiles xmlns="http://www.adtran.com/ns/yang/adtran-service">
  <service-profile>
    <name>%s</name>
    <description>%s</description>
    <service-type>%s</service-type>
    <vlan-config>
      <c-vlan>%d</c-vlan>
      <s-vlan>%d</s-vlan>
      <vlan-action>%s</vlan-action>
    </vlan-config>
    <bandwidth-profile-ref>%s</bandwidth-profile-ref>
  </service-profile>
</service-profiles>`

// SubInterfaceConfigXML creates a subscriber sub-interface on a V-ANI
const SubInterfaceConfigXML = `
<interfaces xmlns="urn:ietf:params:xml:ns:yang:ietf-interfaces">
  <interface>
    <name>%s</name>
    <type xmlns:bbfift="urn:bbf:yang:bbf-if-type">bbfift:vlan-sub-interface</type>
    <enabled>true</enabled>
    <sub-interfaces xmlns="urn:bbf:yang:bbf-sub-interfaces">
      <sub-interface>
        <name>%s</name>
        <ingress-qos-policy-profile>%s</ingress-qos-policy-profile>
        <egress-qos-policy-profile>%s</egress-qos-policy-profile>
      </sub-interface>
    </sub-interfaces>
  </interface>
</interfaces>`

// SuspendONUXML locks an ONU (admin-state locked)
const SuspendONUXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon">
  <onus>
    <onu>
      <name>%s</name>
      <admin-state>locked</admin-state>
    </onu>
  </onus>
</xpon>`

// ResumeONUXML unlocks an ONU (admin-state unlocked)
const ResumeONUXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon">
  <onus>
    <onu>
      <name>%s</name>
      <admin-state>unlocked</admin-state>
    </onu>
  </onus>
</xpon>`

// DeleteONUXML removes an ONU configuration
const DeleteONUXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0">
  <onus>
    <onu nc:operation="delete">
      <name>%s</name>
    </onu>
  </onus>
</xpon>`

// DeleteVANIXML removes a V-ANI configuration
const DeleteVANIXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0">
  <v-anis>
    <v-ani nc:operation="delete">
      <name>%s</name>
    </v-ani>
  </v-anis>
</xpon>`

// GetONUStateFilterXML is the filter for getting ONU operational state
const GetONUStateFilterXML = `
<xpon-onu-states xmlns="urn:bbf:yang:bbf-xpon-onu-states">
  <onu-state>
    <onu-ref>%s</onu-ref>
  </onu-state>
</xpon-onu-states>`

// GetAllONUStatesFilterXML gets all ONU states
const GetAllONUStatesFilterXML = `
<xpon-onu-states xmlns="urn:bbf:yang:bbf-xpon-onu-states"/>`

// GetCTStatsFilterXML is the filter for channel termination statistics
const GetCTStatsFilterXML = `
<interfaces-state xmlns="urn:ietf:params:xml:ns:yang:ietf-interfaces">
  <interface>
    <name>%s</name>
  </interface>
</interfaces-state>`

// GetSystemInfoFilterXML is the filter for system information
const GetSystemInfoFilterXML = `
<system-state xmlns="urn:ietf:params:xml:ns:yang:ietf-system"/>`

// GetONTStateFilterXML is an alias for GetONUStateFilterXML (Adtran uses ONT terminology)
const GetONTStateFilterXML = `
<ont-state xmlns="http://www.adtran.com/ns/yang/adtran-ont">
  <serial-number>%s</serial-number>
</ont-state>`

// GetServiceStatsFilterXML is the filter for service statistics
const GetServiceStatsFilterXML = `
<service-state xmlns="http://www.adtran.com/ns/yang/adtran-service">
  <service-id>%s</service-id>
</service-state>`

// ONU Presence States (BBF TR-385)
const (
	ONUPresenceNotPresent            = "onu-not-present"
	ONUPresenceOnExpectedCT          = "onu-present-and-on-expected-channel-termination"
	ONUPresenceOnUnexpectedCT        = "onu-present-and-on-unexpected-channel-termination"
	ONUPresenceEmergencyStop         = "onu-present-and-in-emergency-stop-state"
	ONUPresenceDyingGasp             = "onu-present-and-in-dying-gasp-state"
	ONUPresenceNotPresentWithVANI    = "onu-not-present-with-v-ani"
	ONUPresencePresentWithoutVANI    = "onu-present-without-v-ani"
)

// Admin States (BBF standard)
const (
	AdminStateLocked       = "locked"
	AdminStateUnlocked     = "unlocked"
	AdminStateShuttingDown = "shutting-down"
)

// SDX 6000 Series Model Information
type SDXModelInfo struct {
	Model           string
	PONPorts        int
	PONType         string // gpon, xgs-pon, combo
	MaxSubscribers  int
	UplinkPorts     string
	FormFactor      string
}

// Supported SDX 6000 Models
var SDX6000Models = map[string]SDXModelInfo{
	"SDX-6324": {
		Model:          "SDX 6324",
		PONPorts:       4,
		PONType:        "combo", // 10G Combo PON
		MaxSubscribers: 500,
		UplinkPorts:    "4x10GbE",
		FormFactor:     "Compact",
	},
	"SDX-6020-48": {
		Model:          "SDX 6020-48",
		PONPorts:       48,
		PONType:        "gpon",
		MaxSubscribers: 3072,
		UplinkPorts:    "4x100GbE + 4x10GbE",
		FormFactor:     "2RU",
	},
	"SDX-6320-16": {
		Model:          "SDX 6320-16",
		PONPorts:       16,
		PONType:        "combo", // GPON, XGS-PON, or 10G Combo PON
		MaxSubscribers: 1024,
		UplinkPorts:    "4x100GbE + 4x10GbE",
		FormFactor:     "1.5RU",
	},
	"SDX-6312-4": {
		Model:          "SDX 6312-4",
		PONPorts:       4,
		PONType:        "combo", // 10G Combo PON Remote OLT
		MaxSubscribers: 512,
		UplinkPorts:    "2x10GbE",
		FormFactor:     "Sealed/Outdoor",
	},
}

// Helper types for parsing responses

// ONUState represents the parsed ONU state from BBF model
type ONUState struct {
	Name             string
	SerialNumber     string
	ChannelPartition string
	VANI             string
	PresenceState    string
	AdminState       string
	OperState        string
	RxPowerDbm       float64
	TxPowerDbm       float64
	Temperature      float64
	Voltage          float64
	Distance         int
	LastStateChange  string
	UptimeSecs       int64
}

// ONUCounters represents ONU traffic counters
type ONUCounters struct {
	BytesRx          uint64
	BytesTx          uint64
	FramesRx         uint64
	FramesTx         uint64
	BroadcastFrames  uint64
	MulticastFrames  uint64
	DroppedFrames    uint64
	ErrorFrames      uint64
	FECCorrected     uint64
	FECUncorrected   uint64
}

// ChannelTerminationStats represents PON port statistics
type ChannelTerminationStats struct {
	Name           string
	AdminState     string
	OperState      string
	TotalONUs      int
	ActiveONUs     int
	BytesRx        uint64
	BytesTx        uint64
	FramesRx       uint64
	FramesTx       uint64
	GEMPortsActive int
}

// SystemInfo represents system information
type SystemInfo struct {
	Hostname       string
	Model          string
	SerialNumber   string
	SoftwareVer    string
	UptimeSecs     int64
	CPUPercent     float64
	MemoryPercent  float64
	Temperature    float64
}

// ServiceProfile represents a configured service profile
type ServiceProfile struct {
	Name           string
	Description    string
	ServiceType    string
	CVLAN          int
	SVLAN          int
	VLANAction     string
	BWProfileRef   string
}

// BandwidthProfile represents a bandwidth/QoS profile
type BandwidthProfile struct {
	Name        string
	Description string
	CIR         uint64 // Committed Information Rate (kbps)
	PIR         uint64 // Peak Information Rate (kbps)
	CBS         uint64 // Committed Burst Size (bytes)
	PBS         uint64 // Peak Burst Size (bytes)
}

// ONTState represents ONT state as used in adapter.go (Adtran terminology)
// This is similar to ONUState but uses Adtran's ONT naming convention
type ONTState struct {
	SerialNumber string
	ONTID        int
	PONPort      string
	AdminState   string
	OperState    string
	Description  string
	RxPower      float64
	TxPower      float64
	Temperature  float64
	Voltage      float64
	Distance     int
	LastOnline   string
	LastOffline  string
	UptimeSecs   int64
}

// ServiceStats represents service statistics for a subscriber
type ServiceStats struct {
	ServiceID   string
	State       string
	BytesUp     uint64
	BytesDown   uint64
	PacketsUp   uint64
	PacketsDown uint64
}
