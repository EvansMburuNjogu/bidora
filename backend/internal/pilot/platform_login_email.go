package pilot

import (
	"errors"
	"fmt"
)

// PlatformLoginEmailConfig enables the extra Platform email-code gate. Delivery
// uses the same active, encrypted Platform Admin SMTP revision as every other
// identity message; no deployment-specific sender or credential is accepted.
type PlatformLoginEmailConfig struct{}

func (PlatformLoginEmailConfig) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED platform login email configuration]")
}

func LoadPlatformLoginEmailConfig(getenv func(string) string) (*PlatformLoginEmailConfig, error) {
	for _, key := range []string{
		"BIDORA_PLATFORM_LOGIN_SMTP_HOST", "BIDORA_PLATFORM_LOGIN_SMTP_PORT",
		"BIDORA_PLATFORM_LOGIN_SMTP_SENDER", "BIDORA_PLATFORM_LOGIN_SMTP_CREDENTIAL_REF",
		"BIDORA_PLATFORM_LOGIN_SMTP_TRUST_REF",
	} {
		if getenv(key) != "" {
			return nil, errors.New("platform_login_email_config_invalid")
		}
	}
	switch getenv("BIDORA_PLATFORM_EMAIL_OTP") {
	case "", "false":
		return nil, nil
	case "true":
		return &PlatformLoginEmailConfig{}, nil
	default:
		return nil, errors.New("platform_login_email_config_invalid")
	}
}
