// Package application owns Platform Administration use-case sequencing and
// infrastructure-free ports. Platform Administration is global control-plane
// work and never accepts a tenant cell or database selector.
package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bidora/backend/internal/platformadmin/domain"
)

type ID [16]byte

func (id ID) IsZero() bool { return id == ID{} }

var (
	ErrBootstrapConsumed   = errors.New("platform_bootstrap_consumed")
	ErrBootstrapIneligible = errors.New("platform_bootstrap_ineligible")
	ErrConcurrentChange    = errors.New("platform_concurrent_change")
	ErrAuthorityNotFound   = errors.New("platform_authority_not_found")
)

type BootstrapCommit struct {
	OwnerUserID, AssignmentID, AuditID, OutboxID, CorrelationID ID
	OccurredAt                                                  time.Time
}

func (BootstrapCommit) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED platform bootstrap commit]")
}

type BootstrapResult struct {
	OwnerUserID       ID
	AuthEpoch         int64
	ContextGeneration int64
	GovernanceVersion int64
	RevokedSessions   int64
}

type GovernanceState struct {
	BootstrapConsumed                     bool
	FirstAdditionalOwnerExceptionConsumed bool
	EffectiveOwnerGeneration              int64
	Version                               int64
}

type BootstrapIdentity struct {
	UserID         ID
	AuthEpoch      int64
	AffiliationKey string
}

func (BootstrapIdentity) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED platform bootstrap identity]")
}

type BootstrapEvidence struct {
	AuditID, OutboxID, CorrelationID, OwnerUserID ID
	GovernanceVersion                             int64
	OccurredAt                                    time.Time
}

type BootstrapTx interface {
	LockGovernance(context.Context) (GovernanceState, error)
	CountOwnerHistory(context.Context) (int64, error)
	ProfileExists(context.Context, ID) (bool, error)
	LockIdentity(context.Context, ID, time.Time) (BootstrapIdentity, error)
	HasTenantTies(context.Context, ID, string, time.Time) (bool, error)
	PromoteIdentity(context.Context, ID, int64, time.Time) (int64, error)
	RevokeSessions(context.Context, ID, time.Time) (int64, error)
	CreateAdministratorProfile(context.Context, ID, time.Time) error
	AssignInitialOwner(context.Context, ID, ID, time.Time) error
	ConsumeBootstrap(context.Context, ID, int64, time.Time) (int64, error)
	AppendBootstrapEvidence(context.Context, BootstrapEvidence) error
}

type BootstrapUnitOfWork interface {
	WithinBootstrap(context.Context, func(context.Context, BootstrapTx) error) error
}

type AuthorityQuery struct {
	UserID, SessionID ID
	ObservedAt        time.Time
}

type IdentityAuthorityFacts struct {
	UserID                                      ID
	AuthEpoch                                   int64
	AffiliationKey                              string
	AssuranceLevel, AssuranceMethod             string
	AssuredAt, IdleExpiresAt, AbsoluteExpiresAt time.Time
}

type PlatformAuthorityFacts struct {
	UserID                                  ID
	ContextGeneration, AdministratorVersion int64
	RoleKeys                                []string
}

type AuthorityFacts struct {
	UserID                                             ID
	AuthEpoch, ContextGeneration, AdministratorVersion int64
	AssuranceLevel, AssuranceMethod                    string
	AssuredAt, IdleExpiresAt, AbsoluteExpiresAt        time.Time
	Roles                                              []domain.Role
}

func (AuthorityFacts) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED platform authority facts]")
}

func (facts AuthorityFacts) Grants(capability string) bool {
	for _, role := range facts.Roles {
		if domain.Grants(role, capability) {
			return true
		}
	}
	return false
}

type AuthorityTx interface {
	GetIdentityAuthority(context.Context, AuthorityQuery) (IdentityAuthorityFacts, error)
	GetPlatformAuthority(context.Context, ID) (PlatformAuthorityFacts, error)
}

type AuthorityUnitOfWork interface {
	WithinAuthority(context.Context, func(context.Context, AuthorityTx) error) error
}
