package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"

	authzapp "bidora/backend/internal/authorization/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type InvitationIssueUseCase interface {
	Issue(context.Context, authzapp.InvitationIssueInput) (authzapp.InvitationIssueResult, error)
}
type InvitationAcceptUseCase interface {
	Accept(context.Context, authzapp.InvitationAcceptanceInput) (authzapp.InvitationAcceptanceResult, error)
}
type InvitationIssueHandler struct{ issue InvitationIssueUseCase }
type InvitationAcceptHandler struct{ accept InvitationAcceptUseCase }

func NewInvitationIssueHandler(issue InvitationIssueUseCase) (*InvitationIssueHandler, error) {
	if issue == nil {
		return nil, errors.New("invitation_issue_handler_invalid")
	}
	return &InvitationIssueHandler{issue}, nil
}
func NewInvitationAcceptHandler(accept InvitationAcceptUseCase) (*InvitationAcceptHandler, error) {
	if accept == nil {
		return nil, errors.New("invitation_accept_handler_invalid")
	}
	return &InvitationAcceptHandler{accept}, nil
}

type invitationIssueData struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expires_at"`
	Status    string `json:"status"`
}
type invitationIssueResponse struct {
	Data invitationIssueData `json:"data"`
	Meta meta                `json:"meta"`
}

func (h *InvitationIssueHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("invitation_issue_http_context_required")
	}
	secureInvitationResponse(c)
	if h == nil || h.issue == nil {
		return fail(c, 503, "INVITATION_UNAVAILABLE", "Invitation is temporarily unavailable.", true)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	if contentType, valid := singleHeader(c, "Content-Type"); !valid || !validJSONMediaType(contentType) {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
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
	emailValue, roleValue, err := decodeInvitationIssue(c.Body())
	if err != nil {
		return fail(c, errorStatus(err.Error()), err.Error(), "The request is invalid.", false)
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	result, err := h.issue.Issue(c.Context(), authzapp.InvitationIssueInput{ActorID: authzapp.ID(principal.UserID), TenantID: authzapp.ID(tenantID), CorrelationID: authzapp.ID(correlation), ContextGeneration: generation, Email: emailValue, Role: authzapp.TenderpreneurRole(roleValue)})
	if err != nil {
		return invitationIssueFailure(c, err)
	}
	if result.InvitationID.IsZero() || result.Role == "" || result.ExpiresAt.IsZero() {
		return fail(c, 503, "INVITATION_UNAVAILABLE", "Invitation is temporarily unavailable.", true)
	}
	return c.Status(fiber.StatusCreated).JSON(invitationIssueResponse{Data: invitationIssueData{ID: uuid.UUID(result.InvitationID).String(), Role: string(result.Role), ExpiresAt: result.ExpiresAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), Status: "PENDING"}, Meta: meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *InvitationAcceptHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("invitation_accept_http_context_required")
	}
	secureInvitationResponse(c)
	if h == nil || h.accept == nil {
		return fail(c, 503, "INVITATION_UNAVAILABLE", "Invitation is temporarily unavailable.", true)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	if contentType, valid := singleHeader(c, "Content-Type"); !valid || !validJSONMediaType(contentType) {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	token, err := decodeInvitationField(c.Body(), "token")
	if err != nil {
		return fail(c, errorStatus(err.Error()), err.Error(), "The request is invalid.", false)
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	result, err := h.accept.Accept(c.Context(), authzapp.InvitationAcceptanceInput{Token: token, UserID: authzapp.ID(principal.UserID), CorrelationID: authzapp.ID(correlation)})
	if err != nil {
		return invitationAcceptFailure(c, err)
	}
	if result.MembershipID.IsZero() || result.TenantID.IsZero() || result.ContextGeneration < 1 {
		return fail(c, 503, "INVITATION_UNAVAILABLE", "Invitation is temporarily unavailable.", true)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
func secureInvitationResponse(c fiber.Ctx) {
	if c.GetRespHeader("X-Request-ID") == "" {
		c.Set("X-Request-ID", requestID())
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
}
func decodeInvitationIssue(body []byte) (string, string, error) {
	if len(body) > LoginBodyLimit || !utf8.Valid(body) || hasDuplicateWorkflowField(body) {
		return "", "", errors.New("INVALID_REQUEST")
	}
	var value struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || value.Email == "" || value.Role == "" {
		return "", "", errors.New("INVALID_REQUEST")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return "", "", errors.New("INVALID_REQUEST")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil || len(fields) != 2 || fields["email"] == nil || fields["role"] == nil {
		return "", "", errors.New("INVALID_REQUEST")
	}
	return value.Email, value.Role, nil
}

func decodeInvitationField(body []byte, key string) (string, error) {
	if len(body) > LoginBodyLimit {
		return "", errors.New("REQUEST_TOO_LARGE")
	}
	if !utf8.Valid(body) {
		return "", errors.New("INVALID_REQUEST")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') || !decoder.More() {
		return "", errors.New("INVALID_REQUEST")
	}
	name, err := decoder.Token()
	if err != nil || name != key {
		return "", errors.New("INVALID_REQUEST")
	}
	var value string
	if err = decoder.Decode(&value); err != nil || value == "" || decoder.More() {
		return "", errors.New("INVALID_REQUEST")
	}
	if closing, err := decoder.Token(); err != nil || closing != json.Delim('}') {
		return "", errors.New("INVALID_REQUEST")
	}
	if _, err = decoder.Token(); err != io.EOF {
		return "", errors.New("INVALID_REQUEST")
	}
	return value, nil
}
func invitationIssueFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "invitation_issue_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case "invitation_not_authorized":
		return fail(c, 403, "INVITATION_FORBIDDEN", "The invitation operation is not permitted.", false)
	case "invitation_delivery_unavailable":
		return fail(c, 503, "INVITATION_DELIVERY_UNAVAILABLE", "Invitation delivery is temporarily unavailable.", true)
	default:
		return fail(c, 503, "INVITATION_UNAVAILABLE", "Invitation is temporarily unavailable.", true)
	}
}
func invitationAcceptFailure(c fiber.Ctx, err error) error {
	if err.Error() == "invitation_invalid" {
		return fail(c, 400, "INVITATION_INVALID", "The invitation is invalid.", false)
	}
	return fail(c, 503, "INVITATION_UNAVAILABLE", "Invitation is temporarily unavailable.", true)
}
