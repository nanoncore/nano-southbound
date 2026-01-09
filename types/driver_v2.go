package types

import (
	"context"
	"time"
)

// DriverV2 extends the base Driver interface with OLT-level operations,
// enhanced diagnostics, and bulk provisioning capabilities.
//
// Adapters should implement this interface to provide full functionality.
// The base Driver interface remains supported for backwards compatibility.
type DriverV2 interface {
	Driver // Embed base interface

	// === ONU Discovery ===

	// DiscoverONUs finds unprovisioned ONUs on the OLT.
	// If ponPorts is empty, discovers on all PON ports.
	// Returns ONUs that have registered but are not yet provisioned.
	DiscoverONUs(ctx context.Context, ponPorts []string) ([]ONUDiscovery, error)

	// GetONUList returns all provisioned ONUs matching the filter.
	// If filter is nil, returns all ONUs.
	GetONUList(ctx context.Context, filter *ONUFilter) ([]ONUInfo, error)

	// GetONUBySerial finds a specific ONU by serial number.
	// Returns nil if not found.
	GetONUBySerial(ctx context.Context, serial string) (*ONUInfo, error)

	// === Optical Diagnostics ===

	// GetPONPower returns optical power readings for a PON port.
	// This is the aggregate power at the OLT splitter.
	GetPONPower(ctx context.Context, ponPort string) (*PONPowerReading, error)

	// GetONUPower returns optical power readings for a specific ONU.
	// Includes both ONU Tx/Rx and what the OLT sees from this ONU.
	GetONUPower(ctx context.Context, ponPort string, onuID int) (*ONUPowerReading, error)

	// GetONUDistance returns estimated fiber distance to ONU in meters.
	// Returns -1 if distance cannot be determined.
	GetONUDistance(ctx context.Context, ponPort string, onuID int) (int, error)

	// === ONU Operations ===

	// RestartONU triggers a reboot of the specified ONU.
	// Useful for troubleshooting and clearing stuck states.
	RestartONU(ctx context.Context, ponPort string, onuID int) error

	// ApplyProfile applies a bandwidth/service profile to an ONU.
	// This is a faster path than full UpdateSubscriber for profile changes.
	ApplyProfile(ctx context.Context, ponPort string, onuID int, profile *ONUProfile) error

	// === Bulk Operations ===

	// BulkProvision provisions multiple ONUs in a single session.
	// More efficient than individual CreateSubscriber calls.
	// Returns results for each operation (some may succeed while others fail).
	BulkProvision(ctx context.Context, operations []BulkProvisionOp) (*BulkResult, error)

	// === Comprehensive Diagnostics ===

	// RunDiagnostics performs comprehensive diagnostics on an ONU.
	// Returns power readings, counters, configuration, and any alarms.
	RunDiagnostics(ctx context.Context, ponPort string, onuID int) (*ONUDiagnostics, error)

	// GetAlarms returns active alarms from the OLT.
	// Includes ONU alarms, port alarms, and system alarms.
	GetAlarms(ctx context.Context) ([]OLTAlarm, error)

	// === OLT Status ===

	// GetOLTStatus returns comprehensive OLT status including
	// PON port status, resource utilization, and uptime.
	GetOLTStatus(ctx context.Context) (*OLTStatus, error)
}

// ONUDiscovery represents an unprovisioned ONU found during discovery.
type ONUDiscovery struct {
	// PONPort is the PON port where the ONU was discovered (e.g., "0/1")
	PONPort string `json:"pon_port"`

	// Serial is the ONU serial number (e.g., "VSOL12345678")
	Serial string `json:"serial"`

	// MAC is the ONU MAC address (optional, not all vendors provide this)
	MAC string `json:"mac,omitempty"`

	// Model is the ONU model (optional)
	Model string `json:"model,omitempty"`

	// Vendor is the ONU vendor (optional, may differ from OLT vendor)
	Vendor string `json:"vendor,omitempty"`

	// DistanceM is the estimated fiber distance in meters
	DistanceM int `json:"distance_m,omitempty"`

	// RxPowerDBm is the optical receive power in dBm
	RxPowerDBm float64 `json:"rx_power_dbm,omitempty"`

	// DiscoveredAt is when the ONU was discovered
	DiscoveredAt time.Time `json:"discovered_at"`

	// Metadata contains vendor-specific discovery data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ONUFilter specifies criteria for filtering ONUs in GetONUList.
type ONUFilter struct {
	// PONPort filters to a specific PON port
	PONPort string `json:"pon_port,omitempty"`

	// Status filters by ONU status (online, offline, all)
	Status string `json:"status,omitempty"`

	// Profile filters by line/service profile name
	Profile string `json:"profile,omitempty"`

	// Serial filters by serial number (partial match supported)
	Serial string `json:"serial,omitempty"`

	// VLAN filters by VLAN ID
	VLAN int `json:"vlan,omitempty"`
}

// ONUInfo represents a provisioned ONU.
type ONUInfo struct {
	// PONPort is the PON port (e.g., "0/1")
	PONPort string `json:"pon_port"`

	// ONUID is the ONU ID on the PON port
	ONUID int `json:"onu_id"`

	// Serial is the ONU serial number
	Serial string `json:"serial"`

	// MAC is the ONU MAC address
	MAC string `json:"mac,omitempty"`

	// Model is the ONU model
	Model string `json:"model,omitempty"`

	// AdminState is the administrative state (enabled, disabled)
	AdminState string `json:"admin_state"`

	// OperState is the operational state (online, offline, los, etc.)
	OperState string `json:"oper_state"`

	// IsOnline indicates if the ONU is currently online
	IsOnline bool `json:"is_online"`

	// RxPowerDBm is the ONU receive power
	RxPowerDBm float64 `json:"rx_power_dbm,omitempty"`

	// TxPowerDBm is the ONU transmit power
	TxPowerDBm float64 `json:"tx_power_dbm,omitempty"`

	// DistanceM is the fiber distance in meters
	DistanceM int `json:"distance_m,omitempty"`

	// Vendor is the ONU vendor (detected from serial prefix, e.g., "FiberHome", "Huawei")
	Vendor string `json:"vendor,omitempty"`

	// Temperature is the ONU temperature in Celsius
	Temperature float64 `json:"temperature_c,omitempty"`

	// Voltage is the ONU power supply voltage in Volts
	Voltage float64 `json:"voltage_v,omitempty"`

	// BiasCurrent is the laser bias current in mA
	BiasCurrent float64 `json:"bias_ma,omitempty"`

	// BytesUp is the total bytes transmitted upstream
	BytesUp uint64 `json:"bytes_up,omitempty"`

	// BytesDown is the total bytes received downstream
	BytesDown uint64 `json:"bytes_down,omitempty"`

	// PacketsUp is the total packets transmitted upstream
	PacketsUp uint64 `json:"packets_up,omitempty"`

	// PacketsDown is the total packets received downstream
	PacketsDown uint64 `json:"packets_down,omitempty"`

	// InputRateBps is the current input rate in bytes per second
	InputRateBps uint64 `json:"input_rate_bps,omitempty"`

	// OutputRateBps is the current output rate in bytes per second
	OutputRateBps uint64 `json:"output_rate_bps,omitempty"`

	// LineProfile is the assigned line profile
	LineProfile string `json:"line_profile,omitempty"`

	// ServiceProfile is the assigned service profile
	ServiceProfile string `json:"service_profile,omitempty"`

	// VLAN is the configured VLAN
	VLAN int `json:"vlan,omitempty"`

	// BandwidthUp is the upstream bandwidth limit in Mbps
	BandwidthUp int `json:"bandwidth_up_mbps,omitempty"`

	// BandwidthDown is the downstream bandwidth limit in Mbps
	BandwidthDown int `json:"bandwidth_down_mbps,omitempty"`

	// UptimeSeconds is the ONU session uptime
	UptimeSeconds int64 `json:"uptime_seconds,omitempty"`

	// LastOnline is when the ONU was last online
	LastOnline time.Time `json:"last_online,omitempty"`

	// ProvisionedAt is when the ONU was provisioned
	ProvisionedAt time.Time `json:"provisioned_at,omitempty"`

	// Metadata contains vendor-specific data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// PONPowerReading represents optical power readings for a PON port.
type PONPowerReading struct {
	// PONPort is the PON port identifier
	PONPort string `json:"pon_port"`

	// TxPowerDBm is the OLT transmit power
	TxPowerDBm float64 `json:"tx_power_dbm"`

	// RxPowerDBm is the aggregate receive power (if available)
	RxPowerDBm float64 `json:"rx_power_dbm,omitempty"`

	// Temperature is the SFP module temperature in Celsius
	Temperature float64 `json:"temperature_celsius,omitempty"`

	// Timestamp is when the reading was taken
	Timestamp time.Time `json:"timestamp"`

	// Metadata contains vendor-specific data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ONUPowerReading represents optical power readings for a specific ONU.
type ONUPowerReading struct {
	// PONPort is the PON port
	PONPort string `json:"pon_port"`

	// ONUID is the ONU ID
	ONUID int `json:"onu_id"`

	// Serial is the ONU serial (for correlation)
	Serial string `json:"serial,omitempty"`

	// TxPowerDBm is the ONU transmit power (what ONU sends)
	TxPowerDBm float64 `json:"tx_power_dbm"`

	// RxPowerDBm is the ONU receive power (what ONU sees from OLT)
	RxPowerDBm float64 `json:"rx_power_dbm"`

	// OLTRxDBm is what the OLT receives from this ONU
	OLTRxDBm float64 `json:"olt_rx_dbm"`

	// DistanceM is the fiber distance in meters
	DistanceM int `json:"distance_m,omitempty"`

	// Thresholds for alerting
	TxHighThreshold float64 `json:"tx_high_threshold,omitempty"`
	TxLowThreshold  float64 `json:"tx_low_threshold,omitempty"`
	RxHighThreshold float64 `json:"rx_high_threshold,omitempty"`
	RxLowThreshold  float64 `json:"rx_low_threshold,omitempty"`

	// IsWithinSpec indicates if all readings are within acceptable ranges
	IsWithinSpec bool `json:"is_within_spec"`

	// Timestamp is when the reading was taken
	Timestamp time.Time `json:"timestamp"`

	// Metadata contains vendor-specific data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ONUProfile defines bandwidth and service configuration for an ONU.
type ONUProfile struct {
	// LineProfile is the line profile name
	LineProfile string `json:"line_profile"`

	// ServiceProfile is the service profile name
	ServiceProfile string `json:"service_profile"`

	// BandwidthUp is upstream bandwidth in kbps
	BandwidthUp int `json:"bandwidth_up_kbps"`

	// BandwidthDown is downstream bandwidth in kbps
	BandwidthDown int `json:"bandwidth_down_kbps"`

	// VLAN is the service VLAN
	VLAN int `json:"vlan"`

	// Priority is the traffic priority (0-7)
	Priority int `json:"priority,omitempty"`

	// Metadata contains vendor-specific profile data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// BulkProvisionOp represents a single operation in bulk provisioning.
type BulkProvisionOp struct {
	// Serial is the ONU serial number
	Serial string `json:"serial"`

	// PONPort is the target PON port (optional, auto-detect if empty)
	PONPort string `json:"pon_port,omitempty"`

	// ONUID is the target ONU ID (optional, auto-assign if 0)
	ONUID int `json:"onu_id,omitempty"`

	// Profile contains the bandwidth/service configuration
	Profile *ONUProfile `json:"profile"`

	// Metadata contains operation-specific data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// BulkResult contains the results of a bulk provisioning operation.
type BulkResult struct {
	// Succeeded is the count of successful operations
	Succeeded int `json:"succeeded"`

	// Failed is the count of failed operations
	Failed int `json:"failed"`

	// Results contains the result for each operation
	Results []BulkOpResult `json:"results"`
}

// BulkOpResult contains the result of a single bulk operation.
type BulkOpResult struct {
	// Serial is the ONU serial number
	Serial string `json:"serial"`

	// Success indicates if the operation succeeded
	Success bool `json:"success"`

	// Error contains the error message if failed
	Error string `json:"error,omitempty"`

	// ErrorCode is the normalized error code
	ErrorCode string `json:"error_code,omitempty"`

	// ONUID is the assigned ONU ID (if successful)
	ONUID int `json:"onu_id,omitempty"`

	// PONPort is the PON port used
	PONPort string `json:"pon_port,omitempty"`

	// Metadata contains vendor-specific result data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ONUDiagnostics contains comprehensive diagnostic information for an ONU.
type ONUDiagnostics struct {
	// Serial is the ONU serial number
	Serial string `json:"serial"`

	// PONPort is the PON port
	PONPort string `json:"pon_port"`

	// ONUID is the ONU ID
	ONUID int `json:"onu_id"`

	// Power contains optical power readings
	Power *ONUPowerReading `json:"power,omitempty"`

	// AdminState is the administrative state
	AdminState string `json:"admin_state"`

	// OperState is the operational state
	OperState string `json:"oper_state"`

	// AuthState is the authentication state (optional)
	AuthState string `json:"auth_state,omitempty"`

	// Traffic counters
	BytesUp   uint64 `json:"bytes_up"`
	BytesDown uint64 `json:"bytes_down"`
	Errors    uint64 `json:"errors"`
	Drops     uint64 `json:"drops"`

	// Configuration
	LineProfile    string `json:"line_profile"`
	ServiceProfile string `json:"service_profile"`
	VLAN           int    `json:"vlan"`
	BandwidthUp    int    `json:"bandwidth_up_kbps"`
	BandwidthDown  int    `json:"bandwidth_down_kbps"`

	// Alarms contains active alarms for this ONU
	Alarms []string `json:"alarms,omitempty"`

	// VendorData contains vendor-specific diagnostic data
	VendorData map[string]interface{} `json:"vendor_data,omitempty"`

	// Timestamp is when diagnostics were collected
	Timestamp time.Time `json:"timestamp"`
}

// OLTAlarm represents an alarm from the OLT.
type OLTAlarm struct {
	// ID is the alarm identifier
	ID string `json:"id"`

	// Severity is the alarm severity (critical, major, minor, warning)
	Severity string `json:"severity"`

	// Type is the alarm type (los, power, config, etc.)
	Type string `json:"type"`

	// Source is the alarm source (port, onu, system)
	Source string `json:"source"`

	// SourceID is the identifier of the source (e.g., PON port, ONU serial)
	SourceID string `json:"source_id,omitempty"`

	// Message is the alarm description
	Message string `json:"message"`

	// RaisedAt is when the alarm was raised
	RaisedAt time.Time `json:"raised_at"`

	// ClearedAt is when the alarm was cleared (nil if still active)
	ClearedAt *time.Time `json:"cleared_at,omitempty"`

	// Metadata contains vendor-specific alarm data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// OLTStatus represents comprehensive OLT status.
type OLTStatus struct {
	// OLTID is the OLT identifier
	OLTID string `json:"olt_id"`

	// Vendor is the OLT vendor
	Vendor string `json:"vendor"`

	// Model is the OLT model
	Model string `json:"model"`

	// Firmware is the firmware version
	Firmware string `json:"firmware"`

	// SerialNumber is the OLT serial number
	SerialNumber string `json:"serial_number"`

	// IsReachable indicates if the OLT is reachable
	IsReachable bool `json:"is_reachable"`

	// IsHealthy indicates if the OLT is functioning properly
	IsHealthy bool `json:"is_healthy"`

	// UptimeSeconds is the OLT uptime
	UptimeSeconds int64 `json:"uptime_seconds"`

	// CPUPercent is CPU utilization
	CPUPercent float64 `json:"cpu_percent"`

	// MemoryPercent is memory utilization
	MemoryPercent float64 `json:"memory_percent"`

	// Temperature is the system temperature in Celsius
	Temperature float64 `json:"temperature_celsius,omitempty"`

	// PONPorts contains status for each PON port
	PONPorts []PONPortStatus `json:"pon_ports"`

	// ActiveONUs is the count of online ONUs
	ActiveONUs int `json:"active_onus"`

	// TotalONUs is the count of all provisioned ONUs
	TotalONUs int `json:"total_onus"`

	// LastPoll is when status was last updated
	LastPoll time.Time `json:"last_poll"`

	// Metadata contains vendor-specific status data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// PONPortStatus represents status of a single PON port.
type PONPortStatus struct {
	// Port is the port identifier (e.g., "0/1")
	Port string `json:"port"`

	// AdminState is the administrative state
	AdminState string `json:"admin_state"`

	// OperState is the operational state
	OperState string `json:"oper_state"`

	// ONUCount is the number of ONUs on this port
	ONUCount int `json:"onu_count"`

	// MaxONUs is the maximum ONUs supported on this port
	MaxONUs int `json:"max_onus"`

	// RxPowerDBm is the port receive power
	RxPowerDBm float64 `json:"rx_power_dbm,omitempty"`

	// TxPowerDBm is the port transmit power
	TxPowerDBm float64 `json:"tx_power_dbm,omitempty"`

	// Metadata contains vendor-specific port data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// HumanError wraps vendor errors with human-readable context.
// Adapters should return this type for better error messages.
type HumanError struct {
	// Code is the normalized error code (e.g., "ONU_EXISTS", "TIMEOUT")
	Code string `json:"code"`

	// Message is the human-readable error message
	Message string `json:"message"`

	// Action is the suggested remediation action
	Action string `json:"action,omitempty"`

	// Vendor is the vendor that produced the error
	Vendor string `json:"vendor"`

	// Raw is the raw error output from the device
	Raw string `json:"raw,omitempty"`

	// Recoverable indicates if this error should be retried
	Recoverable bool `json:"recoverable"`
}

// Error implements the error interface.
func (e *HumanError) Error() string {
	if e.Action != "" {
		return "[" + e.Vendor + "] " + e.Code + ": " + e.Message + " (Suggestion: " + e.Action + ")"
	}
	return "[" + e.Vendor + "] " + e.Code + ": " + e.Message
}

// IsRecoverable returns true if the error should be retried.
func (e *HumanError) IsRecoverable() bool {
	return e.Recoverable
}

// Common error codes that adapters should use
const (
	ErrCodeONUExists       = "ONU_EXISTS"
	ErrCodeONUNotFound     = "ONU_NOT_FOUND"
	ErrCodeInvalidSerial   = "INVALID_SERIAL"
	ErrCodePortNotFound    = "PORT_NOT_FOUND"
	ErrCodeConfigLocked    = "CONFIG_LOCKED"
	ErrCodeTimeout         = "TIMEOUT"
	ErrCodeConnReset       = "CONN_RESET"
	ErrCodeONUFull         = "ONU_FULL"
	ErrCodeProfileNotFound = "PROFILE_NOT_FOUND"
	ErrCodeUnknownCommand  = "UNKNOWN_CMD"
	ErrCodeAuthFailed      = "AUTH_FAILED"
	ErrCodeUnknown         = "UNKNOWN_ERROR"
)

// Typical GPON optical power thresholds
const (
	// ONU Rx acceptable range: -8 to -28 dBm
	GPONRxHighThreshold = -8.0
	GPONRxLowThreshold  = -28.0

	// ONU Tx acceptable range: 0.5 to 5 dBm
	GPONTxHighThreshold = 5.0
	GPONTxLowThreshold  = 0.5
)

// IsPowerWithinSpec checks if optical power readings are within GPON spec.
func IsPowerWithinSpec(rxDBm, txDBm float64) bool {
	rxOK := rxDBm >= GPONRxLowThreshold && rxDBm <= GPONRxHighThreshold
	txOK := txDBm >= GPONTxLowThreshold && txDBm <= GPONTxHighThreshold
	return rxOK && txOK
}
