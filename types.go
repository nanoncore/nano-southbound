package southbound

// Re-export types from the types sub-package for backwards compatibility
// This allows existing code to continue using southbound.Driver, etc.

import (
	"github.com/nanoncore/nano-southbound/types"
)

// Type aliases for backwards compatibility
type (
	Protocol         = types.Protocol
	Vendor           = types.Vendor
	EquipmentType    = types.EquipmentType
	EquipmentConfig  = types.EquipmentConfig
	Driver           = types.Driver
	CLIExecutor      = types.CLIExecutor
	SNMPExecutor     = types.SNMPExecutor
	SubscriberResult = types.SubscriberResult
	SubscriberStatus = types.SubscriberStatus
	SubscriberStats  = types.SubscriberStats
	EquipmentStatus  = types.EquipmentStatus
)

// Re-export constants
const (
	ProtocolNETCONF = types.ProtocolNETCONF
	ProtocolGNMI    = types.ProtocolGNMI
	ProtocolCLI     = types.ProtocolCLI
	ProtocolSNMP    = types.ProtocolSNMP
	ProtocolREST    = types.ProtocolREST

	VendorNokia     = types.VendorNokia
	VendorHuawei    = types.VendorHuawei
	VendorZTE       = types.VendorZTE
	VendorCisco     = types.VendorCisco
	VendorJuniper   = types.VendorJuniper
	VendorAdtran    = types.VendorAdtran
	VendorCalix     = types.VendorCalix
	VendorDZS       = types.VendorDZS
	VendorFiberHome = types.VendorFiberHome
	VendorEricsson  = types.VendorEricsson
	VendorVSOL      = types.VendorVSOL
	VendorCData     = types.VendorCData
	VendorMock      = types.VendorMock

	EquipmentTypeBNG = types.EquipmentTypeBNG
	EquipmentTypeOLT = types.EquipmentTypeOLT
	EquipmentTypeONU = types.EquipmentTypeONU
)
