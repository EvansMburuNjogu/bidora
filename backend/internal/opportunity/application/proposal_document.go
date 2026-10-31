package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrProposalNotFound = errors.New("proposal_not_found")
var ErrProposalConflict = errors.New("proposal_conflict")

type SaveProposalInput struct {
	Subject                                             WorkflowSubject
	BidID, CorrelationID                                ID
	Title, Recipient, Introduction, Closing, ChangeNote string
	LetterheadLines                                     []string
	ExpectedProposalVersion, ExpectedWorkspaceVersion   int64
	IdempotencyKey                                      string
}
type ProposalCommit struct {
	CommandID, ProposalID, ProposalVersionID, AuditID, OutboxID, BidID, CorrelationID ID
	Title, Recipient, Introduction, Closing, ChangeNote                               string
	LetterheadJSON                                                                    []byte
	ExpectedProposalVersion, ExpectedWorkspaceVersion                                 int64
	KeyHash, RequestHash                                                              [32]byte
	OccurredAt                                                                        time.Time
}
type ProposalResponseSnapshot struct {
	RequirementID, ResponseVersionID   ID
	RequirementText, Category, Content string
	Mandatory                          bool
	ResponseRevision                   int64
	Ordinal                            int32
	CreatedAt                          time.Time
}
type ProposalDocument struct {
	ProposalID, ProposalVersionID, BidID                                       ID
	LetterheadVersionID                                                        *ID
	LetterheadVersionNumber                                                    *int64
	LetterheadTemplateKey                                                      *string
	LetterheadContentHash                                                      *[32]byte
	OrganizationLegalName, Title, Recipient, Introduction, Closing, ChangeNote string
	LetterheadLines                                                            []string
	ProposalVersion, Revision, WorkspaceVersion                                int64
	CreatedBy                                                                  ID
	CreatedAt                                                                  time.Time
	Responses                                                                  []ProposalResponseSnapshot
}
type ProposalHistoryItem struct {
	ProposalVersionID ID
	Revision          int64
	Title, ChangeNote string
	CreatedBy         ID
	CreatedAt         time.Time
}
type ProposalSaveResult struct {
	Proposal ProposalDocument
	Replay   bool
}
type ProposalStore interface {
	SaveProposal(context.Context, WorkflowAuthority, ProposalCommit) (ProposalSaveResult, error)
	GetProposal(context.Context, WorkflowAuthority, ID) (ProposalDocument, error)
	GetProposalVersion(context.Context, WorkflowAuthority, ID, ID) (ProposalDocument, error)
	ListProposalHistory(context.Context, WorkflowAuthority, ID) ([]ProposalHistoryItem, error)
}
type ProposalService struct {
	auth  TechnicalResponseAuthorizer
	store ProposalStore
	ids   CaptureIDGenerator
	clock CaptureClock
}

func NewProposalService(auth TechnicalResponseAuthorizer, store ProposalStore, ids CaptureIDGenerator, clock CaptureClock) (*ProposalService, error) {
	if auth == nil || store == nil || ids == nil || clock == nil {
		return nil, errors.New("proposal_service_invalid")
	}
	return &ProposalService{auth, store, ids, clock}, nil
}
func (s *ProposalService) Save(ctx context.Context, in SaveProposalInput) (ProposalSaveResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedProposalVersion < 0 || in.ExpectedWorkspaceVersion < 1 || in.IdempotencyKey == "" || !validRequirementText(in.Title, 300) || !validRequirementText(in.Recipient, 1000) || !validRequirementText(in.Introduction, 10000) || !validRequirementText(in.Closing, 10000) || !validRequirementText(in.ChangeNote, 2000) || !validLetterhead(in.LetterheadLines) {
		return ProposalSaveResult{}, safe("proposal_invalid", nil)
	}
	authority, err := s.auth.AuthorizeResponseEdit(ctx, in.Subject)
	if err != nil {
		return ProposalSaveResult{}, safe("proposal_not_authorized", err)
	}
	letterhead, _ := json.Marshal(in.LetterheadLines)
	requestBytes, _ := json.Marshal(struct {
		Bid                                                 string
		Title, Recipient, Introduction, Closing, ChangeNote string
		Letterhead                                          []string
		ProposalVersion, WorkspaceVersion                   int64
	}{fmt.Sprintf("%x", in.BidID), in.Title, in.Recipient, in.Introduction, in.Closing, in.ChangeNote, in.LetterheadLines, in.ExpectedProposalVersion, in.ExpectedWorkspaceVersion})
	ids, err := responseDraftIDs(s.ids, 5)
	if err != nil {
		return ProposalSaveResult{}, safe("proposal_unavailable", err)
	}
	result, err := s.store.SaveProposal(ctx, authority, ProposalCommit{CommandID: ids[0], ProposalID: ids[1], ProposalVersionID: ids[2], AuditID: ids[3], OutboxID: ids[4], BidID: in.BidID, CorrelationID: in.CorrelationID, Title: in.Title, Recipient: in.Recipient, Introduction: in.Introduction, Closing: in.Closing, ChangeNote: in.ChangeNote, LetterheadJSON: letterhead, ExpectedProposalVersion: in.ExpectedProposalVersion, ExpectedWorkspaceVersion: in.ExpectedWorkspaceVersion, KeyHash: sha256.Sum256([]byte("save-proposal:v1:" + in.IdempotencyKey)), RequestHash: sha256.Sum256(append([]byte("save-proposal:v1:"), requestBytes...)), OccurredAt: s.clock.Now().UTC()})
	if errors.Is(err, ErrProposalConflict) {
		return ProposalSaveResult{}, safe("proposal_conflict", err)
	}
	if err != nil {
		return ProposalSaveResult{}, safe("proposal_unavailable", err)
	}
	return result, nil
}
func (s *ProposalService) Get(ctx context.Context, subject WorkflowSubject, bidID ID) (ProposalDocument, error) {
	a, err := s.readAuthority(ctx, subject, bidID)
	if err != nil {
		return ProposalDocument{}, err
	}
	v, err := s.store.GetProposal(ctx, a, bidID)
	if errors.Is(err, ErrProposalNotFound) {
		return ProposalDocument{}, safe("proposal_not_found", err)
	}
	if err != nil {
		return ProposalDocument{}, safe("proposal_read_unavailable", err)
	}
	return v, nil
}
func (s *ProposalService) History(ctx context.Context, subject WorkflowSubject, bidID ID) ([]ProposalHistoryItem, error) {
	a, err := s.readAuthority(ctx, subject, bidID)
	if err != nil {
		return nil, err
	}
	v, err := s.store.ListProposalHistory(ctx, a, bidID)
	if err != nil {
		return nil, safe("proposal_read_unavailable", err)
	}
	return v, nil
}
func (s *ProposalService) readAuthority(ctx context.Context, subject WorkflowSubject, bidID ID) (WorkflowAuthority, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() {
		return WorkflowAuthority{}, safe("proposal_read_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return WorkflowAuthority{}, safe("proposal_not_authorized", err)
	}
	return a, nil
}
func validLetterhead(lines []string) bool {
	if len(lines) > 10 {
		return false
	}
	total := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != line || !validRequirementText(line, 300) {
			return false
		}
		total += len(line)
	}
	return total <= 3000
}
