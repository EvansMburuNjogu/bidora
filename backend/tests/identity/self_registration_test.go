package identity_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
)

type registrationPasswordFake struct {
	validateErr, hashErr error
	phc                  string
	validated, hashed    []byte
}

func (p *registrationPasswordFake) ValidateNew(value []byte) error {
	p.validated = append([]byte(nil), value...)
	return p.validateErr
}
func (p *registrationPasswordFake) Hash(value []byte) (string, error) {
	p.hashed = append([]byte(nil), value...)
	return p.phc, p.hashErr
}

type registrationCommitterFake struct {
	commit identityapp.SelfRegistrationCommit
	err    error
	calls  int
}

func (c *registrationCommitterFake) CommitSelfRegistration(_ context.Context, commit identityapp.SelfRegistrationCommit) error {
	c.calls++
	c.commit = commit
	return c.err
}
func TestSelfRegistrationCreatesOnlyUnverifiedIdentityCommit(t *testing.T) {
	passwords := &registrationPasswordFake{phc: "$argon2id$v=19$synthetic"}
	committer := &registrationCommitterFake{}
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	service, err := identityapp.NewSelfRegistrationService(passwords, &sequenceIDs{values: []identityapp.ID{identityID(1), identityID(2)}}, loginClockFake{at: at}, committer)
	if err != nil {
		t.Fatal(err)
	}
	input := identityapp.SelfRegistrationInput{FirstName: "Amina", LastName: "Njoroge", Email: "User@example.test", Password: []byte("synthetic password"), CorrelationID: identityID(3)}
	result, err := service.Register(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Accepted || !result.Created || result.UserID != identityID(1) || committer.calls != 1 || committer.commit.Email.Display() != input.Email || committer.commit.Email.Lookup() != "user@example.test" || committer.commit.FirstName != input.FirstName || committer.commit.LastName != input.LastName || committer.commit.AuditID != identityID(2) || !committer.commit.CreatedAt.Equal(at) {
		t.Fatal("self registration orchestration drifted", result)
	}
	formatted := fmt.Sprintf("%#v %#v", committer.commit, result)
	if strings.Contains(formatted, input.Email) || strings.Contains(formatted, input.FirstName) || strings.Contains(formatted, input.LastName) || strings.Contains(formatted, string(input.Password)) || strings.Contains(formatted, passwords.phc) {
		t.Fatal("registration formatting leaked private input")
	}
}
func TestSelfRegistrationConcealsExistingIdentity(t *testing.T) {
	committer := &registrationCommitterFake{err: identityapp.ErrIdentityAlreadyExists}
	service, _ := identityapp.NewSelfRegistrationService(&registrationPasswordFake{phc: "$argon2id$v=19$synthetic"}, &sequenceIDs{values: []identityapp.ID{identityID(1), identityID(2)}}, loginClockFake{at: time.Now()}, committer)
	result, err := service.Register(context.Background(), identityapp.SelfRegistrationInput{FirstName: "Amina", LastName: "Njoroge", Email: "user@example.test", Password: []byte("synthetic password"), CorrelationID: identityID(3)})
	if err != nil || !result.Accepted || result.Created || !result.UserID.IsZero() {
		t.Fatal("existing identity was disclosed", result, err)
	}
}
func TestPersonalNamePolicyPreservesValidUTF8AndRejectsUnsafeValues(t *testing.T) {
	valid := []string{"Amina", "María José", "李", strings.Repeat("é", 50)}
	for _, value := range valid {
		if !identityapp.ValidPersonalName(value) {
			t.Fatalf("valid personal name rejected: %q", value)
		}
	}
	invalid := []string{"", " Amina", "Amina ", "Amina\nNjoroge", "Amina\x00Njoroge", strings.Repeat("a", 101), strings.Repeat("é", 51), string([]byte{0xff})}
	for _, value := range invalid {
		if identityapp.ValidPersonalName(value) {
			t.Fatalf("invalid personal name accepted: %q", value)
		}
	}
}

func TestSelfRegistrationRejectsInvalidInputAndDependencies(t *testing.T) {
	cases := []identityapp.SelfRegistrationInput{{FirstName: "Amina", LastName: "Njoroge", Email: "bad", Password: []byte("synthetic password"), CorrelationID: identityID(3)}, {FirstName: "Amina", LastName: "Njoroge", Email: "user@example.test", Password: []byte("short"), CorrelationID: identityID(3)}, {FirstName: "Amina", LastName: "Njoroge", Email: "user@example.test", Password: []byte("synthetic password")}}
	for index, input := range cases {
		passwords := &registrationPasswordFake{phc: "$argon2id$v=19$synthetic"}
		if index == 1 {
			passwords.validateErr = errors.New("invalid")
		}
		committer := &registrationCommitterFake{}
		service, _ := identityapp.NewSelfRegistrationService(passwords, &sequenceIDs{values: []identityapp.ID{identityID(1), identityID(2)}}, loginClockFake{at: time.Now()}, committer)
		if result, err := service.Register(context.Background(), input); err == nil || err.Error() != "registration_invalid" || result != (identityapp.SelfRegistrationResult{}) || committer.calls != 0 {
			t.Fatal("invalid registration accepted", index, result, err)
		}
	}
	if service, err := identityapp.NewSelfRegistrationService(nil, &sequenceIDs{}, loginClockFake{}, &registrationCommitterFake{}); err == nil || service != nil {
		t.Fatal("nil password service accepted")
	}
	cause := errors.New("secret-canary registration database")
	service, _ := identityapp.NewSelfRegistrationService(&registrationPasswordFake{phc: "$argon2id$v=19$synthetic"}, &sequenceIDs{values: []identityapp.ID{identityID(1), identityID(2)}}, loginClockFake{at: time.Now()}, &registrationCommitterFake{err: cause})
	if result, err := service.Register(context.Background(), identityapp.SelfRegistrationInput{FirstName: "Amina", LastName: "Njoroge", Email: "user@example.test", Password: []byte("synthetic password"), CorrelationID: identityID(3)}); err == nil || err.Error() != "registration_unavailable" || result != (identityapp.SelfRegistrationResult{}) || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
		t.Fatal("registration dependency failure unsafe", result, err)
	}
	var zero *identityapp.SelfRegistrationService
	if _, err := zero.Register(context.Background(), identityapp.SelfRegistrationInput{FirstName: "Amina", LastName: "Njoroge", Email: "user@example.test", Password: []byte("synthetic password"), CorrelationID: identityID(3)}); err == nil {
		t.Fatal("zero registration service accepted")
	}
}
