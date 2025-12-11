package nokia

// Nokia Lightspan OLT YANG Paths and XML Templates
// Reference: Nokia Lightspan FX/DF/SF/MF series + BBF TR-385 YANG models
// Supports: Lightspan FX-4/8/16, DF-16GW, SF-8M, MF-2 series
// Note: Lightspan uses different OS than SR OS (used by 7750 BNG)
// Management: Altiplano Access Controller (NETCONF) or 5520 AMS (SNMP for legacy)

// Lightspan YANG Namespaces - BBF TR-385 standard + Nokia extensions
const (
	// BBF Standard namespaces (TR-385 xPON)
	NSLightspanBBFXpon         = "urn:bbf:yang:bbf-xpon"
	NSLightspanBBFXponTypes    = "urn:bbf:yang:bbf-xpon-types"
	NSLightspanBBFXponIfType   = "urn:bbf:yang:bbf-xpon-if-type"
	NSLightspanBBFXponONUState = "urn:bbf:yang:bbf-xpon-onu-states"
	NSLightspanBBFQos          = "urn:bbf:yang:bbf-qos-policies"
	NSLightspanBBFSubIf        = "urn:bbf:yang:bbf-sub-interfaces"
	NSLightspanBBFForwarder    = "urn:bbf:yang:bbf-forwarder"
	NSLightspanBBFHardware     = "urn:bbf:yang:bbf-hardware"

	// IETF Standard namespaces
	NSLightspanIETFInterfaces = "urn:ietf:params:xml:ns:yang:ietf-interfaces"
	NSLightspanIETFHardware   = "urn:ietf:params:xml:ns:yang:ietf-hardware"
	NSLightspanIETFSystem     = "urn:ietf:params:xml:ns:yang:ietf-system"
	NSLightspanNetconfBase    = "urn:ietf:params:xml:ns:netconf:base:1.0"

	// Nokia Lightspan-specific extensions
	NSNokiaXponExt     = "urn:nokia:yang:nokia-xpon-extensions"
	NSNokiaOLTHardware = "urn:nokia:yang:nokia-olt-hardware"
	NSNokiaONTMgmt     = "urn:nokia:yang:nokia-ont-management"
	NSNokiaQoSExt      = "urn:nokia:yang:nokia-qos-extensions"
)

// Lightspan Configuration Paths - BBF TR-385 compliant
const (
	// Channel group paths (physical port grouping)
	LSPathChannelGroups    = "/bbf-xpon:xpon/channel-groups"
	LSPathChannelGroup     = "/bbf-xpon:xpon/channel-groups/channel-group[name='%s']"

	// Channel partition paths (wavelength partitioning)
	LSPathChannelPartitions = "/bbf-xpon:xpon/channel-partitions"
	LSPathChannelPartition  = "/bbf-xpon:xpon/channel-partitions/channel-partition[name='%s']"

	// Channel pair paths (upstream/downstream wavelength pair)
	LSPathChannelPairs     = "/bbf-xpon:xpon/channel-pairs"
	LSPathChannelPair      = "/bbf-xpon:xpon/channel-pairs/channel-pair[name='%s']"

	// Channel termination paths (OLT-side PON interface)
	LSPathChannelTerminations = "/bbf-xpon:xpon/channel-terminations"
	LSPathChannelTermination  = "/bbf-xpon:xpon/channel-terminations/channel-termination[name='%s']"

	// ONU paths
	LSPathONUs        = "/bbf-xpon:xpon/onus"
	LSPathONU         = "/bbf-xpon:xpon/onus/onu[name='%s']"
	LSPathONUBySerial = "/bbf-xpon:xpon/onus/onu[serial-number='%s']"

	// V-ANI (Virtual Access Network Interface) paths
	LSPathVANIs = "/bbf-xpon:xpon/v-anis"
	LSPathVANI  = "/bbf-xpon:xpon/v-anis/v-ani[name='%s']"

	// OLT V-ONT-ANI (OLT side of ANI) paths
	LSPathOLTVONTANIs = "/bbf-xpon:xpon/olt-v-ont-anis"
	LSPathOLTVONTANI  = "/bbf-xpon:xpon/olt-v-ont-anis/olt-v-ont-ani[name='%s']"

	// Interface paths
	LSPathInterfaces = "/ietf-interfaces:interfaces"
	LSPathInterface  = "/ietf-interfaces:interfaces/interface[name='%s']"

	// Hardware/inventory paths
	LSPathHardware     = "/ietf-hardware:hardware"
	LSPathHardwareSlot = "/ietf-hardware:hardware/component[name='%s']"

	// QoS paths
	LSPathQoSProfiles = "/bbf-qos:qos-policies"
	LSPathQoSProfile  = "/bbf-qos:qos-policies/qos-policy[name='%s']"

	// Forwarder paths (L2 bridging)
	LSPathForwarders = "/bbf-forwarder:forwarders"
	LSPathForwarder  = "/bbf-forwarder:forwarders/forwarder[name='%s']"
)

// Lightspan State/Telemetry Paths
const (
	// ONU operational state (BBF TR-385)
	LSPathONUStates        = "/bbf-xpon-onu-states:xpon-onu-states"
	LSPathONUState         = "/bbf-xpon-onu-states:xpon-onu-states/onu-state[onu-ref='%s']"
	LSPathONUPresenceState = "/bbf-xpon-onu-states:xpon-onu-states/onu-state[onu-ref='%s']/onu-presence-state"
	LSPathONUOpticalInfo   = "/bbf-xpon-onu-states:xpon-onu-states/onu-state[onu-ref='%s']/optical-info"
	LSPathONUCounters      = "/bbf-xpon-onu-states:xpon-onu-states/onu-state[onu-ref='%s']/counters"

	// Interface state
	LSPathInterfaceState    = "/ietf-interfaces:interfaces-state/interface[name='%s']"
	LSPathInterfaceCounters = "/ietf-interfaces:interfaces-state/interface[name='%s']/statistics"

	// Channel termination statistics
	LSPathCTStats = "/ietf-interfaces:interfaces-state/interface[name='%s']/bbf-xpon:statistics"
	LSPathCTONUs  = "/ietf-interfaces:interfaces-state/interface[name='%s']/bbf-xpon:onus"

	// Hardware state
	LSPathHardwareState = "/ietf-hardware:hardware-state"

	// System state
	LSPathSystemState = "/ietf-system:system-state"
)

// Lightspan gNMI Telemetry Paths (for Altiplano integration)
const (
	// ONU telemetry
	LSGNMIONUState    = "/bbf-xpon-onu-states:xpon-onu-states/onu-state"
	LSGNMIONUOptical  = "/bbf-xpon-onu-states:xpon-onu-states/onu-state/optical-info"
	LSGNMIONUCounters = "/bbf-xpon-onu-states:xpon-onu-states/onu-state/counters"

	// Interface telemetry
	LSGNMIIfCounters = "/ietf-interfaces:interfaces-state/interface/statistics"

	// System telemetry
	LSGNMISystem = "/ietf-system:system-state"
)

// XML Templates for Lightspan OLT operations

// LSONUProvisionXML provisions an ONU using BBF TR-385 model
const LSONUProvisionXML = `
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

// LSVANIProvisionXML creates a Virtual ANI for an ONU
const LSVANIProvisionXML = `
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

// LSOLTVONTANIProvisionXML creates an OLT-side V-ONT-ANI
const LSOLTVONTANIProvisionXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon">
  <olt-v-ont-anis>
    <olt-v-ont-ani>
      <name>%s</name>
      <channel-termination-ref>%s</channel-termination-ref>
      <onu-id>%d</onu-id>
    </olt-v-ont-ani>
  </olt-v-ont-anis>
</xpon>`

// LSChannelTerminationConfigXML configures a PON port
const LSChannelTerminationConfigXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon">
  <channel-terminations>
    <channel-termination>
      <name>%s</name>
      <channel-pair-ref>%s</channel-pair-ref>
      <admin-state>unlocked</admin-state>
      <location>%s</location>
    </channel-termination>
  </channel-terminations>
</xpon>`

// LSChannelPairConfigXML configures a channel pair (wavelength)
const LSChannelPairConfigXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon">
  <channel-pairs>
    <channel-pair>
      <name>%s</name>
      <channel-group-ref>%s</channel-group-ref>
      <channel-partition-ref>%s</channel-partition-ref>
      <channel-pair-type xmlns:bbf-xpon-types="urn:bbf:yang:bbf-xpon-types">bbf-xpon-types:%s</channel-pair-type>
    </channel-pair>
  </channel-pairs>
</xpon>`

// LSSubInterfaceConfigXML creates a subscriber sub-interface
const LSSubInterfaceConfigXML = `
<interfaces xmlns="urn:ietf:params:xml:ns:yang:ietf-interfaces">
  <interface>
    <name>%s</name>
    <type xmlns:bbfift="urn:bbf:yang:bbf-xpon-if-type">bbfift:v-ani</type>
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

// LSForwarderConfigXML creates an L2 forwarder for VLAN bridging
const LSForwarderConfigXML = `
<forwarders xmlns="urn:bbf:yang:bbf-forwarder">
  <forwarder>
    <name>%s</name>
    <ports>
      <port>
        <name>user-port</name>
        <sub-interface>%s</sub-interface>
      </port>
      <port>
        <name>network-port</name>
        <sub-interface>%s</sub-interface>
      </port>
    </ports>
  </forwarder>
</forwarders>`

// LSSuspendONUXML locks an ONU
const LSSuspendONUXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon">
  <onus>
    <onu>
      <name>%s</name>
      <admin-state>locked</admin-state>
    </onu>
  </onus>
</xpon>`

// LSResumeONUXML unlocks an ONU
const LSResumeONUXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon">
  <onus>
    <onu>
      <name>%s</name>
      <admin-state>unlocked</admin-state>
    </onu>
  </onus>
</xpon>`

// LSDeleteONUXML removes an ONU configuration
const LSDeleteONUXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0">
  <onus>
    <onu nc:operation="delete">
      <name>%s</name>
    </onu>
  </onus>
</xpon>`

// LSGetONUStateFilterXML gets ONU operational state
const LSGetONUStateFilterXML = `
<xpon-onu-states xmlns="urn:bbf:yang:bbf-xpon-onu-states">
  <onu-state>
    <onu-ref>%s</onu-ref>
  </onu-state>
</xpon-onu-states>`

// LSGetAllONUsFilterXML gets all ONUs
const LSGetAllONUsFilterXML = `
<xpon-onu-states xmlns="urn:bbf:yang:bbf-xpon-onu-states"/>`

// LSGetCTStatsFilterXML gets channel termination statistics
const LSGetCTStatsFilterXML = `
<interfaces-state xmlns="urn:ietf:params:xml:ns:yang:ietf-interfaces">
  <interface>
    <name>%s</name>
  </interface>
</interfaces-state>`

// LSGetHardwareFilterXML gets hardware inventory
const LSGetHardwareFilterXML = `
<hardware xmlns="urn:ietf:params:xml:ns:yang:ietf-hardware"/>`

// Lightspan Model Information
type LightspanModelInfo struct {
	Model          string
	ServiceSlots   int
	PONPortsPerLT  int
	MaxPONPorts    int
	PONTypes       []string // gpon, xgs-pon, 25g-pon, twdm-pon
	UplinkCapacity string
	FormFactor     string
	Description    string
}

// Supported Lightspan Models
var LightspanModels = map[string]LightspanModelInfo{
	"FX-4": {
		Model:          "Lightspan FX-4",
		ServiceSlots:   4,
		PONPortsPerLT:  16,
		MaxPONPorts:    64,
		PONTypes:       []string{"gpon", "xgs-pon", "25g-pon", "twdm-pon"},
		UplinkCapacity: "480 Gbps",
		FormFactor:     "Modular Chassis",
		Description:    "4-slot modular fiber OLT",
	},
	"FX-8": {
		Model:          "Lightspan FX-8",
		ServiceSlots:   8,
		PONPortsPerLT:  16,
		MaxPONPorts:    128,
		PONTypes:       []string{"gpon", "xgs-pon", "25g-pon", "twdm-pon"},
		UplinkCapacity: "480 Gbps",
		FormFactor:     "Modular Chassis",
		Description:    "8-slot modular fiber OLT",
	},
	"FX-16": {
		Model:          "Lightspan FX-16",
		ServiceSlots:   16,
		PONPortsPerLT:  16,
		MaxPONPorts:    256,
		PONTypes:       []string{"gpon", "xgs-pon", "25g-pon", "twdm-pon"},
		UplinkCapacity: "480 Gbps",
		FormFactor:     "Modular Chassis",
		Description:    "16-slot modular fiber OLT for large deployments",
	},
	"DF-16GW": {
		Model:          "7362 ISAM DF-16GW",
		ServiceSlots:   1,
		PONPortsPerLT:  16,
		MaxPONPorts:    16,
		PONTypes:       []string{"gpon", "xgs-pon", "twdm-pon"},
		UplinkCapacity: "80 Gbps (8x10GbE)",
		FormFactor:     "1RU",
		Description:    "Compact 1RU dense fiber OLT",
	},
	"SF-8M": {
		Model:          "Lightspan SF-8M",
		ServiceSlots:   1,
		PONPortsPerLT:  8,
		MaxPONPorts:    8,
		PONTypes:       []string{"gpon", "xgs-pon", "25g-pon"},
		UplinkCapacity: "40 Gbps",
		FormFactor:     "Sealed/Outdoor",
		Description:    "Sealed outdoor OLT for remote deployment",
	},
	"MF-2": {
		Model:          "Lightspan MF-2",
		ServiceSlots:   2,
		PONPortsPerLT:  8,
		MaxPONPorts:    16,
		PONTypes:       []string{"gpon", "xgs-pon", "25g-pon", "multi-pon"},
		UplinkCapacity: "100 Gbps",
		FormFactor:     "2RU",
		Description:    "Modular compact OLT with TSN support",
	},
}

// PON Types supported by Lightspan
const (
	PONTypeGPON    = "gpon"
	PONTypeXGSPON  = "xgs-pon"
	PONType25GPON  = "25gs-pon"
	PONTypeTWDMPON = "ngpon2"
	PONTypeMulti   = "multi-pon"
)

// Lightspan ONU Presence States (BBF TR-385)
const (
	LSONUPresenceNotPresent          = "onu-not-present"
	LSONUPresenceOnExpectedCT        = "onu-present-and-on-expected-channel-termination"
	LSONUPresenceOnUnexpectedCT      = "onu-present-and-on-unexpected-channel-termination"
	LSONUPresenceEmergencyStop       = "onu-present-and-in-emergency-stop-state"
	LSONUPresenceDyingGasp           = "onu-present-and-in-dying-gasp-state"
	LSONUPresenceNotPresentWithVANI  = "onu-not-present-with-v-ani"
	LSONUPresencePresentWithoutVANI  = "onu-present-without-v-ani"
)

// Admin States
const (
	LSAdminStateLocked       = "locked"
	LSAdminStateUnlocked     = "unlocked"
	LSAdminStateShuttingDown = "shutting-down"
)

// Helper types for parsing Lightspan responses

// LightspanONUState represents parsed ONU state
type LightspanONUState struct {
	Name                string
	SerialNumber        string
	ChannelTermination  string
	ChannelPartition    string
	ONUID               int
	PresenceState       string
	AdminState          string
	OperState           string
	RegistrationID      string
	RxPowerDbm          float64
	TxPowerDbm          float64
	LaserBiasCurrent    float64
	Temperature         float64
	Voltage             float64
	Distance            int
	EqualizationDelay   uint32
	FECCorrectedErrors  uint64
	FECUncorrectedBlocks uint64
	BIPErrors           uint64
}

// LightspanONUCounters represents ONU traffic counters
type LightspanONUCounters struct {
	TotalBytesRx       uint64
	TotalBytesTx       uint64
	TotalFramesRx      uint64
	TotalFramesTx      uint64
	BroadcastFramesRx  uint64
	BroadcastFramesTx  uint64
	MulticastFramesRx  uint64
	MulticastFramesTx  uint64
	DroppedFramesRx    uint64
	DroppedFramesTx    uint64
	GEMFramesRx        uint64
	GEMFramesTx        uint64
}

// LightspanCTStats represents channel termination statistics
type LightspanCTStats struct {
	Name            string
	AdminState      string
	OperState       string
	TotalONUs       int
	ActiveONUs      int
	InactiveONUs    int
	BytesRx         uint64
	BytesTx         uint64
	FramesRx        uint64
	FramesTx        uint64
	GEMFramesRx     uint64
	GEMFramesTx     uint64
	PLOAMsRx        uint64
	PLOAMsTx        uint64
	FECCorrected    uint64
	FECUncorrected  uint64
}

// LightspanHardwareInfo represents OLT hardware information
type LightspanHardwareInfo struct {
	Hostname        string
	Model           string
	SerialNumber    string
	SoftwareVersion string
	BootVersion     string
	UptimeSecs      int64
	Slots           []LightspanSlotInfo
}

// LightspanSlotInfo represents a line card slot
type LightspanSlotInfo struct {
	SlotNumber    int
	CardType      string
	SerialNumber  string
	AdminState    string
	OperState     string
	PONPorts      int
	ActivePONPorts int
}
