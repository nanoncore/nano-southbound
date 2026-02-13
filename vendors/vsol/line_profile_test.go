package vsol

import (
	"strings"
	"testing"

	"github.com/nanoncore/nano-southbound/types"
)

func TestBuildLineProfileCreateCommands(t *testing.T) {
	profile := &types.LineProfile{
		Name: "line_vlan_100",
		Tconts: []*types.LineProfileTcont{
			{
				ID:   1,
				Name: "tcont_1",
				DBA:  "default",
				Gemports: []*types.LineProfileGemport{
					{
						ID:             1,
						Name:           "gemport_1",
						TcontID:        1,
						TrafficLimitUp: "default",
						TrafficLimitDn: "default",
						Services: []*types.LineProfileService{
							{
								Name:      "INTERNET",
								GemportID: 1,
								VLAN:      100,
								COS:       "0-7",
							},
						},
						ServicePorts: []*types.LineProfileServicePort{
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
		Mvlan: &types.LineProfileMvlan{VLANs: []int{200, 201}},
	}

	commands := buildLineProfileCreateCommands(profile)
	joined := strings.Join(commands, "\n")

	assertContains(t, joined, "profile line name line_vlan_100")
	assertContains(t, joined, "tcont 1 name tcont_1 dba default")
	assertContains(t, joined, "gemport 1 tcont 1")
	assertContains(t, joined, "gemport 1 traffic-limit upstream default downstream default")
	assertContains(t, joined, "service INTERNET gemport 1 vlan 100 cos 0-7")
	assertContains(t, joined, "service-port 1 gemport 1 uservlan 100 vlan 100")
	assertContains(t, joined, "mvlan 200,201")
	assertContains(t, joined, "commit")
}

func TestBuildLineProfileCreateCommandsCustomTraffic(t *testing.T) {
	profile := &types.LineProfile{
		Name: "plan_100M",
		Tconts: []*types.LineProfileTcont{
			{
				ID:  1,
				DBA: "plan_100M_up",
				Gemports: []*types.LineProfileGemport{
					{
						ID:             1,
						TcontID:        1,
						TrafficLimitUp: "shape_100M",
						TrafficLimitDn: "shape_200M",
						ServicePorts: []*types.LineProfileServicePort{
							{
								ID:        1,
								GemportID: 1,
								UserVLAN:  702,
								VLAN:      702,
							},
						},
					},
				},
			},
		},
	}

	commands := buildLineProfileCreateCommands(profile)
	joined := strings.Join(commands, "\n")

	assertContains(t, joined, "gemport 1 tcont 1")
	assertContains(t, joined, "gemport 1 traffic-limit upstream shape_100M downstream shape_200M")
	assertContains(t, joined, "service-port 1 gemport 1 uservlan 702 vlan 702")

	// Verify traffic-limit is a separate line from gemport creation
	for i, cmd := range commands {
		if cmd == "gemport 1 tcont 1" {
			if i+1 < len(commands) && commands[i+1] != "gemport 1 traffic-limit upstream shape_100M downstream shape_200M" {
				t.Fatalf("expected traffic-limit command immediately after gemport creation, got %q", commands[i+1])
			}
			break
		}
	}
}

func TestParseLineProfiles(t *testing.T) {
	raw := `
###############LINE PROFILE###########
*****************************
Id: 4
Name: line_vlan_999
  tcont 1 name tcont_1 dba default
    gemport 1 name gemport_1 traffic-limit up-stream default down-stream default
      service-port 1 gemport 1 uservlan 999 vlan 999

*****************************
Id: 7
Name: codex_line_20260205_093830
  tcont 1 name tcont_1 dba default
    gemport 1 name gemport_1 traffic-limit up-stream default down-stream default
      service INTERNET gemport 1 vlan 999 cos 0-7
      service-port 1 gemport 1 uservlan 999 vlan 999
`

	profiles, err := parseLineProfiles(raw)
	if err != nil {
		t.Fatalf("parseLineProfiles error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}

	first := profiles[0]
	if first.Name != "line_vlan_999" {
		t.Fatalf("expected name line_vlan_999, got %q", first.Name)
	}
	if len(first.Tconts) != 1 || first.Tconts[0].ID != 1 {
		t.Fatalf("expected tcont 1, got %+v", first.Tconts)
	}
	if len(first.Tconts[0].Gemports) != 1 || first.Tconts[0].Gemports[0].ID != 1 {
		t.Fatalf("expected gemport 1, got %+v", first.Tconts[0].Gemports)
	}
	if len(first.Tconts[0].Gemports[0].ServicePorts) != 1 {
		t.Fatalf("expected service-port, got %+v", first.Tconts[0].Gemports[0].ServicePorts)
	}

	second := profiles[1]
	if second.Name != "codex_line_20260205_093830" {
		t.Fatalf("expected name codex_line_20260205_093830, got %q", second.Name)
	}
	gem := second.Tconts[0].Gemports[0]
	if len(gem.Services) != 1 || gem.Services[0].Name != "INTERNET" {
		t.Fatalf("expected service INTERNET, got %+v", gem.Services)
	}
}

func TestParseLineProfilesNotFound(t *testing.T) {
	profiles, err := parseLineProfiles("profile name profX not found!")
	if err != nil {
		t.Fatalf("parseLineProfiles error: %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("expected no profiles, got %d", len(profiles))
	}
}

func TestDetectLineProfileCLIErrors(t *testing.T) {
	outputs := []string{
		"gpon-olt-lab(profile-line:9)# gemport 1 tcont 1 name gemport_1\n% Unknown command.",
	}
	if err := detectLineProfileCLIErrors([]string{"gemport 1 tcont 1 name gemport_1"}, outputs); err == nil {
		t.Fatalf("expected error for unknown command output")
	}

	outputs = []string{
		"service INTERNET gemport 1 vlan 999 cos 0-7\nunknown gemport:1.",
	}
	if err := detectLineProfileCLIErrors([]string{"service INTERNET gemport 1 vlan 999 cos 0-7"}, outputs); err == nil {
		t.Fatalf("expected error for unknown gemport output")
	}

	outputs = []string{
		"profile_id:11 create success",
	}
	if err := detectLineProfileCLIErrors([]string{"profile line name foo"}, outputs); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	outputs = []string{
		"profile is already existed.",
	}
	if err := detectLineProfileCLIErrors([]string{"profile line name foo"}, outputs); err == nil {
		t.Fatalf("expected error for already existed output")
	}
}
