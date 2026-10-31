package pilot

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/gofiber/fiber/v3"
)

func Run(ctx context.Context, config DeploymentConfig) error {
	if ctx == nil {
		return errors.New("pilot_run_invalid")
	}
	deployment, err := Open(ctx, config)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", config.Address)
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = deployment.Shutdown(shutdownCtx)
		return &lifecycleError{code: "pilot_listener_unavailable", cause: err}
	}
	return Serve(ctx, deployment, listener)
}
func Serve(ctx context.Context, deployment *Deployment, listener net.Listener) error {
	if ctx == nil || deployment == nil || deployment.Runtime == nil || deployment.Runtime.App == nil || deployment.State == nil || listener == nil {
		return errors.New("pilot_serve_invalid")
	}
	if !deployment.State.Ready() {
		_ = listener.Close()
		return errors.New("pilot_not_ready")
	}
	done := make(chan error, 1)
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	workers := make(chan error, 1)
	go func() { workers <- runDocumentWorkers(workerCtx, deployment.Runtime) }()
	go func() {
		done <- deployment.Runtime.App.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true})
	}()
	select {
	case workerErr := <-workers:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownErr := deployment.Shutdown(shutdownCtx)
		if workerErr != nil {
			return &lifecycleError{code: "pilot_document_worker_failed", cause: workerErr}
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		if ctx.Err() != nil {
			return nil
		}
		return errors.New("pilot_document_worker_stopped")
	case err := <-done:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownErr := deployment.Shutdown(shutdownCtx)
		if err != nil {
			return &lifecycleError{code: "pilot_listener_failed", cause: err}
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		if ctx.Err() == nil {
			return errors.New("pilot_listener_stopped")
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		shutdownErr := deployment.Shutdown(shutdownCtx)
		cancel()
		if shutdownErr != nil {
			return shutdownErr
		}
		select {
		case serveErr := <-done:
			if serveErr != nil {
				return &lifecycleError{code: "pilot_listener_failed", cause: serveErr}
			}
			return nil
		case <-time.After(10 * time.Second):
			return errors.New("pilot_http_shutdown_incomplete")
		}
	}
}

func runDocumentWorkers(ctx context.Context, runtime *Runtime) error {
	if ctx == nil || runtime == nil {
		return errors.New("pilot_processing_unavailable")
	}
	for {
		processed, err := runtime.ProcessNextDocumentJobs(ctx)
		if err != nil {
			return err
		}
		if processed {
			continue
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil
		case <-timer.C:
		}
	}
}
