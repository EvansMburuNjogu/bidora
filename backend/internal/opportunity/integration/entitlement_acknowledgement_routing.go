package integration

import (
	"errors"

	dispatchapp "bidora/backend/internal/entitlementdispatch/application"
	tenancyapp "bidora/backend/internal/tenancy/application"
)

type AIEntitlementAcknowledgementEndpointRegistration struct {
	Assignment tenancyapp.Assignment
	Endpoint   dispatchapp.AcknowledgementApplier
}

type AIEntitlementAcknowledgementEndpointCatalog struct {
	endpoints map[tenancyapp.Assignment]dispatchapp.AcknowledgementApplier
}

func NewAIEntitlementAcknowledgementEndpointCatalog(registrations []AIEntitlementAcknowledgementEndpointRegistration) (*AIEntitlementAcknowledgementEndpointCatalog, error) {
	if len(registrations) == 0 {
		return nil, errors.New("ai_entitlement_acknowledgement_endpoint_catalog_invalid")
	}
	endpoints := make(map[tenancyapp.Assignment]dispatchapp.AcknowledgementApplier, len(registrations))
	for _, registration := range registrations {
		if !registration.Assignment.Valid() || registration.Endpoint == nil {
			return nil, errors.New("ai_entitlement_acknowledgement_endpoint_catalog_invalid")
		}
		if _, exists := endpoints[registration.Assignment]; exists {
			return nil, errors.New("ai_entitlement_acknowledgement_endpoint_catalog_invalid")
		}
		endpoints[registration.Assignment] = registration.Endpoint
	}
	return &AIEntitlementAcknowledgementEndpointCatalog{endpoints: endpoints}, nil
}

func (c *AIEntitlementAcknowledgementEndpointCatalog) ResolveAIEntitlementAcknowledgementEndpoint(assignment tenancyapp.Assignment) (dispatchapp.AcknowledgementApplier, error) {
	if c == nil || len(c.endpoints) == 0 || !assignment.Valid() {
		return nil, errors.New("ai_entitlement_acknowledgement_endpoint_unavailable")
	}
	endpoint, found := c.endpoints[assignment]
	if !found || endpoint == nil {
		return nil, errors.New("ai_entitlement_acknowledgement_endpoint_unavailable")
	}
	return endpoint, nil
}

var _ dispatchapp.EndpointResolver = (*AIEntitlementAcknowledgementEndpointCatalog)(nil)
