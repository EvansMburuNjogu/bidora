package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const OpportunityListPath = "/v1/tenderpreneur/opportunities"
const OpportunityDetailPath = "/v1/tenderpreneur/opportunities/:opportunity_id"
const BidListPath = "/v1/tenderpreneur/bids"
const BidDetailPath = "/v1/tenderpreneur/bids/:bid_id"

type OpportunityReadUseCase interface {
	ListOpportunities(context.Context, opportunityapp.ReadQuery) (opportunityapp.OpportunityPage, error)
	GetOpportunity(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.OpportunityDetail, error)
	ListBids(context.Context, opportunityapp.BidReadQuery) (opportunityapp.BidPage, error)
	GetBid(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.BidDetail, error)
}
type OpportunityReadHandler struct{ reader OpportunityReadUseCase }

func NewOpportunityReadHandler(reader OpportunityReadUseCase) (*OpportunityReadHandler, error) {
	if reader == nil {
		return nil, errors.New("opportunity_read_handler_invalid")
	}
	return &OpportunityReadHandler{reader}, nil
}

type readCursorJSON struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}
type opportunityDTO struct {
	ID                string  `json:"id"`
	SourceVersionID   string  `json:"source_version_id"`
	Title             string  `json:"title"`
	VerificationState string  `json:"verification_state"`
	Version           int64   `json:"version"`
	BuyerName         *string `json:"buyer_name"`
	ClosesAt          *string `json:"closes_at"`
	SourceTimezone    *string `json:"source_timezone"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}
type bidDTO struct {
	ID              string  `json:"id"`
	OpportunityID   string  `json:"opportunity_id"`
	SourceVersionID string  `json:"source_version_id"`
	OwnerID         string  `json:"owner_id"`
	State           string  `json:"state"`
	Title           string  `json:"title"`
	Version         int64   `json:"version"`
	BuyerName       *string `json:"buyer_name"`
	ClosesAt        *string `json:"closes_at"`
	SourceTimezone  *string `json:"source_timezone"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	ArchivedAt      *string `json:"archived_at"`
}

func (h *OpportunityReadHandler) ListOpportunities(c fiber.Ctx) error {
	subject, ok := readSubject(c)
	if !ok {
		return nil
	}
	limit, cursor, values, ok := readPage(c)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
	}
	for key := range values {
		if key != "limit" && key != "cursor" {
			return fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
		}
	}
	page, err := h.reader.ListOpportunities(c.Context(), opportunityapp.ReadQuery{Subject: subject, Limit: limit, Cursor: cursor})
	if err != nil {
		return readFailure(c, err)
	}
	items := make([]opportunityDTO, len(page.Items))
	for i, item := range page.Items {
		items[i] = opportunityView(item)
	}
	next, err := nextReadCursor(page.HasMore, page.Items)
	if err != nil {
		return readFailure(c, err)
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"opportunities": items, "next_cursor": next}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *OpportunityReadHandler) GetOpportunity(c fiber.Ctx) error {
	subject, ok := readSubject(c)
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
	}
	id, ok := parseOpportunityID(c.Params("opportunity_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	item, err := h.reader.GetOpportunity(c.Context(), subject, id)
	if err != nil {
		return readFailure(c, err)
	}
	duplicates := make([]string, len(item.PossibleDuplicateIDs))
	for i, id := range item.PossibleDuplicateIDs {
		duplicates[i] = uuid.UUID(id).String()
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"opportunity": opportunityView(item.OpportunitySummary), "revision": item.Revision, "source_uri": item.SourceURI, "source_document_version_id": optionalID(item.SourceDocumentVersionID, item.HasSourceDocument), "source_date_text": item.SourceDateText, "verified_by": optionalID(item.VerifiedBy, !item.VerifiedBy.IsZero()), "verified_at": formatOptionalTime(item.VerifiedAt), "possible_duplicate_ids": duplicates, "possible_duplicates_truncated": item.DuplicatesTruncated}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *OpportunityReadHandler) ListBids(c fiber.Ctx) error {
	subject, ok := readSubject(c)
	if !ok {
		return nil
	}
	limit, cursor, values, ok := readPage(c)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
	}
	for key, list := range values {
		if key != "limit" && key != "cursor" && key != "state" && key != "owner_id" && key != "archive_state" || len(list) != 1 {
			return fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
		}
	}
	archiveState := values.Get("archive_state")
	if archiveState == "" {
		archiveState = "ACTIVE"
	}
	query := opportunityapp.BidReadQuery{ReadQuery: opportunityapp.ReadQuery{Subject: subject, Limit: limit, Cursor: cursor}, State: values.Get("state"), ArchiveState: archiveState}
	if raw := values.Get("owner_id"); raw != "" {
		id, valid := parseOpportunityID(raw)
		if !valid {
			return fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
		}
		query.OwnerID = id
		query.FilterOwner = true
	}
	page, err := h.reader.ListBids(c.Context(), query)
	if err != nil {
		return readFailure(c, err)
	}
	items := make([]bidDTO, len(page.Items))
	for i, item := range page.Items {
		items[i] = bidView(item)
	}
	next, err := nextBidCursor(page.HasMore, page.Items)
	if err != nil {
		return readFailure(c, err)
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"bids": items, "next_cursor": next}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *OpportunityReadHandler) GetBid(c fiber.Ctx) error {
	subject, ok := readSubject(c)
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
	}
	id, ok := parseOpportunityID(c.Params("bid_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	item, err := h.reader.GetBid(c.Context(), subject, id)
	if err != nil {
		return readFailure(c, err)
	}
	decisions := make([]fiber.Map, len(item.Decisions))
	for i, decision := range item.Decisions {
		decisions[i] = fiber.Map{"id": uuid.UUID(decision.ID).String(), "sequence": decision.Sequence, "workspace_version": decision.WorkspaceVersion, "decision": decision.Decision, "rationale": decision.Rationale, "actor_id": uuid.UUID(decision.ActorID).String(), "decided_at": decision.DecidedAt.Format(time.RFC3339Nano)}
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"bid": bidView(item.BidSummary), "created_by": uuid.UUID(item.CreatedBy).String(), "source_revision": item.SourceRevision, "source_uri": item.SourceURI, "source_document_version_id": optionalID(item.SourceDocumentVersionID, item.HasSourceDocument), "source_date_text": item.SourceDateText, "verification_state": item.VerificationState, "archived_by": optionalID(item.ArchivedBy, !item.ArchivedBy.IsZero()), "archive_reason": item.ArchiveReason, "archive_policy_version": item.ArchivePolicyVersion, "decisions": decisions}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func readSubject(c fiber.Ctx) (opportunityapp.WorkflowSubject, bool) {
	c.Set("Cache-Control", "private, no-store")
	if len(c.Body()) != 0 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return opportunityapp.WorkflowSubject{}, false
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		_ = fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		return opportunityapp.WorkflowSubject{}, false
	}
	tenantText, ok := singleHeader(c, TenantIDHeader)
	if !ok {
		_ = invalidHeader(c, TenantIDHeader, "required")
		return opportunityapp.WorkflowSubject{}, false
	}
	tenantID, ok := parseOpportunityID(tenantText)
	if !ok {
		_ = invalidHeader(c, TenantIDHeader, "uuid")
		return opportunityapp.WorkflowSubject{}, false
	}
	generationText, ok := singleHeader(c, ContextGenerationHeader)
	if !ok {
		_ = invalidHeader(c, ContextGenerationHeader, "required")
		return opportunityapp.WorkflowSubject{}, false
	}
	generation, ok := positiveInt64(generationText)
	if !ok {
		_ = invalidHeader(c, ContextGenerationHeader, "positive_integer")
		return opportunityapp.WorkflowSubject{}, false
	}
	return opportunityapp.WorkflowSubject{IdentityID: opportunityapp.ID(principal.UserID), SelectedTenantID: tenantID, ContextGeneration: generation}, true
}
func readPage(c fiber.Ctx) (int, opportunityapp.ReadCursor, url.Values, bool) {
	values, err := url.ParseQuery(string(c.Request().URI().QueryString()))
	if err != nil {
		return 0, opportunityapp.ReadCursor{}, nil, false
	}
	for key, list := range values {
		if (key != "limit" && key != "cursor" && key != "state" && key != "owner_id" && key != "archive_state") || len(list) != 1 {
			return 0, opportunityapp.ReadCursor{}, nil, false
		}
	}
	limit := 50
	if raw := values.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			return 0, opportunityapp.ReadCursor{}, nil, false
		}
		limit = parsed
	}
	cursor, ok := decodeReadCursor(values.Get("cursor"))
	return limit, cursor, values, ok
}
func decodeReadCursor(value string) (opportunityapp.ReadCursor, bool) {
	if value == "" {
		return opportunityapp.ReadCursor{}, true
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) > 256 {
		return opportunityapp.ReadCursor{}, false
	}
	var decoded readCursorJSON
	if json.Unmarshal(raw, &decoded) != nil {
		return opportunityapp.ReadCursor{}, false
	}
	at, err := time.Parse(time.RFC3339Nano, decoded.CreatedAt)
	if err != nil {
		return opportunityapp.ReadCursor{}, false
	}
	id, err := uuid.Parse(decoded.ID)
	if err != nil {
		return opportunityapp.ReadCursor{}, false
	}
	return opportunityapp.ReadCursor{CreatedAt: at.UTC(), ID: opportunityapp.ID(id)}, true
}
func encodeReadCursor(at time.Time, id opportunityapp.ID) (string, error) {
	raw, err := json.Marshal(readCursorJSON{CreatedAt: at.UTC().Format(time.RFC3339Nano), ID: uuid.UUID(id).String()})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
func nextReadCursor(hasMore bool, items []opportunityapp.OpportunitySummary) (*string, error) {
	if !hasMore || len(items) == 0 {
		return nil, nil
	}
	value, err := encodeReadCursor(items[len(items)-1].CreatedAt, items[len(items)-1].ID)
	return &value, err
}
func nextBidCursor(hasMore bool, items []opportunityapp.BidSummary) (*string, error) {
	if !hasMore || len(items) == 0 {
		return nil, nil
	}
	value, err := encodeReadCursor(items[len(items)-1].CreatedAt, items[len(items)-1].ID)
	return &value, err
}
func opportunityView(item opportunityapp.OpportunitySummary) opportunityDTO {
	return opportunityDTO{ID: uuid.UUID(item.ID).String(), SourceVersionID: uuid.UUID(item.SourceVersionID).String(), Version: item.Version, Title: item.Title, BuyerName: item.BuyerName, ClosesAt: formatOptionalTime(item.ClosesAt), SourceTimezone: item.SourceTimezone, VerificationState: item.VerificationState, CreatedAt: item.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.Format(time.RFC3339Nano)}
}
func bidView(item opportunityapp.BidSummary) bidDTO {
	return bidDTO{ID: uuid.UUID(item.ID).String(), OpportunityID: uuid.UUID(item.OpportunityID).String(), SourceVersionID: uuid.UUID(item.SourceVersionID).String(), OwnerID: uuid.UUID(item.OwnerID).String(), State: item.State, Version: item.Version, Title: item.Title, BuyerName: item.BuyerName, ClosesAt: formatOptionalTime(item.ClosesAt), SourceTimezone: item.SourceTimezone, CreatedAt: item.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.Format(time.RFC3339Nano), ArchivedAt: formatOptionalTime(item.ArchivedAt)}
}
func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	text := value.UTC().Format(time.RFC3339Nano)
	return &text
}
func optionalID(id opportunityapp.ID, present bool) *string {
	if !present {
		return nil
	}
	value := uuid.UUID(id).String()
	return &value
}
func readFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "opportunity_read_not_authorized", "bid_read_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to view this resource.", false)
	case "opportunity_not_found", "bid_not_found":
		return fail(c, 404, "NOT_FOUND", "The resource was not found.", false)
	case "opportunity_read_invalid", "bid_read_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	default:
		return fail(c, 503, "READ_UNAVAILABLE", "The resource is temporarily unavailable.", true)
	}
}
