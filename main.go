// Command mkgo scaffolds a Go project, initializes git and creates the
// matching GitHub repository in a single run.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/x-chunk/mkgo/internal/cli"
)

// version is replaced at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(cli.Run(ctx, os.Args[1:], cli.Environment{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Stdin:   os.Stdin,
		Version: version,
	}))
}
