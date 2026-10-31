package persistence

import (
	"context"
	"errors"
	"time"

	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *WorkflowStore) SaveTechnicalResponse(ctx context.Context, a application.WorkflowAuthority, c application.TechnicalResponseCommit) (result application.TechnicalResponseResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, workspaceVersion, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "SAVE_TECHNICAL_RESPONSE", c.KeyHash, c.RequestHash, c.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			row, err := q.GetTechnicalResponseByVersion(txCtx, dbID(ref))
			if err != nil {
				return err
			}
			mapped, err := mapTechnicalResponse(txCtx, q, row.SectionID, row.BidID, row.RequirementID, row.SectionVersion, row.ResponseVersionID, row.Revision, row.Content, row.ChangeNote, row.CreatedBy, row.CreatedAt)
			if err != nil {
				return err
			}
			result = application.TechnicalResponseResult{Response: mapped, WorkspaceVersion: workspaceVersion, Replay: true}
			return nil
		}
		locked, err := q.LockRequirementForResponse(txCtx, capturesql.LockRequirementForResponseParams{RequirementID: dbID(c.RequirementID), BidID: dbID(c.BidID), ExpectedWorkspaceVersion: c.ExpectedWorkspaceVersion})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrTechnicalResponseConflict
		}
		if err != nil {
			return err
		}
		sectionID := c.SectionID
		sectionVersion := int64(1)
		revision := int32(1)
		section, lookupErr := q.GetResponseSectionForUpdate(txCtx, capturesql.GetResponseSectionForUpdateParams{BidID: locked.BidID, RequirementID: locked.ID})
		if c.ExpectedSectionVersion == 0 {
			if lookupErr == nil {
				return application.ErrTechnicalResponseConflict
			}
			if !errors.Is(lookupErr, pgx.ErrNoRows) {
				return lookupErr
			}
			if err = q.InsertResponseSection(txCtx, capturesql.InsertResponseSectionParams{ID: dbID(sectionID), BidID: locked.BidID, RequirementID: locked.ID, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
				return err
			}
		} else {
			if errors.Is(lookupErr, pgx.ErrNoRows) {
				return application.ErrTechnicalResponseConflict
			}
			if lookupErr != nil {
				return lookupErr
			}
			if section.Version != c.ExpectedSectionVersion {
				return application.ErrTechnicalResponseConflict
			}
			sectionID = appID(section.ID)
			sectionVersion = c.ExpectedSectionVersion + 1
			revision = int32(sectionVersion)
		}
		if err = q.InsertResponseVersion(txCtx, capturesql.InsertResponseVersionParams{ID: dbID(c.ResponseVersionID), SectionID: dbID(sectionID), Revision: revision, RequirementVersion: locked.RequirementVersion, Content: c.Content, ChangeNote: c.ChangeNote, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		var affected int64
		if c.ExpectedSectionVersion == 0 {
			affected, err = q.SetInitialResponseRevision(txCtx, capturesql.SetInitialResponseRevisionParams{RevisionID: dbID(c.ResponseVersionID), UpdatedAt: dbTime(c.OccurredAt), ID: dbID(sectionID)})
		} else {
			affected, err = q.AdvanceResponseSection(txCtx, capturesql.AdvanceResponseSectionParams{RevisionID: dbID(c.ResponseVersionID), UpdatedAt: dbTime(c.OccurredAt), ID: dbID(sectionID), ExpectedVersion: c.ExpectedSectionVersion})
		}
		if err != nil {
			return err
		}
		if affected != 1 {
			return application.ErrTechnicalResponseConflict
		}
		for _, evidenceID := range c.EvidenceIDs {
			affected, err = q.InsertResponseEvidence(txCtx, capturesql.InsertResponseEvidenceParams{ResponseVersionID: dbID(c.ResponseVersionID), EvidenceID: dbID(evidenceID), RequirementID: locked.ID, BidID: locked.BidID, ObservedAt: dbTime(c.OccurredAt)})
			if err != nil {
				return err
			}
			if affected != 1 {
				return application.ErrTechnicalResponseConflict
			}
		}
		newWorkspaceVersion := c.ExpectedWorkspaceVersion + 1
		affected, err = q.AdvanceWorkspaceForRequirementChange(txCtx, capturesql.AdvanceWorkspaceForRequirementChangeParams{UpdatedAt: dbTime(c.OccurredAt), ID: locked.BidID, ExpectedVersion: c.ExpectedWorkspaceVersion})
		if err != nil {
			return err
		}
		if affected != 1 {
			return application.ErrTechnicalResponseConflict
		}
		if err = q.InsertTechnicalResponseAudit(txCtx, capturesql.InsertTechnicalResponseAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), SectionID: dbID(sectionID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: locked.BidID, RequirementID: locked.ID, ResponseVersionID: dbID(c.ResponseVersionID), Revision: revision, SectionVersion: sectionVersion, WorkspaceVersion: newWorkspaceVersion}); err != nil {
			return err
		}
		if err = q.InsertTechnicalResponseOutbox(txCtx, capturesql.InsertTechnicalResponseOutboxParams{ID: dbID(c.OutboxID), BidID: locked.BidID, WorkspaceVersion: newWorkspaceVersion, RequirementID: locked.ID, SectionID: dbID(sectionID), ResponseVersionID: dbID(c.ResponseVersionID), Revision: revision, OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.ResponseVersionID, newWorkspaceVersion, c.OccurredAt); err != nil {
			return err
		}
		row, err := q.GetTechnicalResponseByVersion(txCtx, dbID(c.ResponseVersionID))
		if err != nil {
			return err
		}
		mapped, err := mapTechnicalResponse(txCtx, q, row.SectionID, row.BidID, row.RequirementID, row.SectionVersion, row.ResponseVersionID, row.Revision, row.Content, row.ChangeNote, row.CreatedBy, row.CreatedAt)
		if err != nil {
			return err
		}
		result = application.TechnicalResponseResult{Response: mapped, WorkspaceVersion: newWorkspaceVersion}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetTechnicalResponse(ctx context.Context, a application.WorkflowAuthority, bidID, requirementID application.ID) (result application.TechnicalResponse, err error) {
	if err = s.validRead(ctx, a, 1); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		row, err := q.GetCurrentTechnicalResponse(txCtx, capturesql.GetCurrentTechnicalResponseParams{BidID: dbID(bidID), RequirementID: dbID(requirementID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrTechnicalResponseNotFound
		}
		if err != nil {
			return err
		}
		result, err = mapTechnicalResponse(txCtx, q, row.SectionID, row.BidID, row.RequirementID, row.SectionVersion, row.ResponseVersionID, row.Revision, row.Content, row.ChangeNote, row.CreatedBy, row.CreatedAt)
		return err
	})
	return result, err
}
func (s *WorkflowStore) ListTechnicalResponseHistory(ctx context.Context, a application.WorkflowAuthority, bidID, requirementID application.ID) (result []application.TechnicalResponse, err error) {
	if err = s.validRead(ctx, a, 100); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		rows, err := q.ListTechnicalResponseHistory(txCtx, capturesql.ListTechnicalResponseHistoryParams{BidID: dbID(bidID), RequirementID: dbID(requirementID)})
		if err != nil {
			return err
		}
		result = make([]application.TechnicalResponse, len(rows))
		for i, row := range rows {
			result[i], err = mapTechnicalResponse(txCtx, q, row.SectionID, row.BidID, row.RequirementID, row.SectionVersion, row.ResponseVersionID, row.Revision, row.Content, row.ChangeNote, row.CreatedBy, row.CreatedAt)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetGapReview(ctx context.Context, a application.WorkflowAuthority, bidID application.ID, observed time.Time) (result application.GapReview, err error) {
	if err = s.validRead(ctx, a, 1000); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		bid, err := q.GetBidForGapReview(txCtx, dbID(bidID))
		if err != nil {
			return err
		}
		count, err := q.CountBidRequirementsForGap(txCtx, dbID(bidID))
		if err != nil {
			return err
		}
		rows, err := q.GetBidGapInputs(txCtx, capturesql.GetBidGapInputsParams{ObservedAt: dbTime(observed), BidID: dbID(bidID)})
		if err != nil {
			return err
		}
		result = application.GapReview{BidID: appID(bid.ID), WorkspaceState: bid.State, Ready: true, WorkspaceBlockers: []string{}, Requirements: make([]application.RequirementGap, 0, len(rows)), ObservedAt: observed}
		result.RequirementCount = int(count)
		if count > 1000 {
			result.WorkspaceBlockers = append(result.WorkspaceBlockers, "REQUIREMENT_REVIEW_LIMIT_EXCEEDED")
			result.Ready = false
			result.BlockerCount++
		}
		if bid.State != "BID" {
			result.WorkspaceBlockers = append(result.WorkspaceBlockers, "BID_DECISION_REQUIRED")
			result.Ready = false
			result.BlockerCount++
		}
		for _, row := range rows {
			gap := application.RequirementGap{RequirementID: appID(row.RequirementID), RequirementText: row.Text, Mandatory: row.Mandatory, RequirementStatus: row.Status, RequirementVersion: row.RequirementVersion, Blockers: []string{}}
			if row.Mandatory && !row.CurrentRevisionID.Valid {
				gap.Blockers = append(gap.Blockers, "MISSING_TECHNICAL_RESPONSE")
			}
			if row.CurrentRevisionID.Valid && (row.ResponseCurrent == nil || !*row.ResponseCurrent) {
				gap.Blockers = append(gap.Blockers, "TECHNICAL_RESPONSE_OUTDATED")
			}
			effectiveOpen := row.Status != "SATISFIED" || !row.EvidenceCurrent
			if row.Status == "SATISFIED" && row.EvidenceExpired && !row.EvidenceCurrent {
				gap.Blockers = append(gap.Blockers, "EVIDENCE_EXPIRED")
			}
			if row.Mandatory && effectiveOpen {
				gap.Blockers = append(gap.Blockers, "MANDATORY_REQUIREMENT_UNSATISFIED")
			}
			if effectiveOpen && !row.OwnerID.Valid {
				gap.Blockers = append(gap.Blockers, "REQUIREMENT_UNASSIGNED")
			}
			if effectiveOpen && row.DueAt.Valid && row.DueAt.Time.Before(observed) {
				gap.Blockers = append(gap.Blockers, "REQUIREMENT_OVERDUE")
			}
			if len(gap.Blockers) > 0 {
				result.Ready = false
				result.BlockerCount += len(gap.Blockers)
			}
			result.Requirements = append(result.Requirements, gap)
		}
		return nil
	})
	return result, err
}
func mapTechnicalResponse(ctx context.Context, q *capturesql.Queries, sectionID, bidID, requirementID pgtype.UUID, sectionVersion int64, responseVersionID pgtype.UUID, revision int32, content, changeNote string, createdBy pgtype.UUID, createdAt pgtype.Timestamptz) (application.TechnicalResponse, error) {
	evidence, err := q.ListResponseVersionEvidence(ctx, responseVersionID)
	if err != nil {
		return application.TechnicalResponse{}, err
	}
	ids := make([]application.ID, len(evidence))
	for i, id := range evidence {
		ids[i] = appID(id)
	}
	return application.TechnicalResponse{SectionID: appID(sectionID), BidID: appID(bidID), RequirementID: appID(requirementID), ResponseVersionID: appID(responseVersionID), SectionVersion: sectionVersion, Revision: revision, Content: content, ChangeNote: changeNote, CreatedBy: appID(createdBy), CreatedAt: createdAt.Time.UTC(), EvidenceIDs: ids}, nil
}
