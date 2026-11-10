package application

import (
	platformapp "bidora/backend/internal/platformadmin/application"
	policy "bidora/backend/internal/platformadmin/domain"
	"context"
	"errors"
	"time"
)

type OverviewPermissions struct{ Tenants, Subscriptions, Invoices, Payments, Support bool }
type MonthlyInvoicing struct {
	Month           string `json:"month"`
	TotalMinorUnits string `json:"total_minor_units"`
}
type PlatformOverview struct {
	InvoicedMinorUnits    *string            `json:"invoiced_minor_units"`
	PaidInvoices          *string            `json:"paid_invoices"`
	FailedPayments        *string            `json:"failed_payments"`
	MonthlyInvoicing      []MonthlyInvoicing `json:"monthly_invoicing"`
	Tenderpreneurs        *string            `json:"tenderpreneurs"`
	ActiveSubscriptions   *string            `json:"active_subscriptions"`
	OutstandingMinorUnits *string            `json:"outstanding_minor_units"`
	OpenSupportCases      *string            `json:"open_support_cases"`
	ObservedAt            time.Time          `json:"observed_at"`
}
type PlatformOverviewTx interface {
	ResolvePlatformAuthority(context.Context, platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error)
	ReadOverview(context.Context, OverviewPermissions, time.Time) (PlatformOverview, error)
	AppendOverviewRead(context.Context, ID, PlatformAuditSubject, time.Time) error
}
type PlatformOverviewStore interface {
	WithinOverview(context.Context, func(context.Context, PlatformOverviewTx) error) error
}
type PlatformOverviewService struct {
	store PlatformOverviewStore
	ids   PlatformAuditIDs
	clock PlatformAuditClock
}

func NewPlatformOverviewService(store PlatformOverviewStore, ids PlatformAuditIDs, clock PlatformAuditClock) (*PlatformOverviewService, error) {
	if store == nil || ids == nil || clock == nil {
		return nil, errors.New("platform_overview_invalid")
	}
	return &PlatformOverviewService{store, ids, clock}, nil
}
func (s *PlatformOverviewService) Read(ctx context.Context, subject PlatformAuditSubject) (out PlatformOverview, err error) {
	if ctx == nil || !validAuditSubject(subject) {
		return out, ErrPlatformAuditInvalid
	}
	at := s.clock.Now().UTC()
	id, e := s.ids.NewID()
	if e != nil || id.IsZero() || at.IsZero() {
		return out, ErrPlatformAuditUnavailable
	}
	err = s.store.WithinOverview(ctx, func(c context.Context, tx PlatformOverviewTx) error {
		facts, e := tx.ResolvePlatformAuthority(c, platformapp.AuthorityQuery{UserID: platformapp.ID(subject.UserID), SessionID: platformapp.ID(subject.SessionID), ObservedAt: at})
		if errors.Is(e, platformapp.ErrAuthorityNotFound) {
			return ErrPlatformAuditForbidden
		}
		if e != nil {
			return e
		}
		if facts.UserID != platformapp.ID(subject.UserID) || facts.ContextGeneration != subject.ContextGeneration || !policy.CurrentAdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod, facts.AssuredAt, at) {
			return ErrPlatformAuditForbidden
		}
		permissions := OverviewPermissions{Tenants: facts.Grants("platform.tenant.read"), Subscriptions: facts.Grants("platform.subscription.read"), Invoices: facts.Grants("platform.invoice.read"), Payments: facts.Grants("platform.payment.read"), Support: facts.Grants("platform.support.case.read")}
		out, e = tx.ReadOverview(c, permissions, at)
		if e != nil {
			return e
		}
		out.ObservedAt = at
		return tx.AppendOverviewRead(c, id, subject, at)
	})
	if err != nil {
		out = PlatformOverview{}
	}
	return out, err
}
