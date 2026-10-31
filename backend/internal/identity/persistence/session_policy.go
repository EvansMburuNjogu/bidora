package persistence

import (
	"context"
	"errors"
	"time"

	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/postgres"
)

// SessionPolicyReader is a composition-supplied owner port bound to the caller's
// transaction. Identity never reads another module's private setting tables.
type SessionPolicyReader func(context.Context, postgres.DBTX) (int32, int32, error)

func sessionLimits(ctx context.Context, db postgres.DBTX, read SessionPolicyReader) (int32, int32, error) {
	if read == nil {
		return 1800, 43200, nil
	} // explicit legacy/test constructor profile
	idle, absolute, err := read(ctx, db)
	if err != nil {
		return 0, 0, err
	}
	if idle < 300 || idle > 1800 || absolute < 3600 || absolute > 43200 || absolute < idle {
		return 0, 0, errors.New("session_policy_invalid")
	}
	return idle, absolute, nil
}
func (r *SessionReader) EnableSessionPolicy(read SessionPolicyReader) error {
	if r == nil || read == nil {
		return errors.New("session_policy_reader_invalid")
	}
	r.policy = read
	return nil
}
func (t *SessionToucher) EnableSessionPolicy(read SessionPolicyReader) error {
	if t == nil || read == nil {
		return errors.New("session_policy_reader_invalid")
	}
	t.policy = read
	return nil
}

// SessionPolicyRevokerTx is the identity-owned transaction adapter used during
// global policy changes. It neither grants authority nor commits the transaction.
type SessionPolicyRevokerTx struct{ db postgres.DBTX }

func NewSessionPolicyRevokerTx(db postgres.DBTX) (*SessionPolicyRevokerTx, error) {
	if db == nil {
		return nil, errors.New("session_policy_revoker_invalid")
	}
	return &SessionPolicyRevokerTx{db}, nil
}
func (t *SessionPolicyRevokerTx) RevokeOrdinarySessions(ctx context.Context, at time.Time) (int64, error) {
	if t == nil || ctx == nil || at.IsZero() {
		return 0, errors.New("session_policy_revoker_invalid")
	}
	return identitysql.New(t.db).RevokeOrdinarySessionsForPolicy(ctx, identityTime(at))
}
