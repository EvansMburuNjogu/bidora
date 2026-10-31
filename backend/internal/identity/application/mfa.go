package application

import (
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/mfa"
	"bidora/backend/internal/identity/session"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

var ErrMFAConflict = errors.New("mfa_conflict")
var ErrMFANotFound = errors.New("mfa_not_found")
var ErrMFAAttemptDenied = errors.New("mfa_attempt_denied")

type MFAAttemptReservation interface {
	Finish(context.Context, bool) error
}
type MFAAttemptController interface {
	BeginMFA(context.Context, [32]byte) (MFAAttemptReservation, error)
}
type MFAScopeDeriver interface {
	DeriveMFAScope(string, ID, ID) ([32]byte, error)
}

type MFAFactorMaterial struct {
	FactorID, UserID           ID
	Ciphertext, Nonce          []byte
	KeyVersion, FailedAttempts int32
	LastCounter, Version       int64
	ExpiresAt                  time.Time
}
type MFALoginChallenge struct {
	TransactionID, UserID, FactorID ID
	AuthEpoch                       int64
	Purpose, PrimaryMethod          string
	Attempts                        int32
	ExpiresAt                       time.Time
	CodeMAC                         [32]byte
	Recipient                       email.Address
	Factor                          MFAFactorMaterial
}
type MFALoginGateResult struct {
	Required  bool
	Token     string
	ExpiresAt time.Time
}
type MFAEnrollmentStore interface {
	HasActiveMFA(context.Context, ID) (bool, error)
	IsPlatformAdministrator(context.Context, ID) (bool, error)
	SavePendingMFA(context.Context, MFAPendingCommit) error
	GetPendingMFA(context.Context, ID) (MFAFactorMaterial, error)
	ActivateMFA(context.Context, MFAActivationCommit) error
	GetActiveMFA(context.Context, ID) (MFAFactorMaterial, error)
	CommitMFAStepUp(context.Context, MFAStepUpCommit) error
	BeginMFALogin(context.Context, ID, int64, AuthenticationMethod, [32]byte, [32]byte, ID, time.Time) error
	GetMFALoginChallenge(context.Context, [32]byte, [32]byte, time.Time) (MFALoginChallenge, error)
	RecordMFALoginFailure(context.Context, ID, time.Time) error
	RecordMFAEnrollmentFailure(context.Context, ID, ID, time.Time) error
	MFAEnrollmentAttemptDenied(context.Context, ID, time.Time) (bool, error)
	MFALoginAttemptDenied(context.Context, [32]byte, [32]byte, time.Time) (bool, error)
	CancelMFALogin(context.Context, [32]byte, [32]byte, time.Time) error
	CommitMFALogin(context.Context, MFALoginCommit) error
	CheckMFARecoveryCode(context.Context, ID, ID, [32]byte) (bool, error)
	RegenerateMFARecoveryCodes(context.Context, MFARecoveryRegenerationCommit) error
	ReplaceMFAFactor(context.Context, MFAReplacementCommit) error
}
type MFAPendingCommit struct {
	FactorID, UserID  ID
	Ciphertext, Nonce []byte
	KeyVersion        int32
	At                time.Time
}
type MFAEnrollment struct {
	FactorID    ID
	Secret, URI string
	ExpiresAt   time.Time
}
type MFAActivationCommit struct {
	FactorID, UserID, CurrentSessionID, SessionID, AuditID, OutboxID, CorrelationID ID
	RecoveryIDs                                                                     []ID
	RecoveryHashes                                                                  [][32]byte
	ExpectedFactorVersion, Counter, ExpectedAuthEpoch                               int64
	SessionHash, CSRFHash                                                           [32]byte
	At, IdleExpiresAt, AbsoluteExpiresAt                                            time.Time
}
type MFAStepUpCommit struct {
	FactorID, UserID, CurrentSessionID, SessionID, AuditID, OutboxID, CorrelationID ID
	ExpectedFactorVersion, Counter, AuthEpoch                                       int64
	RecoveryHash                                                                    *[32]byte
	SessionHash, CSRFHash                                                           [32]byte
	Method                                                                          string
	At, IdleExpiresAt, AbsoluteExpiresAt                                            time.Time
}
type MFALoginCommit struct {
	TransactionID, FactorID, UserID, SessionID, AuditID, OutboxID, CorrelationID ID
	AuthEpoch, ExpectedFactorVersion, Counter                                    int64
	RecoveryHash                                                                 *[32]byte
	SessionHash, CSRFHash                                                        [32]byte
	Method, PrimaryMethod                                                        string
	At, IdleExpiresAt, AbsoluteExpiresAt                                         time.Time
}
type MFARecoveryRegenerationCommit struct {
	FactorID, UserID, SessionID, AuditID, OutboxID, CorrelationID ID
	RecoveryIDs                                                   []ID
	RecoveryHashes                                                [][32]byte
	ExpectedFactorVersion, AuthEpoch                              int64
	At                                                            time.Time
}
type MFAReplacementCommit struct {
	MFAActivationCommit
	PreviousFactorID        ID
	ExpectedPreviousVersion int64
}
type MFAActivationResult struct {
	Login         LoginResult
	RecoveryCodes []string
}
type EmailLoginStore interface {
	BeginEmailLogin(context.Context, ID, int64, AuthenticationMethod, [32]byte, [32]byte, [32]byte, ID, time.Time) (email.Address, error)
}
type EmailLoginDeliveryEvidence interface {
	RecordEmailLoginSMTPAcceptance(context.Context, ID, ID, time.Time) error
}
type EmailLoginCodeDelivery interface {
	DeliverLoginCode(context.Context, email.Address, string, time.Time) error
}
type PlatformEmailLoginPolicy interface {
	PlatformEmailLoginRequired(context.Context, ID, int64) (bool, error)
}
type EmailLoginAvailability interface {
	EmailLoginAvailable(context.Context) bool
}
type EmailLoginResendLimiter interface {
	Allow(context.Context, [32]byte, [32]byte) error
}
type EmailLoginResendKeyer interface {
	EmailLoginResendIdentity(ID) ([32]byte, error)
}

type MFAService struct {
	store                 MFAEnrollmentStore
	keys                  *mfa.KeyRing
	tokens                SessionTokenGenerator
	ids                   LoginIDGenerator
	clock                 LoginClock
	attempts              MFAAttemptController
	scopes                MFAScopeDeriver
	platformEmailDelivery EmailLoginCodeDelivery
	platformEmailPolicy   PlatformEmailLoginPolicy
	emailDelivery         EmailLoginCodeDelivery
	emailMACKey           []byte
	emailResendLimits     EmailLoginResendLimiter
	emailResendKeys       EmailLoginResendKeyer
}

func NewMFAService(s MFAEnrollmentStore, k *mfa.KeyRing, t SessionTokenGenerator, ids LoginIDGenerator, c LoginClock, attempts MFAAttemptController, scopes MFAScopeDeriver) (*MFAService, error) {
	if s == nil || k == nil || t == nil || ids == nil || c == nil || attempts == nil || scopes == nil {
		return nil, errors.New("mfa_service_invalid")
	}
	return &MFAService{store: s, keys: k, tokens: t, ids: ids, clock: c, attempts: attempts, scopes: scopes}, nil
}

func (s *MFAService) EnableEmailLogin(delivery EmailLoginCodeDelivery, macKey []byte) error {
	if s == nil || delivery == nil || len(macKey) < 32 {
		return errors.New("email_login_config_invalid")
	}
	s.emailDelivery = delivery
	s.emailMACKey = append([]byte(nil), macKey...)
	return nil
}
func (s *MFAService) EnablePlatformEmailLogin(policy PlatformEmailLoginPolicy, delivery EmailLoginCodeDelivery) error {
	if s == nil || policy == nil || delivery == nil {
		return errors.New("platform_email_login_config_invalid")
	}
	s.platformEmailPolicy, s.platformEmailDelivery = policy, delivery
	return nil
}
func (s *MFAService) EnableEmailLoginResend(limits EmailLoginResendLimiter, keys EmailLoginResendKeyer) error {
	if s == nil || limits == nil || keys == nil {
		return errors.New("email_login_resend_config_invalid")
	}
	s.emailResendLimits = limits
	s.emailResendKeys = keys
	return nil
}
func (s *MFAService) Enroll(ctx context.Context, p SessionPrincipal, correlation ID) (MFAEnrollment, error) {
	now := s.clock.Now().UTC()
	if ctx == nil || !validMFAPrincipal(p) || correlation.IsZero() || !recentPrimary(p, now) {
		return MFAEnrollment{}, safe("mfa_enrollment_invalid", nil)
	}
	active, e := s.store.HasActiveMFA(ctx, p.UserID)
	if e != nil {
		return MFAEnrollment{}, safe("mfa_unavailable", e)
	}
	if active {
		return MFAEnrollment{}, safe("mfa_conflict", nil)
	}
	factor, e := s.ids.NewID()
	if e != nil {
		return MFAEnrollment{}, safe("mfa_unavailable", e)
	}
	secret, e := mfa.GenerateSecret()
	if e != nil {
		return MFAEnrollment{}, safe("mfa_unavailable", e)
	}
	defer zero(secret)
	cipher, nonce, ver, e := s.keys.Encrypt([16]byte(p.UserID), [16]byte(factor), secret)
	if e != nil {
		return MFAEnrollment{}, safe("mfa_unavailable", e)
	}
	if e = s.store.SavePendingMFA(ctx, MFAPendingCommit{factor, p.UserID, cipher, nonce, int32(ver), now}); e != nil {
		return MFAEnrollment{}, safe("mfa_unavailable", e)
	}
	encoded, _ := mfa.SecretBase32(secret)
	uri, e := mfa.URI("Bidora", fmt.Sprintf("%x", factor), secret)
	if e != nil {
		return MFAEnrollment{}, safe("mfa_unavailable", e)
	}
	return MFAEnrollment{factor, encoded, uri, now.Add(10 * time.Minute)}, nil
}
func (s *MFAService) Confirm(ctx context.Context, p SessionPrincipal, correlation ID, code string) (MFAActivationResult, error) {
	now := s.clock.Now().UTC()
	if ctx == nil || !validMFAPrincipal(p) || correlation.IsZero() || !recentPrimary(p, now) {
		return MFAActivationResult{}, safe("mfa_enrollment_invalid", nil)
	}
	factor, e := s.store.GetPendingMFA(ctx, p.UserID)
	if e != nil {
		denied, checkErr := s.store.MFAEnrollmentAttemptDenied(ctx, p.UserID, now)
		if checkErr != nil {
			return MFAActivationResult{}, safe("mfa_unavailable", checkErr)
		}
		if denied {
			return MFAActivationResult{}, safe("mfa_attempt_denied", ErrMFAAttemptDenied)
		}
		return MFAActivationResult{}, safe("mfa_invalid", nil)
	}
	reservation, e := s.beginAttempt(ctx, "enrollment", p.UserID, factor.FactorID)
	if e != nil {
		return MFAActivationResult{}, e
	}
	secret, e := s.keys.Decrypt([16]byte(p.UserID), [16]byte(factor.FactorID), factor.Ciphertext, factor.Nonce, int(factor.KeyVersion))
	if e != nil {
		return MFAActivationResult{}, safe("mfa_unavailable", e)
	}
	defer zero(secret)
	counter, ok := mfa.Verify(secret, code, now, factor.LastCounter)
	if !ok {
		_ = s.store.RecordMFAEnrollmentFailure(ctx, factor.FactorID, p.UserID, now)
		if finish := finishMFAAttempt(ctx, reservation, false); finish != nil {
			return MFAActivationResult{}, finish
		}
		return MFAActivationResult{}, safe("mfa_invalid", nil)
	}
	if e = reservation.Finish(ctx, true); e != nil {
		return MFAActivationResult{}, safe("mfa_unavailable", e)
	}
	codes, hashes, e := mfa.GenerateRecoveryCodes()
	if e != nil {
		return MFAActivationResult{}, safe("mfa_unavailable", e)
	}
	pair, e := s.tokens.GenerateSessionPair()
	if e != nil {
		return MFAActivationResult{}, safe("mfa_unavailable", e)
	}
	ids, e := newIdentityIDs(s.ids, 13)
	if e != nil {
		return MFAActivationResult{}, safe("mfa_unavailable", e)
	}
	idle, absolute, e := s.assuredSessionExpiries(ctx, p.UserID, now)
	if e != nil {
		return MFAActivationResult{}, safe("mfa_unavailable", e)
	}
	commit := MFAActivationCommit{FactorID: factor.FactorID, UserID: p.UserID, CurrentSessionID: p.SessionID, SessionID: ids[0], AuditID: ids[1], OutboxID: ids[2], CorrelationID: correlation, RecoveryIDs: ids[3:], RecoveryHashes: hashes, ExpectedFactorVersion: factor.Version, Counter: counter, ExpectedAuthEpoch: p.AuthEpoch, SessionHash: pair.Session.Hash(), CSRFHash: pair.CSRF.Hash(), At: now, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}
	if e = s.store.ActivateMFA(ctx, commit); e != nil {
		return MFAActivationResult{}, mfaStoreError(e)
	}
	return MFAActivationResult{Login: LoginResult{Session: pair.Session, CSRF: pair.CSRF, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}, RecoveryCodes: codes}, nil
}
func (s *MFAService) StepUp(ctx context.Context, p SessionPrincipal, correlation ID, code, kind string) (LoginResult, error) {
	now := s.clock.Now().UTC()
	code = strings.ToUpper(strings.TrimSpace(code))
	if ctx == nil || !validMFAPrincipal(p) || correlation.IsZero() || (kind != "TOTP" && kind != "RECOVERY_CODE") {
		return LoginResult{}, safe("mfa_invalid", nil)
	}
	factor, e := s.store.GetActiveMFA(ctx, p.UserID)
	if e != nil {
		return LoginResult{}, safe("mfa_invalid", nil)
	}
	reservation, e := s.beginAttempt(ctx, "step-up", p.UserID, factor.FactorID)
	if e != nil {
		return LoginResult{}, e
	}
	counter := int64(-1)
	var recovery *[32]byte
	if kind == "TOTP" {
		secret, e := s.keys.Decrypt([16]byte(p.UserID), [16]byte(factor.FactorID), factor.Ciphertext, factor.Nonce, int(factor.KeyVersion))
		if e != nil {
			return LoginResult{}, safe("mfa_unavailable", e)
		}
		defer zero(secret)
		var ok bool
		counter, ok = mfa.Verify(secret, code, now, factor.LastCounter)
		if !ok {
			if finish := finishMFAAttempt(ctx, reservation, false); finish != nil {
				return LoginResult{}, finish
			}
			return LoginResult{}, safe("mfa_invalid", nil)
		}
	} else {
		h := mfa.HashRecovery(code)
		recovery = &h
		exists, checkErr := s.store.CheckMFARecoveryCode(ctx, factor.FactorID, p.UserID, h)
		if checkErr != nil {
			return LoginResult{}, safe("mfa_unavailable", checkErr)
		}
		if !exists {
			if finish := finishMFAAttempt(ctx, reservation, false); finish != nil {
				return LoginResult{}, finish
			}
			return LoginResult{}, safe("mfa_invalid", nil)
		}
	}
	if e = reservation.Finish(ctx, true); e != nil {
		return LoginResult{}, safe("mfa_unavailable", e)
	}
	pair, e := s.tokens.GenerateSessionPair()
	if e != nil {
		return LoginResult{}, safe("mfa_unavailable", e)
	}
	ids, e := newIdentityIDs(s.ids, 3)
	if e != nil {
		return LoginResult{}, safe("mfa_unavailable", e)
	}
	idle, absolute, e := s.assuredSessionExpiries(ctx, p.UserID, now)
	if e != nil {
		return LoginResult{}, safe("mfa_unavailable", e)
	}
	commit := MFAStepUpCommit{FactorID: factor.FactorID, UserID: p.UserID, CurrentSessionID: p.SessionID, SessionID: ids[0], AuditID: ids[1], OutboxID: ids[2], CorrelationID: correlation, ExpectedFactorVersion: factor.Version, Counter: counter, AuthEpoch: p.AuthEpoch, RecoveryHash: recovery, SessionHash: pair.Session.Hash(), CSRFHash: pair.CSRF.Hash(), Method: kind, At: now, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}
	if e = s.store.CommitMFAStepUp(ctx, commit); e != nil {
		return LoginResult{}, mfaStoreError(e)
	}
	return LoginResult{Session: pair.Session, CSRF: pair.CSRF, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}, nil
}
func (s *MFAService) BeginLogin(ctx context.Context, user ID, epoch int64, method AuthenticationMethod) (MFALoginGateResult, error) {
	if ctx == nil || user.IsZero() || epoch < 1 || (method != AuthenticationMethodPassword && method != AuthenticationMethodGoogle) || s == nil || s.store == nil {
		return MFALoginGateResult{}, safe("mfa_unavailable", nil)
	}
	platformRequired := false
	if s.platformEmailPolicy != nil {
		required, err := s.platformEmailPolicy.PlatformEmailLoginRequired(ctx, user, epoch)
		if err != nil {
			return MFALoginGateResult{}, safe("mfa_unavailable", err)
		}
		platformRequired = required
		if required && method == AuthenticationMethodPassword {
			return s.beginEmailLogin(ctx, user, epoch, method)
		}
	}
	active, err := s.store.HasActiveMFA(ctx, user)
	if err != nil {
		return MFALoginGateResult{}, safe("mfa_unavailable", err)
	}
	if active {
		return s.beginTOTPLogin(ctx, user, epoch, method)
	}
	if platformRequired {
		return MFALoginGateResult{}, safe("mfa_unavailable", nil)
	}
	if s.emailDelivery == nil {
		return MFALoginGateResult{}, nil
	}
	if availability, ok := s.emailDelivery.(EmailLoginAvailability); ok && !availability.EmailLoginAvailable(ctx) {
		return MFALoginGateResult{}, nil
	}
	return s.beginEmailLogin(ctx, user, epoch, method)
}

func (s *MFAService) beginTOTPLogin(ctx context.Context, user ID, epoch int64, method AuthenticationMethod) (MFALoginGateResult, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return MFALoginGateResult{}, safe("mfa_unavailable", err)
	}
	defer zero(raw)
	token := base64.RawURLEncoding.EncodeToString(raw)
	tokenHash := sha256.Sum256(append([]byte("BIDORA:EMAIL:LOGIN:TOKEN:V1:"), raw...))
	bindingHash := sha256.Sum256(append([]byte("BIDORA:EMAIL:LOGIN:BROWSER:V1:"), raw...))
	transactionID, err := s.ids.NewID()
	if err != nil {
		return MFALoginGateResult{}, safe("mfa_unavailable", err)
	}
	now := s.clock.Now().UTC()
	if err = s.store.BeginMFALogin(ctx, user, epoch, method, tokenHash, bindingHash, transactionID, now); err != nil {
		return MFALoginGateResult{}, safe("mfa_unavailable", err)
	}
	return MFALoginGateResult{Required: true, Token: token, ExpiresAt: now.Add(5 * time.Minute)}, nil
}

func (s *MFAService) CancelLogin(ctx context.Context, raw string) error {
	if ctx == nil || s == nil || s.store == nil || s.clock == nil {
		return safe("mfa_unavailable", nil)
	}
	decoded, tokenHash, bindingHash, ok := decodeEmailLoginToken(raw)
	if !ok {
		return nil
	}
	defer zero(decoded)
	if err := s.store.CancelMFALogin(ctx, tokenHash, bindingHash, s.clock.Now().UTC()); err != nil {
		return safe("mfa_unavailable", err)
	}
	return nil
}

func (s *MFAService) ResendLogin(ctx context.Context, raw string, networkScope [32]byte) (MFALoginGateResult, error) {
	decoded, tokenHash, bindingHash, ok := decodeEmailLoginToken(raw)
	if !ok {
		return MFALoginGateResult{}, safe("mfa_invalid", nil)
	}
	defer zero(decoded)
	challenge, err := s.store.GetMFALoginChallenge(ctx, tokenHash, bindingHash, s.clock.Now().UTC())
	if err != nil {
		return MFALoginGateResult{}, safe("mfa_invalid", nil)
	}
	if s.emailResendLimits == nil || s.emailResendKeys == nil || networkScope == ([32]byte{}) {
		return MFALoginGateResult{}, safe("mfa_unavailable", nil)
	}
	identityScope, err := s.emailResendKeys.EmailLoginResendIdentity(challenge.UserID)
	if err != nil {
		return MFALoginGateResult{}, safe("mfa_unavailable", err)
	}
	if err = s.emailResendLimits.Allow(ctx, identityScope, networkScope); err != nil {
		if err.Error() == "email_login_resend_denied" {
			return MFALoginGateResult{}, safe("email_login_resend_denied", err)
		}
		return MFALoginGateResult{}, safe("mfa_unavailable", err)
	}
	return s.beginEmailLogin(ctx, challenge.UserID, challenge.AuthEpoch, AuthenticationMethod(challenge.PrimaryMethod))
}

func (s *MFAService) beginEmailLogin(ctx context.Context, user ID, epoch int64, method AuthenticationMethod) (MFALoginGateResult, error) {
	if ctx == nil || user.IsZero() || epoch < 1 || (method != AuthenticationMethodPassword && method != AuthenticationMethodGoogle) || s == nil || s.emailDelivery == nil || len(s.emailMACKey) < 32 {
		return MFALoginGateResult{}, safe("mfa_unavailable", nil)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return MFALoginGateResult{}, safe("mfa_unavailable", err)
	}
	defer zero(raw)
	token := base64.RawURLEncoding.EncodeToString(raw)
	tokenHash := sha256.Sum256(append([]byte("BIDORA:EMAIL:LOGIN:TOKEN:V1:"), raw...))
	bindingHash := sha256.Sum256(append([]byte("BIDORA:EMAIL:LOGIN:BROWSER:V1:"), raw...))
	transactionID, err := s.ids.NewID()
	if err != nil {
		return MFALoginGateResult{}, safe("mfa_unavailable", err)
	}
	code, err := generateEmailLoginCode()
	if err != nil {
		return MFALoginGateResult{}, safe("mfa_unavailable", err)
	}
	codeMAC := emailLoginCodeMAC(s.emailMACKey, transactionID, code)
	now := s.clock.Now().UTC()
	expiresAt := now.Add(5 * time.Minute)
	emailStore, ok := s.store.(EmailLoginStore)
	if !ok {
		return MFALoginGateResult{}, safe("mfa_unavailable", nil)
	}
	recipient, err := emailStore.BeginEmailLogin(ctx, user, epoch, method, tokenHash, bindingHash, codeMAC, transactionID, now)
	if err != nil {
		return MFALoginGateResult{}, safe("mfa_unavailable", err)
	}
	delivery := s.emailDelivery
	if s.platformEmailPolicy != nil {
		required, policyErr := s.platformEmailPolicy.PlatformEmailLoginRequired(ctx, user, epoch)
		if policyErr != nil {
			return MFALoginGateResult{}, safe("mfa_unavailable", policyErr)
		}
		if required {
			delivery = s.platformEmailDelivery
		}
	}
	if delivery == nil {
		return MFALoginGateResult{}, safe("mfa_unavailable", nil)
	}
	if err = delivery.DeliverLoginCode(ctx, recipient, code, expiresAt); err != nil {
		return MFALoginGateResult{}, safe("mfa_unavailable", err)
	}
	if recorder, ok := s.store.(EmailLoginDeliveryEvidence); ok {
		if err = recorder.RecordEmailLoginSMTPAcceptance(ctx, user, transactionID, s.clock.Now().UTC()); err != nil {
			return MFALoginGateResult{}, safe("mfa_unavailable", err)
		}
	}
	return MFALoginGateResult{Required: true, Token: token, ExpiresAt: expiresAt}, nil
}

func (s *MFAService) CompleteLogin(ctx context.Context, raw, code, kind string, correlation ID) (LoginResult, error) {
	decoded, tokenHash, bindingHash, ok := decodeEmailLoginToken(raw)
	if !ok || correlation.IsZero() || (kind != "EMAIL_OTP" && kind != "TOTP" && kind != "RECOVERY_CODE") {
		return LoginResult{}, safe("mfa_invalid", nil)
	}
	defer zero(decoded)
	now := s.clock.Now().UTC()
	challenge, err := s.store.GetMFALoginChallenge(ctx, tokenHash, bindingHash, now)
	if err != nil {
		denied, checkErr := s.store.MFALoginAttemptDenied(ctx, tokenHash, bindingHash, now)
		if checkErr != nil {
			return LoginResult{}, safe("mfa_unavailable", checkErr)
		}
		if denied {
			return LoginResult{}, safe("mfa_attempt_denied", ErrMFAAttemptDenied)
		}
		return LoginResult{}, safe("mfa_invalid", nil)
	}
	if challenge.Purpose == "" && !challenge.Factor.FactorID.IsZero() {
		challenge.Purpose = "MFA_LOGIN"
	}
	if challenge.Purpose == "MFA_LOGIN" {
		// The browser login-code contract is method-neutral in presentation and
		// currently submits EMAIL_OTP. The server-owned transaction determines
		// that this is an enrolled TOTP challenge and preserves AAL2 provenance.
		if kind == "EMAIL_OTP" {
			kind = "TOTP"
		}
		return s.completeTOTPLogin(ctx, challenge, code, kind, correlation, now)
	}
	if challenge.Purpose != "EMAIL_LOGIN" || kind != "EMAIL_OTP" || !validEmailLoginCode(code) {
		return LoginResult{}, safe("mfa_invalid", nil)
	}
	reservation, err := s.beginAttempt(ctx, "email-login", challenge.UserID, challenge.TransactionID)
	if err != nil {
		return LoginResult{}, err
	}
	expected := emailLoginCodeMAC(s.emailMACKey, challenge.TransactionID, code)
	if !hmac.Equal(expected[:], challenge.CodeMAC[:]) {
		_ = s.store.RecordMFALoginFailure(ctx, challenge.TransactionID, now)
		if finish := finishMFAAttempt(ctx, reservation, false); finish != nil {
			return LoginResult{}, finish
		}
		return LoginResult{}, safe("mfa_invalid", nil)
	}
	if err = reservation.Finish(ctx, true); err != nil {
		return LoginResult{}, safe("mfa_unavailable", err)
	}
	pair, err := s.tokens.GenerateSessionPair()
	if err != nil {
		return LoginResult{}, safe("mfa_unavailable", err)
	}
	ids, err := newIdentityIDs(s.ids, 3)
	if err != nil {
		return LoginResult{}, safe("mfa_unavailable", err)
	}
	idle, absolute, err := session.Expiries(now)
	if err != nil {
		return LoginResult{}, safe("mfa_unavailable", err)
	}
	commit := MFALoginCommit{TransactionID: challenge.TransactionID, UserID: challenge.UserID, SessionID: ids[0], AuditID: ids[1], OutboxID: ids[2], CorrelationID: correlation, AuthEpoch: challenge.AuthEpoch, SessionHash: pair.Session.Hash(), CSRFHash: pair.CSRF.Hash(), Method: "EMAIL_OTP", PrimaryMethod: challenge.PrimaryMethod, At: now, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}
	if err = s.store.CommitMFALogin(ctx, commit); err != nil {
		return LoginResult{}, mfaStoreError(err)
	}
	return LoginResult{Session: pair.Session, CSRF: pair.CSRF, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}, nil
}

func (s *MFAService) completeTOTPLogin(ctx context.Context, challenge MFALoginChallenge, code, kind string, correlation ID, now time.Time) (LoginResult, error) {
	if kind != "TOTP" && kind != "RECOVERY_CODE" {
		return LoginResult{}, safe("mfa_invalid", nil)
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	factor := challenge.Factor
	if factor.FactorID.IsZero() || factor.UserID != challenge.UserID {
		return LoginResult{}, safe("mfa_invalid", nil)
	}
	reservation, err := s.beginAttempt(ctx, "login", challenge.UserID, factor.FactorID)
	if err != nil {
		return LoginResult{}, err
	}
	counter := int64(-1)
	var recovery *[32]byte
	if kind == "TOTP" {
		secret, decryptErr := s.keys.Decrypt([16]byte(challenge.UserID), [16]byte(factor.FactorID), factor.Ciphertext, factor.Nonce, int(factor.KeyVersion))
		if decryptErr != nil {
			return LoginResult{}, safe("mfa_unavailable", decryptErr)
		}
		defer zero(secret)
		var valid bool
		counter, valid = mfa.Verify(secret, code, now, factor.LastCounter)
		if !valid {
			_ = s.store.RecordMFALoginFailure(ctx, challenge.TransactionID, now)
			if finish := finishMFAAttempt(ctx, reservation, false); finish != nil {
				return LoginResult{}, finish
			}
			return LoginResult{}, safe("mfa_invalid", nil)
		}
	} else {
		hash := mfa.HashRecovery(code)
		recovery = &hash
		valid, checkErr := s.store.CheckMFARecoveryCode(ctx, factor.FactorID, challenge.UserID, hash)
		if checkErr != nil {
			return LoginResult{}, safe("mfa_unavailable", checkErr)
		}
		if !valid {
			_ = s.store.RecordMFALoginFailure(ctx, challenge.TransactionID, now)
			if finish := finishMFAAttempt(ctx, reservation, false); finish != nil {
				return LoginResult{}, finish
			}
			return LoginResult{}, safe("mfa_invalid", nil)
		}
	}
	if err = reservation.Finish(ctx, true); err != nil {
		return LoginResult{}, safe("mfa_unavailable", err)
	}
	pair, err := s.tokens.GenerateSessionPair()
	if err != nil {
		return LoginResult{}, safe("mfa_unavailable", err)
	}
	ids, err := newIdentityIDs(s.ids, 3)
	if err != nil {
		return LoginResult{}, safe("mfa_unavailable", err)
	}
	idle, absolute, err := s.assuredSessionExpiries(ctx, challenge.UserID, now)
	if err != nil {
		return LoginResult{}, safe("mfa_unavailable", err)
	}
	commit := MFALoginCommit{TransactionID: challenge.TransactionID, FactorID: factor.FactorID, UserID: challenge.UserID, SessionID: ids[0], AuditID: ids[1], OutboxID: ids[2], CorrelationID: correlation, AuthEpoch: challenge.AuthEpoch, ExpectedFactorVersion: factor.Version, Counter: counter, RecoveryHash: recovery, SessionHash: pair.Session.Hash(), CSRFHash: pair.CSRF.Hash(), Method: kind, PrimaryMethod: challenge.PrimaryMethod, At: now, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}
	if err = s.store.CommitMFALogin(ctx, commit); err != nil {
		return LoginResult{}, mfaStoreError(err)
	}
	return LoginResult{Session: pair.Session, CSRF: pair.CSRF, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}, nil
}

func (s *MFAService) assuredSessionExpiries(ctx context.Context, user ID, at time.Time) (time.Time, time.Time, error) {
	platform, err := s.store.IsPlatformAdministrator(ctx, user)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if platform {
		return session.PlatformExpiries(at)
	}
	return session.Expiries(at)
}

func generateEmailLoginCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

func emailLoginCodeMAC(key []byte, transaction ID, code string) [32]byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("BIDORA:EMAIL:LOGIN:CODE:V1\x00"))
	_, _ = mac.Write(transaction[:])
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(code))
	var out [32]byte
	copy(out[:], mac.Sum(nil))
	return out
}

func decodeEmailLoginToken(raw string) ([]byte, [32]byte, [32]byte, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) != 32 {
		return nil, [32]byte{}, [32]byte{}, false
	}
	tokenHash := sha256.Sum256(append([]byte("BIDORA:EMAIL:LOGIN:TOKEN:V1:"), decoded...))
	bindingHash := sha256.Sum256(append([]byte("BIDORA:EMAIL:LOGIN:BROWSER:V1:"), decoded...))
	return decoded, tokenHash, bindingHash, true
}

func validEmailLoginCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for i := range code {
		if code[i] < '0' || code[i] > '9' {
			return false
		}
	}
	return true
}

func (s *MFAService) RegenerateRecoveryCodes(ctx context.Context, p SessionPrincipal, correlation ID) ([]string, error) {
	now := s.clock.Now().UTC()
	if ctx == nil || !validMFAPrincipal(p) || correlation.IsZero() || !recentAAL2(p, now) {
		return nil, safe("mfa_assurance_required", nil)
	}
	factor, e := s.store.GetActiveMFA(ctx, p.UserID)
	if e != nil {
		return nil, safe("mfa_invalid", nil)
	}
	codes, hashes, e := mfa.GenerateRecoveryCodes()
	if e != nil {
		return nil, safe("mfa_unavailable", e)
	}
	ids, e := newIdentityIDs(s.ids, 12)
	if e != nil {
		return nil, safe("mfa_unavailable", e)
	}
	commit := MFARecoveryRegenerationCommit{FactorID: factor.FactorID, UserID: p.UserID, SessionID: p.SessionID, AuditID: ids[0], OutboxID: ids[1], CorrelationID: correlation, RecoveryIDs: ids[2:], RecoveryHashes: hashes, ExpectedFactorVersion: factor.Version, AuthEpoch: p.AuthEpoch, At: now}
	if e = s.store.RegenerateMFARecoveryCodes(ctx, commit); e != nil {
		return nil, mfaStoreError(e)
	}
	return codes, nil
}
func (s *MFAService) BeginReplacement(ctx context.Context, p SessionPrincipal, correlation ID) (MFAEnrollment, error) {
	now := s.clock.Now().UTC()
	if ctx == nil || !validMFAPrincipal(p) || correlation.IsZero() || !recentAAL2(p, now) {
		return MFAEnrollment{}, safe("mfa_assurance_required", nil)
	}
	if _, e := s.store.GetActiveMFA(ctx, p.UserID); e != nil {
		return MFAEnrollment{}, safe("mfa_invalid", nil)
	}
	return s.savePending(ctx, p.UserID, now)
}
func (s *MFAService) ConfirmReplacement(ctx context.Context, p SessionPrincipal, correlation ID, code string) (MFAActivationResult, error) {
	now := s.clock.Now().UTC()
	if ctx == nil || !validMFAPrincipal(p) || correlation.IsZero() || !recentAAL2(p, now) {
		return MFAActivationResult{}, safe("mfa_assurance_required", nil)
	}
	pending, e := s.store.GetPendingMFA(ctx, p.UserID)
	if e != nil {
		denied, checkErr := s.store.MFAEnrollmentAttemptDenied(ctx, p.UserID, now)
		if checkErr != nil {
			return MFAActivationResult{}, safe("mfa_unavailable", checkErr)
		}
		if denied {
			return MFAActivationResult{}, safe("mfa_attempt_denied", ErrMFAAttemptDenied)
		}
		return MFAActivationResult{}, safe("mfa_invalid", nil)
	}
	active, e := s.store.GetActiveMFA(ctx, p.UserID)
	if e != nil {
		return MFAActivationResult{}, safe("mfa_invalid", nil)
	}
	reservation, e := s.beginAttempt(ctx, "replacement", p.UserID, pending.FactorID)
	if e != nil {
		return MFAActivationResult{}, e
	}
	secret, e := s.keys.Decrypt([16]byte(p.UserID), [16]byte(pending.FactorID), pending.Ciphertext, pending.Nonce, int(pending.KeyVersion))
	if e != nil {
		return MFAActivationResult{}, safe("mfa_unavailable", e)
	}
	defer zero(secret)
	counter, ok := mfa.Verify(secret, code, now, pending.LastCounter)
	if !ok {
		_ = s.store.RecordMFAEnrollmentFailure(ctx, pending.FactorID, p.UserID, now)
		if finish := finishMFAAttempt(ctx, reservation, false); finish != nil {
			return MFAActivationResult{}, finish
		}
		return MFAActivationResult{}, safe("mfa_invalid", nil)
	}
	if e = reservation.Finish(ctx, true); e != nil {
		return MFAActivationResult{}, safe("mfa_unavailable", e)
	}
	codes, hashes, e := mfa.GenerateRecoveryCodes()
	if e != nil {
		return MFAActivationResult{}, safe("mfa_unavailable", e)
	}
	pair, e := s.tokens.GenerateSessionPair()
	if e != nil {
		return MFAActivationResult{}, safe("mfa_unavailable", e)
	}
	ids, e := newIdentityIDs(s.ids, 13)
	if e != nil {
		return MFAActivationResult{}, safe("mfa_unavailable", e)
	}
	idle, absolute, e := s.assuredSessionExpiries(ctx, p.UserID, now)
	if e != nil {
		return MFAActivationResult{}, safe("mfa_unavailable", e)
	}
	base := MFAActivationCommit{FactorID: pending.FactorID, UserID: p.UserID, CurrentSessionID: p.SessionID, SessionID: ids[0], AuditID: ids[1], OutboxID: ids[2], CorrelationID: correlation, RecoveryIDs: ids[3:], RecoveryHashes: hashes, ExpectedFactorVersion: pending.Version, Counter: counter, ExpectedAuthEpoch: p.AuthEpoch, SessionHash: pair.Session.Hash(), CSRFHash: pair.CSRF.Hash(), At: now, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}
	if e = s.store.ReplaceMFAFactor(ctx, MFAReplacementCommit{MFAActivationCommit: base, PreviousFactorID: active.FactorID, ExpectedPreviousVersion: active.Version}); e != nil {
		return MFAActivationResult{}, mfaStoreError(e)
	}
	return MFAActivationResult{Login: LoginResult{Session: pair.Session, CSRF: pair.CSRF, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}, RecoveryCodes: codes}, nil
}
func (s *MFAService) savePending(ctx context.Context, user ID, now time.Time) (MFAEnrollment, error) {
	factor, e := s.ids.NewID()
	if e != nil {
		return MFAEnrollment{}, safe("mfa_unavailable", e)
	}
	secret, e := mfa.GenerateSecret()
	if e != nil {
		return MFAEnrollment{}, safe("mfa_unavailable", e)
	}
	defer zero(secret)
	cipher, nonce, ver, e := s.keys.Encrypt([16]byte(user), [16]byte(factor), secret)
	if e != nil {
		return MFAEnrollment{}, safe("mfa_unavailable", e)
	}
	if e = s.store.SavePendingMFA(ctx, MFAPendingCommit{factor, user, cipher, nonce, int32(ver), now}); e != nil {
		return MFAEnrollment{}, safe("mfa_unavailable", e)
	}
	encoded, _ := mfa.SecretBase32(secret)
	uri, e := mfa.URI("Bidora", fmt.Sprintf("%x", factor), secret)
	if e != nil {
		return MFAEnrollment{}, safe("mfa_unavailable", e)
	}
	return MFAEnrollment{factor, encoded, uri, now.Add(10 * time.Minute)}, nil
}
func finishMFAAttempt(ctx context.Context, r MFAAttemptReservation, success bool) error {
	e := r.Finish(ctx, success)
	if errors.Is(e, ErrMFAAttemptDenied) {
		return safe("mfa_attempt_denied", e)
	}
	if e != nil {
		return safe("mfa_unavailable", e)
	}
	return nil
}
func (s *MFAService) beginAttempt(ctx context.Context, purpose string, user, target ID) (MFAAttemptReservation, error) {
	scope, deriveErr := s.scopes.DeriveMFAScope(purpose, user, target)
	if deriveErr != nil {
		return nil, safe("mfa_unavailable", deriveErr)
	}
	r, e := s.attempts.BeginMFA(ctx, scope)
	if e != nil {
		if errors.Is(e, ErrMFAAttemptDenied) {
			return nil, safe("mfa_attempt_denied", e)
		}
		return nil, safe("mfa_unavailable", e)
	}
	return r, nil
}
func validMFAPrincipal(p SessionPrincipal) bool {
	return !p.UserID.IsZero() && !p.SessionID.IsZero() && p.AuthEpoch > 0
}
func recentAAL2(p SessionPrincipal, n time.Time) bool {
	age := n.Sub(p.AssuredAt.UTC())
	return p.AssuranceLevel == "AAL2" && (p.AssuranceMethod == "TOTP" || p.AssuranceMethod == "RECOVERY_CODE") && age >= 0 && age <= 15*time.Minute
}
func recentPrimary(p SessionPrincipal, n time.Time) bool {
	a := p.AbsoluteExpiresAt.UTC().Add(-12 * time.Hour)
	d := n.Sub(a)
	return d >= 0 && d <= 15*time.Minute
}
func zero(v []byte) {
	for i := range v {
		v[i] = 0
	}
}
func mfaStoreError(e error) error {
	if errors.Is(e, ErrMFAConflict) {
		return safe("mfa_conflict", e)
	}
	return safe("mfa_unavailable", e)
}
