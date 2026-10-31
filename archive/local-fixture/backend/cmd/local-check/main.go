// local-check validates native development configuration without contacting PostgreSQL.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"bidora/backend/internal/localcheck"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := localcheck.RunCommand(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
