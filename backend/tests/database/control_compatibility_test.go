package database_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"bidora/backend/internal/tenancy/persistence"
	registrysql "bidora/backend/internal/tenancy/persistence/sqlc"
)

type controlCompatibilityFake struct {
	row registrysql.GetControlRegistryCompatibilityRow
	err error
}

func (f controlCompatibilityFake) GetControlRegistryCompatibility(context.Context) (registrysql.GetControlRegistryCompatibilityRow, error) {
	return f.row, f.err
}
func compatibleControl() registrysql.GetControlRegistryCompatibilityRow {
	return registrysql.GetControlRegistryCompatibilityRow{MigrationSet: "control_registry", SchemaVersion: 51, MinReaderGeneration: 1, MaxReaderGeneration: 1, MinWriterGeneration: 1, MaxWriterGeneration: 1}
}

func TestControlSchemaCompatibility(t *testing.T) {
	if persistence.ControlApplicationGeneration != 1 {
		t.Fatal("review generation change")
	}
	if err := persistence.VerifyControlCompatibility(context.Background(), controlCompatibilityFake{row: compatibleControl()}); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*registrysql.GetControlRegistryCompatibilityRow){
		"wrong set":            func(r *registrysql.GetControlRegistryCompatibilityRow) { r.MigrationSet = "secret-canary" },
		"old schema":           func(r *registrysql.GetControlRegistryCompatibilityRow) { r.SchemaVersion = 33 },
		"pre-audit schema":     func(r *registrysql.GetControlRegistryCompatibilityRow) { r.SchemaVersion = 35 },
		"pre-managed settings": func(r *registrysql.GetControlRegistryCompatibilityRow) { r.SchemaVersion = 38 },
		"future schema":        func(r *registrysql.GetControlRegistryCompatibilityRow) { r.SchemaVersion = 52 },
		"reader too new":       func(r *registrysql.GetControlRegistryCompatibilityRow) { r.MinReaderGeneration = 2 },
		"reader expired":       func(r *registrysql.GetControlRegistryCompatibilityRow) { r.MaxReaderGeneration = 0 },
		"writer too new":       func(r *registrysql.GetControlRegistryCompatibilityRow) { r.MinWriterGeneration = 2 },
		"writer expired":       func(r *registrysql.GetControlRegistryCompatibilityRow) { r.MaxWriterGeneration = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			row := compatibleControl()
			mutate(&row)
			err := persistence.VerifyControlCompatibility(context.Background(), controlCompatibilityFake{row: row})
			if err == nil || err.Error() != "control_schema_incompatible" || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("unsafe compatibility result", err)
			}
		})
	}
}

func TestControlSchemaCompatibilityDependencyFailure(t *testing.T) {
	cause := errors.New("secret-canary control diagnostic")
	err := persistence.VerifyControlCompatibility(context.Background(), controlCompatibilityFake{err: cause})
	if err == nil || !errors.Is(err, cause) || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
		t.Fatal("cause lost or leaked")
	}
	if persistence.VerifyControlCompatibility(nil, controlCompatibilityFake{}) == nil {
		t.Fatal("nil context accepted")
	}
	if persistence.VerifyControlCompatibility(context.Background(), nil) == nil {
		t.Fatal("nil querier accepted")
	}
}
