// Package localpostgres checks native development prerequisites. It does not
// start, initialize, upgrade or connect to a PostgreSQL cluster.
package localpostgres

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Toolchain reports matching versions only, not security or runtime acceptance.
type Toolchain struct {
	Version string
}

// Inspect executes postgres, pg_ctl, initdb and psql with --version, sequentially,
// from an explicitly trusted absolute binary directory. It never searches PATH.
// The caller must trust these executables: this check is not a sandbox.
// Commands have five-second individual and fifteen-second overall deadlines, plus
// at most 250 ms each to drain process pipes. Output is bounded. Ambient
// credentials, libpq settings and loader overrides are not inherited.
func Inspect(ctx context.Context, binDir, expectedVersion string) (Toolchain, error) {
	if ctx == nil {
		return Toolchain{}, errors.New("native PostgreSQL inspection requires a context")
	}
	if !filepath.IsAbs(binDir) || filepath.Clean(binDir) != binDir || strings.IndexByte(binDir, 0) >= 0 {
		return Toolchain{}, errors.New("BIDORA_PG_BIN_DIR must be an explicit clean absolute directory")
	}
	versionPattern := `[1-9][0-9]\.(0|[1-9][0-9]{0,2})`
	if !regexp.MustCompile(`^` + versionPattern + `$`).MatchString(expectedVersion) {
		return Toolchain{}, errors.New("BIDORA_PG_VERSION must be an explicit PostgreSQL major.minor version")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	for _, tool := range []string{"postgres", "pg_ctl", "initdb", "psql"} {
		if err := ctx.Err(); err != nil {
			return Toolchain{}, &checkError{"native PostgreSQL inspection canceled or timed out", err}
		}
		var output boundedOutput
		commandCtx, commandCancel := context.WithTimeout(ctx, 5*time.Second)
		cmd := exec.CommandContext(commandCtx, filepath.Join(binDir, tool), "--version")
		cmd.Dir = binDir
		cmd.Env = []string{"LC_ALL=C", "LANG=C", "PATH=/usr/bin:/bin"}
		cmd.Stdout, cmd.Stderr = &output, &output
		cmd.WaitDelay = 250 * time.Millisecond
		runErr := cmd.Run()
		contextErr := commandCtx.Err()
		commandCancel()
		if runErr != nil {
			if contextErr != nil {
				return Toolchain{}, &checkError{"native PostgreSQL inspection canceled or timed out", contextErr}
			}
			return Toolchain{}, &checkError{tool + " version command failed", runErr}
		}
		pattern := `^` + regexp.QuoteMeta(tool) + ` \(PostgreSQL\) (` + versionPattern + `)(?: \([ -~]+\))?$`
		match := regexp.MustCompile(pattern).FindStringSubmatch(strings.TrimSpace(output.data.String()))
		if match == nil || match[1] != expectedVersion {
			return Toolchain{}, errors.New(tool + " did not report the expected PostgreSQL version")
		}
	}
	return Toolchain{Version: expectedVersion}, nil
}

// Keep command paths and raw process errors out of the public diagnostic while
// preserving the cause for errors.Is/errors.As in trusted diagnostic handling.
type checkError struct {
	message string
	cause   error
}

func (e *checkError) Error() string { return e.message }
func (e *checkError) Unwrap() error { return e.cause }

type boundedOutput struct{ data bytes.Buffer }

func (b *boundedOutput) Write(p []byte) (int, error) {
	if len(p) > 4096-b.data.Len() {
		return 0, errors.New("native PostgreSQL version output exceeds limit")
	}
	return b.data.Write(p)
}
