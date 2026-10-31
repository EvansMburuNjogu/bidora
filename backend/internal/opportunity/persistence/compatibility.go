// Package persistence contains infrastructure checks around generated SQL. It
// does not select a tenant database or authorize a request.
package persistence

import (
	"context"
	"errors"
	"fmt"

	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
)

const CaptureApplicationGeneration int64 = 1

type compatibilityQuerier interface {
	GetCaptureSchemaCompatibility(context.Context) ([]capturesql.GetCaptureSchemaCompatibilityRow, error)
}

// VerifyCaptureCompatibility fails closed unless both migration sets support
// this binary for reads and writes. It is a startup/lease check, not readiness
// by itself and not permission to route a caller-selected tenant.
func VerifyCaptureCompatibility(ctx context.Context, q compatibilityQuerier) error {
	if ctx == nil || q == nil {
		return errors.New("capture_schema_incompatible")
	}
	rows, err := q.GetCaptureSchemaCompatibility(ctx)
	if err != nil {
		return &compatibilityError{cause: err}
	}
	expected := map[string]int64{"tenant_common": 9, "tenant_tenderpreneur": 34}
	seen := make(map[string]bool, len(expected))
	for _, row := range rows {
		minimum, ok := expected[row.MigrationSet]
		if !ok || seen[row.MigrationSet] || row.SchemaVersion < minimum ||
			CaptureApplicationGeneration < row.MinReaderGeneration || CaptureApplicationGeneration > row.MaxReaderGeneration ||
			CaptureApplicationGeneration < row.MinWriterGeneration || CaptureApplicationGeneration > row.MaxWriterGeneration {
			return &compatibilityError{}
		}
		seen[row.MigrationSet] = true
	}
	if len(seen) != len(expected) {
		return &compatibilityError{}
	}
	return nil
}

type compatibilityError struct{ cause error }

func (e *compatibilityError) Error() string                 { return "capture_schema_incompatible" }
func (e *compatibilityError) Unwrap() error                 { return e.cause }
func (e *compatibilityError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
