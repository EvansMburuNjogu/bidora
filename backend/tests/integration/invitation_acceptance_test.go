package integration_test

import (
	"context"
	"sync"
	"testing"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	"bidora/backend/internal/authorization/invitation"
	authzpersistence "bidora/backend/internal/authorization/persistence"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

type invitationAcceptanceIDs struct{}

func (invitationAcceptanceIDs) NewID() (authzapp.ID, error) {
	value, err := uuid.NewRandom()
	return authzapp.ID(value), err
}

type invitationAcceptanceClock struct{ at time.Time }

func (c invitationAcceptanceClock) Now() time.Time { return c.at }

func TestLiveInvitationAcceptanceCreatesFixedRoleBundleAtomically(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := authzapp.ID(uuid.New())
	inviterID := uuid.New()
	tenantID := uuid.New()
	invitationID := uuid.New()
	emailValue := "invitee-" + uuid.NewString() + "@example.invalid"
	token, err := invitation.Generate()
	if err != nil {
		t.Fatal(err)
	}
	hash := token.Hash()
	err = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		for _, row := range []struct {
			id    uuid.UUID
			email string
		}{{uuid.UUID(userID), emailValue}, {inviterID, "inviter-" + uuid.NewString() + "@example.invalid"}} {
			if _, execErr := db.Exec(ctx, `INSERT INTO identity.users (id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES ($1,$2,$2,$3,NULL,1,$3,$3)`, row.id, row.email, now); execErr != nil {
				return execErr
			}
		}
		if _, execErr := db.Exec(ctx, `INSERT INTO bidora_tenancy.tenants (id,tenant_type,state,name,country,timezone,locale,assignment_generation,version,created_at,updated_at) VALUES ($1,'TENDERPRENEUR','ACTIVE','Synthetic invitation tenant','KE','Africa/Nairobi','en-KE',1,1,$2,$2)`, tenantID, now); execErr != nil {
			return execErr
		}
		_, execErr := db.Exec(ctx, `INSERT INTO tenancy.invitations (id,tenant_id,email,email_lookup,token_hash,role_key,capability,created_by,created_at,expires_at) VALUES ($1,$2,$3,$3,$4,'BID_MANAGER','ROLE_BUNDLE_V2',$5,$6::timestamptz,$6::timestamptz+interval '72 hours')`, invitationID, tenantID, emailValue, hash[:], inviterID, now)
		return execErr
	})
	if err != nil {
		t.Fatal(err)
	}
	committer, err := authzpersistence.NewInvitationAcceptanceCommitter(control)
	if err != nil {
		t.Fatal(err)
	}
	service, err := authzapp.NewInvitationAcceptanceService(committer, invitationAcceptanceIDs{}, invitationAcceptanceClock{now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	input := authzapp.InvitationAcceptanceInput{Token: token.RevealForDelivery(), UserID: userID, CorrelationID: authzapp.ID(uuid.New())}
	result, err := service.Accept(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.MembershipID.IsZero() || result.TenantID != authzapp.ID(tenantID) || result.Role != authzapp.BidManager || result.ContextGeneration != 16 {
		t.Fatal("live invitation result drifted", result)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM tenancy.memberships AS m JOIN tenancy.role_grants AS g ON g.membership_id=m.id JOIN tenancy.invitations AS i ON i.tenant_id=m.tenant_id AND i.accepted_by=m.user_id WHERE m.id=$1 AND m.user_id=$2 AND m.tenant_id=$3 AND m.state='ACTIVE' AND m.context_generation=16 AND g.role_key='BID_MANAGER' AND g.effect='ALLOW' AND g.revoked_at IS NULL AND i.id=$4 AND i.consumed_at IS NOT NULL AND i.invalidated_at IS NULL`, uuid.UUID(result.MembershipID), uuid.UUID(userID), tenantID, invitationID); count != 15 {
		t.Fatal("live invitation role bundle persistence drifted", count)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM bidora_control_audit.audit_events WHERE tenant_id=$1 AND actor_id=$2 AND action='InvitationAccepted' AND target_id=$3`, tenantID, uuid.UUID(userID), uuid.UUID(result.MembershipID)); count != 1 {
		t.Fatal("invitation audit missing", count)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM bidora_control_eventing.outbox_events WHERE tenant_id=$1 AND actor_id=$2 AND aggregate_id=$3 AND aggregate_version=16 AND event_type='membership.granted.v1' AND payload->>'membership_id'=$3::text`, tenantID, uuid.UUID(userID), uuid.UUID(result.MembershipID)); count != 1 {
		t.Fatal("membership outbox missing", count)
	}
	if replay, acceptErr := service.Accept(context.Background(), authzapp.InvitationAcceptanceInput{Token: input.Token, UserID: userID, CorrelationID: authzapp.ID(uuid.New())}); acceptErr == nil || acceptErr.Error() != "invitation_invalid" || replay != (authzapp.InvitationAcceptanceResult{}) {
		t.Fatal("invitation replay disclosed state", replay, acceptErr)
	}
}

func TestLiveInvitationAcceptanceAllowsOnlyOneConcurrentConsumer(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := authzapp.ID(uuid.New())
	inviterID := uuid.New()
	tenantID := uuid.New()
	emailValue := "invite-race-" + uuid.NewString() + "@example.invalid"
	token, err := invitation.Generate()
	if err != nil {
		t.Fatal(err)
	}
	hash := token.Hash()
	err = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		if _, e := db.Exec(ctx, `INSERT INTO identity.users (id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES ($1,$2,$2,$3,NULL,1,$3,$3),($4,$5,$5,$3,NULL,1,$3,$3)`, uuid.UUID(userID), emailValue, now, inviterID, "inviter-race-"+uuid.NewString()+"@example.invalid"); e != nil {
			return e
		}
		if _, e := db.Exec(ctx, `INSERT INTO bidora_tenancy.tenants (id,tenant_type,state,name,country,timezone,locale,assignment_generation,version,created_at,updated_at) VALUES ($1,'TENDERPRENEUR','ACTIVE','Synthetic race tenant','KE','Africa/Nairobi','en-KE',1,1,$2,$2)`, tenantID, now); e != nil {
			return e
		}
		_, e := db.Exec(ctx, `INSERT INTO tenancy.invitations (id,tenant_id,email,email_lookup,token_hash,role_key,capability,created_by,created_at,expires_at) VALUES ($1,$2,$3,$3,$4,'BID_MANAGER','ROLE_BUNDLE_V2',$5,$6::timestamptz,$6::timestamptz+interval '72 hours')`, uuid.New(), tenantID, emailValue, hash[:], inviterID, now)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	committer, _ := authzpersistence.NewInvitationAcceptanceCommitter(control)
	input := authzapp.InvitationAcceptanceInput{Token: token.RevealForDelivery(), UserID: userID}
	var wg sync.WaitGroup
	outcomes := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := input
			local.CorrelationID = authzapp.ID(uuid.New())
			service, _ := authzapp.NewInvitationAcceptanceService(committer, invitationAcceptanceIDs{}, invitationAcceptanceClock{now.Add(time.Minute)})
			_, acceptErr := service.Accept(context.Background(), local)
			outcomes <- acceptErr
		}()
	}
	wg.Wait()
	close(outcomes)
	success, invalid := 0, 0
	for outcome := range outcomes {
		if outcome == nil {
			success++
		} else if outcome.Error() == "invitation_invalid" {
			invalid++
		} else {
			t.Fatal(outcome)
		}
	}
	if success != 1 || invalid != 1 {
		t.Fatal("concurrent invitation consumption drifted", success, invalid)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM tenancy.memberships WHERE user_id=$1 AND tenant_id=$2 AND state='ACTIVE'`, uuid.UUID(userID), tenantID); count != 1 {
		t.Fatal("concurrent invitation created duplicate membership", count)
	}
}
