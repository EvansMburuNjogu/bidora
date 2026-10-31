package application

import (
	"context"
	"errors"
	"time"

	"bidora/backend/internal/commercial/domain"
)

var ErrSettlementRequestInvalid = errors.New("commercial_settlement_request_invalid")
var ErrSettlementNotReady = errors.New("commercial_settlement_not_ready")
var ErrSettlementConflict = errors.New("commercial_settlement_conflict")
var ErrSettlementUnavailable = errors.New("commercial_settlement_unavailable")

type LicenceEntitlement struct {
	SubscriptionID, TermID                                   domain.ID
	SubscriptionVersion, IncludedLicences, EffectiveLicences int64
}

type SettlementStored struct {
	PaymentID, InvoiceID, ObservationID, SubscriptionID, ResultingTermID, GrantID domain.ID
	Receipt                                                                       domain.Receipt
	EffectKind                                                                    string
	PriorSubscriptionVersion, ResultingSubscriptionVersion                        int64
	PaymentVersion, InvoiceVersion                                                int64
	SettledAt                                                                     time.Time
	EffectHash                                                                    [32]byte
}

type SettlementCommit struct {
	Candidate                                   domain.SettlementCandidate
	Decision                                    domain.SettlementDecision
	Receipt                                     domain.Receipt
	TermID, UsagePeriodID, GrantID, ExecutionID domain.ID
	TermVersion                                 int64
	EffectHash                                  [32]byte
	OccurredAt                                  time.Time
}

type SettlementTx interface {
	GetLicenceEntitlement(context.Context, domain.ID, time.Time) (LicenceEntitlement, bool, error)
	GetSettlement(context.Context, domain.ID) (SettlementStored, bool, error)
	LockSettlementCandidate(context.Context, domain.ID) (domain.SettlementCandidate, bool, error)
	LockReceiptNumber(context.Context, int, time.Time) (int64, error)
	NextTermVersion(context.Context, domain.ID) (int64, error)
	CommitSettlement(context.Context, SettlementCommit) error
	AppendSettlementEvidence(context.Context, PaymentEvidence) error
}

type SettlementUnitOfWork interface {
	WithinSettlement(context.Context, func(context.Context, SettlementTx) error) error
}

type CompleteSettlementInput struct {
	PaymentID, ExecutionID, CorrelationID domain.ID
}

type SettlementService struct {
	uow   SettlementUnitOfWork
	ids   InvoiceIDGenerator
	clock InvoiceClock
	zone  *time.Location
}

func NewSettlementService(uow SettlementUnitOfWork, ids InvoiceIDGenerator, clock InvoiceClock) (*SettlementService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, errors.New("commercial_settlement_service_invalid")
	}
	zone, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		return nil, ErrSettlementUnavailable
	}
	return &SettlementService{uow: uow, ids: ids, clock: clock, zone: zone}, nil
}

func (s *SettlementService) ReadLicenceEntitlement(ctx context.Context, tenantID domain.ID, at time.Time) (result LicenceEntitlement, err error) {
	if ctx == nil || tenantID.IsZero() || at.IsZero() {
		return result, ErrSettlementRequestInvalid
	}
	found := false
	err = s.uow.WithinSettlement(ctx, func(txCtx context.Context, tx SettlementTx) error {
		var e error
		result, found, e = tx.GetLicenceEntitlement(txCtx, tenantID, at.UTC())
		return e
	})
	if err != nil {
		return LicenceEntitlement{}, mapSettlementError(err)
	}
	if !found {
		return LicenceEntitlement{}, ErrSettlementNotReady
	}
	return result, nil
}

func (s *SettlementService) Complete(ctx context.Context, input CompleteSettlementInput) (result SettlementStored, err error) {
	if ctx == nil || input.PaymentID.IsZero() || input.ExecutionID.IsZero() || input.CorrelationID.IsZero() {
		return result, ErrSettlementRequestInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrSettlementUnavailable
	}
	ids, e := paymentIDs(s.ids, 6)
	if e != nil {
		return result, e
	}
	err = s.uow.WithinSettlement(ctx, func(txCtx context.Context, tx SettlementTx) error {
		stored, found, x := tx.GetSettlement(txCtx, input.PaymentID)
		if x != nil {
			return x
		}
		if found {
			result = stored
			return nil
		}
		candidate, ready, x := tx.LockSettlementCandidate(txCtx, input.PaymentID)
		if x != nil {
			return x
		}
		if !ready {
			return ErrSettlementNotReady
		}
		decision, x := domain.DecideSettlement(candidate, at)
		if x != nil {
			return ErrSettlementConflict
		}
		sequence, x := tx.LockReceiptNumber(txCtx, at.In(s.zone).Year(), at)
		if x != nil {
			return x
		}
		number, x := domain.FormatFinancialNumber("RECEIPT", at.In(s.zone).Year(), sequence)
		if x != nil {
			return ErrSettlementUnavailable
		}
		receipt, x := domain.NewReceipt(ids[0], candidate.Payment.ID, candidate.Invoice.ID, number, candidate.Payment.AmountMinorUnits, at)
		if x != nil {
			return ErrSettlementConflict
		}
		termID, usageID, grantID := domain.ID{}, domain.ID{}, domain.ID{}
		termVersion := int64(0)
		if decision.EffectKind == "PLAN_ACTIVATION" {
			termID, usageID = ids[1], ids[2]
			termVersion, x = tx.NextTermVersion(txCtx, candidate.Subscription.ID)
			if x != nil {
				return x
			}
			if termVersion < 1 {
				return ErrSettlementConflict
			}
		} else {
			grantID = ids[3]
		}
		effectHash, x := domain.SettlementEffectHash(candidate, decision, receipt, termID, grantID)
		if x != nil {
			return ErrSettlementConflict
		}
		commit := SettlementCommit{Candidate: candidate, Decision: decision, Receipt: receipt, TermID: termID, UsagePeriodID: usageID, GrantID: grantID, ExecutionID: input.ExecutionID, TermVersion: termVersion, EffectHash: effectHash, OccurredAt: at}
		if x = tx.CommitSettlement(txCtx, commit); x != nil {
			return x
		}
		evidence := PaymentEvidence{AuditID: ids[4], OutboxID: ids[5], CorrelationID: input.CorrelationID, ActorID: input.ExecutionID, TenantID: candidate.Payment.TenantID, InvoiceID: candidate.Invoice.ID, PaymentID: candidate.Payment.ID, EventType: "commercial.payment.settled.v1", State: "SETTLED", ReconciliationState: "COMPLETE", Version: candidate.Payment.Version + 1, AmountMinorUnits: candidate.Payment.AmountMinorUnits, OccurredAt: at}
		if x = tx.AppendSettlementEvidence(txCtx, evidence); x != nil {
			return x
		}
		result = SettlementStored{PaymentID: candidate.Payment.ID, InvoiceID: candidate.Invoice.ID, ObservationID: candidate.Observation.ID, SubscriptionID: candidate.Subscription.ID, ResultingTermID: termID, GrantID: grantID, Receipt: receipt, EffectKind: decision.EffectKind, PriorSubscriptionVersion: decision.PriorSubscriptionVersion, ResultingSubscriptionVersion: decision.ResultingSubscriptionVersion, PaymentVersion: candidate.Payment.Version + 1, InvoiceVersion: candidate.Invoice.Version + 1, SettledAt: decision.SettledAt, EffectHash: effectHash}
		return nil
	})
	if err != nil {
		return SettlementStored{}, mapSettlementError(err)
	}
	return result, nil
}

func mapSettlementError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrSettlementRequestInvalid), errors.Is(err, ErrSettlementNotReady), errors.Is(err, ErrSettlementConflict):
		return err
	default:
		return ErrSettlementUnavailable
	}
}
