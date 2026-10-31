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

type invitationIDs struct {
	next byte
	err  error
}

func (g *invitationIDs) NewID() (authzapp.ID, error) {
	if g.err != nil {
		return authzapp.ID{}, g.err
	}
	g.next++
	var id authzapp.ID
	id[15] = g.next
	return id, nil
}

type invitationClock struct{ at time.Time }

func (c invitationClock) Now() time.Time { return c.at }

type invitationCommitter struct {
	result authzapp.InvitationAcceptanceResult
	err    error
	commit authzapp.InvitationAcceptanceCommit
	calls  int
}

func (c *invitationCommitter) AcceptInvitation(_ context.Context, commit authzapp.InvitationAcceptanceCommit) (authzapp.InvitationAcceptanceResult, error) {
	c.calls++
	c.commit = commit
	return c.result, c.err
}
func invitationID(value byte) authzapp.ID { var id authzapp.ID; id[15] = value; return id }
func invitationToken(t *testing.T) invitation.Token {
	t.Helper()
	token, err := invitation.GenerateWithRandom(strings.NewReader(strings.Repeat("i", invitation.TokenBytes)))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestInvitationTokenIsOpaqueBoundedAndRedacted(t *testing.T) {
	token := invitationToken(t)
	if token.IsZero() || len(token.RevealForDelivery()) != 43 {
		t.Fatal("invitation token drifted")
	}
	hash, err := invitation.HashPresented(token.RevealForDelivery())
	if err != nil || hash != token.Hash() {
		t.Fatal("invitation hash drifted", err)
	}
	if strings.Contains(fmt.Sprintf("%#v", token), token.RevealForDelivery()) {
		t.Fatal("invitation token formatting leaked")
	}
	if _, err = invitation.HashPresented("invalid"); err == nil {
		t.Fatal("malformed invitation token accepted")
	}
	created := time.Date(2026, 10, 18, 1, 0, 0, 0, time.FixedZone("test", 3600))
	expires, err := invitation.ExpiresAt(created)
	if err != nil || !expires.Equal(created.UTC().Add(72*time.Hour)) {
		t.Fatal("invitation expiry drifted", expires, err)
	}
}

func TestInvitationAcceptanceBuildsHashedAtomicCommit(t *testing.T) {
	token := invitationToken(t)
	at := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	committer := &invitationCommitter{result: authzapp.InvitationAcceptanceResult{MembershipID: invitationID(1), TenantID: invitationID(9), Role: authzapp.BidManager, ContextGeneration: 2}}
	service, err := authzapp.NewInvitationAcceptanceService(committer, &invitationIDs{}, invitationClock{at})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Accept(context.Background(), authzapp.InvitationAcceptanceInput{Token: token.RevealForDelivery(), UserID: invitationID(7), CorrelationID: invitationID(8)})
	if err != nil {
		t.Fatal(err)
	}
	if result != committer.result || committer.calls != 1 || committer.commit.TokenHash != token.Hash() || committer.commit.UserID != invitationID(7) || committer.commit.CorrelationID != invitationID(8) || committer.commit.AcceptedAt != at || committer.commit.MembershipID != invitationID(1) || len(committer.commit.GrantIDs) != 15 || committer.commit.GrantIDs[0] != invitationID(2) || committer.commit.GrantIDs[14] != invitationID(16) || committer.commit.AuditID != invitationID(17) || committer.commit.OutboxID != invitationID(18) {
		t.Fatal("invitation acceptance commit drifted", result)
	}
	formatted := fmt.Sprintf("%#v %#v", authzapp.InvitationAcceptanceInput{Token: token.RevealForDelivery()}, committer.commit)
	if strings.Contains(formatted, token.RevealForDelivery()) {
		t.Fatal("invitation application formatting leaked token")
	}
}

func TestInvitationAcceptanceConcealsInvalidReplayAndDependencyFailure(t *testing.T) {
	token := invitationToken(t)
	for _, tc := range []struct {
		name, token string
		commitErr   error
		want        string
		calls       int
	}{{"malformed", "bad", nil, "invitation_invalid", 0}, {"expired replay mismatch", token.RevealForDelivery(), authzapp.ErrInvitationNotFound, "invitation_invalid", 1}, {"dependency", token.RevealForDelivery(), errors.New("secret-canary database"), "invitation_unavailable", 1}} {
		t.Run(tc.name, func(t *testing.T) {
			committer := &invitationCommitter{err: tc.commitErr}
			service, _ := authzapp.NewInvitationAcceptanceService(committer, &invitationIDs{}, invitationClock{time.Now()})
			result, err := service.Accept(context.Background(), authzapp.InvitationAcceptanceInput{Token: tc.token, UserID: invitationID(1), CorrelationID: invitationID(2)})
			if err == nil || err.Error() != tc.want || result != (authzapp.InvitationAcceptanceResult{}) || committer.calls != tc.calls || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("invitation failure unsafe", result, err, committer.calls)
			}
		})
	}
	if service, err := authzapp.NewInvitationAcceptanceService(nil, nil, nil); err == nil || service != nil {
		t.Fatal("nil invitation dependencies accepted")
	}
}
