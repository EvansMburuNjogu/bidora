package persistence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	platformapp "bidora/backend/internal/platformadmin/application"
	platformdomain "bidora/backend/internal/platformadmin/domain"
	platformpersistence "bidora/backend/internal/platformadmin/persistence"
	settingsapp "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	settingssql "bidora/backend/internal/platformsettings/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type informationRunner interface {
	WithinReadOnlyTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type InformationStore struct{ runner informationRunner }

func NewInformationStore(runner informationRunner) (*InformationStore, error) {
	if runner == nil {
		return nil, errors.New("platform_information_store_invalid")
	}
	return &InformationStore{runner}, nil
}
func (s *InformationStore) WithinInformationRead(ctx context.Context, fn func(context.Context, settingsapp.InformationTx) error) error {
	if s == nil || s.runner == nil || ctx == nil || fn == nil {
		return errors.New("platform_information_read_invalid")
	}
	return s.runner.WithinReadOnlyTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		tx, e := newInformationTx(db)
		if e != nil {
			return e
		}
		return fn(c, tx)
	})
}
func (s *InformationStore) WithinInformationWrite(ctx context.Context, fn func(context.Context, settingsapp.InformationTx) error) error {
	if s == nil || s.runner == nil || ctx == nil || fn == nil {
		return errors.New("platform_information_write_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		tx, e := newInformationTx(db)
		if e != nil {
			return e
		}
		return fn(c, tx)
	})
}

type informationTx struct {
	q         *settingssql.Queries
	authority *platformpersistence.SettingsAuthorityTx
}

func newInformationTx(db postgres.DBTX) (*informationTx, error) {
	auth, e := platformpersistence.NewSettingsAuthorityTx(db)
	if e != nil {
		return nil, e
	}
	return &informationTx{settingssql.New(db), auth}, nil
}
func (t *informationTx) AuthorizeInformation(ctx context.Context, subject settingsapp.Subject, at time.Time) (settingsapp.Authority, error) {
	facts, e := t.authority.Resolve(ctx, platformapp.AuthorityQuery{UserID: platformapp.ID(subject.ActorID), SessionID: platformapp.ID(subject.SessionID), ObservedAt: at})
	if errors.Is(e, platformapp.ErrAuthorityNotFound) {
		return settingsapp.Authority{}, settingsapp.ErrInformationForbidden
	}
	if e != nil {
		return settingsapp.Authority{}, e
	}
	if facts.ContextGeneration != subject.ContextGeneration {
		return settingsapp.Authority{}, settingsapp.ErrInformationForbidden
	}
	owner := false
	for _, role := range facts.Roles {
		if role == platformdomain.PlatformOwner {
			owner = true
		}
	}
	return settingsapp.Authority{UserID: domain.ID(facts.UserID), Capabilities: map[string]bool{"platform.admin.read": facts.Grants("platform.admin.read"), "platform.integration.read": facts.Grants("platform.integration.read")}, EffectiveOwner: owner, AssuranceLevel: facts.AssuranceLevel, AssuranceMethod: facts.AssuranceMethod, AssuredAt: facts.AssuredAt}, nil
}
func (t *informationTx) ReadInformation(ctx context.Context, lock bool) (settingsapp.Information, error) {
	if lock {
		v, e := t.q.LockPlatformInformation(ctx)
		if e != nil {
			return settingsapp.Information{}, e
		}
		return informationView(v.PlatformName, v.ContactPhone, v.SupportEmail, v.Website, v.Version, v.UpdatedAt)
	}
	v, e := t.q.GetPlatformInformation(ctx)
	if e != nil {
		return settingsapp.Information{}, e
	}
	return informationView(v.PlatformName, v.ContactPhone, v.SupportEmail, v.Website, v.Version, v.UpdatedAt)
}
func informationView(name, phone string, email, website *string, version int64, at pgtype.Timestamptz) (settingsapp.Information, error) {
	if name == "" || version < 1 || at.Valid != (email != nil && website != nil) || (email == nil) != (website == nil) {
		return settingsapp.Information{}, errors.New("platform_information_row_invalid")
	}
	var changed *time.Time
	if at.Valid {
		v := at.Time.UTC()
		changed = &v
	}
	return settingsapp.Information{PlatformName: name, ContactPhone: phone, SupportEmail: email, Website: website, Version: version, UpdatedAt: changed}, nil
}
func (t *informationTx) FindInformationCommand(ctx context.Context, actor domain.ID, key [32]byte) (settingsapp.InformationCommand, bool, error) {
	row, e := t.q.GetPlatformInformationCommand(ctx, settingssql.GetPlatformInformationCommandParams{ActorID: uuid(actor), KeyHash: key[:]})
	if errors.Is(e, pgx.ErrNoRows) {
		return settingsapp.InformationCommand{}, false, nil
	}
	if e != nil {
		return settingsapp.InformationCommand{}, false, e
	}
	if len(row.RequestHash) != 32 || row.ResultVersion <= 1 {
		return settingsapp.InformationCommand{}, false, errors.New("platform_information_command_invalid")
	}
	var hash [32]byte
	copy(hash[:], row.RequestHash)
	return settingsapp.InformationCommand{RequestHash: hash, ResultVersion: row.ResultVersion}, true, nil
}
func (t *informationTx) UpdateInformation(ctx context.Context, in settingsapp.InformationSave, at time.Time) (int64, error) {
	return t.q.UpdatePlatformInformation(ctx, settingssql.UpdatePlatformInformationParams{PlatformName: *in.PlatformName, ContactPhone: *in.ContactPhone, SupportEmail: in.SupportEmail, Website: in.Website, UpdatedAt: pgtype.Timestamptz{Time: at, Valid: true}, UpdatedBy: uuid(in.Subject.ActorID), ExpectedVersion: in.ExpectedVersion})
}
func (t *informationTx) AppendInformationEvidence(ctx context.Context, audit, outbox domain.ID, in settingsapp.InformationSave, version int64, at time.Time, key, request [32]byte) error {
	timeValue := pgtype.Timestamptz{Time: at, Valid: true}
	actor := uuid(in.Subject.ActorID)
	changes := in.Changes
	if changes == nil {
		changes = []settingsapp.FieldChange{}
	}
	raw, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	if e := t.q.AppendPlatformInformationAudit(ctx, settingssql.AppendPlatformInformationAuditParams{ID: uuid(audit), ActorID: actor, At: timeValue, CorrelationID: uuid(in.CorrelationID), Version: version, Changes: raw}); e != nil {
		return e
	}
	if e := t.q.AppendPlatformInformationOutbox(ctx, settingssql.AppendPlatformInformationOutboxParams{ID: uuid(outbox), ActorID: actor, Version: version, At: timeValue}); e != nil {
		return e
	}
	return t.q.InsertPlatformInformationCommand(ctx, settingssql.InsertPlatformInformationCommandParams{ActorID: actor, KeyHash: bytes.Clone(key[:]), RequestHash: bytes.Clone(request[:]), ResultVersion: version})
}
