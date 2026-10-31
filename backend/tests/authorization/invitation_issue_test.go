package authorization_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	"bidora/backend/internal/authorization/invitation"
)

type issueTokenGenerator struct {
	token invitation.Token
	err   error
}

func (g issueTokenGenerator) Generate() (invitation.Token, error) { return g.token, g.err }

type invitationIssuerFake struct {
	result authzapp.InvitationIssueCommitResult
	err    error
	commit authzapp.InvitationIssueCommit
	calls  int
}

func (i *invitationIssuerFake) IssueInvitation(_ context.Context, commit authzapp.InvitationIssueCommit) (authzapp.InvitationIssueCommitResult, error) {
	i.calls++
	i.commit = commit
	return i.result, i.err
}

type invitationDeliveryFake struct {
	recipient, raw string
	expires        time.Time
	err            error
	calls          int
}

func (d *invitationDeliveryFake) DeliverTenantInvitation(_ context.Context, recipient string, token invitation.Token, expires time.Time) error {
	d.calls++
	d.recipient = recipient
	d.raw = token.RevealForDelivery()
	d.expires = expires
	return d.err
}
func (d *invitationDeliveryFake) DeliverTenantRoleInvitation(ctx context.Context, recipient string, _ authzapp.TenderpreneurRole, token invitation.Token, expires time.Time) error {
	return d.DeliverTenantInvitation(ctx, recipient, token, expires)
}

func TestInvitationIssueCommitsBeforeDeliveringOpaqueToken(t *testing.T) {
	token := invitationToken(t)
	at := time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
	expires := at.Add(invitation.Lifetime)
	issuer := &invitationIssuerFake{result: authzapp.InvitationIssueCommitResult{InvitationID: invitationID(1), Recipient: "Invitee@example.invalid", Role: authzapp.BidManager, ExpiresAt: expires}}
	delivery := &invitationDeliveryFake{}
	service, err := authzapp.NewInvitationIssueService(issuer, delivery, issueTokenGenerator{token: token}, &invitationIDs{}, invitationClock{at})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Issue(context.Background(), authzapp.InvitationIssueInput{ActorID: invitationID(7), TenantID: invitationID(8), CorrelationID: invitationID(9), ContextGeneration: 3, Email: "Invitee@example.invalid", Role: authzapp.BidManager})
	if err != nil {
		t.Fatal(err)
	}
	if result.InvitationID != invitationID(1) || !result.ExpiresAt.Equal(expires) || issuer.calls != 1 || delivery.calls != 1 || delivery.recipient != "Invitee@example.invalid" || delivery.raw != token.RevealForDelivery() || issuer.commit.EmailLookup != "invitee@example.invalid" || issuer.commit.TokenHash != token.Hash() || issuer.commit.ContextGeneration != 3 || issuer.commit.InvitationID != invitationID(1) || issuer.commit.AuditID != invitationID(2) || issuer.commit.OutboxID != invitationID(3) {
		t.Fatal("invitation issuance drifted", result)
	}
	if strings.Contains(fmt.Sprintf("%#v", issuer.commit), token.RevealForDelivery()) {
		t.Fatal("invitation commit leaked token")
	}
}

func TestInvitationIssueFailsClosedAndPreservesCommittedDeliveryFailure(t *testing.T) {
	token := invitationToken(t)
	at := time.Now().UTC()
	base := authzapp.InvitationIssueInput{ActorID: invitationID(1), TenantID: invitationID(2), CorrelationID: invitationID(3), ContextGeneration: 1, Email: "invitee@example.invalid", Role: authzapp.BidManager}
	for _, tc := range []struct {
		name                       string
		mutate                     func(*authzapp.InvitationIssueInput, *invitationIssuerFake, *invitationDeliveryFake)
		want                       string
		issuerCalls, deliveryCalls int
	}{{"invalid email", func(in *authzapp.InvitationIssueInput, _ *invitationIssuerFake, _ *invitationDeliveryFake) {
		in.Email = "bad"
	}, "invitation_issue_invalid", 0, 0}, {"unauthorized", func(_ *authzapp.InvitationIssueInput, i *invitationIssuerFake, _ *invitationDeliveryFake) {
		i.err = authzapp.ErrNotAuthorized
	}, "invitation_not_authorized", 1, 0}, {"database", func(_ *authzapp.InvitationIssueInput, i *invitationIssuerFake, _ *invitationDeliveryFake) {
		i.err = errors.New("secret-canary database")
	}, "invitation_issue_unavailable", 1, 0}, {"delivery after commit", func(_ *authzapp.InvitationIssueInput, _ *invitationIssuerFake, d *invitationDeliveryFake) {
		d.err = errors.New("secret-canary provider")
	}, "invitation_delivery_unavailable", 1, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			issuer := &invitationIssuerFake{result: authzapp.InvitationIssueCommitResult{InvitationID: invitationID(1), Recipient: base.Email, Role: authzapp.BidManager, ExpiresAt: at.Add(invitation.Lifetime)}}
			delivery := &invitationDeliveryFake{}
			tc.mutate(&input, issuer, delivery)
			service, _ := authzapp.NewInvitationIssueService(issuer, delivery, issueTokenGenerator{token: token}, &invitationIDs{}, invitationClock{at})
			result, err := service.Issue(context.Background(), input)
			if err == nil || err.Error() != tc.want || result != (authzapp.InvitationIssueResult{}) || issuer.calls != tc.issuerCalls || delivery.calls != tc.deliveryCalls || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("invitation issuance failure unsafe", result, err, issuer.calls, delivery.calls)
			}
		})
	}
	if service, err := authzapp.NewInvitationIssueService(nil, nil, nil, nil, nil); err == nil || service != nil {
		t.Fatal("nil invitation issue dependencies accepted")
	}
}
