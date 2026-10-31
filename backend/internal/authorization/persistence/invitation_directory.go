package persistence

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	authzsql "bidora/backend/internal/authorization/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type InvitationDirectoryReader struct{ runner readOnlyRunner }

func NewInvitationDirectoryReader(runner readOnlyRunner) (*InvitationDirectoryReader, error) {
	if runner == nil {
		return nil, errors.New("invitation_directory_reader_invalid")
	}
	return &InvitationDirectoryReader{runner}, nil
}
func (r *InvitationDirectoryReader) ListPending(ctx context.Context, read authzapp.InvitationDirectoryRead) ([]authzapp.PendingInvitation, error) {
	if ctx == nil || read.ActorID.IsZero() || read.TenantID.IsZero() || read.ContextGeneration < 1 || read.ObservedAt.IsZero() || read.MaxRows < 2 || read.MaxRows > 101 || (read.Cursor.CreatedAt.IsZero() != read.Cursor.InvitationID.IsZero()) {
		return nil, errors.New("invitation_directory_read_invalid")
	}
	if r == nil || r.runner == nil {
		return nil, errors.New("invitation_directory_reader_invalid")
	}
	result := []authzapp.PendingInvitation{}
	err := r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("invitation_directory_unavailable")
		}
		queries := authzsql.New(db)
		actor, err := queries.AuthorizeMembershipManagement(txCtx, authzsql.AuthorizeMembershipManagementParams{ActorUserID: authorizationID(read.ActorID), TenantID: authorizationID(read.TenantID), ActorContextGeneration: read.ContextGeneration})
		if errors.Is(err, pgx.ErrNoRows) {
			return authzapp.ErrMembershipManagementNotAuthorized
		}
		if err != nil {
			return err
		}
		if !actor.Valid {
			return errors.New("invitation_directory_result_invalid")
		}
		cursorAt := pgtype.Timestamptz{}
		cursorID := pgtype.UUID{}
		if !read.Cursor.CreatedAt.IsZero() {
			cursorAt = authorizationTime(read.Cursor.CreatedAt)
			cursorID = authorizationID(read.Cursor.InvitationID)
		}
		rows, err := queries.ListPendingInvitations(txCtx, authzsql.ListPendingInvitationsParams{TenantID: authorizationID(read.TenantID), ObservedAt: authorizationTime(read.ObservedAt), CursorCreatedAt: cursorAt, CursorID: cursorID, MaxRows: read.MaxRows})
		if err != nil {
			return err
		}
		result = make([]authzapp.PendingInvitation, len(rows))
		for index, row := range rows {
			if !row.ID.Valid || row.Email == "" || row.RoleKey == "" || !row.CreatedAt.Valid || !row.ExpiresAt.Valid || !row.ExpiresAt.Time.After(read.ObservedAt) {
				return errors.New("invitation_directory_result_invalid")
			}
			result[index] = authzapp.PendingInvitation{InvitationID: authzapp.ID(row.ID.Bytes), Email: row.Email, Role: authzapp.TenderpreneurRole(row.RoleKey), CreatedAt: row.CreatedAt.Time.UTC(), ExpiresAt: row.ExpiresAt.Time.UTC()}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
