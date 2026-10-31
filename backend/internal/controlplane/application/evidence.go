// Package application defines infrastructure-free shared control-plane audit
// and outbox append values. It grants no business authority.
package application

import "time"

type ID [16]byte

func (id ID) IsZero() bool { return id == ID{} }

type PlatformBootstrapEvidence struct {
	AuditID, OutboxID, CorrelationID, OwnerUserID ID
	GovernanceVersion                             int64
	OccurredAt                                    time.Time
}

type PlatformIntegrationSettingEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TargetID ID
	Action, EventType, Kind, State, ResultCode          string
	Version                                             int64
	OccurredAt                                          time.Time
}

type InboxReceipt struct {
	ID, EventID, AggregateID, TenantID     ID
	Consumer, EventType, AggregateType     string
	SchemaVersion                          int32
	AggregateVersion, AssignmentGeneration int64
	RequestHash                            [32]byte
	OccurredAt, ProcessedAt                time.Time
}

type CommercialAIDeniedEvidence struct {
	AuditID, OutboxID, CorrelationID, CheckpointID, TenantID, RequesterUserID ID
	Kind, DenialCode                                                          string
	AssignmentGeneration, EstimatedCredits                                    int64
	OccurredAt                                                                time.Time
}

type CommercialAIUsageEvidence struct {
	AuditID, OutboxID, CorrelationID, CheckpointID, TenantID, ReservationID, RequesterUserID, ExecutionID ID
	Operation, EventType, Kind, Status                                                                    string
	AssignmentGeneration, EstimatedCredits, ActualCredits                                                 int64
	UsageVersion, ReservationVersion                                                                      int64
	OccurredAt                                                                                            time.Time
}

type CommercialSubscriptionLifecycleEvidence struct {
	AuditID, OutboxID, CorrelationID, TenantID, SubscriptionID, ActorID ID
	Operation, EventType, State                                         string
	Version                                                             int64
	CancelAt                                                            *time.Time
	OccurredAt                                                          time.Time
}

type CommercialInvoiceEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TenantID, InvoiceID ID
	InvoiceNumber, State, PurchaseKind                             string
	Version, TotalMinorUnits                                       int64
	OccurredAt                                                     time.Time
}

type CommercialPaymentEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TenantID, InvoiceID, PaymentID ID
	EventType, State, ReconciliationState                                     string
	Version, AmountMinorUnits                                                 int64
	OccurredAt                                                                time.Time
}

type CommercialFinancialReadEvidence struct {
	AuditID, CorrelationID, ActorID, TenantID, TargetID ID
	Action, TargetType                                  string
	OccurredAt                                          time.Time
}

type CommercialPlatformQueueReadEvidence struct {
	AuditID, CorrelationID, ActorID, TargetID ID
	Queue, StateFilter, SecondaryFilter       string
	ResultCount                               int
	HasMore                                   bool
	OccurredAt                                time.Time
}

type CommercialReversalEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TenantID, InvoiceID, PaymentID, SubscriptionID, ObservationID, AccessHoldID ID
	EventType, PaymentState, InvoiceState, SubscriptionState, AccessMode                                                   string
	PaymentVersion, InvoiceVersion, SubscriptionVersion, AmountMinorUnits                                                  int64
	OccurredAt                                                                                                             time.Time
}

type CommercialAdjustmentEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TenantID, InvoiceID, PaymentID, ChangeID ID
	EventType, State, Action, ReasonCode, EntitlementDisposition                        string
	Version, AmountMinorUnits                                                           int64
	OccurredAt                                                                          time.Time
}

type CommercialTrialEvidence struct {
	AuditID, OutboxID, CorrelationID, TenantID, SubscriptionID, InitiatingAdminID, ExecutionID ID
	TrialEndsAt                                                                                time.Time
	OccurredAt                                                                                 time.Time
}

type PlatformInvitationEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, InvitationID ID
	Action, RoleKey                                         string
	Version                                                 int64
	OccurredAt                                              time.Time
}

type PlatformRoleChangeEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TargetUserID, ChangeID ID
	Action, RoleKey, State                                            string
	Version                                                           int64
	OccurredAt                                                        time.Time
}
type PlatformIdentityStateEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TargetUserID, ChangeID ID
	Action, State                                                     string
	Version                                                           int64
	OccurredAt                                                        time.Time
}
type PlatformAdministratorSessionRevocationEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TargetUserID ID
	TargetAuthEpoch                                         int64
	Reason                                                  string
	OccurredAt                                              time.Time
}
