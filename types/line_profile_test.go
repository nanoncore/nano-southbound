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

func TestLineProfileMvlanParse(t *testing.T) {
	mvlan := &LineProfileMvlan{Raw: "200,201-203"}
	if err := mvlan.Validate(); err != nil {
		t.Fatalf("expected mvlan to validate, got %v", err)
	}
	if len(mvlan.VLANs) != 4 {
		t.Fatalf("expected 4 vlans, got %d", len(mvlan.VLANs))
	}
}
