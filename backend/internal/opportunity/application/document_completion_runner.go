package application

import (
	"context"
	"errors"
	"time"
)

const (
	DocumentCompletionAttemptDeadline = 90 * time.Second
	DocumentCompletionShutdownUnwind  = 10 * time.Second
	DocumentCompletionIdlePoll        = time.Second
)

var documentCompletionProcessBackoff = [...]time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second}

var ErrDocumentCompletionRunnerInvalid = errors.New("document_completion_runner_invalid")
var ErrDocumentCompletionRunnerUnwind = errors.New("document_completion_runner_unwind_incomplete")

type DocumentCompletionProcessor interface {
	ProcessNext(context.Context) (bool, error)
}
type DocumentCompletionRegistration struct {
	TenantID             ID
	AssignmentGeneration int64
	Processor            DocumentCompletionProcessor
}
type DocumentCompletionObservation struct{ Signal, Result string }
type DocumentCompletionObserver interface {
	ObserveDocumentCompletion(DocumentCompletionObservation)
}
type DocumentCompletionTimer interface {
	C() <-chan time.Time
	Stop() bool
}
type DocumentCompletionTimerFactory interface {
	NewTimer(time.Duration) DocumentCompletionTimer
}
type SystemDocumentCompletionTimerFactory struct{}
type systemDocumentCompletionTimer struct{ timer *time.Timer }

func (SystemDocumentCompletionTimerFactory) NewTimer(d time.Duration) DocumentCompletionTimer {
	return &systemDocumentCompletionTimer{time.NewTimer(d)}
}
func (t *systemDocumentCompletionTimer) C() <-chan time.Time { return t.timer.C }
func (t *systemDocumentCompletionTimer) Stop() bool          { return t.timer.Stop() }

type documentCompletionRuntimeEntry struct {
	registration DocumentCompletionRegistration
	failures     int
	nextEligible time.Time
}
type DocumentCompletionRunner struct {
	entries   []documentCompletionRuntimeEntry
	placement DocumentCompletionPlacement
	clock     CaptureClock
	timers    DocumentCompletionTimerFactory
	observer  DocumentCompletionObserver
}

func NewDocumentCompletionRunner(registrations []DocumentCompletionRegistration, placement DocumentCompletionPlacement, clock CaptureClock, timers DocumentCompletionTimerFactory, observer DocumentCompletionObserver) (*DocumentCompletionRunner, error) {
	if len(registrations) == 0 || placement == nil || clock == nil || timers == nil || observer == nil {
		return nil, ErrDocumentCompletionRunnerInvalid
	}
	entries := make([]documentCompletionRuntimeEntry, len(registrations))
	seen := make(map[ID]struct{}, len(registrations))
	for index, registration := range registrations {
		if registration.TenantID.IsZero() || registration.AssignmentGeneration < 1 || registration.Processor == nil {
			return nil, ErrDocumentCompletionRunnerInvalid
		}
		if _, exists := seen[registration.TenantID]; exists {
			return nil, ErrDocumentCompletionRunnerInvalid
		}
		seen[registration.TenantID] = struct{}{}
		entries[index].registration = registration
	}
	return &DocumentCompletionRunner{entries: entries, placement: placement, clock: clock, timers: timers, observer: observer}, nil
}

func (r *DocumentCompletionRunner) Run(ctx context.Context) error {
	if r == nil || ctx == nil || len(r.entries) == 0 || r.placement == nil || r.clock == nil || r.timers == nil || r.observer == nil {
		return ErrDocumentCompletionRunnerInvalid
	}
	if ctx.Err() != nil {
		return nil
	}
	index, visited, claimedInPass := 0, 0, false
	for {
		if ctx.Err() != nil {
			return nil
		}
		entry := &r.entries[index]
		index = (index + 1) % len(r.entries)
		visited++
		now := r.clock.Now().UTC()
		if now.IsZero() {
			return ErrDocumentCompletionRunnerInvalid
		}
		processed, err := false, error(nil)
		if entry.nextEligible.IsZero() || !now.Before(entry.nextEligible) {
			registration := entry.registration
			if placementErr := r.placement.ValidateCurrentDocumentCompletionAssignment(ctx, registration.TenantID, registration.AssignmentGeneration); placementErr != nil {
				err = placementErr
			} else {
				processed, err = r.runAttempt(ctx, registration.Processor)
			}
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, ErrDocumentCompletionRunnerUnwind) || errors.Is(err, ErrDocumentCompletionRunnerInvalid) {
				return err
			}
			if processed || err == nil {
				entry.failures, entry.nextEligible = 0, time.Time{}
				if !processed {
					r.observer.ObserveDocumentCompletion(DocumentCompletionObservation{Signal: "CLAIM", Result: "EMPTY"})
				}
			} else {
				r.observer.ObserveDocumentCompletion(DocumentCompletionObservation{Signal: "ENDPOINT", Result: "UNAVAILABLE"})
				delayIndex := entry.failures
				if delayIndex >= len(documentCompletionProcessBackoff) {
					delayIndex = len(documentCompletionProcessBackoff) - 1
				} else {
					entry.failures++
				}
				failedAt := r.clock.Now().UTC()
				if failedAt.IsZero() {
					return ErrDocumentCompletionRunnerInvalid
				}
				entry.nextEligible = failedAt.Add(documentCompletionProcessBackoff[delayIndex])
			}
		}
		if processed {
			claimedInPass = true
			r.observer.ObserveDocumentCompletion(DocumentCompletionObservation{Signal: "ATTEMPT", Result: "PROCESSED"})
		}
		if visited >= len(r.entries) {
			visited = 0
			if !claimedInPass {
				if waitErr := r.wait(ctx, DocumentCompletionIdlePoll); waitErr != nil {
					if ctx.Err() != nil {
						return nil
					}
					return waitErr
				}
			}
			claimedInPass = false
		}
	}
}

type documentCompletionRunResult struct {
	processed bool
	err       error
}

func (r *DocumentCompletionRunner) runAttempt(ctx context.Context, processor DocumentCompletionProcessor) (bool, error) {
	attemptCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	responses := make(chan documentCompletionRunResult, 1)
	go func() { p, e := processor.ProcessNext(attemptCtx); responses <- documentCompletionRunResult{p, e} }()
	deadline := r.timers.NewTimer(DocumentCompletionAttemptDeadline)
	if deadline == nil {
		return false, ErrDocumentCompletionRunnerInvalid
	}
	deadlineC := deadline.C()
	if deadlineC == nil {
		deadline.Stop()
		return false, ErrDocumentCompletionRunnerInvalid
	}
	defer deadline.Stop()
	select {
	case response := <-responses:
		return response.processed, response.err
	case <-ctx.Done():
		cancel()
		return r.awaitCompletionUnwind(ctx, responses)
	case <-deadlineC:
		r.observer.ObserveDocumentCompletion(DocumentCompletionObservation{Signal: "ATTEMPT", Result: "TIMED_OUT"})
		cancel()
		return r.awaitCompletionUnwind(ctx, responses)
	}
}
func (r *DocumentCompletionRunner) awaitCompletionUnwind(ctx context.Context, responses <-chan documentCompletionRunResult) (bool, error) {
	timer := r.timers.NewTimer(DocumentCompletionShutdownUnwind)
	if timer == nil {
		return false, ErrDocumentCompletionRunnerInvalid
	}
	timerC := timer.C()
	if timerC == nil {
		timer.Stop()
		return false, ErrDocumentCompletionRunnerInvalid
	}
	defer timer.Stop()
	select {
	case response := <-responses:
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return response.processed, response.err
	case <-timerC:
		r.observer.ObserveDocumentCompletion(DocumentCompletionObservation{Signal: "LOOP", Result: "UNWIND_INCOMPLETE"})
		return false, ErrDocumentCompletionRunnerUnwind
	}
}
func (r *DocumentCompletionRunner) wait(ctx context.Context, d time.Duration) error {
	timer := r.timers.NewTimer(d)
	if timer == nil {
		return ErrDocumentCompletionRunnerInvalid
	}
	timerC := timer.C()
	if timerC == nil {
		timer.Stop()
		return ErrDocumentCompletionRunnerInvalid
	}
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timerC:
		return nil
	}
}
