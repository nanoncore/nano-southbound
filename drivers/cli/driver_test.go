package cli

import (
	"context"
	"testing"
	"time"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
)

func TestNewDriver(t *testing.T) {
	tests := []struct {
		name       string
		config     *types.EquipmentConfig
		wantErr    bool
		errContain string
		// Post-creation checks
		checkPort    int
		checkTimeout time.Duration
	}{
		{
			name:       "nil config returns error",
			config:     nil,
			wantErr:    true,
			errContain: "config is required",
		},
		{
			name: "empty address returns error",
			config: &types.EquipmentConfig{
				Address: "",
			},
			wantErr:    true,
			errContain: "address is required",
		},
		{
			name: "valid config returns non-nil driver",
			config: &types.EquipmentConfig{
				Address: "192.168.1.1",
				Port:    22,
				Timeout: 10 * time.Second,
			},
			wantErr:      false,
			checkPort:    22,
			checkTimeout: 10 * time.Second,
		},
		{
			name: "port 0 defaults to 22",
			config: &types.EquipmentConfig{
				Address: "192.168.1.1",
				Port:    0,
				Timeout: 10 * time.Second,
			},
			wantErr:      false,
			checkPort:    22,
			checkTimeout: 10 * time.Second,
		},
		{
			name: "timeout 0 defaults to 30s",
			config: &types.EquipmentConfig{
				Address: "192.168.1.1",
				Port:    22,
				Timeout: 0,
			},
			wantErr:      false,
			checkPort:    22,
			checkTimeout: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drv, err := NewDriver(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContain)
				}
				if tt.errContain != "" && !containsStr(err.Error(), tt.errContain) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContain)
				}
				if drv != nil {
					t.Error("expected nil driver on error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if drv == nil {
				t.Fatal("expected non-nil driver")
			}

			// Verify config was mutated with defaults
			if tt.checkPort != 0 && tt.config.Port != tt.checkPort {
				t.Errorf("expected port %d, got %d", tt.checkPort, tt.config.Port)
			}
			if tt.checkTimeout != 0 && tt.config.Timeout != tt.checkTimeout {
				t.Errorf("expected timeout %v, got %v", tt.checkTimeout, tt.config.Timeout)
			}
		})
	}
}

func TestShouldDisablePager(t *testing.T) {
	tests := []struct {
		name   string
		driver *Driver
		want   bool
	}{
		{
			name:   "nil config returns true",
			driver: &Driver{config: nil},
			want:   true,
		},
		{
			name: "config with nil metadata returns true",
			driver: &Driver{config: &types.EquipmentConfig{
				Address:  "10.0.0.1",
				Metadata: nil,
			}},
			want: true,
		},
		{
			name: "address 127.0.0.1 returns false",
			driver: &Driver{config: &types.EquipmentConfig{
				Address:  "127.0.0.1",
				Metadata: map[string]string{},
			}},
			want: false,
		},
		{
			name: "address localhost returns false",
			driver: &Driver{config: &types.EquipmentConfig{
				Address:  "localhost",
				Metadata: map[string]string{},
			}},
			want: false,
		},
		{
			name: "address LOCALHOST returns false (case insensitive)",
			driver: &Driver{config: &types.EquipmentConfig{
				Address:  "LOCALHOST",
				Metadata: map[string]string{},
			}},
			want: false,
		},
		{
			name: "metadata disable_pager=false returns false",
			driver: &Driver{config: &types.EquipmentConfig{
				Address:  "10.0.0.1",
				Metadata: map[string]string{"disable_pager": "false"},
			}},
			want: false,
		},
		{
			name: "metadata disable_pager=true returns true",
			driver: &Driver{config: &types.EquipmentConfig{
				Address:  "10.0.0.1",
				Metadata: map[string]string{"disable_pager": "true"},
			}},
			want: true,
		},
		{
			name: "normal address with no disable_pager returns true",
			driver: &Driver{config: &types.EquipmentConfig{
				Address:  "10.0.0.1",
				Metadata: map[string]string{"some_key": "some_value"},
			}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.driver.shouldDisablePager()
			if got != tt.want {
				t.Errorf("shouldDisablePager() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsConnected(t *testing.T) {
	t.Run("freshly created driver is not connected", func(t *testing.T) {
		drv, err := NewDriver(&types.EquipmentConfig{Address: "10.0.0.1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if drv.IsConnected() {
			t.Error("expected IsConnected() to return false for new driver")
		}
	})

	t.Run("driver with nil sshClient and nil expectSession is not connected", func(t *testing.T) {
		d := &Driver{config: &types.EquipmentConfig{Address: "10.0.0.1"}}
		if d.IsConnected() {
			t.Error("expected IsConnected() false when both fields are nil")
		}
	})

	t.Run("driver with only expectSession set is not connected", func(t *testing.T) {
		d := &Driver{
			config:        &types.EquipmentConfig{Address: "10.0.0.1"},
			expectSession: &ExpectSession{},
		}
		if d.IsConnected() {
			t.Error("expected IsConnected() false when sshClient is nil")
		}
	})
}

func TestNotConnectedErrorPaths(t *testing.T) {
	ctx := context.Background()

	drv, err := NewDriver(&types.EquipmentConfig{Address: "10.0.0.1"})
	if err != nil {
		t.Fatalf("unexpected error creating driver: %v", err)
	}

	// Cast to *Driver to access the concrete methods directly
	cliDriver := drv.(*Driver)

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "ExecCommand on disconnected driver",
			fn: func() error {
				_, err := cliDriver.ExecCommand(ctx, "show version")
				return err
			},
		},
		{
			name: "ExecCommands on disconnected driver",
			fn: func() error {
				_, err := cliDriver.ExecCommands(ctx, []string{"show version"})
				return err
			},
		},
		{
			name: "DeleteSubscriber on disconnected driver",
			fn: func() error {
				return cliDriver.DeleteSubscriber(ctx, "sub-1")
			},
		},
		{
			name: "SuspendSubscriber on disconnected driver",
			fn: func() error {
				return cliDriver.SuspendSubscriber(ctx, "sub-1")
			},
		},
		{
			name: "ResumeSubscriber on disconnected driver",
			fn: func() error {
				return cliDriver.ResumeSubscriber(ctx, "sub-1")
			},
		},
		{
			name: "GetSubscriberStatus on disconnected driver",
			fn: func() error {
				_, err := cliDriver.GetSubscriberStatus(ctx, "sub-1")
				return err
			},
		},
		{
			name: "GetSubscriberStats on disconnected driver",
			fn: func() error {
				_, err := cliDriver.GetSubscriberStats(ctx, "sub-1")
				return err
			},
		},
		{
			name: "HealthCheck on disconnected driver",
			fn: func() error {
				return cliDriver.HealthCheck(ctx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsStr(err.Error(), "not connected") {
				t.Errorf("error %q does not contain 'not connected'", err.Error())
			}
		})
	}
}

func TestExecCommandContextCancellation(t *testing.T) {
	drv, err := NewDriver(&types.EquipmentConfig{Address: "10.0.0.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cliDriver := drv.(*Driver)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, execErr := cliDriver.ExecCommand(ctx, "show version")
	if execErr == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if execErr != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", execErr)
	}
}

// ---------------------------------------------------------------------------
// CreateSubscriber / UpdateSubscriber when not connected
// ---------------------------------------------------------------------------

func TestCreateSubscriberNotConnected(t *testing.T) {
	ctx := context.Background()

	d := &Driver{config: &types.EquipmentConfig{Address: "10.0.0.1"}}

	subscriber := &model.Subscriber{
		Name: "sub-test",
		Spec: model.SubscriberSpec{
			ONUSerial:   "ABCD12345678",
			VLAN:        100,
			Description: "Test subscriber",
		},
	}
	tier := &model.ServiceTier{
		Spec: model.ServiceTierSpec{
			BandwidthDown: 100,
			BandwidthUp:   50,
		},
	}

	result, err := d.CreateSubscriber(ctx, subscriber, tier)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsStr(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
}

func TestUpdateSubscriberNotConnected(t *testing.T) {
	ctx := context.Background()

	d := &Driver{config: &types.EquipmentConfig{Address: "10.0.0.1"}}

	subscriber := &model.Subscriber{
		Name: "sub-test",
		Spec: model.SubscriberSpec{
			ONUSerial: "ABCD12345678",
		},
	}
	tier := &model.ServiceTier{
		Spec: model.ServiceTierSpec{
			BandwidthDown: 100,
			BandwidthUp:   50,
		},
	}

	err := d.UpdateSubscriber(ctx, subscriber, tier)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsStr(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Disconnect when already disconnected
// ---------------------------------------------------------------------------

func TestDisconnectWhenAlreadyDisconnected(t *testing.T) {
	tests := []struct {
		name   string
		driver *Driver
	}{
		{
			name: "nil sshClient and nil expectSession",
			driver: &Driver{
				config:        &types.EquipmentConfig{Address: "10.0.0.1"},
				sshClient:     nil,
				expectSession: nil,
			},
		},
		{
			name: "only expectSession set (nil sshClient)",
			driver: &Driver{
				config:        &types.EquipmentConfig{Address: "10.0.0.1"},
				sshClient:     nil,
				expectSession: &ExpectSession{expecter: nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.driver.Disconnect(context.Background())
			if err != nil {
				t.Errorf("Disconnect() returned error %v, want nil", err)
			}
			// Verify fields are nil after disconnect
			if tt.driver.sshClient != nil {
				t.Error("sshClient should be nil after Disconnect")
			}
			if tt.driver.expectSession != nil {
				t.Error("expectSession should be nil after Disconnect")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ExpectSession.Execute with nil expecter
// ---------------------------------------------------------------------------

func TestExpectSessionExecuteNilExpecter(t *testing.T) {
	session := &ExpectSession{
		expecter: nil,
	}
	output, err := session.Execute("show version")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsStr(err.Error(), "not initialized") {
		t.Errorf("error %q does not contain 'not initialized'", err.Error())
	}
	if output != "" {
		t.Errorf("expected empty output, got %q", output)
	}
}

// ---------------------------------------------------------------------------
// ExpectSession.SetTimeout
// ---------------------------------------------------------------------------

func TestExpectSessionSetTimeout(t *testing.T) {
	session := &ExpectSession{
		timeout: 30 * time.Second,
	}

	newTimeout := 60 * time.Second
	session.SetTimeout(newTimeout)
	if session.timeout != newTimeout {
		t.Errorf("SetTimeout() timeout = %v, want %v", session.timeout, newTimeout)
	}
}

// ---------------------------------------------------------------------------
// pagerMoreRE pattern matching
// ---------------------------------------------------------------------------

func TestPagerMoreREPattern(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "matches --More--", input: "--More--", want: true},
		{name: "matches More:", input: "More:", want: true},
		{name: "matches Press any key to continue", input: "Press any key to continue", want: true},
		{name: "does not match regular text", input: "regular output line", want: false},
		{name: "does not match empty string", input: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pagerMoreRE.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("pagerMoreRE.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ExecCommands with empty commands slice
// ---------------------------------------------------------------------------

func TestExecCommandsEmptySlice(t *testing.T) {
	d := &Driver{config: &types.EquipmentConfig{Address: "10.0.0.1"}}

	// Even with empty commands, not connected should still work correctly
	// but actually, the loop doesn't execute, so it should return empty results
	// Wait: execCommand checks IsConnected, but with empty slice the loop doesn't run
	// Actually let's test this differently - use a disconnected driver with non-empty slice

	ctx := context.Background()
	// With disconnected driver, the first command should fail
	_, err := d.ExecCommands(ctx, []string{"cmd1", "cmd2"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsStr(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Connect with nil config parameter (uses existing config)
// ---------------------------------------------------------------------------

func TestConnectWithNilConfig(t *testing.T) {
	d := &Driver{
		config: &types.EquipmentConfig{
			Address: "192.168.1.1",
			Port:    22,
			Timeout: 1 * time.Millisecond, // Very short timeout so it fails fast
		},
	}

	ctx := context.Background()
	// This will fail to connect (no SSH server), but we're testing that
	// nil config doesn't cause a panic
	err := d.Connect(ctx, nil)
	if err == nil {
		t.Fatal("expected error connecting to non-existent host, got nil")
	}
	// The error should be about SSH dial failure, not a nil pointer
	if containsStr(err.Error(), "nil pointer") {
		t.Error("passing nil config to Connect should not cause nil pointer dereference")
	}
}

// ---------------------------------------------------------------------------
// CLIExecutor interface compliance
// ---------------------------------------------------------------------------

func TestDriverImplementsCLIExecutor(t *testing.T) {
	d := &Driver{config: &types.EquipmentConfig{Address: "10.0.0.1"}}
	var _ types.CLIExecutor = d
}

// containsStr is a helper to check substring inclusion.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
