package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
	"golang.org/x/crypto/ssh"
)

// Driver implements the types.Driver interface using SSH CLI
type Driver struct {
	config        *types.EquipmentConfig
	sshClient     *ssh.Client
	expectSession *ExpectSession
}

// NewDriver creates a new CLI driver
func NewDriver(config *types.EquipmentConfig) (types.Driver, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if config.Address == "" {
		return nil, fmt.Errorf("address is required")
	}

	// Default SSH port
	if config.Port == 0 {
		config.Port = 22
	}

	// Default timeout
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &Driver{
		config: config,
	}, nil
}

// Connect establishes an SSH connection
func (d *Driver) Connect(ctx context.Context, config *types.EquipmentConfig) error {
	if config != nil {
		d.config = config
	}

	// Prepare SSH client configuration with multiple auth methods
	// Some devices (like V-Sol OLTs) may require keyboard-interactive instead of password
	keyboardInteractive := ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, len(questions))
		for i := range questions {
			answers[i] = d.config.Password
		}
		return answers, nil
	})

	sshConfig := &ssh.ClientConfig{
		User: d.config.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(d.config.Password),
			keyboardInteractive,
		},
		Timeout:         d.config.Timeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // User-controlled via TLSSkipVerify
	}

	// Target address
	target := fmt.Sprintf("%s:%d", d.config.Address, d.config.Port)

	// Establish SSH connection
	client, err := ssh.Dial("tcp", target, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to dial SSH: %w", err)
	}

	d.sshClient = client

	// Create expect session for interactive CLI
	// Pass credentials for double-login scenarios (e.g., V-Sol OLTs)
	expectSession, err := NewExpectSession(ExpectSessionConfig{
		SSHClient:    client,
		Vendor:       string(d.config.Vendor),
		Timeout:      d.config.Timeout,
		DisablePager: true,
		Username:     d.config.Username,
		Password:     d.config.Password,
	})
	if err != nil {
		client.Close()
		d.sshClient = nil
		return fmt.Errorf("failed to create expect session: %w", err)
	}

	d.expectSession = expectSession

	return nil
}

// Disconnect closes the SSH connection
func (d *Driver) Disconnect(ctx context.Context) error {
	if d.expectSession != nil {
		_ = d.expectSession.Close()
		d.expectSession = nil
	}
	if d.sshClient != nil {
		err := d.sshClient.Close()
		d.sshClient = nil
		return err
	}
	return nil
}

// IsConnected returns true if connected
func (d *Driver) IsConnected() bool {
	return d.sshClient != nil && d.expectSession != nil
}

// execCommand executes a CLI command over SSH using expect-based PTY session
func (d *Driver) execCommand(ctx context.Context, command string) (string, error) {
	if !d.IsConnected() {
		return "", fmt.Errorf("not connected to device")
	}

	// Execute command using expect session (handles interactive CLI properly)
	output, err := d.expectSession.Execute(command)
	if err != nil {
		return output, fmt.Errorf("command failed: %w", err)
	}

	return output, nil
}

// CreateSubscriber provisions a subscriber using CLI commands
func (d *Driver) CreateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) (*types.SubscriberResult, error) {
	// This is a generic implementation
	// Vendor adapters will override with vendor-specific CLI commands

	// Example generic CLI commands (would be customized per vendor)
	commands := []string{
		"configure terminal",
		fmt.Sprintf("interface sub-%s", subscriber.Spec.ONUSerial),
		fmt.Sprintf("  description %s", subscriber.Spec.Description),
		fmt.Sprintf("  encapsulation dot1q %d", subscriber.Spec.VLAN),
		"  no shutdown",
		fmt.Sprintf("  service-policy output rate-limit-%d", tier.Spec.BandwidthDown),
		fmt.Sprintf("  service-policy input rate-limit-%d", tier.Spec.BandwidthUp),
		"exit",
		"commit",
	}

	// Execute commands
	commandStr := strings.Join(commands, "\n")
	output, err := d.execCommand(ctx, commandStr)
	if err != nil {
		return nil, fmt.Errorf("CLI provisioning failed: %w (output: %s)", err, output)
	}

	// Build result
	result := &types.SubscriberResult{
		SubscriberID:   subscriber.Name,
		SessionID:      fmt.Sprintf("sess-%s", subscriber.Spec.ONUSerial),
		AssignedIP:     subscriber.Spec.IPAddress,
		AssignedIPv6:   subscriber.Spec.IPv6Address,
		AssignedPrefix: subscriber.Spec.DelegatedPrefix,
		InterfaceName:  fmt.Sprintf("sub-%s", subscriber.Spec.ONUSerial),
		VLAN:           subscriber.Spec.VLAN,
		Metadata: map[string]interface{}{
			"cli_output": output,
		},
	}

	return result, nil
}

// UpdateSubscriber updates subscriber configuration
func (d *Driver) UpdateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) error {
	// For CLI, update typically means reconfiguring the interface
	// This is vendor-specific - some vendors support in-place updates, others require delete+create

	// Generic approach: modify existing configuration
	commands := []string{
		"configure terminal",
		fmt.Sprintf("interface sub-%s", subscriber.Spec.ONUSerial),
		fmt.Sprintf("  service-policy output rate-limit-%d", tier.Spec.BandwidthDown),
		fmt.Sprintf("  service-policy input rate-limit-%d", tier.Spec.BandwidthUp),
		"exit",
		"commit",
	}

	commandStr := strings.Join(commands, "\n")
	_, err := d.execCommand(ctx, commandStr)
	return err
}

// DeleteSubscriber removes a subscriber
func (d *Driver) DeleteSubscriber(ctx context.Context, subscriberID string) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}

	// Generic delete commands
	commands := []string{
		"configure terminal",
		fmt.Sprintf("no interface sub-%s", subscriberID),
		"commit",
	}

	commandStr := strings.Join(commands, "\n")
	_, err := d.execCommand(ctx, commandStr)
	return err
}

// SuspendSubscriber suspends a subscriber
func (d *Driver) SuspendSubscriber(ctx context.Context, subscriberID string) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}

	// Set interface admin down
	commands := []string{
		"configure terminal",
		fmt.Sprintf("interface sub-%s", subscriberID),
		"  shutdown",
		"exit",
		"commit",
	}

	commandStr := strings.Join(commands, "\n")
	_, err := d.execCommand(ctx, commandStr)
	return err
}

// ResumeSubscriber resumes a suspended subscriber
func (d *Driver) ResumeSubscriber(ctx context.Context, subscriberID string) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}

	// Set interface admin up
	commands := []string{
		"configure terminal",
		fmt.Sprintf("interface sub-%s", subscriberID),
		"  no shutdown",
		"exit",
		"commit",
	}

	commandStr := strings.Join(commands, "\n")
	_, err := d.execCommand(ctx, commandStr)
	return err
}

// GetSubscriberStatus retrieves subscriber status
func (d *Driver) GetSubscriberStatus(ctx context.Context, subscriberID string) (*types.SubscriberStatus, error) {
	if !d.IsConnected() {
		return nil, fmt.Errorf("not connected to device")
	}

	// Execute show command
	output, err := d.execCommand(ctx, fmt.Sprintf("show interface sub-%s", subscriberID))
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	// Parse output (vendor-specific parsing in adapters)
	status := &types.SubscriberStatus{
		SubscriberID: subscriberID,
		State:        "active",
		IsOnline:     strings.Contains(output, "up"),
		LastActivity: time.Now(),
		Metadata: map[string]interface{}{
			"cli_output": output,
		},
	}

	return status, nil
}

// GetSubscriberStats retrieves subscriber statistics
func (d *Driver) GetSubscriberStats(ctx context.Context, subscriberID string) (*types.SubscriberStats, error) {
	if !d.IsConnected() {
		return nil, fmt.Errorf("not connected to device")
	}

	// Execute show stats command
	output, err := d.execCommand(ctx, fmt.Sprintf("show interface sub-%s statistics", subscriberID))
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// Parse output (vendor-specific parsing in adapters)
	stats := &types.SubscriberStats{
		BytesUp:   0, // Parse from CLI output
		BytesDown: 0,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"cli_output": output,
		},
	}

	return stats, nil
}

// HealthCheck performs a health check
func (d *Driver) HealthCheck(ctx context.Context) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}

	// Execute simple show command
	_, err := d.execCommand(ctx, "show version")
	return err
}

// ExecCommand implements types.CLIExecutor - executes a single CLI command
func (d *Driver) ExecCommand(ctx context.Context, command string) (string, error) {
	return d.execCommand(ctx, command)
}

// ExecCommands implements types.CLIExecutor - executes multiple CLI commands sequentially
func (d *Driver) ExecCommands(ctx context.Context, commands []string) ([]string, error) {
	results := make([]string, 0, len(commands))
	for _, cmd := range commands {
		output, err := d.execCommand(ctx, cmd)
		if err != nil {
			return results, fmt.Errorf("command %q failed: %w", cmd, err)
		}
		results = append(results, output)
	}
	return results, nil
}

// Ensure Driver implements CLIExecutor
var _ types.CLIExecutor = (*Driver)(nil)
