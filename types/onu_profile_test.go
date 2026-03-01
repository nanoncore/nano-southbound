package types

import "testing"

func TestONUHardwareProfileValidate(t *testing.T) {
	t.Run("nil profile", func(t *testing.T) {
		var p *ONUHardwareProfile
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for nil profile")
		}
	})

	t.Run("missing name", func(t *testing.T) {
		p := &ONUHardwareProfile{}
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for missing name")
		}
	})

	t.Run("description length", func(t *testing.T) {
		desc := makeString(65)
		p := &ONUHardwareProfile{Name: "test", Description: &desc}
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for long description")
		}
	})

	t.Run("tcont without gemport", func(t *testing.T) {
		tcont := 1
		p := &ONUHardwareProfile{Name: "test", TcontNum: &tcont}
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for tcont without gemport")
		}
	})

	t.Run("gemport without tcont", func(t *testing.T) {
		gem := 1
		p := &ONUHardwareProfile{Name: "test", GemportNum: &gem}
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for gemport without tcont")
		}
	})

	t.Run("invalid switch num", func(t *testing.T) {
		switchNum := 0
		p := &ONUHardwareProfile{Name: "test", SwitchNum: &switchNum}
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for invalid switch num")
		}
	})

	t.Run("valid profile", func(t *testing.T) {
		desc := "ok"
		eth := 4
		tcont := 8
		gem := 32
		p := &ONUHardwareProfile{
			Name:        "test",
			Description: &desc,
			Ports: &ONUProfilePorts{
				Eth: &eth,
			},
			TcontNum:   &tcont,
			GemportNum: &gem,
		}
		if err := p.Validate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}

func TestONUHardwareProfileValidateEdgeCases(t *testing.T) {
	t.Run("description exactly 64 chars is valid", func(t *testing.T) {
		desc := makeString(64)
		p := &ONUHardwareProfile{Name: "test", Description: &desc}
		if err := p.Validate(); err != nil {
			t.Fatalf("expected no error for 64-char description, got %v", err)
		}
	})

	t.Run("eth port out of range high", func(t *testing.T) {
		eth := 256
		p := &ONUHardwareProfile{
			Name:  "test",
			Ports: &ONUProfilePorts{Eth: &eth},
		}
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for eth port 256")
		}
	})

	t.Run("eth port out of range low", func(t *testing.T) {
		eth := 0
		p := &ONUHardwareProfile{
			Name:  "test",
			Ports: &ONUProfilePorts{Eth: &eth},
		}
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for eth port 0")
		}
	})

	t.Run("tcont out of range high", func(t *testing.T) {
		tcont := 256
		gem := 1
		p := &ONUHardwareProfile{Name: "test", TcontNum: &tcont, GemportNum: &gem}
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for tcont 256")
		}
	})

	t.Run("gemport out of range high", func(t *testing.T) {
		tcont := 1
		gem := 256
		p := &ONUHardwareProfile{Name: "test", TcontNum: &tcont, GemportNum: &gem}
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for gemport 256")
		}
	})

	t.Run("switch num at upper bound is valid", func(t *testing.T) {
		switchNum := 255
		p := &ONUHardwareProfile{Name: "test", SwitchNum: &switchNum}
		if err := p.Validate(); err != nil {
			t.Fatalf("expected no error for switch num 255, got %v", err)
		}
	})

	t.Run("switch num above upper bound", func(t *testing.T) {
		switchNum := 256
		p := &ONUHardwareProfile{Name: "test", SwitchNum: &switchNum}
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for switch num 256")
		}
	})

	t.Run("nil ports is valid", func(t *testing.T) {
		p := &ONUHardwareProfile{Name: "test"}
		if err := p.Validate(); err != nil {
			t.Fatalf("expected no error for nil ports, got %v", err)
		}
	})

	t.Run("all port types set to valid values", func(t *testing.T) {
		eth := 4
		pots := 2
		iphost := 1
		ipv6host := 1
		veip := 1
		p := &ONUHardwareProfile{
			Name: "test",
			Ports: &ONUProfilePorts{
				Eth:      &eth,
				Pots:     &pots,
				IPHost:   &iphost,
				IPv6Host: &ipv6host,
				Veip:     &veip,
			},
		}
		if err := p.Validate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("pots out of range", func(t *testing.T) {
		pots := 0
		p := &ONUHardwareProfile{
			Name:  "test",
			Ports: &ONUProfilePorts{Pots: &pots},
		}
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for pots port 0")
		}
	})

	t.Run("tcont and gemport both at min valid", func(t *testing.T) {
		tcont := 1
		gem := 1
		p := &ONUHardwareProfile{Name: "test", TcontNum: &tcont, GemportNum: &gem}
		if err := p.Validate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("tcont and gemport both at max valid", func(t *testing.T) {
		tcont := 255
		gem := 255
		p := &ONUHardwareProfile{Name: "test", TcontNum: &tcont, GemportNum: &gem}
		if err := p.Validate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}

func makeString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
