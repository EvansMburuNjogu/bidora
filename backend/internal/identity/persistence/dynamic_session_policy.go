package persistence

import (
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/identity/securitypolicy"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

type RuntimePolicyReader func(context.Context, postgres.DBTX) (securitypolicy.Policy, error)

func (r *SessionReader) EnableSecurityPolicy(read RuntimePolicyReader) error {
	if r == nil || read == nil {
		return errors.New("session_policy_reader_invalid")
	}
	r.security = read
	return nil
}
func (t *SessionToucher) EnableSecurityPolicy(read RuntimePolicyReader) error {
	if t == nil || read == nil {
		return errors.New("session_policy_reader_invalid")
	}
	t.security = read
	return nil
}
func readSessionSecurity(ctx context.Context, db postgres.DBTX, legacy SessionPolicyReader, read RuntimePolicyReader) (securitypolicy.Policy, error) {
	if read != nil {
		p, e := read(ctx, db)
		if e != nil {
			return securitypolicy.Policy{}, e
		}
		if !securitypolicy.Valid(p) {
			return securitypolicy.Policy{}, errors.New("session_policy_invalid")
		}
		return p, nil
	}
	p := securitypolicy.Default()
	idle, absolute, e := sessionLimits(ctx, db, legacy)
	p.IdleSeconds = idle
	p.AbsoluteSeconds = absolute
	return p, e
}

// Apply changed limits to every stored session without mass sign-out. Only
// expired/revoked identities or excess concurrent sessions are invalidated;
// persisted tightening prevents later relaxation from resurrecting a session.
func (t *SessionPolicyRevokerTx) ApplySessionPolicy(ctx context.Context, current [16]byte, p securitypolicy.Policy, at time.Time) (int64, error) {
	if t == nil || ctx == nil || current == ([16]byte{}) || !securitypolicy.Valid(p) || at.IsZero() {
		return 0, errors.New("session_policy_revoker_invalid")
	}
	q := identitysql.New(t.db)
	id := pgtype.UUID{Bytes: current, Valid: true}
	_, e := q.TightenSessionsForPolicy(ctx, identitysql.TightenSessionsForPolicyParams{IdleSeconds: p.IdleSeconds, AbsoluteSeconds: p.AbsoluteSeconds, AdminIdleSeconds: p.AdminIdleSeconds, AdminAbsoluteSeconds: p.AdminAbsoluteSeconds})
	if e != nil {
		return 0, e
	}
	expired, e := q.RevokeExpiredSessionsForPolicy(ctx, identityTime(at))
	if e != nil {
		return 0, e
	}
	excess, e := q.RevokeExcessSessionsForPolicy(ctx, identitysql.RevokeExcessSessionsForPolicyParams{At: identityTime(at), CurrentSessionID: id, AllowMultipleSessions: p.AllowMultipleSessions})
	return expired + excess, e
}
