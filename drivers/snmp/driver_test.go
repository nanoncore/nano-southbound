package snmp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/nanoncore/nano-southbound/types"
)

func TestConvertSNMPValue(t *testing.T) {
	tests := []struct {
		name string
		pdu  gosnmp.SnmpPDU
		want interface{}
	}{
		// OctetString with []byte value
		{
			name: "OctetString with byte slice",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.OctetString, Value: []byte("hello")},
			want: "hello",
		},
		{
			name: "OctetString with empty byte slice",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.OctetString, Value: []byte("")},
			want: "",
		},
		{
			name: "OctetString with unexpected type falls back to Sprintf",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.OctetString, Value: 42},
			want: "42",
		},

		// Integer
		{
			name: "Integer with int value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Integer, Value: int(100)},
			want: int64(100),
		},
		{
			name: "Integer with negative value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Integer, Value: int(-1)},
			want: int64(-1),
		},
		{
			name: "Integer with zero",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Integer, Value: int(0)},
			want: int64(0),
		},
		{
			name: "Integer with unexpected type returns raw value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Integer, Value: "not an int"},
			want: "not an int",
		},

		// Counter32 / Gauge32 / TimeTicks
		{
			name: "Counter32 with uint value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Counter32, Value: uint(12345)},
			want: uint64(12345),
		},
		{
			name: "Gauge32 with uint value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Gauge32, Value: uint(999)},
			want: uint64(999),
		},
		{
			name: "TimeTicks with uint value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.TimeTicks, Value: uint(3600)},
			want: uint64(3600),
		},
		{
			name: "Counter32 with zero",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Counter32, Value: uint(0)},
			want: uint64(0),
		},
		{
			name: "Counter32 with unexpected type returns raw value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Counter32, Value: int64(42)},
			want: int64(42),
		},

		// Counter64
		{
			name: "Counter64 with uint64 value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Counter64, Value: uint64(9999999999)},
			want: uint64(9999999999),
		},
		{
			name: "Counter64 with zero",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Counter64, Value: uint64(0)},
			want: uint64(0),
		},
		{
			name: "Counter64 with unexpected type returns raw value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Counter64, Value: "wrong"},
			want: "wrong",
		},

		// NoSuch / EndOfMibView
		{
			name: "NoSuchObject returns nil",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.NoSuchObject, Value: nil},
			want: nil,
		},
		{
			name: "NoSuchInstance returns nil",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.NoSuchInstance, Value: "some value"},
			want: nil,
		},
		{
			name: "EndOfMibView returns nil",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.EndOfMibView, Value: 123},
			want: nil,
		},

		// Default case
		{
			name: "Unknown type returns raw value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Asn1BER(0xFF), Value: "raw"},
			want: "raw",
		},
		{
			name: "Unknown type with nil value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Asn1BER(0xFF), Value: nil},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSNMPValue(tt.pdu)
			if got != tt.want {
				t.Errorf("convertSNMPValue() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestNewSNMPDriver(t *testing.T) {
	tests := []struct {
		name         string
		config       *types.EquipmentConfig
		wantErr      bool
		errContain   string
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
				Port:    161,
				Timeout: 10 * time.Second,
			},
			wantErr:      false,
			checkPort:    161,
			checkTimeout: 10 * time.Second,
		},
		{
			name: "port 0 defaults to 161",
			config: &types.EquipmentConfig{
				Address: "192.168.1.1",
				Port:    0,
				Timeout: 10 * time.Second,
			},
			wantErr:      false,
			checkPort:    161,
			checkTimeout: 10 * time.Second,
		},
		{
			name: "timeout 0 defaults to 30s",
			config: &types.EquipmentConfig{
				Address: "192.168.1.1",
				Port:    161,
				Timeout: 0,
			},
			wantErr:      false,
			checkPort:    161,
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
				if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
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

			if tt.checkPort != 0 && tt.config.Port != tt.checkPort {
				t.Errorf("expected port %d, got %d", tt.checkPort, tt.config.Port)
			}
			if tt.checkTimeout != 0 && tt.config.Timeout != tt.checkTimeout {
				t.Errorf("expected timeout %v, got %v", tt.checkTimeout, tt.config.Timeout)
			}
		})
	}
}

func TestSNMPIsConnected(t *testing.T) {
	t.Run("new driver is not connected", func(t *testing.T) {
		drv, err := NewDriver(&types.EquipmentConfig{Address: "10.0.0.1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if drv.IsConnected() {
			t.Error("expected IsConnected() to return false for new driver")
		}
	})

	t.Run("driver with snmp field set is connected", func(t *testing.T) {
		d := &Driver{
			config: &types.EquipmentConfig{Address: "10.0.0.1"},
			snmp:   &gosnmp.GoSNMP{},
		}
		if !d.IsConnected() {
			t.Error("expected IsConnected() to return true when snmp is set")
		}
	})
}

func TestSNMPSubscriberOperationsNotSupported(t *testing.T) {
	ctx := context.Background()

	drv, err := NewDriver(&types.EquipmentConfig{Address: "10.0.0.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("CreateSubscriber returns does not support error", func(t *testing.T) {
		_, err := drv.CreateSubscriber(ctx, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "does not support") {
			t.Errorf("error %q does not contain 'does not support'", err.Error())
		}
	})

	t.Run("UpdateSubscriber returns does not support error", func(t *testing.T) {
		err := drv.UpdateSubscriber(ctx, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "does not support") {
			t.Errorf("error %q does not contain 'does not support'", err.Error())
		}
	})

	t.Run("DeleteSubscriber returns does not support error", func(t *testing.T) {
		err := drv.DeleteSubscriber(ctx, "sub-1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "does not support") {
			t.Errorf("error %q does not contain 'does not support'", err.Error())
		}
	})
}

func TestSNMPSuspendResumeNotConnected(t *testing.T) {
	ctx := context.Background()

	drv, err := NewDriver(&types.EquipmentConfig{Address: "10.0.0.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("SuspendSubscriber when not connected", func(t *testing.T) {
		err := drv.SuspendSubscriber(ctx, "sub-1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not connected") {
			t.Errorf("error %q does not contain 'not connected'", err.Error())
		}
	})

	t.Run("ResumeSubscriber when not connected", func(t *testing.T) {
		err := drv.ResumeSubscriber(ctx, "sub-1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not connected") {
			t.Errorf("error %q does not contain 'not connected'", err.Error())
		}
	})
}

func TestSNMPSuspendResumeConnectedVendorSpecific(t *testing.T) {
	ctx := context.Background()

	// Create a "connected" driver by setting the snmp field directly
	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
		snmp:   &gosnmp.GoSNMP{},
	}

	t.Run("SuspendSubscriber when connected returns vendor-specific error", func(t *testing.T) {
		err := d.SuspendSubscriber(ctx, "sub-1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "vendor-specific") {
			t.Errorf("error %q does not contain 'vendor-specific'", err.Error())
		}
	})

	t.Run("ResumeSubscriber when connected returns vendor-specific error", func(t *testing.T) {
		err := d.ResumeSubscriber(ctx, "sub-1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "vendor-specific") {
			t.Errorf("error %q does not contain 'vendor-specific'", err.Error())
		}
	})
}

func TestSNMPGetSubscriberStatusNotConnected(t *testing.T) {
	ctx := context.Background()

	drv, err := NewDriver(&types.EquipmentConfig{Address: "10.0.0.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("GetSubscriberStatus when not connected", func(t *testing.T) {
		_, err := drv.GetSubscriberStatus(ctx, "sub-1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not connected") {
			t.Errorf("error %q does not contain 'not connected'", err.Error())
		}
	})

	t.Run("GetSubscriberStats when not connected", func(t *testing.T) {
		_, err := drv.GetSubscriberStats(ctx, "sub-1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not connected") {
			t.Errorf("error %q does not contain 'not connected'", err.Error())
		}
	})
}

func TestSNMPGetSubscriberStatusConnected(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
		snmp:   &gosnmp.GoSNMP{},
	}

	t.Run("GetSubscriberStatus returns placeholder status", func(t *testing.T) {
		status, err := d.GetSubscriberStatus(ctx, "sub-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status == nil {
			t.Fatal("expected non-nil status")
		}
		if status.SubscriberID != "sub-1" {
			t.Errorf("expected SubscriberID 'sub-1', got %q", status.SubscriberID)
		}
		if status.State != "unknown" {
			t.Errorf("expected State 'unknown', got %q", status.State)
		}
	})

	t.Run("GetSubscriberStats returns placeholder stats", func(t *testing.T) {
		stats, err := d.GetSubscriberStats(ctx, "sub-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats == nil {
			t.Fatal("expected non-nil stats")
		}
		if stats.BytesUp != 0 {
			t.Errorf("expected BytesUp 0, got %d", stats.BytesUp)
		}
		if stats.BytesDown != 0 {
			t.Errorf("expected BytesDown 0, got %d", stats.BytesDown)
		}
	})
}

func TestSNMPGetWalkBulkNotConnected(t *testing.T) {
	ctx := context.Background()

	drv, err := NewDriver(&types.EquipmentConfig{Address: "10.0.0.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snmpDriver := drv.(*Driver)

	t.Run("GetSNMP when not connected", func(t *testing.T) {
		_, err := snmpDriver.GetSNMP(ctx, "1.3.6.1.2.1.1.1.0")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not connected") {
			t.Errorf("error %q does not contain 'not connected'", err.Error())
		}
	})

	t.Run("WalkSNMP when not connected", func(t *testing.T) {
		_, err := snmpDriver.WalkSNMP(ctx, "1.3.6.1.2.1.1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not connected") {
			t.Errorf("error %q does not contain 'not connected'", err.Error())
		}
	})

	t.Run("BulkGetSNMP when not connected", func(t *testing.T) {
		_, err := snmpDriver.BulkGetSNMP(ctx, []string{"1.3.6.1.2.1.1.1.0"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not connected") {
			t.Errorf("error %q does not contain 'not connected'", err.Error())
		}
	})
}

func TestSNMPGetWalkBulkContextCancellation(t *testing.T) {
	// Even with a connected driver, cancelled context should return error first
	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
		snmp:   &gosnmp.GoSNMP{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	t.Run("GetSNMP with cancelled context", func(t *testing.T) {
		_, err := d.GetSNMP(ctx, "1.3.6.1.2.1.1.1.0")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("WalkSNMP with cancelled context", func(t *testing.T) {
		_, err := d.WalkSNMP(ctx, "1.3.6.1.2.1.1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("BulkGetSNMP with cancelled context", func(t *testing.T) {
		_, err := d.BulkGetSNMP(ctx, []string{"1.3.6.1.2.1.1.1.0"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

func TestSNMPHealthCheckNotConnected(t *testing.T) {
	ctx := context.Background()

	drv, err := NewDriver(&types.EquipmentConfig{Address: "10.0.0.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = drv.HealthCheck(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Disconnect when already disconnected / snmp is nil
// ---------------------------------------------------------------------------

func TestSNMPDisconnectWhenAlreadyDisconnected(t *testing.T) {
	tests := []struct {
		name   string
		driver *Driver
	}{
		{
			name: "snmp is nil (never connected)",
			driver: &Driver{
				config: &types.EquipmentConfig{Address: "10.0.0.1"},
				snmp:   nil,
			},
		},
		{
			name: "snmp set but Conn is nil",
			driver: &Driver{
				config: &types.EquipmentConfig{Address: "10.0.0.1"},
				snmp:   &gosnmp.GoSNMP{Conn: nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.driver.Disconnect(context.Background())
			if err != nil {
				t.Errorf("Disconnect() returned error %v, want nil", err)
			}
			if tt.driver.snmp != nil {
				t.Error("snmp should be nil after Disconnect")
			}
			// Calling Disconnect again should also be safe
			err = tt.driver.Disconnect(context.Background())
			if err != nil {
				t.Errorf("second Disconnect() returned error %v, want nil", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WalkSNMP when snmp.Conn is nil (connected but Conn closed)
// ---------------------------------------------------------------------------

func TestWalkSNMPConnNil(t *testing.T) {
	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
		snmp:   &gosnmp.GoSNMP{Conn: nil},
	}

	ctx := context.Background()
	_, err := d.WalkSNMP(ctx, "1.3.6.1.2.1.1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "SNMP connection is closed") {
		t.Errorf("error %q does not contain 'SNMP connection is closed'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Additional convertSNMPValue coverage
// ---------------------------------------------------------------------------

func TestConvertSNMPValueAdditional(t *testing.T) {
	tests := []struct {
		name string
		pdu  gosnmp.SnmpPDU
		want interface{}
	}{
		{
			name: "Gauge32 with unexpected type returns raw value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Gauge32, Value: "not a uint"},
			want: "not a uint",
		},
		{
			name: "TimeTicks with unexpected type returns raw value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.TimeTicks, Value: float64(1.5)},
			want: float64(1.5),
		},
		{
			name: "OctetString with nil value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.OctetString, Value: nil},
			want: "<nil>",
		},
		{
			name: "Counter32 with max uint value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Counter32, Value: uint(4294967295)},
			want: uint64(4294967295),
		},
		{
			name: "Gauge32 with zero uint value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Gauge32, Value: uint(0)},
			want: uint64(0),
		},
		{
			name: "TimeTicks with zero uint value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.TimeTicks, Value: uint(0)},
			want: uint64(0),
		},
		{
			name: "Integer with max int value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Integer, Value: int(2147483647)},
			want: int64(2147483647),
		},
		{
			name: "Counter64 with max uint64",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Counter64, Value: uint64(18446744073709551615)},
			want: uint64(18446744073709551615),
		},
		{
			name: "OctetString with string value falls to Sprintf",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.OctetString, Value: "a string"},
			want: "a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSNMPValue(tt.pdu)
			if got != tt.want {
				t.Errorf("convertSNMPValue() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// getSNMPValue when not connected
// ---------------------------------------------------------------------------

func TestGetSNMPValueNotConnected(t *testing.T) {
	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
		snmp:   nil,
	}

	_, err := d.getSNMPValue("1.3.6.1.2.1.1.1.0")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// SNMPExecutor interface compliance
// ---------------------------------------------------------------------------

func TestDriverImplementsSNMPExecutor(t *testing.T) {
	d := &Driver{config: &types.EquipmentConfig{Address: "10.0.0.1"}}
	var _ types.SNMPExecutor = d
}

// ---------------------------------------------------------------------------
// GetSubscriberStatus connected returns correct fields
// ---------------------------------------------------------------------------

func TestSNMPGetSubscriberStatusConnectedFields(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
		snmp:   &gosnmp.GoSNMP{},
	}

	status, err := d.GetSubscriberStatus(ctx, "subscriber-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.SubscriberID != "subscriber-123" {
		t.Errorf("SubscriberID = %q, want %q", status.SubscriberID, "subscriber-123")
	}
	if status.State != "unknown" {
		t.Errorf("State = %q, want %q", status.State, "unknown")
	}
	if status.IsOnline {
		t.Error("IsOnline = true, want false")
	}
	if status.LastActivity.IsZero() {
		t.Error("LastActivity should not be zero")
	}
	if status.Metadata == nil {
		t.Error("Metadata should not be nil")
	}
}

// ---------------------------------------------------------------------------
// GetSubscriberStats connected returns correct fields
// ---------------------------------------------------------------------------

func TestSNMPGetSubscriberStatsConnectedFields(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
		snmp:   &gosnmp.GoSNMP{},
	}

	stats, err := d.GetSubscriberStats(ctx, "subscriber-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.BytesUp != 0 {
		t.Errorf("BytesUp = %d, want 0", stats.BytesUp)
	}
	if stats.BytesDown != 0 {
		t.Errorf("BytesDown = %d, want 0", stats.BytesDown)
	}
	if stats.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if stats.Metadata == nil {
		t.Error("Metadata should not be nil")
	}
}
