package notification_test

import (
	"strings"
	"testing"

	"bidora/backend/internal/emailhtml"
	app "bidora/backend/internal/notification/application"
)

func TestEmailLayoutEscapesContentAndUsesTokenFreeActions(t *testing.T) {
	html := emailhtml.Render(emailhtml.Content{Brand: "<script>alert(1)</script>", Title: "Sign in", Body: "<img src=x onerror=bad>", Code: "123456", Origin: "https://sandbox.bidora.africa", Key: "identity.login_code"})
	for _, want := range []string{"role=\"presentation\"", "123456", "Continue sign-in", "https://sandbox.bidora.africa/auth/login-code", "&lt;script&gt;"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing expected layout element %q", want)
		}
	}
	for _, forbidden := range []string{"<script>", "<img src=x", "?code=", "?token=", "javascript:"} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("unsafe layout element %q", forbidden)
		}
	}
}

func TestEmailUsesExistingBrandLogoAndPalette(t *testing.T) {
	html := emailhtml.Render(emailhtml.Content{Brand: "Bidora", Title: "Sign in", Origin: "https://sandbox.bidora.africa", Key: "identity.login_code"})
	for _, want := range []string{"https://sandbox.bidora.africa/bidora-aperture-192.png", "#0f766e", "#e4f0ed", "#173b3d", `width="36"`, `height="36"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing brand element %q", want)
		}
	}
	if strings.Contains(html, "border-top:4px") {
		t.Fatal("colored top border must not render")
	}
	withoutOrigin := emailhtml.Render(emailhtml.Content{Brand: "Bidora", Title: "Sign in", Origin: "http://untrusted.example"})
	if strings.Contains(withoutOrigin, "<img") || !strings.Contains(withoutOrigin, "Bidora") {
		t.Fatal("missing safe wordmark fallback")
	}
}

func TestEmailActionsRequireExplicitHTTPSOrigin(t *testing.T) {
	for _, origin := range []string{"", "http://example.com", "https://user:pass@example.com", "https://example.com/path", "https://example.com?token=secret", "javascript:alert(1)", "//example.com"} {
		if got := emailhtml.QuickAction(origin, "identity.login_code"); got.URL != "" {
			t.Fatal("unsafe origin accepted")
		}
	}
	for key, path := range map[string]string{"identity.login_code": "/auth/login-code", "identity.password_recovery": "/reset-password", "identity.email_verification": "/verify-email", "tenant.invitation": "/login", "platform.role_change": "/account/security"} {
		if got := emailhtml.QuickAction("https://sandbox.bidora.africa", key); got.URL != "https://sandbox.bidora.africa"+path {
			t.Fatalf("incorrect action for %s", key)
		}
	}
}

func TestPasswordRecoveryEmailIncludesLinkCodeAndInstructions(t *testing.T) {
	code := strings.Repeat("a", 43)
	v, err := app.RenderTemplate(app.Template{Key: "identity.password_recovery", Name: "Recovery", Subject: "Reset your password", Body: "Recovery code: {{token}}. Expires: {{expires_at}}.", Enabled: true, Version: 1}, map[string]string{"platform_name": "Bidora", "token": code, "expires_at": "2030-01-01T12:00:00Z", "web_origin": "https://sandbox.bidora.africa"})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{v.Text, v.HTML} {
		for _, want := range []string{code, "https://sandbox.bidora.africa/reset-password", "confirm your new password", "only be used once", "2030"} {
			if !strings.Contains(body, want) {
				t.Fatalf("recovery email missing %q", want)
			}
		}
		if strings.Contains(body, "?token=") || strings.Contains(body, "?code=") {
			t.Fatal("recovery secret must not appear in link")
		}
	}
	if strings.Count(v.HTML, code) != 1 {
		t.Fatal("recovery code should be displayed once")
	}
}

func TestManagedEmailKeepsEditableCopyAndPlainTextAlternative(t *testing.T) {
	v, err := app.RenderTemplate(app.Template{Key: "identity.login_code", Name: "Login", Subject: "{{platform_name}} sign-in", Body: "Use {{code}} before {{expires_at}}.", Enabled: true, Version: 1}, map[string]string{"platform_name": "Bidora", "code": "123456", "expires_at": "2030-01-01T12:00:00Z", "web_origin": "https://sandbox.bidora.africa"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(v.HTML, "123456") != 1 || !strings.Contains(v.Text, "123456") || !strings.Contains(v.Text, "https://sandbox.bidora.africa/auth/login-code") {
		t.Fatal("code presentation or plain alternative lost")
	}
}
