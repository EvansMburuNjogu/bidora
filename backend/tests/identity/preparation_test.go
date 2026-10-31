package identity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
)

type preparationReaderFake struct {
	facts identityapp.RecoveryAccountFacts
	err   error
	calls int
}

func (r *preparationReaderFake) GetRecoveryAccountFacts(context.Context, identityapp.ID) (identityapp.RecoveryAccountFacts, error) {
	r.calls++
	return r.facts, r.err
}

type mfaStatusFake struct {
	enrolled bool
	err      error
	calls    int
}

func (f *mfaStatusFake) HasActiveMFA(context.Context, identityapp.ID) (bool, error) {
	f.calls++
	return f.enrolled, f.err
}

type preparationClock struct{ at time.Time }

func (c preparationClock) Now() time.Time { return c.at }

func preparationPrincipal(now time.Time) identityapp.SessionPrincipal {
	var sessionID, userID identityapp.ID
	sessionID[15], userID[15] = 1, 2
	return identityapp.SessionPrincipal{SessionID: sessionID, UserID: userID, AuthEpoch: 3, AbsoluteExpiresAt: now.Add(11 * time.Hour)}
}

func TestMFAReadinessReflectsCurrentFactorState(t *testing.T) {
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	for _, enrolled := range []bool{false, true} {
		t.Run(map[bool]string{false: "not_enrolled", true: "enrolled"}[enrolled], func(t *testing.T) {
			reader := &preparationReaderFake{facts: identityapp.RecoveryAccountFacts{EmailVerified: true, PasswordCredentialPresent: true, GoogleIdentityPresent: true}}
			factors := &mfaStatusFake{enrolled: enrolled}
			service, err := identityapp.NewMFAReadinessServiceWithFactors(reader, factors, preparationClock{at: now})
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.Get(context.Background(), preparationPrincipal(now))
			if err != nil {
				t.Fatal(err)
			}
			if reader.calls != 1 || factors.calls != 1 || result.MFAEnrolled != enrolled || result.MFAEnrollmentAvailable == enrolled || result.StepUpAvailable != enrolled || result.RecoveryCodesAvailable != enrolled {
				t.Fatal("MFA readiness did not reflect current factor state", result)
			}
			if !result.StepUpPolicyConfigured || !result.SessionAssuranceProvenanceAvailable || result.SessionAgeSeconds != 3600 || !result.ObservedAt.Equal(now) {
				t.Fatal("MFA readiness policy/session facts drifted", result)
			}
		})
	}
}

func TestMFAReadinessFailsClosedWhenCurrentFactorStateIsUnavailable(t *testing.T) {
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	service, err := identityapp.NewMFAReadinessServiceWithFactors(&preparationReaderFake{}, &mfaStatusFake{err: errors.New("secret database diagnostic")}, preparationClock{at: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Get(context.Background(), preparationPrincipal(now)); err == nil || err.Error() != "mfa_readiness_unavailable" {
		t.Fatal("factor-state failure did not fail closed", err)
	}
}

func TestRecoveryPreparationReportsCurrentAccountPolicyWithoutLookupMaterial(t *testing.T) {
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	reader := &preparationReaderFake{facts: identityapp.RecoveryAccountFacts{EmailVerified: false, AccountDisabled: true, PasswordCredentialPresent: false, GoogleIdentityPresent: true}}
	service, err := identityapp.NewRecoveryPreparationService(reader, preparationClock{at: now}, false)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Get(context.Background(), preparationPrincipal(now))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"EMAIL_NOT_VERIFIED", "ACCOUNT_DISABLED", "PASSWORD_CREDENTIAL_NOT_PRESENT", "RECOVERY_DEPLOYMENT_NOT_CONFIGURED"}
	if reader.calls != 1 || result.RecoveryRequestAvailable || result.PasswordResetAvailable || !result.TokenPolicyConfigured || !result.DeliveryPolicyConfigured || !result.SessionEffectPolicyConfigured || len(result.Blockers) != len(want) {
		t.Fatal("recovery preparation policy drifted", result)
	}
	for i := range want {
		if result.Blockers[i] != want[i] {
			t.Fatal("recovery blocker order drifted", result.Blockers)
		}
	}
}
