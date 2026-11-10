package persistence

import (
	"bidora/backend/internal/identity/securitypolicy"
	pa "bidora/backend/internal/platformadmin/application"
	pd "bidora/backend/internal/platformadmin/domain"
	platformpolicy "bidora/backend/internal/platformadmin/domain"
	owner "bidora/backend/internal/platformadmin/persistence"
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	q "bidora/backend/internal/platformsettings/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"context"
	"encoding/json"
	"errors"
	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

func optionalTime(v *time.Time) pgtype.Timestamptz {
	if v == nil {
		return pgtype.Timestamptz{}
	}
	return ts(*v)
}

type RegionalPolicyStore struct{ runner settingsReadRunner }

func NewRegionalPolicyStore(r settingsReadRunner) (*RegionalPolicyStore, error) {
	if r == nil {
		return nil, app.ErrInformationInvalid
	}
	return &RegionalPolicyStore{r}, nil
}
func (s *RegionalPolicyStore) WithinRegionalPolicy(ctx context.Context, write bool, fn func(context.Context, app.RegionalPolicyTx) error) error {
	run := s.runner.WithinReadOnlyTransaction
	if write {
		r, ok := s.runner.(transactionRunner)
		if !ok {
			return app.ErrInformationUnavailable
		}
		run = r.WithinTransaction
	}
	return run(ctx, func(c context.Context, db postgres.DBTX) error {
		tx, e := NewRegionalPolicyTx(db)
		if e != nil {
			return e
		}
		return fn(c, tx)
	})
}

type RegionalPolicyTx struct {
	q         *q.Queries
	authority *owner.SettingsAuthorityTx
}

func NewRegionalPolicyTx(db postgres.DBTX) (*RegionalPolicyTx, error) {
	authority, e := owner.NewSettingsAuthorityTx(db)
	if e != nil {
		return nil, e
	}
	return &RegionalPolicyTx{q.New(db), authority}, nil
}
func (t *RegionalPolicyTx) AuthorizeRegional(ctx context.Context, s app.RegionalSubject, kind string, write bool, at time.Time) error {
	if write {
		if e := t.authority.LockConfigurationWriter(ctx); e != nil {
			return e
		}
	}
	facts, e := t.authority.Resolve(ctx, pa.AuthorityQuery{UserID: pa.ID(s.ActorID), SessionID: pa.ID(s.SessionID), ObservedAt: at})
	if errors.Is(e, pa.ErrAuthorityNotFound) {
		return app.ErrInformationForbidden
	}
	if e != nil {
		return e
	}
	isOwner := false
	for _, role := range facts.Roles {
		isOwner = isOwner || role == pd.PlatformOwner
	}
	allowed := facts.Grants("platform.reference.read")
	if kind == "OPERATIONS" {
		allowed = facts.Grants("platform.maintenance.read") || facts.Grants("platform.retention.policy.read")
	}
	if kind == "METADATA" {
		allowed = facts.Grants("platform.security.event.read") && facts.Grants("platform.audit.read")
	}
	if write {
		allowed = isOwner && facts.Grants("platform.maintenance.manage")
		if kind == "COUNTRY" {
			allowed = facts.Grants("platform.reference.manage")
		}
	}
	if !allowed || facts.ContextGeneration != s.ContextGeneration || !platformpolicy.CurrentAdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod, facts.AssuredAt, at) || write && !securitypolicy.Recent(ctx, facts.AssuredAt, at) {
		return app.ErrInformationForbidden
	}
	return nil
}
func country(v q.PlatformSettingsCountry) domain.CountryConfiguration {
	return domain.CountryConfiguration{Code: v.CountryCode, Currency: v.Currency, Language: v.Language, Timezone: v.Timezone, Locale: v.Locale, Enabled: v.Enabled, Default: v.IsDefault, Version: v.Version, UpdatedAt: v.UpdatedAt.Time.UTC()}.WithName()
}
func (t *RegionalPolicyTx) Countries(ctx context.Context) (out []domain.CountryConfiguration, err error) {
	rows, e := t.q.ListConfiguredCountries(ctx)
	if e != nil {
		return nil, e
	}
	for _, r := range rows {
		out = append(out, country(r))
	}
	return out, nil
}
func (t *RegionalPolicyTx) Country(ctx context.Context, code string, fence bool) (domain.CountryConfiguration, error) {
	var r q.PlatformSettingsCountry
	var e error
	if fence {
		r, e = t.q.FenceCountryConfiguration(ctx, code)
	} else {
		r, e = t.q.ReadCountryConfiguration(ctx, code)
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.CountryConfiguration{}, app.ErrInformationConflict
	}
	return country(r), e
}
func (t *RegionalPolicyTx) SaveCountry(ctx context.Context, c domain.CountryConfiguration, s app.RegionalSubject, at time.Time) error {
	if c.Default {
		if e := t.q.ClearDefaultCountry(ctx, q.ClearDefaultCountryParams{ActorID: uuid(s.ActorID), At: ts(at), CountryCode: c.Code}); e != nil {
			return e
		}
	}
	var count int64
	var e error
	if c.Version == 0 {
		count, e = t.q.SaveCountryConfiguration(ctx, q.SaveCountryConfigurationParams{CountryCode: c.Code, Currency: c.Currency, Language: c.Language, Timezone: c.Timezone, Locale: c.Locale, Enabled: c.Enabled, IsDefault: c.Default, ActorID: uuid(s.ActorID), At: ts(at), ExpectedVersion: 0})
	} else {
		count, e = t.q.UpdateCountryConfiguration(ctx, q.UpdateCountryConfigurationParams{CountryCode: c.Code, Currency: c.Currency, Language: c.Language, Timezone: c.Timezone, Locale: c.Locale, Enabled: c.Enabled, IsDefault: c.Default, ActorID: uuid(s.ActorID), At: ts(at), ExpectedVersion: c.Version})
	}
	if e != nil {
		return e
	}
	if count != 1 {
		return app.ErrInformationConflict
	}
	defaults, e := t.q.CountDefaultCountries(ctx)
	if e != nil {
		return e
	}
	if defaults != 1 {
		return app.ErrInformationConflict
	}
	return nil
}
func (t *RegionalPolicyTx) CountryHistory(ctx context.Context, code string) (out []domain.CountryConfiguration, err error) {
	rows, e := t.q.ReadCountryVersions(ctx, code)
	if e != nil {
		return nil, e
	}
	for _, v := range rows {
		out = append(out, domain.CountryConfiguration{Code: v.CountryCode, Currency: v.Currency, Language: v.Language, Timezone: v.Timezone, Locale: v.Locale, Enabled: v.Enabled, Default: v.IsDefault, Version: v.Version, UpdatedAt: v.UpdatedAt.Time.UTC()}.WithName())
	}
	return out, nil
}
func policy(v q.PlatformSettingsOperationalPolicy) app.OperationalPolicy {
	return app.OperationalPolicy{Version: v.Version, RegistrationEnabled: v.OrganizationRegistrationEnabled, AIEnabled: v.AiFeaturesEnabled, UploadsEnabled: v.DocumentUploadsEnabled, MaxDocumentBytes: v.MaxDocumentBytes, MaintenanceEnabled: v.MaintenanceEnabled, MaintenanceMessage: v.MaintenanceMessage, MaintenanceStartsAt: timePtr(v.MaintenanceStartsAt), MaintenanceEndsAt: timePtr(v.MaintenanceEndsAt), MetadataRetentionDays: v.RestrictedMetadataRetentionDays, UpdatedAt: v.UpdatedAt.Time.UTC()}
}
func (t *RegionalPolicyTx) Policy(ctx context.Context, fence bool) (app.OperationalPolicy, error) {
	var r q.PlatformSettingsOperationalPolicy
	var e error
	if fence {
		r, e = t.q.FenceOperationalPolicy(ctx)
	} else {
		r, e = t.q.ReadOperationalPolicy(ctx)
	}
	return policy(r), e
}
func (t *RegionalPolicyTx) SavePolicy(ctx context.Context, p app.OperationalPolicy, s app.RegionalSubject, at time.Time) error {
	count, e := t.q.SaveOperationalPolicy(ctx, q.SaveOperationalPolicyParams{OrganizationRegistrationEnabled: p.RegistrationEnabled, AiFeaturesEnabled: p.AIEnabled, DocumentUploadsEnabled: p.UploadsEnabled, MaxDocumentBytes: p.MaxDocumentBytes, MaintenanceEnabled: p.MaintenanceEnabled, MaintenanceMessage: p.MaintenanceMessage, MaintenanceStartsAt: optionalTime(p.MaintenanceStartsAt), MaintenanceEndsAt: optionalTime(p.MaintenanceEndsAt), RestrictedMetadataRetentionDays: p.MetadataRetentionDays, ActorID: uuid(s.ActorID), At: ts(at), ExpectedVersion: p.Version})
	if e != nil {
		return e
	}
	if count != 1 {
		return app.ErrInformationConflict
	}
	p.Version++
	p.UpdatedAt = at
	raw, e := json.Marshal(p)
	if e != nil {
		return e
	}
	return t.q.AppendOperationalPolicyVersion(ctx, q.AppendOperationalPolicyVersionParams{Version: p.Version, Value: raw, ActorID: uuid(s.ActorID), At: ts(at)})
}
func (t *RegionalPolicyTx) PolicyHistory(ctx context.Context) (out []app.OperationalPolicy, err error) {
	rows, e := t.q.ReadOperationalPolicyVersions(ctx)
	if e != nil {
		return nil, e
	}
	for _, raw := range rows {
		var p app.OperationalPolicy
		if json.Unmarshal(raw, &p) != nil || !p.Valid() {
			return nil, app.ErrInformationUnavailable
		}
		out = append(out, p)
	}
	return out, nil
}
func (t *RegionalPolicyTx) Command(ctx context.Context, s app.RegionalSubject, kind string, key [32]byte) (app.RegionalReceipt, bool, error) {
	v, e := t.q.ReadRegionalPolicyCommand(ctx, q.ReadRegionalPolicyCommandParams{ActorID: uuid(s.ActorID), Kind: kind, KeyHash: key[:]})
	if errors.Is(e, pgx.ErrNoRows) {
		return app.RegionalReceipt{}, false, nil
	}
	if e != nil {
		return app.RegionalReceipt{}, false, e
	}
	return app.RegionalReceipt{Hash: hash32(v.RequestHash), Version: v.ResultVersion, Target: v.Target}, true, nil
}
func (t *RegionalPolicyTx) Evidence(ctx context.Context, s app.RegionalSubject, kind, target string, version int64, key, hash [32]byte, audit, event domain.ID, at time.Time) error {
	if s.CorrelationID.IsZero() {
		return app.ErrInformationInvalid
	}
	if e := t.q.AppendRegionalPolicyCommand(ctx, q.AppendRegionalPolicyCommandParams{ActorID: uuid(s.ActorID), Kind: kind, KeyHash: key[:], RequestHash: hash[:], ResultVersion: version, Target: target, At: ts(at)}); e != nil {
		return e
	}
	id := domain.ID(googleuuid.NewSHA1(googleuuid.NameSpaceURL, []byte("bidora:"+kind+":"+target)))
	action := "CountryConfigurationUpdated"
	if kind == "OPERATIONS" {
		action = "OperationalPolicyUpdated"
	}
	summary, _ := json.Marshal(map[string]any{"kind": kind, "target": target, "version": version, "changes": s.Changes})
	if e := t.q.AppendRegionalPolicyAudit(ctx, q.AppendRegionalPolicyAuditParams{ID: uuid(audit), ActorID: uuid(s.ActorID), Action: action, TargetID: uuid(id), At: ts(at), CorrelationID: uuid(s.CorrelationID), Summary: summary}); e != nil {
		return e
	}
	return t.q.AppendRegionalPolicyOutbox(ctx, q.AppendRegionalPolicyOutboxParams{ID: uuid(event), ActorID: uuid(s.ActorID), Action: action, TargetID: uuid(id), Version: version, At: ts(at), Summary: summary})
}
