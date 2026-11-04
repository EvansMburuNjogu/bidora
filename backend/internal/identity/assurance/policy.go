// Package assurance defines accepted server-issued verification provenance.
package assurance

// Interactive accepts completed email verification without falsely upgrading
// ordinary email sessions to AAL2. Legacy TOTP sessions remain valid until expiry;
// active runtimes no longer create new authenticator challenges.
func Interactive(level, method string) bool {
	return (method == "EMAIL_OTP" && (level == "AAL1" || level == "AAL2")) || (level == "AAL2" && method == "TOTP")
}

// Platform never accepts an ordinary AAL1 email session as administrator proof.
func Platform(value string) bool { return value == "AAL2:EMAIL_OTP" || value == "AAL2:TOTP" }
