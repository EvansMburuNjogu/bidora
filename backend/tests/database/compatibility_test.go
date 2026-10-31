package database_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"bidora/backend/internal/opportunity/persistence"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
)

type compatibilityFake struct {
	rows []capturesql.GetCaptureSchemaCompatibilityRow
	err  error
}

func (f compatibilityFake) GetCaptureSchemaCompatibility(context.Context) ([]capturesql.GetCaptureSchemaCompatibilityRow, error) {
	return f.rows, f.err
}

func compatibleRows() []capturesql.GetCaptureSchemaCompatibilityRow {
	return []capturesql.GetCaptureSchemaCompatibilityRow{
		{MigrationSet: "tenant_common", SchemaVersion: 9, MinReaderGeneration: 1, MaxReaderGeneration: 1, MinWriterGeneration: 1, MaxWriterGeneration: 1},
		{MigrationSet: "tenant_tenderpreneur", SchemaVersion: 34, MinReaderGeneration: 1, MaxReaderGeneration: 1, MinWriterGeneration: 1, MaxWriterGeneration: 1},
	}
}

func TestCaptureSchemaCompatibility(t *testing.T) {
	if persistence.CaptureApplicationGeneration != 1 {
		t.Fatal("review generation change")
	}
	if err := persistence.VerifyCaptureCompatibility(context.Background(), compatibilityFake{rows: compatibleRows()}); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func([]capturesql.GetCaptureSchemaCompatibilityRow) []capturesql.GetCaptureSchemaCompatibilityRow{
		"missing": func(r []capturesql.GetCaptureSchemaCompatibilityRow) []capturesql.GetCaptureSchemaCompatibilityRow {
			return r[:1]
		},
		"duplicate": func(r []capturesql.GetCaptureSchemaCompatibilityRow) []capturesql.GetCaptureSchemaCompatibilityRow {
			return append(r, r[0])
		},
		"unknown": func(r []capturesql.GetCaptureSchemaCompatibilityRow) []capturesql.GetCaptureSchemaCompatibilityRow {
			r[0].MigrationSet = "secret-canary"
			return r
		},
		"old schema": func(r []capturesql.GetCaptureSchemaCompatibilityRow) []capturesql.GetCaptureSchemaCompatibilityRow {
			r[0].SchemaVersion = 1
			return r
		},
		"reader too new": func(r []capturesql.GetCaptureSchemaCompatibilityRow) []capturesql.GetCaptureSchemaCompatibilityRow {
			r[0].MinReaderGeneration = 2
			return r
		},
		"reader expired": func(r []capturesql.GetCaptureSchemaCompatibilityRow) []capturesql.GetCaptureSchemaCompatibilityRow {
			r[0].MaxReaderGeneration = 0
			return r
		},
		"writer too new": func(r []capturesql.GetCaptureSchemaCompatibilityRow) []capturesql.GetCaptureSchemaCompatibilityRow {
			r[0].MinWriterGeneration = 2
			return r
		},
		"writer expired": func(r []capturesql.GetCaptureSchemaCompatibilityRow) []capturesql.GetCaptureSchemaCompatibilityRow {
			r[0].MaxWriterGeneration = 0
			return r
		},
	} {
		t.Run(name, func(t *testing.T) {
			rows := compatibleRows()
			err := persistence.VerifyCaptureCompatibility(context.Background(), compatibilityFake{rows: mutate(rows)})
			if err == nil || err.Error() != "capture_schema_incompatible" || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("compatibility failure not safe", err)
			}
		})
	}
}

func TestCaptureSchemaCompatibilityDependencyFailure(t *testing.T) {
	cause := errors.New("secret-canary database diagnostic")
	err := persistence.VerifyCaptureCompatibility(context.Background(), compatibilityFake{err: cause})
	if err == nil || err.Error() != "capture_schema_incompatible" || !errors.Is(err, cause) || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
		t.Fatal("dependency cause lost or leaked")
	}
	if err := persistence.VerifyCaptureCompatibility(nil, compatibilityFake{}); err == nil {
		t.Fatal("nil context accepted")
	}
	if err := persistence.VerifyCaptureCompatibility(context.Background(), nil); err == nil {
		t.Fatal("nil querier accepted")
	}
}
