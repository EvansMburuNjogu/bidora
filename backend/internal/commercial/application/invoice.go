package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"bidora/backend/internal/commercial/domain"
)

var ErrInvoiceRequestInvalid = errors.New("commercial_invoice_request_invalid")
var ErrInvoiceNotAuthorized = errors.New("commercial_invoice_not_authorized")
var ErrInvoiceConflict = errors.New("commercial_invoice_conflict")
var ErrInvoiceTaxUnavailable = errors.New("commercial_tax_profile_unavailable")
var ErrInvoiceQuoteStale = errors.New("commercial_quote_stale")
var ErrInvoiceNotFound = errors.New("commercial_invoice_not_found")
var ErrInvoiceUnavailable = errors.New("commercial_invoice_unavailable")

type PurchaseRequest struct {
	TenantID, SubscriptionID, OperationID domain.ID
	Kind                                  string
	PlanKey                               domain.PlanKey
	Cadence                               domain.BillingCadence
	Quantity                              int64
}

type PreparedPurchase struct {
	Quote   domain.PurchaseQuote
	Billing domain.BillingSnapshot
}

type InvoiceQuoteProvider interface {
	Prepare(context.Context, SubscriptionAccess, PurchaseRequest, time.Time) (PreparedPurchase, error)
}

type InvoiceCommand struct {
	RequestHash [32]byte
	InvoiceID   domain.ID
	Version     int64
}

type InvoiceSubscription struct {
	ID, TenantID domain.ID
	Version      int64
}

type InvoiceEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TenantID, InvoiceID domain.ID
	InvoiceNumber, State, PurchaseKind                             string
	Version, TotalMinorUnits                                       int64
	OccurredAt                                                     time.Time
}

type InvoiceTx interface {
	Authorize(context.Context, SubscriptionAccess, string) (bool, error)
	GetInvoiceCommand(context.Context, domain.ID, [32]byte) (InvoiceCommand, bool, error)
	GetInvoice(context.Context, domain.ID, domain.ID) (domain.Invoice, bool, error)
	LockInvoiceSubscription(context.Context, domain.ID, domain.ID) (InvoiceSubscription, bool, error)
	LockActiveTaxProfile(context.Context, time.Time) (domain.TaxProfile, bool, error)
	LockInvoiceNumber(context.Context, int, time.Time) (int64, error)
	InsertInvoice(context.Context, domain.Invoice, domain.ID, [32]byte, [32]byte) error
	AppendInvoiceEvidence(context.Context, InvoiceEvidence) error
}

type InvoiceUnitOfWork interface {
	WithinInvoice(context.Context, func(context.Context, InvoiceTx) error) error
}

type IssueInvoiceInput struct {
	Access                      SubscriptionAccess
	Purchase                    PurchaseRequest
	ExpectedSubscriptionVersion int64
	IdempotencyKey              string
	CorrelationID               domain.ID
}

type InvoiceIDGenerator interface{ NewID() (domain.ID, error) }
type InvoiceClock interface{ Now() time.Time }

type InvoiceReadService struct{ uow InvoiceUnitOfWork }

func NewInvoiceReadService(uow InvoiceUnitOfWork) (*InvoiceReadService, error) {
	if uow == nil {
		return nil, errors.New("commercial_invoice_read_service_invalid")
	}
	return &InvoiceReadService{uow: uow}, nil
}

func (s *InvoiceReadService) Read(ctx context.Context, access SubscriptionAccess, invoiceID domain.ID) (result domain.Invoice, err error) {
	if ctx == nil || invalidSubscriptionAccess(ctx, access) || invoiceID.IsZero() {
		return result, ErrInvoiceRequestInvalid
	}
	err = s.uow.WithinInvoice(ctx, func(txCtx context.Context, tx InvoiceTx) error {
		allowed, e := tx.Authorize(txCtx, access, "tenderpreneur.organization.subscription.read")
		if e != nil {
			return e
		}
		if !allowed {
			return ErrInvoiceNotAuthorized
		}
		invoice, found, e := tx.GetInvoice(txCtx, access.TenantID, invoiceID)
		if e != nil {
			return e
		}
		if !found || invoice.TenantID != access.TenantID {
			return ErrInvoiceNotFound
		}
		result = invoice
		return nil
	})
	if err != nil {
		return domain.Invoice{}, mapInvoiceError(err)
	}
	return result, nil
}

type InvoiceService struct {
	uow    InvoiceUnitOfWork
	quotes InvoiceQuoteProvider
	ids    InvoiceIDGenerator
	clock  InvoiceClock
	zone   *time.Location
}

func NewInvoiceService(uow InvoiceUnitOfWork, quotes InvoiceQuoteProvider, ids InvoiceIDGenerator, clock InvoiceClock) (*InvoiceService, error) {
	if uow == nil || quotes == nil || ids == nil || clock == nil {
		return nil, errors.New("commercial_invoice_service_invalid")
	}
	zone, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		return nil, errors.New("commercial_invoice_timezone_unavailable")
	}
	return &InvoiceService{uow: uow, quotes: quotes, ids: ids, clock: clock, zone: zone}, nil
}

func (s *InvoiceService) Read(ctx context.Context, access SubscriptionAccess, invoiceID domain.ID) (domain.Invoice, error) {
	reader, err := NewInvoiceReadService(s.uow)
	if err != nil {
		return domain.Invoice{}, err
	}
	return reader.Read(ctx, access, invoiceID)
}

func (s *InvoiceService) Issue(ctx context.Context, in IssueInvoiceInput) (result domain.Invoice, err error) {
	if ctx == nil || invalidSubscriptionAccess(ctx, in.Access) || in.Purchase.TenantID != in.Access.TenantID || in.Purchase.SubscriptionID.IsZero() || in.Purchase.OperationID.IsZero() || in.Purchase.Quantity < 1 || in.ExpectedSubscriptionVersion < 1 || in.IdempotencyKey == "" || in.CorrelationID.IsZero() {
		return result, ErrInvoiceRequestInvalid
	}
	requestHash := invoiceFingerprint("issue:v1", in.Purchase, in.ExpectedSubscriptionVersion)
	keyHash := sha256.Sum256([]byte("commercial-invoice-issue:v1:" + in.IdempotencyKey))
	replayed := false
	err = s.uow.WithinInvoice(ctx, func(txCtx context.Context, tx InvoiceTx) error {
		allowed, e := tx.Authorize(txCtx, in.Access, "tenderpreneur.organization.subscription.manage")
		if e != nil {
			return e
		}
		if !allowed {
			return ErrInvoiceNotAuthorized
		}
		command, found, e := tx.GetInvoiceCommand(txCtx, in.Access.ActorID, keyHash)
		if e != nil || !found {
			return e
		}
		if command.RequestHash != requestHash {
			return ErrInvoiceConflict
		}
		existing, ok, e := tx.GetInvoice(txCtx, in.Access.TenantID, command.InvoiceID)
		if e != nil || !ok || existing.Version != command.Version || existing.TenantID != in.Access.TenantID {
			return ErrInvoiceConflict
		}
		result, replayed = existing, true
		return nil
	})
	if err != nil {
		return domain.Invoice{}, mapInvoiceError(err)
	}
	if replayed {
		return result, nil
	}
	preparedAt := s.clock.Now().UTC()
	if preparedAt.IsZero() {
		return result, ErrInvoiceUnavailable
	}
	prepared, err := s.quotes.Prepare(ctx, in.Access, in.Purchase, preparedAt)
	if err != nil {
		return result, ErrInvoiceUnavailable
	}
	if prepared.Quote.OperationID != in.Purchase.OperationID || prepared.Quote.Kind != in.Purchase.Kind || prepared.Quote.PlanKey != in.Purchase.PlanKey || prepared.Quote.Cadence != in.Purchase.Cadence || prepared.Quote.Quantity != in.Purchase.Quantity || prepared.Quote.Entitlement.ExpectedSubscriptionVersion != in.ExpectedSubscriptionVersion || !prepared.Quote.Valid() || !prepared.Billing.Valid() {
		return result, ErrInvoiceRequestInvalid
	}
	issuedAt := s.clock.Now().UTC()
	if issuedAt.IsZero() || issuedAt.After(prepared.Quote.ExpiresAt) {
		return result, ErrInvoiceQuoteStale
	}
	err = s.uow.WithinInvoice(ctx, func(txCtx context.Context, tx InvoiceTx) error {
		allowed, e := tx.Authorize(txCtx, in.Access, "tenderpreneur.organization.subscription.manage")
		if e != nil {
			return e
		}
		if !allowed {
			return ErrInvoiceNotAuthorized
		}
		subscription, found, e := tx.LockInvoiceSubscription(txCtx, in.Access.TenantID, in.Purchase.SubscriptionID)
		if e != nil {
			return e
		}
		if !found {
			return ErrInvoiceConflict
		}
		command, commandFound, e := tx.GetInvoiceCommand(txCtx, in.Access.ActorID, keyHash)
		if e != nil {
			return e
		}
		if commandFound {
			if command.RequestHash != requestHash {
				return ErrInvoiceConflict
			}
			existing, ok, e := tx.GetInvoice(txCtx, in.Access.TenantID, command.InvoiceID)
			if e != nil || !ok || existing.Version != command.Version || existing.TenantID != in.Access.TenantID {
				return ErrInvoiceConflict
			}
			result = existing
			return nil
		}
		if subscription.Version != in.ExpectedSubscriptionVersion {
			return ErrInvoiceConflict
		}
		tax, found, e := tx.LockActiveTaxProfile(txCtx, issuedAt)
		if e != nil {
			return e
		}
		if !found || !tax.ActiveAt(issuedAt) {
			return ErrInvoiceTaxUnavailable
		}
		sequence, e := tx.LockInvoiceNumber(txCtx, issuedAt.In(s.zone).Year(), issuedAt)
		if e != nil {
			return e
		}
		number, e := domain.FormatFinancialNumber("INVOICE", issuedAt.In(s.zone).Year(), sequence)
		if e != nil {
			return ErrInvoiceUnavailable
		}
		ids, e := invoiceIDs(s.ids, 6)
		if e != nil {
			return e
		}
		result, e = domain.NewIssuedInvoice(ids[1], in.Access.TenantID, subscription.ID, in.Access.ActorID, number, prepared.Quote, tax, prepared.Billing, issuedAt, issuedAt, ids[2], ids[3])
		if e != nil {
			return ErrInvoiceRequestInvalid
		}
		if e = tx.InsertInvoice(txCtx, result, ids[0], keyHash, requestHash); e != nil {
			return e
		}
		return tx.AppendInvoiceEvidence(txCtx, InvoiceEvidence{AuditID: ids[4], OutboxID: ids[5], CorrelationID: in.CorrelationID, ActorID: in.Access.ActorID, TenantID: in.Access.TenantID, InvoiceID: result.ID, InvoiceNumber: result.Number, State: result.State, PurchaseKind: result.PurchaseKind, Version: result.Version, TotalMinorUnits: result.TotalMinorUnits, OccurredAt: issuedAt})
	})
	if err != nil {
		return domain.Invoice{}, mapInvoiceError(err)
	}
	return result, nil
}

func invoiceFingerprint(values ...any) [32]byte {
	encoded, _ := json.Marshal(values)
	return sha256.Sum256(encoded)
}
func invoiceIDs(generator InvoiceIDGenerator, count int) ([]domain.ID, error) {
	values := make([]domain.ID, count)
	for i := range values {
		value, err := generator.NewID()
		if err != nil || value.IsZero() {
			return nil, ErrInvoiceUnavailable
		}
		values[i] = value
	}
	return values, nil
}
func mapInvoiceError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrInvoiceRequestInvalid), errors.Is(err, ErrInvoiceNotAuthorized), errors.Is(err, ErrInvoiceConflict), errors.Is(err, ErrInvoiceTaxUnavailable), errors.Is(err, ErrInvoiceQuoteStale), errors.Is(err, ErrInvoiceNotFound):
		return err
	default:
		return ErrInvoiceUnavailable
	}
}
