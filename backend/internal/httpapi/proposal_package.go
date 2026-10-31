package httpapi

import (
	"bidora/backend/internal/opportunity/application"
	"context"
	"encoding/hex"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"time"
)

const ProposalPackagePreparationPath = "/v1/tenderpreneur/bids/:bid_id/proposal/package-preparations"
const ProposalPackageManifestPath = "/v1/tenderpreneur/bids/:bid_id/proposal/package-preparations/:manifest_id"

type ProposalPackageUseCase interface {
	Prepare(context.Context, application.PrepareProposalPackageInput) (application.ProposalPackageResult, error)
	Get(context.Context, application.WorkflowSubject, application.ID, application.ID) (application.ProposalPackageManifest, error)
	List(context.Context, application.ProposalPackageHistoryInput) (application.ProposalPackageHistoryPage, error)
}
type ProposalPackageHandler struct{ service ProposalPackageUseCase }

func NewProposalPackageHandler(service ProposalPackageUseCase) (*ProposalPackageHandler, error) {
	if service == nil {
		return nil, errors.New("proposal_package_handler_invalid")
	}
	return &ProposalPackageHandler{service}, nil
}

type proposalPackageBody struct {
	ProposalVersionID        string `json:"proposal_version_id"`
	ExpectedProposalVersion  int64  `json:"expected_proposal_version"`
	ExpectedWorkspaceVersion int64  `json:"expected_workspace_version"`
}

func (h *ProposalPackageHandler) Prepare(c fiber.Ctx) error {
	var body proposalPackageBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	versionID, ok := parseOpportunityID(body.ProposalVersionID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The package preparation request is invalid.", false)
	}
	result, err := h.service.Prepare(c.Context(), application.PrepareProposalPackageInput{Subject: common.subject, BidID: common.opportunityID, ProposalVersionID: versionID, CorrelationID: common.correlationID, ExpectedProposalVersion: body.ExpectedProposalVersion, ExpectedWorkspaceVersion: body.ExpectedWorkspaceVersion, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return proposalPackageFailure(c, err)
	}
	if result.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(201).JSON(fiber.Map{"data": proposalPackageView(result.Manifest), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ProposalPackageHandler) List(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommonWithoutQuery(c)
	if !ok {
		return nil
	}
	limit, cursor, values, ok := readPage(c)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The package history request is invalid.", false)
	}
	for key := range values {
		if key != "limit" && key != "cursor" {
			return fail(c, 400, "INVALID_REQUEST", "The package history request is invalid.", false)
		}
	}
	page, err := h.service.List(c.Context(), application.ProposalPackageHistoryInput{Subject: subject, BidID: bidID, Cursor: cursor, Limit: int32(limit)})
	if err != nil {
		return proposalPackageFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	items := make([]fiber.Map, len(page.Items))
	for i, row := range page.Items {
		items[i] = proposalPackageView(row)
	}
	var next *string
	if page.HasMore && len(page.Items) > 0 {
		encoded, err := encodeReadCursor(page.Items[len(page.Items)-1].CreatedAt, page.Items[len(page.Items)-1].ID)
		if err != nil {
			return fail(c, 503, "PROPOSAL_PACKAGE_UNAVAILABLE", "Package history is temporarily unavailable.", true)
		}
		next = &encoded
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"package_preparations": items, "next_cursor": next}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ProposalPackageHandler) Get(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	manifestID, ok := parseOpportunityID(c.Params("manifest_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The package manifest request is invalid.", false)
	}
	result, err := h.service.Get(c.Context(), subject, bidID, manifestID)
	if err != nil {
		return proposalPackageFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": proposalPackageView(result), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func proposalPackageView(v application.ProposalPackageManifest) fiber.Map {
	var letterheadVersionID, letterheadVersionNumber, letterheadTemplateKey, letterheadHash any
	if v.LetterheadVersionID != nil && v.LetterheadVersionNumber != nil && v.LetterheadTemplateKey != nil && v.LetterheadContentHash != nil {
		letterheadVersionID = uuid.UUID(*v.LetterheadVersionID).String()
		letterheadVersionNumber = *v.LetterheadVersionNumber
		letterheadTemplateKey = *v.LetterheadTemplateKey
		letterheadHash = hex.EncodeToString(v.LetterheadContentHash[:])
	}
	return fiber.Map{"manifest_id": uuid.UUID(v.ID).String(), "bid_id": uuid.UUID(v.BidID).String(), "proposal_id": uuid.UUID(v.ProposalID).String(), "proposal_version_id": uuid.UUID(v.ProposalVersionID).String(), "proposal_revision": v.ProposalRevision, "letterhead_version_id": letterheadVersionID, "letterhead_version_number": letterheadVersionNumber, "letterhead_template_key": letterheadTemplateKey, "letterhead_content_hash": letterheadHash, "package_status": v.Status, "manifest_version": v.ManifestVersion, "manifest_sha256": hex.EncodeToString(v.ManifestSHA256[:]), "pdf": fiber.Map{"sha256": hex.EncodeToString(v.PDFSHA256[:]), "size": v.PDFSize, "content_type": "application/pdf", "render_profile": v.PDFRenderProfile, "retrieval": fiber.Map{"method": "GET", "path": "/v1/tenderpreneur/bids/" + uuid.UUID(v.BidID).String() + "/proposal/versions/" + uuid.UUID(v.ProposalVersionID).String() + "/export/pdf", "expected_sha256": hex.EncodeToString(v.PDFSHA256[:])}}, "docx": fiber.Map{"sha256": hex.EncodeToString(v.DOCXSHA256[:]), "size": v.DOCXSize, "content_type": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "render_profile": v.DOCXRenderProfile, "retrieval": fiber.Map{"method": "GET", "path": "/v1/tenderpreneur/bids/" + uuid.UUID(v.BidID).String() + "/proposal/versions/" + uuid.UUID(v.ProposalVersionID).String() + "/export/docx", "expected_sha256": hex.EncodeToString(v.DOCXSHA256[:])}}, "readiness": fiber.Map{"observed_at": v.ReadinessObservedAt.Format(time.RFC3339Nano), "ready_for_approval_review": v.ReadinessReady, "blockers": v.ReadinessBlockers}, "approved": false, "signed": false, "submitted": false, "created_by": uuid.UUID(v.CreatedBy).String(), "created_at": v.CreatedAt.Format(time.RFC3339Nano)}
}
func proposalPackageFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "proposal_package_invalid", "proposal_package_read_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The package preparation request is invalid.", false)
	case "proposal_package_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to prepare this package.", false)
	case "proposal_package_not_found":
		return fail(c, 404, "PROPOSAL_PACKAGE_NOT_FOUND", "The package preparation manifest was not found.", false)
	case "proposal_package_conflict":
		return fail(c, 409, "PROPOSAL_PACKAGE_CONFLICT", "The proposal, workspace or idempotency request changed.", false)
	case "proposal_package_unsupported":
		return fail(c, 422, "PROPOSAL_PACKAGE_UNSUPPORTED", "The proposal contains characters unsupported by the deterministic PDF profile.", false)
	case "proposal_package_too_large":
		return fail(c, 422, "PROPOSAL_PACKAGE_TOO_LARGE", "The prepared artifact exceeds the package limit.", false)
	case "proposal_package_unavailable", "proposal_package_read_unavailable":
		return fail(c, 503, "PROPOSAL_PACKAGE_UNAVAILABLE", "Proposal package preparation is temporarily unavailable.", true)
	default:
		return fail(c, 503, "PROPOSAL_PACKAGE_UNAVAILABLE", "Proposal package preparation is temporarily unavailable.", true)
	}
}
