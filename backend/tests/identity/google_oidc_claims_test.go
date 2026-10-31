package identity_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/identity/googleoidc"
)

func validGoogleClaims(now time.Time) googleoidc.Claims {
	return googleoidc.Claims{Issuer: "https://accounts.google.com", Audience: "client-id.apps.googleusercontent.com", Subject: "123456789012345678901", Email: "User@example.test", EmailVerified: true, Nonce: strings.Repeat("n", 43), IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
}
func TestGoogleOIDCClaimsRequireVerifiedExactBinding(t *testing.T) {
	validator, err := googleoidc.NewValidator(googleoidc.Config{ClientID: "client-id.apps.googleusercontent.com"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	claims := validGoogleClaims(now)
	identity, err := validator.Validate(claims, claims.Nonce, now)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Subject != claims.Subject || identity.Email.Display() != claims.Email || identity.Email.Lookup() != "user@example.test" {
		t.Fatal("Google identity mapping drifted", identity)
	}
	if formatted := fmt.Sprintf("%#v %#v", claims, identity); strings.Contains(formatted, claims.Email) || strings.Contains(formatted, claims.Subject) {
		t.Fatal("Google claims formatting leaked identity")
	}
}
func TestGoogleOIDCClaimsRejectTakeoverRelevantDrift(t *testing.T) {
	now := time.Now().UTC()
	valid := validGoogleClaims(now)
	cases := []struct {
		name   string
		mutate func(*googleoidc.Claims)
		nonce  string
	}{{"issuer", func(c *googleoidc.Claims) { c.Issuer = "https://evil.example" }, valid.Nonce}, {"audience", func(c *googleoidc.Claims) { c.Audience = "other-client" }, valid.Nonce}, {"nonce", func(*googleoidc.Claims) {}, strings.Repeat("x", 43)}, {"unverified email", func(c *googleoidc.Claims) { c.EmailVerified = false }, valid.Nonce}, {"email", func(c *googleoidc.Claims) { c.Email = "bad" }, valid.Nonce}, {"subject", func(c *googleoidc.Claims) { c.Subject = "" }, valid.Nonce}, {"future", func(c *googleoidc.Claims) { c.IssuedAt = now.Add(3 * time.Minute) }, valid.Nonce}, {"expired", func(c *googleoidc.Claims) { c.ExpiresAt = now.Add(-3 * time.Minute) }, valid.Nonce}, {"long lifetime", func(c *googleoidc.Claims) { c.ExpiresAt = c.IssuedAt.Add(3 * time.Hour) }, valid.Nonce}}
	validator, _ := googleoidc.NewValidator(googleoidc.Config{ClientID: valid.Audience})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := valid
			tc.mutate(&claims)
			if identity, err := validator.Validate(claims, tc.nonce, now); err == nil || identity != (googleoidc.Identity{}) {
				t.Fatal("unsafe Google claims accepted", identity)
			}
		})
	}
	if validator, err := googleoidc.NewValidator(googleoidc.Config{}); err == nil || validator != nil {
		t.Fatal("empty Google client ID accepted")
	}
	var zero *googleoidc.Validator
	if _, err := zero.Validate(valid, valid.Nonce, now); err == nil {
		t.Fatal("zero Google validator accepted")
	}
}
