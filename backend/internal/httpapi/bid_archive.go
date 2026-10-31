package httpapi

import (
	app "bidora/backend/internal/opportunity/application"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"time"
)

const BidArchivePath = "/v1/tenderpreneur/bids/:bid_id/archive"

type BidArchiveUseCase interface {
	Archive(context.Context, app.ArchiveBidInput) (app.BidArchiveResult, error)
	Get(context.Context, app.WorkflowSubject, app.ID) (app.BidArchive, error)
}
type BidArchiveHandler struct{ s BidArchiveUseCase }

func NewBidArchiveHandler(s BidArchiveUseCase) (*BidArchiveHandler, error) {
	if s == nil {
		return nil, errors.New("bid_archive_handler_invalid")
	}
	return &BidArchiveHandler{s}, nil
}

type bidArchiveBody struct {
	Reason                   string `json:"reason"`
	ExpectedWorkspaceVersion int64  `json:"expected_workspace_version"`
}

func (h *BidArchiveHandler) Archive(c fiber.Ctx) error {
	var b bidArchiveBody
	if e := bindWorkflow(c, &b); e != nil {
		return e
	}
	if !requireWorkflowFields(c.Body(), []string{"reason", "expected_workspace_version"}) {
		return fail(c, 400, "INVALID_REQUEST", "The archive request is invalid.", false)
	}
	x, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	v, e := h.s.Archive(c.Context(), app.ArchiveBidInput{Subject: x.subject, BidID: x.opportunityID, CorrelationID: x.correlationID, ExpectedWorkspaceVersion: b.ExpectedWorkspaceVersion, Reason: b.Reason, IdempotencyKey: x.idempotencyKey})
	if e != nil {
		return bidArchiveFailure(c, e)
	}
	if v.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(201).JSON(fiber.Map{"data": bidArchiveView(v.Archive), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *BidArchiveHandler) Get(c fiber.Ctx) error {
	sub, bid, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	v, e := h.s.Get(c.Context(), sub, bid)
	if e != nil {
		return bidArchiveFailure(c, e)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": bidArchiveView(v), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func bidArchiveView(v app.BidArchive) fiber.Map {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "bid_id": uuid.UUID(v.BidID).String(), "workspace_version": v.WorkspaceVersion, "archived_by": uuid.UUID(v.ArchivedBy).String(), "reason": v.Reason, "policy_version": v.PolicyVersion, "archived_at": v.ArchivedAt.Format(time.RFC3339Nano), "archived": true, "read_only": true, "deleted": false, "retention_disposition_started": false, "legal_hold_changed": false, "restoration_supported": false}
}
func bidArchiveFailure(c fiber.Ctx, e error) error {
	switch e.Error() {
	case "bid_archive_invalid", "bid_archive_read_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The archive request is invalid.", false)
	case "bid_archive_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to archive this bid.", false)
	case "bid_archive_not_found":
		return fail(c, 404, "ARCHIVE_NOT_FOUND", "The archive record was not found.", false)
	case "bid_archive_conflict":
		return fail(c, 409, "ARCHIVE_BLOCKED", "The workspace is not eligible for archive or changed.", false)
	default:
		return fail(c, 503, "ARCHIVE_UNAVAILABLE", "Archive is temporarily unavailable.", true)
	}
}
