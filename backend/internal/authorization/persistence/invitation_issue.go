package persistence

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	"bidora/backend/internal/authorization/invitation"
	authzsql "bidora/backend/internal/authorization/persistence/sqlc"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type InvitationIssuer struct{ runner writeRunner }

func NewInvitationIssuer(runner writeRunner) (*InvitationIssuer, error) {
	if runner == nil {
		return nil, errors.New("invitation_issuer_invalid")
	}
	return &InvitationIssuer{runner}, nil
}
func (i *InvitationIssuer) IssueInvitation(ctx context.Context, commit authzapp.InvitationIssueCommit) (authzapp.InvitationIssueCommitResult, error) {
	address, addressErr := email.Parse(commit.EmailDisplay)
	if ctx == nil || commit.InvitationID.IsZero() || commit.AuditID.IsZero() || commit.OutboxID.IsZero() || commit.ActorID.IsZero() || commit.TenantID.IsZero() || commit.CorrelationID.IsZero() || commit.ContextGeneration < 1 || addressErr != nil || address.Lookup() != commit.EmailLookup || commit.TokenHash == [32]byte{} || commit.Role == "" || commit.CreatedAt.IsZero() || !commit.ExpiresAt.Equal(commit.CreatedAt.UTC().Add(invitation.Lifetime)) {
		return authzapp.InvitationIssueCommitResult{}, errors.New("invitation_issue_commit_invalid")
	}
	if i == nil || i.runner == nil {
		return authzapp.InvitationIssueCommitResult{}, errors.New("invitation_issuer_invalid")
	}
	var result authzapp.InvitationIssueCommitResult
	err := i.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("invitation_issue_unavailable")
		}
		queries := authzsql.New(db)
		membershipID, err := queries.LockAuthorizedInvitationIssuer(txCtx, authzsql.LockAuthorizedInvitationIssuerParams{UserID: authorizationID(commit.ActorID), TenantID: authorizationID(commit.TenantID), ContextGeneration: commit.ContextGeneration})
		if errors.Is(err, pgx.ErrNoRows) {
			return authzapp.ErrNotAuthorized
		}
		if err != nil {
			return err
		}
		if !membershipID.Valid {
			return errors.New("invitation_issuer_result_invalid")
		}
		if _, err = queries.InvalidateActiveTenantInvitation(txCtx, authzsql.InvalidateActiveTenantInvitationParams{InvalidatedAt: authorizationTime(commit.CreatedAt), TenantID: authorizationID(commit.TenantID), EmailLookup: commit.EmailLookup}); err != nil {
			return err
		}
		if err = queries.InsertTenantInvitation(txCtx, authzsql.InsertTenantInvitationParams{ID: authorizationID(commit.InvitationID), TenantID: authorizationID(commit.TenantID), Email: commit.EmailDisplay, EmailLookup: commit.EmailLookup, TokenHash: commit.TokenHash[:], RoleKey: string(commit.Role), CreatedBy: authorizationID(commit.ActorID), CreatedAt: authorizationTime(commit.CreatedAt), ExpiresAt: authorizationTime(commit.ExpiresAt)}); err != nil {
			return err
		}
		if err = queries.InsertInvitationIssuedAudit(txCtx, authzsql.InsertInvitationIssuedAuditParams{ID: authorizationID(commit.AuditID), TenantID: authorizationID(commit.TenantID), ActorID: authorizationID(commit.ActorID), TargetID: authorizationID(commit.InvitationID), OccurredAt: authorizationTime(commit.CreatedAt), CorrelationID: authorizationID(commit.CorrelationID), RoleKey: string(commit.Role)}); err != nil {
			return err
		}
		if err = queries.InsertInvitationIssuedOutbox(txCtx, authzsql.InsertInvitationIssuedOutboxParams{ID: authorizationID(commit.OutboxID), TenantID: authorizationID(commit.TenantID), ActorID: authorizationID(commit.ActorID), AggregateID: authorizationID(commit.InvitationID), RoleKey: string(commit.Role), OccurredAt: authorizationTime(commit.CreatedAt)}); err != nil {
			return err
		}
		result = authzapp.InvitationIssueCommitResult{InvitationID: commit.InvitationID, Recipient: commit.EmailDisplay, Role: commit.Role, ExpiresAt: commit.ExpiresAt.UTC()}
		return nil
	})
	if err != nil {
		return authzapp.InvitationIssueCommitResult{}, err
	}
	return result, nil
}
