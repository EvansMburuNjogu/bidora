package opportunity_test

import (
	"context"
	"testing"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"bidora/backend/internal/opportunity/integration"
	tenancyapp "bidora/backend/internal/tenancy/application"
)

type assignmentResolverFake struct {
	assignment tenancyapp.Assignment
	err        error
	tenant     tenancyapp.ID
	generation int64
	calls      int
}

func (r *assignmentResolverFake) ResolveCaptureAssignment(_ context.Context, tenant tenancyapp.ID, generation int64) (tenancyapp.Assignment, error) {
	r.calls++
	r.tenant = tenant
	r.generation = generation
	return r.assignment, r.err
}

type routedStoreFake struct {
	authority opportunityapp.CaptureAuthority
	calls     int
	callback  bool
}

func (s *routedStoreFake) WithinCapture(ctx context.Context, authority opportunityapp.CaptureAuthority, fn func(opportunityapp.CaptureTx) error) error {
	s.calls++
	s.authority = authority
	if fn != nil {
		s.callback = true
		return fn(nil)
	}
	return nil
}
func routingAssignment() tenancyapp.Assignment {
	return tenancyapp.Assignment{TenantID: tenancyapp.ID(appID(1)), ClusterID: tenancyapp.ID(appID(2)), Generation: 3, DatabaseName: "tenant_db", SecretRef: "secret/tenant", EndpointRef: "cluster/primary", Region: "local"}
}
func TestRoutedCaptureResolvesAuthorizedGenerationAndRegisteredStore(t *testing.T) {
	assignment := routingAssignment()
	resolver := &assignmentResolverFake{assignment: assignment}
	store := &routedStoreFake{}
	catalog, err := integration.NewCaptureStoreCatalog([]integration.CaptureStoreRegistration{{Assignment: assignment, Store: store}})
	if err != nil {
		t.Fatal(err)
	}
	router, err := integration.NewRoutedCaptureUnitOfWork(resolver, catalog)
	if err != nil {
		t.Fatal(err)
	}
	authority := opportunityapp.CaptureAuthority{ActorID: appID(9), TenantID: appID(1), AssignmentGeneration: 3}
	called := false
	if err = router.WithinCapture(context.Background(), authority, func(opportunityapp.CaptureTx) error { called = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if resolver.calls != 1 || resolver.tenant != tenancyapp.ID(appID(1)) || resolver.generation != 3 || store.calls != 1 || store.authority != authority || !store.callback || !called {
		t.Fatal("capture routing composition drifted")
	}
}
func TestRoutedCaptureHasNoFallbackForStaleOrUnknownAssignments(t *testing.T) {
	assignment := routingAssignment()
	store := &routedStoreFake{}
	catalog, _ := integration.NewCaptureStoreCatalog([]integration.CaptureStoreRegistration{{Assignment: assignment, Store: store}})
	for _, tc := range []struct {
		name      string
		resolver  *assignmentResolverFake
		authority opportunityapp.CaptureAuthority
	}{{"registry denial", &assignmentResolverFake{err: tenancyapp.ErrAssignmentUnavailable}, opportunityapp.CaptureAuthority{TenantID: appID(1), AssignmentGeneration: 3}}, {"unknown store", &assignmentResolverFake{assignment: func() tenancyapp.Assignment { v := assignment; v.SecretRef = "secret/rotated"; return v }()}, opportunityapp.CaptureAuthority{TenantID: appID(1), AssignmentGeneration: 3}}, {"invalid authority", &assignmentResolverFake{assignment: assignment}, opportunityapp.CaptureAuthority{}}} {
		t.Run(tc.name, func(t *testing.T) {
			router, _ := integration.NewRoutedCaptureUnitOfWork(tc.resolver, catalog)
			err := router.WithinCapture(context.Background(), tc.authority, func(opportunityapp.CaptureTx) error { return nil })
			if err == nil || store.calls != 0 {
				t.Fatal("capture router used fallback", err)
			}
		})
	}
	if router, err := integration.NewRoutedCaptureUnitOfWork(nil, catalog); err == nil || router != nil {
		t.Fatal("nil assignment resolver accepted")
	}
	var zero *integration.RoutedCaptureUnitOfWork
	if err := zero.WithinCapture(context.Background(), opportunityapp.CaptureAuthority{TenantID: appID(1), AssignmentGeneration: 1}, func(opportunityapp.CaptureTx) error { return nil }); err == nil {
		t.Fatal("zero router accepted")
	}
}
func TestCaptureStoreCatalogRequiresExactServerRegistration(t *testing.T) {
	assignment := routingAssignment()
	store := &routedStoreFake{}
	catalog, err := integration.NewCaptureStoreCatalog([]integration.CaptureStoreRegistration{{Assignment: assignment, Store: store}})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := catalog.ResolveCaptureStore(assignment)
	if err != nil || resolved != store {
		t.Fatal("registered store not resolved", err)
	}
	changed := assignment
	changed.EndpointRef = "cluster/other"
	if resolved, err = catalog.ResolveCaptureStore(changed); err == nil || resolved != nil {
		t.Fatal("unregistered endpoint resolved")
	}
	for _, registrations := range [][]integration.CaptureStoreRegistration{nil, {{Assignment: tenancyapp.Assignment{}, Store: store}}, {{Assignment: assignment, Store: nil}}, {{Assignment: assignment, Store: store}, {Assignment: assignment, Store: store}}} {
		if catalog, err := integration.NewCaptureStoreCatalog(registrations); err == nil || catalog != nil {
			t.Fatal("invalid store catalog accepted")
		}
	}
	var zero *integration.CaptureStoreCatalog
	if store, err := zero.ResolveCaptureStore(assignment); err == nil || store != nil {
		t.Fatal("zero catalog resolved store")
	}
}
