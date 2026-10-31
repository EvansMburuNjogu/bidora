package persistence

import (
	"context"
	"errors"
	"time"

	entitlementapp "bidora/backend/internal/entitlementcheckpoint/application"
	entitlementpersistence "bidora/backend/internal/entitlementcheckpoint/persistence"
	entitlementsql "bidora/backend/internal/entitlementcheckpoint/persistence/sqlc"
	opportunityapp "bidora/backend/internal/opportunity/application"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type DocumentCompletionStore struct{ runner workflowRunner }

func NewDocumentCompletionStore(runner workflowRunner) (*DocumentCompletionStore, error) {
	if runner == nil {
		return nil, errors.New("document_completion_store_invalid")
	}
	return &DocumentCompletionStore{runner: runner}, nil
}

func (s *DocumentCompletionStore) RecoverExpiredDocumentCompletion(ctx context.Context, outcomeID opportunityapp.ID) (bool, error) {
	if s == nil || s.runner == nil || ctx == nil || outcomeID.IsZero() {
		return false, opportunityapp.ErrDocumentAIAccountingInvalid
	}
	recovered := false
	err := s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		_, err := entitlementsql.New(db).RecoverExpiredAIEntitlementCompletion(txCtx, dbID(outcomeID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		recovered = true
		return nil
	})
	return recovered, err
}

func (s *DocumentCompletionStore) ClaimDocumentCompletion(ctx context.Context, leaseID opportunityapp.ID) (claim opportunityapp.DocumentCompletionClaim, found bool, err error) {
	if s == nil || s.runner == nil || ctx == nil || leaseID.IsZero() {
		return claim, false, opportunityapp.ErrDocumentAIAccountingInvalid
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, claimErr := entitlementsql.New(db).ClaimPendingAIEntitlementCompletion(txCtx, dbID(leaseID))
		if errors.Is(claimErr, pgx.ErrNoRows) {
			return nil
		}
		if claimErr != nil {
			return claimErr
		}
		if !row.LeaseID.Valid || !row.LeaseExpiresAt.Valid || row.Attempts < 1 || row.Attempts > opportunityapp.DocumentCompletionMaxAttempts {
			return opportunityapp.ErrDocumentAIAccountingConflict
		}
		actual := int64(0)
		if row.ActualCredits != nil {
			actual = *row.ActualCredits
		}
		checkpoint := entitlementapp.Checkpoint{ID: entitlementapp.ID(row.CheckpointID.Bytes), TenantID: entitlementapp.ID(row.TenantID.Bytes), RequesterUserID: entitlementapp.ID(row.RequesterUserID.Bytes), ReservationID: entitlementapp.ID(row.ReservationID.Bytes), AssignmentGeneration: row.AssignmentGeneration, EstimatedCredits: row.EstimatedCredits, UsageKind: row.UsageKind, State: entitlementapp.State(row.State), Version: row.Version}
		request := entitlementapp.CompletionRequest{ID: entitlementapp.ID(row.ID.Bytes), CheckpointID: checkpoint.ID, ReservationID: checkpoint.ReservationID, ExecutionID: entitlementapp.ID(row.ExecutionID.Bytes), CorrelationID: entitlementapp.ID(row.CorrelationID.Bytes), Operation: entitlementapp.CompletionOperation(row.Operation), ActualCredits: actual, RequestedAt: row.RequestedAt.Time.UTC(), Checkpoint: checkpoint}
		claim = opportunityapp.DocumentCompletionClaim{Request: request, LeaseID: opportunityapp.ID(row.LeaseID.Bytes), LeaseExpires: row.LeaseExpiresAt.Time.UTC(), Attempts: row.Attempts}
		found = true
		return nil
	})
	return claim, found, err
}

func (s *DocumentCompletionStore) RetryDocumentCompletion(ctx context.Context, claim opportunityapp.DocumentCompletionClaim, outcomeID opportunityapp.ID, code string, at, next time.Time) error {
	backoffs := [...]time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour}
	if !validCompletionRetryCode(code) || claim.Attempts < 1 || claim.Attempts >= opportunityapp.DocumentCompletionMaxAttempts || next.Sub(at) != backoffs[claim.Attempts-1] {
		return opportunityapp.ErrDocumentAIAccountingInvalid
	}
	return s.finish(ctx, claim, outcomeID, "TRANSIENT_FAILURE", code, at, next, false)
}

func (s *DocumentCompletionStore) HoldDocumentCompletion(ctx context.Context, claim opportunityapp.DocumentCompletionClaim, outcomeID opportunityapp.ID, code string, at time.Time) error {
	if !validCompletionHoldCode(code) {
		return opportunityapp.ErrDocumentAIAccountingInvalid
	}
	return s.finish(ctx, claim, outcomeID, "HELD", code, at, time.Time{}, true)
}

func (s *DocumentCompletionStore) finish(ctx context.Context, claim opportunityapp.DocumentCompletionClaim, outcomeID opportunityapp.ID, outcome, code string, at, next time.Time, hold bool) error {
	if s == nil || s.runner == nil || ctx == nil || claim.Request.ID.IsZero() || claim.LeaseID.IsZero() || outcomeID.IsZero() || at.IsZero() {
		return opportunityapp.ErrDocumentAIAccountingInvalid
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := entitlementsql.New(db)
		if err := q.InsertAIEntitlementCompletionReconciliationOutcome(txCtx, entitlementsql.InsertAIEntitlementCompletionReconciliationOutcomeParams{ID: dbID(outcomeID), LeaseID: dbID(claim.LeaseID), Outcome: outcome, ResultCode: code, OccurredAt: dbTime(at), NextAttemptAt: dbTime(next)}); err != nil {
			return err
		}
		var affected int64
		var err error
		if hold {
			holdCode := code
			affected, err = q.HoldAIEntitlementCompletionReconciliation(txCtx, entitlementsql.HoldAIEntitlementCompletionReconciliationParams{HoldCode: &holdCode, OccurredAt: dbTime(at), RequestID: dbID(opportunityapp.ID(claim.Request.ID)), LeaseID: dbID(claim.LeaseID)})
		} else {
			affected, err = q.RetryAIEntitlementCompletionReconciliation(txCtx, entitlementsql.RetryAIEntitlementCompletionReconciliationParams{NextAttemptAt: dbTime(next), OccurredAt: dbTime(at), RequestID: dbID(opportunityapp.ID(claim.Request.ID)), LeaseID: dbID(claim.LeaseID)})
		}
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		return nil
	})
}

func (s *DocumentCompletionStore) AcknowledgeDocumentCompletion(ctx context.Context, claim opportunityapp.DocumentCompletionClaim, ids [3]opportunityapp.ID, outcomeID opportunityapp.ID, at time.Time) error {
	if s == nil || s.runner == nil || ctx == nil || claim.Request.ID.IsZero() || claim.LeaseID.IsZero() || outcomeID.IsZero() || at.IsZero() {
		return opportunityapp.ErrDocumentAIAccountingInvalid
	}
	for _, id := range ids {
		if id.IsZero() {
			return opportunityapp.ErrDocumentAIAccountingInvalid
		}
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := entitlementsql.New(db)
		locked, err := q.LockAIEntitlementCompletionReconciliation(txCtx, entitlementsql.LockAIEntitlementCompletionReconciliationParams{RequestID: dbID(opportunityapp.ID(claim.Request.ID)), LeaseID: dbID(claim.LeaseID)})
		if err != nil || locked.Attempts != claim.Attempts {
			return affectedErr(err)
		}
		tx, err := entitlementpersistence.BindTransaction(db)
		if err != nil {
			return err
		}
		_, _, err = entitlementapp.AcknowledgeCompletionWithinTransaction(txCtx, tx, claim.Request, [3]entitlementapp.ID{entitlementapp.ID(ids[0]), entitlementapp.ID(ids[1]), entitlementapp.ID(ids[2])}, at)
		if err != nil {
			return err
		}
		if err = q.InsertAIEntitlementCompletionReconciliationOutcome(txCtx, entitlementsql.InsertAIEntitlementCompletionReconciliationOutcomeParams{ID: dbID(outcomeID), LeaseID: dbID(claim.LeaseID), Outcome: "ACKNOWLEDGED", ResultCode: "COMPLETED", OccurredAt: dbTime(at)}); err != nil {
			return err
		}
		affected, err := q.MarkAIEntitlementCompletionReconciliationAcknowledged(txCtx, entitlementsql.MarkAIEntitlementCompletionReconciliationAcknowledgedParams{OccurredAt: dbTime(at), RequestID: dbID(opportunityapp.ID(claim.Request.ID)), LeaseID: dbID(claim.LeaseID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		return nil
	})
}

func validCompletionRetryCode(code string) bool {
	return code == "TRANSIENT_UNAVAILABLE" || code == "AMBIGUOUS_INTERNAL_EFFECT" || code == "LOCAL_ACK_UNAVAILABLE"
}
func validCompletionHoldCode(code string) bool {
	return code == "INVALID_REQUEST" || code == "ACCOUNTING_CONFLICT" || code == "STALE_ASSIGNMENT" || code == "GENERATION_INTEGRITY_CONFLICT" || code == "RECONCILIATION_EXHAUSTED"
}

var _ opportunityapp.DocumentCompletionStore = (*DocumentCompletionStore)(nil)
