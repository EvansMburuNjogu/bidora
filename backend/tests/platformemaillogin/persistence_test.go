package platformemaillogin_test

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	app "bidora/backend/internal/identity/application"
	persistence "bidora/backend/internal/identity/persistence"
	platform "bidora/backend/internal/platformadmin"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformpolicy "bidora/backend/internal/platformadmin/domain"
	platformstore "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

func id() app.ID     { return app.ID(uuid.New()) }
func hash() [32]byte { var v [32]byte; _, _ = rand.Read(v[:]); return v }
func pool(t *testing.T) *postgres.Pool {
	t.Helper()
	raw := os.Getenv("BIDORA_PLATFORM_LOGIN_TEST_DSN")
	if raw == "" {
		t.Skip("requires an explicitly designated disposable auth verification database")
	}
	u, e := url.Parse(raw)
	if e != nil || u.Path != "/bidora_sandbox_auth_verification" || u.Hostname() != "postgres" || u.User == nil {
		t.Fatal("refusing non-disposable auth verification database")
	}
	pem, e := os.ReadFile(os.Getenv("BIDORA_TEST_POSTGRES_CA_FILE"))
	if e != nil {
		t.Fatal("verification CA unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		t.Fatal("verification CA invalid")
	}
	port, e := strconv.Atoi(u.Port())
	if e != nil {
		t.Fatal("verification port invalid")
	}
	pw, _ := u.User.Password()
	p, e := postgres.New(postgres.Config{Host: u.Hostname(), Port: uint16(port), Database: strings.TrimPrefix(u.Path, "/"), User: u.User.Username(), Password: pw, RootCAs: roots, MaxConnections: 4, ConnectTimeout: 3 * time.Second, OperationTimeout: 5 * time.Second, LockTimeout: time.Second, MaxLifetime: time.Minute, MaxIdleTime: time.Minute, HealthCheckPeriod: time.Second})
	if e != nil {
		t.Fatal("verification pool invalid")
	}
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	return p
}
func TestEmailAdministratorAssurancePersistence(t *testing.T) {
	p := pool(t)
	ctx := context.Background()
	now := time.Now().UTC()
	seed := func(admin bool) app.ID {
		t.Helper()
		user := id()
		kind := "TENDERPRENEUR"
		if admin {
			kind = "PLATFORM_ADMIN"
		}
		e := p.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
			_, e := db.Exec(c, `INSERT INTO identity.users(id,email,email_lookup,email_verified_at,auth_epoch,created_at,updated_at,account_type) VALUES($1,$2,$2,$3,1,$3,$3,$4)`, uuid.UUID(user), uuid.UUID(user).String()+"@auth-test.invalid", now, kind)
			if e != nil || !admin {
				return e
			}
			if _, e = db.Exec(c, `INSERT INTO platform_administration.administrator_profiles(user_id,context_generation,version,created_at,updated_at) VALUES($1,1,1,$2,$2)`, uuid.UUID(user), now); e != nil {
				return e
			}
			_, e = db.Exec(c, `INSERT INTO platform_administration.role_assignments(id,user_id,role_key,state,source,assigned_at,reason,version) VALUES($1,$2,'PLATFORM_OWNER','ACTIVE','BOOTSTRAP',$3,'Disposable auth verification fixture',1)`, uuid.New(), uuid.UUID(user), now)
			return e
		})
		if e != nil {
			t.Fatal("disposable fixture insert failed", e)
		}
		return user
	}
	challenge := func(s *persistence.MFAStore, user app.ID, at time.Time) app.MFALoginCommit {
		t.Helper()
		tx := id()
		if _, e := s.BeginEmailLogin(ctx, user, 1, app.AuthenticationMethodPassword, hash(), hash(), hash(), tx, at); e != nil {
			t.Fatal("challenge issue failed", e)
		}
		return app.MFALoginCommit{TransactionID: tx, UserID: user, SessionID: id(), AuditID: id(), OutboxID: id(), CorrelationID: id(), AuthEpoch: 1, SessionHash: hash(), CSRFHash: hash(), Method: "EMAIL_OTP", PrimaryMethod: "PASSWORD", At: now, IdleExpiresAt: now.Add(30 * time.Minute), AbsoluteExpiresAt: now.Add(12 * time.Hour)}
	}
	check := func(c app.MFALoginCommit, want string) {
		t.Helper()
		var level, method string
		e := p.WithinReadOnlyTransaction(ctx, func(x context.Context, db postgres.DBTX) error {
			return db.QueryRow(x, `SELECT assurance_level,assurance_method FROM identity.sessions WHERE id=$1`, uuid.UUID(c.SessionID)).Scan(&level, &method)
		})
		if e != nil || level != want || method != "EMAIL_OTP" {
			t.Fatal("persisted session assurance mismatch", e)
		}
	}
	t.Run("admin actual persisted assurance and replay denial", func(t *testing.T) {
		s, _ := persistence.NewMFAStore(p)
		s.EnablePlatformEmailLogin()
		c := challenge(s, seed(true), now)
		if e := s.CommitMFALogin(ctx, c); e != nil {
			t.Fatal(e)
		}
		check(c, "AAL2")
		adapter, e := platformstore.NewStore(p)
		if e != nil {
			t.Fatal(e)
		}
		module, e := platform.NewModule(adapter, adapter)
		if e != nil {
			t.Fatal(e)
		}
		facts, e := module.GetAuthority(ctx, platformapp.AuthorityQuery{UserID: platformapp.ID(c.UserID), SessionID: platformapp.ID(c.SessionID), ObservedAt: now})
		if e != nil || !platformpolicy.AdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod) {
			t.Fatal("persisted email session did not resolve Platform authority", e)
		}
		for _, role := range platformpolicy.Roles() {
			caps, _ := platformpolicy.Capabilities(role)
			for _, cap := range caps {
				if !facts.Grants(cap) {
					t.Fatal("Platform Owner capability parity failed")
				}
			}
		}
		c.SessionID = id()
		if e := s.CommitMFALogin(ctx, c); e == nil {
			t.Fatal("replay accepted")
		}
	})
	t.Run("ordinary email sessions stay AAL1", func(t *testing.T) {
		s, _ := persistence.NewMFAStore(p)
		s.EnablePlatformEmailLogin()
		c := challenge(s, seed(false), now)
		if e := s.CommitMFALogin(ctx, c); e != nil {
			t.Fatal(e)
		}
		check(c, "AAL1")
	})
	t.Run("policy disabled denies admin elevation", func(t *testing.T) {
		s, _ := persistence.NewMFAStore(p)
		c := challenge(s, seed(true), now)
		if e := s.CommitMFALogin(ctx, c); e == nil {
			t.Fatal("disabled policy elevated administrator")
		}
	})
	t.Run("expired transaction denied", func(t *testing.T) {
		s, _ := persistence.NewMFAStore(p)
		s.EnablePlatformEmailLogin()
		c := challenge(s, seed(true), now.Add(-6*time.Minute))
		if e := s.CommitMFALogin(ctx, c); e == nil {
			t.Fatal("expired code accepted")
		}
	})
	t.Run("attempt exhausted transaction denied", func(t *testing.T) {
		s, _ := persistence.NewMFAStore(p)
		s.EnablePlatformEmailLogin()
		c := challenge(s, seed(true), now)
		for i := 0; i < 5; i++ {
			if e := s.RecordMFALoginFailure(ctx, c.TransactionID, now); e != nil {
				t.Fatal(e)
			}
		}
		if e := s.CommitMFALogin(ctx, c); e == nil {
			t.Fatal("exhausted code accepted")
		}
	})
	t.Run("epoch change denied", func(t *testing.T) {
		s, _ := persistence.NewMFAStore(p)
		s.EnablePlatformEmailLogin()
		c := challenge(s, seed(true), now)
		if e := p.WithinTransaction(ctx, func(x context.Context, db postgres.DBTX) error {
			_, e := db.Exec(x, `UPDATE identity.users SET auth_epoch=auth_epoch+1,updated_at=$2 WHERE id=$1`, uuid.UUID(c.UserID), now)
			return e
		}); e != nil {
			t.Fatal(e)
		}
		if e := s.CommitMFALogin(ctx, c); e == nil {
			t.Fatal("stale epoch accepted")
		}
	})
	t.Run("concurrent verification commits exactly once", func(t *testing.T) {
		s, _ := persistence.NewMFAStore(p)
		s.EnablePlatformEmailLogin()
		c := challenge(s, seed(true), now)
		var wg sync.WaitGroup
		var successes atomic.Int32
		for i := 0; i < 2; i++ {
			v := c
			v.SessionID = id()
			v.AuditID = id()
			v.OutboxID = id()
			wg.Go(func() {
				if s.CommitMFALogin(ctx, v) == nil {
					successes.Add(1)
				}
			})
		}
		wg.Wait()
		if successes.Load() != 1 {
			t.Fatal("concurrent verification did not converge to one session")
		}
	})
}
