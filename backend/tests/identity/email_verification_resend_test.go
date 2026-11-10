package identity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
)

func resendService(t *testing.T, credentials *credentialReaderFake, attempts *registrationAttemptsFake, keyer *registrationKeyerFake, verification *verificationIssueFake) *identityapp.EmailVerificationResendService {
	t.Helper()
	service, err := identityapp.NewEmailVerificationResendService(credentials, attempts, keyer, verification)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func resendInput(email string) identityapp.EmailVerificationResendInput {
	var network attempt.Key
	network[31] = 2
	return identityapp.EmailVerificationResendInput{Email: email, CorrelationID: identityID(3), NetworkScope: network}
}

func TestVerificationResendLimitsThenIssuesOnlyForEligibleUnverifiedPasswordIdentity(t *testing.T) {
	var identityKey attempt.Key
	identityKey[31] = 1
	credentials := &credentialReaderFake{record: identityapp.CredentialRecord{UserID: identityID(4)}}
	attempts := &registrationAttemptsFake{}
	keyer := &registrationKeyerFake{key: identityKey}
	verification := &verificationIssueFake{}
	result, err := resendService(t, credentials, attempts, keyer, verification).Resend(context.Background(), resendInput("User@example.test"))
	if err != nil || !result.Accepted || !result.VerificationIssued || attempts.calls != 1 || attempts.scopes.Identity != identityKey || keyer.lookup != "user@example.test" || len(credentials.lookups) != 1 || credentials.lookups[0] != "user@example.test" || verification.calls != 1 || verification.input.UserID != identityID(4) {
		t.Fatal("verification resend orchestration drifted", result, err)
	}
}

func TestVerificationResendConcealsIneligibleAndAmbiguousTargets(t *testing.T) {
	verifiedAt := time.Now().UTC()
	disabledAt := verifiedAt
	for _, tc := range []struct {
		name        string
		email       string
		record      identityapp.CredentialRecord
		lookupErr   error
		deliveryErr error
		wantLookups int
		wantIssues  int
	}{
		{name: "malformed", email: "bad", wantLookups: 0},
		{name: "unknown", email: "unknown@example.test", lookupErr: identityapp.ErrCredentialNotFound, wantLookups: 1},
		{name: "verified", email: "user@example.test", record: identityapp.CredentialRecord{UserID: identityID(4), EmailVerifiedAt: &verifiedAt}, wantLookups: 1},
		{name: "disabled", email: "user@example.test", record: identityapp.CredentialRecord{UserID: identityID(4), DisabledAt: &disabledAt}, wantLookups: 1},
		{name: "delivery ambiguous", email: "user@example.test", record: identityapp.CredentialRecord{UserID: identityID(4)}, deliveryErr: errors.New("secret-canary smtp"), wantLookups: 1, wantIssues: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var key attempt.Key
			key[31] = 1
			credentials := &credentialReaderFake{record: tc.record, err: tc.lookupErr}
			attempts := &registrationAttemptsFake{}
			verification := &verificationIssueFake{err: tc.deliveryErr}
			result, err := resendService(t, credentials, attempts, &registrationKeyerFake{key: key}, verification).Resend(context.Background(), resendInput(tc.email))
			if err != nil || !result.Accepted || result.VerificationIssued || attempts.calls != 1 || len(credentials.lookups) != tc.wantLookups || verification.calls != tc.wantIssues {
				t.Fatal("verification resend disclosed target state", result, err)
			}
		})
	}
}

func TestVerificationResendFailsClosedForLimitsAndDependencies(t *testing.T) {
	var key attempt.Key
	key[31] = 1
	verification := &verificationIssueFake{}
	service := resendService(t, &credentialReaderFake{}, &registrationAttemptsFake{err: identityapp.ErrRegistrationAttemptDenied}, &registrationKeyerFake{key: key}, verification)
	if result, err := service.Resend(context.Background(), resendInput("user@example.test")); err == nil || err.Error() != "verification_resend_attempt_denied" || result != (identityapp.EmailVerificationResendResult{}) || verification.calls != 0 {
		t.Fatal("verification resend attempt denial drifted", result, err)
	}
	service = resendService(t, &credentialReaderFake{err: errors.New("secret-canary database")}, &registrationAttemptsFake{}, &registrationKeyerFake{key: key}, verification)
	if result, err := service.Resend(context.Background(), resendInput("user@example.test")); err == nil || err.Error() != "verification_resend_unavailable" || result != (identityapp.EmailVerificationResendResult{}) {
		t.Fatal("verification resend dependency failure was not fail closed", result, err)
	}
	if service, err := identityapp.NewEmailVerificationResendService(nil, &registrationAttemptsFake{}, &registrationKeyerFake{}, verification); err == nil || service != nil {
		t.Fatal("nil verification resend dependency accepted")
	}
}
