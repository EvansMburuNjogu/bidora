package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
)

var ErrEvidenceExpired = errors.New("evidence_expired")
var ErrRequirementEvidenceConflict = errors.New("requirement_evidence_conflict")

type EvidenceUploadSession struct {
	ID, DocumentID, BidID, CompletedVersionID ID
	ObjectKey, MediaType                      string
	SizeBytes                                 int64
	SHA256                                    [32]byte
	State                                     string
	ExpiresAt                                 time.Time
	EvidenceExpiresAt                         *time.Time
	Replay                                    bool
}
type InitiateEvidenceUploadInput struct {
	Subject                                     WorkflowSubject
	BidID, CorrelationID                        ID
	Title, MediaType, SHA256Hex, IdempotencyKey string
	SizeBytes                                   int64
	EvidenceExpiresAt                           *string
}
type CompleteEvidenceUploadInput struct {
	Subject                        WorkflowSubject
	BidID, UploadID, CorrelationID ID
	IdempotencyKey                 string
}
type EvidenceUploadReservation struct {
	CommandID, UploadID, DocumentID, BidID, AuditID, CorrelationID ID
	ObjectKey, Title, MediaType                                    string
	SizeBytes                                                      int64
	SHA256, KeyHash, RequestHash                                   [32]byte
	CreatedAt, ExpiresAt                                           time.Time
	EvidenceExpiresAt                                              *time.Time
}
type EvidenceUploadCompletion struct {
	CommandID, UploadID, DocumentID, BidID, VersionID, ScanJobID, AuditID, OutboxID, CorrelationID ID
	ObjectKey, ObjectVersion, MediaType                                                            string
	SizeBytes                                                                                      int64
	SHA256, KeyHash, RequestHash                                                                   [32]byte
	OccurredAt                                                                                     time.Time
	EvidenceExpiresAt                                                                              *time.Time
}
type EvidenceUploadResult struct {
	UploadID, DocumentID, DocumentVersionID, ScanJobID ID
	ScanState                                          string
	EvidenceExpiresAt                                  *time.Time
	Replay                                             bool
}
type RequirementEvidence struct {
	ID, RequirementID, BidID, DocumentVersionID, AddedBy ID
	EvidenceLocation, Justification                      string
	AddedAt                                              time.Time
	ExpiresAt                                            *time.Time
	Expired                                              bool
}
type SatisfyRequirementInput struct {
	Subject                                                WorkflowSubject
	BidID, RequirementID, DocumentVersionID, CorrelationID ID
	EvidenceLocation, Justification, IdempotencyKey        string
	ExpectedRequirementVersion, ExpectedWorkspaceVersion   int64
}
type SatisfyRequirementCommit struct {
	CommandID, EvidenceID, BidID, RequirementID, DocumentVersionID, AuditID, OutboxID, CorrelationID ID
	EvidenceLocation, Justification                                                                  string
	ExpectedRequirementVersion, ExpectedWorkspaceVersion                                             int64
	KeyHash, RequestHash                                                                             [32]byte
	OccurredAt                                                                                       time.Time
}
type SatisfyRequirementResult struct {
	Evidence         RequirementEvidence
	Requirement      Requirement
	WorkspaceVersion int64
	Replay           bool
}
type RequirementEvidenceStore interface {
	ReserveEvidenceUpload(context.Context, WorkflowAuthority, EvidenceUploadReservation) (EvidenceUploadSession, error)
	GetEvidenceUpload(context.Context, WorkflowAuthority, ID, ID) (EvidenceUploadSession, error)
	GetCompletedEvidenceUpload(context.Context, WorkflowAuthority, ID, ID) (EvidenceUploadResult, error)
	CompleteEvidenceUpload(context.Context, WorkflowAuthority, EvidenceUploadCompletion) (EvidenceUploadResult, error)
	SatisfyRequirement(context.Context, WorkflowAuthority, SatisfyRequirementCommit) (SatisfyRequirementResult, error)
	ListRequirementEvidence(context.Context, WorkflowAuthority, ID, ID) ([]RequirementEvidence, error)
}
type RequirementEvidenceService struct {
	auth    RequirementAuthorizer
	store   RequirementEvidenceStore
	storage UploadStorage
	ids     CaptureIDGenerator
	clock   CaptureClock
}

func NewRequirementEvidenceService(auth RequirementAuthorizer, store RequirementEvidenceStore, storage UploadStorage, ids CaptureIDGenerator, clock CaptureClock) (*RequirementEvidenceService, error) {
	if auth == nil || store == nil || storage == nil || ids == nil || clock == nil {
		return nil, errors.New("requirement_evidence_service_invalid")
	}
	return &RequirementEvidenceService{auth, store, storage, ids, clock}, nil
}
func (s *RequirementEvidenceService) InitiateUpload(ctx context.Context, in InitiateEvidenceUploadInput) (InitiateUploadResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.CorrelationID.IsZero() || in.IdempotencyKey == "" || !validUploadTitle(in.Title) || !approvedMediaType(in.MediaType) || in.SizeBytes < 1 || in.SizeBytes > MaxTenderDocumentBytes {
		return InitiateUploadResult{}, safe("evidence_upload_invalid", nil)
	}
	sum, ok := parseSHA256Hex(in.SHA256Hex)
	if !ok {
		return InitiateUploadResult{}, safe("evidence_upload_invalid", nil)
	}
	authority, err := s.auth.AuthorizeRequirementCreate(ctx, in.Subject)
	if err != nil {
		return InitiateUploadResult{}, safe("evidence_not_authorized", err)
	}
	ids, err := s.newIDs(4)
	if err != nil {
		return InitiateUploadResult{}, safe("evidence_upload_unavailable", err)
	}
	now := s.clock.Now().UTC()
	var evidenceExpiresAt *time.Time
	expiryFingerprint := ""
	if in.EvidenceExpiresAt != nil {
		parsed, parseErr := time.Parse(time.RFC3339, *in.EvidenceExpiresAt)
		if parseErr != nil || !parsed.After(now) {
			return InitiateUploadResult{}, safe("evidence_upload_invalid", nil)
		}
		normalized := parsed.UTC()
		evidenceExpiresAt = &normalized
		expiryFingerprint = normalized.Format(time.RFC3339Nano)
	}
	key := fmt.Sprintf("tenant/%x/evidence/%x", authority.TenantID, ids[1])
	keyHash := sha256.Sum256([]byte("initiate-evidence-upload:v2:" + in.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("initiate-evidence-upload:v2:%x:%s:%s:%d:%x:%s", in.BidID, in.Title, in.MediaType, in.SizeBytes, sum, expiryFingerprint)))
	session, err := s.store.ReserveEvidenceUpload(ctx, authority, EvidenceUploadReservation{CommandID: ids[0], UploadID: ids[1], DocumentID: ids[2], BidID: in.BidID, AuditID: ids[3], CorrelationID: in.CorrelationID, ObjectKey: key, Title: in.Title, MediaType: in.MediaType, SizeBytes: in.SizeBytes, SHA256: sum, KeyHash: keyHash, RequestHash: requestHash, CreatedAt: now, ExpiresAt: now.Add(UploadInstructionTTL), EvidenceExpiresAt: evidenceExpiresAt})
	if errors.Is(err, ErrRequirementEvidenceConflict) || errors.Is(err, ErrWorkflowConflict) {
		return InitiateUploadResult{}, safe("evidence_conflict", err)
	}
	if err != nil {
		return InitiateUploadResult{}, safe("evidence_upload_unavailable", err)
	}
	ttl := session.ExpiresAt.Sub(s.clock.Now().UTC())
	if ttl <= 0 {
		return InitiateUploadResult{}, safe("evidence_upload_expired", nil)
	}
	if ttl > UploadInstructionTTL {
		ttl = UploadInstructionTTL
	}
	instruction, err := s.storage.PresignPut(ctx, session.ObjectKey, session.MediaType, session.SizeBytes, session.SHA256, ttl)
	if err != nil {
		return InitiateUploadResult{}, safe("evidence_upload_unavailable", err)
	}
	instruction.ExpiresAt = session.ExpiresAt
	return InitiateUploadResult{Session: UploadSession{ID: session.ID, DocumentID: session.DocumentID, ObjectKey: session.ObjectKey, MediaType: session.MediaType, SizeBytes: session.SizeBytes, SHA256: session.SHA256, State: session.State, ExpiresAt: session.ExpiresAt, EvidenceExpiresAt: session.EvidenceExpiresAt, CompletedVersionID: session.CompletedVersionID, Replay: session.Replay}, Instruction: instruction}, nil
}
func (s *RequirementEvidenceService) CompleteUpload(ctx context.Context, in CompleteEvidenceUploadInput) (EvidenceUploadResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.UploadID.IsZero() || in.CorrelationID.IsZero() || in.IdempotencyKey == "" {
		return EvidenceUploadResult{}, safe("evidence_upload_complete_invalid", nil)
	}
	authority, err := s.auth.AuthorizeRequirementCreate(ctx, in.Subject)
	if err != nil {
		return EvidenceUploadResult{}, safe("evidence_not_authorized", err)
	}
	session, err := s.store.GetEvidenceUpload(ctx, authority, in.BidID, in.UploadID)
	if errors.Is(err, ErrRequirementEvidenceConflict) {
		return EvidenceUploadResult{}, safe("evidence_conflict", err)
	}
	if err != nil {
		return EvidenceUploadResult{}, safe("evidence_upload_unavailable", err)
	}
	if session.State == "COMPLETED" {
		result, lookupErr := s.store.GetCompletedEvidenceUpload(ctx, authority, in.BidID, in.UploadID)
		if errors.Is(lookupErr, ErrRequirementEvidenceConflict) {
			return EvidenceUploadResult{}, safe("evidence_conflict", lookupErr)
		}
		if lookupErr != nil {
			return EvidenceUploadResult{}, safe("evidence_upload_unavailable", lookupErr)
		}
		result.Replay = true
		return result, nil
	}
	observedAt := s.clock.Now().UTC()
	if observedAt.After(session.ExpiresAt) {
		return EvidenceUploadResult{}, safe("evidence_upload_expired", nil)
	}
	if session.EvidenceExpiresAt != nil && !session.EvidenceExpiresAt.After(observedAt) {
		return EvidenceUploadResult{}, safe("evidence_document_expired", nil)
	}
	object, err := s.storage.Inspect(ctx, session.ObjectKey)
	if err != nil {
		return EvidenceUploadResult{}, safe("evidence_upload_unavailable", err)
	}
	if object.ObjectVersion == "" || object.MediaType != session.MediaType || object.SizeBytes != session.SizeBytes || object.SHA256 != session.SHA256 {
		return EvidenceUploadResult{}, safe("evidence_upload_mismatch", nil)
	}
	ids, err := s.newIDs(5)
	if err != nil {
		return EvidenceUploadResult{}, safe("evidence_upload_unavailable", err)
	}
	now := s.clock.Now().UTC()
	keyHash := sha256.Sum256([]byte("complete-evidence-upload:v1:" + in.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("complete-evidence-upload:v1:%x:%x:%s:%d:%x", in.UploadID, session.DocumentID, object.ObjectVersion, object.SizeBytes, object.SHA256)))
	result, err := s.store.CompleteEvidenceUpload(ctx, authority, EvidenceUploadCompletion{CommandID: ids[0], UploadID: in.UploadID, DocumentID: session.DocumentID, BidID: in.BidID, VersionID: ids[1], ScanJobID: ids[2], AuditID: ids[3], OutboxID: ids[4], CorrelationID: in.CorrelationID, ObjectKey: session.ObjectKey, ObjectVersion: object.ObjectVersion, MediaType: object.MediaType, SizeBytes: object.SizeBytes, SHA256: object.SHA256, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: now, EvidenceExpiresAt: session.EvidenceExpiresAt})
	if errors.Is(err, ErrRequirementEvidenceConflict) || errors.Is(err, ErrWorkflowConflict) {
		return EvidenceUploadResult{}, safe("evidence_conflict", err)
	}
	if err != nil {
		return EvidenceUploadResult{}, safe("evidence_upload_unavailable", err)
	}
	return result, nil
}
func (s *RequirementEvidenceService) Satisfy(ctx context.Context, in SatisfyRequirementInput) (SatisfyRequirementResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.RequirementID.IsZero() || in.DocumentVersionID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedRequirementVersion < 1 || in.ExpectedWorkspaceVersion < 1 || in.IdempotencyKey == "" || !validRequirementText(in.EvidenceLocation, 500) || !validRequirementText(in.Justification, 2000) {
		return SatisfyRequirementResult{}, safe("requirement_satisfaction_invalid", nil)
	}
	authority, err := s.auth.AuthorizeRequirementCreate(ctx, in.Subject)
	if err != nil {
		return SatisfyRequirementResult{}, safe("evidence_not_authorized", err)
	}
	ids, err := s.newIDs(4)
	if err != nil {
		return SatisfyRequirementResult{}, safe("requirement_satisfaction_unavailable", err)
	}
	keyHash := sha256.Sum256([]byte("satisfy-requirement:v1:" + in.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("satisfy-requirement:v1:%x:%x:%x:%d:%d:%s:%s", in.BidID, in.RequirementID, in.DocumentVersionID, in.ExpectedRequirementVersion, in.ExpectedWorkspaceVersion, in.EvidenceLocation, in.Justification)))
	result, err := s.store.SatisfyRequirement(ctx, authority, SatisfyRequirementCommit{CommandID: ids[0], EvidenceID: ids[1], BidID: in.BidID, RequirementID: in.RequirementID, DocumentVersionID: in.DocumentVersionID, AuditID: ids[2], OutboxID: ids[3], CorrelationID: in.CorrelationID, EvidenceLocation: in.EvidenceLocation, Justification: in.Justification, ExpectedRequirementVersion: in.ExpectedRequirementVersion, ExpectedWorkspaceVersion: in.ExpectedWorkspaceVersion, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: s.clock.Now().UTC()})
	if errors.Is(err, ErrEvidenceExpired) {
		return SatisfyRequirementResult{}, safe("evidence_document_expired", err)
	}
	if errors.Is(err, ErrRequirementEvidenceConflict) || errors.Is(err, ErrWorkflowConflict) {
		return SatisfyRequirementResult{}, safe("evidence_conflict", err)
	}
	if err != nil {
		return SatisfyRequirementResult{}, safe("requirement_satisfaction_unavailable", err)
	}
	return result, nil
}
func (s *RequirementEvidenceService) List(ctx context.Context, subject WorkflowSubject, bidID, requirementID ID) ([]RequirementEvidence, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() || requirementID.IsZero() {
		return nil, safe("requirement_evidence_read_invalid", nil)
	}
	authority, err := s.auth.AuthorizeReadiness(ctx, subject)
	if err != nil {
		return nil, safe("evidence_not_authorized", err)
	}
	rows, err := s.store.ListRequirementEvidence(ctx, authority, bidID, requirementID)
	if err != nil {
		return nil, safe("requirement_evidence_read_unavailable", err)
	}
	now := s.clock.Now().UTC()
	for i := range rows {
		rows[i].Expired = rows[i].ExpiresAt != nil && !rows[i].ExpiresAt.After(now)
	}
	return rows, nil
}
func (s *RequirementEvidenceService) newIDs(n int) ([]ID, error) {
	out := make([]ID, n)
	for i := range out {
		var err error
		out[i], err = s.ids.NewID()
		if err != nil || out[i].IsZero() {
			return nil, errors.New("evidence_id_unavailable")
		}
	}
	return out, nil
}
