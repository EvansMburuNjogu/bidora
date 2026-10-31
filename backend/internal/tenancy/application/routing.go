// Package application defines server-owned tenant routing contracts. Registry
// references and tenant IDs are locators only after authorization.
package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type ID [16]byte

func (id ID) IsZero() bool { return id == ID{} }

var ErrAssignmentUnavailable = errors.New("tenant_assignment_unavailable")

type Assignment struct {
	TenantID, ClusterID                          ID
	Generation                                   int64
	DatabaseName, SecretRef, EndpointRef, Region string
}

func (Assignment) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED tenant assignment]") }
func (a Assignment) Valid() bool {
	return !a.TenantID.IsZero() && !a.ClusterID.IsZero() && a.Generation > 0 && routingValue(a.DatabaseName, 63) && routingValue(a.SecretRef, 500) && routingValue(a.EndpointRef, 500) && routingValue(a.Region, 100)
}

type AssignmentReader interface {
	FindCaptureAssignment(context.Context, ID) (Assignment, error)
}
type AIEntitlementAcknowledgementRoutingStatus string

const (
	AIEntitlementExactCurrentAssignment       AIEntitlementAcknowledgementRoutingStatus = "EXACT_CURRENT_ASSIGNMENT"
	AIEntitlementStaleAssignment              AIEntitlementAcknowledgementRoutingStatus = "STALE_ASSIGNMENT"
	AIEntitlementGenerationAheadOfRegistry    AIEntitlementAcknowledgementRoutingStatus = "GENERATION_AHEAD_OF_REGISTRY"
	AIEntitlementCurrentAssignmentUnavailable AIEntitlementAcknowledgementRoutingStatus = "CURRENT_ASSIGNMENT_TEMPORARILY_UNAVAILABLE"
)

type AIEntitlementAcknowledgementAssignmentObservation struct {
	TenantID          ID
	CurrentGeneration int64
	DispatchReady     bool
	Assignment        Assignment
}
type AIEntitlementAcknowledgementRoutingAssessment struct {
	Status            AIEntitlementAcknowledgementRoutingStatus
	CurrentGeneration int64
	Assignment        Assignment
}
type AIEntitlementAcknowledgementAssignmentReader interface {
	ObserveAIEntitlementAcknowledgementAssignment(context.Context, ID) (AIEntitlementAcknowledgementAssignmentObservation, error)
}
type DocumentCompletionAssignmentReader interface {
	FindDocumentCompletionAssignment(context.Context, ID) (Assignment, error)
}
type AssignmentResolver struct{ reader AssignmentReader }
type AIEntitlementAcknowledgementAssignmentResolver struct {
	reader AIEntitlementAcknowledgementAssignmentReader
}
type DocumentCompletionAssignmentResolver struct {
	reader DocumentCompletionAssignmentReader
}

func NewAssignmentResolver(reader AssignmentReader) (*AssignmentResolver, error) {
	if reader == nil {
		return nil, errors.New("tenant_assignment_resolver_invalid")
	}
	return &AssignmentResolver{reader}, nil
}
func (r *AssignmentResolver) ResolveCaptureAssignment(ctx context.Context, tenantID ID, expectedGeneration int64) (Assignment, error) {
	if ctx == nil || tenantID.IsZero() || expectedGeneration < 1 {
		return Assignment{}, ErrAssignmentUnavailable
	}
	if r == nil || r.reader == nil {
		return Assignment{}, errors.New("tenant_assignment_resolver_invalid")
	}
	assignment, err := r.reader.FindCaptureAssignment(ctx, tenantID)
	if err != nil {
		return Assignment{}, err
	}
	if !assignment.Valid() || assignment.TenantID != tenantID || assignment.Generation != expectedGeneration {
		return Assignment{}, ErrAssignmentUnavailable
	}
	return assignment, nil
}
func NewAIEntitlementAcknowledgementAssignmentResolver(reader AIEntitlementAcknowledgementAssignmentReader) (*AIEntitlementAcknowledgementAssignmentResolver, error) {
	if reader == nil {
		return nil, errors.New("ai_entitlement_acknowledgement_assignment_resolver_invalid")
	}
	return &AIEntitlementAcknowledgementAssignmentResolver{reader: reader}, nil
}
func (r *AIEntitlementAcknowledgementAssignmentResolver) AssessAIEntitlementAcknowledgementAssignment(ctx context.Context, tenantID ID, expectedGeneration int64) (AIEntitlementAcknowledgementRoutingAssessment, error) {
	if ctx == nil || tenantID.IsZero() || expectedGeneration < 1 || r == nil || r.reader == nil {
		return AIEntitlementAcknowledgementRoutingAssessment{}, ErrAssignmentUnavailable
	}
	observation, err := r.reader.ObserveAIEntitlementAcknowledgementAssignment(ctx, tenantID)
	if err != nil {
		return AIEntitlementAcknowledgementRoutingAssessment{}, err
	}
	if observation.TenantID != tenantID || observation.CurrentGeneration < 1 {
		return AIEntitlementAcknowledgementRoutingAssessment{}, ErrAssignmentUnavailable
	}
	assessment := AIEntitlementAcknowledgementRoutingAssessment{CurrentGeneration: observation.CurrentGeneration}
	switch {
	case observation.CurrentGeneration > expectedGeneration:
		assessment.Status = AIEntitlementStaleAssignment
	case observation.CurrentGeneration < expectedGeneration:
		assessment.Status = AIEntitlementGenerationAheadOfRegistry
	case !observation.DispatchReady || !observation.Assignment.Valid() || observation.Assignment.TenantID != tenantID || observation.Assignment.Generation != expectedGeneration:
		assessment.Status = AIEntitlementCurrentAssignmentUnavailable
	default:
		assessment.Status = AIEntitlementExactCurrentAssignment
		assessment.Assignment = observation.Assignment
	}
	return assessment, nil
}
func NewDocumentCompletionAssignmentResolver(reader DocumentCompletionAssignmentReader) (*DocumentCompletionAssignmentResolver, error) {
	if reader == nil {
		return nil, errors.New("document_completion_assignment_resolver_invalid")
	}
	return &DocumentCompletionAssignmentResolver{reader: reader}, nil
}
func (r *DocumentCompletionAssignmentResolver) ResolveDocumentCompletionAssignment(ctx context.Context, tenantID ID, expectedGeneration int64) (Assignment, error) {
	if ctx == nil || tenantID.IsZero() || expectedGeneration < 1 || r == nil || r.reader == nil {
		return Assignment{}, ErrAssignmentUnavailable
	}
	assignment, err := r.reader.FindDocumentCompletionAssignment(ctx, tenantID)
	if err != nil {
		return Assignment{}, err
	}
	if !assignment.Valid() || assignment.TenantID != tenantID || assignment.Generation != expectedGeneration {
		return Assignment{}, ErrAssignmentUnavailable
	}
	return assignment, nil
}
func routingValue(value string, limit int) bool {
	return value != "" && len(value) <= limit && strings.TrimSpace(value) == value
}
