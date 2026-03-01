package testutil

import (
	"context"
	"fmt"
	"sync"

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
	return &types.SubscriberResult{SubscriberID: sub.Name}, nil
}

func (m *MockDriver) UpdateSubscriber(_ context.Context, sub *model.Subscriber, _ *model.ServiceTier) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "UpdateSubscriber:"+sub.Name)
	return nil
}

func (m *MockDriver) DeleteSubscriber(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "DeleteSubscriber:"+id)
	return nil
}

func (m *MockDriver) SuspendSubscriber(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "SuspendSubscriber:"+id)
	return nil
}

func (m *MockDriver) ResumeSubscriber(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "ResumeSubscriber:"+id)
	return nil
}

func (m *MockDriver) GetSubscriberStatus(_ context.Context, id string) (*types.SubscriberStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "GetSubscriberStatus:"+id)
	return &types.SubscriberStatus{SubscriberID: id, State: "active"}, nil
}

func (m *MockDriver) GetSubscriberStats(_ context.Context, id string) (*types.SubscriberStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "GetSubscriberStats:"+id)
	return &types.SubscriberStats{}, nil
}

func (m *MockDriver) HealthCheck(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, "HealthCheck")
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

// Interface compliance checks
var (
	_ types.Driver       = (*MockDriver)(nil)
	_ types.CLIExecutor  = (*MockDriver)(nil)
	_ types.SNMPExecutor = (*MockDriver)(nil)
	_ types.CLIExecutor  = (*MockCLIExecutor)(nil)
	_ types.SNMPExecutor = (*MockSNMPExecutor)(nil)
)
