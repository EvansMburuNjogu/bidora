package identity_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/password"
)

func identityID(n byte) identityapp.ID { var id identityapp.ID; id[15] = n; return id }
func networkScope() attempt.Key        { var key attempt.Key; key[31] = 1; return key }
func loginInput(email string, passwordValue []byte) identityapp.AuthenticateInput {
	return identityapp.AuthenticateInput{Email: email, Password: passwordValue, NetworkScope: networkScope()}
}

type credentialReaderFake struct {
	record  identityapp.CredentialRecord
	err     error
	lookups []string
}

func (r *credentialReaderFake) LookupPasswordCredential(_ context.Context, lookup string) (identityapp.CredentialRecord, error) {
	r.lookups = append(r.lookups, lookup)
	return r.record, r.err
}

type attemptFake struct {
	beginErr, finishErr error
	scopes              []attempt.Scopes
	finishes            []bool
	nilReservation      bool
}

func (a *attemptFake) Begin(_ context.Context, scopes attempt.Scopes) (identityapp.AttemptReservation, error) {
	a.scopes = append(a.scopes, scopes)
	if a.beginErr != nil {
		return nil, a.beginErr
	}
	if a.nilReservation {
		return nil, nil
	}
	return a, nil
}
func (a *attemptFake) Finish(_ context.Context, success bool) error {
	a.finishes = append(a.finishes, success)
	return a.finishErr
}

type verifierFake struct {
	result             password.Verification
	err, validationErr error
	phcs               []string
	passwords          [][]byte
}

func (v *verifierFake) ValidatePHC(string) error { return v.validationErr }
func (v *verifierFake) Verify(candidate []byte, phc string) (password.Verification, error) {
	v.phcs = append(v.phcs, phc)
	v.passwords = append(v.passwords, append([]byte(nil), candidate...))
	return v.result, v.err
}
func verifiedRecord() identityapp.CredentialRecord {
	at := time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
	return identityapp.CredentialRecord{UserID: identityID(1), PasswordPHC: "real-phc", EmailVerifiedAt: &at, AuthEpoch: 4, CredentialVersion: 2}
}
func keyer(t *testing.T) *attempt.Deriver {
	t.Helper()
	d, err := attempt.NewDeriver([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func authenticator(t *testing.T, reader *credentialReaderFake, attempts *attemptFake, verifier *verifierFake) *identityapp.Authenticator {
	t.Helper()
	a, err := identityapp.NewAuthenticator(reader, attempts, keyer(t), verifier, "dummy-phc")
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAuthenticationSuccessUsesCanonicalLookupAndReservation(t *testing.T) {
	reader := &credentialReaderFake{record: verifiedRecord()}
	attempts := &attemptFake{}
	verifier := &verifierFake{result: password.Verification{Match: true, NeedsRehash: true}}
	a := authenticator(t, reader, attempts, verifier)
	result, err := a.Authenticate(context.Background(), loginInput("User+Tag@Example.COM", []byte("synthetic password")))
	if err != nil {
		t.Fatal(err)
	}
	if result.UserID != identityID(1) || result.AuthEpoch != 4 || result.CredentialVersion != 2 || !result.NeedsPasswordRehash {
		t.Fatal("authentication result drifted", result)
	}
	if len(reader.lookups) != 1 || reader.lookups[0] != "user+tag@example.com" || len(verifier.phcs) != 1 || verifier.phcs[0] != "real-phc" || len(attempts.finishes) != 1 || !attempts.finishes[0] || len(attempts.scopes) != 1 || !attempts.scopes[0].Valid() || attempts.scopes[0].Network != networkScope() {
		t.Fatal("authentication orchestration drifted")
	}
	other := &attemptFake{}
	a = authenticator(t, &credentialReaderFake{record: verifiedRecord()}, other, &verifierFake{result: password.Verification{Match: true}})
	_, _ = a.Authenticate(context.Background(), loginInput("user+tag@example.com", []byte("synthetic password")))
	if attempts.scopes[0].Identity != other.scopes[0].Identity {
		t.Fatal("case-equivalent identities received different limiter scope")
	}
}

func TestAuthenticationUnknownAndMalformedEmailsUseDummyWork(t *testing.T) {
	for _, tc := range []struct {
		name, email string
		readerErr   error
		readerCalls int
	}{{"unknown", "missing@example.com", identityapp.ErrCredentialNotFound, 1}, {"malformed", "not an email", nil, 0}, {"non ascii", "josé@example.com", nil, 0}, {"empty", "", nil, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &credentialReaderFake{err: tc.readerErr}
			attempts := &attemptFake{}
			verifier := &verifierFake{}
			a := authenticator(t, reader, attempts, verifier)
			result, err := a.Authenticate(context.Background(), loginInput(tc.email, []byte("guess")))
			if err == nil || err.Error() != "credentials_invalid" || result != (identityapp.Authentication{}) || len(reader.lookups) != tc.readerCalls || len(verifier.phcs) != 1 || verifier.phcs[0] != "dummy-phc" || len(attempts.finishes) != 1 || attempts.finishes[0] || errors.Is(err, identityapp.ErrCredentialNotFound) {
				t.Fatal("unknown identity handling unsafe", result, err)
			}
		})
	}
}

func TestAuthenticationConcealsWrongUnverifiedAndDisabledState(t *testing.T) {
	verified := verifiedRecord()
	unverified := verified
	unverified.EmailVerifiedAt = nil
	disabled := verified
	at := time.Now()
	disabled.DisabledAt = &at
	invalidEpoch := verified
	invalidEpoch.AuthEpoch = 0
	for _, tc := range []struct {
		name   string
		record identityapp.CredentialRecord
		match  bool
	}{{"wrong password", verified, false}, {"unverified", unverified, true}, {"disabled", disabled, true}, {"invalid epoch", invalidEpoch, true}} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &credentialReaderFake{record: tc.record}
			attempts := &attemptFake{}
			verifier := &verifierFake{result: password.Verification{Match: tc.match}}
			a := authenticator(t, reader, attempts, verifier)
			_, err := a.Authenticate(context.Background(), loginInput("user@example.com", []byte("guess")))
			if err == nil || err.Error() != "credentials_invalid" || len(verifier.phcs) != 1 || len(attempts.finishes) != 1 || attempts.finishes[0] {
				t.Fatal("credential state leaked/accepted", err)
			}
			if strings.Contains(fmt.Sprintf("%#v", err), "real-phc") {
				t.Fatal("credential leaked")
			}
		})
	}
}

func TestAuthenticationDependencyAndAttemptFailuresFailClosed(t *testing.T) {
	cause := errors.New("secret-canary dependency diagnostic")
	t.Run("denied", func(t *testing.T) {
		reader := &credentialReaderFake{}
		attempts := &attemptFake{beginErr: identityapp.ErrAttemptDenied}
		verifier := &verifierFake{}
		a := authenticator(t, reader, attempts, verifier)
		_, err := a.Authenticate(context.Background(), loginInput("user@example.com", nil))
		if err == nil || err.Error() != "credentials_invalid" || len(reader.lookups) != 0 || len(verifier.phcs) != 0 || len(attempts.finishes) != 0 || errors.Is(err, identityapp.ErrAttemptDenied) {
			t.Fatal("denied attempt crossed boundary", err)
		}
	})
	for _, tc := range []struct {
		name  string
		setup func(*credentialReaderFake, *attemptFake, *verifierFake)
	}{{"begin unavailable", func(_ *credentialReaderFake, a *attemptFake, _ *verifierFake) { a.beginErr = cause }}, {"nil reservation", func(_ *credentialReaderFake, a *attemptFake, _ *verifierFake) { a.nilReservation = true }}, {"lookup unavailable", func(r *credentialReaderFake, _ *attemptFake, _ *verifierFake) { r.err = cause }}, {"verify unavailable", func(_ *credentialReaderFake, _ *attemptFake, v *verifierFake) { v.err = cause }}, {"finish unavailable", func(_ *credentialReaderFake, a *attemptFake, _ *verifierFake) { a.finishErr = cause }}} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &credentialReaderFake{record: verifiedRecord()}
			attempts := &attemptFake{}
			verifier := &verifierFake{result: password.Verification{Match: true}}
			tc.setup(reader, attempts, verifier)
			a := authenticator(t, reader, attempts, verifier)
			result, err := a.Authenticate(context.Background(), loginInput("user@example.com", []byte("guess")))
			if err == nil || err.Error() != "authentication_unavailable" || result != (identityapp.Authentication{}) || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("dependency failure unsafe", result, err)
			}
			if tc.name != "nil reservation" && !errors.Is(err, cause) {
				t.Fatal("dependency cause lost", err)
			}
		})
	}
	t.Run("missing network scope", func(t *testing.T) {
		a := authenticator(t, &credentialReaderFake{}, &attemptFake{}, &verifierFake{})
		_, err := a.Authenticate(context.Background(), identityapp.AuthenticateInput{Email: "user@example.com"})
		if err == nil || err.Error() != "authentication_context_required" {
			t.Fatal("missing trusted network scope accepted")
		}
	})
}

func TestAuthenticationConfigurationAndRealHasherIntegration(t *testing.T) {
	reader := &credentialReaderFake{}
	attempts := &attemptFake{}
	verifier := &verifierFake{}
	keys := keyer(t)
	for _, build := range []func() (*identityapp.Authenticator, error){func() (*identityapp.Authenticator, error) {
		return identityapp.NewAuthenticator(nil, attempts, keys, verifier, "dummy")
	}, func() (*identityapp.Authenticator, error) {
		return identityapp.NewAuthenticator(reader, nil, keys, verifier, "dummy")
	}, func() (*identityapp.Authenticator, error) {
		return identityapp.NewAuthenticator(reader, attempts, nil, verifier, "dummy")
	}, func() (*identityapp.Authenticator, error) {
		return identityapp.NewAuthenticator(reader, attempts, keys, nil, "dummy")
	}, func() (*identityapp.Authenticator, error) {
		return identityapp.NewAuthenticator(reader, attempts, keys, verifier, "")
	}, func() (*identityapp.Authenticator, error) {
		bad := &verifierFake{validationErr: errors.New("secret-canary invalid dummy")}
		return identityapp.NewAuthenticator(reader, attempts, keys, bad, "dummy")
	}} {
		if a, err := build(); err == nil || err.Error() != "authenticator_invalid" || a != nil {
			t.Fatal("invalid authenticator accepted")
		}
	}
	a := authenticator(t, reader, attempts, verifier)
	if _, err := a.Authenticate(nil, identityapp.AuthenticateInput{}); err == nil || err.Error() != "authentication_context_required" {
		t.Fatal("nil context accepted")
	}
	policy := password.Policy{MemoryKiB: 32, Iterations: 2, Parallelism: 1, SaltBytes: 16, KeyBytes: 32, MaxPasswordBytes: 128}
	hasher, _ := password.NewWithRandom(policy, repeatReader(3))
	realPHC, _ := hasher.Hash([]byte("correct horse"))
	dummyPHC, _ := hasher.Hash([]byte("dummy password"))
	record := verifiedRecord()
	record.PasswordPHC = realPHC
	realReader := &credentialReaderFake{record: record}
	realAttempts := &attemptFake{}
	realAuth, err := identityapp.NewAuthenticator(realReader, realAttempts, keys, hasher, dummyPHC)
	if err != nil {
		t.Fatal(err)
	}
	result, err := realAuth.Authenticate(context.Background(), loginInput("user@example.com", []byte("correct horse")))
	if err != nil || result.UserID != identityID(1) {
		t.Fatal("real password verifier integration failed", result, err)
	}
	if strings.Contains(fmt.Sprintf("%#v", record), realPHC) {
		t.Fatal("credential record formatting leaked PHC")
	}
}
