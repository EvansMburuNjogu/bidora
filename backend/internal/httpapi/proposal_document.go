package httpapi

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const ProposalDocumentPath = "/v1/tenderpreneur/bids/:bid_id/proposal"
const ProposalHistoryPath = "/v1/tenderpreneur/bids/:bid_id/proposal/history"

type ProposalUseCase interface {
	Save(context.Context, opportunityapp.SaveProposalInput) (opportunityapp.ProposalSaveResult, error)
	Get(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.ProposalDocument, error)
	History(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) ([]opportunityapp.ProposalHistoryItem, error)
}
type ProposalHandler struct{ proposals ProposalUseCase }

func NewProposalHandler(p ProposalUseCase) (*ProposalHandler, error) {
	if p == nil {
		return nil, errors.New("proposal_handler_invalid")
	}
	return &ProposalHandler{p}, nil
}

type saveProposalBody struct {
	Title                    string   `json:"title"`
	Recipient                string   `json:"recipient"`
	LetterheadLines          []string `json:"letterhead_lines"`
	Introduction             string   `json:"introduction"`
	Closing                  string   `json:"closing"`
	ChangeNote               string   `json:"change_note"`
	ExpectedProposalVersion  int64    `json:"expected_proposal_version"`
	ExpectedWorkspaceVersion int64    `json:"expected_workspace_version"`
}

func (h *ProposalHandler) Save(c fiber.Ctx) error {
	var body saveProposalBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	result, err := h.proposals.Save(c.Context(), opportunityapp.SaveProposalInput{Subject: common.subject, BidID: common.opportunityID, CorrelationID: common.correlationID, Title: body.Title, Recipient: body.Recipient, LetterheadLines: body.LetterheadLines, Introduction: body.Introduction, Closing: body.Closing, ChangeNote: body.ChangeNote, ExpectedProposalVersion: body.ExpectedProposalVersion, ExpectedWorkspaceVersion: body.ExpectedWorkspaceVersion, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return proposalFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"proposal": proposalView(result.Proposal), "replay": result.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ProposalHandler) Get(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	result, err := h.proposals.Get(c.Context(), subject, bidID)
	if err != nil {
		return proposalFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"proposal": proposalView(result)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ProposalHandler) History(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	rows, err := h.proposals.History(c.Context(), subject, bidID)
	if err != nil {
		return proposalFailure(c, err)
	}
	items := make([]fiber.Map, len(rows))
	for i, row := range rows {
		items[i] = fiber.Map{"proposal_version_id": uuid.UUID(row.ProposalVersionID).String(), "revision": row.Revision, "title": row.Title, "change_note": row.ChangeNote, "created_by": uuid.UUID(row.CreatedBy).String(), "created_at": row.CreatedAt.Format(time.RFC3339Nano)}
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"versions": items}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func proposalReadCommon(c fiber.Ctx) (opportunityapp.WorkflowSubject, opportunityapp.ID, bool) {
	subject, ok := readSubject(c)
	if !ok {
		return opportunityapp.WorkflowSubject{}, opportunityapp.ID{}, false
	}
	if len(c.Request().URI().QueryString()) != 0 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
		return opportunityapp.WorkflowSubject{}, opportunityapp.ID{}, false
	}
	bidID, ok := parseOpportunityID(c.Params("bid_id"))
	if !ok {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return opportunityapp.WorkflowSubject{}, opportunityapp.ID{}, false
	}
	return subject, bidID, true
}
func proposalView(v opportunityapp.ProposalDocument) fiber.Map {
	responses := make([]fiber.Map, len(v.Responses))
	for i, row := range v.Responses {
		responses[i] = fiber.Map{"ordinal": row.Ordinal, "requirement_id": uuid.UUID(row.RequirementID).String(), "requirement_text": row.RequirementText, "category": row.Category, "mandatory": row.Mandatory, "response_version_id": uuid.UUID(row.ResponseVersionID).String(), "response_revision": row.ResponseRevision, "content": row.Content, "created_at": row.CreatedAt.Format(time.RFC3339Nano)}
	}
	var letterheadVersionID, letterheadVersionNumber, letterheadTemplateKey, letterheadHash any
	if v.LetterheadVersionID != nil && v.LetterheadVersionNumber != nil && v.LetterheadTemplateKey != nil && v.LetterheadContentHash != nil {
		letterheadVersionID = uuid.UUID(*v.LetterheadVersionID).String()
		letterheadVersionNumber = *v.LetterheadVersionNumber
		letterheadTemplateKey = *v.LetterheadTemplateKey
		letterheadHash = hex.EncodeToString(v.LetterheadContentHash[:])
	}
	return fiber.Map{"proposal_id": uuid.UUID(v.ProposalID).String(), "proposal_version_id": uuid.UUID(v.ProposalVersionID).String(), "bid_id": uuid.UUID(v.BidID).String(), "letterhead_version_id": letterheadVersionID, "letterhead_version_number": letterheadVersionNumber, "letterhead_template_key": letterheadTemplateKey, "letterhead_content_hash": letterheadHash, "organization_legal_name": v.OrganizationLegalName, "title": v.Title, "recipient": v.Recipient, "letterhead_lines": v.LetterheadLines, "introduction": v.Introduction, "responses": responses, "closing": v.Closing, "change_note": v.ChangeNote, "proposal_version": v.ProposalVersion, "revision": v.Revision, "workspace_version": v.WorkspaceVersion, "created_by": uuid.UUID(v.CreatedBy).String(), "created_at": v.CreatedAt.Format(time.RFC3339Nano)}
}
func proposalFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "proposal_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to access proposals.", false)
	case "proposal_invalid", "proposal_read_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The proposal request is invalid.", false)
	case "proposal_not_found":
		return fail(c, 404, "NOT_FOUND", "The proposal was not found.", false)
	case "proposal_conflict":
		return fail(c, 409, "PROPOSAL_CONFLICT", "The proposal, responses, or workspace changed.", false)
	default:
		return fail(c, 503, "PROPOSAL_UNAVAILABLE", "The proposal service is temporarily unavailable.", true)
	}
}
