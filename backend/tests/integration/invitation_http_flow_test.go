package integration_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/authorization/invitation"
	"bidora/backend/internal/httpapi"
	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/password"
	"bidora/backend/internal/identity/session"
	"bidora/backend/internal/pilot"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

func TestLivePilotInvitationIssueAcceptAndAuthorizedCaptureJourney(t *testing.T) {
	redisFixture := redisIntegration(t)
	config := recoveryDeploymentConfig(t, redisFixture)
	control := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	tenant := livePool(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN")
	now := time.Now().UTC().Truncate(time.Microsecond)
	managerID := uuid.New()
	inviteeID := uuid.New()
	managerMembership := uuid.New()
	managerEmail := "manager-" + uuid.NewString() + "@example.invalid"
	inviteeEmail := "Invitee-" + uuid.NewString() + "@example.invalid"
	managerPassword := "synthetic manager password"
	inviteePassword := "synthetic invitee password"
	hasher, err := password.New(password.PilotPolicy())
	if err != nil {
		t.Fatal(err)
	}
	managerPHC, err := hasher.Hash([]byte(managerPassword))
	if err != nil {
		t.Fatal(err)
	}
	inviteePHC, err := hasher.Hash([]byte(inviteePassword))
	if err != nil {
		t.Fatal(err)
	}
	err = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		for _, row := range []struct {
			id        uuid.UUID
			mail, phc string
		}{{managerID, managerEmail, managerPHC}, {inviteeID, inviteeEmail, inviteePHC}} {
			if _, e := db.Exec(ctx, `INSERT INTO identity.users (id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES ($1,$2,lower($2),$3,NULL,1,$3,$3)`, row.id, row.mail, now); e != nil {
				return e
			}
			if _, e := db.Exec(ctx, `INSERT INTO identity.authentication_identities (user_id,password_phc,credential_version,password_changed_at,created_at,updated_at) VALUES ($1,$2,1,$3,$3,$3)`, row.id, row.phc, now); e != nil {
				return e
			}
		}
		if _, e := db.Exec(ctx, `INSERT INTO tenancy.memberships (id,user_id,tenant_id,state,context_generation,created_at,updated_at) VALUES ($1,$2,$3,'ACTIVE',1,$4,$4)`, managerMembership, managerID, uuid.UUID(config.Tenants[0].Assignment.TenantID), now); e != nil {
			return e
		}
		_, e := db.Exec(ctx, `INSERT INTO tenancy.role_grants (id,membership_id,role_key,capability,effect,created_by,created_at) VALUES ($1,$2,'TENANT_MEMBERSHIP_MANAGER','tenant.memberships.manage','ALLOW',$3,$4)`, uuid.New(), managerMembership, managerID, now)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	smtpFixture := newPilotSMTPFixture(t, inviteeEmail)
	sender, err := email.Parse("noreply@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	config.Runtime.Registration = &pilot.RegistrationConfig{SMTP: deliverysmtp.Config{Host: "127.0.0.1", Port: uint16(smtpFixture.listener.Addr().(*net.TCPAddr).Port), Username: "smtp-user", Password: "smtp-password", Sender: sender, TLS: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "localhost", RootCAs: smtpFixture.roots}, Timeout: time.Second}}
	deployment, err := pilot.Open(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Shutdown(context.Background())
	runtime := deployment.Runtime
	login := func(mail, passwordValue string) (*http.Cookie, *http.Cookie) {
		request := httptest.NewRequest(http.MethodPost, httpapi.LoginPath, bytes.NewBufferString(`{"email":"`+mail+`","password":"`+passwordValue+`"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "https://app.example.test")
		response, requestErr := runtime.App.Test(request)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			t.Fatal("invitation journey login failed", response.StatusCode)
		}
		var sessionCookie, csrfCookie *http.Cookie
		for _, cookie := range response.Cookies() {
			if cookie.Name == session.CookieName {
				sessionCookie = cookie
			}
			if cookie.Name == session.CSRFCookieName {
				csrfCookie = cookie
			}
		}
		if sessionCookie == nil || csrfCookie == nil {
			t.Fatal("invitation journey login cookies missing")
		}
		return sessionCookie, csrfCookie
	}
	managerSession, managerCSRF := login(managerEmail, managerPassword)
	issueRequest := httptest.NewRequest(http.MethodPost, httpapi.InvitationIssuePath, bytes.NewBufferString(`{"email":"`+inviteeEmail+`"}`))
	issueRequest.Header.Set("Content-Type", "application/json")
	issueRequest.Header.Set("Origin", "https://app.example.test")
	issueRequest.Header.Set(session.CSRFHeader, managerCSRF.Value)
	issueRequest.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	issueRequest.Header.Set(httpapi.ContextGenerationHeader, "2")
	issueRequest.AddCookie(managerSession)
	issueRequest.AddCookie(managerCSRF)
	issueResponse, err := runtime.App.Test(issueRequest)
	if err != nil {
		t.Fatal(err)
	}
	issueResponse.Body.Close()
	if issueResponse.StatusCode != http.StatusCreated {
		t.Fatal("composed invitation issue failed", issueResponse.StatusCode)
	}
	var rawToken string
	select {
	case message := <-smtpFixture.message:
		if !strings.Contains(message, "Subject: You are invited to a Bidora workspace") {
			t.Fatal("invitation SMTP subject drifted")
		}
		for _, field := range strings.Fields(message) {
			if _, hashErr := invitation.HashPresented(field); hashErr == nil {
				rawToken = field
				break
			}
		}
	case <-time.After(time.Second):
		t.Fatal("invitation SMTP message missing")
	}
	if rawToken == "" {
		t.Fatal("invitation token missing")
	}
	secondInviteeEmail := "pending-" + uuid.NewString() + "@example.test"
	secondInvitationID := uuid.New()
	secondTokenHash := append(append([]byte{}, secondInvitationID[:]...), secondInvitationID[:]...)
	secondCreatedAt := time.Now().UTC()
	if err = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, execErr := db.Exec(ctx, `INSERT INTO tenancy.invitations (id,tenant_id,email,email_lookup,token_hash,role_key,capability,created_by,created_at,expires_at) VALUES ($1,$2,$3,$3,$4,'TENDERPRENEUR_CAPTURE_CONTRIBUTOR','tenderpreneur.opportunity.capture_external',$5,$6,$7)`, secondInvitationID, uuid.UUID(config.Tenants[0].Assignment.TenantID), secondInviteeEmail, secondTokenHash, managerID, secondCreatedAt, secondCreatedAt.Add(72*time.Hour))
		return execErr
	}); err != nil {
		t.Fatal(err)
	}
	pendingRequest := httptest.NewRequest(http.MethodGet, httpapi.InvitationDirectoryPath+"?limit=1", nil)
	pendingRequest.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	pendingRequest.Header.Set(httpapi.ContextGenerationHeader, "2")
	pendingRequest.AddCookie(managerSession)
	pendingResponse, err := runtime.App.Test(pendingRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer pendingResponse.Body.Close()
	var pending struct {
		Data struct {
			Invitations []struct {
				InvitationID string `json:"invitation_id"`
				Email        string `json:"email"`
				CreatedAt    string `json:"created_at"`
				ExpiresAt    string `json:"expires_at"`
			} `json:"invitations"`
			NextCursor *string `json:"next_cursor"`
		} `json:"data"`
	}
	if pendingResponse.StatusCode != http.StatusOK || json.NewDecoder(pendingResponse.Body).Decode(&pending) != nil || len(pending.Data.Invitations) != 1 || pending.Data.Invitations[0].Email != secondInviteeEmail || pending.Data.Invitations[0].InvitationID == "" || pending.Data.Invitations[0].CreatedAt == "" || pending.Data.Invitations[0].ExpiresAt == "" || pending.Data.NextCursor == nil {
		t.Fatal("pending invitation directory drifted", pendingResponse.StatusCode, pending.Data.Invitations)
	}
	nextPendingRequest := httptest.NewRequest(http.MethodGet, httpapi.InvitationDirectoryPath+"?limit=1&cursor="+url.QueryEscape(*pending.Data.NextCursor), nil)
	nextPendingRequest.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	nextPendingRequest.Header.Set(httpapi.ContextGenerationHeader, "2")
	nextPendingRequest.AddCookie(managerSession)
	nextPendingResponse, err := runtime.App.Test(nextPendingRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer nextPendingResponse.Body.Close()
	var nextPending struct {
		Data struct {
			Invitations []struct {
				InvitationID string `json:"invitation_id"`
				Email        string `json:"email"`
			} `json:"invitations"`
			NextCursor *string `json:"next_cursor"`
		} `json:"data"`
	}
	if nextPendingResponse.StatusCode != http.StatusOK || json.NewDecoder(nextPendingResponse.Body).Decode(&nextPending) != nil || len(nextPending.Data.Invitations) != 1 || nextPending.Data.Invitations[0].Email != inviteeEmail || nextPending.Data.NextCursor != nil {
		t.Fatal("pending invitation cursor drifted", nextPendingResponse.StatusCode, nextPending.Data.Invitations)
	}
	cancelRequest := httptest.NewRequest(http.MethodDelete, "/v1/shared/invitations/"+secondInvitationID.String(), nil)
	cancelRequest.Header.Set("Origin", "https://app.example.test")
	cancelRequest.Header.Set(session.CSRFHeader, managerCSRF.Value)
	cancelRequest.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	cancelRequest.Header.Set(httpapi.ContextGenerationHeader, "2")
	cancelRequest.AddCookie(managerSession)
	cancelRequest.AddCookie(managerCSRF)
	cancelResponse, err := runtime.App.Test(cancelRequest)
	if err != nil {
		t.Fatal(err)
	}
	cancelResponse.Body.Close()
	if cancelResponse.StatusCode != http.StatusNoContent {
		t.Fatal("pending invitation cancellation failed", cancelResponse.StatusCode)
	}
	replayCancelRequest := httptest.NewRequest(http.MethodDelete, "/v1/shared/invitations/"+secondInvitationID.String(), nil)
	replayCancelRequest.Header.Set("Origin", "https://app.example.test")
	replayCancelRequest.Header.Set(session.CSRFHeader, managerCSRF.Value)
	replayCancelRequest.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	replayCancelRequest.Header.Set(httpapi.ContextGenerationHeader, "2")
	replayCancelRequest.AddCookie(managerSession)
	replayCancelRequest.AddCookie(managerCSRF)
	replayCancelResponse, err := runtime.App.Test(replayCancelRequest)
	if err != nil {
		t.Fatal(err)
	}
	replayCancelResponse.Body.Close()
	if replayCancelResponse.StatusCode != http.StatusNotFound {
		t.Fatal("cancel replay was not concealed", replayCancelResponse.StatusCode)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM tenancy.invitations AS i JOIN bidora_control_audit.audit_events AS a ON a.target_id=i.id JOIN bidora_control_eventing.outbox_events AS o ON o.aggregate_id=i.id WHERE i.id=$1 AND i.invalidated_at IS NOT NULL AND a.action='InvitationCancelled' AND o.event_type='invitation.cancelled.v1' AND o.aggregate_version=2`, secondInvitationID); count != 1 {
		t.Fatal("invitation cancellation persistence drifted", count)
	}
	inviteeSession, inviteeCSRF := login(inviteeEmail, inviteePassword)
	unauthorizedPendingRequest := httptest.NewRequest(http.MethodGet, httpapi.InvitationDirectoryPath, nil)
	unauthorizedPendingRequest.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	unauthorizedPendingRequest.Header.Set(httpapi.ContextGenerationHeader, "1")
	unauthorizedPendingRequest.AddCookie(inviteeSession)
	unauthorizedPendingResponse, err := runtime.App.Test(unauthorizedPendingRequest)
	if err != nil {
		t.Fatal(err)
	}
	unauthorizedPendingResponse.Body.Close()
	if unauthorizedPendingResponse.StatusCode != http.StatusForbidden {
		t.Fatal("non-manager listed pending invitations", unauthorizedPendingResponse.StatusCode)
	}
	unauthorizedCancelRequest := httptest.NewRequest(http.MethodDelete, "/v1/shared/invitations/"+nextPending.Data.Invitations[0].InvitationID, nil)
	unauthorizedCancelRequest.Header.Set("Origin", "https://app.example.test")
	unauthorizedCancelRequest.Header.Set(session.CSRFHeader, inviteeCSRF.Value)
	unauthorizedCancelRequest.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	unauthorizedCancelRequest.Header.Set(httpapi.ContextGenerationHeader, "1")
	unauthorizedCancelRequest.AddCookie(inviteeSession)
	unauthorizedCancelRequest.AddCookie(inviteeCSRF)
	unauthorizedCancelResponse, err := runtime.App.Test(unauthorizedCancelRequest)
	if err != nil {
		t.Fatal(err)
	}
	unauthorizedCancelResponse.Body.Close()
	if unauthorizedCancelResponse.StatusCode != http.StatusNotFound {
		t.Fatal("non-manager cancellation was not concealed", unauthorizedCancelResponse.StatusCode)
	}
	acceptRequest := httptest.NewRequest(http.MethodPost, httpapi.InvitationAcceptPath, bytes.NewBufferString(`{"token":"`+rawToken+`"}`))
	acceptRequest.Header.Set("Content-Type", "application/json")
	acceptRequest.Header.Set("Origin", "https://app.example.test")
	acceptRequest.Header.Set(session.CSRFHeader, inviteeCSRF.Value)
	acceptRequest.AddCookie(inviteeSession)
	acceptRequest.AddCookie(inviteeCSRF)
	acceptResponse, err := runtime.App.Test(acceptRequest)
	if err != nil {
		t.Fatal(err)
	}
	acceptResponse.Body.Close()
	if acceptResponse.StatusCode != http.StatusNoContent || len(acceptResponse.Cookies()) != 0 {
		t.Fatal("composed invitation acceptance failed", acceptResponse.StatusCode, acceptResponse.Cookies())
	}
	emptyPendingRequest := httptest.NewRequest(http.MethodGet, httpapi.InvitationDirectoryPath, nil)
	emptyPendingRequest.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	emptyPendingRequest.Header.Set(httpapi.ContextGenerationHeader, "2")
	emptyPendingRequest.AddCookie(managerSession)
	emptyPendingResponse, err := runtime.App.Test(emptyPendingRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer emptyPendingResponse.Body.Close()
	var emptyPending struct {
		Data struct {
			Invitations []struct {
				Email string `json:"email"`
			} `json:"invitations"`
		} `json:"data"`
	}
	if emptyPendingResponse.StatusCode != http.StatusOK || json.NewDecoder(emptyPendingResponse.Body).Decode(&emptyPending) != nil || len(emptyPending.Data.Invitations) != 0 {
		t.Fatal("consumed invitation filtering drifted", emptyPending.Data.Invitations)
	}
	membershipsRequest := httptest.NewRequest(http.MethodGet, httpapi.MembershipDirectoryPath, nil)
	membershipsRequest.AddCookie(inviteeSession)
	membershipsResponse, err := runtime.App.Test(membershipsRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer membershipsResponse.Body.Close()
	var memberships struct {
		Data struct {
			Memberships []struct {
				MembershipID      string   `json:"membership_id"`
				TenantID          string   `json:"tenant_id"`
				ContextGeneration int64    `json:"context_generation"`
				Capabilities      []string `json:"capabilities"`
			} `json:"memberships"`
		} `json:"data"`
	}
	if membershipsResponse.StatusCode != http.StatusOK || json.NewDecoder(membershipsResponse.Body).Decode(&memberships) != nil || len(memberships.Data.Memberships) != 1 || memberships.Data.Memberships[0].TenantID != uuid.UUID(config.Tenants[0].Assignment.TenantID).String() || memberships.Data.Memberships[0].ContextGeneration != 2 || len(memberships.Data.Memberships[0].Capabilities) != 1 || memberships.Data.Memberships[0].Capabilities[0] != "tenderpreneur.opportunity.capture_external" {
		t.Fatal("invited membership directory drifted", membershipsResponse.StatusCode, memberships.Data.Memberships)
	}
	managedDirectoryRequest := httptest.NewRequest(http.MethodGet, httpapi.MembershipDirectoryManagementPath, nil)
	managedDirectoryRequest.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	managedDirectoryRequest.Header.Set(httpapi.ContextGenerationHeader, "2")
	managedDirectoryRequest.AddCookie(managerSession)
	managedDirectoryResponse, err := runtime.App.Test(managedDirectoryRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer managedDirectoryResponse.Body.Close()
	var managedDirectory struct {
		Data struct {
			Memberships []struct {
				MembershipID      string `json:"membership_id"`
				Email             string `json:"email"`
				ContextGeneration int64  `json:"context_generation"`
			} `json:"memberships"`
		} `json:"data"`
	}
	if managedDirectoryResponse.StatusCode != http.StatusOK || json.NewDecoder(managedDirectoryResponse.Body).Decode(&managedDirectory) != nil || len(managedDirectory.Data.Memberships) != 1 || managedDirectory.Data.Memberships[0].MembershipID != memberships.Data.Memberships[0].MembershipID || managedDirectory.Data.Memberships[0].Email != inviteeEmail || managedDirectory.Data.Memberships[0].ContextGeneration != 2 {
		t.Fatal("managed contributor directory drifted", managedDirectoryResponse.StatusCode, managedDirectory.Data.Memberships)
	}
	unauthorizedDirectoryRequest := httptest.NewRequest(http.MethodGet, httpapi.MembershipDirectoryManagementPath, nil)
	unauthorizedDirectoryRequest.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	unauthorizedDirectoryRequest.Header.Set(httpapi.ContextGenerationHeader, "2")
	unauthorizedDirectoryRequest.AddCookie(inviteeSession)
	unauthorizedDirectoryResponse, err := runtime.App.Test(unauthorizedDirectoryRequest)
	if err != nil {
		t.Fatal(err)
	}
	unauthorizedDirectoryResponse.Body.Close()
	if unauthorizedDirectoryResponse.StatusCode != http.StatusForbidden {
		t.Fatal("capture contributor listed managed memberships", unauthorizedDirectoryResponse.StatusCode)
	}
	captureTitle := "invited-capture-" + uuid.NewString()
	captureRequest := httptest.NewRequest(http.MethodPost, httpapi.ManualCapturePath, bytes.NewBufferString(`{"title":"`+captureTitle+`","buyer_name":null}`))
	captureRequest.Header.Set("Content-Type", "application/json")
	captureRequest.Header.Set("Origin", "https://app.example.test")
	captureRequest.Header.Set(session.CSRFHeader, inviteeCSRF.Value)
	captureRequest.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	captureRequest.Header.Set(httpapi.ContextGenerationHeader, "2")
	captureRequest.Header.Set(httpapi.IdempotencyKeyHeader, uuid.NewString())
	captureRequest.AddCookie(inviteeSession)
	captureRequest.AddCookie(inviteeCSRF)
	captureResponse, err := runtime.App.Test(captureRequest)
	if err != nil {
		t.Fatal(err)
	}
	captureResponse.Body.Close()
	if captureResponse.StatusCode != http.StatusCreated {
		t.Fatal("invited capture contributor was not authorized", captureResponse.StatusCode)
	}
	if count := rowCount(t, tenant, `SELECT count(*) FROM bidora_opportunity.source_versions WHERE title=$1`, captureTitle); count != 1 {
		t.Fatal("invited capture did not persist", count)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM tenancy.role_grants AS g JOIN tenancy.memberships AS m ON m.id=g.membership_id WHERE m.user_id=$1 AND m.tenant_id=$2 AND m.state='ACTIVE' AND m.context_generation=2 AND g.capability='tenderpreneur.opportunity.capture_external' AND g.effect='ALLOW' AND g.revoked_at IS NULL`, inviteeID, uuid.UUID(config.Tenants[0].Assignment.TenantID)); count != 1 {
		t.Fatal("invited fixed grant missing", count)
	}
	protectedManagerRequest := httptest.NewRequest(http.MethodDelete, "/v1/shared/memberships/"+managerMembership.String(), nil)
	protectedManagerRequest.Header.Set("Origin", "https://app.example.test")
	protectedManagerRequest.Header.Set(session.CSRFHeader, managerCSRF.Value)
	protectedManagerRequest.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	protectedManagerRequest.Header.Set(httpapi.ContextGenerationHeader, "2")
	protectedManagerRequest.AddCookie(managerSession)
	protectedManagerRequest.AddCookie(managerCSRF)
	protectedManagerResponse, err := runtime.App.Test(protectedManagerRequest)
	if err != nil {
		t.Fatal(err)
	}
	protectedManagerResponse.Body.Close()
	if protectedManagerResponse.StatusCode != http.StatusNotFound {
		t.Fatal("manager membership was eligible for capture-contributor revocation", protectedManagerResponse.StatusCode)
	}
	revokeRequest := httptest.NewRequest(http.MethodDelete, "/v1/shared/memberships/"+memberships.Data.Memberships[0].MembershipID, nil)
	revokeRequest.Header.Set("Origin", "https://app.example.test")
	revokeRequest.Header.Set(session.CSRFHeader, managerCSRF.Value)
	revokeRequest.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	revokeRequest.Header.Set(httpapi.ContextGenerationHeader, "2")
	revokeRequest.AddCookie(managerSession)
	revokeRequest.AddCookie(managerCSRF)
	revokeResponse, err := runtime.App.Test(revokeRequest)
	if err != nil {
		t.Fatal(err)
	}
	revokeResponse.Body.Close()
	if revokeResponse.StatusCode != http.StatusNoContent {
		t.Fatal("fixed contributor revocation failed", revokeResponse.StatusCode)
	}
	revokedDirectoryRequest := httptest.NewRequest(http.MethodGet, httpapi.MembershipDirectoryPath, nil)
	revokedDirectoryRequest.AddCookie(inviteeSession)
	revokedDirectoryResponse, err := runtime.App.Test(revokedDirectoryRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer revokedDirectoryResponse.Body.Close()
	var revokedDirectory struct {
		Data struct {
			Memberships []json.RawMessage `json:"memberships"`
		} `json:"data"`
	}
	if revokedDirectoryResponse.StatusCode != http.StatusOK || json.NewDecoder(revokedDirectoryResponse.Body).Decode(&revokedDirectory) != nil || len(revokedDirectory.Data.Memberships) != 0 {
		t.Fatal("revoked membership remained in directory", revokedDirectoryResponse.StatusCode, len(revokedDirectory.Data.Memberships))
	}
	deniedCapture := httptest.NewRequest(http.MethodPost, httpapi.ManualCapturePath, bytes.NewBufferString(`{"title":"denied-after-revocation","buyer_name":null}`))
	deniedCapture.Header.Set("Content-Type", "application/json")
	deniedCapture.Header.Set("Origin", "https://app.example.test")
	deniedCapture.Header.Set(session.CSRFHeader, inviteeCSRF.Value)
	deniedCapture.Header.Set(httpapi.TenantIDHeader, uuid.UUID(config.Tenants[0].Assignment.TenantID).String())
	deniedCapture.Header.Set(httpapi.ContextGenerationHeader, "2")
	deniedCapture.Header.Set(httpapi.IdempotencyKeyHeader, uuid.NewString())
	deniedCapture.AddCookie(inviteeSession)
	deniedCapture.AddCookie(inviteeCSRF)
	deniedCaptureResponse, err := runtime.App.Test(deniedCapture)
	if err != nil {
		t.Fatal(err)
	}
	deniedCaptureResponse.Body.Close()
	if deniedCaptureResponse.StatusCode != http.StatusForbidden {
		t.Fatal("revoked invited member retained capture authority", deniedCaptureResponse.StatusCode)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM tenancy.memberships AS m JOIN tenancy.role_grants AS g ON g.membership_id=m.id JOIN bidora_control_audit.audit_events AS a ON a.target_id=m.id JOIN bidora_control_eventing.outbox_events AS o ON o.aggregate_id=m.id WHERE m.id=$1 AND m.state='REVOKED' AND m.context_generation=4 AND g.revoked_at IS NOT NULL AND a.action='MembershipRevoked' AND o.event_type='membership.revoked.v1' AND o.aggregate_version=4`, uuid.MustParse(memberships.Data.Memberships[0].MembershipID)); count != 1 {
		t.Fatal("fixed contributor revocation persistence drifted", count)
	}
}
