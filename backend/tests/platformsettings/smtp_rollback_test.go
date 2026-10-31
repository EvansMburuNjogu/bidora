package platformsettings_test

import (
	"bidora/backend/internal/platformsettings/domain"
	"testing"
	"time"
)

func TestSMTPRollbackRequiresExactLatestPassingTestAndDistinctChecker(t *testing.T) {
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	hash := [32]byte{1}
	version := domain.Version{ID: domain.ID{1}, Kind: domain.SMTP, State: domain.Superseded, Version: 4, LatestTestID: domain.ID{2}, ConfigurationHash: hash}
	proof := domain.TestResult{ID: version.LatestTestID, VersionID: version.ID, Outcome: "PASSED", ApplicabilityHash: domain.TestApplicabilityHash(hash)}
	active, maker := domain.ID{3}, domain.ID{4}
	change, err := domain.NewChange(domain.ID{5}, version, proof, domain.Rollback, active, maker, at)
	if err != nil || change.Action != domain.Rollback || change.State != domain.ChangePending || change.ExpectedActiveVersionID != active {
		t.Fatal("rollback proposal did not bind the current active version", err)
	}
	if change.CanDecide(maker, at.Add(time.Minute)) == nil {
		t.Fatal("maker could approve their own rollback")
	}
	if err = change.CanDecide(domain.ID{6}, at.Add(time.Minute)); err != nil {
		t.Fatal("distinct checker rejected", err)
	}
	if change.CanDecide(domain.ID{6}, change.ExpiresAt) == nil {
		t.Fatal("expired rollback accepted")
	}
	for _, mutate := range []func(*domain.Version, *domain.TestResult){
		func(v *domain.Version, _ *domain.TestResult) { v.State = domain.Draft },
		func(_ *domain.Version, p *domain.TestResult) { p.Outcome = "FAILED" },
		func(_ *domain.Version, p *domain.TestResult) { p.ID = domain.ID{7} },
		func(_ *domain.Version, p *domain.TestResult) { p.ApplicabilityHash = [32]byte{9} },
	} {
		v, p := version, proof
		mutate(&v, &p)
		if _, err = domain.NewChange(domain.ID{5}, v, p, domain.Rollback, active, maker, at); err == nil {
			t.Fatal("ineligible rollback accepted")
		}
	}
}
