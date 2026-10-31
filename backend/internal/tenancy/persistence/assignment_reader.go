package persistence

import (
	"context"
	"errors"

	"bidora/backend/internal/postgres"
	tenancyapp "bidora/backend/internal/tenancy/application"
	registrysql "bidora/backend/internal/tenancy/persistence/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type readOnlyRunner interface {
	WithinReadOnlyTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type AssignmentReader struct{ runner readOnlyRunner }

func NewAssignmentReader(runner readOnlyRunner) (*AssignmentReader, error) {
	if runner == nil {
		return nil, errors.New("tenant_assignment_reader_invalid")
	}
	return &AssignmentReader{runner}, nil
}
func (r *AssignmentReader) FindCaptureAssignment(ctx context.Context, tenantID tenancyapp.ID) (tenancyapp.Assignment, error) {
	if ctx == nil || tenantID.IsZero() {
		return tenancyapp.Assignment{}, tenancyapp.ErrAssignmentUnavailable
	}
	if r == nil || r.runner == nil {
		return tenancyapp.Assignment{}, errors.New("tenant_assignment_reader_invalid")
	}
	var result tenancyapp.Assignment
	err := r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("tenant_assignment_lookup_unavailable")
		}
		queries := registrysql.New(db)
		if err := VerifyControlCompatibility(txCtx, queries); err != nil {
			return err
		}
		row, err := queries.GetActiveTenantDatabaseAssignment(txCtx, pgtype.UUID{Bytes: [16]byte(tenantID), Valid: true})
		if errors.Is(err, pgx.ErrNoRows) {
			return tenancyapp.ErrAssignmentUnavailable
		}
		if err != nil {
			return err
		}
		result = tenancyapp.Assignment{TenantID: tenancyapp.ID(row.TenantID.Bytes), ClusterID: tenancyapp.ID(row.ClusterID.Bytes), Generation: row.AssignmentGeneration, DatabaseName: row.DatabaseName, SecretRef: row.SecretRef, EndpointRef: row.EndpointRef, Region: row.Region}
		return nil
	})
	if err != nil {
		return tenancyapp.Assignment{}, err
	}
	return result, nil
}

func (r *AssignmentReader) FindDocumentCompletionAssignment(ctx context.Context, tenantID tenancyapp.ID) (tenancyapp.Assignment, error) {
	if ctx == nil || tenantID.IsZero() || r == nil || r.runner == nil {
		return tenancyapp.Assignment{}, tenancyapp.ErrAssignmentUnavailable
	}
	var result tenancyapp.Assignment
	err := r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		queries := registrysql.New(db)
		compatibility, err := queries.GetControlRegistryCompatibility(txCtx)
		if err != nil || compatibility.MigrationSet != "control_registry" || compatibility.SchemaVersion != ControlSchemaVersion || ControlApplicationGeneration < compatibility.MinReaderGeneration || ControlApplicationGeneration > compatibility.MaxReaderGeneration {
			return errors.New("document_completion_control_schema_incompatible")
		}
		row, err := queries.GetDocumentCompletionAssignment(txCtx, pgtype.UUID{Bytes: [16]byte(tenantID), Valid: true})
		if errors.Is(err, pgx.ErrNoRows) {
			return tenancyapp.ErrAssignmentUnavailable
		}
		if err != nil {
			return err
		}
		result = tenancyapp.Assignment{TenantID: tenancyapp.ID(row.TenantID.Bytes), ClusterID: tenancyapp.ID(row.ClusterID.Bytes), Generation: row.AssignmentGeneration, DatabaseName: row.DatabaseName, SecretRef: row.SecretRef, EndpointRef: row.EndpointRef, Region: row.Region}
		return nil
	})
	if err != nil {
		return tenancyapp.Assignment{}, err
	}
	return result, nil
}

func (r *AssignmentReader) ObserveAIEntitlementAcknowledgementAssignment(ctx context.Context, tenantID tenancyapp.ID) (tenancyapp.AIEntitlementAcknowledgementAssignmentObservation, error) {
	if ctx == nil || tenantID.IsZero() || r == nil || r.runner == nil {
		return tenancyapp.AIEntitlementAcknowledgementAssignmentObservation{}, tenancyapp.ErrAssignmentUnavailable
	}
	var result tenancyapp.AIEntitlementAcknowledgementAssignmentObservation
	err := r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("ai_entitlement_acknowledgement_assignment_lookup_unavailable")
		}
		queries := registrysql.New(db)
		compatibility, err := queries.GetControlRegistryCompatibility(txCtx)
		if err != nil || compatibility.MigrationSet != "control_registry" || compatibility.SchemaVersion != ControlSchemaVersion || ControlApplicationGeneration < compatibility.MinReaderGeneration || ControlApplicationGeneration > compatibility.MaxReaderGeneration {
			return errors.New("ai_entitlement_acknowledgement_control_schema_incompatible")
		}
		row, err := queries.ObserveAIEntitlementAcknowledgementAssignment(txCtx, pgtype.UUID{Bytes: [16]byte(tenantID), Valid: true})
		if errors.Is(err, pgx.ErrNoRows) {
			return tenancyapp.ErrAssignmentUnavailable
		}
		if err != nil {
			return err
		}
		result = tenancyapp.AIEntitlementAcknowledgementAssignmentObservation{TenantID: tenancyapp.ID(row.TenantID.Bytes), CurrentGeneration: row.AssignmentGeneration, DispatchReady: row.DispatchReady}
		if row.DispatchReady && row.ClusterID.Valid && row.DatabaseName != nil && row.SecretRef != nil && row.EndpointRef != nil && row.Region != nil {
			result.Assignment = tenancyapp.Assignment{TenantID: result.TenantID, ClusterID: tenancyapp.ID(row.ClusterID.Bytes), Generation: row.AssignmentGeneration, DatabaseName: *row.DatabaseName, SecretRef: *row.SecretRef, EndpointRef: *row.EndpointRef, Region: *row.Region}
		}
		return nil
	})
	if err != nil {
		return tenancyapp.AIEntitlementAcknowledgementAssignmentObservation{}, err
	}
	return result, nil
}
