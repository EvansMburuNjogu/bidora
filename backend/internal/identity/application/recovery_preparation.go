package application

import (
	"context"
	"errors"
	"time"
)

var ErrRecoveryPreparationNotFound = errors.New("recovery_preparation_not_found")

type RecoveryAccountFacts struct{ EmailVerified, AccountDisabled, PasswordCredentialPresent, GoogleIdentityPresent bool }
type RecoveryPreparation struct {
	RecoveryRequestAvailable, PasswordResetAvailable, TokenPolicyConfigured, DeliveryPolicyConfigured, SessionEffectPolicyConfigured bool
	Facts                                                                                                                            RecoveryAccountFacts
	Blockers                                                                                                                         []string
	ObservedAt                                                                                                                       time.Time
}
type RecoveryPreparationReader interface {
	GetRecoveryAccountFacts(context.Context, ID) (RecoveryAccountFacts, error)
}
type RecoveryPreparationService struct {
	reader  RecoveryPreparationReader
	clock   SessionClock
	enabled bool
}

func NewRecoveryPreparationService(reader RecoveryPreparationReader, clock SessionClock, enabled bool) (*RecoveryPreparationService, error) {
	if reader == nil || clock == nil {
		return nil, errors.New("recovery_preparation_service_invalid")
	}
	return &RecoveryPreparationService{reader: reader, clock: clock, enabled: enabled}, nil
}
func (s *RecoveryPreparationService) Get(ctx context.Context, principal SessionPrincipal) (RecoveryPreparation, error) {
	if ctx == nil || principal.UserID.IsZero() || principal.SessionID.IsZero() || principal.AuthEpoch < 1 {
		return RecoveryPreparation{}, safe("recovery_preparation_invalid", nil)
	}
	facts, err := s.reader.GetRecoveryAccountFacts(ctx, principal.UserID)
	if errors.Is(err, ErrRecoveryPreparationNotFound) {
		return RecoveryPreparation{}, safe("recovery_preparation_not_found", err)
	}
	if err != nil {
		return RecoveryPreparation{}, safe("recovery_preparation_unavailable", err)
	}
	result := RecoveryPreparation{Facts: facts, ObservedAt: s.clock.Now().UTC(), Blockers: []string{}}
	if !facts.EmailVerified {
		result.Blockers = append(result.Blockers, "EMAIL_NOT_VERIFIED")
	}
	if facts.AccountDisabled {
		result.Blockers = append(result.Blockers, "ACCOUNT_DISABLED")
	}
	if !facts.PasswordCredentialPresent {
		result.Blockers = append(result.Blockers, "PASSWORD_CREDENTIAL_NOT_PRESENT")
	}
	result.RecoveryRequestAvailable = s.enabled
	result.PasswordResetAvailable = s.enabled
	if !s.enabled {
		result.Blockers = append(result.Blockers, "RECOVERY_DEPLOYMENT_NOT_CONFIGURED")
	}
	result.TokenPolicyConfigured = true
	result.DeliveryPolicyConfigured = true
	result.SessionEffectPolicyConfigured = true
	return result, nil
}
