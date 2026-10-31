package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type sessionDB struct {
	err      error
	csrf     []byte
	sql      string
	args     []any
	observed time.Time
}

func (d *sessionDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec")
}
func (d *sessionDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (d *sessionDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	d.sql = sql
	d.args = args
	if d.err != nil {
		return identityRow(func(...any) error { return d.err })
	}
	return identityRow(func(dest ...any) error {
		*(dest[0].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte(identityID(1)), Valid: true}
		*(dest[1].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte(identityID(2)), Valid: true}
		*(dest[2].(*[]byte)) = append([]byte(nil), d.csrf...)
		*(dest[3].(*int64)) = 4
		*(dest[4].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: d.observed.Add(30 * time.Minute), Valid: true}
		*(dest[5].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: d.observed.Add(12 * time.Hour), Valid: true}
		return nil
	})
}

func TestSessionReaderExecutesGuardedRefreshAndMapsRecord(t *testing.T) {
	observed := time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
	csrf := make([]byte, 32)
	csrf[31] = 9
	db := &sessionDB{csrf: csrf, observed: observed}
	runner := &readRunner{db: db}
	reader, err := identitypersistence.NewSessionReader(runner)
	if err != nil {
		t.Fatal(err)
	}
	var tokenHash [32]byte
	tokenHash[31] = 8
	record, err := reader.GetSession(context.Background(), tokenHash, observed)
	if err != nil {
		t.Fatal(err)
	}
	if !runner.committed || record.SessionID != identityID(1) || record.UserID != identityID(2) || record.CSRFHash[31] != 9 || record.AuthEpoch != 4 || !record.IdleExpiresAt.Equal(observed.Add(30*time.Minute)) || !strings.Contains(db.sql, "GetActiveSession") {
		t.Fatal("session refresh mapping drifted", record)
	}
	if got := db.args[0].([]byte); len(got) != 32 || got[31] != 8 {
		t.Fatal("session token hash mapping drifted")
	}
}
func TestSessionReaderUsesCurrentOwnerPolicyAndFailsClosed(t *testing.T) {
	observed := time.Now().UTC()
	db := &sessionDB{csrf: make([]byte, 32), observed: observed}
	runner := &readRunner{db: db}
	reader, _ := identitypersistence.NewSessionReader(runner)
	idle, absolute := int32(300), int32(3600)
	calls := 0
	var dependency error
	if err := reader.EnableSessionPolicy(func(ctx context.Context, scoped postgres.DBTX) (int32, int32, error) {
		calls++
		if scoped != db || ctx == nil {
			t.Fatal("policy owner escaped session transaction")
		}
		return idle, absolute, dependency
	}); err != nil {
		t.Fatal(err)
	}
	var hash [32]byte
	hash[0] = 1
	if _, err := reader.GetSession(context.Background(), hash, observed); err != nil || db.args[2] != int32(300) || db.args[3] != int32(3600) {
		t.Fatal("active policy not bound", err)
	}
	idle = 600
	if _, err := reader.GetSession(context.Background(), hash, observed); err != nil || db.args[2] != int32(600) || calls != 2 {
		t.Fatal("stale policy cached", err)
	}
	db.sql = ""
	dependency = errors.New("synthetic policy unavailable")
	if _, err := reader.GetSession(context.Background(), hash, observed); !errors.Is(err, dependency) || db.sql != "" {
		t.Fatal("policy failure used a fallback", err)
	}
	dependency = nil
	idle = 3600
	if _, err := reader.GetSession(context.Background(), hash, observed); err == nil || db.sql != "" {
		t.Fatal("policy weakened mandatory bounds")
	}
}

func TestSessionToucherUpdatesOnlyAcceptedCurrentSession(t *testing.T) {
	observed := time.Now().UTC()
	db := &loginDB{}
	runner := &writeRunnerFake{db: db}
	toucher, err := identitypersistence.NewSessionToucher(runner)
	if err != nil {
		t.Fatal(err)
	}
	if err := toucher.TouchSession(context.Background(), identityID(3), observed); err != nil {
		t.Fatal(err)
	}
	if !runner.committed || len(db.sql) != 1 || !strings.Contains(db.sql[0], "TouchActiveSession") {
		t.Fatal("session touch query drifted")
	}
	conflictRunner := &writeRunnerFake{db: &loginDB{zeroRowsFor: "TouchActiveSession"}}
	toucher, _ = identitypersistence.NewSessionToucher(conflictRunner)
	if err := toucher.TouchSession(context.Background(), identityID(3), observed); !errors.Is(err, identityapp.ErrSessionNotFound) || conflictRunner.committed {
		t.Fatal("stale session touch accepted", err)
	}
	if toucher, err := identitypersistence.NewSessionToucher(nil); err == nil || toucher != nil {
		t.Fatal("nil touch runner accepted")
	}
	var zero *identitypersistence.SessionToucher
	if err := zero.TouchSession(context.Background(), identityID(3), observed); err == nil {
		t.Fatal("zero toucher accepted")
	}
}

func TestSessionReaderMapsMissingAndDependencyFailures(t *testing.T) {
	var hash [32]byte
	hash[0] = 1
	observed := time.Now().UTC()
	for _, tc := range []struct {
		name     string
		db       postgres.DBTX
		notFound bool
	}{{"missing", &sessionDB{err: pgx.ErrNoRows}, true}, {"database", &sessionDB{err: errors.New("secret-canary db")}, false}, {"csrf", &sessionDB{csrf: []byte{1}, observed: observed}, false}, {"nil db", nil, false}} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &readRunner{db: tc.db}
			reader, _ := identitypersistence.NewSessionReader(runner)
			record, err := reader.GetSession(context.Background(), hash, observed)
			if err == nil || record != (identityapp.SessionRecord{}) || runner.committed || errors.Is(err, identityapp.ErrSessionNotFound) != tc.notFound {
				t.Fatal("session dependency mapping unsafe", record, err)
			}
		})
	}
	if reader, err := identitypersistence.NewSessionReader(nil); err == nil || reader != nil {
		t.Fatal("nil runner accepted")
	}
	runner := &readRunner{db: &sessionDB{}}
	reader, _ := identitypersistence.NewSessionReader(runner)
	if _, err := reader.GetSession(nil, hash, observed); err == nil || runner.calls != 0 {
		t.Fatal("nil context crossed transaction")
	}
	if _, err := reader.GetSession(context.Background(), [32]byte{}, observed); err == nil {
		t.Fatal("zero token hash accepted")
	}
	var zero *identitypersistence.SessionReader
	if _, err := zero.GetSession(context.Background(), hash, observed); err == nil {
		t.Fatal("zero reader accepted")
	}
}
