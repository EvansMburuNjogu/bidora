package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/health"
	"bidora/backend/internal/httpapi"
	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/mfa"
	"bidora/backend/internal/identity/password"
	"bidora/backend/internal/identity/session"
	"bidora/backend/internal/identity/verification"
	"bidora/backend/internal/pilot"
	"github.com/google/uuid"
)

func TestLiveIdentityCompositionRegistrationVerificationAndVerifiedLogin(t *testing.T) {
	redisFixture := redisIntegration(t)
	control := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	observer := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	hasher, err := password.New(password.PilotPolicy())
	if err != nil {
		t.Fatal(err)
	}
	dummy, err := hasher.Hash([]byte("synthetic identity composition dummy"))
	if err != nil {
		t.Fatal(err)
	}
	hmacSecret := sha256.Sum256([]byte(uuid.NewString()))
	mfaKey := sha256.Sum256([]byte("synthetic-mfa-" + uuid.NewString()))
	keyRing, err := mfa.NewKeyRing(1, map[int][32]byte{1: mfaKey})
	if err != nil {
		t.Fatal(err)
	}
	emailValue := "identity-composition-" + uuid.NewString() + "@example.invalid"
	smtpFixture := newPilotSMTPFixtureMessages(t, emailValue, 2)
	sender, err := email.Parse("noreply@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	state := &health.State{}
	redisPrefix := "bidora:test:identity:" + uuid.NewString()
	config := pilot.Config{
		PasswordPolicy: password.PilotPolicy(), DummyPasswordPHC: dummy,
		AttemptHMACSecret: hmacSecret[:], Redis: redisFixture.config,
		RedisPrefix:    redisPrefix,
		AllowedOrigins: []string{"https://app.example.test"},
		TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}, MFAKeyRing: keyRing,
		Registration: &pilot.RegistrationConfig{SMTP: deliverysmtp.Config{Host: "127.0.0.1", Port: uint16(smtpFixture.listener.Addr().(*net.TCPAddr).Port), Username: "smtp-user", Password: "smtp-password", Sender: sender, TLS: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "localhost", RootCAs: smtpFixture.roots}, Timeout: time.Second}},
	}
	runtime, err := pilot.BuildIdentityRuntime(config, state, control)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	state.Start()
	post := func(path, body string) *http.Response {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://app.example.test")
		res, requestErr := runtime.App.Test(req)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		return res
	}
	registered := post(httpapi.RegistrationPath, `{"first_name":"Amina","last_name":"Njoroge","email":"`+emailValue+`","password":"synthetic registration password"}`)
	registered.Body.Close()
	if registered.StatusCode != http.StatusAccepted || len(registered.Cookies()) != 0 {
		t.Fatal("registration was not generic", registered.StatusCode)
	}
	var token string
	select {
	case message := <-smtpFixture.message:
		for _, field := range strings.Fields(message) {
			if _, hashErr := verification.HashPresented(field); hashErr == nil {
				token = field
				break
			}
		}
	case <-time.After(time.Second):
		t.Fatal("verification message missing")
	}
	if token == "" {
		t.Fatal("verification token missing")
	}
	tokenHash, err := verification.HashPresented(token)
	if err != nil {
		t.Fatal(err)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM identity.users u JOIN identity.authentication_identities a ON a.user_id=u.id JOIN identity.challenges c ON c.user_id=u.id WHERE u.email_lookup=$1 AND u.first_name='Amina' AND u.last_name='Njoroge' AND u.email_verified_at IS NULL AND a.credential_version=1 AND c.purpose='EMAIL_VERIFY' AND c.token_hash=$2 AND c.consumed_at IS NULL AND c.invalidated_at IS NULL`, emailValue, tokenHash[:]); count != 1 {
		t.Fatal("name or hashed challenge persistence drifted", count)
	}
	resent := post(httpapi.EmailVerificationResendPath, `{"email":"`+emailValue+`"}`)
	resent.Body.Close()
	if resent.StatusCode != http.StatusAccepted || len(resent.Cookies()) != 0 {
		t.Fatal("verification resend was not generic", resent.StatusCode)
	}
	var replacement string
	select {
	case message := <-smtpFixture.message:
		for _, field := range strings.Fields(message) {
			if _, hashErr := verification.HashPresented(field); hashErr == nil {
				replacement = field
				break
			}
		}
	case <-time.After(time.Second):
		t.Fatal("replacement verification message missing")
	}
	if replacement == "" || replacement == token {
		t.Fatal("replacement verification token missing or reused")
	}
	replacementHash, err := verification.HashPresented(replacement)
	if err != nil {
		t.Fatal(err)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM identity.challenges c JOIN identity.users u ON u.id=c.user_id WHERE u.email_lookup=$1 AND c.purpose='EMAIL_VERIFY' AND ((c.token_hash=$2 AND c.invalidated_at IS NOT NULL AND c.consumed_at IS NULL) OR (c.token_hash=$3 AND c.invalidated_at IS NULL AND c.consumed_at IS NULL))`, emailValue, tokenHash[:], replacementHash[:]); count != 2 {
		t.Fatal("replacement challenge invalidation drifted", count)
	}
	oldToken := post(httpapi.EmailVerificationPath, `{"token":"`+token+`"}`)
	oldToken.Body.Close()
	if oldToken.StatusCode != http.StatusBadRequest {
		t.Fatal("invalidated verification token accepted", oldToken.StatusCode)
	}
	token = replacement
	unverified := post(httpapi.LoginPath, `{"email":"`+emailValue+`","password":"synthetic registration password"}`)
	unverified.Body.Close()
	if unverified.StatusCode != http.StatusUnauthorized || len(unverified.Cookies()) != 0 {
		t.Fatal("unverified identity authenticated", unverified.StatusCode)
	}
	verified := post(httpapi.EmailVerificationPath, `{"token":"`+token+`"}`)
	verified.Body.Close()
	if verified.StatusCode != http.StatusNoContent {
		t.Fatal("verification failed", verified.StatusCode)
	}
	replay := post(httpapi.EmailVerificationPath, `{"token":"`+token+`"}`)
	replay.Body.Close()
	if replay.StatusCode != http.StatusBadRequest {
		t.Fatal("verification replay accepted", replay.StatusCode)
	}
	unknownEmail := "unknown-" + uuid.NewString() + "@example.invalid"
	for attemptNumber := 1; attemptNumber <= 3; attemptNumber++ {
		unknownResend := post(httpapi.EmailVerificationResendPath, `{"email":"`+unknownEmail+`"}`)
		unknownResend.Body.Close()
		if unknownResend.StatusCode != http.StatusAccepted {
			t.Fatal("unknown verification target was disclosed", attemptNumber, unknownResend.StatusCode)
		}
	}
	limitedResend := post(httpapi.EmailVerificationResendPath, `{"email":"`+unknownEmail+`"}`)
	limitedResend.Body.Close()
	if limitedResend.StatusCode != http.StatusTooManyRequests || limitedResend.Header.Get("Retry-After") != "900" {
		t.Fatal("verification resend limit did not fail closed", limitedResend.StatusCode)
	}
	duplicate := post(httpapi.RegistrationPath, `{"first_name":"Changed","last_name":"Name","email":"`+emailValue+`","password":"different synthetic password"}`)
	duplicate.Body.Close()
	if duplicate.StatusCode != http.StatusAccepted || len(duplicate.Cookies()) != 0 {
		t.Fatal("duplicate disclosed identity", duplicate.StatusCode)
	}
	if count := rowCount(t, observer, `SELECT count(*) FROM identity.users WHERE email_lookup=$1 AND first_name='Amina' AND last_name='Njoroge'`, emailValue); count != 1 {
		t.Fatal("duplicate registration changed personal names", count)
	}
	loggedIn := post(httpapi.LoginPath, `{"email":"`+emailValue+`","password":"synthetic registration password"}`)
	if loggedIn.StatusCode != http.StatusNoContent {
		var failure struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.NewDecoder(loggedIn.Body).Decode(&failure)
		loggedIn.Body.Close()
		t.Fatal("verified identity could not log in", loggedIn.StatusCode, failure.Error.Code)
	}
	loggedIn.Body.Close()
	cookies := map[string]*http.Cookie{}
	for _, c := range loggedIn.Cookies() {
		cookies[c.Name] = c
	}
	if cookies[session.CookieName] == nil || cookies[session.CSRFCookieName] == nil {
		t.Fatal("verified login cookies missing")
	}
	authenticated := func(method, path, body string, unsafe bool) *http.Response {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Origin", "https://app.example.test")
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.AddCookie(cookies[session.CookieName])
		req.AddCookie(cookies[session.CSRFCookieName])
		if unsafe {
			req.Header.Set(session.CSRFHeader, cookies[session.CSRFCookieName].Value)
			req.Header.Set(httpapi.IdempotencyKeyHeader, uuid.NewString())
		}
		res, requestErr := runtime.App.Test(req)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		return res
	}
	var beforeSelection struct {
		Data struct {
			Portal          json.RawMessage `json:"portal"`
			RequiredActions []string        `json:"required_actions"`
		} `json:"data"`
	}
	before := authenticated(http.MethodGet, httpapi.MePortalPath, "", false)
	if before.StatusCode != http.StatusOK || json.NewDecoder(before.Body).Decode(&beforeSelection) != nil {
		before.Body.Close()
		t.Fatal("unassigned account resolution failed", before.StatusCode)
	}
	before.Body.Close()
	if len(beforeSelection.Data.RequiredActions) != 1 || beforeSelection.Data.RequiredActions[0] != "SELECT_ACCOUNT_TYPE" || string(beforeSelection.Data.Portal) != "null" {
		t.Fatal("unassigned account received authority")
	}
	selected := authenticated(http.MethodPost, httpapi.AccountTypeSelectionPath, `{"account_type":"TENDERPRENEUR"}`, true)
	selected.Body.Close()
	if selected.StatusCode != http.StatusNoContent {
		t.Fatal("Tenderpreneur account selection failed", selected.StatusCode)
	}
	var afterSelection struct {
		Data struct {
			AccountType     string          `json:"account_type"`
			Portal          json.RawMessage `json:"portal"`
			RequiredActions []string        `json:"required_actions"`
		} `json:"data"`
	}
	after := authenticated(http.MethodGet, httpapi.MePortalPath, "", false)
	if after.StatusCode != http.StatusOK || json.NewDecoder(after.Body).Decode(&afterSelection) != nil {
		after.Body.Close()
		t.Fatal("Tenderpreneur account resolution failed", after.StatusCode)
	}
	after.Body.Close()
	if afterSelection.Data.AccountType != "TENDERPRENEUR" || len(afterSelection.Data.RequiredActions) != 1 || afterSelection.Data.RequiredActions[0] != "ESTABLISH_TENDERPRENEUR_MEMBERSHIP" || string(afterSelection.Data.Portal) != "null" {
		t.Fatal("Tenderpreneur account received portal authority without membership")
	}
	var membershipResult struct {
		Data struct {
			Memberships []json.RawMessage `json:"memberships"`
		} `json:"data"`
	}
	memberships := authenticated(http.MethodGet, httpapi.MembershipDirectoryPath, "", false)
	if memberships.StatusCode != http.StatusOK || json.NewDecoder(memberships.Body).Decode(&membershipResult) != nil {
		memberships.Body.Close()
		t.Fatal("membership resolution failed", memberships.StatusCode)
	}
	memberships.Body.Close()
	if len(membershipResult.Data.Memberships) != 0 {
		t.Fatal("synthetic account unexpectedly received membership")
	}
	if count := rowCount(t, observer, `SELECT count(*) FROM identity.challenges c JOIN identity.users u ON u.id=c.user_id WHERE u.email_lookup=$1 AND c.purpose='EMAIL_VERIFY' AND c.consumed_at IS NOT NULL AND c.invalidated_at IS NULL`, emailValue); count != 1 {
		t.Fatal("atomic consumption drifted", count)
	}
	if count := rowCount(t, observer, `SELECT count(*) FROM identity.sessions s JOIN identity.users u ON u.id=s.user_id JOIN bidora_control_audit.audit_events a ON a.target_id=s.id AND a.actor_id=u.id AND a.action='UserLoggedIn' WHERE u.email_lookup=$1 AND s.revoked_at IS NULL`, emailValue); count != 1 {
		t.Fatal("verified login session/audit drifted", count)
	}
	keys, err := redisFixture.direct.Keys(context.Background(), redisPrefix+":{login-attempt}:*").Result()
	if err != nil {
		t.Fatal("limiter inspection unavailable")
	}
	for _, key := range keys {
		if strings.Contains(key, ":i:fail:") || strings.Contains(key, ":i:block:") || strings.Contains(key, ":n:block:") || strings.Contains(key, ":r:") {
			t.Fatal("successful verified login retained failure, denial, or reservation state")
		}
	}
}
