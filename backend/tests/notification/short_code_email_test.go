package notification_test

import (
	"bidora/backend/internal/identity/verification"
	app "bidora/backend/internal/notification/application"
	"bytes"
	"strings"
	"testing"
)

func TestScopedSixDigitEmailsNeverDeliverInternalAuthority(t *testing.T) {
	ref := "00000000-0000-4000-8000-000000000001"
	for _, purpose := range []string{"verify", "recovery"} {
		t.Run(purpose, func(t *testing.T) {
			key := "identity.email_verification"
			if purpose == "recovery" {
				key = "identity.password_recovery"
			}
			token, err := verification.BoundCode(bytes.Repeat([]byte{8}, 32), purpose, ref, "001234")
			if err != nil {
				t.Fatal(err)
			}
			rendered, err := app.RenderTemplate(app.Template{Key: key, Name: "Test", Subject: "Verify", Body: "Use {{token}} before {{expires_at}}.", Enabled: true, Version: 1}, map[string]string{"token": token.RevealForDelivery(), "expires_at": "2030-01-01T12:00:00Z", "challenge_ref": ref, "user_id": "00000000-0000-4000-8000-000000000002", "web_origin": "https://app.example.test"})
			if err != nil {
				t.Fatal(err)
			}
			for _, body := range []string{rendered.Text, rendered.HTML} {
				if !strings.Contains(body, "001234") || !strings.Contains(body, "challenge_ref="+ref) || strings.Contains(body, token.Authority()) || strings.Contains(body, "?code=") || strings.Contains(body, "?token=") {
					t.Fatal("email code or safe action missing")
				}
			}
		})
	}
}
