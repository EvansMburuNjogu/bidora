package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"bidora/backend/internal/identity/accounttype"
	platformpolicy "bidora/backend/internal/platformadmin/domain"
)

var (
	ErrAccountTypeNotFound        = errors.New("account_type_not_found")
	ErrAccountTypeAlreadySelected = errors.New("account_type_already_selected")
	ErrAccountTypeConflict        = errors.New("account_type_selection_conflict")
	ErrPlatformAccessForbidden    = errors.New("platform_access_forbidden")
)

type AccountTypeReader interface {
	GetAccountType(context.Context, ID) (accounttype.Type, error)
}

type AccountTypeSelectionCommit struct {
	CommandID, AuditID, OutboxID, UserID, CorrelationID ID
	AccountType                                         accounttype.Type
	KeyHash, RequestHash                                [32]byte
	OccurredAt                                          time.Time
}

type AccountTypeSelectionResult struct{ Replay bool }

type AccountTypeSelectionStore interface {
	SelectAccountType(context.Context, AccountTypeSelectionCommit) (AccountTypeSelectionResult, error)
}

type PortalDestination struct{ Key, Path string }

type PlatformPortalAccess struct {
	ContextGeneration               int64
	Capabilities                    []string
	MFAEnrolled                     bool
	AssuranceLevel, AssuranceMethod string
	AssuredAt                       time.Time
}

type PlatformPortalResolver interface {
	ResolvePlatformPortal(context.Context, SessionPrincipal, time.Time) (PlatformPortalAccess, error)
}

type TenderpreneurPortalResolver interface {
	HasActiveTenderpreneurMembership(context.Context, ID) (bool, error)
}

type PortalResolution struct {
	AccountType                      accounttype.Type
	Portal                           *PortalDestination
	RequiredActions, Capabilities    []string
	PlatformContextGeneration        *int64
	AssuranceLevel                   string
	IdleExpiresAt, AbsoluteExpiresAt time.Time
}

type SelectAccountTypeInput struct {
	Principal      SessionPrincipal
	AccountType    string
	IdempotencyKey string
	CorrelationID  ID
}

type AccountTypeService struct {
	reader              AccountTypeReader
	store               AccountTypeSelectionStore
	ids                 LoginIDGenerator
	clock               LoginClock
	platformAccess      PlatformPortalResolver
	tenderpreneurAccess TenderpreneurPortalResolver
}

func NewAccountTypeService(reader AccountTypeReader, store AccountTypeSelectionStore, ids LoginIDGenerator, clock LoginClock) (*AccountTypeService, error) {
	if reader == nil || store == nil || ids == nil || clock == nil {
		return nil, errors.New("account_type_service_invalid")
	}
	return &AccountTypeService{reader: reader, store: store, ids: ids, clock: clock}, nil
}

func (s *AccountTypeService) EnablePlatformAccess(resolver PlatformPortalResolver) error {
	if s == nil || resolver == nil {
		return errors.New("platform_portal_resolver_invalid")
	}
	s.platformAccess = resolver
	return nil
}

func (s *AccountTypeService) EnableTenderpreneurAccess(resolver TenderpreneurPortalResolver) error {
	if s == nil || resolver == nil {
		return errors.New("tenderpreneur_portal_resolver_invalid")
	}
	s.tenderpreneurAccess = resolver
	return nil
}

func (s *AccountTypeService) Resolve(ctx context.Context, principal SessionPrincipal) (PortalResolution, error) {
	if ctx == nil || !validPortalPrincipal(principal) {
		return PortalResolution{}, safe("portal_resolution_unavailable", nil)
	}
	selected, err := s.reader.GetAccountType(ctx, principal.UserID)
	if errors.Is(err, ErrAccountTypeNotFound) {
		return PortalResolution{}, safe("session_invalid", err)
	}
	if err != nil {
		return PortalResolution{}, safe("portal_resolution_unavailable", err)
	}
	result := PortalResolution{
		AccountType: selected, AssuranceLevel: principal.AssuranceLevel,
		IdleExpiresAt: principal.IdleExpiresAt.UTC(), AbsoluteExpiresAt: principal.AbsoluteExpiresAt.UTC(),
		RequiredActions: []string{}, Capabilities: []string{},
	}
	if selected == accounttype.Unassigned {
		result.RequiredActions = []string{"SELECT_ACCOUNT_TYPE"}
		return result, nil
	}
	if selected == accounttype.PlatformAdmin {
		return s.resolvePlatform(ctx, principal, result)
	}
	if selected == accounttype.Tenderpreneur {
		return s.resolveTenderpreneur(ctx, principal, result)
	}
	key, path, err := selected.Portal()
	if err != nil {
		return PortalResolution{}, safe(err.Error(), err)
	}
	result.Portal = &PortalDestination{Key: key, Path: path}
	return result, nil
}

func (s *AccountTypeService) resolveTenderpreneur(ctx context.Context, principal SessionPrincipal, result PortalResolution) (PortalResolution, error) {
	if s.tenderpreneurAccess == nil {
		return PortalResolution{}, safe("portal_resolution_unavailable", nil)
	}
	allowed, err := s.tenderpreneurAccess.HasActiveTenderpreneurMembership(ctx, principal.UserID)
	if err != nil {
		return PortalResolution{}, safe("portal_resolution_unavailable", err)
	}
	if !allowed {
		result.RequiredActions = []string{"ESTABLISH_TENDERPRENEUR_MEMBERSHIP"}
		return result, nil
	}
	result.Portal = &PortalDestination{Key: "TENDERPRENEUR", Path: "/tenderpreneur"}
	return result, nil
}

func (s *AccountTypeService) resolvePlatform(ctx context.Context, principal SessionPrincipal, result PortalResolution) (PortalResolution, error) {
	if s.platformAccess == nil {
		return PortalResolution{}, safe("platform_access_forbidden", ErrPlatformAccessForbidden)
	}
	now := s.clock.Now().UTC()
	if now.IsZero() {
		return PortalResolution{}, safe("portal_resolution_unavailable", nil)
	}
	access, err := s.platformAccess.ResolvePlatformPortal(ctx, principal, now)
	if errors.Is(err, ErrPlatformAccessForbidden) {
		return PortalResolution{}, safe("platform_access_forbidden", err)
	}
	if err != nil || access.ContextGeneration < 1 || len(access.Capabilities) == 0 {
		return PortalResolution{}, safe("portal_resolution_unavailable", err)
	}
	generation := access.ContextGeneration
	result.PlatformContextGeneration = &generation
	result.Capabilities = append([]string(nil), access.Capabilities...)
	result.AssuranceLevel = access.AssuranceLevel
	if !access.MFAEnrolled && !platformpolicy.AdministratorAssurance(access.AssuranceLevel, access.AssuranceMethod) {
		result.RequiredActions = []string{"ENROLL_PLATFORM_MFA"}
		return result, nil
	}
	assuranceAge := now.Sub(access.AssuredAt.UTC())
	if !platformpolicy.AdministratorAssurance(access.AssuranceLevel, access.AssuranceMethod) || assuranceAge < 0 || assuranceAge > 15*time.Minute {
		result.RequiredActions = []string{"COMPLETE_PLATFORM_STEP_UP"}
		return result, nil
	}
	result.Portal = &PortalDestination{Key: "PLATFORM_ADMIN", Path: "/platform"}
	return result, nil
}

func (s *AccountTypeService) Select(ctx context.Context, in SelectAccountTypeInput) (AccountTypeSelectionResult, error) {
	if ctx == nil || !validPortalPrincipal(in.Principal) || in.CorrelationID.IsZero() || len(in.IdempotencyKey) < 1 || len(in.IdempotencyKey) > 128 {
		return AccountTypeSelectionResult{}, safe("account_type_selection_invalid", nil)
	}
	selected, err := accounttype.ParseSelfSelection(in.AccountType)
	if err != nil {
		return AccountTypeSelectionResult{}, safe("account_type_selection_invalid", err)
	}
	now := s.clock.Now().UTC()
	if now.IsZero() {
		return AccountTypeSelectionResult{}, safe("portal_resolution_unavailable", nil)
	}
	ids, err := newIdentityIDs(s.ids, 3)
	if err != nil {
		return AccountTypeSelectionResult{}, safe("portal_resolution_unavailable", err)
	}
	commit := AccountTypeSelectionCommit{
		CommandID: ids[0], AuditID: ids[1], OutboxID: ids[2], UserID: in.Principal.UserID,
		CorrelationID: in.CorrelationID, AccountType: selected, OccurredAt: now,
		KeyHash:     sha256.Sum256([]byte("account-type-selection:v1:" + in.IdempotencyKey)),
		RequestHash: sha256.Sum256([]byte("account-type-selection:v1:" + string(selected))),
	}
	result, err := s.store.SelectAccountType(ctx, commit)
	switch {
	case errors.Is(err, ErrAccountTypeAlreadySelected):
		return result, safe("account_type_already_selected", err)
	case errors.Is(err, ErrAccountTypeConflict):
		return result, safe("account_type_selection_conflict", err)
	case errors.Is(err, ErrAccountTypeNotFound):
		return result, safe("session_invalid", err)
	case err != nil:
		return result, safe("portal_resolution_unavailable", err)
	default:
		return result, nil
	}
}

func validPortalPrincipal(p SessionPrincipal) bool {
	return !p.SessionID.IsZero() && !p.UserID.IsZero() && p.AuthEpoch > 0 &&
		(p.AssuranceLevel == "AAL1" || p.AssuranceLevel == "AAL2") &&
		!p.IdleExpiresAt.IsZero() && !p.AbsoluteExpiresAt.IsZero()
}
