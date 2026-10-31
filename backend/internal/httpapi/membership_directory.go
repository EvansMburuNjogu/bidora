package httpapi

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type MembershipDirectoryUseCase interface {
	List(context.Context, authzapp.ID) ([]authzapp.MembershipSummary, error)
	ListManaged(context.Context, authzapp.ManagedContributorQuery) ([]authzapp.ManagedContributor, error)
}
type MembershipDirectoryHandler struct{ directory MembershipDirectoryUseCase }

func NewMembershipDirectoryHandler(directory MembershipDirectoryUseCase) (*MembershipDirectoryHandler, error) {
	if directory == nil {
		return nil, errors.New("membership_directory_handler_invalid")
	}
	return &MembershipDirectoryHandler{directory}, nil
}

type membershipDirectoryItem struct {
	MembershipID      string   `json:"membership_id"`
	TenantID          string   `json:"tenant_id"`
	TenantType        string   `json:"tenant_type"`
	TenantName        string   `json:"tenant_name"`
	ContextGeneration int64    `json:"context_generation"`
	Capabilities      []string `json:"capabilities"`
}
type membershipDirectoryData struct {
	Memberships []membershipDirectoryItem `json:"memberships"`
}
type membershipDirectoryResponse struct {
	Data membershipDirectoryData `json:"data"`
	Meta meta                    `json:"meta"`
}
type managedContributorItem struct {
	MembershipID      string `json:"membership_id"`
	Email             string `json:"email"`
	ContextGeneration int64  `json:"context_generation"`
}
type managedContributorData struct {
	Memberships []managedContributorItem `json:"memberships"`
}
type managedContributorResponse struct {
	Data managedContributorData `json:"data"`
	Meta meta                   `json:"meta"`
}

func (h *MembershipDirectoryHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("membership_directory_http_context_required")
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if h == nil || h.directory == nil {
		return fail(c, 503, "MEMBERSHIPS_UNAVAILABLE", "Memberships are temporarily unavailable.", true)
	}
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	rows, err := h.directory.List(c.Context(), authzapp.ID(principal.UserID))
	if err != nil {
		return fail(c, 503, "MEMBERSHIPS_UNAVAILABLE", "Memberships are temporarily unavailable.", true)
	}
	items := make([]membershipDirectoryItem, len(rows))
	for index, row := range rows {
		items[index] = membershipDirectoryItem{MembershipID: uuid.UUID(row.MembershipID).String(), TenantID: uuid.UUID(row.TenantID).String(), TenantType: row.TenantType, TenantName: row.TenantName, ContextGeneration: row.ContextGeneration, Capabilities: append([]string(nil), row.Capabilities...)}
	}
	return c.Status(fiber.StatusOK).JSON(membershipDirectoryResponse{Data: membershipDirectoryData{Memberships: items}, Meta: meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}

func (h *MembershipDirectoryHandler) HandleManaged(c fiber.Ctx) error {
	if c == nil {
		return errors.New("managed_membership_directory_http_context_required")
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if h == nil || h.directory == nil {
		return fail(c, 503, "MEMBERSHIPS_UNAVAILABLE", "Memberships are temporarily unavailable.", true)
	}
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
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
	rows, err := h.directory.ListManaged(c.Context(), authzapp.ManagedContributorQuery{ActorID: authzapp.ID(principal.UserID), TenantID: authzapp.ID(tenantID), ContextGeneration: generation})
	if err != nil {
		if err.Error() == "membership_directory_not_authorized" {
			return fail(c, 403, "MEMBERSHIPS_NOT_AUTHORIZED", "You are not authorized to view these memberships.", false)
		}
		return fail(c, 503, "MEMBERSHIPS_UNAVAILABLE", "Memberships are temporarily unavailable.", true)
	}
	items := make([]managedContributorItem, len(rows))
	for index, row := range rows {
		items[index] = managedContributorItem{MembershipID: uuid.UUID(row.MembershipID).String(), Email: row.Email, ContextGeneration: row.ContextGeneration}
	}
	return c.Status(fiber.StatusOK).JSON(managedContributorResponse{Data: managedContributorData{Memberships: items}, Meta: meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
