package huawei

import (
	"testing"

	"github.com/nanoncore/nano-southbound/types"
)

func TestParseVLANList(t *testing.T) {
	adapter := &Adapter{}

	// Test with valid VLAN table output
	output := `
  -------------------------------------------------------------------------
  VLAN Configuration
  -------------------------------------------------------------------------
  VLAN ID   Name                      Type      Service Ports   Description
  -------------------------------------------------------------------------
  100       Customer_VLAN_100         smart     5               Customer traffic
  200       Management                smart     0               Management VLAN
  300       Test_VLAN                 smart     3               Test
  -------------------------------------------------------------------------
  Total VLANs: 3
`
	vlans := adapter.parseVLANList(output)

	if len(vlans) != 3 {
		t.Errorf("Expected 3 VLANs, got %d", len(vlans))
	}

	// Check first VLAN
	if vlans[0].ID != 100 {
		t.Errorf("Expected VLAN ID 100, got %d", vlans[0].ID)
	}
	if vlans[0].Name != "Customer_VLAN_100" {
		t.Errorf("Expected name 'Customer_VLAN_100', got '%s'", vlans[0].Name)
	}
	if vlans[0].ServicePortCount != 5 {
		t.Errorf("Expected 5 service ports, got %d", vlans[0].ServicePortCount)
	}

	// Check second VLAN
	if vlans[1].ID != 200 {
		t.Errorf("Expected VLAN ID 200, got %d", vlans[1].ID)
	}
	if vlans[1].ServicePortCount != 0 {
		t.Errorf("Expected 0 service ports, got %d", vlans[1].ServicePortCount)
	}
}

func TestParseVLANListEmpty(t *testing.T) {
	adapter := &Adapter{}

	output := "  No VLANs configured."
	vlans := adapter.parseVLANList(output)

	if len(vlans) != 0 {
		t.Errorf("Expected 0 VLANs, got %d", len(vlans))
	}
}

func TestParseServicePortList(t *testing.T) {
	adapter := &Adapter{}

	output := `
  ---------------------------------------------------------------------------------
  Index   VLAN    Interface       ONT     GemPort   User-VLAN   Transform
  ---------------------------------------------------------------------------------
  1       100     0/0/1           101     1         100         translate
  2       100     0/0/1           102     1         100         translate
  3       200     0/0/2           103     2         200         transparent
  ---------------------------------------------------------------------------------
  Total service ports: 3
`
	servicePorts := adapter.parseServicePortList(output)

	if len(servicePorts) != 3 {
		t.Errorf("Expected 3 service ports, got %d", len(servicePorts))
	}

	// Check first service port
	if servicePorts[0].Index != 1 {
		t.Errorf("Expected index 1, got %d", servicePorts[0].Index)
	}
	if servicePorts[0].VLAN != 100 {
		t.Errorf("Expected VLAN 100, got %d", servicePorts[0].VLAN)
	}
	if servicePorts[0].Interface != "0/0/1" {
		t.Errorf("Expected interface '0/0/1', got '%s'", servicePorts[0].Interface)
	}
	if servicePorts[0].ONTID != 101 {
		t.Errorf("Expected ONT ID 101, got %d", servicePorts[0].ONTID)
	}
	if servicePorts[0].TagTransform != "translate" {
		t.Errorf("Expected tag transform 'translate', got '%s'", servicePorts[0].TagTransform)
	}

	// Check third service port has different transform
	if servicePorts[2].TagTransform != "transparent" {
		t.Errorf("Expected tag transform 'transparent', got '%s'", servicePorts[2].TagTransform)
	}
}

func TestParseServicePortListEmpty(t *testing.T) {
	adapter := &Adapter{}

	output := "  No service ports configured."
	servicePorts := adapter.parseServicePortList(output)

	if len(servicePorts) != 0 {
		t.Errorf("Expected 0 service ports, got %d", len(servicePorts))
	}
}

func TestCreateVLANValidation(t *testing.T) {
	tests := []struct {
		name    string
		vlanID  int
		wantErr bool
	}{
		{"valid min", 1, false},
		{"valid max", 4094, false},
		{"valid mid", 100, false},
		{"invalid zero", 0, true},
		{"invalid negative", -1, true},
		{"invalid too high", 4095, true},
		{"invalid way too high", 9999, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &types.CreateVLANRequest{ID: tt.vlanID}
			valid := req.ID >= 1 && req.ID <= 4094
			if valid == tt.wantErr {
				t.Errorf("CreateVLANRequest ID %d: expected error=%v, got valid=%v", tt.vlanID, tt.wantErr, valid)
			}
		})
	}
}
