package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"time"
)

var ErrProposalDraftConflict = errors.New("proposal_draft_conflict")

type RequestProposalDraftInput struct {
	Subject                                           WorkflowSubject
	BidID, ProposalVersionID, CorrelationID           ID
	ExpectedWorkspaceVersion, ExpectedProposalVersion int64
	IdempotencyKey                                    string
}
type ProposalDraftCommit struct {
	CommandID, RunID, JobID, BidID, ProposalVersionID, AuditID, OutboxID, CorrelationID  ID
	CheckpointID, CheckpointAuditID, CheckpointOutboxID                                  ID
	ExpectedWorkspaceVersion, ExpectedProposalVersion, InputCharacters, EstimatedCredits int64
	Model                                                                                string
	KeyHash, RequestHash                                                                 [32]byte
	OccurredAt                                                                           time.Time
}
type ProposalDraftRequestResult struct {
	RunID, JobID ID
	State        string
	Replay       bool
}
type GeneratedProposalDraft struct {
	ProviderResponseID, Introduction, Closing, Rationale string
	Citations                                            []GeneratedCitation
}
type ProposalWritingGenerator interface {
	GenerateProposal(context.Context, []DraftInput) (GeneratedProposalDraft, error)
	ProposalPromptCharacters() int64
	ProposalMaximumOutputCharacters() int64
	ProposalActualOutputCharacters(GeneratedProposalDraft) (int64, error)
	Model() string
}
type ProposalDraftClaim struct {
	RunID, JobID, BidID, ProposalID, ProposalVersionID, RequestedBy, CorrelationID, TenantID ID
	CheckpointID, ReservationID                                                              ID
	RequestedWorkspaceVersion, RequestedProposalVersion, AssignmentGeneration                int64
	InputCharacters, EstimatedCredits                                                        int64
	Model, UsageKind                                                                         string
	Inputs                                                                                   []ResponseDraftInputObject
	InputsCurrent, SnapshotCurrent                                                           bool
}
type ProposalDraftSuggestion struct {
	ID, RunID, ProposalVersionID, AppliedProposalVersionID, ReviewedBy     ID
	Introduction, Closing, Rationale, ReviewState, RunState, ReviewNote    string
	RequestedWorkspaceVersion, RequestedProposalVersion, SuggestionVersion int64
	Citations                                                              []ResponseDraftCitation
	CreatedAt, ReviewedAt                                                  time.Time
}
type PreparedProposalDraft struct{ Inputs []ResponseDraftInputObject }
type ProposalDraftStore interface {
	PrepareProposalDraft(context.Context, WorkflowAuthority, ID, ID, int64, int64, time.Time) (PreparedProposalDraft, error)
	RequestProposalDraft(context.Context, WorkflowAuthority, ProposalDraftCommit) (ProposalDraftRequestResult, error)
	ListProposalDrafts(context.Context, WorkflowAuthority, ID, ID) ([]ProposalDraftSuggestion, error)
}
type ProposalDraftWorkerStore interface {
	RecoverProposalDraft(context.Context, time.Time, [3]ID, time.Time) (bool, error)
	ClaimProposalDraft(context.Context, time.Time) (ProposalDraftClaim, bool, error)
	CancelProposalDraft(context.Context, ProposalDraftClaim, [3]ID, time.Time) error
	FailProposalDraft(context.Context, ProposalDraftClaim, string, bool, [3]ID, time.Time) error
	CompleteProposalDraft(context.Context, ProposalDraftClaim, GeneratedProposalDraft, int64, ID, ID, ID, [3]ID, time.Time) error
}
type ProposalDraftService struct {
	auth    TechnicalResponseAuthorizer
	store   ProposalDraftStore
	objects DocumentBytes
	counter GenerationTextCounter
	credits GenerationCreditCalculator
	limits  MeasurementLimits
	model   ProposalWritingGenerator
	ids     CaptureIDGenerator
	clock   CaptureClock
}

func NewProposalDraftService(auth TechnicalResponseAuthorizer, store ProposalDraftStore, objects DocumentBytes, counter GenerationTextCounter, credits GenerationCreditCalculator, limits MeasurementLimits, model ProposalWritingGenerator, ids CaptureIDGenerator, clock CaptureClock) (*ProposalDraftService, error) {
	if auth == nil || store == nil || objects == nil || counter == nil || credits == nil || !limits.Valid() || model == nil || model.Model() == "" || model.ProposalPromptCharacters() < 1 || model.ProposalMaximumOutputCharacters() < 1 || ids == nil || clock == nil {
		return nil, errors.New("proposal_draft_service_invalid")
	}
	return &ProposalDraftService{auth: auth, store: store, objects: objects, counter: counter, credits: credits, limits: limits, model: model, ids: ids, clock: clock}, nil
}
func (s *ProposalDraftService) Request(ctx context.Context, in RequestProposalDraftInput) (ProposalDraftRequestResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.ProposalVersionID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedWorkspaceVersion < 1 || in.ExpectedProposalVersion < 1 || in.IdempotencyKey == "" {
		return ProposalDraftRequestResult{}, safe("proposal_draft_invalid", nil)
	}
	a, err := s.auth.AuthorizeResponseEdit(ctx, in.Subject)
	if err != nil {
		return ProposalDraftRequestResult{}, safe("proposal_draft_not_authorized", err)
	}
	model, err := ResolveConfiguredModel(ctx, s.model)
	if err != nil {
		return ProposalDraftRequestResult{}, safe("proposal_draft_unavailable", err)
	}
	now := s.clock.Now().UTC()
	if now.IsZero() {
		return ProposalDraftRequestResult{}, safe("proposal_draft_unavailable", nil)
	}
	prepared, err := s.store.PrepareProposalDraft(ctx, a, in.BidID, in.ProposalVersionID, in.ExpectedWorkspaceVersion, in.ExpectedProposalVersion, now)
	if errors.Is(err, ErrProposalDraftConflict) {
		return ProposalDraftRequestResult{}, safe("proposal_draft_conflict", err)
	}
	if err != nil || len(prepared.Inputs) < 1 || len(prepared.Inputs) > MaxResponseDraftInputs {
		return ProposalDraftRequestResult{}, safe("proposal_draft_unavailable", err)
	}
	inputCharacters := s.model.ProposalPromptCharacters()
	for _, input := range prepared.Inputs {
		request, x := NewMeasurementRequest(a.TenantID, input.DocumentVersionID, input.MediaType, CleanScanState, input.SizeBytes, input.SHA256)
		if x != nil {
			return ProposalDraftRequestResult{}, safe("proposal_draft_unavailable", x)
		}
		reader, x := s.objects.OpenExact(ctx, input.ObjectKey, input.ObjectVersion)
		if x != nil {
			return ProposalDraftRequestResult{}, safe("proposal_draft_unavailable", x)
		}
		count, countErr := s.counter.Count(ctx, reader, request, s.limits)
		closeErr := reader.Close()
		if countErr != nil || closeErr != nil || count < 1 || inputCharacters > int64(^uint64(0)>>1)-count {
			return ProposalDraftRequestResult{}, safe("proposal_draft_unavailable", errors.Join(countErr, closeErr))
		}
		inputCharacters += count
	}
	estimatedCredits, err := s.credits.CalculateGenerationCredits(ctx, inputCharacters, s.model.ProposalMaximumOutputCharacters())
	if err != nil || estimatedCredits < 1 {
		return ProposalDraftRequestResult{}, safe("proposal_draft_unavailable", err)
	}
	ids, err := responseDraftIDs(s.ids, 8)
	if err != nil {
		return ProposalDraftRequestResult{}, safe("proposal_draft_unavailable", err)
	}
	keyHash := sha256.Sum256([]byte("request-proposal-draft:v2:" + in.IdempotencyKey))
	requestHash := sha256.Sum256([]byte(fmt.Sprintf("request-proposal-draft:v2:%x:%x:%d:%d:%d:%d", in.BidID, in.ProposalVersionID, in.ExpectedWorkspaceVersion, in.ExpectedProposalVersion, inputCharacters, estimatedCredits)))
	result, err := s.store.RequestProposalDraft(ctx, a, ProposalDraftCommit{CommandID: ids[0], RunID: ids[1], JobID: ids[2], BidID: in.BidID, ProposalVersionID: in.ProposalVersionID, AuditID: ids[3], OutboxID: ids[4], CheckpointID: ids[5], CheckpointAuditID: ids[6], CheckpointOutboxID: ids[7], CorrelationID: in.CorrelationID, ExpectedWorkspaceVersion: in.ExpectedWorkspaceVersion, ExpectedProposalVersion: in.ExpectedProposalVersion, InputCharacters: inputCharacters, EstimatedCredits: estimatedCredits, Model: model, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: now})
	if errors.Is(err, ErrProposalDraftConflict) {
		return ProposalDraftRequestResult{}, safe("proposal_draft_conflict", err)
	}
	if err != nil {
		return ProposalDraftRequestResult{}, safe("proposal_draft_unavailable", err)
	}
	return result, nil
}
func (s *ProposalDraftService) List(ctx context.Context, subject WorkflowSubject, bidID, versionID ID) ([]ProposalDraftSuggestion, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() || versionID.IsZero() {
		return nil, safe("proposal_draft_read_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return nil, safe("proposal_draft_not_authorized", err)
	}
	rows, err := s.store.ListProposalDrafts(ctx, a, bidID, versionID)
	if err != nil {
		return nil, safe("proposal_draft_read_unavailable", err)
	}
	return rows, nil
}

type ProposalDraftWorker struct {
	store                ProposalDraftWorkerStore
	objects              DocumentBytes
	generator            ProposalWritingGenerator
	authorize            ProcessingAuthorizer
	reservation          ResponseDraftReservationVerifier
	credits              GenerationCreditCalculator
	tenantID             ID
	assignmentGeneration int64
	ids                  CaptureIDGenerator
	clock                CaptureClock
}

func NewProposalDraftWorker(store ProposalDraftWorkerStore, objects DocumentBytes, generator ProposalWritingGenerator, authorize ProcessingAuthorizer, reservation ResponseDraftReservationVerifier, credits GenerationCreditCalculator, tenantID ID, generation int64, ids CaptureIDGenerator, clock CaptureClock) (*ProposalDraftWorker, error) {
	if store == nil || objects == nil || generator == nil || authorize == nil || reservation == nil || credits == nil || tenantID.IsZero() || generation < 1 || ids == nil || clock == nil {
		return nil, errors.New("proposal_draft_worker_invalid")
	}
	return &ProposalDraftWorker{store: store, objects: objects, generator: generator, authorize: authorize, reservation: reservation, credits: credits, tenantID: tenantID, assignmentGeneration: generation, ids: ids, clock: clock}, nil
}
func (w *ProposalDraftWorker) ProcessNext(ctx context.Context) (bool, error) {
	ids, err := responseDraftIDs(w.ids, 6)
	if ctx == nil || err != nil {
		return false, safe("proposal_draft_processing_unavailable", err)
	}
	now := w.clock.Now().UTC()
	recovered, recoveryErr := w.store.RecoverProposalDraft(ctx, now.Add(-15*time.Minute), [3]ID{ids[3], ids[4], ids[5]}, now)
	if recoveryErr != nil || recovered {
		return recovered, safeIf("proposal_draft_processing_unavailable", recoveryErr)
	}
	claim, ok, err := w.store.ClaimProposalDraft(ctx, now)
	if err != nil || !ok {
		return ok, safeIf("proposal_draft_processing_unavailable", err)
	}
	claim.TenantID, claim.AssignmentGeneration = w.tenantID, w.assignmentGeneration
	currentModel, modelErr := ResolveConfiguredModel(ctx, w.generator)
	if modelErr != nil || claim.Model != currentModel {
		_ = w.store.FailProposalDraft(ctx, claim, "MODEL_MISMATCH", false, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("proposal_draft_processing_unavailable", nil)
	}
	if err = w.authorize.AuthorizeDocumentProcessing(ctx, claim.RequestedBy, w.tenantID, w.assignmentGeneration, "tenderpreneur.bid.response.edit"); err != nil {
		_ = w.store.CancelProposalDraft(ctx, claim, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("proposal_draft_processing_not_authorized", err)
	}
	if err = w.reservation.VerifyProposalDraftReservation(ctx, claim); err != nil {
		_ = w.store.CancelProposalDraft(ctx, claim, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("proposal_draft_processing_not_authorized", err)
	}
	if !claim.SnapshotCurrent || !claim.InputsCurrent {
		_ = w.store.FailProposalDraft(ctx, claim, "INPUT_EXPIRED", false, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("proposal_draft_processing_unavailable", nil)
	}
	if len(claim.Inputs) < 1 || len(claim.Inputs) > 5 {
		_ = w.store.FailProposalDraft(ctx, claim, "INPUT_COUNT_INVALID", false, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("proposal_draft_processing_unavailable", nil)
	}
	readers := make([]io.ReadCloser, 0, len(claim.Inputs))
	defer func() {
		for _, r := range readers {
			_ = r.Close()
		}
	}()
	inputs := make([]DraftInput, 0, len(claim.Inputs))
	var total int64
	for _, input := range claim.Inputs {
		total += input.SizeBytes
		if total > MaxResponseDraftInputBytes {
			_ = w.store.FailProposalDraft(ctx, claim, "INPUT_SIZE_EXCEEDED", false, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
			return true, safe("proposal_draft_processing_unavailable", nil)
		}
		reader, e := w.objects.OpenExact(ctx, input.ObjectKey, input.ObjectVersion)
		if e != nil {
			_ = w.store.FailProposalDraft(ctx, claim, "INPUT_UNAVAILABLE", false, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
			return true, safe("proposal_draft_processing_unavailable", e)
		}
		readers = append(readers, reader)
		inputs = append(inputs, DraftInput{Ordinal: input.Ordinal, MediaType: input.MediaType, SizeBytes: input.SizeBytes, Reader: reader})
	}
	generated, err := w.generator.GenerateProposal(WithProviderModel(ctx, claim.Model), inputs)
	if err != nil {
		_ = w.store.FailProposalDraft(ctx, claim, "PROVIDER_OR_OUTPUT_FAILURE", ProviderEffectOf(err) != ProviderConfirmedNoEffect, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("proposal_draft_processing_unavailable", err)
	}
	if !validGeneratedProposalDraft(generated, len(inputs)) {
		_ = w.store.FailProposalDraft(ctx, claim, "INVALID_PROVIDER_OUTPUT", true, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("proposal_draft_processing_unavailable", nil)
	}
	outputCharacters, err := w.generator.ProposalActualOutputCharacters(generated)
	if err != nil {
		_ = w.store.FailProposalDraft(ctx, claim, "INVALID_PROVIDER_OUTPUT", true, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("proposal_draft_processing_unavailable", err)
	}
	actualCredits, err := w.credits.CalculateGenerationCredits(ctx, claim.InputCharacters, outputCharacters)
	if err != nil || actualCredits < 1 {
		_ = w.store.FailProposalDraft(ctx, claim, "ACCOUNTING_FAILURE", true, [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC())
		return true, safe("proposal_draft_processing_unavailable", err)
	}
	if err = w.store.CompleteProposalDraft(ctx, claim, generated, actualCredits, ids[0], ids[1], ids[2], [3]ID{ids[3], ids[4], ids[5]}, w.clock.Now().UTC()); err != nil {
		return true, safe("proposal_draft_processing_unavailable", err)
	}
	return true, nil
}
func validGeneratedProposalDraft(v GeneratedProposalDraft, inputCount int) bool {
	if v.ProviderResponseID == "" || len(v.ProviderResponseID) > 255 || !validPlainResponse(v.Introduction, 10000) || !validPlainResponse(v.Closing, 10000) || !validPlainResponse(v.Rationale, 2000) || len(v.Citations) < 1 || len(v.Citations) > 100 {
		return false
	}
	for _, c := range v.Citations {
		if c.InputOrdinal < 1 || int(c.InputOrdinal) > inputCount || !validRequirementText(c.Location, 500) || !validRequirementText(c.Quote, 2000) {
			return false
		}
	}
	return true
}
