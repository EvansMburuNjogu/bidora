// Package application owns permission-preserving dashboard aggregation contracts.
package application

import (
	"context"
	"errors"
	"time"
)

type ID [16]byte

func (id ID) IsZero() bool { return id == ID{} }

var ErrDashboardInvalid = errors.New("dashboard_invalid")
var ErrDashboardNotAuthorized = errors.New("dashboard_not_authorized")
var ErrDashboardUnavailable = errors.New("dashboard_unavailable")

type Clock interface{ Now() time.Time }
type TenderSubject struct {
	ActorID, TenantID ID
	ContextGeneration int64
}
type TenderRequest struct {
	Subject TenderSubject
	Limit   int32
}
type SubscriptionSnapshot struct {
	ID                                                                                          ID
	State, AccessMode, PlanKey, CatalogueVersion, Cadence                                       string
	Version                                                                                     int64
	CommercialPeriodStart, CommercialPeriodEnd                                                  time.Time
	BidWorkspaceLimit, DocumentVersionLimit, AICreditLimit, StorageLimitBytes                   *int64
	IncludedLicenceLimit                                                                        int64
	ConcurrentAIJobLimit, ConcurrentAIJobs                                                      int32
	BidWorkspacesUsed, DocumentVersionsUsed, AICreditsUsed, AICreditsReserved, StorageBytesUsed int64
}
type OpportunityItem struct {
	ID                       ID
	Title, VerificationState string
	BuyerName                *string
	ClosesAt                 *time.Time
	Version                  int64
	UpdatedAt                time.Time
}
type BidItem struct {
	ID, OpportunityID, OwnerID ID
	Title, State               string
	BuyerName                  *string
	ClosesAt                   *time.Time
	Version                    int64
	UpdatedAt                  time.Time
}
type InvoiceItem struct {
	ID                                              ID
	Number, State, PurchaseKind                     string
	Version, TotalMinorUnits, OutstandingMinorUnits int64
	IssuedAt, DueAt                                 time.Time
}
type PaymentItem struct {
	ID, InvoiceID             ID
	State                     string
	Version, AmountMinorUnits int64
	ReconciliationPending     bool
	UpdatedAt                 time.Time
}
type FinancialActivityItem struct {
	ID, InvoiceID, PaymentID    ID
	Kind, DocumentNumber, State string
	AmountMinorUnits            int64
	OccurredAt                  time.Time
}
type OpportunityPage struct {
	Items   []OpportunityItem
	HasMore bool
}
type BidPage struct {
	Items   []BidItem
	HasMore bool
}
type InvoicePage struct {
	Items   []InvoiceItem
	HasMore bool
}
type PaymentPage struct {
	Items   []PaymentItem
	HasMore bool
}
type FinancialActivityPage struct {
	Items   []FinancialActivityItem
	HasMore bool
}
type TenderDashboard struct {
	TenantID          ID
	ObservedAt        time.Time
	Subscription      SubscriptionSnapshot
	Opportunities     OpportunityPage
	Bids              BidPage
	Invoices          InvoicePage
	Payments          PaymentPage
	FinancialActivity FinancialActivityPage
}
type TenderSubscriptionSource interface {
	ReadSubscription(context.Context, TenderSubject) (SubscriptionSnapshot, error)
}
type TenderWorkflowSource interface {
	ReadWorkflow(context.Context, TenderSubject, int32) (OpportunityPage, BidPage, error)
}
type TenderFinancialSource interface {
	ReadFinancial(context.Context, TenderSubject, int32) (InvoicePage, PaymentPage, FinancialActivityPage, error)
}
type TenderDashboardService struct {
	subscriptions TenderSubscriptionSource
	workflow      TenderWorkflowSource
	financial     TenderFinancialSource
	clock         Clock
}

func NewTenderDashboardService(s TenderSubscriptionSource, w TenderWorkflowSource, f TenderFinancialSource, c Clock) (*TenderDashboardService, error) {
	if s == nil || w == nil || f == nil || c == nil {
		return nil, errors.New("tender_dashboard_service_invalid")
	}
	return &TenderDashboardService{s, w, f, c}, nil
}
func (s *TenderDashboardService) Read(ctx context.Context, in TenderRequest) (TenderDashboard, error) {
	if ctx == nil || in.Subject.ActorID.IsZero() || in.Subject.TenantID.IsZero() || in.Subject.ContextGeneration < 1 || in.Limit < 1 || in.Limit > 25 {
		return TenderDashboard{}, ErrDashboardInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return TenderDashboard{}, ErrDashboardUnavailable
	}
	subscription, e := s.subscriptions.ReadSubscription(ctx, in.Subject)
	if e != nil {
		return TenderDashboard{}, mapDashboardError(e)
	}
	opportunities, bids, e := s.workflow.ReadWorkflow(ctx, in.Subject, in.Limit)
	if e != nil {
		return TenderDashboard{}, mapDashboardError(e)
	}
	invoices, payments, activity, e := s.financial.ReadFinancial(ctx, in.Subject, in.Limit)
	if e != nil {
		return TenderDashboard{}, mapDashboardError(e)
	}
	if !validTenderSources(subscription, opportunities, bids, invoices, payments, activity, int(in.Limit)) {
		return TenderDashboard{}, ErrDashboardUnavailable
	}
	return TenderDashboard{TenantID: in.Subject.TenantID, ObservedAt: at, Subscription: subscription, Opportunities: opportunities, Bids: bids, Invoices: invoices, Payments: payments, FinancialActivity: activity}, nil
}

type PlatformActor struct {
	UserID, SessionID, CorrelationID ID
	ContextGeneration                int64
	SupportSession                   bool
}
type PlatformRequest struct {
	Actor PlatformActor
	Limit int32
}
type PlatformInvoiceItem struct {
	ID, TenantID                                    ID
	Number, State                                   string
	Version, TotalMinorUnits, OutstandingMinorUnits int64
	IssuedAt                                        time.Time
}
type PlatformPaymentItem struct {
	ID, TenantID, InvoiceID    ID
	State, ReconciliationState string
	Version, AmountMinorUnits  int64
	UpdatedAt                  time.Time
}
type PlatformAdjustmentItem struct {
	ID, TenantID, InvoiceID   ID
	Action, State, ReasonCode string
	Version, AmountMinorUnits int64
	CreatedAt, ExpiresAt      time.Time
}
type PlatformReversalItem struct {
	ID, TenantID, PaymentID             ID
	PaymentState, QueueState, HoldState string
	AmountMinorUnits                    int64
	RequestedAt                         time.Time
	AppliedAt                           *time.Time
}
type PlatformInvoicePage struct {
	Items   []PlatformInvoiceItem
	HasMore bool
}
type PlatformPaymentPage struct {
	Items   []PlatformPaymentItem
	HasMore bool
}
type PlatformAdjustmentPage struct {
	Items   []PlatformAdjustmentItem
	HasMore bool
}
type PlatformReversalPage struct {
	Items   []PlatformReversalItem
	HasMore bool
}
type PlatformDashboard struct {
	ObservedAt  time.Time
	Invoices    PlatformInvoicePage
	Payments    PlatformPaymentPage
	Adjustments PlatformAdjustmentPage
	Reversals   PlatformReversalPage
}
type PlatformFinancialSource interface {
	ReadPlatformFinancial(context.Context, PlatformActor, int32) (PlatformInvoicePage, PlatformPaymentPage, PlatformAdjustmentPage, PlatformReversalPage, error)
}
type PlatformDashboardService struct {
	financial PlatformFinancialSource
	clock     Clock
}

func NewPlatformDashboardService(f PlatformFinancialSource, c Clock) (*PlatformDashboardService, error) {
	if f == nil || c == nil {
		return nil, errors.New("platform_dashboard_service_invalid")
	}
	return &PlatformDashboardService{f, c}, nil
}
func (s *PlatformDashboardService) Read(ctx context.Context, in PlatformRequest) (PlatformDashboard, error) {
	a := in.Actor
	if ctx == nil || a.UserID.IsZero() || a.SessionID.IsZero() || a.CorrelationID.IsZero() || a.ContextGeneration < 1 || a.SupportSession || in.Limit < 1 || in.Limit > 25 {
		return PlatformDashboard{}, ErrDashboardInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return PlatformDashboard{}, ErrDashboardUnavailable
	}
	invoices, payments, adjustments, reversals, e := s.financial.ReadPlatformFinancial(ctx, a, in.Limit)
	if e != nil {
		return PlatformDashboard{}, mapDashboardError(e)
	}
	if !validPlatformSources(invoices, payments, adjustments, reversals, int(in.Limit)) {
		return PlatformDashboard{}, ErrDashboardUnavailable
	}
	return PlatformDashboard{ObservedAt: at, Invoices: invoices, Payments: payments, Adjustments: adjustments, Reversals: reversals}, nil
}
func validTenderSources(s SubscriptionSnapshot, o OpportunityPage, b BidPage, i InvoicePage, p PaymentPage, f FinancialActivityPage, limit int) bool {
	if s.ID.IsZero() || s.State == "" || s.AccessMode == "" || s.PlanKey == "" || s.Version < 1 || s.CommercialPeriodStart.IsZero() || !s.CommercialPeriodEnd.After(s.CommercialPeriodStart) || len(o.Items) > limit || len(b.Items) > limit || len(i.Items) > limit || len(p.Items) > limit || len(f.Items) > limit {
		return false
	}
	for _, v := range o.Items {
		if v.ID.IsZero() || v.Title == "" || v.VerificationState == "" || v.Version < 1 || v.UpdatedAt.IsZero() {
			return false
		}
	}
	for _, v := range b.Items {
		if v.ID.IsZero() || v.OpportunityID.IsZero() || v.OwnerID.IsZero() || v.Title == "" || v.State == "" || v.Version < 1 || v.UpdatedAt.IsZero() {
			return false
		}
	}
	for _, v := range i.Items {
		if v.ID.IsZero() || v.Number == "" || v.State == "" || v.Version < 1 || v.TotalMinorUnits < 1 || v.OutstandingMinorUnits < 0 || v.IssuedAt.IsZero() || v.DueAt.IsZero() {
			return false
		}
	}
	for _, v := range p.Items {
		if v.ID.IsZero() || v.InvoiceID.IsZero() || v.State == "" || v.Version < 1 || v.AmountMinorUnits < 1 || v.UpdatedAt.IsZero() {
			return false
		}
	}
	for _, v := range f.Items {
		if v.ID.IsZero() || v.InvoiceID.IsZero() || v.Kind == "" || v.State == "" || v.AmountMinorUnits < 1 || v.OccurredAt.IsZero() {
			return false
		}
	}
	return true
}
func validPlatformSources(i PlatformInvoicePage, p PlatformPaymentPage, a PlatformAdjustmentPage, r PlatformReversalPage, limit int) bool {
	if len(i.Items) > limit || len(p.Items) > limit || len(a.Items) > limit || len(r.Items) > limit {
		return false
	}
	for _, v := range i.Items {
		if v.ID.IsZero() || v.TenantID.IsZero() || v.Number == "" || v.State == "" || v.Version < 1 || v.TotalMinorUnits < 1 || v.IssuedAt.IsZero() {
			return false
		}
	}
	for _, v := range p.Items {
		if v.ID.IsZero() || v.TenantID.IsZero() || v.InvoiceID.IsZero() || v.State == "" || v.ReconciliationState == "" || v.Version < 1 || v.AmountMinorUnits < 1 || v.UpdatedAt.IsZero() {
			return false
		}
	}
	for _, v := range a.Items {
		if v.ID.IsZero() || v.TenantID.IsZero() || v.InvoiceID.IsZero() || v.Action == "" || v.State == "" || v.Version < 1 || v.AmountMinorUnits < 1 || v.CreatedAt.IsZero() {
			return false
		}
	}
	for _, v := range r.Items {
		if v.ID.IsZero() || v.TenantID.IsZero() || v.PaymentID.IsZero() || v.PaymentState == "" || v.QueueState == "" || v.AmountMinorUnits < 1 || v.RequestedAt.IsZero() {
			return false
		}
	}
	return true
}
func mapDashboardError(e error) error {
	if e == nil {
		return nil
	}
	if errors.Is(e, ErrDashboardNotAuthorized) {
		return e
	}
	return ErrDashboardUnavailable
}
