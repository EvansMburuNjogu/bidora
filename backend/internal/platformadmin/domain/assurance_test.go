package domain_test

import (
	"bidora/backend/internal/identity/securitypolicy"
	"bidora/backend/internal/platformadmin/domain"
	"context"
	"testing"
	"time"
)

func TestValidAdministratorSessionIsNotAReverificationTimer(t *testing.T) {
	now := time.Now().UTC()
	if !domain.CurrentAdministratorAssurance("AAL2", "EMAIL_OTP", now.Add(-6*24*time.Hour), now) {
		t.Fatal("ordinary navigation rejected a valid older administrator assurance")
	}
	for _, v := range []struct {
		level, method string
		at            time.Time
	}{{"AAL1", "EMAIL_OTP", now}, {"AAL2", "PRIMARY_ONLY", now}, {"AAL2", "RECOVERY_CODE", now}, {"AAL2", "EMAIL_OTP", time.Time{}}, {"AAL2", "EMAIL_OTP", now.Add(time.Second)}} {
		if domain.CurrentAdministratorAssurance(v.level, v.method, v.at, now) {
			t.Fatal("incomplete, recovery-only or forged future assurance accepted")
		}
	}
}
func TestSensitiveActionWindowUsesValidatedGlobalConfiguration(t *testing.T) {
	now := time.Now().UTC()
	p := securitypolicy.Default()
	p.StepUpSeconds = 300
	ctx, e := securitypolicy.With(context.Background(), p)
	if e != nil {
		t.Fatal(e)
	}
	if !securitypolicy.Recent(ctx, now.Add(-5*time.Minute), now) || securitypolicy.Recent(ctx, now.Add(-5*time.Minute-time.Nanosecond), now) {
		t.Fatal("configured step-up boundary not enforced")
	}
	p.StepUpSeconds = 1200
	ctx, e = securitypolicy.With(context.Background(), p)
	if e != nil || !securitypolicy.Recent(ctx, now.Add(-16*time.Minute), now) {
		t.Fatal("a hardcoded 15-minute limit overrode the configured policy")
	}
	if securitypolicy.Recent(ctx, time.Time{}, now) || securitypolicy.Recent(ctx, now.Add(time.Second), now) {
		t.Fatal("invalid verification timestamp accepted")
	}
}
