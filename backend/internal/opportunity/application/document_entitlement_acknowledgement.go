package application

import (
	"context"
	"errors"
	"time"

	entitlementapp "bidora/backend/internal/entitlementcheckpoint/application"
)

var ErrDocumentEntitlementAcknowledgementInvalid = errors.New("document_entitlement_acknowledgement_invalid")
var ErrDocumentEntitlementAcknowledgementConflict = errors.New("document_entitlement_acknowledgement_conflict")

type DocumentEntitlementTarget struct {
	MeasurementID, JobID, DocumentVersionID, RequestedBy ID
	AssignmentGeneration                                 int64
	JobState                                             string
}

type ResponseDraftEntitlementTarget struct {
	RunID, JobID, RequestedBy ID
	AssignmentGeneration      int64
	RunState, JobState        string
}

type DocumentEntitlementDispositionCommit struct {
	AuditID, OutboxID ID
	Target            DocumentEntitlementTarget
	CheckpointID      ID
	Disposition       string
	DenialCode        string
	CorrelationID     ID
	OccurredAt        time.Time
}

type ResponseDraftEntitlementDispositionCommit struct {
	AuditID, OutboxID ID
	Target            ResponseDraftEntitlementTarget
	CheckpointID      ID
	Disposition       string
	DenialCode        string
	CorrelationID     ID
	OccurredAt        time.Time
}

type DocumentEntitlementAcknowledgementTx interface {
	entitlementapp.Tx
	LockDocumentEntitlementTarget(context.Context, ID) (DocumentEntitlementTarget, bool, error)
	ApplyDocumentEntitlementDisposition(context.Context, DocumentEntitlementDispositionCommit) error
	LockResponseDraftEntitlementTarget(context.Context, ID) (ResponseDraftEntitlementTarget, bool, error)
	ApplyResponseDraftEntitlementDisposition(context.Context, ResponseDraftEntitlementDispositionCommit) error
	LockProposalDraftEntitlementTarget(context.Context, ID) (ResponseDraftEntitlementTarget, bool, error)
	ApplyProposalDraftEntitlementDisposition(context.Context, ResponseDraftEntitlementDispositionCommit) error
}

type DocumentEntitlementAcknowledgementUnitOfWork interface {
	WithinDocumentEntitlementAcknowledgement(context.Context, func(context.Context, DocumentEntitlementAcknowledgementTx) error) error
}

type DocumentEntitlementAcknowledgementService struct {
	uow                  DocumentEntitlementAcknowledgementUnitOfWork
	ids                  CaptureIDGenerator
	clock                CaptureClock
	tenantID             ID
	assignmentGeneration int64
}

func NewDocumentEntitlementAcknowledgementService(uow DocumentEntitlementAcknowledgementUnitOfWork, ids CaptureIDGenerator, clock CaptureClock, tenantID ID, assignmentGeneration int64) (*DocumentEntitlementAcknowledgementService, error) {
	if uow == nil || ids == nil || clock == nil || tenantID.IsZero() || assignmentGeneration < 1 {
		return nil, ErrDocumentEntitlementAcknowledgementInvalid
	}
	return &DocumentEntitlementAcknowledgementService{uow: uow, ids: ids, clock: clock, tenantID: tenantID, assignmentGeneration: assignmentGeneration}, nil
}

func (s *DocumentEntitlementAcknowledgementService) Apply(ctx context.Context, acknowledgement entitlementapp.Acknowledgement) (result entitlementapp.Result, err error) {
	if s == nil || ctx == nil || ID(acknowledgement.TenantID) != s.tenantID || acknowledgement.AssignmentGeneration != s.assignmentGeneration {
		return result, ErrDocumentEntitlementAcknowledgementInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrDocumentEntitlementAcknowledgementInvalid
	}
	ids, err := s.newIDs(6)
	if err != nil {
		return result, err
	}
	evidenceIDs := [4]entitlementapp.ID{entitlementapp.ID(ids[0]), entitlementapp.ID(ids[1]), entitlementapp.ID(ids[2]), entitlementapp.ID(ids[3])}
	err = s.uow.WithinDocumentEntitlementAcknowledgement(ctx, func(txCtx context.Context, tx DocumentEntitlementAcknowledgementTx) error {
		var applyErr error
		result, applyErr = entitlementapp.AcknowledgeWithinTransaction(txCtx, tx, acknowledgement, evidenceIDs, at)
		if applyErr != nil {
			return applyErr
		}
		if !acknowledgementMatchesResult(acknowledgement, result.Checkpoint) {
			return ErrDocumentEntitlementAcknowledgementConflict
		}
		if result.Checkpoint.TargetType == "RESPONSE_DRAFT" || result.Checkpoint.TargetType == "PROPOSAL_DRAFT" {
			var target ResponseDraftEntitlementTarget
			var found bool
			var lockErr error
			if result.Checkpoint.TargetType == "RESPONSE_DRAFT" {
				target, found, lockErr = tx.LockResponseDraftEntitlementTarget(txCtx, ID(acknowledgement.CheckpointID))
			} else {
				target, found, lockErr = tx.LockProposalDraftEntitlementTarget(txCtx, ID(acknowledgement.CheckpointID))
			}
			if lockErr != nil {
				return lockErr
			}
			if !found || target.AssignmentGeneration != acknowledgement.AssignmentGeneration || target.RequestedBy != ID(result.Checkpoint.RequesterUserID) {
				return ErrDocumentEntitlementAcknowledgementConflict
			}
			disposition, denialCode, desiredState := "RESERVED", "", "READY_FOR_PROVIDER"
			if acknowledgement.Disposition == entitlementapp.DispositionDenied {
				disposition, denialCode, desiredState = "DENIED", acknowledgement.DenialCode, "FAILED"
			}
			if target.RunState == desiredState && target.JobState == desiredState {
				result.Replay = true
				return nil
			}
			if target.RunState != "PENDING_RESERVATION" || target.JobState != "PENDING_RESERVATION" {
				return ErrDocumentEntitlementAcknowledgementConflict
			}
			commit := ResponseDraftEntitlementDispositionCommit{AuditID: ids[4], OutboxID: ids[5], Target: target, CheckpointID: ID(acknowledgement.CheckpointID), Disposition: disposition, DenialCode: denialCode, CorrelationID: ID(acknowledgement.CorrelationID), OccurredAt: at}
			if result.Checkpoint.TargetType == "RESPONSE_DRAFT" {
				return tx.ApplyResponseDraftEntitlementDisposition(txCtx, commit)
			}
			return tx.ApplyProposalDraftEntitlementDisposition(txCtx, commit)
		}
		if result.Checkpoint.TargetType != "TENDER_ANALYSIS" {
			return ErrDocumentEntitlementAcknowledgementConflict
		}
		target, found, lockErr := tx.LockDocumentEntitlementTarget(txCtx, ID(acknowledgement.CheckpointID))
		if lockErr != nil {
			return lockErr
		}
		if !found || target.AssignmentGeneration != acknowledgement.AssignmentGeneration || target.RequestedBy != ID(result.Checkpoint.RequesterUserID) {
			return ErrDocumentEntitlementAcknowledgementConflict
		}
		disposition, denialCode, desiredState := "RESERVED", "", "READY_FOR_PROVIDER"
		if acknowledgement.Disposition == entitlementapp.DispositionDenied {
			disposition, denialCode, desiredState = "DENIED", acknowledgement.DenialCode, "FAILED"
		}
		if target.JobState == desiredState {
			result.Replay = true
			return nil
		}
		if target.JobState != "PENDING_RESERVATION" {
			return ErrDocumentEntitlementAcknowledgementConflict
		}
		return tx.ApplyDocumentEntitlementDisposition(txCtx, DocumentEntitlementDispositionCommit{AuditID: ids[4], OutboxID: ids[5], Target: target, CheckpointID: ID(acknowledgement.CheckpointID), Disposition: disposition, DenialCode: denialCode, CorrelationID: ID(acknowledgement.CorrelationID), OccurredAt: at})
	})
	if err != nil {
		return entitlementapp.Result{}, err
	}
	return result, nil
}

func acknowledgementMatchesResult(ack entitlementapp.Acknowledgement, checkpoint entitlementapp.Checkpoint) bool {
	if checkpoint.ID != ack.CheckpointID || checkpoint.TenantID != ack.TenantID || checkpoint.AssignmentGeneration != ack.AssignmentGeneration {
		return false
	}
	if ack.Disposition == entitlementapp.DispositionReserved {
		return checkpoint.State == entitlementapp.Reserved && checkpoint.ReservationID == ack.ReservationID && checkpoint.DenialCode == ""
	}
	return ack.Disposition == entitlementapp.DispositionDenied && checkpoint.State == entitlementapp.Denied && checkpoint.ReservationID.IsZero() && checkpoint.DenialCode == ack.DenialCode
}

func (s *DocumentEntitlementAcknowledgementService) newIDs(count int) ([]ID, error) {
	ids := make([]ID, count)
	for index := range ids {
		id, err := s.ids.NewID()
		if err != nil || id.IsZero() {
			return nil, ErrDocumentEntitlementAcknowledgementInvalid
		}
		ids[index] = id
	}
	return ids, nil
}
