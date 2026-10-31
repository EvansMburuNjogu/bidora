package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bidora/backend/internal/config"
	"bidora/backend/internal/health"
	"bidora/backend/internal/httpapi"
	"github.com/gofiber/fiber/v3"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		logger.Error("configuration rejected", "reason", err.Error())
		os.Exit(1)
	}
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		logger.Error("listener unavailable")
		os.Exit(1)
	}
	state := &health.State{}
	app := httpapi.New(state)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	result := make(chan error, 1)
	state.Start() // Explicit health-only profile: no configured external dependencies.
	go func() { result <- app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true}) }()
	logger.Info("health-only service started")
	select {
	case err := <-result:
		state.Drain()
		if err != nil {
			logger.Error("HTTP service stopped unexpectedly")
			os.Exit(1)
		}
	case <-ctx.Done():
		state.Drain()
		logger.Info("draining health-only service")
		if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
			logger.Error("shutdown deadline exceeded")
			os.Exit(1)
		}
		if err := <-result; err != nil {
			logger.Error("HTTP service shutdown failed")
			os.Exit(1)
		}
		logger.Info("health-only service stopped")
	}
}
