package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"bidora/backend/internal/identity/googleoidc"
	"bidora/backend/internal/identity/session"
)

var ErrGoogleLinkConflict = errors.New("google_link_conflict")

type GoogleLinkCommitter interface {
	CommitGoogleLink(context.Context, GoogleLinkCommit) error
}
type GoogleLinkCommit struct {
	UserID, CurrentSessionID, ExternalIdentityID, LinkAuditID, ReplacementSessionID, LoginAuditID, CorrelationID ID
	ExpectedAuthEpoch, ExpectedCredentialVersion, NewAuthEpoch                                                   int64
	Identity                                                                                                     googleoidc.Identity
	SessionHash, CSRFHash                                                                                        [32]byte
	LinkedAt, IdleExpiresAt, AbsoluteExpiresAt                                                                   time.Time
}

func (GoogleLinkCommit) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED Google link commit]")
}

type GoogleLinkService struct {
	tokens    SessionTokenGenerator
	ids       LoginIDGenerator
	clock     LoginClock
	committer GoogleLinkCommitter
}

func NewGoogleLinkService(tokens SessionTokenGenerator, ids LoginIDGenerator, clock LoginClock, committer GoogleLinkCommitter) (*GoogleLinkService, error) {
	if tokens == nil || ids == nil || clock == nil || committer == nil {
		return nil, errors.New("google_link_service_invalid")
	}
	return &GoogleLinkService{tokens: tokens, ids: ids, clock: clock, committer: committer}, nil
}
func (s *GoogleLinkService) LinkVerified(ctx context.Context, proof GoogleLinkProof, identity googleoidc.Identity, correlationID ID) (LoginResult, error) {
	if ctx == nil || proof.UserID.IsZero() || proof.SessionID.IsZero() || proof.AuthEpoch < 1 || proof.AuthEpoch == math.MaxInt64 || proof.CredentialVersion < 1 || identity.Subject == "" || identity.Email.IsZero() || identity.IssuedAt.IsZero() || identity.ExpiresAt.IsZero() || correlationID.IsZero() {
		return LoginResult{}, safe("google_link_invalid", nil)
	}
	if s == nil || s.tokens == nil || s.ids == nil || s.clock == nil || s.committer == nil {
		return LoginResult{}, safe("google_link_unavailable", nil)
	}
	linkedAt := s.clock.Now().UTC()
	if linkedAt.IsZero() || identity.IssuedAt.After(linkedAt.Add(2*time.Minute)) || !identity.ExpiresAt.After(linkedAt.Add(-2*time.Minute)) {
		return LoginResult{}, safe("google_link_invalid", nil)
	}
	pair, err := s.tokens.GenerateSessionPair()
	if err != nil || pair.Session.IsZero() || pair.CSRF.IsZero() {
		return LoginResult{}, safe("google_link_unavailable", err)
	}
	identifiers := make([]ID, 4)
	for index := range identifiers {
		identifiers[index], err = s.ids.NewID()
		if err != nil || identifiers[index].IsZero() {
			return LoginResult{}, safe("google_link_unavailable", err)
		}
	}
	idle, absolute, err := session.Expiries(linkedAt)
	if err != nil {
		return LoginResult{}, safe("google_link_unavailable", err)
	}
	commit := GoogleLinkCommit{UserID: proof.UserID, CurrentSessionID: proof.SessionID, ExternalIdentityID: identifiers[0], LinkAuditID: identifiers[1], ReplacementSessionID: identifiers[2], LoginAuditID: identifiers[3], CorrelationID: correlationID, ExpectedAuthEpoch: proof.AuthEpoch, ExpectedCredentialVersion: proof.CredentialVersion, NewAuthEpoch: proof.AuthEpoch + 1, Identity: identity, SessionHash: pair.Session.Hash(), CSRFHash: pair.CSRF.Hash(), LinkedAt: linkedAt, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}
	if err = s.committer.CommitGoogleLink(ctx, commit); errors.Is(err, ErrGoogleLinkConflict) {
		return LoginResult{}, safe("google_link_invalid", nil)
	} else if err != nil {
		return LoginResult{}, safe("google_link_unavailable", err)
	}
	return LoginResult{Session: pair.Session, CSRF: pair.CSRF, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}, nil
}
