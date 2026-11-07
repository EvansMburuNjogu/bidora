package httpapi

import (
	"bidora/backend/internal/identity/securitypolicy"
	"encoding/json"
	"errors"
)

// Keep duplicate/unknown/missing members fail-closed at every nesting level.
func decodeSecurityControls(d *json.Decoder, p *securitypolicy.Controls) error {
	t, e := d.Token()
	if e != nil || t != json.Delim('{') {
		return errors.New("invalid_security_policy")
	}
	fields := map[string]*int32{"failed_login_attempts": &p.FailedLoginAttempts, "failure_window_seconds": &p.FailureWindowSeconds, "lockout_seconds": &p.LockoutSeconds, "minimum_password_bytes": &p.MinimumPasswordBytes, "password_reset_seconds": &p.PasswordResetSeconds, "admin_idle_seconds": &p.AdminIdleSeconds, "admin_absolute_seconds": &p.AdminAbsoluteSeconds}
	seen := map[string]bool{}
	for d.More() {
		t, e = d.Token()
		name, ok := t.(string)
		if e != nil || !ok || seen[name] {
			return errors.New("invalid_security_policy")
		}
		seen[name] = true
		if name == "allow_multiple_sessions" {
			var v *bool
			e = d.Decode(&v)
			if e == nil && v != nil {
				p.AllowMultipleSessions = *v
			} else {
				return errors.New("invalid_security_policy")
			}
		} else if dest, ok := fields[name]; ok {
			e = d.Decode(dest)
		} else {
			return errors.New("invalid_security_policy")
		}
		if e != nil {
			return e
		}
	}
	t, e = d.Token()
	if e != nil || t != json.Delim('}') || len(seen) != 8 || !securitypolicy.ValidControls(*p) {
		return errors.New("invalid_security_policy")
	}
	return nil
}
