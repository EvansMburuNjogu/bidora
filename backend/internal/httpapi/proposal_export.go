package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
)

const ProposalExportPath = "/v1/tenderpreneur/bids/:bid_id/proposal/versions/:proposal_version_id/export/:format"

type ProposalExportUseCase interface {
	Export(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID, string) (opportunityapp.ProposalExport, error)
}
type ProposalExportHandler struct{ exports ProposalExportUseCase }

func NewProposalExportHandler(v ProposalExportUseCase) (*ProposalExportHandler, error) {
	if v == nil {
		return nil, errors.New("proposal_export_handler_invalid")
	}
	return &ProposalExportHandler{v}, nil
}
func (h *ProposalExportHandler) Download(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	versionID, ok := parseOpportunityID(c.Params("proposal_version_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The export request is invalid.", false)
	}
	result, err := h.exports.Export(c.Context(), subject, bidID, versionID, c.Params("format"))
	if err != nil {
		return proposalExportFailure(c, err)
	}
	digestHex := hex.EncodeToString(result.SHA256[:])
	c.Set("Cache-Control", "private, no-store")
	c.Set("Content-Type", result.ContentType)
	c.Set("Content-Disposition", `attachment; filename="proposal-revision-`+strconv.FormatInt(result.Revision, 10)+`.`+result.Extension+`"`)
	c.Set("ETag", `"sha256-`+digestHex+`"`)
	c.Set("Digest", "sha-256="+base64.StdEncoding.EncodeToString(result.SHA256[:]))
	c.Set("X-Bidora-Content-SHA256", digestHex)
	return c.Send(result.Bytes)
}
func proposalExportFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "proposal_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to export this proposal.", false)
	case "proposal_export_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The export request is invalid.", false)
	case "proposal_not_found":
		return fail(c, 404, "NOT_FOUND", "The proposal revision was not found.", false)
	case "proposal_export_unsupported":
		return fail(c, 422, "EXPORT_TEXT_UNSUPPORTED", "The proposal contains text that cannot be represented safely in this export format.", false)
	case "proposal_export_too_large":
		return fail(c, 413, "EXPORT_TOO_LARGE", "The proposal is too large to export.", false)
	default:
		return fail(c, 503, "PROPOSAL_EXPORT_UNAVAILABLE", "Proposal export is temporarily unavailable.", true)
	}
}
