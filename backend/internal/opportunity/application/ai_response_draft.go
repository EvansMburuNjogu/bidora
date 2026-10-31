package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"time"
)

var ErrResponseDraftConflict = errors.New("response_draft_conflict")

const MaxResponseDraftInputs = 5
const MaxResponseDraftInputBytes int64 = 50 * 1024 * 1024

type RequestResponseDraftInput struct {
	Subject                                          WorkflowSubject
	BidID, RequirementID, CorrelationID              ID
	ExpectedWorkspaceVersion, ExpectedSectionVersion int64
	IdempotencyKey                                   string
}
type ResponseDraftRequestCommit struct {
	CommandID, RunID, JobID, BidID, RequirementID, AuditID, OutboxID, CorrelationID ID
	CheckpointID, CheckpointAuditID, CheckpointOutboxID                             ID
	Model                                                                           string
	ExpectedWorkspaceVersion, ExpectedSectionVersion                                int64
	InputCharacters, EstimatedCredits                                               int64
	KeyHash, RequestHash                                                            [32]byte
	OccurredAt                                                                      time.Time
}
type ResponseDraftRequestResult struct {
	RunID, JobID ID
	State        string
	Replay       bool
}
type ResponseDraftInputObject struct {
	DocumentVersionID                              ID
	InputKind, ObjectKey, ObjectVersion, MediaType string
	Ordinal                                        int32
	SizeBytes                                      int64
	SHA256                                         [32]byte
}

type PreparedResponseDraft struct {
	RequirementText string
	Inputs          []ResponseDraftInputObject
}
type ResponseDraftClaim struct {
	RunID, JobID, BidID, RequirementID, RequestedBy, CorrelationID, TenantID ID
	CheckpointID, ReservationID                                              ID
	AssignmentGeneration, InputCharacters, EstimatedCredits                  int64
	RequirementText, Model, UsageKind                                        string
	RequestedWorkspaceVersion, RequestedSectionVersion                       int64
	Inputs                                                                   []ResponseDraftInputObject
	InputsCurrent, SnapshotCurrent                                           bool
}
type DraftInput struct {
	Ordinal   int32
	MediaType string
	SizeBytes int64
	Reader    io.Reader
}
type GeneratedCitation struct {
	InputOrdinal    int32
	Location, Quote string
}
type GeneratedGap struct {
	Code, Description string
	InputOrdinal      int32
	Location, Quote   string
}
type GeneratedResponseDraft struct {
	ProviderResponseID, Content, Rationale string
	Citations                              []GeneratedCitation
	Gaps                                   []GeneratedGap
}
type ResponseDraftGenerator interface {
	Generate(context.Context, string, []DraftInput) (GeneratedResponseDraft, error)
	PromptCharacters(string) (int64, error)
	MaximumOutputCharacters() int64
	ActualOutputCharacters(GeneratedResponseDraft) (int64, error)
	Model() string
}

type GenerationTextCounter interface {
	Count(context.Context, io.Reader, MeasurementRequest, MeasurementLimits) (int64, error)
}

type GenerationCreditCalculator interface {
	CalculateGenerationCredits(context.Context, int64, int64) (int64, error)
}
type ResponseDraftCitation struct {
	DocumentVersionID ID
	Location, Quote   string
	Ordinal           int32
}
type ResponseGapSuggestion struct {
	Code, Description string
	DocumentVersionID ID
	HasCitation       bool
	Location, Quote   string
	Ordinal           int32
}
type ResponseDraftSuggestion struct {
	ID, RunID, BidID, RequirementID                    ID
	Content, Rationale, ReviewState, RunState          string
	RequestedWorkspaceVersion, RequestedSectionVersion int64
	Citations                                          []ResponseDraftCitation
	Gaps                                               []ResponseGapSuggestion
	CreatedAt                                          time.Time
}

type ResponseDraftStore interface {
	PrepareResponseDraft(context.Context, WorkflowAuthority, ID, ID, int64, int64, time.Time) (PreparedResponseDraft, error)
	RequestResponseDraft(context.Context, WorkflowAuthority, ResponseDraftRequestCommit) (ResponseDraftRequestResult, error)
	ListResponseDrafts(context.Context, WorkflowAuthority, ID, ID) ([]ResponseDraftSuggestion, error)
}
type ResponseDraftWorkerStore interface {
	RecoverResponseDraft(context.Context, time.Time, [3]ID, time.Time) (bool, error)
	ClaimResponseDraft(context.Context, time.Time) (ResponseDraftClaim, bool, error)
	CancelResponseDraft(context.Context, ResponseDraftClaim, [3]ID, time.Time) error
	FailResponseDraft(context.Context, ResponseDraftClaim, string, bool, [3]ID, time.Time) error
	CompleteResponseDraft(context.Context, ResponseDraftClaim, GeneratedResponseDraft, int64, ID, ID, ID, [3]ID, time.Time) error
}

type ResponseDraftReservationVerifier interface {
	VerifyResponseDraftReservation(context.Context, ResponseDraftClaim) error
	VerifyProposalDraftReservation(context.Context, ProposalDraftClaim) error
}

type ResponseDraftService struct {
	auth    TechnicalResponseAuthorizer
	store   ResponseDraftStore
	objects DocumentBytes
	counter GenerationTextCounter
	credits GenerationCreditCalculator
	limits  MeasurementLimits
	model   ResponseDraftGenerator
	ids     CaptureIDGenerator
	clock   CaptureClock
}

func NewResponseDraftService(auth TechnicalResponseAuthorizer, store ResponseDraftStore, objects DocumentBytes, counter GenerationTextCounter, credits GenerationCreditCalculator, limits MeasurementLimits, model ResponseDraftGenerator, ids CaptureIDGenerator, clock CaptureClock) (*ResponseDraftService, error) {
	if auth == nil || store == nil || objects == nil || counter == nil || credits == nil || !limits.Valid() || model == nil || model.Model() == "" || len(model.Model()) > 200 || model.MaximumOutputCharacters() < 1 || ids == nil || clock == nil {
		return nil, errors.New("response_draft_service_invalid")
	}
	return &ResponseDraftService{auth: auth, store: store, objects: objects, counter: counter, credits: credits, limits: limits, model: model, ids: ids, clock: clock}, nil
}
func (s *ResponseDraftService) Request(ctx context.Context, in RequestResponseDraftInput) (ResponseDraftRequestResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.RequirementID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedWorkspaceVersion < 1 || in.ExpectedSectionVersion < 0 || in.IdempotencyKey == "" {
		return ResponseDraftRequestResult{}, safe("response_draft_invalid", nil)
	}
	authority, err := s.auth.AuthorizeResponseEdit(ctx, in.Subject)
	if err != nil {
		return ResponseDraftRequestResult{}, safe("response_draft_not_authorized", err)
	}
	model, err := ResolveConfiguredModel(ctx, s.model)
	if err != nil {
		return ResponseDraftRequestResult{}, safe("response_draft_unavailable", err)
	}
	now := s.clock.Now().UTC()
	if now.IsZero() {
		return ResponseDraftRequestResult{}, safe("response_draft_unavailable", nil)
	}
	prepared, err := s.store.PrepareResponseDraft(ctx, authority, in.BidID, in.RequirementID, in.ExpectedWorkspaceVersion, in.ExpectedSectionVersion, now)
	if errors.Is(err, ErrResponseDraftConflict) {
		return ResponseDraftRequestResult{}, safe("response_draft_conflict", err)
	}
	if err != nil || len(prepared.Inputs) < 1 || len(prepared.Inputs) > MaxResponseDraftInputs {
		return ResponseDraftRequestResult{}, safe("response_draft_unavailable", err)
	}
	inputCharacters, err := s.model.PromptCharacters(prepared.RequirementText)
	if err != nil || inputCharacters < 1 {
		return ResponseDraftRequestResult{}, safe("response_draft_unavailable", err)
	}
	for _, input := range prepared.Inputs {
		request, requestErr := NewMeasurementRequest(authority.TenantID, input.DocumentVersionID, input.MediaType, CleanScanState, input.SizeBytes, input.SHA256)
		if requestErr != nil {
			return ResponseDraftRequestResult{}, safe("response_draft_unavailable", requestErr)
		}
		reader, openErr := s.objects.OpenExact(ctx, input.ObjectKey, input.ObjectVersion)
		if openErr != nil {
			return ResponseDraftRequestResult{}, safe("response_draft_unavailable", openErr)
		}
		count, countErr := s.counter.Count(ctx, reader, request, s.limits)
		closeErr := reader.Close()
		if countErr != nil || closeErr != nil || count < 1 || inputCharacters > int64(^uint64(0)>>1)-count {
			return ResponseDraftRequestResult{}, safe("response_draft_unavailable", errors.Join(countErr, closeErr))
		}
		inputCharacters += count
	}
	estimatedCredits, err := s.credits.CalculateGenerationCredits(ctx, inputCharacters, s.model.MaximumOutputCharacters())
	if err != nil || estimatedCredits < 1 {
		return ResponseDraftRequestResult{}, safe("response_draft_unavailable", err)
	}
	ids, err := responseDraftIDs(s.ids, 8)
	if err != nil {
		return ResponseDraftRequestResult{}, safe("response_draft_unavailable", err)
	}
	keyHash := sha256.Sum256([]byte("request-response-draft:v2:" + in.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("request-response-draft:v2:%x:%x:%d:%d:%d:%d", in.BidID, in.RequirementID, in.ExpectedWorkspaceVersion, in.ExpectedSectionVersion, inputCharacters, estimatedCredits)))
	result, err := s.store.RequestResponseDraft(ctx, authority, ResponseDraftRequestCommit{CommandID: ids[0], RunID: ids[1], JobID: ids[2], BidID: in.BidID, RequirementID: in.RequirementID, AuditID: ids[3], OutboxID: ids[4], CheckpointID: ids[5], CheckpointAuditID: ids[6], CheckpointOutboxID: ids[7], CorrelationID: in.CorrelationID, Model: model, ExpectedWorkspaceVersion: in.ExpectedWorkspaceVersion, ExpectedSectionVersion: in.ExpectedSectionVersion, InputCharacters: inputCharacters, EstimatedCredits: estimatedCredits, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: now})
	if errors.Is(err, ErrResponseDraftConflict) {
		return ResponseDraftRequestResult{}, safe("response_draft_conflict", err)
	}
	if err != nil {
		return ResponseDraftRequestResult{}, safe("response_draft_unavailable", err)
	}
	return result, nil
}
func (s *ResponseDraftService) List(ctx context.Context, subject WorkflowSubject, bidID, requirementID ID) ([]ResponseDraftSuggestion, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() || requirementID.IsZero() {
		return nil, safe("response_draft_read_invalid", nil)
	}
	authority, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return nil, safe("response_draft_not_authorized", err)
	}
	rows, err := s.store.ListResponseDrafts(ctx, authority, bidID, requirementID)
	if err != nil {
		return nil, safe("response_draft_read_unavailable", err)
	}
	return rows, nil
}

type ResponseDraftWorker struct {
	store                ResponseDraftWorkerStore
	objects              DocumentBytes
	generator            ResponseDraftGenerator
	authorize            ProcessingAuthorizer
	reservation          ResponseDraftReservationVerifier
	credits              GenerationCreditCalculator
	tenantID             ID
	assignmentGeneration int64
	ids                  CaptureIDGenerator
	clock                CaptureClock
}

func NewResponseDraftWorker(store ResponseDraftWorkerStore, objects DocumentBytes, generator ResponseDraftGenerator, authorize ProcessingAuthorizer, reservation ResponseDraftReservationVerifier, credits GenerationCreditCalculator, tenantID ID, assignmentGeneration int64, ids CaptureIDGenerator, clock CaptureClock) (*ResponseDraftWorker, error) {
	if store == nil || objects == nil || generator == nil || authorize == nil || reservation == nil || credits == nil || tenantID.IsZero() || assignmentGeneration < 1 || ids == nil || clock == nil || generator.Model() == "" {
		return nil, errors.New("response_draft_worker_invalid")
	}
	return &ResponseDraftWorker{store: store, objects: objects, generator: generator, authorize: authorize, reservation: reservation, credits: credits, tenantID: tenantID, assignmentGeneration: assignmentGeneration, ids: ids, clock: clock}, nil
}
func (w *ResponseDraftWorker) ProcessNext(ctx context.Context) (bool, error) {
	if ctx == nil {
		return false, safe("response_draft_processing_unavailable", nil)
	}
	ids, err := responseDraftIDs(w.ids, 6)
	if err != nil {
		return false, safe("response_draft_processing_unavailable", err)
	}
	now := w.clock.Now().UTC()
	recovered, err := w.store.RecoverResponseDraft(ctx, now.Add(-15*time.Minute), [3]ID{ids[3], ids[4], ids[5]}, now)
	if err != nil || recovered {
		return recovered, safeIf("response_draft_processing_unavailable", err)
	}
	claim, ok, err := w.store.ClaimResponseDraft(ctx, now)
	if err != nil || !ok {
		return ok, safeIf("response_draft_processing_unavailable", err)
	}
	claim.TenantID, claim.AssignmentGeneration = w.tenantID, w.assignmentGeneration
	currentModel, modelErr := ResolveConfiguredModel(ctx, w.generator)
	if modelErr != nil || claim.Model != currentModel {
		_ = w.store.FailResponseDraft(ctx, claim, "MODEL_MISMATCH", false, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("response_draft_processing_unavailable", nil)
	}
	if err = w.authorize.AuthorizeDocumentProcessing(ctx, claim.RequestedBy, w.tenantID, w.assignmentGeneration, "tenderpreneur.bid.response.edit"); err != nil {
		_ = w.store.CancelResponseDraft(ctx, claim, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("response_draft_processing_not_authorized", err)
	}
	if err = w.reservation.VerifyResponseDraftReservation(ctx, claim); err != nil {
		_ = w.store.CancelResponseDraft(ctx, claim, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("response_draft_processing_not_authorized", err)
	}
	if !claim.SnapshotCurrent || !claim.InputsCurrent {
		_ = w.store.FailResponseDraft(ctx, claim, "INPUT_EXPIRED", false, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("response_draft_processing_unavailable", nil)
	}
	if len(claim.Inputs) < 1 || len(claim.Inputs) > MaxResponseDraftInputs {
		_ = w.store.FailResponseDraft(ctx, claim, "INPUT_COUNT_INVALID", false, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("response_draft_processing_unavailable", nil)
	}
	var total int64
	readers := make([]io.ReadCloser, 0, len(claim.Inputs))
	draftInputs := make([]DraftInput, 0, len(claim.Inputs))
	defer func() {
		for _, reader := range readers {
			_ = reader.Close()
		}
	}()
	for _, input := range claim.Inputs {
		total += input.SizeBytes
		if total > MaxResponseDraftInputBytes {
			_ = w.store.FailResponseDraft(ctx, claim, "INPUT_SIZE_EXCEEDED", false, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
			return true, safe("response_draft_processing_unavailable", nil)
		}
		reader, openErr := w.objects.OpenExact(ctx, input.ObjectKey, input.ObjectVersion)
		if openErr != nil {
			_ = w.store.FailResponseDraft(ctx, claim, "INPUT_UNAVAILABLE", false, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
			return true, safe("response_draft_processing_unavailable", openErr)
		}
		readers = append(readers, reader)
		draftInputs = append(draftInputs, DraftInput{Ordinal: input.Ordinal, MediaType: input.MediaType, SizeBytes: input.SizeBytes, Reader: reader})
	}
	generated, err := w.generator.Generate(WithProviderModel(ctx, claim.Model), claim.RequirementText, draftInputs)
	if err != nil {
		settle := ProviderEffectOf(err) != ProviderConfirmedNoEffect
		_ = w.store.FailResponseDraft(ctx, claim, "PROVIDER_OR_OUTPUT_FAILURE", settle, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("response_draft_processing_unavailable", err)
	}
	if !validGeneratedDraft(generated, len(claim.Inputs)) {
		_ = w.store.FailResponseDraft(ctx, claim, "INVALID_PROVIDER_OUTPUT", true, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("response_draft_processing_unavailable", nil)
	}
	outputCharacters, err := w.generator.ActualOutputCharacters(generated)
	if err != nil {
		_ = w.store.FailResponseDraft(ctx, claim, "INVALID_PROVIDER_OUTPUT", true, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("response_draft_processing_unavailable", err)
	}
	actualCredits, err := w.credits.CalculateGenerationCredits(ctx, claim.InputCharacters, outputCharacters)
	if err != nil || actualCredits < 1 {
		_ = w.store.FailResponseDraft(ctx, claim, "ACCOUNTING_FAILURE", true, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("response_draft_processing_unavailable", err)
	}
	if err = w.store.CompleteResponseDraft(ctx, claim, generated, actualCredits, ids[0], ids[1], ids[2], [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC()); err != nil {
		return true, safe("response_draft_processing_unavailable", err)
	}
	return true, nil
}
func responseDraftIDs(generator CaptureIDGenerator, n int) ([]ID, error) {
	out := make([]ID, n)
	for i := range out {
		var err error
		out[i], err = generator.NewID()
		if err != nil || out[i].IsZero() {
			return nil, errors.New("response_draft_id_unavailable")
		}
	}
	return out, nil
}
func validGeneratedDraft(v GeneratedResponseDraft, inputCount int) bool {
	if v.ProviderResponseID == "" || len(v.ProviderResponseID) > 255 || !validPlainResponse(v.Content, 20000) || !validPlainResponse(v.Rationale, 2000) || len(v.Citations) < 1 || len(v.Citations) > 100 || len(v.Gaps) > 100 {
		return false
	}
	for _, c := range v.Citations {
		if c.InputOrdinal < 1 || int(c.InputOrdinal) > inputCount || !validRequirementText(c.Location, 500) || !validRequirementText(c.Quote, 2000) {
			return false
		}
	}
	allowed := map[string]bool{"MISSING_EVIDENCE": true, "UNSUPPORTED_CLAIM": true, "MISSING_FACT": true, "AMBIGUOUS_REQUIREMENT": true, "REVIEW_NEEDED": true}
	for _, g := range v.Gaps {
		if !allowed[g.Code] || !validRequirementText(g.Description, 2000) || g.InputOrdinal < 0 || int(g.InputOrdinal) > inputCount || (g.InputOrdinal == 0 && (g.Location != "" || g.Quote != "")) || (g.InputOrdinal > 0 && (!validRequirementText(g.Location, 500) || !validRequirementText(g.Quote, 2000))) {
			return false
		}
	}
	return true
}
