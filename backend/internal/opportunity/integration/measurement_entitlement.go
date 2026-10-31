package integration

import (
	"context"
	"errors"

	entitlementapp "bidora/backend/internal/entitlementcheckpoint/application"
	opportunityapp "bidora/backend/internal/opportunity/application"
)

type entitlementRequester interface {
	Request(context.Context, entitlementapp.RequestInput) (entitlementapp.Result, error)
}

type DocumentEntitlementRequester struct{ service entitlementRequester }

func NewDocumentEntitlementRequester(service entitlementRequester) (*DocumentEntitlementRequester, error) {
	if service == nil {
		return nil, errors.New("document_entitlement_requester_invalid")
	}
	return &DocumentEntitlementRequester{service: service}, nil
}

func (r *DocumentEntitlementRequester) RequestDocumentEntitlement(ctx context.Context, input opportunityapp.EntitlementReservationRequest) (opportunityapp.EntitlementReservationReference, error) {
	if r == nil || r.service == nil {
		return opportunityapp.EntitlementReservationReference{}, errors.New("document_entitlement_requester_unavailable")
	}
	result, err := r.service.Request(ctx, entitlementapp.RequestInput{
		TenantID:             entitlementapp.ID(input.TenantID),
		TargetID:             entitlementapp.ID(input.JobID),
		SourceOperationID:    entitlementapp.ID(input.MeasurementID),
		RequesterUserID:      entitlementapp.ID(input.RequestedBy),
		CorrelationID:        entitlementapp.ID(input.CorrelationID),
		TargetType:           "TENDER_ANALYSIS",
		UsageKind:            input.UsageKind,
		AssignmentGeneration: input.AssignmentGeneration,
		EstimatedCredits:     input.EstimatedCredits,
	})
	if err != nil {
		return opportunityapp.EntitlementReservationReference{}, err
	}
	return opportunityapp.EntitlementReservationReference{CheckpointID: opportunityapp.ID(result.Checkpoint.ID)}, nil
}

var _ opportunityapp.DocumentEntitlementRequester = (*DocumentEntitlementRequester)(nil)
