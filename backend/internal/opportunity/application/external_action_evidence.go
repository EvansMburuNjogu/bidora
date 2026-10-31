package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"
)

var ErrExternalActionEvidenceConflict = errors.New("external_action_evidence_conflict")
var ErrExternalActionEvidenceNotFound = errors.New("external_action_evidence_not_found")
var ErrExternalEvidenceReconciliationLimit = errors.New("external_evidence_reconciliation_limit")

type ReportExternalActionEvidenceInput struct {
	Subject                                                       WorkflowSubject
	BidID, PackageManifestID, SupersedesEvidenceID, CorrelationID ID
	ExternalSystemName                                            string
	ExternalReference                                             *string
	ReportedActionTimeText, ReportNote, IdempotencyKey            string
	ExpectedWorkspaceVersion                                      int64
}
type ExternalActionEvidence struct {
	ID, BidID, PackageManifestID, SupersedesEvidenceID, RecordedBy                         ID
	ReportType, VerificationStatus, ExternalSystemName, ReportedActionTimeText, ReportNote string
	ExternalReference                                                                      *string
	ReportedActionAt, RecordedAt                                                           time.Time
}
type ExternalActionEvidenceCommit struct {
	CommandID, EvidenceID, AuditID, OutboxID, BidID, PackageManifestID, SupersedesEvidenceID, CorrelationID ID
	ExternalSystemName                                                                                      string
	ExternalReference                                                                                       *string
	ReportedActionTimeText, ReportNote                                                                      string
	ReportedActionAt                                                                                        time.Time
	ExpectedWorkspaceVersion                                                                                int64
	KeyHash, RequestHash                                                                                    [32]byte
	OccurredAt                                                                                              time.Time
}
type ExternalActionEvidenceResult struct {
	Evidence ExternalActionEvidence
	Replay   bool
}
type ExternalActionEvidenceHistoryInput struct {
	Subject WorkflowSubject
	BidID   ID
	Cursor  ReadCursor
	Limit   int32
}
type ExternalActionEvidencePage struct {
	Items   []ExternalActionEvidence
	HasMore bool
}
type ActiveExternalEvidenceFact struct {
	EvidenceID, PackageManifestID, ProposalVersionID, SupersedesEvidenceID ID
	ReportedActionAt, RecordedAt, ReadinessObservedAt                      time.Time
	VerificationStatus, PackageStatus                                      string
	ReadinessReady                                                         bool
	LineageDepth                                                           int32
	LineageTruncated                                                       bool
}
type ExternalEvidenceReconciliation struct {
	BidID, CurrentProposalVersionID                    ID
	ReportCount, ActiveReportCount                     int64
	ActiveReports                                      []ActiveExternalEvidenceFact
	ProofPolicyConfigured, CanMarkSubmitted, Submitted bool
	Blockers                                           []string
	ObservedAt                                         time.Time
}
type ExternalActionEvidenceStore interface {
	CommitExternalActionEvidence(context.Context, WorkflowAuthority, ExternalActionEvidenceCommit) (ExternalActionEvidenceResult, error)
	GetExternalActionEvidence(context.Context, WorkflowAuthority, ID, ID) (ExternalActionEvidence, error)
	ListExternalActionEvidence(context.Context, WorkflowAuthority, ID, ReadCursor, int32) ([]ExternalActionEvidence, error)
	GetExternalEvidenceReconciliation(context.Context, WorkflowAuthority, ID, time.Time) (ExternalEvidenceReconciliation, error)
}
type ExternalActionEvidenceAuthorizer interface {
	AuthorizeSubmissionReport(context.Context, WorkflowSubject) (WorkflowAuthority, error)
	AuthorizeBidRead(context.Context, WorkflowSubject) (WorkflowAuthority, error)
}
type ExternalActionEvidenceService struct {
	auth  ExternalActionEvidenceAuthorizer
	store ExternalActionEvidenceStore
	ids   CaptureIDGenerator
	clock CaptureClock
}

func NewExternalActionEvidenceService(auth ExternalActionEvidenceAuthorizer, store ExternalActionEvidenceStore, ids CaptureIDGenerator, clock CaptureClock) (*ExternalActionEvidenceService, error) {
	if auth == nil || store == nil || ids == nil || clock == nil {
		return nil, errors.New("external_action_evidence_service_invalid")
	}
	return &ExternalActionEvidenceService{auth: auth, store: store, ids: ids, clock: clock}, nil
}
func (s *ExternalActionEvidenceService) Report(ctx context.Context, in ReportExternalActionEvidenceInput) (ExternalActionEvidenceResult, error) {
	now := s.clock.Now().UTC()
	reportedAt, parseErr := time.Parse(time.RFC3339, in.ReportedActionTimeText)
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.PackageManifestID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedWorkspaceVersion < 1 || !validRequirementText(in.ExternalSystemName, 200) || !validRequirementText(in.ReportedActionTimeText, 100) || !validRequirementText(in.ReportNote, 5000) || in.IdempotencyKey == "" || parseErr != nil || reportedAt.After(now) || (in.ExternalReference != nil && !validRequirementText(*in.ExternalReference, 500)) {
		return ExternalActionEvidenceResult{}, safe("external_action_evidence_invalid", nil)
	}
	a, err := s.auth.AuthorizeSubmissionReport(ctx, in.Subject)
	if err != nil {
		return ExternalActionEvidenceResult{}, safe("external_action_evidence_not_authorized", err)
	}
	ids, err := responseDraftIDs(s.ids, 4)
	if err != nil {
		return ExternalActionEvidenceResult{}, safe("external_action_evidence_unavailable", err)
	}
	reportedAt = reportedAt.UTC()
	payload, _ := json.Marshal(struct {
		BidID, ManifestID, SupersedesID ID
		ExternalSystem                  string
		ExternalReference               *string
		ReportedText, Note              string
		ExpectedWorkspace               int64
	}{in.BidID, in.PackageManifestID, in.SupersedesEvidenceID, in.ExternalSystemName, in.ExternalReference, in.ReportedActionTimeText, in.ReportNote, in.ExpectedWorkspaceVersion})
	commit := ExternalActionEvidenceCommit{CommandID: ids[0], EvidenceID: ids[1], AuditID: ids[2], OutboxID: ids[3], BidID: in.BidID, PackageManifestID: in.PackageManifestID, SupersedesEvidenceID: in.SupersedesEvidenceID, CorrelationID: in.CorrelationID, ExternalSystemName: in.ExternalSystemName, ExternalReference: in.ExternalReference, ReportedActionTimeText: in.ReportedActionTimeText, ReportNote: in.ReportNote, ReportedActionAt: reportedAt, ExpectedWorkspaceVersion: in.ExpectedWorkspaceVersion, KeyHash: sha256.Sum256([]byte("external-action-evidence:v1:" + in.IdempotencyKey)), RequestHash: sha256.Sum256(payload), OccurredAt: now}
	result, err := s.store.CommitExternalActionEvidence(ctx, a, commit)
	if errors.Is(err, ErrExternalActionEvidenceConflict) {
		return ExternalActionEvidenceResult{}, safe("external_action_evidence_conflict", err)
	}
	if err != nil {
		return ExternalActionEvidenceResult{}, safe("external_action_evidence_unavailable", err)
	}
	return result, nil
}
func (s *ExternalActionEvidenceService) Get(ctx context.Context, subject WorkflowSubject, bidID, evidenceID ID) (ExternalActionEvidence, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() || evidenceID.IsZero() {
		return ExternalActionEvidence{}, safe("external_action_evidence_read_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return ExternalActionEvidence{}, safe("external_action_evidence_not_authorized", err)
	}
	result, err := s.store.GetExternalActionEvidence(ctx, a, bidID, evidenceID)
	if errors.Is(err, ErrExternalActionEvidenceNotFound) {
		return ExternalActionEvidence{}, safe("external_action_evidence_not_found", err)
	}
	if err != nil {
		return ExternalActionEvidence{}, safe("external_action_evidence_read_unavailable", err)
	}
	return result, nil
}
func (s *ExternalActionEvidenceService) Reconciliation(ctx context.Context, subject WorkflowSubject, bidID ID) (ExternalEvidenceReconciliation, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() {
		return ExternalEvidenceReconciliation{}, safe("external_evidence_reconciliation_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return ExternalEvidenceReconciliation{}, safe("external_action_evidence_not_authorized", err)
	}
	result, err := s.store.GetExternalEvidenceReconciliation(ctx, a, bidID, s.clock.Now().UTC())
	if errors.Is(err, ErrExternalActionEvidenceNotFound) {
		return ExternalEvidenceReconciliation{}, safe("external_action_evidence_not_found", err)
	}
	if errors.Is(err, ErrExternalEvidenceReconciliationLimit) {
		return ExternalEvidenceReconciliation{}, safe("external_evidence_reconciliation_limit", err)
	}
	if err != nil {
		return ExternalEvidenceReconciliation{}, safe("external_evidence_reconciliation_unavailable", err)
	}
	result.ProofPolicyConfigured = true
	result.CanMarkSubmitted = false
	result.Blockers = []string{"SUBMISSION_CONFIRMATION_COMMAND_RECHECK_REQUIRED"}
	if result.Submitted {
		result.Blockers = []string{"ALREADY_SUBMITTED"}
		return result, nil
	}
	if result.ReportCount == 0 {
		result.Blockers = append(result.Blockers, "NO_EXTERNAL_ACTION_REPORTS")
	}
	if result.ReportCount > 0 && result.ActiveReportCount == 0 {
		result.Blockers = append(result.Blockers, "NO_ACTIVE_EXTERNAL_ACTION_REPORT")
	}
	if result.ActiveReportCount > 1 {
		result.Blockers = append(result.Blockers, "MULTIPLE_ACTIVE_EXTERNAL_ACTION_REPORTS")
	}
	if result.CurrentProposalVersionID.IsZero() {
		result.Blockers = append(result.Blockers, "CURRENT_PROPOSAL_UNAVAILABLE")
	}
	packageStale, notReady, truncated := false, false, false
	for _, row := range result.ActiveReports {
		if result.CurrentProposalVersionID.IsZero() || row.ProposalVersionID != result.CurrentProposalVersionID {
			packageStale = true
		}
		if !row.ReadinessReady {
			notReady = true
		}
		if row.LineageTruncated {
			truncated = true
		}
	}
	if packageStale {
		result.Blockers = append(result.Blockers, "ACTIVE_REPORT_PACKAGE_NOT_CURRENT")
	}
	if notReady {
		result.Blockers = append(result.Blockers, "PACKAGE_PREPARATION_NOT_READY")
	}
	if truncated {
		result.Blockers = append(result.Blockers, "CORRECTION_LINEAGE_LIMIT_EXCEEDED")
	}
	return result, nil
}
func (s *ExternalActionEvidenceService) List(ctx context.Context, in ExternalActionEvidenceHistoryInput) (ExternalActionEvidencePage, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.Limit < 1 || in.Limit > 100 || (in.Cursor.CreatedAt.IsZero() != in.Cursor.ID.IsZero()) {
		return ExternalActionEvidencePage{}, safe("external_action_evidence_read_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, in.Subject)
	if err != nil {
		return ExternalActionEvidencePage{}, safe("external_action_evidence_not_authorized", err)
	}
	rows, err := s.store.ListExternalActionEvidence(ctx, a, in.BidID, in.Cursor, in.Limit+1)
	if errors.Is(err, ErrExternalActionEvidenceNotFound) {
		return ExternalActionEvidencePage{}, safe("external_action_evidence_not_found", err)
	}
	if err != nil {
		return ExternalActionEvidencePage{}, safe("external_action_evidence_read_unavailable", err)
	}
	result := ExternalActionEvidencePage{Items: rows}
	if len(rows) > int(in.Limit) {
		result.HasMore = true
		result.Items = rows[:in.Limit]
	}
	return result, nil
}
