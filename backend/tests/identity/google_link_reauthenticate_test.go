package identity_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	identityapp "bidora/backend/internal/identity/application"
)

func googleLinkPrincipal() identityapp.SessionPrincipal {
	return identityapp.SessionPrincipal{SessionID: identityID(8), UserID: identityID(9), AuthEpoch: 4}
}
func TestGoogleLinkRequiresFreshPasswordForSameCurrentUser(t *testing.T) {
	auth := &authServiceFake{result: identityapp.Authentication{UserID: identityID(9), AuthEpoch: 4, CredentialVersion: 2}}
	rehash := &rehasherFake{}
	service, err := identityapp.NewGoogleLinkReauthenticationService(auth, rehash)
	if err != nil {
		t.Fatal(err)
	}
	input := identityapp.GoogleLinkReauthenticationInput{Principal: googleLinkPrincipal(), Credentials: loginInput("Display@Example.com", []byte("fresh password proof"))}
	proof, err := service.Reauthenticate(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if proof.UserID != identityID(9) || proof.SessionID != identityID(8) || proof.AuthEpoch != 4 || proof.CredentialVersion != 2 || auth.calls != 1 || rehash.calls != 0 {
		t.Fatal("Google link proof drifted", proof)
	}
	formatted := fmt.Sprintf("%#v %#v", input, proof)
	if strings.Contains(formatted, "fresh password proof") || strings.Contains(formatted, "Display@Example.com") {
		t.Fatal("Google link proof formatting leaked credentials")
	}
}
func TestGoogleLinkReauthenticationRehashesBeforeProof(t *testing.T) {
	auth := &authServiceFake{result: identityapp.Authentication{UserID: identityID(9), AuthEpoch: 4, CredentialVersion: 3, NeedsPasswordRehash: true}}
	rehash := &rehasherFake{}
	service, _ := identityapp.NewGoogleLinkReauthenticationService(auth, rehash)
	input := identityapp.GoogleLinkReauthenticationInput{Principal: googleLinkPrincipal(), Credentials: loginInput("user@example.com", []byte("fresh password proof"))}
	proof, err := service.Reauthenticate(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if proof.UserID.IsZero() || proof.CredentialVersion != 4 || rehash.calls != 1 || rehash.user != identityID(9) || rehash.version != 3 || string(rehash.password) != "fresh password proof" {
		t.Fatal("Google link rehash drifted", proof)
	}
}
func TestGoogleLinkReauthenticationRejectsCrossAccountAndStaleEpoch(t *testing.T) {
	for _, authentication := range []identityapp.Authentication{{UserID: identityID(7), AuthEpoch: 4, CredentialVersion: 1}, {UserID: identityID(9), AuthEpoch: 3, CredentialVersion: 1}, {UserID: identityID(9), AuthEpoch: 4, CredentialVersion: 0}} {
		auth := &authServiceFake{result: authentication}
		service, _ := identityapp.NewGoogleLinkReauthenticationService(auth, &rehasherFake{})
		proof, err := service.Reauthenticate(context.Background(), identityapp.GoogleLinkReauthenticationInput{Principal: googleLinkPrincipal(), Credentials: loginInput("user@example.com", []byte("password"))})
		if err == nil || err.Error() != "google_link_reauthentication_invalid" || proof != (identityapp.GoogleLinkProof{}) {
			t.Fatal("unsafe Google link proof accepted", authentication, proof, err)
		}
	}
}
func TestGoogleLinkReauthenticationMapsAttemptAndDependencyFailures(t *testing.T) {
	for _, tc := range []struct {
		authErr, rehashErr error
		needs              bool
		want               string
	}{{errors.New("credentials_invalid"), nil, false, "google_link_reauthentication_invalid"}, {errors.New("attempt_denied"), nil, false, "attempt_denied"}, {errors.New("authentication_unavailable"), nil, false, "google_link_reauthentication_unavailable"}, {nil, errors.New("secret-canary rehash"), true, "google_link_reauthentication_unavailable"}} {
		auth := &authServiceFake{result: identityapp.Authentication{UserID: identityID(9), AuthEpoch: 4, CredentialVersion: 1, NeedsPasswordRehash: tc.needs}, err: tc.authErr}
		service, _ := identityapp.NewGoogleLinkReauthenticationService(auth, &rehasherFake{err: tc.rehashErr})
		proof, err := service.Reauthenticate(context.Background(), identityapp.GoogleLinkReauthenticationInput{Principal: googleLinkPrincipal(), Credentials: loginInput("user@example.com", []byte("password"))})
		if err == nil || err.Error() != tc.want || proof != (identityapp.GoogleLinkProof{}) || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
			t.Fatal("Google link reauthentication failure drifted", proof, err)
		}
	}
	if service, err := identityapp.NewGoogleLinkReauthenticationService(nil, &rehasherFake{}); err == nil || service != nil {
		t.Fatal("nil authenticator accepted")
	}
	var zero *identityapp.GoogleLinkReauthenticationService
	if _, err := zero.Reauthenticate(context.Background(), identityapp.GoogleLinkReauthenticationInput{Principal: googleLinkPrincipal()}); err == nil {
		t.Fatal("zero reauthentication service accepted")
	}
}
