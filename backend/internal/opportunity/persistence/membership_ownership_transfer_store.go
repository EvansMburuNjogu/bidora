package persistence

import (
	authapp "bidora/backend/internal/authorization/application"
	oppapp "bidora/backend/internal/opportunity/application"
	qsql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

func (s *WorkflowStore) TransferMembershipOwnership(ctx context.Context, op authapp.MembershipOffboardingOperation, actor authapp.ID, reason string, correlation authapp.ID, ids authapp.MembershipOffboardingIDs, at time.Time) (out authapp.OwnershipTransferResult, err error) {
	a := oppapp.WorkflowAuthority{TenantID: oppapp.ID(op.TenantID), ActorID: oppapp.ID(actor), AssignmentGeneration: op.AssignmentGeneration}
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		old, e := q.GetOwnershipTransferEvent(tc, dbID(oppapp.ID(op.ID)))
		if e == nil {
			out = authapp.OwnershipTransferResult{WorkspaceCount: old.WorkspaceCount, RequirementCount: old.RequirementCount, ReviewCount: old.ReviewCount, OccurredAt: old.OccurredAt.Time.UTC(), Replay: true}
			return nil
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		g, e := q.LockOwnershipTransferGuard(tc, qsql.LockOwnershipTransferGuardParams{SourceUserID: dbID(oppapp.ID(op.TargetUserID)), OperationID: dbID(oppapp.ID(op.ID)), At: dbTime(at)})
		if e != nil || !g.BlockedByOperationID.Valid {
			return authapp.ErrMembershipOffboardingConflict
		}
		counts, e := q.CountOwnershipTransferWork(tc, dbID(oppapp.ID(op.TargetUserID)))
		if e != nil {
			return e
		}
		if counts.WorkspaceCount+counts.RequirementCount+counts.ReviewCount > 500 {
			return authapp.ErrMembershipTransferLimit
		}
		wn, e := q.TransferWorkspaceOwnership(tc, qsql.TransferWorkspaceOwnershipParams{TargetUserID: dbID(oppapp.ID(op.SuccessorUserID)), At: dbTime(at), SourceUserID: dbID(oppapp.ID(op.TargetUserID))})
		if e != nil {
			return e
		}
		rn, e := q.TransferOpenRequirementOwnership(tc, qsql.TransferOpenRequirementOwnershipParams{TargetUserID: dbID(oppapp.ID(op.SuccessorUserID)), At: dbTime(at), SourceUserID: dbID(oppapp.ID(op.TargetUserID))})
		if e != nil {
			return e
		}
		vn, e := q.TransferOpenReviewAssignments(tc, qsql.TransferOpenReviewAssignmentsParams{TargetUserID: dbID(oppapp.ID(op.SuccessorUserID)), SourceUserID: dbID(oppapp.ID(op.TargetUserID))})
		if e != nil {
			return e
		}
		if wn != counts.WorkspaceCount || rn != counts.RequirementCount || vn != counts.ReviewCount {
			return authapp.ErrMembershipOffboardingConflict
		}
		p := qsql.InsertOwnershipTransferEventParams{ID: dbID(oppapp.ID(ids.TransferEventID)), OperationID: dbID(oppapp.ID(op.ID)), Sequence: 1, SourceUserID: dbID(oppapp.ID(op.TargetUserID)), TargetUserID: dbID(oppapp.ID(op.SuccessorUserID)), WorkspaceCount: int32(wn), RequirementCount: int32(rn), ReviewCount: int32(vn), ActorID: dbID(oppapp.ID(actor)), Reason: reason, OccurredAt: dbTime(at)}
		if e = q.InsertOwnershipTransferEvent(tc, p); e != nil {
			return e
		}
		if e = q.InsertOwnershipTransferAudit(tc, qsql.InsertOwnershipTransferAuditParams{ID: dbID(oppapp.ID(ids.TransferAuditID)), ActorID: dbID(oppapp.ID(actor)), OperationID: dbID(oppapp.ID(op.ID)), OccurredAt: dbTime(at), CorrelationID: dbID(oppapp.ID(correlation)), WorkspaceCount: int32(wn), RequirementCount: int32(rn), ReviewCount: int32(vn)}); e != nil {
			return e
		}
		if e = q.InsertOwnershipTransferOutbox(tc, qsql.InsertOwnershipTransferOutboxParams{ID: dbID(oppapp.ID(ids.TransferOutboxID)), OperationID: dbID(oppapp.ID(op.ID)), WorkspaceCount: int32(wn), RequirementCount: int32(rn), ReviewCount: int32(vn), OccurredAt: dbTime(at)}); e != nil {
			return e
		}
		out = authapp.OwnershipTransferResult{WorkspaceCount: int32(wn), RequirementCount: int32(rn), ReviewCount: int32(vn), OccurredAt: at}
		return nil
	})
	return
}
func (s *WorkflowStore) ClearMembershipOwnershipFence(ctx context.Context, op authapp.MembershipOffboardingOperation, at time.Time) error {
	a := oppapp.WorkflowAuthority{TenantID: oppapp.ID(op.TenantID), ActorID: oppapp.ID(op.ActorID), AssignmentGeneration: op.AssignmentGeneration}
	if e := s.valid(ctx, a); e != nil {
		return e
	}
	return s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		n, e := qsql.New(db).ClearOwnershipTransferGuard(tc, qsql.ClearOwnershipTransferGuardParams{At: dbTime(at), SourceUserID: dbID(oppapp.ID(op.TargetUserID)), OperationID: dbID(oppapp.ID(op.ID))})
		if e != nil {
			return e
		}
		if n != 1 && op.TenantTransferredAt != nil {
			return authapp.ErrMembershipOffboardingConflict
		}
		return nil
	})
}
