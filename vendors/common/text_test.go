package common

import "testing"

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
