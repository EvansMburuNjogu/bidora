package localbudget_test

import (
	"math"
	"sync"
	"testing"
	"time"

	"bidora/backend/internal/localbudget"
)

func baseline(t *testing.T) (*localbudget.Guard, localbudget.Sample) {
	t.Helper()
	at := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	identity := localbudget.Identity{PID: 123, StartedAt: at.Add(-time.Minute)}
	g, err := localbudget.NewGuard(identity)
	if err != nil {
		t.Fatal(err)
	}
	return g, localbudget.Sample{Identity: identity, At: at, Complete: true, RSSBytes: 1024, StorageBytes: 4096}
}

func expect(t *testing.T, got localbudget.Decision, code localbudget.Code) {
	t.Helper()
	if got.Code != code || got.Stop != (code != localbudget.WithinBudget) {
		t.Fatalf("got %+v; want %s", got, code)
	}
}

func TestGuardThresholdBoundariesAndLatch(t *testing.T) {
	for _, test := range []struct {
		rss, storage uint64
		want         localbudget.Code
	}{
		{1536*1024*1024 - 1, 8*1024*1024*1024 - 1, localbudget.WithinBudget},
		{1536 * 1024 * 1024, 0, localbudget.MemoryThreshold},
		{math.MaxUint64, 0, localbudget.MemoryThreshold},
		{0, 8 * 1024 * 1024 * 1024, localbudget.StorageThreshold},
		{0, math.MaxUint64, localbudget.StorageThreshold},
	} {
		g, s := baseline(t)
		s.RSSBytes, s.StorageBytes = test.rss, test.storage
		expect(t, g.Observe(s.At, s), test.want)
		if test.want != localbudget.WithinBudget {
			s.At = s.At.Add(time.Second)
			s.RSSBytes = 0
			s.StorageBytes = 0
			expect(t, g.Observe(s.At, s), test.want)
			expect(t, g.Check(s.At), test.want)
			expect(t, g.MeasurementFailure(), test.want)
		}
	}
}

func TestGuardRejectsIncompleteWrongOrInvalidSamples(t *testing.T) {
	for _, test := range []struct {
		mutate func(*localbudget.Sample)
		want   localbudget.Code
	}{
		{func(s *localbudget.Sample) { s.Complete = false }, localbudget.MeasurementFailed},
		{func(s *localbudget.Sample) { s.Identity.PID++ }, localbudget.IdentityMismatch},
		{func(s *localbudget.Sample) { s.Identity.StartedAt = s.Identity.StartedAt.Add(time.Second) }, localbudget.IdentityMismatch},
		{func(s *localbudget.Sample) { s.At = time.Time{} }, localbudget.InvalidSample},
		{func(s *localbudget.Sample) { s.At = s.At.Add(time.Second) }, localbudget.InvalidSample},
		{func(s *localbudget.Sample) { s.At = s.Identity.StartedAt.Add(-time.Nanosecond) }, localbudget.InvalidSample},
		{func(s *localbudget.Sample) { s.CPUTime = -1 }, localbudget.InvalidSample},
		{func(s *localbudget.Sample) { s.At = s.At.Add(-2*time.Second - time.Nanosecond) }, localbudget.StaleSample},
	} {
		g, s := baseline(t)
		now := s.At
		test.mutate(&s)
		expect(t, g.Observe(now, s), test.want)
	}
}

func TestGuardFreshnessClockAndCounterRegression(t *testing.T) {
	g, s := baseline(t)
	expect(t, g.Check(s.At), localbudget.MissingSample)
	g, s = baseline(t)
	expect(t, g.Observe(s.At, s), localbudget.WithinBudget)
	expect(t, g.Check(s.At.Add(2*time.Second)), localbudget.WithinBudget)
	expect(t, g.Check(s.At.Add(2*time.Second+time.Nanosecond)), localbudget.StaleSample)
	for _, kind := range []string{"duplicate", "backwards sample", "gap", "counter", "clock"} {
		g, s = baseline(t)
		s.CPUTime = time.Second
		expect(t, g.Observe(s.At, s), localbudget.WithinBudget)
		now := s.At.Add(time.Second)
		want := localbudget.InvalidSample
		switch kind {
		case "duplicate":
		case "backwards sample":
			s.At = s.At.Add(-time.Nanosecond)
		case "gap":
			s.At = s.At.Add(3 * time.Second)
			now = s.At
			want = localbudget.StaleSample
		case "counter":
			s.At = now
			s.CPUTime = 0
		case "clock":
			now = s.At.Add(-time.Nanosecond)
		}
		expect(t, g.Observe(now, s), want)
	}
	g, s = baseline(t)
	expect(t, g.Observe(s.At, s), localbudget.WithinBudget)
	expect(t, g.Check(s.At.Add(-time.Nanosecond)), localbudget.InvalidSample)
}

func TestGuardSustainedCPUAndReset(t *testing.T) {
	for _, delta := range []time.Duration{1800 * time.Millisecond, 1799 * time.Millisecond} {
		g, s := baseline(t)
		expect(t, g.Observe(s.At, s), localbudget.WithinBudget)
		for i := 1; i <= 10; i++ {
			s.At = s.At.Add(time.Second)
			s.CPUTime += delta
			want := localbudget.WithinBudget
			if i == 10 && delta == 1800*time.Millisecond {
				want = localbudget.CPUThreshold
			}
			expect(t, g.Observe(s.At, s), want)
		}
	}
	g, s := baseline(t)
	expect(t, g.Observe(s.At, s), localbudget.WithinBudget)
	for i := 1; i <= 15; i++ {
		s.At = s.At.Add(time.Second)
		if i == 5 {
			s.CPUTime += time.Second
		} else {
			s.CPUTime += 2 * time.Second
		}
		want := localbudget.WithinBudget
		if i == 15 {
			want = localbudget.CPUThreshold
		}
		expect(t, g.Observe(s.At, s), want)
	}
}

func TestGuardCPUCalculationDoesNotOverflowOrRoundUp(t *testing.T) {
	g, s := baseline(t)
	expect(t, g.Observe(s.At, s), localbudget.WithinBudget)
	for i := 1; i <= 10; i++ {
		s.At = s.At.Add(time.Second)
		if i == 1 {
			s.CPUTime = time.Duration(math.MaxInt64 / 2)
		} else {
			s.CPUTime += 1800 * time.Millisecond
		}
		want := localbudget.WithinBudget
		if i == 10 {
			want = localbudget.CPUThreshold
		}
		expect(t, g.Observe(s.At, s), want)
	}
	// 1ns CPU over 1ns elapsed is below 180%, despite integer rounding.
	g, s = baseline(t)
	expect(t, g.Observe(s.At, s), localbudget.WithinBudget)
	s.At = s.At.Add(time.Nanosecond)
	s.CPUTime = time.Nanosecond
	expect(t, g.Observe(s.At, s), localbudget.WithinBudget)
	for i := 1; i <= 9; i++ {
		s.At = s.At.Add(time.Second)
		s.CPUTime += 1800 * time.Millisecond
		expect(t, g.Observe(s.At, s), localbudget.WithinBudget)
	}
}

func TestGuardIdentityAndZeroValue(t *testing.T) {
	for _, identity := range []localbudget.Identity{{}, {PID: -1, StartedAt: time.Now()}, {PID: 1}} {
		if g, err := localbudget.NewGuard(identity); err == nil || g != nil {
			t.Fatal("invalid process identity accepted")
		}
	}
	var zero localbudget.Guard
	_, s := baseline(t)
	expect(t, zero.Observe(s.At, s), localbudget.InvalidSample)
	g, s := baseline(t)
	expect(t, g.Check(time.Time{}), localbudget.InvalidSample)
	g, s = baseline(t)
	expect(t, g.Observe(s.Identity.StartedAt.Add(-time.Second), s), localbudget.InvalidSample)
}

func TestGuardConcurrentChecksAndMeasurementFailure(t *testing.T) {
	g, s := baseline(t)
	expect(t, g.Observe(s.At, s), localbudget.WithinBudget)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				g.Check(s.At)
			}
		}()
	}
	wg.Add(1)
	go func() { defer wg.Done(); g.MeasurementFailure() }()
	wg.Wait()
	expect(t, g.Check(s.At), localbudget.MeasurementFailed)
}
