package application

import (
	"context"
	"errors"
	"time"
)

const (
	AcknowledgementClaimLease      = 2 * time.Minute
	AcknowledgementAttemptDeadline = 90 * time.Second
	AcknowledgementShutdownUnwind  = 10 * time.Second
	AcknowledgementIdlePoll        = time.Second
	AcknowledgementProcessedBurst  = 25
	AcknowledgementProcessedYield  = 100 * time.Millisecond
)

var acknowledgementGlobalFailureBackoff = [...]time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second}

var ErrRunnerInvalid = errors.New("ai_entitlement_acknowledgement_runner_invalid")
var ErrRunnerAttemptTimedOut = errors.New("ai_entitlement_acknowledgement_attempt_timed_out")
var ErrRunnerUnwindIncomplete = errors.New("ai_entitlement_acknowledgement_unwind_incomplete")

type Processor interface {
	ProcessNext(context.Context, ID) (Result, error)
}

type Observation struct {
	Signal string
	Result string
}

type Observer interface {
	Observe(Observation)
}

type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

type TimerFactory interface {
	NewTimer(time.Duration) Timer
}

type SystemTimerFactory struct{}

type systemTimer struct{ timer *time.Timer }

func (SystemTimerFactory) NewTimer(duration time.Duration) Timer {
	return &systemTimer{timer: time.NewTimer(duration)}
}
func (t *systemTimer) C() <-chan time.Time { return t.timer.C }
func (t *systemTimer) Stop() bool          { return t.timer.Stop() }

type Runner struct {
	processor Processor
	timers    TimerFactory
	observer  Observer
}

func NewRunner(processor Processor, timers TimerFactory, observer Observer) (*Runner, error) {
	if processor == nil || timers == nil || observer == nil {
		return nil, ErrRunnerInvalid
	}
	return &Runner{processor: processor, timers: timers, observer: observer}, nil
}

func (r *Runner) Run(ctx context.Context) error {
	if r == nil || r.processor == nil || r.timers == nil || r.observer == nil || ctx == nil {
		return ErrRunnerInvalid
	}
	if ctx.Err() != nil {
		r.observer.Observe(Observation{Signal: "LOOP", Result: "STOPPED"})
		return nil
	}
	var previousTenant ID
	processedBurst, globalFailures := 0, 0
	for {
		result, attemptErr, stopped, fatalErr := r.runAttempt(ctx, previousTenant)
		if fatalErr != nil {
			r.observer.Observe(Observation{Signal: "LOOP", Result: "FAILED"})
			return fatalErr
		}
		if stopped || ctx.Err() != nil {
			r.observer.Observe(Observation{Signal: "LOOP", Result: "STOPPED"})
			return nil
		}
		if result.Processed {
			if result.TenantID.IsZero() {
				return ErrRunnerInvalid
			}
			previousTenant = result.TenantID
			processedBurst++
			globalFailures = 0
			r.observer.Observe(Observation{Signal: "ATTEMPT", Result: "PROCESSED"})
			if processedBurst >= AcknowledgementProcessedBurst {
				processedBurst = 0
				if err := r.wait(ctx, AcknowledgementProcessedYield); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					return err
				}
			}
			continue
		}
		processedBurst = 0
		if attemptErr == nil {
			globalFailures = 0
			if err := r.wait(ctx, AcknowledgementIdlePoll); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
			continue
		}
		delayIndex := globalFailures
		if delayIndex >= len(acknowledgementGlobalFailureBackoff) {
			delayIndex = len(acknowledgementGlobalFailureBackoff) - 1
		} else {
			globalFailures++
		}
		if err := r.wait(ctx, acknowledgementGlobalFailureBackoff[delayIndex]); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
	}
}

type attemptResponse struct {
	result Result
	err    error
}

func (r *Runner) runAttempt(ctx context.Context, previousTenant ID) (Result, error, bool, error) {
	attemptCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	responses := make(chan attemptResponse, 1)
	go func() {
		result, err := r.processor.ProcessNext(attemptCtx, previousTenant)
		responses <- attemptResponse{result: result, err: err}
	}()
	deadline := r.timers.NewTimer(AcknowledgementAttemptDeadline)
	if deadline == nil {
		return Result{}, nil, false, ErrRunnerInvalid
	}
	deadlineC := deadline.C()
	if deadlineC == nil {
		deadline.Stop()
		return Result{}, nil, false, ErrRunnerInvalid
	}
	defer deadline.Stop()
	select {
	case response := <-responses:
		return response.result, response.err, false, nil
	case <-ctx.Done():
		cancel()
		return r.awaitUnwind(ctx, responses, false)
	case <-deadlineC:
		r.observer.Observe(Observation{Signal: "ATTEMPT", Result: "TIMED_OUT"})
		cancel()
		return r.awaitUnwind(ctx, responses, true)
	}
}

func (r *Runner) awaitUnwind(ctx context.Context, responses <-chan attemptResponse, timedOut bool) (Result, error, bool, error) {
	unwind := r.timers.NewTimer(AcknowledgementShutdownUnwind)
	if unwind == nil {
		return Result{}, nil, false, ErrRunnerInvalid
	}
	unwindC := unwind.C()
	if unwindC == nil {
		unwind.Stop()
		return Result{}, nil, false, ErrRunnerInvalid
	}
	defer unwind.Stop()
	select {
	case response := <-responses:
		if ctx.Err() != nil {
			return Result{}, nil, true, nil
		}
		if timedOut && response.err == nil {
			response.err = ErrRunnerAttemptTimedOut
		}
		return response.result, response.err, false, nil
	case <-unwindC:
		r.observer.Observe(Observation{Signal: "LOOP", Result: "UNWIND_INCOMPLETE"})
		return Result{}, nil, false, ErrRunnerUnwindIncomplete
	}
}

func (r *Runner) wait(ctx context.Context, duration time.Duration) error {
	timer := r.timers.NewTimer(duration)
	if timer == nil {
		return ErrRunnerInvalid
	}
	timerC := timer.C()
	if timerC == nil {
		timer.Stop()
		return ErrRunnerInvalid
	}
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timerC:
		return nil
	}
}

var _ Processor = (*Dispatcher)(nil)
