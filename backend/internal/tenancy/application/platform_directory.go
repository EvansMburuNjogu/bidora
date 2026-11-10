package application

import (
	platformpolicy "bidora/backend/internal/platformadmin/domain"
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrPlatformDirectoryInvalid           = errors.New("platform_tenant_directory_invalid")
	ErrPlatformDirectoryForbidden         = errors.New("platform_tenant_directory_forbidden")
	ErrPlatformDirectoryAssuranceRequired = errors.New("platform_tenant_directory_assurance_required")
	ErrPlatformDirectoryContextStale      = errors.New("platform_tenant_directory_context_stale")
	ErrPlatformTenantNotFound             = errors.New("platform_tenant_not_found")
)

type PlatformDirectorySubject struct {
	ActorID, SessionID        ID
	PlatformContextGeneration int64
	ObservedAt                time.Time
}

type PlatformDirectoryAuthority struct {
	ContextGeneration               int64
	CanRead                         bool
	AssuranceLevel, AssuranceMethod string
	AssuredAt                       time.Time
}

type PlatformTenantFilter struct {
	State   *string
	Search  *string
	Country *string
}

type PlatformTenantPage struct {
	BeforeCreatedAt *time.Time
	BeforeID        *ID
	Limit           int32
}

type PlatformTenantSchema struct {
	CurrentVersion int64
	Generation     int64
	State          string
	UpdatedAt      time.Time
}

type PlatformTenant struct {
	ID                            ID
	Name, Type, State             string
	Country, Timezone, Locale     string
	AssignmentGeneration, Version int64
	AssignmentPresent             bool
	CommonSchema, TenantSchema    *PlatformTenantSchema
	CreatedAt, UpdatedAt          time.Time
}

type PlatformTenantList struct {
	Tenants    []PlatformTenant
	NextCursor *PlatformTenantPage
}

type PlatformTenantDirectoryTx interface {
	ResolvePlatformDirectoryAuthority(context.Context, PlatformDirectorySubject) (PlatformDirectoryAuthority, error)
	ListPlatformTenants(context.Context, PlatformTenantFilter, PlatformTenantPage) ([]PlatformTenant, error)
	GetPlatformTenant(context.Context, ID) (PlatformTenant, error)
}

type PlatformTenantDirectoryUnitOfWork interface {
	WithinPlatformTenantDirectory(context.Context, func(context.Context, PlatformTenantDirectoryTx) error) error
}

type PlatformTenantDirectoryService struct {
	uow PlatformTenantDirectoryUnitOfWork
}

func NewPlatformTenantDirectoryService(uow PlatformTenantDirectoryUnitOfWork) (*PlatformTenantDirectoryService, error) {
	if uow == nil {
		return nil, errors.New("platform_tenant_directory_service_invalid")
	}
	return &PlatformTenantDirectoryService{uow: uow}, nil
}

func (s *PlatformTenantDirectoryService) List(ctx context.Context, subject PlatformDirectorySubject, filter PlatformTenantFilter, page PlatformTenantPage) (result PlatformTenantList, err error) {
	if err = validatePlatformTenantPage(page); err != nil || !validPlatformTenantStateFilter(filter.State) || !validPlatformTenantSearch(filter.Search) || !validPlatformTenantCountry(filter.Country) {
		return result, ErrPlatformDirectoryInvalid
	}
	err = s.withDirectory(ctx, subject, func(txCtx context.Context, tx PlatformTenantDirectoryTx) error {
		rows, readErr := tx.ListPlatformTenants(txCtx, filter, page)
		if readErr != nil {
			return readErr
		}
		if len(rows) > int(page.Limit) {
			rows = rows[:page.Limit]
			last := rows[len(rows)-1]
			at, id := last.CreatedAt.UTC(), last.ID
			result.NextCursor = &PlatformTenantPage{BeforeCreatedAt: &at, BeforeID: &id, Limit: page.Limit}
		}
		result.Tenants = rows
		return nil
	})
	return result, err
}

func (s *PlatformTenantDirectoryService) Get(ctx context.Context, subject PlatformDirectorySubject, tenantID ID) (result PlatformTenant, err error) {
	if tenantID.IsZero() {
		return result, ErrPlatformTenantNotFound
	}
	err = s.withDirectory(ctx, subject, func(txCtx context.Context, tx PlatformTenantDirectoryTx) error {
		var readErr error
		result, readErr = tx.GetPlatformTenant(txCtx, tenantID)
		return readErr
	})
	return result, err
}

func (s *PlatformTenantDirectoryService) withDirectory(ctx context.Context, subject PlatformDirectorySubject, read func(context.Context, PlatformTenantDirectoryTx) error) error {
	if ctx == nil || s == nil || s.uow == nil || read == nil || subject.ActorID.IsZero() || subject.SessionID.IsZero() || subject.PlatformContextGeneration < 1 || subject.ObservedAt.IsZero() {
		return ErrPlatformDirectoryForbidden
	}
	observedAt := subject.ObservedAt.UTC()
	return s.uow.WithinPlatformTenantDirectory(ctx, func(txCtx context.Context, tx PlatformTenantDirectoryTx) error {
		authority, err := tx.ResolvePlatformDirectoryAuthority(txCtx, subject)
		if err != nil {
			return ErrPlatformDirectoryForbidden
		}
		if authority.ContextGeneration != subject.PlatformContextGeneration {
			return ErrPlatformDirectoryContextStale
		}
		if !authority.CanRead {
			return ErrPlatformDirectoryForbidden
		}
		if !platformpolicy.CurrentAdministratorAssurance(authority.AssuranceLevel, authority.AssuranceMethod, authority.AssuredAt, observedAt) {
			return ErrPlatformDirectoryAssuranceRequired
		}
		return read(txCtx, tx)
	})
}

func validatePlatformTenantPage(page PlatformTenantPage) error {
	if page.Limit < 1 || page.Limit > 100 || (page.BeforeCreatedAt == nil) != (page.BeforeID == nil) {
		return ErrPlatformDirectoryInvalid
	}
	if page.BeforeCreatedAt != nil && (page.BeforeCreatedAt.IsZero() || page.BeforeID.IsZero()) {
		return ErrPlatformDirectoryInvalid
	}
	return nil
}

func validPlatformTenantCountry(country *string) bool {
	if country == nil {
		return true
	}
	if len(*country) != 2 {
		return false
	}
	for i := 0; i < 2; i++ {
		if (*country)[i] < 'A' || (*country)[i] > 'Z' {
			return false
		}
	}
	return true
}

func validPlatformTenantSearch(search *string) bool {
	if search == nil {
		return true
	}
	if len(*search) < 2 || len(*search) > 100 || !utf8.ValidString(*search) || strings.TrimSpace(*search) != *search {
		return false
	}
	for _, r := range *search {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func validPlatformTenantStateFilter(state *string) bool {
	if state == nil {
		return true
	}
	switch *state {
	case "REQUESTED", "PROVISIONING", "MIGRATING", "SEEDING", "ACTIVE", "FAILED", "SUSPENDED", "DECOMMISSIONED":
		return true
	default:
		return false
	}
}
