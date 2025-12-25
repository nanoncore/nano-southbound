package cisco

// Cisco Routed PON YANG Paths and XML Templates
// Reference: Cisco Routed PON Solution for NCS 540/5500/5700 series
// Note: This is different from Catalyst PON (CGP-OLT) which uses proprietary CLI
//
// Architecture:
// - Pluggable XGS-PON OLT SFP+ modules in IOS-XR routers
// - PON Controller runs as Docker container on IOS-XR
// - NETCONF/YANG via Routed PON Manager + NETCONF Server (Netopeer2)
// - MongoDB database for operational data
// - OMCI protocol to ONTs (handled by PON Controller)
//
// Supported Platforms:
// - NCS 540 Series: N540-24Z8Q2C-SYS, N540-ACC-SYS, etc.
// - NCS 5500 Series: NCS-55A2-MOD-S, NCS-55A1-24Q6H-SS
// - NCS 5700 Series: NCS-57C1-48Q6D, NCS-57C3-MOD

// Routed PON YANG Namespaces
const (
	// Cisco PON Controller namespace
	NSCiscoPONController = "http://cisco.com/ns/yang/Cisco-IOS-XR-um-pon-ctlr-cfg"

	// Tibit PON namespaces (Cisco acquired Tibit for PON tech)
	NSTibitPONController = "http://tibit.com/ns/yang/tibit-pon-controller-db"
	NSTibitNETCONF       = "http://tibit.com/ns/yang/tibit-netconf"

	// BBF Standard namespaces (also used by Routed PON NETCONF Server)
	NSRoutedPONBBFXpon      = "urn:bbf:yang:bbf-xpon"
	NSRoutedPONBBFXponTypes = "urn:bbf:yang:bbf-xpon-types"
	NSRoutedPONBBFONUState  = "urn:bbf:yang:bbf-xpon-onu-states"

	// Standard namespaces
	NSRoutedPONIETFInterfaces = "urn:ietf:params:xml:ns:yang:ietf-interfaces"
)

// Routed PON Controller Configuration Paths
const (
	// PON Controller configuration (on IOS-XR router)
	PathPONController    = "/pon-ctlr"
	PathPONControllerCfg = "/pon-ctlr/cfg-file"
	PathPONControllerVRF = "/pon-ctlr/vrf"
	PathPONControllerTLS = "/pon-ctlr/tls-pem"

	// PON interface configuration
	PathPONInterface    = "/interface-configurations/interface-configuration[active='act'][interface-name='%s']"
	PathPONSubinterface = "/interface-configurations/interface-configuration[active='act'][interface-name='%s.4090']"
)

// Routed PON Manager/NETCONF Server Paths (BBF-compliant)
const (
	// ONU management via NETCONF Server
	RPPathONUs        = "/bbf-xpon:xpon/onus"
	RPPathONU         = "/bbf-xpon:xpon/onus/onu[name='%s']"
	RPPathONUBySerial = "/bbf-xpon:xpon/onus/onu[serial-number='%s']"

	// V-ANI paths
	RPPathVANIs = "/bbf-xpon:xpon/v-anis"
	RPPathVANI  = "/bbf-xpon:xpon/v-anis/v-ani[name='%s']"

	// Channel termination paths
	RPPathCTs = "/bbf-xpon:xpon/channel-terminations"
	RPPathCT  = "/bbf-xpon:xpon/channel-terminations/channel-termination[name='%s']"

	// Interface paths
	RPPathInterfaces = "/ietf-interfaces:interfaces"
	RPPathInterface  = "/ietf-interfaces:interfaces/interface[name='%s']"
)

// Routed PON State Paths
const (
	// ONU state (via NETCONF Server)
	RPPathONUStates = "/bbf-xpon-onu-states:xpon-onu-states"
	RPPathONUState  = "/bbf-xpon-onu-states:xpon-onu-states/onu-state[onu-ref='%s']"

	// Interface statistics
	RPPathInterfaceState = "/ietf-interfaces:interfaces-state/interface[name='%s']"
	RPPathInterfaceStats = "/ietf-interfaces:interfaces-state/interface[name='%s']/statistics"
)

// gNMI Telemetry Paths for Routed PON
const (
	// PON interface telemetry (on IOS-XR)
	RPGNMIPONInterface = "/Cisco-IOS-XR-ifmgr-oper:interface-properties/data-nodes/data-node/system-view/interfaces/interface"

	// ONU telemetry (via NETCONF Server subscription)
	RPGNMIONUState   = "/bbf-xpon-onu-states:xpon-onu-states/onu-state"
	RPGNMIONUOptical = "/bbf-xpon-onu-states:xpon-onu-states/onu-state/optical-info"
)

// XML Templates for Routed PON

// RPPONControllerConfigXML configures the PON Controller on IOS-XR
const RPPONControllerConfigXML = `
<pon-ctlr xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-um-pon-ctlr-cfg">
  <cfg-file>harddisk:/%s</cfg-file>
  <vrf>%s</vrf>
  <tls-pem>%s</tls-pem>
</pon-ctlr>`

// RPPONSubinterfaceConfigXML creates the control subinterface for PON
// Note: Subinterface 4090 is required for control packets between PON Controller and OLT
const RPPONSubinterfaceConfigXML = `
<interface-configuration xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ifmgr-cfg">
  <active>act</active>
  <interface-name>%s.4090</interface-name>
  <description>PON Control Plane</description>
  <vlan-sub-configuration xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-l2-eth-infra-cfg">
    <vlan-identifier>
      <vlan-type>vlan-type-dot1q</vlan-type>
      <first-tag>4090</first-tag>
    </vlan-identifier>
  </vlan-sub-configuration>
  <ipv4-network xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-ipv4-io-cfg">
    <addresses>
      <primary>
        <address>%s</address>
        <netmask>%s</netmask>
      </primary>
    </addresses>
  </ipv4-network>
</interface-configuration>`

// RPONUProvisionXML provisions an ONU via Routed PON NETCONF Server
const RPONUProvisionXML = `
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

// RPVANIProvisionXML creates a V-ANI for an ONU
const RPVANIProvisionXML = `
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

// RPSuspendONUXML locks an ONU
const RPSuspendONUXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon">
  <onus>
    <onu>
      <name>%s</name>
      <admin-state>locked</admin-state>
    </onu>
  </onus>
</xpon>`

// RPResumeONUXML unlocks an ONU
const RPResumeONUXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon">
  <onus>
    <onu>
      <name>%s</name>
      <admin-state>unlocked</admin-state>
    </onu>
  </onus>
</xpon>`

// RPDeleteONUXML removes an ONU
const RPDeleteONUXML = `
<xpon xmlns="urn:bbf:yang:bbf-xpon" xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0">
  <onus>
    <onu nc:operation="delete">
      <name>%s</name>
    </onu>
  </onus>
</xpon>`

// RPGetONUStateFilterXML gets ONU operational state
const RPGetONUStateFilterXML = `
<xpon-onu-states xmlns="urn:bbf:yang:bbf-xpon-onu-states">
  <onu-state>
    <onu-ref>%s</onu-ref>
  </onu-state>
</xpon-onu-states>`

// RPGetAllONUsFilterXML gets all ONUs
const RPGetAllONUsFilterXML = `
<xpon-onu-states xmlns="urn:bbf:yang:bbf-xpon-onu-states"/>`

// Supported Cisco Routed PON Platforms
type RoutedPONPlatform struct {
	Model       string
	Series      string
	MaxPONPorts int // Number of 10GbE ports that can accept PON SFP+
	PONType     string
	Description string
}

// CiscoRoutedPONPlatforms lists supported platforms
var CiscoRoutedPONPlatforms = map[string]RoutedPONPlatform{
	"N540-24Z8Q2C-SYS": {
		Model:       "N540-24Z8Q2C-SYS",
		Series:      "NCS 540",
		MaxPONPorts: 24,
		PONType:     "xgs-pon",
		Description: "NCS 540 with 24x10GbE SFP+ ports for PON",
	},
	"N540-ACC-SYS": {
		Model:       "N540-ACC-SYS",
		Series:      "NCS 540",
		MaxPONPorts: 48,
		PONType:     "xgs-pon",
		Description: "NCS 540 Access with 48 SFP+ ports",
	},
	"N540-24Q8L2DD-SYS": {
		Model:       "N540-24Q8L2DD-SYS",
		Series:      "NCS 540",
		MaxPONPorts: 24,
		PONType:     "xgs-pon",
		Description: "NCS 540 large density",
	},
	"NCS-55A2-MOD-S": {
		Model:       "NCS-55A2-MOD-S",
		Series:      "NCS 5500",
		MaxPONPorts: 48,
		PONType:     "xgs-pon",
		Description: "NCS 5500 modular system",
	},
	"NCS-55A1-24Q6H-SS": {
		Model:       "NCS-55A1-24Q6H-SS",
		Series:      "NCS 5500",
		MaxPONPorts: 24,
		PONType:     "xgs-pon",
		Description: "NCS 5500 fixed form factor",
	},
	"NCS-57C1-48Q6D": {
		Model:       "NCS-57C1-48Q6D",
		Series:      "NCS 5700",
		MaxPONPorts: 48,
		PONType:     "xgs-pon",
		Description: "NCS 5700 with 48x10/25GbE",
	},
	"NCS-57C3-MOD": {
		Model:       "NCS-57C3-MOD",
		Series:      "NCS 5700",
		MaxPONPorts: 72,
		PONType:     "xgs-pon",
		Description: "NCS 5700 modular chassis",
	},
}

// Cisco PON OLT SFP+ Module Information
type PONSFPModule struct {
	PartNumber   string
	Type         string // EML or DML
	PONStandard  string
	MaxDistance  int // km
	SplitRatio   string
	DownstreamWL int // nm
	UpstreamWL   int // nm
	DataRate     string
}

// CiscoPONSFPModules lists the PON SFP+ modules
var CiscoPONSFPModules = map[string]PONSFPModule{
	"PON-SFP-10G-EML": {
		PartNumber:   "PON-SFP-10G-EML",
		Type:         "EML",
		PONStandard:  "XGS-PON",
		MaxDistance:  20,
		SplitRatio:   "1:64",
		DownstreamWL: 1577,
		UpstreamWL:   1270,
		DataRate:     "10G symmetric",
	},
	"PON-SFP-10G-DML": {
		PartNumber:   "PON-SFP-10G-DML",
		Type:         "DML",
		PONStandard:  "XGS-PON",
		MaxDistance:  20,
		SplitRatio:   "1:64",
		DownstreamWL: 1577,
		UpstreamWL:   1270,
		DataRate:     "10G symmetric",
	},
}

// Cisco PON ONT Products
type CiscoPONONT struct {
	Model       string
	Type        string // Indoor, Outdoor
	Interfaces  string
	PONStandard string
	Description string
}

// CiscoPONONTs lists supported ONT models
var CiscoPONONTs = map[string]CiscoPONONT{
	"ENC-10G-ONT-10": {
		Model:       "ENC-10G-ONT-10",
		Type:        "Indoor Desktop",
		Interfaces:  "1x 10G RJ-45 (supports 1G/2.5G/5G/10G)",
		PONStandard: "XGS-PON",
		Description: "Desktop ONT with 10G Ethernet",
	},
	"ENC-10G-ONT-14A": {
		Model:       "ENC-10G-ONT-14A",
		Type:        "Indoor Desktop",
		Interfaces:  "Ethernet + ATA (voice)",
		PONStandard: "XGS-PON",
		Description: "Residential/SOHO ONT with voice",
	},
	"ENC-10G-ONT-01PR": {
		Model:       "ENC-10G-ONT-01PR",
		Type:        "Outdoor",
		Interfaces:  "10G Ethernet",
		PONStandard: "XGS-PON",
		Description: "Temperature-hardened outdoor ONT",
	},
}

// Helper types for parsing Routed PON responses

// RoutedPONONUState represents ONU state from Routed PON
type RoutedPONONUState struct {
	Name              string
	SerialNumber      string
	PONPort           string
	ONUID             int
	PresenceState     string
	AdminState        string
	OperState         string
	RxPowerDbm        float64
	TxPowerDbm        float64
	Temperature       float64
	Voltage           float64
	Distance          int
	EqualizationDelay uint32
}

// RoutedPONControllerStatus represents PON Controller status
type RoutedPONControllerStatus struct {
	Running       bool
	ContainerID   string
	Version       string
	ConnectedOLTs int
	TotalONUs     int
	ActiveONUs    int
	MongoDBStatus string
}

// RoutedPONPortStats represents PON port statistics
type RoutedPONPortStats struct {
	PortName   string
	AdminState string
	OperState  string
	TotalONUs  int
	ActiveONUs int
	BytesRx    uint64
	BytesTx    uint64
	FramesRx   uint64
	FramesTx   uint64
}
