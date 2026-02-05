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

func makeString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
