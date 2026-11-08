// Package application exposes provider-free commercial catalogue use cases.
package application

import (
	"context"
	"errors"
	"time"

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

type CatalogueService struct{ published PublishedPlanReader }

func NewCatalogueService() *CatalogueService { return &CatalogueService{} }
func (s *CatalogueService) ConfigurePublished(reader PublishedPlanReader) error {
	if s == nil || reader == nil {
		return ErrPlatformPlanUnavailable
	}
	s.published = reader
	return nil
}
func (s *CatalogueService) List(ctx context.Context) ([]domain.Plan, error) {
	if ctx == nil || s == nil {
		return nil, ErrInvalidRequest
	}
	plans := domain.Catalogue()
	if s.published == nil {
		return plans, nil
	} // Offline/test module; both production runtimes configure the control store.
	versions, err := s.published.PublishedPlans(ctx)
	if err != nil || len(versions) != 3 {
		return nil, ErrPlatformPlanUnavailable
	}
	seen := map[domain.PlanKey]bool{}
	for _, v := range versions {
		if seen[v.Key] {
			return nil, ErrPlatformPlanUnavailable
		}
		updated, e := ApplyPublishedPlan(v)
		if e != nil {
			return nil, e
		}
		seen[v.Key] = true
		for i := range plans {
			if plans[i].Key == v.Key {
				plans[i] = updated
				break
			}
		}
	}
	if !seen[domain.Basic] || !seen[domain.Growth] || !seen[domain.Professional] {
		return nil, ErrPlatformPlanUnavailable
	}
	return plans, nil
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

// PublishedTerm snapshots the same version that quotes read. Invoice issuance
// remains separately gated by its own quote provider and payment workflow.
func (s *CatalogueService) PublishedTerm(ctx context.Context, key domain.PlanKey, cadence domain.BillingCadence, startsAt time.Time) (domain.TermSnapshot, error) {
	plans, err := s.List(ctx)
	if err != nil {
		return domain.TermSnapshot{}, err
	}
	for _, plan := range plans {
		if plan.Key == key {
			return domain.NewTermFromPlan(plan, cadence, startsAt)
		}
	}
	return domain.TermSnapshot{}, ErrInvalidRequest
}
func (s *CatalogueService) QuoteBasePrice(ctx context.Context, planValue, cadenceValue string) (BasePriceQuote, error) {
	if ctx == nil || planValue == "" || cadenceValue == "" {
		return BasePriceQuote{}, ErrInvalidRequest
	}
	key, err := domain.ParsePlan(planValue)
	if err != nil {
		return BasePriceQuote{}, ErrInvalidRequest
	}
	cadence := domain.BillingCadence(cadenceValue)
	plans, err := s.List(ctx)
	if err != nil {
		return BasePriceQuote{}, err
	}
	var plan domain.Plan
	for _, candidate := range plans {
		if candidate.Key == key {
			plan = candidate
			break
		}
	}
	if plan.Key == "" {
		return BasePriceQuote{}, ErrInvalidRequest
	}
	price, err := domain.PriceForPlan(plan, cadence)
	if errors.Is(err, domain.ErrCadenceUnavailable) {
		return BasePriceQuote{}, ErrUnavailableCadence
	}
	if err != nil {
		return BasePriceQuote{}, ErrInvalidRequest
	}
	kind := "FIXED"
	if plan.ContractRequired {
		kind = "FROM"
	}
	return BasePriceQuote{Plan: plan, Cadence: cadence, Price: price, PriceKind: kind}, nil
}
