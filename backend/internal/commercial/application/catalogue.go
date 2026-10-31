// Package application exposes provider-free commercial catalogue use cases.
package application

import (
	"context"
	"errors"

	"bidora/backend/internal/commercial/domain"
)

var ErrInvalidRequest = errors.New("commercial_request_invalid")
var ErrUnavailableCadence = errors.New("commercial_cadence_unavailable")

type BasePriceQuote struct {
	Plan                  domain.Plan
	Cadence               domain.BillingCadence
	Price                 domain.Money
	PriceKind             string
	VATCalculated         bool
	ActivatesSubscription bool
	ActivatesEntitlements bool
}

type AICreditCalculation struct {
	Credits int64
	Rule    string
}

type CatalogueService struct{}

func NewCatalogueService() *CatalogueService { return &CatalogueService{} }

func (*CatalogueService) List(ctx context.Context) ([]domain.Plan, error) {
	if ctx == nil {
		return nil, ErrInvalidRequest
	}
	return domain.Catalogue(), nil
}

func (*CatalogueService) CalculateDocumentCredits(ctx context.Context, documentType string, measuredUnits int64) (AICreditCalculation, error) {
	if ctx == nil {
		return AICreditCalculation{}, ErrInvalidRequest
	}
	var credits int64
	var err error
	switch documentType {
	case "PDF_PAGES":
		credits, err = domain.PDFDocumentCredits(measuredUnits)
	case "DOCX_EXTRACTED_CHARACTERS":
		credits, err = domain.DOCXDocumentCredits(measuredUnits)
	default:
		return AICreditCalculation{}, ErrInvalidRequest
	}
	if err != nil {
		return AICreditCalculation{}, ErrInvalidRequest
	}
	return AICreditCalculation{Credits: credits, Rule: documentType}, nil
}

func (*CatalogueService) CalculateGenerationCredits(ctx context.Context, inputCharacters, outputCharacters int64) (AICreditCalculation, error) {
	if ctx == nil {
		return AICreditCalculation{}, ErrInvalidRequest
	}
	credits, err := domain.GenerationCredits(inputCharacters, outputCharacters)
	if err != nil {
		return AICreditCalculation{}, ErrInvalidRequest
	}
	return AICreditCalculation{Credits: credits, Rule: "GENERATION_TEXT"}, nil
}

func (*CatalogueService) QuoteBasePrice(ctx context.Context, planValue, cadenceValue string) (BasePriceQuote, error) {
	if ctx == nil || planValue == "" || cadenceValue == "" {
		return BasePriceQuote{}, ErrInvalidRequest
	}
	key, err := domain.ParsePlan(planValue)
	if err != nil {
		return BasePriceQuote{}, ErrInvalidRequest
	}
	cadence := domain.BillingCadence(cadenceValue)
	price, err := domain.BasePrice(key, cadence)
	if errors.Is(err, domain.ErrCadenceUnavailable) {
		return BasePriceQuote{}, ErrUnavailableCadence
	}
	if err != nil {
		return BasePriceQuote{}, ErrInvalidRequest
	}
	plan, err := domain.FindPlan(key)
	if err != nil {
		return BasePriceQuote{}, ErrInvalidRequest
	}
	kind := "FIXED"
	if plan.ContractRequired {
		kind = "FROM"
	}
	return BasePriceQuote{Plan: plan, Cadence: cadence, Price: price, PriceKind: kind}, nil
}
