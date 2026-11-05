package persistence

import (
	platformpolicy "bidora/backend/internal/platformadmin/domain"
	"context"
	"encoding/json"
	"errors"
	"time"

	app "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/domain"
	q "bidora/backend/internal/notification/persistence/sqlc"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformdomain "bidora/backend/internal/platformadmin/domain"
	platformowner "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type templateRunner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
	WithinReadOnlyTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type TemplateStore struct{ runner templateRunner }

func NewTemplateStore(r templateRunner) (*TemplateStore, error) {
	if r == nil {
		return nil, app.ErrTemplateInvalid
	}
	return &TemplateStore{r}, nil
}
func (s *TemplateStore) WithinTemplates(ctx context.Context, write bool, fn func(context.Context, app.TemplateTx) error) error {
	if s == nil || ctx == nil || fn == nil {
		return app.ErrTemplateInvalid
	}
	run := s.runner.WithinReadOnlyTransaction
	if write {
		run = s.runner.WithinTransaction
	}
	return run(ctx, func(c context.Context, db postgres.DBTX) error {
		owner, e := platformowner.NewSettingsAuthorityTx(db)
		if e != nil {
			return e
		}
		return fn(c, &templateTx{q.New(db), owner})
	})
}

type templateTx struct {
	q     *q.Queries
	owner *platformowner.SettingsAuthorityTx
}

func (t *templateTx) AuthorizeTemplates(ctx context.Context, s app.TemplateSubject, write bool, at time.Time) error {
	if write {
		if e := t.owner.LockConfigurationWriter(ctx); e != nil {
			return e
		}
	}
	facts, e := t.owner.Resolve(ctx, platformapp.AuthorityQuery{UserID: platformapp.ID(s.ActorID), SessionID: platformapp.ID(s.SessionID), ObservedAt: at})
	if errors.Is(e, platformapp.ErrAuthorityNotFound) {
		return app.ErrTemplateForbidden
	}
	if e != nil {
		return e
	}
	owner := false
	for _, r := range facts.Roles {
		if r == platformdomain.PlatformOwner {
			owner = true
		}
	}
	if facts.ContextGeneration != s.ContextGeneration || !platformpolicy.AdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod) || facts.AssuredAt.IsZero() || facts.AssuredAt.After(at) || facts.AssuredAt.Before(at.Add(-15*time.Minute)) || !(facts.Grants("platform.integration.read") || facts.Grants("platform.announcement.manage")) || write && !owner {
		return app.ErrTemplateForbidden
	}
	return nil
}
func templateRow(v q.NotificationTemplate) app.Template {
	return app.Template{Key: v.TemplateKey, Name: v.Name, Category: v.Category, Channel: v.Channel, Mandatory: v.Mandatory, Enabled: v.Enabled, Subject: v.Subject, Body: v.Body, Version: v.Version, UpdatedAt: v.UpdatedAt.Time}
}
func (t *templateTx) ListTemplates(ctx context.Context) ([]app.Template, error) {
	rows, e := t.q.ListNotificationTemplates(ctx)
	if e != nil {
		return nil, e
	}
	out := make([]app.Template, 0, len(rows))
	for _, v := range rows {
		out = append(out, templateRow(v))
	}
	return out, nil
}
func (t *templateTx) ReadTemplate(ctx context.Context, key string, lock bool) (app.Template, error) {
	if lock {
		v, e := t.q.LockNotificationTemplate(ctx, key)
		return templateRow(v), e
	}
	v, e := t.q.GetNotificationTemplate(ctx, key)
	return templateRow(v), e
}
func (t *templateTx) WriteTemplate(ctx context.Context, v app.Template, s app.TemplateSubject, at time.Time) error {
	when := pgtype.Timestamptz{Time: at, Valid: true}
	actor := pgtype.UUID{Bytes: s.ActorID, Valid: true}
	n, e := t.q.UpdateNotificationTemplate(ctx, q.UpdateNotificationTemplateParams{TemplateKey: v.Key, Name: v.Name, Subject: v.Subject, Body: v.Body, Enabled: v.Enabled, ExpectedVersion: v.Version, At: when, ActorID: actor})
	if e != nil {
		return e
	}
	if n != 1 {
		return app.ErrTemplateConflict
	}
	return t.q.InsertNotificationTemplateVersion(ctx, q.InsertNotificationTemplateVersionParams{TemplateKey: v.Key, Version: v.Version + 1, Name: v.Name, Subject: v.Subject, Body: v.Body, Enabled: v.Enabled, At: when, ActorID: actor})
}
func (t *templateTx) TemplateHistory(ctx context.Context, key string) ([]app.Template, error) {
	rows, e := t.q.ListNotificationTemplateVersions(ctx, key)
	if e != nil {
		return nil, e
	}
	out := make([]app.Template, 0, len(rows))
	for _, v := range rows {
		out = append(out, app.Template{Key: v.TemplateKey, Version: v.Version, Name: v.Name, Subject: v.Subject, Body: v.Body, Enabled: v.Enabled, UpdatedAt: v.CreatedAt.Time, Category: v.Category, Channel: v.Channel, Mandatory: v.Mandatory})
	}
	return out, nil
}
func (t *templateTx) FindTemplateCommand(ctx context.Context, s app.TemplateSubject, operation string, key [32]byte) (app.TemplateReceipt, bool, error) {
	v, e := t.q.GetNotificationTemplateCommand(ctx, q.GetNotificationTemplateCommandParams{ActorID: pgtype.UUID{Bytes: s.ActorID, Valid: true}, Operation: operation, KeyHash: key[:]})
	if errors.Is(e, pgx.ErrNoRows) {
		return app.TemplateReceipt{}, false, nil
	}
	if e != nil {
		return app.TemplateReceipt{}, false, e
	}
	if len(v.RequestHash) != 32 {
		return app.TemplateReceipt{}, false, app.ErrTemplateUnavailable
	}
	var hash [32]byte
	copy(hash[:], v.RequestHash)
	return app.TemplateReceipt{Hash: hash, Key: v.TemplateKey, Version: v.ResultVersion, State: v.State, Result: v.ResultCode}, true, nil
}
func (t *templateTx) RecordTemplateCommand(ctx context.Context, s app.TemplateSubject, operation string, key [32]byte, v app.TemplateReceipt, at time.Time) error {
	return t.q.InsertNotificationTemplateCommand(ctx, q.InsertNotificationTemplateCommandParams{ActorID: pgtype.UUID{Bytes: s.ActorID, Valid: true}, Operation: operation, KeyHash: key[:], RequestHash: v.Hash[:], TemplateKey: v.Key, ResultVersion: v.Version, State: v.State, ResultCode: v.Result, At: pgtype.Timestamptz{Time: at, Valid: true}})
}
func (t *templateTx) FinishTemplateTest(ctx context.Context, s app.TemplateSubject, key [32]byte, result string) error {
	n, e := t.q.CompleteNotificationTemplateTest(ctx, q.CompleteNotificationTemplateTestParams{ActorID: pgtype.UUID{Bytes: s.ActorID, Valid: true}, KeyHash: key[:], ResultCode: result})
	if e != nil {
		return e
	}
	if n != 1 {
		return app.ErrTemplateConflict
	}
	return nil
}
func (t *templateTx) CountTemplateTests(ctx context.Context, s app.TemplateSubject, after time.Time) (int64, error) {
	return t.q.CountRecentNotificationTemplateTests(ctx, q.CountRecentNotificationTemplateTestsParams{ActorID: pgtype.UUID{Bytes: s.ActorID, Valid: true}, After: pgtype.Timestamptz{Time: after, Valid: true}})
}
func (t *templateTx) AppendTemplateEvidence(ctx context.Context, audit, outbox domain.ID, s app.TemplateSubject, v app.Template, action, result string, at time.Time) error {
	actor := pgtype.UUID{Bytes: s.ActorID, Valid: true}
	target := pgtype.UUID{Bytes: googleuuid.NewSHA1(googleuuid.NameSpaceURL, []byte("bidora.notification-template:"+v.Key)), Valid: true}
	when := pgtype.Timestamptz{Time: at, Valid: true}
	changes := v.AuditChanges
	if changes == nil {
		changes = []app.TemplateAuditChange{}
	}
	raw, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	if e := t.q.AppendNotificationTemplateAudit(ctx, q.AppendNotificationTemplateAuditParams{TargetID: target, ID: pgtype.UUID{Bytes: audit, Valid: true}, ActorID: actor, Action: action, At: when, CorrelationID: pgtype.UUID{Bytes: s.CorrelationID, Valid: true}, TemplateKey: v.Key, Version: v.Version, Result: result, Changes: raw}); e != nil {
		return e
	}
	return t.q.AppendNotificationTemplateOutbox(ctx, q.AppendNotificationTemplateOutboxParams{TargetID: target, ID: pgtype.UUID{Bytes: outbox, Valid: true}, ActorID: actor, Action: action, At: when, TemplateKey: v.Key, Version: v.Version})
}
