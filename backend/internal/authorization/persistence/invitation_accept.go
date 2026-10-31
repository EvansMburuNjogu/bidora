package persistence

import (
	"context"
	"errors"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	authzsql "bidora/backend/internal/authorization/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type writeRunner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type InvitationAcceptanceCommitter struct{ runner writeRunner }

func NewInvitationAcceptanceCommitter(runner writeRunner) (*InvitationAcceptanceCommitter, error) {
	if runner == nil {
		return nil, errors.New("invitation_acceptance_committer_invalid")
	}
	return &InvitationAcceptanceCommitter{runner: runner}, nil
}
func (c *InvitationAcceptanceCommitter) AcceptInvitation(ctx context.Context, commit authzapp.InvitationAcceptanceCommit) (authzapp.InvitationAcceptanceResult, error) {
	if ctx == nil || commit.TokenHash == [32]byte{} || commit.UserID.IsZero() || commit.MembershipID.IsZero() || len(commit.GrantIDs) != 15 || commit.AuditID.IsZero() || commit.OutboxID.IsZero() || commit.CorrelationID.IsZero() || commit.AcceptedAt.IsZero() {
		return authzapp.InvitationAcceptanceResult{}, errors.New("invitation_acceptance_commit_invalid")
	}
	if c == nil || c.runner == nil {
		return authzapp.InvitationAcceptanceResult{}, errors.New("invitation_acceptance_committer_invalid")
	}
	var result authzapp.InvitationAcceptanceResult
	err := c.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("invitation_acceptance_unavailable")
		}
		queries := authzsql.New(db)
		invited, err := queries.LockInvitationForAcceptance(txCtx, authzsql.LockInvitationForAcceptanceParams{UserID: authorizationID(commit.UserID), TokenHash: commit.TokenHash[:], AcceptedAt: authorizationTime(commit.AcceptedAt)})
		if errors.Is(err, pgx.ErrNoRows) {
			return authzapp.ErrInvitationNotFound
		}
		if err != nil {
			return err
		}
		if !invited.ID.Valid || !invited.TenantID.Valid {
			return errors.New("invitation_acceptance_result_invalid")
		}
		role, roleErr := authzapp.ParseTenderpreneurRole(invited.RoleKey)
		if roleErr != nil {
			return errors.New("invitation_acceptance_result_invalid")
		}
		capabilities, roleErr := authzapp.TenderpreneurRoleCapabilitiesForBundle(role, invited.Capability)
		if roleErr != nil || len(capabilities) > len(commit.GrantIDs) {
			return errors.New("invitation_acceptance_result_invalid")
		}
		if err = queries.InsertInvitedMembership(txCtx, authzsql.InsertInvitedMembershipParams{ID: authorizationID(commit.MembershipID), UserID: authorizationID(commit.UserID), TenantID: invited.TenantID, CreatedAt: authorizationTime(commit.AcceptedAt)}); err != nil {
			return err
		}
		for index, capability := range capabilities {
			if commit.GrantIDs[index].IsZero() {
				return errors.New("invitation_acceptance_commit_invalid")
			}
			if err = queries.InsertInvitationCapabilityGrant(txCtx, authzsql.InsertInvitationCapabilityGrantParams{ID: authorizationID(commit.GrantIDs[index]), MembershipID: authorizationID(commit.MembershipID), RoleKey: string(role), Capability: capability, CreatedBy: authorizationID(commit.UserID), CreatedAt: authorizationTime(commit.AcceptedAt)}); err != nil {
				return err
			}
		}
		affected, err := queries.ConsumeTenantInvitation(txCtx, authzsql.ConsumeTenantInvitationParams{ConsumedAt: authorizationTime(commit.AcceptedAt), AcceptedBy: authorizationID(commit.UserID), ID: invited.ID})
		if err != nil || affected != 1 {
			if err != nil {
				return err
			}
			return authzapp.ErrInvitationNotFound
		}
		generation, err := queries.GetAcceptedMembershipGeneration(txCtx, authorizationID(commit.MembershipID))
		if err != nil || generation < 1 {
			return errors.New("invitation_acceptance_result_invalid")
		}
		if err = queries.InsertInvitationAcceptedAudit(txCtx, authzsql.InsertInvitationAcceptedAuditParams{ID: authorizationID(commit.AuditID), TenantID: invited.TenantID, ActorID: authorizationID(commit.UserID), TargetID: authorizationID(commit.MembershipID), OccurredAt: authorizationTime(commit.AcceptedAt), CorrelationID: authorizationID(commit.CorrelationID), ContextGeneration: generation, RoleKey: string(role)}); err != nil {
			return err
		}
		if err = queries.InsertMembershipGrantedOutbox(txCtx, authzsql.InsertMembershipGrantedOutboxParams{ID: authorizationID(commit.OutboxID), TenantID: invited.TenantID, ActorID: authorizationID(commit.UserID), AggregateID: authorizationID(commit.MembershipID), AggregateVersion: generation, RoleKey: string(role), OccurredAt: authorizationTime(commit.AcceptedAt)}); err != nil {
			return err
		}
		result = authzapp.InvitationAcceptanceResult{MembershipID: commit.MembershipID, TenantID: authzapp.ID(invited.TenantID.Bytes), Role: role, ContextGeneration: generation}
		return nil
	})
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "memberships_user_id_tenant_id_key" {
			return authzapp.InvitationAcceptanceResult{}, authzapp.ErrInvitationNotFound
		}
		return authzapp.InvitationAcceptanceResult{}, err
	}
	return result, nil
}
func authorizationTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}
