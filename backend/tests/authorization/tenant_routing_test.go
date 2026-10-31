package authorization_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tenancyapp "bidora/backend/internal/tenancy/application"
)

type tenantAssignmentReaderFake struct {
	assignment tenancyapp.Assignment
	err        error
	tenant     tenancyapp.ID
	calls      int
}

func (r *tenantAssignmentReaderFake) FindCaptureAssignment(_ context.Context, tenant tenancyapp.ID) (tenancyapp.Assignment, error) {
	r.calls++
	r.tenant = tenant
	return r.assignment, r.err
}
func tenantID(n byte) tenancyapp.ID { var value tenancyapp.ID; value[15] = n; return value }
func validTenantAssignment() tenancyapp.Assignment {
	return tenancyapp.Assignment{TenantID: tenantID(1), ClusterID: tenantID(2), Generation: 3, DatabaseName: "tenant_db", SecretRef: "secret/tenant", EndpointRef: "cluster/primary", Region: "local"}
}
func TestTenantAssignmentResolverRequiresExactGeneration(t *testing.T) {
	reader := &tenantAssignmentReaderFake{assignment: validTenantAssignment()}
	resolver, err := tenancyapp.NewAssignmentResolver(reader)
	if err != nil {
		t.Fatal(err)
	}
	assignment, err := resolver.ResolveCaptureAssignment(context.Background(), tenantID(1), 3)
	if err != nil || assignment != reader.assignment || reader.calls != 1 || reader.tenant != tenantID(1) {
		t.Fatal("tenant assignment resolution drifted", assignment, err)
	}
}
func TestTenantAssignmentResolverFailsClosedAndRedactsReferences(t *testing.T) {
	valid := validTenantAssignment()
	for _, tc := range []struct {
		name       string
		reader     *tenantAssignmentReaderFake
		tenant     tenancyapp.ID
		generation int64
	}{{"missing", &tenantAssignmentReaderFake{err: tenancyapp.ErrAssignmentUnavailable}, tenantID(1), 3}, {"stale", &tenantAssignmentReaderFake{assignment: valid}, tenantID(1), 2}, {"wrong tenant", &tenantAssignmentReaderFake{assignment: valid}, tenantID(9), 3}, {"invalid result", &tenantAssignmentReaderFake{assignment: tenancyapp.Assignment{}}, tenantID(1), 3}, {"zero input", &tenantAssignmentReaderFake{assignment: valid}, tenancyapp.ID{}, 3}} {
		t.Run(tc.name, func(t *testing.T) {
			resolver, _ := tenancyapp.NewAssignmentResolver(tc.reader)
			assignment, err := resolver.ResolveCaptureAssignment(context.Background(), tc.tenant, tc.generation)
			if err == nil || assignment != (tenancyapp.Assignment{}) {
				t.Fatal("unsafe tenant assignment accepted", assignment, err)
			}
		})
	}
	if formatted := fmt.Sprintf("%#v", valid); strings.Contains(formatted, "secret/tenant") || strings.Contains(formatted, "cluster/primary") {
		t.Fatal("assignment formatter leaked routing references")
	}
	if resolver, err := tenancyapp.NewAssignmentResolver(nil); err == nil || resolver != nil {
		t.Fatal("nil assignment reader accepted")
	}
	reader := &tenantAssignmentReaderFake{err: errors.New("database unavailable")}
	resolver, _ := tenancyapp.NewAssignmentResolver(reader)
	if _, err := resolver.ResolveCaptureAssignment(nil, tenantID(1), 3); !errors.Is(err, tenancyapp.ErrAssignmentUnavailable) {
		t.Fatal("nil context accepted")
	}
	var zero *tenancyapp.AssignmentResolver
	if _, err := zero.ResolveCaptureAssignment(context.Background(), tenantID(1), 3); err == nil {
		t.Fatal("zero resolver accepted")
	}
}
