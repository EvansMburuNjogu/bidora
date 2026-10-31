// Package commercial exposes the inbound facade for source-defined commercial
// catalogue and approved subscription-lifecycle operations. Provider-neutral invoice,
// payment, settlement, maker/checker adjustment, reversal, financial-read and
// Platform Billing queue boundaries remain separately configurable. The pilot composes
// only bounded Platform Billing reads; provider-specific effects remain absent.
package commercial

import (
	"context"
	"errors"
	"time"

	"bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
)

type Module struct {
	catalogue               *application.CatalogueService
	subscriptions           *application.SubscriptionPolicyService
	trialStart              *application.TrialStartService
	lifecycle               *application.SubscriptionLifecycleService
	aiUsage                 *application.AIUsageService
	aiConsumer              *application.AIReservationConsumer
	paidWrites              *application.PaidWriteAccessService
	invoices                *application.InvoiceService
	invoiceReads            *application.InvoiceReadService
	payments                *application.PaymentService
	settlements             *application.SettlementService
	adjustments             *application.AdjustmentService
	reversals               *application.ReversalService
	financialReads          *application.FinancialReadService
	financialLists          *application.FinancialListService
	platformFinancialQueues *application.PlatformFinancialQueueService
}

func NewModule() *Module {
	return &Module{catalogue: application.NewCatalogueService(), subscriptions: application.NewSubscriptionPolicyService()}
}

type UnitOfWork interface {
	application.TrialStartUnitOfWork
	application.SubscriptionUnitOfWork
	application.AIUsageUnitOfWork
	application.AIReservationConsumerUnitOfWork
}

type FinancialUnitOfWork interface {
	application.InvoiceUnitOfWork
}

type PaymentUnitOfWork interface {
	application.PaymentUnitOfWork
}

type SettlementUnitOfWork interface {
	application.SettlementUnitOfWork
}

type PlatformFinancialQueueUnitOfWork interface {
	application.PlatformFinancialQueueUnitOfWork
}

func (m *Module) ConfigureSubscriptionLifecycle(uow application.SubscriptionUnitOfWork, ids application.TrialIDGenerator, clock application.TrialClock) error {
	if m == nil {
		return errors.New("commercial_module_unavailable")
	}
	service, err := application.NewSubscriptionLifecycleService(uow, ids, clock)
	if err != nil {
		return err
	}
	m.lifecycle = service
	return nil
}

func (m *Module) ConfigurePlatformFinancialQueues(uow PlatformFinancialQueueUnitOfWork, ids application.InvoiceIDGenerator, clock application.InvoiceClock) error {
	if m == nil {
		return errors.New("commercial_module_unavailable")
	}
	service, err := application.NewPlatformFinancialQueueService(uow, ids, clock)
	if err != nil {
		return err
	}
	m.platformFinancialQueues = service
	return nil
}
func (m *Module) ListPlatformAIUsage(ctx context.Context, input application.PlatformQueueRequest) (application.PlatformAIUsagePage, error) {
	if m == nil || m.platformFinancialQueues == nil {
		return application.PlatformAIUsagePage{}, application.ErrPlatformFinancialQueueUnavailable
	}
	return m.platformFinancialQueues.ListAIUsage(ctx, input)
}
func (m *Module) ListPlatformInvoiceQueue(ctx context.Context, input application.PlatformQueueRequest) (application.PlatformInvoiceQueuePage, error) {
	if m == nil || m.platformFinancialQueues == nil {
		return application.PlatformInvoiceQueuePage{}, errors.New("commercial_module_unavailable")
	}
	return m.platformFinancialQueues.ListInvoices(ctx, input)
}
func (m *Module) ListPlatformPaymentQueue(ctx context.Context, input application.PlatformQueueRequest) (application.PlatformPaymentQueuePage, error) {
	if m == nil || m.platformFinancialQueues == nil {
		return application.PlatformPaymentQueuePage{}, errors.New("commercial_module_unavailable")
	}
	return m.platformFinancialQueues.ListPayments(ctx, input)
}
func (m *Module) ListPlatformAdjustmentQueue(ctx context.Context, input application.PlatformQueueRequest) (application.PlatformAdjustmentQueuePage, error) {
	if m == nil || m.platformFinancialQueues == nil {
		return application.PlatformAdjustmentQueuePage{}, errors.New("commercial_module_unavailable")
	}
	return m.platformFinancialQueues.ListAdjustments(ctx, input)
}
func (m *Module) ListPlatformReversalQueue(ctx context.Context, input application.PlatformQueueRequest) (application.PlatformReversalQueuePage, error) {
	if m == nil || m.platformFinancialQueues == nil {
		return application.PlatformReversalQueuePage{}, errors.New("commercial_module_unavailable")
	}
	return m.platformFinancialQueues.ListReversals(ctx, input)
}

type FinancialListUnitOfWork interface {
	application.FinancialListUnitOfWork
}

func (m *Module) ConfigureFinancialLists(uow FinancialListUnitOfWork) error {
	if m == nil {
		return errors.New("commercial_module_unavailable")
	}
	service, err := application.NewFinancialListService(uow)
	if err != nil {
		return err
	}
	m.financialLists = service
	return nil
}
func (m *Module) ListInvoices(ctx context.Context, input application.FinancialListRequest) (application.InvoiceSummaryPage, error) {
	if m == nil || m.financialLists == nil {
		return application.InvoiceSummaryPage{}, errors.New("commercial_module_unavailable")
	}
	return m.financialLists.ListInvoices(ctx, input)
}
func (m *Module) ListPayments(ctx context.Context, input application.FinancialListRequest) (application.PaymentSummaryPage, error) {
	if m == nil || m.financialLists == nil {
		return application.PaymentSummaryPage{}, errors.New("commercial_module_unavailable")
	}
	return m.financialLists.ListPayments(ctx, input)
}
func (m *Module) ListFinancialHistory(ctx context.Context, input application.FinancialListRequest) (application.FinancialHistoryPage, error) {
	if m == nil || m.financialLists == nil {
		return application.FinancialHistoryPage{}, errors.New("commercial_module_unavailable")
	}
	return m.financialLists.ListHistory(ctx, input)
}

type FinancialReadUnitOfWork interface {
	application.FinancialReadUnitOfWork
}

func (m *Module) ConfigureFinancialReads(uow FinancialReadUnitOfWork, ids application.InvoiceIDGenerator, clock application.InvoiceClock) error {
	if m == nil {
		return errors.New("commercial_module_unavailable")
	}
	service, err := application.NewFinancialReadService(uow, ids, clock)
	if err != nil {
		return err
	}
	m.financialReads = service
	return nil
}
func (m *Module) ReadPaymentFinancialState(ctx context.Context, access application.SubscriptionAccess, id domain.ID) (application.TenantPaymentRead, error) {
	if m == nil || m.financialReads == nil {
		return application.TenantPaymentRead{}, errors.New("commercial_module_unavailable")
	}
	return m.financialReads.ReadPayment(ctx, access, id)
}

func (m *Module) ReadReceipt(ctx context.Context, access application.SubscriptionAccess, id domain.ID) (domain.Receipt, error) {
	if m == nil || m.financialReads == nil {
		return domain.Receipt{}, errors.New("commercial_module_unavailable")
	}
	return m.financialReads.ReadReceipt(ctx, access, id)
}
func (m *Module) ReadCreditNote(ctx context.Context, access application.SubscriptionAccess, id domain.ID) (domain.CreditNote, error) {
	if m == nil || m.financialReads == nil {
		return domain.CreditNote{}, errors.New("commercial_module_unavailable")
	}
	return m.financialReads.ReadCreditNote(ctx, access, id)
}
func (m *Module) ReadPlatformFinancialAdjustment(ctx context.Context, input application.PlatformFinancialReadInput) (application.FinancialAdjustmentRead, error) {
	if m == nil || m.financialReads == nil {
		return application.FinancialAdjustmentRead{}, errors.New("commercial_module_unavailable")
	}
	return m.financialReads.ReadPlatformAdjustment(ctx, input)
}
func (m *Module) ReadPlatformPaymentReversal(ctx context.Context, input application.PlatformFinancialReadInput) (application.PaymentReversalRead, error) {
	if m == nil || m.financialReads == nil {
		return application.PaymentReversalRead{}, errors.New("commercial_module_unavailable")
	}
	return m.financialReads.ReadPlatformReversal(ctx, input)
}

type ReversalUnitOfWork interface{ application.ReversalUnitOfWork }

func (m *Module) ConfigureReversals(uow ReversalUnitOfWork, ids application.InvoiceIDGenerator, clock application.InvoiceClock) error {
	if m == nil {
		return errors.New("commercial_module_unavailable")
	}
	service, err := application.NewReversalService(uow, ids, clock)
	if err != nil {
		return err
	}
	m.reversals = service
	return nil
}
func (m *Module) RequestPaymentReconciliation(ctx context.Context, input application.RequestReconciliationInput) (domain.Payment, error) {
	if m == nil || m.reversals == nil {
		return domain.Payment{}, errors.New("commercial_module_unavailable")
	}
	return m.reversals.RequestReconciliation(ctx, input)
}
func (m *Module) CompletePaymentReversal(ctx context.Context, input application.CompleteReversalInput) (application.ReversalStored, error) {
	if m == nil || m.reversals == nil {
		return application.ReversalStored{}, errors.New("commercial_module_unavailable")
	}
	return m.reversals.Complete(ctx, input)
}

type AdjustmentUnitOfWork interface {
	application.AdjustmentUnitOfWork
}

func (m *Module) ConfigureAdjustments(uow AdjustmentUnitOfWork, ids application.InvoiceIDGenerator, clock application.InvoiceClock, policy application.AdjustmentPolicy) error {
	if m == nil {
		return errors.New("commercial_module_unavailable")
	}
	service, err := application.NewAdjustmentService(uow, ids, clock, policy)
	if err != nil {
		return err
	}
	m.adjustments = service
	return nil
}
func (m *Module) ProposeFinancialAdjustment(ctx context.Context, input application.ProposeAdjustmentInput) (application.AdjustmentStored, error) {
	if m == nil || m.adjustments == nil {
		return application.AdjustmentStored{}, errors.New("commercial_module_unavailable")
	}
	return m.adjustments.Propose(ctx, input)
}
func (m *Module) DecideFinancialAdjustment(ctx context.Context, input application.DecideAdjustmentInput) (application.AdjustmentStored, error) {
	if m == nil || m.adjustments == nil {
		return application.AdjustmentStored{}, errors.New("commercial_module_unavailable")
	}
	return m.adjustments.Decide(ctx, input)
}

func (m *Module) ConfigureSettlements(uow SettlementUnitOfWork, ids application.InvoiceIDGenerator, clock application.InvoiceClock) error {
	if m == nil {
		return errors.New("commercial_module_unavailable")
	}
	service, err := application.NewSettlementService(uow, ids, clock)
	if err != nil {
		return err
	}
	m.settlements = service
	return nil
}

func (m *Module) CompletePaymentSettlement(ctx context.Context, input application.CompleteSettlementInput) (application.SettlementStored, error) {
	if m == nil || m.settlements == nil {
		return application.SettlementStored{}, errors.New("commercial_module_unavailable")
	}
	return m.settlements.Complete(ctx, input)
}

func (m *Module) ReadLicenceEntitlement(ctx context.Context, tenantID domain.ID, at time.Time) (application.LicenceEntitlement, error) {
	if m == nil || m.settlements == nil {
		return application.LicenceEntitlement{}, errors.New("commercial_module_unavailable")
	}
	return m.settlements.ReadLicenceEntitlement(ctx, tenantID, at)
}

func (m *Module) ConfigurePayments(uow PaymentUnitOfWork, settings application.PaymentConfigurationProvider, provider application.PaymentProvider, ids application.InvoiceIDGenerator, clock application.InvoiceClock) error {
	if m == nil {
		return errors.New("commercial_module_unavailable")
	}
	service, err := application.NewPaymentService(uow, settings, provider, ids, clock)
	if err != nil {
		return err
	}
	m.payments = service
	return nil
}

func (m *Module) InitiatePayment(ctx context.Context, input application.InitiatePaymentInput) (application.PaymentInitiationResult, error) {
	if m == nil || m.payments == nil {
		return application.PaymentInitiationResult{}, errors.New("commercial_module_unavailable")
	}
	return m.payments.Initiate(ctx, input)
}

func (m *Module) ReconcileNextPayment(ctx context.Context, executionID domain.ID) (bool, error) {
	if m == nil || m.payments == nil {
		return false, errors.New("commercial_module_unavailable")
	}
	return m.payments.ReconcileNext(ctx, executionID)
}

func (m *Module) ReadPayment(ctx context.Context, access application.SubscriptionAccess, paymentID domain.ID) (domain.Payment, error) {
	if m == nil || m.payments == nil {
		return domain.Payment{}, errors.New("commercial_module_unavailable")
	}
	return m.payments.Read(ctx, access, paymentID)
}

func (m *Module) ConfigureInvoiceReads(uow FinancialUnitOfWork) error {
	if m == nil {
		return errors.New("commercial_module_unavailable")
	}
	service, err := application.NewInvoiceReadService(uow)
	if err != nil {
		return err
	}
	m.invoiceReads = service
	return nil
}

func (m *Module) ConfigureInvoices(uow FinancialUnitOfWork, quotes application.InvoiceQuoteProvider, ids application.InvoiceIDGenerator, clock application.InvoiceClock) error {
	if m == nil {
		return errors.New("commercial_module_unavailable")
	}
	service, err := application.NewInvoiceService(uow, quotes, ids, clock)
	if err != nil {
		return err
	}
	m.invoices = service
	return nil
}

func (m *Module) IssueInvoice(ctx context.Context, input application.IssueInvoiceInput) (domain.Invoice, error) {
	if m == nil || m.invoices == nil {
		return domain.Invoice{}, errors.New("commercial_module_unavailable")
	}
	return m.invoices.Issue(ctx, input)
}

func (m *Module) ReadInvoice(ctx context.Context, access application.SubscriptionAccess, invoiceID domain.ID) (domain.Invoice, error) {
	if m == nil {
		return domain.Invoice{}, errors.New("commercial_module_unavailable")
	}
	if m.invoiceReads != nil {
		return m.invoiceReads.Read(ctx, access, invoiceID)
	}
	if m.invoices != nil {
		return m.invoices.Read(ctx, access, invoiceID)
	}
	return domain.Invoice{}, errors.New("commercial_module_unavailable")
}

func (m *Module) ConfigureTrialStart(uow UnitOfWork, ids application.TrialIDGenerator, clock application.TrialClock) error {
	if m == nil {
		return errors.New("commercial_module_unavailable")
	}
	service, err := application.NewTrialStartService(uow, ids, clock)
	if err != nil {
		return err
	}
	m.trialStart = service
	return nil
}

func NewModuleWithTrialStart(uow UnitOfWork, ids application.TrialIDGenerator, clock application.TrialClock) (*Module, error) {
	module := NewModule()
	service, err := application.NewTrialStartService(uow, ids, clock)
	if err != nil {
		return nil, err
	}
	lifecycle, err := application.NewSubscriptionLifecycleService(uow, ids, clock)
	if err != nil {
		return nil, err
	}
	aiUsage, err := application.NewAIUsageService(uow, ids, clock)
	if err != nil {
		return nil, err
	}
	aiConsumer, err := application.NewAIReservationConsumer(uow, ids, clock)
	if err != nil {
		return nil, err
	}
	paidWrites, err := application.NewPaidWriteAccessService(uow, clock)
	if err != nil {
		return nil, err
	}
	module.trialStart, module.lifecycle, module.aiUsage, module.aiConsumer, module.paidWrites = service, lifecycle, aiUsage, aiConsumer, paidWrites
	return module, nil
}

func (m *Module) ConfigureAIUsage(uow UnitOfWork, ids application.TrialIDGenerator, clock application.TrialClock) error {
	if m == nil {
		return errors.New("commercial_module_unavailable")
	}
	usage, err := application.NewAIUsageService(uow, ids, clock)
	if err != nil {
		return err
	}
	consumer, err := application.NewAIReservationConsumer(uow, ids, clock)
	if err != nil {
		return err
	}
	m.aiUsage, m.aiConsumer = usage, consumer
	return nil
}

func (m *Module) ConfigurePaidWriteAccess(uow application.SubscriptionUnitOfWork, clock application.InvoiceClock) error {
	if m == nil {
		return errors.New("commercial_module_unavailable")
	}
	service, err := application.NewPaidWriteAccessService(uow, clock)
	if err != nil {
		return err
	}
	m.paidWrites = service
	return nil
}

func (m *Module) RequirePaidWriteAccess(ctx context.Context, tenantID domain.ID) error {
	if m == nil || m.paidWrites == nil {
		return errors.New("commercial_module_unavailable")
	}
	return m.paidWrites.Require(ctx, tenantID)
}

func (m *Module) ConsumeAIReservationRequest(ctx context.Context, input application.AIReservationRequestEvent) (application.AIReservationOutcome, error) {
	if m == nil || m.aiConsumer == nil {
		return application.AIReservationOutcome{}, errors.New("commercial_module_unavailable")
	}
	return m.aiConsumer.Consume(ctx, input)
}

func (m *Module) Consume(ctx context.Context, input application.AIReservationRequestEvent) (application.AIReservationOutcome, error) {
	return m.ConsumeAIReservationRequest(ctx, input)
}
func (m *Module) Settle(ctx context.Context, input application.AIUsageCompleteInput) (application.AIUsageResult, error) {
	return m.SettleAIUsage(ctx, input)
}
func (m *Module) Release(ctx context.Context, input application.AIUsageReleaseInput) (application.AIUsageResult, error) {
	return m.ReleaseAIUsage(ctx, input)
}
func (m *Module) VerifyReserved(ctx context.Context, input application.AIUsageReservationCheck) error {
	if m == nil || m.aiUsage == nil {
		return errors.New("commercial_module_unavailable")
	}
	return m.aiUsage.VerifyReserved(ctx, input)
}
func (m *Module) ReserveAIUsage(ctx context.Context, input application.AIUsageReserveInput) (application.AIUsageResult, error) {
	if m == nil || m.aiUsage == nil {
		return application.AIUsageResult{}, errors.New("commercial_module_unavailable")
	}
	return m.aiUsage.Reserve(ctx, input)
}
func (m *Module) SettleAIUsage(ctx context.Context, input application.AIUsageCompleteInput) (application.AIUsageResult, error) {
	if m == nil || m.aiUsage == nil {
		return application.AIUsageResult{}, errors.New("commercial_module_unavailable")
	}
	return m.aiUsage.Settle(ctx, input)
}
func (m *Module) ReleaseAIUsage(ctx context.Context, input application.AIUsageReleaseInput) (application.AIUsageResult, error) {
	if m == nil || m.aiUsage == nil {
		return application.AIUsageResult{}, errors.New("commercial_module_unavailable")
	}
	return m.aiUsage.Release(ctx, input)
}

func (m *Module) SubscriptionSummary(ctx context.Context, access application.SubscriptionAccess) (application.SubscriptionSummary, error) {
	if m == nil || m.lifecycle == nil {
		return application.SubscriptionSummary{}, errors.New("commercial_module_unavailable")
	}
	return m.lifecycle.Summary(ctx, access)
}
func (m *Module) ScheduleSubscriptionCancellation(ctx context.Context, input application.SubscriptionLifecycleInput) (application.SubscriptionSummary, error) {
	if m == nil || m.lifecycle == nil {
		return application.SubscriptionSummary{}, errors.New("commercial_module_unavailable")
	}
	return m.lifecycle.ScheduleCancellation(ctx, input)
}
func (m *Module) WithdrawSubscriptionCancellation(ctx context.Context, input application.SubscriptionLifecycleInput) (application.SubscriptionSummary, error) {
	if m == nil || m.lifecycle == nil {
		return application.SubscriptionSummary{}, errors.New("commercial_module_unavailable")
	}
	return m.lifecycle.WithdrawCancellation(ctx, input)
}

func (m *Module) StartTrialCommand(ctx context.Context, input application.TrialStartInput) (application.TrialStartResult, error) {
	if m == nil || m.trialStart == nil {
		return application.TrialStartResult{}, errors.New("commercial_module_unavailable")
	}
	return m.trialStart.Start(ctx, input)
}

func (m *Module) StartTrial(ctx context.Context, tenantID domain.ID, eligibility domain.TrialEligibility, at time.Time) (domain.Subscription, error) {
	if m == nil || m.subscriptions == nil {
		return domain.Subscription{}, errors.New("commercial_module_unavailable")
	}
	return m.subscriptions.StartTrial(ctx, tenantID, eligibility, at)
}

func (m *Module) NewPaymentPending(ctx context.Context, tenantID domain.ID, plan domain.PlanKey, cadence domain.BillingCadence) (domain.Subscription, error) {
	if m == nil || m.subscriptions == nil {
		return domain.Subscription{}, errors.New("commercial_module_unavailable")
	}
	return m.subscriptions.NewPaymentPending(ctx, tenantID, plan, cadence)
}

func (m *Module) BeginPayment(ctx context.Context, current domain.Subscription, expectedVersion int64, plan domain.PlanKey, cadence domain.BillingCadence) (domain.Subscription, error) {
	if m == nil || m.subscriptions == nil {
		return domain.Subscription{}, errors.New("commercial_module_unavailable")
	}
	return m.subscriptions.BeginPayment(ctx, current, expectedVersion, plan, cadence)
}

func (m *Module) CancelPendingPayment(ctx context.Context, current domain.Subscription, expectedVersion int64) (domain.Subscription, error) {
	if m == nil || m.subscriptions == nil {
		return domain.Subscription{}, errors.New("commercial_module_unavailable")
	}
	return m.subscriptions.CancelPendingPayment(ctx, current, expectedVersion)
}

func (m *Module) NewPaidSubscription(ctx context.Context, tenantID domain.ID, term domain.TermSnapshot, reconciled bool, at time.Time) (domain.Subscription, error) {
	if m == nil || m.subscriptions == nil {
		return domain.Subscription{}, errors.New("commercial_module_unavailable")
	}
	return m.subscriptions.NewPaid(ctx, tenantID, term, reconciled, at)
}

func (m *Module) ActivatePaid(ctx context.Context, current domain.Subscription, expectedVersion int64, term domain.TermSnapshot, reconciled bool, at time.Time) (domain.Subscription, error) {
	if m == nil || m.subscriptions == nil {
		return domain.Subscription{}, errors.New("commercial_module_unavailable")
	}
	return m.subscriptions.ActivatePaid(ctx, current, expectedVersion, term, reconciled, at)
}

func (m *Module) MarkRenewalUnpaid(ctx context.Context, current domain.Subscription, expectedVersion int64, at time.Time) (domain.Subscription, error) {
	if m == nil || m.subscriptions == nil {
		return domain.Subscription{}, errors.New("commercial_module_unavailable")
	}
	return m.subscriptions.MarkRenewalUnpaid(ctx, current, expectedVersion, at)
}
func (m *Module) SuspendOverdue(ctx context.Context, current domain.Subscription, expectedVersion int64, at time.Time) (domain.Subscription, error) {
	if m == nil || m.subscriptions == nil {
		return domain.Subscription{}, errors.New("commercial_module_unavailable")
	}
	return m.subscriptions.SuspendOverdue(ctx, current, expectedVersion, at)
}
func (m *Module) ResumePaid(ctx context.Context, current domain.Subscription, expectedVersion int64, term domain.TermSnapshot, reconciled bool, at time.Time) (domain.Subscription, error) {
	if m == nil || m.subscriptions == nil {
		return domain.Subscription{}, errors.New("commercial_module_unavailable")
	}
	return m.subscriptions.ResumePaid(ctx, current, expectedVersion, term, reconciled, at)
}
func (m *Module) ScheduleCancellation(ctx context.Context, current domain.Subscription, expectedVersion int64) (domain.Subscription, error) {
	if m == nil || m.subscriptions == nil {
		return domain.Subscription{}, errors.New("commercial_module_unavailable")
	}
	return m.subscriptions.ScheduleCancellation(ctx, current, expectedVersion)
}
func (m *Module) WithdrawCancellation(ctx context.Context, current domain.Subscription, expectedVersion int64, at time.Time) (domain.Subscription, error) {
	if m == nil || m.subscriptions == nil {
		return domain.Subscription{}, errors.New("commercial_module_unavailable")
	}
	return m.subscriptions.WithdrawCancellation(ctx, current, expectedVersion, at)
}
func (m *Module) EndAtBoundary(ctx context.Context, current domain.Subscription, expectedVersion int64, at time.Time) (domain.Subscription, error) {
	if m == nil || m.subscriptions == nil {
		return domain.Subscription{}, errors.New("commercial_module_unavailable")
	}
	return m.subscriptions.EndAtBoundary(ctx, current, expectedVersion, at)
}
func (m *Module) RequireReview(ctx context.Context, current domain.Subscription, expectedVersion int64, at time.Time) (domain.Subscription, error) {
	if m == nil || m.subscriptions == nil {
		return domain.Subscription{}, errors.New("commercial_module_unavailable")
	}
	return m.subscriptions.RequireReview(ctx, current, expectedVersion, at)
}

func (m *Module) Plans(ctx context.Context) ([]domain.Plan, error) {
	if m == nil || m.catalogue == nil {
		return nil, errors.New("commercial_module_unavailable")
	}
	return m.catalogue.List(ctx)
}

func (m *Module) CalculateDocumentCredits(ctx context.Context, documentType string, measuredUnits int64) (application.AICreditCalculation, error) {
	if m == nil || m.catalogue == nil {
		return application.AICreditCalculation{}, errors.New("commercial_module_unavailable")
	}
	return m.catalogue.CalculateDocumentCredits(ctx, documentType, measuredUnits)
}

func (m *Module) CalculateGenerationCredits(ctx context.Context, inputCharacters, outputCharacters int64) (application.AICreditCalculation, error) {
	if m == nil || m.catalogue == nil {
		return application.AICreditCalculation{}, errors.New("commercial_module_unavailable")
	}
	return m.catalogue.CalculateGenerationCredits(ctx, inputCharacters, outputCharacters)
}

func (m *Module) QuoteBasePrice(ctx context.Context, plan, cadence string) (application.BasePriceQuote, error) {
	if m == nil || m.catalogue == nil {
		return application.BasePriceQuote{}, errors.New("commercial_module_unavailable")
	}
	return m.catalogue.QuoteBasePrice(ctx, plan, cadence)
}
