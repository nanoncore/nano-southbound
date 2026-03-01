package southbound

import (
	"testing"

	"github.com/nanoncore/nano-southbound/testutil"
)

func TestNewDriver(t *testing.T) {
	tests := []struct {
		name      string
		vendor    Vendor
		protocol  Protocol
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "VSOL with default protocol uses CLI",
			vendor:   VendorVSOL,
			protocol: "",
			wantErr:  false,
		},
		{
			name:     "VSOL with explicit CLI",
			vendor:   VendorVSOL,
			protocol: ProtocolCLI,
			wantErr:  false,
		},
		{
			name:      "VSOL with unsupported GNMI",
			vendor:    VendorVSOL,
			protocol:  ProtocolGNMI,
			wantErr:   true,
			errSubstr: "does not support protocol",
		},
		{
			name:      "invalid vendor",
			vendor:    "unknown",
			protocol:  "",
			wantErr:   true,
			errSubstr: "unsupported vendor",
		},
		{
			name:     "Mock vendor creates mock driver",
			vendor:   VendorMock,
			protocol: "",
			wantErr:  false,
		},
		{
			name:     "Mock vendor with explicit SNMP still creates mock driver",
			vendor:   VendorMock,
			protocol: ProtocolSNMP,
			wantErr:  false,
		},
		{
			name:     "Huawei with default protocol",
			vendor:   VendorHuawei,
			protocol: "",
			wantErr:  false,
		},
		{
			name:     "Nokia with default protocol (NETCONF)",
			vendor:   VendorNokia,
			protocol: "",
			wantErr:  false,
		},
		{
			name:     "Cisco with default protocol (NETCONF)",
			vendor:   VendorCisco,
			protocol: "",
			wantErr:  false,
		},
		{
			name:     "VSOL with SNMP",
			vendor:   VendorVSOL,
			protocol: ProtocolSNMP,
			wantErr:  false,
		},
		{
			name:     "Nokia with CLI",
			vendor:   VendorNokia,
			protocol: ProtocolCLI,
			wantErr:  false,
		},
		{
			name:      "ZTE with unsupported GNMI",
			vendor:    VendorZTE,
			protocol:  ProtocolGNMI,
			wantErr:   true,
			errSubstr: "does not support protocol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := testutil.NewTestEquipmentConfig(tt.vendor, "10.0.0.1")
			driver, err := NewDriver(tt.vendor, tt.protocol, config)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" {
					if !containsSubstring(err.Error(), tt.errSubstr) {
						t.Fatalf("expected error containing %q, got %q", tt.errSubstr, err.Error())
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if driver == nil {
				t.Fatal("expected non-nil driver")
			}
		})
	}
}

func TestNewDriverAllVendorsReturnNonNil(t *testing.T) {
	vendors := []Vendor{
		VendorVSOL,
		VendorHuawei,
		VendorNokia,
		VendorCisco,
		VendorZTE,
		VendorJuniper,
		VendorAdtran,
		VendorCalix,
		VendorDZS,
		VendorFiberHome,
		VendorEricsson,
		VendorCData,
		VendorMock,
	}

	for _, vendor := range vendors {
		t.Run(string(vendor), func(t *testing.T) {
			config := testutil.NewTestEquipmentConfig(vendor, "10.0.0.1")
			driver, err := NewDriver(vendor, "", config)
			if err != nil {
				t.Fatalf("expected no error for vendor %s, got %v", vendor, err)
			}
			if driver == nil {
				t.Fatalf("expected non-nil driver for vendor %s", vendor)
			}
		})
	}
}

func TestGetSupportedVendors(t *testing.T) {
	vendors := GetSupportedVendors()

	if len(vendors) == 0 {
		t.Fatal("expected non-empty vendor list")
	}

	knownVendors := []Vendor{VendorVSOL, VendorHuawei, VendorNokia, VendorCisco, VendorMock}
	for _, known := range knownVendors {
		found := false
		for _, v := range vendors {
			if v == known {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected vendor %s in list, not found", known)
		}
	}
}

func TestGetVendorCapabilities(t *testing.T) {
	t.Run("valid vendor returns capabilities", func(t *testing.T) {
		caps, ok := GetVendorCapabilities(VendorVSOL)
		if !ok {
			t.Fatal("expected ok=true for VendorVSOL")
		}
		if caps.PrimaryProtocol != ProtocolCLI {
			t.Errorf("expected PrimaryProtocol CLI, got %s", caps.PrimaryProtocol)
		}
		if len(caps.SupportedProtocols) == 0 {
			t.Error("expected non-empty SupportedProtocols")
		}
	})

	t.Run("invalid vendor returns false", func(t *testing.T) {
		_, ok := GetVendorCapabilities("nonexistent")
		if ok {
			t.Fatal("expected ok=false for nonexistent vendor")
		}
	})

	t.Run("Nokia capabilities", func(t *testing.T) {
		caps, ok := GetVendorCapabilities(VendorNokia)
		if !ok {
			t.Fatal("expected ok=true for VendorNokia")
		}
		if caps.PrimaryProtocol != ProtocolNETCONF {
			t.Errorf("expected PrimaryProtocol NETCONF, got %s", caps.PrimaryProtocol)
		}
		if !caps.SupportsStreaming {
			t.Error("expected Nokia to support streaming")
		}
	})
}

func TestCapabilityMatrixContainsAllVendors(t *testing.T) {
	expectedVendors := []Vendor{
		VendorNokia,
		VendorHuawei,
		VendorZTE,
		VendorCisco,
		VendorJuniper,
		VendorAdtran,
		VendorCalix,
		VendorDZS,
		VendorFiberHome,
		VendorEricsson,
		VendorVSOL,
		VendorCData,
		VendorMock,
	}

	for _, vendor := range expectedVendors {
		t.Run(string(vendor), func(t *testing.T) {
			caps, ok := CapabilityMatrix[vendor]
			if !ok {
				t.Fatalf("CapabilityMatrix missing entry for vendor %s", vendor)
			}
			if caps.PrimaryProtocol == "" {
				t.Error("PrimaryProtocol should not be empty")
			}
			if len(caps.SupportedProtocols) == 0 {
				t.Error("SupportedProtocols should not be empty")
			}
			if caps.ConfigMethod == "" {
				t.Error("ConfigMethod should not be empty")
			}
			if caps.TelemetryMethod == "" {
				t.Error("TelemetryMethod should not be empty")
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
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
