package httpapi_test

import (
	"bidora/backend/internal/httpapi"
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/verification"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type shortCodeState struct {
	token       verification.Token
	used        bool
	verifyCalls int
}

func (s *shortCodeState) Register(ctx context.Context, _ app.OpenRegistrationInput) (app.OpenRegistrationResult, error) {
	var err error
	s.token, err = verification.GenerateForContext(ctx, verification.Generate)
	return app.OpenRegistrationResult{Accepted: true}, err
}
func (s *shortCodeState) Request(ctx context.Context, _ app.PublicPasswordRecoveryInput) error {
	var err error
	s.token, err = verification.GenerateForContext(ctx, verification.Generate)
	return err
}
func (s *shortCodeState) Verify(_ context.Context, input app.EmailVerificationInput) (app.EmailVerificationResult, error) {
	s.verifyCalls++
	hash, err := verification.HashPresented(input.Token)
	if err != nil || hash != s.token.Hash() || s.used {
		return app.EmailVerificationResult{}, errors.New("verification_invalid")
	}
	s.used = true
	return app.EmailVerificationResult{UserID: app.ID{1}, AuthEpoch: 1, VerifiedAt: time.Now()}, nil
}
func (s *shortCodeState) Reset(ctx context.Context, input app.PasswordRecoveryResetInput) error {
	_, err := s.Verify(ctx, app.EmailVerificationInput{Token: input.Token})
	if err != nil {
		return errors.New("password_recovery_reset_invalid")
	}
	return nil
}

type shortCodeAttempts struct {
	denied      bool
	unavailable bool
	scopes      []attempt.Scopes
	outcomes    []bool
}

func (a *shortCodeAttempts) Begin(_ context.Context, s attempt.Scopes) (app.AttemptReservation, error) {
	a.scopes = append(a.scopes, s)
	if a.unavailable {
		return nil, errors.New("synthetic redis outage")
	}
	if a.denied {
		return nil, app.ErrAttemptDenied
	}
	return shortCodeReservation{a}, nil
}

type shortCodeReservation struct{ attempts *shortCodeAttempts }

func (r shortCodeReservation) Finish(_ context.Context, success bool) error {
	r.attempts.outcomes = append(r.attempts.outcomes, success)
	return nil
}
func TestSixDigitEmailFlowsKeepScopesAttemptLimitsAndSingleUse(t *testing.T) {
	for _, purpose := range []string{"verify", "recovery"} {
		t.Run(purpose, func(t *testing.T) {
			state := &shortCodeState{}
			limits := &shortCodeAttempts{}
			key := bytes.Repeat([]byte{4}, 32)
			deriver, _ := attempt.NewDeriver(key)
			ips := &loginIPFake{address: netip.MustParseAddr("192.0.2.1")}
			support, _ := httpapi.NewEmailCodes(key, limits, deriver, ips)
			protection, _ := browser.NewProtection([]string{"https://app.example.com"})
			server := fiber.New()
			issuePath, consumePath := "/issue", "/consume"
			if purpose == "verify" {
				h, _ := httpapi.NewRegistrationHandler(state, protection, ips, &registrationNetworkKeyerFake{key: attempt.Key{1}})
				h.EnableShortCodes(support)
				server.Post(issuePath, h.Handle)
				v, _ := httpapi.NewEmailVerificationHandler(state, protection)
				v.EnableShortCodes(support)
				server.Post(consumePath, v.Handle)
			} else {
				h, _ := httpapi.NewPasswordRecoveryHandler(state, state, protection, ips, recoveryKeyerFake{})
				h.EnableShortCodes(support)
				server.Post(issuePath, h.Request)
				server.Post(consumePath, h.Reset)
			}
			send := func(path, body string) (int, string) {
				req := httptest.NewRequest("POST", path, strings.NewReader(body))
				req.Header.Set("Origin", "https://app.example.com")
				req.Header.Set("Content-Type", "application/json")
				response, err := server.Test(req)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				return response.StatusCode, response.Header.Get(httpapi.EmailCodeChallengeHeader)
			}
			body := `{"first_name":"New","last_name":"Person","email":"user@example.test","password":"synthetic password"}`
			if purpose == "recovery" {
				body = `{"email":"user@example.test"}`
			}
			status, ref := send(issuePath, body)
			if status != 202 || len(ref) != 36 || len(state.token.RevealForDelivery()) != 6 {
				t.Fatal("new issuance did not produce scoped six-digit code")
			}
			payload := func(code, reference string) string {
				if purpose == "recovery" {
					return fmt.Sprintf(`{"token":%q,"challenge_ref":%q,"new_password":"synthetic replacement password"}`, code, reference)
				}
				return fmt.Sprintf(`{"token":%q,"challenge_ref":%q}`, code, reference)
			}
			wrong := "000000"
			if state.token.RevealForDelivery() == wrong {
				wrong = "999999"
			}
			if status, _ = send(consumePath, payload(wrong, ref)); status != 400 {
				t.Fatal("wrong code accepted")
			}
			if status, _ = send(consumePath, payload(state.token.RevealForDelivery(), "00000000-0000-4000-8000-000000000002")); status != 400 {
				t.Fatal("cross-challenge code accepted")
			}
			limits.denied = true
			before := state.verifyCalls
			if status, _ = send(consumePath, payload(state.token.RevealForDelivery(), ref)); status != 400 || state.verifyCalls != before {
				t.Fatal("attempt denial bypassed")
			}
			limits.denied = false
			limits.unavailable = true
			if status, _ = send(consumePath, payload(state.token.RevealForDelivery(), ref)); status != 503 || state.verifyCalls != before {
				t.Fatal("dependency failure was not fail-closed")
			}
			limits.unavailable = false
			if status, _ = send(consumePath, payload(state.token.RevealForDelivery(), ref)); status != 204 {
				t.Fatal("correct scoped code rejected")
			}
			if status, _ = send(consumePath, payload(state.token.RevealForDelivery(), ref)); status != 400 {
				t.Fatal("code replay accepted")
			}
			if limits.scopes[0].Identity != limits.scopes[2].Identity {
				t.Fatal("code guesses escaped the challenge attempt scope")
			}
		})
	}
}
