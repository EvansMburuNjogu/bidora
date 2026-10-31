package application

import (
	"context"
	"errors"
	"time"
)

type LogoutCommit struct {
	SessionID, UserID, AuditID, CorrelationID ID
	OccurredAt                                time.Time
}
type LogoutCommitter interface {
	CommitLogout(context.Context, LogoutCommit) (bool, error)
}
type LogoutService struct {
	committer LogoutCommitter
	ids       LoginIDGenerator
	clock     LoginClock
}

func NewLogoutService(committer LogoutCommitter, ids LoginIDGenerator, clock LoginClock) (*LogoutService, error) {
	if committer == nil || ids == nil || clock == nil {
		return nil, errors.New("logout_service_invalid")
	}
	return &LogoutService{committer, ids, clock}, nil
}
func (s *LogoutService) Logout(ctx context.Context, principal SessionPrincipal, correlationID ID) error {
	if ctx == nil || principal.SessionID.IsZero() || principal.UserID.IsZero() || correlationID.IsZero() {
		return safe("logout_context_invalid", nil)
	}
	if s == nil || s.committer == nil || s.ids == nil || s.clock == nil {
		return safe("logout_unavailable", nil)
	}
	auditID, err := s.ids.NewID()
	if err != nil || auditID.IsZero() {
		return safe("logout_unavailable", err)
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return safe("logout_unavailable", nil)
	}
	_, err = s.committer.CommitLogout(ctx, LogoutCommit{SessionID: principal.SessionID, UserID: principal.UserID, AuditID: auditID, CorrelationID: correlationID, OccurredAt: at})
	if err != nil {
		return safe("logout_unavailable", err)
	}
	return nil
}
