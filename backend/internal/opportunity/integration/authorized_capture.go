// Package integration composes identity and control authorization application
// interfaces with opportunity use cases. It contains no HTTP or persistence policy.
package integration

import (
	"context"
	"errors"
	"fmt"

	authzapp "bidora/backend/internal/authorization/application"
	identityapp "bidora/backend/internal/identity/application"
	opportunityapp "bidora/backend/internal/opportunity/application"
)

type ControlCaptureAuthorizer interface {
	Authorize(context.Context, authzapp.CaptureSubject) (authzapp.CaptureGrant, error)
}
type CaptureAuthorizationAdapter struct{ authorizer ControlCaptureAuthorizer }

func NewCaptureAuthorizationAdapter(authorizer ControlCaptureAuthorizer) (*CaptureAuthorizationAdapter, error) {
	if authorizer == nil {
		return nil, errors.New("capture_authorization_adapter_invalid")
	}
	return &CaptureAuthorizationAdapter{authorizer}, nil
}
func (a *CaptureAuthorizationAdapter) AuthorizeCapture(ctx context.Context, subject opportunityapp.CaptureSubject) (opportunityapp.CaptureAuthority, error) {
	if ctx == nil || subject.IdentityID.IsZero() || subject.SelectedTenantID.IsZero() || subject.ContextGeneration < 1 {
		return opportunityapp.CaptureAuthority{}, errors.New("capture_not_authorized")
	}
	if a == nil || a.authorizer == nil {
		return opportunityapp.CaptureAuthority{}, errors.New("capture_authorization_unavailable")
	}
	grant, err := a.authorizer.Authorize(ctx, authzapp.CaptureSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	if err != nil {
		return opportunityapp.CaptureAuthority{}, err
	}
	if grant.TenantID != authzapp.ID(subject.SelectedTenantID) || grant.ContextGeneration != subject.ContextGeneration || grant.AssignmentGeneration < 1 {
		return opportunityapp.CaptureAuthority{}, errors.New("capture_authorization_inconsistent")
	}
	return opportunityapp.CaptureAuthority{ActorID: subject.IdentityID, TenantID: subject.SelectedTenantID, AssignmentGeneration: grant.AssignmentGeneration}, nil
}

type SessionValidator interface {
	Validate(context.Context, string) (identityapp.SessionPrincipal, error)
}
type OpportunityExecutor interface {
	Execute(context.Context, opportunityapp.ExecuteCaptureInput) (opportunityapp.CaptureResult, error)
}
type SessionCaptureCoordinator struct {
	sessions SessionValidator
	executor OpportunityExecutor
}

func NewSessionCaptureCoordinator(sessions SessionValidator, executor OpportunityExecutor) (*SessionCaptureCoordinator, error) {
	if sessions == nil || executor == nil {
		return nil, errors.New("session_capture_coordinator_invalid")
	}
	return &SessionCaptureCoordinator{sessions, executor}, nil
}

type SessionCaptureInput struct {
	SessionToken      string
	SelectedTenantID  opportunityapp.ID
	ContextGeneration int64
	IdempotencyKey    string
	CorrelationID     opportunityapp.ID
	Command           opportunityapp.CaptureCommand
}

func (SessionCaptureInput) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED session capture input]")
}
func (c *SessionCaptureCoordinator) Execute(ctx context.Context, input SessionCaptureInput) (opportunityapp.CaptureResult, error) {
	if ctx == nil {
		return opportunityapp.CaptureResult{}, errors.New("capture_context_required")
	}
	if c == nil || c.sessions == nil || c.executor == nil {
		return opportunityapp.CaptureResult{}, errors.New("capture_not_configured")
	}
	principal, err := c.sessions.Validate(ctx, input.SessionToken)
	if err != nil {
		return opportunityapp.CaptureResult{}, err
	}
	if principal.UserID.IsZero() {
		return opportunityapp.CaptureResult{}, errors.New("session_unavailable")
	}
	return c.executor.Execute(ctx, opportunityapp.ExecuteCaptureInput{Subject: opportunityapp.CaptureSubject{IdentityID: opportunityapp.ID(principal.UserID), SelectedTenantID: input.SelectedTenantID, ContextGeneration: input.ContextGeneration}, IdempotencyKey: input.IdempotencyKey, CorrelationID: input.CorrelationID, Command: input.Command})
}
