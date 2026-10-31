// Package persistence implements Platform integration-setting control-plane ports.
package persistence

import (
	"context"
	"errors"
	"time"

	controlapp "bidora/backend/internal/controlplane/application"
	controlpersistence "bidora/backend/internal/controlplane/persistence"
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

type transactionRunner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type Store struct{ runner transactionRunner }

func NewStore(runner transactionRunner) (*Store, error) {
	if runner == nil {
		return nil, errors.New("platform_settings_store_invalid")
	}
	return &Store{runner}, nil
}
func (s *Store) WithinSettings(ctx context.Context, fn func(context.Context, settingsapp.Tx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("platform_settings_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		tx, e := newTx(db)
		if e != nil {
			return e
		}
		return fn(txCtx, tx)
	})
}

type tx struct {
	q         *settingssql.Queries
	authority *platformpersistence.SettingsAuthorityTx
	evidence  *controlpersistence.EvidenceTx
}

func newTx(db postgres.DBTX) (*tx, error) {
	if db == nil {
		return nil, errors.New("platform_settings_tx_invalid")
	}
	authority, e := platformpersistence.NewSettingsAuthorityTx(db)
	if e != nil {
		return nil, e
	}
	evidence, e := controlpersistence.NewEvidenceTx(db)
	if e != nil {
		return nil, e
	}
	return &tx{settingssql.New(db), authority, evidence}, nil
}
func (t *tx) Authorize(ctx context.Context, subject settingsapp.Subject, at time.Time) (settingsapp.Authority, error) {
	if e := t.authority.LockConfigurationWriter(ctx); e != nil {
		return settingsapp.Authority{}, e
	}
	facts, e := t.authority.Resolve(ctx, platformapp.AuthorityQuery{UserID: platformID(subject.ActorID), SessionID: platformID(subject.SessionID), ObservedAt: at})
	if e != nil {
		return settingsapp.Authority{}, e
	}
	if facts.ContextGeneration != subject.ContextGeneration {
		return settingsapp.Authority{}, settingsapp.ErrForbidden
	}
	capabilities := map[string]bool{}
	for _, capability := range []string{"platform.integration.smtp.manage", "platform.integration.smtp.test", "platform.integration.smtp.activate", "platform.integration.ai.manage", "platform.integration.ai.test", "platform.integration.ai.activate", "platform.integration.pesapal.manage", "platform.integration.pesapal.test", "platform.integration.pesapal.activate", "platform.integration.rollback", "platform.governance.approve"} {
		capabilities[capability] = facts.Grants(capability)
	}
	owner := false
	for _, role := range facts.Roles {
		if role == platformdomain.PlatformOwner {
			owner = true
		}
	}
	return settingsapp.Authority{UserID: settingsID(facts.UserID), Capabilities: capabilities, EffectiveOwner: owner, AssuranceLevel: facts.AssuranceLevel, AssuranceMethod: facts.AssuranceMethod, AssuredAt: facts.AssuredAt}, nil
}
func (t *tx) GetCommand(ctx context.Context, actor domain.ID, operation string, key [32]byte) (settingsapp.Command, bool, error) {
	row, e := t.q.GetIntegrationCommand(ctx, settingssql.GetIntegrationCommandParams{ActorUserID: uuid(actor), Operation: operation, KeyHash: key[:]})
	if errors.Is(e, pgx.ErrNoRows) {
		return settingsapp.Command{}, false, nil
	}
	if e != nil {
		return settingsapp.Command{}, false, e
	}
	c := settingsapp.Command{RequestHash: hash32(row.RequestHash), State: row.State, TargetID: id(row.TargetID)}
	if row.ResponseState != nil {
		c.ResponseState = *row.ResponseState
	}
	if row.ResponseVersion != nil {
		c.ResponseVersion = *row.ResponseVersion
	}
	return c, true, nil
}
func (t *tx) LockHead(ctx context.Context, kind domain.Kind) (settingsapp.Head, error) {
	row, e := t.q.LockIntegrationHead(ctx, string(kind))
	if e != nil {
		return settingsapp.Head{}, e
	}
	return settingsapp.Head{Kind: kind, NextSequence: row.NextSequence, Generation: row.Generation, ActiveVersionID: id(row.ActiveVersionID)}, nil
}
func (t *tx) InsertVersion(ctx context.Context, v domain.Version, reason string) error {
	rows, e := t.q.AdvanceIntegrationSequence(ctx, settingssql.AdvanceIntegrationSequenceParams{At: ts(v.CreatedAt), SettingKind: string(v.Kind), ExpectedSequence: v.Sequence})
	if e != nil {
		return e
	}
	if rows != 1 {
		return settingsapp.ErrConflict
	}
	if e = t.q.InsertIntegrationVersion(ctx, settingssql.InsertIntegrationVersionParams{ID: uuid(v.ID), SettingKind: string(v.Kind), Sequence: v.Sequence, ConfigurationHash: v.ConfigurationHash[:], CreatedBy: uuid(v.CreatedBy), Reason: reason, At: ts(v.CreatedAt)}); e != nil {
		return e
	}
	if v.Kind == domain.SMTP {
		c := v.Configuration.SMTP
		return t.q.InsertSMTPVersion(ctx, settingssql.InsertSMTPVersionParams{VersionID: uuid(v.ID), Host: c.Host, Port: c.Port, CredentialSecretRef: c.CredentialSecretRef, TrustRootsSecretRef: c.TrustRootsSecretRef, SenderName: c.SenderName, SenderAddress: c.SenderAddress, ReplyToAddress: c.ReplyToAddress, ConnectTimeoutSeconds: c.ConnectTimeoutSeconds, CommandTimeoutSeconds: c.CommandTimeoutSeconds, DeliveryTimeoutSeconds: c.DeliveryTimeoutSeconds, MaxConcurrency: c.MaxConcurrency})
	}
	if v.Kind == domain.PesaPal {
		c := v.Configuration.PesaPal
		return t.q.InsertPesaPalVersion(ctx, settingssql.InsertPesaPalVersionParams{VersionID: uuid(v.ID), Enabled: c.Enabled, Environment: c.Environment, MerchantName: c.MerchantName, CredentialsSecretRef: c.CredentialsSecretRef, NotificationID: uuid(c.NotificationID)})
	}
	c := v.Configuration.OpenAI
	return t.q.InsertOpenAIVersion(ctx, settingssql.InsertOpenAIVersionParams{Enabled: !c.Disabled, VersionID: uuid(v.ID), Model: c.Model, ApiKeySecretRef: c.APIKeySecretRef, TrustRootsSecretRef: c.TrustRootsSecretRef, TimeoutSeconds: c.TimeoutSeconds, MaxConcurrency: c.MaxConcurrency})
}
func (t *tx) InsertSealedSecret(ctx context.Context, s settingsapp.SealedIntegrationSecret, actor domain.ID, at time.Time) error {
	return t.q.InsertSMTPSecret(ctx, settingssql.InsertSMTPSecretParams{ID: uuid(s.ID), SecretKind: s.Kind, KeyVersion: s.KeyVersion, Nonce: s.Nonce, Ciphertext: s.Ciphertext, CreatedBy: uuid(actor), CreatedAt: ts(at)})
}

func (t *tx) CompleteCommand(ctx context.Context, c settingsapp.Command, operation string, actor domain.ID, at time.Time) error {
	return t.q.InsertIntegrationCommand(ctx, settingssql.InsertIntegrationCommandParams{ID: uuid(c.ID), ActorUserID: uuid(actor), Operation: operation, KeyHash: c.KeyHash[:], RequestHash: c.RequestHash[:], TargetID: uuid(c.TargetID), ResponseState: stringPtr(c.ResponseState), ResponseVersion: int64Ptr(c.ResponseVersion), ResponseStatus: int32Ptr(200), At: ts(at)})
}
func (t *tx) LockVersion(ctx context.Context, versionID domain.ID) (domain.Version, bool, error) {
	locked, e := t.q.LockIntegrationVersion(ctx, uuid(versionID))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Version{}, false, nil
	}
	if e != nil {
		return domain.Version{}, false, e
	}
	row, e := t.q.GetIntegrationVersionInternal(ctx, uuid(versionID))
	if e != nil {
		return domain.Version{}, false, e
	}
	v := domain.Version{ID: id(locked.ID), Kind: domain.Kind(locked.SettingKind), Sequence: locked.Sequence, State: domain.VersionState(locked.State), Version: locked.Version, ConfigurationHash: hash32(locked.ConfigurationHash), LatestTestID: id(locked.LatestTestID), CreatedBy: id(locked.CreatedBy), CreatedAt: locked.CreatedAt.Time, UpdatedAt: locked.UpdatedAt.Time, ActivatedAt: timePtr(locked.ActivatedAt), SupersededAt: timePtr(locked.SupersededAt)}
	if v.Kind == domain.SMTP && row.Host != nil && row.Port != nil && row.CredentialSecretRef != nil && row.SmtpTrustRootsSecretRef != nil && row.SenderName != nil && row.SenderAddress != nil && row.ConnectTimeoutSeconds != nil && row.CommandTimeoutSeconds != nil && row.DeliveryTimeoutSeconds != nil && row.SmtpMaxConcurrency != nil {
		v.Configuration = domain.Configuration{Kind: v.Kind, SMTP: &domain.SMTPConfiguration{Host: *row.Host, Port: *row.Port, CredentialSecretRef: *row.CredentialSecretRef, TrustRootsSecretRef: *row.SmtpTrustRootsSecretRef, SenderName: *row.SenderName, SenderAddress: *row.SenderAddress, ReplyToAddress: row.ReplyToAddress, ConnectTimeoutSeconds: *row.ConnectTimeoutSeconds, CommandTimeoutSeconds: *row.CommandTimeoutSeconds, DeliveryTimeoutSeconds: *row.DeliveryTimeoutSeconds, MaxConcurrency: *row.SmtpMaxConcurrency}}
	} else if v.Kind == domain.OpenAI && row.Model != nil && row.ApiKeySecretRef != nil && row.OpenaiTrustRootsSecretRef != nil && row.TimeoutSeconds != nil && row.OpenaiMaxConcurrency != nil {
		v.Configuration = domain.Configuration{Kind: v.Kind, OpenAI: &domain.OpenAIConfiguration{Disabled: row.OpenaiEnabled != nil && !*row.OpenaiEnabled, Model: *row.Model, APIKeySecretRef: *row.ApiKeySecretRef, TrustRootsSecretRef: *row.OpenaiTrustRootsSecretRef, TimeoutSeconds: *row.TimeoutSeconds, MaxConcurrency: *row.OpenaiMaxConcurrency}}
	} else if v.Kind == domain.PesaPal && row.PesapalEnabled != nil && row.Environment != nil && row.MerchantName != nil && row.CredentialsSecretRef != nil {
		v.Configuration = domain.Configuration{Kind: v.Kind, PesaPal: &domain.PesaPalConfiguration{Enabled: *row.PesapalEnabled, Environment: *row.Environment, MerchantName: *row.MerchantName, CredentialsSecretRef: *row.CredentialsSecretRef, NotificationID: id(row.NotificationID)}}
	} else {
		return domain.Version{}, false, errors.New("platform_settings_configuration_corrupt")
	}
	return v, true, nil
}
func (t *tx) GetTest(ctx context.Context, testID domain.ID) (domain.TestResult, bool, error) {
	row, e := t.q.GetIntegrationTest(ctx, uuid(testID))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.TestResult{}, false, nil
	}
	if e != nil {
		return domain.TestResult{}, false, e
	}
	return domain.TestResult{ID: id(row.ID), VersionID: id(row.VersionID), TestedBy: id(row.TestedBy), Attempt: row.Attempt, Outcome: row.Outcome, ResultCode: row.ResultCode, Reason: row.Reason, ApplicabilityHash: hash32(row.ApplicabilityHash), OccurredAt: row.OccurredAt.Time}, true, nil
}
func (t *tx) BeginTest(ctx context.Context, c settingsapp.Command, versionID domain.ID, at time.Time) error {
	return t.q.BeginIntegrationTestCommand(ctx, settingssql.BeginIntegrationTestCommandParams{ID: uuid(c.ID), ActorUserID: uuid(c.ActorID), KeyHash: c.KeyHash[:], RequestHash: c.RequestHash[:], TargetID: uuid(versionID), At: ts(at)})
}
func (t *tx) CommitTest(ctx context.Context, c settingsapp.Command, v domain.Version, test domain.TestResult) error {
	count, e := t.q.CountIntegrationTests(ctx, uuid(v.ID))
	if e != nil {
		return e
	}
	if e = t.q.InsertIntegrationTest(ctx, settingssql.InsertIntegrationTestParams{ID: uuid(test.ID), VersionID: uuid(v.ID), Attempt: count + 1, Outcome: test.Outcome, ResultCode: test.ResultCode, ApplicabilityHash: test.ApplicabilityHash[:], TestedBy: uuid(test.TestedBy), Reason: test.Reason, OccurredAt: ts(test.OccurredAt)}); e != nil {
		return e
	}
	rows, e := t.q.ApplyIntegrationTestResult(ctx, settingssql.ApplyIntegrationTestResultParams{State: string(v.State), TestID: uuid(test.ID), At: ts(v.UpdatedAt), ID: uuid(v.ID), ExpectedVersion: v.Version - 1})
	if e != nil {
		return e
	}
	if rows != 1 {
		return settingsapp.ErrConflict
	}
	rows, e = t.q.CompleteIntegrationTestCommand(ctx, settingssql.CompleteIntegrationTestCommandParams{ResponseState: stringPtr(c.ResponseState), ResponseVersion: int64Ptr(v.Version), At: ts(v.UpdatedAt), ID: uuid(c.ID)})
	if e != nil {
		return e
	}
	if rows != 1 {
		return settingsapp.ErrConflict
	}
	return nil
}
func (t *tx) InsertChange(ctx context.Context, c domain.Change, reason string, v domain.Version, command settingsapp.Command) error {
	if e := t.q.InsertIntegrationChange(ctx, settingssql.InsertIntegrationChangeParams{ID: uuid(c.ID), SettingKind: string(c.Kind), TargetVersionID: uuid(c.TargetVersionID), Action: string(c.Action), TargetPriorState: string(c.TargetPriorState), TargetTestID: uuid(c.TargetTestID), ExpectedActiveVersionID: uuid(c.ExpectedActiveVersionID), MakerUserID: uuid(c.MakerUserID), Reason: reason, CreatedAt: ts(c.CreatedAt), ExpiresAt: ts(c.ExpiresAt)}); e != nil {
		return e
	}
	rows, e := t.q.MarkIntegrationVersionPending(ctx, settingssql.MarkIntegrationVersionPendingParams{At: ts(c.CreatedAt), ID: uuid(v.ID), ExpectedVersion: v.Version, ExpectedState: string(v.State)})
	if e != nil {
		return e
	}
	if rows != 1 {
		return settingsapp.ErrConflict
	}
	return t.insertCommand(ctx, command, "PROPOSE_ACTIVATION", c.MakerUserID, c.CreatedAt)
}
func (t *tx) LockChange(ctx context.Context, changeID domain.ID) (domain.Change, bool, error) {
	row, e := t.q.LockIntegrationChange(ctx, uuid(changeID))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Change{}, false, nil
	}
	if e != nil {
		return domain.Change{}, false, e
	}
	return domain.Change{ID: id(row.ID), Kind: domain.Kind(row.SettingKind), TargetVersionID: id(row.TargetVersionID), Action: domain.Action(row.Action), State: domain.ChangeState(row.State), Version: row.Version, TargetPriorState: domain.VersionState(row.TargetPriorState), TargetTestID: id(row.TargetTestID), ExpectedActiveVersionID: id(row.ExpectedActiveVersionID), MakerUserID: id(row.MakerUserID), CheckerUserID: id(row.CheckerUserID), CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time, DecidedAt: timePtr(row.DecidedAt)}, true, nil
}
func (t *tx) RejectChange(ctx context.Context, c domain.Change, v domain.Version, checker domain.ID, reason string, at time.Time, command settingsapp.Command) error {
	rows, e := t.q.RestoreIntegrationVersionAfterDisposition(ctx, settingssql.RestoreIntegrationVersionAfterDispositionParams{State: string(c.TargetPriorState), At: ts(at), ID: uuid(v.ID)})
	if e != nil {
		return e
	}
	if rows != 1 {
		return settingsapp.ErrConflict
	}
	rows, e = t.q.RejectIntegrationChange(ctx, settingssql.RejectIntegrationChangeParams{CheckerUserID: uuid(checker), Reason: stringPtr(reason), At: ts(at), ID: uuid(c.ID), ExpectedVersion: c.Version})
	if e != nil {
		return e
	}
	if rows != 1 {
		return settingsapp.ErrConflict
	}
	return t.insertCommand(ctx, command, "REJECT_CHANGE", checker, at)
}
func (t *tx) ExecuteChange(ctx context.Context, c domain.Change, v domain.Version, head settingsapp.Head, checker, historyID domain.ID, reason string, at time.Time, command settingsapp.Command) error {
	if !head.ActiveVersionID.IsZero() {
		rows, e := t.q.SupersedeActiveIntegrationVersion(ctx, settingssql.SupersedeActiveIntegrationVersionParams{At: ts(at), ID: uuid(head.ActiveVersionID)})
		if e != nil {
			return e
		}
		if rows != 1 {
			return settingsapp.ErrConflict
		}
	}
	rows, e := t.q.ActivateIntegrationVersion(ctx, settingssql.ActivateIntegrationVersionParams{At: ts(at), ID: uuid(v.ID)})
	if e != nil {
		return e
	}
	if rows != 1 {
		return settingsapp.ErrConflict
	}
	rows, e = t.q.AdvanceIntegrationHead(ctx, settingssql.AdvanceIntegrationHeadParams{ActiveVersionID: uuid(v.ID), At: ts(at), SettingKind: string(c.Kind), ExpectedGeneration: head.Generation, ExpectedActiveVersionID: uuid(head.ActiveVersionID)})
	if e != nil {
		return e
	}
	if rows != 1 {
		return settingsapp.ErrConflict
	}
	rows, e = t.q.ApproveIntegrationChange(ctx, settingssql.ApproveIntegrationChangeParams{CheckerUserID: uuid(checker), Reason: stringPtr(reason), At: ts(at), ID: uuid(c.ID), ExpectedVersion: c.Version})
	if e != nil {
		return e
	}
	if rows != 1 {
		return settingsapp.ErrConflict
	}
	if e = t.q.InsertIntegrationActivationHistory(ctx, settingssql.InsertIntegrationActivationHistoryParams{ID: uuid(historyID), SettingKind: string(c.Kind), VersionID: uuid(v.ID), PreviousVersionID: uuid(head.ActiveVersionID), ChangeID: uuid(c.ID), Action: string(c.Action), Generation: head.Generation + 1, ActivatedBy: uuid(checker), ActivatedAt: ts(at)}); e != nil {
		return e
	}
	return t.insertCommand(ctx, command, "APPROVE_CHANGE", checker, at)
}
func (t *tx) ExpireChange(ctx context.Context, c domain.Change, v domain.Version, at time.Time) error {
	rows, e := t.q.RestoreIntegrationVersionAfterDisposition(ctx, settingssql.RestoreIntegrationVersionAfterDispositionParams{State: string(c.TargetPriorState), At: ts(at), ID: uuid(v.ID)})
	if e != nil {
		return e
	}
	if rows != 1 {
		return settingsapp.ErrConflict
	}
	rows, e = t.q.ExpireIntegrationChange(ctx, settingssql.ExpireIntegrationChangeParams{Reason: stringPtr("approval window expired"), At: ts(at), ID: uuid(c.ID), ExpectedVersion: c.Version})
	if e != nil {
		return e
	}
	if rows != 1 {
		return settingsapp.ErrConflict
	}
	return nil
}
func (t *tx) AppendEvidence(ctx context.Context, e settingsapp.Evidence) error {
	return t.evidence.AppendPlatformIntegrationSetting(ctx, controlapp.PlatformIntegrationSettingEvidence{AuditID: controlID(e.AuditID), OutboxID: controlID(e.OutboxID), CorrelationID: controlID(e.CorrelationID), ActorID: controlID(e.ActorID), TargetID: controlID(e.TargetID), Action: e.Action, EventType: e.EventType, Kind: e.Kind, State: e.State, ResultCode: e.ResultCode, Version: e.Version, OccurredAt: e.OccurredAt})
}
func (t *tx) insertCommand(ctx context.Context, c settingsapp.Command, operation string, actor domain.ID, at time.Time) error {
	return t.q.InsertIntegrationCommand(ctx, settingssql.InsertIntegrationCommandParams{ID: uuid(c.ID), ActorUserID: uuid(actor), Operation: operation, KeyHash: c.KeyHash[:], RequestHash: c.RequestHash[:], TargetID: uuid(c.TargetID), ResponseState: stringPtr(c.ResponseState), ResponseVersion: int64Ptr(c.ResponseVersion), ResponseStatus: int32Ptr(200), At: ts(at)})
}
func uuid(v domain.ID) pgtype.UUID { return pgtype.UUID{Bytes: [16]byte(v), Valid: !v.IsZero()} }
func id(v pgtype.UUID) domain.ID {
	if !v.Valid {
		return domain.ID{}
	}
	return domain.ID(v.Bytes)
}
func hash32(v []byte) (out [32]byte)    { copy(out[:], v); return }
func ts(v time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: v.UTC(), Valid: !v.IsZero()} }
func timePtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	value := v.Time.UTC()
	return &value
}
func platformID(v domain.ID) platformapp.ID { return platformapp.ID(v) }
func settingsID(v platformapp.ID) domain.ID { return domain.ID(v) }
func controlID(v domain.ID) controlapp.ID   { return controlapp.ID(v) }
func int64Ptr(v int64) *int64               { return &v }
func int32Ptr(v int32) *int32               { return &v }
func stringPtr(v string) *string            { return &v }
