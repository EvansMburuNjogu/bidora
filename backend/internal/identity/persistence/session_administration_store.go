package persistence

import (
	app "bidora/backend/internal/identity/application"
	qsql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"bytes"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

type SessionAdministrationStore struct{ runner writeRunner }

func NewSessionAdministrationStore(r writeRunner) (*SessionAdministrationStore, error) {
	if r == nil {
		return nil, errors.New("session_administration_store_invalid")
	}
	return &SessionAdministrationStore{r}, nil
}
func (s *SessionAdministrationStore) RevokeOtherSession(ctx context.Context, c app.RevokeOtherSessionCommit) (out app.RevokeOtherSessionResult, err error) {
	if ctx == nil || c.CommandID.IsZero() || c.AuditID.IsZero() || c.OutboxID.IsZero() || c.TargetSessionID.IsZero() || c.CorrelationID.IsZero() || c.KeyHash == ([32]byte{}) || c.RequestHash == ([32]byte{}) || c.OccurredAt.IsZero() {
		return out, errors.New("session_administration_commit_invalid")
	}
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		n, e := q.ClaimSessionRevocationCommand(tc, qsql.ClaimSessionRevocationCommandParams{ID: identityID(c.CommandID), ActorUserID: identityID(c.Principal.UserID), KeyHash: c.KeyHash[:], RequestHash: c.RequestHash[:], CompletedAt: identityTime(c.OccurredAt)})
		if e != nil {
			return e
		}
		if n == 0 {
			old, e := q.GetSessionRevocationCommand(tc, qsql.GetSessionRevocationCommandParams{ActorUserID: identityID(c.Principal.UserID), KeyHash: c.KeyHash[:]})
			if e != nil {
				return e
			}
			if !bytes.Equal(old.RequestHash, c.RequestHash[:]) || old.ResponseStatus != 204 {
				return app.ErrSessionRevocationConflict
			}
			out.Replay = true
			return nil
		}
		target, e := q.LockOtherActiveUserSession(tc, qsql.LockOtherActiveUserSessionParams{TargetSessionID: identityID(c.TargetSessionID), UserID: identityID(c.Principal.UserID), CurrentSessionID: identityID(c.Principal.SessionID), ExpectedAuthEpoch: c.Principal.AuthEpoch, ObservedAt: identityTime(c.OccurredAt)})
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		n, e = q.RevokeOwnedSession(tc, qsql.RevokeOwnedSessionParams{RevokedAt: identityTime(c.OccurredAt), ID: target, UserID: identityID(c.Principal.UserID)})
		if e != nil {
			return e
		}
		if n != 1 {
			return errors.New("session_revoke_count_invalid")
		}
		if e = q.InsertOtherSessionRevokedAudit(tc, qsql.InsertOtherSessionRevokedAuditParams{ID: identityID(c.AuditID), ActorID: identityID(c.Principal.UserID), TargetID: target, OccurredAt: identityTime(c.OccurredAt), CorrelationID: identityID(c.CorrelationID)}); e != nil {
			return e
		}
		if e = q.InsertOtherSessionRevokedOutbox(tc, qsql.InsertOtherSessionRevokedOutboxParams{ID: identityID(c.OutboxID), ActorID: identityID(c.Principal.UserID), TargetID: target, OccurredAt: identityTime(c.OccurredAt)}); e != nil {
			return e
		}
		out.Revoked = true
		return nil
	})
	return
}
func (s *SessionAdministrationStore) RevokeAllSessions(ctx context.Context, c app.RevokeAllSessionsCommit) (out app.RevokeAllSessionsResult, err error) {
	if ctx == nil || c.AuditID.IsZero() || c.OutboxID.IsZero() || c.CorrelationID.IsZero() || c.OccurredAt.IsZero() {
		return out, errors.New("session_administration_commit_invalid")
	}
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		locked, e := q.LockCurrentSessionForRevokeAll(tc, qsql.LockCurrentSessionForRevokeAllParams{UserID: identityID(c.Principal.UserID), ExpectedAuthEpoch: c.Principal.AuthEpoch, CurrentSessionID: identityID(c.Principal.SessionID), ObservedAt: identityTime(c.OccurredAt)})
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrSessionAdministrationInvalid
		}
		if e != nil {
			return e
		}
		count, e := q.CountActiveCurrentEpochSessions(tc, qsql.CountActiveCurrentEpochSessionsParams{UserID: identityID(c.Principal.UserID), AuthEpoch: locked.AuthEpoch, ObservedAt: identityTime(c.OccurredAt)})
		if e != nil {
			return e
		}
		if count < 1 {
			return app.ErrSessionAdministrationInvalid
		}
		if count > 100 {
			return app.ErrSessionAdministrationLimit
		}
		n, e := q.AdvanceUserAuthEpochForSessionRevocation(tc, qsql.AdvanceUserAuthEpochForSessionRevocationParams{At: identityTime(c.OccurredAt), ID: identityID(c.Principal.UserID), ExpectedAuthEpoch: locked.AuthEpoch})
		if e != nil {
			return e
		}
		if n != 1 {
			return app.ErrSessionAdministrationInvalid
		}
		n, e = q.RevokeActiveCurrentEpochSessions(tc, qsql.RevokeActiveCurrentEpochSessionsParams{At: identityTime(c.OccurredAt), UserID: identityID(c.Principal.UserID), AuthEpoch: locked.AuthEpoch})
		if e != nil {
			return e
		}
		if n != count {
			return errors.New("session_revoke_all_count_invalid")
		}
		next := locked.AuthEpoch + 1
		if e = q.InsertAllSessionsRevokedAudit(tc, qsql.InsertAllSessionsRevokedAuditParams{ID: identityID(c.AuditID), ActorID: identityID(c.Principal.UserID), OccurredAt: identityTime(c.OccurredAt), CorrelationID: identityID(c.CorrelationID), AuthEpoch: next, RevokedSessionCount: int32(n)}); e != nil {
			return e
		}
		if e = q.InsertAllSessionsRevokedOutbox(tc, qsql.InsertAllSessionsRevokedOutboxParams{ID: identityID(c.OutboxID), ActorID: identityID(c.Principal.UserID), AuthEpoch: next, RevokedSessionCount: int32(n), OccurredAt: identityTime(c.OccurredAt)}); e != nil {
			return e
		}
		out = app.RevokeAllSessionsResult{AuthEpoch: next, RevokedCount: int32(n)}
		return nil
	})
	return
}
