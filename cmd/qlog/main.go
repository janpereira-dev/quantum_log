package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/janpereira-dev/quantum_log/internal/simplecli"
)

var (
	version = "simple-dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := simplecli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, simplecli.Version{Version: version, Commit: commit, BuildDate: date}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
