package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/mfa"
	"bidora/backend/internal/identity/password"
	"bidora/backend/internal/identity/session"
	opportunityapp "bidora/backend/internal/opportunity/application"
	"bidora/backend/internal/opportunity/extractopenai"
	"bidora/backend/internal/opportunity/scanclamav"
	"bidora/backend/internal/opportunity/storageminio"
	"bidora/backend/internal/pilot"
	"bidora/backend/internal/postgres"
	tenancyapp "bidora/backend/internal/tenancy/application"
	"github.com/google/uuid"
)

func responseBody(t *testing.T, response *http.Response) []byte {
	t.Helper()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
func addBrowserCookies(request *http.Request, sessionCookie, csrfCookie *http.Cookie) {
	request.AddCookie(sessionCookie)
	request.AddCookie(csrfCookie)
}

func TestLiveHTTPLoginCaptureLogoutFlow(t *testing.T) {
	controlMigrator := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	controlRuntime := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	tenantRuntime := livePool(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN")
	redisFixture := redisIntegration(t)
	redisPrefix := "bidora:test:" + uuid.NewString()
	defer cleanupPrefix(t, redisFixture, redisPrefix)
	hasher, err := password.New(password.PilotPolicy())
	if err != nil {
		t.Fatal(err)
	}
	passwordValue := []byte("synthetic browser password")
	phc, err := hasher.Hash(passwordValue)
	if err != nil {
		t.Fatal(err)
	}
	dummyPHC, err := hasher.Hash([]byte("synthetic browser dummy"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := identityapp.ID(uuid.New())
	tenantID := opportunityapp.ID(uuid.New())
	membershipID := uuid.New()
	grantID := uuid.New()
	clusterID := uuid.New()
	assignmentID := uuid.New()
	endpointRef := "local/test-postgres/" + uuid.NewString()
	emailValue := "browser-" + uuid.NewString() + "@example.invalid"
	err = controlMigrator.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		if _, err := db.Exec(ctx, `INSERT INTO identity.users (id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES ($1,$2,$3,$4,NULL,1,$5,$5)`, uuid.UUID(userID), emailValue, emailValue, now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO identity.authentication_identities (user_id,password_phc,credential_version,password_changed_at,created_at,updated_at) VALUES ($1,$2,1,$3,$3,$3)`, uuid.UUID(userID), phc, now.Add(-time.Minute)); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO bidora_tenancy.tenants (id,tenant_type,state,name,country,timezone,locale,assignment_generation,version,created_at,updated_at) VALUES ($1,'TENDERPRENEUR','ACTIVE','Synthetic Browser Tenant','KE','Africa/Nairobi','en-KE',1,1,$2,$2)`, uuid.UUID(tenantID), now); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO bidora_tenancy.database_clusters (id,region,endpoint_ref,state,capacity_policy,version,created_at,updated_at) VALUES ($1,'local-test',$2,'ACTIVE','{}'::jsonb,1,$3,$3)`, clusterID, endpointRef, now); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO bidora_tenancy.tenant_database_assignments (id,tenant_id,cluster_id,database_name,secret_ref,generation,active,created_at,deactivated_at) VALUES ($1,$2,$3,'bidora_tenant_a_test','test/tenant-a',1,true,$4,NULL)`, assignmentID, uuid.UUID(tenantID), clusterID, now); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO bidora_tenancy.tenant_schema_versions (tenant_id,migration_set,current_version,target_version,state,checksum,generation,updated_at) VALUES ($1,'tenant_common',9,NULL,'COMPATIBLE',repeat('0',64),1,$2),($1,'tenant_tenderpreneur',34,NULL,'COMPATIBLE',repeat('1',64),1,$2)`, uuid.UUID(tenantID), now); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO tenancy.memberships (id,user_id,tenant_id,state,context_generation,created_at,updated_at) VALUES ($1,$2,$3,'ACTIVE',1,$4,$4)`, membershipID, uuid.UUID(userID), uuid.UUID(tenantID), now); err != nil {
			return err
		}
		_, err := db.Exec(ctx, `INSERT INTO tenancy.role_grants (id,membership_id,role_key,capability,effect,created_by,created_at,revoked_at) VALUES ($1,$2,'pilot-member','tenderpreneur.opportunity.capture_external','ALLOW',$3,$4,NULL)`, grantID, membershipID, uuid.UUID(userID), now)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	secret := sha256.Sum256([]byte(uuid.NewString()))
	mfaKey := sha256.Sum256([]byte("core-acceptance-mfa-" + uuid.NewString()))
	mfaKeyRing, err := mfa.NewKeyRing(1, map[int][32]byte{1: mfaKey})
	if err != nil {
		t.Fatal(err)
	}
	minioFixture := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Query().Has("location"):
			w.Header().Set("Content-Type", "application/xml")
			_, _ = io.WriteString(w, `<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">us-east-1</LocationConstraint>`)
		case request.Method == http.MethodHead:
			w.WriteHeader(http.StatusOK)
		case request.URL.Query().Has("versioning"):
			w.Header().Set("Content-Type", "application/xml")
			_, _ = io.WriteString(w, `<VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Status>Enabled</Status></VersioningConfiguration>`)
		case request.URL.Query().Has("policy"):
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `<Error><Code>NoSuchBucketPolicy</Code><Message>synthetic private bucket</Message></Error>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer minioFixture.Close()
	minioURL, err := url.Parse(minioFixture.URL)
	if err != nil {
		t.Fatal(err)
	}
	providerRoots := x509.NewCertPool()
	providerRoots.AddCert(minioFixture.Certificate())
	assignment := tenancyapp.Assignment{TenantID: tenancyapp.ID(tenantID), ClusterID: tenancyapp.ID(clusterID), Generation: 1, DatabaseName: "bidora_tenant_a_test", SecretRef: "test/tenant-a", EndpointRef: endpointRef, Region: "local-test"}
	deploymentConfig := pilot.DeploymentConfig{Environment: "test", Address: "127.0.0.1:0", Runtime: pilot.Config{PasswordPolicy: password.PilotPolicy(), DummyPasswordPHC: dummyPHC, AttemptHMACSecret: secret[:], Redis: redisFixture.config, RedisPrefix: redisPrefix, AllowedOrigins: []string{"https://app.example.test"}, TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, MFAKeyRing: mfaKeyRing, MinIO: &storageminio.Config{Endpoint: minioURL.Host, Bucket: "core-acceptance", AccessKey: "synthetic-access", SecretKey: "synthetic-secret", RootCAs: providerRoots, ServerName: minioURL.Hostname(), Timeout: time.Second}, OpenAI: &extractopenai.Config{APIKey: "synthetic-not-contacted", Model: "synthetic-not-contacted", RootCAs: providerRoots, Timeout: 10 * time.Second}, ClamAV: &scanclamav.Config{SocketPath: "/tmp/bidora-core-acceptance-clamav.sock", Timeout: time.Second, ChunkBytes: 4096}}, Control: livePostgresConfig(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN"), Tenants: []pilot.TenantDatabaseConfig{{Assignment: assignment, Postgres: livePostgresConfig(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN")}}}
	openCtx, openCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer openCancel()
	deployment, err := pilot.Open(openCtx, deploymentConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := deployment.Shutdown(shutdownCtx); err != nil {
			t.Error(err)
		}
	})
	if !deployment.State.Ready() {
		t.Fatal("pilot deployment did not become ready after successful probes")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serveCtx, stopServing := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- pilot.Serve(serveCtx, deployment, listener) }()
	t.Cleanup(func() {
		stopServing()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
			if deployment.State.Ready() {
				t.Error("pilot remained ready after shutdown")
			}
		case <-time.After(12 * time.Second):
			t.Error("integration listener did not stop")
		}
	})
	baseURL := "http://" + listener.Addr().String()
	client := &http.Client{Timeout: 5 * time.Second}
	loginBody, _ := json.Marshal(map[string]string{"email": emailValue, "password": string(passwordValue)})
	request, err := http.NewRequest(http.MethodPost, baseURL+httpapi.LoginPath, bytes.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://app.example.test")
	request.Header.Set("X-Forwarded-For", "203.0.113.200")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != 204 {
		t.Fatal("live HTTP login failed", response.StatusCode)
	}
	var sessionCookie, csrfCookie *http.Cookie
	for _, cookie := range response.Cookies() {
		switch cookie.Name {
		case session.CookieName:
			sessionCookie = cookie
		case session.CSRFCookieName:
			csrfCookie = cookie
		}
	}
	if sessionCookie == nil || csrfCookie == nil || !sessionCookie.Secure || !sessionCookie.HttpOnly || csrfCookie.HttpOnly {
		t.Fatal("live HTTP login cookies invalid")
	}
	bootstrapRequest, err := http.NewRequest(http.MethodGet, baseURL+httpapi.SessionBootstrapPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	bootstrapRequest.Header.Set("Origin", "https://app.example.test")
	bootstrapRequest.AddCookie(sessionCookie)
	bootstrapRequest.AddCookie(csrfCookie)
	response, err = client.Do(bootstrapRequest)
	if err != nil {
		t.Fatal(err)
	}
	bootstrapBody := responseBody(t, response)
	if response.StatusCode != 200 {
		t.Fatal("live session bootstrap failed", response.StatusCode)
	}
	var bootstrap struct {
		Data struct {
			CSRFToken string `json:"csrf_token"`
		} `json:"data"`
	}
	if err = json.Unmarshal(bootstrapBody, &bootstrap); err != nil || bootstrap.Data.CSRFToken != csrfCookie.Value {
		t.Fatal("live bootstrap token drifted", err)
	}
	sessionHash, err := session.HashPresentedSession(sessionCookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	var sessionID uuid.UUID
	var beforeRejected time.Time
	if err = controlRuntime.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		return db.QueryRow(ctx, `SELECT id,last_seen_at FROM identity.sessions WHERE token_hash=$1`, sessionHash[:]).Scan(&sessionID, &beforeRejected)
	}); err != nil {
		t.Fatal(err)
	}
	captureURL := baseURL + httpapi.ManualCapturePath
	badCSRF, err := http.NewRequest(http.MethodPost, captureURL, bytes.NewBufferString(`{"title":"Rejected","buyer_name":null}`))
	if err != nil {
		t.Fatal(err)
	}
	badCSRF.Header.Set("Content-Type", "application/json")
	badCSRF.Header.Set("Origin", "https://app.example.test")
	badCSRF.Header.Set(session.CSRFHeader, "invalid")
	badCSRF.Header.Set(httpapi.TenantIDHeader, uuid.UUID(tenantID).String())
	badCSRF.Header.Set(httpapi.ContextGenerationHeader, "2")
	badCSRF.Header.Set(httpapi.IdempotencyKeyHeader, uuid.NewString())
	addBrowserCookies(badCSRF, sessionCookie, csrfCookie)
	response, err = client.Do(badCSRF)
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != 403 {
		t.Fatal("invalid CSRF accepted", response.StatusCode)
	}
	var afterRejected time.Time
	if err = controlRuntime.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		return db.QueryRow(ctx, `SELECT last_seen_at FROM identity.sessions WHERE id=$1`, sessionID).Scan(&afterRejected)
	}); err != nil || !afterRejected.Equal(beforeRejected) {
		t.Fatal("rejected CSRF touched session", beforeRejected, afterRejected, err)
	}
	idempotencyKey := uuid.NewString()
	captureBody := []byte(`{"title":"Synthetic HTTP tender","buyer_name":null}`)
	captureRequest := func(tenant, generation, key string) *http.Request {
		req, err := http.NewRequest(http.MethodPost, captureURL, bytes.NewReader(captureBody))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://app.example.test")
		req.Header.Set(session.CSRFHeader, csrfCookie.Value)
		req.Header.Set(httpapi.TenantIDHeader, tenant)
		req.Header.Set(httpapi.ContextGenerationHeader, generation)
		req.Header.Set(httpapi.IdempotencyKeyHeader, key)
		addBrowserCookies(req, sessionCookie, csrfCookie)
		return req
	}
	response, err = client.Do(captureRequest(uuid.NewString(), "2", idempotencyKey))
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != 403 {
		t.Fatal("foreign tenant selector authorized", response.StatusCode)
	}
	response, err = client.Do(captureRequest(uuid.UUID(tenantID).String(), "2", idempotencyKey))
	if err != nil {
		t.Fatal(err)
	}
	firstBody := responseBody(t, response)
	if response.StatusCode != 201 {
		t.Fatal("live HTTP capture failed", response.StatusCode, string(firstBody))
	}
	var first struct {
		Data struct {
			ID              string `json:"id"`
			SourceVersionID string `json:"source_version_id"`
		} `json:"data"`
	}
	if err = json.Unmarshal(firstBody, &first); err != nil || first.Data.ID == "" {
		t.Fatal("live HTTP capture response invalid", err)
	}
	response, err = client.Do(captureRequest(uuid.UUID(tenantID).String(), "2", idempotencyKey))
	if err != nil {
		t.Fatal(err)
	}
	replayBody := responseBody(t, response)
	if response.StatusCode != 201 {
		t.Fatal("live HTTP replay failed", response.StatusCode)
	}
	var replay struct {
		Data struct {
			ID              string `json:"id"`
			SourceVersionID string `json:"source_version_id"`
		} `json:"data"`
	}
	if err = json.Unmarshal(replayBody, &replay); err != nil || replay.Data != first.Data {
		t.Fatal("live HTTP replay changed result", err)
	}

	denyGrantID := uuid.New()
	if err = controlMigrator.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, execErr := db.Exec(ctx, `INSERT INTO tenancy.role_grants (id,membership_id,role_key,capability,effect,created_by,created_at,revoked_at) VALUES ($1,$2,'pilot-deny','tenderpreneur.opportunity.capture_external','DENY',$3,$4,NULL)`, denyGrantID, membershipID, uuid.UUID(userID), time.Now().UTC())
		return execErr
	}); err != nil {
		t.Fatal(err)
	}
	response, err = client.Do(captureRequest(uuid.UUID(tenantID).String(), "3", uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != 403 {
		t.Fatal("active DENY did not override ALLOW", response.StatusCode)
	}
	if err = controlMigrator.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, execErr := db.Exec(ctx, `UPDATE tenancy.role_grants SET revoked_at=$2 WHERE id=$1 AND revoked_at IS NULL`, denyGrantID, time.Now().UTC())
		return execErr
	}); err != nil {
		t.Fatal(err)
	}
	response, err = client.Do(captureRequest(uuid.UUID(tenantID).String(), "3", uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != 403 {
		t.Fatal("stale context generation authorized", response.StatusCode)
	}
	if err = controlMigrator.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, execErr := db.Exec(ctx, `UPDATE bidora_tenancy.tenants SET assignment_generation=2,version=version+1,updated_at=$2 WHERE id=$1`, uuid.UUID(tenantID), time.Now().UTC())
		return execErr
	}); err != nil {
		t.Fatal(err)
	}
	response, err = client.Do(captureRequest(uuid.UUID(tenantID).String(), "4", uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != 503 {
		t.Fatal("stale tenant assignment reached tenant store", response.StatusCode)
	}
	if err = controlMigrator.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, execErr := db.Exec(ctx, `UPDATE bidora_tenancy.tenants SET assignment_generation=1,version=version+1,updated_at=$2 WHERE id=$1`, uuid.UUID(tenantID), time.Now().UTC())
		return execErr
	}); err != nil {
		t.Fatal(err)
	}
	response, err = client.Do(captureRequest(uuid.UUID(tenantID).String(), "4", uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != 201 {
		t.Fatal("capture did not recover after assignment restoration", response.StatusCode)
	}
	badChangeBody, _ := json.Marshal(map[string]string{"email": emailValue, "current_password": "wrong synthetic password", "new_password": "synthetic ignored password"})
	badChangeRequest, err := http.NewRequest(http.MethodPost, baseURL+httpapi.PasswordChangePath, bytes.NewReader(badChangeBody))
	if err != nil {
		t.Fatal(err)
	}
	badChangeRequest.Header.Set("Content-Type", "application/json")
	badChangeRequest.Header.Set("Origin", "https://app.example.test")
	badChangeRequest.Header.Set(session.CSRFHeader, csrfCookie.Value)
	badChangeRequest.Header.Set("X-Forwarded-For", "203.0.113.200")
	addBrowserCookies(badChangeRequest, sessionCookie, csrfCookie)
	response, err = client.Do(badChangeRequest)
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatal("wrong current password changed credential", response.StatusCode)
	}
	if rowCount(t, controlRuntime, `SELECT count(*) FROM identity.users AS u JOIN identity.authentication_identities AS a ON a.user_id=u.id WHERE u.id=$1 AND u.auth_epoch=1 AND a.credential_version=1`, uuid.UUID(userID)) != 1 {
		t.Fatal("failed password proof mutated identity")
	}
	oldSessionCookie, oldCSRFCookie := sessionCookie, csrfCookie
	newPasswordValue := "synthetic replacement password"
	changeBody, _ := json.Marshal(map[string]string{"email": emailValue, "current_password": string(passwordValue), "new_password": newPasswordValue})
	changeRequest, err := http.NewRequest(http.MethodPost, baseURL+httpapi.PasswordChangePath, bytes.NewReader(changeBody))
	if err != nil {
		t.Fatal(err)
	}
	changeRequest.Header.Set("Content-Type", "application/json")
	changeRequest.Header.Set("Origin", "https://app.example.test")
	changeRequest.Header.Set(session.CSRFHeader, csrfCookie.Value)
	changeRequest.Header.Set("X-Forwarded-For", "203.0.113.200")
	addBrowserCookies(changeRequest, sessionCookie, csrfCookie)
	response, err = client.Do(changeRequest)
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != http.StatusNoContent {
		t.Fatal("live password change failed", response.StatusCode)
	}
	sessionCookie, csrfCookie = nil, nil
	for _, cookie := range response.Cookies() {
		switch cookie.Name {
		case session.CookieName:
			sessionCookie = cookie
		case session.CSRFCookieName:
			csrfCookie = cookie
		}
	}
	if sessionCookie == nil || csrfCookie == nil || sessionCookie.Value == oldSessionCookie.Value || csrfCookie.Value == oldCSRFCookie.Value {
		t.Fatal("password change did not rotate browser credentials")
	}
	oldBootstrapRequest, err := http.NewRequest(http.MethodGet, baseURL+httpapi.SessionBootstrapPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	oldBootstrapRequest.Header.Set("Origin", "https://app.example.test")
	addBrowserCookies(oldBootstrapRequest, oldSessionCookie, oldCSRFCookie)
	response, err = client.Do(oldBootstrapRequest)
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatal("old session survived password change", response.StatusCode)
	}
	newBootstrapRequest, err := http.NewRequest(http.MethodGet, baseURL+httpapi.SessionBootstrapPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	newBootstrapRequest.Header.Set("Origin", "https://app.example.test")
	addBrowserCookies(newBootstrapRequest, sessionCookie, csrfCookie)
	response, err = client.Do(newBootstrapRequest)
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatal("replacement session failed after password change", response.StatusCode)
	}
	if err = controlRuntime.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		var authEpoch, credentialVersion int64
		var storedPHC string
		if queryErr := db.QueryRow(ctx, `SELECT u.auth_epoch,a.credential_version,a.password_phc FROM identity.users AS u JOIN identity.authentication_identities AS a ON a.user_id=u.id WHERE u.id=$1`, uuid.UUID(userID)).Scan(&authEpoch, &credentialVersion, &storedPHC); queryErr != nil {
			return queryErr
		}
		verification, verifyErr := hasher.Verify([]byte(newPasswordValue), storedPHC)
		oldVerification, oldVerifyErr := hasher.Verify(passwordValue, storedPHC)
		if verifyErr != nil || !verification.Match || oldVerifyErr != nil || oldVerification.Match || authEpoch != 2 || credentialVersion != 2 {
			return errors.New("password change persistence drifted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if rowCount(t, controlRuntime, `SELECT count(*) FROM identity.sessions WHERE user_id=$1 AND revoked_at IS NULL AND auth_epoch=2`, uuid.UUID(userID)) != 1 || rowCount(t, controlRuntime, `SELECT count(*) FROM bidora_control_audit.audit_events WHERE actor_id=$1 AND action='PasswordChanged'`, uuid.UUID(userID)) != 1 {
		t.Fatal("password change rotation evidence drifted")
	}
	if err = deployment.Runtime.Close(); err != nil {
		t.Fatal(err)
	}
	response, err = client.Do(captureRequest(uuid.UUID(tenantID).String(), "4", uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != 201 {
		t.Fatal("Redis loss interrupted an existing authorized capture session", response.StatusCode)
	}

	logoutRequest, err := http.NewRequest(http.MethodPost, baseURL+httpapi.LogoutPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	logoutRequest.Header.Set("Origin", "https://app.example.test")
	logoutRequest.Header.Set(session.CSRFHeader, csrfCookie.Value)
	addBrowserCookies(logoutRequest, sessionCookie, csrfCookie)
	response, err = client.Do(logoutRequest)
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != 204 {
		t.Fatal("live HTTP logout failed", response.StatusCode)
	}
	response, err = client.Do(captureRequest(uuid.UUID(tenantID).String(), "4", uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != 401 {
		t.Fatal("revoked session reached capture", response.StatusCode)
	}
	if rowCount(t, tenantRuntime, `SELECT count(*) FROM bidora_opportunity.opportunities WHERE id=$1`, first.Data.ID) != 1 {
		t.Fatal("live HTTP replay duplicated opportunity")
	}
	sessionCount := rowCount(t, controlRuntime, `SELECT count(*) FROM identity.sessions WHERE user_id=$1`, uuid.UUID(userID))
	reloginRequest, err := http.NewRequest(http.MethodPost, baseURL+httpapi.LoginPath, bytes.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	reloginRequest.Header.Set("Content-Type", "application/json")
	reloginRequest.Header.Set("Origin", "https://app.example.test")
	response, err = client.Do(reloginRequest)
	if err != nil {
		t.Fatal(err)
	}
	responseBody(t, response)
	if response.StatusCode != 503 {
		t.Fatal("login did not fail closed after Redis loss", response.StatusCode)
	}
	for _, cookie := range response.Cookies() {
		if cookie.Name == session.CookieName || cookie.Name == session.CSRFCookieName {
			t.Fatal("failed login issued browser credentials")
		}
	}
	if got := rowCount(t, controlRuntime, `SELECT count(*) FROM identity.sessions WHERE user_id=$1`, uuid.UUID(userID)); got != sessionCount {
		t.Fatal("Redis-unavailable login persisted a session", sessionCount, got)
	}
}
