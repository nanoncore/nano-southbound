package common

import (
	"regexp"
	"strings"
)

// ansiRegex matches ANSI escape sequences (colors, cursor movement, etc.)
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripANSI removes ANSI escape codes from a string.
// Useful for parsing CLI output that may contain terminal formatting.
func StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// SanitizeCLIParam removes or escapes shell metacharacters from a string
// before interpolation into CLI commands. This prevents command injection
// on OLT devices where user-supplied values (serial numbers, descriptions,
// profile names, VLAN names) are interpolated via fmt.Sprintf.
func SanitizeCLIParam(s string) string {
	// Strip characters that could be used for command injection
	r := strings.NewReplacer(
		";", "",
		"|", "",
		"&", "",
		"`", "",
		"$", "",
		"\n", "",
		"\r", "",
		"\"", "",
		"'", "",
		"\\", "",
		"(", "",
		")", "",
		"<", "",
		">", "",
		"!", "",
		"?", "",
		"~", "",
		"{", "",
		"}", "",
	)
	return strings.TrimSpace(r.Replace(s))
}
