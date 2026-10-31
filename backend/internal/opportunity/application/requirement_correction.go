package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"
)

var ErrRequirementCorrectionConflict = errors.New("requirement_correction_conflict")

type CorrectRequirementInput struct {
	Subject                                              WorkflowSubject
	BidID, RequirementID, CorrelationID                  ID
	SourceLocation, Text, Category, CorrectionReason     string
	Mandatory                                            bool
	ExpectedRequirementVersion, ExpectedWorkspaceVersion int64
	IdempotencyKey                                       string
}
type RequirementCorrectionCommit struct {
	CommandID, ContentRevisionID, AuditID, OutboxID, BidID, RequirementID, CorrelationID ID
	SourceLocation, Text, Category, CorrectionReason                                     string
	Mandatory                                                                            bool
	ExpectedRequirementVersion, ExpectedWorkspaceVersion                                 int64
	KeyHash, RequestHash                                                                 [32]byte
	OccurredAt                                                                           time.Time
}
type RequirementContentRevision struct {
	ID, RequirementID, BidID, SourceVersionID, SourceDocumentVersionID, CreatedBy ID
	HasSourceDocument                                                             bool
	Revision                                                                      int64
	SourceLocation, Text, Category, CorrectionReason                              string
	Mandatory                                                                     bool
	CreatedAt                                                                     time.Time
}
type RequirementCorrectionResult struct {
	Requirement      Requirement
	ContentRevision  RequirementContentRevision
	WorkspaceVersion int64
	Replay           bool
}
type RequirementCorrectionStore interface {
	CorrectRequirement(context.Context, WorkflowAuthority, RequirementCorrectionCommit) (RequirementCorrectionResult, error)
	ListRequirementCorrections(context.Context, WorkflowAuthority, ID, ID) ([]RequirementContentRevision, error)
}
type RequirementCorrectionService struct {
	auth  RequirementAuthorizer
	store RequirementCorrectionStore
	ids   CaptureIDGenerator
	clock CaptureClock
}

func NewRequirementCorrectionService(auth RequirementAuthorizer, store RequirementCorrectionStore, ids CaptureIDGenerator, clock CaptureClock) (*RequirementCorrectionService, error) {
	if auth == nil || store == nil || ids == nil || clock == nil {
		return nil, errors.New("requirement_correction_service_invalid")
	}
	return &RequirementCorrectionService{auth, store, ids, clock}, nil
}
func (s *RequirementCorrectionService) Correct(ctx context.Context, in CorrectRequirementInput) (RequirementCorrectionResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.RequirementID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedRequirementVersion < 1 || in.ExpectedWorkspaceVersion < 1 || in.IdempotencyKey == "" || !validRequirementText(in.SourceLocation, 500) || !validRequirementText(in.Text, 10000) || !validRequirementText(in.Category, 100) || !validRequirementText(in.CorrectionReason, 2000) {
		return RequirementCorrectionResult{}, safe("requirement_correction_invalid", nil)
	}
	a, err := s.auth.AuthorizeRequirementCreate(ctx, in.Subject)
	if err != nil {
		return RequirementCorrectionResult{}, safe("requirement_correction_not_authorized", err)
	}
	ids, err := responseDraftIDs(s.ids, 4)
	if err != nil {
		return RequirementCorrectionResult{}, safe("requirement_correction_unavailable", err)
	}
	payload, _ := json.Marshal(struct {
		Bid, Requirement                       ID
		SourceLocation, Text, Category, Reason string
		Mandatory                              bool
		RequirementVersion, WorkspaceVersion   int64
	}{in.BidID, in.RequirementID, in.SourceLocation, in.Text, in.Category, in.CorrectionReason, in.Mandatory, in.ExpectedRequirementVersion, in.ExpectedWorkspaceVersion})
	result, err := s.store.CorrectRequirement(ctx, a, RequirementCorrectionCommit{CommandID: ids[0], ContentRevisionID: ids[1], AuditID: ids[2], OutboxID: ids[3], BidID: in.BidID, RequirementID: in.RequirementID, CorrelationID: in.CorrelationID, SourceLocation: in.SourceLocation, Text: in.Text, Category: in.Category, CorrectionReason: in.CorrectionReason, Mandatory: in.Mandatory, ExpectedRequirementVersion: in.ExpectedRequirementVersion, ExpectedWorkspaceVersion: in.ExpectedWorkspaceVersion, KeyHash: sha256.Sum256([]byte("correct-requirement:v1:" + in.IdempotencyKey)), RequestHash: sha256.Sum256(append([]byte("correct-requirement:v1:"), payload...)), OccurredAt: s.clock.Now().UTC()})
	if errors.Is(err, ErrRequirementCorrectionConflict) {
		return RequirementCorrectionResult{}, safe("requirement_correction_conflict", err)
	}
	if err != nil {
		return RequirementCorrectionResult{}, safe("requirement_correction_unavailable", err)
	}
	return result, nil
}
func (s *RequirementCorrectionService) History(ctx context.Context, subject WorkflowSubject, bidID, requirementID ID) ([]RequirementContentRevision, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() || requirementID.IsZero() {
		return nil, safe("requirement_correction_read_invalid", nil)
	}
	a, err := s.auth.AuthorizeReadiness(ctx, subject)
	if err != nil {
		return nil, safe("requirement_correction_not_authorized", err)
	}
	rows, err := s.store.ListRequirementCorrections(ctx, a, bidID, requirementID)
	if err != nil {
		return nil, safe("requirement_correction_read_unavailable", err)
	}
	return rows, nil
}
