// Package organization exposes tenant-private company profile and letterhead use cases.
package organization

import (
	app "bidora/backend/internal/organization/application"
	"context"
	"errors"
)

type Module struct{ service *app.Service }

func NewModule(authorizer app.Authorizer, store app.Store, ids app.IDGenerator, clock app.Clock) (*Module, error) {
	service, err := app.NewService(authorizer, store, ids, clock)
	if err != nil {
		return nil, err
	}
	return &Module{service}, nil
}
func (m *Module) SaveProfile(ctx context.Context, in app.SaveProfileInput) (app.ProfileResult, error) {
	if m == nil || m.service == nil {
		return app.ProfileResult{}, errors.New("organization_module_unavailable")
	}
	return m.service.SaveProfile(ctx, in)
}
func (m *Module) CreateLetterhead(ctx context.Context, in app.CreateLetterheadInput) (app.LetterheadResult, error) {
	if m == nil || m.service == nil {
		return app.LetterheadResult{}, errors.New("organization_module_unavailable")
	}
	return m.service.CreateLetterhead(ctx, in)
}
func (m *Module) ActivateLetterhead(ctx context.Context, in app.ActivateLetterheadInput) (app.ActivationResult, error) {
	if m == nil || m.service == nil {
		return app.ActivationResult{}, errors.New("organization_module_unavailable")
	}
	return m.service.ActivateLetterhead(ctx, in)
}
func (m *Module) GetProfile(ctx context.Context, subject app.Subject) (app.CompanyProfile, error) {
	if m == nil || m.service == nil {
		return app.CompanyProfile{}, errors.New("organization_module_unavailable")
	}
	return m.service.GetProfile(ctx, subject)
}
func (m *Module) GetLetterhead(ctx context.Context, subject app.Subject, id app.ID) (app.LetterheadVersion, error) {
	if m == nil || m.service == nil {
		return app.LetterheadVersion{}, errors.New("organization_module_unavailable")
	}
	return m.service.GetLetterhead(ctx, subject, id)
}
func (m *Module) GetActiveLetterhead(ctx context.Context, subject app.Subject) (app.LetterheadVersion, app.LetterheadConfiguration, error) {
	if m == nil || m.service == nil {
		return app.LetterheadVersion{}, app.LetterheadConfiguration{}, errors.New("organization_module_unavailable")
	}
	return m.service.GetActiveLetterhead(ctx, subject)
}
