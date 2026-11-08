package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	app "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	q "bidora/backend/internal/commercial/persistence/sqlc"
	platformapp "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type PlanAuthorityTx interface {
	Resolve(context.Context, platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error)
}
type PlanAuthorityFactory func(postgres.DBTX) (PlanAuthorityTx, error)
type PlanStore struct {
	runner    transactionRunner
	authority PlanAuthorityFactory
}

func NewPlanStore(runner transactionRunner, authority PlanAuthorityFactory) (*PlanStore, error) {
	if runner == nil || authority == nil {
		return nil, errors.New("plan_store_invalid")
	}
	return &PlanStore{runner, authority}, nil
}
func (s *PlanStore) WithinPlan(ctx context.Context, fn func(context.Context, app.PlanTx) error) error {
	if s == nil || ctx == nil || fn == nil {
		return errors.New("plan_transaction_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		authority, err := s.authority(db)
		if err != nil {
			return err
		}
		return fn(c, &planTx{q.New(db), authority})
	})
}
func (s *PlanStore) PublishedPlans(ctx context.Context) (out []app.PlanConfig, err error) {
	if s == nil || ctx == nil {
		return nil, app.ErrPlatformPlanUnavailable
	}
	err = s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		var x error
		out, x = (&planTx{q: q.New(db)}).ListPublished(c)
		return x
	})
	if err != nil {
		return nil, app.ErrPlatformPlanUnavailable
	}
	return out, nil
}
func (t *planTx) ListPublished(c context.Context) ([]app.PlanConfig, error) {
	rows, e := t.q.ListPublishedPlatformPlans(c)
	if e != nil {
		return nil, e
	}
	if len(rows) != 3 {
		return nil, app.ErrPlatformPlanUnavailable
	}
	out := make([]app.PlanConfig, 0, 3)
	for _, r := range rows {
		v := app.PlanConfig{Key: domain.PlanKey(r.PlanKey), Version: r.ActiveVersion, HeadVersion: r.HeadVersion, MonthlyMinor: r.MonthlyMinor, AnnualMinor: r.AnnualMinor, AdditionalLicenceMinor: r.AdditionalLicenceMinor, BidWorkspaces: r.BidWorkspaces, IncludedLicences: r.IncludedLicences, StorageBytes: r.StorageBytes, DocumentVersions: r.DocumentVersions, AICredits: r.AiCredits, ConcurrentAIJobs: r.ConcurrentAiJobs, CreatedBy: domain.ID(r.CreatedBy.Bytes)}
		if !v.Valid() {
			return nil, app.ErrPlatformPlanUnavailable
		}
		out = append(out, v)
	}
	return out, nil
}

type planTx struct {
	q         *q.Queries
	authority PlanAuthorityTx
}

func (t *planTx) ResolvePlatformAuthority(c context.Context, in platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error) {
	return t.authority.Resolve(c, in)
}
func planUUID(v domain.ID) pgtype.UUID { return pgtype.UUID{Bytes: v, Valid: !v.IsZero()} }
func planTime(v time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: v.UTC(), Valid: !v.IsZero()}
}
func planTarget(key domain.PlanKey) domain.ID {
	return domain.ID(uuid.NewSHA1(uuid.NameSpaceURL, []byte("bidora:platform-plan:"+string(key))))
}
func (t *planTx) LockPlanHead(c context.Context, key domain.PlanKey) (app.PlanHead, error) {
	v, e := t.q.LockPlatformPlanHead(c, string(key))
	if errors.Is(e, pgx.ErrNoRows) {
		return app.PlanHead{}, app.ErrPlatformPlanUnavailable
	}
	return app.PlanHead{ActiveVersion: v.ActiveVersion, NextVersion: v.NextVersion, HeadVersion: v.HeadVersion}, e
}
func (t *planTx) GetPlanVersion(c context.Context, key domain.PlanKey, version int64) (app.PlanConfig, error) {
	v, e := t.q.GetPlatformPlanVersion(c, q.GetPlatformPlanVersionParams{PlanKey: string(key), Version: version})
	if errors.Is(e, pgx.ErrNoRows) {
		return app.PlanConfig{}, app.ErrPlatformPlanConflict
	}
	if e != nil {
		return app.PlanConfig{}, e
	}
	return app.PlanConfig{Key: domain.PlanKey(v.PlanKey), Version: v.Version, MonthlyMinor: v.MonthlyMinor, AnnualMinor: v.AnnualMinor, AdditionalLicenceMinor: v.AdditionalLicenceMinor, BidWorkspaces: v.BidWorkspaces, IncludedLicences: v.IncludedLicences, StorageBytes: v.StorageBytes, DocumentVersions: v.DocumentVersions, AICredits: v.AiCredits, ConcurrentAIJobs: v.ConcurrentAiJobs, CreatedBy: domain.ID(v.CreatedBy.Bytes)}, nil
}
func (t *planTx) GetPlanCommand(c context.Context, actor domain.ID, op string, key [32]byte) (app.PlanCommandRecord, bool, error) {
	row, e := t.q.GetPlatformPlanCommand(c, q.GetPlatformPlanCommandParams{ActorID: planUUID(actor), Operation: op, KeyHash: key[:]})
	if errors.Is(e, pgx.ErrNoRows) {
		return app.PlanCommandRecord{}, false, nil
	}
	if e != nil {
		return app.PlanCommandRecord{}, false, e
	}
	if len(row.RequestHash) != 32 {
		return app.PlanCommandRecord{}, false, errors.New("plan_command_invalid")
	}
	var hash [32]byte
	copy(hash[:], row.RequestHash)
	return app.PlanCommandRecord{RequestHash: hash, Key: domain.PlanKey(row.PlanKey), Version: row.ResultVersion, HeadVersion: row.ResultHeadVersion}, true, nil
}
func (t *planTx) InsertPlanVersion(c context.Context, v app.PlanConfig, actor domain.ID, at time.Time) error {
	return t.q.InsertPlatformPlanVersion(c, q.InsertPlatformPlanVersionParams{PlanKey: string(v.Key), Version: v.Version, MonthlyMinor: v.MonthlyMinor, AnnualMinor: v.AnnualMinor, AdditionalLicenceMinor: v.AdditionalLicenceMinor, BidWorkspaces: v.BidWorkspaces, IncludedLicences: v.IncludedLicences, StorageBytes: v.StorageBytes, DocumentVersions: v.DocumentVersions, AiCredits: v.AICredits, ConcurrentAiJobs: v.ConcurrentAIJobs, CreatedBy: planUUID(actor), At: planTime(at)})
}
func (t *planTx) AdvancePlanDraft(c context.Context, key domain.PlanKey, head, version int64, at time.Time) (bool, error) {
	n, e := t.q.AdvancePlatformPlanDraft(c, q.AdvancePlatformPlanDraftParams{At: planTime(at), PlanKey: string(key), ExpectedHeadVersion: head, NextVersion: version})
	return n == 1, e
}
func (t *planTx) PublishPlan(c context.Context, key domain.PlanKey, head, version int64, at time.Time) (bool, error) {
	n, e := t.q.PublishPlatformPlanVersion(c, q.PublishPlatformPlanVersionParams{Version: version, At: planTime(at), PlanKey: string(key), ExpectedHeadVersion: head})
	return n == 1, e
}
func (t *planTx) AppendPlanCommand(c context.Context, actor domain.ID, op string, key, hash [32]byte, r app.PlanReceipt, at time.Time) error {
	return t.q.InsertPlatformPlanCommand(c, q.InsertPlatformPlanCommandParams{ActorID: planUUID(actor), Operation: op, KeyHash: key[:], RequestHash: hash[:], PlanKey: string(r.Key), ResultVersion: r.Version, ResultHeadVersion: r.HeadVersion, At: planTime(at)})
}
func (t *planTx) AppendPlanRead(c context.Context, audit, actor, correlation domain.ID, at time.Time) error {
	return t.q.InsertPlatformPlanManagementRead(c, q.InsertPlatformPlanManagementReadParams{ID: planUUID(audit), ActorID: planUUID(actor), At: planTime(at), CorrelationID: planUUID(correlation)})
}
func (t *planTx) AppendPlanEvidence(c context.Context, audit, outbox, actor, correlation domain.ID, key domain.PlanKey, op string, head int64, at time.Time) error {
	target := planTarget(key)
	summary, _ := json.Marshal(struct {
		PlanKey string `json:"plan_key"`
		Version int64  `json:"head_version"`
	}{string(key), head})
	action, event := "PlatformPlanDraftCreated", "platform.plan.drafted.v1"
	if op == "PUBLISH" {
		action, event = "PlatformPlanPublished", "platform.plan.published.v1"
	}
	if e := t.q.InsertPlatformPlanAudit(c, q.InsertPlatformPlanAuditParams{ID: planUUID(audit), ActorID: planUUID(actor), Action: action, TargetID: planUUID(target), At: planTime(at), CorrelationID: planUUID(correlation), Summary: summary}); e != nil {
		return e
	}
	return t.q.InsertPlatformPlanOutbox(c, q.InsertPlatformPlanOutboxParams{ID: planUUID(outbox), ActorID: planUUID(actor), AggregateID: planUUID(target), Version: head, EventType: event, Payload: summary, At: planTime(at)})
}
