package domain

// AdministratorAssurance is Bidora's administrator sign-in policy, not an
// assertion of NIST certification. EMAIL_OTP is issued at AAL2 only after the
// identity owner verifies a browser-bound, single-use password + mailbox challenge
// for a current Platform Administrator under enabled deployment policy.
// Password-only and ordinary-user email sessions remain AAL1. Financial and
// tenant-specific stronger-action policies are deliberately separate.
func AdministratorAssurance(level, method string) bool {
	return level == "AAL2" && (method == "TOTP" || method == "EMAIL_OTP")
}
