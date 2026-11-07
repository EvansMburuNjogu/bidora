// Package securitypolicy carries a validated per-request snapshot of global
// security settings. It contains no credentials or mutable process-wide state.
package securitypolicy

import (
	"context"
	"errors"
	"time"
)

type Controls struct {
	FailedLoginAttempts   int32 `json:"failed_login_attempts"`
	FailureWindowSeconds  int32 `json:"failure_window_seconds"`
	LockoutSeconds        int32 `json:"lockout_seconds"`
	MinimumPasswordBytes  int32 `json:"minimum_password_bytes"`
	PasswordResetSeconds  int32 `json:"password_reset_seconds"`
	AllowMultipleSessions bool  `json:"allow_multiple_sessions"`
	AdminIdleSeconds      int32 `json:"admin_idle_seconds"`
	AdminAbsoluteSeconds  int32 `json:"admin_absolute_seconds"`
}
type Policy struct {
	Controls
	IdleSeconds, AbsoluteSeconds int32
}

func DefaultControls() Controls { return Controls{5, 900, 900, 12, 1800, true, 604800, 604800} }
func Default() Policy           { return Policy{DefaultControls(), 1800, 43200} }
func ValidControls(p Controls) bool {
	return p.FailedLoginAttempts >= 1 && p.FailedLoginAttempts <= 5 && p.FailureWindowSeconds >= 60 && p.FailureWindowSeconds <= 3600 && p.LockoutSeconds >= 60 && p.LockoutSeconds <= 3600 && p.MinimumPasswordBytes >= 12 && p.MinimumPasswordBytes <= 128 && p.PasswordResetSeconds >= 300 && p.PasswordResetSeconds <= 1800 && p.AdminIdleSeconds >= 300 && p.AdminIdleSeconds <= 604800 && p.AdminAbsoluteSeconds >= 3600 && p.AdminAbsoluteSeconds <= 604800 && p.AdminAbsoluteSeconds >= p.AdminIdleSeconds && p.FailureWindowSeconds%60 == 0 && p.LockoutSeconds%60 == 0 && p.PasswordResetSeconds%60 == 0 && p.AdminIdleSeconds%60 == 0 && p.AdminAbsoluteSeconds%60 == 0
}
func Valid(p Policy) bool {
	return ValidControls(p.Controls) && p.IdleSeconds >= 300 && p.IdleSeconds <= 1800 && p.AbsoluteSeconds >= 3600 && p.AbsoluteSeconds <= 43200 && p.AbsoluteSeconds >= p.IdleSeconds
}

type key struct{}

func With(ctx context.Context, p Policy) (context.Context, error) {
	if ctx == nil || !Valid(p) {
		return nil, errors.New("security_policy_invalid")
	}
	return context.WithValue(ctx, key{}, p), nil
}

// Defaults are exclusively for explicit legacy/test constructors. Production
// middleware and session adapters load validated current values from storage.
func From(ctx context.Context) Policy {
	if ctx != nil {
		if p, ok := ctx.Value(key{}).(Policy); ok {
			return p
		}
	}
	return Default()
}
func PasswordAllowed(ctx context.Context, password []byte) bool {
	return len(password) >= int(From(ctx).MinimumPasswordBytes) && len(password) <= 128
}
func Expiries(ctx context.Context, at time.Time, administrator bool) (time.Time, time.Time, error) {
	if at.IsZero() {
		return time.Time{}, time.Time{}, errors.New("session_time_invalid")
	}
	p := From(ctx)
	idle, absolute := p.IdleSeconds, p.AbsoluteSeconds
	if administrator {
		idle, absolute = p.AdminIdleSeconds, p.AdminAbsoluteSeconds
	}
	return at.UTC().Add(time.Duration(idle) * time.Second), at.UTC().Add(time.Duration(absolute) * time.Second), nil
}
