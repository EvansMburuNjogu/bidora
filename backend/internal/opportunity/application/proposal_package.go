package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

var ErrProposalPackageConflict = errors.New("proposal_package_conflict")
var ErrProposalPackageNotFound = errors.New("proposal_package_not_found")

type PrepareProposalPackageInput struct {
	Subject                                           WorkflowSubject
	BidID, ProposalVersionID, CorrelationID           ID
	ExpectedProposalVersion, ExpectedWorkspaceVersion int64
	IdempotencyKey                                    string
}
type ProposalPackageManifest struct {
	ID, BidID, ProposalID, ProposalVersionID, CreatedBy ID
	LetterheadVersionID                                 *ID
	LetterheadVersionNumber                             *int64
	LetterheadTemplateKey                               *string
	LetterheadContentHash                               *[32]byte
	ProposalRevision                                    int64
	Status, PDFRenderProfile, DOCXRenderProfile         string
	PDFSHA256, DOCXSHA256, ManifestSHA256               [32]byte
	PDFSize, DOCXSize                                   int64
	ReadinessObservedAt                                 time.Time
	ReadinessReady                                      bool
	ReadinessBlockers                                   []string
	ManifestVersion                                     int32
	CreatedAt                                           time.Time
}
type ProposalPackageCommit struct {
	CommandID, ManifestID, AuditID, OutboxID, BidID, ProposalID, ProposalVersionID, LetterheadVersionID, CorrelationID ID
	ProposalRevision, ExpectedProposalVersion, ExpectedWorkspaceVersion                                                int64
	LetterheadVersionNumber                                                                                            int64
	LetterheadTemplateKey                                                                                              string
	PDFSHA256, DOCXSHA256, ManifestSHA256, LetterheadContentHash                                                       [32]byte
	PDFSize, DOCXSize                                                                                                  int64
	ReadinessObservedAt                                                                                                time.Time
	ReadinessReady                                                                                                     bool
	ReadinessBlockers                                                                                                  []string
	KeyHash, RequestHash                                                                                               [32]byte
	OccurredAt                                                                                                         time.Time
}
type ProposalPackageHistoryInput struct {
	Subject WorkflowSubject
	BidID   ID
	Cursor  ReadCursor
	Limit   int32
}
type ProposalPackageHistoryPage struct {
	Items   []ProposalPackageManifest
	HasMore bool
}
type ProposalPackageResult struct {
	Manifest ProposalPackageManifest
	Replay   bool
}
type ProposalPackageStore interface {
	FindProposalPackageReplay(context.Context, WorkflowAuthority, ID, [32]byte, [32]byte) (ProposalPackageResult, bool, error)
	CommitProposalPackage(context.Context, WorkflowAuthority, ProposalPackageCommit) (ProposalPackageResult, error)
	GetProposalPackageManifest(context.Context, WorkflowAuthority, ID, ID) (ProposalPackageManifest, error)
	ListProposalPackageManifests(context.Context, WorkflowAuthority, ID, ReadCursor, int32) ([]ProposalPackageManifest, error)
}
type ProposalPackageSource interface {
	GetProposalVersion(context.Context, WorkflowAuthority, ID, ID) (ProposalDocument, error)
	GetProposalApprovalReadiness(context.Context, WorkflowAuthority, ID, time.Time) (ProposalApprovalReadiness, error)
}
type ProposalPackageService struct {
	auth     TechnicalResponseAuthorizer
	source   ProposalPackageSource
	store    ProposalPackageStore
	renderer ProposalRenderer
	ids      CaptureIDGenerator
	clock    CaptureClock
}

func NewProposalPackageService(auth TechnicalResponseAuthorizer, source ProposalPackageSource, store ProposalPackageStore, renderer ProposalRenderer, ids CaptureIDGenerator, clock CaptureClock) (*ProposalPackageService, error) {
	if auth == nil || source == nil || store == nil || renderer == nil || ids == nil || clock == nil {
		return nil, errors.New("proposal_package_service_invalid")
	}
	return &ProposalPackageService{auth: auth, source: source, store: store, renderer: renderer, ids: ids, clock: clock}, nil
}
func (s *ProposalPackageService) Prepare(ctx context.Context, in PrepareProposalPackageInput) (ProposalPackageResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.ProposalVersionID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedProposalVersion < 1 || in.ExpectedWorkspaceVersion < 1 || in.IdempotencyKey == "" {
		return ProposalPackageResult{}, safe("proposal_package_invalid", nil)
	}
	a, err := s.auth.AuthorizeResponseEdit(ctx, in.Subject)
	if err != nil {
		return ProposalPackageResult{}, safe("proposal_package_not_authorized", err)
	}
	keyHash := sha256.Sum256([]byte("proposal-package:v1:" + in.IdempotencyKey))
	request, _ := json.Marshal(struct {
		BidID, VersionID                    ID
		ExpectedProposal, ExpectedWorkspace int64
	}{in.BidID, in.ProposalVersionID, in.ExpectedProposalVersion, in.ExpectedWorkspaceVersion})
	requestHash := sha256.Sum256(request)
	if replay, found, err := s.store.FindProposalPackageReplay(ctx, a, in.BidID, keyHash, requestHash); errors.Is(err, ErrProposalPackageConflict) {
		return ProposalPackageResult{}, safe("proposal_package_conflict", err)
	} else if err != nil {
		return ProposalPackageResult{}, safe("proposal_package_unavailable", err)
	} else if found {
		return replay, nil
	}
	proposal, err := s.source.GetProposalVersion(ctx, a, in.BidID, in.ProposalVersionID)
	if err != nil {
		return ProposalPackageResult{}, safe("proposal_package_unavailable", err)
	}
	if proposal.ProposalVersion != in.ExpectedProposalVersion || proposal.WorkspaceVersion != in.ExpectedWorkspaceVersion || proposal.LetterheadVersionID == nil || proposal.LetterheadVersionID.IsZero() || proposal.LetterheadVersionNumber == nil || *proposal.LetterheadVersionNumber < 1 || proposal.LetterheadTemplateKey == nil || *proposal.LetterheadTemplateKey != "STANDARD_PROPOSAL" || proposal.LetterheadContentHash == nil {
		return ProposalPackageResult{}, safe("proposal_package_conflict", ErrProposalPackageConflict)
	}
	letterheadVersionID, letterheadVersionNumber, letterheadTemplateKey, letterheadHash := *proposal.LetterheadVersionID, *proposal.LetterheadVersionNumber, *proposal.LetterheadTemplateKey, *proposal.LetterheadContentHash
	observedAt := s.clock.Now().UTC()
	readiness, err := s.source.GetProposalApprovalReadiness(ctx, a, in.BidID, observedAt)
	if err != nil {
		return ProposalPackageResult{}, safe("proposal_package_unavailable", err)
	}
	if readiness.ProposalVersionID != in.ProposalVersionID || readiness.ProposalVersion != in.ExpectedProposalVersion || readiness.WorkspaceVersion != in.ExpectedWorkspaceVersion {
		return ProposalPackageResult{}, safe("proposal_package_conflict", ErrProposalPackageConflict)
	}
	pdf, err := s.renderer.Render("pdf", proposal)
	if err != nil {
		return ProposalPackageResult{}, mapPackageRenderError(err)
	}
	docx, err := s.renderer.Render("docx", proposal)
	if err != nil {
		return ProposalPackageResult{}, mapPackageRenderError(err)
	}
	pdfHash, docxHash := sha256.Sum256(pdf), sha256.Sum256(docx)
	blockers := append([]string{}, readiness.Blockers...)
	sort.Strings(blockers)
	canonical, _ := json.Marshal(struct {
		BidID, ProposalID, ProposalVersionID       string
		ProposalRevision                           int64
		PDFRenderProfile, PDFSHA256                string
		PDFSize                                    int
		DOCXRenderProfile, DOCXSHA256              string
		DOCXSize                                   int
		LetterheadVersionID, LetterheadContentHash string
		LetterheadVersionNumber                    int64
		LetterheadTemplateKey                      string
		ReadinessObservedAt                        string
		ReadinessReady                             bool
		ReadinessBlockers                          []string
		Status                                     string
		Version                                    int
	}{fmt.Sprintf("%x", in.BidID), fmt.Sprintf("%x", proposal.ProposalID), fmt.Sprintf("%x", in.ProposalVersionID), proposal.Revision, "BIDORA_PDF_V1", hex.EncodeToString(pdfHash[:]), len(pdf), "BIDORA_DOCX_V1", hex.EncodeToString(docxHash[:]), len(docx), fmt.Sprintf("%x", letterheadVersionID), hex.EncodeToString(letterheadHash[:]), letterheadVersionNumber, letterheadTemplateKey, observedAt.Format(time.RFC3339Nano), readiness.Ready, blockers, "UNAPPROVED_PREPARATION_ONLY", 2})
	manifestHash := sha256.Sum256(canonical)
	ids, err := responseDraftIDs(s.ids, 4)
	if err != nil {
		return ProposalPackageResult{}, safe("proposal_package_unavailable", err)
	}
	commit := ProposalPackageCommit{CommandID: ids[0], ManifestID: ids[1], AuditID: ids[2], OutboxID: ids[3], BidID: in.BidID, ProposalID: proposal.ProposalID, ProposalVersionID: in.ProposalVersionID, LetterheadVersionID: letterheadVersionID, LetterheadVersionNumber: letterheadVersionNumber, LetterheadTemplateKey: letterheadTemplateKey, CorrelationID: in.CorrelationID, ProposalRevision: proposal.Revision, ExpectedProposalVersion: in.ExpectedProposalVersion, ExpectedWorkspaceVersion: in.ExpectedWorkspaceVersion, PDFSHA256: pdfHash, DOCXSHA256: docxHash, ManifestSHA256: manifestHash, LetterheadContentHash: letterheadHash, PDFSize: int64(len(pdf)), DOCXSize: int64(len(docx)), ReadinessObservedAt: observedAt, ReadinessReady: readiness.Ready, ReadinessBlockers: blockers, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: observedAt}
	result, err := s.store.CommitProposalPackage(ctx, a, commit)
	if errors.Is(err, ErrProposalPackageConflict) {
		return ProposalPackageResult{}, safe("proposal_package_conflict", err)
	}
	if err != nil {
		return ProposalPackageResult{}, safe("proposal_package_unavailable", err)
	}
	return result, nil
}
func (s *ProposalPackageService) Get(ctx context.Context, subject WorkflowSubject, bidID, manifestID ID) (ProposalPackageManifest, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() || manifestID.IsZero() {
		return ProposalPackageManifest{}, safe("proposal_package_read_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return ProposalPackageManifest{}, safe("proposal_package_not_authorized", err)
	}
	result, err := s.store.GetProposalPackageManifest(ctx, a, bidID, manifestID)
	if errors.Is(err, ErrProposalPackageNotFound) {
		return ProposalPackageManifest{}, safe("proposal_package_not_found", err)
	}
	if err != nil {
		return ProposalPackageManifest{}, safe("proposal_package_read_unavailable", err)
	}
	return result, nil
}
func (s *ProposalPackageService) List(ctx context.Context, in ProposalPackageHistoryInput) (ProposalPackageHistoryPage, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.Limit < 1 || in.Limit > 100 || (in.Cursor.CreatedAt.IsZero() != in.Cursor.ID.IsZero()) {
		return ProposalPackageHistoryPage{}, safe("proposal_package_read_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, in.Subject)
	if err != nil {
		return ProposalPackageHistoryPage{}, safe("proposal_package_not_authorized", err)
	}
	rows, err := s.store.ListProposalPackageManifests(ctx, a, in.BidID, in.Cursor, in.Limit+1)
	if errors.Is(err, ErrProposalPackageNotFound) {
		return ProposalPackageHistoryPage{}, safe("proposal_package_not_found", err)
	}
	if err != nil {
		return ProposalPackageHistoryPage{}, safe("proposal_package_read_unavailable", err)
	}
	result := ProposalPackageHistoryPage{Items: rows}
	if len(rows) > int(in.Limit) {
		result.HasMore = true
		result.Items = rows[:in.Limit]
	}
	return result, nil
}
func mapPackageRenderError(err error) error {
	if errors.Is(err, ErrProposalExportUnsupported) {
		return safe("proposal_package_unsupported", err)
	}
	if errors.Is(err, ErrProposalExportTooLarge) {
		return safe("proposal_package_too_large", err)
	}
	return safe("proposal_package_unavailable", err)
}
