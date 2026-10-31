// Package localcheck provides native development configuration and tool preflights.
package localcheck

import (
	"context"
	"fmt"
	"io"

	"bidora/backend/internal/localpostgres"
	"bidora/backend/internal/localprofile"
)

// RunCommand selects configuration-only checking (no arguments) or explicit
// native tool inspection (--tools). Unknown arguments fail before any execution.
func RunCommand(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return Run(getenv, stdout, stderr)
	}
	if len(args) != 1 || args[0] != "--tools" {
		fmt.Fprintln(stderr, "usage: local-check [--tools]")
		return 2
	}
	if _, err := localprofile.LoadPostgres(getenv); err != nil {
		fmt.Fprintf(stderr, "local-check: %s\n", err)
		return 1
	}
	tools, err := localpostgres.Inspect(ctx, getenv("BIDORA_PG_BIN_DIR"), getenv("BIDORA_PG_VERSION"))
	if err != nil {
		fmt.Fprintf(stderr, "local-check: %s\n", err)
		return 1
	}
	if _, err := fmt.Fprintf(stdout, "Native PostgreSQL tools match %s; security acceptance, database connectivity, roles and resource enforcement NOT checked.\n", tools.Version); err != nil {
		fmt.Fprintln(stderr, "local-check: unable to write result")
		return 1
	}
	return 0
}

// Run writes a safe configuration verdict and returns a process exit code.
// A successful result says nothing about database availability or permissions.
func Run(getenv func(string) string, stdout, stderr io.Writer) int {
	if _, err := localprofile.LoadPostgres(getenv); err != nil {
		fmt.Fprintf(stderr, "local-check: %s\n", err)
		return 1
	}
	if _, err := fmt.Fprintln(stdout, "Local configuration valid; database connectivity, credentials, roles and resource enforcement NOT checked."); err != nil {
		fmt.Fprintln(stderr, "local-check: unable to write result")
		return 1
	}
	return 0
}
