// Package worker provides the separately mountable process boundary for the
// control-to-tenant AI entitlement acknowledgement runner. It does not load
// configuration, open dependencies, run migrations or install signal handlers.
package worker

import (
	"context"
	"errors"
	"fmt"

	dispatchapp "bidora/backend/internal/entitlementdispatch/application"
)

type StartupProbe interface {
	ProbeAIEntitlementAcknowledgementDispatch(context.Context) error
}

type Worker struct {
	runner *dispatchapp.Runner
	probe  StartupProbe
}

func New(processor dispatchapp.Processor, observer dispatchapp.Observer, probe StartupProbe) (*Worker, error) {
	if probe == nil {
		return nil, errors.New("ai_entitlement_acknowledgement_worker_invalid")
	}
	runner, err := dispatchapp.NewRunner(processor, dispatchapp.SystemTimerFactory{}, observer)
	if err != nil {
		return nil, err
	}
	return &Worker{runner: runner, probe: probe}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	if w == nil || w.runner == nil || w.probe == nil || ctx == nil {
		return errors.New("ai_entitlement_acknowledgement_worker_invalid")
	}
	if err := w.probe.ProbeAIEntitlementAcknowledgementDispatch(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return &workerError{cause: err}
	}
	return w.runner.Run(ctx)
}

type workerError struct{ cause error }

func (e *workerError) Error() string                 { return "ai_entitlement_acknowledgement_worker_unready" }
func (e *workerError) Unwrap() error                 { return e.cause }
func (e *workerError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
