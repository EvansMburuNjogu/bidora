package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bidora/backend/internal/authorization/invitation"
)

var ErrInvitationNotFound = errors.New("invitation_not_found")

type InvitationIDGenerator interface{ NewID() (ID, error) }
type InvitationClock interface{ Now() time.Time }
type InvitationAcceptanceCommitter interface {
	AcceptInvitation(context.Context, InvitationAcceptanceCommit) (InvitationAcceptanceResult, error)
}
type InvitationAcceptanceInput struct {
	Token         string
	UserID        ID
	CorrelationID ID
}
type InvitationAcceptanceCommit struct {
	TokenHash                                              [32]byte
	UserID, MembershipID, AuditID, OutboxID, CorrelationID ID
	GrantIDs                                               []ID
	AcceptedAt                                             time.Time
}
type InvitationAcceptanceResult struct {
	MembershipID, TenantID ID
	Role                   TenderpreneurRole
	ContextGeneration      int64
}

func (InvitationAcceptanceInput) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED invitation acceptance input]")
}
func (InvitationAcceptanceCommit) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED invitation acceptance commit]")
}
func (InvitationAcceptanceResult) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED invitation acceptance result]")
}

type InvitationAcceptanceService struct {
	committer InvitationAcceptanceCommitter
	ids       InvitationIDGenerator
	clock     InvitationClock
}

func NewInvitationAcceptanceService(committer InvitationAcceptanceCommitter, ids InvitationIDGenerator, clock InvitationClock) (*InvitationAcceptanceService, error) {
	if committer == nil || ids == nil || clock == nil {
		return nil, errors.New("invitation_acceptance_service_invalid")
	}
	return &InvitationAcceptanceService{committer: committer, ids: ids, clock: clock}, nil
}
func (s *InvitationAcceptanceService) Accept(ctx context.Context, input InvitationAcceptanceInput) (InvitationAcceptanceResult, error) {
	if ctx == nil || input.UserID.IsZero() || input.CorrelationID.IsZero() {
		return InvitationAcceptanceResult{}, safe("invitation_invalid", nil)
	}
	if s == nil || s.committer == nil || s.ids == nil || s.clock == nil {
		return InvitationAcceptanceResult{}, safe("invitation_unavailable", nil)
	}
	hash, err := invitation.HashPresented(input.Token)
	if err != nil {
		return InvitationAcceptanceResult{}, safe("invitation_invalid", nil)
	}
	ids := make([]ID, 18)
	for index := range ids {
		ids[index], err = s.ids.NewID()
		if err != nil || ids[index].IsZero() {
			return InvitationAcceptanceResult{}, safe("invitation_unavailable", err)
		}
	}
	acceptedAt := s.clock.Now().UTC()
	if acceptedAt.IsZero() {
		return InvitationAcceptanceResult{}, safe("invitation_unavailable", nil)
	}
	result, err := s.committer.AcceptInvitation(ctx, InvitationAcceptanceCommit{TokenHash: hash, UserID: input.UserID, MembershipID: ids[0], GrantIDs: ids[1:16], AuditID: ids[16], OutboxID: ids[17], CorrelationID: input.CorrelationID, AcceptedAt: acceptedAt})
	if errors.Is(err, ErrInvitationNotFound) {
		return InvitationAcceptanceResult{}, safe("invitation_invalid", nil)
	}
	if err != nil {
		return InvitationAcceptanceResult{}, safe("invitation_unavailable", err)
	}
	if result.MembershipID != ids[0] || result.TenantID.IsZero() || result.ContextGeneration < 1 {
		return InvitationAcceptanceResult{}, safe("invitation_unavailable", nil)
	}
	if _, err := ParseTenderpreneurRole(string(result.Role)); err != nil {
		return InvitationAcceptanceResult{}, safe("invitation_unavailable", nil)
	}
	return result, nil
}
