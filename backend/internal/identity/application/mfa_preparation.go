package application

import (
	"context"
	"errors"
	"time"
)

type MFAReadiness struct {
	Facts                                                                                                                                     RecoveryAccountFacts
	MFAEnrollmentAvailable, MFAEnrolled, StepUpAvailable, StepUpPolicyConfigured, SessionAssuranceProvenanceAvailable, RecoveryCodesAvailable bool
	SessionAuthenticatedAt, ObservedAt                                                                                                        time.Time
	SessionAgeSeconds                                                                                                                         int64
	Blockers                                                                                                                                  []string
}
type MFAStatusReader interface {
	HasActiveMFA(context.Context, ID) (bool, error)
}
type MFAReadinessService struct {
	reader    RecoveryPreparationReader
	factors   MFAStatusReader
	clock     SessionClock
	emailOnly bool
}

func NewMFAReadinessService(reader RecoveryPreparationReader, clock SessionClock) (*MFAReadinessService, error) {
	if reader == nil || clock == nil {
		return nil, errors.New("mfa_readiness_service_invalid")
	}
	return &MFAReadinessService{reader: reader, clock: clock}, nil
}
func NewMFAReadinessServiceWithFactors(reader RecoveryPreparationReader, factors MFAStatusReader, clock SessionClock, emailOnly ...bool) (*MFAReadinessService, error) {
	if reader == nil || factors == nil || clock == nil || len(emailOnly) > 1 {
		return nil, errors.New("mfa_readiness_service_invalid")
	}
	return &MFAReadinessService{reader: reader, factors: factors, clock: clock, emailOnly: len(emailOnly) == 1 && emailOnly[0]}, nil
}
func (s *MFAReadinessService) Get(ctx context.Context, principal SessionPrincipal) (MFAReadiness, error) {
	if ctx == nil || principal.UserID.IsZero() || principal.SessionID.IsZero() || principal.AuthEpoch < 1 || principal.AbsoluteExpiresAt.IsZero() {
		return MFAReadiness{}, safe("mfa_readiness_invalid", nil)
	}
	facts, err := s.reader.GetRecoveryAccountFacts(ctx, principal.UserID)
	if errors.Is(err, ErrRecoveryPreparationNotFound) {
		return MFAReadiness{}, safe("mfa_readiness_not_found", err)
	}
	if err != nil {
		return MFAReadiness{}, safe("mfa_readiness_unavailable", err)
	}
	observed := s.clock.Now().UTC()
	authenticatedAt := principal.AbsoluteExpiresAt.UTC().Add(-12 * time.Hour)
	age := int64(observed.Sub(authenticatedAt).Seconds())
	if age < 0 {
		age = 0
	}
	enrolled := false
	if s.factors != nil {
		enrolled, err = s.factors.HasActiveMFA(ctx, principal.UserID)
		if err != nil {
			return MFAReadiness{}, safe("mfa_readiness_unavailable", err)
		}
	}
	return MFAReadiness{Facts: facts, MFAEnrollmentAvailable: !s.emailOnly && !enrolled, MFAEnrolled: enrolled, StepUpAvailable: !s.emailOnly && enrolled, StepUpPolicyConfigured: true, SessionAssuranceProvenanceAvailable: true, RecoveryCodesAvailable: !s.emailOnly && enrolled, SessionAuthenticatedAt: authenticatedAt, ObservedAt: observed, SessionAgeSeconds: age}, nil
}
