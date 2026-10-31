package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/identity/verification"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type issuanceDB struct {
	targetErr, auditErr error
	calls               []string
	args                [][]any
}

func (d *issuanceDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	d.calls = append(d.calls, sql)
	d.args = append(d.args, args)
	if strings.Contains(sql, "InsertEmailVerificationIssuedAudit") && d.auditErr != nil {
		return pgconn.CommandTag{}, d.auditErr
	}
	if strings.Contains(sql, "InvalidateEmailVerificationChallenges") {
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (d *issuanceDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (d *issuanceDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	d.calls = append(d.calls, sql)
	d.args = append(d.args, args)
	return identityRow(func(dest ...any) error {
		if d.targetErr != nil {
			return d.targetErr
		}
		*(dest[0].(*string)) = "User@example.test"
		return nil
	})
}
func validIssuanceChallenge() identityapp.EmailVerificationChallenge {
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	var hash [32]byte
	hash[31] = 7
	return identityapp.EmailVerificationChallenge{ChallengeID: identityID(1), UserID: identityID(2), ActorID: identityID(3), AuditID: identityID(4), CorrelationID: identityID(5), TokenHash: hash, CreatedAt: at, ExpiresAt: at.Add(verification.Lifetime)}
}
func TestEmailVerificationIssuerLocksInvalidatesInsertsAndAuditsAtomically(t *testing.T) {
	db := &issuanceDB{}
	runner := &writeRunnerFake{db: db}
	issuer, err := identitypersistence.NewEmailVerificationIssuer(runner)
	if err != nil {
		t.Fatal(err)
	}
	challenge := validIssuanceChallenge()
	result, err := issuer.IssueEmailVerification(context.Background(), challenge)
	if err != nil {
		t.Fatal(err)
	}
	if !runner.committed || result.UserID != challenge.UserID || result.Recipient != "User@example.test" || !result.ExpiresAt.Equal(challenge.ExpiresAt) || len(db.calls) != 4 {
		t.Fatal("issuance persistence drifted", result, len(db.calls))
	}
	for index, name := range []string{"LockEmailVerificationIssuanceTarget", "InvalidateEmailVerificationChallenges", "InsertEmailVerificationChallenge", "InsertEmailVerificationIssuedAudit"} {
		if !strings.Contains(db.calls[index], name) {
			t.Fatal("issuance order drifted", index)
		}
	}
	if len(db.args[3]) != 5 {
		t.Fatal("issuance audit gained unreviewed payload")
	}
}
func TestEmailVerificationIssuerRollsBackTargetAndAuditFailures(t *testing.T) {
	challenge := validIssuanceChallenge()
	cause := errors.New("secret-canary issuance database")
	for _, tc := range []struct {
		name   string
		db     postgres.DBTX
		target bool
	}{{"missing target", &issuanceDB{targetErr: pgx.ErrNoRows}, true}, {"audit", &issuanceDB{auditErr: cause}, false}, {"nil db", nil, false}} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &writeRunnerFake{db: tc.db}
			issuer, _ := identitypersistence.NewEmailVerificationIssuer(runner)
			result, err := issuer.IssueEmailVerification(context.Background(), challenge)
			if err == nil || result != (identityapp.EmailVerificationIssuance{}) || runner.committed || errors.Is(err, identityapp.ErrVerificationTargetUnavailable) != tc.target {
				t.Fatal("issuance failure committed or leaked", result, err)
			}
		})
	}
	if issuer, err := identitypersistence.NewEmailVerificationIssuer(nil); err == nil || issuer != nil {
		t.Fatal("nil runner accepted")
	}
	runner := &writeRunnerFake{db: &issuanceDB{}}
	issuer, _ := identitypersistence.NewEmailVerificationIssuer(runner)
	challenge.ExpiresAt = challenge.CreatedAt.Add(time.Minute)
	if _, err := issuer.IssueEmailVerification(context.Background(), challenge); err == nil || runner.calls != 0 {
		t.Fatal("invalid lifetime crossed transaction")
	}
	var zero *identitypersistence.EmailVerificationIssuer
	if _, err := zero.IssueEmailVerification(context.Background(), validIssuanceChallenge()); err == nil {
		t.Fatal("zero issuer accepted")
	}
}
