package localpostgres_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/localpostgres"
)

// Shell fixtures are unit-test executables, not PostgreSQL runtime evidence.
func fixtureTools(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("native Unix tool fixture")
	}
	dir := t.TempDir()
	for _, tool := range []string{"postgres", "pg_ctl", "initdb", "psql"} {
		writeTool(t, dir, tool, `
[ "$#" -eq 1 ] && [ "$1" = '--version' ] || exit 41
[ -z "${PGPASSWORD+x}" ] && [ -z "${PGSERVICE+x}" ] && [ -z "${BIDORA_PROVIDER_SECRET+x}" ] || exit 42
[ "$LC_ALL" = C ] && [ "$PATH" = /usr/bin:/bin ] || exit 43
printf '`+tool+` (PostgreSQL) 15.17 (Homebrew)\n'
`)
	}
	return dir
}

func writeTool(t *testing.T, dir, tool, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, tool), []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
}

func TestInspectMatchingToolsAndSanitizedEnvironment(t *testing.T) {
	dir := fixtureTools(t)
	t.Setenv("PGPASSWORD", "secret-canary")
	t.Setenv("PGSERVICE", "private-service")
	t.Setenv("BIDORA_PROVIDER_SECRET", "secret-canary")
	t.Setenv("PATH", dir)
	got, err := localpostgres.Inspect(context.Background(), dir, "15.17")
	if err != nil || got.Version != "15.17" {
		t.Fatalf("unexpected result: %+v, %v", got, err)
	}
}

func TestInspectRejectsInvalidInputs(t *testing.T) {
	for _, dir := range []string{"", ".", "relative", "/tmp/../tmp", "/tmp/", "/secret-canary\x00"} {
		if got, err := localpostgres.Inspect(context.Background(), dir, "15.17"); err == nil || got != (localpostgres.Toolchain{}) || strings.Contains(err.Error(), "secret-canary") {
			t.Fatal("unsafe binary directory accepted or disclosed")
		}
	}
	for _, version := range []string{"", "15", "9.6", "015.17", "15.017", "15.17.1", "15.17\n", "15.17;secret-canary", "15.1000"} {
		if got, err := localpostgres.Inspect(context.Background(), t.TempDir(), version); err == nil || got != (localpostgres.Toolchain{}) || strings.Contains(err.Error(), "secret-canary") {
			t.Fatal("invalid version accepted or disclosed")
		}
	}
	if _, err := localpostgres.Inspect(nil, t.TempDir(), "15.17"); err == nil {
		t.Fatal("nil context accepted")
	}
}

func TestInspectRejectsEachMissingOrMismatchedTool(t *testing.T) {
	for _, tool := range []string{"postgres", "pg_ctl", "initdb", "psql"} {
		t.Run(tool, func(t *testing.T) {
			dir := fixtureTools(t)
			if err := os.Remove(filepath.Join(dir, tool)); err != nil {
				t.Fatal(err)
			}
			got, err := localpostgres.Inspect(context.Background(), dir, "15.17")
			if err == nil || got != (localpostgres.Toolchain{}) || err.Error() != tool+" version command failed" || errors.Unwrap(err) == nil {
				t.Fatalf("missing tool not safely reported with cause: %v", err)
			}
			writeTool(t, dir, tool, "printf '"+tool+" (PostgreSQL) 15.16\\n'")
			if got, err := localpostgres.Inspect(context.Background(), dir, "15.17"); err == nil || got != (localpostgres.Toolchain{}) {
				t.Fatal("mismatched versions accepted")
			}
		})
	}
}

func TestInspectSafeProcessFailures(t *testing.T) {
	for name, body := range map[string]string{
		"failed exit": "printf 'secret-canary' >&2; exit 7",
		"malformed":   "printf 'secret-canary\\n'",
		"wrong tool":  "printf 'psql (PostgreSQL) 15.17\\n'",
		"extra line":  "printf 'postgres (PostgreSQL) 15.17\\nsecret-canary\\n'",
		"oversized":   "i=0; while [ \"$i\" -lt 1000 ]; do printf 'secret-canary-secret-canary-secret-canary\\n'; i=$((i+1)); done",
	} {
		t.Run(name, func(t *testing.T) {
			dir := fixtureTools(t)
			writeTool(t, dir, "postgres", body)
			got, err := localpostgres.Inspect(context.Background(), dir, "15.17")
			if err == nil || got != (localpostgres.Toolchain{}) || strings.Contains(err.Error(), "secret-canary") {
				t.Fatal("process failure accepted or unsafe output disclosed")
			}
		})
	}
}

func TestInspectCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := localpostgres.Inspect(ctx, t.TempDir(), "15.17"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation cause lost: %v", err)
	}
}

func TestInspectDeadline(t *testing.T) {
	dir := fixtureTools(t)
	writeTool(t, dir, "postgres", "exec /bin/sleep 10")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if got, err := localpostgres.Inspect(ctx, dir, "15.17"); !errors.Is(err, context.DeadlineExceeded) || got != (localpostgres.Toolchain{}) {
		t.Fatalf("deadline not preserved: %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("deadline did not bound the subprocess")
	}
}
