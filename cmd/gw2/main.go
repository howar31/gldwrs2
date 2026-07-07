package main

import (
	"fmt"
	"os"

	gldwrs2 "github.com/howar31/gldwrs2"
	"github.com/howar31/gldwrs2/internal/commands"
)

func main() {
	root := commands.Root(gldwrs2.Version)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "gw2:", err)
		// TODO(task-3): restore os.Exit(api.ExitCode(err)) once internal/api exists.
		os.Exit(1)
	}
}
