package testutil

import (
	"context"
	"fmt"
	"sync"

	"github.com/nanoncore/nano-southbound/drivers/netconf"
	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
)

// MockCLIExecutor is a reusable mock for types.CLIExecutor.
// Supports command→output map, command→error map, sequential outputs, and command recording.
type MockCLIExecutor struct {
	mu sync.Mutex

	// Outputs maps command strings to their output.
	Outputs map[string]string

	// Errors maps command strings to errors they should return.
	Errors map[string]error

	// SequentialOutputs is used when the same command should return
	// different outputs on subsequent calls. Takes priority over Outputs.
	SequentialOutputs map[string][]string
	seqIndex          map[string]int

	// Commands records all commands that were executed.
	Commands []string
}

func (m *MockCLIExecutor) ExecCommand(_ context.Context, command string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Commands = append(m.Commands, command)

	if m.Errors != nil {
		if err, ok := m.Errors[command]; ok {
			out := ""
			if m.Outputs != nil {
				out = m.Outputs[command]
			}
			return out, err
		}
	}

	if m.SequentialOutputs != nil {
		if seq, ok := m.SequentialOutputs[command]; ok {
			if m.seqIndex == nil {
				m.seqIndex = make(map[string]int)
			}
			idx := m.seqIndex[command]
			if idx < len(seq) {
				m.seqIndex[command] = idx + 1
				return seq[idx], nil
			}
			// Exhausted sequential outputs, fall through to Outputs
		}
	}

	if m.Outputs != nil {
		if out, ok := m.Outputs[command]; ok {
			return out, nil
		}
	}

	return "", nil
}

func (m *MockCLIExecutor) ExecCommands(ctx context.Context, commands []string) ([]string, error) {
	results := make([]string, 0, len(commands))
	for _, cmd := range commands {
		out, err := m.ExecCommand(ctx, cmd)
		if err != nil {
			return results, err
		}
		results = append(results, out)
	}
	return results, nil
}

// MockSNMPExecutor is a reusable mock for types.SNMPExecutor.
type MockSNMPExecutor struct {
	mu sync.Mutex

	// GetResults maps OID to return value.
	GetResults map[string]interface{}

	// GetErrors maps OID to error.
	GetErrors map[string]error

	// WalkResults maps base OID to walk results.
	WalkResults map[string]map[string]interface{}

	// WalkErrors maps base OID to error.
	WalkErrors map[string]error

	// BulkGetResults is the return value for BulkGetSNMP.
	BulkGetResults map[string]interface{}

	// BulkGetError is the error for BulkGetSNMP.
	BulkGetError error

	// Calls records all method calls for verification.
	Calls []string
}

func (m *MockSNMPExecutor) GetSNMP(_ context.Context, oid string) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Calls = append(m.Calls, "GetSNMP:"+oid)

	if m.GetErrors != nil {
		if err, ok := m.GetErrors[oid]; ok {
			return nil, err
		}
	}

	if m.GetResults != nil {
		if val, ok := m.GetResults[oid]; ok {
			return val, nil
		}
	}

	return nil, fmt.Errorf("no result for OID %s", oid)
}

func (m *MockSNMPExecutor) WalkSNMP(_ context.Context, oid string) (map[string]interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Calls = append(m.Calls, "WalkSNMP:"+oid)

	if m.WalkErrors != nil {
		if err, ok := m.WalkErrors[oid]; ok {
			return nil, err
		}
	}

	if m.WalkResults != nil {
		if values, ok := m.WalkResults[oid]; ok {
			return values, nil
		}
	}

	return map[string]interface{}{}, nil
}

func (m *MockSNMPExecutor) BulkGetSNMP(_ context.Context, oids []string) (map[string]interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Calls = append(m.Calls, fmt.Sprintf("BulkGetSNMP:%v", oids))

	if m.BulkGetError != nil {
		return nil, m.BulkGetError
	}

	if m.BulkGetResults != nil {
		return m.BulkGetResults, nil
	}

	return map[string]interface{}{}, nil
}

// MockDriver implements types.Driver for testing.
type MockDriver struct {
	mu sync.Mutex

	// Connected tracks connection state.
	Connected bool

	// ConnectError is returned by Connect if set.
	ConnectError error

	// DisconnectError is returned by Disconnect if set.
	DisconnectError error

	// CLIExec is an embedded CLI executor (optional).
	CLIExec *MockCLIExecutor

	// SNMPExec is an embedded SNMP executor (optional).
	SNMPExec *MockSNMPExecutor

	// NETCONFExec is an embedded NETCONF executor (optional).
	NETCONFExec *MockNETCONFExecutor

	// CreateSubscriberResult overrides the default return from CreateSubscriber when set.
	CreateSubscriberResult *types.SubscriberResult

	// CreateSubscriberError is returned by CreateSubscriber if set.
	CreateSubscriberError error

	// UpdateSubscriberError is returned by UpdateSubscriber if set.
	UpdateSubscriberError error

	// DeleteSubscriberError is returned by DeleteSubscriber if set.
	DeleteSubscriberError error

	// SuspendSubscriberError is returned by SuspendSubscriber if set.
	SuspendSubscriberError error

	// ResumeSubscriberError is returned by ResumeSubscriber if set.
	ResumeSubscriberError error

	// GetSubscriberStatusResult overrides the default return.
	GetSubscriberStatusResult *types.SubscriberStatus

	// GetSubscriberStatusError is returned if set.
	GetSubscriberStatusError error

	// GetSubscriberStatsResult overrides the default return.
	GetSubscriberStatsResult *types.SubscriberStats

	// GetSubscriberStatsError is returned if set.
	GetSubscriberStatsError error

	// HealthCheckError is returned by HealthCheck if set.
	HealthCheckError error

	// Calls records method calls.
	Calls []string
}

func (m *MockDriver) Connect(_ context.Context, _ *types.EquipmentConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "Connect")
	if m.ConnectError != nil {
		return m.ConnectError
	}
	m.Connected = true
	return nil
}

func (m *MockDriver) Disconnect(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "Disconnect")
	if m.DisconnectError != nil {
		return m.DisconnectError
	}
	m.Connected = false
	return nil
}

func (m *MockDriver) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Connected
}

func (m *MockDriver) CreateSubscriber(_ context.Context, sub *model.Subscriber, _ *model.ServiceTier) (*types.SubscriberResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "CreateSubscriber:"+sub.Name)
	if m.CreateSubscriberError != nil {
		return nil, m.CreateSubscriberError
	}
	if m.CreateSubscriberResult != nil {
		return m.CreateSubscriberResult, nil
	}
	return &types.SubscriberResult{SubscriberID: sub.Name}, nil
}

func (m *MockDriver) UpdateSubscriber(_ context.Context, sub *model.Subscriber, _ *model.ServiceTier) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "UpdateSubscriber:"+sub.Name)
	return m.UpdateSubscriberError
}

func (m *MockDriver) DeleteSubscriber(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "DeleteSubscriber:"+id)
	return m.DeleteSubscriberError
}

func (m *MockDriver) SuspendSubscriber(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "SuspendSubscriber:"+id)
	return m.SuspendSubscriberError
}

func (m *MockDriver) ResumeSubscriber(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "ResumeSubscriber:"+id)
	return m.ResumeSubscriberError
}

func (m *MockDriver) GetSubscriberStatus(_ context.Context, id string) (*types.SubscriberStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "GetSubscriberStatus:"+id)
	if m.GetSubscriberStatusError != nil {
		return nil, m.GetSubscriberStatusError
	}
	if m.GetSubscriberStatusResult != nil {
		return m.GetSubscriberStatusResult, nil
	}
	return &types.SubscriberStatus{SubscriberID: id, State: "active"}, nil
}

func (m *MockDriver) GetSubscriberStats(_ context.Context, id string) (*types.SubscriberStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "GetSubscriberStats:"+id)
	if m.GetSubscriberStatsError != nil {
		return nil, m.GetSubscriberStatsError
	}
	if m.GetSubscriberStatsResult != nil {
		return m.GetSubscriberStatsResult, nil
	}
	return &types.SubscriberStats{}, nil
}

func (m *MockDriver) HealthCheck(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "HealthCheck")
	if m.HealthCheckError != nil {
		return m.HealthCheckError
	}
	if !m.Connected {
		return fmt.Errorf("not connected")
	}
	return nil
}

// ExecCommand delegates to CLIExec if available (implements CLIExecutor).
func (m *MockDriver) ExecCommand(ctx context.Context, command string) (string, error) {
	if m.CLIExec != nil {
		return m.CLIExec.ExecCommand(ctx, command)
	}
	return "", fmt.Errorf("CLI executor not available")
}

// ExecCommands delegates to CLIExec if available (implements CLIExecutor).
func (m *MockDriver) ExecCommands(ctx context.Context, commands []string) ([]string, error) {
	if m.CLIExec != nil {
		return m.CLIExec.ExecCommands(ctx, commands)
	}
	return nil, fmt.Errorf("CLI executor not available")
}

// GetSNMP delegates to SNMPExec if available (implements SNMPExecutor).
func (m *MockDriver) GetSNMP(ctx context.Context, oid string) (interface{}, error) {
	if m.SNMPExec != nil {
		return m.SNMPExec.GetSNMP(ctx, oid)
	}
	return nil, fmt.Errorf("SNMP executor not available")
}

// WalkSNMP delegates to SNMPExec if available (implements SNMPExecutor).
func (m *MockDriver) WalkSNMP(ctx context.Context, oid string) (map[string]interface{}, error) {
	if m.SNMPExec != nil {
		return m.SNMPExec.WalkSNMP(ctx, oid)
	}
	return nil, fmt.Errorf("SNMP executor not available")
}

// BulkGetSNMP delegates to SNMPExec if available (implements SNMPExecutor).
func (m *MockDriver) BulkGetSNMP(ctx context.Context, oids []string) (map[string]interface{}, error) {
	if m.SNMPExec != nil {
		return m.SNMPExec.BulkGetSNMP(ctx, oids)
	}
	return nil, fmt.Errorf("SNMP executor not available")
}

// MockNETCONFExecutor is a reusable mock for netconf.NETCONFExecutor.
// Supports configurable responses for Get/GetConfig/EditConfig/RPC and command recording.
type MockNETCONFExecutor struct {
	mu sync.Mutex

	// GetResponses maps filter string to response bytes.
	GetResponses map[string][]byte

	// GetErrors maps filter string to error.
	GetErrors map[string]error

	// GetConfigResponses maps "source|filter" to response bytes.
	GetConfigResponses map[string][]byte

	// GetConfigErrors maps "source|filter" to error.
	GetConfigErrors map[string]error

	// EditConfigError is returned by EditConfig if set.
	EditConfigError error

	// RPCResponses maps operation string to response bytes.
	RPCResponses map[string][]byte

	// RPCErrors maps operation string to error.
	RPCErrors map[string]error

	// CommitError is returned by Commit if set.
	CommitError error

	// LockError is returned by Lock if set.
	LockError error

	// UnlockError is returned by Unlock if set.
	UnlockError error

	// Capabilities is the list of capabilities returned by GetCapabilities.
	Capabilities []string

	// Calls records all method calls for verification.
	Calls []string
}

func (m *MockNETCONFExecutor) RPC(_ context.Context, operation string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "RPC:"+operation)
	if m.RPCErrors != nil {
		if err, ok := m.RPCErrors[operation]; ok {
			return nil, err
		}
	}
	if m.RPCResponses != nil {
		if resp, ok := m.RPCResponses[operation]; ok {
			return resp, nil
		}
	}
	return []byte("<ok/>"), nil
}

func (m *MockNETCONFExecutor) Get(_ context.Context, filter string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "Get:"+filter)
	if m.GetErrors != nil {
		if err, ok := m.GetErrors[filter]; ok {
			return nil, err
		}
	}
	if m.GetResponses != nil {
		if resp, ok := m.GetResponses[filter]; ok {
			return resp, nil
		}
	}
	return []byte("<data/>"), nil
}

func (m *MockNETCONFExecutor) GetConfig(_ context.Context, source, filter string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := source + "|" + filter
	m.Calls = append(m.Calls, "GetConfig:"+key)
	if m.GetConfigErrors != nil {
		if err, ok := m.GetConfigErrors[key]; ok {
			return nil, err
		}
	}
	if m.GetConfigResponses != nil {
		if resp, ok := m.GetConfigResponses[key]; ok {
			return resp, nil
		}
	}
	return []byte("<data/>"), nil
}

func (m *MockNETCONFExecutor) EditConfig(_ context.Context, _, _ string, _ ...netconf.EditOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "EditConfig")
	return m.EditConfigError
}

func (m *MockNETCONFExecutor) Commit(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "Commit")
	return m.CommitError
}

func (m *MockNETCONFExecutor) Lock(_ context.Context, target string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "Lock:"+target)
	return m.LockError
}

func (m *MockNETCONFExecutor) Unlock(_ context.Context, target string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "Unlock:"+target)
	return m.UnlockError
}

func (m *MockNETCONFExecutor) HasCapability(cap string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

func (m *MockNETCONFExecutor) GetCapabilities() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Capabilities
}

// NETCONFExec is an embedded NETCONF executor (optional) for MockDriver.
// When set, MockDriver delegates NETCONF methods to this executor.
func (m *MockDriver) RPC(ctx context.Context, operation string) ([]byte, error) {
	if m.NETCONFExec != nil {
		return m.NETCONFExec.RPC(ctx, operation)
	}
	return nil, fmt.Errorf("NETCONF executor not available")
}

func (m *MockDriver) Get(ctx context.Context, filter string) ([]byte, error) {
	if m.NETCONFExec != nil {
		return m.NETCONFExec.Get(ctx, filter)
	}
	return nil, fmt.Errorf("NETCONF executor not available")
}

func (m *MockDriver) GetConfig(ctx context.Context, source, filter string) ([]byte, error) {
	if m.NETCONFExec != nil {
		return m.NETCONFExec.GetConfig(ctx, source, filter)
	}
	return nil, fmt.Errorf("NETCONF executor not available")
}

func (m *MockDriver) EditConfig(ctx context.Context, target, config string, options ...netconf.EditOption) error {
	if m.NETCONFExec != nil {
		return m.NETCONFExec.EditConfig(ctx, target, config, options...)
	}
	return fmt.Errorf("NETCONF executor not available")
}

func (m *MockDriver) Commit(ctx context.Context) error {
	if m.NETCONFExec != nil {
		return m.NETCONFExec.Commit(ctx)
	}
	return fmt.Errorf("NETCONF executor not available")
}

func (m *MockDriver) Lock(ctx context.Context, target string) error {
	if m.NETCONFExec != nil {
		return m.NETCONFExec.Lock(ctx, target)
	}
	return fmt.Errorf("NETCONF executor not available")
}

func (m *MockDriver) Unlock(ctx context.Context, target string) error {
	if m.NETCONFExec != nil {
		return m.NETCONFExec.Unlock(ctx, target)
	}
	return fmt.Errorf("NETCONF executor not available")
}

func (m *MockDriver) HasCapability(cap string) bool {
	if m.NETCONFExec != nil {
		return m.NETCONFExec.HasCapability(cap)
	}
	return false
}

func (m *MockDriver) GetCapabilities() []string {
	if m.NETCONFExec != nil {
		return m.NETCONFExec.GetCapabilities()
	}
	return nil
}

// Interface compliance checks
var (
	_ types.Driver          = (*MockDriver)(nil)
	_ types.CLIExecutor     = (*MockDriver)(nil)
	_ types.SNMPExecutor    = (*MockDriver)(nil)
	_ netconf.NETCONFExecutor = (*MockDriver)(nil)
	_ types.CLIExecutor     = (*MockCLIExecutor)(nil)
	_ types.SNMPExecutor    = (*MockSNMPExecutor)(nil)
	_ netconf.NETCONFExecutor = (*MockNETCONFExecutor)(nil)
)
