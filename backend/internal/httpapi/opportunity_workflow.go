package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"strings"
	"unicode/utf8"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const OpportunityVerificationPath = "/v1/tenderpreneur/opportunities/:opportunity_id/verify-facts"
const WorkspaceCreationPath = "/v1/tenderpreneur/opportunities/:opportunity_id/create-workspace"
const BidDecisionPath = "/v1/tenderpreneur/bids/:bid_id/decision"

type OpportunityWorkflowUseCase interface {
	Verify(context.Context, opportunityapp.VerifyOpportunityInput) (opportunityapp.VerificationResult, error)
	CreateWorkspace(context.Context, opportunityapp.CreateWorkspaceInput) (opportunityapp.WorkspaceResult, error)
	RecordDecision(context.Context, opportunityapp.RecordDecisionInput) (opportunityapp.DecisionResult, error)
}
type OpportunityWorkflowHandler struct{ workflow OpportunityWorkflowUseCase }

func NewOpportunityWorkflowHandler(workflow OpportunityWorkflowUseCase) (*OpportunityWorkflowHandler, error) {
	if workflow == nil {
		return nil, errors.New("opportunity_workflow_handler_invalid")
	}
	return &OpportunityWorkflowHandler{workflow}, nil
}

type verifyOpportunityBody struct {
	SourceVersionID string `json:"source_version_id"`
	ExpectedVersion int64  `json:"expected_version"`
}
type createWorkspaceBody struct {
	SourceVersionID string `json:"source_version_id"`
	OwnerID         string `json:"owner_id"`
	ExpectedVersion int64  `json:"expected_version"`
}
type bidDecisionBody struct {
	Decision        string `json:"decision"`
	Rationale       string `json:"rationale"`
	ExpectedVersion int64  `json:"expected_version"`
}
type workflowResponse struct {
	Data struct {
		ID              string `json:"id"`
		SourceVersionID string `json:"source_version_id,omitempty"`
		WorkspaceID     string `json:"workspace_id,omitempty"`
		Decision        string `json:"decision,omitempty"`
		Version         int64  `json:"version"`
		Replay          bool   `json:"replay"`
	} `json:"data"`
	Meta meta `json:"meta"`
}

func (h *OpportunityWorkflowHandler) Verify(c fiber.Ctx) error {
	if c == nil {
		return errors.New("opportunity_verification_context_required")
	}
	var body verifyOpportunityBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, ok := workflowCommon(c, "opportunity_id")
	if !ok {
		return nil
	}
	sourceID, ok := parseOpportunityID(body.SourceVersionID)
	if !ok || body.ExpectedVersion < 1 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.workflow.Verify(c.Context(), opportunityapp.VerifyOpportunityInput{Subject: common.subject, OpportunityID: common.opportunityID, SourceVersionID: sourceID, CorrelationID: common.correlationID, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return workflowFailure(c, err)
	}
	response := workflowResponse{Meta: meta{TraceID: c.GetRespHeader("X-Request-ID")}}
	response.Data.ID = uuid.UUID(result.OpportunityID).String()
	response.Data.SourceVersionID = uuid.UUID(result.SourceVersionID).String()
	response.Data.Version = result.Version
	response.Data.Replay = result.Replay
	return c.Status(201).JSON(response)
}
func (h *OpportunityWorkflowHandler) CreateWorkspace(c fiber.Ctx) error {
	if c == nil {
		return errors.New("workspace_creation_context_required")
	}
	var body createWorkspaceBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, ok := workflowCommon(c, "opportunity_id")
	if !ok {
		return nil
	}
	sourceID, sourceOK := parseOpportunityID(body.SourceVersionID)
	ownerID, ownerOK := parseOpportunityID(body.OwnerID)
	if !sourceOK || !ownerOK || body.ExpectedVersion < 1 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.workflow.CreateWorkspace(c.Context(), opportunityapp.CreateWorkspaceInput{Subject: common.subject, OpportunityID: common.opportunityID, SourceVersionID: sourceID, OwnerID: ownerID, CorrelationID: common.correlationID, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return workflowFailure(c, err)
	}
	response := workflowResponse{Meta: meta{TraceID: c.GetRespHeader("X-Request-ID")}}
	response.Data.ID = uuid.UUID(result.WorkspaceID).String()
	response.Data.Version = result.Version
	response.Data.Replay = result.Replay
	return c.Status(201).JSON(response)
}

func (h *OpportunityWorkflowHandler) RecordDecision(c fiber.Ctx) error {
	if c == nil {
		return errors.New("bid_decision_context_required")
	}
	var body bidDecisionBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	result, err := h.workflow.RecordDecision(c.Context(), opportunityapp.RecordDecisionInput{Subject: common.subject, WorkspaceID: common.opportunityID, CorrelationID: common.correlationID, ExpectedVersion: body.ExpectedVersion, Decision: body.Decision, Rationale: body.Rationale, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return workflowFailure(c, err)
	}
	response := workflowResponse{Meta: meta{TraceID: c.GetRespHeader("X-Request-ID")}}
	response.Data.ID = uuid.UUID(result.DecisionID).String()
	response.Data.WorkspaceID = uuid.UUID(result.WorkspaceID).String()
	response.Data.Decision = result.Decision
	response.Data.Version = result.Version
	response.Data.Replay = result.Replay
	return c.Status(201).JSON(response)
}

type workflowCommonInput struct {
	subject                      opportunityapp.WorkflowSubject
	opportunityID, correlationID opportunityapp.ID
	idempotencyKey               string
}

func workflowCommon(c fiber.Ctx, resourceParam string) (workflowCommonInput, bool) {
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		_ = fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		return workflowCommonInput{}, false
	}
	opportunityID, ok := parseOpportunityID(c.Params(resourceParam))
	if !ok {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return workflowCommonInput{}, false
	}
	tenantText, ok := singleHeader(c, TenantIDHeader)
	if !ok {
		_ = invalidHeader(c, TenantIDHeader, "required")
		return workflowCommonInput{}, false
	}
	tenantID, ok := parseOpportunityID(tenantText)
	if !ok {
		_ = invalidHeader(c, TenantIDHeader, "uuid")
		return workflowCommonInput{}, false
	}
	generationText, ok := singleHeader(c, ContextGenerationHeader)
	if !ok {
		_ = invalidHeader(c, ContextGenerationHeader, "required")
		return workflowCommonInput{}, false
	}
	generation, ok := positiveInt64(generationText)
	if !ok {
		_ = invalidHeader(c, ContextGenerationHeader, "positive_integer")
		return workflowCommonInput{}, false
	}
	key, ok := singleHeader(c, IdempotencyKeyHeader)
	if !ok {
		_ = invalidHeader(c, IdempotencyKeyHeader, "required")
		return workflowCommonInput{}, false
	}
	if _, err := uuid.Parse(key); err != nil {
		_ = invalidHeader(c, IdempotencyKeyHeader, "uuid")
		return workflowCommonInput{}, false
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		_ = fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
		return workflowCommonInput{}, false
	}
	return workflowCommonInput{subject: opportunityapp.WorkflowSubject{IdentityID: opportunityapp.ID(principal.UserID), SelectedTenantID: tenantID, ContextGeneration: generation}, opportunityID: opportunityID, correlationID: opportunityapp.ID(correlation), idempotencyKey: key}, true
}
func bindWorkflow(c fiber.Ctx, target any) error { return bindWorkflowLimit(c, target, 4096) }
func bindWorkflowLimit(c fiber.Ctx, target any, limit int) error {
	return bindWorkflowJSON(c, target, limit, false)
}

// bindWorkflowJSON keeps strict bounded JSON validation when a read-only search
// explicitly supports separately validated query filters.
func bindWorkflowJSON(c fiber.Ctx, target any, limit int, allowQuery bool) error {
	c.Set("Cache-Control", "private, no-store")
	if limit < 1 || len(c.Body()) > limit {
		return fail(c, 413, "REQUEST_TOO_LARGE", "The request is too large.", false)
	}
	if !allowQuery && len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	contentType, ok := singleHeader(c, "Content-Type")
	if !ok {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" || (len(params) > 1) || (len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8")) {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	if !utf8.Valid(c.Body()) || !validLoginSurrogates(c.Body()) || hasDuplicateWorkflowField(c.Body()) {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	return nil
}
func requireWorkflowFields(body []byte, required []string, optional ...string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		return false
	}
	allowed := make(map[string]struct{}, len(required)+len(optional))
	for _, name := range required {
		allowed[name] = struct{}{}
		if _, ok := fields[name]; !ok {
			return false
		}
	}
	for _, name := range optional {
		allowed[name] = struct{}{}
	}
	if len(fields) > len(allowed) {
		return false
	}
	for name := range fields {
		if _, ok := allowed[name]; !ok {
			return false
		}
	}
	return true
}
func hasDuplicateWorkflowField(body []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return true
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err = decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] {
			return true
		}
		seen[key] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return true
		}
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return true
	}
	_, err = decoder.Token()
	return err != io.EOF
}
func workflowFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "opportunity_verification_not_authorized", "workspace_creation_not_authorized", "bid_decision_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to perform this operation.", false)
	case "opportunity_verification_invalid", "workspace_creation_invalid", "bid_decision_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case "opportunity_verification_conflict", "workspace_creation_conflict", "bid_decision_conflict":
		return fail(c, 409, "WORKFLOW_CONFLICT", "The operation could not be completed for the requested version.", false)
	case "opportunity_verification_unavailable", "workspace_creation_unavailable", "bid_decision_unavailable":
		return fail(c, 503, "WORKFLOW_UNAVAILABLE", "The operation is temporarily unavailable.", true)
	default:
		return fail(c, 503, "WORKFLOW_UNAVAILABLE", "The operation is temporarily unavailable.", true)
	}
}
