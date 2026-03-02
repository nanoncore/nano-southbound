package model

import "testing"

func TestGetPrimaryONU_LegacySubscriber(t *testing.T) {
	sub := &Subscriber{
		Name: "legacy-sub",
		Spec: SubscriberSpec{
			ONUSerial: "VSOL12345678",
			VLAN:      100,
		},
	}

	got := sub.GetPrimaryONU()
	if got == nil {
		t.Fatal("GetPrimaryONU() returned nil for legacy subscriber")
	}
	if got.Serial != "VSOL12345678" {
		t.Errorf("Serial = %q, want %q", got.Serial, "VSOL12345678")
	}
	if got.Role != ONUBindingRolePrimary {
		t.Errorf("Role = %q, want %q", got.Role, ONUBindingRolePrimary)
	}
}

func TestGetPrimaryONU_NewStyleSubscriber(t *testing.T) {
	sub := &Subscriber{
		Name: "new-sub",
		Spec: SubscriberSpec{
			ONUSerial: "VSOL12345678", // legacy field still set
			VLAN:      100,
			ONUBindings: []ONUBinding{
				{Serial: "VSOL12345678", PONPort: "0/1", ONUID: 1, Role: ONUBindingRolePrimary},
				{Serial: "VSOL87654321", PONPort: "0/2", ONUID: 3, Role: ONUBindingRoleSecondary},
			},
		},
	}

	got := sub.GetPrimaryONU()
	if got == nil {
		t.Fatal("GetPrimaryONU() returned nil")
	}
	if got.Serial != "VSOL12345678" {
		t.Errorf("Serial = %q, want %q", got.Serial, "VSOL12345678")
	}
	if got.Role != ONUBindingRolePrimary {
		t.Errorf("Role = %q, want %q", got.Role, ONUBindingRolePrimary)
	}
	if got.PONPort != "0/1" {
		t.Errorf("PONPort = %q, want %q", got.PONPort, "0/1")
	}
}

func TestGetPrimaryONU_EmptySubscriber(t *testing.T) {
	sub := &Subscriber{Name: "empty-sub", Spec: SubscriberSpec{VLAN: 100}}

	got := sub.GetPrimaryONU()
	if got != nil {
		t.Errorf("GetPrimaryONU() = %v, want nil for subscriber with no ONU", got)
	}
}

func TestGetONUBySerial(t *testing.T) {
	sub := &Subscriber{
		Spec: SubscriberSpec{
			ONUBindings: []ONUBinding{
				{Serial: "VSOL11111111", PONPort: "0/1", ONUID: 1, Role: ONUBindingRolePrimary},
				{Serial: "VSOL22222222", PONPort: "0/2", ONUID: 2, Role: ONUBindingRoleSecondary},
			},
		},
	}

	tests := []struct {
		name   string
		serial string
		found  bool
	}{
		{"existing primary", "VSOL11111111", true},
		{"existing secondary", "VSOL22222222", true},
		{"non-existent", "VSOL99999999", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sub.GetONUBySerial(tt.serial)
			if tt.found && got == nil {
				t.Errorf("GetONUBySerial(%q) returned nil, expected binding", tt.serial)
			}
			if !tt.found && got != nil {
				t.Errorf("GetONUBySerial(%q) returned %v, expected nil", tt.serial, got)
			}
			if tt.found && got != nil && got.Serial != tt.serial {
				t.Errorf("Serial = %q, want %q", got.Serial, tt.serial)
			}
		})
	}
}

func TestGetONUBySerial_LegacyFallback(t *testing.T) {
	sub := &Subscriber{
		Spec: SubscriberSpec{
			ONUSerial: "VSOL12345678",
		},
	}

	got := sub.GetONUBySerial("VSOL12345678")
	if got == nil {
		t.Fatal("GetONUBySerial() returned nil for legacy serial match")
	}
	if got.Role != ONUBindingRolePrimary {
		t.Errorf("Role = %q, want %q", got.Role, ONUBindingRolePrimary)
	}

	// Should not match a different serial
	got = sub.GetONUBySerial("VSOL99999999")
	if got != nil {
		t.Errorf("GetONUBySerial(wrong serial) = %v, want nil", got)
	}
}

func TestAddONU(t *testing.T) {
	sub := &Subscriber{
		Spec: SubscriberSpec{
			ONUBindings: []ONUBinding{
				{Serial: "VSOL11111111", Role: ONUBindingRolePrimary},
			},
		},
	}

	sub.AddONU(ONUBinding{
		Serial:  "VSOL22222222",
		PONPort: "0/3",
		ONUID:   5,
		Role:    ONUBindingRoleSecondary,
	})

	if len(sub.Spec.ONUBindings) != 2 {
		t.Fatalf("ONUBindings length = %d, want 2", len(sub.Spec.ONUBindings))
	}

	got := sub.GetONUBySerial("VSOL22222222")
	if got == nil {
		t.Fatal("newly added ONU not found by GetONUBySerial")
	}
	if got.PONPort != "0/3" {
		t.Errorf("PONPort = %q, want %q", got.PONPort, "0/3")
	}
	if got.ONUID != 5 {
		t.Errorf("ONUID = %d, want 5", got.ONUID)
	}
}

func TestRemoveONU(t *testing.T) {
	sub := &Subscriber{
		Spec: SubscriberSpec{
			ONUBindings: []ONUBinding{
				{Serial: "VSOL11111111", Role: ONUBindingRolePrimary},
				{Serial: "VSOL22222222", Role: ONUBindingRoleSecondary},
				{Serial: "VSOL33333333", Role: ONUBindingRoleRedundant},
			},
		},
	}

	// Remove middle element
	removed := sub.RemoveONU("VSOL22222222")
	if !removed {
		t.Error("RemoveONU returned false for existing binding")
	}
	if len(sub.Spec.ONUBindings) != 2 {
		t.Fatalf("ONUBindings length = %d, want 2", len(sub.Spec.ONUBindings))
	}
	if sub.GetONUBySerial("VSOL22222222") != nil {
		t.Error("removed ONU still found by GetONUBySerial")
	}

	// Remove non-existent
	removed = sub.RemoveONU("VSOL99999999")
	if removed {
		t.Error("RemoveONU returned true for non-existent binding")
	}
	if len(sub.Spec.ONUBindings) != 2 {
		t.Errorf("ONUBindings length changed after failed remove: got %d", len(sub.Spec.ONUBindings))
	}

	// Remaining bindings intact
	if sub.GetONUBySerial("VSOL11111111") == nil {
		t.Error("primary binding missing after removing secondary")
	}
	if sub.GetONUBySerial("VSOL33333333") == nil {
		t.Error("redundant binding missing after removing secondary")
	}
}

func TestRemoveONU_LastBinding(t *testing.T) {
	sub := &Subscriber{
		Spec: SubscriberSpec{
			ONUBindings: []ONUBinding{
				{Serial: "VSOL11111111", Role: ONUBindingRolePrimary},
			},
		},
	}

	removed := sub.RemoveONU("VSOL11111111")
	if !removed {
		t.Error("RemoveONU returned false for last binding")
	}
	if len(sub.Spec.ONUBindings) != 0 {
		t.Errorf("ONUBindings length = %d, want 0", len(sub.Spec.ONUBindings))
	}
}
