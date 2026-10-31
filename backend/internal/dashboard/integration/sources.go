// Package integration adapts owner-module application ports into dashboard reads.
package integration

import (
	commercialapp "bidora/backend/internal/commercial/application"
	commercialdomain "bidora/backend/internal/commercial/domain"
	dashboard "bidora/backend/internal/dashboard/application"
	opportunityapp "bidora/backend/internal/opportunity/application"
	"context"
	"errors"
	"strings"
	"time"
)

type commercialTenderReader interface {
	SubscriptionSummary(context.Context, commercialapp.SubscriptionAccess) (commercialapp.SubscriptionSummary, error)
	ListInvoices(context.Context, commercialapp.FinancialListRequest) (commercialapp.InvoiceSummaryPage, error)
	ListPayments(context.Context, commercialapp.FinancialListRequest) (commercialapp.PaymentSummaryPage, error)
	ListFinancialHistory(context.Context, commercialapp.FinancialListRequest) (commercialapp.FinancialHistoryPage, error)
}
type workflowReader interface {
	ListOpportunities(context.Context, opportunityapp.ReadQuery) (opportunityapp.OpportunityPage, error)
	ListBids(context.Context, opportunityapp.BidReadQuery) (opportunityapp.BidPage, error)
}
type TenderSources struct {
	commercial commercialTenderReader
	workflow   workflowReader
}

func NewTenderSources(c commercialTenderReader, w workflowReader) (*TenderSources, error) {
	if c == nil || w == nil {
		return nil, errors.New("dashboard_tender_sources_invalid")
	}
	return &TenderSources{c, w}, nil
}
func (s *TenderSources) ReadSubscription(ctx context.Context, subject dashboard.TenderSubject) (dashboard.SubscriptionSnapshot, error) {
	a := commercialAccess(subject)
	v, e := s.commercial.SubscriptionSummary(ctx, a)
	if e != nil {
		return dashboard.SubscriptionSnapshot{}, dashboardError(e)
	}
	limits := v.Subscription.Term.Limits
	return dashboard.SubscriptionSnapshot{ID: dashboard.ID(v.SubscriptionID), State: string(v.Subscription.State), AccessMode: string(v.Subscription.Access()), PlanKey: string(v.Subscription.Term.PlanKey), CatalogueVersion: v.Subscription.Term.CatalogueVersion, Cadence: string(v.Subscription.Term.Cadence), Version: v.Subscription.Version, CommercialPeriodStart: v.Subscription.Term.CommercialPeriodStart, CommercialPeriodEnd: v.Subscription.Term.CommercialPeriodEnd, BidWorkspaceLimit: int64ptr(limits.BidWorkspaces), DocumentVersionLimit: int64ptr(limits.DocumentVersions), AICreditLimit: int64ptr(limits.AICredits), StorageLimitBytes: int64ptr(limits.StorageBytes), IncludedLicenceLimit: limits.IncludedLicences, ConcurrentAIJobLimit: limits.ConcurrentAIJobs, ConcurrentAIJobs: v.Usage.ConcurrentAIJobs, BidWorkspacesUsed: v.Usage.BidWorkspaces, DocumentVersionsUsed: v.Usage.DocumentVersions, AICreditsUsed: v.Usage.AICredits, AICreditsReserved: v.Usage.AICreditsReserved, StorageBytesUsed: v.Usage.StorageBytes}, nil
}
func (s *TenderSources) ReadWorkflow(ctx context.Context, subject dashboard.TenderSubject, limit int32) (dashboard.OpportunityPage, dashboard.BidPage, error) {
	sub := opportunityapp.WorkflowSubject{IdentityID: opportunityapp.ID(subject.ActorID), SelectedTenantID: opportunityapp.ID(subject.TenantID), ContextGeneration: subject.ContextGeneration}
	opportunities, e := s.workflow.ListOpportunities(ctx, opportunityapp.ReadQuery{Subject: sub, Limit: int(limit)})
	if e != nil {
		return dashboard.OpportunityPage{}, dashboard.BidPage{}, dashboardError(e)
	}
	bids, e := s.workflow.ListBids(ctx, opportunityapp.BidReadQuery{ReadQuery: opportunityapp.ReadQuery{Subject: sub, Limit: int(limit)}, ArchiveState: "ACTIVE"})
	if e != nil {
		return dashboard.OpportunityPage{}, dashboard.BidPage{}, dashboardError(e)
	}
	op := dashboard.OpportunityPage{Items: make([]dashboard.OpportunityItem, len(opportunities.Items)), HasMore: opportunities.HasMore}
	for i, v := range opportunities.Items {
		op.Items[i] = dashboard.OpportunityItem{ID: dashboard.ID(v.ID), Title: v.Title, VerificationState: v.VerificationState, BuyerName: cloneString(v.BuyerName), ClosesAt: cloneTime(v.ClosesAt), Version: v.Version, UpdatedAt: v.UpdatedAt.UTC()}
	}
	bp := dashboard.BidPage{Items: make([]dashboard.BidItem, len(bids.Items)), HasMore: bids.HasMore}
	for i, v := range bids.Items {
		bp.Items[i] = dashboard.BidItem{ID: dashboard.ID(v.ID), OpportunityID: dashboard.ID(v.OpportunityID), OwnerID: dashboard.ID(v.OwnerID), Title: v.Title, State: v.State, BuyerName: cloneString(v.BuyerName), ClosesAt: cloneTime(v.ClosesAt), Version: v.Version, UpdatedAt: v.UpdatedAt.UTC()}
	}
	return op, bp, nil
}
func (s *TenderSources) ReadFinancial(ctx context.Context, subject dashboard.TenderSubject, limit int32) (dashboard.InvoicePage, dashboard.PaymentPage, dashboard.FinancialActivityPage, error) {
	request := commercialapp.FinancialListRequest{Access: commercialAccess(subject), Limit: limit}
	invoices, e := s.commercial.ListInvoices(ctx, request)
	if e != nil {
		return dashboard.InvoicePage{}, dashboard.PaymentPage{}, dashboard.FinancialActivityPage{}, dashboardError(e)
	}
	payments, e := s.commercial.ListPayments(ctx, request)
	if e != nil {
		return dashboard.InvoicePage{}, dashboard.PaymentPage{}, dashboard.FinancialActivityPage{}, dashboardError(e)
	}
	history, e := s.commercial.ListFinancialHistory(ctx, request)
	if e != nil {
		return dashboard.InvoicePage{}, dashboard.PaymentPage{}, dashboard.FinancialActivityPage{}, dashboardError(e)
	}
	ip := dashboard.InvoicePage{Items: make([]dashboard.InvoiceItem, len(invoices.Items)), HasMore: invoices.NextCursor != nil}
	for i, v := range invoices.Items {
		ip.Items[i] = dashboard.InvoiceItem{ID: dashboard.ID(v.ID), Number: v.Number, State: v.State, PurchaseKind: v.PurchaseKind, Version: v.Version, TotalMinorUnits: v.TotalMinorUnits, OutstandingMinorUnits: v.OutstandingMinorUnits, IssuedAt: v.IssuedAt.UTC(), DueAt: v.DueAt.UTC()}
	}
	pp := dashboard.PaymentPage{Items: make([]dashboard.PaymentItem, len(payments.Items)), HasMore: payments.NextCursor != nil}
	for i, v := range payments.Items {
		pp.Items[i] = dashboard.PaymentItem{ID: dashboard.ID(v.ID), InvoiceID: dashboard.ID(v.InvoiceID), State: v.State, Version: v.Version, AmountMinorUnits: v.AmountMinorUnits, ReconciliationPending: v.ReconciliationPending, UpdatedAt: v.UpdatedAt.UTC()}
	}
	fp := dashboard.FinancialActivityPage{Items: make([]dashboard.FinancialActivityItem, len(history.Items)), HasMore: history.NextCursor != nil}
	for i, v := range history.Items {
		fp.Items[i] = dashboard.FinancialActivityItem{ID: dashboard.ID(v.ID), InvoiceID: dashboard.ID(v.InvoiceID), PaymentID: dashboard.ID(v.PaymentID), Kind: v.Kind, DocumentNumber: v.DocumentNumber, State: v.State, AmountMinorUnits: v.AmountMinorUnits, OccurredAt: v.OccurredAt.UTC()}
	}
	return ip, pp, fp, nil
}
func commercialAccess(s dashboard.TenderSubject) commercialapp.SubscriptionAccess {
	return commercialapp.SubscriptionAccess{ActorID: commercialdomain.ID(s.ActorID), TenantID: commercialdomain.ID(s.TenantID), ContextGeneration: s.ContextGeneration}
}

type commercialPlatformReader interface {
	ListPlatformInvoiceQueue(context.Context, commercialapp.PlatformQueueRequest) (commercialapp.PlatformInvoiceQueuePage, error)
	ListPlatformPaymentQueue(context.Context, commercialapp.PlatformQueueRequest) (commercialapp.PlatformPaymentQueuePage, error)
	ListPlatformAdjustmentQueue(context.Context, commercialapp.PlatformQueueRequest) (commercialapp.PlatformAdjustmentQueuePage, error)
	ListPlatformReversalQueue(context.Context, commercialapp.PlatformQueueRequest) (commercialapp.PlatformReversalQueuePage, error)
}
type PlatformSources struct{ commercial commercialPlatformReader }

func NewPlatformSources(c commercialPlatformReader) (*PlatformSources, error) {
	if c == nil {
		return nil, errors.New("dashboard_platform_sources_invalid")
	}
	return &PlatformSources{c}, nil
}
func (s *PlatformSources) ReadPlatformFinancial(ctx context.Context, actor dashboard.PlatformActor, limit int32) (dashboard.PlatformInvoicePage, dashboard.PlatformPaymentPage, dashboard.PlatformAdjustmentPage, dashboard.PlatformReversalPage, error) {
	request := commercialapp.PlatformQueueRequest{Actor: commercialapp.AdjustmentActor{UserID: commercialdomain.ID(actor.UserID), SessionID: commercialdomain.ID(actor.SessionID), ContextGeneration: actor.ContextGeneration, SupportSession: actor.SupportSession}, CorrelationID: commercialdomain.ID(actor.CorrelationID), Limit: limit}
	invoices, e := s.commercial.ListPlatformInvoiceQueue(ctx, request)
	if e != nil {
		return dashboard.PlatformInvoicePage{}, dashboard.PlatformPaymentPage{}, dashboard.PlatformAdjustmentPage{}, dashboard.PlatformReversalPage{}, dashboardError(e)
	}
	payments, e := s.commercial.ListPlatformPaymentQueue(ctx, request)
	if e != nil {
		return dashboard.PlatformInvoicePage{}, dashboard.PlatformPaymentPage{}, dashboard.PlatformAdjustmentPage{}, dashboard.PlatformReversalPage{}, dashboardError(e)
	}
	adjustments, e := s.commercial.ListPlatformAdjustmentQueue(ctx, request)
	if e != nil {
		return dashboard.PlatformInvoicePage{}, dashboard.PlatformPaymentPage{}, dashboard.PlatformAdjustmentPage{}, dashboard.PlatformReversalPage{}, dashboardError(e)
	}
	reversals, e := s.commercial.ListPlatformReversalQueue(ctx, request)
	if e != nil {
		return dashboard.PlatformInvoicePage{}, dashboard.PlatformPaymentPage{}, dashboard.PlatformAdjustmentPage{}, dashboard.PlatformReversalPage{}, dashboardError(e)
	}
	ip := dashboard.PlatformInvoicePage{Items: make([]dashboard.PlatformInvoiceItem, len(invoices.Items)), HasMore: invoices.NextCursor != nil}
	for i, v := range invoices.Items {
		ip.Items[i] = dashboard.PlatformInvoiceItem{ID: dashboard.ID(v.ID), TenantID: dashboard.ID(v.TenantID), Number: v.Number, State: v.State, Version: v.Version, TotalMinorUnits: v.TotalMinorUnits, OutstandingMinorUnits: v.OutstandingMinorUnits, IssuedAt: v.IssuedAt.UTC()}
	}
	pp := dashboard.PlatformPaymentPage{Items: make([]dashboard.PlatformPaymentItem, len(payments.Items)), HasMore: payments.NextCursor != nil}
	for i, v := range payments.Items {
		pp.Items[i] = dashboard.PlatformPaymentItem{ID: dashboard.ID(v.ID), TenantID: dashboard.ID(v.TenantID), InvoiceID: dashboard.ID(v.InvoiceID), State: v.State, ReconciliationState: v.ReconciliationState, Version: v.Version, AmountMinorUnits: v.AmountMinorUnits, UpdatedAt: v.UpdatedAt.UTC()}
	}
	ap := dashboard.PlatformAdjustmentPage{Items: make([]dashboard.PlatformAdjustmentItem, len(adjustments.Items)), HasMore: adjustments.NextCursor != nil}
	for i, v := range adjustments.Items {
		ap.Items[i] = dashboard.PlatformAdjustmentItem{ID: dashboard.ID(v.ID), TenantID: dashboard.ID(v.TenantID), InvoiceID: dashboard.ID(v.InvoiceID), Action: v.Action, State: v.State, ReasonCode: v.ReasonCode, Version: v.Version, AmountMinorUnits: v.AmountMinorUnits, CreatedAt: v.CreatedAt.UTC(), ExpiresAt: v.ExpiresAt.UTC()}
	}
	rp := dashboard.PlatformReversalPage{Items: make([]dashboard.PlatformReversalItem, len(reversals.Items)), HasMore: reversals.NextCursor != nil}
	for i, v := range reversals.Items {
		rp.Items[i] = dashboard.PlatformReversalItem{ID: dashboard.ID(v.ID), TenantID: dashboard.ID(v.TenantID), PaymentID: dashboard.ID(v.PaymentID), PaymentState: v.PaymentState, QueueState: v.QueueState, HoldState: v.HoldState, AmountMinorUnits: v.AmountMinorUnits, RequestedAt: v.RequestedAt.UTC(), AppliedAt: cloneTime(v.AppliedAt)}
	}
	return ip, pp, ap, rp, nil
}
func dashboardError(e error) error {
	if errors.Is(e, commercialapp.ErrSubscriptionAccessDenied) || errors.Is(e, commercialapp.ErrFinancialListNotAuthorized) || errors.Is(e, commercialapp.ErrPlatformFinancialQueueNotAuthorized) || strings.Contains(e.Error(), "not_authorized") {
		return dashboard.ErrDashboardNotAuthorized
	}
	return e
}
func int64ptr(v int64) *int64 { x := v; return &x }
func cloneString(v *string) *string {
	if v == nil {
		return nil
	}
	x := *v
	return &x
}
func cloneTime(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	x := v.UTC()
	return &x
}

var _ dashboard.TenderSubscriptionSource = (*TenderSources)(nil)
var _ dashboard.TenderWorkflowSource = (*TenderSources)(nil)
var _ dashboard.TenderFinancialSource = (*TenderSources)(nil)
var _ dashboard.PlatformFinancialSource = (*PlatformSources)(nil)
