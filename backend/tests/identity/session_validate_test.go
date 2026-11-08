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

type sessionRepositoryFake struct {
	record identityapp.SessionRecord
	err    error
	hashes [][32]byte
	times  []time.Time
}

func (r *sessionRepositoryFake) GetSession(_ context.Context, hash [32]byte, observed time.Time) (identityapp.SessionRecord, error) {
	r.hashes = append(r.hashes, hash)
	r.times = append(r.times, observed)
	return r.record, r.err
}

type sessionClockFake struct{ at time.Time }

func (c sessionClockFake) Now() time.Time { return c.at }
func validSessionRecord(observed time.Time) identityapp.SessionRecord {
	var csrf [32]byte
	csrf[31] = 1
	return identityapp.SessionRecord{AuthenticatedAt: observed.Add(-time.Minute), SessionID: identityID(1), UserID: identityID(2), CSRFHash: csrf, AuthEpoch: 3, IdleExpiresAt: observed.Add(30 * time.Minute), AbsoluteExpiresAt: observed.Add(12 * time.Hour), AssuranceLevel: "AAL1", AssuranceMethod: "PRIMARY_ONLY", AssuredAt: observed.Add(-time.Minute)}
}

func TestSessionValidatorHashesRefreshesAndReturnsCurrentPrincipal(t *testing.T) {
	pair := sessionPair(t)
	observed := time.Date(2026, 10, 18, 10, 0, 0, 0, time.FixedZone("source", 3600))
	repository := &sessionRepositoryFake{record: validSessionRecord(observed.UTC())}
	validator, err := identityapp.NewSessionValidator(repository, sessionClockFake{observed})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := validator.Validate(context.Background(), pair.Session.RevealForCookie())
	if err != nil {
		t.Fatal(err)
	}
	if principal.SessionID != identityID(1) || principal.UserID != identityID(2) || principal.AuthEpoch != 3 || principal.CSRFHash != repository.record.CSRFHash || len(repository.hashes) != 1 || repository.hashes[0] != pair.Session.Hash() || !repository.times[0].Equal(observed.UTC()) {
		t.Fatal("session principal drifted", principal)
	}
	if strings.Contains(fmt.Sprintf("%#v", principal), pair.Session.RevealForCookie()) {
		t.Fatal("session principal leaked token")
	}
}
func TestSessionValidatorConcealsMalformedMissingAndExpiredSessions(t *testing.T) {
	observed := time.Now().UTC()
	pair := sessionPair(t)
	t.Run("malformed", func(t *testing.T) {
		repo := &sessionRepositoryFake{}
		validator, _ := identityapp.NewSessionValidator(repo, sessionClockFake{observed})
		principal, err := validator.Validate(context.Background(), "malformed")
		if err == nil || err.Error() != "session_invalid" || principal != (identityapp.SessionPrincipal{}) || len(repo.hashes) != 0 {
			t.Fatal("malformed session crossed repository", err)
		}
	})
	t.Run("missing", func(t *testing.T) {
		repo := &sessionRepositoryFake{err: identityapp.ErrSessionNotFound}
		validator, _ := identityapp.NewSessionValidator(repo, sessionClockFake{observed})
		_, err := validator.Validate(context.Background(), pair.Session.RevealForCookie())
		if err == nil || err.Error() != "session_invalid" || errors.Is(err, identityapp.ErrSessionNotFound) {
			t.Fatal("missing session state leaked", err)
		}
	})
	for _, mutate := range []func(*identityapp.SessionRecord){func(r *identityapp.SessionRecord) { r.SessionID = identityapp.ID{} }, func(r *identityapp.SessionRecord) { r.CSRFHash = [32]byte{} }, func(r *identityapp.SessionRecord) { r.AuthEpoch = 0 }, func(r *identityapp.SessionRecord) { r.IdleExpiresAt = observed }, func(r *identityapp.SessionRecord) { r.IdleExpiresAt = r.AbsoluteExpiresAt.Add(time.Second) }, func(r *identityapp.SessionRecord) { r.AssuranceLevel = "" }, func(r *identityapp.SessionRecord) { r.AssuredAt = time.Time{} }} {
		record := validSessionRecord(observed)
		mutate(&record)
		repo := &sessionRepositoryFake{record: record}
		validator, _ := identityapp.NewSessionValidator(repo, sessionClockFake{observed})
		if _, err := validator.Validate(context.Background(), pair.Session.RevealForCookie()); err == nil || err.Error() != "session_unavailable" {
			t.Fatal("inconsistent session accepted", record, err)
		}
	}
}
func TestSessionValidatorDependenciesFailClosed(t *testing.T) {
	cause := errors.New("secret-canary session database")
	pair := sessionPair(t)
	repo := &sessionRepositoryFake{err: cause}
	validator, _ := identityapp.NewSessionValidator(repo, sessionClockFake{time.Now()})
	_, err := validator.Validate(context.Background(), pair.Session.RevealForCookie())
	if err == nil || err.Error() != "session_unavailable" || !errors.Is(err, cause) || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
		t.Fatal("session dependency failure unsafe", err)
	}
	if validator, err := identityapp.NewSessionValidator(nil, sessionClockFake{}); err == nil || validator != nil {
		t.Fatal("nil repository accepted")
	}
	if validator, err := identityapp.NewSessionValidator(&sessionRepositoryFake{}, nil); err == nil || validator != nil {
		t.Fatal("nil clock accepted")
	}
	validator, _ = identityapp.NewSessionValidator(&sessionRepositoryFake{}, sessionClockFake{})
	if _, err := validator.Validate(context.Background(), pair.Session.RevealForCookie()); err == nil || err.Error() != "session_unavailable" {
		t.Fatal("zero clock accepted")
	}
	if _, err := validator.Validate(nil, pair.Session.RevealForCookie()); err == nil || err.Error() != "session_invalid" {
		t.Fatal("nil context accepted")
	}
	var zero *identityapp.SessionValidator
	if _, err := zero.Validate(context.Background(), pair.Session.RevealForCookie()); err == nil {
		t.Fatal("zero validator accepted")
	}
}
