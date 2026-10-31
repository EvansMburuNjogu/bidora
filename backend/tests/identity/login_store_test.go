package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/identity/session"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type loginDB struct {
	sql                  []string
	args                 [][]any
	zeroRowsFor, failFor string
}

func (d *loginDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	d.sql = append(d.sql, sql)
	d.args = append(d.args, args)
	if d.failFor != "" && strings.Contains(sql, d.failFor) {
		return pgconn.CommandTag{}, errors.New("secret-canary database diagnostic")
	}
	if d.zeroRowsFor != "" && strings.Contains(sql, d.zeroRowsFor) {
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
	if strings.HasPrefix(strings.TrimSpace(sql), "UPDATE") {
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (d *loginDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (d *loginDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return identityRow(func(...any) error { return errors.New("unexpected query row") })
}

type writeRunnerFake struct {
	db        postgres.DBTX
	calls     int
	committed bool
}

func (r *writeRunnerFake) WithinTransaction(ctx context.Context, fn func(context.Context, postgres.DBTX) error) error {
	r.calls++
	err := fn(ctx, r.db)
	r.committed = err == nil
	return err
}

type hasherFake struct {
	phc      string
	err      error
	password []byte
	calls    int
}

func (h *hasherFake) Hash(value []byte) (string, error) {
	h.calls++
	h.password = append([]byte(nil), value...)
	return h.phc, h.err
}

type persistenceClockFake struct{ at time.Time }

func (c persistenceClockFake) Now() time.Time { return c.at }

func TestPasswordRehasherHashesOutsideAndUpdatesExpectedVersion(t *testing.T) {
	db := &loginDB{}
	runner := &writeRunnerFake{db: db}
	hasher := &hasherFake{phc: "$argon2id$v=19$synthetic"}
	at := time.Date(2026, 10, 18, 10, 0, 0, 0, time.FixedZone("source", 3600))
	service, err := identitypersistence.NewPasswordRehasher(runner, hasher, persistenceClockFake{at})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RehashPassword(context.Background(), identityID(4), 7, []byte("private password")); err != nil {
		t.Fatal(err)
	}
	if !runner.committed || runner.calls != 1 || hasher.calls != 1 || string(hasher.password) != "private password" || len(db.sql) != 1 || !strings.Contains(db.sql[0], "RehashPasswordCredential") {
		t.Fatal("rehash orchestration drifted")
	}
	if db.args[0][0] != "$argon2id$v=19$synthetic" || db.args[0][3] != int64(7) {
		t.Fatal("rehash query mapping drifted")
	}
	for _, arg := range db.args[0] {
		if value, ok := arg.(string); ok && value == "private password" {
			t.Fatal("plaintext password reached database")
		}
	}
}

func TestPasswordRehasherFailsClosedOnHashConflictAndDependencies(t *testing.T) {
	cause := errors.New("secret-canary hash diagnostic")
	for _, tc := range []struct {
		name   string
		hasher *hasherFake
		db     postgres.DBTX
		clock  time.Time
	}{{"hash", &hasherFake{err: cause}, &loginDB{}, time.Now()}, {"empty hash", &hasherFake{}, &loginDB{}, time.Now()}, {"conflict", &hasherFake{phc: "phc"}, &loginDB{zeroRowsFor: "RehashPasswordCredential"}, time.Now()}, {"database", &hasherFake{phc: "phc"}, &loginDB{failFor: "RehashPasswordCredential"}, time.Now()}, {"nil database", &hasherFake{phc: "phc"}, nil, time.Now()}, {"clock", &hasherFake{phc: "phc"}, &loginDB{}, time.Time{}}} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &writeRunnerFake{db: tc.db}
			service, _ := identitypersistence.NewPasswordRehasher(runner, tc.hasher, persistenceClockFake{tc.clock})
			err := service.RehashPassword(context.Background(), identityID(1), 1, []byte("value"))
			if err == nil || runner.committed {
				t.Fatal("rehash failure committed", err)
			}
		})
	}
	if service, err := identitypersistence.NewPasswordRehasher(nil, &hasherFake{}, persistenceClockFake{}); err == nil || service != nil {
		t.Fatal("nil rehasher dependency accepted")
	}
}

func validSessionCommit(t *testing.T) identityapp.SessionCommit {
	t.Helper()
	pair := sessionPair(t)
	created := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	idle, absolute, _ := session.Expiries(created)
	return identityapp.SessionCommit{SessionID: identityID(1), AuditID: identityID(2), UserID: identityID(3), CorrelationID: identityID(4), SessionHash: pair.Session.Hash(), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 5, AuthenticationMethod: identityapp.AuthenticationMethodPassword, CreatedAt: created, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}
}
func TestLoginCommitterWritesSessionThenMinimalAuditInOneTransaction(t *testing.T) {
	db := &loginDB{}
	runner := &writeRunnerFake{db: db}
	committer, err := identitypersistence.NewLoginCommitter(runner)
	if err != nil {
		t.Fatal(err)
	}
	commit := validSessionCommit(t)
	if err := committer.CommitLoginSession(context.Background(), commit); err != nil {
		t.Fatal(err)
	}
	if !runner.committed || len(db.sql) != 2 || !strings.Contains(db.sql[0], "InsertSession") || !strings.Contains(db.sql[1], "InsertLoginAudit") {
		t.Fatal("login persistence order drifted")
	}
	if db.args[0][2].([]byte)[0] != commit.SessionHash[0] || db.args[0][3].([]byte)[0] != commit.CSRFHash[0] || db.args[0][4] != int64(5) {
		t.Fatal("session persistence mapping drifted")
	}
	if summary := string(db.args[1][5].([]byte)); summary != "{\"auth_epoch\":5,\"authentication_method\":\"PASSWORD\"}" || strings.Contains(summary, "token") || strings.Contains(summary, "email") {
		t.Fatal("login audit leaked private data", summary)
	}
	if id := db.args[1][0].(pgtype.UUID); id.Bytes != [16]byte(identityID(2)) {
		t.Fatal("audit ID drifted")
	}
}

func TestLoginCommitterRejectsInvalidOrPartialCommits(t *testing.T) {
	for _, query := range []string{"InsertSession", "InsertLoginAudit"} {
		t.Run(query, func(t *testing.T) {
			db := &loginDB{failFor: query}
			runner := &writeRunnerFake{db: db}
			committer, _ := identitypersistence.NewLoginCommitter(runner)
			if err := committer.CommitLoginSession(context.Background(), validSessionCommit(t)); err == nil || runner.committed {
				t.Fatal("partial login committed", err)
			}
		})
	}
	base := validSessionCommit(t)
	cases := []identityapp.SessionCommit{func() identityapp.SessionCommit { v := base; v.SessionID = identityapp.ID{}; return v }(), func() identityapp.SessionCommit { v := base; v.AuditID = identityapp.ID{}; return v }(), func() identityapp.SessionCommit { v := base; v.SessionHash = [32]byte{}; return v }(), func() identityapp.SessionCommit {
		v := base
		v.IdleExpiresAt = v.IdleExpiresAt.Add(time.Second)
		return v
	}()}
	for i, commit := range cases {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			runner := &writeRunnerFake{db: &loginDB{}}
			committer, _ := identitypersistence.NewLoginCommitter(runner)
			if err := committer.CommitLoginSession(context.Background(), commit); err == nil || runner.calls != 0 {
				t.Fatal("invalid login commit crossed transaction")
			}
		})
	}
	if committer, err := identitypersistence.NewLoginCommitter(nil); err == nil || committer != nil {
		t.Fatal("nil login runner accepted")
	}
	var zero *identitypersistence.LoginCommitter
	if err := zero.CommitLoginSession(context.Background(), base); err == nil {
		t.Fatal("zero committer accepted")
	}
}
