package identity_test

import (
	"context"
	"errors"
	"testing"

	app "bidora/backend/internal/identity/application"
	persistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/postgres"
)

func TestManagedGoogleSignupPolicyKeepsLinkedSigninAndDeniesNewUsers(t *testing.T) {
	for _, existing := range []bool{false, true} {
		db := &googleDB{existing: existing}
		runner := &writeRunnerFake{db: db}
		resolver, err := persistence.NewGoogleIdentityResolver(runner)
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		if err = resolver.EnablePolicy(func(ctx context.Context, tx postgres.DBTX, address string) (bool, error) {
			calls++
			if tx != db || address != "googleuser@example.test" {
				t.Fatal("policy not supplied verified canonical identity in owner transaction")
			}
			return false, nil
		}); err != nil {
			t.Fatal(err)
		}
		result, err := resolver.ResolveGoogleIdentity(context.Background(), googleCommit(t))
		if calls != 1 {
			t.Fatal("commit omitted current policy")
		}
		if existing {
			if err != nil || result.Created || !runner.committed {
				t.Fatal("signup-disabled blocked linked sign-in", err)
			}
		} else if !errors.Is(err, app.ErrGoogleAuthenticationDenied) || runner.committed || len(db.calls) != 1 {
			t.Fatal("signup-disabled created identity", err)
		}
	}
}
func TestManagedGooglePolicyFailureBlocksIdentityAndExplicitLinkCommit(t *testing.T) {
	db := &googleDB{existing: true}
	runner := &writeRunnerFake{db: db}
	resolver, err := persistence.NewGoogleIdentityResolver(runner)
	if err != nil {
		t.Fatal(err)
	}
	deny := func(context.Context, postgres.DBTX, string) (bool, error) {
		return false, app.ErrGoogleAuthenticationDenied
	}
	if err = resolver.EnablePolicy(deny); err != nil {
		t.Fatal(err)
	}
	if _, err = resolver.ResolveGoogleIdentity(context.Background(), googleCommit(t)); !errors.Is(err, app.ErrGoogleAuthenticationDenied) || runner.committed || len(db.calls) != 0 {
		t.Fatal("disabled/domain/version policy bypassed", err)
	}
	linkDB := &googleLinkDB{}
	linkRunner := &writeRunnerFake{db: linkDB}
	link, err := persistence.NewGoogleLinkCommitter(linkRunner)
	if err != nil {
		t.Fatal(err)
	}
	if err = link.EnablePolicy(deny); err != nil {
		t.Fatal(err)
	}
	if err = link.CommitGoogleLink(context.Background(), validGoogleLinkCommit(t)); !errors.Is(err, app.ErrGoogleAuthenticationDenied) || linkRunner.committed || len(linkDB.calls) != 0 {
		t.Fatal("explicit link bypassed configuration policy", err)
	}
}
func TestManagedGoogleSignupDisabledStillAllowsExplicitAuthenticatedLink(t *testing.T) {
	db := &googleLinkDB{}
	runner := &writeRunnerFake{db: db}
	link, err := persistence.NewGoogleLinkCommitter(runner)
	if err != nil {
		t.Fatal(err)
	}
	if err = link.EnablePolicy(func(context.Context, postgres.DBTX, string) (bool, error) { return false, nil }); err != nil {
		t.Fatal(err)
	}
	if err = link.CommitGoogleLink(context.Background(), validGoogleLinkCommit(t)); err != nil || !runner.committed {
		t.Fatal("signup policy blocked an existing user's explicit authenticated link", err)
	}
}
