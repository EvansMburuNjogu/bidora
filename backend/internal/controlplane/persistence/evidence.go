// Package persistence owns shared control-plane audit/outbox adapters. It does
// not authorize actors or expose generated repositories to application code.
package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	controlapp "bidora/backend/internal/controlplane/application"
	controlsql "bidora/backend/internal/controlplane/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type EvidenceTx struct{ queries *controlsql.Queries }

func NewEvidenceTx(db postgres.DBTX) (*EvidenceTx, error) {
	if db == nil {
		return nil, errors.New("control_evidence_tx_invalid")
	}
	return &EvidenceTx{queries: controlsql.New(db)}, nil
}

func (tx *EvidenceTx) AppendPlatformIntegrationSetting(ctx context.Context, evidence controlapp.PlatformIntegrationSettingEvidence) error {
	if ctx == nil || evidence.AuditID.IsZero() || evidence.OutboxID.IsZero() || evidence.CorrelationID.IsZero() || evidence.ActorID.IsZero() || evidence.TargetID.IsZero() || evidence.Action == "" || evidence.EventType == "" || evidence.Kind == "" || evidence.State == "" || evidence.Version < 1 || evidence.OccurredAt.IsZero() {
		return errors.New("platform_integration_setting_evidence_invalid")
	}
	summary, err := json.Marshal(struct {
		Kind       string `json:"kind"`
		State      string `json:"state"`
		ResultCode string `json:"result_code,omitempty"`
		Version    int64  `json:"version"`
	}{evidence.Kind, evidence.State, evidence.ResultCode, evidence.Version})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		TargetID   string `json:"target_id"`
		Kind       string `json:"kind"`
		State      string `json:"state"`
		ResultCode string `json:"result_code,omitempty"`
		Version    int64  `json:"version"`
	}{uuid.UUID(evidence.TargetID).String(), evidence.Kind, evidence.State, evidence.ResultCode, evidence.Version})
	if err != nil {
		return err
	}
	at, policyVersion := controlTime(evidence.OccurredAt), int64(1)
	if err = tx.queries.InsertControlAudit(ctx, controlsql.InsertControlAuditParams{ID: controlID(evidence.AuditID), ActorID: controlID(evidence.ActorID), Action: evidence.Action, TargetType: "platform_integration_setting", TargetID: controlID(evidence.TargetID), OccurredAt: at, CorrelationID: controlID(evidence.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion}); err != nil {
		return err
	}
	return tx.queries.InsertControlOutbox(ctx, controlsql.InsertControlOutboxParams{ID: controlID(evidence.OutboxID), ActorID: controlID(evidence.ActorID), AggregateType: "platform_integration_setting", AggregateID: controlID(evidence.TargetID), AggregateVersion: evidence.Version, EventType: evidence.EventType, SchemaVersion: 1, Classification: "INTERNAL", Payload: payload, OccurredAt: at})
}

func (tx *EvidenceTx) AppendCommercialAIDenied(ctx context.Context, evidence controlapp.CommercialAIDeniedEvidence) error {
	if ctx == nil || evidence.AuditID.IsZero() || evidence.OutboxID.IsZero() || evidence.CorrelationID.IsZero() || evidence.CheckpointID.IsZero() || evidence.TenantID.IsZero() || evidence.RequesterUserID.IsZero() || evidence.AssignmentGeneration < 1 || evidence.EstimatedCredits < 1 || (evidence.DenialCode != "LIMIT_EXCEEDED" && evidence.DenialCode != "ACCESS_UNAVAILABLE" && evidence.DenialCode != "SUBSCRIPTION_UNAVAILABLE") || evidence.OccurredAt.IsZero() {
		return errors.New("commercial_ai_denial_evidence_invalid")
	}
	summary, err := json.Marshal(struct {
		Kind, DenialCode                       string
		AssignmentGeneration, EstimatedCredits int64
	}{evidence.Kind, evidence.DenialCode, evidence.AssignmentGeneration, evidence.EstimatedCredits})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		CheckpointID, TenantID, CorrelationID, Disposition, DenialCode, Kind string
		AssignmentGeneration, EstimatedCredits                               int64
	}{uuid.UUID(evidence.CheckpointID).String(), uuid.UUID(evidence.TenantID).String(), uuid.UUID(evidence.CorrelationID).String(), "DENIED", evidence.DenialCode, evidence.Kind, evidence.AssignmentGeneration, evidence.EstimatedCredits})
	if err != nil {
		return err
	}
	at, policyVersion := controlTime(evidence.OccurredAt), int64(1)
	if err = tx.queries.InsertRoutedControlAudit(ctx, controlsql.InsertRoutedControlAuditParams{ID: controlID(evidence.AuditID), TenantID: controlID(evidence.TenantID), ActorID: controlID(evidence.RequesterUserID), Action: "CommercialAIUsageDenied", TargetType: "ai_usage_checkpoint", TargetID: controlID(evidence.CheckpointID), OccurredAt: at, CorrelationID: controlID(evidence.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion}); err != nil {
		return err
	}
	return tx.queries.InsertRoutedControlOutbox(ctx, controlsql.InsertRoutedControlOutboxParams{ID: controlID(evidence.OutboxID), TenantID: controlID(evidence.TenantID), ActorID: controlID(evidence.RequesterUserID), AggregateType: "ai_usage_checkpoint", AggregateID: controlID(evidence.CheckpointID), AggregateVersion: 1, EventType: "commercial.ai_usage.denied.v1", SchemaVersion: 1, Classification: "INTERNAL", Payload: payload, OccurredAt: at})
}

func (tx *EvidenceTx) AppendCommercialAIUsage(ctx context.Context, evidence controlapp.CommercialAIUsageEvidence) error {
	if ctx == nil || evidence.AuditID.IsZero() || evidence.OutboxID.IsZero() || evidence.CorrelationID.IsZero() || evidence.CheckpointID.IsZero() || evidence.TenantID.IsZero() || evidence.ReservationID.IsZero() || evidence.RequesterUserID.IsZero() || evidence.Operation == "" || evidence.EventType == "" || evidence.Kind == "" || evidence.Status == "" || evidence.AssignmentGeneration < 1 || evidence.EstimatedCredits < 1 || evidence.UsageVersion < 2 || evidence.ReservationVersion < 1 || evidence.OccurredAt.IsZero() {
		return errors.New("commercial_ai_usage_evidence_invalid")
	}
	executionID := ""
	if !evidence.ExecutionID.IsZero() {
		executionID = uuid.UUID(evidence.ExecutionID).String()
	}
	summary, err := json.Marshal(struct {
		Operation, Kind, Status                                             string
		AssignmentGeneration, EstimatedCredits, ActualCredits, UsageVersion int64
	}{evidence.Operation, evidence.Kind, evidence.Status, evidence.AssignmentGeneration, evidence.EstimatedCredits, evidence.ActualCredits, evidence.UsageVersion})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		CheckpointID, TenantID, ReservationID, CorrelationID, ExecutionID, Kind, Status string
		AssignmentGeneration, EstimatedCredits, ActualCredits, UsageVersion             int64
	}{uuid.UUID(evidence.CheckpointID).String(), uuid.UUID(evidence.TenantID).String(), uuid.UUID(evidence.ReservationID).String(), uuid.UUID(evidence.CorrelationID).String(), executionID, evidence.Kind, evidence.Status, evidence.AssignmentGeneration, evidence.EstimatedCredits, evidence.ActualCredits, evidence.UsageVersion})
	if err != nil {
		return err
	}
	at, policyVersion := controlTime(evidence.OccurredAt), int64(1)
	if err = tx.queries.InsertRoutedControlAudit(ctx, controlsql.InsertRoutedControlAuditParams{ID: controlID(evidence.AuditID), TenantID: controlID(evidence.TenantID), ActorID: controlID(evidence.RequesterUserID), Action: evidence.Operation, TargetType: "commercial_ai_usage_reservation", TargetID: controlID(evidence.ReservationID), OccurredAt: at, CorrelationID: controlID(evidence.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion}); err != nil {
		return err
	}
	return tx.queries.InsertRoutedControlOutbox(ctx, controlsql.InsertRoutedControlOutboxParams{ID: controlID(evidence.OutboxID), TenantID: controlID(evidence.TenantID), ActorID: controlID(evidence.RequesterUserID), AggregateType: "commercial_ai_usage_reservation", AggregateID: controlID(evidence.ReservationID), AggregateVersion: evidence.ReservationVersion, EventType: evidence.EventType, SchemaVersion: 1, Classification: "INTERNAL", Payload: payload, OccurredAt: at})
}

func (tx *EvidenceTx) AppendCommercialPayment(ctx context.Context, evidence controlapp.CommercialPaymentEvidence) error {
	if ctx == nil || evidence.AuditID.IsZero() || evidence.OutboxID.IsZero() || evidence.CorrelationID.IsZero() || evidence.ActorID.IsZero() || evidence.TenantID.IsZero() || evidence.InvoiceID.IsZero() || evidence.PaymentID.IsZero() || evidence.Version < 1 || evidence.AmountMinorUnits < 1 || evidence.OccurredAt.IsZero() {
		return errors.New("commercial_payment_evidence_invalid")
	}
	action, ok := map[string]string{"commercial.payment.create_requested.v1": "CommercialPaymentCreateRequested", "commercial.payment.create_observed.v1": "CommercialPaymentCreateObserved", "commercial.payment.reconciliation_deferred.v1": "CommercialPaymentReconciliationDeferred", "commercial.payment.reconciled.v1": "CommercialPaymentReconciled", "commercial.payment.settled.v1": "CommercialPaymentSettled", "commercial.payment.reconciliation_requested.v1": "CommercialPaymentReconciliationRequested"}[evidence.EventType]
	if !ok || evidence.State == "" || evidence.ReconciliationState == "" {
		return errors.New("commercial_payment_evidence_invalid")
	}
	summary, err := json.Marshal(struct {
		State, ReconciliationState string
		AmountMinorUnits           int64
	}{evidence.State, evidence.ReconciliationState, evidence.AmountMinorUnits})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		TenantID, InvoiceID, PaymentID, State, ReconciliationState, Currency string
		AmountMinorUnits, Version                                            int64
	}{uuid.UUID(evidence.TenantID).String(), uuid.UUID(evidence.InvoiceID).String(), uuid.UUID(evidence.PaymentID).String(), evidence.State, evidence.ReconciliationState, "KES", evidence.AmountMinorUnits, evidence.Version})
	if err != nil {
		return err
	}
	at, policyVersion := controlTime(evidence.OccurredAt), int64(1)
	if err = tx.queries.InsertRoutedControlAudit(ctx, controlsql.InsertRoutedControlAuditParams{ID: controlID(evidence.AuditID), TenantID: controlID(evidence.TenantID), ActorID: controlID(evidence.ActorID), Action: action, TargetType: "commercial_payment", TargetID: controlID(evidence.PaymentID), OccurredAt: at, CorrelationID: controlID(evidence.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion}); err != nil {
		return err
	}
	return tx.queries.InsertRoutedControlOutbox(ctx, controlsql.InsertRoutedControlOutboxParams{ID: controlID(evidence.OutboxID), TenantID: controlID(evidence.TenantID), ActorID: controlID(evidence.ActorID), AggregateType: "commercial_payment", AggregateID: controlID(evidence.PaymentID), AggregateVersion: evidence.Version, EventType: evidence.EventType, SchemaVersion: 1, Classification: "CONFIDENTIAL", Payload: payload, OccurredAt: at})
}

func (tx *EvidenceTx) AppendCommercialFinancialRead(ctx context.Context, e controlapp.CommercialFinancialReadEvidence) error {
	allowed := map[string]string{"CommercialFinancialAdjustmentRead": "commercial_financial_adjustment", "CommercialPaymentReversalRead": "commercial_payment_reversal"}
	target, ok := allowed[e.Action]
	if ctx == nil || !ok || target != e.TargetType || e.AuditID.IsZero() || e.CorrelationID.IsZero() || e.ActorID.IsZero() || e.TenantID.IsZero() || e.TargetID.IsZero() || e.OccurredAt.IsZero() {
		return errors.New("commercial_financial_read_evidence_invalid")
	}
	summary, err := json.Marshal(struct{ Read bool }{true})
	if err != nil {
		return err
	}
	policyVersion := int64(1)
	return tx.queries.InsertRoutedControlAudit(ctx, controlsql.InsertRoutedControlAuditParams{ID: controlID(e.AuditID), TenantID: controlID(e.TenantID), ActorID: controlID(e.ActorID), Action: e.Action, TargetType: e.TargetType, TargetID: controlID(e.TargetID), OccurredAt: controlTime(e.OccurredAt), CorrelationID: controlID(e.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion})
}

func (tx *EvidenceTx) AppendCommercialPlatformQueueRead(ctx context.Context, e controlapp.CommercialPlatformQueueReadEvidence) error {
	actions := map[string]string{"INVOICE": "CommercialPlatformInvoiceQueueRead", "PAYMENT": "CommercialPlatformPaymentQueueRead", "ADJUSTMENT": "CommercialPlatformAdjustmentQueueRead", "REVERSAL": "CommercialPlatformReversalQueueRead", "AI_USAGE": "CommercialPlatformAIUsageRead"}
	action, ok := actions[e.Queue]
	if ctx == nil || !ok || e.AuditID.IsZero() || e.CorrelationID.IsZero() || e.ActorID.IsZero() || e.TargetID.IsZero() || e.ResultCount < 0 || e.ResultCount > 100 || e.OccurredAt.IsZero() {
		return errors.New("commercial_platform_queue_read_evidence_invalid")
	}
	summary, err := json.Marshal(struct {
		Queue, StateFilter, SecondaryFilter string
		ResultCount                         int
		HasMore                             bool
	}{e.Queue, e.StateFilter, e.SecondaryFilter, e.ResultCount, e.HasMore})
	if err != nil {
		return err
	}
	policyVersion := int64(1)
	return tx.queries.InsertControlAudit(ctx, controlsql.InsertControlAuditParams{ID: controlID(e.AuditID), ActorID: controlID(e.ActorID), Action: action, TargetType: "commercial_platform_queue_read", TargetID: controlID(e.TargetID), OccurredAt: controlTime(e.OccurredAt), CorrelationID: controlID(e.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion})
}

func (tx *EvidenceTx) AppendCommercialReversal(ctx context.Context, e controlapp.CommercialReversalEvidence) error {
	if ctx == nil || e.AuditID.IsZero() || e.OutboxID.IsZero() || e.CorrelationID.IsZero() || e.ActorID.IsZero() || e.TenantID.IsZero() || e.InvoiceID.IsZero() || e.PaymentID.IsZero() || e.SubscriptionID.IsZero() || e.ObservationID.IsZero() || e.AccessHoldID.IsZero() || e.EventType != "commercial.payment.reversed.v1" || e.PaymentState != "REVERSED" || e.InvoiceState != "REVIEW_REQUIRED" || e.SubscriptionState != "SUSPENDED" || e.AccessMode != "RETAINED_READ_ONLY" || e.PaymentVersion < 2 || e.InvoiceVersion < 2 || e.SubscriptionVersion < 2 || e.AmountMinorUnits < 1 || e.OccurredAt.IsZero() {
		return errors.New("commercial_reversal_evidence_invalid")
	}
	summary, err := json.Marshal(struct {
		PaymentState, InvoiceState, SubscriptionState, AccessMode             string
		PaymentVersion, InvoiceVersion, SubscriptionVersion, AmountMinorUnits int64
	}{e.PaymentState, e.InvoiceState, e.SubscriptionState, e.AccessMode, e.PaymentVersion, e.InvoiceVersion, e.SubscriptionVersion, e.AmountMinorUnits})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		TenantID, InvoiceID, PaymentID, SubscriptionID, ObservationID, AccessHoldID, PaymentState, InvoiceState, SubscriptionState, AccessMode, Currency string
		PaymentVersion, InvoiceVersion, SubscriptionVersion, AmountMinorUnits                                                                            int64
	}{uuid.UUID(e.TenantID).String(), uuid.UUID(e.InvoiceID).String(), uuid.UUID(e.PaymentID).String(), uuid.UUID(e.SubscriptionID).String(), uuid.UUID(e.ObservationID).String(), uuid.UUID(e.AccessHoldID).String(), e.PaymentState, e.InvoiceState, e.SubscriptionState, e.AccessMode, "KES", e.PaymentVersion, e.InvoiceVersion, e.SubscriptionVersion, e.AmountMinorUnits})
	if err != nil {
		return err
	}
	at, policyVersion := controlTime(e.OccurredAt), int64(1)
	if err = tx.queries.InsertRoutedControlAudit(ctx, controlsql.InsertRoutedControlAuditParams{ID: controlID(e.AuditID), TenantID: controlID(e.TenantID), ActorID: controlID(e.ActorID), Action: "CommercialPaymentReversed", TargetType: "commercial_payment", TargetID: controlID(e.PaymentID), OccurredAt: at, CorrelationID: controlID(e.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion}); err != nil {
		return err
	}
	return tx.queries.InsertRoutedControlOutbox(ctx, controlsql.InsertRoutedControlOutboxParams{ID: controlID(e.OutboxID), TenantID: controlID(e.TenantID), ActorID: controlID(e.ActorID), AggregateType: "commercial_payment", AggregateID: controlID(e.PaymentID), AggregateVersion: e.PaymentVersion, EventType: e.EventType, SchemaVersion: 1, Classification: "CONFIDENTIAL", Payload: payload, OccurredAt: at})
}

func (tx *EvidenceTx) AppendCommercialAdjustment(ctx context.Context, evidence controlapp.CommercialAdjustmentEvidence) error {
	if ctx == nil || evidence.AuditID.IsZero() || evidence.OutboxID.IsZero() || evidence.CorrelationID.IsZero() || evidence.ActorID.IsZero() || evidence.TenantID.IsZero() || evidence.InvoiceID.IsZero() || evidence.PaymentID.IsZero() || evidence.ChangeID.IsZero() || evidence.Version < 1 || evidence.AmountMinorUnits < 1 || evidence.OccurredAt.IsZero() {
		return errors.New("commercial_adjustment_evidence_invalid")
	}
	action, ok := map[string]string{"commercial.financial_adjustment.proposed.v1": "CommercialFinancialAdjustmentProposed", "commercial.financial_adjustment.approved.v1": "CommercialFinancialAdjustmentApproved", "commercial.financial_adjustment.rejected.v1": "CommercialFinancialAdjustmentRejected", "commercial.financial_adjustment.expired.v1": "CommercialFinancialAdjustmentExpired"}[evidence.EventType]
	_, actionOK := map[string]struct{}{"ISSUE_CREDIT": {}, "REQUEST_REFUND": {}}[evidence.Action]
	_, reasonOK := map[string]struct{}{"SERVICE_CREDIT": {}, "DUPLICATE_PAYMENT_REFUND": {}, "SERVICE_FAILURE_REFUND": {}, "PAYMENT_REVERSAL_CORRECTION": {}, "OTHER_CONTRACTUAL_REFUND": {}}[evidence.ReasonCode]
	_, dispositionOK := map[string]struct{}{"NO_ENTITLEMENT_CHANGE": {}, "REDUCE_ADDITIONAL_LICENCES_AT_RENEWAL": {}, "END_PAID_ACCESS_ON_REFUND_SETTLEMENT": {}, "REVIEW_REQUIRED": {}}[evidence.EntitlementDisposition]
	if !ok || !actionOK || !reasonOK || !dispositionOK {
		return errors.New("commercial_adjustment_evidence_invalid")
	}
	summary, err := json.Marshal(struct {
		State, Operation, ReasonCode, EntitlementDisposition string
		AmountMinorUnits                                     int64
	}{evidence.State, evidence.Action, evidence.ReasonCode, evidence.EntitlementDisposition, evidence.AmountMinorUnits})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		TenantID, InvoiceID, PaymentID, ChangeID, State, Operation, ReasonCode, EntitlementDisposition, Currency string
		AmountMinorUnits, Version                                                                                int64
	}{uuid.UUID(evidence.TenantID).String(), uuid.UUID(evidence.InvoiceID).String(), uuid.UUID(evidence.PaymentID).String(), uuid.UUID(evidence.ChangeID).String(), evidence.State, evidence.Action, evidence.ReasonCode, evidence.EntitlementDisposition, "KES", evidence.AmountMinorUnits, evidence.Version})
	if err != nil {
		return err
	}
	at, policyVersion := controlTime(evidence.OccurredAt), int64(1)
	if err = tx.queries.InsertRoutedControlAudit(ctx, controlsql.InsertRoutedControlAuditParams{ID: controlID(evidence.AuditID), TenantID: controlID(evidence.TenantID), ActorID: controlID(evidence.ActorID), Action: action, TargetType: "commercial_financial_adjustment", TargetID: controlID(evidence.ChangeID), OccurredAt: at, CorrelationID: controlID(evidence.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion}); err != nil {
		return err
	}
	return tx.queries.InsertRoutedControlOutbox(ctx, controlsql.InsertRoutedControlOutboxParams{ID: controlID(evidence.OutboxID), TenantID: controlID(evidence.TenantID), ActorID: controlID(evidence.ActorID), AggregateType: "commercial_financial_adjustment", AggregateID: controlID(evidence.ChangeID), AggregateVersion: evidence.Version, EventType: evidence.EventType, SchemaVersion: 1, Classification: "CONFIDENTIAL", Payload: payload, OccurredAt: at})
}

func (tx *EvidenceTx) AppendCommercialInvoiceIssued(ctx context.Context, evidence controlapp.CommercialInvoiceEvidence) error {
	if ctx == nil || evidence.AuditID.IsZero() || evidence.OutboxID.IsZero() || evidence.CorrelationID.IsZero() || evidence.ActorID.IsZero() || evidence.TenantID.IsZero() || evidence.InvoiceID.IsZero() || evidence.InvoiceNumber == "" || evidence.State != "ISSUED" || evidence.PurchaseKind == "" || evidence.Version != 1 || evidence.TotalMinorUnits < 0 || evidence.OccurredAt.IsZero() {
		return errors.New("commercial_invoice_evidence_invalid")
	}
	summary, err := json.Marshal(struct {
		InvoiceNumber, State, PurchaseKind string
		TotalMinorUnits                    int64
	}{evidence.InvoiceNumber, evidence.State, evidence.PurchaseKind, evidence.TotalMinorUnits})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		TenantID, InvoiceID, InvoiceNumber, State, PurchaseKind, Currency string
		TotalMinorUnits, Version                                          int64
	}{uuid.UUID(evidence.TenantID).String(), uuid.UUID(evidence.InvoiceID).String(), evidence.InvoiceNumber, evidence.State, evidence.PurchaseKind, "KES", evidence.TotalMinorUnits, evidence.Version})
	if err != nil {
		return err
	}
	at, policyVersion := controlTime(evidence.OccurredAt), int64(1)
	if err = tx.queries.InsertRoutedControlAudit(ctx, controlsql.InsertRoutedControlAuditParams{ID: controlID(evidence.AuditID), TenantID: controlID(evidence.TenantID), ActorID: controlID(evidence.ActorID), Action: "CommercialInvoiceIssued", TargetType: "commercial_invoice", TargetID: controlID(evidence.InvoiceID), OccurredAt: at, CorrelationID: controlID(evidence.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion}); err != nil {
		return err
	}
	return tx.queries.InsertRoutedControlOutbox(ctx, controlsql.InsertRoutedControlOutboxParams{ID: controlID(evidence.OutboxID), TenantID: controlID(evidence.TenantID), ActorID: controlID(evidence.ActorID), AggregateType: "commercial_invoice", AggregateID: controlID(evidence.InvoiceID), AggregateVersion: evidence.Version, EventType: "commercial.invoice.issued.v1", SchemaVersion: 1, Classification: "CONFIDENTIAL", Payload: payload, OccurredAt: at})
}

func (tx *EvidenceTx) AppendCommercialSubscriptionLifecycle(ctx context.Context, evidence controlapp.CommercialSubscriptionLifecycleEvidence) error {
	if ctx == nil || evidence.AuditID.IsZero() || evidence.OutboxID.IsZero() || evidence.CorrelationID.IsZero() || evidence.TenantID.IsZero() || evidence.SubscriptionID.IsZero() || evidence.ActorID.IsZero() || evidence.Operation == "" || evidence.EventType == "" || evidence.State == "" || evidence.Version < 2 || evidence.OccurredAt.IsZero() {
		return errors.New("commercial_subscription_lifecycle_evidence_invalid")
	}
	cancelAt := ""
	if evidence.CancelAt != nil {
		cancelAt = evidence.CancelAt.UTC().Format(time.RFC3339Nano)
	}
	summary, err := json.Marshal(struct {
		Operation, State, CancelAt string
		Version                    int64
	}{evidence.Operation, evidence.State, cancelAt, evidence.Version})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		TenantID, SubscriptionID, State, CancelAt string
		Version                                   int64
	}{uuid.UUID(evidence.TenantID).String(), uuid.UUID(evidence.SubscriptionID).String(), evidence.State, cancelAt, evidence.Version})
	if err != nil {
		return err
	}
	at, policyVersion := controlTime(evidence.OccurredAt), int64(1)
	if err = tx.queries.InsertControlAudit(ctx, controlsql.InsertControlAuditParams{ID: controlID(evidence.AuditID), ActorID: controlID(evidence.ActorID), Action: evidence.Operation, TargetType: "commercial_subscription", TargetID: controlID(evidence.SubscriptionID), OccurredAt: at, CorrelationID: controlID(evidence.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion}); err != nil {
		return err
	}
	return tx.queries.InsertControlOutbox(ctx, controlsql.InsertControlOutboxParams{ID: controlID(evidence.OutboxID), ActorID: controlID(evidence.ActorID), AggregateType: "commercial_subscription", AggregateID: controlID(evidence.SubscriptionID), AggregateVersion: evidence.Version, EventType: evidence.EventType, SchemaVersion: 1, Classification: "INTERNAL", Payload: payload, OccurredAt: at})
}

func (tx *EvidenceTx) AppendCommercialTrial(ctx context.Context, evidence controlapp.CommercialTrialEvidence) error {
	if ctx == nil || evidence.AuditID.IsZero() || evidence.OutboxID.IsZero() || evidence.CorrelationID.IsZero() || evidence.TenantID.IsZero() || evidence.SubscriptionID.IsZero() || evidence.InitiatingAdminID.IsZero() || evidence.ExecutionID.IsZero() || evidence.TrialEndsAt.IsZero() || evidence.OccurredAt.IsZero() {
		return errors.New("commercial_trial_evidence_invalid")
	}
	summary, err := json.Marshal(struct {
		PlanKey, CatalogueVersion string
		TrialEndsAt               string
	}{"TRIAL", "PHASE1_V1", evidence.TrialEndsAt.UTC().Format(time.RFC3339Nano)})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		TenantID, SubscriptionID, InitiatingAdminID, ExecutionID, TrialEndsAt string
	}{uuid.UUID(evidence.TenantID).String(), uuid.UUID(evidence.SubscriptionID).String(), uuid.UUID(evidence.InitiatingAdminID).String(), uuid.UUID(evidence.ExecutionID).String(), evidence.TrialEndsAt.UTC().Format(time.RFC3339Nano)})
	if err != nil {
		return err
	}
	at, policyVersion := controlTime(evidence.OccurredAt), int64(1)
	if err = tx.queries.InsertControlAudit(ctx, controlsql.InsertControlAuditParams{ID: controlID(evidence.AuditID), ActorID: controlID(evidence.InitiatingAdminID), Action: "CommercialTrialStarted", TargetType: "commercial_subscription", TargetID: controlID(evidence.SubscriptionID), OccurredAt: at, CorrelationID: controlID(evidence.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion}); err != nil {
		return err
	}
	return tx.queries.InsertControlOutbox(ctx, controlsql.InsertControlOutboxParams{ID: controlID(evidence.OutboxID), ActorID: controlID(evidence.InitiatingAdminID), AggregateType: "commercial_subscription", AggregateID: controlID(evidence.SubscriptionID), AggregateVersion: 1, EventType: "commercial.trial.started.v1", SchemaVersion: 1, Classification: "INTERNAL", Payload: payload, OccurredAt: at})
}

func (tx *EvidenceTx) AppendPlatformBootstrap(ctx context.Context, evidence controlapp.PlatformBootstrapEvidence) error {
	if ctx == nil || evidence.AuditID.IsZero() || evidence.OutboxID.IsZero() || evidence.CorrelationID.IsZero() || evidence.OwnerUserID.IsZero() || evidence.GovernanceVersion < 1 || evidence.OccurredAt.IsZero() {
		return errors.New("platform_bootstrap_evidence_invalid")
	}
	summary, err := json.Marshal(struct {
		RoleKey           string `json:"role_key"`
		ContextGeneration int64  `json:"context_generation"`
	}{RoleKey: "PLATFORM_OWNER", ContextGeneration: 1})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		UserID            string `json:"user_id"`
		RoleKey           string `json:"role_key"`
		ContextGeneration int64  `json:"context_generation"`
	}{UserID: uuid.UUID(evidence.OwnerUserID).String(), RoleKey: "PLATFORM_OWNER", ContextGeneration: 1})
	if err != nil {
		return err
	}
	at := controlTime(evidence.OccurredAt)
	policyVersion := int64(1)
	if err = tx.queries.InsertControlAudit(ctx, controlsql.InsertControlAuditParams{
		ID: controlID(evidence.AuditID), ActorID: pgtype.UUID{}, Action: "PlatformOwnerBootstrapped",
		TargetType: "platform_administrator", TargetID: controlID(evidence.OwnerUserID), OccurredAt: at,
		CorrelationID: controlID(evidence.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion,
	}); err != nil {
		return err
	}
	return tx.queries.InsertControlOutbox(ctx, controlsql.InsertControlOutboxParams{
		ID: controlID(evidence.OutboxID), ActorID: pgtype.UUID{}, AggregateType: "platform_governance",
		AggregateID: controlID(evidence.OwnerUserID), AggregateVersion: evidence.GovernanceVersion,
		EventType: "platform.owner.bootstrapped.v1", SchemaVersion: 1, Classification: "INTERNAL",
		Payload: payload, OccurredAt: at,
	})
}

func (tx *EvidenceTx) AppendPlatformInvitation(ctx context.Context, evidence controlapp.PlatformInvitationEvidence) error {
	if ctx == nil || evidence.AuditID.IsZero() || evidence.OutboxID.IsZero() || evidence.CorrelationID.IsZero() || evidence.ActorID.IsZero() || evidence.InvitationID.IsZero() || evidence.RoleKey == "" || evidence.Version < 1 || evidence.OccurredAt.IsZero() {
		return errors.New("platform_invitation_evidence_invalid")
	}
	auditAction, eventType := "", ""
	switch evidence.Action {
	case "ISSUED":
		auditAction, eventType = "PlatformAdministratorInvitationIssued", "platform.administrator_invitation.issued.v1"
	case "CANCELLED":
		auditAction, eventType = "PlatformAdministratorInvitationCancelled", "platform.administrator_invitation.cancelled.v1"
	case "ACTIVATED":
		auditAction, eventType = "PlatformOwnerInvitationActivated", "platform.owner_invitation.activated.v1"
	case "REJECTED":
		auditAction, eventType = "PlatformOwnerInvitationRejected", "platform.owner_invitation.rejected.v1"
	case "ACCEPTED":
		auditAction, eventType = "PlatformAdministratorInvitationAccepted", "platform.administrator_invitation.accepted.v1"
	default:
		return errors.New("platform_invitation_evidence_invalid")
	}
	summary, err := json.Marshal(struct {
		RoleKey string `json:"role_key"`
	}{evidence.RoleKey})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		InvitationID string `json:"invitation_id"`
		RoleKey      string `json:"role_key"`
	}{uuid.UUID(evidence.InvitationID).String(), evidence.RoleKey})
	if err != nil {
		return err
	}
	at, policyVersion := controlTime(evidence.OccurredAt), int64(1)
	if err = tx.queries.InsertControlAudit(ctx, controlsql.InsertControlAuditParams{
		ID: controlID(evidence.AuditID), ActorID: controlID(evidence.ActorID), Action: auditAction,
		TargetType: "platform_administrator_invitation", TargetID: controlID(evidence.InvitationID), OccurredAt: at,
		CorrelationID: controlID(evidence.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion,
	}); err != nil {
		return err
	}
	return tx.queries.InsertControlOutbox(ctx, controlsql.InsertControlOutboxParams{
		ID: controlID(evidence.OutboxID), ActorID: controlID(evidence.ActorID), AggregateType: "platform_administrator_invitation",
		AggregateID: controlID(evidence.InvitationID), AggregateVersion: evidence.Version,
		EventType: eventType, SchemaVersion: 1, Classification: "INTERNAL", Payload: payload, OccurredAt: at,
	})
}

func (tx *EvidenceTx) AppendPlatformRoleChange(ctx context.Context, evidence controlapp.PlatformRoleChangeEvidence) error {
	if ctx == nil || evidence.AuditID.IsZero() || evidence.OutboxID.IsZero() || evidence.CorrelationID.IsZero() || evidence.ActorID.IsZero() || evidence.TargetUserID.IsZero() || evidence.ChangeID.IsZero() || evidence.RoleKey == "" || (evidence.State != "EXECUTED" && evidence.State != "PENDING_APPROVAL" && evidence.State != "REJECTED") || evidence.Version < 1 || evidence.OccurredAt.IsZero() {
		return errors.New("platform_role_change_evidence_invalid")
	}
	auditAction, eventType := "", ""
	switch evidence.Action {
	case "ASSIGN_ROLE":
		auditAction, eventType = "PlatformRoleAssignmentChanged", "platform.role_assignment.proposed.v1"
		if evidence.State == "EXECUTED" {
			eventType = "platform.role_assignment.executed.v1"
		}
		if evidence.State == "REJECTED" {
			eventType = "platform.role_assignment.rejected.v1"
		}
	case "REVOKE_ROLE":
		auditAction, eventType = "PlatformRoleRevocationChanged", "platform.role_revocation.proposed.v1"
		if evidence.State == "EXECUTED" {
			eventType = "platform.role_revocation.executed.v1"
		}
		if evidence.State == "REJECTED" {
			eventType = "platform.role_revocation.rejected.v1"
		}
	default:
		return errors.New("platform_role_change_evidence_invalid")
	}
	summary, err := json.Marshal(struct{ RoleKey, Action, State string }{evidence.RoleKey, evidence.Action, evidence.State})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct{ ChangeID, TargetUserID, RoleKey, Action, State string }{uuid.UUID(evidence.ChangeID).String(), uuid.UUID(evidence.TargetUserID).String(), evidence.RoleKey, evidence.Action, evidence.State})
	if err != nil {
		return err
	}
	at, policyVersion := controlTime(evidence.OccurredAt), int64(1)
	if err = tx.queries.InsertControlAudit(ctx, controlsql.InsertControlAuditParams{ID: controlID(evidence.AuditID), ActorID: controlID(evidence.ActorID), Action: auditAction, TargetType: "platform_administrator_change", TargetID: controlID(evidence.ChangeID), OccurredAt: at, CorrelationID: controlID(evidence.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion}); err != nil {
		return err
	}
	return tx.queries.InsertControlOutbox(ctx, controlsql.InsertControlOutboxParams{ID: controlID(evidence.OutboxID), ActorID: controlID(evidence.ActorID), AggregateType: "platform_administrator_change", AggregateID: controlID(evidence.ChangeID), AggregateVersion: evidence.Version, EventType: eventType, SchemaVersion: 1, Classification: "INTERNAL", Payload: payload, OccurredAt: at})
}

func (tx *EvidenceTx) AppendPlatformIdentityStateChange(ctx context.Context, evidence controlapp.PlatformIdentityStateEvidence) error {
	if ctx == nil || evidence.AuditID.IsZero() || evidence.OutboxID.IsZero() || evidence.CorrelationID.IsZero() || evidence.ActorID.IsZero() || evidence.TargetUserID.IsZero() || evidence.ChangeID.IsZero() || (evidence.State != "EXECUTED" && evidence.State != "PENDING_APPROVAL" && evidence.State != "REJECTED") || evidence.Version < 1 || evidence.OccurredAt.IsZero() {
		return errors.New("platform_identity_state_evidence_invalid")
	}
	auditAction, eventType := "", ""
	switch evidence.Action {
	case "DISABLE_IDENTITY":
		auditAction, eventType = "PlatformAdministratorDisabled", "platform.administrator.disable.proposed.v1"
	case "REACTIVATE_IDENTITY":
		auditAction, eventType = "PlatformAdministratorReactivated", "platform.administrator.reactivate.proposed.v1"
	case "DELETE_IDENTITY":
		if evidence.State != "EXECUTED" {
			return errors.New("platform_identity_state_evidence_invalid")
		}
		auditAction, eventType = "PlatformAdministratorDeleted", "platform.administrator.deleted.v1"
	default:
		return errors.New("platform_identity_state_evidence_invalid")
	}
	if evidence.State == "EXECUTED" {
		eventType = "platform.administrator." + map[string]string{"DISABLE_IDENTITY": "disabled", "REACTIVATE_IDENTITY": "reactivated", "DELETE_IDENTITY": "deleted"}[evidence.Action] + ".v1"
	}
	if evidence.State == "REJECTED" {
		eventType = "platform.administrator.identity_change.rejected.v1"
	}
	summary, err := json.Marshal(struct{ Action, State string }{evidence.Action, evidence.State})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct{ ChangeID, TargetUserID, Action, State string }{uuid.UUID(evidence.ChangeID).String(), uuid.UUID(evidence.TargetUserID).String(), evidence.Action, evidence.State})
	if err != nil {
		return err
	}
	at, policyVersion := controlTime(evidence.OccurredAt), int64(1)
	if err = tx.queries.InsertControlAudit(ctx, controlsql.InsertControlAuditParams{ID: controlID(evidence.AuditID), ActorID: controlID(evidence.ActorID), Action: auditAction, TargetType: "platform_administrator_change", TargetID: controlID(evidence.ChangeID), OccurredAt: at, CorrelationID: controlID(evidence.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion}); err != nil {
		return err
	}
	return tx.queries.InsertControlOutbox(ctx, controlsql.InsertControlOutboxParams{ID: controlID(evidence.OutboxID), ActorID: controlID(evidence.ActorID), AggregateType: "platform_administrator_change", AggregateID: controlID(evidence.ChangeID), AggregateVersion: evidence.Version, EventType: eventType, SchemaVersion: 1, Classification: "INTERNAL", Payload: payload, OccurredAt: at})
}

func (tx *EvidenceTx) AppendPlatformAdministratorSessionRevocation(ctx context.Context, evidence controlapp.PlatformAdministratorSessionRevocationEvidence) error {
	if ctx == nil || evidence.AuditID.IsZero() || evidence.OutboxID.IsZero() || evidence.CorrelationID.IsZero() || evidence.ActorID.IsZero() || evidence.TargetUserID.IsZero() || evidence.TargetAuthEpoch < 1 || evidence.Reason == "" || evidence.OccurredAt.IsZero() {
		return errors.New("platform_session_revocation_evidence_invalid")
	}
	summary, err := json.Marshal(struct {
		Reason string `json:"reason"`
	}{evidence.Reason})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		TargetUserID string `json:"target_user_id"`
	}{uuid.UUID(evidence.TargetUserID).String()})
	if err != nil {
		return err
	}
	at, policyVersion := controlTime(evidence.OccurredAt), int64(1)
	if err = tx.queries.InsertControlAudit(ctx, controlsql.InsertControlAuditParams{ID: controlID(evidence.AuditID), ActorID: controlID(evidence.ActorID), Action: "PlatformAdministratorSessionsRevoked", TargetType: "platform_administrator", TargetID: controlID(evidence.TargetUserID), OccurredAt: at, CorrelationID: controlID(evidence.CorrelationID), ChangeSummary: summary, PolicyVersion: &policyVersion}); err != nil {
		return err
	}
	return tx.queries.InsertControlOutbox(ctx, controlsql.InsertControlOutboxParams{ID: controlID(evidence.OutboxID), ActorID: controlID(evidence.ActorID), AggregateType: "platform_administrator", AggregateID: controlID(evidence.TargetUserID), AggregateVersion: evidence.TargetAuthEpoch, EventType: "platform.administrator.sessions_revoked.v1", SchemaVersion: 1, Classification: "INTERNAL", Payload: payload, OccurredAt: at})
}

func controlID(id controlapp.ID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: !id.IsZero()}
}
func controlTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}
