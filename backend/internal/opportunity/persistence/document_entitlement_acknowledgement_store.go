package persistence

import (
	"context"
	"errors"

	entitlementapp "bidora/backend/internal/entitlementcheckpoint/application"
	entitlementpersistence "bidora/backend/internal/entitlementcheckpoint/persistence"
	opportunityapp "bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type DocumentEntitlementAcknowledgementStore struct{ runner workflowRunner }

func NewDocumentEntitlementAcknowledgementStore(runner workflowRunner) (*DocumentEntitlementAcknowledgementStore, error) {
	if runner == nil {
		return nil, errors.New("document_entitlement_acknowledgement_store_invalid")
	}
	return &DocumentEntitlementAcknowledgementStore{runner: runner}, nil
}

func (s *DocumentEntitlementAcknowledgementStore) WithinDocumentEntitlementAcknowledgement(ctx context.Context, fn func(context.Context, opportunityapp.DocumentEntitlementAcknowledgementTx) error) error {
	if s == nil || s.runner == nil || ctx == nil || fn == nil {
		return errors.New("document_entitlement_acknowledgement_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		entitlementTx, err := entitlementpersistence.BindTransaction(db)
		if err != nil {
			return err
		}
		return fn(txCtx, &documentEntitlementAcknowledgementTx{Tx: entitlementTx, queries: capturesql.New(db)})
	})
}

type documentEntitlementAcknowledgementTx struct {
	entitlementapp.Tx
	queries *capturesql.Queries
}

func (t *documentEntitlementAcknowledgementTx) LockDocumentEntitlementTarget(ctx context.Context, checkpointID opportunityapp.ID) (opportunityapp.DocumentEntitlementTarget, bool, error) {
	row, err := t.queries.LockDocumentEntitlementTarget(ctx, dbID(checkpointID))
	if errors.Is(err, pgx.ErrNoRows) {
		return opportunityapp.DocumentEntitlementTarget{}, false, nil
	}
	if err != nil {
		return opportunityapp.DocumentEntitlementTarget{}, false, err
	}
	return opportunityapp.DocumentEntitlementTarget{MeasurementID: appID(row.MeasurementID), JobID: appID(row.JobID), DocumentVersionID: appID(row.DocumentVersionID), RequestedBy: appID(row.RequesterUserID), AssignmentGeneration: row.AssignmentGeneration, JobState: row.State}, true, nil
}

func (t *documentEntitlementAcknowledgementTx) LockResponseDraftEntitlementTarget(ctx context.Context, checkpointID opportunityapp.ID) (opportunityapp.ResponseDraftEntitlementTarget, bool, error) {
	row, err := t.queries.LockResponseDraftEntitlementTarget(ctx, dbID(checkpointID))
	if errors.Is(err, pgx.ErrNoRows) {
		return opportunityapp.ResponseDraftEntitlementTarget{}, false, nil
	}
	if err != nil {
		return opportunityapp.ResponseDraftEntitlementTarget{}, false, err
	}
	return opportunityapp.ResponseDraftEntitlementTarget{RunID: appID(row.RunID), JobID: appID(row.JobID), RequestedBy: appID(row.RequestedBy), AssignmentGeneration: row.AssignmentGeneration, RunState: row.RunState, JobState: row.JobState}, true, nil
}

func (t *documentEntitlementAcknowledgementTx) ApplyResponseDraftEntitlementDisposition(ctx context.Context, commit opportunityapp.ResponseDraftEntitlementDispositionCommit) error {
	if commit.AuditID.IsZero() || commit.OutboxID.IsZero() || commit.Target.RunID.IsZero() || commit.Target.JobID.IsZero() || commit.Target.RequestedBy.IsZero() || commit.CheckpointID.IsZero() || commit.CorrelationID.IsZero() || commit.OccurredAt.IsZero() {
		return opportunityapp.ErrDocumentEntitlementAcknowledgementInvalid
	}
	action, eventType, jobState := "AIResponseDraftEntitlementReserved", "ai.response_draft.entitlement_reserved.v1", "READY_FOR_PROVIDER"
	var affected int64
	var err error
	if commit.Disposition == "RESERVED" && commit.DenialCode == "" {
		affected, err = t.queries.MarkResponseDraftReadyForProvider(ctx, capturesql.MarkResponseDraftReadyForProviderParams{RunID: dbID(commit.Target.RunID), JobID: dbID(commit.Target.JobID)})
	} else if commit.Disposition == "DENIED" && (commit.DenialCode == "LIMIT_EXCEEDED" || commit.DenialCode == "ACCESS_UNAVAILABLE" || commit.DenialCode == "SUBSCRIPTION_UNAVAILABLE") {
		action, eventType, jobState = "AIResponseDraftEntitlementDenied", "ai.response_draft.entitlement_denied.v1", "FAILED"
		affected, err = t.queries.MarkResponseDraftEntitlementDenied(ctx, capturesql.MarkResponseDraftEntitlementDeniedParams{CompletedAt: dbTime(commit.OccurredAt), RunID: dbID(commit.Target.RunID), JobID: dbID(commit.Target.JobID)})
	} else {
		return opportunityapp.ErrDocumentEntitlementAcknowledgementInvalid
	}
	if err != nil {
		return err
	}
	if affected != 1 {
		return opportunityapp.ErrDocumentEntitlementAcknowledgementConflict
	}
	if err = t.queries.InsertResponseDraftEntitlementDispositionAudit(ctx, capturesql.InsertResponseDraftEntitlementDispositionAuditParams{ID: dbID(commit.AuditID), ActorID: dbID(commit.Target.RequestedBy), Action: action, RunID: dbID(commit.Target.RunID), OccurredAt: dbTime(commit.OccurredAt), CorrelationID: dbID(commit.CorrelationID), CheckpointID: dbID(commit.CheckpointID), JobID: dbID(commit.Target.JobID), JobState: jobState, Disposition: commit.Disposition, DenialCode: commit.DenialCode}); err != nil {
		return err
	}
	return t.queries.InsertResponseDraftEntitlementDispositionOutbox(ctx, capturesql.InsertResponseDraftEntitlementDispositionOutboxParams{ID: dbID(commit.OutboxID), ActorID: dbID(commit.Target.RequestedBy), RunID: dbID(commit.Target.RunID), EventType: eventType, CheckpointID: dbID(commit.CheckpointID), JobID: dbID(commit.Target.JobID), JobState: jobState, Disposition: commit.Disposition, DenialCode: commit.DenialCode, OccurredAt: dbTime(commit.OccurredAt)})
}

func (t *documentEntitlementAcknowledgementTx) LockProposalDraftEntitlementTarget(ctx context.Context, checkpointID opportunityapp.ID) (opportunityapp.ResponseDraftEntitlementTarget, bool, error) {
	row, err := t.queries.LockProposalDraftEntitlementTarget(ctx, dbID(checkpointID))
	if errors.Is(err, pgx.ErrNoRows) {
		return opportunityapp.ResponseDraftEntitlementTarget{}, false, nil
	}
	if err != nil {
		return opportunityapp.ResponseDraftEntitlementTarget{}, false, err
	}
	return opportunityapp.ResponseDraftEntitlementTarget{RunID: appID(row.RunID), JobID: appID(row.JobID), RequestedBy: appID(row.RequestedBy), AssignmentGeneration: row.AssignmentGeneration, RunState: row.RunState, JobState: row.JobState}, true, nil
}
func (t *documentEntitlementAcknowledgementTx) ApplyProposalDraftEntitlementDisposition(ctx context.Context, c opportunityapp.ResponseDraftEntitlementDispositionCommit) error {
	if c.AuditID.IsZero() || c.OutboxID.IsZero() || c.Target.RunID.IsZero() || c.Target.JobID.IsZero() || c.Target.RequestedBy.IsZero() || c.CheckpointID.IsZero() || c.CorrelationID.IsZero() || c.OccurredAt.IsZero() {
		return opportunityapp.ErrDocumentEntitlementAcknowledgementInvalid
	}
	action, event, state := "AIProposalDraftEntitlementReserved", "ai.proposal_draft.entitlement_reserved.v1", "READY_FOR_PROVIDER"
	var n int64
	var err error
	if c.Disposition == "RESERVED" && c.DenialCode == "" {
		n, err = t.queries.MarkProposalDraftReadyForProvider(ctx, capturesql.MarkProposalDraftReadyForProviderParams{RunID: dbID(c.Target.RunID), JobID: dbID(c.Target.JobID)})
	} else if c.Disposition == "DENIED" && (c.DenialCode == "LIMIT_EXCEEDED" || c.DenialCode == "ACCESS_UNAVAILABLE" || c.DenialCode == "SUBSCRIPTION_UNAVAILABLE") {
		action, event, state = "AIProposalDraftEntitlementDenied", "ai.proposal_draft.entitlement_denied.v1", "FAILED"
		n, err = t.queries.MarkProposalDraftEntitlementDenied(ctx, capturesql.MarkProposalDraftEntitlementDeniedParams{CompletedAt: dbTime(c.OccurredAt), RunID: dbID(c.Target.RunID), JobID: dbID(c.Target.JobID)})
	} else {
		return opportunityapp.ErrDocumentEntitlementAcknowledgementInvalid
	}
	if err != nil {
		return err
	}
	if n != 1 {
		return opportunityapp.ErrDocumentEntitlementAcknowledgementConflict
	}
	if err = t.queries.InsertProposalDraftEntitlementDispositionAudit(ctx, capturesql.InsertProposalDraftEntitlementDispositionAuditParams{ID: dbID(c.AuditID), ActorID: dbID(c.Target.RequestedBy), Action: action, RunID: dbID(c.Target.RunID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), CheckpointID: dbID(c.CheckpointID), JobID: dbID(c.Target.JobID), JobState: state, Disposition: c.Disposition, DenialCode: c.DenialCode}); err != nil {
		return err
	}
	return t.queries.InsertProposalDraftEntitlementDispositionOutbox(ctx, capturesql.InsertProposalDraftEntitlementDispositionOutboxParams{ID: dbID(c.OutboxID), ActorID: dbID(c.Target.RequestedBy), RunID: dbID(c.Target.RunID), EventType: event, CheckpointID: dbID(c.CheckpointID), JobID: dbID(c.Target.JobID), JobState: state, Disposition: c.Disposition, DenialCode: c.DenialCode, OccurredAt: dbTime(c.OccurredAt)})
}

func (t *documentEntitlementAcknowledgementTx) ApplyDocumentEntitlementDisposition(ctx context.Context, commit opportunityapp.DocumentEntitlementDispositionCommit) error {
	if commit.AuditID.IsZero() || commit.OutboxID.IsZero() || commit.Target.MeasurementID.IsZero() || commit.Target.JobID.IsZero() || commit.Target.DocumentVersionID.IsZero() || commit.Target.RequestedBy.IsZero() || commit.CheckpointID.IsZero() || commit.CorrelationID.IsZero() || commit.OccurredAt.IsZero() {
		return opportunityapp.ErrDocumentEntitlementAcknowledgementInvalid
	}
	action, eventType, jobState := "DocumentAIEntitlementReserved", "ai.document.entitlement_reserved.v1", "READY_FOR_PROVIDER"
	var affected int64
	var err error
	if commit.Disposition == "RESERVED" && commit.DenialCode == "" {
		affected, err = t.queries.MarkDocumentAnalysisReadyForProvider(ctx, dbID(commit.Target.JobID))
	} else if commit.Disposition == "DENIED" && (commit.DenialCode == "LIMIT_EXCEEDED" || commit.DenialCode == "ACCESS_UNAVAILABLE" || commit.DenialCode == "SUBSCRIPTION_UNAVAILABLE") {
		action, eventType, jobState = "DocumentAIEntitlementDenied", "ai.document.entitlement_denied.v1", "FAILED"
		affected, err = t.queries.MarkDocumentAnalysisEntitlementDenied(ctx, capturesql.MarkDocumentAnalysisEntitlementDeniedParams{CompletedAt: dbTime(commit.OccurredAt), JobID: dbID(commit.Target.JobID)})
	} else {
		return opportunityapp.ErrDocumentEntitlementAcknowledgementInvalid
	}
	if err != nil {
		return err
	}
	if affected != 1 {
		return opportunityapp.ErrDocumentEntitlementAcknowledgementConflict
	}
	if err = t.queries.InsertDocumentEntitlementDispositionAudit(ctx, capturesql.InsertDocumentEntitlementDispositionAuditParams{ID: dbID(commit.AuditID), ActorID: dbID(commit.Target.RequestedBy), Action: action, MeasurementID: dbID(commit.Target.MeasurementID), OccurredAt: dbTime(commit.OccurredAt), CorrelationID: dbID(commit.CorrelationID), CheckpointID: dbID(commit.CheckpointID), JobID: dbID(commit.Target.JobID), JobState: jobState, Disposition: commit.Disposition, DenialCode: commit.DenialCode}); err != nil {
		return err
	}
	return t.queries.InsertDocumentEntitlementDispositionOutbox(ctx, capturesql.InsertDocumentEntitlementDispositionOutboxParams{ID: dbID(commit.OutboxID), ActorID: dbID(commit.Target.RequestedBy), MeasurementID: dbID(commit.Target.MeasurementID), EventType: eventType, DocumentVersionID: dbID(commit.Target.DocumentVersionID), CheckpointID: dbID(commit.CheckpointID), JobID: dbID(commit.Target.JobID), JobState: jobState, Disposition: commit.Disposition, DenialCode: commit.DenialCode, OccurredAt: dbTime(commit.OccurredAt)})
}

var _ opportunityapp.DocumentEntitlementAcknowledgementUnitOfWork = (*DocumentEntitlementAcknowledgementStore)(nil)
