// Package domain owns the source-defined Phase 1 commercial catalogue values.
// It has no dependency on HTTP, persistence, payment providers, or tenant data.
package domain

import (
	"errors"
	"math"
)

type PlanKey string

type BillingCadence string

const (
	Trial        PlanKey = "TRIAL"
	Basic        PlanKey = "BASIC"
	Growth       PlanKey = "GROWTH"
	Professional PlanKey = "PROFESSIONAL"
	Enterprise   PlanKey = "ENTERPRISE"

	TrialCadence    BillingCadence = "TRIAL"
	MonthlyCadence  BillingCadence = "MONTHLY"
	AnnualCadence   BillingCadence = "ANNUAL"
	ContractCadence BillingCadence = "CONTRACT"

	CatalogueVersion = "PHASE1_V1"
	CurrencyKES      = "KES"

	AITextUnitCharacters    int64 = 3000
	AIGenerationBaseCredits int64 = 10
)

var ErrPlanInvalid = errors.New("commercial_plan_invalid")
var ErrCadenceUnavailable = errors.New("commercial_cadence_unavailable")
var ErrUsageInvalid = errors.New("commercial_usage_invalid")

type Money struct {
	Currency   string
	MinorUnits int64
}

type Plan struct {
	Key                           PlanKey
	Name                          string
	CatalogueVersion              string
	TrialDays                     *int32
	BidWorkspacesPerPeriod        *int32
	IncludedLicences              *int32
	StorageLimitBytes             *int64
	DocumentVersionLimit          *int64
	AICreditLimit                 *int64
	ConcurrentAIJobLimit          *int32
	LimitResetPeriod              string
	TrialBasePrice                *Money
	MonthlyBasePrice              *Money
	AnnualBasePrice               *Money
	AdditionalLicenceMonthlyPrice *Money
	ContractRequired              bool
	VATExclusive                  bool
	CoreTenderAssistantTerm       string
}

func Catalogue() []Plan {
	trialDays, trialBids, trialLicences, trialJobs := int32(14), int32(1), int32(2), int32(1)
	basicBids, basicLicences, basicJobs := int32(3), int32(3), int32(2)
	growthBids, growthLicences, growthJobs := int32(10), int32(8), int32(5)
	professionalBids, professionalLicences, professionalJobs := int32(30), int32(20), int32(10)
	trialStorage, basicStorage := int64(1<<30), int64(10<<30)
	growthStorage, professionalStorage := int64(50<<30), int64(200<<30)
	trialDocuments, basicDocuments, growthDocuments, professionalDocuments := int64(25), int64(250), int64(1500), int64(6000)
	trialCredits, basicCredits, growthCredits, professionalCredits := int64(500), int64(5000), int64(25000), int64(100000)
	return []Plan{
		{Key: Trial, Name: "Trial", CatalogueVersion: CatalogueVersion, TrialDays: &trialDays, BidWorkspacesPerPeriod: &trialBids, IncludedLicences: &trialLicences, StorageLimitBytes: &trialStorage, DocumentVersionLimit: &trialDocuments, AICreditLimit: &trialCredits, ConcurrentAIJobLimit: &trialJobs, LimitResetPeriod: "TRIAL_TERM", TrialBasePrice: money(0), ContractRequired: false, VATExclusive: true, CoreTenderAssistantTerm: "NOT_SPECIFIED"},
		{Key: Basic, Name: "Basic", CatalogueVersion: CatalogueVersion, BidWorkspacesPerPeriod: &basicBids, IncludedLicences: &basicLicences, StorageLimitBytes: &basicStorage, DocumentVersionLimit: &basicDocuments, AICreditLimit: &basicCredits, ConcurrentAIJobLimit: &basicJobs, LimitResetPeriod: "BILLING_MONTH", MonthlyBasePrice: money(450000), AnnualBasePrice: money(4500000), AdditionalLicenceMonthlyPrice: money(75000), VATExclusive: true, CoreTenderAssistantTerm: "INCLUDED"},
		{Key: Growth, Name: "Growth", CatalogueVersion: CatalogueVersion, BidWorkspacesPerPeriod: &growthBids, IncludedLicences: &growthLicences, StorageLimitBytes: &growthStorage, DocumentVersionLimit: &growthDocuments, AICreditLimit: &growthCredits, ConcurrentAIJobLimit: &growthJobs, LimitResetPeriod: "BILLING_MONTH", MonthlyBasePrice: money(1250000), AnnualBasePrice: money(12500000), AdditionalLicenceMonthlyPrice: money(65000), VATExclusive: true, CoreTenderAssistantTerm: "INCLUDED"},
		{Key: Professional, Name: "Professional", CatalogueVersion: CatalogueVersion, BidWorkspacesPerPeriod: &professionalBids, IncludedLicences: &professionalLicences, StorageLimitBytes: &professionalStorage, DocumentVersionLimit: &professionalDocuments, AICreditLimit: &professionalCredits, ConcurrentAIJobLimit: &professionalJobs, LimitResetPeriod: "BILLING_MONTH", MonthlyBasePrice: money(2950000), AnnualBasePrice: money(29500000), AdditionalLicenceMonthlyPrice: money(50000), VATExclusive: true, CoreTenderAssistantTerm: "INCLUDED"},
		{Key: Enterprise, Name: "Enterprise", CatalogueVersion: CatalogueVersion, LimitResetPeriod: "CONTRACT", MonthlyBasePrice: money(6000000), ContractRequired: true, VATExclusive: true, CoreTenderAssistantTerm: "INCLUDED"},
	}
}

func ParsePlan(value string) (PlanKey, error) {
	key := PlanKey(value)
	for _, plan := range Catalogue() {
		if plan.Key == key {
			return key, nil
		}
	}
	return "", ErrPlanInvalid
}

func FindPlan(key PlanKey) (Plan, error) {
	for _, plan := range Catalogue() {
		if plan.Key == key {
			return clone(plan), nil
		}
	}
	return Plan{}, ErrPlanInvalid
}

func BasePrice(key PlanKey, cadence BillingCadence) (Money, error) {
	plan, err := FindPlan(key)
	if err != nil {
		return Money{}, err
	}
	var price *Money
	switch cadence {
	case TrialCadence:
		if key == Trial {
			price = plan.TrialBasePrice
		}
	case MonthlyCadence:
		if key != Trial && key != Enterprise {
			price = plan.MonthlyBasePrice
		}
	case AnnualCadence:
		price = plan.AnnualBasePrice
	case ContractCadence:
		if key == Enterprise {
			price = plan.MonthlyBasePrice
		}
	}
	if price == nil {
		return Money{}, ErrCadenceUnavailable
	}
	return *price, nil
}

func PDFDocumentCredits(pages int64) (int64, error) {
	if pages < 1 {
		return 0, ErrUsageInvalid
	}
	return pages, nil
}

func DOCXDocumentCredits(extractedCharacters int64) (int64, error) {
	return textUnits(extractedCharacters)
}

func GenerationCredits(inputCharacters, outputCharacters int64) (int64, error) {
	if inputCharacters < 1 || outputCharacters < 0 {
		return 0, ErrUsageInvalid
	}
	input, err := textUnits(inputCharacters)
	if err != nil {
		return 0, err
	}
	output := int64(0)
	if outputCharacters > 0 {
		output, err = textUnits(outputCharacters)
		if err != nil {
			return 0, err
		}
	}
	if input > math.MaxInt64-AIGenerationBaseCredits || output > math.MaxInt64-AIGenerationBaseCredits-input {
		return 0, ErrUsageInvalid
	}
	return AIGenerationBaseCredits + input + output, nil
}

func textUnits(characters int64) (int64, error) {
	if characters < 1 {
		return 0, ErrUsageInvalid
	}
	return 1 + (characters-1)/AITextUnitCharacters, nil
}

func money(minorUnits int64) *Money {
	return &Money{Currency: CurrencyKES, MinorUnits: minorUnits}
}

func clone(plan Plan) Plan {
	cloneInt := func(value *int32) *int32 {
		if value == nil {
			return nil
		}
		copy := *value
		return &copy
	}
	cloneInt64 := func(value *int64) *int64 {
		if value == nil {
			return nil
		}
		copy := *value
		return &copy
	}
	cloneMoney := func(value *Money) *Money {
		if value == nil {
			return nil
		}
		copy := *value
		return &copy
	}
	plan.TrialDays = cloneInt(plan.TrialDays)
	plan.BidWorkspacesPerPeriod = cloneInt(plan.BidWorkspacesPerPeriod)
	plan.IncludedLicences = cloneInt(plan.IncludedLicences)
	plan.StorageLimitBytes = cloneInt64(plan.StorageLimitBytes)
	plan.DocumentVersionLimit = cloneInt64(plan.DocumentVersionLimit)
	plan.AICreditLimit = cloneInt64(plan.AICreditLimit)
	plan.ConcurrentAIJobLimit = cloneInt(plan.ConcurrentAIJobLimit)
	plan.TrialBasePrice = cloneMoney(plan.TrialBasePrice)
	plan.MonthlyBasePrice = cloneMoney(plan.MonthlyBasePrice)
	plan.AnnualBasePrice = cloneMoney(plan.AnnualBasePrice)
	plan.AdditionalLicenceMonthlyPrice = cloneMoney(plan.AdditionalLicenceMonthlyPrice)
	return plan
}
