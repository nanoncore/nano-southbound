package common

import (
	"testing"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "no ANSI codes",
			input: "Hello, World!",
			want:  "Hello, World!",
		},
		{
			name:  "red text",
			input: "\x1b[31mError\x1b[0m",
			want:  "Error",
		},
		{
			name:  "bold text",
			input: "\x1b[1mBold\x1b[0m",
			want:  "Bold",
		},
		{
			name:  "multiple colors",
			input: "\x1b[31mRed\x1b[0m and \x1b[32mGreen\x1b[0m",
			want:  "Red and Green",
		},
		{
			name:  "cursor movement",
			input: "\x1b[2J\x1b[HHello",
			want:  "Hello",
		},
		{
			name:  "256 color code",
			input: "\x1b[38;5;196mBright Red\x1b[0m",
			want:  "Bright Red",
		},
		{
			name:  "complex sequence",
			input: "\x1b[1;31;40mBold Red on Black\x1b[0m",
			want:  "Bold Red on Black",
		},
		{
			name:  "mixed with newlines",
			input: "\x1b[32mLine1\x1b[0m\nLine2",
			want:  "Line1\nLine2",
		},
		{
			name:  "OLT CLI output simulation",
			input: "\x1b[0mAdmin#\x1b[K show onu-info",
			want:  "Admin# show onu-info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripANSI(tt.input)
			if got != tt.want {
				t.Errorf("StripANSI() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeCLIParam(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Safe values pass through unchanged
		{
			name:  "alphanumeric",
			input: "VSOL12345678",
			want:  "VSOL12345678",
		},
		{
			name:  "serial with dashes and dots",
			input: "VSOL-1234.5678",
			want:  "VSOL-1234.5678",
		},
		{
			name:  "ONU serial with colon",
			input: "VSOL:12345678",
			want:  "VSOL:12345678",
		},
		{
			name:  "path-like value",
			input: "gpon-olt_1/2/3",
			want:  "gpon-olt_1/2/3",
		},
		{
			name:  "value with @",
			input: "user@host",
			want:  "user@host",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},

		// Metacharacter stripping
		{
			name:  "semicolon injection",
			input: "VSOL; reboot",
			want:  "VSOL reboot",
		},
		{
			name:  "pipe injection",
			input: "serial | cat /etc/passwd",
			want:  "serial  cat /etc/passwd",
		},
		{
			name:  "ampersand injection",
			input: "cmd && rm -rf /",
			want:  "cmd  rm -rf /",
		},
		{
			name:  "backtick injection",
			input: "`whoami`",
			want:  "whoami",
		},
		{
			name:  "dollar injection",
			input: "$(reboot)",
			want:  "reboot",
		},
		{
			name:  "newline injection",
			input: "cmd1\ncmd2",
			want:  "cmd1cmd2",
		},
		{
			name:  "carriage return injection",
			input: "cmd1\rcmd2",
			want:  "cmd1cmd2",
		},
		{
			name:  "double quote stripping",
			input: `desc "test"`,
			want:  "desc test",
		},
		{
			name:  "single quote stripping",
			input: "desc 'test'",
			want:  "desc test",
		},
		{
			name:  "backslash stripping",
			input: `path\to\file`,
			want:  "pathtofile",
		},
		{
			name:  "parentheses stripping",
			input: "cmd(arg)",
			want:  "cmdarg",
		},
		{
			name:  "angle brackets stripping",
			input: "val<>test",
			want:  "valtest",
		},
		{
			name:  "exclamation mark stripping",
			input: "!important",
			want:  "important",
		},
		{
			name:  "question mark stripping",
			input: "help?",
			want:  "help",
		},
		{
			name:  "tilde stripping",
			input: "~root",
			want:  "root",
		},
		{
			name:  "curly braces stripping",
			input: "{cmd}",
			want:  "cmd",
		},

		// Combined
		{
			name:  "all metacharacters at once",
			input: ";|&`$\"\\'()\n\r<>!?~{}",
			want:  "",
		},
		{
			name:  "realistic injection attempt",
			input: "VSOL12345678; show running-config | include password",
			want:  "VSOL12345678 show running-config  include password",
		},
		{
			name:  "description with safe special chars",
			input: "Customer #123 - Floor 2/Room 3",
			want:  "Customer #123 - Floor 2/Room 3",
		},

		// Whitespace trimming
		{
			name:  "leading whitespace trimmed",
			input: "  value",
			want:  "value",
		},
		{
			name:  "trailing whitespace trimmed",
			input: "value  ",
			want:  "value",
		},
		{
			name:  "internal spaces preserved",
			input: "hello world",
			want:  "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeCLIParam(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeCLIParam(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
