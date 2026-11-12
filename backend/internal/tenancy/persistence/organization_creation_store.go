package persistence

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	"bidora/backend/internal/postgres"
	tenantapp "bidora/backend/internal/tenancy/application"
	registrysql "bidora/backend/internal/tenancy/persistence/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type organizationCreationRunner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}

type OrganizationRegionalGuard func(context.Context, postgres.DBTX, tenantapp.OrganizationCreationCommit) error

type OrganizationCreationStore struct {
	runner        organizationCreationRunner
	regionalGuard OrganizationRegionalGuard
}

func (s *OrganizationCreationStore) EnableRegionalPolicy(guard OrganizationRegionalGuard) error {
	if s == nil || guard == nil {
		return errors.New("organization_regional_policy_invalid")
	}
	s.regionalGuard = guard
	return nil
}

func NewOrganizationCreationStore(runner organizationCreationRunner) (*OrganizationCreationStore, error) {
	if runner == nil {
		return nil, errors.New("organization_creation_store_invalid")
	}
	return &OrganizationCreationStore{runner: runner}, nil
}
func (s *OrganizationCreationStore) WithinOrganizationCreation(ctx context.Context, fn func(context.Context, tenantapp.OrganizationCreationTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("organization_creation_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		return fn(txCtx, &organizationCreationTx{queries: registrysql.New(db), db: db, guard: s.regionalGuard})
	})
}

type organizationCreationTx struct {
	queries *registrysql.Queries
	db      postgres.DBTX
	guard   OrganizationRegionalGuard
}

func (tx *organizationCreationTx) LockCreator(ctx context.Context, userID tenantapp.ID) (tenantapp.OrganizationCreatorFacts, error) {
	row, err := tx.queries.LockOrganizationCreator(ctx, organizationCreationUUID(userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return tenantapp.OrganizationCreatorFacts{}, nil
	}
	if err != nil {
		return tenantapp.OrganizationCreatorFacts{}, err
	}
	return tenantapp.OrganizationCreatorFacts{Verified: row.Verified, Disabled: row.Disabled, Tenderpreneur: row.Tenderpreneur, MFAEnrolled: row.MfaEnrolled}, nil
}
func (tx *organizationCreationTx) LatestOrganizationCreationStatus(ctx context.Context, creatorID tenantapp.ID) (tenantapp.OrganizationCreationStatus, bool, error) {
	row, err := tx.queries.GetLatestOrganizationCreationStatus(ctx, organizationCreationUUID(creatorID))
	if errors.Is(err, pgx.ErrNoRows) {
		return tenantapp.OrganizationCreationStatus{}, false, nil
	}
	if err != nil {
		return tenantapp.OrganizationCreationStatus{}, false, err
	}
	if !row.ID.Valid || !row.TenantID.Valid || !row.CreatedAt.Valid || !row.UpdatedAt.Valid {
		return tenantapp.OrganizationCreationStatus{}, false, errors.New("organization_creation_status_invalid")
	}
	return tenantapp.OrganizationCreationStatus{OperationID: tenantapp.ID(row.ID.Bytes), TenantID: tenantapp.ID(row.TenantID.Bytes), Name: row.Name, State: row.State, JobState: row.JobState, CommercialPath: tenantapp.OrganizationCommercialPath(row.CommercialPath), CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC()}, true, nil
}
func (tx *organizationCreationTx) GetOrganizationCreationCommand(ctx context.Context, creatorID tenantapp.ID, keyHash [32]byte) (tenantapp.OrganizationCreationCommand, bool, error) {
	row, err := tx.queries.GetOrganizationCreationCommand(ctx, registrysql.GetOrganizationCreationCommandParams{CreatorUserID: organizationCreationUUID(creatorID), KeyHash: keyHash[:]})
	if errors.Is(err, pgx.ErrNoRows) {
		return tenantapp.OrganizationCreationCommand{}, false, nil
	}
	if err != nil {
		return tenantapp.OrganizationCreationCommand{}, false, err
	}
	if len(row.RequestHash) != 32 {
		return tenantapp.OrganizationCreationCommand{}, false, errors.New("organization_creation_command_invalid")
	}
	var requestHash [32]byte
	copy(requestHash[:], row.RequestHash)
	return tenantapp.OrganizationCreationCommand{RequestHash: requestHash, OperationID: tenantapp.ID(row.OperationID.Bytes), TenantID: tenantapp.ID(row.TenantID.Bytes), State: row.ResponseState}, true, nil
}
func (tx *organizationCreationTx) CreatorLifetimeMarkerExists(ctx context.Context, creatorID tenantapp.ID) (bool, error) {
	return tx.queries.OrganizationCreatorLifetimeMarkerExists(ctx, organizationCreationUUID(creatorID))
}
func (tx *organizationCreationTx) CreatorOpenOperationExists(ctx context.Context, creatorID tenantapp.ID) (bool, error) {
	return tx.queries.OrganizationCreatorOpenOperationExists(ctx, organizationCreationUUID(creatorID))
}
func (tx *organizationCreationTx) CreateOrganization(ctx context.Context, c tenantapp.OrganizationCreationCommit) error {
	if tx.guard != nil {
		if e := tx.guard(ctx, tx.db, c); e != nil {
			return e
		}
	}
	capabilities, err := authzapp.TenderpreneurRoleCapabilities(authzapp.OrganizationAdmin)
	if err != nil || len(c.GrantIDs) != len(capabilities) || len(capabilities) != 7 {
		return errors.New("organization_creation_grant_bundle_invalid")
	}
	at := pgtype.Timestamptz{Time: c.OccurredAt.UTC(), Valid: true}
	if err = tx.queries.InsertRequestedTenderpreneurTenant(ctx, registrysql.InsertRequestedTenderpreneurTenantParams{ID: organizationCreationUUID(c.TenantID), Name: c.Name, Country: c.Country, Timezone: c.Timezone, Locale: c.Locale, At: at}); err != nil {
		return err
	}
	if err = tx.queries.InsertOrganizationCreatorMembership(ctx, registrysql.InsertOrganizationCreatorMembershipParams{ID: organizationCreationUUID(c.MembershipID), UserID: organizationCreationUUID(c.CreatorUserID), TenantID: organizationCreationUUID(c.TenantID), At: at}); err != nil {
		return err
	}
	for index, capability := range capabilities {
		if err = tx.queries.InsertOrganizationCreatorGrant(ctx, registrysql.InsertOrganizationCreatorGrantParams{ID: organizationCreationUUID(c.GrantIDs[index]), MembershipID: organizationCreationUUID(c.MembershipID), Capability: capability, CreatedBy: organizationCreationUUID(c.CreatorUserID), At: at}); err != nil {
			return err
		}
	}
	if err = tx.queries.InsertOrganizationCreationOperation(ctx, registrysql.InsertOrganizationCreationOperationParams{ID: organizationCreationUUID(c.OperationID), TenantID: organizationCreationUUID(c.TenantID), CreatorUserID: organizationCreationUUID(c.CreatorUserID), CreatorMembershipID: organizationCreationUUID(c.MembershipID), CommercialPath: string(c.CommercialPath), BusinessType: c.BusinessType, PrimaryIndustry: c.PrimaryIndustry, PreferredLanguage: c.PreferredLanguage, Currency: c.Currency, CorrelationID: organizationCreationUUID(c.CorrelationID), At: at}); err != nil {
		return err
	}
	jobState := "PENDING"
	next := at
	if c.CommercialPath == tenantapp.OrganizationPaid {
		jobState = "WAITING_PAYMENT"
		next = pgtype.Timestamptz{}
	}
	if err = tx.queries.InsertOrganizationProvisioningJob(ctx, registrysql.InsertOrganizationProvisioningJobParams{ID: organizationCreationUUID(c.JobID), OperationID: organizationCreationUUID(c.OperationID), TenantID: organizationCreationUUID(c.TenantID), State: jobState, NextAttemptAt: next, At: at}); err != nil {
		return err
	}
	if err = tx.queries.InsertOrganizationCreationCommand(ctx, registrysql.InsertOrganizationCreationCommandParams{ID: organizationCreationUUID(c.CommandID), CreatorUserID: organizationCreationUUID(c.CreatorUserID), KeyHash: c.KeyHash[:], RequestHash: c.RequestHash[:], OperationID: organizationCreationUUID(c.OperationID), TenantID: organizationCreationUUID(c.TenantID), At: at}); err != nil {
		return err
	}
	if err = tx.queries.InsertOrganizationCreationAudit(ctx, registrysql.InsertOrganizationCreationAuditParams{ID: organizationCreationUUID(c.AuditID), TenantID: organizationCreationUUID(c.TenantID), ActorID: organizationCreationUUID(c.CreatorUserID), OperationID: organizationCreationUUID(c.OperationID), At: at, CorrelationID: organizationCreationUUID(c.CorrelationID), CommercialPath: string(c.CommercialPath)}); err != nil {
		return err
	}
	return tx.queries.InsertOrganizationCreationOutbox(ctx, registrysql.InsertOrganizationCreationOutboxParams{ID: organizationCreationUUID(c.OutboxID), TenantID: organizationCreationUUID(c.TenantID), ActorID: organizationCreationUUID(c.CreatorUserID), OperationID: organizationCreationUUID(c.OperationID), CommercialPath: string(c.CommercialPath), At: at})
}
func organizationCreationUUID(id tenantapp.ID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(id), Valid: true}
}
