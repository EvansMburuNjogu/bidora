package identity_test

import (
	"bidora/backend/internal/commercial/domain"
	"bidora/backend/internal/identity/assurance"
	"testing"
	"time"
)

func TestEmailAssuranceKeepsOrdinaryAndAdministratorAuthorityDistinct(t *testing.T) {
	for _, tc := range []struct {
		level, method         string
		interactive, platform bool
	}{
		{"AAL1", "EMAIL_OTP", true, false}, {"AAL2", "EMAIL_OTP", true, true},
		{"AAL1", "PRIMARY_ONLY", false, false}, {"AAL2", "PRIMARY_ONLY", false, false},
		{"AAL2", "RECOVERY_CODE", false, false}, {"AAL1", "TOTP", false, false}, {"AAL2", "TOTP", true, true},
	} {
		if assurance.Interactive(tc.level, tc.method) != tc.interactive || assurance.Platform(tc.level+":"+tc.method) != tc.platform {
			t.Fatal("verification provenance boundary drifted")
		}
	}
	at := time.Now().UTC()
	if _, err := domain.AdjustmentAuthorityHash(domain.ID{1}, 1, 1, "PLATFORM_OWNER", "platform.governance.approve", "AAL2:EMAIL_OTP", at); err != nil {
		t.Fatal("verified administrator email proof rejected", err)
	}
	if _, err := domain.AdjustmentAuthorityHash(domain.ID{1}, 1, 1, "PLATFORM_OWNER", "platform.governance.approve", "AAL1:EMAIL_OTP", at); err == nil {
		t.Fatal("ordinary email proof gained administrator authority")
	}
}
