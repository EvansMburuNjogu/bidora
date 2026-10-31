package persistence

import (
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/email"
	qsql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

type MFAStore struct {
	runner             writeRunner
	platformEmailLogin bool
}

func (s *MFAStore) EnablePlatformEmailLogin() { s.platformEmailLogin = true }
func (s *MFAStore) PlatformEmailLoginRequired(ctx context.Context, user app.ID, epoch int64) (required bool, err error) {
	if !s.platformEmailLogin {
		return false, nil
	}
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		var e error
		required, e = qsql.New(db).GetPlatformEmailLoginPolicy(tc, qsql.GetPlatformEmailLoginPolicyParams{UserID: identityID(user), AuthEpoch: epoch})
		return e
	})
	return
}

func (s *MFAStore) RecordEmailLoginSMTPAcceptance(ctx context.Context, user, id app.ID, at time.Time) error {
	return s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		return qsql.New(db).InsertEmailLoginDeliveryAudit(tc, qsql.InsertEmailLoginDeliveryAuditParams{ID: identityID(app.ID(uuid.New())), ActorID: identityID(user), TransactionID: identityID(id), At: identityTime(at)})
	})
}
func NewMFAStore(r writeRunner) (*MFAStore, error) {
	if r == nil {
		return nil, errors.New("mfa_store_invalid")
	}
	return &MFAStore{runner: r}, nil
}
func (s *MFAStore) IsPlatformAdministrator(ctx context.Context, user app.ID) (result bool, err error) {
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		result, err = qsql.New(db).IsPlatformAdministrator(tc, identityID(user))
		return err
	})
	return result, err
}
func (s *MFAStore) HasActiveMFA(ctx context.Context, u app.ID) (bool, error) {
	found := false
	e := s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		_, e := qsql.New(db).GetActiveMFAFactor(tc, identityID(u))
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e == nil {
			found = true
		}
		return e
	})
	return found, e
}
func (s *MFAStore) SavePendingMFA(ctx context.Context, c app.MFAPendingCommit) error {
	return s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		if e := q.CancelPendingMFAFactor(tc, qsql.CancelPendingMFAFactorParams{At: identityTime(c.At), UserID: identityID(c.UserID)}); e != nil {
			return e
		}
		return q.InsertPendingMFAFactor(tc, qsql.InsertPendingMFAFactorParams{ID: identityID(c.FactorID), UserID: identityID(c.UserID), Ciphertext: c.Ciphertext, Nonce: c.Nonce, KeyVersion: c.KeyVersion, CreatedAt: identityTime(c.At)})
	})
}
func (s *MFAStore) GetPendingMFA(ctx context.Context, u app.ID) (out app.MFAFactorMaterial, err error) {
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		v, e := qsql.New(db).GetPendingMFAFactor(tc, identityID(u))
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrMFANotFound
		}
		if e != nil {
			return e
		}
		out = app.MFAFactorMaterial{FactorID: app.ID(v.ID.Bytes), UserID: app.ID(v.UserID.Bytes), Ciphertext: v.SecretCiphertext, Nonce: v.SecretNonce, KeyVersion: v.KeyVersion, LastCounter: v.LastAcceptedCounter, Version: v.Version, ExpiresAt: v.PendingExpiresAt.Time.UTC(), FailedAttempts: v.FailedAttempts}
		return nil
	})
	return
}
func (s *MFAStore) GetActiveMFA(ctx context.Context, u app.ID) (out app.MFAFactorMaterial, err error) {
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		v, e := qsql.New(db).GetActiveMFAFactor(tc, identityID(u))
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrMFANotFound
		}
		if e != nil {
			return e
		}
		out = app.MFAFactorMaterial{FactorID: app.ID(v.ID.Bytes), UserID: app.ID(v.UserID.Bytes), Ciphertext: v.SecretCiphertext, Nonce: v.SecretNonce, KeyVersion: v.KeyVersion, LastCounter: v.LastAcceptedCounter, Version: v.Version}
		return nil
	})
	return
}
func (s *MFAStore) MFAEnrollmentAttemptDenied(ctx context.Context, user app.ID, at time.Time) (denied bool, err error) {
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		denied, err = qsql.New(db).MFAEnrollmentAttemptDenied(tc, qsql.MFAEnrollmentAttemptDeniedParams{UserID: identityID(user), At: identityTime(at)})
		return err
	})
	return
}
func (s *MFAStore) MFALoginAttemptDenied(ctx context.Context, token, binding [32]byte, at time.Time) (denied bool, err error) {
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		denied, err = qsql.New(db).MFALoginAttemptDenied(tc, qsql.MFALoginAttemptDeniedParams{TokenHash: token[:], BrowserBindingHash: binding[:], At: identityTime(at)})
		return err
	})
	return
}
func (s *MFAStore) CheckMFARecoveryCode(ctx context.Context, factor, user app.ID, hash [32]byte) (found bool, err error) {
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		found, err = qsql.New(db).CheckMFARecoveryCode(tc, qsql.CheckMFARecoveryCodeParams{FactorID: identityID(factor), UserID: identityID(user), CodeHash: hash[:]})
		return err
	})
	return
}
func (s *MFAStore) RecordMFAEnrollmentFailure(ctx context.Context, factor, user app.ID, at time.Time) error {
	return s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		n, e := qsql.New(db).RecordMFAEnrollmentFailure(tc, qsql.RecordMFAEnrollmentFailureParams{At: identityTime(at), ID: identityID(factor), UserID: identityID(user)})
		if e != nil {
			return e
		}
		if n != 1 {
			return app.ErrMFAConflict
		}
		return nil
	})
}
func (s *MFAStore) ActivateMFA(ctx context.Context, c app.MFAActivationCommit) error {
	return s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		if _, e := q.LockMFAUserSession(tc, qsql.LockMFAUserSessionParams{UserID: identityID(c.UserID), AuthEpoch: c.ExpectedAuthEpoch, SessionID: identityID(c.CurrentSessionID), At: identityTime(c.At)}); e != nil {
			return app.ErrMFAConflict
		}
		n, e := q.ActivateMFAFactor(tc, qsql.ActivateMFAFactorParams{At: identityTime(c.At), Counter: c.Counter, ID: identityID(c.FactorID), UserID: identityID(c.UserID), ExpectedVersion: c.ExpectedFactorVersion})
		if e != nil || n != 1 {
			return app.ErrMFAConflict
		}
		if e = q.InvalidateMFARecoveryCodes(tc, qsql.InvalidateMFARecoveryCodesParams{At: identityTime(c.At), UserID: identityID(c.UserID)}); e != nil {
			return e
		}
		if len(c.RecoveryIDs) != 10 || len(c.RecoveryHashes) != 10 {
			return errors.New("mfa_recovery_count_invalid")
		}
		for i := range c.RecoveryIDs {
			if e = q.InsertMFARecoveryCode(tc, qsql.InsertMFARecoveryCodeParams{ID: identityID(c.RecoveryIDs[i]), FactorID: identityID(c.FactorID), UserID: identityID(c.UserID), CodeHash: c.RecoveryHashes[i][:], CreatedAt: identityTime(c.At)}); e != nil {
				return e
			}
		}
		n, e = q.AdvanceMFAAuthEpoch(tc, qsql.AdvanceMFAAuthEpochParams{At: identityTime(c.At), ID: identityID(c.UserID), ExpectedAuthEpoch: c.ExpectedAuthEpoch})
		if e != nil || n != 1 {
			return app.ErrMFAConflict
		}
		if e = q.RevokeMFAUserSessions(tc, qsql.RevokeMFAUserSessionsParams{At: identityTime(c.At), UserID: identityID(c.UserID)}); e != nil {
			return e
		}
		next := c.ExpectedAuthEpoch + 1
		if e = q.InsertSession(tc, qsql.InsertSessionParams{ID: identityID(c.SessionID), UserID: identityID(c.UserID), TokenHash: c.SessionHash[:], CsrfHash: c.CSRFHash[:], AuthEpoch: next, RotatedFromSessionID: identityID(c.CurrentSessionID), CreatedAt: identityTime(c.At), IdleExpiresAt: identityTime(c.IdleExpiresAt), AbsoluteExpiresAt: identityTime(c.AbsoluteExpiresAt), AssuranceLevel: "AAL2", AssuredAt: identityTime(c.At), AssuranceMethod: "TOTP", IdleTimeoutSeconds: int32(c.IdleExpiresAt.Sub(c.At).Seconds()), AbsoluteTimeoutSeconds: int32(c.AbsoluteExpiresAt.Sub(c.At).Seconds())}); e != nil {
			return e
		}
		if e = q.InsertMFAAudit(tc, qsql.InsertMFAAuditParams{ID: identityID(c.AuditID), ActorID: identityID(c.UserID), Action: "MFAFactorActivated", FactorID: identityID(c.FactorID), OccurredAt: identityTime(c.At), CorrelationID: identityID(c.CorrelationID), Method: "TOTP"}); e != nil {
			return e
		}
		return q.InsertMFAOutbox(tc, qsql.InsertMFAOutboxParams{ID: identityID(c.OutboxID), ActorID: identityID(c.UserID), FactorID: identityID(c.FactorID), Version: c.ExpectedFactorVersion + 1, EventType: "identity.mfa.factor_activated.v1", Method: "TOTP", OccurredAt: identityTime(c.At)})
	})
}
func (s *MFAStore) RegenerateMFARecoveryCodes(ctx context.Context, c app.MFARecoveryRegenerationCommit) error {
	return s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		if _, e := q.LockMFAUserSession(tc, qsql.LockMFAUserSessionParams{UserID: identityID(c.UserID), AuthEpoch: c.AuthEpoch, SessionID: identityID(c.SessionID), At: identityTime(c.At)}); e != nil {
			return app.ErrMFAConflict
		}
		n, e := q.TouchActiveMFAFactor(tc, qsql.TouchActiveMFAFactorParams{ID: identityID(c.FactorID), UserID: identityID(c.UserID), ExpectedVersion: c.ExpectedFactorVersion})
		if e != nil || n != 1 {
			return app.ErrMFAConflict
		}
		if e = q.InvalidateMFARecoveryCodes(tc, qsql.InvalidateMFARecoveryCodesParams{At: identityTime(c.At), UserID: identityID(c.UserID)}); e != nil {
			return e
		}
		if len(c.RecoveryIDs) != 10 || len(c.RecoveryHashes) != 10 {
			return errors.New("mfa_recovery_count_invalid")
		}
		for i := range c.RecoveryIDs {
			if e = q.InsertMFARecoveryCode(tc, qsql.InsertMFARecoveryCodeParams{ID: identityID(c.RecoveryIDs[i]), FactorID: identityID(c.FactorID), UserID: identityID(c.UserID), CodeHash: c.RecoveryHashes[i][:], CreatedAt: identityTime(c.At)}); e != nil {
				return e
			}
		}
		if e = q.InsertMFAAudit(tc, qsql.InsertMFAAuditParams{ID: identityID(c.AuditID), ActorID: identityID(c.UserID), Action: "MFARecoveryCodesRegenerated", FactorID: identityID(c.FactorID), OccurredAt: identityTime(c.At), CorrelationID: identityID(c.CorrelationID), Method: "RECOVERY_CODE"}); e != nil {
			return e
		}
		return q.InsertMFAOutbox(tc, qsql.InsertMFAOutboxParams{ID: identityID(c.OutboxID), ActorID: identityID(c.UserID), FactorID: identityID(c.FactorID), Version: c.ExpectedFactorVersion + 1, EventType: "identity.mfa.recovery_codes_regenerated.v1", Method: "RECOVERY_CODE", OccurredAt: identityTime(c.At)})
	})
}
func (s *MFAStore) ReplaceMFAFactor(ctx context.Context, c app.MFAReplacementCommit) error {
	return s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		if _, e := q.LockMFAUserSession(tc, qsql.LockMFAUserSessionParams{UserID: identityID(c.UserID), AuthEpoch: c.ExpectedAuthEpoch, SessionID: identityID(c.CurrentSessionID), At: identityTime(c.At)}); e != nil {
			return app.ErrMFAConflict
		}
		n, e := q.ReplaceActiveMFAFactor(tc, qsql.ReplaceActiveMFAFactorParams{At: identityTime(c.At), ID: identityID(c.PreviousFactorID), UserID: identityID(c.UserID), ExpectedVersion: c.ExpectedPreviousVersion})
		if e != nil || n != 1 {
			return app.ErrMFAConflict
		}
		n, e = q.ActivateMFAFactor(tc, qsql.ActivateMFAFactorParams{At: identityTime(c.At), Counter: c.Counter, ID: identityID(c.FactorID), UserID: identityID(c.UserID), ExpectedVersion: c.ExpectedFactorVersion})
		if e != nil || n != 1 {
			return app.ErrMFAConflict
		}
		if e = q.InvalidateMFARecoveryCodes(tc, qsql.InvalidateMFARecoveryCodesParams{At: identityTime(c.At), UserID: identityID(c.UserID)}); e != nil {
			return e
		}
		if len(c.RecoveryIDs) != 10 || len(c.RecoveryHashes) != 10 {
			return errors.New("mfa_recovery_count_invalid")
		}
		for i := range c.RecoveryIDs {
			if e = q.InsertMFARecoveryCode(tc, qsql.InsertMFARecoveryCodeParams{ID: identityID(c.RecoveryIDs[i]), FactorID: identityID(c.FactorID), UserID: identityID(c.UserID), CodeHash: c.RecoveryHashes[i][:], CreatedAt: identityTime(c.At)}); e != nil {
				return e
			}
		}
		n, e = q.AdvanceMFAAuthEpoch(tc, qsql.AdvanceMFAAuthEpochParams{At: identityTime(c.At), ID: identityID(c.UserID), ExpectedAuthEpoch: c.ExpectedAuthEpoch})
		if e != nil || n != 1 {
			return app.ErrMFAConflict
		}
		if e = q.RevokeMFAUserSessions(tc, qsql.RevokeMFAUserSessionsParams{At: identityTime(c.At), UserID: identityID(c.UserID)}); e != nil {
			return e
		}
		if e = q.InsertSession(tc, qsql.InsertSessionParams{ID: identityID(c.SessionID), UserID: identityID(c.UserID), TokenHash: c.SessionHash[:], CsrfHash: c.CSRFHash[:], AuthEpoch: c.ExpectedAuthEpoch + 1, RotatedFromSessionID: identityID(c.CurrentSessionID), CreatedAt: identityTime(c.At), IdleExpiresAt: identityTime(c.IdleExpiresAt), AbsoluteExpiresAt: identityTime(c.AbsoluteExpiresAt), AssuranceLevel: "AAL2", AssuredAt: identityTime(c.At), AssuranceMethod: "TOTP", IdleTimeoutSeconds: int32(c.IdleExpiresAt.Sub(c.At).Seconds()), AbsoluteTimeoutSeconds: int32(c.AbsoluteExpiresAt.Sub(c.At).Seconds())}); e != nil {
			return e
		}
		if e = q.InsertMFAAudit(tc, qsql.InsertMFAAuditParams{ID: identityID(c.AuditID), ActorID: identityID(c.UserID), Action: "MFAFactorReplaced", FactorID: identityID(c.FactorID), OccurredAt: identityTime(c.At), CorrelationID: identityID(c.CorrelationID), Method: "TOTP"}); e != nil {
			return e
		}
		return q.InsertMFAOutbox(tc, qsql.InsertMFAOutboxParams{ID: identityID(c.OutboxID), ActorID: identityID(c.UserID), FactorID: identityID(c.FactorID), Version: c.ExpectedFactorVersion + 1, EventType: "identity.mfa.factor_replaced.v1", Method: "TOTP", OccurredAt: identityTime(c.At)})
	})
}
func (s *MFAStore) CommitMFAStepUp(ctx context.Context, c app.MFAStepUpCommit) error {
	return s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		if _, e := q.LockMFAUserSession(tc, qsql.LockMFAUserSessionParams{UserID: identityID(c.UserID), AuthEpoch: c.AuthEpoch, SessionID: identityID(c.CurrentSessionID), At: identityTime(c.At)}); e != nil {
			return app.ErrMFAConflict
		}
		if c.Method == "TOTP" {
			n, e := q.AdvanceMFAFactorCounter(tc, qsql.AdvanceMFAFactorCounterParams{Counter: c.Counter, ID: identityID(c.FactorID), UserID: identityID(c.UserID), ExpectedVersion: c.ExpectedFactorVersion})
			if e != nil || n != 1 {
				return app.ErrMFAConflict
			}
		} else {
			if c.RecoveryHash == nil {
				return app.ErrMFAConflict
			}
			n, e := q.ConsumeMFARecoveryCode(tc, qsql.ConsumeMFARecoveryCodeParams{At: identityTime(c.At), FactorID: identityID(c.FactorID), UserID: identityID(c.UserID), CodeHash: c.RecoveryHash[:]})
			if e != nil || n != 1 {
				return app.ErrMFAConflict
			}
		}
		n, e := q.RevokeMFACurrentSession(tc, qsql.RevokeMFACurrentSessionParams{At: identityTime(c.At), ID: identityID(c.CurrentSessionID), UserID: identityID(c.UserID)})
		if e != nil || n != 1 {
			return app.ErrMFAConflict
		}
		if e = q.InsertSession(tc, qsql.InsertSessionParams{ID: identityID(c.SessionID), UserID: identityID(c.UserID), TokenHash: c.SessionHash[:], CsrfHash: c.CSRFHash[:], AuthEpoch: c.AuthEpoch, RotatedFromSessionID: identityID(c.CurrentSessionID), CreatedAt: identityTime(c.At), IdleExpiresAt: identityTime(c.IdleExpiresAt), AbsoluteExpiresAt: identityTime(c.AbsoluteExpiresAt), AssuranceLevel: "AAL2", AssuredAt: identityTime(c.At), AssuranceMethod: c.Method, IdleTimeoutSeconds: int32(c.IdleExpiresAt.Sub(c.At).Seconds()), AbsoluteTimeoutSeconds: int32(c.AbsoluteExpiresAt.Sub(c.At).Seconds())}); e != nil {
			return e
		}
		if e = q.InsertMFAAudit(tc, qsql.InsertMFAAuditParams{ID: identityID(c.AuditID), ActorID: identityID(c.UserID), Action: "MFAStepUpCompleted", FactorID: identityID(c.FactorID), OccurredAt: identityTime(c.At), CorrelationID: identityID(c.CorrelationID), Method: c.Method}); e != nil {
			return e
		}
		return q.InsertMFAOutbox(tc, qsql.InsertMFAOutboxParams{ID: identityID(c.OutboxID), ActorID: identityID(c.UserID), FactorID: identityID(c.FactorID), Version: c.ExpectedFactorVersion + 1, EventType: "identity.mfa.step_up_completed.v1", Method: c.Method, OccurredAt: identityTime(c.At)})
	})
}

func (s *MFAStore) BeginEmailLogin(ctx context.Context, u app.ID, epoch int64, method app.AuthenticationMethod, token, binding, codeMAC [32]byte, id app.ID, at time.Time) (recipient email.Address, err error) {
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		if e := q.CancelMFALoginTransactions(tc, qsql.CancelMFALoginTransactionsParams{At: identityTime(at), UserID: identityID(u)}); e != nil {
			return e
		}
		if e := q.InsertEmailLoginTransaction(tc, qsql.InsertEmailLoginTransactionParams{ID: identityID(id), UserID: identityID(u), TokenHash: token[:], BrowserBindingHash: binding[:], PrimaryMethod: string(method), AuthEpoch: epoch, At: identityTime(at), EmailCodeMac: codeMAC[:]}); e != nil {
			return e
		}
		row, e := q.GetMFALoginChallenge(tc, qsql.GetMFALoginChallengeParams{TokenHash: token[:], BrowserBindingHash: binding[:], At: identityTime(at)})
		if e != nil {
			return e
		}
		recipient, e = email.Parse(row.Email)
		return e
	})
	return recipient, err
}
func (s *MFAStore) BeginMFALogin(ctx context.Context, u app.ID, epoch int64, method app.AuthenticationMethod, token, binding [32]byte, id app.ID, at time.Time) error {
	return s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		if e := q.CancelMFALoginTransactions(tc, qsql.CancelMFALoginTransactionsParams{At: identityTime(at), UserID: identityID(u)}); e != nil {
			return e
		}
		return q.InsertTOTPLoginTransaction(tc, qsql.InsertTOTPLoginTransactionParams{ID: identityID(id), UserID: identityID(u), TokenHash: token[:], BrowserBindingHash: binding[:], PrimaryMethod: string(method), AuthEpoch: epoch, At: identityTime(at)})
	})
}
func (s *MFAStore) GetMFALoginChallenge(ctx context.Context, token, binding [32]byte, at time.Time) (out app.MFALoginChallenge, err error) {
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		v, e := qsql.New(db).GetMFALoginChallenge(tc, qsql.GetMFALoginChallengeParams{TokenHash: token[:], BrowserBindingHash: binding[:], At: identityTime(at)})
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrMFANotFound
		}
		if e != nil {
			return e
		}
		out = app.MFALoginChallenge{TransactionID: app.ID(v.TransactionID.Bytes), UserID: app.ID(v.UserID.Bytes), Purpose: v.Purpose, AuthEpoch: v.AuthEpoch, PrimaryMethod: v.PrimaryMethod, Attempts: v.Attempts, ExpiresAt: v.ExpiresAt.Time.UTC()}
		if v.Purpose == "EMAIL_LOGIN" {
			if len(v.EmailCodeMac) != 32 {
				return app.ErrMFANotFound
			}
			recipient, parseErr := email.Parse(v.Email)
			if parseErr != nil {
				return parseErr
			}
			copy(out.CodeMAC[:], v.EmailCodeMac)
			out.Recipient = recipient
			return nil
		}
		if v.Purpose != "MFA_LOGIN" || !v.FactorID.Valid || v.KeyVersion == nil || v.LastAcceptedCounter == nil || v.FactorVersion == nil {
			return app.ErrMFANotFound
		}
		out.Factor = app.MFAFactorMaterial{FactorID: app.ID(v.FactorID.Bytes), UserID: app.ID(v.UserID.Bytes), Ciphertext: v.SecretCiphertext, Nonce: v.SecretNonce, KeyVersion: *v.KeyVersion, LastCounter: *v.LastAcceptedCounter, Version: *v.FactorVersion}
		return nil
	})
	return
}
func (s *MFAStore) CancelMFALogin(ctx context.Context, token, binding [32]byte, at time.Time) error {
	return s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		return qsql.New(db).CancelEmailLoginTransaction(tc, qsql.CancelEmailLoginTransactionParams{At: identityTime(at), TokenHash: token[:], BrowserBindingHash: binding[:]})
	})
}
func (s *MFAStore) RecordMFALoginFailure(ctx context.Context, id app.ID, at time.Time) error {
	return s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		n, e := qsql.New(db).RecordMFALoginFailure(tc, qsql.RecordMFALoginFailureParams{At: identityTime(at), ID: identityID(id)})
		if e != nil {
			return e
		}
		if n != 1 {
			return app.ErrMFAConflict
		}
		return nil
	})
}
func (s *MFAStore) CommitMFALogin(ctx context.Context, c app.MFALoginCommit) error {
	return s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		if c.Method != "EMAIL_OTP" && c.Method != "TOTP" && c.Method != "RECOVERY_CODE" {
			return app.ErrMFAConflict
		}
		level := "AAL1"
		if c.Method == "EMAIL_OTP" {
			policy, err := q.LockEmailLoginSessionPolicy(tc, qsql.LockEmailLoginSessionPolicyParams{UserID: identityID(c.UserID), AuthEpoch: c.AuthEpoch, TransactionID: identityID(c.TransactionID), At: identityTime(c.At)})
			if err != nil || policy.PrimaryMethod != c.PrimaryMethod {
				return app.ErrMFAConflict
			}
			if policy.PlatformAdmin {
				if !s.platformEmailLogin || policy.PrimaryMethod != string(app.AuthenticationMethodPassword) {
					return app.ErrMFAConflict
				}
				level = "AAL2"
				if err = q.RevokePreOTPAdministratorSessions(tc, qsql.RevokePreOTPAdministratorSessionsParams{UserID: identityID(c.UserID), At: identityTime(c.At)}); err != nil {
					return err
				}
			}
		}
		n, e := q.ConsumeMFALoginTransaction(tc, qsql.ConsumeMFALoginTransactionParams{At: identityTime(c.At), ID: identityID(c.TransactionID)})
		if e != nil || n != 1 {
			return app.ErrMFAConflict
		}
		if c.Method == "EMAIL_OTP" {
			if e = q.InsertSession(tc, qsql.InsertSessionParams{ID: identityID(c.SessionID), UserID: identityID(c.UserID), TokenHash: c.SessionHash[:], CsrfHash: c.CSRFHash[:], AuthEpoch: c.AuthEpoch, CreatedAt: identityTime(c.At), IdleExpiresAt: identityTime(c.IdleExpiresAt), AbsoluteExpiresAt: identityTime(c.AbsoluteExpiresAt), AssuranceLevel: level, AssuredAt: identityTime(c.At), AssuranceMethod: "EMAIL_OTP", IdleTimeoutSeconds: int32(c.IdleExpiresAt.Sub(c.At).Seconds()), AbsoluteTimeoutSeconds: int32(c.AbsoluteExpiresAt.Sub(c.At).Seconds())}); e != nil {
				return e
			}
			if e = q.InsertEmailLoginAudit(tc, qsql.InsertEmailLoginAuditParams{AssuranceLevel: level, ID: identityID(c.AuditID), ActorID: identityID(c.UserID), SessionID: identityID(c.SessionID), OccurredAt: identityTime(c.At), CorrelationID: identityID(c.CorrelationID)}); e != nil {
				return e
			}
			return q.InsertEmailLoginOutbox(tc, qsql.InsertEmailLoginOutboxParams{AssuranceLevel: level, ID: identityID(c.OutboxID), ActorID: identityID(c.UserID), TransactionID: identityID(c.TransactionID), OccurredAt: identityTime(c.At)})
		}
		if c.Method == "TOTP" {
			n, e = q.AdvanceMFAFactorCounter(tc, qsql.AdvanceMFAFactorCounterParams{Counter: c.Counter, ID: identityID(c.FactorID), UserID: identityID(c.UserID), ExpectedVersion: c.ExpectedFactorVersion})
		} else {
			if c.RecoveryHash == nil {
				return app.ErrMFAConflict
			}
			n, e = q.ConsumeMFARecoveryCode(tc, qsql.ConsumeMFARecoveryCodeParams{At: identityTime(c.At), FactorID: identityID(c.FactorID), UserID: identityID(c.UserID), CodeHash: c.RecoveryHash[:]})
		}
		if e != nil || n != 1 {
			return app.ErrMFAConflict
		}
		if e = q.InsertSession(tc, qsql.InsertSessionParams{ID: identityID(c.SessionID), UserID: identityID(c.UserID), TokenHash: c.SessionHash[:], CsrfHash: c.CSRFHash[:], AuthEpoch: c.AuthEpoch, CreatedAt: identityTime(c.At), IdleExpiresAt: identityTime(c.IdleExpiresAt), AbsoluteExpiresAt: identityTime(c.AbsoluteExpiresAt), AssuranceLevel: "AAL2", AssuredAt: identityTime(c.At), AssuranceMethod: c.Method, IdleTimeoutSeconds: int32(c.IdleExpiresAt.Sub(c.At).Seconds()), AbsoluteTimeoutSeconds: int32(c.AbsoluteExpiresAt.Sub(c.At).Seconds())}); e != nil {
			return e
		}
		if e = q.InsertMFAAudit(tc, qsql.InsertMFAAuditParams{ID: identityID(c.AuditID), ActorID: identityID(c.UserID), Action: "MFALoginCompleted", FactorID: identityID(c.FactorID), OccurredAt: identityTime(c.At), CorrelationID: identityID(c.CorrelationID), Method: c.Method}); e != nil {
			return e
		}
		return q.InsertMFAOutbox(tc, qsql.InsertMFAOutboxParams{ID: identityID(c.OutboxID), ActorID: identityID(c.UserID), FactorID: identityID(c.FactorID), Version: c.ExpectedFactorVersion + 1, EventType: "identity.mfa.login_completed.v1", Method: c.Method, OccurredAt: identityTime(c.At)})
	})
}
