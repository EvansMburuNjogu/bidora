package integration

import (
	app "bidora/backend/internal/controlplane/application"
	qsql "bidora/backend/internal/controlplane/persistence/sqlc"
	platformapp "bidora/backend/internal/platformadmin/application"
	authority "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"time"
)

type PlatformOverviewStore struct{ runner platformAuditRunner }

func NewPlatformOverviewStore(r platformAuditRunner) (*PlatformOverviewStore, error) {
	if r == nil {
		return nil, errors.New("overview_store_invalid")
	}
	return &PlatformOverviewStore{r}, nil
}
func (s *PlatformOverviewStore) WithinOverview(ctx context.Context, use func(context.Context, app.PlatformOverviewTx) error) error {
	return s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		a, e := authority.NewSettingsAuthorityTx(db)
		if e != nil {
			return e
		}
		return use(c, &overviewTx{qsql.New(db), a})
	})
}

type overviewTx struct {
	q         *qsql.Queries
	authority *authority.SettingsAuthorityTx
}

func (t *overviewTx) ResolvePlatformAuthority(c context.Context, q platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error) {
	return t.authority.Resolve(c, q)
}
func (t *overviewTx) ReadOverview(c context.Context, p app.OverviewPermissions, at time.Time) (app.PlatformOverview, error) {
	r, e := t.q.ReadPlatformOverview(c, qsql.ReadPlatformOverviewParams{TenantsAllowed: p.Tenants, SubscriptionsAllowed: p.Subscriptions, InvoicesAllowed: p.Invoices, PaymentsAllowed: p.Payments, SupportAllowed: p.Support, ObservedAt: supportTime(at)})
	if e != nil {
		return app.PlatformOverview{}, e
	}
	value := func(allowed bool, s string) *string {
		if !allowed {
			return nil
		}
		return &s
	}
	monthly := []app.MonthlyInvoicing{}
	if p.Invoices {
		rows, e := t.q.ReadPlatformMonthlyInvoicing(c, supportTime(at))
		if e != nil {
			return app.PlatformOverview{}, e
		}
		for _, row := range rows {
			monthly = append(monthly, app.MonthlyInvoicing{Month: row.Month, TotalMinorUnits: row.TotalMinorUnits})
		}
	}
	return app.PlatformOverview{InvoicedMinorUnits: value(p.Invoices, r.InvoicedMinorUnits), PaidInvoices: value(p.Invoices, r.PaidInvoices), FailedPayments: value(p.Payments, r.FailedPayments), MonthlyInvoicing: monthly, Tenderpreneurs: value(p.Tenants, r.Tenderpreneurs), ActiveSubscriptions: value(p.Subscriptions, r.ActiveSubscriptions), OutstandingMinorUnits: value(p.Invoices, r.OutstandingMinorUnits), OpenSupportCases: value(p.Support, r.OpenSupportCases)}, nil
}
func (t *overviewTx) AppendOverviewRead(c context.Context, id app.ID, s app.PlatformAuditSubject, at time.Time) error {
	return t.q.InsertPlatformOverviewReadAudit(c, qsql.InsertPlatformOverviewReadAuditParams{ID: supportUUID(id), ActorID: supportUUID(s.UserID), CorrelationID: supportUUID(s.CorrelationID), At: supportTime(at)})
}
