package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxTenderDocumentBytes int64 = 25 * 1024 * 1024
const UploadInstructionTTL = 10 * time.Minute
const PDFMediaType = "application/pdf"
const DOCXMediaType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

var ErrDocumentJobNotFound = errors.New("document_job_not_found")

type UploadAuthorization interface {
	AuthorizeTenderDocumentUpload(context.Context, WorkflowSubject) (WorkflowAuthority, error)
	AuthorizeDocumentJobRead(context.Context, WorkflowSubject) (WorkflowAuthority, error)
}
type UploadStorage interface {
	PresignPut(context.Context, string, string, int64, [32]byte, time.Duration) (UploadInstruction, error)
	Inspect(context.Context, string) (StoredObject, error)
}
type UploadStore interface {
	ReserveUpload(context.Context, WorkflowAuthority, UploadReservation) (UploadSession, error)
	GetUpload(context.Context, WorkflowAuthority, ID, ID) (UploadSession, error)
	GetCompletedUpload(context.Context, WorkflowAuthority, ID, ID) (UploadResult, error)
	CompleteUpload(context.Context, WorkflowAuthority, UploadCompletion) (UploadResult, error)
	GetDocumentJob(context.Context, WorkflowAuthority, ID) (DocumentJob, error)
}
type UploadInstruction struct {
	Method    string
	URL       string
	Fields    map[string]string
	ExpiresAt time.Time
}
type StoredObject struct {
	ObjectVersion string
	MediaType     string
	SizeBytes     int64
	SHA256        [32]byte
}
type UploadSession struct {
	ID, DocumentID, OpportunityID ID
	ObjectKey, MediaType, Title   string
	SizeBytes                     int64
	SHA256                        [32]byte
	State                         string
	ExpiresAt                     time.Time
	EvidenceExpiresAt             *time.Time
	CompletedVersionID            ID
	Replay                        bool
}
type UploadReservation struct {
	CommandID, UploadID, DocumentID, OpportunityID, ActorID, AuditID, CorrelationID ID
	ObjectKey, Title, MediaType                                                     string
	SizeBytes                                                                       int64
	SHA256, KeyHash, RequestHash                                                    [32]byte
	CreatedAt, ExpiresAt                                                            time.Time
}
type UploadCompletion struct {
	CommandID, UploadID, DocumentID, OpportunityID, VersionID, ScanJobID, AnalysisJobID, AuditID, OutboxID, CorrelationID ID
	ObjectKey, ObjectVersion, MediaType                                                                                   string
	SizeBytes                                                                                                             int64
	SHA256, KeyHash, RequestHash                                                                                          [32]byte
	OccurredAt                                                                                                            time.Time
}
type DocumentJob struct {
	ID, DocumentVersionID, DocumentID, OpportunityID, BidID ID
	HasOpportunity, HasBid                                  bool
	Kind, State, BlockedReason, DocumentKind                string
	Attempts                                                int32
	CreatedAt                                               time.Time
	StartedAt, CompletedAt                                  *time.Time
}
type UploadResult struct {
	UploadID, DocumentID, DocumentVersionID, ScanJobID, AnalysisJobID ID
	ScanState, AnalysisState                                          string
	Replay                                                            bool
}
type InitiateUploadInput struct {
	Subject                                     WorkflowSubject
	OpportunityID, CorrelationID                ID
	Title, MediaType, SHA256Hex, IdempotencyKey string
	SizeBytes                                   int64
}
type InitiateUploadResult struct {
	Session     UploadSession
	Instruction UploadInstruction
}
type CompleteUploadInput struct {
	Subject                                WorkflowSubject
	OpportunityID, UploadID, CorrelationID ID
	IdempotencyKey                         string
}
type DocumentUploadService struct {
	uploadPolicy func(context.Context) (int64, error)
	auth         UploadAuthorization
	store        UploadStore
	storage      UploadStorage
	ids          CaptureIDGenerator
	clock        CaptureClock
}

func NewDocumentUploadService(auth UploadAuthorization, store UploadStore, storage UploadStorage, ids CaptureIDGenerator, clock CaptureClock) (*DocumentUploadService, error) {
	if auth == nil || store == nil || storage == nil || ids == nil || clock == nil {
		return nil, errors.New("document_upload_service_invalid")
	}
	return &DocumentUploadService{auth: auth, store: store, storage: storage, ids: ids, clock: clock}, nil
}
func (s *DocumentUploadService) EnableOperationalPolicy(reader func(context.Context) (int64, error)) error {
	if s == nil || reader == nil {
		return errors.New("document_upload_policy_invalid")
	}
	s.uploadPolicy = reader
	return nil
}

func (s *DocumentUploadService) Initiate(ctx context.Context, in InitiateUploadInput) (InitiateUploadResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.OpportunityID.IsZero() || in.CorrelationID.IsZero() || in.IdempotencyKey == "" || !validUploadTitle(in.Title) || !approvedMediaType(in.MediaType) || in.SizeBytes < 1 || in.SizeBytes > MaxTenderDocumentBytes {
		return InitiateUploadResult{}, safe("document_upload_invalid", nil)
	}
	sum, ok := parseSHA256Hex(in.SHA256Hex)
	if !ok {
		return InitiateUploadResult{}, safe("document_upload_invalid", nil)
	}
	authority, err := s.auth.AuthorizeTenderDocumentUpload(ctx, in.Subject)
	if err != nil {
		return InitiateUploadResult{}, safe("document_upload_not_authorized", err)
	}
	if s.uploadPolicy != nil {
		limit, e := s.uploadPolicy(ctx)
		if e != nil || limit < 1 || limit > MaxTenderDocumentBytes {
			return InitiateUploadResult{}, safe("document_upload_unavailable", e)
		}
		if in.SizeBytes > limit {
			return InitiateUploadResult{}, safe("document_upload_invalid", nil)
		}
	}
	ids, err := s.newIDs(4)
	if err != nil {
		return InitiateUploadResult{}, safe("document_upload_unavailable", err)
	}
	now := s.clock.Now().UTC()
	objectKey := fmt.Sprintf("tenant/%x/quarantine/%x", authority.TenantID, ids[1])
	keyHash := sha256.Sum256([]byte("initiate-document-upload:v1:" + in.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("initiate-document-upload:v1:%x:%s:%s:%d:%x", in.OpportunityID, in.Title, in.MediaType, in.SizeBytes, sum)))
	session, err := s.store.ReserveUpload(ctx, authority, UploadReservation{CommandID: ids[0], UploadID: ids[1], DocumentID: ids[2], OpportunityID: in.OpportunityID, ActorID: in.Subject.IdentityID, AuditID: ids[3], CorrelationID: in.CorrelationID, ObjectKey: objectKey, Title: in.Title, MediaType: in.MediaType, SizeBytes: in.SizeBytes, SHA256: sum, KeyHash: keyHash, RequestHash: requestHash, CreatedAt: now, ExpiresAt: now.Add(UploadInstructionTTL)})
	if err != nil {
		return InitiateUploadResult{}, safe("document_upload_unavailable", err)
	}
	ttl := session.ExpiresAt.Sub(s.clock.Now().UTC())
	if ttl <= 0 {
		return InitiateUploadResult{}, safe("document_upload_expired", nil)
	}
	if ttl > UploadInstructionTTL {
		ttl = UploadInstructionTTL
	}
	instruction, err := s.storage.PresignPut(ctx, session.ObjectKey, session.MediaType, session.SizeBytes, session.SHA256, ttl)
	if err != nil {
		return InitiateUploadResult{}, safe("document_upload_unavailable", err)
	}
	instruction.ExpiresAt = session.ExpiresAt
	return InitiateUploadResult{Session: session, Instruction: instruction}, nil
}
func (s *DocumentUploadService) Complete(ctx context.Context, in CompleteUploadInput) (UploadResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.OpportunityID.IsZero() || in.UploadID.IsZero() || in.CorrelationID.IsZero() || in.IdempotencyKey == "" {
		return UploadResult{}, safe("document_upload_complete_invalid", nil)
	}
	authority, err := s.auth.AuthorizeTenderDocumentUpload(ctx, in.Subject)
	if err != nil {
		return UploadResult{}, safe("document_upload_complete_not_authorized", err)
	}
	session, err := s.store.GetUpload(ctx, authority, in.OpportunityID, in.UploadID)
	if err != nil {
		return UploadResult{}, safe("document_upload_complete_unavailable", err)
	}
	if session.State == "COMPLETED" {
		result, lookupErr := s.store.GetCompletedUpload(ctx, authority, in.OpportunityID, in.UploadID)
		if lookupErr != nil {
			return UploadResult{}, safe("document_upload_complete_unavailable", lookupErr)
		}
		result.Replay = true
		return result, nil
	}
	if s.clock.Now().UTC().After(session.ExpiresAt) {
		return UploadResult{}, safe("document_upload_expired", nil)
	}
	object, err := s.storage.Inspect(ctx, session.ObjectKey)
	if err != nil {
		return UploadResult{}, safe("document_upload_complete_unavailable", err)
	}
	if object.ObjectVersion == "" || object.MediaType != session.MediaType || object.SizeBytes != session.SizeBytes || object.SHA256 != session.SHA256 {
		return UploadResult{}, safe("document_upload_mismatch", nil)
	}
	ids, err := s.newIDs(6)
	if err != nil {
		return UploadResult{}, safe("document_upload_complete_unavailable", err)
	}
	now := s.clock.Now().UTC()
	keyHash := sha256.Sum256([]byte("complete-document-upload:v1:" + in.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("complete-document-upload:v1:%x:%x:%s:%d:%x", in.UploadID, session.DocumentID, object.ObjectVersion, object.SizeBytes, object.SHA256)))
	result, err := s.store.CompleteUpload(ctx, authority, UploadCompletion{CommandID: ids[0], UploadID: in.UploadID, DocumentID: session.DocumentID, OpportunityID: in.OpportunityID, VersionID: ids[1], ScanJobID: ids[2], AnalysisJobID: ids[3], AuditID: ids[4], OutboxID: ids[5], CorrelationID: in.CorrelationID, ObjectKey: session.ObjectKey, ObjectVersion: object.ObjectVersion, MediaType: object.MediaType, SizeBytes: object.SizeBytes, SHA256: object.SHA256, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: now})
	if err != nil {
		return UploadResult{}, safe("document_upload_complete_unavailable", err)
	}
	return result, nil
}
func (s *DocumentUploadService) Job(ctx context.Context, subject WorkflowSubject, jobID ID) (DocumentJob, error) {
	if ctx == nil || !validWorkflowSubject(subject) || jobID.IsZero() {
		return DocumentJob{}, safe("document_job_invalid", nil)
	}
	authority, err := s.auth.AuthorizeDocumentJobRead(ctx, subject)
	if err != nil {
		return DocumentJob{}, safe("document_job_not_authorized", err)
	}
	job, err := s.store.GetDocumentJob(ctx, authority, jobID)
	if errors.Is(err, ErrDocumentJobNotFound) {
		return DocumentJob{}, safe("document_job_not_found", err)
	}
	if err != nil {
		return DocumentJob{}, safe("document_job_unavailable", err)
	}
	return job, nil
}
func (s *DocumentUploadService) newIDs(n int) ([]ID, error) {
	out := make([]ID, n)
	for i := range out {
		var err error
		out[i], err = s.ids.NewID()
		if err != nil || out[i].IsZero() {
			return nil, errors.New("document_upload_id_unavailable")
		}
	}
	return out, nil
}
func validUploadTitle(v string) bool {
	return v != "" && len(v) <= 255 && utf8.ValidString(v) && strings.TrimSpace(v) == v
}
func approvedMediaType(v string) bool { return v == PDFMediaType || v == DOCXMediaType }
func parseSHA256Hex(v string) ([32]byte, bool) {
	var out [32]byte
	if len(v) != 64 {
		return out, false
	}
	for i := 0; i < 32; i++ {
		var b byte
		for _, c := range []byte{v[i*2], v[i*2+1]} {
			b <<= 4
			switch {
			case c >= '0' && c <= '9':
				b += c - '0'
			case c >= 'a' && c <= 'f':
				b += c - 'a' + 10
			default:
				return out, false
			}
		}
		out[i] = b
	}
	return out, true
}
func SHA256Base64(sum [32]byte) string { return base64.StdEncoding.EncodeToString(sum[:]) }
