package identity_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/verification"
)

type verificationIssuerFake struct {
	issued identityapp.EmailVerificationChallenge
	result identityapp.EmailVerificationIssuance
	err    error
	calls  int
}

func (i *verificationIssuerFake) IssueEmailVerification(_ context.Context, challenge identityapp.EmailVerificationChallenge) (identityapp.EmailVerificationIssuance, error) {
	i.calls++
	i.issued = challenge
	return i.result, i.err
}

type verificationDeliveryFake struct {
	recipient email.Address
	token     verification.Token
	expires   time.Time
	err       error
	calls     int
}

func (d *verificationDeliveryFake) DeliverEmailVerification(_ context.Context, recipient email.Address, token verification.Token, expires time.Time) error {
	d.calls++
	d.recipient = recipient
	d.token = token
	d.expires = expires
	return d.err
}

type verificationTokenGeneratorFake struct {
	token verification.Token
	err   error
}

func (g verificationTokenGeneratorFake) Generate() (verification.Token, error) { return g.token, g.err }

type sequenceIDs struct {
	values []identityapp.ID
	index  int
	err    error
}

func (g *sequenceIDs) NewID() (identityapp.ID, error) {
	if g.err != nil {
		return identityapp.ID{}, g.err
	}
	if g.index >= len(g.values) {
		return identityapp.ID{}, errors.New("id exhausted")
	}
	value := g.values[g.index]
	g.index++
	return value, nil
}
func issueToken(t *testing.T) verification.Token {
	t.Helper()
	token, err := verification.GenerateWithRandom(tokenReader(5))
	if err != nil {
		t.Fatal(err)
	}
	return token
}
func TestEmailVerificationIssuePersistsHashBeforeDelivery(t *testing.T) {
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	token := issueToken(t)
	issuer := &verificationIssuerFake{result: identityapp.EmailVerificationIssuance{UserID: identityID(1), Recipient: "User@example.test", ExpiresAt: at.Add(verification.Lifetime)}}
	delivery := &verificationDeliveryFake{}
	service, err := identityapp.NewEmailVerificationIssueService(issuer, delivery, verificationTokenGeneratorFake{token: token}, &sequenceIDs{values: []identityapp.ID{identityID(4), identityID(5)}}, loginClockFake{at: at})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Issue(context.Background(), identityapp.EmailVerificationIssueInput{UserID: identityID(1), ActorID: identityID(2), CorrelationID: identityID(3)})
	if err != nil {
		t.Fatal(err)
	}
	if issuer.calls != 1 || delivery.calls != 1 || issuer.issued.ChallengeID != identityID(4) || issuer.issued.AuditID != identityID(5) || issuer.issued.TokenHash != token.Hash() || delivery.token.RevealForDelivery() != token.RevealForDelivery() || delivery.recipient.Display() != "User@example.test" || !result.ExpiresAt.Equal(at.Add(verification.Lifetime)) {
		t.Fatal("verification issuance orchestration drifted", result)
	}
	formatted := fmt.Sprintf("%#v %#v", issuer.result, result)
	if strings.Contains(formatted, "User@example.test") || strings.Contains(formatted, token.RevealForDelivery()) {
		t.Fatal("issuance formatting leaked private material")
	}
}
func TestEmailVerificationIssueFailsClosedButDoesNotPretendDeliveryRollback(t *testing.T) {
	at := time.Now().UTC()
	token := issueToken(t)
	cause := errors.New("secret-canary delivery")
	issuer := &verificationIssuerFake{result: identityapp.EmailVerificationIssuance{UserID: identityID(1), Recipient: "user@example.test", ExpiresAt: at.Add(verification.Lifetime)}}
	delivery := &verificationDeliveryFake{err: cause}
	service, _ := identityapp.NewEmailVerificationIssueService(issuer, delivery, verificationTokenGeneratorFake{token: token}, &sequenceIDs{values: []identityapp.ID{identityID(4), identityID(5)}}, loginClockFake{at: at})
	result, err := service.Issue(context.Background(), identityapp.EmailVerificationIssueInput{UserID: identityID(1), CorrelationID: identityID(3)})
	if err == nil || err.Error() != "verification_delivery_unavailable" || result != (identityapp.EmailVerificationIssueResult{}) || issuer.calls != 1 || delivery.calls != 1 || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
		t.Fatal("delivery failure semantics drifted", result, err)
	}
	issuer.err = identityapp.ErrVerificationTargetUnavailable
	delivery.err = nil
	service, _ = identityapp.NewEmailVerificationIssueService(issuer, delivery, verificationTokenGeneratorFake{token: token}, &sequenceIDs{values: []identityapp.ID{identityID(4), identityID(5)}}, loginClockFake{at: at})
	if _, err = service.Issue(context.Background(), identityapp.EmailVerificationIssueInput{UserID: identityID(1), CorrelationID: identityID(3)}); err == nil || err.Error() != "verification_issuance_unavailable" {
		t.Fatal("target state leaked", err)
	}
}
func TestEmailVerificationIssueRejectsInvalidDependenciesAndInput(t *testing.T) {
	if service, err := identityapp.NewEmailVerificationIssueService(nil, &verificationDeliveryFake{}, verificationTokenGeneratorFake{}, &sequenceIDs{}, loginClockFake{}); err == nil || service != nil {
		t.Fatal("nil issuer accepted")
	}
	service, _ := identityapp.NewEmailVerificationIssueService(&verificationIssuerFake{}, &verificationDeliveryFake{}, verificationTokenGeneratorFake{err: errors.New("random failed")}, &sequenceIDs{}, loginClockFake{at: time.Now()})
	if _, err := service.Issue(context.Background(), identityapp.EmailVerificationIssueInput{UserID: identityID(1), CorrelationID: identityID(2)}); err == nil || err.Error() != "verification_issuance_unavailable" {
		t.Fatal("token failure accepted")
	}
	var zero *identityapp.EmailVerificationIssueService
	if _, err := zero.Issue(context.Background(), identityapp.EmailVerificationIssueInput{UserID: identityID(1), CorrelationID: identityID(2)}); err == nil {
		t.Fatal("zero service accepted")
	}
	if _, err := service.Issue(nil, identityapp.EmailVerificationIssueInput{}); err == nil || err.Error() != "verification_issuance_invalid" {
		t.Fatal("invalid issuance input accepted")
	}
}
