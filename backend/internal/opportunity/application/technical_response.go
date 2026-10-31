package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrTechnicalResponseNotFound = errors.New("technical_response_not_found")
var ErrTechnicalResponseConflict = errors.New("technical_response_conflict")

type TechnicalResponse struct {
	SectionID, BidID, RequirementID, ResponseVersionID, CreatedBy ID
	SectionVersion                                                int64
	Revision                                                      int32
	Content, ChangeNote                                           string
	EvidenceIDs                                                   []ID
	CreatedAt                                                     time.Time
}
type SaveTechnicalResponseInput struct {
	Subject                                          WorkflowSubject
	BidID, RequirementID, CorrelationID              ID
	Content, ChangeNote, IdempotencyKey              string
	EvidenceIDs                                      []ID
	ExpectedSectionVersion, ExpectedWorkspaceVersion int64
}
type TechnicalResponseCommit struct {
	CommandID, SectionID, ResponseVersionID, BidID, RequirementID, AuditID, OutboxID, CorrelationID ID
	Content, ChangeNote                                                                             string
	EvidenceIDs                                                                                     []ID
	ExpectedSectionVersion, ExpectedWorkspaceVersion                                                int64
	KeyHash, RequestHash                                                                            [32]byte
	OccurredAt                                                                                      time.Time
}
type TechnicalResponseResult struct {
	Response         TechnicalResponse
	WorkspaceVersion int64
	Replay           bool
}
type RequirementGap struct {
	RequirementID      ID
	RequirementText    string
	Mandatory          bool
	RequirementStatus  string
	RequirementVersion int64
	Blockers           []string
}
type GapReview struct {
	BidID                          ID
	WorkspaceState                 string
	Ready                          bool
	RequirementCount, BlockerCount int
	WorkspaceBlockers              []string
	Requirements                   []RequirementGap
	ObservedAt                     time.Time
}
type TechnicalResponseAuthorizer interface {
	AuthorizeResponseEdit(context.Context, WorkflowSubject) (WorkflowAuthority, error)
	AuthorizeBidRead(context.Context, WorkflowSubject) (WorkflowAuthority, error)
}
type TechnicalResponseStore interface {
	SaveTechnicalResponse(context.Context, WorkflowAuthority, TechnicalResponseCommit) (TechnicalResponseResult, error)
	GetTechnicalResponse(context.Context, WorkflowAuthority, ID, ID) (TechnicalResponse, error)
	ListTechnicalResponseHistory(context.Context, WorkflowAuthority, ID, ID) ([]TechnicalResponse, error)
	GetGapReview(context.Context, WorkflowAuthority, ID, time.Time) (GapReview, error)
}
type TechnicalResponseService struct {
	auth  TechnicalResponseAuthorizer
	store TechnicalResponseStore
	ids   CaptureIDGenerator
	clock CaptureClock
}

func NewTechnicalResponseService(auth TechnicalResponseAuthorizer, store TechnicalResponseStore, ids CaptureIDGenerator, clock CaptureClock) (*TechnicalResponseService, error) {
	if auth == nil || store == nil || ids == nil || clock == nil {
		return nil, errors.New("technical_response_service_invalid")
	}
	return &TechnicalResponseService{auth, store, ids, clock}, nil
}
func (s *TechnicalResponseService) Save(ctx context.Context, in SaveTechnicalResponseInput) (TechnicalResponseResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.RequirementID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedSectionVersion < 0 || in.ExpectedWorkspaceVersion < 1 || in.IdempotencyKey == "" || !validPlainResponse(in.Content, 20000) || !validPlainResponse(in.ChangeNote, 2000) || len(in.EvidenceIDs) > 20 || !uniqueNonzeroIDs(in.EvidenceIDs) {
		return TechnicalResponseResult{}, safe("technical_response_invalid", nil)
	}
	authority, err := s.auth.AuthorizeResponseEdit(ctx, in.Subject)
	if err != nil {
		return TechnicalResponseResult{}, safe("technical_response_not_authorized", err)
	}
	ids, err := s.newIDs(5)
	if err != nil {
		return TechnicalResponseResult{}, safe("technical_response_unavailable", err)
	}
	keyHash := sha256.Sum256([]byte("save-technical-response:v1:" + in.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("save-technical-response:v1:%x:%x:%d:%d:%s:%s:%x", in.BidID, in.RequirementID, in.ExpectedSectionVersion, in.ExpectedWorkspaceVersion, in.Content, in.ChangeNote, in.EvidenceIDs)))
	result, err := s.store.SaveTechnicalResponse(ctx, authority, TechnicalResponseCommit{CommandID: ids[0], SectionID: ids[1], ResponseVersionID: ids[2], BidID: in.BidID, RequirementID: in.RequirementID, AuditID: ids[3], OutboxID: ids[4], CorrelationID: in.CorrelationID, Content: in.Content, ChangeNote: in.ChangeNote, EvidenceIDs: append([]ID(nil), in.EvidenceIDs...), ExpectedSectionVersion: in.ExpectedSectionVersion, ExpectedWorkspaceVersion: in.ExpectedWorkspaceVersion, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: s.clock.Now().UTC()})
	if errors.Is(err, ErrTechnicalResponseConflict) || errors.Is(err, ErrWorkflowConflict) {
		return TechnicalResponseResult{}, safe("technical_response_conflict", err)
	}
	if err != nil {
		return TechnicalResponseResult{}, safe("technical_response_unavailable", err)
	}
	return result, nil
}
func (s *TechnicalResponseService) Get(ctx context.Context, subject WorkflowSubject, bidID, requirementID ID) (TechnicalResponse, error) {
	authority, err := s.readAuthority(ctx, subject, bidID, requirementID)
	if err != nil {
		return TechnicalResponse{}, err
	}
	result, err := s.store.GetTechnicalResponse(ctx, authority, bidID, requirementID)
	if errors.Is(err, ErrTechnicalResponseNotFound) {
		return TechnicalResponse{}, safe("technical_response_not_found", err)
	}
	if err != nil {
		return TechnicalResponse{}, safe("technical_response_read_unavailable", err)
	}
	return result, nil
}
func (s *TechnicalResponseService) History(ctx context.Context, subject WorkflowSubject, bidID, requirementID ID) ([]TechnicalResponse, error) {
	authority, err := s.readAuthority(ctx, subject, bidID, requirementID)
	if err != nil {
		return nil, err
	}
	result, err := s.store.ListTechnicalResponseHistory(ctx, authority, bidID, requirementID)
	if err != nil {
		return nil, safe("technical_response_read_unavailable", err)
	}
	return result, nil
}
func (s *TechnicalResponseService) GapReview(ctx context.Context, subject WorkflowSubject, bidID ID) (GapReview, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() {
		return GapReview{}, safe("gap_review_invalid", nil)
	}
	authority, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return GapReview{}, safe("technical_response_not_authorized", err)
	}
	result, err := s.store.GetGapReview(ctx, authority, bidID, s.clock.Now().UTC())
	if err != nil {
		return GapReview{}, safe("gap_review_unavailable", err)
	}
	return result, nil
}
func (s *TechnicalResponseService) readAuthority(ctx context.Context, subject WorkflowSubject, bidID, requirementID ID) (WorkflowAuthority, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() || requirementID.IsZero() {
		return WorkflowAuthority{}, safe("technical_response_read_invalid", nil)
	}
	authority, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return WorkflowAuthority{}, safe("technical_response_not_authorized", err)
	}
	return authority, nil
}
func (s *TechnicalResponseService) newIDs(n int) ([]ID, error) {
	out := make([]ID, n)
	for i := range out {
		var err error
		out[i], err = s.ids.NewID()
		if err != nil || out[i].IsZero() {
			return nil, errors.New("technical_response_id_unavailable")
		}
	}
	return out, nil
}
func validPlainResponse(value string, max int) bool {
	return value != "" && len(value) <= max && utf8.ValidString(value) && strings.TrimSpace(value) != ""
}
func uniqueNonzeroIDs(values []ID) bool {
	seen := map[ID]struct{}{}
	for _, v := range values {
		if v.IsZero() {
			return false
		}
		if _, ok := seen[v]; ok {
			return false
		}
		seen[v] = struct{}{}
	}
	return true
}
