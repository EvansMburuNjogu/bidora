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

type readOnlyRunner interface {
	WithinReadOnlyTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type CaptureGrantReader struct{ runner readOnlyRunner }

func NewCaptureGrantReader(runner readOnlyRunner) (*CaptureGrantReader, error) {
	if runner == nil {
		return nil, errors.New("capture_grant_reader_invalid")
	}
	return &CaptureGrantReader{runner}, nil
}
func (r *CaptureGrantReader) FindExternalCaptureGrant(ctx context.Context, subject authzapp.CaptureSubject) (authzapp.CaptureGrant, error) {
	if ctx == nil || subject.UserID.IsZero() || subject.TenantID.IsZero() || subject.ContextGeneration < 1 {
		return authzapp.CaptureGrant{}, authzapp.ErrNotAuthorized
	}
	if r == nil || r.runner == nil {
		return authzapp.CaptureGrant{}, errors.New("capture_grant_reader_invalid")
	}
	var result authzapp.CaptureGrant
	err := r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("capture_grant_lookup_unavailable")
		}
		row, err := authzsql.New(db).AuthorizeExternalOpportunityCapture(txCtx, authzsql.AuthorizeExternalOpportunityCaptureParams{UserID: authorizationID(subject.UserID), TenantID: authorizationID(subject.TenantID), ContextGeneration: subject.ContextGeneration})
		if errors.Is(err, pgx.ErrNoRows) {
			return authzapp.ErrNotAuthorized
		}
		if err != nil {
			return err
		}
		result = authzapp.CaptureGrant{MembershipID: authzapp.ID(row.MembershipID.Bytes), TenantID: authzapp.ID(row.TenantID.Bytes), ContextGeneration: row.ContextGeneration, AssignmentGeneration: row.AssignmentGeneration}
		return nil
	})
	if err != nil {
		return authzapp.CaptureGrant{}, err
	}
	return result, nil
}
func authorizationID(id authzapp.ID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(id), Valid: !id.IsZero()}
}
