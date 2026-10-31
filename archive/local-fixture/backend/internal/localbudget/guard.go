// Package localbudget evaluates observations of an owned native fixture.
// It does not sample the OS, impose quotas, or stop a process.
package localbudget

import (
	"errors"
	"sync"
	"time"
)

// Identity must come from the controlled launcher and trusted OS collector.
// PID alone is insufficient because it may be reused.
type Identity struct {
	PID       int
	StartedAt time.Time
}

// Sample contains complete aggregate observations, not PostgreSQL settings.
// CPUTime is cumulative CPU across the tracked process tree, including retained
// accounting for exited descendants. StorageBytes includes all experiment
// artifacts, not only tables. Complete is a collector assertion, not proof.
type Sample struct {
	Identity     Identity
	At           time.Time
	Complete     bool
	RSSBytes     uint64
	StorageBytes uint64
	CPUTime      time.Duration
}

type Code string

const (
	WithinBudget      Code = "within_observed_budget"
	MissingSample     Code = "resource_sample_missing"
	InvalidSample     Code = "resource_sample_invalid"
	StaleSample       Code = "resource_sample_stale"
	IdentityMismatch  Code = "resource_identity_mismatch"
	MeasurementFailed Code = "resource_measurement_failed"
	MemoryThreshold   Code = "resource_memory_threshold"
	StorageThreshold  Code = "resource_storage_threshold"
	CPUThreshold      Code = "resource_cpu_threshold"
)

// Decision is a request to the future supervisor, not evidence of a shutdown.
type Decision struct {
	Stop bool
	Code Code
}

// Guard uses fixed conservative thresholds within the approved experiment
// budget. Stop decisions latch; no sample can reset them. A fresh owned run must
// explicitly construct a new guard. The zero value fails closed. Do not copy.
type Guard struct {
	mu        sync.Mutex
	identity  Identity
	last      Sample
	seen      bool
	lastNow   time.Time
	highSince time.Time
	stopped   Decision
}

const (
	maxSampleAge = 2 * time.Second
	sustainedCPU = 10 * time.Second
	memoryLimit  = uint64(1536 * 1024 * 1024)
	storageLimit = uint64(8 * 1024 * 1024 * 1024)
)

func NewGuard(identity Identity) (*Guard, error) {
	if identity.PID <= 0 || identity.StartedAt.IsZero() {
		return nil, errors.New("resource guard requires an explicit process identity")
	}
	return &Guard{identity: identity}, nil
}

// Observe accepts a complete fresh sample for exactly the expected process.
// The intended collector interval is one second; gaps over two seconds stop.
// RSS >= 1.5 GiB, storage >= 8 GiB, or >=180% CPU sustained for ten seconds
// request stop. CPU compares integer deltas, without floats or counter overflow.
func (g *Guard) Observe(now time.Time, sample Sample) Decision {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopped.Stop {
		return g.stopped
	}
	if !g.clockValid(now) {
		return g.stop(InvalidSample)
	}
	if sample.Identity.PID != g.identity.PID || !sample.Identity.StartedAt.Equal(g.identity.StartedAt) {
		return g.stop(IdentityMismatch)
	}
	if !sample.Complete {
		return g.stop(MeasurementFailed)
	}
	if sample.At.IsZero() || sample.At.After(now) || sample.At.Before(g.identity.StartedAt) || sample.CPUTime < 0 {
		return g.stop(InvalidSample)
	}
	if now.Sub(sample.At) > maxSampleAge {
		return g.stop(StaleSample)
	}
	if sample.RSSBytes >= memoryLimit {
		return g.stop(MemoryThreshold)
	}
	if sample.StorageBytes >= storageLimit {
		return g.stop(StorageThreshold)
	}
	if g.seen {
		elapsed := sample.At.Sub(g.last.At)
		if elapsed <= 0 || sample.CPUTime < g.last.CPUTime {
			return g.stop(InvalidSample)
		}
		if elapsed > maxSampleAge {
			return g.stop(StaleSample)
		}
		delta := sample.CPUTime - g.last.CPUTime
		// elapsed <= two seconds: this multiplication cannot overflow int64.
		// Round the 9/5 ratio up, rather than rounding a sub-threshold delta up.
		threshold := time.Duration((int64(elapsed)*9 + 4) / 5)
		if delta >= threshold {
			if g.highSince.IsZero() {
				g.highSince = g.last.At
			}
			if sample.At.Sub(g.highSince) >= sustainedCPU {
				return g.stop(CPUThreshold)
			}
		} else {
			g.highSince = time.Time{}
		}
	}
	g.last, g.seen, g.lastNow = sample, true, now
	return Decision{Code: WithinBudget}
}

// Check must be called even if collection hangs or yields no new samples. It
// starts no timer itself: the future supervisor owns scheduling and shutdown.
func (g *Guard) Check(now time.Time) Decision {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopped.Stop {
		return g.stopped
	}
	if !g.clockValid(now) {
		return g.stop(InvalidSample)
	}
	if !g.seen {
		return g.stop(MissingSample)
	}
	if now.Sub(g.last.At) > maxSampleAge {
		return g.stop(StaleSample)
	}
	g.lastNow = now
	return Decision{Code: WithinBudget}
}

// MeasurementFailure discards raw collector diagnostics and latches stop.
func (g *Guard) MeasurementFailure() Decision {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopped.Stop {
		return g.stopped
	}
	return g.stop(MeasurementFailed)
}

func (g *Guard) clockValid(now time.Time) bool {
	return g.identity.PID > 0 && !g.identity.StartedAt.IsZero() && !now.IsZero() &&
		!now.Before(g.identity.StartedAt) && (g.lastNow.IsZero() || !now.Before(g.lastNow))
}

func (g *Guard) stop(code Code) Decision {
	g.stopped = Decision{Stop: true, Code: code}
	return g.stopped
}
