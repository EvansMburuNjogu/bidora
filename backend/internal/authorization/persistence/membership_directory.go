package persistence

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	authzsql "bidora/backend/internal/authorization/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type MembershipDirectoryReader struct{ runner readOnlyRunner }

func NewMembershipDirectoryReader(runner readOnlyRunner) (*MembershipDirectoryReader, error) {
	if runner == nil {
		return nil, errors.New("membership_directory_reader_invalid")
	}
	return &MembershipDirectoryReader{runner}, nil
}
func (r *MembershipDirectoryReader) ListActiveMemberships(ctx context.Context, userID authzapp.ID) ([]authzapp.MembershipSummary, error) {
	if ctx == nil || userID.IsZero() {
		return nil, errors.New("membership_directory_lookup_invalid")
	}
	if r == nil || r.runner == nil {
		return nil, errors.New("membership_directory_reader_invalid")
	}
	result := []authzapp.MembershipSummary{}
	err := r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("membership_directory_unavailable")
		}
		rows, err := authzsql.New(db).ListActiveMembershipCapabilities(txCtx, authorizationID(userID))
		if err != nil {
			return err
		}
		result = make([]authzapp.MembershipSummary, len(rows))
		for index, row := range rows {
			if !row.MembershipID.Valid || !row.TenantID.Valid {
				return errors.New("membership_directory_result_invalid")
			}
			result[index] = authzapp.MembershipSummary{MembershipID: authzapp.ID(row.MembershipID.Bytes), TenantID: authzapp.ID(row.TenantID.Bytes), TenantType: row.TenantType, TenantName: row.TenantName, ContextGeneration: row.ContextGeneration, Capabilities: append(make([]string, 0, len(row.Capabilities)), row.Capabilities...)}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *MembershipDirectoryReader) ListManagedContributors(ctx context.Context, query authzapp.ManagedContributorQuery) ([]authzapp.ManagedContributor, error) {
	if ctx == nil || query.ActorID.IsZero() || query.TenantID.IsZero() || query.ContextGeneration < 1 {
		return nil, errors.New("managed_contributor_lookup_invalid")
	}
	if r == nil || r.runner == nil {
		return nil, errors.New("membership_directory_reader_invalid")
	}
	result := []authzapp.ManagedContributor{}
	err := r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("membership_directory_unavailable")
		}
		queries := authzsql.New(db)
		actorMembership, err := queries.AuthorizeMembershipManagement(txCtx, authzsql.AuthorizeMembershipManagementParams{ActorUserID: authorizationID(query.ActorID), TenantID: authorizationID(query.TenantID), ActorContextGeneration: query.ContextGeneration})
		if errors.Is(err, pgx.ErrNoRows) {
			return authzapp.ErrMembershipManagementNotAuthorized
		}
		if err != nil {
			return err
		}
		if !actorMembership.Valid {
			return errors.New("membership_directory_result_invalid")
		}
		rows, err := queries.ListCaptureContributorMemberships(txCtx, authorizationID(query.TenantID))
		if err != nil {
			return err
		}
		result = make([]authzapp.ManagedContributor, len(rows))
		for index, row := range rows {
			if !row.MembershipID.Valid || row.Email == "" || row.ContextGeneration < 1 {
				return errors.New("membership_directory_result_invalid")
			}
			result[index] = authzapp.ManagedContributor{MembershipID: authzapp.ID(row.MembershipID.Bytes), Email: row.Email, ContextGeneration: row.ContextGeneration}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
