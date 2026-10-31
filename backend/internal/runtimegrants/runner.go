package runtimegrants

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

var safeRole = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

var (
	ErrInvalid      = errors.New("runtime grant input is invalid")
	ErrPrecondition = errors.New("runtime grant precondition failed")
	ErrVerification = errors.New("runtime grant verification failed")
)

// Apply replaces direct runtime table/schema/sequence privileges in the
// manifest schemas with the reviewed query-derived set in one transaction.
func Apply(ctx context.Context, db *sql.DB, role string, manifest Manifest) error {
	if ctx == nil || db == nil || !safeRole.MatchString(role) || role == "public" || manifest.Plane == "" || len(manifest.Schemas) == 0 || len(manifest.Tables) == 0 {
		return ErrInvalid
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("%w: begin", ErrPrecondition)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SET LOCAL lock_timeout='2s'; SET LOCAL statement_timeout='30s'"); err != nil {
		return fmt.Errorf("%w: configure", ErrPrecondition)
	}
	lockKey := int64(0x4249444f52414754)
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
		return fmt.Errorf("%w: lock", ErrPrecondition)
	}
	if err = verifyRoleAndDatabase(ctx, tx, role); err != nil {
		return err
	}
	if err = verifyCompatibility(ctx, tx, manifest); err != nil {
		return err
	}

	tables, err := listRelations(ctx, tx, manifest.Schemas, "BASE TABLE")
	if err != nil {
		return err
	}
	// Views are grantable relations too. Include them in the deny-first sweep;
	// the approved audit projection is SELECT-only, never the private base table.
	views, err := listRelations(ctx, tx, manifest.Schemas, "VIEW")
	if err != nil {
		return err
	}
	for view := range views {
		tables[view] = struct{}{}
	}
	sequences, err := listRelations(ctx, tx, manifest.Schemas, "SEQUENCE")
	if err != nil {
		return err
	}
	roleSQL := pgx.Identifier{role}.Sanitize()
	for table := range tables {
		if _, err = tx.ExecContext(ctx, "REVOKE ALL PRIVILEGES ON TABLE "+qualified(table)+" FROM "+roleSQL); err != nil {
			return fmt.Errorf("%w: revoke table", ErrPrecondition)
		}
	}
	for sequence := range sequences {
		if _, err = tx.ExecContext(ctx, "REVOKE ALL PRIVILEGES ON SEQUENCE "+qualified(sequence)+" FROM "+roleSQL); err != nil {
			return fmt.Errorf("%w: revoke sequence", ErrPrecondition)
		}
	}
	for _, schema := range manifest.Schemas {
		schemaSQL := pgx.Identifier{schema}.Sanitize()
		if _, err = tx.ExecContext(ctx, "REVOKE ALL PRIVILEGES ON SCHEMA "+schemaSQL+" FROM "+roleSQL); err != nil {
			return fmt.Errorf("%w: revoke schema", ErrPrecondition)
		}
		if _, err = tx.ExecContext(ctx, "GRANT USAGE ON SCHEMA "+schemaSQL+" TO "+roleSQL); err != nil {
			return fmt.Errorf("%w: grant schema", ErrPrecondition)
		}
	}
	for _, grant := range manifest.Tables {
		if _, ok := tables[grant.Table]; !ok {
			return fmt.Errorf("%w: missing table", ErrPrecondition)
		}
		privileges := make([]string, len(grant.Privileges))
		for index, privilege := range grant.Privileges {
			privileges[index] = string(privilege)
		}
		if _, err = tx.ExecContext(ctx, "GRANT "+strings.Join(privileges, ", ")+" ON TABLE "+qualified(grant.Table)+" TO "+roleSQL); err != nil {
			return fmt.Errorf("%w: grant table", ErrPrecondition)
		}
	}
	if err = verifyEffectivePrivileges(ctx, tx, role, manifest, tables, sequences); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("%w: commit", ErrPrecondition)
	}
	return nil
}

func verifyRoleAndDatabase(ctx context.Context, tx *sql.Tx, role string) error {
	var login, superuser, createDB, createRole, replication, bypassRLS bool
	var memberships, ownedRelations int64
	err := tx.QueryRowContext(ctx, `SELECT rolcanlogin,rolsuper,rolcreatedb,rolcreaterole,rolreplication,rolbypassrls,
 (SELECT count(*) FROM pg_auth_members WHERE member=r.oid),
 (SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relowner=r.oid AND n.nspname !~ '^pg_' AND n.nspname<>'information_schema')
FROM pg_roles r WHERE rolname=$1`, role).Scan(&login, &superuser, &createDB, &createRole, &replication, &bypassRLS, &memberships, &ownedRelations)
	if err != nil || !login || superuser || createDB || createRole || replication || bypassRLS || memberships != 0 || ownedRelations != 0 {
		return fmt.Errorf("%w: role shape", ErrPrecondition)
	}
	var connect, create, temporary bool
	if err = tx.QueryRowContext(ctx, `SELECT has_database_privilege($1,current_database(),'CONNECT'),has_database_privilege($1,current_database(),'CREATE'),has_database_privilege($1,current_database(),'TEMPORARY')`, role).Scan(&connect, &create, &temporary); err != nil || !connect || create || temporary {
		return fmt.Errorf("%w: database privileges", ErrPrecondition)
	}
	var publicTableGrants int64
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.table_privileges WHERE grantee='PUBLIC' AND table_schema !~ '^pg_' AND table_schema NOT IN ('information_schema','bidora_migrations')`).Scan(&publicTableGrants); err != nil || publicTableGrants != 0 {
		return fmt.Errorf("%w: public table privileges", ErrPrecondition)
	}
	return nil
}

func verifyCompatibility(ctx context.Context, tx *sql.Tx, manifest Manifest) error {
	var matched int64
	switch manifest.Plane {
	case "control":
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM bidora_control_meta.schema_compatibility WHERE migration_set='control_registry' AND schema_version=43`).Scan(&matched); err != nil || matched != 1 {
			return fmt.Errorf("%w: control compatibility", ErrPrecondition)
		}
	case "tenant":
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM bidora_meta.schema_compatibility WHERE (migration_set='tenant_common' AND schema_version=9) OR (migration_set='tenant_tenderpreneur' AND schema_version=34)`).Scan(&matched); err != nil || matched != 2 {
			return fmt.Errorf("%w: tenant compatibility", ErrPrecondition)
		}
	default:
		return ErrInvalid
	}
	return nil
}

func listRelations(ctx context.Context, tx *sql.Tx, schemas []string, kind string) (map[string]struct{}, error) {
	query := `SELECT table_schema||'.'||table_name FROM information_schema.tables WHERE table_schema=ANY($1) AND table_type=$2 ORDER BY 1`
	args := []any{schemas, kind}
	if kind == "SEQUENCE" {
		query = `SELECT sequence_schema||'.'||sequence_name FROM information_schema.sequences WHERE sequence_schema=ANY($1) ORDER BY 1`
		args = []any{schemas}
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: list relations", ErrPrecondition)
	}
	defer rows.Close()
	result := map[string]struct{}{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("%w: read relations", ErrPrecondition)
		}
		result[name] = struct{}{}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: read relations", ErrPrecondition)
	}
	return result, nil
}

func verifyEffectivePrivileges(ctx context.Context, tx *sql.Tx, role string, manifest Manifest, tables, sequences map[string]struct{}) error {
	expected := map[string]map[Privilege]bool{}
	for _, grant := range manifest.Tables {
		expected[grant.Table] = map[Privilege]bool{}
		for _, privilege := range grant.Privileges {
			expected[grant.Table][privilege] = true
		}
	}
	for table := range tables {
		var selectOK, insertOK, updateOK, deleteOK bool
		if err := tx.QueryRowContext(ctx, `SELECT has_table_privilege($1,$2,'SELECT'),has_table_privilege($1,$2,'INSERT'),has_table_privilege($1,$2,'UPDATE'),has_table_privilege($1,$2,'DELETE')`, role, table).Scan(&selectOK, &insertOK, &updateOK, &deleteOK); err != nil {
			return fmt.Errorf("%w: table privileges", ErrVerification)
		}
		want := expected[table]
		if selectOK != want[Select] || insertOK != want[Insert] || updateOK != want[Update] || deleteOK != want[Delete] {
			return fmt.Errorf("%w: table mismatch", ErrVerification)
		}
	}
	for sequence := range sequences {
		var usage, selectOK, updateOK bool
		if err := tx.QueryRowContext(ctx, `SELECT has_sequence_privilege($1,$2,'USAGE'),has_sequence_privilege($1,$2,'SELECT'),has_sequence_privilege($1,$2,'UPDATE')`, role, sequence).Scan(&usage, &selectOK, &updateOK); err != nil || usage || selectOK || updateOK {
			return fmt.Errorf("%w: sequence privileges", ErrVerification)
		}
	}
	for _, schema := range manifest.Schemas {
		var usage, create bool
		if err := tx.QueryRowContext(ctx, `SELECT has_schema_privilege($1,$2,'USAGE'),has_schema_privilege($1,$2,'CREATE')`, role, schema).Scan(&usage, &create); err != nil || !usage || create {
			return fmt.Errorf("%w: schema privileges", ErrVerification)
		}
	}
	return nil
}

func qualified(name string) string {
	parts := strings.Split(name, ".")
	return pgx.Identifier(parts).Sanitize()
}
