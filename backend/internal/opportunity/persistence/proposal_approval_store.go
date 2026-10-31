package persistence

import (
	application "bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"bytes"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s *WorkflowStore) FindProposalApprovalReplay(ctx context.Context, a application.WorkflowAuthority, bidID application.ID, key, request [32]byte) (result application.ProposalApprovalResult, found bool, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		row, e := q.GetProposalApprovalIdempotency(tc, capturesql.GetProposalApprovalIdempotencyParams{ActorID: dbID(a.ActorID), KeyHash: key[:]})
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		found = true
		if !bytes.Equal(row.RequestHash, request[:]) {
			return application.ErrProposalApprovalConflict
		}
		if row.State != "COMPLETED" || row.ResponseStatus == nil || *row.ResponseStatus != 201 || !row.ResponseRef.Valid {
			return errors.New("proposal_approval_idempotency_pending")
		}
		v, e := q.GetProposalApproval(tc, row.ResponseRef)
		if e != nil {
			return e
		}
		result = application.ProposalApprovalResult{Approval: mapProposalApproval(v), Replay: true}
		return nil
	})
	return
}
func (s *WorkflowStore) CommitProposalApproval(ctx context.Context, a application.WorkflowAuthority, c application.ProposalApprovalCommit) (result application.ProposalApprovalResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, _, e := claimWorkflow(tc, q, c.CommandID, a.ActorID, "APPROVE_PROPOSAL_PACKAGE", c.KeyHash, c.RequestHash, c.OccurredAt)
		if e != nil {
			return e
		}
		if replay {
			v, e := q.GetProposalApproval(tc, dbID(ref))
			if e != nil {
				return e
			}
			result = application.ProposalApprovalResult{Approval: mapProposalApproval(v), Replay: true}
			return nil
		}
		locked, e := q.LockProposalPackageForApproval(tc, capturesql.LockProposalPackageForApprovalParams{ApproverID: dbID(a.ActorID), ManifestID: dbID(c.ManifestID), BidID: dbID(c.BidID)})
		if errors.Is(e, pgx.ErrNoRows) {
			return application.ErrProposalApprovalConflict
		}
		if e != nil {
			return e
		}
		if locked.ProposalVersion != c.ExpectedProposalVersion || locked.WorkspaceVersion != c.ExpectedWorkspaceVersion || appID(locked.ProposalCreatorID) == a.ActorID || locked.ReviewDecision != "REVIEW_COMPLETE" {
			return application.ErrProposalApprovalConflict
		}
		blockers, e := q.GetProposalApprovalReviewBlockers(tc, capturesql.GetProposalApprovalReviewBlockersParams{BidID: dbID(c.BidID), ObservedAt: dbTime(c.OccurredAt), ProposalVersionID: locked.ProposalVersionID})
		if e != nil {
			return e
		}
		if blockers.ChangesRequestedCount > 0 || blockers.OpenThreadCount > 0 || blockers.UnassignedCount > 0 || blockers.OverdueCount > 0 || blockers.MandatoryUnsatisfied > 0 || blockers.OpenUnassigned > 0 || blockers.OpenOverdue > 0 || blockers.MandatoryResponseMissing > 0 || blockers.ExpiredEvidenceCount > 0 {
			return application.ErrProposalApprovalConflict
		}
		if _, e = q.GetProposalApprovalByManifest(tc, capturesql.GetProposalApprovalByManifestParams{BidID: dbID(c.BidID), ManifestID: dbID(c.ManifestID)}); e == nil {
			return application.ErrProposalApprovalConflict
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		var rationale *string
		if c.Rationale != "" {
			rationale = &c.Rationale
		}
		if e = q.InsertProposalApproval(tc, capturesql.InsertProposalApprovalParams{ID: dbID(c.ApprovalID), BidID: dbID(c.BidID), ProposalID: locked.ProposalID, ProposalVersionID: locked.ProposalVersionID, ManifestID: dbID(c.ManifestID), ReviewDecisionID: locked.ReviewDecisionID, ApproverID: dbID(a.ActorID), Rationale: rationale, ApprovedAt: dbTime(c.OccurredAt)}); e != nil {
			return e
		}
		if e = q.InsertProposalApprovalAudit(tc, capturesql.InsertProposalApprovalAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), ApprovalID: dbID(c.ApprovalID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), ProposalVersionID: locked.ProposalVersionID, ManifestID: dbID(c.ManifestID), ReviewDecisionID: locked.ReviewDecisionID}); e != nil {
			return e
		}
		if e = q.InsertProposalApprovalOutbox(tc, capturesql.InsertProposalApprovalOutboxParams{ID: dbID(c.OutboxID), ApprovalID: dbID(c.ApprovalID), BidID: dbID(c.BidID), ProposalVersionID: locked.ProposalVersionID, ManifestID: dbID(c.ManifestID), ReviewDecisionID: locked.ReviewDecisionID, ActorID: dbID(a.ActorID), OccurredAt: dbTime(c.OccurredAt)}); e != nil {
			return e
		}
		if e = completeWorkflow(tc, q, c.CommandID, c.ApprovalID, 1, c.OccurredAt); e != nil {
			return e
		}
		v, e := q.GetProposalApproval(tc, dbID(c.ApprovalID))
		if e != nil {
			return e
		}
		result = application.ProposalApprovalResult{Approval: mapProposalApproval(v)}
		return nil
	})
	return
}
func (s *WorkflowStore) GetProposalApprovalByManifest(ctx context.Context, a application.WorkflowAuthority, bidID, manifestID application.ID) (result application.ProposalApproval, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		v, e := capturesql.New(db).GetProposalApprovalByManifest(tc, capturesql.GetProposalApprovalByManifestParams{BidID: dbID(bidID), ManifestID: dbID(manifestID)})
		if errors.Is(e, pgx.ErrNoRows) {
			return application.ErrProposalApprovalNotFound
		}
		if e != nil {
			return e
		}
		result = mapProposalApproval(v)
		return nil
	})
	return
}
func mapProposalApproval(v capturesql.BidProposalApproval) application.ProposalApproval {
	rationale := ""
	if v.Rationale != nil {
		rationale = *v.Rationale
	}
	return application.ProposalApproval{ID: appID(v.ID), BidID: appID(v.BidID), ProposalID: appID(v.ProposalID), ProposalVersionID: appID(v.ProposalVersionID), ManifestID: appID(v.PackageManifestID), ReviewDecisionID: appID(v.ReviewDecisionID), ApproverID: appID(v.ApproverID), PolicyVersion: v.PolicyVersion, Rationale: rationale, ApprovedAt: v.ApprovedAt.Time.UTC()}
}
