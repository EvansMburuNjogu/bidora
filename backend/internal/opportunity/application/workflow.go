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

const VerifyExternalCapability = "tenderpreneur.opportunity.verify_external"
const CreateBidWorkspaceCapability = "tenderpreneur.bid.workspace.create"
const RecordBidDecisionCapability = "tenderpreneur.bid.decision.record"

var ErrWorkflowConflict = errors.New("opportunity_workflow_conflict")

type WorkflowSubject struct {
	IdentityID, SelectedTenantID ID
	ContextGeneration            int64
}
type WorkflowAuthority struct {
	ActorID, TenantID    ID
	AssignmentGeneration int64
}
type VerifyOpportunityInput struct {
	Subject                                       WorkflowSubject
	OpportunityID, SourceVersionID, CorrelationID ID
	ExpectedVersion                               int64
	IdempotencyKey                                string
}
type CreateWorkspaceInput struct {
	Subject                                                WorkflowSubject
	OpportunityID, SourceVersionID, OwnerID, CorrelationID ID
	ExpectedVersion                                        int64
	IdempotencyKey                                         string
}
type RecordDecisionInput struct {
	Subject                    WorkflowSubject
	WorkspaceID, CorrelationID ID
	ExpectedVersion            int64
	Decision, Rationale        string
	IdempotencyKey             string
}
type VerificationResult struct {
	OpportunityID, SourceVersionID ID
	Version                        int64
	Replay                         bool
}
type WorkspaceResult struct {
	WorkspaceID ID
	Version     int64
	Replay      bool
}
type DecisionResult struct {
	DecisionID, WorkspaceID ID
	Version                 int64
	Decision                string
	Replay                  bool
}

func (VerifyOpportunityInput) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED verify opportunity input]")
}
func (CreateWorkspaceInput) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED create workspace input]")
}
func (RecordDecisionInput) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED record decision input]")
}

type WorkflowAuthorizer interface {
	AuthorizeVerification(context.Context, WorkflowSubject) (WorkflowAuthority, error)
	AuthorizeWorkspaceCreation(context.Context, WorkflowSubject, ID) (WorkflowAuthority, error)
	AuthorizeDecision(context.Context, WorkflowSubject) (WorkflowAuthority, error)
	AuthorizeOpportunityRead(context.Context, WorkflowSubject) (WorkflowAuthority, error)
	AuthorizeBidRead(context.Context, WorkflowSubject) (WorkflowAuthority, error)
}
type WorkflowStore interface {
	Verify(context.Context, WorkflowAuthority, VerifyCommit) (VerificationResult, error)
	CreateWorkspace(context.Context, WorkflowAuthority, WorkspaceCommit) (WorkspaceResult, error)
	RecordDecision(context.Context, WorkflowAuthority, DecisionCommit) (DecisionResult, error)
}
type VerifyCommit struct {
	CommandID, OpportunityID, SourceVersionID, VerifiedSourceVersionID, AuditID, OutboxID, CorrelationID ID
	ExpectedVersion                                                                                      int64
	KeyHash, RequestHash                                                                                 [32]byte
	OccurredAt                                                                                           time.Time
}
type WorkspaceCommit struct {
	CommandID, WorkspaceID, OpportunityID, SourceVersionID, OwnerID, AuditID, OutboxID, CorrelationID ID
	ExpectedVersion                                                                                   int64
	KeyHash, RequestHash                                                                              [32]byte
	OccurredAt                                                                                        time.Time
}
type DecisionCommit struct {
	CommandID, DecisionID, WorkspaceID, AuditID, OutboxID, CorrelationID ID
	ExpectedVersion                                                      int64
	Decision, Rationale                                                  string
	KeyHash, RequestHash                                                 [32]byte
	OccurredAt                                                           time.Time
}
type WorkflowService struct {
	auth  WorkflowAuthorizer
	store WorkflowStore
	ids   CaptureIDGenerator
	clock CaptureClock
}

func NewWorkflowService(auth WorkflowAuthorizer, store WorkflowStore, ids CaptureIDGenerator, clock CaptureClock) (*WorkflowService, error) {
	if auth == nil || store == nil || ids == nil || clock == nil {
		return nil, errors.New("opportunity_workflow_invalid")
	}
	return &WorkflowService{auth, store, ids, clock}, nil
}
func (s *WorkflowService) Verify(ctx context.Context, input VerifyOpportunityInput) (VerificationResult, error) {
	if ctx == nil || !validWorkflowSubject(input.Subject) || input.OpportunityID.IsZero() || input.SourceVersionID.IsZero() || input.CorrelationID.IsZero() || input.ExpectedVersion < 1 || input.IdempotencyKey == "" {
		return VerificationResult{}, safe("opportunity_verification_invalid", nil)
	}
	authority, err := s.auth.AuthorizeVerification(ctx, input.Subject)
	if err != nil {
		return VerificationResult{}, safe("opportunity_verification_not_authorized", err)
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return VerificationResult{}, safe("opportunity_verification_unavailable", nil)
	}
	ids, err := s.newIDs(4)
	if err != nil {
		return VerificationResult{}, safe("opportunity_verification_unavailable", err)
	}
	keyHash := sha256.Sum256([]byte("verify-opportunity:v1:" + input.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("verify-opportunity:v1:%x:%x:%d", input.OpportunityID, input.SourceVersionID, input.ExpectedVersion)))
	result, err := s.store.Verify(ctx, authority, VerifyCommit{CommandID: ids[0], OpportunityID: input.OpportunityID, SourceVersionID: input.SourceVersionID, VerifiedSourceVersionID: ids[1], AuditID: ids[2], OutboxID: ids[3], CorrelationID: input.CorrelationID, ExpectedVersion: input.ExpectedVersion, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: at})
	if err != nil {
		if errors.Is(err, ErrWorkflowConflict) {
			return VerificationResult{}, safe("opportunity_verification_conflict", err)
		}
		return VerificationResult{}, safe("opportunity_verification_unavailable", err)
	}
	return result, nil
}
func (s *WorkflowService) CreateWorkspace(ctx context.Context, input CreateWorkspaceInput) (WorkspaceResult, error) {
	if ctx == nil || !validWorkflowSubject(input.Subject) || input.OpportunityID.IsZero() || input.SourceVersionID.IsZero() || input.OwnerID.IsZero() || input.CorrelationID.IsZero() || input.ExpectedVersion < 1 || input.IdempotencyKey == "" {
		return WorkspaceResult{}, safe("workspace_creation_invalid", nil)
	}
	authority, err := s.auth.AuthorizeWorkspaceCreation(ctx, input.Subject, input.OwnerID)
	if err != nil {
		return WorkspaceResult{}, safe("workspace_creation_not_authorized", err)
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return WorkspaceResult{}, safe("workspace_creation_unavailable", nil)
	}
	ids, err := s.newIDs(4)
	if err != nil {
		return WorkspaceResult{}, safe("workspace_creation_unavailable", err)
	}
	keyHash := sha256.Sum256([]byte("create-workspace:v1:" + input.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("create-workspace:v1:%x:%x:%x:%d", input.OpportunityID, input.SourceVersionID, input.OwnerID, input.ExpectedVersion)))
	result, err := s.store.CreateWorkspace(ctx, authority, WorkspaceCommit{CommandID: ids[0], WorkspaceID: ids[1], OpportunityID: input.OpportunityID, SourceVersionID: input.SourceVersionID, OwnerID: input.OwnerID, AuditID: ids[2], OutboxID: ids[3], CorrelationID: input.CorrelationID, ExpectedVersion: input.ExpectedVersion, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: at})
	if err != nil {
		if errors.Is(err, ErrWorkflowConflict) {
			return WorkspaceResult{}, safe("workspace_creation_conflict", err)
		}
		return WorkspaceResult{}, safe("workspace_creation_unavailable", err)
	}
	return result, nil
}
func (s *WorkflowService) RecordDecision(ctx context.Context, input RecordDecisionInput) (DecisionResult, error) {
	if ctx == nil || !validWorkflowSubject(input.Subject) || input.WorkspaceID.IsZero() || input.CorrelationID.IsZero() || input.ExpectedVersion < 1 || input.IdempotencyKey == "" || (input.Decision != "BID" && input.Decision != "NO_BID") || len(input.Rationale) > 2000 || !utf8.ValidString(input.Rationale) || strings.TrimSpace(input.Rationale) == "" {
		return DecisionResult{}, safe("bid_decision_invalid", nil)
	}
	authority, err := s.auth.AuthorizeDecision(ctx, input.Subject)
	if err != nil {
		return DecisionResult{}, safe("bid_decision_not_authorized", err)
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return DecisionResult{}, safe("bid_decision_unavailable", nil)
	}
	ids, err := s.newIDs(4)
	if err != nil {
		return DecisionResult{}, safe("bid_decision_unavailable", err)
	}
	keyHash := sha256.Sum256([]byte("record-bid-decision:v1:" + input.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("record-bid-decision:v1:%x:%d:%s:%s", input.WorkspaceID, input.ExpectedVersion, input.Decision, input.Rationale)))
	result, err := s.store.RecordDecision(ctx, authority, DecisionCommit{CommandID: ids[0], DecisionID: ids[1], WorkspaceID: input.WorkspaceID, AuditID: ids[2], OutboxID: ids[3], CorrelationID: input.CorrelationID, ExpectedVersion: input.ExpectedVersion, Decision: input.Decision, Rationale: input.Rationale, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: at})
	if err != nil {
		if errors.Is(err, ErrWorkflowConflict) {
			return DecisionResult{}, safe("bid_decision_conflict", err)
		}
		return DecisionResult{}, safe("bid_decision_unavailable", err)
	}
	return result, nil
}
func (s *WorkflowService) newIDs(count int) ([]ID, error) {
	ids := make([]ID, count)
	for index := range ids {
		var err error
		ids[index], err = s.ids.NewID()
		if err != nil || ids[index].IsZero() {
			return nil, errors.New("workflow_id_unavailable")
		}
	}
	return ids, nil
}
func validWorkflowSubject(s WorkflowSubject) bool {
	return !s.IdentityID.IsZero() && !s.SelectedTenantID.IsZero() && s.ContextGeneration > 0
}
