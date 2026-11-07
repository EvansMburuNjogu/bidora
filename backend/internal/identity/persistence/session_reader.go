package persistence

import (
	"context"
	"errors"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type SessionReader struct {
	security RuntimePolicyReader
	runner   readOnlyRunner
	policy   SessionPolicyReader
}

func NewSessionReader(runner readOnlyRunner) (*SessionReader, error) {
	if runner == nil {
		return nil, errors.New("session_reader_invalid")
	}
	return &SessionReader{runner: runner}, nil
}
func (r *SessionReader) GetSession(ctx context.Context, tokenHash [32]byte, observedAt time.Time) (identityapp.SessionRecord, error) {
	if ctx == nil || tokenHash == [32]byte{} || observedAt.IsZero() {
		return identityapp.SessionRecord{}, errors.New("session_lookup_invalid")
	}
	if r == nil || r.runner == nil {
		return identityapp.SessionRecord{}, errors.New("session_reader_invalid")
	}
	var result identityapp.SessionRecord
	err := r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("session_lookup_unavailable")
		}
		policy, err := readSessionSecurity(txCtx, db, r.policy, r.security)
		if err != nil {
			return err
		}
		row, err := identitysql.New(db).GetActiveSession(txCtx, identitysql.GetActiveSessionParams{PolicyIdleSeconds: policy.IdleSeconds, PolicyAbsoluteSeconds: policy.AbsoluteSeconds, AdminIdleSeconds: policy.AdminIdleSeconds, AdminAbsoluteSeconds: policy.AdminAbsoluteSeconds, ObservedAt: identityTime(observedAt), TokenHash: tokenHash[:]})
		if errors.Is(err, pgx.ErrNoRows) {
			return identityapp.ErrSessionNotFound
		}
		if err != nil {
			return err
		}
		if len(row.CsrfHash) != 32 {
			return errors.New("session_csrf_hash_invalid")
		}
		result = identityapp.SessionRecord{SessionID: identityapp.ID(row.ID.Bytes), UserID: identityapp.ID(row.UserID.Bytes), AuthEpoch: row.AuthEpoch, IdleExpiresAt: row.IdleExpiresAt.Time.UTC(), AbsoluteExpiresAt: row.AbsoluteExpiresAt.Time.UTC(), AssuranceLevel: row.AssuranceLevel, AssuranceMethod: row.AssuranceMethod, AssuredAt: row.AssuredAt.Time.UTC()}
		copy(result.CSRFHash[:], row.CsrfHash)
		return nil
	})
	if err != nil {
		return identityapp.SessionRecord{}, err
	}
	return result, nil
}

func (r *SessionReader) ListActiveUserSessions(ctx context.Context, userID identityapp.ID, observedAt time.Time) (result []identityapp.SessionInventoryItem, err error) {
	if ctx == nil || userID.IsZero() || observedAt.IsZero() || r == nil || r.runner == nil {
		return nil, errors.New("session_inventory_invalid")
	}
	err = r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		policy, err := readSessionSecurity(txCtx, db, r.policy, r.security)
		if err != nil {
			return err
		}
		rows, err := identitysql.New(db).ListCurrentUserActiveSessions(txCtx, identitysql.ListCurrentUserActiveSessionsParams{PolicyIdleSeconds: policy.IdleSeconds, PolicyAbsoluteSeconds: policy.AbsoluteSeconds, AdminIdleSeconds: policy.AdminIdleSeconds, AdminAbsoluteSeconds: policy.AdminAbsoluteSeconds, UserID: identityID(userID), ObservedAt: identityTime(observedAt)})
		if err != nil {
			return err
		}
		if len(rows) > 100 {
			return identityapp.ErrSessionInventoryLimit
		}
		result = make([]identityapp.SessionInventoryItem, len(rows))
		for i, row := range rows {
			result[i] = identityapp.SessionInventoryItem{SessionID: identityapp.ID(row.ID.Bytes), RotatedFromSessionID: identityapp.ID(row.RotatedFromSessionID.Bytes), CreatedAt: row.CreatedAt.Time.UTC(), LastSeenAt: row.LastSeenAt.Time.UTC(), IdleExpiresAt: row.IdleExpiresAt.Time.UTC(), AbsoluteExpiresAt: row.AbsoluteExpiresAt.Time.UTC()}
		}
		return nil
	})
	return result, err
}

type SessionToucher struct {
	security RuntimePolicyReader
	runner   writeRunner
	policy   SessionPolicyReader
}

func NewSessionToucher(runner writeRunner) (*SessionToucher, error) {
	if runner == nil {
		return nil, errors.New("session_toucher_invalid")
	}
	return &SessionToucher{runner: runner}, nil
}
func (t *SessionToucher) TouchSession(ctx context.Context, sessionID identityapp.ID, observedAt time.Time) error {
	if ctx == nil || sessionID.IsZero() || observedAt.IsZero() {
		return errors.New("session_touch_invalid")
	}
	if t == nil || t.runner == nil {
		return errors.New("session_toucher_invalid")
	}
	return t.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("session_touch_unavailable")
		}
		policy, err := readSessionSecurity(txCtx, db, t.policy, t.security)
		if err != nil {
			return err
		}
		rows, err := identitysql.New(db).TouchActiveSession(txCtx, identitysql.TouchActiveSessionParams{PolicyIdleSeconds: policy.IdleSeconds, PolicyAbsoluteSeconds: policy.AbsoluteSeconds, AdminIdleSeconds: policy.AdminIdleSeconds, AdminAbsoluteSeconds: policy.AdminAbsoluteSeconds, ObservedAt: identityTime(observedAt), ID: identityID(sessionID)})
		if err != nil {
			return err
		}
		if rows != 1 {
			return identityapp.ErrSessionNotFound
		}
		return nil
	})
}
