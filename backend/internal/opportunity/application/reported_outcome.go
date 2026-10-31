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

var ErrReportedOutcomeConflict = errors.New("reported_outcome_conflict")
var ErrReportedOutcomeNotFound = errors.New("reported_outcome_not_found")
var ErrReportedOutcomeReadLimit = errors.New("reported_outcome_read_limit")

type OutcomeProofUpload struct {
	ID, DocumentID, BidID, CompletedVersionID ID
	ObjectKey                                 string
	SizeBytes                                 int64
	SHA256                                    [32]byte
	State                                     string
	ExpiresAt                                 time.Time
	Replay                                    bool
}
type InitiateOutcomeProofInput struct {
	Subject                   WorkflowSubject
	BidID, CorrelationID      ID
	SizeBytes                 int64
	SHA256Hex, IdempotencyKey string
}
type CompleteOutcomeProofInput struct {
	Subject                        WorkflowSubject
	BidID, UploadID, CorrelationID ID
	IdempotencyKey                 string
}
type OutcomeProofReservation struct {
	CommandID, UploadID, DocumentID, BidID, AuditID, CorrelationID ID
	ObjectKey                                                      string
	SizeBytes                                                      int64
	SHA256, KeyHash, RequestHash                                   [32]byte
	CreatedAt, ExpiresAt                                           time.Time
}
type OutcomeProofCompletion struct {
	CommandID, UploadID, DocumentID, BidID, VersionID, JobID, AuditID, OutboxID, CorrelationID ID
	ObjectKey, ObjectVersion                                                                   string
	SizeBytes                                                                                  int64
	SHA256, KeyHash, RequestHash                                                               [32]byte
	OccurredAt                                                                                 time.Time
}
type OutcomeProofResult struct {
	UploadID, DocumentID, VersionID, JobID ID
	ScanState                              string
	Replay                                 bool
}
type ReportOutcomeInput struct {
	Subject                                                                                   WorkflowSubject
	BidID, ProofVersionID, SupersedesID, CorrelationID                                        ID
	Outcome, SourceClass, Reason, Lessons, ReportedTimeText, CorrectionReason, IdempotencyKey string
	ExpectedSequence                                                                          int32
	ExpectedWorkspaceVersion                                                                  int64
}
type ReportedOutcome struct {
	ID, BidID, ProofVersionID, SupersedesID, ReportedBy                                                       ID
	Sequence                                                                                                  int32
	Outcome, VerificationStatus, Provenance, SourceClass, Reason, Lessons, ReportedTimeText, CorrectionReason string
	ReportedAt, RecordedAt                                                                                    time.Time
}
type ReportOutcomeCommit struct {
	CommandID, OutcomeID, AuditID, OutboxID, BidID, ProofVersionID, SupersedesID, CorrelationID ID
	ExpectedSequence                                                                            int32
	ExpectedWorkspaceVersion                                                                    int64
	Outcome, SourceClass, Reason, Lessons, ReportedText, CorrectionReason                       string
	ReportedAt, OccurredAt                                                                      time.Time
	KeyHash, RequestHash                                                                        [32]byte
}
type ReportOutcomeResult struct {
	Outcome ReportedOutcome
	Replay  bool
}
type ReportedOutcomeStore interface {
	ReserveOutcomeProof(context.Context, WorkflowAuthority, OutcomeProofReservation) (OutcomeProofUpload, error)
	GetOutcomeProofUpload(context.Context, WorkflowAuthority, ID, ID) (OutcomeProofUpload, error)
	GetCompletedOutcomeProof(context.Context, WorkflowAuthority, ID, ID) (OutcomeProofResult, error)
	CompleteOutcomeProof(context.Context, WorkflowAuthority, OutcomeProofCompletion) (OutcomeProofResult, error)
	FindReportedOutcomeReplay(context.Context, WorkflowAuthority, ID, [32]byte, [32]byte) (ReportOutcomeResult, bool, error)
	CommitReportedOutcome(context.Context, WorkflowAuthority, ReportOutcomeCommit) (ReportOutcomeResult, error)
	GetCurrentReportedOutcome(context.Context, WorkflowAuthority, ID) (ReportedOutcome, error)
	ListReportedOutcomes(context.Context, WorkflowAuthority, ID) ([]ReportedOutcome, error)
}
type ReportedOutcomeAuthorizer interface {
	AuthorizeReportedOutcome(context.Context, WorkflowSubject) (WorkflowAuthority, error)
	AuthorizeBidRead(context.Context, WorkflowSubject) (WorkflowAuthority, error)
}
type ReportedOutcomeService struct {
	auth    ReportedOutcomeAuthorizer
	store   ReportedOutcomeStore
	storage UploadStorage
	ids     CaptureIDGenerator
	clock   CaptureClock
}

func NewReportedOutcomeService(a ReportedOutcomeAuthorizer, s ReportedOutcomeStore, o UploadStorage, ids CaptureIDGenerator, c CaptureClock) (*ReportedOutcomeService, error) {
	if a == nil || s == nil || o == nil || ids == nil || c == nil {
		return nil, errors.New("reported_outcome_service_invalid")
	}
	return &ReportedOutcomeService{a, s, o, ids, c}, nil
}
func (s *ReportedOutcomeService) InitiateProof(ctx context.Context, in InitiateOutcomeProofInput) (InitiateUploadResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.CorrelationID.IsZero() || in.SizeBytes < 1 || in.SizeBytes > MaxTenderDocumentBytes || in.IdempotencyKey == "" {
		return InitiateUploadResult{}, safe("outcome_proof_invalid", nil)
	}
	sum, ok := parseSHA256Hex(in.SHA256Hex)
	if !ok {
		return InitiateUploadResult{}, safe("outcome_proof_invalid", nil)
	}
	a, e := s.auth.AuthorizeReportedOutcome(ctx, in.Subject)
	if e != nil {
		return InitiateUploadResult{}, safe("outcome_not_authorized", e)
	}
	ids, e := responseDraftIDs(s.ids, 4)
	if e != nil {
		return InitiateUploadResult{}, safe("outcome_unavailable", e)
	}
	now := s.clock.Now().UTC()
	key := fmt.Sprintf("tenant/%x/outcome-proof/%x", a.TenantID, ids[1])
	kh := sha256.Sum256([]byte("outcome-proof-init:v1:" + in.IdempotencyKey))
	rh := sha256.Sum256([]byte(fmt.Sprintf("%x:%d:%s", in.BidID, in.SizeBytes, hex.EncodeToString(sum[:]))))
	u, e := s.store.ReserveOutcomeProof(ctx, a, OutcomeProofReservation{CommandID: ids[0], UploadID: ids[1], DocumentID: ids[2], BidID: in.BidID, AuditID: ids[3], CorrelationID: in.CorrelationID, ObjectKey: key, SizeBytes: in.SizeBytes, SHA256: sum, KeyHash: kh, RequestHash: rh, CreatedAt: now, ExpiresAt: now.Add(UploadInstructionTTL)})
	if errors.Is(e, ErrReportedOutcomeConflict) {
		return InitiateUploadResult{}, safe("outcome_conflict", e)
	}
	if e != nil {
		return InitiateUploadResult{}, safe("outcome_unavailable", e)
	}
	ttl := u.ExpiresAt.Sub(s.clock.Now().UTC())
	if ttl <= 0 {
		return InitiateUploadResult{}, safe("outcome_proof_expired", nil)
	}
	if ttl > UploadInstructionTTL {
		ttl = UploadInstructionTTL
	}
	instruction, e := s.storage.PresignPut(ctx, u.ObjectKey, "application/pdf", u.SizeBytes, u.SHA256, ttl)
	if e != nil {
		return InitiateUploadResult{}, safe("outcome_unavailable", e)
	}
	instruction.ExpiresAt = u.ExpiresAt
	return InitiateUploadResult{Session: UploadSession{ID: u.ID, DocumentID: u.DocumentID, ObjectKey: u.ObjectKey, MediaType: "application/pdf", SizeBytes: u.SizeBytes, SHA256: u.SHA256, State: u.State, ExpiresAt: u.ExpiresAt, CompletedVersionID: u.CompletedVersionID, Replay: u.Replay}, Instruction: instruction}, nil
}
func (s *ReportedOutcomeService) CompleteProof(ctx context.Context, in CompleteOutcomeProofInput) (OutcomeProofResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.UploadID.IsZero() || in.CorrelationID.IsZero() || in.IdempotencyKey == "" {
		return OutcomeProofResult{}, safe("outcome_proof_invalid", nil)
	}
	a, e := s.auth.AuthorizeReportedOutcome(ctx, in.Subject)
	if e != nil {
		return OutcomeProofResult{}, safe("outcome_not_authorized", e)
	}
	u, e := s.store.GetOutcomeProofUpload(ctx, a, in.BidID, in.UploadID)
	if e != nil {
		return OutcomeProofResult{}, safe("outcome_unavailable", e)
	}
	if u.State == "COMPLETED" {
		v, e := s.store.GetCompletedOutcomeProof(ctx, a, in.BidID, in.UploadID)
		if e == nil {
			v.Replay = true
		}
		return v, e
	}
	now := s.clock.Now().UTC()
	if now.After(u.ExpiresAt) {
		return OutcomeProofResult{}, safe("outcome_proof_expired", nil)
	}
	obj, e := s.storage.Inspect(ctx, u.ObjectKey)
	if e != nil {
		return OutcomeProofResult{}, safe("outcome_unavailable", e)
	}
	if obj.MediaType != "application/pdf" || obj.SizeBytes != u.SizeBytes || obj.SHA256 != u.SHA256 || obj.ObjectVersion == "" {
		return OutcomeProofResult{}, safe("outcome_proof_mismatch", nil)
	}
	ids, e := responseDraftIDs(s.ids, 5)
	if e != nil {
		return OutcomeProofResult{}, safe("outcome_unavailable", e)
	}
	kh := sha256.Sum256([]byte("outcome-proof-complete:v1:" + in.IdempotencyKey))
	rh := sha256.Sum256([]byte(fmt.Sprintf("%x:%s:%d:%x", in.UploadID, obj.ObjectVersion, obj.SizeBytes, obj.SHA256)))
	v, e := s.store.CompleteOutcomeProof(ctx, a, OutcomeProofCompletion{CommandID: ids[0], UploadID: in.UploadID, DocumentID: u.DocumentID, BidID: in.BidID, VersionID: ids[1], JobID: ids[2], AuditID: ids[3], OutboxID: ids[4], CorrelationID: in.CorrelationID, ObjectKey: u.ObjectKey, ObjectVersion: obj.ObjectVersion, SizeBytes: obj.SizeBytes, SHA256: obj.SHA256, KeyHash: kh, RequestHash: rh, OccurredAt: now})
	if errors.Is(e, ErrReportedOutcomeConflict) {
		return OutcomeProofResult{}, safe("outcome_conflict", e)
	}
	if e != nil {
		return OutcomeProofResult{}, safe("outcome_unavailable", e)
	}
	return v, nil
}
func (s *ReportedOutcomeService) Report(ctx context.Context, in ReportOutcomeInput) (ReportOutcomeResult, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	in.Lessons = strings.TrimSpace(in.Lessons)
	in.CorrectionReason = strings.TrimSpace(in.CorrectionReason)
	reported, e := time.Parse(time.RFC3339, in.ReportedTimeText)
	now := s.clock.Now().UTC()
	terminal := in.Outcome != "PENDING" && in.Outcome != "UNKNOWN"
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedWorkspaceVersion < 1 || in.ExpectedSequence < 0 || in.ExpectedSequence > 99 || !validOutcome(in.Outcome) || !validOutcomeSource(in.SourceClass) || !validRequirementText(in.Reason, 5000) || len(in.Lessons) > 5000 || in.ReportedTimeText == "" || e != nil || reported.After(now) || in.IdempotencyKey == "" || (terminal && in.ProofVersionID.IsZero()) || (in.ExpectedSequence == 0 && (!in.SupersedesID.IsZero() || in.CorrectionReason != "")) || (in.ExpectedSequence > 0 && (in.SupersedesID.IsZero() || !validRequirementText(in.CorrectionReason, 5000))) {
		return ReportOutcomeResult{}, safe("outcome_invalid", nil)
	}
	a, e := s.auth.AuthorizeReportedOutcome(ctx, in.Subject)
	if e != nil {
		return ReportOutcomeResult{}, safe("outcome_not_authorized", e)
	}
	payload, _ := json.Marshal(struct {
		Bid, Proof, Super                                  ID
		Outcome, Source, Reason, Lessons, Text, Correction string
		Seq                                                int32
		Workspace                                          int64
	}{in.BidID, in.ProofVersionID, in.SupersedesID, in.Outcome, in.SourceClass, in.Reason, in.Lessons, in.ReportedTimeText, in.CorrectionReason, in.ExpectedSequence, in.ExpectedWorkspaceVersion})
	kh := sha256.Sum256([]byte("reported-outcome:v1:" + in.IdempotencyKey))
	rh := sha256.Sum256(payload)
	if v, found, e := s.store.FindReportedOutcomeReplay(ctx, a, in.BidID, kh, rh); errors.Is(e, ErrReportedOutcomeConflict) {
		return ReportOutcomeResult{}, safe("outcome_conflict", e)
	} else if e != nil {
		return ReportOutcomeResult{}, safe("outcome_unavailable", e)
	} else if found {
		return v, nil
	}
	ids, e := responseDraftIDs(s.ids, 4)
	if e != nil {
		return ReportOutcomeResult{}, safe("outcome_unavailable", e)
	}
	v, e := s.store.CommitReportedOutcome(ctx, a, ReportOutcomeCommit{CommandID: ids[0], OutcomeID: ids[1], AuditID: ids[2], OutboxID: ids[3], BidID: in.BidID, ProofVersionID: in.ProofVersionID, SupersedesID: in.SupersedesID, CorrelationID: in.CorrelationID, ExpectedSequence: in.ExpectedSequence, ExpectedWorkspaceVersion: in.ExpectedWorkspaceVersion, Outcome: in.Outcome, SourceClass: in.SourceClass, Reason: in.Reason, Lessons: in.Lessons, ReportedText: in.ReportedTimeText, CorrectionReason: in.CorrectionReason, ReportedAt: reported.UTC(), OccurredAt: now, KeyHash: kh, RequestHash: rh})
	if errors.Is(e, ErrReportedOutcomeConflict) {
		return ReportOutcomeResult{}, safe("outcome_conflict", e)
	}
	if e != nil {
		return ReportOutcomeResult{}, safe("outcome_unavailable", e)
	}
	return v, nil
}
func validOutcome(v string) bool {
	switch v {
	case "PENDING", "UNKNOWN", "WON", "LOST", "DISQUALIFIED", "CANCELLED", "WITHDRAWN":
		return true
	}
	return false
}
func validOutcomeSource(v string) bool {
	switch v {
	case "BUYER_NOTICE", "EXTERNAL_PORTAL", "EMAIL", "INTERNAL_OBSERVATION", "OTHER":
		return true
	}
	return false
}
func (s *ReportedOutcomeService) Current(ctx context.Context, sub WorkflowSubject, bid ID) (ReportedOutcome, error) {
	if ctx == nil || !validWorkflowSubject(sub) || bid.IsZero() {
		return ReportedOutcome{}, safe("outcome_read_invalid", nil)
	}
	a, e := s.auth.AuthorizeBidRead(ctx, sub)
	if e != nil {
		return ReportedOutcome{}, safe("outcome_not_authorized", e)
	}
	v, e := s.store.GetCurrentReportedOutcome(ctx, a, bid)
	if errors.Is(e, ErrReportedOutcomeNotFound) {
		return v, safe("outcome_not_found", e)
	}
	if e != nil {
		return v, safe("outcome_unavailable", e)
	}
	return v, nil
}
func (s *ReportedOutcomeService) History(ctx context.Context, sub WorkflowSubject, bid ID) ([]ReportedOutcome, error) {
	if ctx == nil || !validWorkflowSubject(sub) || bid.IsZero() {
		return nil, safe("outcome_read_invalid", nil)
	}
	a, e := s.auth.AuthorizeBidRead(ctx, sub)
	if e != nil {
		return nil, safe("outcome_not_authorized", e)
	}
	v, e := s.store.ListReportedOutcomes(ctx, a, bid)
	if errors.Is(e, ErrReportedOutcomeReadLimit) {
		return nil, safe("outcome_read_limit", e)
	}
	if e != nil {
		return nil, safe("outcome_unavailable", e)
	}
	return v, nil
}
