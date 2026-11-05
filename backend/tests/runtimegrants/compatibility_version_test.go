package runtimegrants_test

import (
	"bidora/backend/internal/migrationrunner"
	"bidora/backend/internal/runtimegrants"
	"bidora/backend/internal/tenancy/persistence"
	"testing"
)

func TestControlGrantManifestMatchesRuntimeAndMigrationVersion(t *testing.T) {
	manifest, err := runtimegrants.Load("control")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.CompatibilityVersion != persistence.ControlSchemaVersion {
		t.Fatal("grant manifest and runtime schema versions differ")
	}
	plan, err := migrationrunner.Resolve("control", manifest.CompatibilityVersion)
	if err != nil || plan.Target != manifest.CompatibilityVersion {
		t.Fatal("grant manifest and migration target differ", err)
	}
}
