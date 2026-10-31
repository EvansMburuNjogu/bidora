package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
)

var ErrAIRequirementReviewConflict = errors.New("ai_requirement_review_conflict")

type AISuggestion struct {
	ID, DocumentVersionID, AcceptedRequirementID, ReviewedBy                            ID
	HasAcceptedRequirement, HasReviewedBy                                               bool
	Ordinal                                                                             int32
	RequirementText, Category, CitationLocation, CitationQuote, ReviewState, ReviewNote string
	Mandatory                                                                           bool
	CitationPage                                                                        int32
	Confidence                                                                          float64
	Version                                                                             int64
	CreatedAt                                                                           time.Time
	ReviewedAt                                                                          *time.Time
}
type AcceptAISuggestionInput struct {
	Subject                                                                                        WorkflowSubject
	OpportunityID, DocumentVersionID, SuggestionID, BidID, SourceVersionID, OwnerID, CorrelationID ID
	HasOwner                                                                                       bool
	RequirementText, Category, ReviewNote, IdempotencyKey                                          string
	Mandatory                                                                                      bool
	DueAt                                                                                          *time.Time
	ExpectedSuggestionVersion, ExpectedWorkspaceVersion                                            int64
}
type RejectAISuggestionInput struct {
	Subject                                                       WorkflowSubject
	OpportunityID, DocumentVersionID, SuggestionID, CorrelationID ID
	ReviewNote, IdempotencyKey                                    string
	ExpectedSuggestionVersion                                     int64
}
type AISuggestionReviewResult struct {
	Suggestion       AISuggestion
	Requirement      *Requirement
	WorkspaceVersion int64
	Replay           bool
}
type AIAcceptCommit struct {
	CommandID, OpportunityID, DocumentVersionID, SuggestionID, BidID, SourceVersionID, RequirementID, OwnerID, AuditID, OutboxID, CorrelationID ID
	HasOwner                                                                                                                                    bool
	RequirementText, Category, ReviewNote                                                                                                       string
	Mandatory                                                                                                                                   bool
	DueAt                                                                                                                                       *time.Time
	ExpectedSuggestionVersion, ExpectedWorkspaceVersion                                                                                         int64
	KeyHash, RequestHash                                                                                                                        [32]byte
	OccurredAt                                                                                                                                  time.Time
}
type AIRejectCommit struct {
	CommandID, OpportunityID, DocumentVersionID, SuggestionID, AuditID, OutboxID, CorrelationID ID
	ReviewNote                                                                                  string
	ExpectedSuggestionVersion                                                                   int64
	KeyHash, RequestHash                                                                        [32]byte
	OccurredAt                                                                                  time.Time
}
type AIRequirementReviewStore interface {
	ListAISuggestions(context.Context, WorkflowAuthority, ID, ID) ([]AISuggestion, error)
	AcceptAISuggestion(context.Context, WorkflowAuthority, AIAcceptCommit) (AISuggestionReviewResult, error)
	RejectAISuggestion(context.Context, WorkflowAuthority, AIRejectCommit) (AISuggestionReviewResult, error)
}
type AIRequirementReviewService struct {
	auth  RequirementAuthorizer
	store AIRequirementReviewStore
	ids   CaptureIDGenerator
	clock CaptureClock
}

func NewAIRequirementReviewService(auth RequirementAuthorizer, store AIRequirementReviewStore, ids CaptureIDGenerator, clock CaptureClock) (*AIRequirementReviewService, error) {
	if auth == nil || store == nil || ids == nil || clock == nil {
		return nil, errors.New("ai_requirement_review_service_invalid")
	}
	return &AIRequirementReviewService{auth, store, ids, clock}, nil
}
func (s *AIRequirementReviewService) List(ctx context.Context, subject WorkflowSubject, opportunityID, documentVersionID ID) ([]AISuggestion, error) {
	if ctx == nil || !validWorkflowSubject(subject) || opportunityID.IsZero() || documentVersionID.IsZero() {
		return nil, safe("ai_requirement_review_invalid", nil)
	}
	authority, err := s.auth.AuthorizeReadiness(ctx, subject)
	if err != nil {
		return nil, safe("ai_requirement_review_not_authorized", err)
	}
	rows, err := s.store.ListAISuggestions(ctx, authority, opportunityID, documentVersionID)
	if err != nil {
		return nil, safe("ai_requirement_review_unavailable", err)
	}
	return rows, nil
}
func (s *AIRequirementReviewService) Accept(ctx context.Context, in AcceptAISuggestionInput) (AISuggestionReviewResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.OpportunityID.IsZero() || in.DocumentVersionID.IsZero() || in.SuggestionID.IsZero() || in.BidID.IsZero() || in.SourceVersionID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedSuggestionVersion < 1 || in.ExpectedWorkspaceVersion < 1 || in.IdempotencyKey == "" || !validRequirementText(in.RequirementText, 10000) || !validRequirementText(in.Category, 100) || !validOptionalReviewNote(in.ReviewNote) || (in.HasOwner && in.OwnerID.IsZero()) {
		return AISuggestionReviewResult{}, safe("ai_requirement_accept_invalid", nil)
	}
	authority, err := s.auth.AuthorizeRequirementCreate(ctx, in.Subject)
	if err != nil {
		return AISuggestionReviewResult{}, safe("ai_requirement_review_not_authorized", err)
	}
	if in.HasOwner {
		if _, err = s.auth.AuthorizeRequirementAssign(ctx, in.Subject, in.OwnerID); err != nil {
			return AISuggestionReviewResult{}, safe("ai_requirement_review_not_authorized", err)
		}
	}
	ids, err := s.newIDs(4)
	if err != nil {
		return AISuggestionReviewResult{}, safe("ai_requirement_review_unavailable", err)
	}
	keyHash := sha256.Sum256([]byte("accept-ai-requirement:v1:" + in.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("accept-ai-requirement:v1:%x:%x:%x:%x:%x:%d:%d:%s:%s:%t:%x:%v:%s", in.OpportunityID, in.DocumentVersionID, in.SuggestionID, in.BidID, in.SourceVersionID, in.ExpectedSuggestionVersion, in.ExpectedWorkspaceVersion, in.RequirementText, in.Category, in.Mandatory, in.OwnerID, in.DueAt, in.ReviewNote)))
	result, err := s.store.AcceptAISuggestion(ctx, authority, AIAcceptCommit{CommandID: ids[0], OpportunityID: in.OpportunityID, DocumentVersionID: in.DocumentVersionID, SuggestionID: in.SuggestionID, BidID: in.BidID, SourceVersionID: in.SourceVersionID, RequirementID: ids[1], OwnerID: in.OwnerID, AuditID: ids[2], OutboxID: ids[3], CorrelationID: in.CorrelationID, HasOwner: in.HasOwner, RequirementText: in.RequirementText, Category: in.Category, ReviewNote: in.ReviewNote, Mandatory: in.Mandatory, DueAt: in.DueAt, ExpectedSuggestionVersion: in.ExpectedSuggestionVersion, ExpectedWorkspaceVersion: in.ExpectedWorkspaceVersion, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: s.clock.Now().UTC()})
	if errors.Is(err, ErrAIRequirementReviewConflict) {
		return AISuggestionReviewResult{}, safe("ai_requirement_review_conflict", err)
	}
	if err != nil {
		return AISuggestionReviewResult{}, safe("ai_requirement_review_unavailable", err)
	}
	return result, nil
}
func (s *AIRequirementReviewService) Reject(ctx context.Context, in RejectAISuggestionInput) (AISuggestionReviewResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.OpportunityID.IsZero() || in.DocumentVersionID.IsZero() || in.SuggestionID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedSuggestionVersion < 1 || in.IdempotencyKey == "" || !validRequirementText(in.ReviewNote, 2000) {
		return AISuggestionReviewResult{}, safe("ai_requirement_reject_invalid", nil)
	}
	authority, err := s.auth.AuthorizeRequirementCreate(ctx, in.Subject)
	if err != nil {
		return AISuggestionReviewResult{}, safe("ai_requirement_review_not_authorized", err)
	}
	ids, err := s.newIDs(3)
	if err != nil {
		return AISuggestionReviewResult{}, safe("ai_requirement_review_unavailable", err)
	}
	keyHash := sha256.Sum256([]byte("reject-ai-requirement:v1:" + in.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("reject-ai-requirement:v1:%x:%x:%x:%d:%s", in.OpportunityID, in.DocumentVersionID, in.SuggestionID, in.ExpectedSuggestionVersion, in.ReviewNote)))
	result, err := s.store.RejectAISuggestion(ctx, authority, AIRejectCommit{CommandID: ids[0], OpportunityID: in.OpportunityID, DocumentVersionID: in.DocumentVersionID, SuggestionID: in.SuggestionID, AuditID: ids[1], OutboxID: ids[2], CorrelationID: in.CorrelationID, ReviewNote: in.ReviewNote, ExpectedSuggestionVersion: in.ExpectedSuggestionVersion, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: s.clock.Now().UTC()})
	if errors.Is(err, ErrAIRequirementReviewConflict) {
		return AISuggestionReviewResult{}, safe("ai_requirement_review_conflict", err)
	}
	if err != nil {
		return AISuggestionReviewResult{}, safe("ai_requirement_review_unavailable", err)
	}
	return result, nil
}
func (s *AIRequirementReviewService) newIDs(n int) ([]ID, error) {
	out := make([]ID, n)
	for i := range out {
		var err error
		out[i], err = s.ids.NewID()
		if err != nil || out[i].IsZero() {
			return nil, errors.New("ai_review_id_unavailable")
		}
	}
	return out, nil
}
func validOptionalReviewNote(value string) bool {
	return value == "" || validRequirementText(value, 2000)
}
