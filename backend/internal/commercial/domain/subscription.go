package domain

import (
	"errors"
	"time"
)

type ID [16]byte

func (id ID) IsZero() bool { return id == ID{} }

type SubscriptionState string

type AccessMode string

const (
	TrialActive    SubscriptionState = "TRIAL_ACTIVE"
	PaymentPending SubscriptionState = "PAYMENT_PENDING"
	Active         SubscriptionState = "ACTIVE"
	PastDue        SubscriptionState = "PAST_DUE"
	Suspended      SubscriptionState = "SUSPENDED"
	Ended          SubscriptionState = "ENDED"
	ReviewRequired SubscriptionState = "REVIEW_REQUIRED"

	FullAccess           AccessMode = "FULL"
	RetainedReadOnly     AccessMode = "RETAINED_READ_ONLY"
	LastReconciledAccess AccessMode = "LAST_RECONCILED"

	PastDueGrace = 72 * time.Hour
)

var ErrSubscriptionInvalid = errors.New("commercial_subscription_invalid")
var ErrSubscriptionConflict = errors.New("commercial_subscription_conflict")
var ErrTrialIneligible = errors.New("commercial_trial_ineligible")
var ErrReconciliationRequired = errors.New("commercial_reconciliation_required")
var ErrContractTermsRequired = errors.New("commercial_contract_terms_required")

type LimitSnapshot struct {
	BidWorkspaces, IncludedLicences, StorageBytes, DocumentVersions, AICredits int64
	ConcurrentAIJobs                                                           int32
}

type TermSnapshot struct {
	PlanKey                  PlanKey
	CatalogueVersion         string
	Cadence                  BillingCadence
	Price                    Money
	PriceKind                string
	AdditionalLicenceMonthly *Money
	Limits                   LimitSnapshot
	CommercialPeriodStart    time.Time
	CommercialPeriodEnd      time.Time
	UsagePeriodStart         time.Time
	UsagePeriodEnd           time.Time
}

type TrialEligibility struct {
	TenantActive, ProvisioningComplete, EffectiveOrganizationAdmin bool
	TrialPreviouslyStarted                                         bool
}

type Subscription struct {
	TenantID                    ID
	State                       SubscriptionState
	Term                        TermSnapshot
	Version                     int64
	TrialStartedAt, TrialEndsAt *time.Time
	GraceEndsAt, CancelAt       *time.Time
	PendingPlan                 *PlanKey
	PendingCadence              *BillingCadence
	LastReconciledState         SubscriptionState
	LastReconciledAt            *time.Time
}

func StartTrial(tenantID ID, eligibility TrialEligibility, at time.Time) (Subscription, error) {
	if tenantID.IsZero() || at.IsZero() || !eligibility.TenantActive || !eligibility.ProvisioningComplete || !eligibility.EffectiveOrganizationAdmin || eligibility.TrialPreviouslyStarted {
		return Subscription{}, ErrTrialIneligible
	}
	at = at.UTC()
	term, err := NewCatalogueTerm(Trial, TrialCadence, at)
	if err != nil {
		return Subscription{}, err
	}
	ends := term.CommercialPeriodEnd
	return Subscription{TenantID: tenantID, State: TrialActive, Term: term, Version: 1, TrialStartedAt: timePointer(at), TrialEndsAt: timePointer(ends), LastReconciledState: TrialActive, LastReconciledAt: timePointer(at)}, nil
}

func NewPaymentPending(tenantID ID, plan PlanKey, cadence BillingCadence) (Subscription, error) {
	if tenantID.IsZero() || !validPendingSelection(plan, cadence) {
		return Subscription{}, ErrSubscriptionInvalid
	}
	return Subscription{TenantID: tenantID, State: PaymentPending, Version: 1, PendingPlan: planPointer(plan), PendingCadence: cadencePointer(cadence)}, nil
}

func (s *Subscription) BeginPayment(plan PlanKey, cadence BillingCadence) error {
	if !s.valid() || !validPendingSelection(plan, cadence) || s.PendingPlan != nil || s.CancelAt != nil || s.State == Suspended || s.State == Ended || s.State == ReviewRequired || s.State == PaymentPending {
		return ErrSubscriptionConflict
	}
	s.PendingPlan, s.PendingCadence = planPointer(plan), cadencePointer(cadence)
	s.Version++
	return nil
}

func (s *Subscription) CancelPendingPayment() error {
	if !s.valid() || s.PendingPlan == nil || s.PendingCadence == nil {
		return ErrSubscriptionConflict
	}
	s.PendingPlan, s.PendingCadence = nil, nil
	if s.State == PaymentPending {
		s.State = Ended
	}
	s.Version++
	return nil
}

func NewPaidSubscription(tenantID ID, term TermSnapshot, reconciled bool, at time.Time) (Subscription, error) {
	if tenantID.IsZero() || at.IsZero() || !reconciled || !validPaidTerm(term) {
		if !reconciled {
			return Subscription{}, ErrReconciliationRequired
		}
		return Subscription{}, ErrSubscriptionInvalid
	}
	at = at.UTC()
	return Subscription{TenantID: tenantID, State: Active, Term: cloneTerm(term), Version: 1, LastReconciledState: Active, LastReconciledAt: timePointer(at)}, nil
}

func (s *Subscription) ActivatePaid(term TermSnapshot, reconciled bool, at time.Time) error {
	if !s.valid() || at.IsZero() || !validPaidTerm(term) {
		return ErrSubscriptionInvalid
	}
	if s.PendingPlan != nil && (s.PendingCadence == nil || *s.PendingPlan != term.PlanKey || *s.PendingCadence != term.Cadence) {
		return ErrSubscriptionConflict
	}
	if !reconciled {
		return ErrReconciliationRequired
	}
	s.State = Active
	s.Term = cloneTerm(term)
	s.GraceEndsAt = nil
	s.CancelAt = nil
	s.PendingPlan = nil
	s.PendingCadence = nil
	s.LastReconciledState = Active
	s.LastReconciledAt = timePointer(at.UTC())
	s.Version++
	return nil
}

func (s *Subscription) MarkRenewalUnpaid(at time.Time) error {
	if !s.valid() || s.State != Active || s.CancelAt != nil || at.IsZero() || at.UTC().Before(s.Term.CommercialPeriodEnd) {
		return ErrSubscriptionConflict
	}
	at = at.UTC()
	s.State = PastDue
	s.GraceEndsAt = timePointer(s.Term.CommercialPeriodEnd.Add(PastDueGrace))
	s.Version++
	return nil
}

func (s *Subscription) SuspendOverdue(at time.Time) error {
	if !s.valid() || s.State != PastDue || s.GraceEndsAt == nil || at.IsZero() || at.UTC().Before(*s.GraceEndsAt) {
		return ErrSubscriptionConflict
	}
	s.State = Suspended
	s.GraceEndsAt = nil
	s.Version++
	return nil
}

func (s *Subscription) ResumePaid(term TermSnapshot, reconciled bool, at time.Time) error {
	if s.State != PastDue && s.State != Suspended && s.State != ReviewRequired {
		return ErrSubscriptionConflict
	}
	return s.ActivatePaid(term, reconciled, at)
}

func (s *Subscription) ScheduleCancellation() error {
	if !s.valid() || (s.State != TrialActive && s.State != Active) || s.CancelAt != nil || s.PendingPlan != nil {
		return ErrSubscriptionConflict
	}
	s.CancelAt = timePointer(s.Term.CommercialPeriodEnd)
	s.Version++
	return nil
}

func (s *Subscription) WithdrawCancellation(at time.Time) error {
	if !s.valid() || s.CancelAt == nil || at.IsZero() || !at.UTC().Before(*s.CancelAt) || s.State == Ended {
		return ErrSubscriptionConflict
	}
	s.CancelAt = nil
	s.Version++
	return nil
}

func (s *Subscription) EndAtBoundary(at time.Time) error {
	if !s.valid() || at.IsZero() {
		return ErrSubscriptionInvalid
	}
	boundary := s.Term.CommercialPeriodEnd
	if s.CancelAt != nil {
		boundary = *s.CancelAt
	} else if s.State != TrialActive {
		return ErrSubscriptionConflict
	}
	if at.UTC().Before(boundary) {
		return ErrSubscriptionConflict
	}
	s.State = Ended
	s.GraceEndsAt = nil
	s.CancelAt = nil
	s.Version++
	return nil
}

func (s *Subscription) RequireReview(at time.Time) error {
	if !s.valid() || at.IsZero() || s.State == Ended || s.State == ReviewRequired || s.State == PaymentPending {
		return ErrSubscriptionConflict
	}
	if s.State == TrialActive || s.State == Active || s.State == PastDue {
		s.LastReconciledState = s.State
	}
	s.State = ReviewRequired
	s.Version++
	return nil
}

func (s Subscription) Access() AccessMode {
	switch s.State {
	case TrialActive, Active, PastDue:
		return FullAccess
	case ReviewRequired:
		return LastReconciledAccess
	default:
		return RetainedReadOnly
	}
}

func NewCatalogueTerm(key PlanKey, cadence BillingCadence, startsAt time.Time) (TermSnapshot, error) {
	if key == Enterprise {
		return TermSnapshot{}, ErrContractTermsRequired
	}
	// Source defaults are only authoritative for the fixed Trial. Paid terms
	// must be built from a published version supplied by CatalogueService.
	if key != Trial {
		return TermSnapshot{}, ErrSubscriptionInvalid
	}
	plan, err := FindPlan(key)
	if err != nil {
		return TermSnapshot{}, err
	}
	return NewTermFromPlan(plan, cadence, startsAt)
}

// NewTermFromPlan snapshots the actual published plan supplied by the owner
// catalogue service; existing terms never change when a new version publishes.
func NewTermFromPlan(plan Plan, cadence BillingCadence, startsAt time.Time) (TermSnapshot, error) {
	key := plan.Key
	if startsAt.IsZero() || key == Enterprise {
		if key == Enterprise {
			return TermSnapshot{}, ErrContractTermsRequired
		}
		return TermSnapshot{}, ErrSubscriptionInvalid
	}
	price, err := PriceForPlan(plan, cadence)
	if err != nil {
		return TermSnapshot{}, err
	}
	if plan.BidWorkspacesPerPeriod == nil || plan.IncludedLicences == nil || plan.StorageLimitBytes == nil || plan.DocumentVersionLimit == nil || plan.AICreditLimit == nil || plan.ConcurrentAIJobLimit == nil {
		return TermSnapshot{}, ErrSubscriptionInvalid
	}
	startsAt = startsAt.UTC()
	commercialEnd, usageEnd := time.Time{}, time.Time{}
	switch cadence {
	case TrialCadence:
		commercialEnd = startsAt.Add(14 * 24 * time.Hour)
		usageEnd = commercialEnd
	case MonthlyCadence:
		commercialEnd = MonthBoundary(startsAt, 1)
		usageEnd = commercialEnd
	case AnnualCadence:
		commercialEnd = MonthBoundary(startsAt, 12)
		usageEnd = MonthBoundary(startsAt, 1)
	default:
		return TermSnapshot{}, ErrCadenceUnavailable
	}
	kind := "FIXED"
	return TermSnapshot{PlanKey: key, CatalogueVersion: plan.CatalogueVersion, Cadence: cadence, Price: price, PriceKind: kind, AdditionalLicenceMonthly: cloneMoneyValue(plan.AdditionalLicenceMonthlyPrice), Limits: LimitSnapshot{BidWorkspaces: int64(*plan.BidWorkspacesPerPeriod), IncludedLicences: int64(*plan.IncludedLicences), StorageBytes: *plan.StorageLimitBytes, DocumentVersions: *plan.DocumentVersionLimit, AICredits: *plan.AICreditLimit, ConcurrentAIJobs: *plan.ConcurrentAIJobLimit}, CommercialPeriodStart: startsAt, CommercialPeriodEnd: commercialEnd, UsagePeriodStart: startsAt, UsagePeriodEnd: usageEnd}, nil
}

func NewEnterpriseTerm(startsAt, endsAt, usageEndsAt time.Time, price Money, limits LimitSnapshot) (TermSnapshot, error) {
	startsAt, endsAt, usageEndsAt = startsAt.UTC(), endsAt.UTC(), usageEndsAt.UTC()
	if startsAt.IsZero() || !endsAt.After(startsAt) || !usageEndsAt.After(startsAt) || usageEndsAt.After(endsAt) || price.Currency != CurrencyKES || price.MinorUnits < 6000000 || !validLimits(limits) {
		return TermSnapshot{}, ErrContractTermsRequired
	}
	return TermSnapshot{PlanKey: Enterprise, CatalogueVersion: CatalogueVersion, Cadence: ContractCadence, Price: price, PriceKind: "FIXED", Limits: limits, CommercialPeriodStart: startsAt, CommercialPeriodEnd: endsAt, UsagePeriodStart: startsAt, UsagePeriodEnd: usageEndsAt}, nil
}

func MonthBoundary(anchor time.Time, months int) time.Time {
	if anchor.IsZero() || months < 1 {
		return time.Time{}
	}
	anchor = anchor.UTC()
	first := time.Date(anchor.Year(), anchor.Month(), 1, anchor.Hour(), anchor.Minute(), anchor.Second(), anchor.Nanosecond(), time.UTC)
	targetFirst := first.AddDate(0, months, 0)
	lastDay := targetFirst.AddDate(0, 1, -1).Day()
	day := anchor.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(targetFirst.Year(), targetFirst.Month(), day, anchor.Hour(), anchor.Minute(), anchor.Second(), anchor.Nanosecond(), time.UTC)
}

func (s Subscription) valid() bool {
	if s.TenantID.IsZero() || s.Version < 1 || s.State == "" {
		return false
	}
	if s.State == PaymentPending {
		return s.PendingPlan != nil && s.PendingCadence != nil && validPendingSelection(*s.PendingPlan, *s.PendingCadence)
	}
	return !s.Term.CommercialPeriodStart.IsZero() && s.Term.CommercialPeriodEnd.After(s.Term.CommercialPeriodStart)
}

func validPendingSelection(plan PlanKey, cadence BillingCadence) bool {
	if plan == Trial {
		return false
	}
	if plan == Enterprise {
		return cadence == ContractCadence
	}
	return (plan == Basic || plan == Growth || plan == Professional) && (cadence == MonthlyCadence || cadence == AnnualCadence)
}

func validPaidTerm(term TermSnapshot) bool {
	_, planErr := ParsePlan(string(term.PlanKey))
	return planErr == nil && term.PlanKey != Trial && term.CatalogueVersion != "" && term.Cadence != TrialCadence && term.Price.Currency == CurrencyKES && term.Price.MinorUnits >= 0 && validLimits(term.Limits) && term.CommercialPeriodEnd.After(term.CommercialPeriodStart) && term.UsagePeriodEnd.After(term.UsagePeriodStart) && !term.UsagePeriodEnd.After(term.CommercialPeriodEnd)
}

func validLimits(limits LimitSnapshot) bool {
	return limits.BidWorkspaces > 0 && limits.IncludedLicences > 0 && limits.StorageBytes > 0 && limits.DocumentVersions > 0 && limits.AICredits > 0 && limits.ConcurrentAIJobs > 0
}

func cloneTerm(term TermSnapshot) TermSnapshot {
	term.AdditionalLicenceMonthly = cloneMoneyValue(term.AdditionalLicenceMonthly)
	return term
}

func cloneMoneyValue(value *Money) *Money {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func planPointer(value PlanKey) *PlanKey                  { return &value }
func cadencePointer(value BillingCadence) *BillingCadence { return &value }

func timePointer(value time.Time) *time.Time {
	value = value.UTC()
	return &value
}
