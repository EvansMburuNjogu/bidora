package authorization_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	authzapp "bidora/backend/internal/authorization/application"
	authzpersistence "bidora/backend/internal/authorization/persistence"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func id(n byte) authzapp.ID { var value authzapp.ID; value[15] = n; return value }

type grantReaderFake struct {
	grant   authzapp.CaptureGrant
	err     error
	calls   int
	subject authzapp.CaptureSubject
}

func (r *grantReaderFake) FindExternalCaptureGrant(_ context.Context, subject authzapp.CaptureSubject) (authzapp.CaptureGrant, error) {
	r.calls++
	r.subject = subject
	return r.grant, r.err
}

type rowFunc func(...any) error

func (f rowFunc) Scan(values ...any) error { return f(values...) }

type grantDB struct {
	err  error
	sql  string
	args []any
}

func (d *grantDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec")
}
func (d *grantDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (d *grantDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	d.sql = sql
	d.args = args
	if d.err != nil {
		return rowFunc(func(...any) error { return d.err })
	}
	return rowFunc(func(dest ...any) error {
		*(dest[0].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte(id(3)), Valid: true}
		*(dest[1].(*int64)) = 4
		*(dest[2].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte(id(2)), Valid: true}
		*(dest[3].(*int64)) = 7
		return nil
	})
}

type readRunner struct {
	db        postgres.DBTX
	calls     int
	committed bool
}

func (r *readRunner) WithinReadOnlyTransaction(ctx context.Context, fn func(context.Context, postgres.DBTX) error) error {
	r.calls++
	err := fn(ctx, r.db)
	r.committed = err == nil
	return err
}

func TestCaptureAuthorizerReturnsOnlyCurrentExplicitGrant(t *testing.T) {
	subject := authzapp.CaptureSubject{UserID: id(1), TenantID: id(2), ContextGeneration: 4}
	reader := &grantReaderFake{grant: authzapp.CaptureGrant{MembershipID: id(3), TenantID: id(2), ContextGeneration: 4, AssignmentGeneration: 7}}
	service, err := authzapp.NewCaptureAuthorizer(reader)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := service.Authorize(context.Background(), subject)
	if err != nil {
		t.Fatal(err)
	}
	if grant != reader.grant || reader.calls != 1 || reader.subject != subject {
		t.Fatal("capture authorization drifted", grant)
	}
}
func TestCaptureAuthorizerFailsClosedWithoutLeakingDenialState(t *testing.T) {
	valid := authzapp.CaptureSubject{UserID: id(1), TenantID: id(2), ContextGeneration: 4}
	for _, tc := range []struct {
		name    string
		subject authzapp.CaptureSubject
		reader  *grantReaderFake
		code    string
	}{{"missing user", authzapp.CaptureSubject{}, &grantReaderFake{}, "capture_not_authorized"}, {"denied", valid, &grantReaderFake{err: authzapp.ErrNotAuthorized}, "capture_not_authorized"}, {"dependency", valid, &grantReaderFake{err: errors.New("secret-canary authorization db")}, "authorization_unavailable"}, {"wrong tenant", valid, &grantReaderFake{grant: authzapp.CaptureGrant{MembershipID: id(3), TenantID: id(9), ContextGeneration: 4, AssignmentGeneration: 7}}, "authorization_unavailable"}, {"stale result", valid, &grantReaderFake{grant: authzapp.CaptureGrant{MembershipID: id(3), TenantID: id(2), ContextGeneration: 3, AssignmentGeneration: 7}}, "authorization_unavailable"}} {
		t.Run(tc.name, func(t *testing.T) {
			service, _ := authzapp.NewCaptureAuthorizer(tc.reader)
			grant, err := service.Authorize(context.Background(), tc.subject)
			if err == nil || err.Error() != tc.code || grant != (authzapp.CaptureGrant{}) || errors.Is(err, authzapp.ErrNotAuthorized) || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("unsafe authorization result", grant, err)
			}
		})
	}
	if service, err := authzapp.NewCaptureAuthorizer(nil); err == nil || service != nil {
		t.Fatal("nil reader accepted")
	}
	service, _ := authzapp.NewCaptureAuthorizer(&grantReaderFake{})
	if _, err := service.Authorize(nil, valid); err == nil {
		t.Fatal("nil context accepted")
	}
	var zero *authzapp.CaptureAuthorizer
	if _, err := zero.Authorize(context.Background(), valid); err == nil {
		t.Fatal("zero authorizer accepted")
	}
}
func TestCaptureGrantReaderUsesTypedFailClosedQuery(t *testing.T) {
	subject := authzapp.CaptureSubject{UserID: id(1), TenantID: id(2), ContextGeneration: 4}
	db := &grantDB{}
	runner := &readRunner{db: db}
	reader, err := authzpersistence.NewCaptureGrantReader(runner)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := reader.FindExternalCaptureGrant(context.Background(), subject)
	if err != nil {
		t.Fatal(err)
	}
	if !runner.committed || grant.MembershipID != id(3) || grant.TenantID != id(2) || grant.ContextGeneration != 4 || grant.AssignmentGeneration != 7 || !strings.Contains(db.sql, "AuthorizeExternalOpportunityCapture") || len(db.args) != 3 {
		t.Fatal("grant query mapping drifted", grant)
	}
}
func TestCaptureGrantReaderMapsNoRowsAndDependencies(t *testing.T) {
	subject := authzapp.CaptureSubject{UserID: id(1), TenantID: id(2), ContextGeneration: 4}
	for _, tc := range []struct {
		name   string
		db     postgres.DBTX
		denied bool
	}{{"denied", &grantDB{err: pgx.ErrNoRows}, true}, {"database", &grantDB{err: errors.New("secret-canary db")}, false}, {"nil db", nil, false}} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &readRunner{db: tc.db}
			reader, _ := authzpersistence.NewCaptureGrantReader(runner)
			grant, err := reader.FindExternalCaptureGrant(context.Background(), subject)
			if err == nil || grant != (authzapp.CaptureGrant{}) || runner.committed || errors.Is(err, authzapp.ErrNotAuthorized) != tc.denied {
				t.Fatal("grant dependency mapping unsafe", grant, err)
			}
		})
	}
	if reader, err := authzpersistence.NewCaptureGrantReader(nil); err == nil || reader != nil {
		t.Fatal("nil runner accepted")
	}
	runner := &readRunner{db: &grantDB{}}
	reader, _ := authzpersistence.NewCaptureGrantReader(runner)
	if _, err := reader.FindExternalCaptureGrant(nil, subject); !errors.Is(err, authzapp.ErrNotAuthorized) || runner.calls != 0 {
		t.Fatal("invalid subject crossed database")
	}
}
