package persistence

import (
	app "bidora/backend/internal/organization/application"
	sql "bidora/backend/internal/organization/persistence/sqlc"
	"bidora/backend/internal/postgres"
	evidence "bidora/backend/internal/tenantevidence/persistence"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

type runner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
	WithinConsistentReadTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type Store struct{ runner runner }

func New(r runner) (*Store, error) {
	if r == nil {
		return nil, errors.New("organization_store_invalid")
	}
	return &Store{r}, nil
}
func (s *Store) SaveProfile(ctx context.Context, c app.SaveProfileCommit) (result app.ProfileResult, err error) {
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := sql.New(db)
		ev, _ := evidence.New(db)
		if replay, ok, e := ev.GetCommand(txCtx, eid(c.Actor.ActorID), "ORGANIZATION_PROFILE_SAVE", c.KeyHash, c.RequestHash); e != nil {
			return commandError(e)
		} else if ok {
			row, e := q.GetCompanyProfile(txCtx)
			if e != nil {
				return e
			}
			profile, e := profile(row)
			if e != nil || profile.ID != app.ID(replay.ResponseRef) || profile.Version != replay.ResponseVersion {
				return app.ErrConflict
			}
			result = app.ProfileResult{Profile: profile, Replay: true}
			return nil
		}
		var version int64
		targetID := c.ProfileID
		if c.ExpectedVersion == 0 {
			if _, e := q.LockCompanyProfile(txCtx); e == nil {
				return app.ErrConflict
			} else if !errors.Is(e, pgx.ErrNoRows) {
				return e
			}
			if e := q.InsertCompanyProfile(txCtx, insertProfile(c)); e != nil {
				return conflict(e)
			}
			version = 1
		} else {
			row, e := q.LockCompanyProfile(txCtx)
			if errors.Is(e, pgx.ErrNoRows) {
				return app.ErrConflict
			}
			if e != nil {
				return e
			}
			if row.Version != c.ExpectedVersion || !row.ID.Valid {
				return app.ErrConflict
			}
			targetID = app.ID(row.ID.Bytes)
			changed, e := q.UpdateCompanyProfile(txCtx, updateProfile(c))
			if e != nil {
				return e
			}
			if changed != 1 {
				return app.ErrConflict
			}
			version = c.ExpectedVersion + 1
		}
		summary := []byte(`{"fields":"organization_profile"}`)
		payload, _ := json.Marshal(struct {
			ProfileID string `json:"profile_id"`
		}{uuidText(targetID)})
		if e := ev.Complete(txCtx, eid(c.CommandID), eid(c.Actor.ActorID), "ORGANIZATION_PROFILE_SAVE", c.KeyHash, c.RequestHash, 200, eid(targetID), version, c.At); e != nil {
			return conflict(e)
		}
		if e := ev.Append(txCtx, evidence.Evidence{AuditID: eid(c.AuditID), OutboxID: eid(c.OutboxID), ActorID: eid(c.Actor.ActorID), TargetID: eid(targetID), CorrelationID: eid(c.CorrelationID), Action: "OrganizationProfileSaved", TargetType: "organization_profile", AggregateType: "organization_profile", EventType: "organization.profile.saved.v1", AggregateVersion: version, Summary: summary, Payload: payload, At: c.At}); e != nil {
			return e
		}
		row, e := q.GetCompanyProfile(txCtx)
		if e != nil {
			return e
		}
		value, e := profile(row)
		result = app.ProfileResult{Profile: value}
		return e
	})
	return result, err
}
func (s *Store) CreateLetterhead(ctx context.Context, c app.CreateLetterheadCommit) (result app.LetterheadResult, err error) {
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := sql.New(db)
		ev, _ := evidence.New(db)
		if replay, ok, e := ev.GetCommand(txCtx, eid(c.Actor.ActorID), "LETTERHEAD_VERSION_CREATE", c.KeyHash, c.RequestHash); e != nil {
			return commandError(e)
		} else if ok {
			row, e := q.GetLetterheadVersion(txCtx, id(app.ID(replay.ResponseRef)))
			if e != nil {
				return e
			}
			v, e := letterhead(row)
			result = app.LetterheadResult{Version: v, ConfigurationVersion: replay.ResponseVersion, Replay: true}
			return e
		}
		config, e := q.LockLetterheadConfiguration(txCtx)
		if e != nil {
			return e
		}
		if config.Version != c.ExpectedConfigurationVersion {
			return app.ErrConflict
		}
		if e = q.InsertLetterheadVersion(txCtx, insertLetterhead(c, config.NextVersionNumber)); e != nil {
			return conflict(e)
		}
		changed, e := q.AdvanceLetterheadSequence(txCtx, sql.AdvanceLetterheadSequenceParams{ActorID: id(c.Actor.ActorID), At: ts(c.At), ExpectedVersion: config.Version, ExpectedNextVersion: config.NextVersionNumber})
		if e != nil {
			return e
		}
		if changed != 1 {
			return app.ErrConflict
		}
		newConfig := config.Version + 1
		if e = ev.Complete(txCtx, eid(c.CommandID), eid(c.Actor.ActorID), "LETTERHEAD_VERSION_CREATE", c.KeyHash, c.RequestHash, 201, eid(c.VersionID), newConfig, c.At); e != nil {
			return conflict(e)
		}
		payload, _ := json.Marshal(struct {
			VersionID string `json:"version_id"`
		}{uuidText(c.VersionID)})
		if e = ev.Append(txCtx, evidence.Evidence{AuditID: eid(c.AuditID), OutboxID: eid(c.OutboxID), ActorID: eid(c.Actor.ActorID), TargetID: eid(c.VersionID), CorrelationID: eid(c.CorrelationID), Action: "LetterheadVersionCreated", TargetType: "letterhead_version", AggregateType: "letterhead_version", EventType: "organization.letterhead.version_created.v1", AggregateVersion: config.NextVersionNumber, Summary: []byte(`{"immutable":true}`), Payload: payload, At: c.At}); e != nil {
			return e
		}
		row, e := q.GetLetterheadVersion(txCtx, id(c.VersionID))
		if e != nil {
			return e
		}
		v, e := letterhead(row)
		result = app.LetterheadResult{Version: v, ConfigurationVersion: newConfig}
		return e
	})
	return result, err
}
func (s *Store) ActivateLetterhead(ctx context.Context, c app.ActivateLetterheadCommit) (result app.ActivationResult, err error) {
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := sql.New(db)
		ev, _ := evidence.New(db)
		if replay, ok, e := ev.GetCommand(txCtx, eid(c.Actor.ActorID), "LETTERHEAD_ACTIVATE", c.KeyHash, c.RequestHash); e != nil {
			return commandError(e)
		} else if ok {
			row, e := q.GetLetterheadVersion(txCtx, id(app.ID(replay.ResponseRef)))
			if e != nil {
				return e
			}
			v, e := letterhead(row)
			result = app.ActivationResult{Version: v, ConfigurationVersion: replay.ResponseVersion, Replay: true}
			return e
		}
		config, e := q.LockLetterheadConfiguration(txCtx)
		if e != nil {
			return e
		}
		if config.Version != c.ExpectedConfigurationVersion {
			return app.ErrConflict
		}
		row, e := q.GetLetterheadVersion(txCtx, id(c.VersionID))
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrNotFound
		}
		if e != nil {
			return e
		}
		v, e := letterhead(row)
		if e != nil {
			return e
		}
		changed, e := q.ActivateLetterheadVersion(txCtx, sql.ActivateLetterheadVersionParams{VersionID: id(c.VersionID), ActorID: id(c.Actor.ActorID), At: ts(c.At), ExpectedVersion: config.Version})
		if e != nil {
			return e
		}
		if changed != 1 {
			return app.ErrConflict
		}
		newConfig := config.Version + 1
		if e = q.InsertLetterheadActivationEvent(txCtx, sql.InsertLetterheadActivationEventParams{ID: id(c.ActivationID), VersionID: id(c.VersionID), PreviousVersionID: config.ActiveVersionID, ConfigurationVersion: newConfig, ActorID: id(c.Actor.ActorID), Reason: c.Reason, At: ts(c.At)}); e != nil {
			return e
		}
		if e = ev.Complete(txCtx, eid(c.CommandID), eid(c.Actor.ActorID), "LETTERHEAD_ACTIVATE", c.KeyHash, c.RequestHash, 200, eid(c.VersionID), newConfig, c.At); e != nil {
			return conflict(e)
		}
		payload, _ := json.Marshal(struct {
			VersionID string `json:"version_id"`
		}{uuidText(c.VersionID)})
		if e = ev.Append(txCtx, evidence.Evidence{AuditID: eid(c.AuditID), OutboxID: eid(c.OutboxID), ActorID: eid(c.Actor.ActorID), TargetID: eid(c.VersionID), CorrelationID: eid(c.CorrelationID), Action: "LetterheadActivated", TargetType: "letterhead_version", AggregateType: "letterhead_configuration", EventType: "organization.letterhead.activated.v1", AggregateVersion: newConfig, Summary: []byte(`{"activation":"recorded"}`), Payload: payload, At: c.At}); e != nil {
			return e
		}
		result = app.ActivationResult{Version: v, ConfigurationVersion: newConfig}
		return nil
	})
	return result, err
}
func (s *Store) GetProfile(ctx context.Context, _ app.Authority) (result app.CompanyProfile, err error) {
	err = s.runner.WithinConsistentReadTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, e := sql.New(db).GetCompanyProfile(txCtx)
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrNotFound
		}
		if e != nil {
			return e
		}
		result, e = profile(row)
		return e
	})
	return result, err
}
func (s *Store) GetLetterhead(ctx context.Context, _ app.Authority, versionID app.ID) (result app.LetterheadVersion, err error) {
	err = s.runner.WithinConsistentReadTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, e := sql.New(db).GetLetterheadVersion(txCtx, id(versionID))
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrNotFound
		}
		if e != nil {
			return e
		}
		result, e = letterhead(row)
		return e
	})
	return result, err
}
func (s *Store) GetActiveLetterhead(ctx context.Context, _ app.Authority) (result app.LetterheadVersion, config app.LetterheadConfiguration, err error) {
	err = s.runner.WithinConsistentReadTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := sql.New(db)
		row, e := q.GetActiveLetterhead(txCtx)
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrNotFound
		}
		if e != nil {
			return e
		}
		result, e = activeLetterhead(row)
		if e != nil {
			return e
		}
		config = app.LetterheadConfiguration{ActiveVersionID: &result.ID, NextVersionNumber: row.NextVersionNumber, Version: row.ConfigurationVersion, UpdatedBy: appIDPointer(row.ConfigurationUpdatedBy), UpdatedAt: row.ConfigurationUpdatedAt.Time.UTC()}
		return nil
	})
	return
}
func insertProfile(c app.SaveProfileCommit) sql.InsertCompanyProfileParams {
	f := c.Fields
	return sql.InsertCompanyProfileParams{ID: id(c.ProfileID), RegisteredName: f.RegisteredName, TradingName: f.TradingName, BusinessType: f.BusinessType, PrimaryIndustry: f.PrimaryIndustry, Description: f.Description, ServicesCapabilities: f.ServicesCapabilities, WorkEmail: f.WorkEmail, WorkPhone: f.WorkPhone, PreferredLanguage: f.PreferredLanguage, Locale: f.Locale, Website: f.Website, PostalAddress: f.PostalAddress, PhysicalAddress: f.PhysicalAddress, RegistrationIdentifier: f.RegistrationIdentifier, TaxIdentifier: f.TaxIdentifier, ActorID: id(c.Actor.ActorID), At: ts(c.At)}
}
func updateProfile(c app.SaveProfileCommit) sql.UpdateCompanyProfileParams {
	f := c.Fields
	return sql.UpdateCompanyProfileParams{RegisteredName: f.RegisteredName, TradingName: f.TradingName, BusinessType: f.BusinessType, PrimaryIndustry: f.PrimaryIndustry, Description: f.Description, ServicesCapabilities: f.ServicesCapabilities, WorkEmail: f.WorkEmail, WorkPhone: f.WorkPhone, PreferredLanguage: f.PreferredLanguage, Locale: f.Locale, Website: f.Website, PostalAddress: f.PostalAddress, PhysicalAddress: f.PhysicalAddress, RegistrationIdentifier: f.RegistrationIdentifier, TaxIdentifier: f.TaxIdentifier, ActorID: id(c.Actor.ActorID), At: ts(c.At), ExpectedVersion: c.ExpectedVersion}
}
func insertLetterhead(c app.CreateLetterheadCommit, n int64) sql.InsertLetterheadVersionParams {
	f := c.Fields
	return sql.InsertLetterheadVersionParams{ID: id(c.VersionID), VersionNumber: n, RegisteredName: f.RegisteredName, TradingName: f.TradingName, RegistrationIdentifier: f.RegistrationIdentifier, TaxIdentifier: f.TaxIdentifier, PostalAddress: f.PostalAddress, PhysicalAddress: f.PhysicalAddress, Phone: f.Phone, Email: f.Email, Website: f.Website, Slogan: f.Slogan, HeaderLayout: f.HeaderLayout, FooterLayout: f.FooterLayout, PageNumberStyle: f.PageNumberStyle, PrimaryColor: f.PrimaryColor, SecondaryColor: f.SecondaryColor, Disclaimer: f.Disclaimer, TemplateKey: f.TemplateKey, ContentHash: c.ContentHash[:], ActorID: id(c.Actor.ActorID), At: ts(c.At)}
}
func profile(r sql.OrganizationCompanyProfile) (app.CompanyProfile, error) {
	if !r.ID.Valid || !r.CreatedBy.Valid || !r.UpdatedBy.Valid || !r.CreatedAt.Valid || !r.UpdatedAt.Valid || r.Version < 1 {
		return app.CompanyProfile{}, errors.New("organization_profile_result_invalid")
	}
	return app.CompanyProfile{ID: app.ID(r.ID.Bytes), CompanyProfileFields: app.CompanyProfileFields{RegisteredName: r.RegisteredName, TradingName: r.TradingName, BusinessType: r.BusinessType, PrimaryIndustry: r.PrimaryIndustry, Description: r.Description, ServicesCapabilities: r.ServicesCapabilities, WorkEmail: r.WorkEmail, WorkPhone: r.WorkPhone, PreferredLanguage: r.PreferredLanguage, Locale: r.Locale, Website: r.Website, PostalAddress: r.PostalAddress, PhysicalAddress: r.PhysicalAddress, RegistrationIdentifier: r.RegistrationIdentifier, TaxIdentifier: r.TaxIdentifier}, Version: r.Version, CreatedBy: app.ID(r.CreatedBy.Bytes), UpdatedBy: app.ID(r.UpdatedBy.Bytes), CreatedAt: r.CreatedAt.Time.UTC(), UpdatedAt: r.UpdatedAt.Time.UTC()}, nil
}
func letterhead(r sql.OrganizationLetterheadVersion) (app.LetterheadVersion, error) {
	if !r.ID.Valid || !r.CreatedBy.Valid || !r.CreatedAt.Valid || r.VersionNumber < 1 || len(r.ContentHash) != 32 {
		return app.LetterheadVersion{}, errors.New("letterhead_result_invalid")
	}
	var hash [32]byte
	copy(hash[:], r.ContentHash)
	return app.LetterheadVersion{ID: app.ID(r.ID.Bytes), VersionNumber: r.VersionNumber, LetterheadFields: app.LetterheadFields{RegisteredName: r.RegisteredName, TradingName: r.TradingName, RegistrationIdentifier: r.RegistrationIdentifier, TaxIdentifier: r.TaxIdentifier, PostalAddress: r.PostalAddress, PhysicalAddress: r.PhysicalAddress, Phone: r.Phone, Email: r.Email, Website: r.Website, Slogan: r.Slogan, HeaderLayout: r.HeaderLayout, FooterLayout: r.FooterLayout, PageNumberStyle: r.PageNumberStyle, PrimaryColor: r.PrimaryColor, SecondaryColor: r.SecondaryColor, Disclaimer: r.Disclaimer, TemplateKey: r.TemplateKey}, ContentHash: hash, CreatedBy: app.ID(r.CreatedBy.Bytes), CreatedAt: r.CreatedAt.Time.UTC()}, nil
}
func activeLetterhead(r sql.GetActiveLetterheadRow) (app.LetterheadVersion, error) {
	return letterhead(sql.OrganizationLetterheadVersion{ID: r.ID, VersionNumber: r.VersionNumber, RegisteredName: r.RegisteredName, TradingName: r.TradingName, RegistrationIdentifier: r.RegistrationIdentifier, TaxIdentifier: r.TaxIdentifier, PostalAddress: r.PostalAddress, PhysicalAddress: r.PhysicalAddress, Phone: r.Phone, Email: r.Email, Website: r.Website, Slogan: r.Slogan, HeaderLayout: r.HeaderLayout, FooterLayout: r.FooterLayout, PageNumberStyle: r.PageNumberStyle, PrimaryColor: r.PrimaryColor, SecondaryColor: r.SecondaryColor, Disclaimer: r.Disclaimer, TemplateKey: r.TemplateKey, ContentHash: r.ContentHash, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt})
}
func appIDPointer(v pgtype.UUID) *app.ID {
	if !v.Valid {
		return nil
	}
	value := app.ID(v.Bytes)
	return &value
}
func id(v app.ID) pgtype.UUID           { return pgtype.UUID{Bytes: v, Valid: !v.IsZero()} }
func ts(v time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: v.UTC(), Valid: !v.IsZero()} }
func eid(v app.ID) evidence.ID          { return evidence.ID(v) }
func conflict(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && (pg.Code == "23505" || pg.Code == "23503") {
		return app.ErrConflict
	}
	return err
}
func commandError(err error) error {
	if err != nil && err.Error() == "tenant_command_conflict" {
		return app.ErrConflict
	}
	return err
}
func uuidText(v app.ID) string {
	b := v
	hex := "0123456789abcdef"
	out := make([]byte, 36)
	j := 0
	for i, x := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out[j] = '-'
			j++
		}
		out[j] = hex[x>>4]
		out[j+1] = hex[x&15]
		j += 2
	}
	return string(out)
}
