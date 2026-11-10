package domain

import "time"

// CurrentAdministratorAssurance is valid only after authoritative session,
// identity/epoch and permission checks. It does not extend session expiry or
// manufacture a fresh verification timestamp for sensitive actions.
func CurrentAdministratorAssurance(level, method string, verifiedAt, at time.Time) bool {
	return AdministratorAssurance(level, method) && !verifiedAt.IsZero() && !at.IsZero() && !verifiedAt.After(at)
}

// AdministratorAssurance is Bidora's administrator sign-in policy, not an
// assertion of NIST certification. EMAIL_OTP is issued at AAL2 only after the
// identity owner verifies a browser-bound, single-use password + mailbox challenge
// for a current Platform Administrator under enabled deployment policy.
// Invitation promotion may classify its fresh existing EMAIL_OTP proof as AAL2
// only with committed administrator identity/role promotion, preserving the original
// verification timestamp. Other ordinary-user and password-only sessions remain AAL1. Financial and
// tenant-specific stronger-action policies are deliberately separate.
func AdministratorAssurance(level, method string) bool {
	return level == "AAL2" && (method == "TOTP" || method == "EMAIL_OTP")
}
