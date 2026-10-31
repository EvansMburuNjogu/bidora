package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"bidora/backend/internal/pilot"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	config, err := pilot.LoadDeploymentConfig(os.Getenv, os.ReadFile)
	if err == nil {
		err = pilot.Run(ctx, config)
	}
	if err != nil {
		log.Printf("pilot process stopped: %s", err)
		os.Exit(1)
	}
}
