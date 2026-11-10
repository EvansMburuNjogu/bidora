package emailhtml

import (
	"bidora/backend/internal/invitationcode"
	"encoding/base64"
	"net/url"
)

// InvitationRoleLabel renders system role keys as human-readable invitation copy.
func InvitationRoleLabel(role string) string {
	switch role {
	case "PLATFORM_OWNER":
		return "Platform Owner"
	case "OPERATIONS_ADMINISTRATOR":
		return "Operations Administrator"
	case "BILLING_ADMINISTRATOR":
		return "Billing Administrator"
	case "INTEGRATION_ADMINISTRATOR":
		return "Integration Administrator"
	case "SUPPORT_ADMINISTRATOR":
		return "Support Administrator"
	case "SECURITY_AUDIT_ADMINISTRATOR":
		return "Security and Audit Administrator"
	default:
		return role
	}
}

// InvitationAction keeps the credential in the fragment, not HTTP request URLs.
// Opening the destination never accepts an invitation automatically.
func InvitationAction(origin, key, code string) Action {
	if key != "platform.invitation.issued" && key != "tenant.invitation" {
		return Action{}
	}
	action := QuickAction(origin, key)
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(code)
	_, _, short := invitationcode.Split(code)
	if action.URL == "" || (!short && (err != nil || len(decoded) != 32)) {
		return action // Previews and reminders can still open the manual-entry page.
	}
	u, _ := url.Parse(action.URL)
	u.Fragment = "code=" + url.QueryEscape(code)
	return Action{Label: action.Label, URL: u.String()}
}
