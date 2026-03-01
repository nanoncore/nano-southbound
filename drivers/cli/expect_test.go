package cli

import (
	"regexp"
	"testing"
)

func TestVendorPromptsMatch(t *testing.T) {
	tests := []struct {
		name    string
		vendor  string
		inputs  []string
		matches []bool
	}{
		{
			name:    "huawei matches angle bracket prompt",
			vendor:  "huawei",
			inputs:  []string{"<MA5800>"},
			matches: []bool{true},
		},
		{
			name:    "huawei matches square bracket prompt",
			vendor:  "huawei",
			inputs:  []string{"[MA5800]"},
			matches: []bool{true},
		},
		{
			name:   "vsol matches various prompts",
			vendor: "vsol",
			inputs: []string{
				"OLT#",
				"OLT>",
				"OLT(config)#",
				"OLT(config-if-gpon-0/1)#",
			},
			matches: []bool{true, true, true, true},
		},
		{
			name:   "cdata matches various prompts",
			vendor: "cdata",
			inputs: []string{
				"OLT#",
				"OLT(config)#",
			},
			matches: []bool{true, true},
		},
		{
			name:   "zte matches angle and square bracket prompts",
			vendor: "zte",
			inputs: []string{
				"<ZTE>",
				"[ZTE]",
			},
			matches: []bool{true, true},
		},
		{
			name:   "cisco matches various prompts",
			vendor: "cisco",
			inputs: []string{
				"Router#",
				"Router>",
				"Router(config)#",
				"Router(config-if)#",
			},
			matches: []bool{true, true, true, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, ok := VendorPrompts[tt.vendor]
			if !ok {
				t.Fatalf("vendor %q not found in VendorPrompts", tt.vendor)
			}

			for i, input := range tt.inputs {
				got := re.MatchString(input)
				want := tt.matches[i]
				if got != want {
					t.Errorf("VendorPrompts[%q].MatchString(%q) = %v, want %v", tt.vendor, input, got, want)
				}
			}
		})
	}
}

func TestDefaultPromptPattern(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "hostname with hash",
			input: "hostname#",
			want:  true,
		},
		{
			name:  "hostname with angle bracket",
			input: "hostname>",
			want:  true,
		},
		{
			name:  "hostname with trailing space",
			input: "hostname# ",
			want:  true,
		},
		{
			name:  "empty string does not match",
			input: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultPromptPattern.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("DefaultPromptPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestPagerDisableCommands(t *testing.T) {
	tests := []struct {
		vendor  string
		command string
	}{
		{"huawei", "screen-length 0 temporary"},
		{"vsol", "terminal length 0"},
		{"cdata", "terminal length 0"},
		{"zte", "screen-length 0 temporary"},
		{"cisco", "terminal length 0"},
	}

	for _, tt := range tests {
		t.Run(tt.vendor, func(t *testing.T) {
			cmd, ok := PagerDisableCommands[tt.vendor]
			if !ok {
				t.Fatalf("vendor %q not found in PagerDisableCommands", tt.vendor)
			}
			if cmd != tt.command {
				t.Errorf("PagerDisableCommands[%q] = %q, want %q", tt.vendor, cmd, tt.command)
			}
		})
	}
}

func TestCleanOutput(t *testing.T) {
	// Create an ExpectSession with a known promptRE for testing cleanOutput
	promptRE := regexp.MustCompile(`(?m)[\w\-]+[#>]\s*$`)

	session := &ExpectSession{
		promptRE: promptRE,
	}

	tests := []struct {
		name    string
		output  string
		command string
		want    string
	}{
		{
			name:    "removes command echo on first line",
			output:  "show version\nSoftware Version 1.0\nBuild 2024",
			command: "show version",
			want:    "Software Version 1.0\nBuild 2024",
		},
		{
			name:    "removes prompt at end",
			output:  "Software Version 1.0\nRouter#",
			command: "show version",
			want:    "Software Version 1.0",
		},
		{
			name:    "removes both command echo and trailing prompt",
			output:  "show version\nSoftware Version 1.0\nBuild 2024\nRouter#",
			command: "show version",
			want:    "Software Version 1.0\nBuild 2024",
		},
		{
			name:    "empty output returns empty string",
			output:  "",
			command: "show version",
			want:    "",
		},
		{
			name:    "multi-line output preserves middle lines",
			output:  "show interfaces\neth0: up\neth1: down\neth2: up\nSwitch#",
			command: "show interfaces",
			want:    "eth0: up\neth1: down\neth2: up",
		},
		{
			name:    "output with only prompt is empty after cleaning",
			output:  "Router#",
			command: "show version",
			want:    "",
		},
		{
			name:    "output with command echo only is empty after cleaning",
			output:  "show version",
			command: "show version",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := session.cleanOutput(tt.output, tt.command)
			if got != tt.want {
				t.Errorf("cleanOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpectSessionCloseNilExpecter(t *testing.T) {
	session := &ExpectSession{
		expecter: nil,
	}

	err := session.Close()
	if err != nil {
		t.Errorf("Close() on nil expecter should return nil, got %v", err)
	}
}
