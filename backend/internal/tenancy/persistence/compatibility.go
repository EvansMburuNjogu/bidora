// Package persistence contains control-registry infrastructure checks. It does
// not authenticate membership or permit caller-selected database routing.
package persistence

import (
	"context"
	"errors"
	"fmt"

	registrysql "bidora/backend/internal/tenancy/persistence/sqlc"
)

const ControlApplicationGeneration int64 = 1

// ControlSchemaVersion is the exact current application schema, including the
// restricted Platform audit projection. Reader/writer generations are checked
// independently; an unknown newer schema is not implicitly accepted.
const ControlSchemaVersion int64 = 54

type compatibilityQuerier interface {
	GetControlRegistryCompatibility(context.Context) (registrysql.GetControlRegistryCompatibilityRow, error)
}

// VerifyControlCompatibility fails closed before registry use when the control
// schema is missing, stale or outside this binary's read/write interval.
func VerifyControlCompatibility(ctx context.Context, q compatibilityQuerier) error {
	if ctx == nil || q == nil {
		return errors.New("control_schema_incompatible")
	}
	row, err := q.GetControlRegistryCompatibility(ctx)
	if err != nil {
		return &compatibilityError{cause: err}
	}
	if row.MigrationSet != "control_registry" || row.SchemaVersion != ControlSchemaVersion ||
		ControlApplicationGeneration < row.MinReaderGeneration || ControlApplicationGeneration > row.MaxReaderGeneration ||
		ControlApplicationGeneration < row.MinWriterGeneration || ControlApplicationGeneration > row.MaxWriterGeneration {
		return &compatibilityError{}
	}
	return nil
}

type compatibilityError struct{ cause error }

func (e *compatibilityError) Error() string                 { return "control_schema_incompatible" }
func (e *compatibilityError) Unwrap() error                 { return e.cause }
func (e *compatibilityError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
