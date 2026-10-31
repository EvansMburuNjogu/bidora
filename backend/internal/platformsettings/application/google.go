package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"bidora/backend/internal/platformsettings/domain"
)

type GoogleCipher struct {
	ID                domain.ID
	KeyVersion        int32
	Nonce, Ciphertext []byte
}

func (GoogleCipher) Format(s fmt.State, _ rune) { fmt.Fprint(s, "[REDACTED Google ciphertext]") }

type GoogleConfiguration struct {
	ID                domain.ID `json:"id"`
	Version           int64     `json:"version"`
	Enabled           bool      `json:"enabled"`
	AllowSignup       bool      `json:"allow_signup"`
	ClientID          string    `json:"client_id"`
	AllowedDomains    []string  `json:"allowed_domains"`
	SecretConfigured  bool      `json:"secret_configured"`
	ConnectionStatus  string    `json:"connection_status"`
	RuntimeConfigured bool      `json:"runtime_configured"`
	CreatedAt         time.Time `json:"created_at"`
}
type GoogleStored struct {
	Configuration GoogleConfiguration
	Secret        GoogleCipher
}

func (GoogleStored) Format(s fmt.State, _ rune) {
	fmt.Fprint(s, "[REDACTED stored Google configuration]")
}

type GoogleSave struct {
	Subject        Subject
	Configuration  GoogleConfiguration
	ClientSecret   *string
	IdempotencyKey string
	CorrelationID  domain.ID
}

func (GoogleSave) Format(s fmt.State, _ rune) {
	fmt.Fprint(s, "[REDACTED Google configuration command]")
}

type GoogleReceipt struct {
	Hash          [32]byte
	Version       int64
	Target        domain.ID
	State, Result string
}
type GoogleTx interface {
	AuthorizeInformation(context.Context, Subject, time.Time) (Authority, error)
	LockGoogle(context.Context) error
	PasswordFallbackAvailable(context.Context) (bool, error)
	ReadGoogle(context.Context) (GoogleStored, error)
	GoogleHistory(context.Context) ([]GoogleStored, error)
	FindGoogleCommand(context.Context, domain.ID, string, [32]byte) (GoogleReceipt, bool, error)
	WriteGoogle(context.Context, GoogleStored, domain.ID, time.Time) error
	RecordGoogleCommand(context.Context, domain.ID, string, [32]byte, GoogleReceipt, time.Time) error
	FinishGoogleTest(context.Context, domain.ID, [32]byte, string) error
	GoogleTestCount(context.Context, domain.ID, time.Time) (int64, error)
	AppendGoogleEvidence(context.Context, domain.ID, domain.ID, domain.ID, domain.ID, string, GoogleStored, string, time.Time) error
}
type GoogleUnitOfWork interface {
	WithinGoogle(context.Context, bool, func(context.Context, GoogleTx) error) error
}
type GoogleSealer interface {
	SealGoogleClientSecret(domain.ID, []byte) (GoogleCipher, error)
	OpenGoogleClientSecret(GoogleCipher) ([]byte, error)
}
type GoogleConnectionTester interface{ TestGoogle(context.Context) error }
type GoogleService struct {
	uow               GoogleUnitOfWork
	vault             GoogleSealer
	tester            GoogleConnectionTester
	ids               IDGenerator
	clock             Clock
	runtimeConfigured bool
}

func NewGoogleService(uow GoogleUnitOfWork, vault GoogleSealer, tester GoogleConnectionTester, ids IDGenerator, clock Clock, runtimeConfigured bool) (*GoogleService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, ErrInformationInvalid
	}
	return &GoogleService{uow, vault, tester, ids, clock, runtimeConfigured}, nil
}

var googleClientIDPattern = regexp.MustCompile(`^[0-9]+-[a-z0-9_-]+\.apps\.googleusercontent\.com$`)
var googleDomainPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$`)

func canReadGoogleConfiguration(a Authority, at time.Time) bool {
	return a.Capabilities["platform.integration.read"] && recent(a, at)
}

func ValidGoogleConfiguration(c GoogleConfiguration) bool {
	if len(c.ClientID) > 255 || c.ClientID != "" && !googleClientIDPattern.MatchString(c.ClientID) || c.Enabled && c.ClientID == "" || c.AllowSignup && !c.Enabled || len(c.AllowedDomains) > 50 {
		return false
	}
	seen := map[string]bool{}
	for _, d := range c.AllowedDomains {
		if len(d) > 253 || !googleDomainPattern.MatchString(d) || seen[d] {
			return false
		}
		for _, label := range strings.Split(d, ".") {
			if len(label) > 63 {
				return false
			}
		}
		seen[d] = true
	}
	return true
}

// AllowsVerifiedEmail is an additional configuration gate, not authentication.
// The identity owner supplies an already verified, canonical provider email and
// holds the current head fence in its commit transaction.
func (c GoogleConfiguration) AllowsVerifiedEmail(expectedVersion int64, address string) bool {
	if expectedVersion < 1 || !c.Enabled || c.Version != expectedVersion || !ValidGoogleConfiguration(c) {
		return false
	}
	parts := strings.Split(address, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ToLower(address) != address {
		return false
	}
	if len(c.AllowedDomains) == 0 {
		return true
	}
	for _, domain := range c.AllowedDomains {
		if domain == parts[1] {
			return true
		}
	}
	return false
}

func (s *GoogleService) PublicStatus(v GoogleStored) map[string]bool {
	enabled := v.Configuration.Enabled && s.ready(v)
	return map[string]bool{"enabled": enabled, "allow_signup": enabled && v.Configuration.AllowSignup}
}

func (s *GoogleService) ready(v GoogleStored) bool {
	if s == nil || !s.runtimeConfigured || s.vault == nil || v.Secret.ID.IsZero() {
		return false
	}
	plain, e := s.vault.OpenGoogleClientSecret(v.Secret)
	for i := range plain {
		plain[i] = 0
	}
	return e == nil
}

func (s *GoogleService) decorate(v GoogleStored) GoogleConfiguration {
	c := v.Configuration
	c.SecretConfigured = !v.Secret.ID.IsZero()
	c.RuntimeConfigured = s.ready(v)
	if c.AllowedDomains == nil {
		c.AllowedDomains = []string{}
	}
	if c.ConnectionStatus == "" {
		c.ConnectionStatus = "UNTESTED"
	}
	return c
}
func (s *GoogleService) Read(ctx context.Context, subject Subject) (out GoogleConfiguration, err error) {
	if s == nil || !validInformationSubject(ctx, subject) {
		return out, ErrInformationInvalid
	}
	err = s.uow.WithinGoogle(ctx, false, func(c context.Context, tx GoogleTx) error {
		a, e := tx.AuthorizeInformation(c, subject, s.clock.Now())
		if e != nil {
			return e
		}
		if !canReadGoogleConfiguration(a, s.clock.Now()) {
			return ErrInformationForbidden
		}
		v, e := tx.ReadGoogle(c)
		out = s.decorate(v)
		return e
	})
	return out, googleError(err)
}
func (s *GoogleService) History(ctx context.Context, subject Subject) (out []GoogleConfiguration, err error) {
	if s == nil || !validInformationSubject(ctx, subject) {
		return nil, ErrInformationInvalid
	}
	err = s.uow.WithinGoogle(ctx, false, func(c context.Context, tx GoogleTx) error {
		a, e := tx.AuthorizeInformation(c, subject, s.clock.Now())
		if e != nil {
			return e
		}
		if !canReadGoogleConfiguration(a, s.clock.Now()) {
			return ErrInformationForbidden
		}
		rows, e := tx.GoogleHistory(c)
		for _, v := range rows {
			out = append(out, s.decorate(v))
		}
		return e
	})
	if out == nil {
		out = []GoogleConfiguration{}
	}
	return out, googleError(err)
}
func (s *GoogleService) Runtime(ctx context.Context) (out GoogleStored, err error) {
	if s == nil || ctx == nil {
		return out, ErrInformationInvalid
	}
	err = s.uow.WithinGoogle(ctx, false, func(c context.Context, tx GoogleTx) error { var e error; out, e = tx.ReadGoogle(c); return e })
	return out, googleError(err)
}
func (s *GoogleService) Secret(v GoogleStored) ([]byte, error) {
	if s == nil || s.vault == nil {
		return nil, ErrInformationUnavailable
	}
	return s.vault.OpenGoogleClientSecret(v.Secret)
}
func googleError(err error) error {
	if err == nil || errors.Is(err, ErrInformationInvalid) || errors.Is(err, ErrInformationForbidden) || errors.Is(err, ErrInformationConflict) {
		return err
	}
	return ErrInformationUnavailable
}
func (s *GoogleService) Save(ctx context.Context, in GoogleSave) (out GoogleConfiguration, err error) {
	if s == nil || !validInformationSubject(ctx, in.Subject) || !ValidGoogleConfiguration(in.Configuration) || in.Configuration.Version < 1 || in.CorrelationID.IsZero() || len(in.IdempotencyKey) < 1 || len(in.IdempotencyKey) > 128 || in.ClientSecret != nil && (len(*in.ClientSecret) < 8 || len(*in.ClientSecret) > 1024 || strings.TrimSpace(*in.ClientSecret) != *in.ClientSecret || strings.ContainsAny(*in.ClientSecret, "\r\n\x00")) {
		return out, ErrInformationInvalid
	}
	encoded, e := json.Marshal(struct {
		Enabled, Signup bool
		ClientID        string
		Domains         []string
		Secret          *string
		Version         int64
	}{in.Configuration.Enabled, in.Configuration.AllowSignup, in.Configuration.ClientID, in.Configuration.AllowedDomains, in.ClientSecret, in.Configuration.Version})
	if e != nil {
		return out, ErrInformationInvalid
	}
	hash := sha256.Sum256(encoded)
	for i := range encoded {
		encoded[i] = 0
	}
	key := sha256.Sum256([]byte("google-save-v1\x00" + in.IdempotencyKey))
	at := s.clock.Now().UTC()
	err = s.uow.WithinGoogle(ctx, true, func(c context.Context, tx GoogleTx) error {
		auth, e := tx.AuthorizeInformation(c, in.Subject, at)
		if e != nil {
			return e
		}
		if !canReadGoogleConfiguration(auth, at) || !auth.EffectiveOwner {
			return ErrInformationForbidden
		}
		// Owner/identity changes use this same governance fence. Never disable the
		// remaining password+TOTP administrator recovery path.
		fallback, e := tx.PasswordFallbackAvailable(c)
		if e != nil {
			return e
		}
		if !fallback {
			return ErrInformationConflict
		}
		if e = tx.LockGoogle(c); e != nil {
			return e
		}
		auth, e = tx.AuthorizeInformation(c, in.Subject, at)
		if e != nil {
			return e
		}
		if !canReadGoogleConfiguration(auth, at) || !auth.EffectiveOwner {
			return ErrInformationForbidden
		}
		current, e := tx.ReadGoogle(c)
		if e != nil {
			return e
		}
		old, found, e := tx.FindGoogleCommand(c, in.Subject.ActorID, "SAVE", key)
		if e != nil {
			return e
		}
		if found {
			if old.Hash != hash || old.Version != current.Configuration.Version {
				return ErrInformationConflict
			}
			out = s.decorate(current)
			return nil
		}
		if current.Configuration.Version != in.Configuration.Version {
			return ErrInformationConflict
		}
		if in.ClientSecret == nil && current.Configuration.ClientID != in.Configuration.ClientID && !current.Secret.ID.IsZero() {
			return ErrInformationInvalid
		}
		stored := GoogleStored{Configuration: in.Configuration, Secret: current.Secret}
		stored.Configuration.Version++
		stored.Configuration.CreatedAt = at
		stored.Configuration.ConnectionStatus = "UNTESTED"
		stored.Configuration.ID, e = s.ids.NewID()
		if e != nil || stored.Configuration.ID.IsZero() {
			return ErrInformationUnavailable
		}
		if in.ClientSecret != nil {
			if s.vault == nil {
				return ErrInformationUnavailable
			}
			id, e := s.ids.NewID()
			if e != nil {
				return e
			}
			plaintext := []byte(*in.ClientSecret)
			stored.Secret, e = s.vault.SealGoogleClientSecret(id, plaintext)
			for i := range plaintext {
				plaintext[i] = 0
			}
			if e != nil {
				return e
			}
		}
		if stored.Configuration.Enabled && stored.Secret.ID.IsZero() {
			return ErrInformationInvalid
		}
		if e = tx.WriteGoogle(c, stored, in.Subject.ActorID, at); e != nil {
			return e
		}
		if e = tx.RecordGoogleCommand(c, in.Subject.ActorID, "SAVE", key, GoogleReceipt{Hash: hash, Version: stored.Configuration.Version, Target: stored.Configuration.ID, State: "COMPLETED", Result: "SAVED"}, at); e != nil {
			return e
		}
		if e = s.evidence(c, tx, in.Subject.ActorID, in.CorrelationID, "PlatformGoogleConfigurationUpdated", stored, "SAVED", at); e != nil {
			return e
		}
		out = s.decorate(stored)
		return nil
	})
	return out, googleError(err)
}
func (s *GoogleService) evidence(ctx context.Context, tx GoogleTx, actor, correlation domain.ID, action string, stored GoogleStored, result string, at time.Time) error {
	audit, e := s.ids.NewID()
	if e != nil {
		return e
	}
	outbox, e := s.ids.NewID()
	if e != nil {
		return e
	}
	return tx.AppendGoogleEvidence(ctx, audit, outbox, actor, correlation, action, stored, result, at)
}
func (s *GoogleService) Test(ctx context.Context, subject Subject, version int64, idempotency string, correlation domain.ID) (result string, err error) {
	if s == nil || s.tester == nil || !validInformationSubject(ctx, subject) || version < 1 || correlation.IsZero() || len(idempotency) < 1 || len(idempotency) > 128 {
		return "", ErrInformationInvalid
	}
	key := sha256.Sum256([]byte("google-test-v1\x00" + idempotency))
	hash := sha256.Sum256([]byte(fmt.Sprintf("google-test:%d", version)))
	var stored GoogleStored
	replay := false
	at := s.clock.Now().UTC()
	err = s.uow.WithinGoogle(ctx, true, func(c context.Context, tx GoogleTx) error {
		a, e := tx.AuthorizeInformation(c, subject, at)
		if e != nil {
			return e
		}
		if !canReadGoogleConfiguration(a, at) || !a.EffectiveOwner {
			return ErrInformationForbidden
		}
		if e = tx.LockGoogle(c); e != nil {
			return e
		}
		a, e = tx.AuthorizeInformation(c, subject, at)
		if e != nil {
			return e
		}
		if !canReadGoogleConfiguration(a, at) || !a.EffectiveOwner {
			return ErrInformationForbidden
		}
		stored, e = tx.ReadGoogle(c)
		if e != nil {
			return e
		}
		if stored.Configuration.Version != version {
			return ErrInformationConflict
		}
		old, found, e := tx.FindGoogleCommand(c, subject.ActorID, "TEST", key)
		if e != nil {
			return e
		}
		if found {
			if old.Hash != hash {
				return ErrInformationConflict
			}
			result = old.Result
			replay = true
			return nil
		}
		count, e := tx.GoogleTestCount(c, subject.ActorID, at.Add(-10*time.Minute))
		if e != nil {
			return e
		}
		if count >= 3 {
			return ErrInformationConflict
		}
		return tx.RecordGoogleCommand(c, subject.ActorID, "TEST", key, GoogleReceipt{Hash: hash, Version: version, Target: stored.Configuration.ID, State: "PENDING", Result: "PENDING"}, at)
	})
	if err != nil || replay {
		return result, googleError(err)
	}
	probe, cancel := context.WithTimeout(ctx, 8*time.Second)
	probeErr := s.tester.TestGoogle(probe)
	cancel()
	result = "JWKS_REACHABLE_CREDENTIALS_UNVERIFIED"
	if probeErr != nil {
		result = "UNAVAILABLE"
	}
	err = s.uow.WithinGoogle(ctx, true, func(c context.Context, tx GoogleTx) error {
		a, e := tx.AuthorizeInformation(c, subject, s.clock.Now())
		if e != nil {
			return e
		}
		if !canReadGoogleConfiguration(a, s.clock.Now()) || !a.EffectiveOwner {
			return ErrInformationForbidden
		}
		if e = tx.FinishGoogleTest(c, subject.ActorID, key, result); e != nil {
			return e
		}
		return s.evidence(c, tx, subject.ActorID, correlation, "PlatformGoogleConnectionTested", stored, result, s.clock.Now())
	})
	return result, googleError(err)
}
