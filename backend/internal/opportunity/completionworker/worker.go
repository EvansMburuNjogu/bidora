// Package completionworker provides an unmounted cell-worker boundary for
// document AI Commercial completion reconciliation.
package completionworker

import (
	"context"
	"errors"
	"fmt"

	opportunityapp "bidora/backend/internal/opportunity/application"
)

type StartupProbe interface {
	ProbeDocumentCompletionReconciliation(context.Context) error
}
type Worker struct {
	runner *opportunityapp.DocumentCompletionRunner
	probe  StartupProbe
}

func New(runner *opportunityapp.DocumentCompletionRunner, probe StartupProbe) (*Worker, error) {
	if runner == nil || probe == nil {
		return nil, errors.New("document_completion_worker_invalid")
	}
	return &Worker{runner: runner, probe: probe}, nil
}
func (w *Worker) Run(ctx context.Context) error {
	if w == nil || w.runner == nil || w.probe == nil || ctx == nil {
		return errors.New("document_completion_worker_invalid")
	}
	if err := w.probe.ProbeDocumentCompletionReconciliation(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return &workerError{err}
	}
	return w.runner.Run(ctx)
}

type workerError struct{ cause error }

func (e *workerError) Error() string                 { return "document_completion_worker_unready" }
func (e *workerError) Unwrap() error                 { return e.cause }
func (e *workerError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
