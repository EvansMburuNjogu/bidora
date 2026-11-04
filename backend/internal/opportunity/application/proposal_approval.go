package application

import (
	"bidora/backend/internal/identity/assurance"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrProposalApprovalConflict = errors.New("proposal_approval_conflict")
var ErrProposalApprovalNotFound = errors.New("proposal_approval_not_found")

type ApproveProposalInput struct {
	Subject                                           WorkflowSubject
	BidID, ManifestID, CorrelationID                  ID
	ExpectedProposalVersion, ExpectedWorkspaceVersion int64
	IdempotencyKey, Rationale                         string
	SessionAuthenticatedAt, AssuranceAt               time.Time
	AssuranceLevel, AssuranceMethod                   string
}
type ProposalApproval struct {
	ID, BidID, ProposalID, ProposalVersionID, ManifestID, ReviewDecisionID, ApproverID ID
	PolicyVersion                                                                      int32
	Rationale                                                                          string
	ApprovedAt                                                                         time.Time
}
type ProposalApprovalResult struct {
	Approval ProposalApproval
	Replay   bool
}
type ProposalApprovalCommit struct {
	CommandID, ApprovalID, AuditID, OutboxID, BidID, ManifestID, CorrelationID ID
	ExpectedProposalVersion, ExpectedWorkspaceVersion                          int64
	KeyHash, RequestHash                                                       [32]byte
	Rationale                                                                  string
	OccurredAt                                                                 time.Time
}
type ProposalApprovalStore interface {
	FindProposalApprovalReplay(context.Context, WorkflowAuthority, ID, [32]byte, [32]byte) (ProposalApprovalResult, bool, error)
	CommitProposalApproval(context.Context, WorkflowAuthority, ProposalApprovalCommit) (ProposalApprovalResult, error)
	GetProposalApprovalByManifest(context.Context, WorkflowAuthority, ID, ID) (ProposalApproval, error)
	GetProposalPackageManifest(context.Context, WorkflowAuthority, ID, ID) (ProposalPackageManifest, error)
}
type ProposalApprovalAuthorizer interface {
	AuthorizeProposalApproval(context.Context, WorkflowSubject) (WorkflowAuthority, error)
	AuthorizeBidRead(context.Context, WorkflowSubject) (WorkflowAuthority, error)
}
type ProposalApprovalService struct {
	auth   ProposalApprovalAuthorizer
	source ProposalPackageSource
	store  ProposalApprovalStore
	ids    CaptureIDGenerator
	clock  CaptureClock
}

func NewProposalApprovalService(a ProposalApprovalAuthorizer, s ProposalPackageSource, st ProposalApprovalStore, ids CaptureIDGenerator, clock CaptureClock) (*ProposalApprovalService, error) {
	if a == nil || s == nil || st == nil || ids == nil || clock == nil {
		return nil, errors.New("proposal_approval_service_invalid")
	}
	return &ProposalApprovalService{a, s, st, ids, clock}, nil
}
func (s *ProposalApprovalService) Approve(ctx context.Context, in ApproveProposalInput) (ProposalApprovalResult, error) {
	in.Rationale = strings.TrimSpace(in.Rationale)
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.ManifestID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedProposalVersion < 1 || in.ExpectedWorkspaceVersion < 1 || in.IdempotencyKey == "" || len(in.IdempotencyKey) > 200 || len(in.Rationale) > 5000 || in.SessionAuthenticatedAt.IsZero() {
		return ProposalApprovalResult{}, safe("proposal_approval_invalid", nil)
	}
	a, err := s.auth.AuthorizeProposalApproval(ctx, in.Subject)
	if err != nil {
		return ProposalApprovalResult{}, safe("proposal_approval_not_authorized", err)
	}
	now := s.clock.Now().UTC()
	age := now.Sub(in.SessionAuthenticatedAt.UTC())
	assuranceAge := now.Sub(in.AssuranceAt.UTC())
	if age < 0 || age > 15*time.Minute || !assurance.Interactive(in.AssuranceLevel, in.AssuranceMethod) || assuranceAge < 0 || assuranceAge > 15*time.Minute {
		return ProposalApprovalResult{}, safe("proposal_approval_fresh_auth_required", nil)
	}
	key := sha256.Sum256([]byte("proposal-approval:v1:" + in.IdempotencyKey))
	raw, _ := json.Marshal(struct {
		BidID, ManifestID   ID
		Proposal, Workspace int64
		Rationale           string
	}{in.BidID, in.ManifestID, in.ExpectedProposalVersion, in.ExpectedWorkspaceVersion, in.Rationale})
	request := sha256.Sum256(raw)
	if replay, found, e := s.store.FindProposalApprovalReplay(ctx, a, in.BidID, key, request); errors.Is(e, ErrProposalApprovalConflict) {
		return ProposalApprovalResult{}, safe("proposal_approval_conflict", e)
	} else if e != nil {
		return ProposalApprovalResult{}, safe("proposal_approval_unavailable", e)
	} else if found {
		return replay, nil
	}
	manifest, e := s.store.GetProposalPackageManifest(ctx, a, in.BidID, in.ManifestID)
	if e != nil {
		return ProposalApprovalResult{}, safe("proposal_approval_not_found", e)
	}
	if manifest.Status != "UNAPPROVED_PREPARATION_ONLY" || !manifest.ReadinessReady || len(manifest.ReadinessBlockers) != 0 {
		return ProposalApprovalResult{}, safe("proposal_approval_blocked", nil)
	}
	readiness, e := s.source.GetProposalApprovalReadiness(ctx, a, in.BidID, now)
	if e != nil {
		return ProposalApprovalResult{}, safe("proposal_approval_unavailable", e)
	}
	if !readiness.Ready || readiness.ProposalVersionID != manifest.ProposalVersionID || readiness.ProposalVersion != in.ExpectedProposalVersion || readiness.WorkspaceVersion != in.ExpectedWorkspaceVersion {
		return ProposalApprovalResult{}, safe("proposal_approval_blocked", nil)
	}
	generated, e := responseDraftIDs(s.ids, 4)
	if e != nil {
		return ProposalApprovalResult{}, safe("proposal_approval_unavailable", e)
	}
	result, e := s.store.CommitProposalApproval(ctx, a, ProposalApprovalCommit{CommandID: generated[0], ApprovalID: generated[1], AuditID: generated[2], OutboxID: generated[3], BidID: in.BidID, ManifestID: in.ManifestID, CorrelationID: in.CorrelationID, ExpectedProposalVersion: in.ExpectedProposalVersion, ExpectedWorkspaceVersion: in.ExpectedWorkspaceVersion, KeyHash: key, RequestHash: request, Rationale: in.Rationale, OccurredAt: now})
	if errors.Is(e, ErrProposalApprovalConflict) {
		return ProposalApprovalResult{}, safe("proposal_approval_conflict", e)
	}
	if e != nil {
		return ProposalApprovalResult{}, safe("proposal_approval_unavailable", e)
	}
	return result, nil
}
func (s *ProposalApprovalService) Get(ctx context.Context, subject WorkflowSubject, bidID, manifestID ID) (ProposalApproval, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() || manifestID.IsZero() {
		return ProposalApproval{}, safe("proposal_approval_read_invalid", nil)
	}
	a, e := s.auth.AuthorizeBidRead(ctx, subject)
	if e != nil {
		return ProposalApproval{}, safe("proposal_approval_not_authorized", e)
	}
	v, e := s.store.GetProposalApprovalByManifest(ctx, a, bidID, manifestID)
	if errors.Is(e, ErrProposalApprovalNotFound) {
		return ProposalApproval{}, safe("proposal_approval_not_found", e)
	}
	if e != nil {
		return ProposalApproval{}, safe("proposal_approval_unavailable", e)
	}
	return v, nil
}
