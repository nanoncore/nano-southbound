//go:build integration
// +build integration

package vsol

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nanoncore/nano-southbound/drivers/cli"
	"github.com/nanoncore/nano-southbound/types"
)

// TestDiscoverONUs_Integration tests the full discovery flow against a real OLT simulator.
// Run with: go test -tags=integration -v ./vendors/vsol/... -run TestDiscoverONUs_Integration
func TestDiscoverONUs_Integration(t *testing.T) {
	// Skip if not running integration tests
	host := os.Getenv("OLT_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("OLT_SSH_PORT")
	if port == "" {
		port = "2222"
	}

	config := &types.EquipmentConfig{
		Name:     "test-vsol-olt",
		Type:     types.EquipmentTypeOLT,
		Vendor:   types.VendorVSOL,
		Address:  host,
		Port:     2222,
		Protocol: types.ProtocolCLI,
		Username: "admin",
		Password: "admin",
		Timeout:  30 * time.Second,
		Metadata: map[string]string{
			"model": "v1600g",
		},
	}

	// Create CLI base driver and wrap with V-SOL adapter
	baseDriver, err := cli.NewDriver(config)
	if err != nil {
		t.Fatalf("failed to create base driver: %v", err)
	}
	driver := NewAdapter(baseDriver, config)
	ctx := context.Background()

	// Connect
	if err := driver.Connect(ctx, config); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer driver.Disconnect(ctx)

	// Cast to DriverV2 for access to DiscoverONUs
	driverV2, ok := driver.(types.DriverV2)
	if !ok {
		t.Fatal("adapter does not implement DriverV2")
	}

	// Test DiscoverONUs for all ports
	t.Run("discover_all_ports", func(t *testing.T) {
		discoveries, err := driverV2.DiscoverONUs(ctx, nil)
		if err != nil {
			t.Fatalf("DiscoverONUs failed: %v", err)
		}

		t.Logf("Found %d unprovisioned ONUs", len(discoveries))
		for i, d := range discoveries {
			t.Logf("  [%d] Port: %s, Serial: %s, State: %s", i, d.PONPort, d.Serial, d.State)
		}

		// Based on simulator seed data, we expect multiple ONUs
		if len(discoveries) == 0 {
			t.Error("expected some unprovisioned ONUs, got 0")
		}
	})

	// Test DiscoverONUs for specific port
	t.Run("discover_specific_port", func(t *testing.T) {
		discoveries, err := driverV2.DiscoverONUs(ctx, []string{"0/1"})
		if err != nil {
			t.Fatalf("DiscoverONUs failed: %v", err)
		}

		t.Logf("Found %d unprovisioned ONUs on port 0/1", len(discoveries))
		for _, d := range discoveries {
			if d.PONPort != "0/1" {
				t.Errorf("expected PONPort 0/1, got %s", d.PONPort)
			}
			t.Logf("  Serial: %s, State: %s", d.Serial, d.State)
		}
	})

	// Test that discoveries have proper PON port format
	t.Run("verify_pon_port_format", func(t *testing.T) {
		discoveries, err := driverV2.DiscoverONUs(ctx, nil)
		if err != nil {
			t.Fatalf("DiscoverONUs failed: %v", err)
		}

		for _, d := range discoveries {
			// PON port should be in format "0/X" (converted from "1/1/X:Y")
			if len(d.PONPort) < 3 || d.PONPort[0] != '0' || d.PONPort[1] != '/' {
				t.Errorf("invalid PONPort format: %s (expected 0/X)", d.PONPort)
			}
		}
	})
}
