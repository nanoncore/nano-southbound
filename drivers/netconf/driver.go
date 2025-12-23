package netconf

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
	"golang.org/x/crypto/ssh"
)

// NETCONF message IDs
var messageID uint64 = 0
var messageIDMu sync.Mutex

func nextMessageID() uint64 {
	messageIDMu.Lock()
	defer messageIDMu.Unlock()
	messageID++
	return messageID
}

// NETCONF constants
const (
	NetconfBase10   = "urn:ietf:params:netconf:base:1.0"
	NetconfBase11   = "urn:ietf:params:netconf:base:1.1"
	NetconfFrameEnd = "]]>]]>"

	// NETCONF capabilities
	CapWritableRunning = "urn:ietf:params:netconf:capability:writable-running:1.0"
	CapCandidate       = "urn:ietf:params:netconf:capability:candidate:1.0"
	CapConfirmedCommit = "urn:ietf:params:netconf:capability:confirmed-commit:1.0"
	CapRollback        = "urn:ietf:params:netconf:capability:rollback-on-error:1.0"
	CapValidate        = "urn:ietf:params:netconf:capability:validate:1.0"
	CapStartup         = "urn:ietf:params:netconf:capability:startup:1.0"
	CapXPath           = "urn:ietf:params:netconf:capability:xpath:1.0"
)

// Driver implements the types.Driver interface using NETCONF/YANG
type Driver struct {
	config       *types.EquipmentConfig
	sshClient    *ssh.Client
	session      *ssh.Session
	stdin        *netconfWriter
	stdout       *netconfReader
	connected    bool
	capabilities []string
	sessionID    string
	mu           sync.Mutex
}

// netconfWriter wraps SSH stdin for NETCONF framing
type netconfWriter struct {
	writer   interface{ Write([]byte) (int, error) }
	useChunk bool // NETCONF 1.1 chunked framing
}

func (w *netconfWriter) Write(data []byte) (int, error) {
	if w.useChunk {
		// NETCONF 1.1 chunked framing
		chunk := fmt.Sprintf("\n#%d\n%s\n##\n", len(data), string(data))
		return w.writer.Write([]byte(chunk))
	}
	// NETCONF 1.0 EOM framing
	return w.writer.Write(append(data, []byte(NetconfFrameEnd)...))
}

// netconfReader wraps SSH stdout for NETCONF framing
type netconfReader struct {
	reader   interface{ Read([]byte) (int, error) }
	useChunk bool
}

func (r *netconfReader) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

// ReadMessage reads a complete NETCONF message
func (r *netconfReader) ReadMessage() ([]byte, error) {
	buf := make([]byte, 64*1024)
	var message []byte

	for {
		n, err := r.reader.Read(buf)
		if err != nil {
			return nil, err
		}

		message = append(message, buf[:n]...)

		// Check for end-of-message marker (NETCONF 1.0)
		if !r.useChunk && strings.Contains(string(message), NetconfFrameEnd) {
			// Remove the EOM marker
			msg := strings.TrimSuffix(string(message), NetconfFrameEnd)
			return []byte(strings.TrimSpace(msg)), nil
		}

		// NETCONF 1.1 chunked framing
		if r.useChunk && strings.Contains(string(message), "\n##\n") {
			// Parse chunks and reassemble
			return parseChunkedMessage(message)
		}
	}
}

func parseChunkedMessage(data []byte) ([]byte, error) {
	// Simple parser for NETCONF 1.1 chunked framing
	// Format: \n#<size>\n<data>\n##\n
	str := string(data)
	if idx := strings.Index(str, "\n##\n"); idx > 0 {
		// Find the chunk size and extract data
		// This is simplified - production should handle multiple chunks
		start := strings.Index(str, "\n#")
		if start >= 0 {
			sizeEnd := strings.Index(str[start+2:], "\n")
			if sizeEnd > 0 {
				dataStart := start + 2 + sizeEnd + 1
				return []byte(str[dataStart:idx]), nil
			}
		}
	}
	return data, nil
}

// NewDriver creates a new NETCONF driver
func NewDriver(config *types.EquipmentConfig) (types.Driver, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if config.Address == "" {
		return nil, fmt.Errorf("address is required")
	}

	// Default port for NETCONF
	if config.Port == 0 {
		config.Port = 830
	}

	// Default timeout
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &Driver{
		config: config,
	}, nil
}

// Connect establishes a NETCONF session over SSH
func (d *Driver) Connect(ctx context.Context, config *types.EquipmentConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if config != nil {
		d.config = config
	}

	// Build SSH config
	sshConfig := &ssh.ClientConfig{
		User: d.config.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(d.config.Password),
		},
		Timeout: d.config.Timeout,
	}

	// Handle TLS/HostKey verification
	if d.config.TLSSkipVerify {
		sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec // user requested skip verify
	} else {
		// In production, use ssh.FixedHostKey or ssh.KnownHosts
		sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec // TODO: implement proper host key verification
	}

	// Connect to SSH
	addr := fmt.Sprintf("%s:%d", d.config.Address, d.config.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("SSH dial failed: %w", err)
	}
	d.sshClient = client

	// Open session with NETCONF subsystem
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return fmt.Errorf("SSH session failed: %w", err)
	}
	d.session = session

	// Get stdin/stdout for NETCONF messages
	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		return fmt.Errorf("stdin pipe failed: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return fmt.Errorf("stdout pipe failed: %w", err)
	}

	d.stdin = &netconfWriter{writer: stdin, useChunk: false}
	d.stdout = &netconfReader{reader: stdout, useChunk: false}

	// Start NETCONF subsystem
	if err := session.RequestSubsystem("netconf"); err != nil {
		session.Close()
		client.Close()
		return fmt.Errorf("NETCONF subsystem request failed: %w", err)
	}

	// Exchange hello messages
	if err := d.exchangeHello(); err != nil {
		session.Close()
		client.Close()
		return fmt.Errorf("NETCONF hello exchange failed: %w", err)
	}

	d.connected = true
	return nil
}

// exchangeHello performs NETCONF hello exchange
func (d *Driver) exchangeHello() error {
	// Read server hello
	serverHello, err := d.stdout.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read server hello: %w", err)
	}

	// Parse server capabilities
	d.capabilities, d.sessionID = parseHello(serverHello)

	// Check for NETCONF 1.1 support and switch to chunked framing
	for _, cap := range d.capabilities {
		if strings.Contains(cap, "base:1.1") {
			d.stdin.useChunk = true
			d.stdout.useChunk = true
			break
		}
	}

	// Send client hello
	clientHello := `<?xml version="1.0" encoding="UTF-8"?>
<hello xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <capabilities>
    <capability>urn:ietf:params:netconf:base:1.0</capability>
    <capability>urn:ietf:params:netconf:base:1.1</capability>
    <capability>urn:ietf:params:netconf:capability:writable-running:1.0</capability>
    <capability>urn:ietf:params:netconf:capability:candidate:1.0</capability>
    <capability>urn:ietf:params:netconf:capability:confirmed-commit:1.0</capability>
    <capability>urn:ietf:params:netconf:capability:rollback-on-error:1.0</capability>
    <capability>urn:ietf:params:netconf:capability:validate:1.0</capability>
  </capabilities>
</hello>`

	if _, err := d.stdin.Write([]byte(clientHello)); err != nil {
		return fmt.Errorf("failed to send client hello: %w", err)
	}

	return nil
}

// parseHello extracts capabilities and session ID from hello message
func parseHello(data []byte) ([]string, string) {
	var capabilities []string
	var sessionID string

	// Simple XML parsing for hello
	type Hello struct {
		XMLName      xml.Name `xml:"hello"`
		SessionID    string   `xml:"session-id"`
		Capabilities struct {
			Capability []string `xml:"capability"`
		} `xml:"capabilities"`
	}

	var hello Hello
	if err := xml.Unmarshal(data, &hello); err == nil {
		capabilities = hello.Capabilities.Capability
		sessionID = hello.SessionID
	}

	return capabilities, sessionID
}

// Disconnect closes the NETCONF session
func (d *Driver) Disconnect(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.connected {
		// Send close-session RPC
		closeSession := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rpc message-id="%d" xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <close-session/>
</rpc>`, nextMessageID())

		d.stdin.Write([]byte(closeSession)) //nolint:errcheck // best effort
	}

	if d.session != nil {
		d.session.Close()
	}
	if d.sshClient != nil {
		d.sshClient.Close()
	}

	d.connected = false
	return nil
}

// IsConnected returns true if connected
func (d *Driver) IsConnected() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.connected
}

// GetCapabilities returns the server's NETCONF capabilities
func (d *Driver) GetCapabilities() []string {
	return d.capabilities
}

// HasCapability checks if server supports a specific capability
func (d *Driver) HasCapability(cap string) bool {
	for _, c := range d.capabilities {
		if strings.Contains(c, cap) {
			return true
		}
	}
	return false
}

// RPC sends a NETCONF RPC and returns the response
func (d *Driver) RPC(ctx context.Context, operation string) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected {
		return nil, fmt.Errorf("not connected to device")
	}

	msgID := nextMessageID()
	rpc := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rpc message-id="%d" xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
%s
</rpc>`, msgID, operation)

	if _, err := d.stdin.Write([]byte(rpc)); err != nil {
		return nil, fmt.Errorf("failed to send RPC: %w", err)
	}

	reply, err := d.stdout.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("failed to read RPC reply: %w", err)
	}

	// Check for RPC error
	if strings.Contains(string(reply), "<rpc-error>") {
		return reply, fmt.Errorf("RPC error: %s", extractRPCError(reply))
	}

	return reply, nil
}

// extractRPCError extracts error message from RPC error response
func extractRPCError(data []byte) string {
	type RPCError struct {
		XMLName      xml.Name `xml:"rpc-error"`
		ErrorType    string   `xml:"error-type"`
		ErrorTag     string   `xml:"error-tag"`
		ErrorMessage string   `xml:"error-message"`
	}
	type RPCReply struct {
		XMLName xml.Name   `xml:"rpc-reply"`
		Errors  []RPCError `xml:"rpc-error"`
	}

	var reply RPCReply
	if err := xml.Unmarshal(data, &reply); err == nil && len(reply.Errors) > 0 {
		e := reply.Errors[0]
		return fmt.Sprintf("%s: %s - %s", e.ErrorType, e.ErrorTag, e.ErrorMessage)
	}
	return string(data)
}

// Get performs NETCONF get operation
func (d *Driver) Get(ctx context.Context, filter string) ([]byte, error) {
	var operation string
	if filter != "" {
		operation = fmt.Sprintf(`<get>
  <filter type="subtree">
    %s
  </filter>
</get>`, filter)
	} else {
		operation = "<get/>"
	}
	return d.RPC(ctx, operation)
}

// GetConfig performs NETCONF get-config operation
func (d *Driver) GetConfig(ctx context.Context, source, filter string) ([]byte, error) {
	if source == "" {
		source = "running"
	}

	operation := fmt.Sprintf(`<get-config>
  <source>
    <%s/>
  </source>`, source)

	if filter != "" {
		operation += fmt.Sprintf(`
  <filter type="subtree">
    %s
  </filter>`, filter)
	}
	operation += "\n</get-config>"

	return d.RPC(ctx, operation)
}

// EditConfig performs NETCONF edit-config operation
func (d *Driver) EditConfig(ctx context.Context, target, config string, options ...EditOption) error {
	if target == "" {
		target = "running"
		// Use candidate if available
		if d.HasCapability(CapCandidate) {
			target = "candidate"
		}
	}

	// Build options
	opts := &editOptions{}
	for _, opt := range options {
		opt(opts)
	}

	operation := fmt.Sprintf(`<edit-config>
  <target>
    <%s/>
  </target>`, target)

	if opts.defaultOperation != "" {
		operation += fmt.Sprintf("\n  <default-operation>%s</default-operation>", opts.defaultOperation)
	}
	if opts.testOption != "" && d.HasCapability(CapValidate) {
		operation += fmt.Sprintf("\n  <test-option>%s</test-option>", opts.testOption)
	}
	if opts.errorOption != "" {
		operation += fmt.Sprintf("\n  <error-option>%s</error-option>", opts.errorOption)
	}

	operation += fmt.Sprintf(`
  <config>
    %s
  </config>
</edit-config>`, config)

	_, err := d.RPC(ctx, operation)
	if err != nil {
		return err
	}

	// If using candidate, commit the changes
	if target == "candidate" {
		return d.Commit(ctx)
	}

	return nil
}

// EditOption configures edit-config behavior
type EditOption func(*editOptions)

type editOptions struct {
	defaultOperation string
	testOption       string
	errorOption      string
}

// WithMerge sets default-operation to merge
func WithMerge() EditOption {
	return func(o *editOptions) { o.defaultOperation = "merge" }
}

// WithReplace sets default-operation to replace
func WithReplace() EditOption {
	return func(o *editOptions) { o.defaultOperation = "replace" }
}

// WithTestThenSet sets test-option to test-then-set
func WithTestThenSet() EditOption {
	return func(o *editOptions) { o.testOption = "test-then-set" }
}

// WithRollbackOnError sets error-option to rollback-on-error
func WithRollbackOnError() EditOption {
	return func(o *editOptions) { o.errorOption = "rollback-on-error" }
}

// Commit commits candidate configuration
func (d *Driver) Commit(ctx context.Context) error {
	_, err := d.RPC(ctx, "<commit/>")
	return err
}

// DiscardChanges discards uncommitted candidate changes
func (d *Driver) DiscardChanges(ctx context.Context) error {
	_, err := d.RPC(ctx, "<discard-changes/>")
	return err
}

// Lock locks a configuration datastore
func (d *Driver) Lock(ctx context.Context, target string) error {
	if target == "" {
		target = "running"
	}
	operation := fmt.Sprintf(`<lock>
  <target>
    <%s/>
  </target>
</lock>`, target)
	_, err := d.RPC(ctx, operation)
	return err
}

// Unlock unlocks a configuration datastore
func (d *Driver) Unlock(ctx context.Context, target string) error {
	if target == "" {
		target = "running"
	}
	operation := fmt.Sprintf(`<unlock>
  <target>
    <%s/>
  </target>
</unlock>`, target)
	_, err := d.RPC(ctx, operation)
	return err
}

// Validate validates configuration
func (d *Driver) Validate(ctx context.Context, source string) error {
	if !d.HasCapability(CapValidate) {
		return nil // Skip if not supported
	}
	if source == "" {
		source = "candidate"
	}
	operation := fmt.Sprintf(`<validate>
  <source>
    <%s/>
  </source>
</validate>`, source)
	_, err := d.RPC(ctx, operation)
	return err
}

// CreateSubscriber provisions a subscriber using NETCONF edit-config
// This is a base implementation - vendor adapters should override
func (d *Driver) CreateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) (*types.SubscriberResult, error) {
	if !d.IsConnected() {
		return nil, fmt.Errorf("not connected to device")
	}

	// Base implementation returns stub - vendor adapters provide real config
	result := &types.SubscriberResult{
		SubscriberID:   subscriber.Name,
		SessionID:      fmt.Sprintf("sess-%s", subscriber.Spec.ONUSerial),
		AssignedIP:     subscriber.Spec.IPAddress,
		AssignedIPv6:   subscriber.Spec.IPv6Address,
		AssignedPrefix: subscriber.Spec.DelegatedPrefix,
		InterfaceName:  fmt.Sprintf("sub-%s", subscriber.Spec.ONUSerial),
		VLAN:           subscriber.Spec.VLAN,
		Metadata: map[string]interface{}{
			"driver":       "netconf",
			"session_id":   d.sessionID,
			"capabilities": d.capabilities,
		},
	}

	return result, nil
}

// UpdateSubscriber updates subscriber configuration
func (d *Driver) UpdateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) error {
	_, err := d.CreateSubscriber(ctx, subscriber, tier)
	return err
}

// DeleteSubscriber removes a subscriber
func (d *Driver) DeleteSubscriber(ctx context.Context, subscriberID string) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}
	// Base implementation - vendor adapters provide real delete config
	return nil
}

// SuspendSubscriber suspends a subscriber
func (d *Driver) SuspendSubscriber(ctx context.Context, subscriberID string) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}
	return nil
}

// ResumeSubscriber resumes a suspended subscriber
func (d *Driver) ResumeSubscriber(ctx context.Context, subscriberID string) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}
	return nil
}

// GetSubscriberStatus retrieves subscriber status
func (d *Driver) GetSubscriberStatus(ctx context.Context, subscriberID string) (*types.SubscriberStatus, error) {
	if !d.IsConnected() {
		return nil, fmt.Errorf("not connected to device")
	}

	status := &types.SubscriberStatus{
		SubscriberID: subscriberID,
		State:        "active",
		IsOnline:     true,
		LastActivity: time.Now(),
		Metadata:     map[string]interface{}{},
	}

	return status, nil
}

// GetSubscriberStats retrieves subscriber statistics
func (d *Driver) GetSubscriberStats(ctx context.Context, subscriberID string) (*types.SubscriberStats, error) {
	if !d.IsConnected() {
		return nil, fmt.Errorf("not connected to device")
	}

	stats := &types.SubscriberStats{
		BytesUp:   0,
		BytesDown: 0,
		Timestamp: time.Now(),
		Metadata:  map[string]interface{}{},
	}

	return stats, nil
}

// HealthCheck performs a health check using get operation
func (d *Driver) HealthCheck(ctx context.Context) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}

	// Query system info as health check
	_, err := d.Get(ctx, "")
	return err
}

// Ensure Driver implements NETCONFExecutor interface
var _ NETCONFExecutor = (*Driver)(nil)

// NETCONFExecutor is the interface for NETCONF operations
// Vendor adapters can use this to perform NETCONF operations
type NETCONFExecutor interface {
	// RPC sends a raw NETCONF RPC
	RPC(ctx context.Context, operation string) ([]byte, error)

	// Get performs NETCONF get
	Get(ctx context.Context, filter string) ([]byte, error)

	// GetConfig performs NETCONF get-config
	GetConfig(ctx context.Context, source, filter string) ([]byte, error)

	// EditConfig performs NETCONF edit-config
	EditConfig(ctx context.Context, target, config string, options ...EditOption) error

	// Commit commits candidate configuration
	Commit(ctx context.Context) error

	// Lock locks a configuration datastore
	Lock(ctx context.Context, target string) error

	// Unlock unlocks a configuration datastore
	Unlock(ctx context.Context, target string) error

	// HasCapability checks for a specific capability
	HasCapability(cap string) bool

	// GetCapabilities returns server capabilities
	GetCapabilities() []string
}
