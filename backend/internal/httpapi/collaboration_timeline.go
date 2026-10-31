package httpapi

import (
	"bidora/backend/internal/opportunity/application"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const CollaborationTimelinePath = "/v1/tenderpreneur/bids/:bid_id/activity"

type CollaborationTimelineUseCase interface {
	List(context.Context, application.CollaborationTimelineInput) (application.CollaborationTimelinePage, error)
}
type CollaborationTimelineHandler struct{ service CollaborationTimelineUseCase }

func NewCollaborationTimelineHandler(service CollaborationTimelineUseCase) (*CollaborationTimelineHandler, error) {
	if service == nil {
		return nil, errors.New("collaboration_timeline_handler_invalid")
	}
	return &CollaborationTimelineHandler{service}, nil
}
func (h *CollaborationTimelineHandler) List(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommonWithoutQuery(c)
	if !ok {
		return nil
	}
	limit, cursor, values, ok := readPage(c)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The activity timeline request is invalid.", false)
	}
	for key := range values {
		if key != "limit" && key != "cursor" {
			return fail(c, 400, "INVALID_REQUEST", "The activity timeline request is invalid.", false)
		}
	}
	page, err := h.service.List(c.Context(), application.CollaborationTimelineInput{Subject: subject, BidID: bidID, Cursor: cursor, Limit: int32(limit)})
	if err != nil {
		return collaborationTimelineFailure(c, err)
	}
	items := make([]fiber.Map, len(page.Items))
	for i, row := range page.Items {
		items[i] = fiber.Map{"id": uuid.UUID(row.ID).String(), "event_type": row.EventType, "actor_id": uuid.UUID(row.ActorID).String(), "occurred_at": row.OccurredAt.Format("2006-01-02T15:04:05.999999999Z07:00"), "target_type": row.TargetType, "target_id": uuid.UUID(row.TargetID).String(), "event_version": row.Version, "detail_code": row.DetailCode, "related_id": optionalUUID(row.RelatedID), "due_at": formatOptionalTime(row.DueAt)}
	}
	var next *string
	if page.HasMore && len(page.Items) > 0 {
		encoded, err := encodeReadCursor(page.Items[len(page.Items)-1].OccurredAt, page.Items[len(page.Items)-1].ID)
		if err != nil {
			return fail(c, 503, "COLLABORATION_TIMELINE_UNAVAILABLE", "The activity timeline is temporarily unavailable.", true)
		}
		next = &encoded
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"activities": items, "next_cursor": next}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func proposalReadCommonWithoutQuery(c fiber.Ctx) (application.WorkflowSubject, application.ID, bool) {
	subject, ok := readSubject(c)
	if !ok {
		return application.WorkflowSubject{}, application.ID{}, false
	}
	bidID, ok := parseOpportunityID(c.Params("bid_id"))
	if !ok {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return application.WorkflowSubject{}, application.ID{}, false
	}
	return subject, bidID, true
}
func collaborationTimelineFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "collaboration_timeline_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The activity timeline request is invalid.", false)
	case "collaboration_timeline_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to read this activity timeline.", false)
	case "collaboration_timeline_not_found":
		return fail(c, 404, "ACTIVITY_TIMELINE_NOT_FOUND", "The bid activity timeline was not found.", false)
	case "collaboration_timeline_unavailable":
		return fail(c, 503, "COLLABORATION_TIMELINE_UNAVAILABLE", "The activity timeline is temporarily unavailable.", true)
	default:
		return fail(c, 503, "COLLABORATION_TIMELINE_UNAVAILABLE", "The activity timeline is temporarily unavailable.", true)
	}
}
