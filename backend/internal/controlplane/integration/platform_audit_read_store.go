package integration

import (
	"context"
	"errors"
	"time"

	controlapp "bidora/backend/internal/controlplane/application"
	controlsql "bidora/backend/internal/controlplane/persistence/sqlc"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformpersistence "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type platformAuditRunner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type PlatformAuditReadStore struct{ runner platformAuditRunner }

func NewPlatformAuditReadStore(runner platformAuditRunner) (*PlatformAuditReadStore, error) {
	if runner == nil {
		return nil, errors.New("platform_audit_store_invalid")
	}
	return &PlatformAuditReadStore{runner}, nil
}
func (s *PlatformAuditReadStore) WithinPlatformAudit(ctx context.Context, fn func(context.Context, controlapp.PlatformAuditTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("platform_audit_tx_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		authority, err := platformpersistence.NewSettingsAuthorityTx(db)
		if err != nil {
			return err
		}
		return fn(c, &platformAuditTx{q: controlsql.New(db), authority: authority})
	})
}

type platformAuditTx struct {
	q         *controlsql.Queries
	authority *platformpersistence.SettingsAuthorityTx
}

func (t *platformAuditTx) ResolvePlatformAuthority(ctx context.Context, query platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error) {
	return t.authority.Resolve(ctx, query)
}
func optionalString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
func (t *platformAuditTx) ListPlatformAuditActions(ctx context.Context, limit int32, filter controlapp.PlatformAuditFilter, cursor *controlapp.PlatformAuditCursor) ([]controlapp.PlatformAuditAction, error) {
	var at pgtype.Timestamptz
	var cursorID pgtype.UUID
	if cursor != nil {
		at = pgtype.Timestamptz{Time: cursor.OccurredAt.UTC(), Valid: true}
		cursorID = pgtype.UUID{Bytes: cursor.ID, Valid: true}
	}
	var actor pgtype.UUID
	if !filter.Administrator.IsZero() {
		actor = pgtype.UUID{Bytes: filter.Administrator, Valid: true}
	}
	var from, before pgtype.Timestamptz
	if !filter.From.IsZero() {
		from = pgtype.Timestamptz{Time: filter.From.UTC(), Valid: true}
	}
	if !filter.Before.IsZero() {
		before = pgtype.Timestamptz{Time: filter.Before.UTC(), Valid: true}
	}
	rows, err := t.q.ListPlatformAuditActions(ctx, controlsql.ListPlatformAuditActionsParams{ActorFilter: actor, ActionFilter: optionalString(filter.Action), ModuleFilter: optionalString(filter.Module), ResultFilter: optionalString(filter.Result), FromAt: from, BeforeAt: before, SearchTerm: filter.Search, CursorAt: at, CursorID: cursorID, SortOrder: filter.Sort, PageLimit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]controlapp.PlatformAuditAction, 0, len(rows))
	for _, row := range rows {
		item, e := mapPlatformAuditAction(row)
		if e != nil {
			return nil, e
		}
		out = append(out, item)
	}
	return out, nil
}
func (t *platformAuditTx) GetPlatformAuditAction(ctx context.Context, id controlapp.ID) (controlapp.PlatformAuditAction, bool, error) {
	row, err := t.q.GetPlatformAuditAction(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return controlapp.PlatformAuditAction{}, false, nil
	}
	if err != nil {
		return controlapp.PlatformAuditAction{}, false, err
	}
	item, err := mapPlatformAuditAction(row)
	return item, err == nil, err
}
func mapPlatformAuditAction(row controlsql.BidoraControlAuditPlatformActionTimesV1) (controlapp.PlatformAuditAction, error) {
	if !row.ID.Valid || row.ID.Bytes == [16]byte{} || !row.OccurredAt.Valid || row.OccurredAt.Time.IsZero() || !controlapp.AllowedPlatformAuditAction(row.Action) || (row.Module != "ADMINISTRATION" && row.Module != "INTEGRATIONS") || (row.Result != "SUCCESSFUL" && row.Result != "FAILED" && row.Result != "PENDING") {
		return controlapp.PlatformAuditAction{}, errors.New("platform_audit_row_invalid")
	}
	if row.ActorID.Valid && row.ActorID.Bytes == [16]byte{} || row.AffectedRecordID.Valid && row.AffectedRecordID.Bytes == [16]byte{} {
		return controlapp.PlatformAuditAction{}, errors.New("platform_audit_row_invalid")
	}
	return controlapp.PlatformAuditAction{ID: controlapp.ID(row.ID.Bytes), Administrator: controlapp.ID(row.ActorID.Bytes), AffectedRecord: controlapp.ID(row.AffectedRecordID.Bytes), Action: row.Action, Module: row.Module, Result: row.Result, OccurredAt: row.OccurredAt.Time.UTC()}, nil
}
func (t *platformAuditTx) AppendPlatformAuditRead(ctx context.Context, auditID controlapp.ID, subject controlapp.PlatformAuditSubject, at time.Time) error {
	if auditID.IsZero() || subject.UserID.IsZero() || subject.CorrelationID.IsZero() || at.IsZero() {
		return errors.New("platform_audit_read_evidence_invalid")
	}
	return t.q.InsertPlatformAuditPageRead(ctx, controlsql.InsertPlatformAuditPageReadParams{ID: pgtype.UUID{Bytes: auditID, Valid: true}, ActorID: pgtype.UUID{Bytes: subject.UserID, Valid: true}, At: pgtype.Timestamptz{Time: at, Valid: true}, CorrelationID: pgtype.UUID{Bytes: subject.CorrelationID, Valid: true}})
}
