package integration_test

import (
	"context"
	"testing"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	"bidora/backend/internal/authorization/invitation"
	authzpersistence "bidora/backend/internal/authorization/persistence"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

type liveInvitationDelivery struct {
	tokens     []invitation.Token
	recipients []string
	expires    []time.Time
	err        error
}

func (d *liveInvitationDelivery) DeliverTenantInvitation(_ context.Context, recipient string, token invitation.Token, expires time.Time) error {
	d.recipients = append(d.recipients, recipient)
	d.tokens = append(d.tokens, token)
	d.expires = append(d.expires, expires)
	return d.err
}
func (d *liveInvitationDelivery) DeliverTenantRoleInvitation(ctx context.Context, recipient string, _ authzapp.TenderpreneurRole, token invitation.Token, expires time.Time) error {
	return d.DeliverTenantInvitation(ctx, recipient, token, expires)
}

func TestLiveInvitationIssuanceAuthorizesDenyFirstAndInvalidatesResend(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	now := time.Now().UTC().Truncate(time.Microsecond)
	actorID := authzapp.ID(uuid.New())
	tenantID := authzapp.ID(uuid.New())
	membershipID := uuid.New()
	allowID := uuid.New()
	actorEmail := "invite-manager-" + uuid.NewString() + "@example.invalid"
	recipient := "Capture.User-" + uuid.NewString() + "@example.invalid"
	err := control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		if _, e := db.Exec(ctx, `INSERT INTO identity.users (id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES ($1,$2,$2,$3,NULL,1,$3,$3)`, uuid.UUID(actorID), actorEmail, now); e != nil {
			return e
		}
		if _, e := db.Exec(ctx, `INSERT INTO bidora_tenancy.tenants (id,tenant_type,state,name,country,timezone,locale,assignment_generation,version,created_at,updated_at) VALUES ($1,'TENDERPRENEUR','ACTIVE','Synthetic invite manager tenant','KE','Africa/Nairobi','en-KE',1,1,$2,$2)`, uuid.UUID(tenantID), now); e != nil {
			return e
		}
		if _, e := db.Exec(ctx, `INSERT INTO tenancy.memberships (id,user_id,tenant_id,state,context_generation,created_at,updated_at) VALUES ($1,$2,$3,'ACTIVE',1,$4,$4)`, membershipID, uuid.UUID(actorID), uuid.UUID(tenantID), now); e != nil {
			return e
		}
		_, e := db.Exec(ctx, `INSERT INTO tenancy.role_grants (id,membership_id,role_key,capability,effect,created_by,created_at) VALUES ($1,$2,'TENANT_MEMBERSHIP_MANAGER','tenant.memberships.manage','ALLOW',$3,$4)`, allowID, membershipID, uuid.UUID(actorID), now)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := authzpersistence.NewInvitationIssuer(control)
	if err != nil {
		t.Fatal(err)
	}
	delivery := &liveInvitationDelivery{}
	clock := invitationAcceptanceClock{now.Add(time.Minute)}
	service, err := authzapp.NewInvitationIssueService(issuer, delivery, authzapp.SecureInvitationTokenGenerator{}, invitationAcceptanceIDs{}, clock)
	if err != nil {
		t.Fatal(err)
	}
	input := authzapp.InvitationIssueInput{ActorID: actorID, TenantID: tenantID, CorrelationID: authzapp.ID(uuid.New()), ContextGeneration: 2, Email: recipient, Role: authzapp.BidManager}
	first, err := service.Issue(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	input.CorrelationID = authzapp.ID(uuid.New())
	second, err := service.Issue(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if first.InvitationID.IsZero() || second.InvitationID.IsZero() || first.InvitationID == second.InvitationID || len(delivery.tokens) != 2 || delivery.tokens[0].RevealForDelivery() == delivery.tokens[1].RevealForDelivery() {
		t.Fatal("invitation resend token/result drifted")
	}
	lookup := "capture.user-" + recipient[len("Capture.User-"):]
	if count := rowCount(t, control, `SELECT count(*) FROM tenancy.invitations WHERE tenant_id=$1 AND email_lookup=$2 AND invalidated_at IS NULL AND consumed_at IS NULL`, uuid.UUID(tenantID), lookup); count != 1 {
		t.Fatal("invitation resend active count drifted", count)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM tenancy.invitations WHERE tenant_id=$1 AND email_lookup=$2 AND invalidated_at IS NOT NULL`, uuid.UUID(tenantID), lookup); count != 1 {
		t.Fatal("invitation resend did not invalidate prior token", count)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM bidora_control_audit.audit_events WHERE tenant_id=$1 AND actor_id=$2 AND action='InvitationIssued'`, uuid.UUID(tenantID), uuid.UUID(actorID)); count != 2 {
		t.Fatal("invitation issue audit drifted", count)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM bidora_control_eventing.outbox_events WHERE tenant_id=$1 AND actor_id=$2 AND event_type='invitation.issued.v1'`, uuid.UUID(tenantID), uuid.UUID(actorID)); count != 2 {
		t.Fatal("invitation issue outbox drifted", count)
	}
	if err = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, e := db.Exec(ctx, `INSERT INTO tenancy.role_grants (id,membership_id,role_key,capability,effect,created_by,created_at) VALUES ($1,$2,'TENANT_MEMBERSHIP_MANAGER','tenant.memberships.manage','DENY',$3,$4)`, uuid.New(), membershipID, uuid.UUID(actorID), now.Add(2*time.Minute))
		return e
	}); err != nil {
		t.Fatal(err)
	}
	input.ContextGeneration = 3
	input.CorrelationID = authzapp.ID(uuid.New())
	if denied, issueErr := service.Issue(context.Background(), input); issueErr == nil || issueErr.Error() != "invitation_not_authorized" || denied != (authzapp.InvitationIssueResult{}) {
		t.Fatal("active invitation deny did not win", denied, issueErr)
	}
	if len(delivery.tokens) != 2 {
		t.Fatal("denied invitation reached delivery")
	}
}
