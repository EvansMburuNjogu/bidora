package identity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"bidora/backend/internal/identity/accounttype"
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/securitypolicy"
)

type classifiedSessionCommitter struct {
	committerFake
	administrator bool
}

func (c *classifiedSessionCommitter) SessionAdministrator(context.Context, app.ID) (bool, error) {
	return c.administrator, nil
}
func TestSessionIssuerSelectsAuthoritativeAccountPolicyWithoutUpgradingAssurance(t *testing.T) {
	for _, administrator := range []bool{false, true} {
		now := time.Now().UTC()
		committer := &classifiedSessionCommitter{administrator: administrator}
		issuer, _ := app.NewSessionIssuer(app.SecureSessionTokenGenerator{}, &loginIDsFake{id: identityID(7)}, loginClockFake{at: now}, committer)
		_, err := issuer.Issue(context.Background(), app.AuthenticatedSessionInput{UserID: identityID(1), CorrelationID: identityID(2), AuthEpoch: 1, Method: app.AuthenticationMethodPassword})
		idle, absolute, _ := securitypolicy.Expiries(context.Background(), now, administrator)
		if err != nil || committer.calls != 1 || !committer.commit.IdleExpiresAt.Equal(idle) || !committer.commit.AbsoluteExpiresAt.Equal(absolute) || committer.commit.AssuranceLevel != "AAL1" || committer.commit.AssuranceMethod != "PRIMARY_ONLY" {
			t.Fatal("account policy or actual proof changed", err)
		}
	}
}

type transitionAccountStore struct{ app.AccountTypeSelectionStore }

func (transitionAccountStore) GetAccountType(context.Context, app.ID) (accounttype.Type, error) {
	return accounttype.PlatformAdmin, nil
}

type transitionPortal struct{ calls int }

func (p *transitionPortal) ResolvePlatformPortal(context.Context, app.SessionPrincipal, time.Time) (app.PlatformPortalAccess, error) {
	p.calls++
	return app.PlatformPortalAccess{}, app.ErrPlatformAccessForbidden
}

func TestRotatedPrimaryOnlySessionRequiresExplicitReauthenticationWithoutInventingAuthority(t *testing.T) {
	now := time.Now().UTC()
	store := transitionAccountStore{}
	service, err := app.NewAccountTypeService(store, store, &loginIDsFake{id: identityID(7)}, loginClockFake{at: now})
	if err != nil {
		t.Fatal(err)
	}
	portal := &transitionPortal{}
	if err = service.EnablePlatformAccess(portal); err != nil {
		t.Fatal(err)
	}
	principal := app.SessionPrincipal{SessionID: identityID(1), UserID: identityID(2), AuthEpoch: 2, AssuranceLevel: "AAL1", AssuranceMethod: "PRIMARY_ONLY", AssuredAt: now, IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(2 * time.Hour)}
	result, err := service.Resolve(context.Background(), principal)
	if err != nil || result.Portal != nil || result.PlatformContextGeneration != nil || len(result.Capabilities) != 0 || result.AssuranceLevel != "AAL1" || len(result.RequiredActions) != 1 || result.RequiredActions[0] != "COMPLETE_PLATFORM_STEP_UP" || portal.calls != 0 {
		t.Fatalf("rotation was stranded or invented access: %+v %v", result, err)
	}
	principal.AssuranceLevel = "AAL2"
	principal.AssuranceMethod = "EMAIL_OTP"
	if _, err = service.Resolve(context.Background(), principal); !errors.Is(err, app.ErrPlatformAccessForbidden) || portal.calls != 1 {
		t.Fatal("fresh assurance bypassed actual platform authorization", err)
	}
}
