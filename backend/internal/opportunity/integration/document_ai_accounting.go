package integration

import (
	"context"
	"errors"

	commercialapp "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	entitlementapp "bidora/backend/internal/entitlementcheckpoint/application"
	opportunityapp "bidora/backend/internal/opportunity/application"
	tenancyapp "bidora/backend/internal/tenancy/application"
)

type commercialAIUsage interface {
	VerifyReserved(context.Context, commercialapp.AIUsageReservationCheck) error
	Settle(context.Context, commercialapp.AIUsageCompleteInput) (commercialapp.AIUsageResult, error)
	Release(context.Context, commercialapp.AIUsageReleaseInput) (commercialapp.AIUsageResult, error)
}

type DocumentCommercialAccounting struct{ usage commercialAIUsage }

type documentCompletionAssignmentResolver interface {
	ResolveDocumentCompletionAssignment(context.Context, tenancyapp.ID, int64) (tenancyapp.Assignment, error)
}
type DocumentCompletionPlacement struct {
	resolver documentCompletionAssignmentResolver
}

func NewDocumentCompletionPlacement(resolver documentCompletionAssignmentResolver) (*DocumentCompletionPlacement, error) {
	if resolver == nil {
		return nil, errors.New("document_completion_placement_invalid")
	}
	return &DocumentCompletionPlacement{resolver: resolver}, nil
}
func (p *DocumentCompletionPlacement) ValidateCurrentDocumentCompletionAssignment(ctx context.Context, tenantID opportunityapp.ID, generation int64) error {
	if p == nil || p.resolver == nil || ctx == nil || tenantID.IsZero() || generation < 1 {
		return opportunityapp.ErrDocumentAIAccountingInvalid
	}
	_, err := p.resolver.ResolveDocumentCompletionAssignment(ctx, tenancyapp.ID(tenantID), generation)
	return err
}

func NewDocumentCommercialAccounting(usage commercialAIUsage) (*DocumentCommercialAccounting, error) {
	if usage == nil {
		return nil, errors.New("document_commercial_accounting_invalid")
	}
	return &DocumentCommercialAccounting{usage: usage}, nil
}

func (a *DocumentCommercialAccounting) VerifyDocumentReservation(ctx context.Context, claim opportunityapp.ProcessingClaim) error {
	if a == nil || a.usage == nil || ctx == nil || claim.ReservationID.IsZero() || claim.CheckpointID.IsZero() || claim.TenantID.IsZero() || claim.RequestedBy.IsZero() || claim.AssignmentGeneration < 1 || claim.EstimatedCredits < 1 {
		return opportunityapp.ErrDocumentAIAccountingInvalid
	}
	kind := domain.AIUsageKind(claim.UsageKind)
	if !domain.ValidAIUsageKind(kind) || (kind != domain.AIUsageDocumentPDF && kind != domain.AIUsageDocumentDOCX) {
		return opportunityapp.ErrDocumentAIAccountingInvalid
	}
	err := a.usage.VerifyReserved(ctx, commercialapp.AIUsageReservationCheck{ReservationID: domain.ID(claim.ReservationID), CheckpointID: domain.ID(claim.CheckpointID), TenantID: domain.ID(claim.TenantID), RequesterUserID: domain.ID(claim.RequestedBy), Kind: kind, AssignmentGeneration: claim.AssignmentGeneration, EstimatedCredits: claim.EstimatedCredits})
	if errors.Is(err, commercialapp.ErrAIUsageAccessUnavailable) {
		return opportunityapp.ErrDocumentPaidAccessDenied
	}
	return err
}

func (a *DocumentCommercialAccounting) VerifyResponseDraftReservation(ctx context.Context, claim opportunityapp.ResponseDraftClaim) error {
	if a == nil || a.usage == nil || ctx == nil || claim.ReservationID.IsZero() || claim.CheckpointID.IsZero() || claim.TenantID.IsZero() || claim.RequestedBy.IsZero() || claim.AssignmentGeneration < 1 || claim.EstimatedCredits < 1 || claim.UsageKind != "GENERATION_TEXT" {
		return opportunityapp.ErrDocumentAIAccountingInvalid
	}
	err := a.usage.VerifyReserved(ctx, commercialapp.AIUsageReservationCheck{ReservationID: domain.ID(claim.ReservationID), CheckpointID: domain.ID(claim.CheckpointID), TenantID: domain.ID(claim.TenantID), RequesterUserID: domain.ID(claim.RequestedBy), Kind: domain.AIUsageGeneration, AssignmentGeneration: claim.AssignmentGeneration, EstimatedCredits: claim.EstimatedCredits})
	if errors.Is(err, commercialapp.ErrAIUsageAccessUnavailable) {
		return opportunityapp.ErrDocumentPaidAccessDenied
	}
	return err
}

func (a *DocumentCommercialAccounting) VerifyProposalDraftReservation(ctx context.Context, claim opportunityapp.ProposalDraftClaim) error {
	if a == nil || a.usage == nil || ctx == nil || claim.ReservationID.IsZero() || claim.CheckpointID.IsZero() || claim.TenantID.IsZero() || claim.RequestedBy.IsZero() || claim.AssignmentGeneration < 1 || claim.EstimatedCredits < 1 || claim.UsageKind != "GENERATION_TEXT" {
		return opportunityapp.ErrDocumentAIAccountingInvalid
	}
	err := a.usage.VerifyReserved(ctx, commercialapp.AIUsageReservationCheck{ReservationID: domain.ID(claim.ReservationID), CheckpointID: domain.ID(claim.CheckpointID), TenantID: domain.ID(claim.TenantID), RequesterUserID: domain.ID(claim.RequestedBy), Kind: domain.AIUsageGeneration, AssignmentGeneration: claim.AssignmentGeneration, EstimatedCredits: claim.EstimatedCredits})
	if errors.Is(err, commercialapp.ErrAIUsageAccessUnavailable) {
		return opportunityapp.ErrDocumentPaidAccessDenied
	}
	return err
}

func (a *DocumentCommercialAccounting) CompleteDocumentReservation(ctx context.Context, request entitlementapp.CompletionRequest) (opportunityapp.DocumentCompletionEffect, error) {
	if a == nil || a.usage == nil || ctx == nil || (request.Operation != entitlementapp.CompletionSettle && request.Operation != entitlementapp.CompletionRelease) {
		return opportunityapp.DocumentCompletionConflict, opportunityapp.ErrDocumentAIAccountingInvalid
	}
	var err error
	if request.Operation == entitlementapp.CompletionSettle {
		if request.ActualCredits < 1 {
			return opportunityapp.DocumentCompletionConflict, opportunityapp.ErrDocumentAIAccountingInvalid
		}
		_, err = a.usage.Settle(ctx, commercialapp.AIUsageCompleteInput{ReservationID: domain.ID(request.ReservationID), ExecutionID: domain.ID(request.ExecutionID), CorrelationID: domain.ID(request.CorrelationID), ActualCredits: request.ActualCredits})
	} else {
		if request.ActualCredits != 0 {
			return opportunityapp.DocumentCompletionConflict, opportunityapp.ErrDocumentAIAccountingInvalid
		}
		_, err = a.usage.Release(ctx, commercialapp.AIUsageReleaseInput{ReservationID: domain.ID(request.ReservationID), ExecutionID: domain.ID(request.ExecutionID), CorrelationID: domain.ID(request.CorrelationID)})
	}
	if err == nil {
		return opportunityapp.DocumentCompletionConfirmed, nil
	}
	if errors.Is(err, commercialapp.ErrAIUsageConflict) || errors.Is(err, commercialapp.ErrAIUsageRequestInvalid) {
		return opportunityapp.DocumentCompletionConflict, err
	}
	return opportunityapp.DocumentCompletionRetryable, err
}

var _ opportunityapp.DocumentCommercialAccounting = (*DocumentCommercialAccounting)(nil)
var _ opportunityapp.DocumentCompletionPlacement = (*DocumentCompletionPlacement)(nil)
var _ opportunityapp.DocumentReservationVerifier = (*DocumentCommercialAccounting)(nil)
var _ opportunityapp.ResponseDraftReservationVerifier = (*DocumentCommercialAccounting)(nil)
