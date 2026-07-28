package main

import (
	"fmt"
	"os"

	"github.com/janpereira-dev/quantum_log/internal/cli"
)

var (
	version = "0.3.2"
	commit  = "none"
	date    = "unknown"
)

func main() {
	command := cli.New(cli.Version{Version: version, Commit: commit, BuildDate: date})
	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
