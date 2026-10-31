package pilot_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"bidora/backend/internal/pilot"
)

func TestPilotRuntimeRejectsIncompleteComposition(t *testing.T) {
	runtime, err := pilot.Build(pilot.Config{}, pilot.Dependencies{})
	if err == nil || runtime != nil {
		t.Fatal("incomplete pilot composition accepted")
	}
	if err := ((*pilot.Runtime)(nil)).Close(); err != nil {
		t.Fatal(err)
	}
	if err := (&pilot.Runtime{}).Close(); err != nil {
		t.Fatal(err)
	}
	if err := ((*pilot.Runtime)(nil)).Probe(context.Background()); err == nil || strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatal("invalid runtime probe did not fail safely", err)
	}
	if err := ((*pilot.Deployment)(nil)).Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := ((*pilot.Deployment)(nil)).Shutdown(nil); err == nil {
		t.Fatal("nil shutdown context accepted")
	}
	if err := pilot.Serve(context.Background(), nil, nil); err == nil {
		t.Fatal("invalid pilot server accepted")
	}
	if err := pilot.Run(nil, pilot.DeploymentConfig{}); err == nil {
		t.Fatal("nil pilot run context accepted")
	}
}
func TestPilotConfigFormattingRedactsSecurityValues(t *testing.T) {
	config := pilot.Config{DummyPasswordPHC: "dummy-sensitive", AttemptHMACSecret: []byte("hmac-sensitive"), RedisPrefix: "prefix-sensitive", AllowedOrigins: []string{"https://sensitive.example"}}
	formatted := fmt.Sprintf("%+v", config)
	for _, secret := range []string{"dummy-sensitive", "hmac-sensitive", "prefix-sensitive", "sensitive.example"} {
		if strings.Contains(formatted, secret) {
			t.Fatal("pilot config formatting leaked value")
		}
	}
}
