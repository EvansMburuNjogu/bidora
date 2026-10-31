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

	"bidora/backend/internal/migrationrunner"
	"bidora/backend/migrations"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const dsnEnvironment = "BIDORA_MIGRATION_DSN"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("bidora-migrate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	setName := flags.String("set", "", "allowlisted migration set")
	targetText := flags.String("target", "", "allowlisted migration target")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *setName == "" || *targetText == "" {
		fmt.Fprintln(stderr, "migration_failed code=migration_arguments_invalid")
		return 2
	}
	target, err := strconv.ParseInt(*targetText, 10, 64)
	if err != nil {
		fmt.Fprintln(stderr, "migration_failed code=migration_arguments_invalid")
		return 2
	}
	plan, err := migrationrunner.Resolve(*setName, target)
	if err != nil {
		fmt.Fprintln(stderr, "migration_failed code=migration_plan_not_allowed")
		return 2
	}
	dsn := os.Getenv(dsnEnvironment)
	if dsn == "" {
		fmt.Fprintln(stderr, "migration_failed code=migration_dsn_missing")
		return 2
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintln(stderr, "migration_failed code=migration_database_unavailable")
		return 1
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(15 * time.Minute)

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signalContext, 10*time.Minute)
	defer cancel()
	if err := migrationrunner.Run(ctx, db, migrations.Files, plan); err != nil {
		fmt.Fprintln(stderr, "migration_failed code=migration_execution_failed")
		return 1
	}
	fmt.Fprintf(stdout, "migration_applied set=%s target=%d\n", plan.Name, plan.Target)
	return 0
}
