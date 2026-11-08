package integration_test

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"testing"
	"time"

	platformapp "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	platformpersistence "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

// This fixture is deliberately confined to one rollback-only PostgreSQL transaction.
// Its synthetic MFA/session records are NOT evidence of a real TOTP ceremony or
// approved invitation. It exercises production SQL adapters without publishing an
// artificial owner, irreversible audit record, or usable authentication material.
type rollbackPlatformRunner struct{ db postgres.DBTX }

func (r rollbackPlatformRunner) WithinTransaction(ctx context.Context, fn func(context.Context, postgres.DBTX) error) error {
	return fn(ctx, r.db)
}
func (r rollbackPlatformRunner) WithinConsistentReadTransaction(ctx context.Context, fn func(context.Context, postgres.DBTX) error) error {
	return fn(ctx, r.db)
}

var errPlatformFixtureRollback = errors.New("synthetic platform fixture rolled back")

func TestLivePlatformSecondOwnerRollbackFixture(t *testing.T) {
	pool := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	var fixtureUser, fixtureTarget, fixtureChange uuid.UUID
	err := pool.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		now := time.Now().UTC().Truncate(time.Microsecond)
		var original uuid.UUID
		var epoch, checkerGeneration int64
		if e := db.QueryRow(ctx, `SELECT u.id,u.auth_epoch,p.context_generation FROM identity.users u JOIN platform_administration.administrator_profiles p ON p.user_id=u.id JOIN platform_administration.role_assignments r ON r.user_id=u.id AND r.role_key='PLATFORM_OWNER' AND r.state='ACTIVE' WHERE u.account_type='PLATFORM_ADMIN' AND u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL AND EXISTS (SELECT 1 FROM identity.mfa_factors f WHERE f.user_id=u.id AND f.state='ACTIVE') ORDER BY r.assigned_at LIMIT 1`).Scan(&original, &epoch, &checkerGeneration); e != nil {
			return fmt.Errorf("eligible existing synthetic owner unavailable: %w", e)
		}
		second, target, makerSession, checkerSession := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		fixtureUser, fixtureTarget = second, target
		for _, id := range []uuid.UUID{second, target} {
			email := "rollback-platform-" + id.String() + "@example.invalid"
			if _, e := db.Exec(ctx, `INSERT INTO identity.users(id,email,email_lookup,email_verified_at,account_type,created_at,updated_at) VALUES($1,$2,$2,$3,'PLATFORM_ADMIN',$3,$3)`, id, email, now); e != nil {
				return e
			}
			if _, e := db.Exec(ctx, `INSERT INTO platform_administration.administrator_profiles(user_id,created_at,updated_at) VALUES($1,$2,$2)`, id, now); e != nil {
				return e
			}
			// Non-decryptable synthetic factor: used only for owner-continuity reads in the rolled-back transaction.
			if _, e := db.Exec(ctx, `INSERT INTO identity.mfa_factors(id,user_id,state,algorithm,digits,period_seconds,secret_ciphertext,secret_nonce,key_version,created_at,pending_expires_at,activated_at) VALUES($1,$2,'ACTIVE','TOTP_SHA1',6,30,$3,$4,1,$5::timestamptz,$5::timestamptz+interval '10 minutes',$5::timestamptz)`, uuid.New(), id, randomFixtureBytes(t, 32), randomFixtureBytes(t, 12), now); e != nil {
				return e
			}
		}
		if _, e := db.Exec(ctx, `INSERT INTO platform_administration.role_assignments(id,user_id,role_key,state,source,assigned_by,assigned_at,reason) VALUES($1,$2,'PLATFORM_OWNER','ACTIVE','OWNER_CHANGE',$3,$4,'Rollback-only synthetic second owner')`, uuid.New(), second, original, now); e != nil {
			return e
		}
		for _, s := range []struct {
			id, user uuid.UUID
			epoch    int64
		}{{makerSession, second, 1}, {checkerSession, original, epoch}} {
			if _, e := db.Exec(ctx, `INSERT INTO identity.sessions(id,user_id,token_hash,csrf_hash,auth_epoch,created_at,last_seen_at,idle_expires_at,absolute_expires_at,assurance_level,assurance_method,assured_at,idle_timeout_seconds,absolute_timeout_seconds) VALUES($1,$2,$3,$4,$5,$6::timestamptz,$6::timestamptz,$6::timestamptz+interval '30 minutes',$6::timestamptz+interval '12 hours','AAL2','TOTP',$6::timestamptz,1800,43200)`, s.id, s.user, randomFixtureBytes(t, 32), randomFixtureBytes(t, 32), s.epoch, now); e != nil {
				return e
			}
		}
		store, e := platformpersistence.NewStore(rollbackPlatformRunner{db})
		if e != nil {
			return e
		}
		service, e := platformapp.NewRoleChangeService(store, platformIntegrationIDs{}, platformIntegrationClock{})
		if e != nil {
			return e
		}
		maker, subject := platformapp.ID(second), platformapp.ID(target)
		roleInput := platformapp.RoleChangeInput{ActorID: maker, SessionID: platformapp.ID(makerSession), TargetUserID: subject, CorrelationID: platformapp.ID(uuid.New()), IdempotencyKey: platformapp.ID(uuid.New()), PlatformContextGeneration: 1, ExpectedAdministratorVersion: 1, ExpectedTargetContextGeneration: 1, Action: platformapp.AssignRole, Role: domain.PlatformOwner, Reason: "Rollback fixture owner proposal"}
		proposal, e := service.Change(ctx, roleInput)
		if e != nil || proposal.State != "EXECUTED" {
			return fmt.Errorf("owner proposal: %v %s", e, proposal.State)
		}
		fixtureChange = uuid.UUID(proposal.ChangeID)
		replay, e := service.Change(ctx, roleInput)
		if e != nil || !replay.Replay || replay.ChangeID != proposal.ChangeID {
			return fmt.Errorf("owner proposal replay: %v", e)
		}
		wrong := roleInput
		wrong.Reason = "Changed request with same key"
		if _, e = service.Change(ctx, wrong); !errors.Is(e, platformapp.ErrInvitationConflict) {
			return fmt.Errorf("same-key changed body must conflict: %v", e)
		}
		var roleCount, auditCount, eventCount int64
		if e = db.QueryRow(ctx, `SELECT (SELECT count(*) FROM platform_administration.role_assignments WHERE user_id=$1 AND role_key='PLATFORM_OWNER' AND state='ACTIVE'),(SELECT count(*) FROM bidora_control_audit.audit_events WHERE target_id=$2),(SELECT count(*) FROM bidora_control_eventing.outbox_events WHERE aggregate_id=$2)`, target, uuid.UUID(proposal.ChangeID)).Scan(&roleCount, &auditCount, &eventCount); e != nil {
			return e
		}
		if roleCount != 1 || auditCount < 1 || eventCount < 1 {
			return fmt.Errorf("owner approval lacks transactional role/audit/outbox evidence: %d %d %d", roleCount, auditCount, eventCount)
		}
		var version, generation int64
		if e = db.QueryRow(ctx, `SELECT version,context_generation FROM platform_administration.administrator_profiles WHERE user_id=$1`, target).Scan(&version, &generation); e != nil {
			return e
		}
		stale := roleInput
		stale.IdempotencyKey = platformapp.ID(uuid.New())
		stale.ExpectedAdministratorVersion = 1
		if _, e = service.Change(ctx, stale); !errors.Is(e, platformapp.ErrConcurrentChange) {
			return fmt.Errorf("stale version must conflict: %v", e)
		}
		identity := platformapp.IdentityStateChangeInput{ActorID: maker, SessionID: platformapp.ID(makerSession), TargetUserID: subject, CorrelationID: platformapp.ID(uuid.New()), IdempotencyKey: platformapp.ID(uuid.New()), PlatformContextGeneration: 1, ExpectedAdministratorVersion: version, ExpectedTargetContextGeneration: generation, Action: platformapp.DisableAdministrator, Reason: "Rollback fixture disable proposal"}
		pending, e := service.ChangeIdentityState(ctx, identity)
		if e != nil || pending.State != "EXECUTED" {
			return fmt.Errorf("disable proposal: %v %s", e, pending.State)
		}
		var disabled bool
		if e = db.QueryRow(ctx, `SELECT disabled_at IS NOT NULL FROM identity.users WHERE id=$1`, target).Scan(&disabled); e != nil || !disabled {
			return fmt.Errorf("approved disable not applied: %v", e)
		}
		if e = db.QueryRow(ctx, `SELECT (SELECT count(*) FROM bidora_control_audit.audit_events WHERE target_id=$1),(SELECT count(*) FROM bidora_control_eventing.outbox_events WHERE aggregate_id=$1)`, uuid.UUID(pending.ChangeID)).Scan(&auditCount, &eventCount); e != nil || auditCount < 1 || eventCount < 1 {
			return fmt.Errorf("disable lacks transactional audit/outbox: %v %d %d", e, auditCount, eventCount)
		}
		return errPlatformFixtureRollback
	})
	if !errors.Is(err, errPlatformFixtureRollback) {
		t.Fatal("rollback fixture failed", err)
	}
	var retained int64
	err = pool.WithinConsistentReadTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		return db.QueryRow(ctx, `SELECT (SELECT count(*) FROM identity.users WHERE id IN ($1,$2)) + (SELECT count(*) FROM platform_administration.administrator_changes WHERE id=$3) + (SELECT count(*) FROM bidora_control_audit.audit_events WHERE target_id=$3) + (SELECT count(*) FROM bidora_control_eventing.outbox_events WHERE aggregate_id=$3)`, fixtureUser, fixtureTarget, fixtureChange).Scan(&retained)
	})
	if err != nil || retained != 0 {
		t.Fatalf("rollback retained synthetic authority or evidence: rows=%d error=%v", retained, err)
	}
}

func randomFixtureBytes(t *testing.T, size int) []byte {
	t.Helper()
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return value
}
