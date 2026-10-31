package integration

import (
	app "bidora/backend/internal/organization/application"
	tenancy "bidora/backend/internal/tenancy/application"
	"context"
	"errors"
)

type AssignmentResolver interface {
	ResolveCaptureAssignment(context.Context, tenancy.ID, int64) (tenancy.Assignment, error)
}
type StoreRegistration struct {
	Assignment tenancy.Assignment
	Store      app.Store
}
type RoutedStore struct {
	assignments AssignmentResolver
	stores      map[tenancy.Assignment]app.Store
}

func NewRoutedStore(resolver AssignmentResolver, registrations []StoreRegistration) (*RoutedStore, error) {
	if resolver == nil || len(registrations) == 0 {
		return nil, errors.New("organization_router_invalid")
	}
	stores := make(map[tenancy.Assignment]app.Store, len(registrations))
	for _, r := range registrations {
		if !r.Assignment.Valid() || r.Store == nil {
			return nil, errors.New("organization_router_invalid")
		}
		if _, exists := stores[r.Assignment]; exists {
			return nil, errors.New("organization_router_invalid")
		}
		stores[r.Assignment] = r.Store
	}
	return &RoutedStore{resolver, stores}, nil
}
func (r *RoutedStore) resolve(ctx context.Context, a app.Authority) (app.Store, error) {
	if ctx == nil || a.ActorID.IsZero() || a.TenantID.IsZero() || a.AssignmentGeneration < 1 {
		return nil, errors.New("organization_routing_invalid")
	}
	assignment, err := r.assignments.ResolveCaptureAssignment(ctx, tenancy.ID(a.TenantID), a.AssignmentGeneration)
	if err != nil {
		return nil, err
	}
	store, ok := r.stores[assignment]
	if !ok {
		return nil, errors.New("organization_store_unavailable")
	}
	return store, nil
}
func (r *RoutedStore) SaveProfile(ctx context.Context, c app.SaveProfileCommit) (app.ProfileResult, error) {
	s, e := r.resolve(ctx, c.Actor)
	if e != nil {
		return app.ProfileResult{}, e
	}
	return s.SaveProfile(ctx, c)
}
func (r *RoutedStore) CreateLetterhead(ctx context.Context, c app.CreateLetterheadCommit) (app.LetterheadResult, error) {
	s, e := r.resolve(ctx, c.Actor)
	if e != nil {
		return app.LetterheadResult{}, e
	}
	return s.CreateLetterhead(ctx, c)
}
func (r *RoutedStore) ActivateLetterhead(ctx context.Context, c app.ActivateLetterheadCommit) (app.ActivationResult, error) {
	s, e := r.resolve(ctx, c.Actor)
	if e != nil {
		return app.ActivationResult{}, e
	}
	return s.ActivateLetterhead(ctx, c)
}
func (r *RoutedStore) GetProfile(ctx context.Context, a app.Authority) (app.CompanyProfile, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return app.CompanyProfile{}, e
	}
	return s.GetProfile(ctx, a)
}
func (r *RoutedStore) GetLetterhead(ctx context.Context, a app.Authority, id app.ID) (app.LetterheadVersion, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return app.LetterheadVersion{}, e
	}
	return s.GetLetterhead(ctx, a, id)
}
func (r *RoutedStore) GetActiveLetterhead(ctx context.Context, a app.Authority) (app.LetterheadVersion, app.LetterheadConfiguration, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return app.LetterheadVersion{}, app.LetterheadConfiguration{}, e
	}
	return s.GetActiveLetterhead(ctx, a)
}
