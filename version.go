package gldwrs2

import _ "embed"

//go:embed VERSION
var version string

// Version is the CLI version, sourced from the VERSION file.
var Version = trimSpace(version)

func trimSpace(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
