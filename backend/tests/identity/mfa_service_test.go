package identity_test

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/mfa"
)

type mfaStoreFake struct {
	platformAdministrator bool
	classificationErr error
	active, pending                   *identityapp.MFAFactorMaterial
	pendingCommit                     identityapp.MFAPendingCommit
	stepUpCommit                      identityapp.MFAStepUpCommit
	loginCommit                       identityapp.MFALoginCommit
	loginChallenge                    identityapp.MFALoginChallenge
	loginTokenHash, loginBindingHash  [32]byte
	loginConsumed                     bool
	recoveryHash                      [32]byte
	recoveryAvailable                 bool
	enrollmentFailures, loginFailures int
	activation                        identityapp.MFAActivationCommit
}

func (s *mfaStoreFake) HasActiveMFA(context.Context, identityapp.ID) (bool, error) {
	return s.active != nil, nil
}
func (s *mfaStoreFake) IsPlatformAdministrator(context.Context, identityapp.ID) (bool, error) {
	return s.platformAdministrator, s.classificationErr
}
func (s *mfaStoreFake) SavePendingMFA(_ context.Context, c identityapp.MFAPendingCommit) error {
	s.pendingCommit = c
	material := identityapp.MFAFactorMaterial{FactorID: c.FactorID, UserID: c.UserID, Ciphertext: append([]byte(nil), c.Ciphertext...), Nonce: append([]byte(nil), c.Nonce...), KeyVersion: c.KeyVersion, LastCounter: -1, Version: 1, ExpiresAt: c.At.Add(10 * time.Minute)}
	s.pending = &material
	return nil
}
func (s *mfaStoreFake) GetPendingMFA(context.Context, identityapp.ID) (identityapp.MFAFactorMaterial, error) {
	if s.pending == nil {
		return identityapp.MFAFactorMaterial{}, identityapp.ErrMFANotFound
	}
	return *s.pending, nil
}
func (s *mfaStoreFake) ActivateMFA(_ context.Context, c identityapp.MFAActivationCommit) error {
	s.activation = c
	return nil
}
func (s *mfaStoreFake) GetActiveMFA(context.Context, identityapp.ID) (identityapp.MFAFactorMaterial, error) {
	if s.active == nil {
		return identityapp.MFAFactorMaterial{}, identityapp.ErrMFANotFound
	}
	return *s.active, nil
}
func (s *mfaStoreFake) CommitMFAStepUp(_ context.Context, c identityapp.MFAStepUpCommit) error {
	if s.active == nil || c.ExpectedFactorVersion != s.active.Version {
		return identityapp.ErrMFAConflict
	}
	if c.Method == "TOTP" {
		if c.Counter <= s.active.LastCounter {
			return identityapp.ErrMFAConflict
		}
		s.active.LastCounter = c.Counter
	} else {
		if c.RecoveryHash == nil || !s.recoveryAvailable || *c.RecoveryHash != s.recoveryHash {
			return identityapp.ErrMFAConflict
		}
		s.recoveryAvailable = false
	}
	s.active.Version++
	s.stepUpCommit = c
	return nil
}
func (s *mfaStoreFake) BeginEmailLogin(_ context.Context, user identityapp.ID, epoch int64, method identityapp.AuthenticationMethod, token, binding, codeMAC [32]byte, id identityapp.ID, at time.Time) (email.Address, error) {
	s.loginTokenHash, s.loginBindingHash, s.loginConsumed = token, binding, false
	s.loginChallenge = identityapp.MFALoginChallenge{TransactionID: id, UserID: user, Purpose: "EMAIL_LOGIN", AuthEpoch: epoch, PrimaryMethod: string(method), ExpiresAt: at.Add(5 * time.Minute), CodeMAC: codeMAC}
	return email.Parse("member@example.com")
}
func (s *mfaStoreFake) BeginMFALogin(_ context.Context, user identityapp.ID, epoch int64, method identityapp.AuthenticationMethod, token, binding [32]byte, id identityapp.ID, at time.Time) error {
	s.loginTokenHash, s.loginBindingHash, s.loginConsumed = token, binding, false
	s.loginChallenge = identityapp.MFALoginChallenge{TransactionID: id, UserID: user, FactorID: s.active.FactorID, AuthEpoch: epoch, PrimaryMethod: string(method), ExpiresAt: at.Add(5 * time.Minute), Factor: *s.active}
	return nil
}
func (s *mfaStoreFake) GetMFALoginChallenge(_ context.Context, token, binding [32]byte, at time.Time) (identityapp.MFALoginChallenge, error) {
	if s.loginConsumed || token != s.loginTokenHash || binding != s.loginBindingHash || !s.loginChallenge.ExpiresAt.After(at) {
		return identityapp.MFALoginChallenge{}, identityapp.ErrMFANotFound
	}
	return s.loginChallenge, nil
}
func (s *mfaStoreFake) CancelMFALogin(context.Context, [32]byte, [32]byte, time.Time) error {
	s.loginConsumed = true
	return nil
}
func (s *mfaStoreFake) RecordMFALoginFailure(context.Context, identityapp.ID, time.Time) error {
	s.loginFailures++
	return nil
}
func (s *mfaStoreFake) RecordMFAEnrollmentFailure(context.Context, identityapp.ID, identityapp.ID, time.Time) error {
	s.enrollmentFailures++
	return nil
}
func (s *mfaStoreFake) MFAEnrollmentAttemptDenied(context.Context, identityapp.ID, time.Time) (bool, error) {
	return false, nil
}
func (s *mfaStoreFake) MFALoginAttemptDenied(context.Context, [32]byte, [32]byte, time.Time) (bool, error) {
	return false, nil
}
func (s *mfaStoreFake) CommitMFALogin(_ context.Context, c identityapp.MFALoginCommit) error {
	if s.loginConsumed || s.active == nil || c.ExpectedFactorVersion != s.active.Version {
		return identityapp.ErrMFAConflict
	}
	if c.Method == "TOTP" {
		if c.Counter <= s.active.LastCounter {
			return identityapp.ErrMFAConflict
		}
		s.active.LastCounter = c.Counter
	} else if c.RecoveryHash == nil || !s.recoveryAvailable || *c.RecoveryHash != s.recoveryHash {
		return identityapp.ErrMFAConflict
	} else {
		s.recoveryAvailable = false
	}
	s.active.Version++
	s.loginConsumed = true
	s.loginCommit = c
	return nil
}
func (s *mfaStoreFake) CheckMFARecoveryCode(_ context.Context, _, _ identityapp.ID, hash [32]byte) (bool, error) {
	return s.recoveryAvailable && hash == s.recoveryHash, nil
}
func (s *mfaStoreFake) RegenerateMFARecoveryCodes(context.Context, identityapp.MFARecoveryRegenerationCommit) error {
	return nil
}
func (s *mfaStoreFake) ReplaceMFAFactor(context.Context, identityapp.MFAReplacementCommit) error {
	return nil
}

type mfaReservationFake struct{ finish func(bool) error }

func (r mfaReservationFake) Finish(_ context.Context, success bool) error { return r.finish(success) }

type mfaAttemptsFake struct{ failures, threshold int }

func (a *mfaAttemptsFake) BeginMFA(context.Context, [32]byte) (identityapp.MFAAttemptReservation, error) {
	if a.threshold > 0 && a.failures >= a.threshold {
		return nil, identityapp.ErrMFAAttemptDenied
	}
	return mfaReservationFake{finish: func(success bool) error {
		if success {
			a.failures = 0
			return nil
		}
		a.failures++
		if a.threshold > 0 && a.failures >= a.threshold {
			return identityapp.ErrMFAAttemptDenied
		}
		return nil
	}}, nil
}

type mfaScopesFake struct{}

func (mfaScopesFake) DeriveMFAScope(p string, user, target identityapp.ID) ([32]byte, error) {
	return sha256.Sum256(append(append([]byte(p), user[:]...), target[:]...)), nil
}

type mfaGateFake struct {
	result identityapp.MFALoginGateResult
	calls  int
}

func (g *mfaGateFake) BeginLogin(context.Context, identityapp.ID, int64, identityapp.AuthenticationMethod) (identityapp.MFALoginGateResult, error) {
	g.calls++
	return g.result, nil
}

func mfaServiceFixture(t *testing.T, now time.Time, store *mfaStoreFake, attempts *mfaAttemptsFake) (*identityapp.MFAService, *mfa.KeyRing) {
	t.Helper()
	var key [32]byte
	for i := range key {
		key[i] = byte(i + 1)
	}
	keys, err := mfa.NewKeyRing(1, map[int][32]byte{1: key})
	if err != nil {
		t.Fatal(err)
	}
	tokens := &tokenGeneratorFake{pair: sessionPair(t)}
	service, err := identityapp.NewMFAService(store, keys, tokens, &loginIDsFake{id: identityID(7)}, loginClockFake{at: now}, attempts, mfaScopesFake{})
	if err != nil {
		t.Fatal(err)
	}
	return service, keys
}
func encryptedFactor(t *testing.T, keys *mfa.KeyRing, user, factor identityapp.ID, secret []byte) identityapp.MFAFactorMaterial {
	t.Helper()
	ciphertext, nonce, version, err := keys.Encrypt([16]byte(user), [16]byte(factor), secret)
	if err != nil {
		t.Fatal(err)
	}
	return identityapp.MFAFactorMaterial{FactorID: factor, UserID: user, Ciphertext: ciphertext, Nonce: nonce, KeyVersion: int32(version), LastCounter: -1, Version: 1}
}

func TestMFAEnrollmentStoresAADBoundSecretWithoutPersistingPlaintext(t *testing.T) {
	now := time.Unix(59, 0).UTC()
	store := &mfaStoreFake{}
	service, keys := mfaServiceFixture(t, now, store, &mfaAttemptsFake{threshold: 5})
	principal := identityapp.SessionPrincipal{SessionID: identityID(1), UserID: identityID(2), AuthEpoch: 1, AuthenticatedAt: now, AbsoluteExpiresAt: now.Add(12 * time.Hour)}
	enrollment, err := service.Enroll(context.Background(), principal, identityID(3))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(enrollment.Secret)
	if err != nil || len(decoded) != mfa.SecretBytes {
		t.Fatal("invalid enrollment secret")
	}
	plain, err := keys.Decrypt([16]byte(principal.UserID), [16]byte(enrollment.FactorID), store.pendingCommit.Ciphertext, store.pendingCommit.Nonce, int(store.pendingCommit.KeyVersion))
	if err != nil || string(plain) != string(decoded) {
		t.Fatal("pending secret binding drifted", err)
	}
	if strings.Contains(string(store.pendingCommit.Ciphertext), enrollment.Secret) || !strings.HasPrefix(enrollment.URI, "otpauth://totp/") || !enrollment.ExpiresAt.Equal(now.Add(10*time.Minute)) {
		t.Fatal("unsafe enrollment material")
	}
}

func TestMFAStepUpRejectsTOTPReplayConsumesRecoveryOnceAndRotatesSession(t *testing.T) {
	now := time.Unix(59, 0).UTC()
	user, factor := identityID(2), identityID(3)
	store := &mfaStoreFake{}
	service, keys := mfaServiceFixture(t, now, store, &mfaAttemptsFake{threshold: 5})
	material := encryptedFactor(t, keys, user, factor, []byte("12345678901234567890"))
	store.active = &material
	principal := identityapp.SessionPrincipal{SessionID: identityID(1), UserID: user, AuthEpoch: 4, AssuranceLevel: "AAL1", AssuranceMethod: "PRIMARY_ONLY"}
	result, err := service.StepUp(context.Background(), principal, identityID(4), "287082", "TOTP")
	if err != nil {
		t.Fatal(err)
	}
	if store.stepUpCommit.CurrentSessionID != principal.SessionID || store.stepUpCommit.SessionID == principal.SessionID || store.stepUpCommit.Method != "TOTP" || result.Session.IsZero() || result.CSRF.IsZero() {
		t.Fatal("step-up session did not rotate safely")
	}
	if replay, replayErr := service.StepUp(context.Background(), principal, identityID(5), "287082", "TOTP"); replayErr == nil || replayErr.Error() != "mfa_invalid" || replay != (identityapp.LoginResult{}) {
		t.Fatal("TOTP replay accepted", replayErr)
	}
	store.recoveryHash = mfa.HashRecovery("ABCD2345EFGH6789")
	store.recoveryAvailable = true
	if _, err = service.StepUp(context.Background(), principal, identityID(6), "abcd2345efgh6789", "RECOVERY_CODE"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.StepUp(context.Background(), principal, identityID(7), "ABCD2345EFGH6789", "RECOVERY_CODE"); err == nil || err.Error() != "mfa_invalid" {
		t.Fatal("recovery-code replay accepted", err)
	}
}

type emailLoginDeliveryFake struct {
	available  bool
	deliveries int
}

func (d *emailLoginDeliveryFake) EmailLoginAvailable(context.Context) bool { return d.available }
func (d *emailLoginDeliveryFake) DeliverLoginCode(context.Context, email.Address, string, time.Time) error {
	d.deliveries++
	return nil
}

func TestMFALoginBypassesEmailCodeOnlyWhenSMTPIsUnavailable(t *testing.T) {
	now := time.Unix(59, 0).UTC()
	store := &mfaStoreFake{}
	service, _ := mfaServiceFixture(t, now, store, &mfaAttemptsFake{threshold: 5})
	delivery := &emailLoginDeliveryFake{available: false}
	if err := service.EnableEmailLogin(delivery, []byte("01234567890123456789012345678901")); err != nil {
		t.Fatal(err)
	}
	gate, err := service.BeginLogin(context.Background(), identityID(2), 4, identityapp.AuthenticationMethodPassword)
	if err != nil || gate.Required || delivery.deliveries != 0 {
		t.Fatal("unavailable SMTP did not bypass ordinary email OTP", gate, err)
	}
	delivery.available = true
	gate, err = service.BeginLogin(context.Background(), identityID(2), 4, identityapp.AuthenticationMethodPassword)
	if err != nil || !gate.Required || gate.Token == "" || delivery.deliveries != 1 {
		t.Fatal("available SMTP did not require and deliver email OTP", gate, err)
	}
}

func TestMFALoginDefersPrimarySessionAndConsumesChallengeBeforeReplay(t *testing.T) {
	now := time.Unix(59, 0).UTC()
	user, factor := identityID(2), identityID(3)
	store := &mfaStoreFake{}
	service, keys := mfaServiceFixture(t, now, store, &mfaAttemptsFake{threshold: 5})
	material := encryptedFactor(t, keys, user, factor, []byte("12345678901234567890"))
	store.active = &material
	gate, err := service.BeginLogin(context.Background(), user, 4, identityapp.AuthenticationMethodPassword)
	if err != nil || !gate.Required || gate.Token == "" {
		t.Fatal("MFA login was not deferred", err)
	}
	result, err := service.CompleteLogin(context.Background(), gate.Token, "287082", "EMAIL_OTP", identityID(4))
	if err != nil || result.Session.IsZero() || result.CSRF.IsZero() || store.loginCommit.Method != "TOTP" || store.loginCommit.SessionHash != result.Session.Hash() || store.loginCommit.PrimaryMethod != "PASSWORD" {
		t.Fatal("MFA login completion drifted", err)
	}
	if replay, replayErr := service.CompleteLogin(context.Background(), gate.Token, "287082", "TOTP", identityID(5)); replayErr == nil || replayErr.Error() != "mfa_invalid" || replay != (identityapp.LoginResult{}) {
		t.Fatal("MFA login challenge replay accepted", replayErr)
	}
}

func TestMFALimiterDeniesFifthFailureAndSubsequentAttempts(t *testing.T) {
	now := time.Unix(59, 0).UTC()
	user, factor := identityID(2), identityID(3)
	store := &mfaStoreFake{}
	attempts := &mfaAttemptsFake{threshold: 5}
	service, keys := mfaServiceFixture(t, now, store, attempts)
	material := encryptedFactor(t, keys, user, factor, []byte("12345678901234567890"))
	store.active = &material
	principal := identityapp.SessionPrincipal{SessionID: identityID(1), UserID: user, AuthEpoch: 4}
	for i := 1; i <= 4; i++ {
		if _, err := service.StepUp(context.Background(), principal, identityID(byte(10+i)), "000000", "TOTP"); err == nil || err.Error() != "mfa_invalid" {
			t.Fatalf("failure %d drifted: %v", i, err)
		}
	}
	if _, err := service.StepUp(context.Background(), principal, identityID(20), "000000", "TOTP"); err == nil || err.Error() != "mfa_attempt_denied" {
		t.Fatal("fifth failure was not denied", err)
	}
	if _, err := service.StepUp(context.Background(), principal, identityID(21), "287082", "TOTP"); err == nil || err.Error() != "mfa_attempt_denied" {
		t.Fatal("subsequent attempt bypassed denial", err)
	}
}

func TestLoginServiceReturnsOnlyMFAChallengeWhenFactorIsActive(t *testing.T) {
	auth := &authServiceFake{result: validAuthentication()}
	rehash := &rehasherFake{}
	tokens := &tokenGeneratorFake{pair: sessionPair(t)}
	ids := &loginIDsFake{id: identityID(7)}
	commit := &committerFake{}
	service := loginService(t, auth, rehash, tokens, ids, loginClockFake{at: time.Unix(59, 0)}, commit)
	gate := &mfaGateFake{result: identityapp.MFALoginGateResult{Required: true, Token: "synthetic-mfa-token", ExpiresAt: time.Unix(359, 0)}}
	if err := service.EnableMFA(gate); err != nil {
		t.Fatal(err)
	}
	result, err := service.Login(context.Background(), validLoginInput())
	if err != nil || !result.MFARequired || result.MFAToken != "synthetic-mfa-token" || !result.Session.IsZero() || !result.CSRF.IsZero() || tokens.calls != 0 || commit.calls != 0 || gate.calls != 1 {
		t.Fatal("primary login issued pre-MFA session", result, err)
	}
}
