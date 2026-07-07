package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/howar31/gldwrs2/internal/auth"
	"github.com/spf13/cobra"
)

func testApp(t *testing.T, out *bytes.Buffer) *App {
	t.Helper()
	dir := t.TempDir()
	key := make([]byte, 32)
	return &App{Out: out, Store: auth.NewStore(dir, key)}
}

func TestAuthSetListRemove(t *testing.T) {
	var out bytes.Buffer
	app := testApp(t, &out)

	run := func(args ...string) {
		root := &cobra.Command{Use: "gw2"}
		root.AddCommand(newAuthCmd(app))
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("execute %v: %v", args, err)
		}
	}

	run("auth", "set", "main", "--key", "SECRET")
	out.Reset()
	run("auth", "list")
	if !strings.Contains(out.String(), "main") {
		t.Fatalf("list = %q", out.String())
	}
	if strings.Contains(out.String(), "SECRET") {
		t.Fatal("list must never print the key")
	}
	out.Reset()
	run("auth", "remove", "main")
	out.Reset()
	run("auth", "list")
	if strings.Contains(out.String(), "main") {
		t.Fatalf("main should be gone, got %q", out.String())
	}
}
