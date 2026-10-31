package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	platformapp "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	platformpersistence "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

type platformIntegrationClock struct{}

func (platformIntegrationClock) Now() time.Time { return time.Now().UTC() }

type platformIntegrationIDs struct{}

func (platformIntegrationIDs) NewID() (platformapp.ID, error) { return platformapp.ID(uuid.New()), nil }

// This verification uses the existing isolated synthetic owner without inserting
// identities, sessions, roles or token material. Denied operations must remain read-only.
func TestLivePlatformAdministratorCurrentGovernanceAndSelfChangeDenial(t *testing.T) {
	pool := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	ctx := context.Background()
	var user, session uuid.UUID
	var version, generation int64
	err := pool.WithinConsistentReadTransaction(ctx, func(ctx context.Context, db postgres.DBTX) error {
		return db.QueryRow(ctx, `SELECT p.user_id,p.version,p.context_generation,s.id
			FROM platform_administration.administrator_profiles p
			JOIN identity.users u ON u.id=p.user_id
			JOIN identity.sessions s ON s.user_id=p.user_id AND s.auth_epoch=u.auth_epoch
			JOIN platform_administration.role_assignments r ON r.user_id=p.user_id AND r.role_key='PLATFORM_OWNER' AND r.state='ACTIVE'
			WHERE u.account_type='PLATFORM_ADMIN' AND u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL
			  AND s.revoked_at IS NULL AND s.idle_expires_at>now() AND s.absolute_expires_at>now()
			  AND s.assurance_level='AAL2' AND s.assurance_method='TOTP' AND s.assured_at>now()-interval '15 minutes'
			ORDER BY s.assured_at DESC LIMIT 1`).Scan(&user, &version, &generation, &session)
	})
	if err != nil {
		t.Skip("no currently eligible synthetic Platform Owner session for read-only live verification")
	}
	observer := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	store, err := platformpersistence.NewStore(pool)
	if err != nil {
		t.Fatal(err)
	}
	reads, err := platformapp.NewGovernanceReadService(store)
	if err != nil {
		t.Fatal(err)
	}
	authority := platformapp.ReadAuthority{ActorID: platformapp.ID(user), SessionID: platformapp.ID(session), PlatformContextGeneration: generation, ObservedAt: time.Now().UTC()}
	if _, err = reads.Administrator(ctx, authority, platformapp.ID(user)); err != nil {
		t.Fatal("current administrator detail read failed", err)
	}
	if _, err = reads.Changes(ctx, authority, nil, platformapp.PageCursor{Limit: 10}); err != nil {
		t.Fatal("current governance queue read failed", err)
	}
	counts := func() [3]int64 {
		t.Helper()
		var result [3]int64
		err := observer.WithinConsistentReadTransaction(ctx, func(ctx context.Context, db postgres.DBTX) error {
			return db.QueryRow(ctx, `SELECT (SELECT count(*) FROM platform_administration.commands WHERE actor_user_id=$1),
				(SELECT count(*) FROM bidora_control_audit.audit_events WHERE actor_id=$1),
				(SELECT count(*) FROM bidora_control_eventing.outbox_events WHERE actor_id=$1)`, user).Scan(&result[0], &result[1], &result[2])
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	before := counts()
	changes, err := platformapp.NewRoleChangeService(store, platformIntegrationIDs{}, platformIntegrationClock{})
	if err != nil {
		t.Fatal(err)
	}
	base := platformapp.RoleChangeInput{ActorID: platformapp.ID(user), SessionID: platformapp.ID(session), TargetUserID: platformapp.ID(user), CorrelationID: platformapp.ID(uuid.New()), IdempotencyKey: platformapp.ID(uuid.New()), PlatformContextGeneration: generation, ExpectedAdministratorVersion: version, ExpectedTargetContextGeneration: generation, Action: platformapp.AssignRole, Role: domain.OperationsAdministrator, Reason: "Synthetic denied self-assignment"}
	if _, err = changes.Change(ctx, base); !errors.Is(err, platformapp.ErrPlatformAccessForbidden) {
		t.Fatalf("self-assignment must be denied: %v", err)
	}
	identity := platformapp.IdentityStateChangeInput{ActorID: platformapp.ID(user), SessionID: platformapp.ID(session), TargetUserID: platformapp.ID(user), CorrelationID: platformapp.ID(uuid.New()), IdempotencyKey: platformapp.ID(uuid.New()), PlatformContextGeneration: generation, ExpectedAdministratorVersion: version, ExpectedTargetContextGeneration: generation, Action: platformapp.DisableAdministrator, Reason: "Synthetic denied self-disable"}
	if _, err = changes.ChangeIdentityState(ctx, identity); !errors.Is(err, platformapp.ErrPlatformAccessForbidden) {
		t.Fatalf("self-disable must be denied: %v", err)
	}
	if after := counts(); after != before {
		t.Fatalf("denied commands changed command/audit/outbox counts: before=%v after=%v", before, after)
	}
}
