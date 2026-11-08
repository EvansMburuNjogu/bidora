package emailhtml

import (
	"encoding/base64"
	"net/url"

	"github.com/google/uuid"
)

// RecoveryAction puts the one-time credential in a fragment, never in server
// request URLs or referrers. The user ID is a selector, not an authority.
func RecoveryAction(origin, userID, code string) Action {
	action := QuickAction(origin, "identity.password_recovery")
	id, err := uuid.Parse(userID)
	decoded, tokenErr := base64.RawURLEncoding.Strict().DecodeString(code)
	if action.URL == "" || err != nil || id == uuid.Nil || tokenErr != nil || len(decoded) != 32 {
		return Action{}
	}
	u, _ := url.Parse(action.URL)
	u.Path = "/recovery_email/" + id.String()
	u.Fragment = "code=" + url.QueryEscape(code)
	return Action{Label: "Reset password", URL: u.String()}
}

func InvitationAction(origin, code string) Action {
	return contentAction(Content{Origin: origin, Code: code, Key: "platform.invitation.issued"})
}

func contentAction(c Content) Action {
	if c.Key == "platform.invitation.issued" {
		action := QuickAction(c.Origin, c.Key)
		decoded, e := base64.RawURLEncoding.Strict().DecodeString(c.Code)
		if action.URL == "" || e != nil || len(decoded) != 32 {
			return Action{}
		}
		u, _ := url.Parse(action.URL)
		u.Path = "/administrator-invitation"
		u.Fragment = "code=" + url.QueryEscape(c.Code)
		return Action{Label: "Accept invitation", URL: u.String()}
	}
	if c.Key == "identity.password_recovery" && c.RecoveryUserID != "" {
		return RecoveryAction(c.Origin, c.RecoveryUserID, c.Code)
	}
	return QuickAction(c.Origin, c.Key)
}
