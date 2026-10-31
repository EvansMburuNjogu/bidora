package identity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
)

type registrationAttemptsFake struct {
	err    error
	scopes attempt.Scopes
	calls  int
}

func (a *registrationAttemptsFake) Allow(_ context.Context, scopes attempt.Scopes) error {
	a.calls++
	a.scopes = scopes
	return a.err
}

type registrationKeyerFake struct {
	key    attempt.Key
	lookup string
	err    error
}

func (k *registrationKeyerFake) RegistrationIdentity(lookup string) (attempt.Key, error) {
	k.lookup = lookup
	return k.key, k.err
}

type verificationIssueFake struct {
	input identityapp.EmailVerificationIssueInput
	err   error
	calls int
}

func (v *verificationIssueFake) Issue(_ context.Context, input identityapp.EmailVerificationIssueInput) (identityapp.EmailVerificationIssueResult, error) {
	v.calls++
	v.input = input
	return identityapp.EmailVerificationIssueResult{UserID: input.UserID}, v.err
}
func openRegistrationService(t *testing.T, committer *registrationCommitterFake, attempts *registrationAttemptsFake, keyer *registrationKeyerFake, verification *verificationIssueFake) *identityapp.OpenRegistrationService {
	t.Helper()
	core, err := identityapp.NewSelfRegistrationService(&registrationPasswordFake{phc: "$argon2id$v=19$synthetic"}, &sequenceIDs{values: []identityapp.ID{identityID(1), identityID(2)}}, loginClockFake{at: time.Now()}, committer)
	if err != nil {
		t.Fatal(err)
	}
	service, err := identityapp.NewOpenRegistrationService(core, attempts, keyer, verification)
	if err != nil {
		t.Fatal(err)
	}
	return service
}
func TestOpenRegistrationLimitsBeforeHashAndIssuesVerificationForNewIdentity(t *testing.T) {
	var identityKey, networkKey attempt.Key
	identityKey[31] = 1
	networkKey[31] = 2
	committer := &registrationCommitterFake{}
	attempts := &registrationAttemptsFake{}
	keyer := &registrationKeyerFake{key: identityKey}
	verification := &verificationIssueFake{}
	service := openRegistrationService(t, committer, attempts, keyer, verification)
	result, err := service.Register(context.Background(), identityapp.OpenRegistrationInput{Registration: identityapp.SelfRegistrationInput{FirstName: "Amina", LastName: "Njoroge", Email: "User@example.test", Password: []byte("synthetic password"), CorrelationID: identityID(3)}, NetworkScope: networkKey})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Accepted || !result.Created || !result.VerificationIssued || attempts.calls != 1 || attempts.scopes.Identity != identityKey || attempts.scopes.Network != networkKey || keyer.lookup != "user@example.test" || verification.calls != 1 || verification.input.UserID != identityID(1) {
		t.Fatal("open registration orchestration drifted", result)
	}
}
func TestOpenRegistrationConsumesMalformedAndExistingAttemptsWithoutDisclosure(t *testing.T) {
	var key, network attempt.Key
	key[31] = 1
	network[31] = 2
	for _, tc := range []struct {
		name, email string
		existing    bool
	}{{"malformed", "bad", false}, {"existing", "user@example.test", true}} {
		t.Run(tc.name, func(t *testing.T) {
			committer := &registrationCommitterFake{}
			if tc.existing {
				committer.err = identityapp.ErrIdentityAlreadyExists
			}
			attempts := &registrationAttemptsFake{}
			keyer := &registrationKeyerFake{key: key}
			verification := &verificationIssueFake{}
			service := openRegistrationService(t, committer, attempts, keyer, verification)
			result, err := service.Register(context.Background(), identityapp.OpenRegistrationInput{Registration: identityapp.SelfRegistrationInput{FirstName: "Amina", LastName: "Njoroge", Email: tc.email, Password: []byte("synthetic password"), CorrelationID: identityID(3)}, NetworkScope: network})
			if attempts.calls != 1 {
				t.Fatal("registration attempt was not consumed")
			}
			if tc.existing {
				if err != nil || !result.Accepted || result.Created || verification.calls != 0 {
					t.Fatal("existing identity disclosed", result, err)
				}
			} else if err == nil || err.Error() != "registration_invalid" {
				t.Fatal("malformed registration accepted", err)
			}
		})
	}
}
func TestOpenRegistrationDenialAndDeliveryFailureSemantics(t *testing.T) {
	var key, network attempt.Key
	key[31] = 1
	network[31] = 2
	attempts := &registrationAttemptsFake{err: identityapp.ErrRegistrationAttemptDenied}
	service := openRegistrationService(t, &registrationCommitterFake{}, attempts, &registrationKeyerFake{key: key}, &verificationIssueFake{})
	input := identityapp.OpenRegistrationInput{Registration: identityapp.SelfRegistrationInput{FirstName: "Amina", LastName: "Njoroge", Email: "user@example.test", Password: []byte("synthetic password"), CorrelationID: identityID(3)}, NetworkScope: network}
	if result, err := service.Register(context.Background(), input); err == nil || err.Error() != "registration_attempt_denied" || result != (identityapp.OpenRegistrationResult{}) {
		t.Fatal("registration denial drifted", result, err)
	}
	verification := &verificationIssueFake{err: errors.New("delivery unavailable")}
	service = openRegistrationService(t, &registrationCommitterFake{}, &registrationAttemptsFake{}, &registrationKeyerFake{key: key}, verification)
	result, err := service.Register(context.Background(), input)
	if err != nil || !result.Accepted || !result.Created || result.VerificationIssued {
		t.Fatal("ambiguous delivery was not generically accepted", result, err)
	}
	if service, err := identityapp.NewOpenRegistrationService(nil, &registrationAttemptsFake{}, &registrationKeyerFake{}, verification); err == nil || service != nil {
		t.Fatal("nil registration core accepted")
	}
}
