package httpapi

import (
	"bidora/backend/internal/opportunity/application"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"time"
)

const ArchivePreparationPath = "/v1/tenderpreneur/bids/:bid_id/archive-preparation"

type ArchivePreparationUseCase interface {
	Get(context.Context, application.WorkflowSubject, application.ID) (application.BidArchivePreparation, error)
}
type ArchivePreparationHandler struct{ service ArchivePreparationUseCase }

func NewArchivePreparationHandler(service ArchivePreparationUseCase) (*ArchivePreparationHandler, error) {
	if service == nil {
		return nil, errors.New("archive_preparation_handler_invalid")
	}
	return &ArchivePreparationHandler{service}, nil
}
func (h *ArchivePreparationHandler) Get(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	result, err := h.service.Get(c.Context(), subject, bidID)
	if err != nil {
		return archivePreparationFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"bid_id": uuid.UUID(result.BidID).String(), "workspace_state": result.WorkspaceState, "workspace_version": result.WorkspaceVersion, "archived_at": formatOptionalTime(result.ArchivedAt), "current_proposal_version_id": optionalUUID(result.ProposalVersionID), "requirement_count": result.RequirementCount, "open_requirement_count": result.OpenRequirementCount, "open_review_thread_count": result.OpenReviewThreadCount, "pending_upload_count": result.PendingUploadCount, "pending_background_job_count": result.PendingJobCount, "package_manifest_count": result.PackageManifestCount, "external_report_count": result.ExternalReportCount, "active_external_report_count": result.ActiveExternalReportCount, "has_submission_confirmation": result.HasSubmissionConfirmation, "current_reported_outcome": result.CurrentReportedOutcome, "archive_policy_configured": result.ArchivePolicyConfigured, "read_only_closure_supported": result.ReadOnlyClosureSupported, "can_archive": result.CanArchive, "blockers": result.Blockers, "observed_at": result.ObservedAt.Format(time.RFC3339Nano)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func archivePreparationFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "archive_preparation_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The archive preparation request is invalid.", false)
	case "archive_preparation_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to read this archive preparation.", false)
	case "archive_preparation_not_found":
		return fail(c, 404, "ARCHIVE_PREPARATION_NOT_FOUND", "The bid archive preparation was not found.", false)
	case "archive_preparation_unavailable":
		return fail(c, 503, "ARCHIVE_PREPARATION_UNAVAILABLE", "Archive preparation is temporarily unavailable.", true)
	default:
		return fail(c, 503, "ARCHIVE_PREPARATION_UNAVAILABLE", "Archive preparation is temporarily unavailable.", true)
	}
}
