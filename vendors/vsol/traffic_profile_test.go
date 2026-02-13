package vsol

import (
	"testing"

	"github.com/nanoncore/nano-southbound/types"
)

func TestParseTrafficProfiles(t *testing.T) {
	output := `
###############TRAFFIC PROFILE###########
*****************************
Id:   1
Name: default
sir:  0 Kbps
pir:  1024000 Kbps

*****************************
Id:   3
Name: nano_traffic_50000
sir:  0 Kbps
pir:  50000 Kbps

`
	profiles, err := parseTrafficProfiles(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}

	// Check first profile
	if profiles[0].ID != 1 || profiles[0].Name != "default" {
		t.Errorf("profile 0: got ID=%d Name=%q, want ID=1 Name=default", profiles[0].ID, profiles[0].Name)
	}
	if profiles[0].SIR != 0 || profiles[0].PIR != 1024000 {
		t.Errorf("profile 0: got SIR=%d PIR=%d, want SIR=0 PIR=1024000", profiles[0].SIR, profiles[0].PIR)
	}

	// Check second profile
	if profiles[1].ID != 3 || profiles[1].Name != "nano_traffic_50000" {
		t.Errorf("profile 1: got ID=%d Name=%q, want ID=3 Name=nano_traffic_50000", profiles[1].ID, profiles[1].Name)
	}
	if profiles[1].SIR != 0 || profiles[1].PIR != 50000 {
		t.Errorf("profile 1: got SIR=%d PIR=%d, want SIR=0 PIR=50000", profiles[1].SIR, profiles[1].PIR)
	}
}

func TestParseTrafficProfilesEmpty(t *testing.T) {
	output := `
###############TRAFFIC PROFILE###########
`
	profiles, err := parseTrafficProfiles(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("expected 0 profiles, got %d", len(profiles))
	}
}

func TestParseTrafficProfilesWithSIR(t *testing.T) {
	output := `
###############TRAFFIC PROFILE###########
*****************************
Id:   1
Name: default
sir:  0 Kbps
pir:  1024000 Kbps

*****************************
Id:   2
Name: guaranteed_50m
sir:  50000 Kbps
pir:  100000 Kbps

`
	profiles, err := parseTrafficProfiles(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}
	if profiles[1].SIR != 50000 || profiles[1].PIR != 100000 {
		t.Errorf("profile 1: got SIR=%d PIR=%d, want SIR=50000 PIR=100000", profiles[1].SIR, profiles[1].PIR)
	}
}

func TestBuildTrafficProfileCreateCommands(t *testing.T) {
	profile := types.TrafficProfile{
		Name: "nano_traffic_100000",
		SIR:  0,
		PIR:  100000,
	}
	got := buildTrafficProfileCreateCommands(5, profile)
	want := []string{
		"configure terminal",
		"profile traffic id 5 name nano_traffic_100000",
		"sir 0 pir 100000",
		"commit",
		"exit",
		"exit",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d commands, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("command[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
