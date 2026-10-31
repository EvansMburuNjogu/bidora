package integration

import (
	"context"
	"errors"

	commercialapp "bidora/backend/internal/commercial/application"
	opportunityapp "bidora/backend/internal/opportunity/application"
)

type documentCreditCatalogue interface {
	CalculateDocumentCredits(context.Context, string, int64) (commercialapp.AICreditCalculation, error)
	CalculateGenerationCredits(context.Context, int64, int64) (commercialapp.AICreditCalculation, error)
}

type DocumentCreditCalculator struct{ catalogue documentCreditCatalogue }

func NewDocumentCreditCalculator(catalogue documentCreditCatalogue) (*DocumentCreditCalculator, error) {
	if catalogue == nil {
		return nil, errors.New("document_credit_calculator_invalid")
	}
	return &DocumentCreditCalculator{catalogue: catalogue}, nil
}

func (c *DocumentCreditCalculator) CalculateDocumentCredits(ctx context.Context, unit string, measuredUnits int64) (int64, error) {
	if c == nil || c.catalogue == nil {
		return 0, errors.New("document_credit_calculator_unavailable")
	}
	calculation, err := c.catalogue.CalculateDocumentCredits(ctx, unit, measuredUnits)
	if err != nil {
		return 0, err
	}
	return calculation.Credits, nil
}
func (c *DocumentCreditCalculator) CalculateGenerationCredits(ctx context.Context, inputCharacters, outputCharacters int64) (int64, error) {
	if c == nil || c.catalogue == nil {
		return 0, errors.New("generation_credit_calculator_unavailable")
	}
	calculation, err := c.catalogue.CalculateGenerationCredits(ctx, inputCharacters, outputCharacters)
	if err != nil {
		return 0, err
	}
	return calculation.Credits, nil
}

var _ opportunityapp.DocumentCreditCalculator = (*DocumentCreditCalculator)(nil)
var _ opportunityapp.GenerationCreditCalculator = (*DocumentCreditCalculator)(nil)
