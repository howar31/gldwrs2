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
