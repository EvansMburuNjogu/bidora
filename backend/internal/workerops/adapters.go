// Package workerops provides pull-based, low-cardinality operational adapters
// for separately mounted background workers. It does not expose HTTP, register
// collectors, load deployment configuration, or start worker loops.
package workerops

import (
	"context"
	"errors"
	"sync"
	"time"

	dispatchapp "bidora/backend/internal/entitlementdispatch/application"
	opportunityapp "bidora/backend/internal/opportunity/application"
)

type Component string

const (
	DocumentMeasurement      Component = "document_measurement"
	AcknowledgementDispatch  Component = "ai_entitlement_acknowledgement_dispatch"
	CompletionReconciliation Component = "document_completion_reconciliation"
)

type Health string

const (
	HealthStarting  Health = "STARTING"
	HealthHealthy   Health = "HEALTHY"
	HealthDegraded  Health = "DEGRADED"
	HealthUnhealthy Health = "UNHEALTHY"
	HealthStopped   Health = "STOPPED"
	HealthStale     Health = "STALE"
)

var ErrAdapterInvalid = errors.New("worker_operational_adapter_invalid")

type Clock interface{ Now() time.Time }
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

type Runnable interface{ Run(context.Context) error }

type Status struct {
	Component      Component
	Health         Health
	Ready          bool
	Running        bool
	StartedAt      time.Time
	LastObservedAt time.Time
}

type Counters struct {
	ProbeSucceeded      uint64
	ProbeFailed         uint64
	Claims              uint64
	Processed           uint64
	Empty               uint64
	Failed              uint64
	TimedOut            uint64
	Published           uint64
	Deferred            uint64
	Held                uint64
	Acknowledged        uint64
	Retried             uint64
	Unavailable         uint64
	UnwindIncomplete    uint64
	InvalidObservations uint64
}

type Metric struct {
	Name  string
	Value uint64
}

type Snapshot struct {
	Status   Status
	Counters Counters
	Metrics  []Metric
}

type monitor struct {
	mu             sync.RWMutex
	component      Component
	clock          Clock
	staleAfter     time.Duration
	health         Health
	ready          bool
	running        bool
	startedAt      time.Time
	lastObservedAt time.Time
	counters       Counters
}

func newMonitor(component Component, clock Clock, staleAfter time.Duration) (*monitor, error) {
	if clock == nil || staleAfter < time.Second || staleAfter > 24*time.Hour || (component != DocumentMeasurement && component != AcknowledgementDispatch && component != CompletionReconciliation) {
		return nil, ErrAdapterInvalid
	}
	return &monitor{component: component, clock: clock, staleAfter: staleAfter, health: HealthStarting}, nil
}

func (m *monitor) now() (time.Time, bool) {
	if m == nil || m.clock == nil {
		return time.Time{}, false
	}
	now := m.clock.Now().UTC()
	return now, !now.IsZero()
}

func (m *monitor) begin() error {
	now, ok := m.now()
	if !ok {
		return ErrAdapterInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running, m.ready, m.health = true, false, HealthStarting
	m.startedAt, m.lastObservedAt = now, now
	return nil
}

func (m *monitor) finish(err error) {
	now, ok := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running, m.ready = false, false
	if ok {
		m.lastObservedAt = now
	}
	if err == nil {
		m.health = HealthStopped
	} else {
		if m.health != HealthUnhealthy {
			m.counters.Failed++
		}
		m.health = HealthUnhealthy
	}
}

func (m *monitor) probe(ok bool) {
	now, valid := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	if valid {
		m.lastObservedAt = now
	}
	if ok && valid {
		m.counters.ProbeSucceeded++
		m.ready = true
		m.health = HealthHealthy
		return
	}
	m.counters.ProbeFailed++
	m.ready = false
	m.health = HealthUnhealthy
}

func (m *monitor) observe(update func(*Counters), health Health, ready *bool) {
	now, ok := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	if update != nil {
		update(&m.counters)
	}
	if !ok {
		m.counters.InvalidObservations++
		m.ready = false
		m.health = HealthUnhealthy
		return
	}
	m.lastObservedAt = now
	if health != "" {
		m.health = health
	}
	if ready != nil {
		m.ready = *ready
	}
}

func (m *monitor) invalidObservation() {
	m.observe(func(c *Counters) { c.InvalidObservations++ }, HealthDegraded, nil)
}

func (m *monitor) snapshot() Snapshot {
	now, nowOK := m.now()
	m.mu.RLock()
	status := Status{Component: m.component, Health: m.health, Ready: m.ready, Running: m.running, StartedAt: m.startedAt, LastObservedAt: m.lastObservedAt}
	counters := m.counters
	staleAfter := m.staleAfter
	m.mu.RUnlock()
	if !nowOK {
		status.Ready = false
		status.Health = HealthUnhealthy
	} else if !status.LastObservedAt.IsZero() && now.Sub(status.LastObservedAt) > staleAfter {
		status.Ready = false
		if status.Health != HealthStopped && status.Health != HealthUnhealthy {
			status.Health = HealthStale
		}
	}
	metrics := []Metric{
		{Name: "ready", Value: boolMetric(status.Ready)}, {Name: "running", Value: boolMetric(status.Running)},
		{Name: "probe_succeeded_total", Value: counters.ProbeSucceeded}, {Name: "probe_failed_total", Value: counters.ProbeFailed},
		{Name: "claims_total", Value: counters.Claims}, {Name: "processed_total", Value: counters.Processed}, {Name: "empty_total", Value: counters.Empty},
		{Name: "failed_total", Value: counters.Failed}, {Name: "timed_out_total", Value: counters.TimedOut}, {Name: "published_total", Value: counters.Published},
		{Name: "deferred_total", Value: counters.Deferred}, {Name: "held_total", Value: counters.Held}, {Name: "acknowledged_total", Value: counters.Acknowledged},
		{Name: "retried_total", Value: counters.Retried}, {Name: "unavailable_total", Value: counters.Unavailable},
		{Name: "unwind_incomplete_total", Value: counters.UnwindIncomplete}, {Name: "invalid_observations_total", Value: counters.InvalidObservations},
	}
	return Snapshot{Status: status, Counters: counters, Metrics: metrics}
}

func boolMetric(value bool) uint64 {
	if value {
		return 1
	}
	return 0
}

func run(ctx context.Context, runnable Runnable, monitor *monitor) error {
	if ctx == nil || runnable == nil || monitor == nil {
		return ErrAdapterInvalid
	}
	if err := monitor.begin(); err != nil {
		return err
	}
	err := runnable.Run(ctx)
	monitor.finish(err)
	return err
}

type AcknowledgementProbe interface{ ProbeAIEntitlementAcknowledgementDispatch(context.Context) error }

type AcknowledgementAdapter struct {
	monitor      *monitor
	probe        AcknowledgementProbe
	probeTimeout time.Duration
}

func NewAcknowledgementAdapter(probe AcknowledgementProbe, clock Clock, probeTimeout, staleAfter time.Duration) (*AcknowledgementAdapter, error) {
	monitor, err := newMonitor(AcknowledgementDispatch, clock, staleAfter)
	if err != nil || probe == nil || !validProbeTimeout(probeTimeout) {
		return nil, ErrAdapterInvalid
	}
	return &AcknowledgementAdapter{monitor: monitor, probe: probe, probeTimeout: probeTimeout}, nil
}
func (a *AcknowledgementAdapter) ProbeAIEntitlementAcknowledgementDispatch(ctx context.Context) error {
	if a == nil || a.monitor == nil || a.probe == nil || ctx == nil {
		return ErrAdapterInvalid
	}
	probeCtx, cancel := context.WithTimeout(ctx, a.probeTimeout)
	defer cancel()
	err := a.probe.ProbeAIEntitlementAcknowledgementDispatch(probeCtx)
	a.monitor.probe(err == nil)
	return err
}
func (a *AcknowledgementAdapter) Observe(value dispatchapp.Observation) {
	if a == nil || a.monitor == nil {
		return
	}
	switch {
	case value.Signal == "CLAIM" && value.Result == "CLAIMED":
		a.monitor.observe(func(c *Counters) { c.Claims++ }, HealthHealthy, nil)
	case value.Signal == "CLAIM" && value.Result == "EMPTY":
		a.monitor.observe(func(c *Counters) { c.Empty++ }, HealthHealthy, nil)
	case value.Signal == "CLAIM" && value.Result == "UNAVAILABLE":
		a.monitor.observe(func(c *Counters) { c.Unavailable++ }, HealthDegraded, nil)
	case value.Signal == "ATTEMPT" && value.Result == "PROCESSED":
		a.monitor.observe(func(c *Counters) { c.Processed++ }, HealthHealthy, nil)
	case value.Signal == "ATTEMPT" && value.Result == "TIMED_OUT":
		a.monitor.observe(func(c *Counters) { c.TimedOut++ }, HealthDegraded, nil)
	case value.Signal == "DISPOSITION" && value.Result == "PUBLISHED":
		a.monitor.observe(func(c *Counters) { c.Published++ }, HealthHealthy, nil)
	case value.Signal == "DISPOSITION" && hasPrefix(value.Result, "DEFERRED_"):
		a.monitor.observe(func(c *Counters) { c.Deferred++ }, HealthDegraded, nil)
	case value.Signal == "DISPOSITION" && hasPrefix(value.Result, "HELD_"):
		a.monitor.observe(func(c *Counters) { c.Held++ }, HealthDegraded, nil)
	case value.Signal == "LOOP" && value.Result == "STOPPED":
		ready := false
		a.monitor.observe(nil, HealthStopped, &ready)
	case value.Signal == "LOOP" && value.Result == "FAILED":
		ready := false
		a.monitor.observe(func(c *Counters) { c.Failed++ }, HealthUnhealthy, &ready)
	case value.Signal == "LOOP" && value.Result == "UNWIND_INCOMPLETE":
		ready := false
		a.monitor.observe(func(c *Counters) { c.UnwindIncomplete++ }, HealthUnhealthy, &ready)
	default:
		a.monitor.invalidObservation()
	}
}
func (a *AcknowledgementAdapter) Run(ctx context.Context, runnable Runnable) error {
	if a == nil {
		return ErrAdapterInvalid
	}
	return run(ctx, runnable, a.monitor)
}
func (a *AcknowledgementAdapter) Snapshot() Snapshot {
	if a == nil || a.monitor == nil {
		return Snapshot{}
	}
	return a.monitor.snapshot()
}

type CompletionProbe interface{ ProbeDocumentCompletionReconciliation(context.Context) error }
type CompletionAdapter struct {
	monitor      *monitor
	probe        CompletionProbe
	probeTimeout time.Duration
}

func NewCompletionAdapter(probe CompletionProbe, clock Clock, probeTimeout, staleAfter time.Duration) (*CompletionAdapter, error) {
	monitor, err := newMonitor(CompletionReconciliation, clock, staleAfter)
	if err != nil || probe == nil || !validProbeTimeout(probeTimeout) {
		return nil, ErrAdapterInvalid
	}
	return &CompletionAdapter{monitor: monitor, probe: probe, probeTimeout: probeTimeout}, nil
}
func (a *CompletionAdapter) ProbeDocumentCompletionReconciliation(ctx context.Context) error {
	if a == nil || a.monitor == nil || a.probe == nil || ctx == nil {
		return ErrAdapterInvalid
	}
	probeCtx, cancel := context.WithTimeout(ctx, a.probeTimeout)
	defer cancel()
	err := a.probe.ProbeDocumentCompletionReconciliation(probeCtx)
	a.monitor.probe(err == nil)
	return err
}
func (a *CompletionAdapter) ObserveDocumentCompletion(value opportunityapp.DocumentCompletionObservation) {
	if a == nil || a.monitor == nil {
		return
	}
	switch {
	case value.Signal == "CLAIM" && value.Result == "EMPTY":
		a.monitor.observe(func(c *Counters) { c.Empty++ }, HealthHealthy, nil)
	case value.Signal == "ENDPOINT" && value.Result == "UNAVAILABLE":
		a.monitor.observe(func(c *Counters) { c.Unavailable++ }, HealthDegraded, nil)
	case value.Signal == "ATTEMPT" && value.Result == "PROCESSED":
		a.monitor.observe(func(c *Counters) { c.Processed++ }, HealthHealthy, nil)
	case value.Signal == "ATTEMPT" && value.Result == "TIMED_OUT":
		a.monitor.observe(func(c *Counters) { c.TimedOut++ }, HealthDegraded, nil)
	case value.Signal == "DISPOSITION" && value.Result == "ACKNOWLEDGED":
		a.monitor.observe(func(c *Counters) { c.Acknowledged++ }, HealthHealthy, nil)
	case value.Signal == "DISPOSITION" && hasPrefix(value.Result, "RETRY_"):
		a.monitor.observe(func(c *Counters) { c.Retried++ }, HealthDegraded, nil)
	case value.Signal == "DISPOSITION" && hasPrefix(value.Result, "HELD_"):
		a.monitor.observe(func(c *Counters) { c.Held++ }, HealthDegraded, nil)
	case value.Signal == "LOOP" && value.Result == "UNWIND_INCOMPLETE":
		ready := false
		a.monitor.observe(func(c *Counters) { c.UnwindIncomplete++ }, HealthUnhealthy, &ready)
	default:
		a.monitor.invalidObservation()
	}
}
func (a *CompletionAdapter) Run(ctx context.Context, runnable Runnable) error {
	if a == nil {
		return ErrAdapterInvalid
	}
	return run(ctx, runnable, a.monitor)
}
func (a *CompletionAdapter) Snapshot() Snapshot {
	if a == nil || a.monitor == nil {
		return Snapshot{}
	}
	return a.monitor.snapshot()
}

type MeasurementProbe interface{ ProbeDocumentMeasurement(context.Context) error }
type MeasurementProcessor interface {
	ProcessNext(context.Context) (bool, error)
}
type MeasurementAdapter struct {
	monitor      *monitor
	probe        MeasurementProbe
	processor    MeasurementProcessor
	probeTimeout time.Duration
}

func NewMeasurementAdapter(processor MeasurementProcessor, probe MeasurementProbe, clock Clock, probeTimeout, staleAfter time.Duration) (*MeasurementAdapter, error) {
	monitor, err := newMonitor(DocumentMeasurement, clock, staleAfter)
	if err != nil || processor == nil || probe == nil || !validProbeTimeout(probeTimeout) {
		return nil, ErrAdapterInvalid
	}
	return &MeasurementAdapter{monitor: monitor, probe: probe, processor: processor, probeTimeout: probeTimeout}, nil
}
func (a *MeasurementAdapter) ProbeDocumentMeasurement(ctx context.Context) error {
	if a == nil || a.monitor == nil || a.probe == nil || ctx == nil {
		return ErrAdapterInvalid
	}
	probeCtx, cancel := context.WithTimeout(ctx, a.probeTimeout)
	defer cancel()
	err := a.probe.ProbeDocumentMeasurement(probeCtx)
	a.monitor.probe(err == nil)
	return err
}
func (a *MeasurementAdapter) ProcessNext(ctx context.Context) (bool, error) {
	if a == nil || a.monitor == nil || a.processor == nil || ctx == nil {
		return false, ErrAdapterInvalid
	}
	processed, err := a.processor.ProcessNext(ctx)
	if err != nil {
		if ctx.Err() == nil {
			a.monitor.observe(func(c *Counters) { c.Failed++ }, HealthDegraded, nil)
		}
		return processed, err
	}
	if processed {
		a.monitor.observe(func(c *Counters) { c.Processed++ }, HealthHealthy, nil)
	} else {
		a.monitor.observe(func(c *Counters) { c.Empty++ }, HealthHealthy, nil)
	}
	return processed, nil
}
func (a *MeasurementAdapter) Run(ctx context.Context, runnable Runnable) error {
	if a == nil {
		return ErrAdapterInvalid
	}
	return run(ctx, runnable, a.monitor)
}
func (a *MeasurementAdapter) Snapshot() Snapshot {
	if a == nil || a.monitor == nil {
		return Snapshot{}
	}
	return a.monitor.snapshot()
}

func validProbeTimeout(value time.Duration) bool {
	return value >= 100*time.Millisecond && value <= 30*time.Second
}
func hasPrefix(value, prefix string) bool {
	return len(value) > len(prefix) && value[:len(prefix)] == prefix
}

var _ dispatchapp.Observer = (*AcknowledgementAdapter)(nil)
var _ opportunityapp.DocumentCompletionObserver = (*CompletionAdapter)(nil)
