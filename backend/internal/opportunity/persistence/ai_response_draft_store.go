package persistence

import (
	"context"
	"errors"
	"time"

	entitlementapp "bidora/backend/internal/entitlementcheckpoint/application"
	entitlementpersistence "bidora/backend/internal/entitlementcheckpoint/persistence"
	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *WorkflowStore) PrepareResponseDraft(ctx context.Context, a application.WorkflowAuthority, bidID, requirementID application.ID, workspaceVersion, sectionVersion int64, observed time.Time) (result application.PreparedResponseDraft, err error) {
	if err = s.validRead(ctx, a, 5); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		row, e := q.PrepareRequirementForAIDraft(txCtx, capturesql.PrepareRequirementForAIDraftParams{RequirementID: dbID(requirementID), BidID: dbID(bidID), ExpectedWorkspaceVersion: workspaceVersion, ExpectedSectionVersion: sectionVersion})
		if errors.Is(e, pgx.ErrNoRows) {
			return application.ErrResponseDraftConflict
		}
		if e != nil {
			return e
		}
		evidence, e := q.ListCleanRequirementEvidenceForDraft(txCtx, capturesql.ListCleanRequirementEvidenceForDraftParams{RequirementID: row.ID, BidID: row.BidID, ObservedAt: dbTime(observed)})
		if e != nil {
			return e
		}
		ids := append([]pgtype.UUID{row.SourceDocumentVersionID}, evidence...)
		result = application.PreparedResponseDraft{RequirementText: row.Text, Inputs: make([]application.ResponseDraftInputObject, len(ids))}
		for i, id := range ids {
			kind := "REQUIREMENT_EVIDENCE"
			if i == 0 {
				kind = "TENDER_SOURCE"
			}
			input, x := q.GetResponseDraftInputObject(txCtx, capturesql.GetResponseDraftInputObjectParams{InputKind: kind, Ordinal: int32(i + 1), DocumentVersionID: id, ObservedAt: dbTime(observed)})
			if errors.Is(x, pgx.ErrNoRows) {
				return application.ErrResponseDraftConflict
			}
			if x != nil {
				return x
			}
			var digest [32]byte
			if len(input.Sha256) != 32 {
				return application.ErrResponseDraftConflict
			}
			copy(digest[:], input.Sha256)
			result.Inputs[i] = application.ResponseDraftInputObject{DocumentVersionID: appID(input.DocumentVersionID), InputKind: input.InputKind, Ordinal: input.Ordinal, ObjectKey: input.ObjectKey, ObjectVersion: input.ObjectVersion, MediaType: input.MediaType, SizeBytes: input.SizeBytes, SHA256: digest}
		}
		return nil
	})
	return result, err
}

func (s *WorkflowStore) RequestResponseDraft(ctx context.Context, a application.WorkflowAuthority, c application.ResponseDraftRequestCommit) (result application.ResponseDraftRequestResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, _, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "REQUEST_AI_RESPONSE_DRAFT", c.KeyHash, c.RequestHash, c.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			run, err := q.GetResponseDraftRun(txCtx, dbID(ref))
			if err != nil {
				return err
			}
			result = application.ResponseDraftRequestResult{RunID: appID(run.ID), JobID: appID(run.JobID), State: run.State, Replay: true}
			return nil
		}
		locked, err := q.LockRequirementForAIDraft(txCtx, capturesql.LockRequirementForAIDraftParams{RequirementID: dbID(c.RequirementID), BidID: dbID(c.BidID), ExpectedWorkspaceVersion: c.ExpectedWorkspaceVersion, ExpectedSectionVersion: c.ExpectedSectionVersion})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrResponseDraftConflict
		}
		if err != nil {
			return err
		}
		evidence, err := q.ListCleanRequirementEvidenceForDraft(txCtx, capturesql.ListCleanRequirementEvidenceForDraftParams{RequirementID: locked.ID, BidID: locked.BidID, ObservedAt: dbTime(c.OccurredAt)})
		if err != nil {
			return err
		}
		entitlementTx, err := entitlementpersistence.BindTransaction(db)
		if err != nil {
			return err
		}
		checkpoint, err := entitlementapp.RequestWithinTransaction(txCtx, entitlementTx, entitlementapp.RequestInput{TenantID: entitlementapp.ID(a.TenantID), TargetID: entitlementapp.ID(c.JobID), SourceOperationID: entitlementapp.ID(c.RunID), RequesterUserID: entitlementapp.ID(a.ActorID), CorrelationID: entitlementapp.ID(c.CorrelationID), TargetType: "RESPONSE_DRAFT", UsageKind: "GENERATION_TEXT", AssignmentGeneration: a.AssignmentGeneration, EstimatedCredits: c.EstimatedCredits}, [3]entitlementapp.ID{entitlementapp.ID(c.CheckpointID), entitlementapp.ID(c.CheckpointAuditID), entitlementapp.ID(c.CheckpointOutboxID)}, c.OccurredAt)
		if err != nil || checkpoint.Replay || application.ID(checkpoint.Checkpoint.ID) != c.CheckpointID {
			return application.ErrResponseDraftConflict
		}
		if err = q.InsertResponseDraftJob(txCtx, capturesql.InsertResponseDraftJobParams{ID: dbID(c.JobID), DocumentVersionID: locked.SourceDocumentVersionID, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		if err = q.InsertResponseDraftRun(txCtx, capturesql.InsertResponseDraftRunParams{ID: dbID(c.RunID), JobID: dbID(c.JobID), BidID: locked.BidID, RequirementID: locked.ID, SourceDocumentVersionID: locked.SourceDocumentVersionID, WorkspaceVersion: c.ExpectedWorkspaceVersion, SectionVersion: c.ExpectedSectionVersion, CurrentResponseVersionID: locked.CurrentRevisionID, Model: c.Model, ActorID: dbID(a.ActorID), CorrelationID: dbID(c.CorrelationID), CreatedAt: dbTime(c.OccurredAt), InputCharacters: &c.InputCharacters, EstimatedCredits: &c.EstimatedCredits, CheckpointID: dbID(c.CheckpointID)}); err != nil {
			return err
		}
		if err = q.InsertResponseDraftInput(txCtx, capturesql.InsertResponseDraftInputParams{RunID: dbID(c.RunID), DocumentVersionID: locked.SourceDocumentVersionID, InputKind: "TENDER_SOURCE", Ordinal: 1}); err != nil {
			return err
		}
		for i, id := range evidence {
			if err = q.InsertResponseDraftInput(txCtx, capturesql.InsertResponseDraftInputParams{RunID: dbID(c.RunID), DocumentVersionID: id, InputKind: "REQUIREMENT_EVIDENCE", Ordinal: int32(i + 2)}); err != nil {
				return err
			}
		}
		if err = q.InsertResponseDraftAudit(txCtx, capturesql.InsertResponseDraftAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), Action: "AIResponseDraftRequested", RunID: dbID(c.RunID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: locked.BidID, RequirementID: locked.ID, JobID: dbID(c.JobID)}); err != nil {
			return err
		}
		if err = q.InsertResponseDraftOutbox(txCtx, capturesql.InsertResponseDraftOutboxParams{ID: dbID(c.OutboxID), RunID: dbID(c.RunID), EventType: "ai.response_draft.requested.v1", JobID: dbID(c.JobID), BidID: locked.BidID, RequirementID: locked.ID, OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.RunID, c.ExpectedWorkspaceVersion, c.OccurredAt); err != nil {
			return err
		}
		result = application.ResponseDraftRequestResult{RunID: c.RunID, JobID: c.JobID, State: "PENDING_RESERVATION"}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) ListResponseDrafts(ctx context.Context, a application.WorkflowAuthority, bidID, requirementID application.ID) (result []application.ResponseDraftSuggestion, err error) {
	if err = s.validRead(ctx, a, 20); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		rows, err := q.ListResponseDraftSuggestions(txCtx, capturesql.ListResponseDraftSuggestionsParams{BidID: dbID(bidID), RequirementID: dbID(requirementID)})
		if err != nil {
			return err
		}
		result = make([]application.ResponseDraftSuggestion, len(rows))
		for i, row := range rows {
			item := application.ResponseDraftSuggestion{ID: appID(row.SuggestionID), RunID: appID(row.RunID), BidID: appID(row.BidID), RequirementID: appID(row.RequirementID), RunState: row.RunState, RequestedWorkspaceVersion: row.RequestedWorkspaceVersion, RequestedSectionVersion: row.RequestedSectionVersion, CreatedAt: row.CreatedAt.Time.UTC(), Citations: []application.ResponseDraftCitation{}, Gaps: []application.ResponseGapSuggestion{}}
			if row.SuggestedContent != nil {
				item.Content = *row.SuggestedContent
			}
			if row.Rationale != nil {
				item.Rationale = *row.Rationale
			}
			if row.ReviewState != nil {
				item.ReviewState = *row.ReviewState
			}
			if row.SuggestionID.Valid {
				citations, err := q.ListResponseDraftCitations(txCtx, row.SuggestionID)
				if err != nil {
					return err
				}
				for _, c := range citations {
					item.Citations = append(item.Citations, application.ResponseDraftCitation{Ordinal: c.Ordinal, DocumentVersionID: appID(c.DocumentVersionID), Location: c.CitationLocation, Quote: c.CitationQuote})
				}
				gaps, err := q.ListResponseGapSuggestions(txCtx, row.SuggestionID)
				if err != nil {
					return err
				}
				for _, g := range gaps {
					gap := application.ResponseGapSuggestion{Ordinal: g.Ordinal, Code: g.GapCode, Description: g.Description, DocumentVersionID: appID(g.DocumentVersionID), HasCitation: g.DocumentVersionID.Valid}
					if g.CitationLocation != nil {
						gap.Location = *g.CitationLocation
					}
					if g.CitationQuote != nil {
						gap.Quote = *g.CitationQuote
					}
					item.Gaps = append(item.Gaps, gap)
				}
			}
			result[i] = item
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) RecoverResponseDraft(ctx context.Context, staleBefore time.Time, completionIDs [3]application.ID, at time.Time) (recovered bool, err error) {
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		row, e := q.LockStaleRunningResponseDraft(txCtx, dbTime(staleBefore))
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		if row.InputCharacters == nil || row.EstimatedCredits == nil || *row.InputCharacters < 1 || *row.EstimatedCredits < 1 {
			return application.ErrResponseDraftConflict
		}
		if affected, x := q.FailResponseDraftRun(txCtx, capturesql.FailResponseDraftRunParams{State: "FAILED", FailureCode: stringPointer("AMBIGUOUS_PROVIDER_OUTCOME"), CompletedAt: dbTime(at), ID: row.ID}); x != nil || affected != 1 {
			return affectedErr(x)
		}
		if affected, x := q.CompleteResponseDraftJob(txCtx, capturesql.CompleteResponseDraftJobParams{State: "FAILED", CompletedAt: dbTime(at), ID: row.JobID}); x != nil || affected != 1 {
			return affectedErr(x)
		}
		entitlementTx, e := entitlementpersistence.BindTransaction(db)
		if e != nil {
			return e
		}
		_, _, e = entitlementapp.RequestCompletionWithinTransaction(txCtx, entitlementTx, entitlementapp.CompletionInput{CheckpointID: entitlementapp.ID(row.CheckpointID.Bytes), ReservationID: entitlementapp.ID(row.ReservationID.Bytes), ExecutionID: entitlementapp.ID(row.ID.Bytes), CorrelationID: entitlementapp.ID(row.CorrelationID.Bytes), Operation: entitlementapp.CompletionSettle, ActualCredits: *row.EstimatedCredits}, [3]entitlementapp.ID{entitlementapp.ID(completionIDs[0]), entitlementapp.ID(completionIDs[1]), entitlementapp.ID(completionIDs[2])}, at)
		if e == nil {
			recovered = true
		}
		return e
	})
	return recovered, err
}

func stringPointer(value string) *string { return &value }

func (s *WorkflowStore) ClaimResponseDraft(ctx context.Context, at time.Time) (claim application.ResponseDraftClaim, found bool, err error) {
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		row, e := q.ClaimNextResponseDraft(txCtx, dbTime(at))
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		affected, e := q.StartResponseDraftJob(txCtx, capturesql.StartResponseDraftJobParams{StartedAt: dbTime(at), ID: row.JobID})
		if e != nil || affected != 1 {
			return affectedErr(e)
		}
		inputCount, e := q.CountResponseDraftInputs(txCtx, row.ID)
		if e != nil {
			return e
		}
		inputs, e := q.ListResponseDraftInputObjects(txCtx, capturesql.ListResponseDraftInputObjectsParams{RunID: row.ID, ObservedAt: dbTime(at)})
		if e != nil {
			return e
		}
		if row.InputCharacters == nil || row.EstimatedCredits == nil || *row.InputCharacters < 1 || *row.EstimatedCredits < 1 {
			return application.ErrResponseDraftConflict
		}
		claim = application.ResponseDraftClaim{RunID: appID(row.ID), JobID: appID(row.JobID), BidID: appID(row.BidID), RequirementID: appID(row.RequirementID), RequestedBy: appID(row.RequestedBy), CorrelationID: appID(row.CorrelationID), TenantID: s.tenantID, CheckpointID: appID(row.CheckpointID), ReservationID: appID(row.ReservationID), AssignmentGeneration: s.assignmentGeneration, InputCharacters: *row.InputCharacters, EstimatedCredits: *row.EstimatedCredits, RequirementText: row.Text, Model: row.Model, UsageKind: row.UsageKind, RequestedWorkspaceVersion: row.RequestedWorkspaceVersion, RequestedSectionVersion: row.RequestedSectionVersion, Inputs: make([]application.ResponseDraftInputObject, len(inputs)), InputsCurrent: int64(len(inputs)) == inputCount, SnapshotCurrent: row.SnapshotCurrent}
		for i, input := range inputs {
			var digest [32]byte
			copy(digest[:], input.Sha256)
			claim.Inputs[i] = application.ResponseDraftInputObject{DocumentVersionID: appID(input.DocumentVersionID), InputKind: input.InputKind, Ordinal: input.Ordinal, ObjectKey: input.ObjectKey, ObjectVersion: input.ObjectVersion, MediaType: input.MediaType, SizeBytes: input.SizeBytes, SHA256: digest}
		}
		found = true
		return nil
	})
	return claim, found, err
}
func (s *WorkflowStore) CancelResponseDraft(ctx context.Context, c application.ResponseDraftClaim, completionIDs [3]application.ID, at time.Time) error {
	return s.finishResponseDraft(ctx, c, "CANCELLED", "", false, completionIDs, at)
}
func (s *WorkflowStore) FailResponseDraft(ctx context.Context, c application.ResponseDraftClaim, code string, settle bool, completionIDs [3]application.ID, at time.Time) error {
	return s.finishResponseDraft(ctx, c, "FAILED", code, settle, completionIDs, at)
}
func (s *WorkflowStore) finishResponseDraft(ctx context.Context, c application.ResponseDraftClaim, state, code string, settle bool, completionIDs [3]application.ID, at time.Time) error {
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		var failure *string
		if code != "" {
			failure = &code
		}
		affected, err := q.FailResponseDraftRun(txCtx, capturesql.FailResponseDraftRunParams{State: state, FailureCode: failure, CompletedAt: dbTime(at), ID: dbID(c.RunID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		affected, err = q.CompleteResponseDraftJob(txCtx, capturesql.CompleteResponseDraftJobParams{State: state, CompletedAt: dbTime(at), ID: dbID(c.JobID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		entitlementTx, err := entitlementpersistence.BindTransaction(db)
		if err != nil {
			return err
		}
		operation, actual := entitlementapp.CompletionRelease, int64(0)
		if settle {
			operation, actual = entitlementapp.CompletionSettle, c.EstimatedCredits
		}
		_, _, err = entitlementapp.RequestCompletionWithinTransaction(txCtx, entitlementTx, entitlementapp.CompletionInput{CheckpointID: entitlementapp.ID(c.CheckpointID), ReservationID: entitlementapp.ID(c.ReservationID), ExecutionID: entitlementapp.ID(c.RunID), CorrelationID: entitlementapp.ID(c.CorrelationID), Operation: operation, ActualCredits: actual}, [3]entitlementapp.ID{entitlementapp.ID(completionIDs[0]), entitlementapp.ID(completionIDs[1]), entitlementapp.ID(completionIDs[2])}, at)
		return err
	})
}
func (s *WorkflowStore) CompleteResponseDraft(ctx context.Context, c application.ResponseDraftClaim, g application.GeneratedResponseDraft, actualCredits int64, suggestionID, auditID, outboxID application.ID, completionIDs [3]application.ID, at time.Time) error {
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		if err := q.InsertResponseDraftSuggestion(txCtx, capturesql.InsertResponseDraftSuggestionParams{ID: dbID(suggestionID), RunID: dbID(c.RunID), Content: g.Content, Rationale: g.Rationale, CreatedAt: dbTime(at)}); err != nil {
			return err
		}
		for i, citation := range g.Citations {
			input, ok := draftInput(c.Inputs, citation.InputOrdinal)
			if !ok {
				return errors.New("response_draft_citation_invalid")
			}
			if err := q.InsertResponseDraftCitation(txCtx, capturesql.InsertResponseDraftCitationParams{SuggestionID: dbID(suggestionID), Ordinal: int32(i + 1), DocumentVersionID: dbID(input.DocumentVersionID), Location: citation.Location, Quote: citation.Quote}); err != nil {
				return err
			}
		}
		for i, gap := range g.Gaps {
			var documentID pgtype.UUID
			var location, quote *string
			if gap.InputOrdinal > 0 {
				input, ok := draftInput(c.Inputs, gap.InputOrdinal)
				if !ok {
					return errors.New("response_draft_gap_invalid")
				}
				documentID = dbID(input.DocumentVersionID)
				location = &gap.Location
				quote = &gap.Quote
			}
			if err := q.InsertResponseGapSuggestion(txCtx, capturesql.InsertResponseGapSuggestionParams{SuggestionID: dbID(suggestionID), Ordinal: int32(i + 1), GapCode: gap.Code, Description: gap.Description, DocumentVersionID: documentID, Location: location, Quote: quote}); err != nil {
				return err
			}
		}
		responseID := g.ProviderResponseID
		affected, err := q.CompleteResponseDraftRun(txCtx, capturesql.CompleteResponseDraftRunParams{ProviderResponseID: &responseID, CompletedAt: dbTime(at), ID: dbID(c.RunID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		affected, err = q.CompleteResponseDraftJob(txCtx, capturesql.CompleteResponseDraftJobParams{State: "SUCCEEDED", CompletedAt: dbTime(at), ID: dbID(c.JobID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		entitlementTx, err := entitlementpersistence.BindTransaction(db)
		if err != nil {
			return err
		}
		if _, _, err = entitlementapp.RequestCompletionWithinTransaction(txCtx, entitlementTx, entitlementapp.CompletionInput{CheckpointID: entitlementapp.ID(c.CheckpointID), ReservationID: entitlementapp.ID(c.ReservationID), ExecutionID: entitlementapp.ID(c.RunID), CorrelationID: entitlementapp.ID(c.CorrelationID), Operation: entitlementapp.CompletionSettle, ActualCredits: actualCredits}, [3]entitlementapp.ID{entitlementapp.ID(completionIDs[0]), entitlementapp.ID(completionIDs[1]), entitlementapp.ID(completionIDs[2])}, at); err != nil {
			return err
		}
		if err = q.InsertResponseDraftAudit(txCtx, capturesql.InsertResponseDraftAuditParams{ID: dbID(auditID), ActorID: dbID(c.RequestedBy), Action: "AIResponseDraftProposed", RunID: dbID(c.RunID), OccurredAt: dbTime(at), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), RequirementID: dbID(c.RequirementID), JobID: dbID(c.JobID)}); err != nil {
			return err
		}
		return q.InsertResponseDraftOutbox(txCtx, capturesql.InsertResponseDraftOutboxParams{ID: dbID(outboxID), RunID: dbID(c.RunID), EventType: "ai.response_draft.proposed.v1", JobID: dbID(c.JobID), BidID: dbID(c.BidID), RequirementID: dbID(c.RequirementID), OccurredAt: dbTime(at), ActorID: dbID(c.RequestedBy)})
	})
}
func draftInput(inputs []application.ResponseDraftInputObject, ordinal int32) (application.ResponseDraftInputObject, bool) {
	for _, input := range inputs {
		if input.Ordinal == ordinal {
			return input, true
		}
	}
	return application.ResponseDraftInputObject{}, false
}
