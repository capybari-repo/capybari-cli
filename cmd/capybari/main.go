// Command capybari is the Capybari Source Intelligence tool: one binary that
// inspects a folder, archive, repository or website and produces a unified
// Software X-Ray.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/capybari-repo/capybari-cli/internal/cli"
)

// Set at build time: -ldflags "-X main.version=... -X main.commit=..."
var (
	version = "dev"
	commit  = ""
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	app := &cli.App{Build: cli.Build{Version: version, Commit: commit}, Stdout: os.Stdout, Stderr: os.Stderr}
	code := app.Run(ctx, os.Args[1:])
	stop()
	os.Exit(code)
}
