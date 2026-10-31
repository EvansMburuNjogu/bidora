package application

import (
	"context"
	"errors"
	"time"
)

var ErrDocumentMeasurementOrchestrationInvalid = errors.New("document_measurement_orchestration_invalid")

const (
	DocumentMeasurementLeaseDuration = 5 * time.Minute
	DocumentMeasurementFirstBackoff  = 30 * time.Second
	DocumentMeasurementSecondBackoff = 120 * time.Second
	DocumentMeasurementMaxAttempts   = int32(3)
)

type MeasurementAttemptIdentity struct {
	ParserKey, ParserVersion, PDFSemantics, DOCXSemantics, ProfileVersion string
}

func (v MeasurementAttemptIdentity) Valid() bool {
	return safeMeasurementLabel(v.ParserKey, 100) && safeMeasurementLabel(v.ParserVersion, 100) && safeMeasurementLabel(v.PDFSemantics, 100) && safeMeasurementLabel(v.DOCXSemantics, 100) && safeMeasurementLabel(v.ProfileVersion, 100)
}

type DocumentMeasurementLeaseClaim struct {
	LeaseID  ID
	Identity MeasurementAttemptIdentity
}
type DocumentMeasurementClaim struct {
	JobID, DocumentVersionID, OpportunityID, RequestedBy, TenantID, LeaseID ID
	AssignmentGeneration                                                    int64
	ObjectKey, ObjectVersion, MediaType, Capability                         string
	ParserKey, ParserVersion, SemanticsVersion, ProfileVersion              string
	SizeBytes                                                               int64
	SHA256                                                                  [32]byte
	Attempts, MeasurementAttempts                                           int32
	LeaseExpiresAt                                                          time.Time
}

type PendingDocumentReservation struct {
	MeasurementID, JobID, DocumentVersionID, RequestedBy, TenantID, CorrelationID ID
	AssignmentGeneration, MeasuredUnits, EstimatedCredits                         int64
	Unit, UsageKind                                                               string
}

type DocumentMeasurementCommit struct {
	MeasurementID, AuditID, OutboxID, CorrelationID, AttemptOutcomeID ID
	Claim                                                             DocumentMeasurementClaim
	Measurement                                                       DocumentMeasurement
	UsageKind                                                         string
	EstimatedCredits                                                  int64
	OccurredAt                                                        time.Time
}

type EntitlementReservationRequest struct {
	MeasurementID, JobID, TenantID, RequestedBy, CorrelationID ID
	AssignmentGeneration, EstimatedCredits                     int64
	UsageKind                                                  string
}

type EntitlementReservationReference struct{ CheckpointID ID }

type DocumentMeasurementStore interface {
	FindPendingDocumentReservation(context.Context) (PendingDocumentReservation, bool, error)
	RecoverExpiredDocumentMeasurement(context.Context, ID) (bool, error)
	ClaimDocumentMeasurement(context.Context, DocumentMeasurementLeaseClaim) (DocumentMeasurementClaim, bool, error)
	RetryDocumentMeasurement(context.Context, DocumentMeasurementClaim, ID, string, time.Time, time.Time) error
	FailDocumentMeasurement(context.Context, DocumentMeasurementClaim, ID, string, time.Time) error
	CancelDocumentMeasurement(context.Context, DocumentMeasurementClaim, ID, time.Time) error
	CommitDocumentMeasurement(context.Context, DocumentMeasurementCommit) error
	LinkDocumentMeasurementEntitlement(context.Context, PendingDocumentReservation, ID, ID, time.Time) error
}

type DocumentCreditCalculator interface {
	CalculateDocumentCredits(context.Context, string, int64) (int64, error)
}

type DocumentEntitlementRequester interface {
	RequestDocumentEntitlement(context.Context, EntitlementReservationRequest) (EntitlementReservationReference, error)
}

type DocumentMeasurementCoordinator struct {
	store       DocumentMeasurementStore
	objects     DocumentBytes
	measurer    DocumentMeasurer
	authorizer  ProcessingAuthorizer
	credits     DocumentCreditCalculator
	entitlement DocumentEntitlementRequester
	ids         CaptureIDGenerator
	clock       CaptureClock
	limits      MeasurementLimits
	identity    MeasurementAttemptIdentity
}

func NewDocumentMeasurementCoordinator(store DocumentMeasurementStore, objects DocumentBytes, measurer DocumentMeasurer, authorizer ProcessingAuthorizer, credits DocumentCreditCalculator, entitlement DocumentEntitlementRequester, ids CaptureIDGenerator, clock CaptureClock, limits MeasurementLimits, identity MeasurementAttemptIdentity) (*DocumentMeasurementCoordinator, error) {
	if store == nil || objects == nil || measurer == nil || authorizer == nil || credits == nil || entitlement == nil || ids == nil || clock == nil || !limits.Valid() || !identity.Valid() {
		return nil, ErrDocumentMeasurementOrchestrationInvalid
	}
	return &DocumentMeasurementCoordinator{store: store, objects: objects, measurer: measurer, authorizer: authorizer, credits: credits, entitlement: entitlement, ids: ids, clock: clock, limits: limits, identity: identity}, nil
}

// ProcessNext reconciles committed measurement work, recovers one expired
// fenced attempt, or measures one CLEAN exact version. No SQL transaction is
// held while bytes are opened or parsed.
func (c *DocumentMeasurementCoordinator) ProcessNext(ctx context.Context) (bool, error) {
	if c == nil || ctx == nil {
		return false, safe("document_measurement_unavailable", nil)
	}
	pending, found, err := c.store.FindPendingDocumentReservation(ctx)
	if err != nil {
		return false, safe("document_measurement_unavailable", err)
	}
	if found {
		return true, c.requestEntitlement(ctx, pending)
	}
	at := c.clock.Now().UTC()
	if at.IsZero() {
		return false, safe("document_measurement_unavailable", nil)
	}
	recoveryID, err := c.ids.NewID()
	if err != nil || recoveryID.IsZero() {
		return false, safe("document_measurement_unavailable", err)
	}
	recovered, err := c.store.RecoverExpiredDocumentMeasurement(ctx, recoveryID)
	if err != nil || recovered {
		return recovered, safeIf("document_measurement_unavailable", err)
	}
	attemptIDs, err := c.newIDs(2)
	if err != nil {
		return false, safe("document_measurement_unavailable", err)
	}
	lease := DocumentMeasurementLeaseClaim{LeaseID: attemptIDs[0], Identity: c.identity}
	claim, found, err := c.store.ClaimDocumentMeasurement(ctx, lease)
	if err != nil || !found {
		return found, safeIf("document_measurement_unavailable", err)
	}
	outcomeID := attemptIDs[1]
	if err = c.authorizer.AuthorizeDocumentProcessing(ctx, claim.RequestedBy, claim.TenantID, claim.AssignmentGeneration, claim.Capability); err != nil {
		if ctx.Err() != nil {
			return true, safe("document_measurement_unavailable", err)
		}
		transitionErr := c.store.CancelDocumentMeasurement(ctx, claim, outcomeID, c.clock.Now().UTC())
		if transitionErr != nil {
			return true, safe("document_measurement_unavailable", errors.Join(err, transitionErr))
		}
		return true, safe("document_measurement_not_authorized", err)
	}

	request, err := NewMeasurementRequest(claim.TenantID, claim.DocumentVersionID, claim.MediaType, CleanScanState, claim.SizeBytes, claim.SHA256)
	if err != nil {
		return true, c.finishFailure(ctx, claim, outcomeID, "MEASUREMENT_INPUT_INVALID", false, err)
	}
	reader, err := c.objects.OpenExact(ctx, claim.ObjectKey, claim.ObjectVersion)
	if err != nil {
		if ctx.Err() != nil {
			return true, safe("document_measurement_unavailable", err)
		}
		return true, c.finishFailure(ctx, claim, outcomeID, "OBJECT_STORAGE_UNAVAILABLE", true, err)
	}
	measurement, measureErr := c.measurer.Measure(ctx, reader, request, c.limits)
	closeErr := reader.Close()
	if measureErr != nil || closeErr != nil {
		failure := measureErr
		if failure == nil {
			failure = closeErr
		}
		if ctx.Err() != nil {
			return true, safe("document_measurement_unavailable", failure)
		}
		code, transient := measurementFailureDisposition(failure)
		if closeErr != nil && measureErr == nil {
			code, transient = "OBJECT_CLOSE_UNAVAILABLE", true
		}
		return true, c.finishFailure(ctx, claim, outcomeID, code, transient, failure)
	}
	if measurement.ParserKey != claim.ParserKey || measurement.ParserVersion != claim.ParserVersion || measurement.SemanticsVersion != claim.SemanticsVersion || measurement.ProfileVersion != claim.ProfileVersion {
		return true, c.finishFailure(ctx, claim, outcomeID, "MEASUREMENT_RESULT_MISMATCH", false, ErrDocumentMeasurementMismatch)
	}
	if err = measurement.ValidateFor(request, c.limits); err != nil {
		return true, c.finishFailure(ctx, claim, outcomeID, "MEASUREMENT_RESULT_MISMATCH", false, err)
	}
	calculation, err := c.credits.CalculateDocumentCredits(ctx, string(measurement.Unit), measurement.MeasuredUnits)
	if err != nil || calculation < 1 {
		if ctx.Err() != nil {
			return true, safe("document_measurement_unavailable", err)
		}
		return true, c.finishFailure(ctx, claim, outcomeID, "CREDIT_CALCULATION_UNAVAILABLE", true, err)
	}
	usageKind := "DOCUMENT_PDF"
	if measurement.Unit == DOCXExtractedCharacters {
		usageKind = "DOCUMENT_DOCX"
	}
	ids, err := c.newIDs(4)
	if err != nil {
		return true, c.finishFailure(ctx, claim, outcomeID, "ID_GENERATION_UNAVAILABLE", true, err)
	}
	at = c.clock.Now().UTC()
	if at.IsZero() {
		return true, safe("document_measurement_unavailable", nil)
	}
	commit := DocumentMeasurementCommit{MeasurementID: ids[0], AuditID: ids[1], OutboxID: ids[2], CorrelationID: ids[3], AttemptOutcomeID: outcomeID, Claim: claim, Measurement: measurement, UsageKind: usageKind, EstimatedCredits: calculation, OccurredAt: at}
	if err = c.store.CommitDocumentMeasurement(ctx, commit); err != nil {
		return true, safe("document_measurement_unavailable", err)
	}
	pending = PendingDocumentReservation{MeasurementID: commit.MeasurementID, JobID: claim.JobID, DocumentVersionID: claim.DocumentVersionID, RequestedBy: claim.RequestedBy, TenantID: claim.TenantID, CorrelationID: commit.CorrelationID, AssignmentGeneration: claim.AssignmentGeneration, MeasuredUnits: measurement.MeasuredUnits, EstimatedCredits: calculation, Unit: string(measurement.Unit), UsageKind: usageKind}
	return true, c.requestEntitlement(ctx, pending)
}

func (c *DocumentMeasurementCoordinator) finishFailure(ctx context.Context, claim DocumentMeasurementClaim, outcomeID ID, code string, transient bool, cause error) error {
	at := c.clock.Now().UTC()
	if at.IsZero() {
		return safe("document_measurement_unavailable", cause)
	}
	var err error
	if transient && claim.MeasurementAttempts < DocumentMeasurementMaxAttempts {
		backoff := DocumentMeasurementFirstBackoff
		if claim.MeasurementAttempts == 2 {
			backoff = DocumentMeasurementSecondBackoff
		}
		err = c.store.RetryDocumentMeasurement(ctx, claim, outcomeID, code, at, at.Add(backoff))
	} else {
		if transient {
			code = "MEASUREMENT_RETRY_EXHAUSTED"
		}
		err = c.store.FailDocumentMeasurement(ctx, claim, outcomeID, code, at)
	}
	if err != nil {
		return safe("document_measurement_unavailable", errors.Join(cause, err))
	}
	return safe("document_measurement_unavailable", cause)
}

func measurementFailureDisposition(err error) (string, bool) {
	code, ok := MeasurementFailureCodeOf(err)
	if !ok {
		return "MEASUREMENT_UNKNOWN_FAILURE", false
	}
	return string(code), code == MeasurementParserUnavailable
}

func (c *DocumentMeasurementCoordinator) requestEntitlement(ctx context.Context, pending PendingDocumentReservation) error {
	if !validPendingDocumentReservation(pending) {
		return safe("document_measurement_unavailable", ErrDocumentMeasurementOrchestrationInvalid)
	}
	if err := c.authorizer.AuthorizeDocumentProcessing(ctx, pending.RequestedBy, pending.TenantID, pending.AssignmentGeneration, "tenderpreneur.opportunity.document.upload"); err != nil {
		return safe("document_measurement_not_authorized", err)
	}
	reference, err := c.entitlement.RequestDocumentEntitlement(ctx, EntitlementReservationRequest{MeasurementID: pending.MeasurementID, JobID: pending.JobID, TenantID: pending.TenantID, RequestedBy: pending.RequestedBy, CorrelationID: pending.CorrelationID, AssignmentGeneration: pending.AssignmentGeneration, EstimatedCredits: pending.EstimatedCredits, UsageKind: pending.UsageKind})
	if err != nil || reference.CheckpointID.IsZero() {
		return safe("document_measurement_unavailable", err)
	}
	auditID, err := c.ids.NewID()
	if err != nil || auditID.IsZero() {
		return safe("document_measurement_unavailable", err)
	}
	if err = c.store.LinkDocumentMeasurementEntitlement(ctx, pending, reference.CheckpointID, auditID, c.clock.Now().UTC()); err != nil {
		return safe("document_measurement_unavailable", err)
	}
	return nil
}

func (c *DocumentMeasurementCoordinator) newIDs(count int) ([]ID, error) {
	ids := make([]ID, count)
	for index := range ids {
		id, err := c.ids.NewID()
		if err != nil || id.IsZero() {
			return nil, errors.New("document_measurement_id_unavailable")
		}
		ids[index] = id
	}
	return ids, nil
}

func validPendingDocumentReservation(value PendingDocumentReservation) bool {
	return !value.MeasurementID.IsZero() && !value.JobID.IsZero() && !value.DocumentVersionID.IsZero() && !value.RequestedBy.IsZero() && !value.TenantID.IsZero() && !value.CorrelationID.IsZero() && value.AssignmentGeneration > 0 && value.MeasuredUnits > 0 && value.EstimatedCredits > 0 && (value.Unit == string(PDFPages) && value.UsageKind == "DOCUMENT_PDF" || value.Unit == string(DOCXExtractedCharacters) && value.UsageKind == "DOCUMENT_DOCX")
}
