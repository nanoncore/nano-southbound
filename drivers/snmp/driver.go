package snmp

import (
	"context"
	"fmt"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
)

// Driver implements the types.Driver interface using SNMP
// Note: SNMP is primarily used for monitoring, not configuration
// Configuration typically requires CLI or other protocol
type Driver struct {
	config *types.EquipmentConfig
	snmp   *gosnmp.GoSNMP
}

// NewDriver creates a new SNMP driver
func NewDriver(config *types.EquipmentConfig) (types.Driver, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if config.Address == "" {
		return nil, fmt.Errorf("address is required")
	}

	// Default SNMP port
	if config.Port == 0 {
		config.Port = 161
	}

	// Default timeout
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &Driver{
		config: config,
	}, nil
}

// Connect establishes an SNMP connection
func (d *Driver) Connect(ctx context.Context, config *types.EquipmentConfig) error {
	if config != nil {
		d.config = config
	}

	// Get SNMP version from metadata (default v2c)
	version := gosnmp.Version2c
	if v, ok := d.config.Metadata["snmp_version"]; ok {
		switch v {
		case "1":
			version = gosnmp.Version1
		case "2c":
			version = gosnmp.Version2c
		case "3":
			version = gosnmp.Version3
		}
	}

	// Get community string (default: public)
	community := "public"
	if c, ok := d.config.Metadata["snmp_community"]; ok {
		community = c
	}

	// Create SNMP client
	port := d.config.Port
	if port < 0 || port > 65535 {
		port = 161 // default SNMP port
	}
	snmpClient := &gosnmp.GoSNMP{
		Target:    d.config.Address,
		Port:      uint16(port), //nolint:gosec // validated above
		Community: community,
		Version:   version,
		Timeout:   d.config.Timeout,
		Retries:   3,
	}

	// For SNMPv3, set security parameters
	if version == gosnmp.Version3 {
		snmpClient.SecurityModel = gosnmp.UserSecurityModel
		snmpClient.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 d.config.Username,
			AuthenticationProtocol:   gosnmp.SHA,
			AuthenticationPassphrase: d.config.Password,
			PrivacyProtocol:          gosnmp.AES,
			PrivacyPassphrase:        d.config.Password,
		}
		snmpClient.MsgFlags = gosnmp.AuthPriv
	}

	// Connect
	err := snmpClient.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect SNMP: %w", err)
	}

	d.snmp = snmpClient

	return nil
}

// Disconnect closes the SNMP connection
func (d *Driver) Disconnect(ctx context.Context) error {
	if d.snmp != nil {
		err := d.snmp.Conn.Close()
		d.snmp = nil
		return err
	}
	return nil
}

// IsConnected returns true if connected
func (d *Driver) IsConnected() bool {
	return d.snmp != nil
}

// CreateSubscriber provisions a subscriber
// Note: SNMP typically cannot create subscribers - this would require CLI/NETCONF
// This implementation returns an error directing to use appropriate protocol
func (d *Driver) CreateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) (*types.SubscriberResult, error) {
	return nil, fmt.Errorf("SNMP driver does not support subscriber provisioning - use CLI or NETCONF for configuration")
}

// UpdateSubscriber updates subscriber configuration
// Note: SNMP typically cannot update configuration
func (d *Driver) UpdateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) error {
	return fmt.Errorf("SNMP driver does not support subscriber configuration - use CLI or NETCONF")
}

// DeleteSubscriber removes a subscriber
// Note: SNMP typically cannot delete subscribers
func (d *Driver) DeleteSubscriber(ctx context.Context, subscriberID string) error {
	return fmt.Errorf("SNMP driver does not support subscriber deletion - use CLI or NETCONF")
}

// SuspendSubscriber suspends a subscriber
// Note: Some SNMP implementations support setting interface admin status via SET
func (d *Driver) SuspendSubscriber(ctx context.Context, subscriberID string) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}

	// This is vendor-specific
	// Standard MIB-II ifAdminStatus OID: .1.3.6.1.2.1.2.2.1.7
	// Value 2 = down
	// Vendor adapters will implement with correct OIDs
	return fmt.Errorf("SNMP suspend requires vendor-specific OID mapping")
}

// ResumeSubscriber resumes a suspended subscriber
func (d *Driver) ResumeSubscriber(ctx context.Context, subscriberID string) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}

	// Standard MIB-II ifAdminStatus OID: .1.3.6.1.2.1.2.2.1.7
	// Value 1 = up
	return fmt.Errorf("SNMP resume requires vendor-specific OID mapping")
}

// GetSubscriberStatus retrieves subscriber status
func (d *Driver) GetSubscriberStatus(ctx context.Context, subscriberID string) (*types.SubscriberStatus, error) {
	if !d.IsConnected() {
		return nil, fmt.Errorf("not connected to device")
	}

	// This requires vendor-specific OID mapping
	// Generic approach: query interface table
	// Vendor adapters will implement with correct OIDs

	// Example: Query ifOperStatus (1.3.6.1.2.1.2.2.1.8)
	// This is just a placeholder - vendor adapters provide real implementation

	status := &types.SubscriberStatus{
		SubscriberID: subscriberID,
		State:        "unknown",
		IsOnline:     false,
		LastActivity: time.Now(),
		Metadata:     map[string]interface{}{},
	}

	return status, nil
}

// GetSubscriberStats retrieves subscriber statistics using SNMP
func (d *Driver) GetSubscriberStats(ctx context.Context, subscriberID string) (*types.SubscriberStats, error) {
	if !d.IsConnected() {
		return nil, fmt.Errorf("not connected to device")
	}

	// Standard MIB-II interface counters
	// These OIDs are from RFC 1213 (MIB-II)
	// Vendor-specific implementations will use correct interface index

	// Example OIDs (need interface index):
	// ifInOctets:    .1.3.6.1.2.1.2.2.1.10.<ifIndex>
	// ifOutOctets:   .1.3.6.1.2.1.2.2.1.16.<ifIndex>
	// ifInUcastPkts: .1.3.6.1.2.1.2.2.1.11.<ifIndex>
	// ifOutUcastPkts:.1.3.6.1.2.1.2.2.1.17.<ifIndex>

	// This is a placeholder - vendor adapters provide real implementation
	stats := &types.SubscriberStats{
		BytesUp:   0,
		BytesDown: 0,
		Timestamp: time.Now(),
		Metadata:  map[string]interface{}{},
	}

	return stats, nil
}

// getSNMPValue retrieves a single SNMP value
func (d *Driver) getSNMPValue(oid string) (interface{}, error) {
	if !d.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	result, err := d.snmp.Get([]string{oid})
	if err != nil {
		return nil, fmt.Errorf("SNMP GET failed: %w", err)
	}

	if len(result.Variables) == 0 {
		return nil, fmt.Errorf("no result for OID %s", oid)
	}

	variable := result.Variables[0]

	// Convert based on type
	switch variable.Type {
	case gosnmp.OctetString:
		return string(variable.Value.([]byte)), nil
	case gosnmp.Integer:
		return variable.Value.(int), nil
	case gosnmp.Counter32, gosnmp.Counter64:
		return variable.Value.(uint64), nil
	default:
		return variable.Value, nil
	}
}

// HealthCheck performs a health check
func (d *Driver) HealthCheck(ctx context.Context) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}

	// Query sysDescr (1.3.6.1.2.1.1.1.0) as health check
	_, err := d.getSNMPValue("1.3.6.1.2.1.1.1.0")
	return err
}

// GetSNMP implements types.SNMPExecutor - retrieves a single SNMP value
func (d *Driver) GetSNMP(ctx context.Context, oid string) (interface{}, error) {
	return d.getSNMPValue(oid)
}

// WalkSNMP implements types.SNMPExecutor - performs SNMP walk
func (d *Driver) WalkSNMP(ctx context.Context, oid string) (map[string]interface{}, error) {
	if !d.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	results := make(map[string]interface{})

	err := d.snmp.Walk(oid, func(pdu gosnmp.SnmpPDU) error {
		// Extract the index from the OID (last part after base OID)
		index := pdu.Name[len(oid)+1:] // Skip base OID and dot

		switch pdu.Type {
		case gosnmp.OctetString:
			results[index] = string(pdu.Value.([]byte))
		case gosnmp.Integer:
			results[index] = int64(pdu.Value.(int))
		case gosnmp.Counter32:
			results[index] = uint64(pdu.Value.(uint))
		case gosnmp.Counter64:
			results[index] = pdu.Value.(uint64)
		case gosnmp.Gauge32:
			results[index] = uint64(pdu.Value.(uint))
		default:
			results[index] = pdu.Value
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("SNMP WALK failed: %w", err)
	}

	return results, nil
}

// BulkGetSNMP implements types.SNMPExecutor - retrieves multiple OIDs
func (d *Driver) BulkGetSNMP(ctx context.Context, oids []string) (map[string]interface{}, error) {
	if !d.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	result, err := d.snmp.Get(oids)
	if err != nil {
		return nil, fmt.Errorf("SNMP GET failed: %w", err)
	}

	results := make(map[string]interface{})
	for _, variable := range result.Variables {
		switch variable.Type {
		case gosnmp.OctetString:
			results[variable.Name] = string(variable.Value.([]byte))
		case gosnmp.Integer:
			results[variable.Name] = int64(variable.Value.(int))
		case gosnmp.Counter32:
			results[variable.Name] = uint64(variable.Value.(uint))
		case gosnmp.Counter64:
			results[variable.Name] = variable.Value.(uint64)
		case gosnmp.Gauge32:
			results[variable.Name] = uint64(variable.Value.(uint))
		default:
			results[variable.Name] = variable.Value
		}
	}

	return results, nil
}

// Ensure Driver implements SNMPExecutor
var _ types.SNMPExecutor = (*Driver)(nil)
