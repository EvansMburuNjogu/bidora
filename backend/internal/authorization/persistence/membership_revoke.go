package persistence

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	authzsql "bidora/backend/internal/authorization/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type MembershipRevocationCommitter struct{ runner writeRunner }

func NewMembershipRevocationCommitter(runner writeRunner) (*MembershipRevocationCommitter, error) {
	if runner == nil {
		return nil, errors.New("membership_revocation_committer_invalid")
	}
	return &MembershipRevocationCommitter{runner}, nil
}
func (c *MembershipRevocationCommitter) RevokeCaptureContributor(ctx context.Context, commit authzapp.MembershipRevocationCommit) (authzapp.MembershipRevocationResult, error) {
	if ctx == nil || commit.ActorID.IsZero() || commit.TenantID.IsZero() || commit.TargetMembershipID.IsZero() || commit.AuditID.IsZero() || commit.OutboxID.IsZero() || commit.CorrelationID.IsZero() || commit.ActorContextGeneration < 1 || commit.RevokedAt.IsZero() {
		return authzapp.MembershipRevocationResult{}, errors.New("membership_revocation_commit_invalid")
	}
	if c == nil || c.runner == nil {
		return authzapp.MembershipRevocationResult{}, errors.New("membership_revocation_committer_invalid")
	}
	var result authzapp.MembershipRevocationResult
	err := c.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("membership_revocation_unavailable")
		}
		queries := authzsql.New(db)
		target, err := queries.LockRevocableCaptureContributor(txCtx, authzsql.LockRevocableCaptureContributorParams{TargetMembershipID: authorizationID(commit.TargetMembershipID), ActorUserID: authorizationID(commit.ActorID), TenantID: authorizationID(commit.TenantID), ActorContextGeneration: commit.ActorContextGeneration})
		if errors.Is(err, pgx.ErrNoRows) {
			return authzapp.ErrMembershipNotRevocable
		}
		if err != nil {
			return err
		}
		if !target.TargetMembershipID.Valid || !target.TargetUserID.Valid || target.TargetContextGeneration < 1 {
			return errors.New("membership_revocation_result_invalid")
		}
		affected, err := queries.RevokeCaptureContributorMembership(txCtx, authzsql.RevokeCaptureContributorMembershipParams{RevokedAt: authorizationTime(commit.RevokedAt), ID: target.TargetMembershipID})
		if err != nil {
			return err
		}
		if affected != 1 {
			return authzapp.ErrMembershipNotRevocable
		}
		affected, err = queries.RevokeCaptureContributorGrant(txCtx, authzsql.RevokeCaptureContributorGrantParams{RevokedAt: authorizationTime(commit.RevokedAt), MembershipID: target.TargetMembershipID})
		if err != nil {
			return err
		}
		if affected != 1 {
			return authzapp.ErrMembershipNotRevocable
		}
		generation, err := queries.GetRevokedMembershipGeneration(txCtx, target.TargetMembershipID)
		if err != nil || generation < 1 {
			return errors.New("membership_revocation_result_invalid")
		}
		if err = queries.InsertMembershipRevokedAudit(txCtx, authzsql.InsertMembershipRevokedAuditParams{ID: authorizationID(commit.AuditID), TenantID: authorizationID(commit.TenantID), ActorID: authorizationID(commit.ActorID), TargetID: target.TargetMembershipID, OccurredAt: authorizationTime(commit.RevokedAt), CorrelationID: authorizationID(commit.CorrelationID), ContextGeneration: generation}); err != nil {
			return err
		}
		if err = queries.InsertMembershipRevokedOutbox(txCtx, authzsql.InsertMembershipRevokedOutboxParams{ID: authorizationID(commit.OutboxID), TenantID: authorizationID(commit.TenantID), ActorID: authorizationID(commit.ActorID), AggregateID: target.TargetMembershipID, AggregateVersion: generation, OccurredAt: authorizationTime(commit.RevokedAt)}); err != nil {
			return err
		}
		result = authzapp.MembershipRevocationResult{MembershipID: authzapp.ID(target.TargetMembershipID.Bytes), TenantID: commit.TenantID, ContextGeneration: generation}
		return nil
	})
	if err != nil {
		return authzapp.MembershipRevocationResult{}, err
	}
	return result, nil
}
