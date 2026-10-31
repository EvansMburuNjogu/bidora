// Package application owns control-plane authorization decisions. Tenant IDs,
// membership IDs, context generations and role labels never confer authority.
package application

import (
	"context"
	"errors"
	"fmt"
)

type ID [16]byte

func (id ID) IsZero() bool { return id == ID{} }

var ErrNotAuthorized = errors.New("not_authorized")

type CaptureSubject struct {
	UserID, TenantID  ID
	ContextGeneration int64
}
type CaptureGrant struct {
	MembershipID, TenantID                  ID
	ContextGeneration, AssignmentGeneration int64
}
type CaptureGrantReader interface {
	FindExternalCaptureGrant(context.Context, CaptureSubject) (CaptureGrant, error)
}
type CaptureAuthorizer struct{ reader CaptureGrantReader }

func NewCaptureAuthorizer(reader CaptureGrantReader) (*CaptureAuthorizer, error) {
	if reader == nil {
		return nil, errors.New("capture_authorizer_invalid")
	}
	return &CaptureAuthorizer{reader}, nil
}
func (a *CaptureAuthorizer) Authorize(ctx context.Context, subject CaptureSubject) (CaptureGrant, error) {
	if ctx == nil || subject.UserID.IsZero() || subject.TenantID.IsZero() || subject.ContextGeneration < 1 {
		return CaptureGrant{}, safe("capture_not_authorized", nil)
	}
	if a == nil || a.reader == nil {
		return CaptureGrant{}, safe("authorization_unavailable", nil)
	}
	grant, err := a.reader.FindExternalCaptureGrant(ctx, subject)
	if errors.Is(err, ErrNotAuthorized) {
		return CaptureGrant{}, safe("capture_not_authorized", nil)
	}
	if err != nil {
		return CaptureGrant{}, safe("authorization_unavailable", err)
	}
	if grant.MembershipID.IsZero() || grant.TenantID != subject.TenantID || grant.ContextGeneration != subject.ContextGeneration || grant.AssignmentGeneration < 1 {
		return CaptureGrant{}, safe("authorization_unavailable", nil)
	}
	return grant, nil
}

type operationError struct {
	code  string
	cause error
}

func safe(code string, cause error) error               { return &operationError{code, cause} }
func (e *operationError) Error() string                 { return e.code }
func (e *operationError) Unwrap() error                 { return e.cause }
func (e *operationError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.code) }
