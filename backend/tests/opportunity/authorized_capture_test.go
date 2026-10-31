package opportunity_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	authzapp "bidora/backend/internal/authorization/application"
	identityapp "bidora/backend/internal/identity/application"
	opportunityapp "bidora/backend/internal/opportunity/application"
	"bidora/backend/internal/opportunity/integration"
)

type controlAuthorizerFake struct {
	grant   authzapp.CaptureGrant
	err     error
	subject authzapp.CaptureSubject
	calls   int
}

func (a *controlAuthorizerFake) Authorize(_ context.Context, subject authzapp.CaptureSubject) (authzapp.CaptureGrant, error) {
	a.calls++
	a.subject = subject
	return a.grant, a.err
}

type sessionValidatorFake struct {
	principal identityapp.SessionPrincipal
	err       error
	token     string
	calls     int
}

func (v *sessionValidatorFake) Validate(_ context.Context, token string) (identityapp.SessionPrincipal, error) {
	v.calls++
	v.token = token
	return v.principal, v.err
}

type opportunityExecutorFake struct {
	result opportunityapp.CaptureResult
	err    error
	input  opportunityapp.ExecuteCaptureInput
	calls  int
}

func (e *opportunityExecutorFake) Execute(_ context.Context, input opportunityapp.ExecuteCaptureInput) (opportunityapp.CaptureResult, error) {
	e.calls++
	e.input = input
	return e.result, e.err
}
func identityID(n byte) identityapp.ID   { var value identityapp.ID; value[15] = n; return value }
func authorizationID(n byte) authzapp.ID { var value authzapp.ID; value[15] = n; return value }

func TestCaptureAuthorizationAdapterMapsConfirmedGrantOnly(t *testing.T) {
	subject := opportunityapp.CaptureSubject{IdentityID: appID(1), SelectedTenantID: appID(2), ContextGeneration: 4}
	control := &controlAuthorizerFake{grant: authzapp.CaptureGrant{MembershipID: authorizationID(3), TenantID: authorizationID(2), ContextGeneration: 4, AssignmentGeneration: 7}}
	adapter, err := integration.NewCaptureAuthorizationAdapter(control)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := adapter.AuthorizeCapture(context.Background(), subject)
	if err != nil {
		t.Fatal(err)
	}
	if authority.ActorID != appID(1) || authority.TenantID != appID(2) || authority.AssignmentGeneration != 7 || control.calls != 1 || control.subject.UserID != authorizationID(1) || control.subject.TenantID != authorizationID(2) || control.subject.ContextGeneration != 4 {
		t.Fatal("control authorization mapping drifted", authority)
	}
}
func TestCaptureAuthorizationAdapterFailsClosed(t *testing.T) {
	valid := opportunityapp.CaptureSubject{IdentityID: appID(1), SelectedTenantID: appID(2), ContextGeneration: 4}
	for _, tc := range []struct {
		name    string
		subject opportunityapp.CaptureSubject
		control *controlAuthorizerFake
	}{{"invalid subject", opportunityapp.CaptureSubject{}, &controlAuthorizerFake{}}, {"denied", valid, &controlAuthorizerFake{err: errors.New("capture_not_authorized")}}, {"tenant mismatch", valid, &controlAuthorizerFake{grant: authzapp.CaptureGrant{MembershipID: authorizationID(3), TenantID: authorizationID(9), ContextGeneration: 4, AssignmentGeneration: 7}}}, {"stale generation", valid, &controlAuthorizerFake{grant: authzapp.CaptureGrant{MembershipID: authorizationID(3), TenantID: authorizationID(2), ContextGeneration: 3, AssignmentGeneration: 7}}}} {
		t.Run(tc.name, func(t *testing.T) {
			adapter, _ := integration.NewCaptureAuthorizationAdapter(tc.control)
			authority, err := adapter.AuthorizeCapture(context.Background(), tc.subject)
			if err == nil || authority != (opportunityapp.CaptureAuthority{}) {
				t.Fatal("invalid control grant accepted", authority, err)
			}
			if tc.name == "invalid subject" && tc.control.calls != 0 {
				t.Fatal("invalid subject crossed boundary")
			}
		})
	}
	if adapter, err := integration.NewCaptureAuthorizationAdapter(nil); err == nil || adapter != nil {
		t.Fatal("nil control authorizer accepted")
	}
	var zero *integration.CaptureAuthorizationAdapter
	if _, err := zero.AuthorizeCapture(context.Background(), valid); err == nil {
		t.Fatal("zero adapter accepted")
	}
}
func TestSessionCaptureCoordinatorDerivesActorFromValidatedSession(t *testing.T) {
	raw := "secret-canary-session-token"
	sessions := &sessionValidatorFake{principal: identityapp.SessionPrincipal{UserID: identityID(1)}}
	executor := &opportunityExecutorFake{result: opportunityapp.CaptureResult{OpportunityID: appID(9)}}
	coordinator, err := integration.NewSessionCaptureCoordinator(sessions, executor)
	if err != nil {
		t.Fatal(err)
	}
	input := integration.SessionCaptureInput{SessionToken: raw, SelectedTenantID: appID(2), ContextGeneration: 4, IdempotencyKey: "019c0000-0000-7000-8000-000000000099", CorrelationID: appID(5), Command: captureCommand(t, false)}
	result, err := coordinator.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.OpportunityID != appID(9) || sessions.calls != 1 || sessions.token != raw || executor.calls != 1 || executor.input.Subject.IdentityID != appID(1) || executor.input.Subject.SelectedTenantID != appID(2) || executor.input.Subject.ContextGeneration != 4 {
		t.Fatal("authenticated capture composition drifted", result)
	}
	if strings.Contains(fmt.Sprintf("%#v", input), raw) {
		t.Fatal("capture input leaked session token")
	}
}
func TestSessionCaptureCoordinatorStopsBeforeAuthorizationOnSessionFailure(t *testing.T) {
	cause := errors.New("session_invalid")
	sessions := &sessionValidatorFake{err: cause}
	executor := &opportunityExecutorFake{}
	coordinator, _ := integration.NewSessionCaptureCoordinator(sessions, executor)
	result, err := coordinator.Execute(context.Background(), integration.SessionCaptureInput{SessionToken: "invalid"})
	if !errors.Is(err, cause) || result != (opportunityapp.CaptureResult{}) || executor.calls != 0 {
		t.Fatal("session failure crossed capture boundary", result, err)
	}
	if coordinator, err := integration.NewSessionCaptureCoordinator(nil, executor); err == nil || coordinator != nil {
		t.Fatal("nil session validator accepted")
	}
	coordinator, _ = integration.NewSessionCaptureCoordinator(sessions, executor)
	if _, err := coordinator.Execute(nil, integration.SessionCaptureInput{}); err == nil {
		t.Fatal("nil context accepted")
	}
	sessions.err = nil
	sessions.principal = identityapp.SessionPrincipal{}
	if _, err := coordinator.Execute(context.Background(), integration.SessionCaptureInput{}); err == nil || executor.calls != 0 {
		t.Fatal("empty principal crossed capture boundary")
	}
	var zero *integration.SessionCaptureCoordinator
	if _, err := zero.Execute(context.Background(), integration.SessionCaptureInput{}); err == nil {
		t.Fatal("zero coordinator accepted")
	}
}
