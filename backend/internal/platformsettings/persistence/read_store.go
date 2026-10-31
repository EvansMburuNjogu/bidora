package persistence

import (
	"context"
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

type settingsReadRunner interface {
	WithinReadOnlyTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type ReadStore struct{ runner settingsReadRunner }

func NewReadStore(runner settingsReadRunner) (*ReadStore, error) {
	if runner == nil {
		return nil, errors.New("platform_settings_read_store_invalid")
	}
	return &ReadStore{runner: runner}, nil
}
func (s *ReadStore) WithinSettingsRead(ctx context.Context, fn func(context.Context, settingsapp.ReadTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("platform_settings_read_unit_of_work_invalid")
	}
	return s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		authority, err := platformpersistence.NewSettingsAuthorityTx(db)
		if err != nil {
			return err
		}
		return fn(txCtx, &settingsReadTx{q: settingssql.New(db), authority: authority})
	})
}

type settingsReadTx struct {
	q         *settingssql.Queries
	authority *platformpersistence.SettingsAuthorityTx
}

func (t *settingsReadTx) Authorize(ctx context.Context, subject settingsapp.Subject, at time.Time) (settingsapp.Authority, error) {
	facts, err := t.authority.Resolve(ctx, platformapp.AuthorityQuery{UserID: platformID(subject.ActorID), SessionID: platformID(subject.SessionID), ObservedAt: at})
	if err != nil {
		return settingsapp.Authority{}, err
	}
	if facts.ContextGeneration != subject.ContextGeneration {
		return settingsapp.Authority{}, settingsapp.ErrForbidden
	}
	capabilities := map[string]bool{"platform.integration.read": facts.Grants("platform.integration.read")}
	owner := false
	for _, role := range facts.Roles {
		if role == platformdomain.PlatformOwner {
			owner = true
		}
	}
	return settingsapp.Authority{UserID: settingsID(facts.UserID), Capabilities: capabilities, EffectiveOwner: owner, AssuranceLevel: facts.AssuranceLevel, AssuranceMethod: facts.AssuranceMethod, AssuredAt: facts.AssuredAt}, nil
}
func (t *settingsReadTx) GetActiveVersion(ctx context.Context, kind domain.Kind) (settingsapp.RedactedVersion, bool, error) {
	row, err := t.q.GetActiveIntegrationVersion(ctx, string(kind))
	if errors.Is(err, pgx.ErrNoRows) {
		return settingsapp.RedactedVersion{}, false, nil
	}
	if err != nil {
		return settingsapp.RedactedVersion{}, false, err
	}
	view, err := t.version(ctx, versionRow{id: row.ID, kind: row.SettingKind, sequence: row.Sequence, state: row.State, version: row.Version, latestTestID: row.LatestTestID, createdAt: row.CreatedAt, updatedAt: row.UpdatedAt, activatedAt: row.ActivatedAt, supersededAt: row.SupersededAt, host: row.Host, port: row.Port, senderName: row.SenderName, senderAddress: row.SenderAddress, replyToAddress: row.ReplyToAddress, connectTimeout: row.ConnectTimeoutSeconds, commandTimeout: row.CommandTimeoutSeconds, deliveryTimeout: row.DeliveryTimeoutSeconds, smtpConcurrency: row.SmtpMaxConcurrency, model: row.Model, openaiTimeout: row.TimeoutSeconds, openaiConcurrency: row.OpenaiMaxConcurrency, openaiEnabled: row.OpenaiEnabled, pesapalEnabled: row.PesapalEnabled, environment: row.Environment, merchantName: row.MerchantName, notificationID: row.NotificationID, credentialsConfigured: row.PesapalCredentialsConfigured})
	return view, err == nil, err
}
func (t *settingsReadTx) ListVersions(ctx context.Context, kind domain.Kind, before *int64, limit int32) ([]settingsapp.RedactedVersion, error) {
	rows, err := t.q.ListIntegrationVersions(ctx, settingssql.ListIntegrationVersionsParams{SettingKind: string(kind), BeforeSequence: before, ResultLimit: limit})
	if err != nil {
		return nil, err
	}
	result := make([]settingsapp.RedactedVersion, len(rows))
	for i, row := range rows {
		result[i], err = t.version(ctx, versionRow{id: row.ID, kind: row.SettingKind, sequence: row.Sequence, state: row.State, version: row.Version, latestTestID: row.LatestTestID, createdAt: row.CreatedAt, updatedAt: row.UpdatedAt, activatedAt: row.ActivatedAt, supersededAt: row.SupersededAt, host: row.Host, port: row.Port, senderName: row.SenderName, senderAddress: row.SenderAddress, replyToAddress: row.ReplyToAddress, connectTimeout: row.ConnectTimeoutSeconds, commandTimeout: row.CommandTimeoutSeconds, deliveryTimeout: row.DeliveryTimeoutSeconds, smtpConcurrency: row.SmtpMaxConcurrency, model: row.Model, openaiTimeout: row.TimeoutSeconds, openaiConcurrency: row.OpenaiMaxConcurrency, openaiEnabled: row.OpenaiEnabled, pesapalEnabled: row.PesapalEnabled, environment: row.Environment, merchantName: row.MerchantName, notificationID: row.NotificationID, credentialsConfigured: row.PesapalCredentialsConfigured})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
func (t *settingsReadTx) GetChangeView(ctx context.Context, changeID domain.ID) (settingsapp.RedactedChange, bool, error) {
	row, err := t.q.GetIntegrationChange(ctx, uuid(changeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return settingsapp.RedactedChange{}, false, nil
	}
	if err != nil {
		return settingsapp.RedactedChange{}, false, err
	}
	if !row.ID.Valid || !row.TargetVersionID.Valid || !row.TargetTestID.Valid || !row.MakerUserID.Valid || !row.CreatedAt.Valid || !row.ExpiresAt.Valid || row.Version < 1 {
		return settingsapp.RedactedChange{}, false, errors.New("platform_setting_change_view_invalid")
	}
	return settingsapp.RedactedChange{ID: id(row.ID), TargetVersionID: id(row.TargetVersionID), TargetTestID: id(row.TargetTestID), ExpectedActiveVersionID: id(row.ExpectedActiveVersionID), MakerUserID: id(row.MakerUserID), CheckerUserID: id(row.CheckerUserID), Kind: domain.Kind(row.SettingKind), Action: domain.Action(row.Action), State: domain.ChangeState(row.State), TargetPriorState: domain.VersionState(row.TargetPriorState), Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), ExpiresAt: row.ExpiresAt.Time.UTC(), DecidedAt: timePtr(row.DecidedAt)}, true, nil
}

type versionRow struct {
	credentialsConfigured                                            bool
	pesapalEnabled                                                   *bool
	environment, merchantName                                        *string
	notificationID                                                   pgtype.UUID
	openaiEnabled                                                    *bool
	id                                                               pgtype.UUID
	kind                                                             string
	sequence                                                         int64
	state                                                            string
	version                                                          int64
	latestTestID                                                     pgtype.UUID
	createdAt, updatedAt, activatedAt, supersededAt                  pgtype.Timestamptz
	host                                                             *string
	port                                                             *int32
	senderName, senderAddress, replyToAddress                        *string
	connectTimeout, commandTimeout, deliveryTimeout, smtpConcurrency *int32
	model                                                            *string
	openaiTimeout, openaiConcurrency                                 *int32
}

func (t *settingsReadTx) version(ctx context.Context, row versionRow) (settingsapp.RedactedVersion, error) {
	if !row.id.Valid || row.sequence < 1 || row.version < 1 || !row.createdAt.Valid || !row.updatedAt.Valid {
		return settingsapp.RedactedVersion{}, errors.New("platform_setting_version_view_invalid")
	}
	view := settingsapp.RedactedVersion{ID: id(row.id), LatestTestID: id(row.latestTestID), Kind: domain.Kind(row.kind), Sequence: row.sequence, State: domain.VersionState(row.state), Version: row.version, TestStatus: "NOT_TESTED", CreatedAt: row.createdAt.Time.UTC(), UpdatedAt: row.updatedAt.Time.UTC(), ActivatedAt: timePtr(row.activatedAt), SupersededAt: timePtr(row.supersededAt)}
	switch view.Kind {
	case domain.PesaPal:
		if row.pesapalEnabled == nil || row.environment == nil || row.merchantName == nil {
			return settingsapp.RedactedVersion{}, errors.New("platform_setting_version_view_invalid")
		}
		view.PesaPal = &settingsapp.RedactedPesaPal{CredentialsConfigured: row.credentialsConfigured, Enabled: *row.pesapalEnabled, Environment: *row.environment, MerchantName: *row.merchantName, NotificationID: id(row.notificationID)}
	case domain.SMTP:
		if row.host == nil || row.port == nil || row.senderName == nil || row.senderAddress == nil || row.connectTimeout == nil || row.commandTimeout == nil || row.deliveryTimeout == nil || row.smtpConcurrency == nil {
			return settingsapp.RedactedVersion{}, errors.New("platform_setting_version_view_invalid")
		}
		view.SMTP = &settingsapp.RedactedSMTP{Host: *row.host, Port: *row.port, SenderName: *row.senderName, SenderAddress: *row.senderAddress, ReplyToAddress: row.replyToAddress, ConnectTimeoutSeconds: *row.connectTimeout, CommandTimeoutSeconds: *row.commandTimeout, DeliveryTimeoutSeconds: *row.deliveryTimeout, MaxConcurrency: *row.smtpConcurrency}
	case domain.OpenAI:
		if row.model == nil || row.openaiTimeout == nil || row.openaiConcurrency == nil {
			return settingsapp.RedactedVersion{}, errors.New("platform_setting_version_view_invalid")
		}
		view.OpenAI = &settingsapp.RedactedOpenAI{Enabled: row.openaiEnabled == nil || *row.openaiEnabled, Model: *row.model, TimeoutSeconds: *row.openaiTimeout, MaxConcurrency: *row.openaiConcurrency}
	default:
		return settingsapp.RedactedVersion{}, errors.New("platform_setting_version_view_invalid")
	}
	if !view.LatestTestID.IsZero() {
		test, err := t.q.GetIntegrationTest(ctx, uuid(view.LatestTestID))
		if err != nil {
			return settingsapp.RedactedVersion{}, err
		}
		view.TestStatus, view.TestCode = test.Outcome, test.ResultCode
		at := test.OccurredAt.Time.UTC()
		view.TestedAt = &at
	}
	return view, nil
}
