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
	if p.Description == nil || *p.Description != "4ETH,1POTS,4WiFi" {
		t.Fatalf("expected description, got %v", p.Description)
	}
	if p.TcontNum == nil || *p.TcontNum != 255 {
		t.Fatalf("expected tcont 255, got %v", p.TcontNum)
	}
	if p.GemportNum == nil || *p.GemportNum != 255 {
		t.Fatalf("expected gemport 255, got %v", p.GemportNum)
	}
	if p.SwitchNum == nil || *p.SwitchNum != 255 {
		t.Fatalf("expected switch 255, got %v", p.SwitchNum)
	}
	if p.Ports == nil {
		t.Fatalf("expected ports, got nil")
	}
	if p.Ports.Eth == nil || *p.Ports.Eth != 4 {
		t.Fatalf("expected eth 4, got %v", p.Ports.Eth)
	}
	if p.Ports.Pots == nil || *p.Ports.Pots != 1 {
		t.Fatalf("expected pots 1, got %v", p.Ports.Pots)
	}
	if p.Ports.IPHost == nil || *p.Ports.IPHost != 2 {
		t.Fatalf("expected iphost 2, got %v", p.Ports.IPHost)
	}
	if p.Ports.IPv6Host == nil || *p.Ports.IPv6Host != 0 {
		t.Fatalf("expected ipv6host 0, got %v", p.Ports.IPv6Host)
	}
	if p.Ports.Veip == nil || *p.Ports.Veip != 1 {
		t.Fatalf("expected veip 1, got %v", p.Ports.Veip)
	}
	if p.ServiceAbility == nil || *p.ServiceAbility != "n:1" {
		t.Fatalf("expected service ability n:1, got %v", p.ServiceAbility)
	}
	if p.WifiMngViaNonOMCI == nil || *p.WifiMngViaNonOMCI {
		t.Fatalf("expected wifi mgmt disabled, got %v", p.WifiMngViaNonOMCI)
	}
	if p.OmciSendMode == nil || *p.OmciSendMode != "async" {
		t.Fatalf("expected omci send mode async, got %v", p.OmciSendMode)
	}
	if p.DefaultMulticastRange == nil || *p.DefaultMulticastRange != "none" {
		t.Fatalf("expected default multicast range none, got %v", p.DefaultMulticastRange)
	}
	if p.Committed == nil || !*p.Committed {
		t.Fatalf("expected committed true, got %v", p.Committed)
	}
}

func TestParseONUProfilesMultiple(t *testing.T) {
	raw := "\x1b[31m###############ONU PROFILE###########\x1b[0m\n" +
		"Id: 1\n" +
		"Name: prof1\n" +
		"Max tcont: 8\n" +
		"Max gemport: 32\n" +
		"Max switch per slot: 8\n" +
		"Max eth: 1\n" +
		"Max pots: 0\n" +
		"Max iphost: 2\n" +
		"Max ipv6host: 0\n" +
		"Max veip: 0\n" +
		"Service ability N:1: 1\n" +
		"Wifi mgmt via non OMCI: disable\n" +
		"Omci send mode: async\n" +
		"Default multicast range: none\n" +
		"commit: Yes\n" +
		"--More--\n" +
		"Id: 2\n" +
		"Name: prof2\n" +
		"Max tcont: 1\n" +
		"Max gemport: 1\n" +
		"Max switch per slot: 1\n" +
		"Max eth: 4\n" +
		"Max pots: 2\n" +
		"Max iphost: 2\n" +
		"Max ipv6host: 0\n" +
		"Max veip: 1\n" +
		"Service ability N:1: 1\n" +
		"Wifi mgmt via non OMCI: enable\n" +
		"Omci send mode: sync\n" +
		"Default multicast range: none\n" +
		"commit: No\n"

	profiles, err := parseONUProfiles(raw)
	if err != nil {
		t.Fatalf("parseONUProfiles error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}

	if profiles[0].Name != "prof1" || profiles[1].Name != "prof2" {
		t.Fatalf("unexpected names: %+v", []string{profiles[0].Name, profiles[1].Name})
	}
	if profiles[1].WifiMngViaNonOMCI == nil || !*profiles[1].WifiMngViaNonOMCI {
		t.Fatalf("expected wifi mgmt enabled for prof2")
	}
	if profiles[1].Committed == nil || *profiles[1].Committed {
		t.Fatalf("expected committed false for prof2")
	}
}

func TestParseONUProfilesNotFound(t *testing.T) {
	profiles, err := parseONUProfiles("profile name profX not found!")
	if err != nil {
		t.Fatalf("parseONUProfiles error: %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("expected no profiles, got %d", len(profiles))
	}
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected to contain %q in:\n%s", needle, haystack)
	}
}
