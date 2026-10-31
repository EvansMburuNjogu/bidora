package integration

import (
	"context"
	"errors"

	opportunityapp "bidora/backend/internal/opportunity/application"
	tenancyapp "bidora/backend/internal/tenancy/application"
)

type CaptureAssignmentResolver interface {
	ResolveCaptureAssignment(context.Context, tenancyapp.ID, int64) (tenancyapp.Assignment, error)
}
type CaptureStoreResolver interface {
	ResolveCaptureStore(tenancyapp.Assignment) (opportunityapp.CaptureUnitOfWork, error)
}
type RoutedCaptureUnitOfWork struct {
	assignments CaptureAssignmentResolver
	stores      CaptureStoreResolver
}

func NewRoutedCaptureUnitOfWork(assignments CaptureAssignmentResolver, stores CaptureStoreResolver) (*RoutedCaptureUnitOfWork, error) {
	if assignments == nil || stores == nil {
		return nil, errors.New("capture_router_invalid")
	}
	return &RoutedCaptureUnitOfWork{assignments, stores}, nil
}
func (r *RoutedCaptureUnitOfWork) WithinCapture(ctx context.Context, authority opportunityapp.CaptureAuthority, fn func(opportunityapp.CaptureTx) error) error {
	if ctx == nil || authority.TenantID.IsZero() || authority.AssignmentGeneration < 1 || fn == nil {
		return errors.New("capture_routing_invalid")
	}
	if r == nil || r.assignments == nil || r.stores == nil {
		return errors.New("capture_router_invalid")
	}
	assignment, err := r.assignments.ResolveCaptureAssignment(ctx, tenancyapp.ID(authority.TenantID), authority.AssignmentGeneration)
	if err != nil {
		return err
	}
	store, err := r.stores.ResolveCaptureStore(assignment)
	if err != nil {
		return err
	}
	if store == nil {
		return errors.New("capture_store_unavailable")
	}
	return store.WithinCapture(ctx, authority, fn)
}

type CaptureStoreRegistration struct {
	Assignment tenancyapp.Assignment
	Store      opportunityapp.CaptureUnitOfWork
}
type CaptureStoreCatalog struct {
	stores map[tenancyapp.Assignment]opportunityapp.CaptureUnitOfWork
}

func NewCaptureStoreCatalog(registrations []CaptureStoreRegistration) (*CaptureStoreCatalog, error) {
	if len(registrations) == 0 {
		return nil, errors.New("capture_store_catalog_invalid")
	}
	stores := make(map[tenancyapp.Assignment]opportunityapp.CaptureUnitOfWork, len(registrations))
	for _, registration := range registrations {
		if !registration.Assignment.Valid() || registration.Store == nil {
			return nil, errors.New("capture_store_catalog_invalid")
		}
		if _, exists := stores[registration.Assignment]; exists {
			return nil, errors.New("capture_store_catalog_invalid")
		}
		stores[registration.Assignment] = registration.Store
	}
	return &CaptureStoreCatalog{stores}, nil
}
func (c *CaptureStoreCatalog) ResolveCaptureStore(assignment tenancyapp.Assignment) (opportunityapp.CaptureUnitOfWork, error) {
	if c == nil || len(c.stores) == 0 || !assignment.Valid() {
		return nil, errors.New("capture_store_unavailable")
	}
	store, ok := c.stores[assignment]
	if !ok {
		return nil, errors.New("capture_store_unavailable")
	}
	return store, nil
}
