package securitypolicy

import (
	"context"
	"testing"
	"time"
)

func TestCurrentPolicyIsScopedAndEveryControlHasBounds(t *testing.T) {
	p := Default()
	p.MinimumPasswordBytes = 20
	p.AdminIdleSeconds = 600
	p.AdminAbsoluteSeconds = 3600
	p.IdleSeconds = 300
	p.AbsoluteSeconds = 3600
	p.AllowMultipleSessions = false
	ctx, e := With(context.Background(), p)
	if e != nil {
		t.Fatal(e)
	}
	if PasswordAllowed(ctx, []byte("twelve-bytes!")) || !PasswordAllowed(ctx, []byte("a sufficiently long password")) {
		t.Fatal("new-password minimum not applied")
	}
	if From(context.Background()).MinimumPasswordBytes != 12 || !From(context.Background()).AllowMultipleSessions {
		t.Fatal("policy leaked between requests")
	}
	at := time.Now().UTC()
	for _, admin := range []bool{false, true} {
		idle, absolute, e := Expiries(ctx, at, admin)
		want := 300
		if admin {
			want = 600
		}
		if e != nil || idle.Sub(at) != time.Duration(want)*time.Second || absolute.Sub(at) != time.Hour {
			t.Fatal("configured expiry not used", admin, e)
		}
	}
	for _, mutate := range []func(*Policy){func(p *Policy) { p.FailedLoginAttempts = 6 }, func(p *Policy) { p.LockoutSeconds = 0 }, func(p *Policy) { p.FailureWindowSeconds = 61 }, func(p *Policy) { p.MinimumPasswordBytes = 11 }, func(p *Policy) { p.PasswordResetSeconds = 1801 }, func(p *Policy) { p.AdminAbsoluteSeconds = 3600; p.AdminIdleSeconds = 7200 }, func(p *Policy) { p.AdminAbsoluteSeconds = 604860 }} {
		bad := Default()
		mutate(&bad)
		if _, e := With(context.Background(), bad); e == nil {
			t.Fatal("unsafe policy accepted", bad)
		}
	}
}
