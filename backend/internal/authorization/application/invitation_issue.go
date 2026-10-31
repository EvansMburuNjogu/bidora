package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bidora/backend/internal/authorization/invitation"
	"bidora/backend/internal/identity/email"
)

type InvitationTokenGenerator interface {
	Generate() (invitation.Token, error)
}
type SecureInvitationTokenGenerator struct{}

func (SecureInvitationTokenGenerator) Generate() (invitation.Token, error) {
	return invitation.Generate()
}

type InvitationIssuer interface {
	IssueInvitation(context.Context, InvitationIssueCommit) (InvitationIssueCommitResult, error)
}
type InvitationDelivery interface {
	DeliverTenantInvitation(context.Context, string, invitation.Token, time.Time) error
}
type RoleInvitationDelivery interface {
	DeliverTenantRoleInvitation(context.Context, string, TenderpreneurRole, invitation.Token, time.Time) error
}
type TenantInvitationDeliveryRequest struct {
	InvitationID, ActorID, TenantID, CorrelationID ID
	Recipient                                      string
	Role                                           TenderpreneurRole
	Token                                          invitation.Token
	ExpiresAt                                      time.Time
}
type EvidencedRoleInvitationDelivery interface {
	DeliverTenantRoleInvitationWithEvidence(context.Context, TenantInvitationDeliveryRequest) error
}
type InvitationIssueInput struct {
	ActorID, TenantID, CorrelationID ID
	ContextGeneration                int64
	Email                            string
	Role                             TenderpreneurRole
}
type InvitationIssueCommit struct {
	InvitationID, AuditID, OutboxID  ID
	ActorID, TenantID, CorrelationID ID
	ContextGeneration                int64
	EmailDisplay, EmailLookup        string
	Role                             TenderpreneurRole
	TokenHash                        [32]byte
	CreatedAt, ExpiresAt             time.Time
}
type InvitationIssueCommitResult struct {
	InvitationID ID
	Recipient    string
	Role         TenderpreneurRole
	ExpiresAt    time.Time
}
type InvitationIssueResult struct {
	InvitationID ID
	Role         TenderpreneurRole
	ExpiresAt    time.Time
}

func (InvitationIssueInput) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED invitation issue input]")
}
func (InvitationIssueCommit) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED invitation issue commit]")
}
func (InvitationIssueCommitResult) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED invitation issue commit result]")
}
func (InvitationIssueResult) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED invitation issue result]")
}

type InvitationIssueService struct {
	issuer   InvitationIssuer
	delivery InvitationDelivery
	tokens   InvitationTokenGenerator
	ids      InvitationIDGenerator
	clock    InvitationClock
}

func NewInvitationIssueService(issuer InvitationIssuer, delivery InvitationDelivery, tokens InvitationTokenGenerator, ids InvitationIDGenerator, clock InvitationClock) (*InvitationIssueService, error) {
	if issuer == nil || delivery == nil || tokens == nil || ids == nil || clock == nil {
		return nil, errors.New("invitation_issue_service_invalid")
	}
	return &InvitationIssueService{issuer, delivery, tokens, ids, clock}, nil
}
func (s *InvitationIssueService) Issue(ctx context.Context, input InvitationIssueInput) (InvitationIssueResult, error) {
	if ctx == nil || input.ActorID.IsZero() || input.TenantID.IsZero() || input.CorrelationID.IsZero() || input.ContextGeneration < 1 {
		return InvitationIssueResult{}, safe("invitation_issue_invalid", nil)
	}
	if s == nil || s.issuer == nil || s.delivery == nil || s.tokens == nil || s.ids == nil || s.clock == nil {
		return InvitationIssueResult{}, safe("invitation_issue_unavailable", nil)
	}
	address, err := email.Parse(input.Email)
	if err != nil {
		return InvitationIssueResult{}, safe("invitation_issue_invalid", nil)
	}
	role, err := ParseTenderpreneurRole(string(input.Role))
	if err != nil {
		return InvitationIssueResult{}, safe("invitation_issue_invalid", nil)
	}
	token, err := s.tokens.Generate()
	if err != nil || token.IsZero() {
		return InvitationIssueResult{}, safe("invitation_issue_unavailable", err)
	}
	ids := make([]ID, 3)
	for index := range ids {
		ids[index], err = s.ids.NewID()
		if err != nil || ids[index].IsZero() {
			return InvitationIssueResult{}, safe("invitation_issue_unavailable", err)
		}
	}
	createdAt := s.clock.Now().UTC()
	expiresAt, err := invitation.ExpiresAt(createdAt)
	if err != nil {
		return InvitationIssueResult{}, safe("invitation_issue_unavailable", err)
	}
	committed, err := s.issuer.IssueInvitation(ctx, InvitationIssueCommit{InvitationID: ids[0], AuditID: ids[1], OutboxID: ids[2], ActorID: input.ActorID, TenantID: input.TenantID, CorrelationID: input.CorrelationID, ContextGeneration: input.ContextGeneration, EmailDisplay: address.Display(), EmailLookup: address.Lookup(), Role: role, TokenHash: token.Hash(), CreatedAt: createdAt, ExpiresAt: expiresAt})
	if errors.Is(err, ErrNotAuthorized) {
		return InvitationIssueResult{}, safe("invitation_not_authorized", nil)
	}
	if err != nil {
		return InvitationIssueResult{}, safe("invitation_issue_unavailable", err)
	}
	if committed.InvitationID != ids[0] || committed.Recipient != address.Display() || committed.Role != role || !committed.ExpiresAt.Equal(expiresAt) {
		return InvitationIssueResult{}, safe("invitation_issue_unavailable", nil)
	}
	if delivery, ok := s.delivery.(EvidencedRoleInvitationDelivery); ok {
		err = delivery.DeliverTenantRoleInvitationWithEvidence(ctx, TenantInvitationDeliveryRequest{InvitationID: committed.InvitationID, ActorID: input.ActorID, TenantID: input.TenantID, CorrelationID: input.CorrelationID, Recipient: committed.Recipient, Role: committed.Role, Token: token, ExpiresAt: expiresAt})
	} else if roleDelivery, ok := s.delivery.(RoleInvitationDelivery); ok {
		err = roleDelivery.DeliverTenantRoleInvitation(ctx, committed.Recipient, committed.Role, token, expiresAt)
	} else {
		return InvitationIssueResult{}, safe("invitation_delivery_unavailable", nil)
	}
	if err != nil {
		return InvitationIssueResult{}, safe("invitation_delivery_unavailable", err)
	}
	return InvitationIssueResult{InvitationID: ids[0], Role: role, ExpiresAt: expiresAt}, nil
}
