package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrDocumentPaidAccessDenied = errors.New("document_paid_access_denied")

type ProcessingClaim struct {
	JobID, RunID, DocumentVersionID, OpportunityID, RequestedBy, TenantID ID
	MeasurementID, CheckpointID, ReservationID, CorrelationID             ID
	AssignmentGeneration, EstimatedCredits                                int64
	ObjectKey, ObjectVersion, MediaType, Capability, UsageKind            string
	SizeBytes                                                             int64
	Attempts                                                              int32
}
type ScanOutcome struct {
	Clean              bool
	ResultCode, Engine string
}
type ExtractionSuggestion struct {
	RequirementText, Category, CitationLocation, CitationQuote string
	Mandatory                                                  bool
	CitationPage                                               int32
	Confidence                                                 float64
}
type ExtractionOutput struct {
	ProviderResponseID string
	Suggestions        []ExtractionSuggestion
}
type ProviderEffectClassification string

const (
	ProviderConfirmedNoEffect ProviderEffectClassification = "CONFIRMED_NO_EFFECT"
	ProviderAmbiguousEffect   ProviderEffectClassification = "AMBIGUOUS_EFFECT"
)

type ProviderExecutionError struct {
	classification ProviderEffectClassification
	cause          error
}

func NewProviderExecutionError(classification ProviderEffectClassification, cause error) error {
	if classification != ProviderConfirmedNoEffect && classification != ProviderAmbiguousEffect {
		classification = ProviderAmbiguousEffect
	}
	return &ProviderExecutionError{classification: classification, cause: cause}
}
func (e *ProviderExecutionError) Error() string {
	if e != nil && e.classification == ProviderConfirmedNoEffect {
		return "document_provider_confirmed_no_effect"
	}
	return "document_provider_outcome_unknown"
}
func (e *ProviderExecutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}
func (e *ProviderExecutionError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
func ProviderEffectOf(err error) ProviderEffectClassification {
	var providerErr *ProviderExecutionError
	if errors.As(err, &providerErr) && providerErr.classification == ProviderConfirmedNoEffect {
		return ProviderConfirmedNoEffect
	}
	return ProviderAmbiguousEffect
}

type DocumentBytes interface {
	OpenExact(context.Context, string, string) (io.ReadCloser, error)
}
type MalwareScanner interface {
	Scan(context.Context, io.Reader, int64) (ScanOutcome, error)
}
type TenderExtractor interface {
	Extract(context.Context, io.Reader, int64, string) (ExtractionOutput, error)
	Model() string
}
type ProcessingAuthorizer interface {
	AuthorizeDocumentProcessing(context.Context, ID, ID, int64, string) error
}
type DocumentReservationVerifier interface {
	VerifyDocumentReservation(context.Context, ProcessingClaim) error
}
type ProcessingStore interface {
	ClaimScan(context.Context, time.Time) (ProcessingClaim, bool, error)
	ReleaseScan(context.Context, ID) error
	CancelJob(context.Context, ID, time.Time) error
	CancelAnalysisAndRequestRelease(context.Context, ProcessingClaim, [3]ID, time.Time) error
	CommitScan(context.Context, ProcessingClaim, ScanOutcome, ID, ID, ID, time.Time) error
	ClaimAnalysis(context.Context, ID, string, time.Time) (ProcessingClaim, bool, error)
	ReleaseAnalysis(context.Context, ID) error
	CommitExtraction(context.Context, ProcessingClaim, ExtractionOutput, []ID, ID, ID, ID, [3]ID, time.Time) error
	FailExtractionAndRequestCompletion(context.Context, ProcessingClaim, string, bool, [3]ID, time.Time) error
}
type DocumentProcessor struct {
	store       ProcessingStore
	objects     DocumentBytes
	scanner     MalwareScanner
	extractor   TenderExtractor
	authorizer  ProcessingAuthorizer
	reservation DocumentReservationVerifier
	ids         CaptureIDGenerator
	clock       CaptureClock
}

func NewDocumentProcessor(store ProcessingStore, objects DocumentBytes, scanner MalwareScanner, extractor TenderExtractor, authorizer ProcessingAuthorizer, ids CaptureIDGenerator, clock CaptureClock) (*DocumentProcessor, error) {
	if store == nil || objects == nil || scanner == nil || extractor == nil || authorizer == nil || ids == nil || clock == nil || extractor.Model() == "" {
		return nil, errors.New("document_processor_invalid")
	}
	return &DocumentProcessor{store: store, objects: objects, scanner: scanner, extractor: extractor, authorizer: authorizer, ids: ids, clock: clock}, nil
}
func NewEntitledDocumentProcessor(store ProcessingStore, objects DocumentBytes, scanner MalwareScanner, extractor TenderExtractor, authorizer ProcessingAuthorizer, reservation DocumentReservationVerifier, ids CaptureIDGenerator, clock CaptureClock) (*DocumentProcessor, error) {
	processor, err := NewDocumentProcessor(store, objects, scanner, extractor, authorizer, ids, clock)
	if err != nil || reservation == nil {
		return nil, errors.New("document_processor_invalid")
	}
	processor.reservation = reservation
	return processor, nil
}
func (p *DocumentProcessor) ProcessNextScan(ctx context.Context) (bool, error) {
	if ctx == nil {
		return false, safe("document_scan_unavailable", nil)
	}
	claim, ok, err := p.store.ClaimScan(ctx, p.clock.Now().UTC())
	if err != nil || !ok {
		return ok, safeIf("document_scan_unavailable", err)
	}
	if err = p.authorizer.AuthorizeDocumentProcessing(ctx, claim.RequestedBy, claim.TenantID, claim.AssignmentGeneration, claim.Capability); err != nil {
		_ = p.store.CancelJob(ctx, claim.JobID, p.clock.Now().UTC())
		return true, safe("document_scan_not_authorized", err)
	}
	reader, err := p.objects.OpenExact(ctx, claim.ObjectKey, claim.ObjectVersion)
	if err != nil {
		_ = p.store.ReleaseScan(ctx, claim.JobID)
		return true, safe("document_scan_unavailable", err)
	}
	outcome, scanErr := p.scanner.Scan(ctx, reader, claim.SizeBytes)
	closeErr := reader.Close()
	if scanErr != nil || closeErr != nil {
		_ = p.store.ReleaseScan(ctx, claim.JobID)
		if scanErr != nil {
			return true, safe("document_scan_unavailable", scanErr)
		}
		return true, safe("document_scan_unavailable", closeErr)
	}
	if !validScanOutcome(outcome) {
		_ = p.store.ReleaseScan(ctx, claim.JobID)
		return true, safe("document_scan_unavailable", nil)
	}
	ids, err := p.newIDs(3)
	if err != nil {
		_ = p.store.ReleaseScan(ctx, claim.JobID)
		return true, safe("document_scan_unavailable", err)
	}
	if err = p.store.CommitScan(ctx, claim, outcome, ids[0], ids[1], ids[2], p.clock.Now().UTC()); err != nil {
		return true, safe("document_scan_unavailable", err)
	}
	return true, nil
}
func (p *DocumentProcessor) ProcessNextAnalysis(ctx context.Context) (bool, error) {
	if ctx == nil || p == nil || p.reservation == nil {
		return false, safe("document_analysis_unavailable", nil)
	}
	initialIDs, err := p.newIDs(4)
	if err != nil {
		return false, safe("document_analysis_unavailable", err)
	}
	runID := initialIDs[0]
	completionIDs := [3]ID{initialIDs[1], initialIDs[2], initialIDs[3]}
	model, err := ResolveConfiguredModel(ctx, p.extractor)
	if err != nil {
		return false, safe("document_analysis_unavailable", err)
	}
	claim, ok, err := p.store.ClaimAnalysis(ctx, runID, model, p.clock.Now().UTC())
	if err != nil || !ok {
		return ok, safeIf("document_analysis_unavailable", err)
	}
	if err = p.authorizer.AuthorizeDocumentProcessing(ctx, claim.RequestedBy, claim.TenantID, claim.AssignmentGeneration, claim.Capability); err != nil {
		if releaseErr := p.store.CancelAnalysisAndRequestRelease(ctx, claim, completionIDs, p.clock.Now().UTC()); releaseErr != nil {
			return true, safe("document_analysis_unavailable", errors.Join(err, releaseErr))
		}
		return true, safe("document_analysis_not_authorized", err)
	}
	if err = p.reservation.VerifyDocumentReservation(ctx, claim); err != nil {
		if errors.Is(err, ErrDocumentPaidAccessDenied) {
			if releaseErr := p.store.CancelAnalysisAndRequestRelease(ctx, claim, completionIDs, p.clock.Now().UTC()); releaseErr != nil {
				return true, safe("document_analysis_unavailable", errors.Join(err, releaseErr))
			}
			return true, safe("document_analysis_not_authorized", err)
		}
		_ = p.store.ReleaseAnalysis(ctx, claim.JobID)
		return true, safe("document_analysis_unavailable", err)
	}
	reader, err := p.objects.OpenExact(ctx, claim.ObjectKey, claim.ObjectVersion)
	if err != nil {
		_ = p.store.ReleaseAnalysis(ctx, claim.JobID)
		return true, safe("document_analysis_unavailable", err)
	}
	output, extractErr := p.extractor.Extract(WithProviderModel(ctx, model), reader, claim.SizeBytes, claim.MediaType)
	closeErr := reader.Close()
	if extractErr != nil || closeErr != nil {
		failure := extractErr
		if failure == nil {
			failure = closeErr
		}
		settle, code := true, "PROVIDER_OUTCOME_UNKNOWN"
		if ProviderEffectOf(failure) == ProviderConfirmedNoEffect {
			settle, code = false, "PROVIDER_CONFIRMED_NO_EFFECT"
		}
		if completionErr := p.store.FailExtractionAndRequestCompletion(ctx, claim, code, settle, completionIDs, p.clock.Now().UTC()); completionErr != nil {
			return true, safe("document_analysis_unavailable", errors.Join(failure, completionErr))
		}
		return true, safe("document_analysis_unavailable", failure)
	}
	if !validExtraction(output, claim.MediaType) {
		if completionErr := p.store.FailExtractionAndRequestCompletion(ctx, claim, "PROVIDER_OUTCOME_UNKNOWN", true, completionIDs, p.clock.Now().UTC()); completionErr != nil {
			return true, safe("document_analysis_unavailable", completionErr)
		}
		return true, safe("document_analysis_unavailable", nil)
	}
	ids, err := p.newIDs(len(output.Suggestions) + 3)
	if err != nil {
		if completionErr := p.store.FailExtractionAndRequestCompletion(ctx, claim, "PROVIDER_OUTCOME_UNKNOWN", true, completionIDs, p.clock.Now().UTC()); completionErr != nil {
			return true, safe("document_analysis_unavailable", errors.Join(err, completionErr))
		}
		return true, safe("document_analysis_unavailable", err)
	}
	suggestionIDs := ids[:len(output.Suggestions)]
	evidenceStart := len(output.Suggestions)
	if err = p.store.CommitExtraction(ctx, claim, output, suggestionIDs, ids[evidenceStart], ids[evidenceStart+1], ids[evidenceStart+2], completionIDs, p.clock.Now().UTC()); err != nil {
		return true, safe("document_analysis_unavailable", err)
	}
	return true, nil
}
func (p *DocumentProcessor) newIDs(n int) ([]ID, error) {
	out := make([]ID, n)
	for i := range out {
		var err error
		out[i], err = p.ids.NewID()
		if err != nil || out[i].IsZero() {
			return nil, errors.New("processing_id_unavailable")
		}
	}
	return out, nil
}
func validScanOutcome(v ScanOutcome) bool {
	return v.Engine != "" && len(v.Engine) <= 100 && v.ResultCode != "" && len(v.ResultCode) <= 100 && utf8.ValidString(v.Engine) && utf8.ValidString(v.ResultCode)
}
func validExtraction(v ExtractionOutput, mediaType string) bool {
	if v.ProviderResponseID == "" || len(v.ProviderResponseID) > 255 || len(v.Suggestions) > 500 {
		return false
	}
	for _, s := range v.Suggestions {
		if !boundedText(s.RequirementText, 10000) || !boundedText(s.Category, 100) || !boundedText(s.CitationLocation, 500) || !boundedText(s.CitationQuote, 2000) || (mediaType == PDFMediaType && s.CitationPage < 1) || s.CitationPage < 0 || s.CitationPage > 10000 || s.Confidence < 0 || s.Confidence > 1 {
			return false
		}
	}
	return true
}
func boundedText(v string, max int) bool {
	return v != "" && len(v) <= max && utf8.ValidString(v) && strings.TrimSpace(v) != ""
}
func safeIf(code string, err error) error {
	if err == nil {
		return nil
	}
	return safe(code, err)
}
