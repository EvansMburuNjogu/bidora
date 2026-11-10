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

// ActionFor keeps HTML buttons and plain-text action links consistent.
func ActionFor(c Content) Action {
	if (c.Key == "identity.email_verification" || c.Key == "identity.password_recovery") && len(c.Code) == 6 && c.ChallengeRef != "" {
		ref, err := uuid.Parse(c.ChallengeRef)
		action := QuickAction(c.Origin, c.Key)
		if err != nil || ref == uuid.Nil || action.URL == "" {
			return Action{}
		}
		u, _ := url.Parse(action.URL)
		if c.Key == "identity.password_recovery" && c.RecoveryUserID != "" {
			if id, e := uuid.Parse(c.RecoveryUserID); e == nil && id != uuid.Nil {
				u.Path = "/recovery_email/" + id.String()
			}
		}
		q := u.Query()
		q.Set("challenge_ref", ref.String())
		u.RawQuery = q.Encode()
		return Action{Label: action.Label, URL: u.String()}
	}
	if (c.Key == "platform.invitation.issued" || c.Key == "tenant.invitation") && c.Code != "" {
		return InvitationAction(c.Origin, c.Key, c.Code)
	}
	if c.Key == "identity.password_recovery" && c.RecoveryUserID != "" {
		return RecoveryAction(c.Origin, c.RecoveryUserID, c.Code)
	}
	return QuickAction(c.Origin, c.Key)
}
