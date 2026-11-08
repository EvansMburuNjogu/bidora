package integration_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/mfa"
	"bidora/backend/internal/identity/mfaredis"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/identity/session"
	platformadmin "bidora/backend/internal/platformadmin"
	platformapp "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	platformpersistence "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// Only the disposable database is eligible. Direct synthetic second-owner
// seeding is test setup, never an accepted invitation/onboarding operation.
type paHTTPClock struct{ now time.Time }

func (c *paHTTPClock) Now() time.Time { return c.now }

type paHTTPWallClock struct{}

func (paHTTPWallClock) Now() time.Time { return time.Now().UTC() }

type paIdentityIDs struct{}

func (paIdentityIDs) NewID() (identityapp.ID, error) { return identityapp.ID(uuid.New()), nil }

type paPlatformIDs struct{}

func (paPlatformIDs) NewID() (platformapp.ID, error) { return platformapp.ID(uuid.New()), nil }

type paMFAScopes struct{ d *attempt.Deriver }

func (s paMFAScopes) DeriveMFAScope(p string, u, target identityapp.ID) ([32]byte, error) {
	v, e := s.d.MFA(p, [16]byte(u), [16]byte(target))
	return [32]byte(v), e
}

type paMFAAttempts struct{ c *mfaredis.Controller }

func (a paMFAAttempts) BeginMFA(ctx context.Context, scope [32]byte) (identityapp.MFAAttemptReservation, error) {
	r, e := a.c.Begin(ctx, scope)
	if errors.Is(e, mfaredis.ErrDenied) {
		return nil, identityapp.ErrMFAAttemptDenied
	}
	if e != nil {
		return nil, e
	}
	return paMFAReservation{r}, nil
}

type paMFAReservation struct{ r mfaredis.Reservation }

func (r paMFAReservation) Finish(ctx context.Context, valid bool) error {
	e := r.r.Finish(ctx, valid)
	if errors.Is(e, mfaredis.ErrDenied) {
		return identityapp.ErrMFAAttemptDenied
	}
	return e
}
func paCode(t *testing.T, encoded string, at time.Time) string {
	t.Helper()
	secret, e := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(encoded)
	if e != nil {
		t.Fatal(e)
	}
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(at.Unix()/30))
	h := hmac.New(sha1.New, secret)
	_, _ = h.Write(b[:])
	sum := h.Sum(nil)
	o := sum[len(sum)-1] & 15
	value := (uint32(sum[o])&127)<<24 | uint32(sum[o+1])<<16 | uint32(sum[o+2])<<8 | uint32(sum[o+3])
	return fmt.Sprintf("%06d", value%1000000)
}
func paUUID(id identityapp.ID) uuid.UUID { return uuid.UUID(id) }
func paSessionID(t *testing.T, pool *postgres.Pool, token session.SessionToken) identityapp.ID {
	t.Helper()
	var id uuid.UUID
	h := token.Hash()
	e := pool.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		return db.QueryRow(ctx, `SELECT id FROM identity.sessions WHERE token_hash=$1`, h[:]).Scan(&id)
	})
	if e != nil {
		t.Fatal(e)
	}
	return identityapp.ID(id)
}

type paBrowserActor struct {
	id                identityapp.ID
	login             identityapp.LoginResult
	contextGeneration int64
}

type paPortalAdapter struct {
	platform *platformadmin.Module
	mfa      *identitypersistence.MFAStore
}

func (a paPortalAdapter) ResolvePlatformPortal(ctx context.Context, principal identityapp.SessionPrincipal, at time.Time) (identityapp.PlatformPortalAccess, error) {
	facts, err := a.platform.GetAuthority(ctx, platformapp.AuthorityQuery{UserID: platformapp.ID(principal.UserID), SessionID: platformapp.ID(principal.SessionID), ObservedAt: at})
	if err != nil || facts.AuthEpoch != principal.AuthEpoch {
		return identityapp.PlatformPortalAccess{}, identityapp.ErrPlatformAccessForbidden
	}
	enrolled, err := a.mfa.HasActiveMFA(ctx, principal.UserID)
	if err != nil {
		return identityapp.PlatformPortalAccess{}, err
	}
	set := make(map[string]bool)
	for _, role := range facts.Roles {
		capabilities, err := domain.Capabilities(role)
		if err != nil {
			return identityapp.PlatformPortalAccess{}, err
		}
		for _, capability := range capabilities {
			set[capability] = true
		}
	}
	capabilities := make([]string, 0, len(set))
	for capability := range set {
		capabilities = append(capabilities, capability)
	}
	sort.Strings(capabilities)
	return identityapp.PlatformPortalAccess{ContextGeneration: facts.ContextGeneration, Capabilities: capabilities, MFAEnrolled: enrolled, AssuranceLevel: facts.AssuranceLevel, AssuranceMethod: facts.AssuranceMethod, AssuredAt: facts.AssuredAt}, nil
}

func TestLivePlatformCommittedRealTOTPHTTPMakerChecker(t *testing.T) {
	control := livePool(t, "PA05_CONTROL_RUNTIME_DSN")
	observer := livePool(t, "PA05_CONTROL_MIGRATOR_DSN")
	var before int64
	if e := observer.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		return db.QueryRow(ctx, `SELECT count(*) FROM identity.users`).Scan(&before)
	}); e != nil || (before != 0 && before < 3) {
		t.Fatalf("disposable target has unexpected partial fixture: count=%d err=%v", before, e)
	}
	redisFixture := redisIntegration(t)
	prefix := "bidora:test:pa05:" + uuid.NewString()
	defer cleanupPrefix(t, redisFixture, prefix)
	runner, e := attemptredis.NewClient(redisFixture.config)
	if e != nil {
		t.Fatal(e)
	}
	defer runner.Close()
	limiter, e := mfaredis.New(runner, prefix)
	if e != nil {
		t.Fatal(e)
	}
	key := paRandomBytes(t, 32)
	var encryptionKey [32]byte
	copy(encryptionKey[:], key)
	keys, e := mfa.NewKeyRing(1, map[int][32]byte{1: encryptionKey})
	if e != nil {
		t.Fatal(e)
	}
	deriver, e := attempt.NewDeriver(paRandomBytes(t, 32))
	if e != nil {
		t.Fatal(e)
	}
	clock := &paHTTPClock{now: time.Now().UTC().Add(-time.Minute).Truncate(time.Second)}
	mfaStore, e := identitypersistence.NewMFAStore(control)
	if e != nil {
		t.Fatal(e)
	}
	service, e := identityapp.NewMFAService(mfaStore, keys, identityapp.SecureSessionTokenGenerator{}, paIdentityIDs{}, clock, paMFAAttempts{limiter}, paMFAScopes{deriver})
	if e != nil {
		t.Fatal(e)
	}
	committer, e := identitypersistence.NewLoginCommitter(control)
	if e != nil {
		t.Fatal(e)
	}
	newActor := func(unassigned bool) (paBrowserActor, string) {
		t.Helper()
		id := identityapp.ID(uuid.New())
		created := clock.now.Add(-time.Minute)
		email := "pa05-" + uuid.NewString() + "@example.invalid"
		e := control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
			var account any = "PLATFORM_ADMIN"
			if unassigned {
				account = nil
			}
			_, err := db.Exec(ctx, `INSERT INTO identity.users(id,email,email_lookup,email_verified_at,account_type,created_at,updated_at) VALUES($1,$2,$2,$3,$4,$3,$3)`, paUUID(id), email, created, account)
			return err
		})
		if e != nil {
			t.Fatal(e)
		}
		pair, e := session.GeneratePair()
		if e != nil {
			t.Fatal(e)
		}
		sessionID := identityapp.ID(uuid.New())
		e = committer.CommitLoginSession(context.Background(), identityapp.SessionCommit{SessionID: sessionID, AuditID: identityapp.ID(uuid.New()), UserID: id, CorrelationID: identityapp.ID(uuid.New()), SessionHash: pair.Session.Hash(), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, AuthenticationMethod: identityapp.AuthenticationMethodPassword, AssuranceLevel: "AAL1", AssuranceMethod: "PRIMARY_ONLY", CreatedAt: clock.now, IdleExpiresAt: clock.now.Add(30 * time.Minute), AbsoluteExpiresAt: clock.now.Add(12 * time.Hour), AssuredAt: clock.now})
		if e != nil {
			t.Fatal(e)
		}
		principal := identityapp.SessionPrincipal{SessionID: sessionID, UserID: id, AuthEpoch: 1, AssuranceLevel: "AAL1", AssuranceMethod: "PRIMARY_ONLY", AssuredAt: clock.now, AbsoluteExpiresAt: clock.now.Add(12 * time.Hour)}
		enrollment, e := service.Enroll(context.Background(), principal, identityapp.ID(uuid.New()))
		if e != nil {
			t.Fatal(e)
		}
		activation, e := service.Confirm(context.Background(), principal, identityapp.ID(uuid.New()), paCode(t, enrollment.Secret, clock.now))
		if e != nil {
			t.Fatal(e)
		}
		return paBrowserActor{id: id, login: activation.Login, contextGeneration: 1}, enrollment.Secret
	}
	maker, secret := newActor(before == 0)
	checker, _ := newActor(false)
	target, _ := newActor(false)
	store, e := platformpersistence.NewStore(control)
	if e != nil {
		t.Fatal(e)
	}
	module, e := platformadmin.NewModuleWithAdministratorChanges(store, store, store, store, paPlatformIDs{}, paHTTPWallClock{})
	if e != nil {
		t.Fatal(e)
	}
	if before == 0 {
		bootstrap, err := module.Bootstrap(context.Background(), platformapp.BootstrapCommit{OwnerUserID: platformapp.ID(maker.id), AssignmentID: platformapp.ID(uuid.New()), AuditID: platformapp.ID(uuid.New()), OutboxID: platformapp.ID(uuid.New()), CorrelationID: platformapp.ID(uuid.New()), OccurredAt: time.Now().UTC()})
		if err != nil {
			t.Fatal(err)
		}
		clock.now = time.Now().UTC().Truncate(time.Second)
		gate, err := service.BeginLogin(context.Background(), maker.id, bootstrap.AuthEpoch, identityapp.AuthenticationMethodPassword)
		if err != nil || !gate.Required {
			t.Fatal("post-bootstrap TOTP gate failed", err)
		}
		maker.login, err = service.CompleteLogin(context.Background(), gate.Token, paCode(t, secret, clock.now), "TOTP", identityapp.ID(uuid.New()))
		if err != nil {
			t.Fatal(err)
		}
	}
	// The second owner and target are isolated synthetic fixtures, not invitation acceptance.
	e = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		now := time.Now().UTC()
		users := []identityapp.ID{checker.id, target.id}
		if before != 0 {
			users = append(users, maker.id)
		}
		for _, id := range users {
			if _, err := db.Exec(ctx, `INSERT INTO platform_administration.administrator_profiles(user_id,created_at,updated_at) VALUES($1,$2,$2)`, paUUID(id), now); err != nil {
				return err
			}
		}
		if before != 0 {
			if _, err := db.Exec(ctx, `INSERT INTO platform_administration.role_assignments(id,user_id,role_key,state,source,assigned_by,assigned_at,reason) VALUES($1,$2,'PLATFORM_OWNER','ACTIVE','OWNER_CHANGE',$3,$4,'Isolated synthetic maker fixture')`, uuid.New(), paUUID(maker.id), paUUID(checker.id), now); err != nil {
				return err
			}
		}
		if _, err := db.Exec(ctx, `INSERT INTO platform_administration.role_assignments(id,user_id,role_key,state,source,assigned_by,assigned_at,reason) VALUES($1,$2,'PLATFORM_OWNER','ACTIVE','OWNER_CHANGE',$3,$4,'Isolated synthetic checker fixture')`, uuid.New(), paUUID(checker.id), paUUID(maker.id), now); err != nil {
			return err
		}
		_, err := db.Exec(ctx, `UPDATE platform_administration.governance_state SET effective_owner_generation=effective_owner_generation+1,version=version+1,updated_at=$1`, now)
		return err
	})
	if e != nil {
		t.Fatal(e)
	}
	reader, e := identitypersistence.NewSessionReader(control)
	if e != nil {
		t.Fatal(e)
	}
	validator, e := identityapp.NewSessionValidator(reader, paHTTPWallClock{})
	if e != nil {
		t.Fatal(e)
	}
	toucher, e := identitypersistence.NewSessionToucher(control)
	if e != nil {
		t.Fatal(e)
	}
	origin := "http://127.0.0.1:43000"
	if os.Getenv("PA05_BROWSER") == "1" {
		reserved, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		origin = "http://" + reserved.Addr().String()
		if err = reserved.Close(); err != nil {
			t.Fatal(err)
		}
	}
	protection, e := browser.NewLoopbackDevelopmentProtection([]string{origin})
	if e != nil {
		t.Fatal(e)
	}
	middleware, e := httpapi.NewBrowserSessionMiddleware(validator, toucher, protection, paHTTPWallClock{})
	if e != nil {
		t.Fatal(e)
	}
	handler, e := httpapi.NewPlatformAdministrationHandler(module, paHTTPWallClock{})
	if e != nil {
		t.Fatal(e)
	}
	app := fiber.New(fiber.Config{BodyLimit: 4096})
	app.Use(func(c fiber.Ctx) error { c.Set("X-Request-ID", uuid.NewString()); return c.Next() })
	accountStore, e := identitypersistence.NewAccountTypeStore(control)
	if e != nil {
		t.Fatal(e)
	}
	accountService, e := identityapp.NewAccountTypeService(accountStore, accountStore, paIdentityIDs{}, paHTTPWallClock{})
	if e != nil {
		t.Fatal(e)
	}
	if e = accountService.EnablePlatformAccess(paPortalAdapter{platform: module, mfa: mfaStore}); e != nil {
		t.Fatal(e)
	}
	accountHandler, e := httpapi.NewAccountTypeHandler(accountService, protection)
	if e != nil {
		t.Fatal(e)
	}
	bootstrapHandler, e := httpapi.NewSessionBootstrapHandler(protection)
	if e != nil {
		t.Fatal(e)
	}
	app.Get(httpapi.SessionBootstrapPath, middleware.Bootstrap, bootstrapHandler.Handle)
	app.Get(httpapi.MePortalPath, middleware.Handle, accountHandler.Get)
	if e = httpapi.RegisterPlatformDirectoryRoutes(app, middleware, handler); e != nil {
		t.Fatal(e)
	}
	if e = httpapi.RegisterPlatformAdministratorChangeRoutes(app, middleware, handler); e != nil {
		t.Fatal(e)
	}
	call := func(actor paBrowserActor, method, path string, body any, key uuid.UUID, csrf string) (int, map[string]any, http.Header) {
		t.Helper()
		var payload []byte
		if body != nil {
			var err error
			payload, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		request, e := http.NewRequest(method, "http://127.0.0.1:43001"+path, bytes.NewReader(payload))
		if e != nil {
			t.Fatal(e)
		}
		request.Header.Set("Origin", origin)
		request.Header.Set("X-Bidora-Platform-Context-Generation", strconv.FormatInt(actor.contextGeneration, 10))
		request.AddCookie(&http.Cookie{Name: session.CookieName, Value: actor.login.Session.RevealForCookie()})
		request.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: actor.login.CSRF.RevealForBootstrap()})
		if method != "GET" {
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", key.String())
			request.Header.Set("X-CSRF-Token", csrf)
		}
		response, e := app.Test(request)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		data, e := io.ReadAll(io.LimitReader(response.Body, 65536))
		if e != nil {
			t.Fatal(e)
		}
		var parsed map[string]any
		if len(data) > 0 {
			if e = json.Unmarshal(data, &parsed); e != nil {
				t.Fatalf("invalid API JSON status=%d", response.StatusCode)
			}
		}
		return response.StatusCode, parsed, response.Header
	}
	body := map[string]any{"action": "ASSIGN_ROLE", "target_user_id": paUUID(target.id).String(), "role_key": "PLATFORM_OWNER", "expected_user_version": 1, "expected_target_context_generation": 1, "reason": "Synthetic owner proposal"}
	key1 := uuid.New()
	status, change, _ := call(maker, "POST", "/v1/platform/administrator-changes", body, key1, maker.login.CSRF.RevealForBootstrap())
	if status != 201 {
		t.Fatalf("maker create status %d response=%v", status, change)
	}
	data, ok := change["data"].(map[string]any)
	if !ok || data["state"] != "EXECUTED" {
		t.Fatal("direct owner assignment not executed", change)
	}
	changeID, ok := data["id"].(string)
	if !ok {
		t.Fatal("change id missing")
	}
	status, _, headers := call(maker, "POST", "/v1/platform/administrator-changes", body, key1, maker.login.CSRF.RevealForBootstrap())
	if status != 201 || headers.Get("Idempotency-Replayed") != "true" {
		t.Fatal("proposal replay failed", status)
	}
	status, _, _ = call(checker, "POST", "/v1/platform/administrator-changes", body, uuid.New(), "bad-csrf")
	if status != 403 {
		t.Fatal("CSRF denial missing", status)
	}
	var roleCount, auditCount, outboxCount int64
	e = observer.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		return db.QueryRow(ctx, `SELECT (SELECT count(*) FROM platform_administration.role_assignments WHERE user_id=$1 AND role_key='PLATFORM_OWNER' AND state='ACTIVE'),(SELECT count(*) FROM bidora_control_audit.audit_events WHERE target_id=$2),(SELECT count(*) FROM bidora_control_eventing.outbox_events WHERE aggregate_id=$2)`, paUUID(target.id), changeID).Scan(&roleCount, &auditCount, &outboxCount)
	})
	if e != nil || roleCount != 1 || auditCount < 1 || outboxCount < 1 {
		t.Fatalf("committed role/audit/outbox mismatch: role=%d audit=%d outbox=%d err=%v", roleCount, auditCount, outboxCount, e)
	}
	_ = paSessionID(t, control, maker.login.Session)
	if os.Getenv("PA05_BROWSER") == "1" {
		status, bootstrap, _ := call(maker, "GET", httpapi.SessionBootstrapPath, nil, uuid.Nil, "")
		if status != 200 || bootstrap["data"] == nil {
			t.Fatalf("production session bootstrap unavailable: status=%d", status)
		}
		status, me, _ := call(maker, "GET", httpapi.MePortalPath, nil, uuid.Nil, "")
		if status != 200 {
			t.Fatalf("production portal resolution unavailable: status=%d", status)
		}
		meData, ok := me["data"].(map[string]any)
		if !ok || meData["portal"] == nil {
			t.Fatal("platform portal unavailable to maker")
		}
		status, _, _ = call(maker, "GET", "/v1/platform/administrators/"+paUUID(target.id).String(), nil, uuid.Nil, "")
		if status != 200 {
			t.Fatalf("administrator detail unavailable before browser: status=%d", status)
		}
		paRunBrowser(t, app, origin, maker, checker, target)
		var disabled, audits, events int64
		e = observer.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
			return db.QueryRow(ctx, `SELECT (SELECT count(*) FROM identity.users WHERE id=$1 AND disabled_at IS NOT NULL),
			(SELECT count(*) FROM bidora_control_audit.audit_events a JOIN platform_administration.administrator_changes c ON c.id=a.target_id WHERE c.target_user_id=$1 AND c.action='DISABLE_IDENTITY' AND c.state='EXECUTED'),
			(SELECT count(*) FROM bidora_control_eventing.outbox_events o JOIN platform_administration.administrator_changes c ON c.id=o.aggregate_id WHERE c.target_user_id=$1 AND c.action='DISABLE_IDENTITY' AND c.state='EXECUTED')`, paUUID(target.id)).Scan(&disabled, &audits, &events)
		})
		if e != nil || disabled != 1 || audits < 2 || events < 2 {
			t.Fatalf("browser disable lacks committed identity/audit/outbox: disabled=%d audit=%d outbox=%d err=%v", disabled, audits, events, e)
		}
	}
}

func paRunBrowser(t *testing.T, app *fiber.App, origin string, maker, checker, target paBrowserActor) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	apiOrigin := "http://" + listener.Addr().String()
	serverDone := make(chan error, 1)
	go func() { serverDone <- app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true}) }()
	defer func() { _ = app.ShutdownWithTimeout(3 * time.Second); _ = listener.Close(); <-serverDone }()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	web := ""
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(dir, "web", "package.json")
		if _, err = os.Stat(candidate); err == nil {
			web = filepath.Join(dir, "web")
			break
		}
		dir = filepath.Dir(dir)
	}
	if web == "" {
		t.Fatal("web project root unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "tests/platform-admin-browser-journey.mjs")
	cmd.Dir = web
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "PA05_API_ORIGIN=" + apiOrigin, "PA05_WEB_ORIGIN=" + origin,
		"PA05_MAKER_SESSION=" + maker.login.Session.RevealForCookie(), "PA05_MAKER_CSRF=" + maker.login.CSRF.RevealForBootstrap(),
		"PA05_CHECKER_SESSION=" + checker.login.Session.RevealForCookie(), "PA05_CHECKER_CSRF=" + checker.login.CSRF.RevealForBootstrap(), "PA05_TARGET_ID=" + paUUID(target.id).String()}
	if output, err := cmd.CombinedOutput(); err != nil {
		match := regexp.MustCompile(`isolated_browser_stage_failed:([a-z-]+) bootstrap=([0-9]{1,3}) me=([0-9]{1,3}) detail=([0-9]{1,3})`).FindStringSubmatch(string(output))
		if len(match) == 5 {
			t.Fatalf("isolated browser failed at %s (bootstrap=%s me=%s detail=%s)", match[1], match[2], match[3], match[4])
		}
		t.Fatalf("isolated browser process failed (diagnostics withheld): %v", err)
	}
}

func paRandomBytes(t *testing.T, n int) []byte {
	t.Helper()
	value := make([]byte, n)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return value
}
