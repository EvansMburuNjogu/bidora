package persistence

import (
	"context"
	"errors"
	"time"

	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *WorkflowStore) ListOpportunities(ctx context.Context, a application.WorkflowAuthority, cursor application.ReadCursor, maxRows int32) (result []application.OpportunitySummary, err error) {
	if err = s.validRead(ctx, a, maxRows); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		cursorAt, cursorID := readCursor(cursor)
		rows, err := capturesql.New(db).ListOpportunities(txCtx, capturesql.ListOpportunitiesParams{CursorCreatedAt: cursorAt, CursorID: cursorID, MaxRows: maxRows})
		if err != nil {
			return err
		}
		result = make([]application.OpportunitySummary, len(rows))
		for i, row := range rows {
			result[i] = opportunitySummary(row.ID, row.SourceVersionID, row.Version, row.Title, row.BuyerName, row.ClosesAt, row.SourceTimezone, row.VerificationState, row.CreatedAt, row.UpdatedAt)
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetOpportunity(ctx context.Context, a application.WorkflowAuthority, id application.ID) (result application.OpportunityDetail, err error) {
	if err = s.validRead(ctx, a, 1); err != nil || id.IsZero() {
		return result, errors.New("opportunity_read_invalid")
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		row, err := q.GetOpportunityDetail(txCtx, dbID(id))
		if err != nil {
			return err
		}
		result.OpportunitySummary = opportunitySummary(row.ID, row.SourceVersionID, row.Version, row.Title, row.BuyerName, row.ClosesAt, row.SourceTimezone, row.VerificationState, row.CreatedAt, row.UpdatedAt)
		result.Revision = row.Revision
		result.SourceURI = row.SourceUri
		result.SourceDocumentVersionID = appID(row.SourceDocumentVersionID)
		result.HasSourceDocument = row.SourceDocumentVersionID.Valid
		result.SourceDateText = row.SourceDateText
		result.VerifiedBy = appID(row.VerifiedBy)
		result.VerifiedAt = timePointer(row.VerifiedAt)
		duplicates, err := q.ListPossibleDuplicateOpportunities(txCtx, dbID(id))
		if err != nil {
			return err
		}
		result.DuplicatesTruncated = len(duplicates) > 20
		if result.DuplicatesTruncated {
			duplicates = duplicates[:20]
		}
		result.PossibleDuplicateIDs = make([]application.ID, len(duplicates))
		for i, duplicate := range duplicates {
			result.PossibleDuplicateIDs[i] = appID(duplicate)
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) ListBids(ctx context.Context, a application.WorkflowAuthority, query application.BidReadQuery, maxRows int32) (result []application.BidSummary, err error) {
	if err = s.validRead(ctx, a, maxRows); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		cursorAt, cursorID := readCursor(query.Cursor)
		var state *string
		if query.State != "" {
			state = &query.State
		}
		owner := pgtype.UUID{}
		if query.FilterOwner {
			owner = dbID(query.OwnerID)
		}
		rows, err := capturesql.New(db).ListBidWorkspaces(txCtx, capturesql.ListBidWorkspacesParams{ArchiveState: query.ArchiveState, State: state, OwnerID: owner, CursorCreatedAt: cursorAt, CursorID: cursorID, MaxRows: maxRows})
		if err != nil {
			return err
		}
		result = make([]application.BidSummary, len(rows))
		for i, row := range rows {
			result[i] = bidSummary(row.ID, row.OpportunityID, row.SourceVersionID, row.OwnerID, row.State, row.Version, row.ArchivedAt, row.Title, row.BuyerName, row.ClosesAt, row.SourceTimezone, row.CreatedAt, row.UpdatedAt)
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetBid(ctx context.Context, a application.WorkflowAuthority, id application.ID) (result application.BidDetail, err error) {
	if err = s.validRead(ctx, a, 1); err != nil || id.IsZero() {
		return result, errors.New("bid_read_invalid")
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		row, err := q.GetBidWorkspaceDetail(txCtx, dbID(id))
		if err != nil {
			return err
		}
		result.BidSummary = bidSummary(row.ID, row.OpportunityID, row.SourceVersionID, row.OwnerID, row.State, row.Version, row.ArchivedAt, row.Title, row.BuyerName, row.ClosesAt, row.SourceTimezone, row.CreatedAt, row.UpdatedAt)
		result.CreatedBy = appID(row.CreatedBy)
		result.ArchivedBy = appID(row.ArchivedBy)
		result.ArchiveReason = row.ArchiveReason
		result.ArchivePolicyVersion = row.ArchivePolicyVersion
		result.SourceRevision = row.SourceRevision
		result.SourceURI = row.SourceUri
		result.SourceDocumentVersionID = appID(row.SourceDocumentVersionID)
		result.HasSourceDocument = row.SourceDocumentVersionID.Valid
		result.SourceDateText = row.SourceDateText
		result.VerificationState = row.VerificationState
		decisions, err := q.ListBidDecisionHistory(txCtx, dbID(id))
		if err != nil {
			return err
		}
		result.Decisions = make([]application.DecisionHistoryItem, len(decisions))
		for i, decision := range decisions {
			result.Decisions[i] = application.DecisionHistoryItem{ID: appID(decision.ID), ActorID: appID(decision.ActorID), Sequence: decision.Sequence, WorkspaceVersion: decision.WorkspaceVersion, Decision: decision.Decision, Rationale: decision.Rationale, DecidedAt: decision.DecidedAt.Time.UTC()}
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) validRead(ctx context.Context, a application.WorkflowAuthority, maxRows int32) error {
	if err := s.valid(ctx, a); err != nil {
		return err
	}
	if maxRows < 1 || maxRows > 1000 {
		return errors.New("workflow_read_limit_invalid")
	}
	return nil
}
func readCursor(cursor application.ReadCursor) (pgtype.Timestamptz, pgtype.UUID) {
	if cursor.CreatedAt.IsZero() {
		return pgtype.Timestamptz{}, pgtype.UUID{}
	}
	return dbTime(cursor.CreatedAt), dbID(cursor.ID)
}
func timePointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
func opportunitySummary(id, sourceID pgtype.UUID, version int64, title string, buyer *string, closes pgtype.Timestamptz, zone *string, state string, created, updated pgtype.Timestamptz) application.OpportunitySummary {
	return application.OpportunitySummary{ID: appID(id), SourceVersionID: appID(sourceID), Version: version, Title: title, BuyerName: buyer, ClosesAt: timePointer(closes), SourceTimezone: zone, VerificationState: state, CreatedAt: created.Time.UTC(), UpdatedAt: updated.Time.UTC()}
}
func bidSummary(id, opportunityID, sourceID, ownerID pgtype.UUID, state string, version int64, archived pgtype.Timestamptz, title string, buyer *string, closes pgtype.Timestamptz, zone *string, created, updated pgtype.Timestamptz) application.BidSummary {
	return application.BidSummary{ID: appID(id), OpportunityID: appID(opportunityID), SourceVersionID: appID(sourceID), OwnerID: appID(ownerID), State: state, Version: version, ArchivedAt: timePointer(archived), Title: title, BuyerName: buyer, ClosesAt: timePointer(closes), SourceTimezone: zone, CreatedAt: created.Time.UTC(), UpdatedAt: updated.Time.UTC()}
}
