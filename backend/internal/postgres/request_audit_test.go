package postgres

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type auditContextDB struct {
	calls int
	query string
	args  []any
	err   error
}

func (d *auditContextDB) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	d.calls++
	d.query = query
	d.args = args
	return pgconn.CommandTag{}, d.err
}
func (*auditContextDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (*auditContextDB) QueryRow(context.Context, string, ...any) pgx.Row { return nil }
func TestRequestAuditUsesBoundParametersAndTransactionLocalState(t *testing.T) {
	ctx, err := WithRequestAudit(context.Background(), netip.MustParseAddr("192.0.2.20"), "Firefox")
	if err != nil {
		t.Fatal(err)
	}
	bounded, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	metadata, ok := RequestAuditFrom(bounded)
	if !ok || metadata.Browser != "Firefox" {
		t.Fatal("metadata lost through operation deadline")
	}
	db := &auditContextDB{}
	if err = applyRequestAudit(bounded, db); err != nil {
		t.Fatal(err)
	}
	if db.calls != 1 || len(db.args) != 2 || db.args[0] != "192.0.2.20" || db.args[1] != "Firefox" || strings.Contains(db.query, "192.0.2.20") || strings.Count(db.query, ",true)") != 3 {
		t.Fatal("request metadata is not parameterized and transaction-local")
	}
	db.err = errors.New("synthetic persistence failure")
	if err = applyRequestAudit(bounded, db); err == nil || err.Error() != "postgres_audit_context_failed" {
		t.Fatal("metadata failure was silently ignored")
	}
	fresh := &auditContextDB{}
	if err = applyRequestAudit(context.Background(), fresh); err != nil || fresh.calls != 0 {
		t.Fatal("request context leaked into background work")
	}
}
func TestRequestAuditRejectsUnusableMetadata(t *testing.T) {
	for _, name := range []string{"", "bad\nname", strings.Repeat("x", 41)} {
		if _, err := WithRequestAudit(context.Background(), netip.MustParseAddr("192.0.2.20"), name); err == nil {
			t.Fatal("unsafe browser metadata accepted")
		}
	}
	if _, err := WithRequestAudit(context.Background(), netip.Addr{}, "Firefox"); err == nil {
		t.Fatal("invalid IP accepted")
	}
}
