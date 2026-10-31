package identity_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/googleoidc"
)

type googleLinkCommitterFake struct {
	commit identityapp.GoogleLinkCommit
	err    error
	calls  int
}

func (c *googleLinkCommitterFake) CommitGoogleLink(_ context.Context, commit identityapp.GoogleLinkCommit) error {
	c.calls++
	c.commit = commit
	return c.err
}
func TestGoogleLinkRotatesSessionOnlyAfterAtomicCommit(t *testing.T) {
	now := time.Now().UTC()
	identity := googleFlowIdentity(t)
	tokens := &tokenGeneratorFake{pair: sessionPair(t)}
	ids := &sequenceIDs{values: []identityapp.ID{identityID(1), identityID(2), identityID(3), identityID(4)}}
	committer := &googleLinkCommitterFake{}
	service, err := identityapp.NewGoogleLinkService(tokens, ids, loginClockFake{at: now}, committer)
	if err != nil {
		t.Fatal(err)
	}
	proof := identityapp.GoogleLinkProof{UserID: identityID(9), SessionID: identityID(8), AuthEpoch: 4, CredentialVersion: 3}
	result, err := service.LinkVerified(context.Background(), proof, identity, identityID(7))
	if err != nil {
		t.Fatal(err)
	}
	commit := committer.commit
	if committer.calls != 1 || commit.UserID != proof.UserID || commit.CurrentSessionID != proof.SessionID || commit.ExpectedAuthEpoch != 4 || commit.ExpectedCredentialVersion != 3 || commit.NewAuthEpoch != 5 || commit.ExternalIdentityID != identityID(1) || commit.LinkAuditID != identityID(2) || commit.ReplacementSessionID != identityID(3) || commit.LoginAuditID != identityID(4) || commit.Identity.Subject != identity.Subject || commit.SessionHash != tokens.pair.Session.Hash() || result.Session.RevealForCookie() != tokens.pair.Session.RevealForCookie() {
		t.Fatal("Google link commit mapping drifted", commit)
	}
	if strings.Contains(fmt.Sprintf("%#v %#v", commit, result), identity.Email.Display()) || strings.Contains(fmt.Sprintf("%#v", result), result.Session.RevealForCookie()) {
		t.Fatal("Google link formatting leaked private data")
	}
}
func TestGoogleLinkReturnsNoReplacementSecretsOnConflictOrFailure(t *testing.T) {
	for _, tc := range []struct {
		name      string
		commitErr error
		want      string
	}{{"conflict", identityapp.ErrGoogleLinkConflict, "google_link_invalid"}, {"database", errors.New("secret-canary database"), "google_link_unavailable"}} {
		t.Run(tc.name, func(t *testing.T) {
			committer := &googleLinkCommitterFake{err: tc.commitErr}
			service, _ := identityapp.NewGoogleLinkService(&tokenGeneratorFake{pair: sessionPair(t)}, &sequenceIDs{values: []identityapp.ID{identityID(1), identityID(2), identityID(3), identityID(4)}}, loginClockFake{at: time.Now().UTC()}, committer)
			result, err := service.LinkVerified(context.Background(), identityapp.GoogleLinkProof{UserID: identityID(9), SessionID: identityID(8), AuthEpoch: 1, CredentialVersion: 1}, googleFlowIdentity(t), identityID(7))
			if err == nil || err.Error() != tc.want || result != (identityapp.LoginResult{}) || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("Google link failure exposed replacement", result, err)
			}
		})
	}
	var zero *identityapp.GoogleLinkService
	if _, err := zero.LinkVerified(context.Background(), identityapp.GoogleLinkProof{}, googleoidc.Identity{}, identityID(1)); err == nil {
		t.Fatal("zero Google link service accepted")
	}
}
