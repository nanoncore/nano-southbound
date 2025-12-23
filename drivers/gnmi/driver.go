package gnmi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// SubscriptionMode defines the type of telemetry subscription
type SubscriptionMode int

const (
	// SubscriptionModeOnChange sends updates only when values change
	SubscriptionModeOnChange SubscriptionMode = iota
	// SubscriptionModeSample sends updates at regular intervals
	SubscriptionModeSample
	// SubscriptionModeTargetDefined lets the target decide the mode
	SubscriptionModeTargetDefined
)

// TelemetryUpdate represents a single telemetry update from a subscription
type TelemetryUpdate struct {
	Path      string                 // XPath-style path
	Value     interface{}            // Decoded value
	Timestamp time.Time              // Update timestamp
	Metadata  map[string]interface{} // Additional metadata
}

// TelemetryHandler is called when telemetry updates are received
type TelemetryHandler func(updates []TelemetryUpdate)

// SubscriptionConfig defines a telemetry subscription
type SubscriptionConfig struct {
	Paths             []string         // YANG paths to subscribe to
	Mode              SubscriptionMode // Subscription mode
	SampleInterval    time.Duration    // Sample interval (for SAMPLE mode)
	Handler           TelemetryHandler // Callback for updates
	SuppressRedundant bool             // Suppress redundant updates
	HeartbeatInterval time.Duration    // Heartbeat interval for ON_CHANGE
}

// GNMIExecutor interface for vendor adapters to use gNMI operations
type GNMIExecutor interface {
	// Get retrieves values at the specified paths
	Get(ctx context.Context, paths []string) (map[string]interface{}, error)
	// Set performs a gNMI Set operation
	Set(ctx context.Context, updates map[string]interface{}, deletes []string) error
	// Subscribe starts a telemetry subscription
	Subscribe(ctx context.Context, config *SubscriptionConfig) (Subscription, error)
	// Capabilities returns the device's gNMI capabilities
	Capabilities(ctx context.Context) (*DeviceCapabilities, error)
}

// Subscription represents an active telemetry subscription
type Subscription interface {
	// Stop stops the subscription
	Stop() error
	// Updates returns a channel for receiving updates
	Updates() <-chan []TelemetryUpdate
	// Errors returns a channel for subscription errors
	Errors() <-chan error
}

// DeviceCapabilities contains gNMI capability information
type DeviceCapabilities struct {
	SupportedModels    []ModelInfo
	SupportedEncodings []string
	GNMIVersion        string
}

// ModelInfo describes a supported YANG model
type ModelInfo struct {
	Name         string
	Organization string
	Version      string
}

// Driver implements the types.Driver interface using gNMI/gRPC
type Driver struct {
	config       *types.EquipmentConfig
	conn         *grpc.ClientConn
	gnmiClient   gnmipb.GNMIClient
	capabilities *DeviceCapabilities
	mu           sync.RWMutex

	// Subscription management
	subscriptions map[string]*subscriptionState
	subMu         sync.Mutex
}

// subscriptionState tracks an active subscription
type subscriptionState struct {
	cancel   context.CancelFunc
	updates  chan []TelemetryUpdate
	errors   chan error
	stopped  bool
	stopOnce sync.Once
}

func (s *subscriptionState) Stop() error {
	s.stopOnce.Do(func() {
		s.stopped = true
		s.cancel()
		close(s.updates)
		close(s.errors)
	})
	return nil
}

func (s *subscriptionState) Updates() <-chan []TelemetryUpdate {
	return s.updates
}

func (s *subscriptionState) Errors() <-chan error {
	return s.errors
}

// NewDriver creates a new gNMI driver
func NewDriver(config *types.EquipmentConfig) (types.Driver, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if config.Address == "" {
		return nil, fmt.Errorf("address is required")
	}

	// Default port for gNMI
	if config.Port == 0 {
		config.Port = 9339
	}

	// Default timeout
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &Driver{
		config:        config,
		subscriptions: make(map[string]*subscriptionState),
	}, nil
}

// GetGNMIExecutor returns the GNMIExecutor interface for advanced operations
func (d *Driver) GetGNMIExecutor() GNMIExecutor {
	return d
}

// Connect establishes a gRPC connection to the device
func (d *Driver) Connect(ctx context.Context, config *types.EquipmentConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if config != nil {
		d.config = config
	}

	// Prepare gRPC dial options
	var opts []grpc.DialOption

	// TLS configuration
	if d.config.TLSEnabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: d.config.TLSSkipVerify, //nolint:gosec // User-controlled
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Connection timeout
	opts = append(opts, grpc.WithBlock()) //nolint:staticcheck // supported throughout 1.x

	// Target address
	target := fmt.Sprintf("%s:%d", d.config.Address, d.config.Port)

	// Create context with timeout
	connectCtx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	// Establish connection
	conn, err := grpc.DialContext(connectCtx, target, opts...) //nolint:staticcheck // supported throughout 1.x
	if err != nil {
		return fmt.Errorf("failed to dial %s: %w", target, err)
	}

	d.conn = conn
	d.gnmiClient = gnmipb.NewGNMIClient(conn)

	// Fetch capabilities
	caps, err := d.Capabilities(ctx)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("capabilities check failed: %w", err)
	}
	d.capabilities = caps

	return nil
}

// Disconnect closes the gRPC connection
func (d *Driver) Disconnect(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Stop all subscriptions
	d.subMu.Lock()
	for _, sub := range d.subscriptions {
		_ = sub.Stop()
	}
	d.subscriptions = make(map[string]*subscriptionState)
	d.subMu.Unlock()

	if d.conn != nil {
		err := d.conn.Close()
		d.conn = nil
		d.gnmiClient = nil
		d.capabilities = nil
		return err
	}
	return nil
}

// IsConnected returns true if connected
func (d *Driver) IsConnected() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.conn != nil
}

// Capabilities returns the device's gNMI capabilities
func (d *Driver) Capabilities(ctx context.Context) (*DeviceCapabilities, error) {
	if d.gnmiClient == nil {
		return nil, fmt.Errorf("not connected to device")
	}

	// Add authentication if configured
	ctx = d.addAuthMetadata(ctx)

	capReq := &gnmipb.CapabilityRequest{}
	capCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := d.gnmiClient.Capabilities(capCtx, capReq)
	if err != nil {
		return nil, fmt.Errorf("capabilities request failed: %w", err)
	}

	caps := &DeviceCapabilities{
		GNMIVersion: resp.GNMIVersion,
	}

	// Parse supported models
	for _, model := range resp.SupportedModels {
		caps.SupportedModels = append(caps.SupportedModels, ModelInfo{
			Name:         model.Name,
			Organization: model.Organization,
			Version:      model.Version,
		})
	}

	// Parse supported encodings
	for _, enc := range resp.SupportedEncodings {
		caps.SupportedEncodings = append(caps.SupportedEncodings, enc.String())
	}

	return caps, nil
}

// Get retrieves values at the specified paths
func (d *Driver) Get(ctx context.Context, paths []string) (map[string]interface{}, error) {
	if d.gnmiClient == nil {
		return nil, fmt.Errorf("not connected to device")
	}

	ctx = d.addAuthMetadata(ctx)

	// Build gNMI paths
	gnmiPaths := make([]*gnmipb.Path, len(paths))
	for i, p := range paths {
		gnmiPaths[i] = ParsePath(p)
	}

	getReq := &gnmipb.GetRequest{
		Path:     gnmiPaths,
		Encoding: gnmipb.Encoding_JSON_IETF,
	}

	getCtx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	resp, err := d.gnmiClient.Get(getCtx, getReq)
	if err != nil {
		return nil, fmt.Errorf("gNMI Get failed: %w", err)
	}

	// Parse response into map
	result := make(map[string]interface{})
	for _, notification := range resp.Notification {
		for _, update := range notification.Update {
			path := PathToString(update.Path)
			value := decodeTypedValue(update.Val)
			result[path] = value
		}
	}

	return result, nil
}

// Set performs a gNMI Set operation
func (d *Driver) Set(ctx context.Context, updates map[string]interface{}, deletes []string) error {
	if d.gnmiClient == nil {
		return fmt.Errorf("not connected to device")
	}

	ctx = d.addAuthMetadata(ctx)

	setReq := &gnmipb.SetRequest{}

	// Build updates
	for path, value := range updates {
		gnmiPath := ParsePath(path)
		typedVal, err := encodeTypedValue(value)
		if err != nil {
			return fmt.Errorf("failed to encode value for %s: %w", path, err)
		}
		setReq.Update = append(setReq.Update, &gnmipb.Update{
			Path: gnmiPath,
			Val:  typedVal,
		})
	}

	// Build deletes
	for _, path := range deletes {
		setReq.Delete = append(setReq.Delete, ParsePath(path))
	}

	setCtx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	_, err := d.gnmiClient.Set(setCtx, setReq)
	if err != nil {
		return fmt.Errorf("gNMI Set failed: %w", err)
	}

	return nil
}

// Subscribe starts a telemetry subscription
func (d *Driver) Subscribe(ctx context.Context, config *SubscriptionConfig) (Subscription, error) {
	if d.gnmiClient == nil {
		return nil, fmt.Errorf("not connected to device")
	}

	ctx = d.addAuthMetadata(ctx)

	// Create subscription context
	subCtx, cancel := context.WithCancel(ctx)

	state := &subscriptionState{
		cancel:  cancel,
		updates: make(chan []TelemetryUpdate, 100),
		errors:  make(chan error, 10),
	}

	// Build subscription list
	subs := make([]*gnmipb.Subscription, len(config.Paths))
	for i, path := range config.Paths {
		sub := &gnmipb.Subscription{
			Path:              ParsePath(path),
			SuppressRedundant: config.SuppressRedundant,
		}

		switch config.Mode {
		case SubscriptionModeOnChange:
			sub.Mode = gnmipb.SubscriptionMode_ON_CHANGE
			if config.HeartbeatInterval > 0 {
				sub.HeartbeatInterval = uint64(config.HeartbeatInterval.Nanoseconds())
			}
		case SubscriptionModeSample:
			sub.Mode = gnmipb.SubscriptionMode_SAMPLE
			sub.SampleInterval = uint64(config.SampleInterval.Nanoseconds())
		case SubscriptionModeTargetDefined:
			sub.Mode = gnmipb.SubscriptionMode_TARGET_DEFINED
		}

		subs[i] = sub
	}

	// Create subscribe request
	subReq := &gnmipb.SubscribeRequest{
		Request: &gnmipb.SubscribeRequest_Subscribe{
			Subscribe: &gnmipb.SubscriptionList{
				Subscription: subs,
				Mode:         gnmipb.SubscriptionList_STREAM,
				Encoding:     gnmipb.Encoding_JSON_IETF,
			},
		},
	}

	// Start subscription stream
	stream, err := d.gnmiClient.Subscribe(subCtx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create subscription stream: %w", err)
	}

	// Send subscribe request
	if err := stream.Send(subReq); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to send subscribe request: %w", err)
	}

	// Start goroutine to process updates
	go d.processSubscriptionUpdates(subCtx, stream, state, config.Handler)

	// Track subscription
	d.subMu.Lock()
	subID := fmt.Sprintf("sub-%d", time.Now().UnixNano())
	d.subscriptions[subID] = state
	d.subMu.Unlock()

	return state, nil
}

// processSubscriptionUpdates handles incoming subscription updates
func (d *Driver) processSubscriptionUpdates(
	ctx context.Context,
	stream gnmipb.GNMI_SubscribeClient,
	state *subscriptionState,
	handler TelemetryHandler,
) {
	defer func() {
		if r := recover(); r != nil {
			state.errors <- fmt.Errorf("subscription panic: %v", r)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF || ctx.Err() != nil {
				return
			}
			if !state.stopped {
				state.errors <- fmt.Errorf("subscription error: %w", err)
			}
			return
		}

		// Process response
		switch r := resp.Response.(type) {
		case *gnmipb.SubscribeResponse_Update:
			updates := d.parseNotification(r.Update)
			if len(updates) > 0 {
				// Send to channel
				select {
				case state.updates <- updates:
				default:
					// Channel full, drop oldest
				}

				// Call handler if provided
				if handler != nil {
					handler(updates)
				}
			}

		case *gnmipb.SubscribeResponse_SyncResponse:
			// Initial sync complete
			continue

		case *gnmipb.SubscribeResponse_Error:
			state.errors <- fmt.Errorf("subscription error from device: %s", r.Error.Message) //nolint:staticcheck // deprecated but needed for backwards compat
		}
	}
}

// parseNotification converts a gNMI Notification to TelemetryUpdates
func (d *Driver) parseNotification(notification *gnmipb.Notification) []TelemetryUpdate {
	var updates []TelemetryUpdate

	timestamp := time.Unix(0, notification.Timestamp)

	for _, update := range notification.Update {
		path := PathToString(update.Path)
		value := decodeTypedValue(update.Val)

		updates = append(updates, TelemetryUpdate{
			Path:      path,
			Value:     value,
			Timestamp: timestamp,
			Metadata:  make(map[string]interface{}),
		})
	}

	// Handle deletes
	for _, deletePath := range notification.Delete {
		path := PathToString(deletePath)
		updates = append(updates, TelemetryUpdate{
			Path:      path,
			Value:     nil, // nil indicates deletion
			Timestamp: timestamp,
			Metadata: map[string]interface{}{
				"deleted": true,
			},
		})
	}

	return updates
}

// addAuthMetadata adds authentication to the context
func (d *Driver) addAuthMetadata(ctx context.Context) context.Context {
	if d.config.Username != "" && d.config.Password != "" {
		md := metadata.Pairs(
			"username", d.config.Username,
			"password", d.config.Password,
		)
		return metadata.NewOutgoingContext(ctx, md)
	}
	return ctx
}

// ParsePath converts a string path to gNMI Path
// Supports formats:
//   - /interfaces/interface[name=eth0]/state/counters
//   - interfaces/interface[name=eth0]/state/counters
func ParsePath(path string) *gnmipb.Path {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		return &gnmipb.Path{}
	}

	gnmiPath := &gnmipb.Path{}

	// Split by / but handle keys properly
	var elems []string
	var current strings.Builder
	inKey := false

	for _, c := range path {
		switch c {
		case '[':
			inKey = true
			current.WriteRune(c)
		case ']':
			inKey = false
			current.WriteRune(c)
		case '/':
			if inKey {
				current.WriteRune(c)
			} else {
				if current.Len() > 0 {
					elems = append(elems, current.String())
					current.Reset()
				}
			}
		default:
			current.WriteRune(c)
		}
	}
	if current.Len() > 0 {
		elems = append(elems, current.String())
	}

	// Parse each element
	for _, elem := range elems {
		pathElem := &gnmipb.PathElem{}

		// Check for keys
		if idx := strings.Index(elem, "["); idx != -1 {
			pathElem.Name = elem[:idx]
			pathElem.Key = make(map[string]string)

			// Parse keys
			keyPart := elem[idx:]
			for keyPart != "" {
				// Find key-value pair
				start := strings.Index(keyPart, "[")
				if start == -1 {
					break
				}
				end := strings.Index(keyPart, "]")
				if end == -1 {
					break
				}

				kvPair := keyPart[start+1 : end]
				if eqIdx := strings.Index(kvPair, "="); eqIdx != -1 {
					key := kvPair[:eqIdx]
					value := strings.Trim(kvPair[eqIdx+1:], "'\"")
					pathElem.Key[key] = value
				}

				keyPart = keyPart[end+1:]
			}
		} else {
			pathElem.Name = elem
		}

		gnmiPath.Elem = append(gnmiPath.Elem, pathElem)
	}

	return gnmiPath
}

// PathToString converts a gNMI Path to string format
func PathToString(path *gnmipb.Path) string {
	if path == nil {
		return ""
	}

	var parts []string
	for _, elem := range path.Elem {
		part := elem.Name
		if len(elem.Key) > 0 {
			for k, v := range elem.Key {
				part += fmt.Sprintf("[%s=%s]", k, v)
			}
		}
		parts = append(parts, part)
	}

	return "/" + strings.Join(parts, "/")
}

// decodeTypedValue converts a gNMI TypedValue to Go value
func decodeTypedValue(tv *gnmipb.TypedValue) interface{} {
	if tv == nil {
		return nil
	}

	switch v := tv.Value.(type) {
	case *gnmipb.TypedValue_StringVal:
		return v.StringVal
	case *gnmipb.TypedValue_IntVal:
		return v.IntVal
	case *gnmipb.TypedValue_UintVal:
		return v.UintVal
	case *gnmipb.TypedValue_BoolVal:
		return v.BoolVal
	case *gnmipb.TypedValue_BytesVal:
		return v.BytesVal
	case *gnmipb.TypedValue_FloatVal:
		return v.FloatVal //nolint:staticcheck // deprecated but needed for backwards compat
	case *gnmipb.TypedValue_DoubleVal:
		return v.DoubleVal
	case *gnmipb.TypedValue_DecimalVal:
		return float64(v.DecimalVal.Digits) / float64(int64(1)<<v.DecimalVal.Precision) //nolint:staticcheck // deprecated but needed for backwards compat
	case *gnmipb.TypedValue_LeaflistVal:
		var result []interface{}
		for _, elem := range v.LeaflistVal.Element {
			result = append(result, decodeTypedValue(elem))
		}
		return result
	case *gnmipb.TypedValue_JsonVal:
		var result interface{}
		if err := json.Unmarshal(v.JsonVal, &result); err != nil {
			return string(v.JsonVal)
		}
		return result
	case *gnmipb.TypedValue_JsonIetfVal:
		var result interface{}
		if err := json.Unmarshal(v.JsonIetfVal, &result); err != nil {
			return string(v.JsonIetfVal)
		}
		return result
	case *gnmipb.TypedValue_AsciiVal:
		return v.AsciiVal
	case *gnmipb.TypedValue_ProtoBytes:
		return v.ProtoBytes
	default:
		return nil
	}
}

// encodeTypedValue converts a Go value to gNMI TypedValue
func encodeTypedValue(value interface{}) (*gnmipb.TypedValue, error) {
	switch v := value.(type) {
	case string:
		return &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: v}}, nil
	case int:
		return &gnmipb.TypedValue{Value: &gnmipb.TypedValue_IntVal{IntVal: int64(v)}}, nil
	case int32:
		return &gnmipb.TypedValue{Value: &gnmipb.TypedValue_IntVal{IntVal: int64(v)}}, nil
	case int64:
		return &gnmipb.TypedValue{Value: &gnmipb.TypedValue_IntVal{IntVal: v}}, nil
	case uint:
		return &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: uint64(v)}}, nil
	case uint32:
		return &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: uint64(v)}}, nil
	case uint64:
		return &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: v}}, nil
	case bool:
		return &gnmipb.TypedValue{Value: &gnmipb.TypedValue_BoolVal{BoolVal: v}}, nil
	case float32:
		return &gnmipb.TypedValue{Value: &gnmipb.TypedValue_FloatVal{FloatVal: v}}, nil
	case float64:
		return &gnmipb.TypedValue{Value: &gnmipb.TypedValue_DoubleVal{DoubleVal: v}}, nil
	case []byte:
		return &gnmipb.TypedValue{Value: &gnmipb.TypedValue_BytesVal{BytesVal: v}}, nil
	default:
		// Try JSON encoding for complex types
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("cannot encode value of type %T: %w", value, err)
		}
		return &gnmipb.TypedValue{Value: &gnmipb.TypedValue_JsonIetfVal{JsonIetfVal: jsonBytes}}, nil
	}
}

// CreateSubscriber provisions a subscriber using gNMI Set operation
func (d *Driver) CreateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) (*types.SubscriberResult, error) {
	if !d.IsConnected() {
		return nil, fmt.Errorf("not connected to device")
	}

	// Build subscriber configuration
	interfaceName := fmt.Sprintf("sub-%s", subscriber.Spec.ONUSerial)

	config := map[string]interface{}{
		"name":        interfaceName,
		"type":        "iana-if-type:l2vlan",
		"enabled":     subscriber.Spec.Enabled == nil || *subscriber.Spec.Enabled,
		"description": subscriber.Spec.Description,
	}

	if subscriber.Spec.VLAN > 0 {
		config["vlan-id"] = subscriber.Spec.VLAN
	}

	// QoS configuration
	qosConfig := map[string]interface{}{
		"ingress-bandwidth": tier.Spec.BandwidthUp * 1000000,
		"egress-bandwidth":  tier.Spec.BandwidthDown * 1000000,
		"priority":          tier.Spec.Priority,
	}
	config["qos"] = qosConfig

	// Build path and set
	basePath := fmt.Sprintf("/interfaces/interface[name=%s]/config", interfaceName)
	updates := map[string]interface{}{
		basePath: config,
	}

	if err := d.Set(ctx, updates, nil); err != nil {
		return nil, fmt.Errorf("failed to create subscriber: %w", err)
	}

	result := &types.SubscriberResult{
		SubscriberID:   subscriber.Name,
		SessionID:      fmt.Sprintf("sess-%s", subscriber.Spec.ONUSerial),
		AssignedIP:     subscriber.Spec.IPAddress,
		AssignedIPv6:   subscriber.Spec.IPv6Address,
		AssignedPrefix: subscriber.Spec.DelegatedPrefix,
		InterfaceName:  interfaceName,
		VLAN:           subscriber.Spec.VLAN,
		Metadata:       make(map[string]interface{}),
	}

	return result, nil
}

// UpdateSubscriber updates subscriber configuration using gNMI Set
func (d *Driver) UpdateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) error {
	// Delete and recreate for now
	_ = d.DeleteSubscriber(ctx, subscriber.Name)
	_, err := d.CreateSubscriber(ctx, subscriber, tier)
	return err
}

// DeleteSubscriber removes a subscriber using gNMI Delete operation
func (d *Driver) DeleteSubscriber(ctx context.Context, subscriberID string) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}

	interfacePath := fmt.Sprintf("/interfaces/interface[name=sub-%s]", subscriberID)
	return d.Set(ctx, nil, []string{interfacePath})
}

// SuspendSubscriber suspends a subscriber (set interface admin down)
func (d *Driver) SuspendSubscriber(ctx context.Context, subscriberID string) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}

	enabledPath := fmt.Sprintf("/interfaces/interface[name=sub-%s]/config/enabled", subscriberID)
	return d.Set(ctx, map[string]interface{}{enabledPath: false}, nil)
}

// ResumeSubscriber resumes a suspended subscriber (set interface admin up)
func (d *Driver) ResumeSubscriber(ctx context.Context, subscriberID string) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}

	enabledPath := fmt.Sprintf("/interfaces/interface[name=sub-%s]/config/enabled", subscriberID)
	return d.Set(ctx, map[string]interface{}{enabledPath: true}, nil)
}

// GetSubscriberStatus retrieves subscriber status using gNMI Get
func (d *Driver) GetSubscriberStatus(ctx context.Context, subscriberID string) (*types.SubscriberStatus, error) {
	if !d.IsConnected() {
		return nil, fmt.Errorf("not connected to device")
	}

	statePath := fmt.Sprintf("/interfaces/interface[name=sub-%s]/state", subscriberID)
	result, err := d.Get(ctx, []string{statePath})
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber status: %w", err)
	}

	// Parse response
	status := &types.SubscriberStatus{
		SubscriberID: subscriberID,
		State:        "active",
		IsOnline:     true,
		LastActivity: time.Now(),
		Metadata:     result,
	}

	// Try to extract state from response
	for path, value := range result {
		if strings.HasSuffix(path, "/oper-status") {
			if operStatus, ok := value.(string); ok {
				status.State = operStatus
				status.IsOnline = operStatus == "UP" || operStatus == "up"
			}
		}
		if strings.HasSuffix(path, "/admin-status") {
			if adminStatus, ok := value.(string); ok {
				if adminStatus == "DOWN" || adminStatus == "down" {
					status.State = "suspended"
				}
			}
		}
	}

	return status, nil
}

// GetSubscriberStats retrieves subscriber statistics using gNMI Get
func (d *Driver) GetSubscriberStats(ctx context.Context, subscriberID string) (*types.SubscriberStats, error) {
	if !d.IsConnected() {
		return nil, fmt.Errorf("not connected to device")
	}

	countersPath := fmt.Sprintf("/interfaces/interface[name=sub-%s]/state/counters", subscriberID)
	result, err := d.Get(ctx, []string{countersPath})
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber stats: %w", err)
	}

	stats := &types.SubscriberStats{
		Timestamp: time.Now(),
		Metadata:  result,
	}

	// Parse counters from response
	for path, value := range result {
		switch {
		case strings.HasSuffix(path, "/in-octets"):
			if v, ok := value.(uint64); ok {
				stats.BytesUp = v
			}
		case strings.HasSuffix(path, "/out-octets"):
			if v, ok := value.(uint64); ok {
				stats.BytesDown = v
			}
		case strings.HasSuffix(path, "/in-pkts"):
			if v, ok := value.(uint64); ok {
				stats.PacketsUp = v
			}
		case strings.HasSuffix(path, "/out-pkts"):
			if v, ok := value.(uint64); ok {
				stats.PacketsDown = v
			}
		}
	}

	return stats, nil
}

// HealthCheck performs a health check using gNMI Capabilities request
func (d *Driver) HealthCheck(ctx context.Context) error {
	if !d.IsConnected() {
		return fmt.Errorf("not connected to device")
	}

	_, err := d.Capabilities(ctx)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	return nil
}

// SubscribeToTelemetry is a convenience method for common telemetry subscriptions
func (d *Driver) SubscribeToTelemetry(ctx context.Context, paths []string, interval time.Duration, handler TelemetryHandler) (Subscription, error) {
	config := &SubscriptionConfig{
		Paths:          paths,
		Mode:           SubscriptionModeSample,
		SampleInterval: interval,
		Handler:        handler,
	}
	return d.Subscribe(ctx, config)
}

// SubscribeOnChange is a convenience method for on-change subscriptions
func (d *Driver) SubscribeOnChange(ctx context.Context, paths []string, handler TelemetryHandler) (Subscription, error) {
	config := &SubscriptionConfig{
		Paths:   paths,
		Mode:    SubscriptionModeOnChange,
		Handler: handler,
	}
	return d.Subscribe(ctx, config)
}

// Common telemetry path constants for convenience
const (
	// OpenConfig interface paths
	PathInterfaceState    = "/interfaces/interface[name=%s]/state"
	PathInterfaceCounters = "/interfaces/interface[name=%s]/state/counters"
	PathInterfaceStatus   = "/interfaces/interface[name=%s]/state/oper-status"

	// System paths
	PathSystemState  = "/system/state"
	PathSystemCPU    = "/system/cpus/cpu[index=%d]/state"
	PathSystemMemory = "/system/memory/state"

	// QoS paths
	PathQoSInterface = "/qos/interfaces/interface[interface-id=%s]"
)

// BuildInterfaceCountersPath returns the counters path for an interface
func BuildInterfaceCountersPath(interfaceName string) string {
	return fmt.Sprintf(PathInterfaceCounters, interfaceName)
}

// BuildInterfaceStatusPath returns the oper-status path for an interface
func BuildInterfaceStatusPath(interfaceName string) string {
	return fmt.Sprintf(PathInterfaceStatus, interfaceName)
}
