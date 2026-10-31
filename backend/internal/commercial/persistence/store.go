// Package persistence composes owner adapters inside one control-plane transaction.
package persistence

import (
	"context"
	"errors"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	authzpersistence "bidora/backend/internal/authorization/persistence"
	commercialapp "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	commercialsql "bidora/backend/internal/commercial/persistence/sqlc"
	controlapp "bidora/backend/internal/controlplane/application"
	controlpersistence "bidora/backend/internal/controlplane/persistence"
	identityapp "bidora/backend/internal/identity/application"
	identitypersistence "bidora/backend/internal/identity/persistence"
	platformpersistence "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	tenancyapp "bidora/backend/internal/tenancy/application"
	tenancypersistence "bidora/backend/internal/tenancy/persistence"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type transactionRunner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}

type Store struct{ runner transactionRunner }

func NewStore(runner transactionRunner) (*Store, error) {
	if runner == nil {
		return nil, errors.New("commercial_store_invalid")
	}
	return &Store{runner: runner}, nil
}

func (s *Store) WithinTrialStart(ctx context.Context, fn func(context.Context, commercialapp.TrialStartTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("commercial_trial_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		tx, err := newTrialTx(db)
		if err != nil {
			return err
		}
		return fn(txCtx, tx)
	})
}

type trialTx struct {
	commercial         *commercialsql.Queries
	identity           *identitypersistence.CommercialTrialIdentityTx
	authority          *authzpersistence.CommercialTrialAuthorityTx
	tenant             *tenancypersistence.CommercialTrialTenantTx
	evidence           *controlpersistence.EvidenceTx
	consumer           *controlpersistence.ConsumerTx
	aiAuthority        *authzpersistence.AIEntitlementAuthorityTx
	aiTenant           *tenancypersistence.AIEntitlementTenantTx
	financialAuthority *platformpersistence.FinancialAuthorityTx
}

func newTrialTx(db postgres.DBTX) (*trialTx, error) {
	identity, err := identitypersistence.NewCommercialTrialIdentityTx(db)
	if err != nil {
		return nil, err
	}
	authority, err := authzpersistence.NewCommercialTrialAuthorityTx(db)
	if err != nil {
		return nil, err
	}
	tenant, err := tenancypersistence.NewCommercialTrialTenantTx(db)
	if err != nil {
		return nil, err
	}
	evidence, err := controlpersistence.NewEvidenceTx(db)
	if err != nil {
		return nil, err
	}
	consumer, err := controlpersistence.NewConsumerTx(db)
	if err != nil {
		return nil, err
	}
	aiAuthority, err := authzpersistence.NewAIEntitlementAuthorityTx(db)
	if err != nil {
		return nil, err
	}
	aiTenant, err := tenancypersistence.NewAIEntitlementTenantTx(db)
	if err != nil {
		return nil, err
	}
	financialAuthority, err := platformpersistence.NewFinancialAuthorityTx(db)
	if err != nil {
		return nil, err
	}
	return &trialTx{commercial: commercialsql.New(db), identity: identity, authority: authority, tenant: tenant, evidence: evidence, consumer: consumer, aiAuthority: aiAuthority, aiTenant: aiTenant, financialAuthority: financialAuthority}, nil
}

func (tx *trialTx) LockTenant(ctx context.Context, tenantID domain.ID) (commercialapp.TrialTenantFacts, error) {
	facts, err := tx.tenant.LockEligibility(ctx, tenancyapp.ID(tenantID))
	return commercialapp.TrialTenantFacts{Active: facts.Active, Tenderpreneur: facts.Tenderpreneur, ProvisioningComplete: facts.ProvisioningComplete}, err
}
func (tx *trialTx) LockIdentity(ctx context.Context, userID domain.ID) (bool, error) {
	return tx.identity.LockEligible(ctx, identityapp.ID(userID))
}
func (tx *trialTx) LockMembership(ctx context.Context, userID, tenantID domain.ID, contextGeneration int64) (domain.ID, bool, error) {
	id, active, err := tx.authority.LockActiveMembership(ctx, authzapp.ID(userID), authzapp.ID(tenantID), contextGeneration)
	return domain.ID(id), active, err
}
func (tx *trialTx) CapabilityAllowed(ctx context.Context, membershipID domain.ID) (bool, error) {
	return tx.authority.SubscriptionManageAllowed(ctx, authzapp.ID(membershipID))
}
func (tx *trialTx) GetCommand(ctx context.Context, tenantID domain.ID, keyHash [32]byte) (commercialapp.TrialCommandRecord, bool, error) {
	row, err := tx.commercial.GetTrialStartCommand(ctx, commercialsql.GetTrialStartCommandParams{TenantID: commercialID(tenantID), KeyHash: keyHash[:]})
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.TrialCommandRecord{}, false, nil
	}
	if err != nil {
		return commercialapp.TrialCommandRecord{}, false, err
	}
	if len(row.RequestHash) != 32 || !row.TrialEndsAt.Valid {
		return commercialapp.TrialCommandRecord{}, false, errors.New("commercial_trial_command_invalid")
	}
	var requestHash [32]byte
	copy(requestHash[:], row.RequestHash)
	return commercialapp.TrialCommandRecord{RequestHash: requestHash, SubscriptionID: domain.ID(row.SubscriptionID.Bytes), ResponseVersion: row.ResponseVersion, State: domain.SubscriptionState(row.State), TrialEndsAt: row.TrialEndsAt.Time.UTC()}, true, nil
}
func (tx *trialTx) SubscriptionExists(ctx context.Context, tenantID domain.ID) (bool, error) {
	return tx.commercial.CommercialSubscriptionExists(ctx, commercialID(tenantID))
}
func (tx *trialTx) TrialMarkerExists(ctx context.Context, tenantID domain.ID) (bool, error) {
	return tx.commercial.CommercialTrialMarkerExists(ctx, commercialID(tenantID))
}
func (tx *trialTx) InsertTrial(ctx context.Context, c commercialapp.TrialStartCommit) error {
	if c.Subscription.TrialStartedAt == nil || c.Subscription.TrialEndsAt == nil || c.Subscription.Term.PlanKey != domain.Trial || c.Subscription.Term.CatalogueVersion != domain.CatalogueVersion || c.Subscription.Version != 1 || c.KeyHash == [32]byte{} || c.RequestHash == [32]byte{} {
		return errors.New("commercial_trial_commit_invalid")
	}
	started, ended := commercialTime(*c.Subscription.TrialStartedAt), commercialTime(*c.Subscription.TrialEndsAt)
	if err := tx.commercial.InsertTrialSubscription(ctx, commercialsql.InsertTrialSubscriptionParams{ID: commercialID(c.SubscriptionID), TenantID: commercialID(c.Subscription.TenantID), TermID: commercialID(c.TermID), StartedAt: started, EndsAt: ended}); err != nil {
		return err
	}
	limits := c.Subscription.Term.Limits
	if err := tx.commercial.InsertTrialTerm(ctx, commercialsql.InsertTrialTermParams{ID: commercialID(c.TermID), SubscriptionID: commercialID(c.SubscriptionID), CatalogueVersion: c.Subscription.Term.CatalogueVersion, BidWorkspaceLimit: limits.BidWorkspaces, IncludedLicenceLimit: limits.IncludedLicences, StorageLimitBytes: limits.StorageBytes, DocumentVersionLimit: limits.DocumentVersions, AiCreditLimit: limits.AICredits, ConcurrentAiJobLimit: limits.ConcurrentAIJobs, PeriodStart: started, PeriodEnd: ended}); err != nil {
		return err
	}
	if err := tx.commercial.InsertTrialUsagePeriod(ctx, commercialsql.InsertTrialUsagePeriodParams{ID: commercialID(c.UsagePeriodID), SubscriptionID: commercialID(c.SubscriptionID), TermID: commercialID(c.TermID), PeriodStart: started, PeriodEnd: ended}); err != nil {
		return err
	}
	if err := tx.commercial.InsertTrialLifetimeMarker(ctx, commercialsql.InsertTrialLifetimeMarkerParams{TenantID: commercialID(c.Subscription.TenantID), SubscriptionID: commercialID(c.SubscriptionID), InitiatingUserID: commercialID(c.InitiatingAdminID), ExecutionID: commercialID(c.ExecutionID), MarkedAt: commercialTime(c.OccurredAt)}); err != nil {
		return err
	}
	return tx.commercial.InsertTrialStartCommand(ctx, commercialsql.InsertTrialStartCommandParams{ID: commercialID(c.CommandID), TenantID: commercialID(c.Subscription.TenantID), KeyHash: c.KeyHash[:], RequestHash: c.RequestHash[:], SubscriptionID: commercialID(c.SubscriptionID), At: commercialTime(c.OccurredAt)})
}
func (tx *trialTx) AppendTrialEvidence(ctx context.Context, c commercialapp.TrialStartCommit) error {
	return tx.evidence.AppendCommercialTrial(ctx, controlapp.CommercialTrialEvidence{AuditID: controlapp.ID(c.AuditID), OutboxID: controlapp.ID(c.OutboxID), CorrelationID: controlapp.ID(c.CorrelationID), TenantID: controlapp.ID(c.Subscription.TenantID), SubscriptionID: controlapp.ID(c.SubscriptionID), InitiatingAdminID: controlapp.ID(c.InitiatingAdminID), ExecutionID: controlapp.ID(c.ExecutionID), TrialEndsAt: *c.Subscription.TrialEndsAt, OccurredAt: c.OccurredAt})
}

func (s *Store) WithinSubscription(ctx context.Context, fn func(context.Context, commercialapp.SubscriptionTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("commercial_subscription_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		tx, err := newTrialTx(db)
		if err != nil {
			return err
		}
		return fn(txCtx, tx)
	})
}

func (tx *trialTx) Authorize(ctx context.Context, access commercialapp.SubscriptionAccess, capability string) (bool, error) {
	tenant, err := tx.tenant.LockEligibility(ctx, tenancyapp.ID(access.TenantID))
	if err != nil || !tenant.Active || !tenant.Tenderpreneur || !tenant.ProvisioningComplete {
		return false, err
	}
	identity, err := tx.identity.LockEligible(ctx, identityapp.ID(access.ActorID))
	if err != nil || !identity {
		return false, err
	}
	return tx.authority.AuthorizeSubscription(ctx, authzapp.ID(access.ActorID), authzapp.ID(access.TenantID), access.ContextGeneration, capability)
}
func (tx *trialTx) Read(ctx context.Context, tenantID domain.ID, forUpdate bool) (commercialapp.SubscriptionSummary, bool, error) {
	if forUpdate {
		if _, err := tx.commercial.LockCommercialSubscription(ctx, commercialID(tenantID)); errors.Is(err, pgx.ErrNoRows) {
			return commercialapp.SubscriptionSummary{}, false, nil
		} else if err != nil {
			return commercialapp.SubscriptionSummary{}, false, err
		}
	}
	row, err := tx.commercial.GetCommercialSubscriptionSummary(ctx, commercialID(tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.SubscriptionSummary{}, false, nil
	}
	if err != nil {
		return commercialapp.SubscriptionSummary{}, false, err
	}
	return mapSubscriptionSummary(row), true, nil
}
func (tx *trialTx) GetLifecycleCommand(ctx context.Context, tenantID domain.ID, operation string, keyHash [32]byte) (commercialapp.SubscriptionLifecycleRecord, bool, error) {
	row, err := tx.commercial.GetSubscriptionLifecycleCommand(ctx, commercialsql.GetSubscriptionLifecycleCommandParams{TenantID: commercialID(tenantID), Operation: operation, KeyHash: keyHash[:]})
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.SubscriptionLifecycleRecord{}, false, nil
	}
	if err != nil {
		return commercialapp.SubscriptionLifecycleRecord{}, false, err
	}
	if len(row.RequestHash) != 32 {
		return commercialapp.SubscriptionLifecycleRecord{}, false, errors.New("commercial_subscription_command_invalid")
	}
	var hash [32]byte
	copy(hash[:], row.RequestHash)
	summary, found, err := tx.Read(ctx, tenantID, false)
	if err != nil || !found {
		return commercialapp.SubscriptionLifecycleRecord{}, false, err
	}
	if summary.SubscriptionID != domain.ID(row.SubscriptionID.Bytes) || summary.Subscription.Version != row.ResponseVersion {
		return commercialapp.SubscriptionLifecycleRecord{}, false, errors.New("commercial_subscription_command_result_invalid")
	}
	return commercialapp.SubscriptionLifecycleRecord{RequestHash: hash, Summary: summary}, true, nil
}
func (tx *trialTx) CommitLifecycle(ctx context.Context, c commercialapp.SubscriptionLifecycleCommit) error {
	if c.After.Subscription.Version != c.Before.Subscription.Version+1 || c.After.SubscriptionID != c.Before.SubscriptionID {
		return errors.New("commercial_subscription_lifecycle_commit_invalid")
	}
	var rows int64
	var err error
	switch c.Operation {
	case "SCHEDULE_CANCELLATION":
		if c.After.Subscription.CancelAt == nil {
			return errors.New("commercial_subscription_cancel_at_missing")
		}
		rows, err = tx.commercial.ScheduleSubscriptionCancellation(ctx, commercialsql.ScheduleSubscriptionCancellationParams{CancelAt: commercialTime(*c.After.Subscription.CancelAt), UpdatedAt: commercialTime(c.OccurredAt), ID: commercialID(c.After.SubscriptionID), ExpectedVersion: c.Before.Subscription.Version})
	case "WITHDRAW_CANCELLATION":
		rows, err = tx.commercial.WithdrawSubscriptionCancellation(ctx, commercialsql.WithdrawSubscriptionCancellationParams{UpdatedAt: commercialTime(c.OccurredAt), ID: commercialID(c.After.SubscriptionID), ExpectedVersion: c.Before.Subscription.Version})
	default:
		return errors.New("commercial_subscription_operation_invalid")
	}
	if err != nil {
		return err
	}
	if rows != 1 {
		return commercialapp.ErrSubscriptionLifecycleConflict
	}
	if err = tx.commercial.InsertSubscriptionTransition(ctx, commercialsql.InsertSubscriptionTransitionParams{ID: commercialID(c.TransitionID), SubscriptionID: commercialID(c.After.SubscriptionID), FromState: string(c.Before.Subscription.State), ToState: string(c.After.Subscription.State), Action: c.Operation, SubscriptionVersion: c.After.Subscription.Version, ActorUserID: commercialID(c.ActorID), OccurredAt: commercialTime(c.OccurredAt)}); err != nil {
		return err
	}
	return tx.commercial.InsertSubscriptionLifecycleCommand(ctx, commercialsql.InsertSubscriptionLifecycleCommandParams{ID: commercialID(c.CommandID), TenantID: commercialID(c.After.Subscription.TenantID), Operation: c.Operation, KeyHash: c.KeyHash[:], RequestHash: c.RequestHash[:], SubscriptionID: commercialID(c.After.SubscriptionID), ResponseVersion: c.After.Subscription.Version, At: commercialTime(c.OccurredAt)})
}
func (tx *trialTx) AppendLifecycleEvidence(ctx context.Context, c commercialapp.SubscriptionLifecycleCommit) error {
	return tx.evidence.AppendCommercialSubscriptionLifecycle(ctx, controlapp.CommercialSubscriptionLifecycleEvidence{AuditID: controlapp.ID(c.AuditID), OutboxID: controlapp.ID(c.OutboxID), CorrelationID: controlapp.ID(c.CorrelationID), TenantID: controlapp.ID(c.After.Subscription.TenantID), SubscriptionID: controlapp.ID(c.After.SubscriptionID), ActorID: controlapp.ID(c.ActorID), Operation: c.Operation, EventType: c.EventType, State: string(c.After.Subscription.State), Version: c.After.Subscription.Version, CancelAt: c.After.Subscription.CancelAt, OccurredAt: c.OccurredAt})
}
func mapSubscriptionSummary(row commercialsql.GetCommercialSubscriptionSummaryRow) commercialapp.SubscriptionSummary {
	pointer := func(value pgtype.Timestamptz) *time.Time {
		if !value.Valid {
			return nil
		}
		v := value.Time.UTC()
		return &v
	}
	return commercialapp.SubscriptionSummary{SubscriptionID: domain.ID(row.ID.Bytes), Subscription: domain.Subscription{TenantID: domain.ID(row.TenantID.Bytes), State: domain.SubscriptionState(row.State), Version: row.Version, TrialStartedAt: pointer(row.TrialStartedAt), TrialEndsAt: pointer(row.TrialEndsAt), GraceEndsAt: pointer(row.GraceEndsAt), CancelAt: pointer(row.CancelAt), Term: domain.TermSnapshot{PlanKey: domain.PlanKey(row.PlanKey), CatalogueVersion: row.CatalogueVersion, Cadence: domain.BillingCadence(row.Cadence), Price: domain.Money{Currency: row.Currency, MinorUnits: row.PriceMinorUnits}, PriceKind: "FIXED", Limits: domain.LimitSnapshot{BidWorkspaces: row.BidWorkspaceLimit, IncludedLicences: row.IncludedLicenceLimit, StorageBytes: row.StorageLimitBytes, DocumentVersions: row.DocumentVersionLimit, AICredits: row.AiCreditLimit, ConcurrentAIJobs: row.ConcurrentAiJobLimit}, CommercialPeriodStart: row.CommercialPeriodStart.Time.UTC(), CommercialPeriodEnd: row.CommercialPeriodEnd.Time.UTC(), UsagePeriodStart: row.UsagePeriodStart.Time.UTC(), UsagePeriodEnd: row.UsagePeriodEnd.Time.UTC()}}, Usage: commercialapp.UsageSummary{BidWorkspaces: row.BidWorkspacesConsumed, DocumentVersions: row.DocumentVersionsConsumed, AICredits: row.AiCreditsConsumed, AICreditsReserved: row.AiCreditsReserved, StorageBytes: row.StorageBytesUsed, ConcurrentAIJobs: row.ConcurrentAiJobs, Version: row.UsageVersion}}
}

func (s *Store) WithinAIReservationConsumption(ctx context.Context, fn func(context.Context, commercialapp.AIReservationConsumerTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("commercial_ai_reservation_consumer_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		tx, err := newTrialTx(db)
		if err != nil {
			return err
		}
		return fn(txCtx, tx)
	})
}
func (tx *trialTx) FindAIReservationOutcome(ctx context.Context, eventID domain.ID) (commercialapp.AIReservationOutcome, bool, error) {
	row, err := tx.commercial.GetAIReservationRequestOutcome(ctx, commercialID(eventID))
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.AIReservationOutcome{}, false, nil
	}
	if err != nil {
		return commercialapp.AIReservationOutcome{}, false, err
	}
	value, err := mapAIReservationOutcome(row)
	return value, err == nil, err
}
func (tx *trialTx) FindAIReservationOutcomeByCheckpoint(ctx context.Context, checkpointID domain.ID) (commercialapp.AIReservationOutcome, bool, error) {
	row, err := tx.commercial.GetAIReservationRequestOutcomeByCheckpoint(ctx, commercialID(checkpointID))
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.AIReservationOutcome{}, false, nil
	}
	if err != nil {
		return commercialapp.AIReservationOutcome{}, false, err
	}
	value, err := mapAIReservationOutcome(row)
	return value, err == nil, err
}
func mapAIReservationOutcome(row commercialsql.CommercialAiReservationRequestOutcome) (commercialapp.AIReservationOutcome, error) {
	if len(row.RequestHash) != 32 || !row.ProcessedAt.Valid {
		return commercialapp.AIReservationOutcome{}, errors.New("commercial_ai_reservation_outcome_invalid")
	}
	var hash [32]byte
	copy(hash[:], row.RequestHash)
	denial := ""
	if row.DenialCode != nil {
		denial = *row.DenialCode
	}
	return commercialapp.AIReservationOutcome{EventID: domain.ID(row.EventID.Bytes), CheckpointID: domain.ID(row.CheckpointID.Bytes), TenantID: domain.ID(row.TenantID.Bytes), SourceOperationID: domain.ID(row.SourceOperationID.Bytes), RequesterUserID: domain.ID(row.RequesterUserID.Bytes), ReservationID: domain.ID(row.ReservationID.Bytes), RequestHash: hash, AssignmentGeneration: row.AssignmentGeneration, Disposition: row.Disposition, DenialCode: denial, ProcessedAt: row.ProcessedAt.Time.UTC()}, nil
}
func (tx *trialTx) LockAIEntitlementAssignment(ctx context.Context, tenantID domain.ID) (int64, bool, error) {
	facts, found, err := tx.aiTenant.LockAssignment(ctx, tenancyapp.ID(tenantID))
	if err != nil || !found {
		return 0, false, err
	}
	return facts.Generation, facts.Eligible, nil
}
func (tx *trialTx) AIRequestCapabilityAllowed(ctx context.Context, userID, tenantID domain.ID, generation int64, capability string) (bool, error) {
	return tx.aiAuthority.CurrentCapabilityAllowed(ctx, authzapp.ID(userID), authzapp.ID(tenantID), generation, capability)
}
func (tx *trialTx) RecordAIReservationConsumption(ctx context.Context, c commercialapp.AIReservationConsumptionCommit) error {
	e := c.Event
	o := c.Outcome
	if err := tx.consumer.InsertInbox(ctx, controlapp.InboxReceipt{ID: controlapp.ID(c.InboxID), EventID: controlapp.ID(e.EventID), AggregateID: controlapp.ID(e.AggregateID), TenantID: controlapp.ID(e.TenantID), Consumer: commercialapp.AIReservationConsumerName, EventType: e.EventType, AggregateType: e.AggregateType, SchemaVersion: e.SchemaVersion, AggregateVersion: e.AggregateVersion, AssignmentGeneration: e.AssignmentGeneration, RequestHash: e.RequestHash, OccurredAt: e.OccurredAt, ProcessedAt: c.ProcessedAt}); err != nil {
		return err
	}
	var reservation pgtype.UUID
	var denial *string
	if o.Disposition == "RESERVED" {
		reservation = commercialID(o.ReservationID)
	} else {
		denial = &o.DenialCode
	}
	if err := tx.commercial.InsertAIReservationRequestOutcome(ctx, commercialsql.InsertAIReservationRequestOutcomeParams{ID: commercialID(c.OutcomeID), EventID: commercialID(e.EventID), CheckpointID: commercialID(e.CheckpointID), TenantID: commercialID(e.TenantID), SourceOperationID: commercialID(e.SourceOperationID), RequestHash: e.RequestHash[:], AssignmentGeneration: e.AssignmentGeneration, RequesterUserID: commercialID(e.RequesterUserID), Disposition: o.Disposition, ReservationID: reservation, DenialCode: denial, ProcessedAt: commercialTime(c.ProcessedAt)}); err != nil {
		return err
	}
	if o.Disposition == "DENIED" {
		return tx.evidence.AppendCommercialAIDenied(ctx, controlapp.CommercialAIDeniedEvidence{AuditID: controlapp.ID(c.AuditID), OutboxID: controlapp.ID(c.OutboxID), CorrelationID: controlapp.ID(e.CorrelationID), CheckpointID: controlapp.ID(e.CheckpointID), TenantID: controlapp.ID(e.TenantID), RequesterUserID: controlapp.ID(e.RequesterUserID), Kind: string(e.Kind), DenialCode: o.DenialCode, AssignmentGeneration: e.AssignmentGeneration, EstimatedCredits: e.EstimatedCredits, OccurredAt: c.ProcessedAt})
	}
	return nil
}

func (s *Store) WithinAIUsage(ctx context.Context, fn func(context.Context, commercialapp.AIUsageTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("commercial_ai_usage_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		tx, err := newTrialTx(db)
		if err != nil {
			return err
		}
		return fn(txCtx, tx)
	})
}
func (tx *trialTx) LockCurrent(ctx context.Context, tenantID domain.ID) (commercialapp.AIUsageCurrent, bool, error) {
	row, err := tx.commercial.LockCurrentAIUsage(ctx, commercialID(tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.AIUsageCurrent{}, false, nil
	}
	if err != nil {
		return commercialapp.AIUsageCurrent{}, false, err
	}
	last := ""
	if row.LastReconciledState != nil {
		last = *row.LastReconciledState
	}
	return commercialapp.AIUsageCurrent{SubscriptionID: domain.ID(row.SubscriptionID.Bytes), UsagePeriodID: domain.ID(row.UsagePeriodID.Bytes), State: domain.SubscriptionState(row.State), LastReconciledState: domain.SubscriptionState(last), Counters: aiCounters(row.AiCreditLimit, row.AiCreditsConsumed, row.AiCreditsReserved, row.ConcurrentAiJobLimit, row.ConcurrentAiJobs, row.UsagePeriodStart, row.UsagePeriodEnd, row.GraceEndsAt, row.Version)}, true, nil
}
func (tx *trialTx) FindReservation(ctx context.Context, tenantID, sourceOperationID domain.ID) (commercialapp.AIUsageStored, bool, error) {
	row, err := tx.commercial.GetAIUsageReservationByOperation(ctx, commercialsql.GetAIUsageReservationByOperationParams{TenantID: commercialID(tenantID), SourceOperationID: commercialID(sourceOperationID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.AIUsageStored{}, false, nil
	}
	if err != nil {
		return commercialapp.AIUsageStored{}, false, err
	}
	period, err := tx.commercial.GetAIUsagePeriod(ctx, row.UsagePeriodID)
	if err != nil {
		return commercialapp.AIUsageStored{}, false, err
	}
	stored, err := mapAIReservation(row, aiCounters(period.AiCreditLimit, period.AiCreditsConsumed, period.AiCreditsReserved, period.ConcurrentAiJobLimit, period.ConcurrentAiJobs, period.UsagePeriodStart, period.UsagePeriodEnd, pgtype.Timestamptz{}, period.Version))
	return stored, err == nil, err
}
func (tx *trialTx) FindReservationCompletion(ctx context.Context, id domain.ID) (commercialapp.AIUsageCompletionEvidence, bool, error) {
	row, err := tx.commercial.GetAIUsageReservationCompletion(ctx, commercialID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.AIUsageCompletionEvidence{}, false, nil
	}
	if err != nil {
		return commercialapp.AIUsageCompletionEvidence{}, false, err
	}
	if !row.ExecutionID.Valid || !row.OccurredAt.Valid {
		return commercialapp.AIUsageCompletionEvidence{}, false, errors.New("commercial_ai_usage_completion_invalid")
	}
	actual := int64(0)
	if row.ActualCredits != nil {
		actual = *row.ActualCredits
	}
	return commercialapp.AIUsageCompletionEvidence{ExecutionID: domain.ID(row.ExecutionID.Bytes), Status: domain.AIReservationStatus(row.ToStatus), ActualCredits: actual, OccurredAt: row.OccurredAt.Time.UTC()}, true, nil
}
func (tx *trialTx) LockReservation(ctx context.Context, id domain.ID) (commercialapp.AIUsageStored, bool, error) {
	row, err := tx.commercial.LockAIUsageReservation(ctx, commercialID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.AIUsageStored{}, false, nil
	}
	if err != nil {
		return commercialapp.AIUsageStored{}, false, err
	}
	period, err := tx.commercial.LockAIUsagePeriod(ctx, row.UsagePeriodID)
	if err != nil {
		return commercialapp.AIUsageStored{}, false, err
	}
	stored, err := mapAIReservation(row, aiCounters(period.AiCreditLimit, period.AiCreditsConsumed, period.AiCreditsReserved, period.ConcurrentAiJobLimit, period.ConcurrentAiJobs, period.UsagePeriodStart, period.UsagePeriodEnd, pgtype.Timestamptz{}, period.Version))
	return stored, err == nil, err
}
func (tx *trialTx) InsertReservation(ctx context.Context, stored commercialapp.AIUsageStored, auditID, outboxID, correlationID domain.ID, at time.Time) error {
	r := stored.Reservation
	if r.ID.IsZero() || r.CheckpointID.IsZero() || r.AssignmentGeneration < 1 || stored.Counters.Version < 2 || stored.OperationHash == [32]byte{} || stored.RequestHash == [32]byte{} {
		return errors.New("commercial_ai_usage_reservation_commit_invalid")
	}
	rows, err := tx.commercial.ReserveAIUsageCounters(ctx, commercialsql.ReserveAIUsageCountersParams{Credits: r.EstimatedCredits, At: commercialTime(at), UsagePeriodID: commercialID(r.UsagePeriodID), ExpectedVersion: stored.Counters.Version - 1})
	if err != nil {
		return err
	}
	if rows != 1 {
		return commercialapp.ErrAIUsageLimit
	}
	if err = tx.commercial.InsertAIUsageReservation(ctx, commercialsql.InsertAIUsageReservationParams{ID: commercialID(r.ID), CheckpointID: commercialID(r.CheckpointID), AssignmentGeneration: r.AssignmentGeneration, TenantID: commercialID(r.TenantID), SubscriptionID: commercialID(r.SubscriptionID), UsagePeriodID: commercialID(r.UsagePeriodID), SourceOperationID: commercialID(r.SourceOperationID), OperationHash: stored.OperationHash[:], RequestHash: stored.RequestHash[:], RequesterUserID: commercialID(r.RequesterUserID), Kind: string(r.Kind), EstimatedCredits: r.EstimatedCredits, At: commercialTime(at)}); err != nil {
		return err
	}
	return tx.evidence.AppendCommercialAIUsage(ctx, controlapp.CommercialAIUsageEvidence{AuditID: controlapp.ID(auditID), OutboxID: controlapp.ID(outboxID), CorrelationID: controlapp.ID(correlationID), CheckpointID: controlapp.ID(r.CheckpointID), TenantID: controlapp.ID(r.TenantID), ReservationID: controlapp.ID(r.ID), RequesterUserID: controlapp.ID(r.RequesterUserID), Operation: "CommercialAIUsageReserved", EventType: "commercial.ai_usage.reserved.v1", Kind: string(r.Kind), Status: string(r.Status), AssignmentGeneration: r.AssignmentGeneration, EstimatedCredits: r.EstimatedCredits, UsageVersion: stored.Counters.Version, ReservationVersion: r.Version, OccurredAt: at})
}
func (tx *trialTx) CompleteReservation(ctx context.Context, c commercialapp.AIUsageCommit) error {
	r := c.Before.Reservation
	var rows int64
	var err error
	actual := c.After.Reservation.ActualCredits
	if c.Operation == "SETTLE" {
		rows, err = tx.commercial.SettleAIUsageCounters(ctx, commercialsql.SettleAIUsageCountersParams{EstimatedCredits: r.EstimatedCredits, ActualCredits: actual, At: commercialTime(c.OccurredAt), UsagePeriodID: commercialID(r.UsagePeriodID), ExpectedVersion: c.Before.Counters.Version})
	} else if c.Operation == "RELEASE" {
		rows, err = tx.commercial.ReleaseAIUsageCounters(ctx, commercialsql.ReleaseAIUsageCountersParams{EstimatedCredits: r.EstimatedCredits, At: commercialTime(c.OccurredAt), UsagePeriodID: commercialID(r.UsagePeriodID), ExpectedVersion: c.Before.Counters.Version})
	} else {
		return errors.New("commercial_ai_usage_completion_invalid")
	}
	if err != nil {
		return err
	}
	if rows != 1 {
		return commercialapp.ErrAIUsageConflict
	}
	if c.Operation == "SETTLE" {
		rows, err = tx.commercial.SettleAIUsageReservation(ctx, commercialsql.SettleAIUsageReservationParams{ActualCredits: &actual, At: commercialTime(c.OccurredAt), ID: commercialID(r.ID)})
	} else {
		rows, err = tx.commercial.ReleaseAIUsageReservation(ctx, commercialsql.ReleaseAIUsageReservationParams{At: commercialTime(c.OccurredAt), ID: commercialID(r.ID)})
	}
	if err != nil {
		return err
	}
	if rows != 1 {
		return commercialapp.ErrAIUsageConflict
	}
	var actualPtr *int64
	if c.Operation == "SETTLE" {
		actualPtr = &actual
	}
	return tx.commercial.InsertAIUsageReservationTransition(ctx, commercialsql.InsertAIUsageReservationTransitionParams{ID: commercialID(c.TransitionID), ReservationID: commercialID(r.ID), ToStatus: string(c.After.Reservation.Status), ActualCredits: actualPtr, ExecutionID: commercialID(c.ExecutionID), OccurredAt: commercialTime(c.OccurredAt)})
}
func (tx *trialTx) AppendAIUsageEvidence(ctx context.Context, c commercialapp.AIUsageCommit) error {
	r := c.After.Reservation
	return tx.evidence.AppendCommercialAIUsage(ctx, controlapp.CommercialAIUsageEvidence{AuditID: controlapp.ID(c.AuditID), OutboxID: controlapp.ID(c.OutboxID), CorrelationID: controlapp.ID(c.CorrelationID), TenantID: controlapp.ID(r.TenantID), ReservationID: controlapp.ID(r.ID), RequesterUserID: controlapp.ID(r.RequesterUserID), ExecutionID: controlapp.ID(c.ExecutionID), CheckpointID: controlapp.ID(r.CheckpointID), Operation: map[string]string{"SETTLE": "CommercialAIUsageSettled", "RELEASE": "CommercialAIUsageReleased"}[c.Operation], EventType: c.EventType, Kind: string(r.Kind), Status: string(r.Status), AssignmentGeneration: r.AssignmentGeneration, EstimatedCredits: r.EstimatedCredits, ActualCredits: r.ActualCredits, UsageVersion: c.After.Counters.Version, ReservationVersion: r.Version, OccurredAt: c.OccurredAt})
}
func aiCounters(limit, consumed, reserved int64, jobLimit, jobs int32, start, end, grace pgtype.Timestamptz, version int64) domain.AIUsageCounters {
	var graceEnd *time.Time
	if grace.Valid {
		value := grace.Time.UTC()
		graceEnd = &value
	}
	return domain.AIUsageCounters{CreditLimit: limit, ConsumedCredits: consumed, ReservedCredits: reserved, ConcurrentJobLimit: jobLimit, ConcurrentJobs: jobs, PeriodStart: start.Time.UTC(), PeriodEnd: end.Time.UTC(), GraceEndsAt: graceEnd, Version: version}
}
func mapAIReservation(row commercialsql.CommercialAiUsageReservation, counters domain.AIUsageCounters) (commercialapp.AIUsageStored, error) {
	if len(row.OperationHash) != 32 || len(row.RequestHash) != 32 || !row.ReservedAt.Valid {
		return commercialapp.AIUsageStored{}, errors.New("commercial_ai_usage_reservation_invalid")
	}
	var op, req [32]byte
	copy(op[:], row.OperationHash)
	copy(req[:], row.RequestHash)
	actual := int64(0)
	if row.ActualCredits != nil {
		actual = *row.ActualCredits
	}
	var completed *time.Time
	if row.SettledAt.Valid {
		v := row.SettledAt.Time.UTC()
		completed = &v
	} else if row.ReleasedAt.Valid {
		v := row.ReleasedAt.Time.UTC()
		completed = &v
	}
	return commercialapp.AIUsageStored{Reservation: domain.AIUsageReservation{ID: domain.ID(row.ID.Bytes), CheckpointID: domain.ID(row.CheckpointID.Bytes), AssignmentGeneration: row.AssignmentGeneration, TenantID: domain.ID(row.TenantID.Bytes), SubscriptionID: domain.ID(row.SubscriptionID.Bytes), UsagePeriodID: domain.ID(row.UsagePeriodID.Bytes), SourceOperationID: domain.ID(row.SourceOperationID.Bytes), RequesterUserID: domain.ID(row.RequesterUserID.Bytes), Kind: domain.AIUsageKind(row.Kind), EstimatedCredits: row.EstimatedCredits, ActualCredits: actual, Status: domain.AIReservationStatus(row.Status), Version: row.Version, ReservedAt: row.ReservedAt.Time.UTC(), CompletedAt: completed}, Counters: counters, OperationHash: op, RequestHash: req}, nil
}

func commercialID(id domain.ID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(id), Valid: !id.IsZero()}
}
func commercialTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}
