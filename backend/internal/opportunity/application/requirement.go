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

var ErrRequirementConflict = errors.New("requirement_conflict")

type CreateRequirementInput struct {
	Subject                                        WorkflowSubject
	BidID, SourceVersionID, OwnerID, CorrelationID ID
	SourceLocation, Text, Category, IdempotencyKey string
	Mandatory, HasOwner                            bool
	DueAt                                          *time.Time
	ExpectedWorkspaceVersion                       int64
}
type AssignRequirementInput struct {
	Subject                                              WorkflowSubject
	BidID, RequirementID, OwnerID, CorrelationID         ID
	DueAt                                                *time.Time
	ExpectedWorkspaceVersion, ExpectedRequirementVersion int64
	IdempotencyKey                                       string
}
type Requirement struct {
	ID, BidID, SourceVersionID, OwnerID    ID
	HasOwner                               bool
	SourceLocation, Text, Category, Status string
	Mandatory                              bool
	DueAt                                  *time.Time
	Version                                int64
	CreatedAt, UpdatedAt                   time.Time
}
type RequirementResult struct {
	Requirement      Requirement
	WorkspaceVersion int64
	Replay           bool
}
type Readiness struct {
	BidID                                                                                              ID
	WorkspaceState                                                                                     string
	WorkspaceVersion, OpenMandatoryCount, UnassignedMandatoryCount, OverdueCount, ExpiredEvidenceCount int64
	Ready                                                                                              bool
	Blockers                                                                                           []string
}
type RequirementCommit struct {
	CommandID, RequirementID, BidID, SourceVersionID, OwnerID, AuditID, OutboxID, CorrelationID ID
	HasOwner                                                                                    bool
	SourceLocation, Text, Category                                                              string
	Mandatory                                                                                   bool
	DueAt                                                                                       *time.Time
	ExpectedWorkspaceVersion                                                                    int64
	KeyHash, RequestHash                                                                        [32]byte
	OccurredAt                                                                                  time.Time
}
type AssignmentCommit struct {
	CommandID, BidID, RequirementID, OwnerID, AuditID, OutboxID, CorrelationID ID
	DueAt                                                                      *time.Time
	ExpectedWorkspaceVersion, ExpectedRequirementVersion                       int64
	KeyHash, RequestHash                                                       [32]byte
	OccurredAt                                                                 time.Time
}
type RequirementAuthorizer interface {
	AuthorizeRequirementCreate(context.Context, WorkflowSubject) (WorkflowAuthority, error)
	AuthorizeRequirementAssign(context.Context, WorkflowSubject, ID) (WorkflowAuthority, error)
	AuthorizeReadiness(context.Context, WorkflowSubject) (WorkflowAuthority, error)
}
type RequirementStore interface {
	CreateRequirement(context.Context, WorkflowAuthority, RequirementCommit) (RequirementResult, error)
	AssignRequirement(context.Context, WorkflowAuthority, AssignmentCommit) (RequirementResult, error)
	ListRequirements(context.Context, WorkflowAuthority, ID) ([]Requirement, error)
	GetReadiness(context.Context, WorkflowAuthority, ID, time.Time) (Readiness, error)
}
type RequirementService struct {
	auth  RequirementAuthorizer
	store RequirementStore
	ids   CaptureIDGenerator
	clock CaptureClock
}

func NewRequirementService(auth RequirementAuthorizer, store RequirementStore, ids CaptureIDGenerator, clock CaptureClock) (*RequirementService, error) {
	if auth == nil || store == nil || ids == nil || clock == nil {
		return nil, errors.New("requirement_service_invalid")
	}
	return &RequirementService{auth, store, ids, clock}, nil
}
func (s *RequirementService) Create(ctx context.Context, input CreateRequirementInput) (RequirementResult, error) {
	if ctx == nil || !validWorkflowSubject(input.Subject) || input.BidID.IsZero() || input.SourceVersionID.IsZero() || input.CorrelationID.IsZero() || input.ExpectedWorkspaceVersion < 1 || input.IdempotencyKey == "" || !validRequirementText(input.Text, 10000) || !validRequirementText(input.SourceLocation, 500) || !validRequirementText(input.Category, 100) || (input.HasOwner && input.OwnerID.IsZero()) {
		return RequirementResult{}, safe("requirement_invalid", nil)
	}
	authority, err := s.auth.AuthorizeRequirementCreate(ctx, input.Subject)
	if err != nil {
		return RequirementResult{}, safe("requirement_not_authorized", err)
	}
	if input.HasOwner {
		if _, err = s.auth.AuthorizeRequirementAssign(ctx, input.Subject, input.OwnerID); err != nil {
			return RequirementResult{}, safe("requirement_not_authorized", err)
		}
	}
	ids, err := s.newIDs(4)
	if err != nil {
		return RequirementResult{}, safe("requirement_unavailable", err)
	}
	keyHash := sha256.Sum256([]byte("create-requirement:v1:" + input.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("create-requirement:v1:%x:%x:%d:%t:%s:%s:%s:%x:%v", input.BidID, input.SourceVersionID, input.ExpectedWorkspaceVersion, input.Mandatory, input.SourceLocation, input.Text, input.Category, input.OwnerID, input.DueAt)))
	result, err := s.store.CreateRequirement(ctx, authority, RequirementCommit{CommandID: ids[0], RequirementID: ids[1], BidID: input.BidID, SourceVersionID: input.SourceVersionID, OwnerID: input.OwnerID, AuditID: ids[2], OutboxID: ids[3], CorrelationID: input.CorrelationID, HasOwner: input.HasOwner, SourceLocation: input.SourceLocation, Text: input.Text, Category: input.Category, Mandatory: input.Mandatory, DueAt: input.DueAt, ExpectedWorkspaceVersion: input.ExpectedWorkspaceVersion, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: s.clock.Now().UTC()})
	if errors.Is(err, ErrRequirementConflict) || errors.Is(err, ErrWorkflowConflict) {
		return RequirementResult{}, safe("requirement_conflict", err)
	}
	if err != nil {
		return RequirementResult{}, safe("requirement_unavailable", err)
	}
	return result, nil
}
func (s *RequirementService) Assign(ctx context.Context, input AssignRequirementInput) (RequirementResult, error) {
	if ctx == nil || !validWorkflowSubject(input.Subject) || input.BidID.IsZero() || input.RequirementID.IsZero() || input.OwnerID.IsZero() || input.CorrelationID.IsZero() || input.ExpectedWorkspaceVersion < 1 || input.ExpectedRequirementVersion < 1 || input.IdempotencyKey == "" {
		return RequirementResult{}, safe("requirement_assignment_invalid", nil)
	}
	authority, err := s.auth.AuthorizeRequirementAssign(ctx, input.Subject, input.OwnerID)
	if err != nil {
		return RequirementResult{}, safe("requirement_assignment_not_authorized", err)
	}
	ids, err := s.newIDs(3)
	if err != nil {
		return RequirementResult{}, safe("requirement_assignment_unavailable", err)
	}
	keyHash := sha256.Sum256([]byte("assign-requirement:v1:" + input.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("assign-requirement:v1:%x:%x:%x:%d:%d:%v", input.BidID, input.RequirementID, input.OwnerID, input.ExpectedWorkspaceVersion, input.ExpectedRequirementVersion, input.DueAt)))
	result, err := s.store.AssignRequirement(ctx, authority, AssignmentCommit{CommandID: ids[0], BidID: input.BidID, RequirementID: input.RequirementID, OwnerID: input.OwnerID, AuditID: ids[1], OutboxID: ids[2], CorrelationID: input.CorrelationID, DueAt: input.DueAt, ExpectedWorkspaceVersion: input.ExpectedWorkspaceVersion, ExpectedRequirementVersion: input.ExpectedRequirementVersion, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: s.clock.Now().UTC()})
	if errors.Is(err, ErrRequirementConflict) || errors.Is(err, ErrWorkflowConflict) {
		return RequirementResult{}, safe("requirement_assignment_conflict", err)
	}
	if err != nil {
		return RequirementResult{}, safe("requirement_assignment_unavailable", err)
	}
	return result, nil
}
func (s *RequirementService) List(ctx context.Context, subject WorkflowSubject, bidID ID) ([]Requirement, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() {
		return nil, safe("requirement_read_invalid", nil)
	}
	authority, err := s.auth.AuthorizeReadiness(ctx, subject)
	if err != nil {
		return nil, safe("requirement_read_not_authorized", err)
	}
	rows, err := s.store.ListRequirements(ctx, authority, bidID)
	if err != nil {
		return nil, safe("requirement_read_unavailable", err)
	}
	return rows, nil
}
func (s *RequirementService) Readiness(ctx context.Context, subject WorkflowSubject, bidID ID) (Readiness, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() {
		return Readiness{}, safe("readiness_invalid", nil)
	}
	authority, err := s.auth.AuthorizeReadiness(ctx, subject)
	if err != nil {
		return Readiness{}, safe("readiness_not_authorized", err)
	}
	result, err := s.store.GetReadiness(ctx, authority, bidID, s.clock.Now().UTC())
	if err != nil {
		return Readiness{}, safe("readiness_unavailable", err)
	}
	return result, nil
}
func (s *RequirementService) newIDs(count int) ([]ID, error) {
	ids := make([]ID, count)
	for i := range ids {
		var err error
		ids[i], err = s.ids.NewID()
		if err != nil || ids[i].IsZero() {
			return nil, errors.New("requirement_id_unavailable")
		}
	}
	return ids, nil
}
func validRequirementText(value string, max int) bool {
	return value != "" && len(value) <= max && utf8.ValidString(value) && strings.TrimSpace(value) != ""
}
