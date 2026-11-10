package notification_test

import (
	app "bidora/backend/internal/notification/application"
	"strings"
	"testing"
)

func TestRetiredInvitationSetupHintCannotChangeRecoveryMeaning(t *testing.T) {
	template := app.Template{Key: "identity.password_recovery", Name: "Recovery", Subject: "Reset your Bidora password", Body: "Use this password recovery token: {{token}}. Expires: {{expires_at}}.", Enabled: true, Version: 1}
	values := map[string]string{"token": "001234", "expires_at": "2030-01-01T12:00:00Z", "challenge_ref": "00000000-0000-4000-8000-000000000001", "web_origin": "https://app.example.test", "password_setup": "true"}
	rendered, err := app.RenderTemplate(template, values)
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Subject != "Reset your Bidora password" || !strings.Contains(rendered.Text, "recovery") || !strings.Contains(rendered.Text, "001234") {
		t.Fatal("recovery repurposed as invitation onboarding")
	}
	values["password_setup"] = "false"
	ordinary, err := app.RenderTemplate(template, values)
	if err != nil || ordinary.Subject != "Reset your Bidora password" || !strings.Contains(ordinary.Text, "recovery") {
		t.Fatal("normal recovery copy changed")
	}
}
func TestSeedAdministratorInvitationIsLinkOnly(t *testing.T) {
	token := "AAAAAAAAAAAAAAAAAAAAAA.001234"
	rendered, err := app.RenderTemplate(app.Template{Key: "platform.invitation.issued", Name: "Invitation", Subject: "Your Bidora administrator invitation", Body: "You were invited as {{role}}. Invitation code: {{token}}. Expires: {{expires_at}}.", Enabled: true, Version: 1}, map[string]string{"role": "PLATFORM_OWNER", "token": token, "expires_at": "2030-01-01T12:00:00Z", "web_origin": "https://app.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{rendered.Text, rendered.HTML} {
		withoutLink := strings.ReplaceAll(content, token, "")
		if strings.Contains(withoutLink, "001234") || strings.Contains(content, "Enter this code") || strings.Contains(content, "PLATFORM_OWNER") || !strings.Contains(content, "Open invitation") {
			t.Fatal("invitation should use only the link")
		}
	}
}
