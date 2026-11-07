// Package migrationrunner applies one explicitly allowlisted migration set.
// It is intended only for the standalone migrator command, never API startup.
package migrationrunner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

var (
	ErrInvalidSet    = errors.New("migration set is not allowlisted")
	ErrInvalidTarget = errors.New("migration target is not allowlisted")
)

// Plan is an immutable allowlisted migration operation.
type Plan struct {
	Name      string
	Target    int64
	TableName string
	Directory string
	LockKey   uint32
}

var plans = map[string]Plan{
	// control-auth is the bounded authentication milestone. It deliberately
	// stops before unrelated notification and organization migrations at versions 30 and 31.
	"control-auth": {
		Name:      "control-auth",
		Target:    29,
		TableName: "bidora_migrations.goose_control_version",
		Directory: "control",
		LockKey:   0x434f4e54,
	},
	"control": {
		Name:      "control",
		Target:    52,
		TableName: "bidora_migrations.goose_control_version",
		Directory: "control",
		LockKey:   0x434f4e54,
	},
	"tenant-common": {
		Name:      "tenant-common",
		Target:    9,
		TableName: "bidora_migrations.goose_tenant_common_version",
		Directory: "tenant_common",
		LockKey:   0x54434f4d,
	},
	"tenant-tenderpreneur": {
		Name:      "tenant-tenderpreneur",
		Target:    34,
		TableName: "bidora_migrations.goose_tenderpreneur_version",
		Directory: "tenant_tenderpreneur",
		LockKey:   0x5450524e,
	},
}

// Resolve returns a plan only when both the set and its application target match.
func Resolve(name string, target int64) (Plan, error) {
	plan, ok := plans[name]
	if !ok {
		return Plan{}, ErrInvalidSet
	}
	if target != plan.Target {
		return Plan{}, ErrInvalidTarget
	}
	return plan, nil
}

// Run applies the plan under a database-and-set-scoped PostgreSQL advisory lock.
// The supplied database must already be configured with the private ledger schema.
func Run(ctx context.Context, db *sql.DB, migrationFS fs.FS, plan Plan) error {
	if db == nil {
		return errors.New("migration database is required")
	}

	var databaseOID int64
	if err := db.QueryRowContext(ctx, "SELECT oid::bigint FROM pg_database WHERE datname = current_database()").Scan(&databaseOID); err != nil {
		return fmt.Errorf("read migration database identity: %w", err)
	}
	if databaseOID <= 0 || databaseOID > int64(^uint32(0)) {
		return errors.New("migration database identity is invalid")
	}

	setFS, err := fs.Sub(migrationFS, plan.Directory)
	if err != nil {
		return fmt.Errorf("select migration set: %w", err)
	}
	lockID := int64((uint64(uint32(databaseOID)) << 32) | uint64(plan.LockKey))
	sessionLocker, err := lock.NewPostgresSessionLocker(lock.WithLockID(lockID))
	if err != nil {
		return fmt.Errorf("configure migration lock: %w", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		setFS,
		goose.WithTableName(plan.TableName),
		goose.WithSessionLocker(sessionLocker),
	)
	if err != nil {
		return fmt.Errorf("configure migration provider: %w", err)
	}
	if _, err := provider.UpTo(ctx, plan.Target); err != nil {
		return fmt.Errorf("apply migration set: %w", err)
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("read migration version: %w", err)
	}
	if version != plan.Target {
		return fmt.Errorf("migration target mismatch: got %d", version)
	}
	return nil
}
