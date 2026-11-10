package platformusers_test

import (
	"bidora/backend/internal/identity/password"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	app "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	invite "bidora/backend/internal/platformadmin/invitation"
	store "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"os"
	"testing"
	"time"
)

// Every fixture and application write is contained in an outer transaction that
// ALWAYS rolls back. No real invite, account, email or provider is used.
type enrollmentRollbackRunner struct{ tx pgx.Tx }

func (r enrollmentRollbackRunner) WithinTransaction(ctx context.Context, fn func(context.Context, postgres.DBTX) error) error {
	nested, e := r.tx.Begin(ctx)
	if e != nil {
		return e
	}
	defer nested.Rollback(context.Background())
	if e = fn(ctx, nested); e != nil {
		return e
	}
	return nested.Commit(ctx)
}
func (r enrollmentRollbackRunner) WithinConsistentReadTransaction(ctx context.Context, fn func(context.Context, postgres.DBTX) error) error {
	return r.WithinTransaction(ctx, fn)
}

type enrollmentClock struct{ now time.Time }

func (c enrollmentClock) Now() time.Time { return c.now }

type enrollmentIDs struct{}

func (enrollmentIDs) NewID() (app.ID, error) { id, e := uuid.NewRandom(); return app.ID(id), e }

type failedEnrollmentTx struct {
	app.InvitationTx
	app.InvitationActivationTx
}

func (*failedEnrollmentTx) AssignInvitedRole(context.Context, app.ID, app.ID, domain.Role, app.ID, string, time.Time) error {
	return errors.New("injected role failure")
}

type failedEnrollmentUOW struct{ inner app.InvitationUnitOfWork }

func (u failedEnrollmentUOW) WithinInvitation(ctx context.Context, fn func(context.Context, app.InvitationTx) error) error {
	return u.inner.WithinInvitation(ctx, func(ctx context.Context, tx app.InvitationTx) error {
		return fn(ctx, &failedEnrollmentTx{tx, tx.(app.InvitationActivationTx)})
	})
}
func enrollmentCheck(t *testing.T, err error, step string) {
	t.Helper()
	if err == nil {
		return
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		t.Fatalf("%s failed: SQLSTATE %s (%s)", step, pg.Code, pg.Message)
	}
	t.Fatalf("%s failed (private details suppressed)", step)
}
func TestLocalInvitationActivationPostgresRollbackOnly(t *testing.T) {
	if os.Getenv("BIDORA_TEST_INVITATION_ACTIVATION_ROLLBACK") != "1" {
		t.Skip("explicit local rollback-only verification opt-in required")
	}
	config, e := pgx.ParseConfig(os.Getenv("BIDORA_TEST_CONTROL_RUNTIME_DSN"))
	enrollmentCheck(t, e, "parse local configuration")
	if config.Host != "127.0.0.1" && config.Host != "localhost" || config.Port != 55432 {
		t.Fatal("refusing non-designated local database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	connection, e := pgx.ConnectConfig(ctx, config)
	enrollmentCheck(t, e, "connect local TLS database")
	defer connection.Close(context.Background())
	hasher, _ := password.New(password.PilotPolicy())
	oldPHC, e := hasher.Hash([]byte("old fixture password only"))
	enrollmentCheck(t, e, "hash fixture password")
	for _, name := range []string{"new owner", "unfinished password account", "established account denied", "role failure rolls back", "google-linked account denied", "disabled account denied"} {
		t.Run(name, func(t *testing.T) {
			outer, e := connection.Begin(ctx)
			enrollmentCheck(t, e, "begin rollback boundary")
			defer outer.Rollback(context.Background())
			_, e = outer.Exec(ctx, "SET LOCAL lock_timeout='2s'")
			enrollmentCheck(t, e, "bound fixture lock wait")
			now := time.Now().UTC().Truncate(time.Microsecond)
			newID := func() pgtype.UUID { return pgtype.UUID{Bytes: [16]byte(uuid.New()), Valid: true} }
			inviter, recipient, invitation := newID(), newID(), newID()
			email := "activation-" + uuid.NewString() + "@example.invalid"
			first, last := "Fixture", "Invitee"
			q := identitysql.New(outer)
			e = q.InsertPasswordUser(ctx, identitysql.InsertPasswordUserParams{ID: inviter, Email: "issuer-" + uuid.NewString() + "@example.invalid", EmailLookup: "issuer-" + uuid.NewString() + "@example.invalid", FirstName: &first, LastName: &last, CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}})
			enrollmentCheck(t, e, "insert rollback-only issuer")
			if name != "new owner" {
				e = q.InsertPasswordUser(ctx, identitysql.InsertPasswordUserParams{ID: recipient, Email: email, EmailLookup: email, FirstName: &first, LastName: &last, CreatedAt: pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true}})
				enrollmentCheck(t, e, "insert pending fixture")
				e = q.InsertPasswordIdentity(ctx, identitysql.InsertPasswordIdentityParams{UserID: recipient, PasswordPhc: oldPHC, CreatedAt: pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true}})
				enrollmentCheck(t, e, "insert old fixture credential")
				_, e = outer.Exec(ctx, "UPDATE identity.users SET email_verified_at=$2,updated_at=$2 WHERE id=$1", recipient, now.Add(-time.Second))
				enrollmentCheck(t, e, "verify fixture")
				if name == "google-linked account denied" {
					enrollmentCheck(t, q.InsertGoogleIdentity(ctx, identitysql.InsertGoogleIdentityParams{ID: newID(), UserID: recipient, ProviderSubject: uuid.NewString(), EmailAtLink: email, CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}}), "link protected fixture")
				}
				if name == "disabled account denied" {
					_, e = outer.Exec(ctx, "UPDATE identity.users SET disabled_at=$2,updated_at=$2 WHERE id=$1", recipient, now)
					enrollmentCheck(t, e, "disable protected fixture")
				}
				if name == "established account denied" {
					_, e = outer.Exec(ctx, "UPDATE identity.users SET account_type='PROCUREMENT',updated_at=$2 WHERE id=$1", recipient, now)
					enrollmentCheck(t, e, "assign protected fixture")
				}
			}
			token, _ := invite.Generate()
			hash, _ := invite.HashPresented(token.RevealForDelivery())
			_, e = outer.Exec(ctx, `INSERT INTO platform_administration.administrator_invitations(id,email,email_lookup,first_name,last_name,role_key,authorization_path,state,token_hash,invited_by,reason,created_at,activated_at,expires_at,version)
   VALUES($1,$2,$2,$3,$4,'PLATFORM_OWNER','OWNER_DIRECT','ACTIVE',$5,$6,'Rollback-only activation verification',$7,$7,$8,1)`, invitation, email, first, last, hash[:], inviter, now, now.Add(72*time.Hour))
			enrollmentCheck(t, e, "insert strong fixture offer")
			persistence, e := store.NewStore(enrollmentRollbackRunner{outer})
			enrollmentCheck(t, e, "compose application store")
			var uow app.InvitationUnitOfWork = persistence
			if name == "role failure rolls back" {
				uow = failedEnrollmentUOW{persistence}
			}
			service, e := app.NewInvitationService(uow, app.SecureInvitationTokenGenerator{}, enrollmentIDs{}, enrollmentClock{now}, &delivery{})
			enrollmentCheck(t, e, "compose activation")
			enrollmentCheck(t, service.EnablePasswordActivation(hasher), "enable password policy")
			input := app.ActivateInvitationInput{PresentedToken: token.RevealForDelivery(), Password: []byte("new fixture password only"), CorrelationID: app.ID(uuid.New()), IdempotencyKey: app.ID(uuid.New())}
			e = service.Activate(ctx, input)
			if name == "established account denied" || name == "google-linked account denied" || name == "disabled account denied" || name == "role failure rolls back" {
				if e == nil {
					t.Fatal("protected/failed transition succeeded")
				}
				var state, kind, phc string
				enrollmentCheck(t, outer.QueryRow(ctx, "SELECT i.state,coalesce(u.account_type,''),p.password_phc FROM platform_administration.administrator_invitations i JOIN identity.users u ON u.email_lookup=i.email_lookup JOIN identity.authentication_identities p ON p.user_id=u.id WHERE i.id=$1", invitation).Scan(&state, &kind, &phc), "read rollback evidence")
				if state != "ACTIVE" || phc != oldPHC || name == "role failure rolls back" && kind != "" {
					t.Fatal("partial password/account/invitation change escaped rollback")
				}
				return
			}
			enrollmentCheck(t, e, "activate fixture")
			var state, kind, phc string
			var verified bool
			var roles, sessions int
			enrollmentCheck(t, outer.QueryRow(ctx, `SELECT i.state,u.account_type,u.email_verified_at IS NOT NULL,p.password_phc,
   (SELECT count(*) FROM platform_administration.role_assignments r WHERE r.user_id=u.id AND r.role_key='PLATFORM_OWNER' AND r.revoked_at IS NULL),
   (SELECT count(*) FROM identity.sessions s WHERE s.user_id=u.id)
   FROM platform_administration.administrator_invitations i JOIN identity.users u ON u.email_lookup=i.email_lookup JOIN identity.authentication_identities p ON p.user_id=u.id WHERE i.id=$1`, invitation).Scan(&state, &kind, &verified, &phc, &roles, &sessions), "read activation evidence")
			match, verifyErr := hasher.Verify([]byte("new fixture password only"), phc)
			if state != "ACCEPTED" || kind != "PLATFORM_ADMIN" || !verified || roles != 1 || sessions != 0 || verifyErr != nil || !match.Match {
				t.Fatal("activation state, password, role or no-login invariant failed")
			}
			enrollmentCheck(t, service.Activate(ctx, input), "replay identical activation")
			input.Password = []byte("changed replay password only")
			if service.Activate(ctx, input) == nil {
				t.Fatal("changed-password replay accepted")
			}
		})
	}
}
