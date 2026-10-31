package persistence

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	authzsql "bidora/backend/internal/authorization/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type InvitationCancellationCommitter struct{ runner writeRunner }

func NewInvitationCancellationCommitter(runner writeRunner) (*InvitationCancellationCommitter, error) {
	if runner == nil {
		return nil, errors.New("invitation_cancellation_committer_invalid")
	}
	return &InvitationCancellationCommitter{runner}, nil
}
func (c *InvitationCancellationCommitter) Cancel(ctx context.Context, commit authzapp.InvitationCancellationCommit) error {
	if ctx == nil || commit.ActorID.IsZero() || commit.TenantID.IsZero() || commit.InvitationID.IsZero() || commit.AuditID.IsZero() || commit.OutboxID.IsZero() || commit.CorrelationID.IsZero() || commit.ContextGeneration < 1 || commit.CancelledAt.IsZero() {
		return errors.New("invitation_cancellation_commit_invalid")
	}
	if c == nil || c.runner == nil {
		return errors.New("invitation_cancellation_committer_invalid")
	}
	return c.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("invitation_cancellation_unavailable")
		}
		queries := authzsql.New(db)
		locked, err := queries.LockCancelableInvitation(txCtx, authzsql.LockCancelableInvitationParams{InvitationID: authorizationID(commit.InvitationID), ActorID: authorizationID(commit.ActorID), TenantID: authorizationID(commit.TenantID), ContextGeneration: commit.ContextGeneration, CancelledAt: authorizationTime(commit.CancelledAt)})
		if errors.Is(err, pgx.ErrNoRows) {
			return authzapp.ErrInvitationNotCancelable
		}
		if err != nil {
			return err
		}
		if !locked.ID.Valid || authzapp.ID(locked.ID.Bytes) != commit.InvitationID || locked.RoleKey == "" {
			return errors.New("invitation_cancellation_result_invalid")
		}
		affected, err := queries.CancelInvitation(txCtx, authzsql.CancelInvitationParams{CancelledAt: authorizationTime(commit.CancelledAt), ID: locked.ID})
		if err != nil {
			return err
		}
		if affected != 1 {
			return authzapp.ErrInvitationNotCancelable
		}
		if err = queries.InsertInvitationCancelledAudit(txCtx, authzsql.InsertInvitationCancelledAuditParams{ID: authorizationID(commit.AuditID), TenantID: authorizationID(commit.TenantID), ActorID: authorizationID(commit.ActorID), TargetID: locked.ID, OccurredAt: authorizationTime(commit.CancelledAt), CorrelationID: authorizationID(commit.CorrelationID), RoleKey: locked.RoleKey}); err != nil {
			return err
		}
		return queries.InsertInvitationCancelledOutbox(txCtx, authzsql.InsertInvitationCancelledOutboxParams{ID: authorizationID(commit.OutboxID), TenantID: authorizationID(commit.TenantID), ActorID: authorizationID(commit.ActorID), AggregateID: locked.ID, RoleKey: locked.RoleKey, OccurredAt: authorizationTime(commit.CancelledAt)})
	})
}
