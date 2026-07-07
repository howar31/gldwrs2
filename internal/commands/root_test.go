package commands

import (
	"bytes"
	"testing"
)

func TestRootVersion(t *testing.T) {
	root := Root("9.9.9")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := out.String(); got == "" || !bytes.Contains(out.Bytes(), []byte("9.9.9")) {
		t.Fatalf("version output = %q, want it to contain 9.9.9", got)
	}
}

// TestRootHelpHasSource verifies that `gw2 --help` prints the "Source:"
// footer required by the design spec (§2), same as `gw2 --version` does.
func TestRootHelpHasSource(t *testing.T) {
	root := Root("0.0.0")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := out.String(); !bytes.Contains(out.Bytes(), []byte("Source: github.com/howar31/gldwrs2")) {
		t.Fatalf("help output = %q, want it to contain %q", got, "Source: github.com/howar31/gldwrs2")
	}
}
