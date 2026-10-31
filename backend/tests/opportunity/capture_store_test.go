package opportunity_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/opportunity/application"
	"bidora/backend/internal/opportunity/persistence"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type rowFunc func(...any) error

func (f rowFunc) Scan(dest ...any) error { return f(dest...) }

type captureDB struct {
	replay                  bool
	badRows                 string
	sql                     []string
	args                    [][]any
	opportunityID, sourceID application.ID
	created                 time.Time
	requestHash             [32]byte
}

func (d *captureDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	d.sql = append(d.sql, sql)
	d.args = append(d.args, args)
	if d.badRows != "" && strings.Contains(sql, d.badRows) {
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
	if strings.HasPrefix(strings.TrimSpace(sql), "UPDATE") {
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (d *captureDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (d *captureDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	d.sql = append(d.sql, sql)
	d.args = append(d.args, args)
	if strings.Contains(sql, "ClaimCaptureIdempotency") {
		if d.replay {
			return rowFunc(func(...any) error { return pgx.ErrNoRows })
		}
		return rowFunc(func(dest ...any) error { *(dest[0].(*pgtype.UUID)) = args[0].(pgtype.UUID); return nil })
	}
	if strings.Contains(sql, "GetCaptureIdempotencyForUpdate") {
		return rowFunc(func(dest ...any) error {
			status := int32(201)
			version := int64(1)
			*(dest[0].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte(appID(1)), Valid: true}
			*(dest[1].(*[]byte)) = append([]byte(nil), d.requestHash[:]...)
			*(dest[2].(*string)) = "COMPLETED"
			*(dest[3].(**int32)) = &status
			*(dest[4].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte(d.opportunityID), Valid: true}
			*(dest[5].(**int64)) = &version
			*(dest[6].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: d.created, Valid: true}
			*(dest[7].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: d.created, Valid: true}
			*(dest[8].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte(d.sourceID), Valid: true}
			*(dest[9].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: d.created, Valid: true}
			return nil
		})
	}
	return rowFunc(func(...any) error { return errors.New("unexpected query row") })
}

type captureRunner struct {
	db        postgres.DBTX
	calls     int
	committed bool
}

func (r *captureRunner) WithinTransaction(ctx context.Context, fn func(context.Context, postgres.DBTX) error) error {
	r.calls++
	err := fn(ctx, r.db)
	r.committed = err == nil
	return err
}

func TestCaptureStoreExecutesGeneratedQueriesWithoutRawAuditContent(t *testing.T) {
	db := &captureDB{}
	runner := &captureRunner{db: db}
	store, err := persistence.NewCaptureStore(runner, appID(21), 3)
	if err != nil {
		t.Fatal(err)
	}
	command := captureCommand(t, false)
	hash, _ := command.Fingerprint()
	txExecutor, auth, docs, ids, _ := executor(t, &txFake{})
	txExecutor.UOW = store
	input := executeInput(t, false)
	input.Command = command
	result, err := txExecutor.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !runner.committed || runner.calls != 1 || result.OpportunityID != appID(2) || auth.calls != 1 || len(docs.refs) != 0 || ids.calls != 5 {
		t.Fatal("capture store did not commit expected operation")
	}
	want := []string{"ClaimCaptureIdempotency", "InsertCapturedOpportunity", "InsertUnverifiedSourceVersion", "SetOpportunityCurrentSource", "InsertCaptureAudit", "InsertCaptureOutbox", "CompleteCaptureIdempotency"}
	if len(db.sql) != len(want) {
		t.Fatal("unexpected SQL count", len(db.sql))
	}
	for i, name := range want {
		if !strings.Contains(db.sql[i], name) {
			t.Fatalf("SQL order %d: want %s", i, name)
		}
	}
	if got := db.args[2][9].([]byte); string(got) != string(hash[:]) {
		t.Fatal("source fingerprint changed")
	}
	for _, index := range []int{4, 5} {
		for _, arg := range db.args[index] {
			switch value := arg.(type) {
			case string:
				if strings.Contains(value, "Synthetic") {
					t.Fatal("raw source leaked to evidence")
				}
			case []byte:
				if strings.Contains(string(value), "Synthetic") {
					t.Fatal("raw source leaked to evidence")
				}
			}
		}
	}
	if actor := db.args[5][1].(pgtype.UUID); actor.Bytes != [16]byte(appID(20)) || !actor.Valid {
		t.Fatal("outbox actor provenance missing")
	}
}

func TestCaptureStoreReplayUsesLockedPersistedResult(t *testing.T) {
	command := captureCommand(t, true)
	hash, _ := command.Fingerprint()
	created := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	db := &captureDB{replay: true, opportunityID: appID(50), sourceID: appID(51), created: created, requestHash: hash}
	runner := &captureRunner{db: db}
	store, err := persistence.NewCaptureStore(runner, appID(21), 3)
	if err != nil {
		t.Fatal(err)
	}
	e, _, docs, _, _ := executor(t, &txFake{})
	e.UOW = store
	input := executeInput(t, true)
	input.Command = command
	result, err := e.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replayed || result.OpportunityID != appID(50) || result.SourceVersionID != appID(51) || !result.CreatedAt.Equal(created) || !runner.committed || len(docs.refs) != 1 {
		t.Fatal("invalid replay", result)
	}
	if len(db.sql) != 2 || !strings.Contains(db.sql[1], "FOR UPDATE OF r") {
		t.Fatal("replay did not lock idempotency record")
	}
}

func TestCaptureStoreRejectsLeaseAndAffectedRowConflicts(t *testing.T) {
	if store, err := persistence.NewCaptureStore(nil, appID(1), 1); err == nil || store != nil {
		t.Fatal("nil runner accepted")
	}
	runner := &captureRunner{db: &captureDB{}}
	store, err := persistence.NewCaptureStore(runner, appID(21), 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WithinCapture(context.Background(), application.CaptureAuthority{ActorID: appID(1), TenantID: appID(22), AssignmentGeneration: 3}, func(application.CaptureTx) error { return nil }); err == nil || runner.calls != 0 {
		t.Fatal("foreign tenant lease accepted")
	}
	if err := store.WithinCapture(context.Background(), application.CaptureAuthority{ActorID: appID(1), TenantID: appID(21), AssignmentGeneration: 2}, func(application.CaptureTx) error { return nil }); err == nil || runner.calls != 0 {
		t.Fatal("stale assignment lease accepted")
	}
	for _, query := range []string{"SetOpportunityCurrentSource", "CompleteCaptureIdempotency"} {
		t.Run(query, func(t *testing.T) {
			command := captureCommand(t, false)
			hash, _ := command.Fingerprint()
			db := &captureDB{badRows: query}
			runner := &captureRunner{db: db}
			store, _ := persistence.NewCaptureStore(runner, appID(21), 3)
			e, _, _, _, _ := executor(t, &txFake{})
			e.UOW = store
			input := executeInput(t, false)
			input.Command = command
			_, err := e.Execute(context.Background(), input)
			if err == nil || err.Error() != "capture_persistence_failed" || runner.committed {
				t.Fatal("row conflict committed", err)
			}
			_ = hash
		})
	}
}
