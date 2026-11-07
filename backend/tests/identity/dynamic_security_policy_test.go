package identity_test

import (
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attemptredis"
	persistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/identity/securitypolicy"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestPasswordResetRejectsBelowConfiguredMinimumBeforeHashOrCommit(t *testing.T) {
	policy := securitypolicy.Default()
	policy.MinimumPasswordBytes = 24
	ctx, e := securitypolicy.With(context.Background(), policy)
	if e != nil {
		t.Fatal(e)
	}
	passwords := &registrationPasswordFake{phc: "$argon2id$v=19$synthetic-reset"}
	commit := &recoveryResetCommitFake{}
	service, _ := app.NewPasswordRecoveryResetService(commit, passwords, &sequenceIDs{values: []app.ID{identityID(1), identityID(3)}}, loginClockFake{at: time.Now().UTC()})
	token := issueToken(t)
	e = service.Reset(ctx, app.PasswordRecoveryResetInput{Token: token.RevealForDelivery(), NewPassword: []byte("replacement password"), CorrelationID: identityID(2)})
	if e == nil || len(passwords.hashed) != 0 || commit.calls != 0 {
		t.Fatal("configured minimum did not protect password reset", e)
	}
}

func TestAttemptReservationsUseConfiguredBudgetWithoutChangingNetworkProtection(t *testing.T) {
	runner := &scriptRunnerFake{results: []scriptResult{{value: int64(1)}, {value: int64(1)}}}
	controller, e := attemptredis.NewController(runner, redisRandom(3), "dynamic")
	if e != nil {
		t.Fatal(e)
	}
	policy := securitypolicy.Default()
	policy.FailedLoginAttempts = 3
	policy.FailureWindowSeconds = 120
	policy.LockoutSeconds = 600
	ctx, e := securitypolicy.With(context.Background(), policy)
	if e != nil {
		t.Fatal(e)
	}
	reservation, e := controller.Begin(ctx, attemptScopes())
	if e != nil {
		t.Fatal(e)
	}
	if e = reservation.Finish(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	if fmt.Sprint(runner.calls[1].args) != fmt.Sprint([]any{"0", int64(120000), 3, int64(600000)}) {
		t.Fatal("identity settings ignored", runner.calls[1].args)
	}
	if fmt.Sprint(runner.calls[0].args) != fmt.Sprint([]any{int64(60000), int64(900000), 30, 100, int64(900000), int64(300000)}) {
		t.Fatal("fixed network protections changed")
	}
}
func TestAdministratorSessionReadsBindFreshPolicyAndFailClosed(t *testing.T) {
	now := time.Now().UTC()
	db := &sessionDB{csrf: make([]byte, 32), observed: now}
	reader, _ := persistence.NewSessionReader(&readRunner{db: db})
	policy := securitypolicy.Default()
	policy.AdminIdleSeconds = 600
	policy.AdminAbsoluteSeconds = 3600
	var dependency error
	if e := reader.EnableSecurityPolicy(func(_ context.Context, scoped postgres.DBTX) (securitypolicy.Policy, error) {
		if scoped != db {
			t.Fatal("policy escaped session transaction")
		}
		return policy, dependency
	}); e != nil {
		t.Fatal(e)
	}
	var hash [32]byte
	hash[0] = 1
	if _, e := reader.GetSession(context.Background(), hash, now); e != nil || db.args[4] != int32(600) || db.args[5] != int32(3600) {
		t.Fatal("administrator limits ignored", e, db.args)
	}
	policy.AdminIdleSeconds = 900
	if _, e := reader.GetSession(context.Background(), hash, now); e != nil || db.args[4] != int32(900) {
		t.Fatal("administrator policy was cached", e)
	}
	dependency = errors.New("policy unavailable")
	db.sql = ""
	if _, e := reader.GetSession(context.Background(), hash, now); !errors.Is(e, dependency) || db.sql != "" {
		t.Fatal("missing policy did not fail closed", e)
	}
}
