package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/url"
	"time"

	"bidora/backend/internal/commercial/domain"
	"github.com/google/uuid"
)

var ErrPaymentRequestInvalid = errors.New("commercial_payment_request_invalid")
var ErrPaymentNotAuthorized = errors.New("commercial_payment_not_authorized")
var ErrPaymentNotFound = errors.New("commercial_payment_not_found")
var ErrPaymentConflict = errors.New("commercial_payment_conflict")
var ErrPaymentInProgress = errors.New("commercial_payment_in_progress")
var ErrPaymentReconciliationRequired = errors.New("commercial_payment_reconciliation_required")
var ErrPaymentAmountConflict = errors.New("commercial_payment_amount_conflict")
var ErrPaymentUnavailable = errors.New("commercial_payment_unavailable")

type PaymentProviderConfiguration struct {
	ID                     domain.ID
	Provider               string
	AllowedRedirectOrigin  string
	CreateUncertaintyDelay time.Duration
	CreateTimeout          time.Duration
	QueryTimeout           time.Duration
	MaxQueryAttempts       int32
	QueryRetryDelays       []time.Duration
}

func (c PaymentProviderConfiguration) Valid() bool {
	origin, err := url.Parse(c.AllowedRedirectOrigin)
	if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || (origin.Path != "" && origin.Path != "/") || origin.RawQuery != "" || origin.Fragment != "" || c.ID.IsZero() || c.Provider != "PESAPAL" || c.CreateUncertaintyDelay < time.Minute || c.CreateUncertaintyDelay > 15*time.Minute || c.CreateTimeout < time.Second || c.CreateTimeout > 90*time.Second || c.QueryTimeout < time.Second || c.QueryTimeout > 90*time.Second || c.MaxQueryAttempts < 1 || c.MaxQueryAttempts > 9 || len(c.QueryRetryDelays) != int(c.MaxQueryAttempts-1) {
		return false
	}
	previous := time.Duration(0)
	for _, delay := range c.QueryRetryDelays {
		if delay <= previous || delay > 24*time.Hour {
			return false
		}
		previous = delay
	}
	return true
}

type PaymentConfigurationProvider interface {
	Current(context.Context, domain.ID) (PaymentProviderConfiguration, error)
	Version(context.Context, domain.ID) (PaymentProviderConfiguration, error)
}

// PaymentPayerValidator lets the real adapter reject missing/invalid billing
// contact before any payment intent is recorded. Test/provider implementations
// without a contact requirement retain their existing command contract.
type PaymentPayerValidator interface{ ValidatePayer(string, string) bool }

type PaymentCreateRequest struct {
	PayerEmail, PayerCountry                                                    string
	PaymentID, InvoiceID, TenantID, MerchantOperationID, ConfigurationVersionID domain.ID
	Currency                                                                    string
	AmountMinorUnits                                                            int64
}

type PaymentQueryRequest struct {
	PaymentID, TenantID, MerchantOperationID, ConfigurationVersionID domain.ID
	ProviderOrderReference                                           *string
	Currency                                                         string
	AmountMinorUnits                                                 int64
}

type PaymentProvider interface {
	CreateOrder(context.Context, PaymentProviderConfiguration, PaymentCreateRequest) (domain.PaymentCreateResult, error)
	QueryOrder(context.Context, PaymentProviderConfiguration, PaymentQueryRequest) (domain.PaymentQueryResult, error)
}

type PaymentCommand struct {
	RequestHash [32]byte
	PaymentID   domain.ID
	Version     int64
}

type PaymentInvoice struct {
	ID, TenantID                   domain.ID
	State, Currency                string
	Version, OutstandingMinorUnits int64
}

type PaymentEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TenantID, InvoiceID, PaymentID domain.ID
	EventType, State, ReconciliationState                                     string
	Version, AmountMinorUnits                                                 int64
	OccurredAt                                                                time.Time
}

type PaymentObservation struct {
	ID, PaymentID                              domain.ID
	Source, ProviderStateCode, NormalizedState string
	AmountMinorUnits                           *int64
	Currency                                   *string
	RawEvidenceHash                            [32]byte
	ProviderObservedAt                         *time.Time
	RecordedAt                                 time.Time
}

type PaymentClaim struct {
	ClaimID, LeaseID          domain.ID
	Payment                   domain.Payment
	StartedAt, LeaseExpiresAt time.Time
}

type PaymentTx interface {
	Authorize(context.Context, SubscriptionAccess, string) (bool, error)
	GetPaymentCommand(context.Context, domain.ID, [32]byte) (PaymentCommand, bool, error)
	GetPayment(context.Context, domain.ID, domain.ID) (domain.Payment, bool, error)
	LockPaymentInvoice(context.Context, domain.ID, domain.ID) (PaymentInvoice, bool, error)
	GetActivePayment(context.Context, domain.ID) (domain.Payment, bool, error)
	InsertPaymentIntent(context.Context, domain.Payment, domain.ID, domain.ID, [32]byte, [32]byte, int64, PaymentEvidence) error
	ApplyPaymentCreateResult(context.Context, domain.Payment, domain.PaymentCreateResult, *PaymentObservation, PaymentEvidence) error
	ClaimPaymentReconciliation(context.Context, domain.ID, domain.ID) (PaymentClaim, bool, error)
	CompletePaymentReconciliation(context.Context, PaymentClaim, string, string, *string, *string, *time.Time, *PaymentObservation, PaymentEvidence) error
	LockPaymentReversalCandidate(context.Context, domain.ID) (domain.ReversalCandidate, bool, error)
	CommitPaymentReversal(context.Context, domain.ReversalDecision, domain.ID, ReversalEvidence) error
}

type PaymentUnitOfWork interface {
	WithinPayment(context.Context, func(context.Context, PaymentTx) error) error
}

type InitiatePaymentInput struct {
	PayerEmail, PayerCountry string
	Access                   SubscriptionAccess
	InvoiceID                domain.ID
	ExpectedInvoiceVersion   int64
	IdempotencyKey           string
	CorrelationID            domain.ID
}

type PaymentInitiationResult struct {
	Payment           domain.Payment
	RedirectURL       string
	RedirectExpiresAt *time.Time
}

type PaymentService struct {
	uow      PaymentUnitOfWork
	config   PaymentConfigurationProvider
	provider PaymentProvider
	ids      InvoiceIDGenerator
	clock    InvoiceClock
}

func NewPaymentService(uow PaymentUnitOfWork, config PaymentConfigurationProvider, provider PaymentProvider, ids InvoiceIDGenerator, clock InvoiceClock) (*PaymentService, error) {
	if uow == nil || config == nil || provider == nil || ids == nil || clock == nil {
		return nil, errors.New("commercial_payment_service_invalid")
	}
	return &PaymentService{uow: uow, config: config, provider: provider, ids: ids, clock: clock}, nil
}

func (s *PaymentService) Read(ctx context.Context, access SubscriptionAccess, paymentID domain.ID) (result domain.Payment, err error) {
	if ctx == nil || invalidSubscriptionAccess(ctx, access) || paymentID.IsZero() {
		return result, ErrPaymentRequestInvalid
	}
	err = s.uow.WithinPayment(ctx, func(txCtx context.Context, tx PaymentTx) error {
		allowed, e := tx.Authorize(txCtx, access, "tenderpreneur.organization.subscription.read")
		if e != nil {
			return e
		}
		if !allowed {
			return ErrPaymentNotAuthorized
		}
		payment, found, e := tx.GetPayment(txCtx, access.TenantID, paymentID)
		if e != nil {
			return e
		}
		if !found || payment.TenantID != access.TenantID {
			return ErrPaymentNotFound
		}
		result = payment
		return nil
	})
	if err != nil {
		return domain.Payment{}, mapPaymentError(err)
	}
	return result, nil
}

func (s *PaymentService) Initiate(ctx context.Context, in InitiatePaymentInput) (result PaymentInitiationResult, err error) {
	if _, parseErr := uuid.Parse(in.IdempotencyKey); ctx == nil || invalidSubscriptionAccess(ctx, in.Access) || in.InvoiceID.IsZero() || in.ExpectedInvoiceVersion < 1 || parseErr != nil || in.CorrelationID.IsZero() {
		return result, ErrPaymentRequestInvalid
	}
	if validator, ok := s.provider.(PaymentPayerValidator); ok && !validator.ValidatePayer(in.PayerEmail, in.PayerCountry) {
		return result, ErrPaymentRequestInvalid
	}
	keyHash := sha256.Sum256([]byte("commercial-payment-initiate:v1:" + in.IdempotencyKey))
	requestHash := invoiceFingerprint("payment-initiate:v1", in.Access.TenantID, in.InvoiceID, in.ExpectedInvoiceVersion)
	if _, ok := s.provider.(PaymentPayerValidator); ok {
		requestHash = sha256.Sum256(append(requestHash[:], []byte("\x00"+in.PayerEmail+"\x00"+in.PayerCountry)...))
	}
	err = s.uow.WithinPayment(ctx, func(txCtx context.Context, tx PaymentTx) error {
		allowed, e := tx.Authorize(txCtx, in.Access, "tenderpreneur.organization.subscription.manage")
		if e != nil {
			return e
		}
		if !allowed {
			return ErrPaymentNotAuthorized
		}
		command, found, e := tx.GetPaymentCommand(txCtx, in.Access.ActorID, keyHash)
		if e != nil {
			return e
		}
		if !found {
			return nil
		}
		if command.RequestHash != requestHash {
			return ErrPaymentConflict
		}
		payment, ok, e := tx.GetPayment(txCtx, in.Access.TenantID, command.PaymentID)
		if e != nil || !ok || payment.Version < command.Version {
			return ErrPaymentConflict
		}
		result.Payment = payment
		return nil
	})
	if err != nil {
		return result, mapPaymentError(err)
	}
	if !result.Payment.ID.IsZero() {
		return result, nil
	}
	configuration, err := s.config.Current(ctx, in.Access.TenantID)
	if err != nil || !configuration.Valid() {
		return result, ErrPaymentUnavailable
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrPaymentUnavailable
	}
	ids, e := paymentIDs(s.ids, 7)
	if e != nil {
		return result, e
	}
	intent, e := domain.NewPaymentIntent(ids[1], in.InvoiceID, in.Access.TenantID, ids[2], configuration.ID, ids[3], in.Access.ActorID, 1, at, at.Add(configuration.CreateUncertaintyDelay))
	if e != nil {
		return result, ErrPaymentRequestInvalid
	}
	err = s.uow.WithinPayment(ctx, func(txCtx context.Context, tx PaymentTx) error {
		allowed, x := tx.Authorize(txCtx, in.Access, "tenderpreneur.organization.subscription.manage")
		if x != nil {
			return x
		}
		if !allowed {
			return ErrPaymentNotAuthorized
		}
		invoice, found, x := tx.LockPaymentInvoice(txCtx, in.Access.TenantID, in.InvoiceID)
		if x != nil {
			return x
		}
		if !found {
			return ErrPaymentNotFound
		}
		command, commandFound, x := tx.GetPaymentCommand(txCtx, in.Access.ActorID, keyHash)
		if x != nil {
			return x
		}
		if commandFound {
			if command.RequestHash != requestHash {
				return ErrPaymentConflict
			}
			existing, ok, x := tx.GetPayment(txCtx, in.Access.TenantID, command.PaymentID)
			if x != nil || !ok {
				return ErrPaymentConflict
			}
			result.Payment = existing
			return nil
		}
		active, activeFound, x := tx.GetActivePayment(txCtx, in.InvoiceID)
		if x != nil {
			return x
		}
		if activeFound {
			result.Payment = active
			return ErrPaymentInProgress
		}
		if invoice.Version != in.ExpectedInvoiceVersion || (invoice.State != "ISSUED" && invoice.State != "PAYMENT_PENDING") || invoice.Currency != "KES" || invoice.OutstandingMinorUnits < 1 {
			return ErrPaymentConflict
		}
		intent.AmountMinorUnits = invoice.OutstandingMinorUnits
		evidence := PaymentEvidence{AuditID: ids[4], OutboxID: ids[5], CorrelationID: in.CorrelationID, ActorID: in.Access.ActorID, TenantID: in.Access.TenantID, InvoiceID: in.InvoiceID, PaymentID: intent.ID, EventType: "commercial.payment.create_requested.v1", State: intent.State, ReconciliationState: intent.ReconciliationState, Version: intent.Version, AmountMinorUnits: intent.AmountMinorUnits, OccurredAt: at}
		if x = tx.InsertPaymentIntent(txCtx, intent, in.Access.ActorID, ids[0], keyHash, requestHash, in.ExpectedInvoiceVersion, evidence); x != nil {
			return x
		}
		result.Payment = intent
		return nil
	})
	if err != nil {
		return result, mapPaymentError(err)
	}
	if result.Payment.ID != intent.ID {
		return result, nil
	}
	createCtx, cancelCreate := context.WithTimeout(ctx, configuration.CreateTimeout)
	createResult, providerErr := s.provider.CreateOrder(createCtx, configuration, PaymentCreateRequest{PayerEmail: in.PayerEmail, PayerCountry: in.PayerCountry, PaymentID: intent.ID, InvoiceID: intent.InvoiceID, TenantID: intent.TenantID, MerchantOperationID: intent.MerchantOperationID, ConfigurationVersionID: intent.ProviderConfigurationVersionID, Currency: intent.Currency, AmountMinorUnits: intent.AmountMinorUnits})
	cancelCreate()
	completedAt := s.clock.Now().UTC()
	if completedAt.IsZero() {
		return result, ErrPaymentUnavailable
	}
	if providerErr != nil || !createResult.Valid(configuration.AllowedRedirectOrigin) {
		createResult = domain.PaymentCreateResult{Disposition: domain.PaymentCreateAmbiguous}
	}
	applyIDs, e := paymentIDs(s.ids, 3)
	if e != nil {
		return result, e
	}
	var observation *PaymentObservation
	if createResult.Disposition != domain.PaymentCreateAmbiguous {
		observation = &PaymentObservation{ID: applyIDs[0], PaymentID: intent.ID, Source: "CREATE_RESPONSE", ProviderStateCode: createResult.ProviderStateCode, NormalizedState: mapCreateNormalized(createResult.Disposition), RawEvidenceHash: createResult.RawEvidenceHash, ProviderObservedAt: createResult.ProviderObservedAt, RecordedAt: completedAt}
		amount, currency := intent.AmountMinorUnits, intent.Currency
		observation.AmountMinorUnits = &amount
		observation.Currency = &currency
	}
	err = s.uow.WithinPayment(ctx, func(txCtx context.Context, tx PaymentTx) error {
		current, found, x := tx.GetPayment(txCtx, intent.TenantID, intent.ID)
		if x != nil {
			return x
		}
		if !found {
			return ErrPaymentNotFound
		}
		state, reconciliationState := current.State, current.ReconciliationState
		switch createResult.Disposition {
		case domain.PaymentOrderCreated:
			state, reconciliationState = "AWAITING_CUSTOMER", "COMPLETE"
		case domain.PaymentCreateNoEffect:
			state, reconciliationState = "FAILED", "COMPLETE"
		case domain.PaymentCreateAmbiguous:
			reconciliationState = "DUE"
		}
		evidence := PaymentEvidence{AuditID: applyIDs[1], OutboxID: applyIDs[2], CorrelationID: in.CorrelationID, ActorID: in.Access.ActorID, TenantID: intent.TenantID, InvoiceID: intent.InvoiceID, PaymentID: intent.ID, EventType: "commercial.payment.create_observed.v1", State: state, ReconciliationState: reconciliationState, Version: current.Version + 1, AmountMinorUnits: intent.AmountMinorUnits, OccurredAt: completedAt}
		return tx.ApplyPaymentCreateResult(txCtx, current, createResult, observation, evidence)
	})
	if err != nil {
		return result, mapPaymentError(err)
	}
	result.Payment.Version++
	switch createResult.Disposition {
	case domain.PaymentOrderCreated:
		result.Payment.State = "AWAITING_CUSTOMER"
		result.Payment.ReconciliationState = "COMPLETE"
		ref := createResult.ProviderOrderReference
		result.Payment.ProviderOrderReference = &ref
		if createResult.RedirectExpiresAt != nil && completedAt.Before(*createResult.RedirectExpiresAt) {
			result.RedirectURL = createResult.RedirectURL
			result.RedirectExpiresAt = createResult.RedirectExpiresAt
		}
	case domain.PaymentCreateNoEffect:
		result.Payment.State = "FAILED"
		result.Payment.ReconciliationState = "COMPLETE"
	case domain.PaymentCreateAmbiguous:
		result.Payment.ReconciliationState = "DUE"
	}
	return result, nil
}

func (s *PaymentService) ReconcileNext(ctx context.Context, executionID domain.ID) (bool, error) {
	if ctx == nil || executionID.IsZero() {
		return false, ErrPaymentRequestInvalid
	}
	ids, e := paymentIDs(s.ids, 7)
	if e != nil {
		return false, e
	}
	var claim PaymentClaim
	err := s.uow.WithinPayment(ctx, func(txCtx context.Context, tx PaymentTx) error {
		var found bool
		var x error
		claim, found, x = tx.ClaimPaymentReconciliation(txCtx, ids[0], ids[1])
		if x != nil {
			return x
		}
		if !found {
			return nil
		}
		return nil
	})
	if err != nil {
		return false, mapPaymentError(err)
	}
	if claim.Payment.ID.IsZero() {
		return false, nil
	}
	configuration, e := s.config.Version(ctx, claim.Payment.ProviderConfigurationVersionID)
	if e != nil || !configuration.Valid() {
		return true, s.completeRetry(ctx, claim, ids, executionID, "INTEGRITY_CONFLICT", nil, nil)
	}
	queryCtx, cancelQuery := context.WithTimeout(ctx, configuration.QueryTimeout)
	query, providerErr := s.provider.QueryOrder(queryCtx, configuration, PaymentQueryRequest{PaymentID: claim.Payment.ID, TenantID: claim.Payment.TenantID, MerchantOperationID: claim.Payment.MerchantOperationID, ConfigurationVersionID: claim.Payment.ProviderConfigurationVersionID, ProviderOrderReference: claim.Payment.ProviderOrderReference, Currency: claim.Payment.Currency, AmountMinorUnits: claim.Payment.AmountMinorUnits})
	cancelQuery()
	if ctx.Err() != nil {
		return true, ctx.Err()
	}
	if providerErr != nil {
		return true, s.completeRetry(ctx, claim, ids, executionID, "RETRYABLE_OR_AMBIGUOUS", nil, &configuration)
	}
	if !query.Valid(claim.Payment) {
		query = domain.PaymentQueryResult{Disposition: domain.PaymentIntegrityConflict, ProviderStateCode: "INVALID_PROVIDER_RESULT", NormalizedState: "UNKNOWN", RawEvidenceHash: query.RawEvidenceHash}
	}
	return true, s.completeQuery(ctx, claim, ids, executionID, query, configuration)
}

func (s *PaymentService) completeRetry(ctx context.Context, claim PaymentClaim, ids []domain.ID, executionID domain.ID, outcome string, observation *PaymentObservation, configuration *PaymentProviderConfiguration) error {
	now := s.clock.Now().UTC()
	if now.IsZero() {
		return ErrPaymentUnavailable
	}
	state, reason := "RETRY_WAIT", "CREATE_AMBIGUOUS"
	var due *time.Time
	if outcome == "INTEGRITY_CONFLICT" {
		state, reason = "HELD", "INTEGRITY_CONFLICT"
	} else if configuration == nil || claim.Payment.ReconciliationAttempts >= configuration.MaxQueryAttempts {
		state, reason = "HELD", "DELIVERY_EXHAUSTED"
	} else {
		value := now.Add(configuration.QueryRetryDelays[claim.Payment.ReconciliationAttempts-1])
		due = &value
	}
	evidence := PaymentEvidence{AuditID: ids[2], OutboxID: ids[3], CorrelationID: claim.ClaimID, ActorID: executionID, TenantID: claim.Payment.TenantID, InvoiceID: claim.Payment.InvoiceID, PaymentID: claim.Payment.ID, EventType: "commercial.payment.reconciliation_deferred.v1", State: claim.Payment.State, ReconciliationState: state, Version: claim.Payment.Version + 1, AmountMinorUnits: claim.Payment.AmountMinorUnits, OccurredAt: now}
	return s.uow.WithinPayment(ctx, func(txCtx context.Context, tx PaymentTx) error {
		return tx.CompletePaymentReconciliation(txCtx, claim, outcome, state, &reason, nil, due, observation, evidence)
	})
}

func (s *PaymentService) completeQuery(ctx context.Context, claim PaymentClaim, ids []domain.ID, executionID domain.ID, result domain.PaymentQueryResult, configuration PaymentProviderConfiguration) error {
	now := s.clock.Now().UTC()
	if now.IsZero() {
		return ErrPaymentUnavailable
	}
	paymentState, reconciliationState, reason, outcome := claim.Payment.State, "COMPLETE", "", string(result.Disposition)
	switch result.Disposition {
	case domain.PaymentOrderAbsent:
		paymentState = "FAILED"
	case domain.PaymentProviderPending:
		paymentState = "PROVIDER_PENDING"
		reconciliationState = "RETRY_WAIT"
		reason = "PROVIDER_PENDING"
	case domain.PaymentTerminalFailed:
		paymentState = "FAILED"
	case domain.PaymentTerminalCancelled:
		paymentState = "CANCELLED"
	case domain.PaymentAccountingRequired:
		if result.NormalizedState == "SETTLED" && claim.Payment.State == "SETTLED" {
			paymentState = "SETTLED"
			outcome = "SETTLEMENT_CONFIRMED"
		} else {
			paymentState = "REVIEW_REQUIRED"
			reconciliationState = "HELD"
			reason = "ACCOUNTING_COMPLETION_REQUIRED"
		}
	case domain.PaymentIntegrityConflict:
		paymentState = "REVIEW_REQUIRED"
		reconciliationState = "HELD"
		reason = "INTEGRITY_CONFLICT"
	}
	var due *time.Time
	if reconciliationState == "RETRY_WAIT" {
		if claim.Payment.ReconciliationAttempts >= configuration.MaxQueryAttempts {
			reconciliationState = "HELD"
			reason = "DELIVERY_EXHAUSTED"
			outcome = "DELIVERY_EXHAUSTED"
		} else {
			value := now.Add(configuration.QueryRetryDelays[claim.Payment.ReconciliationAttempts-1])
			due = &value
		}
	}
	var observation *PaymentObservation
	if result.RawEvidenceHash != [32]byte{} {
		observation = &PaymentObservation{ID: ids[0], PaymentID: claim.Payment.ID, Source: "STATUS_QUERY", ProviderStateCode: result.ProviderStateCode, NormalizedState: result.NormalizedState, AmountMinorUnits: result.AmountMinorUnits, Currency: result.Currency, RawEvidenceHash: result.RawEvidenceHash, ProviderObservedAt: result.ProviderObservedAt, RecordedAt: now}
	}
	evidence := PaymentEvidence{AuditID: ids[2], OutboxID: ids[3], CorrelationID: claim.ClaimID, ActorID: executionID, TenantID: claim.Payment.TenantID, InvoiceID: claim.Payment.InvoiceID, PaymentID: claim.Payment.ID, EventType: "commercial.payment.reconciled.v1", State: paymentState, ReconciliationState: reconciliationState, Version: claim.Payment.Version + 1, AmountMinorUnits: claim.Payment.AmountMinorUnits, OccurredAt: now}
	return s.uow.WithinPayment(ctx, func(txCtx context.Context, tx PaymentTx) error {
		if err := tx.CompletePaymentReconciliation(txCtx, claim, outcome, reconciliationState, optionalString(reason), optionalString(result.ProviderOrderReference), due, observation, evidence); err != nil {
			return err
		}
		if result.NormalizedState != "REVERSED" || claim.Payment.State != "SETTLED" {
			return nil
		}
		candidate, ready, err := tx.LockPaymentReversalCandidate(txCtx, claim.Payment.ID)
		if err != nil {
			return err
		}
		if !ready {
			return ErrPaymentConflict
		}
		decision, err := domain.DecidePaymentReversal(candidate, now)
		if err != nil {
			return ErrPaymentConflict
		}
		reversalEvidence := ReversalEvidence{AuditID: ids[5], OutboxID: ids[6], CorrelationID: claim.ClaimID, ActorID: executionID, TenantID: decision.TenantID, InvoiceID: decision.InvoiceID, PaymentID: decision.PaymentID, SubscriptionID: decision.SubscriptionID, ObservationID: decision.ReversalObservationID, AccessHoldID: ids[4], EventType: "commercial.payment.reversed.v1", PaymentState: "REVERSED", InvoiceState: "REVIEW_REQUIRED", SubscriptionState: "SUSPENDED", AccessMode: decision.AccessMode, PaymentVersion: decision.ResultingPaymentVersion, InvoiceVersion: decision.ResultingInvoiceVersion, SubscriptionVersion: decision.ResultingSubscriptionVersion, AmountMinorUnits: decision.AmountMinorUnits, OccurredAt: now}
		return tx.CommitPaymentReversal(txCtx, decision, ids[4], reversalEvidence)
	})
}

func mapCreateNormalized(disposition domain.PaymentCreateDisposition) string {
	if disposition == domain.PaymentOrderCreated {
		return "PENDING"
	}
	return "FAILED"
}
func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func paymentIDs(generator InvoiceIDGenerator, count int) ([]domain.ID, error) {
	values := make([]domain.ID, count)
	for i := range values {
		value, err := generator.NewID()
		if err != nil || value.IsZero() {
			return nil, ErrPaymentUnavailable
		}
		values[i] = value
	}
	return values, nil
}
func mapPaymentError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrPaymentRequestInvalid), errors.Is(err, ErrPaymentNotAuthorized), errors.Is(err, ErrPaymentNotFound), errors.Is(err, ErrPaymentConflict), errors.Is(err, ErrPaymentInProgress), errors.Is(err, ErrPaymentReconciliationRequired), errors.Is(err, ErrPaymentAmountConflict):
		return err
	default:
		return ErrPaymentUnavailable
	}
}
