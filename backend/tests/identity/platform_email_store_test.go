package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	app "bidora/backend/internal/identity/application"
	store "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type emailAssuranceDB struct {
	admin          bool
	primary        string
	err            error
	consumed       bool
	sessionLevel   string
	revokedPrimary bool
	auditLevel     string
}
type emailPolicyRow struct{ db *emailAssuranceDB }

func (r emailPolicyRow) Scan(dest ...any) error {
	if r.db.err != nil {
		return r.db.err
	}
	if len(dest) != 2 {
		return errors.New("unexpected query shape")
	}
	*dest[0].(*bool) = r.db.admin
	*dest[1].(*string) = r.db.primary
	return nil
}
func (d *emailAssuranceDB) QueryRow(_ context.Context, q string, args ...any) pgx.Row {
	return emailPolicyRow{d}
}
func (d *emailAssuranceDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (d *emailAssuranceDB) Exec(_ context.Context, q string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(q, "-- name: ConsumeMFALoginTransaction") && d.consumed {
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
	if strings.Contains(q, "-- name: RevokePreOTPAdministratorSessions") {
		d.revokedPrimary = true
	}
	if strings.Contains(q, "-- name: InsertSession") {
		for _, a := range args {
			if s, ok := a.(string); ok && (s == "AAL1" || s == "AAL2") {
				d.sessionLevel = s
			}
		}
	}
	if strings.Contains(q, "-- name: InsertEmailLoginAudit") {
		for _, a := range args {
			if s, ok := a.(string); ok && (s == "AAL1" || s == "AAL2") {
				d.auditLevel = s
			}
		}
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}
func (d *emailAssuranceDB) WithinTransaction(ctx context.Context, fn func(context.Context, postgres.DBTX) error) error {
	return fn(ctx, d)
}
func TestEmailSessionAssuranceIsChosenByLockedIdentityAndTrustedPolicy(t *testing.T) {
	tests := []struct {
		name                     string
		admin, enabled, consumed bool
		primary                  string
		rowErr                   error
		want                     string
	}{
		{name: "ordinary unchanged", primary: "PASSWORD", want: "AAL1"},
		{name: "ordinary with admin policy unchanged", enabled: true, primary: "PASSWORD", want: "AAL1"},
		{name: "admin policy enabled", admin: true, enabled: true, primary: "PASSWORD", want: "AAL2"},
		{name: "admin policy disabled", admin: true, primary: "PASSWORD"},
		{name: "google is not a password challenge", admin: true, enabled: true, primary: "GOOGLE"},
		{name: "disabled stale or wrong identity", admin: true, enabled: true, primary: "PASSWORD", rowErr: pgx.ErrNoRows},
		{name: "consumed challenge", admin: true, enabled: true, consumed: true, primary: "PASSWORD"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := &emailAssuranceDB{admin: tc.admin, primary: tc.primary, err: tc.rowErr, consumed: tc.consumed}
			s, err := store.NewMFAStore(db)
			if err != nil {
				t.Fatal(err)
			}
			if tc.enabled {
				s.EnablePlatformEmailLogin()
			}
			at := time.Now().UTC()
			err = s.CommitMFALogin(context.Background(), app.MFALoginCommit{UserID: identityID(2), TransactionID: identityID(3), SessionID: identityID(4), AuditID: identityID(5), OutboxID: identityID(6), CorrelationID: identityID(7), AuthEpoch: 1, Method: "EMAIL_OTP", PrimaryMethod: "PASSWORD", At: at, IdleExpiresAt: at.Add(time.Minute), AbsoluteExpiresAt: at.Add(time.Hour)})
			if tc.want == "" {
				if err == nil || db.sessionLevel != "" {
					t.Fatal("invalid administrator challenge established a session")
				}
				return
			}
			if err != nil || db.sessionLevel != tc.want || db.auditLevel != tc.want {
				t.Fatal("session assurance or audit diverged from authoritative identity policy")
			}
			if (tc.want == "AAL2") != db.revokedPrimary {
				t.Fatal("pre-OTP administrator session rotation drifted")
			}
		})
	}
}
