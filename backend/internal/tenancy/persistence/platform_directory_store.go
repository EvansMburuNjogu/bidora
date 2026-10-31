package persistence

import (
	"context"
	"errors"

	platformapp "bidora/backend/internal/platformadmin/application"
	platformpersistence "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	tenancyapp "bidora/backend/internal/tenancy/application"
	registrysql "bidora/backend/internal/tenancy/persistence/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type PlatformTenantDirectoryStore struct{ runner readOnlyRunner }

func NewPlatformTenantDirectoryStore(runner readOnlyRunner) (*PlatformTenantDirectoryStore, error) {
	if runner == nil {
		return nil, errors.New("platform_tenant_directory_store_invalid")
	}
	return &PlatformTenantDirectoryStore{runner: runner}, nil
}

func (s *PlatformTenantDirectoryStore) WithinPlatformTenantDirectory(ctx context.Context, fn func(context.Context, tenancyapp.PlatformTenantDirectoryTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("platform_tenant_directory_unit_of_work_invalid")
	}
	return s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if db == nil {
			return errors.New("platform_tenant_directory_tx_invalid")
		}
		authority, err := platformpersistence.NewTenantDirectoryAuthorityTx(db)
		if err != nil {
			return err
		}
		tx := &platformTenantDirectoryTx{queries: registrysql.New(db), authority: authority}
		if err = VerifyControlCompatibility(txCtx, tx.queries); err != nil {
			return err
		}
		return fn(txCtx, tx)
	})
}

type platformTenantDirectoryTx struct {
	queries   *registrysql.Queries
	authority *platformpersistence.TenantDirectoryAuthorityTx
}

func (tx *platformTenantDirectoryTx) ResolvePlatformDirectoryAuthority(ctx context.Context, subject tenancyapp.PlatformDirectorySubject) (tenancyapp.PlatformDirectoryAuthority, error) {
	facts, err := tx.authority.Resolve(ctx, platformapp.AuthorityQuery{UserID: platformapp.ID(subject.ActorID), SessionID: platformapp.ID(subject.SessionID), ObservedAt: subject.ObservedAt})
	if err != nil {
		return tenancyapp.PlatformDirectoryAuthority{}, err
	}
	return tenancyapp.PlatformDirectoryAuthority{ContextGeneration: facts.ContextGeneration, CanRead: facts.Grants("platform.tenant.read"), AssuranceLevel: facts.AssuranceLevel, AssuranceMethod: facts.AssuranceMethod, AssuredAt: facts.AssuredAt}, nil
}

func (tx *platformTenantDirectoryTx) ListPlatformTenants(ctx context.Context, filter tenancyapp.PlatformTenantFilter, page tenancyapp.PlatformTenantPage) ([]tenancyapp.PlatformTenant, error) {
	beforeAt, beforeID := platformTenantCursor(page)
	rows, err := tx.queries.ListPlatformTenantDirectory(ctx, registrysql.ListPlatformTenantDirectoryParams{FilterState: filter.State, FilterSearch: filter.Search, FilterCountry: filter.Country, BeforeCreatedAt: beforeAt, BeforeID: beforeID, PageLimit: page.Limit + 1})
	if err != nil {
		return nil, err
	}
	result := make([]tenancyapp.PlatformTenant, len(rows))
	for index, row := range rows {
		result[index], err = platformTenantView(platformTenantRow{
			id: row.ID, tenantType: row.TenantType, state: row.State, name: row.Name, country: row.Country, timezone: row.Timezone, locale: row.Locale,
			assignmentGeneration: row.AssignmentGeneration, version: row.Version, createdAt: row.CreatedAt, updatedAt: row.UpdatedAt, assignmentPresent: row.AssignmentPresent,
			commonVersion: row.CommonSchemaVersion, commonState: row.CommonSchemaState, commonGeneration: row.CommonSchemaGeneration, commonUpdatedAt: row.CommonSchemaUpdatedAt,
			tenantVersion: row.TenantSchemaVersion, tenantState: row.TenantSchemaState, tenantGeneration: row.TenantSchemaGeneration, tenantUpdatedAt: row.TenantSchemaUpdatedAt,
		})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (tx *platformTenantDirectoryTx) GetPlatformTenant(ctx context.Context, tenantID tenancyapp.ID) (tenancyapp.PlatformTenant, error) {
	row, err := tx.queries.GetPlatformTenantDirectoryEntry(ctx, pgtype.UUID{Bytes: [16]byte(tenantID), Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return tenancyapp.PlatformTenant{}, tenancyapp.ErrPlatformTenantNotFound
	}
	if err != nil {
		return tenancyapp.PlatformTenant{}, err
	}
	return platformTenantView(platformTenantRow{
		id: row.ID, tenantType: row.TenantType, state: row.State, name: row.Name, country: row.Country, timezone: row.Timezone, locale: row.Locale,
		assignmentGeneration: row.AssignmentGeneration, version: row.Version, createdAt: row.CreatedAt, updatedAt: row.UpdatedAt, assignmentPresent: row.AssignmentPresent,
		commonVersion: row.CommonSchemaVersion, commonState: row.CommonSchemaState, commonGeneration: row.CommonSchemaGeneration, commonUpdatedAt: row.CommonSchemaUpdatedAt,
		tenantVersion: row.TenantSchemaVersion, tenantState: row.TenantSchemaState, tenantGeneration: row.TenantSchemaGeneration, tenantUpdatedAt: row.TenantSchemaUpdatedAt,
	})
}

type platformTenantRow struct {
	id                              pgtype.UUID
	tenantType, state, name         string
	country, timezone, locale       string
	assignmentGeneration, version   int64
	createdAt, updatedAt            pgtype.Timestamptz
	assignmentPresent               bool
	commonVersion, commonGeneration *int64
	commonState                     *string
	commonUpdatedAt                 pgtype.Timestamptz
	tenantVersion, tenantGeneration *int64
	tenantState                     *string
	tenantUpdatedAt                 pgtype.Timestamptz
}

func platformTenantView(row platformTenantRow) (tenancyapp.PlatformTenant, error) {
	if !row.id.Valid || row.tenantType != "TENDERPRENEUR" || row.name == "" || row.country == "" || row.timezone == "" || row.locale == "" || row.assignmentGeneration < 0 || row.version < 1 || !row.createdAt.Valid || !row.updatedAt.Valid {
		return tenancyapp.PlatformTenant{}, errors.New("platform_tenant_directory_row_invalid")
	}
	common, err := platformTenantSchema(row.commonVersion, row.commonState, row.commonGeneration, row.commonUpdatedAt)
	if err != nil {
		return tenancyapp.PlatformTenant{}, err
	}
	tenant, err := platformTenantSchema(row.tenantVersion, row.tenantState, row.tenantGeneration, row.tenantUpdatedAt)
	if err != nil {
		return tenancyapp.PlatformTenant{}, err
	}
	return tenancyapp.PlatformTenant{ID: tenancyapp.ID(row.id.Bytes), Name: row.name, Type: row.tenantType, State: row.state, Country: row.country, Timezone: row.timezone, Locale: row.locale, AssignmentGeneration: row.assignmentGeneration, Version: row.version, AssignmentPresent: row.assignmentPresent, CommonSchema: common, TenantSchema: tenant, CreatedAt: row.createdAt.Time.UTC(), UpdatedAt: row.updatedAt.Time.UTC()}, nil
}

func platformTenantSchema(version *int64, state *string, generation *int64, updatedAt pgtype.Timestamptz) (*tenancyapp.PlatformTenantSchema, error) {
	missing := version == nil && state == nil && generation == nil && !updatedAt.Valid
	if missing {
		return nil, nil
	}
	if version == nil || state == nil || generation == nil || !updatedAt.Valid || *version < 1 || *generation < 1 {
		return nil, errors.New("platform_tenant_schema_row_invalid")
	}
	return &tenancyapp.PlatformTenantSchema{CurrentVersion: *version, Generation: *generation, State: *state, UpdatedAt: updatedAt.Time.UTC()}, nil
}

func platformTenantCursor(page tenancyapp.PlatformTenantPage) (pgtype.Timestamptz, pgtype.UUID) {
	if page.BeforeCreatedAt == nil || page.BeforeID == nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}
	}
	return pgtype.Timestamptz{Time: page.BeforeCreatedAt.UTC(), Valid: true}, pgtype.UUID{Bytes: [16]byte(*page.BeforeID), Valid: true}
}
