// Package dashboard exposes runtime-composed inbound facades for bounded
// Platform and Tenderpreneur dashboard aggregation.
package dashboard

import (
	app "bidora/backend/internal/dashboard/application"
	"context"
	"errors"
)

type Module struct {
	clock    app.Clock
	tender   *app.TenderDashboardService
	platform *app.PlatformDashboardService
}

func NewModule(clock app.Clock) (*Module, error) {
	if clock == nil {
		return nil, errors.New("dashboard_module_invalid")
	}
	return &Module{clock: clock}, nil
}
func (m *Module) ConfigureTender(subscriptions app.TenderSubscriptionSource, workflow app.TenderWorkflowSource, financial app.TenderFinancialSource) error {
	if m == nil || m.clock == nil {
		return errors.New("dashboard_module_unavailable")
	}
	service, e := app.NewTenderDashboardService(subscriptions, workflow, financial, m.clock)
	if e != nil {
		return e
	}
	m.tender = service
	return nil
}
func (m *Module) ConfigurePlatform(financial app.PlatformFinancialSource) error {
	if m == nil || m.clock == nil {
		return errors.New("dashboard_module_unavailable")
	}
	service, e := app.NewPlatformDashboardService(financial, m.clock)
	if e != nil {
		return e
	}
	m.platform = service
	return nil
}
func (m *Module) Tender(ctx context.Context, input app.TenderRequest) (app.TenderDashboard, error) {
	if m == nil || m.tender == nil {
		return app.TenderDashboard{}, errors.New("dashboard_module_unavailable")
	}
	return m.tender.Read(ctx, input)
}
func (m *Module) Platform(ctx context.Context, input app.PlatformRequest) (app.PlatformDashboard, error) {
	if m == nil || m.platform == nil {
		return app.PlatformDashboard{}, errors.New("dashboard_module_unavailable")
	}
	return m.platform.Read(ctx, input)
}
