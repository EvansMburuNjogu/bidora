package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	q "bidora/backend/internal/platformsettings/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type GoogleStore struct{ runner informationRunner }

func NewGoogleStore(r informationRunner) (*GoogleStore, error) {
	if r == nil {
		return nil, app.ErrInformationInvalid
	}
	return &GoogleStore{r}, nil
}
func (s *GoogleStore) WithinGoogle(ctx context.Context, write bool, fn func(context.Context, app.GoogleTx) error) error {
	if s == nil || ctx == nil || fn == nil {
		return app.ErrInformationInvalid
	}
	run := s.runner.WithinReadOnlyTransaction
	if write {
		run = s.runner.WithinTransaction
	}
	return run(ctx, func(c context.Context, db postgres.DBTX) error {
		tx, e := NewGoogleSettingsTx(db)
		if e != nil {
			return e
		}
		return fn(c, tx)
	})
}

type GoogleSettingsTx struct{ *informationTx }

func NewGoogleSettingsTx(db postgres.DBTX) (*GoogleSettingsTx, error) {
	tx, e := newInformationTx(db)
	if e != nil {
		return nil, e
	}
	return &GoogleSettingsTx{tx}, nil
}
func (t *GoogleSettingsTx) LockGoogle(ctx context.Context) error {
	if e := t.authority.LockConfigurationWriter(ctx); e != nil {
		return e
	}
	_, e := t.q.LockGoogleHead(ctx)
	return e
}
func (t *GoogleSettingsTx) LockGoogleForAuthentication(ctx context.Context) error {
	_, e := t.q.LockGoogleHeadShared(ctx)
	return e
}
func (t *GoogleSettingsTx) PasswordFallbackAvailable(ctx context.Context) (bool, error) {
	return t.authority.PasswordFallbackAvailable(ctx)
}
func (t *GoogleSettingsTx) ReadGoogle(ctx context.Context) (app.GoogleStored, error) {
	v, e := t.q.GetGoogleConfiguration(ctx)
	if e != nil {
		return app.GoogleStored{}, e
	}
	out := googleStored(v)
	status, e := t.q.LatestGoogleTest(ctx, uuid(out.Configuration.ID))
	if e == nil {
		out.Configuration.ConnectionStatus = status
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return app.GoogleStored{}, e
	}
	return out, nil
}
func googleStored(v q.PlatformSettingsGoogleVersion) app.GoogleStored {
	var keyVersion int32
	if v.KeyVersion != nil {
		keyVersion = *v.KeyVersion
	}
	return app.GoogleStored{Configuration: app.GoogleConfiguration{ID: domain.ID(v.ID.Bytes), Version: v.Version, Enabled: v.Enabled, AllowSignup: v.AllowSignup, ClientID: v.ClientID, AllowedDomains: v.AllowedDomains, CreatedAt: v.CreatedAt.Time}, Secret: app.GoogleCipher{ID: domain.ID(v.SecretID.Bytes), KeyVersion: keyVersion, Nonce: v.Nonce, Ciphertext: v.Ciphertext}}
}
func (t *GoogleSettingsTx) GoogleHistory(ctx context.Context) ([]app.GoogleStored, error) {
	rows, e := t.q.ListGoogleVersions(ctx)
	if e != nil {
		return nil, e
	}
	out := make([]app.GoogleStored, 0, len(rows))
	for _, v := range rows {
		stored := googleStored(v)
		status, err := t.q.LatestGoogleTest(ctx, uuid(stored.Configuration.ID))
		if err == nil {
			stored.Configuration.ConnectionStatus = status
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		out = append(out, stored)
	}
	return out, nil
}
func (t *GoogleSettingsTx) FindGoogleCommand(ctx context.Context, actor domain.ID, operation string, key [32]byte) (app.GoogleReceipt, bool, error) {
	v, e := t.q.GetGoogleCommand(ctx, q.GetGoogleCommandParams{ActorID: uuid(actor), Operation: operation, KeyHash: key[:]})
	if errors.Is(e, pgx.ErrNoRows) {
		return app.GoogleReceipt{}, false, nil
	}
	if e != nil {
		return app.GoogleReceipt{}, false, e
	}
	if len(v.RequestHash) != 32 {
		return app.GoogleReceipt{}, false, app.ErrInformationUnavailable
	}
	var hash [32]byte
	copy(hash[:], v.RequestHash)
	return app.GoogleReceipt{Hash: hash, Version: v.ResultVersion, Target: domain.ID(v.VersionID.Bytes), State: v.State, Result: v.ResultCode}, true, nil
}
func (t *GoogleSettingsTx) WriteGoogle(ctx context.Context, v app.GoogleStored, actor domain.ID, at time.Time) error {
	secretID := uuid(v.Secret.ID)
	var keyVersion *int32
	if !v.Secret.ID.IsZero() {
		keyVersion = &v.Secret.KeyVersion
	}
	domains := v.Configuration.AllowedDomains
	if domains == nil {
		domains = []string{}
	}
	if e := t.q.InsertGoogleVersion(ctx, q.InsertGoogleVersionParams{ID: uuid(v.Configuration.ID), Version: v.Configuration.Version, Enabled: v.Configuration.Enabled, AllowSignup: v.Configuration.AllowSignup, ClientID: v.Configuration.ClientID, AllowedDomains: domains, SecretID: secretID, KeyVersion: keyVersion, Nonce: v.Secret.Nonce, Ciphertext: v.Secret.Ciphertext, At: pgtype.Timestamptz{Time: at, Valid: true}, ActorID: uuid(actor)}); e != nil {
		return e
	}
	n, e := t.q.AdvanceGoogleHead(ctx, q.AdvanceGoogleHeadParams{VersionID: uuid(v.Configuration.ID), ExpectedVersion: v.Configuration.Version - 1})
	if e != nil {
		return e
	}
	if n != 1 {
		return app.ErrInformationConflict
	}
	return nil
}
func (t *GoogleSettingsTx) RecordGoogleCommand(ctx context.Context, actor domain.ID, operation string, key [32]byte, v app.GoogleReceipt, _ time.Time) error {
	return t.q.InsertGoogleCommand(ctx, q.InsertGoogleCommandParams{ActorID: uuid(actor), Operation: operation, KeyHash: key[:], RequestHash: v.Hash[:], VersionID: uuid(v.Target), ResultVersion: v.Version, State: v.State, ResultCode: v.Result})
}
func (t *GoogleSettingsTx) FinishGoogleTest(ctx context.Context, actor domain.ID, key [32]byte, result string) error {
	n, e := t.q.CompleteGoogleTestCommand(ctx, q.CompleteGoogleTestCommandParams{ActorID: uuid(actor), KeyHash: key[:], ResultCode: result})
	if e != nil {
		return e
	}
	if n != 1 {
		return app.ErrInformationConflict
	}
	return nil
}
func (t *GoogleSettingsTx) GoogleTestCount(ctx context.Context, actor domain.ID, after time.Time) (int64, error) {
	return t.q.CountRecentGoogleTests(ctx, q.CountRecentGoogleTestsParams{ActorID: uuid(actor), After: pgtype.Timestamptz{Time: after, Valid: true}})
}
func (t *GoogleSettingsTx) AppendGoogleEvidence(ctx context.Context, audit, outbox, actor, correlation domain.ID, action string, v app.GoogleStored, result string, at time.Time) error {
	when := pgtype.Timestamptz{Time: at, Valid: true}
	if action == "PlatformGoogleConnectionTested" {
		if e := t.q.InsertGoogleTest(ctx, q.InsertGoogleTestParams{ID: uuid(audit), VersionID: uuid(v.Configuration.ID), ActorID: uuid(actor), Status: result, At: when}); e != nil {
			return e
		}
	}
	changes := v.Changes
	if changes == nil {
		changes = []app.FieldChange{}
	}
	raw, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	if e := t.q.AppendGoogleSettingsAudit(ctx, q.AppendGoogleSettingsAuditParams{ID: uuid(audit), ActorID: uuid(actor), TargetID: uuid(v.Configuration.ID), CorrelationID: uuid(correlation), Action: action, Version: v.Configuration.Version, Result: result, At: when, Changes: raw}); e != nil {
		return e
	}
	return t.q.AppendGoogleSettingsOutbox(ctx, q.AppendGoogleSettingsOutboxParams{ID: uuid(outbox), ActorID: uuid(actor), TargetID: uuid(v.Configuration.ID), Action: action, Version: v.Configuration.Version, At: when})
}
