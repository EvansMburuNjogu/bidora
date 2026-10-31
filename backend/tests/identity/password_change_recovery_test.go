package identity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/verification"
)

type passwordReauthFake struct {
	proof identityapp.GoogleLinkProof
	err   error
	calls int
}

func (f *passwordReauthFake) Reauthenticate(context.Context, identityapp.GoogleLinkReauthenticationInput) (identityapp.GoogleLinkProof, error) {
	f.calls++
	return f.proof, f.err
}

type passwordChangeCommitFake struct {
	commit identityapp.PasswordChangeCommit
	err    error
	calls  int
}

func (f *passwordChangeCommitFake) CommitPasswordChange(_ context.Context, c identityapp.PasswordChangeCommit) error {
	f.calls++
	f.commit = c
	return f.err
}

func TestPasswordChangeAdvancesEpochVersionRevokesThroughAtomicCommitAndRotatesSession(t *testing.T) {
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	pair := sessionPair(t)
	reauth := &passwordReauthFake{proof: identityapp.GoogleLinkProof{UserID: identityID(2), SessionID: identityID(1), AuthEpoch: 4, CredentialVersion: 7}}
	passwords := &registrationPasswordFake{phc: "$argon2id$v=19$synthetic-new"}
	commit := &passwordChangeCommitFake{}
	service, err := identityapp.NewPasswordChangeService(reauth, passwords, &tokenGeneratorFake{pair: pair}, &sequenceIDs{values: []identityapp.ID{identityID(3), identityID(4), identityID(5)}}, loginClockFake{at: now}, commit)
	if err != nil {
		t.Fatal(err)
	}
	principal := identityapp.SessionPrincipal{SessionID: identityID(1), UserID: identityID(2), AuthEpoch: 4}
	result, err := service.Change(context.Background(), identityapp.PasswordChangeInput{Principal: principal, Credentials: loginInput("user@example.test", []byte("current password")), NewPassword: []byte("replacement password"), CorrelationID: identityID(6)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Session.IsZero() || result.CSRF.IsZero() || commit.calls != 1 || commit.commit.ExpectedAuthEpoch != 4 || commit.commit.NewAuthEpoch != 5 || commit.commit.ExpectedCredentialVersion != 7 || commit.commit.NewCredentialVersion != 8 || commit.commit.CurrentSessionID != identityID(1) || commit.commit.ReplacementSessionID != identityID(3) || commit.commit.SessionHash != pair.Session.Hash() {
		t.Fatal("password change rotation commit drifted")
	}
}
func TestPasswordChangeRejectsCurrentPasswordFailureBeforeHashOrRotation(t *testing.T) {
	reauth := &passwordReauthFake{err: errors.New("google_link_reauthentication_invalid")}
	passwords := &registrationPasswordFake{phc: "unused"}
	commit := &passwordChangeCommitFake{}
	service, _ := identityapp.NewPasswordChangeService(reauth, passwords, &tokenGeneratorFake{pair: sessionPair(t)}, &sequenceIDs{values: []identityapp.ID{identityID(3)}}, loginClockFake{at: time.Now()}, commit)
	_, err := service.Change(context.Background(), identityapp.PasswordChangeInput{Principal: identityapp.SessionPrincipal{SessionID: identityID(1), UserID: identityID(2), AuthEpoch: 1}, Credentials: loginInput("user@example.test", []byte("wrong")), NewPassword: []byte("replacement password"), CorrelationID: identityID(6)})
	if err == nil || err.Error() != "password_change_credentials_invalid" || commit.calls != 0 || len(passwords.hashed) != 0 {
		t.Fatal("current-password failure crossed mutation boundary", err)
	}
}

type recoveryIssuerFake struct {
	command identityapp.PasswordRecoveryIssueCommand
	result  identityapp.PasswordRecoveryIssuance
	err     error
	calls   int
}

func (f *recoveryIssuerFake) IssuePasswordRecovery(_ context.Context, c identityapp.PasswordRecoveryIssueCommand) (identityapp.PasswordRecoveryIssuance, error) {
	f.calls++
	f.command = c
	return f.result, f.err
}

type recoveryDeliveryFake struct {
	token verification.Token
	calls int
	err   error
}

func (f *recoveryDeliveryFake) DeliverPasswordRecovery(_ context.Context, _ email.Address, t verification.Token, _ time.Time) error {
	f.calls++
	f.token = t
	return f.err
}

type recoveryResetCommitFake struct {
	commit identityapp.PasswordRecoveryResetCommit
	err    error
	calls  int
}

func (f *recoveryResetCommitFake) CommitPasswordRecoveryReset(_ context.Context, c identityapp.PasswordRecoveryResetCommit) error {
	f.calls++
	f.commit = c
	return f.err
}

func TestPasswordRecoveryPersistsHashBeforeGenericDeliveryAndConcealsUnknownIdentity(t *testing.T) {
	now := time.Date(2026, 10, 22, 11, 0, 0, 0, time.UTC)
	token := issueToken(t)
	issuer := &recoveryIssuerFake{result: identityapp.PasswordRecoveryIssuance{Issued: true, Recipient: "user@example.test", ExpiresAt: now.Add(30 * time.Minute)}}
	delivery := &recoveryDeliveryFake{err: errors.New("synthetic ambiguous delivery")}
	service, err := identityapp.NewPasswordRecoveryRequestService(issuer, delivery, verificationTokenGeneratorFake{token: token}, &sequenceIDs{values: []identityapp.ID{identityID(1), identityID(2)}}, loginClockFake{at: now})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Request(context.Background(), "User@example.test", identityID(3)); err != nil {
		t.Fatal(err)
	}
	if issuer.command.TokenHash != token.Hash() || delivery.calls != 1 || delivery.token.Hash() != token.Hash() {
		t.Fatal("recovery hash/delivery ordering drifted")
	}
	unknown := &recoveryIssuerFake{}
	hidden, _ := identityapp.NewPasswordRecoveryRequestService(unknown, &recoveryDeliveryFake{}, verificationTokenGeneratorFake{token: token}, &sequenceIDs{values: []identityapp.ID{identityID(4), identityID(5)}}, loginClockFake{at: now})
	if err = hidden.Request(context.Background(), "unknown@example.test", identityID(6)); err != nil {
		t.Fatal(err)
	}
}
func TestPasswordRecoveryResetHashesTokenAndMapsSingleUseConflict(t *testing.T) {
	token := issueToken(t)
	passwords := &registrationPasswordFake{phc: "$argon2id$v=19$synthetic-reset"}
	commit := &recoveryResetCommitFake{}
	service, _ := identityapp.NewPasswordRecoveryResetService(commit, passwords, &sequenceIDs{values: []identityapp.ID{identityID(1), identityID(3)}}, loginClockFake{at: time.Now().UTC()})
	if err := service.Reset(context.Background(), identityapp.PasswordRecoveryResetInput{Token: token.RevealForDelivery(), NewPassword: []byte("replacement password"), CorrelationID: identityID(2)}); err != nil {
		t.Fatal(err)
	}
	if commit.commit.TokenHash != token.Hash() || commit.calls != 1 {
		t.Fatal("recovery reset commit drifted")
	}
	commit.err = identityapp.ErrPasswordRecoveryInvalid
	if err := service.Reset(context.Background(), identityapp.PasswordRecoveryResetInput{Token: token.RevealForDelivery(), NewPassword: []byte("replacement password"), CorrelationID: identityID(2)}); err == nil || err.Error() != "password_recovery_reset_invalid" {
		t.Fatal("recovery replay not concealed", err)
	}
}

type recoveryAttemptsFake struct {
	err   error
	calls int
}

func (f *recoveryAttemptsFake) Allow(context.Context, attempt.Scopes) error { f.calls++; return f.err }

type recoveryKeyFake struct{ key attempt.Key }

func (f recoveryKeyFake) RecoveryIdentity(string) (attempt.Key, error) { return f.key, nil }

type recoveryRequesterFake struct{ calls int }

func (f *recoveryRequesterFake) Request(context.Context, string, identityapp.ID) error {
	f.calls++
	return nil
}
func TestPublicPasswordRecoveryAppliesLimiterBeforeGenericRequest(t *testing.T) {
	var key attempt.Key
	key[0] = 1
	var network attempt.Key
	network[0] = 2
	attempts := &recoveryAttemptsFake{err: identityapp.ErrRecoveryAttemptDenied}
	requester := &recoveryRequesterFake{}
	service, _ := identityapp.NewPublicPasswordRecoveryService(requester, attempts, recoveryKeyFake{key: key})
	err := service.Request(context.Background(), identityapp.PublicPasswordRecoveryInput{Email: "user@example.test", CorrelationID: identityID(1), NetworkScope: network})
	if err == nil || err.Error() != "password_recovery_attempt_denied" || requester.calls != 0 {
		t.Fatal("recovery limiter did not fail before request", err)
	}
}
