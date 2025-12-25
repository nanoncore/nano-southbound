package southbound

import (
	"fmt"

	"github.com/nanoncore/nano-southbound/drivers/cli"
	"github.com/nanoncore/nano-southbound/drivers/gnmi"
	"github.com/nanoncore/nano-southbound/drivers/mock"
	"github.com/nanoncore/nano-southbound/drivers/netconf"
	"github.com/nanoncore/nano-southbound/drivers/snmp"
	"github.com/nanoncore/nano-southbound/vendors/adtran"
	"github.com/nanoncore/nano-southbound/vendors/calix"
	"github.com/nanoncore/nano-southbound/vendors/cdata"
	"github.com/nanoncore/nano-southbound/vendors/cisco"
	"github.com/nanoncore/nano-southbound/vendors/dzs"
	"github.com/nanoncore/nano-southbound/vendors/ericsson"
	"github.com/nanoncore/nano-southbound/vendors/fiberhome"
	"github.com/nanoncore/nano-southbound/vendors/huawei"
	"github.com/nanoncore/nano-southbound/vendors/juniper"
	"github.com/nanoncore/nano-southbound/vendors/nokia"
	"github.com/nanoncore/nano-southbound/vendors/vsol"
	"github.com/nanoncore/nano-southbound/vendors/zte"
)

// CapabilityMatrix defines what each vendor supports
var CapabilityMatrix = map[Vendor]VendorCapabilities{
	VendorNokia: {
		PrimaryProtocol: ProtocolNETCONF,
		SupportedProtocols: []Protocol{
			ProtocolNETCONF,
			ProtocolGNMI,
			ProtocolCLI,
		},
		ConfigMethod:      ProtocolNETCONF,
		TelemetryMethod:   ProtocolGNMI,
		SupportsStreaming: true,
	},
	VendorHuawei: {
		PrimaryProtocol: ProtocolCLI,
		SupportedProtocols: []Protocol{
			ProtocolCLI,
			ProtocolSNMP,
			ProtocolNETCONF, // newer devices
		},
		ConfigMethod:      ProtocolCLI,
		TelemetryMethod:   ProtocolSNMP,
		SupportsStreaming: false,
	},
	VendorZTE: {
		PrimaryProtocol: ProtocolCLI,
		SupportedProtocols: []Protocol{
			ProtocolCLI,
			ProtocolSNMP,
		},
		ConfigMethod:      ProtocolCLI,
		TelemetryMethod:   ProtocolSNMP,
		SupportsStreaming: false,
	},
	VendorCisco: {
		PrimaryProtocol: ProtocolNETCONF,
		SupportedProtocols: []Protocol{
			ProtocolNETCONF,
			ProtocolGNMI,
			ProtocolREST,
			ProtocolCLI,
		},
		ConfigMethod:      ProtocolNETCONF,
		TelemetryMethod:   ProtocolGNMI,
		SupportsStreaming: true,
	},
	VendorJuniper: {
		PrimaryProtocol: ProtocolNETCONF,
		SupportedProtocols: []Protocol{
			ProtocolNETCONF,
			ProtocolGNMI,
			ProtocolCLI,
		},
		ConfigMethod:      ProtocolNETCONF,
		TelemetryMethod:   ProtocolGNMI,
		SupportsStreaming: true,
	},
	VendorAdtran: {
		PrimaryProtocol: ProtocolNETCONF,
		SupportedProtocols: []Protocol{
			ProtocolNETCONF,
			ProtocolREST,
		},
		ConfigMethod:      ProtocolNETCONF,
		TelemetryMethod:   ProtocolNETCONF,
		SupportsStreaming: false,
	},
	VendorCalix: {
		PrimaryProtocol: ProtocolNETCONF,
		SupportedProtocols: []Protocol{
			ProtocolNETCONF,
			ProtocolREST,
			ProtocolCLI,
		},
		ConfigMethod:      ProtocolNETCONF,
		TelemetryMethod:   ProtocolNETCONF,
		SupportsStreaming: false,
	},
	VendorDZS: {
		PrimaryProtocol: ProtocolCLI,
		SupportedProtocols: []Protocol{
			ProtocolCLI,
			ProtocolSNMP,
		},
		ConfigMethod:      ProtocolCLI,
		TelemetryMethod:   ProtocolSNMP,
		SupportsStreaming: false,
	},
	VendorFiberHome: {
		PrimaryProtocol: ProtocolSNMP,
		SupportedProtocols: []Protocol{
			ProtocolSNMP,
			ProtocolCLI,
		},
		ConfigMethod:      ProtocolCLI,
		TelemetryMethod:   ProtocolSNMP,
		SupportsStreaming: false,
	},
	VendorEricsson: {
		PrimaryProtocol: ProtocolNETCONF,
		SupportedProtocols: []Protocol{
			ProtocolNETCONF,
			ProtocolCLI,
		},
		ConfigMethod:      ProtocolNETCONF,
		TelemetryMethod:   ProtocolNETCONF,
		SupportsStreaming: false,
	},
	VendorVSOL: {
		PrimaryProtocol: ProtocolCLI,
		SupportedProtocols: []Protocol{
			ProtocolCLI,
			ProtocolSNMP,
			ProtocolREST, // via EMS
		},
		ConfigMethod:      ProtocolCLI,
		TelemetryMethod:   ProtocolSNMP,
		SupportsStreaming: false,
	},
	VendorCData: {
		PrimaryProtocol: ProtocolCLI,
		SupportedProtocols: []Protocol{
			ProtocolCLI,
			ProtocolSNMP,
		},
		ConfigMethod:      ProtocolCLI,
		TelemetryMethod:   ProtocolSNMP,
		SupportsStreaming: false,
	},
	VendorMock: {
		PrimaryProtocol: ProtocolCLI,
		SupportedProtocols: []Protocol{
			ProtocolCLI,
			ProtocolSNMP,
			ProtocolNETCONF,
			ProtocolGNMI,
		},
		ConfigMethod:      ProtocolCLI,
		TelemetryMethod:   ProtocolCLI,
		SupportsStreaming: false,
	},
}

// VendorCapabilities defines what protocols and features a vendor supports
type VendorCapabilities struct {
	PrimaryProtocol    Protocol
	SupportedProtocols []Protocol
	ConfigMethod       Protocol
	TelemetryMethod    Protocol
	SupportsStreaming  bool
}

// NewDriver creates a new southbound driver based on vendor and protocol
func NewDriver(vendor Vendor, protocol Protocol, config *EquipmentConfig) (Driver, error) {
	// Validate vendor capabilities
	caps, ok := CapabilityMatrix[vendor]
	if !ok {
		return nil, fmt.Errorf("unsupported vendor: %s", vendor)
	}

	// If protocol not specified, use primary
	if protocol == "" {
		protocol = caps.PrimaryProtocol
	}

	// Validate protocol is supported
	supported := false
	for _, p := range caps.SupportedProtocols {
		if p == protocol {
			supported = true
			break
		}
	}
	if !supported {
		return nil, fmt.Errorf("vendor %s does not support protocol %s", vendor, protocol)
	}

	// Create protocol driver
	var baseDriver Driver
	var err error

	// Mock vendor uses mock driver regardless of protocol
	if vendor == VendorMock {
		return mock.NewDriver(config)
	}

	switch protocol {
	case ProtocolNETCONF:
		baseDriver, err = netconf.NewDriver(config)
	case ProtocolGNMI:
		baseDriver, err = gnmi.NewDriver(config)
	case ProtocolCLI:
		baseDriver, err = cli.NewDriver(config)
	case ProtocolSNMP:
		baseDriver, err = snmp.NewDriver(config)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create %s driver: %w", protocol, err)
	}

	// Wrap with vendor-specific adapter
	switch vendor {
	case VendorNokia:
		return nokia.NewAdapter(baseDriver, config), nil
	case VendorHuawei:
		return huawei.NewAdapter(baseDriver, config), nil
	case VendorZTE:
		return zte.NewAdapter(baseDriver, config), nil
	case VendorCisco:
		return cisco.NewAdapter(baseDriver, config), nil
	case VendorJuniper:
		return juniper.NewAdapter(baseDriver, config), nil
	case VendorAdtran:
		return adtran.NewAdapter(baseDriver, config), nil
	case VendorCalix:
		return calix.NewAdapter(baseDriver, config), nil
	case VendorDZS:
		return dzs.NewAdapter(baseDriver, config), nil
	case VendorFiberHome:
		return fiberhome.NewAdapter(baseDriver, config), nil
	case VendorEricsson:
		return ericsson.NewAdapter(baseDriver, config), nil
	case VendorVSOL:
		return vsol.NewAdapter(baseDriver, config), nil
	case VendorCData:
		return cdata.NewAdapter(baseDriver, config), nil
	case VendorMock:
		// Mock driver doesn't need a vendor adapter - it handles everything
		return baseDriver, nil
	default:
		return nil, fmt.Errorf("vendor adapter not implemented: %s", vendor)
	}
}

// GetSupportedVendors returns a list of all supported vendors
func GetSupportedVendors() []Vendor {
	vendors := make([]Vendor, 0, len(CapabilityMatrix))
	for v := range CapabilityMatrix {
		vendors = append(vendors, v)
	}
	return vendors
}

// GetVendorCapabilities returns the capabilities for a vendor
func GetVendorCapabilities(vendor Vendor) (VendorCapabilities, bool) {
	caps, ok := CapabilityMatrix[vendor]
	return caps, ok
}
