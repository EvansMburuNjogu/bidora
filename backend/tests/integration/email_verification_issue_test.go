package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/email"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/identity/verification"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

type liveVerificationDelivery struct {
	token     verification.Token
	recipient email.Address
	expires   time.Time
	err       error
	calls     int
}

func (d *liveVerificationDelivery) DeliverEmailVerification(_ context.Context, recipient email.Address, token verification.Token, expires time.Time) error {
	d.calls++
	d.recipient = recipient
	d.token = token
	d.expires = expires
	return d.err
}

type liveIssueClock struct{}

func (liveIssueClock) Now() time.Time { return time.Now().UTC() }
func TestLiveEmailVerificationIssuanceResendAndAtomicConsumption(t *testing.T) {
	migrator := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	runtime := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	userID := identityapp.ID(uuid.New())
	emailValue := "verify-" + uuid.NewString() + "@example.invalid"
	now := time.Now().UTC().Add(-time.Minute)
	if err := migrator.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, err := db.Exec(ctx, `INSERT INTO identity.users (id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES ($1,$2,$3,NULL,NULL,1,$4,$4)`, uuid.UUID(userID), emailValue, emailValue, now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	issuer, err := identitypersistence.NewEmailVerificationIssuer(runtime)
	if err != nil {
		t.Fatal(err)
	}
	delivery := &liveVerificationDelivery{}
	service, err := identityapp.NewEmailVerificationIssueService(issuer, delivery, identityapp.SecureVerificationTokenGenerator{}, identityIDs{}, liveIssueClock{})
	if err != nil {
		t.Fatal(err)
	}
	input := identityapp.EmailVerificationIssueInput{UserID: userID, CorrelationID: identityapp.ID(uuid.New())}
	first, err := service.Issue(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	firstToken := delivery.token.RevealForDelivery()
	if firstToken == "" || delivery.recipient.Display() != emailValue || first.ExpiresAt.IsZero() {
		t.Fatal("live verification delivery boundary drifted")
	}
	input.CorrelationID = identityapp.ID(uuid.New())
	if _, err = service.Issue(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	secondToken := delivery.token.RevealForDelivery()
	if secondToken == firstToken {
		t.Fatal("resend reused raw token")
	}
	if count := rowCount(t, runtime, `SELECT count(*) FROM identity.challenges WHERE user_id=$1 AND purpose='EMAIL_VERIFY' AND consumed_at IS NULL AND invalidated_at IS NULL`, uuid.UUID(userID)); count != 1 {
		t.Fatal("resend did not leave exactly one active challenge", count)
	}
	if count := rowCount(t, runtime, `SELECT count(*) FROM identity.challenges WHERE user_id=$1 AND invalidated_at IS NOT NULL`, uuid.UUID(userID)); count != 1 {
		t.Fatal("resend did not invalidate prior challenge", count)
	}
	delivery.err = errors.New("synthetic delivery unavailable")
	input.CorrelationID = identityapp.ID(uuid.New())
	if _, err = service.Issue(context.Background(), input); err == nil || err.Error() != "verification_delivery_unavailable" {
		t.Fatal("delivery failure was reported as success", err)
	}
	thirdToken := delivery.token.RevealForDelivery()
	if thirdToken == secondToken {
		t.Fatal("failed delivery issuance reused token")
	}
	committer, err := identitypersistence.NewEmailVerificationCommitter(runtime)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := identityapp.NewEmailVerificationService(committer, identityIDs{}, liveIssueClock{})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := consumer.Verify(context.Background(), identityapp.EmailVerificationInput{Token: thirdToken, CorrelationID: identityapp.ID(uuid.New())})
	if err != nil {
		t.Fatal(err)
	}
	if verified.UserID != userID || verified.AuthEpoch != 1 {
		t.Fatal("issued token did not consume atomically", verified)
	}
	if count := rowCount(t, migrator, `SELECT count(*) FROM bidora_control_audit.audit_events WHERE target_id=$1 AND action='EmailVerificationIssued'`, uuid.UUID(userID)); count != 3 {
		t.Fatal("issuance audit count drifted", count)
	}
	if count := rowCount(t, migrator, `SELECT count(*) FROM bidora_control_audit.audit_events WHERE target_id=$1 AND action='EmailVerified'`, uuid.UUID(userID)); count != 1 {
		t.Fatal("verification audit count drifted", count)
	}
}
