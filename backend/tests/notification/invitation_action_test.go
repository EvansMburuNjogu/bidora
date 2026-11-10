package notification_test

import (
	"net/url"
	"strings"
	"testing"

	"bidora/backend/internal/emailhtml"
	app "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/message"
)

func TestInvitationActionsUseFragmentsAndFixedPages(t *testing.T) {
	code := strings.Repeat("A", 43) // Synthetic, canonical 32-byte value.
	for key, path := range map[string]string{"platform.invitation.issued": "/invitations/platform", "tenant.invitation": "/invitations/tenant"} {
		a := emailhtml.InvitationAction("https://sandbox.bidora.africa", key, code)
		u, err := url.Parse(a.URL)
		if err != nil || u.Path != path || u.RawQuery != "" || u.Fragment != "code="+code || a.Label != "Open invitation" {
			t.Fatal("incorrect invitation action")
		}
		if strings.Contains(message.Redact(a.URL), code) {
			t.Fatal("invitation link escaped preview redaction")
		}
		if invalid := emailhtml.InvitationAction("https://sandbox.bidora.africa", key, "invalid"); strings.Contains(invalid.URL, "#") || invalid.URL == "" {
			t.Fatal("invalid/preview tokens must use the manual-entry page")
		}
	}
	if emailhtml.InvitationAction("https://sandbox.bidora.africa", "identity.login_code", code).URL != "" {
		t.Fatal("invitation credential used for another purpose")
	}
}

func TestEmailActionsAllowOnlyExplicitLoopbackHTTPDevelopment(t *testing.T) {
	for _, origin := range []string{"http://127.0.0.1:3000", "http://localhost:3000", "http://[::1]:3000"} {
		if emailhtml.QuickAction(origin, "platform.invitation.issued").URL != origin+"/invitations/platform" {
			t.Fatal("missing local invitation action")
		}
	}
	for _, origin := range []string{"http://example.com", "http://localhost.evil.example", "http://127.0.0.1.evil.example:3000", "http://192.168.1.2:3000", "http://user@localhost:3000", "http://localhost:3000/path", "http://localhost:3000?token=secret"} {
		if emailhtml.QuickAction(origin, "platform.invitation.issued").URL != "" {
			t.Fatal("unsafe development origin accepted")
		}
	}
}

func TestAllManagedEmailsUseMatchingButtonsAndPlainTextActions(t *testing.T) {
	bodies := map[string]string{
		"identity.email_verification": "{{token}} {{expires_at}}", "identity.password_recovery": "{{token}} {{expires_at}}", "identity.login_code": "{{code}} {{expires_at}}",
		"tenant.invitation": "{{role}} {{token}} {{expires_at}}", "platform.invitation.issued": "{{role}} {{token}} {{expires_at}}", "platform.invitation.reminder": "{{role}} {{expires_at}}",
		"platform.session_revocation": "{{occurred_at}}", "platform.identity_state": "{{action}} {{occurred_at}}", "platform.role_change": "{{role}} {{action}} {{occurred_at}}",
	}
	values := app.TemplatePreviewValues()
	values["token"], values["code"], values["web_origin"] = strings.Repeat("A", 43), "123456", "http://127.0.0.1:3000"
	values["user_id"] = "00000000-0000-4000-8000-000000000001"
	for key, body := range bodies {
		t.Run(key, func(t *testing.T) {
			v, err := app.RenderTemplate(app.Template{Key: key, Name: "Test", Subject: "{{platform_name}} notification", Body: body, Enabled: true, Version: 1}, values)
			if err != nil {
				t.Fatal(err)
			}
			code := values["token"]
			if key == "identity.login_code" {
				code = values["code"]
			}
			a := emailhtml.ActionFor(emailhtml.Content{Origin: values["web_origin"], Key: key, Code: code, RecoveryUserID: values["user_id"]})
			if a.URL == "" || !strings.Contains(v.HTML, `href="`+a.URL+`"`) || !strings.Contains(v.Text, a.Label+": "+a.URL) || !strings.Contains(v.HTML, `bgcolor="#0f766e"`) {
				t.Fatal("missing consistent HTML/plain-text action")
			}
		})
	}
}
