package application

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrInvitationNotCancelable = errors.New("invitation_not_cancelable")

type InvitationCancellationInput struct {
	ActorID, TenantID, InvitationID, CorrelationID ID
	ContextGeneration                              int64
}
type InvitationCancellationCommit struct {
	ActorID, TenantID, InvitationID, AuditID, OutboxID, CorrelationID ID
	ContextGeneration                                                 int64
	CancelledAt                                                       time.Time
}
type InvitationCancellationCommitter interface {
	Cancel(context.Context, InvitationCancellationCommit) error
}

func (InvitationCancellationInput) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED invitation cancellation input]")
}
func (InvitationCancellationCommit) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED invitation cancellation commit]")
}

type InvitationCancellationService struct {
	committer InvitationCancellationCommitter
	ids       InvitationIDGenerator
	clock     InvitationClock
}

func NewInvitationCancellationService(committer InvitationCancellationCommitter, ids InvitationIDGenerator, clock InvitationClock) (*InvitationCancellationService, error) {
	if committer == nil || ids == nil || clock == nil {
		return nil, errors.New("invitation_cancellation_service_invalid")
	}
	return &InvitationCancellationService{committer, ids, clock}, nil
}
func (s *InvitationCancellationService) Cancel(ctx context.Context, input InvitationCancellationInput) error {
	if ctx == nil || input.ActorID.IsZero() || input.TenantID.IsZero() || input.InvitationID.IsZero() || input.CorrelationID.IsZero() || input.ContextGeneration < 1 {
		return safe("invitation_cancellation_invalid", nil)
	}
	if s == nil || s.committer == nil || s.ids == nil || s.clock == nil {
		return safe("invitation_cancellation_unavailable", nil)
	}
	auditID, err := s.ids.NewID()
	if err != nil || auditID.IsZero() {
		return safe("invitation_cancellation_unavailable", err)
	}
	outboxID, err := s.ids.NewID()
	if err != nil || outboxID.IsZero() {
		return safe("invitation_cancellation_unavailable", err)
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return safe("invitation_cancellation_unavailable", nil)
	}
	err = s.committer.Cancel(ctx, InvitationCancellationCommit{ActorID: input.ActorID, TenantID: input.TenantID, InvitationID: input.InvitationID, AuditID: auditID, OutboxID: outboxID, CorrelationID: input.CorrelationID, ContextGeneration: input.ContextGeneration, CancelledAt: at})
	if errors.Is(err, ErrInvitationNotCancelable) {
		return safe("invitation_cancellation_not_found", nil)
	}
	if err != nil {
		return safe("invitation_cancellation_unavailable", err)
	}
	return nil
}
