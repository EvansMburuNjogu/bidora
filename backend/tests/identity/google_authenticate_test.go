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

type googleResolverFake struct {
	commit identityapp.GoogleIdentityCommit
	result identityapp.GoogleIdentityResult
	err    error
	calls  int
}

func (r *googleResolverFake) ResolveGoogleIdentity(_ context.Context, commit identityapp.GoogleIdentityCommit) (identityapp.GoogleIdentityResult, error) {
	r.calls++
	r.commit = commit
	return r.result, r.err
}
func TestGoogleAuthenticationValidatesClaimsBeforeResolution(t *testing.T) {
	now := time.Now().UTC()
	claims := validGoogleClaims(now)
	validator, _ := googleoidc.NewValidator(googleoidc.Config{ClientID: claims.Audience})
	resolver := &googleResolverFake{result: identityapp.GoogleIdentityResult{UserID: identityID(9), AuthEpoch: 1, Created: true}}
	service, err := identityapp.NewGoogleAuthenticationService(validator, resolver, &sequenceIDs{values: []identityapp.ID{identityID(1), identityID(2), identityID(3)}}, loginClockFake{at: now})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Authenticate(context.Background(), identityapp.GoogleAuthenticationInput{Claims: claims, ExpectedNonce: claims.Nonce, CorrelationID: identityID(4)})
	if err != nil {
		t.Fatal(err)
	}
	if result.UserID != identityID(9) || resolver.calls != 1 || resolver.commit.UserID != identityID(1) || resolver.commit.ExternalIdentityID != identityID(2) || resolver.commit.AuditID != identityID(3) || resolver.commit.Identity.Subject != claims.Subject {
		t.Fatal("Google authentication orchestration drifted", result)
	}
	if strings.Contains(fmt.Sprintf("%#v", resolver.commit), claims.Email) {
		t.Fatal("Google commit formatting leaked identity")
	}
}
func TestGoogleAuthenticationAcceptsOnlyCurrentVerifiedIdentityBoundary(t *testing.T) {
	now := time.Now().UTC()
	claims := validGoogleClaims(now)
	validator, _ := googleoidc.NewValidator(googleoidc.Config{ClientID: claims.Audience})
	identity, err := validator.Validate(claims, claims.Nonce, now)
	if err != nil {
		t.Fatal(err)
	}
	resolver := &googleResolverFake{result: identityapp.GoogleIdentityResult{UserID: identityID(9), AuthEpoch: 1}}
	service, _ := identityapp.NewGoogleAuthenticationService(validator, resolver, &sequenceIDs{values: []identityapp.ID{identityID(1), identityID(2), identityID(3)}}, loginClockFake{at: now})
	result, err := service.AuthenticateVerified(context.Background(), identityapp.VerifiedGoogleAuthenticationInput{Identity: identity, CorrelationID: identityID(4)})
	if err != nil || result.UserID != identityID(9) || resolver.calls != 1 {
		t.Fatal("verified Google identity boundary failed", result, err)
	}
	identity.ExpiresAt = now.Add(-3 * time.Minute)
	if result, err = service.AuthenticateVerified(context.Background(), identityapp.VerifiedGoogleAuthenticationInput{Identity: identity, CorrelationID: identityID(4)}); err == nil || err.Error() != "google_authentication_invalid" || result != (identityapp.GoogleIdentityResult{}) || resolver.calls != 1 {
		t.Fatal("expired verified identity accepted", result, err)
	}
}

func TestGoogleAuthenticationConcealsLinkAndClaimFailures(t *testing.T) {
	now := time.Now().UTC()
	claims := validGoogleClaims(now)
	validator, _ := googleoidc.NewValidator(googleoidc.Config{ClientID: claims.Audience})
	for _, tc := range []struct {
		name        string
		mutate      func(*identityapp.GoogleAuthenticationInput)
		resolverErr error
		want        string
	}{{"nonce", func(i *identityapp.GoogleAuthenticationInput) { i.ExpectedNonce = strings.Repeat("x", 43) }, nil, "google_authentication_invalid"}, {"link", func(*identityapp.GoogleAuthenticationInput) {}, identityapp.ErrGoogleLinkRequired, "google_authentication_invalid"}, {"disabled", func(*identityapp.GoogleAuthenticationInput) {}, identityapp.ErrGoogleAuthenticationDenied, "google_authentication_invalid"}, {"database", func(*identityapp.GoogleAuthenticationInput) {}, errors.New("secret-canary Google database"), "google_authentication_unavailable"}} {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &googleResolverFake{err: tc.resolverErr}
			service, _ := identityapp.NewGoogleAuthenticationService(validator, resolver, &sequenceIDs{values: []identityapp.ID{identityID(1), identityID(2), identityID(3)}}, loginClockFake{at: now})
			input := identityapp.GoogleAuthenticationInput{Claims: claims, ExpectedNonce: claims.Nonce, CorrelationID: identityID(4)}
			tc.mutate(&input)
			result, err := service.Authenticate(context.Background(), input)
			if err == nil || err.Error() != tc.want || result != (identityapp.GoogleIdentityResult{}) || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("Google failure unsafe", result, err)
			}
			if tc.name == "nonce" && resolver.calls != 0 {
				t.Fatal("invalid claims reached persistence")
			}
		})
	}
	if service, err := identityapp.NewGoogleAuthenticationService(nil, &googleResolverFake{}, &sequenceIDs{}, loginClockFake{}); err == nil || service != nil {
		t.Fatal("nil Google validator accepted")
	}
	var zero *identityapp.GoogleAuthenticationService
	if _, err := zero.Authenticate(context.Background(), identityapp.GoogleAuthenticationInput{}); err == nil {
		t.Fatal("zero Google service accepted")
	}
}
