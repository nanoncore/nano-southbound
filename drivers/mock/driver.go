package mock

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
)

// Driver implements a mock southbound driver for testing
// It simulates OLT/BNG behavior without connecting to real equipment
type Driver struct {
	config      *types.EquipmentConfig
	connected   bool
	mu          sync.RWMutex
	subscribers map[string]*mockSubscriber
	onus        []mockONU
	stats       map[string]*types.SubscriberStats
	cmdHistory  []string
}

type mockSubscriber struct {
	ID          string
	ONUSerial   string
	VLAN        int
	PONPort     string
	ONUID       int
	State       string
	IPAddress   string
	IPv6Address string
	CreatedAt   time.Time
}

type mockONU struct {
	PONPort  string
	Serial   string
	MAC      string
	Model    string
	Distance int
	RxPower  float64
}

// NewDriver creates a new mock driver
func NewDriver(config *types.EquipmentConfig) (types.Driver, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	d := &Driver{
		config:      config,
		subscribers: make(map[string]*mockSubscriber),
		stats:       make(map[string]*types.SubscriberStats),
		cmdHistory:  make([]string, 0),
	}

	// Generate some simulated unprovisioned ONUs
	d.generateMockONUs()

	return d, nil
}

// Connect simulates connecting to equipment
func (d *Driver) Connect(ctx context.Context, config *types.EquipmentConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if config != nil {
		d.config = config
	}

	// Simulate connection delay
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}

	d.connected = true
	d.recordCommand("connect")

	return nil
}

// Disconnect closes the simulated connection
func (d *Driver) Disconnect(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.connected = false
	d.recordCommand("disconnect")

	return nil
}

// IsConnected returns connection status
func (d *Driver) IsConnected() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.connected
}

// CreateSubscriber simulates provisioning a subscriber
func (d *Driver) CreateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) (*types.SubscriberResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected {
		return nil, fmt.Errorf("not connected to device")
	}

	// Check if subscriber already exists
	if _, exists := d.subscribers[subscriber.Name]; exists {
		return nil, fmt.Errorf("subscriber %s already exists", subscriber.Name)
	}

	// Simulate provisioning delay
	select {
	case <-time.After(50 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Generate mock ONU ID
	onuID := len(d.subscribers) + 1
	ponPort := "0/1"
	if subscriber.Annotations != nil {
		if port, ok := subscriber.Annotations["nanoncore.com/pon-port"]; ok {
			ponPort = port
		}
	}

	// Create mock subscriber
	mockSub := &mockSubscriber{
		ID:          subscriber.Name,
		ONUSerial:   subscriber.Spec.ONUSerial,
		VLAN:        subscriber.Spec.VLAN,
		PONPort:     ponPort,
		ONUID:       onuID,
		State:       "online",
		IPAddress:   subscriber.Spec.IPAddress,
		IPv6Address: subscriber.Spec.IPv6Address,
		CreatedAt:   time.Now(),
	}

	// If no IP assigned, generate one
	if mockSub.IPAddress == "" {
		mockSub.IPAddress = fmt.Sprintf("100.64.%d.%d", rand.Intn(255), rand.Intn(255)) //nolint:gosec // mock data
	}

	d.subscribers[subscriber.Name] = mockSub

	// Initialize stats
	d.stats[subscriber.Name] = &types.SubscriberStats{
		BytesUp:     0,
		BytesDown:   0,
		PacketsUp:   0,
		PacketsDown: 0,
		Timestamp:   time.Now(),
	}

	// Remove from autofind list if present
	d.removeFromAutofind(subscriber.Spec.ONUSerial)

	// Record CLI command
	d.recordCommand(fmt.Sprintf("onu %d type router sn %s", onuID, subscriber.Spec.ONUSerial))
	d.recordCommand(fmt.Sprintf("onu profile %d line-profile %dM service-profile internet", onuID, tier.Spec.BandwidthDown))
	d.recordCommand(fmt.Sprintf("onu vlan %d user-vlan %d", onuID, subscriber.Spec.VLAN))

	result := &types.SubscriberResult{
		SubscriberID:   subscriber.Name,
		SessionID:      fmt.Sprintf("sess-%d-%d", time.Now().Unix(), onuID),
		AssignedIP:     mockSub.IPAddress,
		AssignedIPv6:   mockSub.IPv6Address,
		InterfaceName:  fmt.Sprintf("gpon %s onu %d", ponPort, onuID),
		VLAN:           subscriber.Spec.VLAN,
		AssignedPrefix: "",
		Metadata: map[string]interface{}{
			"vendor":   "mock",
			"model":    "simulator",
			"pon_port": ponPort,
			"onu_id":   onuID,
		},
	}

	return result, nil
}

// UpdateSubscriber simulates updating subscriber configuration
func (d *Driver) UpdateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected {
		return fmt.Errorf("not connected to device")
	}

	mockSub, exists := d.subscribers[subscriber.Name]
	if !exists {
		return fmt.Errorf("subscriber %s not found", subscriber.Name)
	}

	// Update fields
	mockSub.VLAN = subscriber.Spec.VLAN

	// Record CLI command
	d.recordCommand(fmt.Sprintf("onu profile %d line-profile %dM service-profile internet", mockSub.ONUID, tier.Spec.BandwidthDown))

	return nil
}

// DeleteSubscriber simulates removing a subscriber
func (d *Driver) DeleteSubscriber(ctx context.Context, subscriberID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected {
		return fmt.Errorf("not connected to device")
	}

	mockSub, exists := d.subscribers[subscriberID]
	if !exists {
		return fmt.Errorf("subscriber %s not found", subscriberID)
	}

	// Record CLI command
	d.recordCommand(fmt.Sprintf("no onu %d", mockSub.ONUID))

	delete(d.subscribers, subscriberID)
	delete(d.stats, subscriberID)

	return nil
}

// SuspendSubscriber simulates suspending a subscriber
func (d *Driver) SuspendSubscriber(ctx context.Context, subscriberID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected {
		return fmt.Errorf("not connected to device")
	}

	mockSub, exists := d.subscribers[subscriberID]
	if !exists {
		return fmt.Errorf("subscriber %s not found", subscriberID)
	}

	mockSub.State = "suspended"

	// Record CLI command
	d.recordCommand(fmt.Sprintf("onu disable %d", mockSub.ONUID))

	return nil
}

// ResumeSubscriber simulates resuming a subscriber
func (d *Driver) ResumeSubscriber(ctx context.Context, subscriberID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected {
		return fmt.Errorf("not connected to device")
	}

	mockSub, exists := d.subscribers[subscriberID]
	if !exists {
		return fmt.Errorf("subscriber %s not found", subscriberID)
	}

	mockSub.State = "online"

	// Record CLI command
	d.recordCommand(fmt.Sprintf("no onu disable %d", mockSub.ONUID))

	return nil
}

// GetSubscriberStatus returns simulated subscriber status
func (d *Driver) GetSubscriberStatus(ctx context.Context, subscriberID string) (*types.SubscriberStatus, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.connected {
		return nil, fmt.Errorf("not connected to device")
	}

	mockSub, exists := d.subscribers[subscriberID]
	if !exists {
		return nil, fmt.Errorf("subscriber %s not found", subscriberID)
	}

	status := &types.SubscriberStatus{
		SubscriberID:  subscriberID,
		State:         mockSub.State,
		SessionID:     fmt.Sprintf("sess-%s", subscriberID),
		IPv4Address:   mockSub.IPAddress,
		IPv6Address:   mockSub.IPv6Address,
		UptimeSeconds: int64(time.Since(mockSub.CreatedAt).Seconds()),
		LastActivity:  time.Now(),
		IsOnline:      mockSub.State == "online",
		Metadata: map[string]interface{}{
			"pon_port":     mockSub.PONPort,
			"onu_id":       mockSub.ONUID,
			"rx_power_dbm": fmt.Sprintf("-%.1f", 18.0+rand.Float64()*4), //nolint:gosec // mock data
			"tx_power_dbm": fmt.Sprintf("%.1f", 2.0+rand.Float64()*2),   //nolint:gosec // mock data
		},
	}

	return status, nil
}

// GetSubscriberStats returns simulated subscriber statistics
func (d *Driver) GetSubscriberStats(ctx context.Context, subscriberID string) (*types.SubscriberStats, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected {
		return nil, fmt.Errorf("not connected to device")
	}

	stats, exists := d.stats[subscriberID]
	if !exists {
		return nil, fmt.Errorf("subscriber %s not found", subscriberID)
	}

	// Simulate traffic growth (nolint:gosec - mock data, not security sensitive)
	stats.BytesUp += uint64(rand.Intn(1000000))    //nolint:gosec // mock data
	stats.BytesDown += uint64(rand.Intn(10000000)) //nolint:gosec // mock data
	stats.PacketsUp += uint64(rand.Intn(10000))    //nolint:gosec // mock data
	stats.PacketsDown += uint64(rand.Intn(100000)) //nolint:gosec // mock data
	stats.RateUp = uint64(rand.Intn(100000000))    //nolint:gosec // mock data - Up to 100 Mbps
	stats.RateDown = uint64(rand.Intn(500000000))  //nolint:gosec // mock data - Up to 500 Mbps
	stats.Timestamp = time.Now()

	// Return a copy
	return &types.SubscriberStats{
		BytesUp:     stats.BytesUp,
		BytesDown:   stats.BytesDown,
		PacketsUp:   stats.PacketsUp,
		PacketsDown: stats.PacketsDown,
		ErrorsUp:    0,
		ErrorsDown:  0,
		Drops:       0,
		RateUp:      stats.RateUp,
		RateDown:    stats.RateDown,
		Timestamp:   stats.Timestamp,
		Metadata:    map[string]interface{}{},
	}, nil
}

// HealthCheck returns success for mock driver
func (d *Driver) HealthCheck(ctx context.Context) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.connected {
		return fmt.Errorf("not connected to device")
	}

	d.recordCommand("show system")
	return nil
}

// ExecCommand implements CLIExecutor - simulates CLI execution
func (d *Driver) ExecCommand(ctx context.Context, command string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected {
		return "", fmt.Errorf("not connected to device")
	}

	d.cmdHistory = append(d.cmdHistory, command)

	// Handle autofind commands
	cmdLower := strings.ToLower(command)
	if strings.Contains(cmdLower, "autofind") || strings.Contains(cmdLower, "uncfg") {
		return d.generateAutofindOutput(), nil
	}

	// Handle show version
	if strings.Contains(cmdLower, "version") || strings.Contains(cmdLower, "system") {
		return d.generateVersionOutput(), nil
	}

	// Handle show onu info
	if strings.Contains(cmdLower, "onu-info") || strings.Contains(cmdLower, "ont info") {
		return d.generateONUInfoOutput(), nil
	}

	// Handle show statistics
	if strings.Contains(cmdLower, "statistics") || strings.Contains(cmdLower, "traffic") {
		return d.generateStatsOutput(), nil
	}

	// Default: return success
	return fmt.Sprintf("OK: %s", command), nil
}

// ExecCommands implements CLIExecutor - executes multiple commands
func (d *Driver) ExecCommands(ctx context.Context, commands []string) ([]string, error) {
	results := make([]string, 0, len(commands))
	for _, cmd := range commands {
		output, err := d.ExecCommand(ctx, cmd)
		if err != nil {
			return results, err
		}
		results = append(results, output)
	}
	return results, nil
}

// GetCommandHistory returns the command history (useful for testing)
func (d *Driver) GetCommandHistory() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	history := make([]string, len(d.cmdHistory))
	copy(history, d.cmdHistory)
	return history
}

// Helper methods

func (d *Driver) recordCommand(cmd string) {
	d.cmdHistory = append(d.cmdHistory, cmd)
}

func (d *Driver) generateMockONUs() {
	// Generate some unprovisioned ONUs
	prefixes := []string{"VSOL", "HWTC", "ZTEG", "GPON"}
	models := []string{"V2802GWT", "HG8245Q2", "F670L", "ONU-001"}

	//nolint:gosec // mock data - all rand usage below is for simulating test data
	for i := 0; i < 5; i++ {
		prefix := prefixes[rand.Intn(len(prefixes))]
		model := models[rand.Intn(len(models))]
		serial := fmt.Sprintf("%s%08X", prefix, rand.Uint32())
		mac := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
			rand.Intn(256), rand.Intn(256), rand.Intn(256),
			rand.Intn(256), rand.Intn(256), rand.Intn(256))

		d.onus = append(d.onus, mockONU{
			PONPort:  fmt.Sprintf("0/%d", rand.Intn(4)+1),
			Serial:   serial,
			MAC:      mac,
			Model:    model,
			Distance: 100 + rand.Intn(5000),
			RxPower:  -18.0 - rand.Float64()*10,
		})
	}
}

func (d *Driver) removeFromAutofind(serial string) {
	newONUs := make([]mockONU, 0)
	for _, onu := range d.onus {
		if onu.Serial != serial {
			newONUs = append(newONUs, onu)
		}
	}
	d.onus = newONUs
}

func (d *Driver) generateAutofindOutput() string {
	var sb strings.Builder
	sb.WriteString("Port      Serial              MAC               Model         Distance  Rx Power\n")
	sb.WriteString("--------  ------------------  ----------------  ------------  --------  --------\n")

	for _, onu := range d.onus {
		sb.WriteString(fmt.Sprintf("%-8s  %-18s  %-16s  %-12s  %4dm     %.1fdBm\n",
			onu.PONPort, onu.Serial, onu.MAC, onu.Model, onu.Distance, onu.RxPower))
	}

	sb.WriteString(fmt.Sprintf("\nTotal: %d ONU(s) discovered\n", len(d.onus)))
	return sb.String()
}

func (d *Driver) generateVersionOutput() string {
	return `
System Information
==================
Model:          Mock OLT Simulator
Version:        1.0.0
Serial Number:  MOCK-SIM-001
Uptime:         10 days, 5:30:22
CPU Usage:      15%
Memory Usage:   42%
Temperature:    45C

PON Ports:      8 GPON
Active ONUs:    %d
`
}

func (d *Driver) generateONUInfoOutput() string {
	return `
ONU Information
===============
Run state:          online
Config state:       normal
Online duration:    5 days 12:30:45
Rx optical power:   -19.5 dBm
Tx optical power:   2.3 dBm
OLT Rx ONT power:   -20.1 dBm
Temperature:        42 C
`
}

func (d *Driver) generateStatsOutput() string {
	return fmt.Sprintf(`
ONU Traffic Statistics
======================
Upstream traffic:   %d bytes
Downstream traffic: %d bytes
Upstream packets:   %d
Downstream packets: %d
Errors:             0
Drops:              0
`, rand.Uint64()%100000000, rand.Uint64()%1000000000, rand.Uint64()%1000000, rand.Uint64()%10000000) //nolint:gosec // mock data
}

// Ensure Driver implements required interfaces
var _ types.Driver = (*Driver)(nil)
var _ types.CLIExecutor = (*Driver)(nil)
