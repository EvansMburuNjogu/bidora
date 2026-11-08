package integration

import (
	"context"
	"errors"
	"time"

	app "bidora/backend/internal/controlplane/application"
	qsql "bidora/backend/internal/controlplane/persistence/sqlc"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformpersistence "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SupportStore struct{ runner platformAuditRunner }

func NewSupportStore(runner platformAuditRunner) (*SupportStore, error) {
	if runner == nil {
		return nil, errors.New("support_store_invalid")
	}
	return &SupportStore{runner}, nil
}
func (s *SupportStore) WithinSupport(ctx context.Context, fn func(context.Context, app.SupportTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("support_tx_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		authority, err := platformpersistence.NewSettingsAuthorityTx(db)
		if err != nil {
			return err
		}
		return fn(c, &supportTx{q: qsql.New(db), authority: authority})
	})
}

type supportTx struct {
	q         *qsql.Queries
	authority *platformpersistence.SettingsAuthorityTx
}

func (t *supportTx) ResolvePlatformAuthority(c context.Context, q platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error) {
	return t.authority.Resolve(c, q)
}
func supportUUID(id app.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }
func supportTime(at time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: at.UTC(), Valid: true}
}
func mapSupportCase(row qsql.BidoraSupportCase) (app.SupportCase, error) {
	if !row.ID.Valid || !row.TenantID.Valid || !row.CreatedBy.Valid || row.ID.Bytes == [16]byte{} || row.TenantID.Bytes == [16]byte{} || row.Version < 1 || !row.CreatedAt.Valid || !row.UpdatedAt.Valid || row.UpdatedAt.Time.Before(row.CreatedAt.Time) || (row.State != "OPEN" && row.State != "RESOLVED") || row.AssignedTo.Valid && row.AssignedTo.Bytes == [16]byte{} {
		return app.SupportCase{}, errors.New("support_row_invalid")
	}
	return app.SupportCase{ID: app.ID(row.ID.Bytes), TenantID: app.ID(row.TenantID.Bytes), AssignedTo: app.ID(row.AssignedTo.Bytes), CreatedBy: app.ID(row.CreatedBy.Bytes), Title: row.Title, Description: row.Description, State: row.State, Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC()}, nil
}
func (t *supportTx) ListCases(c context.Context, limit int32, cursor *app.PlatformAuditCursor) ([]app.SupportCase, error) {
	arg := qsql.ListSupportCasesParams{PageLimit: limit}
	if cursor != nil {
		arg.CursorAt = supportTime(cursor.OccurredAt)
		arg.CursorID = supportUUID(cursor.ID)
	}
	rows, err := t.q.ListSupportCases(c, arg)
	if err != nil {
		return nil, err
	}
	out := make([]app.SupportCase, 0, len(rows))
	for _, row := range rows {
		v, e := mapSupportCase(row)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (t *supportTx) ReadCase(c context.Context, id app.ID, lock bool) (app.SupportCase, bool, error) {
	var row qsql.BidoraSupportCase
	var err error
	if lock {
		row, err = t.q.GetSupportCaseForUpdate(c, supportUUID(id))
	} else {
		row, err = t.q.GetSupportCase(c, supportUUID(id))
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return app.SupportCase{}, false, nil
	}
	if err != nil {
		return app.SupportCase{}, false, err
	}
	v, err := mapSupportCase(row)
	return v, err == nil, err
}
func (t *supportTx) ListMessages(c context.Context, id app.ID) ([]app.SupportMessage, error) {
	rows, err := t.q.ListSupportCaseMessages(c, supportUUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]app.SupportMessage, 0, len(rows))
	for _, row := range rows {
		if !row.ID.Valid || !row.ActorID.Valid || !row.CreatedAt.Valid || row.ID.Bytes == [16]byte{} {
			return nil, errors.New("support_message_invalid")
		}
		out = append(out, app.SupportMessage{ID: app.ID(row.ID.Bytes), ActorID: app.ID(row.ActorID.Bytes), Body: row.Body, CreatedAt: row.CreatedAt.Time.UTC()})
	}
	return out, nil
}
func (t *supportTx) TenantExists(c context.Context, id app.ID) (bool, error) {
	return t.q.SupportTenantExists(c, supportUUID(id))
}
func (t *supportTx) AssigneeEligible(c context.Context, id app.ID) (bool, error) {
	return t.q.SupportAssigneeEligible(c, supportUUID(id))
}
func (t *supportTx) MessageCount(c context.Context, id app.ID) (int64, error) {
	return t.q.CountSupportCaseMessages(c, supportUUID(id))
}
func (t *supportTx) FindCommand(c context.Context, actor app.ID, op string, key [32]byte) (app.SupportCommandRecord, bool, error) {
	row, err := t.q.GetSupportCaseCommand(c, qsql.GetSupportCaseCommandParams{ActorID: supportUUID(actor), Operation: op, KeyHash: key[:]})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.SupportCommandRecord{}, false, nil
	}
	if err != nil {
		return app.SupportCommandRecord{}, false, err
	}
	if len(row.RequestHash) != 32 || !row.CaseID.Valid || row.CaseID.Bytes == [16]byte{} || row.ResultVersion < 1 {
		return app.SupportCommandRecord{}, false, errors.New("support_command_invalid")
	}
	var hash [32]byte
	copy(hash[:], row.RequestHash)
	return app.SupportCommandRecord{RequestHash: hash, CaseID: app.ID(row.CaseID.Bytes), Version: row.ResultVersion}, true, nil
}
func (t *supportTx) InsertCase(c context.Context, v app.SupportCase) error {
	return t.q.InsertSupportCase(c, qsql.InsertSupportCaseParams{ID: supportUUID(v.ID), TenantID: supportUUID(v.TenantID), Title: v.Title, Description: v.Description, CreatedBy: supportUUID(v.CreatedBy), At: supportTime(v.CreatedAt)})
}
func (t *supportTx) UpdateCase(c context.Context, v app.SupportCase, expected int64) (bool, error) {
	var assignee pgtype.UUID
	if !v.AssignedTo.IsZero() {
		assignee = supportUUID(v.AssignedTo)
	}
	count, err := t.q.UpdateSupportCase(c, qsql.UpdateSupportCaseParams{State: v.State, AssignedTo: assignee, At: supportTime(v.UpdatedAt), ID: supportUUID(v.ID), ExpectedVersion: expected})
	return count == 1, err
}
func (t *supportTx) InsertMessage(c context.Context, caseID app.ID, v app.SupportMessage) error {
	return t.q.InsertSupportCaseMessage(c, qsql.InsertSupportCaseMessageParams{ID: supportUUID(v.ID), CaseID: supportUUID(caseID), ActorID: supportUUID(v.ActorID), Body: v.Body, At: supportTime(v.CreatedAt)})
}
func (t *supportTx) AppendEvent(c context.Context, eventID, caseID, actor app.ID, op string, version int64, at time.Time) error {
	return t.q.InsertSupportCaseEvent(c, qsql.InsertSupportCaseEventParams{ID: supportUUID(eventID), CaseID: supportUUID(caseID), ActorID: supportUUID(actor), Action: op, Version: version, At: supportTime(at)})
}
func (t *supportTx) AppendCommand(c context.Context, actor app.ID, op string, key, request [32]byte, caseID app.ID, version int64, at time.Time) error {
	return t.q.InsertSupportCaseCommand(c, qsql.InsertSupportCaseCommandParams{ActorID: supportUUID(actor), Operation: op, KeyHash: key[:], RequestHash: request[:], CaseID: supportUUID(caseID), ResultVersion: version, At: supportTime(at)})
}
func (t *supportTx) AppendAudit(c context.Context, audit, actor, correlation, target app.ID, action string, at time.Time) error {
	targetType := "platform_support_case"
	if action == "PlatformSupportCasesRead" {
		targetType = "platform_support_page"
	}
	return t.q.InsertSupportCaseAudit(c, qsql.InsertSupportCaseAuditParams{ID: supportUUID(audit), ActorID: supportUUID(actor), Action: action, TargetType: targetType, CaseID: supportUUID(target), At: supportTime(at), CorrelationID: supportUUID(correlation), Summary: []byte("{}")})
}
