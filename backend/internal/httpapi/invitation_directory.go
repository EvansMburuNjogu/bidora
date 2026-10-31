package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type InvitationDirectoryUseCase interface {
	List(context.Context, authzapp.InvitationDirectoryQuery) (authzapp.InvitationPage, error)
}
type InvitationDirectoryHandler struct{ directory InvitationDirectoryUseCase }

func NewInvitationDirectoryHandler(directory InvitationDirectoryUseCase) (*InvitationDirectoryHandler, error) {
	if directory == nil {
		return nil, errors.New("invitation_directory_handler_invalid")
	}
	return &InvitationDirectoryHandler{directory}, nil
}

type invitationCursorJSON struct {
	CreatedAt    string `json:"created_at"`
	InvitationID string `json:"invitation_id"`
}
type pendingInvitationItem struct {
	InvitationID string `json:"invitation_id"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	CreatedAt    string `json:"created_at"`
	ExpiresAt    string `json:"expires_at"`
}
type pendingInvitationData struct {
	Invitations []pendingInvitationItem `json:"invitations"`
	NextCursor  *string                 `json:"next_cursor"`
}
type pendingInvitationResponse struct {
	Data pendingInvitationData `json:"data"`
	Meta meta                  `json:"meta"`
}

func decodeInvitationCursor(value string) (authzapp.InvitationCursor, bool) {
	if value == "" {
		return authzapp.InvitationCursor{}, true
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) > 256 {
		return authzapp.InvitationCursor{}, false
	}
	var decoded invitationCursorJSON
	if json.Unmarshal(raw, &decoded) != nil {
		return authzapp.InvitationCursor{}, false
	}
	at, err := time.Parse(time.RFC3339Nano, decoded.CreatedAt)
	if err != nil {
		return authzapp.InvitationCursor{}, false
	}
	id, err := uuid.Parse(decoded.InvitationID)
	if err != nil {
		return authzapp.InvitationCursor{}, false
	}
	return authzapp.InvitationCursor{CreatedAt: at.UTC(), InvitationID: authzapp.ID(id)}, true
}
func encodeInvitationCursor(invitation authzapp.PendingInvitation) (string, error) {
	raw, err := json.Marshal(invitationCursorJSON{CreatedAt: invitation.CreatedAt.UTC().Format(time.RFC3339Nano), InvitationID: uuid.UUID(invitation.InvitationID).String()})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
func (h *InvitationDirectoryHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("invitation_directory_http_context_required")
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if h == nil || h.directory == nil {
		return fail(c, 503, "INVITATIONS_UNAVAILABLE", "Invitations are temporarily unavailable.", true)
	}
	if len(c.Body()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body.", false)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	values, err := url.ParseQuery(string(c.Request().URI().QueryString()))
	if err != nil {
		return fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
	}
	for key, list := range values {
		if (key != "limit" && key != "cursor") || len(list) != 1 {
			return fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
		}
	}
	limit := 50
	if raw := values.Get("limit"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 || parsed > 100 {
			return fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
		}
		limit = parsed
	}
	cursor, ok := decodeInvitationCursor(values.Get("cursor"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
	}
	tenantText, ok := singleHeader(c, TenantIDHeader)
	if !ok {
		return invalidHeader(c, TenantIDHeader, "required")
	}
	tenantID, ok := parseOpportunityID(tenantText)
	if !ok {
		return invalidHeader(c, TenantIDHeader, "uuid")
	}
	generationText, ok := singleHeader(c, ContextGenerationHeader)
	if !ok {
		return invalidHeader(c, ContextGenerationHeader, "required")
	}
	generation, ok := positiveInt64(generationText)
	if !ok {
		return invalidHeader(c, ContextGenerationHeader, "positive_integer")
	}
	page, err := h.directory.List(c.Context(), authzapp.InvitationDirectoryQuery{ActorID: authzapp.ID(principal.UserID), TenantID: authzapp.ID(tenantID), ContextGeneration: generation, Limit: limit, Cursor: cursor})
	if err != nil {
		if err.Error() == "invitation_directory_not_authorized" {
			return fail(c, 403, "INVITATIONS_NOT_AUTHORIZED", "You are not authorized to view these invitations.", false)
		}
		if err.Error() == "invitation_directory_invalid" {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		return fail(c, 503, "INVITATIONS_UNAVAILABLE", "Invitations are temporarily unavailable.", true)
	}
	items := make([]pendingInvitationItem, len(page.Invitations))
	for index, item := range page.Invitations {
		items[index] = pendingInvitationItem{InvitationID: uuid.UUID(item.InvitationID).String(), Email: item.Email, Role: string(item.Role), CreatedAt: item.CreatedAt.Format(time.RFC3339Nano), ExpiresAt: item.ExpiresAt.Format(time.RFC3339Nano)}
	}
	var next *string
	if page.HasMore && len(page.Invitations) > 0 {
		encoded, encodeErr := encodeInvitationCursor(page.Invitations[len(page.Invitations)-1])
		if encodeErr != nil {
			return fail(c, 503, "INVITATIONS_UNAVAILABLE", "Invitations are temporarily unavailable.", true)
		}
		next = &encoded
	}
	return c.Status(fiber.StatusOK).JSON(pendingInvitationResponse{Data: pendingInvitationData{Invitations: items, NextCursor: next}, Meta: meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
