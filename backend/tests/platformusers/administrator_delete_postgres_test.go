package platformusers_test

import (
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	app "bidora/backend/internal/platformadmin/application"
	store "bidora/backend/internal/platformadmin/persistence"
	platformsql "bidora/backend/internal/platformadmin/persistence/sqlc"
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLocalAdministratorDeletionMigrationAndEmailReuseRollbackOnly(t *testing.T) {
	if os.Getenv("BIDORA_TEST_DELETE_ROLLBACK") != "1" {
		t.Skip("explicit rollback-only local schema verification opt-in required")
	}
	config, e := pgx.ParseConfig(os.Getenv("BIDORA_TEST_DELETE_ADMIN_DSN"))
	enrollmentCheck(t, e, "parse test target")
	if config.Host != "127.0.0.1" && config.Host != "localhost" || config.Port != 55432 {
		t.Fatal("refusing non-designated local target")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	connection, e := pgx.ConnectConfig(ctx, config)
	enrollmentCheck(t, e, "connect test target")
	defer connection.Close(context.Background())
	outer, e := connection.Begin(ctx)
	enrollmentCheck(t, e, "begin rollback-only boundary")
	defer outer.Rollback(context.Background())
	migration, e := os.ReadFile("../../migrations/control/00060_disabled_administrator_deletion.sql")
	enrollmentCheck(t, e, "read migration")
	up := strings.Split(string(migration), "-- +goose Down")[0]
	var version int64
	enrollmentCheck(t, outer.QueryRow(ctx, "SELECT schema_version FROM bidora_control_meta.schema_compatibility WHERE migration_set='control_registry'").Scan(&version), "read migration checkpoint")
	if version == 59 {
		_, e = outer.Exec(ctx, up)
		enrollmentCheck(t, e, "apply migration inside rollback-only boundary")
	} else if version != 60 {
		t.Fatal("unsupported local schema checkpoint")
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	clock := enrollmentClock{now}
	q := identitysql.New(outer)
	p := platformsql.New(outer)
	newID := func() pgtype.UUID { return pgtype.UUID{Bytes: [16]byte(uuid.New()), Valid: true} }
	actorID, targetID, sessionID := newID(), newID(), newID()
	first, last := "Deletion", "Fixture"
	email := "delete-" + uuid.NewString() + "@example.invalid"
	addUser := func(id pgtype.UUID, address string, disabled bool) {
		enrollmentCheck(t, q.InsertPasswordUser(ctx, identitysql.InsertPasswordUserParams{ID: id, Email: address, EmailLookup: address, FirstName: &first, LastName: &last, CreatedAt: pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true}}), "insert synthetic user")
		_, e = outer.Exec(ctx, "UPDATE identity.users SET email_verified_at=$2::timestamptz,account_type='PLATFORM_ADMIN',disabled_at=CASE WHEN $3::boolean THEN $2::timestamptz ELSE NULL END,updated_at=$2::timestamptz WHERE id=$1", id, now, disabled)
		enrollmentCheck(t, e, "prepare fixture identity")
		enrollmentCheck(t, q.InsertPasswordIdentity(ctx, identitysql.InsertPasswordIdentityParams{UserID: id, PasswordPhc: "$argon2id$v=19$m=65536,t=3,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", CreatedAt: pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true}}), "insert synthetic credential")
		enrollmentCheck(t, p.InsertPlatformAdministratorProfile(ctx, platformsql.InsertPlatformAdministratorProfileParams{UserID: id, At: pgtype.Timestamptz{Time: now, Valid: true}}), "insert fixture profile")
	}
	addUser(actorID, "owner-"+uuid.NewString()+"@example.invalid", false)
	addUser(targetID, email, true)
	for _, row := range []struct {
		id   pgtype.UUID
		role string
	}{{actorID, "PLATFORM_OWNER"}, {targetID, "SUPPORT_ADMINISTRATOR"}} {
		enrollmentCheck(t, p.InsertPlatformRoleAssignment(ctx, platformsql.InsertPlatformRoleAssignmentParams{ID: newID(), UserID: row.id, RoleKey: row.role, Source: "BOOTSTRAP", AssignedAt: pgtype.Timestamptz{Time: now, Valid: true}, Reason: "Rollback-only deletion fixture"}), "assign fixture role")
	}
	enrollmentCheck(t, q.InsertSession(ctx, identitysql.InsertSessionParams{ID: sessionID, UserID: actorID, TokenHash: make([]byte, 32), CsrfHash: make([]byte, 32), AuthEpoch: 1, CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}, IdleExpiresAt: pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true}, AbsoluteExpiresAt: pgtype.Timestamptz{Time: now.Add(2 * time.Hour), Valid: true}, AssuranceLevel: "AAL2", AssuranceMethod: "EMAIL_OTP", AssuredAt: pgtype.Timestamptz{Time: now, Valid: true}, IdleTimeoutSeconds: 3600, AbsoluteTimeoutSeconds: 7200}), "insert verified fixture session")
	persistence, e := store.NewStore(enrollmentRollbackRunner{outer})
	enrollmentCheck(t, e, "compose deletion store")
	service, e := app.NewRoleChangeService(persistence, enrollmentIDs{}, clock)
	enrollmentCheck(t, e, "compose lifecycle service")
	input := app.IdentityStateChangeInput{ActorID: app.ID(actorID.Bytes), SessionID: app.ID(sessionID.Bytes), TargetUserID: app.ID(targetID.Bytes), CorrelationID: app.ID(uuid.New()), IdempotencyKey: app.ID(uuid.New()), PlatformContextGeneration: 1, ExpectedAdministratorVersion: 1, ExpectedTargetContextGeneration: 1, Action: app.DeleteAdministrator, Reason: "Rollback-only account deletion"}
	runtimeConfig, e := pgx.ParseConfig(os.Getenv("BIDORA_TEST_CONTROL_RUNTIME_DSN"))
	enrollmentCheck(t, e, "parse runtime identity")
	runtimeRole := pgx.Identifier{runtimeConfig.User}.Sanitize()
	_, e = outer.Exec(ctx, "GRANT DELETE ON identity.users TO "+runtimeRole)
	enrollmentCheck(t, e, "grant only the new runtime privilege inside rollback boundary")
	_, e = outer.Exec(ctx, "SET LOCAL ROLE "+runtimeRole)
	enrollmentCheck(t, e, "use actual runtime permissions")
	result, e := service.ChangeIdentityState(ctx, input)
	enrollmentCheck(t, e, "delete disabled synthetic account")
	_, e = outer.Exec(ctx, "RESET ROLE")
	enrollmentCheck(t, e, "return to fixture administrator for history assertions")
	if result.State != "EXECUTED" {
		t.Fatal("deletion not executed")
	}
	var users, credentials, profiles, roles, refs, audits int
	enrollmentCheck(t, outer.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM identity.users WHERE id=$1),
 (SELECT count(*) FROM identity.authentication_identities WHERE user_id=$1),
 (SELECT count(*) FROM platform_administration.administrator_profiles WHERE user_id=$1),
 (SELECT count(*) FROM platform_administration.role_assignments WHERE user_id=$1),
 (SELECT count(*) FROM identity.user_references WHERE id=$1 AND deleted_at IS NOT NULL),
 (SELECT count(*) FROM bidora_control_audit.platform_action_times_v1 v JOIN bidora_control_audit.audit_events a ON a.id=v.id WHERE v.action='PlatformAdministratorDeleted' AND a.target_id=$2)`, targetID, pgtype.UUID{Bytes: [16]byte(result.ChangeID), Valid: true}).Scan(&users, &credentials, &profiles, &roles, &refs, &audits), "verify actual deletion and preserved history")
	if users != 0 || credentials != 0 || profiles != 1 || roles != 1 || refs != 1 || audits != 1 {
		t.Fatalf("deletion/history invariant failed: %d %d %d %d %d %d", users, credentials, profiles, roles, refs, audits)
	}
	replay, e := service.ChangeIdentityState(ctx, input)
	enrollmentCheck(t, e, "replay deletion")
	if !replay.Replay {
		t.Fatal("deletion replay not recognized")
	}
	freshID := newID()
	_, e = outer.Exec(ctx, "SET LOCAL ROLE "+runtimeRole)
	enrollmentCheck(t, e, "verify runtime enrollment permissions")
	enrollmentCheck(t, q.InsertPasswordUser(ctx, identitysql.InsertPasswordUserParams{ID: freshID, Email: email, EmailLookup: email, FirstName: &first, LastName: &last, CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}}), "reuse deleted email with fresh identity")
	_, e = outer.Exec(ctx, "RESET ROLE")
	enrollmentCheck(t, e, "restore fixture administrator")
	var inherited int
	enrollmentCheck(t, outer.QueryRow(ctx, "SELECT count(*) FROM platform_administration.role_assignments WHERE user_id=$1", freshID).Scan(&inherited), "check fresh account grants")
	if inherited != 0 {
		t.Fatal("new identity inherited deleted user's grants")
	}
	// The retained profile must not poison the unfiltered directory read.
	rows, e := p.ListPlatformAdministratorProfiles(ctx, platformsql.ListPlatformAdministratorProfilesParams{PageLimit: 100})
	enrollmentCheck(t, e, "read directory after deletion")
	for _, row := range rows {
		if row.UserID == targetID {
			t.Fatal("deleted identity still appears in directory")
		}
	}
	// Direct database deletion of an active account is guarded independently.
	nested, e := outer.Begin(ctx)
	enrollmentCheck(t, e, "begin negative deletion check")
	_, e = nested.Exec(ctx, "DELETE FROM identity.users WHERE id=$1", actorID)
	if e == nil {
		t.Fatal("active user deletion was allowed")
	}
	nested.Rollback(ctx)
}
