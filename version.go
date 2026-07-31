package gldwrs2

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var version string

// Version is the CLI version, sourced from the VERSION file.
var Version = strings.TrimSpace(version)
