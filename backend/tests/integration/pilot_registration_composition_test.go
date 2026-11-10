package integration_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	"bidora/backend/internal/identity/session"
	"bidora/backend/internal/identity/verification"
	"bidora/backend/internal/pilot"
	"github.com/google/uuid"
)

func TestLivePilotCompositionDeliversRegistrationAndKeepsDuplicatesGeneric(t *testing.T) {
	redisFixture := redisIntegration(t)
	config := recoveryDeploymentConfig(t, redisFixture)
	emailValue := "pilot-delivery-" + uuid.NewString() + "@example.invalid"
	smtpFixture := newPilotSMTPFixture(t, emailValue)
	configureManagedSMTPFixture(t, &config.Runtime, smtpFixture, 0)
	deployment, err := pilot.Open(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Shutdown(context.Background())
	control := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	observer := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	register := func(password string) *http.Response {
		request := httptest.NewRequest(http.MethodPost, httpapi.RegistrationPath, bytes.NewBufferString(`{"first_name":"Amina","last_name":"Njoroge","email":"`+emailValue+`","password":"`+password+`"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "https://app.example.test")
		response, requestErr := deployment.Runtime.App.Test(request)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		return response
	}
	response := register("synthetic registration password")
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted || len(response.Cookies()) != 0 {
		t.Fatal("successful composed registration was not generic", response.StatusCode, response.Cookies())
	}
	var deliveredToken string
	select {
	case message := <-smtpFixture.message:
		if !strings.Contains(message, "To: "+emailValue) || !strings.Contains(message, "Subject: Verify your Bidora email address") || strings.Contains(strings.ToLower(message), "bcc:") {
			t.Fatal("composed verification message drifted")
		}
		for _, field := range strings.Fields(message) {
			if _, hashErr := verification.HashPresented(field); hashErr == nil {
				deliveredToken = field
				break
			}
		}
		if deliveredToken == "" {
			t.Fatal("verification token missing from composed message")
		}
	case <-time.After(time.Second):
		t.Fatal("composed verification message missing")
	}
	login := func(password string) *http.Response {
		request := httptest.NewRequest(http.MethodPost, httpapi.LoginPath, bytes.NewBufferString(`{"email":"`+emailValue+`","password":"`+password+`"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "https://app.example.test")
		loginResponse, requestErr := deployment.Runtime.App.Test(request)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		return loginResponse
	}
	unverifiedLogin := login("synthetic registration password")
	unverifiedLogin.Body.Close()
	if unverifiedLogin.StatusCode != http.StatusUnauthorized || len(unverifiedLogin.Cookies()) != 0 {
		t.Fatal("unverified identity authenticated", unverifiedLogin.StatusCode, unverifiedLogin.Cookies())
	}
	wrongPasswordLogin := login("different synthetic password")
	wrongPasswordLogin.Body.Close()
	if wrongPasswordLogin.StatusCode != unverifiedLogin.StatusCode || len(wrongPasswordLogin.Cookies()) != 0 {
		t.Fatal("unverified state was distinguishable from wrong credentials", wrongPasswordLogin.StatusCode, wrongPasswordLogin.Cookies())
	}
	if count := rowCount(t, control, `SELECT count(*) FROM identity.sessions AS s JOIN identity.users AS u ON u.id=s.user_id WHERE u.email_lookup=$1`, emailValue); count != 0 {
		t.Fatal("unverified login persisted a session", count)
	}
	verify := func() *http.Response {
		request := httptest.NewRequest(http.MethodPost, httpapi.EmailVerificationPath, bytes.NewBufferString(`{"token":"`+deliveredToken+`"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "https://app.example.test")
		verificationResponse, requestErr := deployment.Runtime.App.Test(request)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		return verificationResponse
	}
	verified := verify()
	verified.Body.Close()
	if verified.StatusCode != http.StatusNoContent || len(verified.Cookies()) != 0 {
		t.Fatal("composed email verification failed", verified.StatusCode, verified.Cookies())
	}
	replayed := verify()
	replayed.Body.Close()
	if replayed.StatusCode != http.StatusBadRequest || len(replayed.Cookies()) != 0 {
		t.Fatal("verification replay was not rejected generically", replayed.StatusCode, replayed.Cookies())
	}
	duplicate := register("different synthetic password")
	duplicate.Body.Close()
	if duplicate.StatusCode != http.StatusAccepted || len(duplicate.Cookies()) != 0 {
		t.Fatal("duplicate registration disclosed identity", duplicate.StatusCode, duplicate.Cookies())
	}
	if count := rowCount(t, control, `SELECT count(*) FROM identity.users AS u JOIN identity.authentication_identities AS a ON a.user_id=u.id JOIN identity.challenges AS c ON c.user_id=u.id WHERE u.email_lookup=$1 AND u.email_verified_at IS NOT NULL AND a.credential_version=1 AND c.purpose='EMAIL_VERIFY' AND c.consumed_at IS NOT NULL AND c.invalidated_at IS NULL`, emailValue); count != 1 {
		t.Fatal("successful composed registration persistence drifted", count)
	}
	if count := rowCount(t, observer, `SELECT count(*) FROM bidora_control_audit.audit_events AS e JOIN identity.users AS u ON u.id=e.target_id WHERE u.email_lookup=$1 AND e.action IN ('UserSelfRegistered','EmailVerificationIssued','EmailVerified')`, emailValue); count != 3 {
		t.Fatal("duplicate registration appended success evidence", count)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM tenancy.memberships AS m JOIN identity.users AS u ON u.id=m.user_id WHERE u.email_lookup=$1`, emailValue); count != 0 {
		t.Fatal("successful registration granted tenant authority", count)
	}
	loginResponse := login("synthetic registration password")
	loginResponse.Body.Close()
	if loginResponse.StatusCode != http.StatusNoContent {
		t.Fatal("verified registration could not log in", loginResponse.StatusCode)
	}
	var sessionCookie, csrfCookie *http.Cookie
	for _, cookie := range loginResponse.Cookies() {
		switch cookie.Name {
		case session.CookieName:
			sessionCookie = cookie
		case session.CSRFCookieName:
			csrfCookie = cookie
		}
	}
	if sessionCookie == nil || csrfCookie == nil || !sessionCookie.Secure || !sessionCookie.HttpOnly || !csrfCookie.Secure || csrfCookie.HttpOnly {
		t.Fatal("verified registration login cookies missing or unsafe")
	}
	bootstrapRequest := httptest.NewRequest(http.MethodGet, httpapi.SessionBootstrapPath, nil)
	bootstrapRequest.Header.Set("Origin", "https://app.example.test")
	bootstrapRequest.AddCookie(sessionCookie)
	bootstrapRequest.AddCookie(csrfCookie)
	bootstrapResponse, err := deployment.Runtime.App.Test(bootstrapRequest)
	if err != nil {
		t.Fatal(err)
	}
	bootstrapResponse.Body.Close()
	if bootstrapResponse.StatusCode != http.StatusOK {
		t.Fatal("verified registration session bootstrap failed", bootstrapResponse.StatusCode)
	}
	captureTitle := "zero-authority-" + uuid.NewString()
	captureRequest := httptest.NewRequest(http.MethodPost, httpapi.ManualCapturePath, bytes.NewBufferString(`{"title":"`+captureTitle+`","buyer_name":null}`))
	captureRequest.Header.Set("Content-Type", "application/json")
	captureRequest.Header.Set("Origin", "https://app.example.test")
	captureRequest.Header.Set(session.CSRFHeader, csrfCookie.Value)
	captureRequest.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	captureRequest.Header.Set(httpapi.ContextGenerationHeader, "1")
	captureRequest.Header.Set(httpapi.IdempotencyKeyHeader, uuid.NewString())
	captureRequest.AddCookie(sessionCookie)
	captureRequest.AddCookie(csrfCookie)
	captureResponse, err := deployment.Runtime.App.Test(captureRequest)
	if err != nil {
		t.Fatal(err)
	}
	captureResponse.Body.Close()
	if captureResponse.StatusCode != http.StatusForbidden {
		t.Fatal("zero-authority registration accessed tenant capture", captureResponse.StatusCode)
	}
	tenant := livePool(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN")
	if count := rowCount(t, tenant, `SELECT count(*) FROM bidora_opportunity.source_versions WHERE title=$1`, captureTitle); count != 0 {
		t.Fatal("denied zero-authority capture persisted tenant data", count)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM identity.users AS u JOIN identity.sessions AS s ON s.user_id=u.id JOIN bidora_control_audit.audit_events AS e ON e.target_id=s.id AND e.actor_id=u.id WHERE u.email_lookup=$1 AND u.email_verified_at IS NOT NULL AND s.revoked_at IS NULL AND e.action='UserLoggedIn' AND e.change_summary->>'authentication_method'='PASSWORD'`, emailValue); count != 1 {
		t.Fatal("verified registration login persistence drifted", count)
	}
	sessionHash, err := session.HashPresentedSession(sessionCookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	logoutRequest := httptest.NewRequest(http.MethodPost, httpapi.LogoutPath, nil)
	logoutRequest.Header.Set("Origin", "https://app.example.test")
	logoutRequest.Header.Set(session.CSRFHeader, csrfCookie.Value)
	logoutRequest.AddCookie(sessionCookie)
	logoutRequest.AddCookie(csrfCookie)
	logoutResponse, err := deployment.Runtime.App.Test(logoutRequest)
	if err != nil {
		t.Fatal(err)
	}
	logoutResponse.Body.Close()
	if logoutResponse.StatusCode != http.StatusNoContent {
		t.Fatal("onboarded session logout failed", logoutResponse.StatusCode)
	}
	cleared := map[string]bool{}
	for _, cookie := range logoutResponse.Cookies() {
		if (cookie.Name == session.CookieName || cookie.Name == session.CSRFCookieName) && cookie.MaxAge < 0 && cookie.Secure {
			cleared[cookie.Name] = true
		}
	}
	if !cleared[session.CookieName] || !cleared[session.CSRFCookieName] {
		t.Fatal("logout did not clear both secure browser cookies")
	}
	staleBootstrap := httptest.NewRequest(http.MethodGet, httpapi.SessionBootstrapPath, nil)
	staleBootstrap.Header.Set("Origin", "https://app.example.test")
	staleBootstrap.AddCookie(sessionCookie)
	staleBootstrap.AddCookie(csrfCookie)
	staleResponse, err := deployment.Runtime.App.Test(staleBootstrap)
	if err != nil {
		t.Fatal(err)
	}
	staleResponse.Body.Close()
	if staleResponse.StatusCode != http.StatusUnauthorized {
		t.Fatal("logged-out onboarding session remained valid", staleResponse.StatusCode)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM identity.sessions AS s JOIN bidora_control_audit.audit_events AS e ON e.target_id=s.id WHERE s.token_hash=$1 AND s.revoked_at IS NOT NULL AND e.action='UserLoggedOut'`, sessionHash[:]); count != 1 {
		t.Fatal("onboarded session logout persistence drifted", count)
	}
}

func TestLivePilotCompositionRegistersPasswordIdentityAndChallengeBeforeSMTPFailure(t *testing.T) {
	redisFixture := redisIntegration(t)
	config := recoveryDeploymentConfig(t, redisFixture)
	smtpFixture := newPilotSMTPFixture(t, "unused@example.invalid")
	configureManagedSMTPFixture(t, &config.Runtime, smtpFixture, 1)
	deployment, err := pilot.Open(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Shutdown(context.Background())
	control := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	emailValue := "pilot-register-" + uuid.NewString() + "@example.invalid"
	request := httptest.NewRequest(http.MethodPost, httpapi.RegistrationPath, bytes.NewBufferString(`{"first_name":"Amina","last_name":"Njoroge","email":"`+emailValue+`","password":"synthetic registration password"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://app.example.test")
	response, err := deployment.Runtime.App.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted || len(response.Cookies()) != 0 {
		t.Fatal("composed registration did not return generic acceptance", response.StatusCode, response.Cookies())
	}
	if count := rowCount(t, control, `SELECT count(*) FROM identity.users AS u JOIN identity.authentication_identities AS a ON a.user_id=u.id JOIN identity.challenges AS c ON c.user_id=u.id WHERE u.email_lookup=$1 AND u.email_verified_at IS NULL AND u.disabled_at IS NULL AND a.credential_version=1 AND c.purpose='EMAIL_VERIFY' AND c.consumed_at IS NULL AND c.invalidated_at IS NULL`, emailValue); count != 1 {
		t.Fatal("composed registration persistence missing", count)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM tenancy.memberships AS m JOIN identity.users AS u ON u.id=m.user_id WHERE u.email_lookup=$1`, emailValue); count != 0 {
		t.Fatal("composed registration granted tenant authority", count)
	}
}
