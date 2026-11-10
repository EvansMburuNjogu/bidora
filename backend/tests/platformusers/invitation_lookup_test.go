package platformusers_test

import (
	app "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"strings"
	"testing"
	"time"
)

type invitationLookupDB struct{}

func (invitationLookupDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected mutation")
}
func (invitationLookupDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (invitationLookupDB) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	return invitationLookupRow{sql: sql}
}

type invitationLookupRow struct{ sql string }

func (r invitationLookupRow) Scan(dest ...any) error {
	if !strings.Contains(r.sql, "first_name, last_name") {
		return errors.New("token lookup omitted name snapshots")
	}
	*(dest[0].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	*(dest[1].(*string)) = "saved@example.test"
	*(dest[2].(*string)) = "saved@example.test"
	*(dest[3].(*string)) = "Saved"
	*(dest[4].(*string)) = "Invitee"
	*(dest[5].(*string)) = "PLATFORM_OWNER"
	*(dest[6].(*string)) = "OWNER_DIRECT"
	*(dest[7].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	*(dest[8].(*string)) = "Invite administrator"
	*(dest[9].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: at.Add(-time.Minute), Valid: true}
	*(dest[10].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: at.Add(-time.Minute), Valid: true}
	*(dest[11].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: at.Add(time.Hour), Valid: true}
	*(dest[12].(*int64)) = 1
	return nil
}

type invitationLookupRunner struct{}

func (invitationLookupRunner) WithinTransaction(ctx context.Context, fn func(context.Context, postgres.DBTX) error) error {
	return fn(ctx, invitationLookupDB{})
}
func (r invitationLookupRunner) WithinConsistentReadTransaction(ctx context.Context, fn func(context.Context, postgres.DBTX) error) error {
	return r.WithinTransaction(ctx, fn)
}
func TestInvitationTokenLookupPreservesNamesForPasswordSetup(t *testing.T) {
	store, err := persistence.NewStore(invitationLookupRunner{})
	if err != nil {
		t.Fatal(err)
	}
	err = store.WithinInvitation(context.Background(), func(ctx context.Context, tx app.InvitationTx) error {
		record, err := tx.LockInvitationByToken(ctx, [32]byte{1}, at)
		if err != nil {
			return err
		}
		if record.FirstName != "Saved" || record.LastName != "Invitee" || record.EmailDisplay != "saved@example.test" {
			t.Fatal("token lookup lost invitation identity")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
