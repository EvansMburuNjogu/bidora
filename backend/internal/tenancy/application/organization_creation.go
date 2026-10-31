package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

var (
	ErrOrganizationCreationInvalid           = errors.New("organization_creation_invalid")
	ErrOrganizationCreationIneligible        = errors.New("organization_creation_ineligible")
	ErrOrganizationCreationAssuranceRequired = errors.New("organization_creation_assurance_required")
	ErrOrganizationCreationConflict          = errors.New("organization_creation_conflict")
	ErrOrganizationCreationRateLimited       = errors.New("organization_creation_rate_limited")
	ErrOrganizationCreationUnavailable       = errors.New("organization_creation_unavailable")
)

const OrganizationCreationPolicyVersion int64 = 1

var organizationTag = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

type OrganizationCommercialPath string

const (
	OrganizationTrial OrganizationCommercialPath = "TRIAL"
	OrganizationPaid  OrganizationCommercialPath = "PAID"
)

type OrganizationCreationInput struct {
	CreatorUserID, CorrelationID        ID
	IdempotencyKey                      string
	IdentityScopeHash, NetworkScopeHash [32]byte
	Name, Country                       string
	Timezone, Locale                    string
	BusinessType, PrimaryIndustry       string
	PreferredLanguage, Currency         string
	CommercialPath                      OrganizationCommercialPath
	AssuranceLevel, AssuranceMethod     string
	AssuredAt                           time.Time
}

type OrganizationCreationResult struct {
	OperationID, TenantID ID
	State                 string
	Replay                bool
}

type OrganizationCreatorFacts struct {
	Verified, Disabled, Tenderpreneur, MFAEnrolled bool
}

type OrganizationCreationCommand struct {
	RequestHash           [32]byte
	OperationID, TenantID ID
	State                 string
}

type OrganizationCreationCommit struct {
	TenantID, MembershipID, OperationID, JobID, CommandID, AuditID, OutboxID ID
	GrantIDs                                                                 []ID
	CreatorUserID, CorrelationID                                             ID
	KeyHash, RequestHash                                                     [32]byte
	Name, Country, Timezone, Locale                                          string
	BusinessType, PrimaryIndustry, PreferredLanguage, Currency               string
	CommercialPath                                                           OrganizationCommercialPath
	OccurredAt                                                               time.Time
}

type OrganizationCreationTx interface {
	LockCreator(context.Context, ID) (OrganizationCreatorFacts, error)
	GetOrganizationCreationCommand(context.Context, ID, [32]byte) (OrganizationCreationCommand, bool, error)
	CreatorLifetimeMarkerExists(context.Context, ID) (bool, error)
	CreatorOpenOperationExists(context.Context, ID) (bool, error)
	CreateOrganization(context.Context, OrganizationCreationCommit) error
}

type OrganizationCreationUnitOfWork interface {
	WithinOrganizationCreation(context.Context, func(context.Context, OrganizationCreationTx) error) error
}

type OrganizationCreationLimiter interface {
	AllowOrganizationCreation(context.Context, [32]byte, [32]byte, time.Time) (bool, error)
}

type OrganizationCreationIDGenerator interface{ NewID() (ID, error) }
type OrganizationCreationClock interface{ Now() time.Time }

type OrganizationCreationService struct {
	uow     OrganizationCreationUnitOfWork
	limiter OrganizationCreationLimiter
	ids     OrganizationCreationIDGenerator
	clock   OrganizationCreationClock
}

func NewOrganizationCreationService(uow OrganizationCreationUnitOfWork, limiter OrganizationCreationLimiter, ids OrganizationCreationIDGenerator, clock OrganizationCreationClock) (*OrganizationCreationService, error) {
	if uow == nil || limiter == nil || ids == nil || clock == nil {
		return nil, errors.New("organization_creation_service_invalid")
	}
	return &OrganizationCreationService{uow: uow, limiter: limiter, ids: ids, clock: clock}, nil
}

func (s *OrganizationCreationService) Create(ctx context.Context, in OrganizationCreationInput) (result OrganizationCreationResult, err error) {
	at := s.clock.Now().UTC()
	if !validOrganizationCreation(ctx, in, at) {
		return result, ErrOrganizationCreationInvalid
	}
	if !recentOrganizationCreationAssurance(in, at) {
		return result, ErrOrganizationCreationAssuranceRequired
	}
	allowed, limitErr := s.limiter.AllowOrganizationCreation(ctx, in.IdentityScopeHash, in.NetworkScopeHash, at)
	if limitErr != nil {
		return result, ErrOrganizationCreationUnavailable
	}
	if !allowed {
		return result, ErrOrganizationCreationRateLimited
	}
	requestBytes, _ := json.Marshal(struct {
		Name, Country, Timezone, Locale, BusinessType, PrimaryIndustry, PreferredLanguage, Currency, CommercialPath string
	}{in.Name, in.Country, in.Timezone, in.Locale, in.BusinessType, in.PrimaryIndustry, in.PreferredLanguage, in.Currency, string(in.CommercialPath)})
	keyHash := sha256.Sum256([]byte("organization-create:v1:" + in.IdempotencyKey))
	requestHash := sha256.Sum256(append([]byte("organization-create:v1:"), requestBytes...))
	err = s.uow.WithinOrganizationCreation(ctx, func(txCtx context.Context, tx OrganizationCreationTx) error {
		facts, txErr := tx.LockCreator(txCtx, in.CreatorUserID)
		if txErr != nil {
			return txErr
		}
		if !facts.Verified || facts.Disabled || !facts.Tenderpreneur {
			return ErrOrganizationCreationIneligible
		}
		if facts.MFAEnrolled && (in.AssuranceLevel != "AAL2" || in.AssuranceMethod != "TOTP") {
			return ErrOrganizationCreationAssuranceRequired
		}
		replay, found, txErr := tx.GetOrganizationCreationCommand(txCtx, in.CreatorUserID, keyHash)
		if txErr != nil {
			return txErr
		}
		if found {
			if replay.RequestHash != requestHash || replay.OperationID.IsZero() || replay.TenantID.IsZero() || replay.State != "REQUESTED" {
				return ErrOrganizationCreationConflict
			}
			result = OrganizationCreationResult{OperationID: replay.OperationID, TenantID: replay.TenantID, State: replay.State, Replay: true}
			return nil
		}
		consumed, txErr := tx.CreatorLifetimeMarkerExists(txCtx, in.CreatorUserID)
		if txErr != nil {
			return txErr
		}
		open, txErr := tx.CreatorOpenOperationExists(txCtx, in.CreatorUserID)
		if txErr != nil {
			return txErr
		}
		if consumed || open {
			return ErrOrganizationCreationIneligible
		}
		ids, txErr := organizationCreationIDs(s.ids, 14)
		if txErr != nil {
			return txErr
		}
		commit := OrganizationCreationCommit{TenantID: ids[0], MembershipID: ids[1], OperationID: ids[2], JobID: ids[3], CommandID: ids[4], AuditID: ids[5], OutboxID: ids[6], GrantIDs: append([]ID(nil), ids[7:]...), CreatorUserID: in.CreatorUserID, CorrelationID: in.CorrelationID, KeyHash: keyHash, RequestHash: requestHash, Name: in.Name, Country: in.Country, Timezone: in.Timezone, Locale: in.Locale, BusinessType: in.BusinessType, PrimaryIndustry: in.PrimaryIndustry, PreferredLanguage: in.PreferredLanguage, Currency: in.Currency, CommercialPath: in.CommercialPath, OccurredAt: at}
		if txErr = tx.CreateOrganization(txCtx, commit); txErr != nil {
			return txErr
		}
		result = OrganizationCreationResult{OperationID: commit.OperationID, TenantID: commit.TenantID, State: "REQUESTED"}
		return nil
	})
	if err == nil || errors.Is(err, ErrOrganizationCreationIneligible) || errors.Is(err, ErrOrganizationCreationAssuranceRequired) || errors.Is(err, ErrOrganizationCreationConflict) {
		return result, err
	}
	return OrganizationCreationResult{}, ErrOrganizationCreationUnavailable
}

func validOrganizationCreation(ctx context.Context, in OrganizationCreationInput, now time.Time) bool {
	trimmed := []string{in.Name, in.BusinessType, in.PrimaryIndustry}
	if ctx == nil || now.IsZero() || in.CreatorUserID.IsZero() || in.CorrelationID.IsZero() || in.IdentityScopeHash == [32]byte{} || in.NetworkScopeHash == [32]byte{} || len(in.IdempotencyKey) < 1 || len(in.IdempotencyKey) > 128 || len(in.Name) < 1 || len(in.Name) > 300 || len(in.BusinessType) < 1 || len(in.BusinessType) > 100 || len(in.PrimaryIndustry) < 1 || len(in.PrimaryIndustry) > 160 {
		return false
	}
	for _, value := range trimmed {
		if strings.TrimSpace(value) != value {
			return false
		}
	}
	if len(in.Country) != 2 || in.Country[0] < 'A' || in.Country[0] > 'Z' || in.Country[1] < 'A' || in.Country[1] > 'Z' || len(in.Timezone) < 1 || len(in.Timezone) > 128 {
		return false
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return false
	}
	if len(in.Locale) < 1 || len(in.Locale) > 35 || !organizationTag.MatchString(in.Locale) || len(in.PreferredLanguage) < 1 || len(in.PreferredLanguage) > 35 || !organizationTag.MatchString(in.PreferredLanguage) || in.Currency != "KES" || (in.CommercialPath != OrganizationTrial && in.CommercialPath != OrganizationPaid) {
		return false
	}
	return true
}
func recentOrganizationCreationAssurance(in OrganizationCreationInput, now time.Time) bool {
	age := now.Sub(in.AssuredAt.UTC())
	return !in.AssuredAt.IsZero() && age >= 0 && age <= 15*time.Minute && (in.AssuranceLevel == "AAL1" || in.AssuranceLevel == "AAL2")
}

func organizationCreationIDs(generator OrganizationCreationIDGenerator, count int) ([]ID, error) {
	ids := make([]ID, count)
	for index := range ids {
		id, err := generator.NewID()
		if err != nil || id.IsZero() {
			return nil, ErrOrganizationCreationUnavailable
		}
		ids[index] = id
	}
	return ids, nil
}
