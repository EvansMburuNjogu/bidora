package pilot_test

import (
	"bytes"
	"encoding/base64"
	"maps"
	"strings"
	"testing"
)

func TestNotificationEvidenceConfigRequiresExplicitKeyAndActiveSMTPComposition(t *testing.T) {
	values := deploymentFixture(t)
	values["BIDORA_SMTP_VAULT_KEY_FILE"] = "/smtp-key"
	values["BIDORA_SMTP_VAULT_KEY_VERSION"] = "1"
	values["/smtp-key"] = base64.RawStdEncoding.EncodeToString(make([]byte, 32))
	key := bytes.Repeat([]byte{42}, 32)
	values["BIDORA_NOTIFICATION_FINGERPRINT_KEY_FILE"] = "/notification-key"
	values["BIDORA_NOTIFICATION_FINGERPRINT_KEY_VERSION"] = "2"
	values["/notification-key"] = base64.RawStdEncoding.EncodeToString(key)
	config, err := loadFixture(values)
	if err != nil || !bytes.Equal(config.Runtime.NotificationFingerprintKey, key) || config.Runtime.NotificationFingerprintKeyVersion != 2 {
		t.Fatal("notification evidence key not loaded", err)
	}
	for name, mutate := range map[string]func(map[string]string){
		"missing key file":      func(v map[string]string) { delete(v, "BIDORA_NOTIFICATION_FINGERPRINT_KEY_FILE") },
		"empty key file":        func(v map[string]string) { delete(v, "/notification-key") },
		"missing key version":   func(v map[string]string) { delete(v, "BIDORA_NOTIFICATION_FINGERPRINT_KEY_VERSION") },
		"zero key version":      func(v map[string]string) { v["BIDORA_NOTIFICATION_FINGERPRINT_KEY_VERSION"] = "0" },
		"oversized key version": func(v map[string]string) { v["BIDORA_NOTIFICATION_FINGERPRINT_KEY_VERSION"] = "2147483648" },
		"malformed key":         func(v map[string]string) { v["/notification-key"] = "private-key-canary" },
		"wrong key length": func(v map[string]string) {
			v["/notification-key"] = base64.RawStdEncoding.EncodeToString(make([]byte, 31))
		},
		"missing SMTP vault": func(v map[string]string) {
			delete(v, "BIDORA_SMTP_VAULT_KEY_FILE")
			delete(v, "BIDORA_SMTP_VAULT_KEY_VERSION")
		},
	} {
		t.Run(name, func(t *testing.T) {
			v := maps.Clone(values)
			mutate(v)
			_, err := loadFixture(v)
			if err == nil || strings.Contains(err.Error(), "private-key-canary") {
				t.Fatal("invalid notification configuration accepted or leaked", err)
			}
		})
	}
}
