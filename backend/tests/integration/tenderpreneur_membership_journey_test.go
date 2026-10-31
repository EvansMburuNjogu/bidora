package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	authzpersistence "bidora/backend/internal/authorization/persistence"
	"bidora/backend/internal/health"
	"bidora/backend/internal/httpapi"
	"bidora/backend/internal/identity/mfa"
	"bidora/backend/internal/identity/password"
	"bidora/backend/internal/identity/session"
	"bidora/backend/internal/pilot"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

func TestLiveTenderpreneurInvitationMembershipEnablesPortalWithoutCreatingOrganization(t *testing.T) {
	redisFixture := redisIntegration(t)
	control := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	runtimeControl := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	tenantCountBefore := rowCount(t, control, `SELECT count(*) FROM bidora_tenancy.tenants`)

	var tenantID uuid.UUID
	if err := control.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		return db.QueryRow(ctx, `SELECT id FROM bidora_tenancy.tenants WHERE tenant_type='TENDERPRENEUR' AND state='ACTIVE' ORDER BY created_at,id LIMIT 1`).Scan(&tenantID)
	}); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	managerID, inviteeID, managerMembershipID := uuid.New(), uuid.New(), uuid.New()
	managerEmail := "membership-manager-" + uuid.NewString() + "@example.invalid"
	inviteeEmail := "membership-invitee-" + uuid.NewString() + "@example.invalid"
	inviteePassword := "synthetic membership invitee password"
	hasher, err := password.New(password.PilotPolicy())
	if err != nil {
		t.Fatal(err)
	}
	dummy, err := hasher.Hash([]byte("synthetic membership dummy password"))
	if err != nil {
		t.Fatal(err)
	}
	inviteePHC, err := hasher.Hash([]byte(inviteePassword))
	if err != nil {
		t.Fatal(err)
	}
	if err = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		for _, row := range []struct {
			id, membership uuid.UUID
			email, phc     string
		}{{managerID, uuid.Nil, managerEmail, inviteePHC}, {inviteeID, uuid.Nil, inviteeEmail, inviteePHC}} {
			if _, execErr := db.Exec(ctx, `INSERT INTO identity.users (id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES ($1,$2,lower($2),$3,NULL,1,$3,$3)`, row.id, row.email, now); execErr != nil {
				return execErr
			}
			if _, execErr := db.Exec(ctx, `INSERT INTO identity.authentication_identities (user_id,password_phc,credential_version,password_changed_at,created_at,updated_at) VALUES ($1,$2,1,$3,$3,$3)`, row.id, row.phc, now); execErr != nil {
				return execErr
			}
		}
		if _, execErr := db.Exec(ctx, `INSERT INTO tenancy.memberships (id,user_id,tenant_id,state,context_generation,created_at,updated_at) VALUES ($1,$2,$3,'ACTIVE',1,$4,$4)`, managerMembershipID, managerID, tenantID, now); execErr != nil {
			return execErr
		}
		_, execErr := db.Exec(ctx, `INSERT INTO tenancy.role_grants (id,membership_id,role_key,capability,effect,created_by,created_at) VALUES ($1,$2,'TENANT_MEMBERSHIP_MANAGER','tenant.memberships.manage','ALLOW',$3,$4)`, uuid.New(), managerMembershipID, managerID, now)
		return execErr
	}); err != nil {
		t.Fatal(err)
	}

	attemptSecret := sha256.Sum256([]byte(uuid.NewString()))
	mfaSecret := sha256.Sum256([]byte("membership-mfa-" + uuid.NewString()))
	keyRing, err := mfa.NewKeyRing(1, map[int][32]byte{1: mfaSecret})
	if err != nil {
		t.Fatal(err)
	}
	state := &health.State{}
	runtime, err := pilot.BuildIdentityRuntime(pilot.Config{
		PasswordPolicy: password.PilotPolicy(), DummyPasswordPHC: dummy, AttemptHMACSecret: attemptSecret[:],
		Redis: redisFixture.config, RedisPrefix: "bidora:test:membership:" + uuid.NewString(),
		AllowedOrigins: []string{"https://app.example.test"}, TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}, MFAKeyRing: keyRing,
	}, state, runtimeControl)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	state.Start()

	login := httptest.NewRequest(http.MethodPost, httpapi.LoginPath, bytes.NewBufferString(`{"email":"`+inviteeEmail+`","password":"`+inviteePassword+`"}`))
	login.Header.Set("Content-Type", "application/json")
	login.Header.Set("Origin", "https://app.example.test")
	loginResponse, err := runtime.App.Test(login)
	if err != nil {
		t.Fatal(err)
	}
	loginResponse.Body.Close()
	if loginResponse.StatusCode != http.StatusNoContent {
		t.Fatal("synthetic invitee login failed", loginResponse.StatusCode)
	}
	var sessionCookie, csrfCookie *http.Cookie
	for _, cookie := range loginResponse.Cookies() {
		if cookie.Name == session.CookieName {
			sessionCookie = cookie
		}
		if cookie.Name == session.CSRFCookieName {
			csrfCookie = cookie
		}
	}
	if sessionCookie == nil || csrfCookie == nil {
		t.Fatal("synthetic invitee session cookies missing")
	}

	selectType := httptest.NewRequest(http.MethodPost, httpapi.AccountTypeSelectionPath, bytes.NewBufferString(`{"account_type":"TENDERPRENEUR"}`))
	selectType.Header.Set("Content-Type", "application/json")
	selectType.Header.Set("Origin", "https://app.example.test")
	selectType.Header.Set(session.CSRFHeader, csrfCookie.Value)
	selectType.Header.Set(httpapi.IdempotencyKeyHeader, uuid.NewString())
	selectType.AddCookie(sessionCookie)
	selectType.AddCookie(csrfCookie)
	selectResponse, err := runtime.App.Test(selectType)
	if err != nil {
		t.Fatal(err)
	}
	selectResponse.Body.Close()
	if selectResponse.StatusCode != http.StatusNoContent {
		t.Fatal("Tenderpreneur selection failed", selectResponse.StatusCode)
	}
	assertTenderpreneurPortal := func(wantPortal bool) {
		request := httptest.NewRequest(http.MethodGet, httpapi.MePortalPath, nil)
		request.Header.Set("Origin", "https://app.example.test")
		request.AddCookie(sessionCookie)
		request.AddCookie(csrfCookie)
		response, requestErr := runtime.App.Test(request)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		defer response.Body.Close()
		var payload struct {
			Data struct {
				Portal *struct {
					Key, Path string
				} `json:"portal"`
				RequiredActions []string `json:"required_actions"`
			} `json:"data"`
		}
		if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&payload) != nil {
			t.Fatal("portal resolution failed", response.StatusCode)
		}
		if !wantPortal && (payload.Data.Portal != nil || len(payload.Data.RequiredActions) != 1 || payload.Data.RequiredActions[0] != "ESTABLISH_TENDERPRENEUR_MEMBERSHIP") {
			t.Fatal("portal granted before membership")
		}
		if wantPortal && (payload.Data.Portal == nil || payload.Data.Portal.Key != "TENDERPRENEUR" || payload.Data.Portal.Path != "/tenderpreneur" || len(payload.Data.RequiredActions) != 0) {
			t.Fatal("portal not granted after membership")
		}
	}
	assertTenderpreneurPortal(false)

	issuer, err := authzpersistence.NewInvitationIssuer(control)
	if err != nil {
		t.Fatal(err)
	}
	delivery := &liveInvitationDelivery{}
	issueService, err := authzapp.NewInvitationIssueService(issuer, delivery, authzapp.SecureInvitationTokenGenerator{}, invitationAcceptanceIDs{}, invitationAcceptanceClock{now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	issued, err := issueService.Issue(context.Background(), authzapp.InvitationIssueInput{ActorID: authzapp.ID(managerID), TenantID: authzapp.ID(tenantID), CorrelationID: authzapp.ID(uuid.New()), ContextGeneration: 2, Email: inviteeEmail, Role: authzapp.BidManager})
	if err != nil || issued.InvitationID.IsZero() || len(delivery.tokens) != 1 {
		t.Fatal("approved invitation issuance failed", err)
	}
	committer, err := authzpersistence.NewInvitationAcceptanceCommitter(control)
	if err != nil {
		t.Fatal(err)
	}
	acceptService, err := authzapp.NewInvitationAcceptanceService(committer, invitationAcceptanceIDs{}, invitationAcceptanceClock{now.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := acceptService.Accept(context.Background(), authzapp.InvitationAcceptanceInput{Token: delivery.tokens[0].RevealForDelivery(), UserID: authzapp.ID(inviteeID), CorrelationID: authzapp.ID(uuid.New())})
	if err != nil || accepted.MembershipID.IsZero() || accepted.TenantID != authzapp.ID(tenantID) {
		t.Fatal("approved invitation acceptance failed", err)
	}
	assertTenderpreneurPortal(true)
	if count := rowCount(t, control, `SELECT count(*) FROM tenancy.memberships WHERE id=$1 AND user_id=$2 AND tenant_id=$3 AND state='ACTIVE'`, uuid.UUID(accepted.MembershipID), inviteeID, tenantID); count != 1 {
		t.Fatal("accepted active membership missing", count)
	}
	if tenantCountAfter := rowCount(t, control, `SELECT count(*) FROM bidora_tenancy.tenants`); tenantCountAfter != tenantCountBefore {
		t.Fatal("membership journey created an organization", tenantCountBefore, tenantCountAfter)
	}
}
