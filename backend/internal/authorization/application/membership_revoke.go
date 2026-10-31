package application

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrMembershipNotRevocable = errors.New("membership_not_revocable")

type MembershipRevocationCommitter interface {
	RevokeCaptureContributor(context.Context, MembershipRevocationCommit) (MembershipRevocationResult, error)
}
type MembershipRevocationInput struct {
	ActorID, TenantID, TargetMembershipID, CorrelationID ID
	ActorContextGeneration                               int64
}
type MembershipRevocationCommit struct {
	ActorID, TenantID, TargetMembershipID, AuditID, OutboxID, CorrelationID ID
	ActorContextGeneration                                                  int64
	RevokedAt                                                               time.Time
}
type MembershipRevocationResult struct {
	MembershipID, TenantID ID
	ContextGeneration      int64
}

func (MembershipRevocationInput) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED membership revocation input]")
}
func (MembershipRevocationCommit) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED membership revocation commit]")
}
func (MembershipRevocationResult) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED membership revocation result]")
}

type MembershipRevocationService struct {
	committer MembershipRevocationCommitter
	ids       InvitationIDGenerator
	clock     InvitationClock
}

func NewMembershipRevocationService(committer MembershipRevocationCommitter, ids InvitationIDGenerator, clock InvitationClock) (*MembershipRevocationService, error) {
	if committer == nil || ids == nil || clock == nil {
		return nil, errors.New("membership_revocation_service_invalid")
	}
	return &MembershipRevocationService{committer, ids, clock}, nil
}
func (s *MembershipRevocationService) Revoke(ctx context.Context, input MembershipRevocationInput) (MembershipRevocationResult, error) {
	if ctx == nil || input.ActorID.IsZero() || input.TenantID.IsZero() || input.TargetMembershipID.IsZero() || input.CorrelationID.IsZero() || input.ActorContextGeneration < 1 {
		return MembershipRevocationResult{}, safe("membership_revocation_invalid", nil)
	}
	if s == nil || s.committer == nil || s.ids == nil || s.clock == nil {
		return MembershipRevocationResult{}, safe("membership_revocation_unavailable", nil)
	}
	auditID, err := s.ids.NewID()
	if err != nil || auditID.IsZero() {
		return MembershipRevocationResult{}, safe("membership_revocation_unavailable", err)
	}
	outboxID, err := s.ids.NewID()
	if err != nil || outboxID.IsZero() {
		return MembershipRevocationResult{}, safe("membership_revocation_unavailable", err)
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return MembershipRevocationResult{}, safe("membership_revocation_unavailable", nil)
	}
	result, err := s.committer.RevokeCaptureContributor(ctx, MembershipRevocationCommit{ActorID: input.ActorID, TenantID: input.TenantID, TargetMembershipID: input.TargetMembershipID, AuditID: auditID, OutboxID: outboxID, CorrelationID: input.CorrelationID, ActorContextGeneration: input.ActorContextGeneration, RevokedAt: at})
	if errors.Is(err, ErrMembershipNotRevocable) {
		return MembershipRevocationResult{}, safe("membership_revocation_not_authorized", nil)
	}
	if err != nil {
		return MembershipRevocationResult{}, safe("membership_revocation_unavailable", err)
	}
	if result.MembershipID != input.TargetMembershipID || result.TenantID != input.TenantID || result.ContextGeneration < 1 {
		return MembershipRevocationResult{}, safe("membership_revocation_unavailable", nil)
	}
	return result, nil
}
