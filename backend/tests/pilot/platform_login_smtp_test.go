package pilot_test

import (
	"testing"

	"bidora/backend/internal/pilot"
)

func TestPlatformEmailLoginRequiresManagedSMTPOnly(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values map[string]string
		active bool
		valid  bool
	}{
		{"disabled", map[string]string{}, false, true},
		{"managed", map[string]string{"BIDORA_PLATFORM_EMAIL_OTP": "true"}, true, true},
		{"invalid flag", map[string]string{"BIDORA_PLATFORM_EMAIL_OTP": "yes"}, false, false},
		{"legacy sender disabled", map[string]string{"BIDORA_PLATFORM_LOGIN_SMTP_HOST": "smtp.example.test"}, false, false},
		{"legacy sender enabled", map[string]string{"BIDORA_PLATFORM_EMAIL_OTP": "true", "BIDORA_PLATFORM_LOGIN_SMTP_CREDENTIAL_REF": "old-secret"}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := pilot.LoadPlatformLoginEmailConfig(func(key string) string { return tc.values[key] })
			if (err == nil) != tc.valid || (config != nil) != tc.active {
				t.Fatalf("unexpected platform email configuration: active=%t valid=%t", config != nil, err == nil)
			}
		})
	}
}
