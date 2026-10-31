package persistence

import (
	"context"
	"errors"

	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *WorkflowStore) ListAISuggestions(ctx context.Context, a application.WorkflowAuthority, opportunityID, documentVersionID application.ID) (result []application.AISuggestion, err error) {
	if err = s.validRead(ctx, a, 500); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		rows, err := capturesql.New(db).ListAIRequirementSuggestions(txCtx, capturesql.ListAIRequirementSuggestionsParams{DocumentVersionID: dbID(documentVersionID), OpportunityID: dbID(opportunityID)})
		if err != nil {
			return err
		}
		result = make([]application.AISuggestion, len(rows))
		for i, row := range rows {
			result[i] = mapAISuggestion(row.ID, row.DocumentVersionID, row.Ordinal, row.RequirementText, row.Category, row.Mandatory, row.CitationPage, row.CitationLocation, row.CitationQuote, row.Confidence, row.ReviewState, row.Version, row.AcceptedRequirementID, row.ReviewNote, row.CreatedAt, row.ReviewedBy, row.ReviewedAt)
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) AcceptAISuggestion(ctx context.Context, a application.WorkflowAuthority, c application.AIAcceptCommit) (result application.AISuggestionReviewResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, workspaceVersion, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "ACCEPT_AI_REQUIREMENT", c.KeyHash, c.RequestHash, c.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			suggestion, err := q.GetAIRequirementSuggestion(txCtx, dbID(c.SuggestionID))
			if err != nil {
				return err
			}
			requirement, err := q.GetBidRequirement(txCtx, dbID(ref))
			if err != nil {
				return err
			}
			mapped := mapRequirement(requirement.ID, requirement.BidID, requirement.SourceVersionID, requirement.SourceLocation, requirement.Text, requirement.Mandatory, requirement.Category, requirement.Status, requirement.OwnerID, requirement.DueAt, requirement.Version, requirement.CreatedAt, requirement.UpdatedAt)
			result = application.AISuggestionReviewResult{Suggestion: mapSuggestionRow(suggestion), Requirement: &mapped, WorkspaceVersion: workspaceVersion, Replay: true}
			return nil
		}
		locked, err := q.LockAISuggestionForAcceptance(txCtx, capturesql.LockAISuggestionForAcceptanceParams{SuggestionID: dbID(c.SuggestionID), DocumentVersionID: dbID(c.DocumentVersionID), OpportunityID: dbID(c.OpportunityID), ExpectedSuggestionVersion: c.ExpectedSuggestionVersion, BidID: dbID(c.BidID), SourceVersionID: dbID(c.SourceVersionID), ExpectedWorkspaceVersion: c.ExpectedWorkspaceVersion})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrAIRequirementReviewConflict
		}
		if err != nil {
			return err
		}
		owner := pgtype.UUID{}
		if c.HasOwner {
			owner = dbID(c.OwnerID)
		}
		due := pgtype.Timestamptz{}
		if c.DueAt != nil {
			due = dbTime(*c.DueAt)
		}
		if err = q.InsertAcceptedAIRequirement(txCtx, capturesql.InsertAcceptedAIRequirementParams{ID: dbID(c.RequirementID), BidID: locked.BidID, SourceVersionID: locked.SourceVersionID, DocumentVersionID: locked.DocumentVersionID, SourceLocation: locked.CitationLocation, Text: c.RequirementText, Mandatory: c.Mandatory, Category: c.Category, OwnerID: owner, DueAt: due, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		if err = q.InsertRequirementCorrection(txCtx, capturesql.InsertRequirementCorrectionParams{ID: dbID(c.RequirementID), RequirementID: dbID(c.RequirementID), Revision: 1, SourceVersionID: locked.SourceVersionID, SourceDocumentVersionID: locked.DocumentVersionID, SourceLocation: locked.CitationLocation, Text: c.RequirementText, Mandatory: c.Mandatory, Category: c.Category, CorrectionReason: "Initial requirement accepted from AI suggestion", ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		var note *string
		if c.ReviewNote != "" {
			note = &c.ReviewNote
		}
		affected, err := q.AcceptAIRequirementSuggestion(txCtx, capturesql.AcceptAIRequirementSuggestionParams{RequirementID: dbID(c.RequirementID), ReviewNote: note, ActorID: dbID(a.ActorID), ReviewedAt: dbTime(c.OccurredAt), ID: locked.ID, ExpectedVersion: c.ExpectedSuggestionVersion})
		if err != nil {
			return err
		}
		if affected != 1 {
			return application.ErrAIRequirementReviewConflict
		}
		newWorkspaceVersion := c.ExpectedWorkspaceVersion + 1
		affected, err = q.AdvanceWorkspaceForRequirementChange(txCtx, capturesql.AdvanceWorkspaceForRequirementChangeParams{UpdatedAt: dbTime(c.OccurredAt), ID: locked.BidID, ExpectedVersion: c.ExpectedWorkspaceVersion})
		if err != nil {
			return err
		}
		if affected != 1 {
			return application.ErrAIRequirementReviewConflict
		}
		if err = q.InsertAISuggestionReviewedAudit(txCtx, capturesql.InsertAISuggestionReviewedAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), Action: "AIRequirementAccepted", SuggestionID: locked.ID, OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), ReviewState: "ACCEPTED", RequirementID: dbID(c.RequirementID), WorkspaceVersion: &newWorkspaceVersion}); err != nil {
			return err
		}
		if err = q.InsertAISuggestionReviewedOutbox(txCtx, capturesql.InsertAISuggestionReviewedOutboxParams{ID: dbID(c.OutboxID), SuggestionID: locked.ID, EventType: "ai.requirement.accepted.v1", DocumentVersionID: locked.DocumentVersionID, ReviewState: "ACCEPTED", RequirementID: dbID(c.RequirementID), OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.RequirementID, newWorkspaceVersion, c.OccurredAt); err != nil {
			return err
		}
		suggestion, err := q.GetAIRequirementSuggestion(txCtx, locked.ID)
		if err != nil {
			return err
		}
		requirement, err := q.GetBidRequirement(txCtx, dbID(c.RequirementID))
		if err != nil {
			return err
		}
		mapped := mapRequirement(requirement.ID, requirement.BidID, requirement.SourceVersionID, requirement.SourceLocation, requirement.Text, requirement.Mandatory, requirement.Category, requirement.Status, requirement.OwnerID, requirement.DueAt, requirement.Version, requirement.CreatedAt, requirement.UpdatedAt)
		result = application.AISuggestionReviewResult{Suggestion: mapSuggestionRow(suggestion), Requirement: &mapped, WorkspaceVersion: newWorkspaceVersion}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) RejectAISuggestion(ctx context.Context, a application.WorkflowAuthority, c application.AIRejectCommit) (result application.AISuggestionReviewResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, _, _, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "REJECT_AI_REQUIREMENT", c.KeyHash, c.RequestHash, c.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			row, err := q.GetAIRequirementSuggestion(txCtx, dbID(c.SuggestionID))
			if err != nil {
				return err
			}
			result = application.AISuggestionReviewResult{Suggestion: mapSuggestionRow(row), Replay: true}
			return nil
		}
		locked, err := q.LockAISuggestionForRejection(txCtx, capturesql.LockAISuggestionForRejectionParams{SuggestionID: dbID(c.SuggestionID), DocumentVersionID: dbID(c.DocumentVersionID), OpportunityID: dbID(c.OpportunityID), ExpectedVersion: c.ExpectedSuggestionVersion})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrAIRequirementReviewConflict
		}
		if err != nil {
			return err
		}
		affected, err := q.RejectAIRequirementSuggestion(txCtx, capturesql.RejectAIRequirementSuggestionParams{ReviewNote: &c.ReviewNote, ActorID: dbID(a.ActorID), ReviewedAt: dbTime(c.OccurredAt), ID: locked.ID, ExpectedVersion: c.ExpectedSuggestionVersion})
		if err != nil {
			return err
		}
		if affected != 1 {
			return application.ErrAIRequirementReviewConflict
		}
		if err = q.InsertAISuggestionReviewedAudit(txCtx, capturesql.InsertAISuggestionReviewedAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), Action: "AIRequirementRejected", SuggestionID: locked.ID, OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), ReviewState: "REJECTED"}); err != nil {
			return err
		}
		if err = q.InsertAISuggestionReviewedOutbox(txCtx, capturesql.InsertAISuggestionReviewedOutboxParams{ID: dbID(c.OutboxID), SuggestionID: locked.ID, EventType: "ai.requirement.rejected.v1", DocumentVersionID: locked.DocumentVersionID, ReviewState: "REJECTED", OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.SuggestionID, c.ExpectedSuggestionVersion+1, c.OccurredAt); err != nil {
			return err
		}
		row, err := q.GetAIRequirementSuggestion(txCtx, locked.ID)
		if err != nil {
			return err
		}
		result = application.AISuggestionReviewResult{Suggestion: mapSuggestionRow(row)}
		return nil
	})
	return result, err
}
func mapSuggestionRow(r capturesql.GetAIRequirementSuggestionRow) application.AISuggestion {
	return mapAISuggestion(r.ID, r.DocumentVersionID, r.Ordinal, r.RequirementText, r.Category, r.Mandatory, r.CitationPage, r.CitationLocation, r.CitationQuote, r.Confidence, r.ReviewState, r.Version, r.AcceptedRequirementID, r.ReviewNote, r.CreatedAt, r.ReviewedBy, r.ReviewedAt)
}
func mapAISuggestion(id, documentVersionID pgtype.UUID, ordinal int32, text, category string, mandatory bool, page *int32, location, quote string, confidence pgtype.Numeric, state string, version int64, accepted pgtype.UUID, note *string, created pgtype.Timestamptz, reviewedBy pgtype.UUID, reviewedAt pgtype.Timestamptz) application.AISuggestion {
	value := 0.0
	if converted, err := confidence.Float64Value(); err == nil && converted.Valid {
		value = converted.Float64
	}
	pageValue := int32(0)
	if page != nil {
		pageValue = *page
	}
	noteValue := ""
	if note != nil {
		noteValue = *note
	}
	return application.AISuggestion{ID: appID(id), DocumentVersionID: appID(documentVersionID), AcceptedRequirementID: appID(accepted), HasAcceptedRequirement: accepted.Valid, ReviewedBy: appID(reviewedBy), HasReviewedBy: reviewedBy.Valid, Ordinal: ordinal, RequirementText: text, Category: category, Mandatory: mandatory, CitationPage: pageValue, CitationLocation: location, CitationQuote: quote, Confidence: value, ReviewState: state, Version: version, ReviewNote: noteValue, CreatedAt: created.Time.UTC(), ReviewedAt: timePointer(reviewedAt)}
}
