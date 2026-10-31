package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrSubmissionConflict = errors.New("submission_conflict")
var ErrSubmissionNotFound = errors.New("submission_not_found")

type SubmissionProofUpload struct {
	ID, DocumentID, BidID, EvidenceID, CompletedVersionID ID
	ObjectKey                                             string
	SizeBytes                                             int64
	SHA256                                                [32]byte
	State                                                 string
	ExpiresAt                                             time.Time
	Replay                                                bool
}
type InitiateSubmissionProofInput struct {
	Subject                          WorkflowSubject
	BidID, EvidenceID, CorrelationID ID
	SizeBytes                        int64
	SHA256Hex, IdempotencyKey        string
}
type CompleteSubmissionProofInput struct {
	Subject                                    WorkflowSubject
	BidID, EvidenceID, UploadID, CorrelationID ID
	IdempotencyKey                             string
}
type SubmissionProofReservation struct {
	CommandID, UploadID, DocumentID, BidID, EvidenceID, AuditID, CorrelationID ID
	ObjectKey                                                                  string
	SizeBytes                                                                  int64
	SHA256, KeyHash, RequestHash                                               [32]byte
	CreatedAt, ExpiresAt                                                       time.Time
}
type SubmissionProofCompletion struct {
	CommandID, UploadID, DocumentID, BidID, EvidenceID, VersionID, JobID, AuditID, OutboxID, CorrelationID ID
	ObjectKey, ObjectVersion                                                                               string
	SizeBytes                                                                                              int64
	SHA256, KeyHash, RequestHash                                                                           [32]byte
	OccurredAt                                                                                             time.Time
}
type SubmissionProofResult struct {
	UploadID, DocumentID, VersionID, JobID ID
	ScanState                              string
	Replay                                 bool
}
type ConfirmSubmissionInput struct {
	Subject                                                      WorkflowSubject
	BidID, EvidenceID, ManifestID, ProofVersionID, CorrelationID ID
	ExpectedWorkspaceVersion                                     int64
	Rationale, IdempotencyKey                                    string
	SessionAuthenticatedAt, AssuranceAt                          time.Time
	AssuranceLevel, AssuranceMethod                              string
}
type SubmissionConfirmation struct {
	ID, BidID, EvidenceID, ManifestID, ApprovalID, ProofVersionID, ReporterID, VerifierID ID
	ReportedActionAt, ReportRecordedAt, ConfirmedAt                                       time.Time
	Rationale, SubmissionBasis                                                            string
	PolicyVersion                                                                         int32
	WorkspaceVersion                                                                      int64
}
type SubmissionConfirmationResult struct {
	Confirmation SubmissionConfirmation
	Replay       bool
}
type SubmissionConfirmationCommit struct {
	CommandID, ConfirmationID, AuditID, OutboxID, BidID, EvidenceID, ManifestID, ProofVersionID, CorrelationID ID
	ExpectedWorkspaceVersion                                                                                   int64
	Rationale                                                                                                  string
	KeyHash, RequestHash                                                                                       [32]byte
	OccurredAt                                                                                                 time.Time
}
type SubmissionStore interface {
	ReserveSubmissionProof(context.Context, WorkflowAuthority, SubmissionProofReservation) (SubmissionProofUpload, error)
	GetSubmissionProofUpload(context.Context, WorkflowAuthority, ID, ID, ID) (SubmissionProofUpload, error)
	GetCompletedSubmissionProof(context.Context, WorkflowAuthority, ID, ID, ID) (SubmissionProofResult, error)
	CompleteSubmissionProof(context.Context, WorkflowAuthority, SubmissionProofCompletion) (SubmissionProofResult, error)
	FindSubmissionReplay(context.Context, WorkflowAuthority, ID, [32]byte, [32]byte) (SubmissionConfirmationResult, bool, error)
	CommitSubmissionConfirmation(context.Context, WorkflowAuthority, SubmissionConfirmationCommit) (SubmissionConfirmationResult, error)
	GetSubmissionConfirmationByBid(context.Context, WorkflowAuthority, ID) (SubmissionConfirmation, error)
}
type SubmissionAuthorizer interface {
	AuthorizeSubmissionReport(context.Context, WorkflowSubject) (WorkflowAuthority, error)
	AuthorizeSubmissionVerify(context.Context, WorkflowSubject) (WorkflowAuthority, error)
	AuthorizeBidRead(context.Context, WorkflowSubject) (WorkflowAuthority, error)
}
type SubmissionService struct {
	auth      SubmissionAuthorizer
	store     SubmissionStore
	readiness ProposalPackageSource
	storage   UploadStorage
	ids       CaptureIDGenerator
	clock     CaptureClock
}

func NewSubmissionService(a SubmissionAuthorizer, st SubmissionStore, r ProposalPackageSource, o UploadStorage, ids CaptureIDGenerator, c CaptureClock) (*SubmissionService, error) {
	if a == nil || st == nil || r == nil || o == nil || ids == nil || c == nil {
		return nil, errors.New("submission_service_invalid")
	}
	return &SubmissionService{a, st, r, o, ids, c}, nil
}
func (s *SubmissionService) Initiate(ctx context.Context, in InitiateSubmissionProofInput) (InitiateUploadResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.EvidenceID.IsZero() || in.CorrelationID.IsZero() || in.SizeBytes < 1 || in.SizeBytes > MaxTenderDocumentBytes || in.IdempotencyKey == "" {
		return InitiateUploadResult{}, safe("submission_proof_invalid", nil)
	}
	sum, ok := parseSHA256Hex(in.SHA256Hex)
	if !ok {
		return InitiateUploadResult{}, safe("submission_proof_invalid", nil)
	}
	a, e := s.auth.AuthorizeSubmissionReport(ctx, in.Subject)
	if e != nil {
		return InitiateUploadResult{}, safe("submission_not_authorized", e)
	}
	ids, e := responseDraftIDs(s.ids, 4)
	if e != nil {
		return InitiateUploadResult{}, safe("submission_unavailable", e)
	}
	now := s.clock.Now().UTC()
	key := fmt.Sprintf("tenant/%x/submission-proof/%x", a.TenantID, ids[1])
	kh := sha256.Sum256([]byte("submission-proof-init:v1:" + in.IdempotencyKey))
	rh := sha256.Sum256([]byte(fmt.Sprintf("%x:%x:%d:%s", in.BidID, in.EvidenceID, in.SizeBytes, hex.EncodeToString(sum[:]))))
	u, e := s.store.ReserveSubmissionProof(ctx, a, SubmissionProofReservation{CommandID: ids[0], UploadID: ids[1], DocumentID: ids[2], BidID: in.BidID, EvidenceID: in.EvidenceID, AuditID: ids[3], CorrelationID: in.CorrelationID, ObjectKey: key, SizeBytes: in.SizeBytes, SHA256: sum, KeyHash: kh, RequestHash: rh, CreatedAt: now, ExpiresAt: now.Add(UploadInstructionTTL)})
	if errors.Is(e, ErrSubmissionConflict) {
		return InitiateUploadResult{}, safe("submission_conflict", e)
	}
	if e != nil {
		return InitiateUploadResult{}, safe("submission_unavailable", e)
	}
	instruction, e := s.storage.PresignPut(ctx, u.ObjectKey, "application/pdf", u.SizeBytes, u.SHA256, u.ExpiresAt.Sub(s.clock.Now().UTC()))
	if e != nil {
		return InitiateUploadResult{}, safe("submission_unavailable", e)
	}
	instruction.ExpiresAt = u.ExpiresAt
	return InitiateUploadResult{Session: UploadSession{ID: u.ID, DocumentID: u.DocumentID, ObjectKey: u.ObjectKey, MediaType: "application/pdf", SizeBytes: u.SizeBytes, SHA256: u.SHA256, State: u.State, ExpiresAt: u.ExpiresAt, CompletedVersionID: u.CompletedVersionID, Replay: u.Replay}, Instruction: instruction}, nil
}
func (s *SubmissionService) Complete(ctx context.Context, in CompleteSubmissionProofInput) (SubmissionProofResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.EvidenceID.IsZero() || in.UploadID.IsZero() || in.CorrelationID.IsZero() || in.IdempotencyKey == "" {
		return SubmissionProofResult{}, safe("submission_proof_invalid", nil)
	}
	a, e := s.auth.AuthorizeSubmissionReport(ctx, in.Subject)
	if e != nil {
		return SubmissionProofResult{}, safe("submission_not_authorized", e)
	}
	u, e := s.store.GetSubmissionProofUpload(ctx, a, in.BidID, in.EvidenceID, in.UploadID)
	if e != nil {
		return SubmissionProofResult{}, safe("submission_unavailable", e)
	}
	if u.State == "COMPLETED" {
		v, e := s.store.GetCompletedSubmissionProof(ctx, a, in.BidID, in.EvidenceID, in.UploadID)
		if e == nil {
			v.Replay = true
		}
		return v, e
	}
	now := s.clock.Now().UTC()
	if now.After(u.ExpiresAt) {
		return SubmissionProofResult{}, safe("submission_proof_expired", nil)
	}
	obj, e := s.storage.Inspect(ctx, u.ObjectKey)
	if e != nil {
		return SubmissionProofResult{}, safe("submission_unavailable", e)
	}
	if obj.MediaType != "application/pdf" || obj.SizeBytes != u.SizeBytes || obj.SHA256 != u.SHA256 || obj.ObjectVersion == "" {
		return SubmissionProofResult{}, safe("submission_proof_mismatch", nil)
	}
	ids, e := responseDraftIDs(s.ids, 5)
	if e != nil {
		return SubmissionProofResult{}, safe("submission_unavailable", e)
	}
	kh := sha256.Sum256([]byte("submission-proof-complete:v1:" + in.IdempotencyKey))
	rh := sha256.Sum256([]byte(fmt.Sprintf("%x:%x:%s:%d:%x", in.UploadID, u.DocumentID, obj.ObjectVersion, obj.SizeBytes, obj.SHA256)))
	v, e := s.store.CompleteSubmissionProof(ctx, a, SubmissionProofCompletion{CommandID: ids[0], UploadID: in.UploadID, DocumentID: u.DocumentID, BidID: in.BidID, EvidenceID: in.EvidenceID, VersionID: ids[1], JobID: ids[2], AuditID: ids[3], OutboxID: ids[4], CorrelationID: in.CorrelationID, ObjectKey: u.ObjectKey, ObjectVersion: obj.ObjectVersion, SizeBytes: obj.SizeBytes, SHA256: obj.SHA256, KeyHash: kh, RequestHash: rh, OccurredAt: now})
	if errors.Is(e, ErrSubmissionConflict) {
		return SubmissionProofResult{}, safe("submission_conflict", e)
	}
	if e != nil {
		return SubmissionProofResult{}, safe("submission_unavailable", e)
	}
	return v, nil
}
func (s *SubmissionService) Confirm(ctx context.Context, in ConfirmSubmissionInput) (SubmissionConfirmationResult, error) {
	in.Rationale = strings.TrimSpace(in.Rationale)
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.EvidenceID.IsZero() || in.ManifestID.IsZero() || in.ProofVersionID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedWorkspaceVersion < 1 || !validRequirementText(in.Rationale, 5000) || in.IdempotencyKey == "" || in.SessionAuthenticatedAt.IsZero() {
		return SubmissionConfirmationResult{}, safe("submission_invalid", nil)
	}
	a, e := s.auth.AuthorizeSubmissionVerify(ctx, in.Subject)
	if e != nil {
		return SubmissionConfirmationResult{}, safe("submission_not_authorized", e)
	}
	now := s.clock.Now().UTC()
	age := now.Sub(in.SessionAuthenticatedAt.UTC())
	assuranceAge := now.Sub(in.AssuranceAt.UTC())
	if age < 0 || age > 15*time.Minute || in.AssuranceLevel != "AAL2" || in.AssuranceMethod != "TOTP" || assuranceAge < 0 || assuranceAge > 15*time.Minute {
		return SubmissionConfirmationResult{}, safe("submission_fresh_auth_required", nil)
	}
	raw, _ := json.Marshal(struct {
		BidID, EvidenceID, ManifestID, ProofVersionID ID
		ExpectedWorkspaceVersion                      int64
		Rationale                                     string
	}{in.BidID, in.EvidenceID, in.ManifestID, in.ProofVersionID, in.ExpectedWorkspaceVersion, in.Rationale})
	kh := sha256.Sum256([]byte("submission-confirm:v1:" + in.IdempotencyKey))
	rh := sha256.Sum256(raw)
	// Reauthorize and recheck assurance above, then return an exact committed
	// replay before current readiness. A successful confirmation changes the
	// workspace to SUBMITTED, so checking BID readiness first would make lost
	// response recovery impossible.
	if v, found, e := s.store.FindSubmissionReplay(ctx, a, in.BidID, kh, rh); errors.Is(e, ErrSubmissionConflict) {
		return SubmissionConfirmationResult{}, safe("submission_conflict", e)
	} else if e != nil {
		return SubmissionConfirmationResult{}, safe("submission_unavailable", e)
	} else if found {
		return v, nil
	}
	ready, e := s.readiness.GetProposalApprovalReadiness(ctx, a, in.BidID, now)
	if e != nil || !ready.Ready || ready.WorkspaceVersion != in.ExpectedWorkspaceVersion {
		return SubmissionConfirmationResult{}, safe("submission_blocked", e)
	}
	ids, e := responseDraftIDs(s.ids, 4)
	if e != nil {
		return SubmissionConfirmationResult{}, safe("submission_unavailable", e)
	}
	v, e := s.store.CommitSubmissionConfirmation(ctx, a, SubmissionConfirmationCommit{CommandID: ids[0], ConfirmationID: ids[1], AuditID: ids[2], OutboxID: ids[3], BidID: in.BidID, EvidenceID: in.EvidenceID, ManifestID: in.ManifestID, ProofVersionID: in.ProofVersionID, CorrelationID: in.CorrelationID, ExpectedWorkspaceVersion: in.ExpectedWorkspaceVersion, Rationale: in.Rationale, KeyHash: kh, RequestHash: rh, OccurredAt: now})
	if errors.Is(e, ErrSubmissionConflict) {
		return SubmissionConfirmationResult{}, safe("submission_conflict", e)
	}
	if e != nil {
		return SubmissionConfirmationResult{}, safe("submission_unavailable", e)
	}
	return v, nil
}
func (s *SubmissionService) Get(ctx context.Context, subject WorkflowSubject, bidID ID) (SubmissionConfirmation, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() {
		return SubmissionConfirmation{}, safe("submission_read_invalid", nil)
	}
	a, e := s.auth.AuthorizeBidRead(ctx, subject)
	if e != nil {
		return SubmissionConfirmation{}, safe("submission_not_authorized", e)
	}
	v, e := s.store.GetSubmissionConfirmationByBid(ctx, a, bidID)
	if errors.Is(e, ErrSubmissionNotFound) {
		return SubmissionConfirmation{}, safe("submission_not_found", e)
	}
	if e != nil {
		return SubmissionConfirmation{}, safe("submission_unavailable", e)
	}
	return v, nil
}
