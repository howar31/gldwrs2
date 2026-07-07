package main

import (
	"fmt"
	"os"

	gldwrs2 "github.com/howar31/gldwrs2"
	"github.com/howar31/gldwrs2/internal/api"
	"github.com/howar31/gldwrs2/internal/commands"
)

func main() {
	root := commands.Root(gldwrs2.Version)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "gw2:", err)
		os.Exit(api.ExitCode(err))
	}
}
