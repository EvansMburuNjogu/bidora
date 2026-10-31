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
)

func (s *WorkflowStore) PrepareProposalDraft(ctx context.Context, a application.WorkflowAuthority, bidID, versionID application.ID, workspaceVersion, proposalVersion int64, observed time.Time) (result application.PreparedProposalDraft, err error) {
	if err = s.validRead(ctx, a, 5); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		_, e := q.PrepareProposalForAIDraft(txCtx, capturesql.PrepareProposalForAIDraftParams{BidID: dbID(bidID), ProposalVersionID: dbID(versionID), ExpectedProposalVersion: proposalVersion, ExpectedWorkspaceVersion: workspaceVersion})
		if errors.Is(e, pgx.ErrNoRows) {
			return application.ErrProposalDraftConflict
		}
		if e != nil {
			return e
		}
		sources, e := q.ListCleanProposalDraftSources(txCtx, capturesql.ListCleanProposalDraftSourcesParams{ProposalVersionID: dbID(versionID), ObservedAt: dbTime(observed)})
		if e != nil {
			return e
		}
		if len(sources) < 1 || len(sources) > 5 {
			return application.ErrProposalDraftConflict
		}
		result.Inputs = make([]application.ResponseDraftInputObject, len(sources))
		for i, id := range sources {
			v, x := q.GetProposalDraftInputObject(txCtx, capturesql.GetProposalDraftInputObjectParams{Ordinal: int32(i + 1), DocumentVersionID: id, ObservedAt: dbTime(observed)})
			if errors.Is(x, pgx.ErrNoRows) {
				return application.ErrProposalDraftConflict
			}
			if x != nil {
				return x
			}
			if len(v.Sha256) != 32 {
				return application.ErrProposalDraftConflict
			}
			var digest [32]byte
			copy(digest[:], v.Sha256)
			result.Inputs[i] = application.ResponseDraftInputObject{DocumentVersionID: appID(v.DocumentVersionID), Ordinal: v.Ordinal, ObjectKey: v.ObjectKey, ObjectVersion: v.ObjectVersion, MediaType: v.MediaType, SizeBytes: v.SizeBytes, SHA256: digest}
		}
		return nil
	})
	return result, err
}

func (s *WorkflowStore) RequestProposalDraft(ctx context.Context, a application.WorkflowAuthority, c application.ProposalDraftCommit) (result application.ProposalDraftRequestResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, _, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "REQUEST_AI_PROPOSAL_DRAFT", c.KeyHash, c.RequestHash, c.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			run, err := q.GetProposalDraftRun(txCtx, dbID(ref))
			if err != nil {
				return err
			}
			result = application.ProposalDraftRequestResult{RunID: appID(run.ID), JobID: appID(run.JobID), State: run.State, Replay: true}
			return nil
		}
		locked, err := q.LockProposalForAIDraft(txCtx, capturesql.LockProposalForAIDraftParams{BidID: dbID(c.BidID), ProposalVersionID: dbID(c.ProposalVersionID), ExpectedProposalVersion: c.ExpectedProposalVersion, ExpectedWorkspaceVersion: c.ExpectedWorkspaceVersion})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrProposalDraftConflict
		}
		if err != nil {
			return err
		}
		sources, err := q.ListCleanProposalDraftSources(txCtx, capturesql.ListCleanProposalDraftSourcesParams{ProposalVersionID: dbID(c.ProposalVersionID), ObservedAt: dbTime(c.OccurredAt)})
		if err != nil {
			return err
		}
		if len(sources) < 1 || len(sources) > 5 {
			return application.ErrProposalDraftConflict
		}
		entitlementTx, err := entitlementpersistence.BindTransaction(db)
		if err != nil {
			return err
		}
		checkpoint, err := entitlementapp.RequestWithinTransaction(txCtx, entitlementTx, entitlementapp.RequestInput{TenantID: entitlementapp.ID(a.TenantID), TargetID: entitlementapp.ID(c.JobID), SourceOperationID: entitlementapp.ID(c.RunID), RequesterUserID: entitlementapp.ID(a.ActorID), CorrelationID: entitlementapp.ID(c.CorrelationID), TargetType: "PROPOSAL_DRAFT", UsageKind: "GENERATION_TEXT", AssignmentGeneration: a.AssignmentGeneration, EstimatedCredits: c.EstimatedCredits}, [3]entitlementapp.ID{entitlementapp.ID(c.CheckpointID), entitlementapp.ID(c.CheckpointAuditID), entitlementapp.ID(c.CheckpointOutboxID)}, c.OccurredAt)
		if err != nil || checkpoint.Replay || application.ID(checkpoint.Checkpoint.ID) != c.CheckpointID {
			return application.ErrProposalDraftConflict
		}
		if err = q.InsertProposalDraftJob(txCtx, capturesql.InsertProposalDraftJobParams{ID: dbID(c.JobID), DocumentVersionID: sources[0], ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		if err = q.InsertProposalDraftRun(txCtx, capturesql.InsertProposalDraftRunParams{ID: dbID(c.RunID), JobID: dbID(c.JobID), BidID: dbID(c.BidID), ProposalID: locked.ProposalID, ProposalVersionID: dbID(c.ProposalVersionID), WorkspaceVersion: c.ExpectedWorkspaceVersion, ProposalVersion: c.ExpectedProposalVersion, Model: c.Model, ActorID: dbID(a.ActorID), CorrelationID: dbID(c.CorrelationID), CreatedAt: dbTime(c.OccurredAt), InputCharacters: &c.InputCharacters, EstimatedCredits: &c.EstimatedCredits, CheckpointID: dbID(c.CheckpointID)}); err != nil {
			return err
		}
		for i, id := range sources {
			if err = q.InsertProposalDraftInput(txCtx, capturesql.InsertProposalDraftInputParams{RunID: dbID(c.RunID), DocumentVersionID: id, Ordinal: int32(i + 1)}); err != nil {
				return err
			}
		}
		if err = q.InsertProposalDraftAudit(txCtx, capturesql.InsertProposalDraftAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), Action: "AIProposalDraftRequested", RunID: dbID(c.RunID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), ProposalVersionID: dbID(c.ProposalVersionID), JobID: dbID(c.JobID)}); err != nil {
			return err
		}
		if err = q.InsertProposalDraftOutbox(txCtx, capturesql.InsertProposalDraftOutboxParams{ID: dbID(c.OutboxID), RunID: dbID(c.RunID), EventType: "ai.proposal_draft.requested.v1", JobID: dbID(c.JobID), BidID: dbID(c.BidID), ProposalVersionID: dbID(c.ProposalVersionID), OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.RunID, c.ExpectedWorkspaceVersion, c.OccurredAt); err != nil {
			return err
		}
		result = application.ProposalDraftRequestResult{RunID: c.RunID, JobID: c.JobID, State: "PENDING_RESERVATION"}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) ListProposalDrafts(ctx context.Context, a application.WorkflowAuthority, bidID, versionID application.ID) (result []application.ProposalDraftSuggestion, err error) {
	if err = s.validRead(ctx, a, 20); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		rows, err := q.ListProposalDraftSuggestions(txCtx, capturesql.ListProposalDraftSuggestionsParams{BidID: dbID(bidID), ProposalVersionID: dbID(versionID)})
		if err != nil {
			return err
		}
		result = make([]application.ProposalDraftSuggestion, len(rows))
		for i, row := range rows {
			item := application.ProposalDraftSuggestion{ID: appID(row.SuggestionID), RunID: appID(row.RunID), ProposalVersionID: appID(row.ProposalVersionID), RunState: row.RunState, RequestedWorkspaceVersion: row.RequestedWorkspaceVersion, RequestedProposalVersion: row.RequestedProposalVersion, CreatedAt: row.CreatedAt.Time.UTC(), Citations: []application.ResponseDraftCitation{}}
			if row.SuggestionVersion != nil {
				item.SuggestionVersion = *row.SuggestionVersion
			}
			if row.AppliedProposalVersionID.Valid {
				item.AppliedProposalVersionID = appID(row.AppliedProposalVersionID)
			}
			if row.ReviewedBy.Valid {
				item.ReviewedBy = appID(row.ReviewedBy)
			}
			if row.ReviewedAt.Valid {
				item.ReviewedAt = row.ReviewedAt.Time.UTC()
			}
			if row.ReviewNote != nil {
				item.ReviewNote = *row.ReviewNote
			}
			if row.Introduction != nil {
				item.Introduction = *row.Introduction
			}
			if row.Closing != nil {
				item.Closing = *row.Closing
			}
			if row.Rationale != nil {
				item.Rationale = *row.Rationale
			}
			if row.ReviewState != nil {
				item.ReviewState = *row.ReviewState
			}
			if row.SuggestionID.Valid {
				citations, err := q.ListProposalDraftCitations(txCtx, row.SuggestionID)
				if err != nil {
					return err
				}
				for _, v := range citations {
					item.Citations = append(item.Citations, application.ResponseDraftCitation{Ordinal: v.Ordinal, DocumentVersionID: appID(v.DocumentVersionID), Location: v.CitationLocation, Quote: v.CitationQuote})
				}
			}
			result[i] = item
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) RecoverProposalDraft(ctx context.Context, staleBefore time.Time, ids [3]application.ID, at time.Time) (recovered bool, err error) {
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		row, e := q.LockStaleRunningProposalDraft(txCtx, dbTime(staleBefore))
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		if row.InputCharacters == nil || row.EstimatedCredits == nil {
			return application.ErrProposalDraftConflict
		}
		if n, x := q.FailProposalDraftRun(txCtx, capturesql.FailProposalDraftRunParams{State: "FAILED", FailureCode: stringPointer("AMBIGUOUS_PROVIDER_OUTCOME"), CompletedAt: dbTime(at), ID: row.ID}); x != nil || n != 1 {
			return affectedErr(x)
		}
		if n, x := q.CompleteProposalDraftJob(txCtx, capturesql.CompleteProposalDraftJobParams{State: "FAILED", CompletedAt: dbTime(at), ID: row.JobID}); x != nil || n != 1 {
			return affectedErr(x)
		}
		ep, x := entitlementpersistence.BindTransaction(db)
		if x != nil {
			return x
		}
		_, _, x = entitlementapp.RequestCompletionWithinTransaction(txCtx, ep, entitlementapp.CompletionInput{CheckpointID: entitlementapp.ID(row.CheckpointID.Bytes), ReservationID: entitlementapp.ID(row.ReservationID.Bytes), ExecutionID: entitlementapp.ID(row.ID.Bytes), CorrelationID: entitlementapp.ID(row.CorrelationID.Bytes), Operation: entitlementapp.CompletionSettle, ActualCredits: *row.EstimatedCredits}, [3]entitlementapp.ID{entitlementapp.ID(ids[0]), entitlementapp.ID(ids[1]), entitlementapp.ID(ids[2])}, at)
		recovered = x == nil
		return x
	})
	return
}

func (s *WorkflowStore) ClaimProposalDraft(ctx context.Context, at time.Time) (claim application.ProposalDraftClaim, found bool, err error) {
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		row, e := q.ClaimNextProposalDraft(txCtx, dbTime(at))
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		affected, e := q.StartProposalDraftJob(txCtx, capturesql.StartProposalDraftJobParams{StartedAt: dbTime(at), ID: row.JobID})
		if e != nil || affected != 1 {
			return affectedErr(e)
		}
		inputCount, e := q.CountProposalDraftInputs(txCtx, row.ID)
		if e != nil {
			return e
		}
		inputs, e := q.ListProposalDraftInputObjects(txCtx, capturesql.ListProposalDraftInputObjectsParams{RunID: row.ID, ObservedAt: dbTime(at)})
		if e != nil {
			return e
		}
		if row.InputCharacters == nil || row.EstimatedCredits == nil {
			return application.ErrProposalDraftConflict
		}
		claim = application.ProposalDraftClaim{RunID: appID(row.ID), JobID: appID(row.JobID), BidID: appID(row.BidID), ProposalID: appID(row.ProposalID), ProposalVersionID: appID(row.ProposalVersionID), RequestedBy: appID(row.RequestedBy), CorrelationID: appID(row.CorrelationID), TenantID: s.tenantID, CheckpointID: appID(row.CheckpointID), ReservationID: appID(row.ReservationID), AssignmentGeneration: s.assignmentGeneration, InputCharacters: *row.InputCharacters, EstimatedCredits: *row.EstimatedCredits, RequestedWorkspaceVersion: row.RequestedWorkspaceVersion, RequestedProposalVersion: row.RequestedProposalVersion, Model: row.Model, UsageKind: row.UsageKind, Inputs: make([]application.ResponseDraftInputObject, len(inputs)), InputsCurrent: int64(len(inputs)) == inputCount, SnapshotCurrent: row.SnapshotCurrent}
		for i, input := range inputs {
			var digest [32]byte
			copy(digest[:], input.Sha256)
			claim.Inputs[i] = application.ResponseDraftInputObject{DocumentVersionID: appID(input.DocumentVersionID), Ordinal: input.Ordinal, ObjectKey: input.ObjectKey, ObjectVersion: input.ObjectVersion, MediaType: input.MediaType, SizeBytes: input.SizeBytes, SHA256: digest}
		}
		found = true
		return nil
	})
	return claim, found, err
}
func (s *WorkflowStore) CancelProposalDraft(ctx context.Context, c application.ProposalDraftClaim, ids [3]application.ID, at time.Time) error {
	return s.finishProposalDraft(ctx, c, "CANCELLED", "", false, ids, at)
}
func (s *WorkflowStore) FailProposalDraft(ctx context.Context, c application.ProposalDraftClaim, code string, settle bool, ids [3]application.ID, at time.Time) error {
	return s.finishProposalDraft(ctx, c, "FAILED", code, settle, ids, at)
}
func (s *WorkflowStore) finishProposalDraft(ctx context.Context, c application.ProposalDraftClaim, state, code string, settle bool, ids [3]application.ID, at time.Time) error {
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		var failure *string
		if code != "" {
			failure = &code
		}
		affected, err := q.FailProposalDraftRun(txCtx, capturesql.FailProposalDraftRunParams{State: state, FailureCode: failure, CompletedAt: dbTime(at), ID: dbID(c.RunID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		affected, err = q.CompleteProposalDraftJob(txCtx, capturesql.CompleteProposalDraftJobParams{State: state, CompletedAt: dbTime(at), ID: dbID(c.JobID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		ep, err := entitlementpersistence.BindTransaction(db)
		if err != nil {
			return err
		}
		operation, actual := entitlementapp.CompletionRelease, int64(0)
		if settle {
			operation, actual = entitlementapp.CompletionSettle, c.EstimatedCredits
		}
		_, _, err = entitlementapp.RequestCompletionWithinTransaction(txCtx, ep, entitlementapp.CompletionInput{CheckpointID: entitlementapp.ID(c.CheckpointID), ReservationID: entitlementapp.ID(c.ReservationID), ExecutionID: entitlementapp.ID(c.RunID), CorrelationID: entitlementapp.ID(c.CorrelationID), Operation: operation, ActualCredits: actual}, [3]entitlementapp.ID{entitlementapp.ID(ids[0]), entitlementapp.ID(ids[1]), entitlementapp.ID(ids[2])}, at)
		return err
	})
}
func (s *WorkflowStore) CompleteProposalDraft(ctx context.Context, c application.ProposalDraftClaim, g application.GeneratedProposalDraft, actualCredits int64, suggestionID, auditID, outboxID application.ID, ids [3]application.ID, at time.Time) error {
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		if err := q.InsertProposalDraftSuggestion(txCtx, capturesql.InsertProposalDraftSuggestionParams{ID: dbID(suggestionID), RunID: dbID(c.RunID), Introduction: g.Introduction, Closing: g.Closing, Rationale: g.Rationale, CreatedAt: dbTime(at)}); err != nil {
			return err
		}
		for i, citation := range g.Citations {
			input, ok := draftInput(c.Inputs, citation.InputOrdinal)
			if !ok {
				return errors.New("proposal_draft_citation_invalid")
			}
			if err := q.InsertProposalDraftCitation(txCtx, capturesql.InsertProposalDraftCitationParams{SuggestionID: dbID(suggestionID), Ordinal: int32(i + 1), DocumentVersionID: dbID(input.DocumentVersionID), Location: citation.Location, Quote: citation.Quote}); err != nil {
				return err
			}
		}
		ep, err := entitlementpersistence.BindTransaction(db)
		if err != nil {
			return err
		}
		if _, _, err = entitlementapp.RequestCompletionWithinTransaction(txCtx, ep, entitlementapp.CompletionInput{CheckpointID: entitlementapp.ID(c.CheckpointID), ReservationID: entitlementapp.ID(c.ReservationID), ExecutionID: entitlementapp.ID(c.RunID), CorrelationID: entitlementapp.ID(c.CorrelationID), Operation: entitlementapp.CompletionSettle, ActualCredits: actualCredits}, [3]entitlementapp.ID{entitlementapp.ID(ids[0]), entitlementapp.ID(ids[1]), entitlementapp.ID(ids[2])}, at); err != nil {
			return err
		}
		responseID := g.ProviderResponseID
		affected, err := q.CompleteProposalDraftRun(txCtx, capturesql.CompleteProposalDraftRunParams{ProviderResponseID: &responseID, CompletedAt: dbTime(at), ID: dbID(c.RunID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		affected, err = q.CompleteProposalDraftJob(txCtx, capturesql.CompleteProposalDraftJobParams{State: "SUCCEEDED", CompletedAt: dbTime(at), ID: dbID(c.JobID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		if err = q.InsertProposalDraftAudit(txCtx, capturesql.InsertProposalDraftAuditParams{ID: dbID(auditID), ActorID: dbID(c.RequestedBy), Action: "AIProposalDraftProposed", RunID: dbID(c.RunID), OccurredAt: dbTime(at), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), ProposalVersionID: dbID(c.ProposalVersionID), JobID: dbID(c.JobID)}); err != nil {
			return err
		}
		return q.InsertProposalDraftOutbox(txCtx, capturesql.InsertProposalDraftOutboxParams{ID: dbID(outboxID), RunID: dbID(c.RunID), EventType: "ai.proposal_draft.proposed.v1", JobID: dbID(c.JobID), BidID: dbID(c.BidID), ProposalVersionID: dbID(c.ProposalVersionID), OccurredAt: dbTime(at), ActorID: dbID(c.RequestedBy)})
	})
}
