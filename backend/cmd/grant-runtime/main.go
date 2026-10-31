package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"bidora/backend/internal/runtimegrants"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	dsnEnvironment     = "BIDORA_GRANT_DSN"
	runtimeEnvironment = "BIDORA_RUNTIME_DSN"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("bidora-grant-runtime", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	plane := flags.String("plane", "", "allowlisted database plane")
	versionText := flags.String("manifest-version", "", "exact reviewed manifest version")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *plane == "" || *versionText == "" {
		fmt.Fprintln(stderr, "runtime_grants_failed code=arguments_invalid")
		return 2
	}
	version, err := strconv.Atoi(*versionText)
	if err != nil || version != runtimegrants.Version {
		fmt.Fprintln(stderr, "runtime_grants_failed code=manifest_not_allowed")
		return 2
	}
	manifest, err := runtimegrants.Load(*plane)
	if err != nil {
		fmt.Fprintln(stderr, "runtime_grants_failed code=manifest_not_allowed")
		return 2
	}
	dsn, runtimeDSN := os.Getenv(dsnEnvironment), os.Getenv(runtimeEnvironment)
	if dsn == "" || runtimeDSN == "" {
		fmt.Fprintln(stderr, "runtime_grants_failed code=configuration_missing")
		return 2
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintln(stderr, "runtime_grants_failed code=database_unavailable")
		return 1
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(5 * time.Minute)
	runtimeDB, err := sql.Open("pgx", runtimeDSN)
	if err != nil {
		fmt.Fprintln(stderr, "runtime_grants_failed code=database_unavailable")
		return 1
	}
	defer runtimeDB.Close()
	runtimeDB.SetMaxOpenConns(1)
	runtimeDB.SetMaxIdleConns(0)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	role, runtimeDatabase, runtimeTLS, err := connectionIdentity(ctx, runtimeDB)
	if err != nil {
		fmt.Fprintln(stderr, "runtime_grants_failed code=runtime_unavailable")
		return 1
	}
	_, migratorDatabase, migratorTLS, err := connectionIdentity(ctx, db)
	if err != nil || runtimeDatabase != migratorDatabase || !runtimeTLS || !migratorTLS {
		fmt.Fprintln(stderr, "runtime_grants_failed code=connection_mismatch")
		return 1
	}
	if err = runtimegrants.Apply(ctx, db, role, manifest); err != nil {
		fmt.Fprintln(stderr, "runtime_grants_failed code=application_failed")
		return 1
	}
	fmt.Fprintf(stdout, "runtime_grants_applied plane=%s manifest_version=%d tables=%d\n", manifest.Plane, runtimegrants.Version, len(manifest.Tables))
	return 0
}

func connectionIdentity(ctx context.Context, db *sql.DB) (role, database string, tls bool, err error) {
	err = db.QueryRowContext(ctx, `SELECT current_user,current_database(),COALESCE((SELECT ssl FROM pg_stat_ssl WHERE pid=pg_backend_pid()),false)`).Scan(&role, &database, &tls)
	return
}
