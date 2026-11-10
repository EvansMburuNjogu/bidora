package notification_test

import (
	tenant "bidora/backend/internal/authorization/invitation"
	"bidora/backend/internal/invitationcode"
	app "bidora/backend/internal/notification/application"
	platform "bidora/backend/internal/platformadmin/invitation"
	"strings"
	"testing"
)

func TestInvitationEmailsKeepAuthorityInLinkAndAdministratorCodeHidden(t *testing.T) {
	for _, key := range []string{"platform.invitation.issued", "tenant.invitation"} {
		t.Run(key, func(t *testing.T) {
			var wire string
			if key == "tenant.invitation" {
				tok, e := tenant.Generate()
				if e != nil {
					t.Fatal(e)
				}
				wire = tok.RevealForDelivery()
			} else {
				tok, e := platform.Generate()
				if e != nil {
					t.Fatal(e)
				}
				wire = tok.RevealForDelivery()
			}
			rendered, err := app.RenderTemplate(app.Template{Key: key, Name: "Invite", Subject: "Invitation", Body: "Invited as {{role}}. Code: {{token}}. Expires {{expires_at}}.", Enabled: true, Version: 1}, map[string]string{"role": "PLATFORM_OWNER", "token": wire, "expires_at": "2030-01-01T12:00:00Z", "web_origin": "https://app.example.test"})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(rendered.Text, "#code="+wire) || !strings.Contains(rendered.HTML, "#code="+wire) {
				t.Fatal("secret invitation link missing")
			}
			if key == "platform.invitation.issued" {
				if strings.Contains(rendered.HTML, ">"+invitationcode.Display(wire)+"<") || strings.Contains(rendered.Text, "Code: "+invitationcode.Display(wire)) || strings.Contains(rendered.Text, "PLATFORM_OWNER") {
					t.Fatal("administrator invitation shows an unused code or technical role")
				}
				if !strings.Contains(rendered.Text, "Platform Owner") {
					t.Fatal("missing friendly role")
				}
			} else if !strings.Contains(rendered.Text, "Code: "+invitationcode.Display(wire)) {
				t.Fatal("tenant manual-entry compatibility removed")
			}
		})
	}
}
