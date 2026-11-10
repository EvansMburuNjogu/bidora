package application

import (
	"bidora/backend/internal/identity/securitypolicy"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"bidora/backend/internal/commercial/domain"
	pa "bidora/backend/internal/platformadmin/application"
	pd "bidora/backend/internal/platformadmin/domain"
	"github.com/google/uuid"
)

var ErrPlatformSubscriptionInvalid = errors.New("platform_subscription_invalid")
var ErrPlatformSubscriptionForbidden = errors.New("platform_subscription_forbidden")
var ErrPlatformSubscriptionNotFound = errors.New("platform_subscription_not_found")
var ErrPlatformSubscriptionConflict = errors.New("platform_subscription_conflict")
var ErrPlatformSubscriptionUnavailable = errors.New("platform_subscription_unavailable")

// PlatformSubscriptionInventory contains control-owned commercial metadata only.
// It never opens a tenant database or returns tenant-private bids/documents.
type PlatformSubscriptionInventoryRow struct {
	TenantName               string
	TenantID, SubscriptionID domain.ID
	State                    domain.SubscriptionState
	Version                  int64
	PlanKey                  domain.PlanKey
	Cadence                  domain.BillingCadence
	HasCurrentTerm           bool
	PeriodEnd, CancelAt      *time.Time
	UpdatedAt                time.Time
}
type PlatformSubscriptionInventoryPage struct {
	Rows []PlatformSubscriptionInventoryRow
	Next *domain.ID
}
type PlatformSubscriptionInventoryInput struct {
	Subject       PlatformSubscriptionSubject
	AfterID       domain.ID // zero only on the first page
	Limit         int32
	State         string // empty or an exact subscription state
	Plan, Cadence string // empty or exact catalogue values
}

type PlatformSubscriptionSubject struct {
	UserID, SessionID, CorrelationID domain.ID
	ContextGeneration                int64
}
type PlatformSubscriptionChange struct {
	Subject                   PlatformSubscriptionSubject
	TenantID                  domain.ID
	ExpectedVersion           int64
	Operation, IdempotencyKey string
}
type PlatformSubscriptionReceipt struct {
	TenantID, SubscriptionID domain.ID
	State                    domain.SubscriptionState
	Version                  int64
	CancelAt                 *time.Time
	Replay                   bool
}
type PlatformSubscriptionCommand struct {
	RequestHash [32]byte
	Receipt     PlatformSubscriptionReceipt
}
type PlatformSubscriptionCommit struct {
	ActorID, CorrelationID, AuditID, OutboxID, TransitionID domain.ID
	Operation                                               string
	Before, After                                           SubscriptionSummary
	At                                                      time.Time
	KeyHash, RequestHash                                    [32]byte
}
type PlatformSubscriptionTx interface {
	ResolvePlatformAuthority(context.Context, pa.AuthorityQuery) (pa.AuthorityFacts, error)
	ReadPlatformSubscription(context.Context, domain.ID, bool) (SubscriptionSummary, bool, error)
	ListPlatformSubscriptionInventory(context.Context, domain.ID, int32, string) ([]PlatformSubscriptionInventoryRow, error)
	AppendPlatformSubscriptionInventoryRead(context.Context, domain.ID, domain.ID, domain.ID, int32, string, time.Time) error
	GetPlatformSubscriptionCommand(context.Context, domain.ID, string, [32]byte) (PlatformSubscriptionCommand, bool, error)
	CommitPlatformSubscription(context.Context, PlatformSubscriptionCommit) error
	AppendPlatformSubscriptionRead(context.Context, domain.ID, domain.ID, domain.ID, domain.ID, time.Time) error
}
type filteredSubscriptionInventoryTx interface {
	ListFilteredPlatformSubscriptionInventory(context.Context, domain.ID, int32, string, string, string) ([]PlatformSubscriptionInventoryRow, error)
}
type PlatformSubscriptionUnitOfWork interface {
	WithinPlatformSubscription(context.Context, func(context.Context, PlatformSubscriptionTx) error) error
}
type PlatformSubscriptionService struct {
	uow   PlatformSubscriptionUnitOfWork
	ids   PlanIDs
	clock PlanClock
}

func NewPlatformSubscriptionService(uow PlatformSubscriptionUnitOfWork, ids PlanIDs, clock PlanClock) (*PlatformSubscriptionService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, ErrPlatformSubscriptionUnavailable
	}
	return &PlatformSubscriptionService{uow, ids, clock}, nil
}
func (s *PlatformSubscriptionService) authorize(ctx context.Context, tx PlatformSubscriptionTx, subject PlatformSubscriptionSubject, capability string, at time.Time) error {
	facts, e := tx.ResolvePlatformAuthority(ctx, pa.AuthorityQuery{UserID: pa.ID(subject.UserID), SessionID: pa.ID(subject.SessionID), ObservedAt: at})
	if errors.Is(e, pa.ErrAuthorityNotFound) {
		return ErrPlatformSubscriptionForbidden
	}
	if e != nil {
		return e
	}
	if facts.UserID != pa.ID(subject.UserID) || facts.ContextGeneration != subject.ContextGeneration || !facts.Grants(capability) || !pd.CurrentAdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod, facts.AssuredAt, at) || capability != "platform.subscription.read" && !securitypolicy.Recent(ctx, facts.AssuredAt, at) {
		return ErrPlatformSubscriptionForbidden
	}
	return nil
}
func validPlatformSubscriptionSubject(s PlatformSubscriptionSubject) bool {
	return !s.UserID.IsZero() && !s.SessionID.IsZero() && !s.CorrelationID.IsZero() && s.ContextGeneration > 0
}
func receiptFromSummary(v SubscriptionSummary) PlatformSubscriptionReceipt {
	return PlatformSubscriptionReceipt{TenantID: v.Subscription.TenantID, SubscriptionID: v.SubscriptionID, State: v.Subscription.State, Version: v.Subscription.Version, CancelAt: v.Subscription.CancelAt}
}
func (s *PlatformSubscriptionService) Inventory(ctx context.Context, in PlatformSubscriptionInventoryInput) (out PlatformSubscriptionInventoryPage, err error) {
	if s == nil || ctx == nil || !validPlatformSubscriptionSubject(in.Subject) || in.Limit < 1 || in.Limit > 50 || !platformSubscriptionStateFilter(in.State) || !platformSubscriptionPlanFilter(in.Plan) || !platformSubscriptionCadenceFilter(in.Cadence) {
		return out, ErrPlatformSubscriptionInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return out, ErrPlatformSubscriptionUnavailable
	}
	err = s.uow.WithinPlatformSubscription(ctx, func(c context.Context, tx PlatformSubscriptionTx) error {
		if e := s.authorize(c, tx, in.Subject, "platform.subscription.read", at); e != nil {
			return e
		}
		var rows []PlatformSubscriptionInventoryRow
		var e error
		if in.Plan != "" || in.Cadence != "" {
			filtered, ok := tx.(filteredSubscriptionInventoryTx)
			if !ok {
				return ErrPlatformSubscriptionUnavailable
			}
			rows, e = filtered.ListFilteredPlatformSubscriptionInventory(c, in.AfterID, in.Limit+1, in.State, in.Plan, in.Cadence)
		} else {
			rows, e = tx.ListPlatformSubscriptionInventory(c, in.AfterID, in.Limit+1, in.State)
		}
		if e != nil {
			return e
		}
		if len(rows) > int(in.Limit+1) {
			return ErrPlatformSubscriptionUnavailable
		}
		for i, row := range rows {
			if row.SubscriptionID.IsZero() || row.TenantID.IsZero() || row.Version < 1 || row.UpdatedAt.IsZero() || i > 0 && bytes.Compare(rows[i-1].SubscriptionID[:], row.SubscriptionID[:]) >= 0 || bytes.Compare(row.SubscriptionID[:], in.AfterID[:]) <= 0 || !platformSubscriptionStateFilter(string(row.State)) || row.State == "" || in.State != "" && string(row.State) != in.State || in.Plan != "" && string(row.PlanKey) != in.Plan || in.Cadence != "" && string(row.Cadence) != in.Cadence || row.HasCurrentTerm != (row.PeriodEnd != nil) || row.HasCurrentTerm && (row.PlanKey == "" || row.Cadence == "") {
				return ErrPlatformSubscriptionUnavailable
			}
		}
		if len(rows) > int(in.Limit) {
			next := rows[in.Limit-1].SubscriptionID
			out.Next = &next
			rows = rows[:in.Limit]
		}
		out.Rows = rows
		id, e := s.ids.NewID()
		if e != nil || id.IsZero() {
			return ErrPlatformSubscriptionUnavailable
		}
		return tx.AppendPlatformSubscriptionInventoryRead(c, id, in.Subject.UserID, in.Subject.CorrelationID, int32(len(rows)), in.State, at)
	})
	if err != nil {
		return PlatformSubscriptionInventoryPage{}, platformSubscriptionError(err)
	}
	return out, nil
}
func platformSubscriptionPlanFilter(value string) bool {
	switch value {
	case "", "TRIAL", "BASIC", "GROWTH", "PROFESSIONAL", "ENTERPRISE":
		return true
	}
	return false
}
func platformSubscriptionCadenceFilter(value string) bool {
	switch value {
	case "", "TRIAL", "MONTHLY", "ANNUAL", "CONTRACT":
		return true
	}
	return false
}
func platformSubscriptionStateFilter(value string) bool {
	switch value {
	case "", "TRIAL_ACTIVE", "PAYMENT_PENDING", "ACTIVE", "PAST_DUE", "SUSPENDED", "ENDED", "REVIEW_REQUIRED":
		return true
	default:
		return false
	}
}
func (s *PlatformSubscriptionService) Read(ctx context.Context, subject PlatformSubscriptionSubject, tenantID domain.ID) (out SubscriptionSummary, err error) {
	if s == nil || ctx == nil || !validPlatformSubscriptionSubject(subject) || tenantID.IsZero() {
		return out, ErrPlatformSubscriptionInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return out, ErrPlatformSubscriptionUnavailable
	}
	err = s.uow.WithinPlatformSubscription(ctx, func(c context.Context, tx PlatformSubscriptionTx) error {
		if e := s.authorize(c, tx, subject, "platform.subscription.read", at); e != nil {
			return e
		}
		v, found, e := tx.ReadPlatformSubscription(c, tenantID, false)
		if e != nil {
			return e
		}
		if !found {
			return ErrPlatformSubscriptionNotFound
		}
		if v.Subscription.TenantID != tenantID || v.SubscriptionID.IsZero() {
			return ErrPlatformSubscriptionUnavailable
		}
		id, e := s.ids.NewID()
		if e != nil || id.IsZero() {
			return ErrPlatformSubscriptionUnavailable
		}
		if e = tx.AppendPlatformSubscriptionRead(c, id, subject.UserID, subject.CorrelationID, v.SubscriptionID, at); e != nil {
			return e
		}
		out = v
		return nil
	})
	if err != nil {
		return SubscriptionSummary{}, platformSubscriptionError(err)
	}
	return out, nil
}
func (s *PlatformSubscriptionService) Change(ctx context.Context, in PlatformSubscriptionChange) (out PlatformSubscriptionReceipt, err error) {
	if s == nil || ctx == nil || !validPlatformSubscriptionSubject(in.Subject) || in.TenantID.IsZero() || in.ExpectedVersion < 1 || (in.Operation != "SCHEDULE_CANCELLATION" && in.Operation != "WITHDRAW_CANCELLATION") || len(in.IdempotencyKey) > 128 {
		return out, ErrPlatformSubscriptionInvalid
	}
	key, e := uuid.Parse(in.IdempotencyKey)
	if e != nil || key == uuid.Nil || key.String() != in.IdempotencyKey {
		return out, ErrPlatformSubscriptionInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return out, ErrPlatformSubscriptionUnavailable
	}
	keyHash := sha256.Sum256([]byte("platform-subscription:" + in.Operation + ":" + in.IdempotencyKey))
	body, _ := json.Marshal(struct {
		Actor, Tenant        domain.ID
		Generation, Expected int64
		Operation            string
	}{in.Subject.UserID, in.TenantID, in.Subject.ContextGeneration, in.ExpectedVersion, in.Operation})
	hash := sha256.Sum256(append([]byte("platform-subscription:v1:"), body...))
	err = s.uow.WithinPlatformSubscription(ctx, func(c context.Context, tx PlatformSubscriptionTx) error {
		if e := s.authorize(c, tx, in.Subject, "platform.subscription.manage", at); e != nil {
			return e
		}
		old, found, e := tx.GetPlatformSubscriptionCommand(c, in.Subject.UserID, in.Operation, keyHash)
		if e != nil {
			return e
		}
		if found {
			if old.RequestHash != hash {
				return ErrPlatformSubscriptionConflict
			}
			out = old.Receipt
			out.Replay = true
			return nil
		}
		before, found, e := tx.ReadPlatformSubscription(c, in.TenantID, true)
		if e != nil {
			return e
		}
		if !found {
			return ErrPlatformSubscriptionNotFound
		}
		if before.Subscription.TenantID != in.TenantID || before.Subscription.Version != in.ExpectedVersion {
			return ErrPlatformSubscriptionConflict
		}
		after := before
		if in.Operation == "SCHEDULE_CANCELLATION" {
			e = after.Subscription.ScheduleCancellation()
		} else {
			e = after.Subscription.WithdrawCancellation(at)
		}
		if e != nil {
			return ErrPlatformSubscriptionConflict
		}
		ids := make([]domain.ID, 3)
		for i := range ids {
			ids[i], e = s.ids.NewID()
			if e != nil || ids[i].IsZero() {
				return ErrPlatformSubscriptionUnavailable
			}
		}
		commit := PlatformSubscriptionCommit{ActorID: in.Subject.UserID, CorrelationID: in.Subject.CorrelationID, AuditID: ids[0], OutboxID: ids[1], TransitionID: ids[2], Operation: in.Operation, Before: before, After: after, At: at, KeyHash: keyHash, RequestHash: hash}
		if e = tx.CommitPlatformSubscription(c, commit); e != nil {
			return e
		}
		out = receiptFromSummary(after)
		return nil
	})
	if err != nil {
		return PlatformSubscriptionReceipt{}, platformSubscriptionError(err)
	}
	return out, nil
}
func platformSubscriptionError(e error) error {
	switch {
	case errors.Is(e, ErrPlatformSubscriptionInvalid), errors.Is(e, ErrPlatformSubscriptionForbidden), errors.Is(e, ErrPlatformSubscriptionNotFound), errors.Is(e, ErrPlatformSubscriptionConflict):
		return e
	default:
		return ErrPlatformSubscriptionUnavailable
	}
}
