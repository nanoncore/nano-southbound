package types

import "testing"

func TestLineProfileValidate(t *testing.T) {
	profile := &LineProfile{
		Name: "line_vlan_100",
		Tconts: []*LineProfileTcont{
			{
				ID:   1,
				Name: "tcont_1",
				DBA:  "default",
				Gemports: []*LineProfileGemport{
					{
						ID:      1,
						Name:    "gemport_1",
						TcontID: 1,
						Services: []*LineProfileService{
							{
								Name:      "INTERNET",
								GemportID: 1,
								VLAN:      100,
								COS:       "0-7",
							},
						},
						ServicePorts: []*LineProfileServicePort{
							{
								ID:        1,
								GemportID: 1,
								UserVLAN:  100,
								VLAN:      100,
							},
						},
					},
				},
			},
		},
		Mvlan: &LineProfileMvlan{
			VLANs: []int{200, 201},
		},
	}

	if err := profile.Validate(); err != nil {
		t.Fatalf("expected profile to validate, got %v", err)
	}
}

func TestLineProfileValidateMissingName(t *testing.T) {
	if err := (&LineProfile{}).Validate(); err == nil {
		t.Fatalf("expected validation error for missing name")
	}
}

func TestLineProfileValidateNilTcont(t *testing.T) {
	profile := &LineProfile{
		Name:   "line_vlan_100",
		Tconts: []*LineProfileTcont{nil},
	}
	if err := profile.Validate(); err == nil {
		t.Fatalf("expected validation error for nil tcont")
	}
}

func TestLineProfileValidateInvalidCOS(t *testing.T) {
	profile := &LineProfile{
		Name: "line_vlan_100",
		Tconts: []*LineProfileTcont{
			{
				ID: 1,
				Gemports: []*LineProfileGemport{
					{
						ID:      1,
						TcontID: 1,
						Services: []*LineProfileService{
							{
								Name:      "INTERNET",
								GemportID: 1,
								VLAN:      100,
								COS:       "foo",
							},
						},
					},
				},
			},
		},
	}
	if err := profile.Validate(); err == nil {
		t.Fatalf("expected validation error for invalid cos")
	}
}

func TestLineProfileMvlanParse(t *testing.T) {
	mvlan := &LineProfileMvlan{Raw: "200,201-203"}
	if err := mvlan.Validate(); err != nil {
		t.Fatalf("expected mvlan to validate, got %v", err)
	}
	if len(mvlan.VLANs) != 4 {
		t.Fatalf("expected 4 vlans, got %d", len(mvlan.VLANs))
	}
}

func TestLineProfileValidateNilProfile(t *testing.T) {
	var p *LineProfile
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for nil line profile")
	}
}

func TestLineProfileGemportValidate(t *testing.T) {
	tcontIDs := map[int]struct{}{1: {}}

	t.Run("nil gemport", func(t *testing.T) {
		var g *LineProfileGemport
		if err := g.Validate(tcontIDs); err == nil {
			t.Fatal("expected error for nil gemport")
		}
	})

	t.Run("gemport id out of range low", func(t *testing.T) {
		g := &LineProfileGemport{ID: 0, TcontID: 1}
		if err := g.Validate(tcontIDs); err == nil {
			t.Fatal("expected error for gemport id 0")
		}
	})

	t.Run("gemport id out of range high", func(t *testing.T) {
		g := &LineProfileGemport{ID: 256, TcontID: 1}
		if err := g.Validate(tcontIDs); err == nil {
			t.Fatal("expected error for gemport id 256")
		}
	})

	t.Run("missing tcont id", func(t *testing.T) {
		g := &LineProfileGemport{ID: 1, TcontID: 0}
		if err := g.Validate(tcontIDs); err == nil {
			t.Fatal("expected error for missing tcont id")
		}
	})

	t.Run("tcont id not in profile", func(t *testing.T) {
		g := &LineProfileGemport{ID: 1, TcontID: 99}
		if err := g.Validate(tcontIDs); err == nil {
			t.Fatal("expected error for tcont id not in profile")
		}
	})

	t.Run("valid gemport", func(t *testing.T) {
		g := &LineProfileGemport{ID: 1, TcontID: 1}
		if err := g.Validate(tcontIDs); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("valid gemport with empty tcontIDs map", func(t *testing.T) {
		g := &LineProfileGemport{ID: 1, TcontID: 5}
		if err := g.Validate(map[int]struct{}{}); err != nil {
			t.Fatalf("expected no error with empty tcontIDs, got %v", err)
		}
	})
}

func TestLineProfileServicePortValidate(t *testing.T) {
	t.Run("nil service port", func(t *testing.T) {
		var sp *LineProfileServicePort
		if err := sp.Validate(); err == nil {
			t.Fatal("expected error for nil service port")
		}
	})

	t.Run("service port id out of range low", func(t *testing.T) {
		sp := &LineProfileServicePort{ID: 0, GemportID: 1}
		if err := sp.Validate(); err == nil {
			t.Fatal("expected error for service port id 0")
		}
	})

	t.Run("service port id out of range high", func(t *testing.T) {
		sp := &LineProfileServicePort{ID: 129, GemportID: 1}
		if err := sp.Validate(); err == nil {
			t.Fatal("expected error for service port id 129")
		}
	})

	t.Run("service port gemport id out of range", func(t *testing.T) {
		sp := &LineProfileServicePort{ID: 1, GemportID: 0}
		if err := sp.Validate(); err == nil {
			t.Fatal("expected error for gemport id 0")
		}
	})

	t.Run("service port user vlan out of range", func(t *testing.T) {
		sp := &LineProfileServicePort{ID: 1, GemportID: 1, UserVLAN: 4095}
		if err := sp.Validate(); err == nil {
			t.Fatal("expected error for user vlan 4095")
		}
	})

	t.Run("service port vlan out of range", func(t *testing.T) {
		sp := &LineProfileServicePort{ID: 1, GemportID: 1, VLAN: 4095}
		if err := sp.Validate(); err == nil {
			t.Fatal("expected error for vlan 4095")
		}
	})

	t.Run("valid service port", func(t *testing.T) {
		sp := &LineProfileServicePort{ID: 1, GemportID: 1, UserVLAN: 100, VLAN: 200}
		if err := sp.Validate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("service port zero vlans are ok", func(t *testing.T) {
		sp := &LineProfileServicePort{ID: 1, GemportID: 1}
		if err := sp.Validate(); err != nil {
			t.Fatalf("expected no error for zero vlans, got %v", err)
		}
	})
}

func TestLineProfileServiceValidate(t *testing.T) {
	t.Run("nil service", func(t *testing.T) {
		var s *LineProfileService
		if err := s.Validate(); err == nil {
			t.Fatal("expected error for nil service")
		}
	})

	t.Run("missing service name", func(t *testing.T) {
		s := &LineProfileService{GemportID: 1}
		if err := s.Validate(); err == nil {
			t.Fatal("expected error for missing service name")
		}
	})

	t.Run("invalid gemport id", func(t *testing.T) {
		s := &LineProfileService{Name: "INTERNET", GemportID: 0}
		if err := s.Validate(); err == nil {
			t.Fatal("expected error for gemport id 0")
		}
	})

	t.Run("invalid vlan", func(t *testing.T) {
		s := &LineProfileService{Name: "INTERNET", GemportID: 1, VLAN: 4095}
		if err := s.Validate(); err == nil {
			t.Fatal("expected error for vlan 4095")
		}
	})

	t.Run("valid service with zero vlan", func(t *testing.T) {
		s := &LineProfileService{Name: "INTERNET", GemportID: 1}
		if err := s.Validate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("valid service with vlan and cos", func(t *testing.T) {
		s := &LineProfileService{Name: "INTERNET", GemportID: 1, VLAN: 100, COS: "0-7"}
		if err := s.Validate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}

func TestLineProfileMvlanValidateEdgeCases(t *testing.T) {
	t.Run("nil mvlan is valid", func(t *testing.T) {
		var m *LineProfileMvlan
		if err := m.Validate(); err != nil {
			t.Fatalf("expected no error for nil mvlan, got %v", err)
		}
	})

	t.Run("mvlan with out of range vlan", func(t *testing.T) {
		m := &LineProfileMvlan{VLANs: []int{4095}}
		if err := m.Validate(); err == nil {
			t.Fatal("expected error for vlan 4095")
		}
	})

	t.Run("mvlan with zero vlan", func(t *testing.T) {
		m := &LineProfileMvlan{VLANs: []int{0}}
		if err := m.Validate(); err == nil {
			t.Fatal("expected error for vlan 0")
		}
	})

	t.Run("mvlan with valid vlans", func(t *testing.T) {
		m := &LineProfileMvlan{VLANs: []int{100, 200, 4094}}
		if err := m.Validate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("mvlan raw with invalid range", func(t *testing.T) {
		m := &LineProfileMvlan{Raw: "200-100"}
		if err := m.Validate(); err == nil {
			t.Fatal("expected error for reversed range")
		}
	})

	t.Run("mvlan raw with non-numeric value", func(t *testing.T) {
		m := &LineProfileMvlan{Raw: "abc"}
		if err := m.Validate(); err == nil {
			t.Fatal("expected error for non-numeric mvlan")
		}
	})

	t.Run("mvlan empty raw and empty vlans is valid", func(t *testing.T) {
		m := &LineProfileMvlan{}
		if err := m.Validate(); err != nil {
			t.Fatalf("expected no error for empty mvlan, got %v", err)
		}
	})
}

func TestValidateCOS(t *testing.T) {
	tests := []struct {
		name    string
		cos     string
		wantErr bool
	}{
		{"empty string is valid", "", false},
		{"single digit valid", "0", false},
		{"range valid", "0-7", false},
		{"single digit 7", "7", false},
		{"invalid format", "foo", true},
		{"range with spaces", "0 -7", true},
		{"multiple ranges", "0-3,4-7", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCOS(tt.cos)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCOS(%q) error = %v, wantErr %v", tt.cos, err, tt.wantErr)
			}
		})
	}
}

func TestLineProfileTcontIDValidation(t *testing.T) {
	t.Run("tcont id 0 is invalid", func(t *testing.T) {
		profile := &LineProfile{
			Name:   "test",
			Tconts: []*LineProfileTcont{{ID: 0}},
		}
		if err := profile.Validate(); err == nil {
			t.Fatal("expected error for tcont id 0")
		}
	})

	t.Run("tcont id 256 is invalid", func(t *testing.T) {
		profile := &LineProfile{
			Name:   "test",
			Tconts: []*LineProfileTcont{{ID: 256}},
		}
		if err := profile.Validate(); err == nil {
			t.Fatal("expected error for tcont id 256")
		}
	})

	t.Run("tcont id 255 is valid", func(t *testing.T) {
		profile := &LineProfile{
			Name:   "test",
			Tconts: []*LineProfileTcont{{ID: 255}},
		}
		if err := profile.Validate(); err != nil {
			t.Fatalf("expected no error for tcont id 255, got %v", err)
		}
	})
}
