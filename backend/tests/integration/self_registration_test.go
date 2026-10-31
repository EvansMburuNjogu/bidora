package integration_test

import (
	"context"
	"testing"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/password"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"github.com/google/uuid"
)

func TestLiveSelfRegistrationCreatesUnverifiedIdentityWithoutMembership(t *testing.T) {
	runtime := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	observer := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	hasher, err := password.New(password.PilotPolicy())
	if err != nil {
		t.Fatal(err)
	}
	committer, err := identitypersistence.NewSelfRegistrationCommitter(runtime)
	if err != nil {
		t.Fatal(err)
	}
	service, err := identityapp.NewSelfRegistrationService(hasher, identityIDs{}, liveIssueClock{}, committer)
	if err != nil {
		t.Fatal(err)
	}
	emailValue := "register-" + uuid.NewString() + "@example.invalid"
	input := identityapp.SelfRegistrationInput{FirstName: "Amina", LastName: "Njoroge", Email: emailValue, Password: []byte("synthetic registration password"), CorrelationID: identityapp.ID(uuid.New())}
	result, err := service.Register(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Accepted || !result.Created || result.UserID.IsZero() {
		t.Fatal("live self registration failed", result)
	}
	if count := rowCount(t, runtime, `SELECT count(*) FROM identity.users WHERE id=$1 AND email_lookup=$2 AND first_name='Amina' AND last_name='Njoroge' AND email_verified_at IS NULL AND disabled_at IS NULL AND auth_epoch=1`, uuid.UUID(result.UserID), emailValue); count != 1 {
		t.Fatal("unverified user persistence drifted", count)
	}
	if count := rowCount(t, runtime, `SELECT count(*) FROM identity.authentication_identities WHERE user_id=$1 AND credential_version=1`, uuid.UUID(result.UserID)); count != 1 {
		t.Fatal("password identity persistence drifted", count)
	}
	if count := rowCount(t, observer, `SELECT count(*) FROM bidora_control_audit.audit_events WHERE target_id=$1 AND action='UserSelfRegistered'`, uuid.UUID(result.UserID)); count != 1 {
		t.Fatal("registration audit persistence drifted", count)
	}
	if count := rowCount(t, runtime, `SELECT count(*) FROM tenancy.memberships WHERE user_id=$1`, uuid.UUID(result.UserID)); count != 0 {
		t.Fatal("self registration granted tenant authority", count)
	}
	duplicate := input
	duplicate.Password = []byte("different synthetic password")
	duplicate.CorrelationID = identityapp.ID(uuid.New())
	duplicateResult, err := service.Register(context.Background(), duplicate)
	if err != nil {
		t.Fatal(err)
	}
	if !duplicateResult.Accepted || duplicateResult.Created || !duplicateResult.UserID.IsZero() {
		t.Fatal("duplicate identity was disclosed", duplicateResult)
	}
	if count := rowCount(t, runtime, `SELECT count(*) FROM identity.users WHERE email_lookup=$1`, emailValue); count != 1 {
		t.Fatal("duplicate registration created another user", count)
	}
	if count := rowCount(t, observer, `SELECT count(*) FROM bidora_control_audit.audit_events WHERE target_id=$1 AND action='UserSelfRegistered'`, uuid.UUID(result.UserID)); count != 1 {
		t.Fatal("duplicate registration appended success audit", count)
	}
}
