package application

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrPlatformIdentityIneligible = errors.New("platform_identity_ineligible")
	ErrPlatformIdentityConflict   = errors.New("platform_identity_conflict")
	ErrPlatformIdentityNotFound   = errors.New("platform_identity_not_found")
)

type PlatformIdentityCandidate struct {
	UserID         ID
	AuthEpoch      int64
	AffiliationKey string
}

func (PlatformIdentityCandidate) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED platform identity candidate]")
}

type PlatformAuthorityQuery struct {
	UserID, SessionID ID
	ObservedAt        time.Time
}

type PlatformAdministratorIdentitySummary struct {
	FirstName, LastName                      string
	UserID                                   ID
	EmailDisplay                             string
	Disabled, TOTPEnrolled, EmailOTPVerified bool
}

type PlatformAdministratorIdentity struct {
	UserID                       ID
	EmailDisplay, AffiliationKey string
	AuthEpoch                    int64
	Disabled                     bool
}

func (PlatformAdministratorIdentity) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED platform administrator identity]")
}

type PlatformIdentityAuthority struct {
	UserID                                      ID
	AuthEpoch                                   int64
	AffiliationKey                              string
	AssuranceLevel, AssuranceMethod             string
	AssuredAt, IdleExpiresAt, AbsoluteExpiresAt time.Time
}
