package httpapi

import (
	"context"
	"errors"
	"mime"
	"strconv"
	"strings"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
	opportunitytransport "bidora/backend/internal/opportunity/transport"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const TenantIDHeader = "X-Bidora-Tenant-ID"
const ContextGenerationHeader = "X-Bidora-Context-Generation"
const IdempotencyKeyHeader = "Idempotency-Key"

type ManualCaptureExecutor interface {
	Execute(context.Context, opportunityapp.ExecuteCaptureInput) (opportunityapp.CaptureResult, error)
}
type ManualCaptureHandler struct{ executor ManualCaptureExecutor }

func NewManualCaptureHandler(executor ManualCaptureExecutor) (*ManualCaptureHandler, error) {
	if executor == nil {
		return nil, errors.New("manual_capture_handler_invalid")
	}
	return &ManualCaptureHandler{executor}, nil
}

type captureData struct {
	ID                string    `json:"id"`
	SourceVersionID   string    `json:"source_version_id"`
	Version           int64     `json:"version"`
	VerificationState string    `json:"verification_state"`
	CreatedAt         time.Time `json:"created_at"`
}
type captureSuccess struct {
	Data captureData `json:"data"`
	Meta meta        `json:"meta"`
}

func (h *ManualCaptureHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("capture_http_context_required")
	}
	if c.GetRespHeader("X-Request-ID") == "" {
		c.Set("X-Request-ID", requestID())
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if h == nil || h.executor == nil {
		return fail(c, 503, "CAPTURE_UNAVAILABLE", "Capture is temporarily unavailable.", true)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	contentType, ok := singleHeader(c, "Content-Type")
	if !ok {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" || len(params) > 1 || (len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8")) {
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
	idempotencyKey, ok := singleHeader(c, IdempotencyKeyHeader)
	if !ok {
		return invalidHeader(c, IdempotencyKeyHeader, "required")
	}
	if _, ok = parseOpportunityID(idempotencyKey); !ok {
		return invalidHeader(c, IdempotencyKeyHeader, "uuid")
	}
	correlationID, ok := parseOpportunityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	command, err := opportunitytransport.DecodeCaptureJSON(c.Body())
	if err != nil {
		return captureDecodeFailure(c, err)
	}
	result, err := h.executor.Execute(c.Context(), opportunityapp.ExecuteCaptureInput{Subject: opportunityapp.CaptureSubject{IdentityID: opportunityapp.ID(principal.UserID), SelectedTenantID: tenantID, ContextGeneration: generation}, IdempotencyKey: idempotencyKey, CorrelationID: correlationID, Command: command})
	if err != nil {
		return captureOperationFailure(c, err)
	}
	if result.OpportunityID.IsZero() || result.SourceVersionID.IsZero() || result.Version != 1 || result.CreatedAt.IsZero() {
		return fail(c, 503, "CAPTURE_UNAVAILABLE", "Capture is temporarily unavailable.", true)
	}
	return c.Status(fiber.StatusCreated).JSON(captureSuccess{Data: captureData{ID: uuid.UUID(result.OpportunityID).String(), SourceVersionID: uuid.UUID(result.SourceVersionID).String(), Version: result.Version, VerificationState: "UNVERIFIED", CreatedAt: result.CreatedAt.UTC()}, Meta: meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func singleHeader(c fiber.Ctx, name string) (string, bool) {
	values := c.Request().Header.PeekAll(name)
	if len(values) != 1 || len(values[0]) == 0 {
		return "", false
	}
	return string(values[0]), true
}
func parseOpportunityID(value string) (opportunityapp.ID, bool) {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return opportunityapp.ID{}, false
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return opportunityapp.ID{}, false
	}
	var result opportunityapp.ID
	copy(result[:], parsed[:])
	return result, !result.IsZero()
}
func positiveInt64(value string) (int64, bool) {
	if value == "" {
		return 0, false
	}
	for i := range len(value) {
		if value[i] < '0' || value[i] > '9' {
			return 0, false
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return parsed, err == nil && parsed > 0
}
func invalidHeader(c fiber.Ctx, name, rule string) error {
	return failWithFields(c, 400, "INVALID_REQUEST", "The request is invalid.", false, []fieldError{{Path: "header." + name, Code: rule, Message: "The header is invalid."}})
}
func captureDecodeFailure(c fiber.Ctx, err error) error {
	var validation *opportunityapp.ValidationError
	if errors.As(err, &validation) {
		return failWithFields(c, 422, "VALIDATION_FAILED", "The request is invalid.", false, []fieldError{{Path: validation.Field(), Code: validation.Rule(), Message: "The field is invalid."}})
	}
	var decoded *opportunitytransport.DecodeError
	if !errors.As(err, &decoded) {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	status := 400
	if decoded.Code() == "REQUEST_TOO_LARGE" {
		status = 413
	}
	if decoded.Code() == "VALIDATION_FAILED" {
		status = 422
	}
	return failWithFields(c, status, decoded.Code(), "The request is invalid.", false, []fieldError{{Path: decoded.Field(), Code: decoded.Rule(), Message: "The field is invalid."}})
}
func captureOperationFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "capture_not_authorized":
		return fail(c, 403, "CAPTURE_FORBIDDEN", "The operation is not permitted.", false)
	case "capture_source_unavailable":
		return fail(c, 404, "SOURCE_NOT_FOUND", "The referenced source is unavailable.", false)
	case "capture_idempotency_conflict":
		return fail(c, 409, "IDEMPOTENCY_CONFLICT", "The idempotency key was used for different input.", false)
	case "capture_operation_in_progress":
		return fail(c, 409, "OPERATION_IN_PROGRESS", "The operation is still in progress.", true)
	case "capture_idempotency_key_invalid", "capture_command_invalid", "capture_correlation_required":
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	default:
		return fail(c, 503, "CAPTURE_UNAVAILABLE", "Capture is temporarily unavailable.", true)
	}
}
func failWithFields(c fiber.Ctx, status int, code, message string, retryable bool, fields []fieldError) error {
	return c.Status(status).JSON(failure{Error: errorData{Code: code, Message: message, FieldErrors: fields, TraceID: c.GetRespHeader("X-Request-ID"), Retryable: retryable}})
}
