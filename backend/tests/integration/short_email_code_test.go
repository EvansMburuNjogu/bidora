package integration_test

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
	"github.com/google/uuid"
	"net/http/httptest"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"
)

type shortEmailIP struct{}

func (shortEmailIP) Resolve(netip.AddrPort, string) (netip.Addr, error) {
	return netip.MustParseAddr("198.51.100.9"), nil
}

type shortEmailState struct {
	token verification.Token
	calls int
	used  bool
}

func (s *shortEmailState) Register(ctx context.Context, _ app.OpenRegistrationInput) (app.OpenRegistrationResult, error) {
	var err error
	s.token, err = verification.GenerateForContext(ctx, verification.Generate)
	s.used = false
	return app.OpenRegistrationResult{Accepted: true}, err
}
func (s *shortEmailState) Verify(_ context.Context, input app.EmailVerificationInput) (app.EmailVerificationResult, error) {
	s.calls++
	hash, err := verification.HashPresented(input.Token)
	if err != nil || hash != s.token.Hash() || s.used {
		return app.EmailVerificationResult{}, errors.New("verification_invalid")
	}
	s.used = true
	return app.EmailVerificationResult{UserID: app.ID{1}, AuthEpoch: 1, VerifiedAt: time.Now()}, nil
}
func TestLiveSixDigitCodeAttemptLimitCannotBeBypassed(t *testing.T) {
	fixture := redisIntegration(t)
	base := os.Getenv("BIDORA_TEST_RECOVERY_REDIS_PREFIX")
	if base == "" {
		base = "bidora:test"
	}
	prefix := base + ":six:" + uuid.NewString()[:8]
	controller, _ := liveController(t, fixture, prefix)
	deriver, _ := attempt.NewDeriver(bytes.Repeat([]byte{7}, 32))
	codes, err := httpapi.NewEmailCodes(bytes.Repeat([]byte{8}, 32), controller, deriver, shortEmailIP{})
	if err != nil {
		t.Fatal(err)
	}
	protection, _ := browser.NewProtection([]string{"https://app.example.test"})
	state := &shortEmailState{}
	register, _ := httpapi.NewRegistrationHandler(state, protection, shortEmailIP{}, deriver)
	register.EnableShortCodes(codes)
	verify, _ := httpapi.NewEmailVerificationHandler(state, protection)
	verify.EnableShortCodes(codes)
	server := fiber.New()
	server.Post("/issue", register.Handle)
	server.Post("/verify", verify.Handle)
	send := func(path, body string) (int, string) {
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set("Origin", "https://app.example.test")
		req.Header.Set("Content-Type", "application/json")
		response, e := server.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		return response.StatusCode, response.Header.Get(httpapi.EmailCodeChallengeHeader)
	}
	status, ref := send("/issue", `{"first_name":"Test","last_name":"Invitee","email":"synthetic@example.invalid","password":"synthetic long password"}`)
	if status != 202 {
		t.Fatal("could not issue test code")
	}
	identity, _ := deriver.Identity("email-code:verify:" + ref)
	network, _ := deriver.Network(netip.MustParseAddr("198.51.100.9"))
	keyBase := prefix + ":{login-attempt}"
	keys := []string{fmt.Sprintf("%s:i:fail:%x", keyBase, identity[:]), fmt.Sprintf("%s:i:block:%x", keyBase, identity[:]), fmt.Sprintf("%s:n:minute:%x", keyBase, network[:]), fmt.Sprintf("%s:n:window:%x", keyBase, network[:]), fmt.Sprintf("%s:n:block:%x", keyBase, network[:])}
	defer fixture.direct.Del(context.Background(), keys...)
	wrong := "000000"
	if state.token.RevealForDelivery() == wrong {
		wrong = "999999"
	}
	for range 5 {
		status, _ = send("/verify", fmt.Sprintf(`{"token":%q,"challenge_ref":%q}`, wrong, ref))
		if status != 400 {
			t.Fatal("incorrect code was accepted")
		}
	}
	before := state.calls
	status, _ = send("/verify", fmt.Sprintf(`{"token":%q,"challenge_ref":%q}`, state.token.RevealForDelivery(), ref))
	if status != 400 || state.calls != before || state.used {
		t.Fatal("real Redis attempt limit was bypassed")
	}
}
