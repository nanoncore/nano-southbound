package types

import (
	"context"
	"time"

	"github.com/nanoncore/nano-southbound/model"
)

// Protocol represents the southbound protocol type
type Protocol string

const (
	ProtocolNETCONF Protocol = "netconf"
	ProtocolGNMI    Protocol = "gnmi"
	ProtocolCLI     Protocol = "cli"
	ProtocolSNMP    Protocol = "snmp"
	ProtocolREST    Protocol = "rest"
)

// Vendor represents the network equipment vendor
type Vendor string

const (
	VendorNokia     Vendor = "nokia"
	VendorHuawei    Vendor = "huawei"
	VendorZTE       Vendor = "zte"
	VendorCisco     Vendor = "cisco"
	VendorJuniper   Vendor = "juniper"
	VendorAdtran    Vendor = "adtran"
	VendorCalix     Vendor = "calix"
	VendorDZS       Vendor = "dzs"
	VendorFiberHome Vendor = "fiberhome"
	VendorEricsson  Vendor = "ericsson"
	VendorVSOL      Vendor = "vsol"
	VendorCData     Vendor = "cdata" // C-Data OLTs (FD1104S, FD1208S series)
	VendorMock      Vendor = "mock"  // For testing/simulation
)

// EquipmentType represents the type of network equipment
type EquipmentType string

const (
	EquipmentTypeBNG EquipmentType = "bng"
	EquipmentTypeOLT EquipmentType = "olt"
	EquipmentTypeONU EquipmentType = "onu"
)

// EquipmentConfig contains configuration for a network equipment instance
type EquipmentConfig struct {
	// Name is a unique identifier for this equipment
	Name string

	// Type is the equipment type (BNG, OLT, ONU)
	Type EquipmentType

	// Vendor is the equipment vendor
	Vendor Vendor

	// Address is the management IP/hostname
	Address string

	// Port is the management port (if not default)
	Port int

	// Protocol is the primary management protocol
	Protocol Protocol

	// Username for authentication
	Username string

	// Password for authentication
	Password string

	// TLSEnabled indicates if TLS should be used
	TLSEnabled bool

	// TLSSkipVerify skips TLS certificate verification (insecure, for testing)
	TLSSkipVerify bool

	// Timeout for operations
	Timeout time.Duration

	// Metadata contains vendor-specific configuration
	Metadata map[string]string
}

// Driver is the interface that all southbound drivers must implement
// This abstracts the protocol-specific details (NETCONF, gNMI, CLI, SNMP)
type Driver interface {
	// Connect establishes a connection to the equipment
	Connect(ctx context.Context, config *EquipmentConfig) error

	// Disconnect closes the connection
	Disconnect(ctx context.Context) error

	// IsConnected returns true if connected
	IsConnected() bool

	// CreateSubscriber provisions a subscriber on the equipment
	CreateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) (*SubscriberResult, error)

	// UpdateSubscriber updates subscriber configuration
	UpdateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) error

	// DeleteSubscriber removes a subscriber from the equipment
	DeleteSubscriber(ctx context.Context, subscriberID string) error

	// SuspendSubscriber suspends a subscriber (keep config, disable traffic)
	SuspendSubscriber(ctx context.Context, subscriberID string) error

	// ResumeSubscriber resumes a suspended subscriber
	ResumeSubscriber(ctx context.Context, subscriberID string) error

	// GetSubscriberStatus retrieves current subscriber status
	GetSubscriberStatus(ctx context.Context, subscriberID string) (*SubscriberStatus, error)

	// GetSubscriberStats retrieves subscriber traffic statistics
	GetSubscriberStats(ctx context.Context, subscriberID string) (*SubscriberStats, error)

	// HealthCheck performs a health check on the connection
	HealthCheck(ctx context.Context) error
}

// CLIExecutor is an optional interface for drivers that support CLI execution
// Vendor adapters can use this to send vendor-specific commands
type CLIExecutor interface {
	// ExecCommand executes a CLI command and returns the output
	ExecCommand(ctx context.Context, command string) (string, error)

	// ExecCommands executes multiple CLI commands sequentially
	ExecCommands(ctx context.Context, commands []string) ([]string, error)
}

// SNMPExecutor is an optional interface for drivers that support SNMP queries
// Used for monitoring and telemetry collection
type SNMPExecutor interface {
	// GetSNMP retrieves a single SNMP value by OID
	GetSNMP(ctx context.Context, oid string) (interface{}, error)

	// WalkSNMP performs an SNMP walk on an OID subtree
	WalkSNMP(ctx context.Context, oid string) (map[string]interface{}, error)

	// BulkGetSNMP retrieves multiple OIDs in one request
	BulkGetSNMP(ctx context.Context, oids []string) (map[string]interface{}, error)
}

// SubscriberResult contains the result of subscriber provisioning
type SubscriberResult struct {
	// SubscriberID is the unique identifier on the equipment
	SubscriberID string

	// SessionID is the session identifier (PPPoE, IPoE, etc.)
	SessionID string

	// AssignedIP is the assigned IPv4 address
	AssignedIP string

	// AssignedIPv6 is the assigned IPv6 address
	AssignedIPv6 string

	// AssignedPrefix is the delegated IPv6 prefix
	AssignedPrefix string

	// InterfaceName is the logical interface name
	InterfaceName string

	// VLAN is the assigned VLAN ID
	VLAN int

	// Metadata contains vendor-specific data
	Metadata map[string]interface{}
}

// SubscriberStatus represents the current status of a subscriber
type SubscriberStatus struct {
	// SubscriberID is the unique identifier
	SubscriberID string

	// State is the current state (active, suspended, offline, etc.)
	State string

	// SessionID is the active session ID
	SessionID string

	// IP addresses
	IPv4Address string
	IPv6Address string
	IPv6Prefix  string

	// Uptime in seconds
	UptimeSeconds int64

	// LastActivity timestamp
	LastActivity time.Time

	// IsOnline indicates if subscriber is currently online
	IsOnline bool

	// Metadata contains vendor-specific status data
	Metadata map[string]interface{}
}

// SubscriberStats contains subscriber traffic statistics
type SubscriberStats struct {
	// Bytes
	BytesUp   uint64
	BytesDown uint64

	// Packets
	PacketsUp   uint64
	PacketsDown uint64

	// Errors
	ErrorsUp   uint64
	ErrorsDown uint64
	Drops      uint64

	// Rates (bits per second)
	RateUp   uint64
	RateDown uint64

	// Timestamp of measurement
	Timestamp time.Time

	// Metadata contains vendor-specific metrics
	Metadata map[string]interface{}
}

// EquipmentStatus represents the status of the equipment itself
type EquipmentStatus struct {
	// IsReachable indicates if equipment is reachable
	IsReachable bool

	// IsHealthy indicates if equipment is functioning properly
	IsHealthy bool

	// Version is the software version
	Version string

	// Model is the hardware model
	Model string

	// SerialNumber is the equipment serial number
	SerialNumber string

	// Uptime in seconds
	UptimeSeconds int64

	// ActiveSubscribers is the number of active subscribers
	ActiveSubscribers int

	// CPU and memory utilization (percentage)
	CPUUtilization    float64
	MemoryUtilization float64

	// Metadata contains vendor-specific status
	Metadata map[string]interface{}
}
