package common

import "regexp"

// ansiRegex matches ANSI escape sequences (colors, cursor movement, etc.)
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripANSI removes ANSI escape codes from a string.
// Useful for parsing CLI output that may contain terminal formatting.
func StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}
