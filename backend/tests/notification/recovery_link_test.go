package notification_test

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"bidora/backend/internal/emailhtml"
	app "bidora/backend/internal/notification/application"
)

func TestRecoveryLinkUsesBoundSelectorAndFragmentOnlySecret(t *testing.T) {
	code := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	id := "019c0000-0000-4000-8000-000000000041"
	action := emailhtml.RecoveryAction("https://app.example.test:8443", id, code)
	u, err := url.Parse(action.URL)
	if err != nil || u.Path != "/recovery_email/"+id || u.RawQuery != "" || u.Fragment != "code="+code || u.Host != "app.example.test:8443" {
		t.Fatal("invalid recovery link")
	}
	for _, args := range [][3]string{{"http://app.example.test", id, code}, {"https://evil.test/path", id, code}, {"https://app.example.test", "invalid", code}, {"https://app.example.test", id, "bad"}} {
		if emailhtml.RecoveryAction(args[0], args[1], args[2]).URL != "" {
			t.Fatal("unsafe action accepted")
		}
	}
	rendered, err := app.RenderTemplate(app.Template{Key: "identity.password_recovery", Name: "Recovery", Subject: "Reset password", Body: "Recovery code: {{token}}. Expires: {{expires_at}}.", Enabled: true, Version: 1}, map[string]string{"platform_name": "Bidora", "web_origin": "https://app.example.test:8443", "user_id": id, "token": code, "expires_at": "2030-01-01T12:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{rendered.HTML, rendered.Text} {
		if !strings.Contains(body, action.URL) || strings.Contains(body, "?token=") || strings.Contains(body, "?code=") {
			t.Fatal("missing safe link")
		}
	}
	// Asset URLs must remain first-party and never inherit a recovery credential.
	if strings.Contains(rendered.HTML, ".woff2#") || strings.Contains(rendered.HTML, ".png#") {
		t.Fatal("credential leaked to asset")
	}
}
