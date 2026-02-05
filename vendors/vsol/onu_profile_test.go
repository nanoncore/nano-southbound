package vsol

import (
	"strings"
	"testing"

	"github.com/nanoncore/nano-southbound/types"
)

func TestBuildONUProfileCreateCommands(t *testing.T) {
	desc := "AN5506-04-F1"
	eth := 4
	veip := 1
	tcont := 8
	gem := 32
	ability := "n:1"
	profile := &types.ONUHardwareProfile{
		Name:        "AN5506-04-F1",
		Description: &desc,
		Ports: &types.ONUProfilePorts{
			Eth:  &eth,
			Veip: &veip,
		},
		TcontNum:       &tcont,
		GemportNum:     &gem,
		ServiceAbility: &ability,
	}

	commands := buildONUProfileCreateCommands(profile)
	joined := strings.Join(commands, "\n")

	assertContains(t, joined, "configure terminal")
	assertContains(t, joined, "profile onu name AN5506-04-F1")
	assertContains(t, joined, "port-num eth 4")
	assertContains(t, joined, "port-num veip 1")
	assertContains(t, joined, "tcont-num 8 gemport-num 32")
	assertContains(t, joined, "service-ability n:1")
	assertContains(t, joined, "description \"AN5506-04-F1\"")
	assertContains(t, joined, "commit")
	assertContains(t, joined, "exit")
}

func TestParseONUProfiles(t *testing.T) {
	raw := `
###############ONU PROFILE###########
*************************************
                      Id: 0
                    Name: default
             Description: 4ETH,1POTS,4WiFi
               Max tcont: 255
             Max gemport: 255
     Max switch per slot: 255
                 Max eth: 4
                Max pots: 1
              Max iphost: 2
            Max ipv6host: 0
                Max veip: 1
     Service ability N:1: 1
  Wifi mgmt via non OMCI: disable
          Omci send mode: async
 Default multicast range: none
                  commit: Yes
`
	profiles, err := parseONUProfiles(raw)
	if err != nil {
		t.Fatalf("parseONUProfiles error: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}

	p := profiles[0]
	if p.Name != "default" {
		t.Fatalf("expected name 'default', got %q", p.Name)
	}
	if p.ID == nil || *p.ID != 0 {
		t.Fatalf("expected id 0, got %v", p.ID)
	}
	if p.Ports == nil || p.Ports.Eth == nil || *p.Ports.Eth != 4 {
		t.Fatalf("expected eth 4, got %+v", p.Ports)
	}
	if p.TcontNum == nil || *p.TcontNum != 255 {
		t.Fatalf("expected tcont 255, got %v", p.TcontNum)
	}
	if p.GemportNum == nil || *p.GemportNum != 255 {
		t.Fatalf("expected gemport 255, got %v", p.GemportNum)
	}
	if p.Committed == nil || !*p.Committed {
		t.Fatalf("expected committed true, got %v", p.Committed)
	}
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected to contain %q in:\n%s", needle, haystack)
	}
}
