package persistence

import (
	"context"
	"errors"
	"time"

	identitypersistence "bidora/backend/internal/identity/persistence"
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	q "bidora/backend/internal/platformsettings/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SessionPolicyStore struct{ runner informationRunner }

func NewSessionPolicyStore(r informationRunner) (*SessionPolicyStore, error) {
	if r == nil {
		return nil, app.ErrInformationInvalid
	}
	return &SessionPolicyStore{r}, nil
}
func (s *SessionPolicyStore) WithinSessionPolicy(ctx context.Context, write bool, fn func(context.Context, app.SessionPolicyTx) error) error {
	if s == nil || ctx == nil || fn == nil {
		return app.ErrInformationInvalid
	}
	run := s.runner.WithinReadOnlyTransaction
	if write {
		run = s.runner.WithinTransaction
	}
	return run(ctx, func(c context.Context, db postgres.DBTX) error {
		tx, e := NewSessionPolicyTx(db)
		if e != nil {
			return e
		}
		return fn(c, tx)
	})
}

// SessionPolicyTx is the settings owner's transaction-scoped port. Composition
// can supply it to identity without exposing settings repositories or tables.
type SessionPolicyTx struct {
	*informationTx
	revoker *identitypersistence.SessionPolicyRevokerTx
}

func NewSessionPolicyTx(db postgres.DBTX) (*SessionPolicyTx, error) {
	base, e := newInformationTx(db)
	if e != nil {
		return nil, e
	}
	revoker, e := identitypersistence.NewSessionPolicyRevokerTx(db)
	if e != nil {
		return nil, e
	}
	return &SessionPolicyTx{base, revoker}, nil
}
func (t *SessionPolicyTx) ReadSessionPolicy(ctx context.Context, lock bool) (app.SessionPolicy, error) {
	if lock {
		v, e := t.q.LockPlatformSessionPolicy(ctx)
		if e != nil {
			return app.SessionPolicy{}, e
		}
		return sessionPolicyView(v.IdleSeconds, v.AbsoluteSeconds, v.Version)
	}
	v, e := t.q.GetPlatformSessionPolicy(ctx)
	if e != nil {
		return app.SessionPolicy{}, e
	}
	return sessionPolicyView(v.IdleSeconds, v.AbsoluteSeconds, v.Version)
}
func sessionPolicyView(idle, absolute int32, version int64) (app.SessionPolicy, error) {
	p := app.SessionPolicy{IdleSeconds: idle, AbsoluteSeconds: absolute, Version: version}
	if !app.ValidSessionPolicy(p) {
		return app.SessionPolicy{}, errors.New("session_policy_row_invalid")
	}
	return p, nil
}
func (t *SessionPolicyTx) FindSessionPolicyCommand(ctx context.Context, actor domain.ID, key [32]byte) (app.InformationCommand, bool, error) {
	v, e := t.q.GetPlatformSessionPolicyCommand(ctx, q.GetPlatformSessionPolicyCommandParams{ActorID: uuid(actor), KeyHash: key[:]})
	if errors.Is(e, pgx.ErrNoRows) {
		return app.InformationCommand{}, false, nil
	}
	if e != nil {
		return app.InformationCommand{}, false, e
	}
	if len(v.RequestHash) != 32 || v.ResultVersion < 2 {
		return app.InformationCommand{}, false, app.ErrInformationUnavailable
	}
	var hash [32]byte
	copy(hash[:], v.RequestHash)
	return app.InformationCommand{RequestHash: hash, ResultVersion: v.ResultVersion}, true, nil
}
func (t *SessionPolicyTx) UpdateSessionPolicy(ctx context.Context, in app.SessionPolicySave, at time.Time) (int64, error) {
	return t.q.UpdatePlatformSessionPolicy(ctx, q.UpdatePlatformSessionPolicyParams{IdleSeconds: in.Policy.IdleSeconds, AbsoluteSeconds: in.Policy.AbsoluteSeconds, ExpectedVersion: in.Policy.Version, At: pgtype.Timestamptz{Time: at, Valid: true}, ActorID: uuid(in.Subject.ActorID)})
}
func (t *SessionPolicyTx) RevokeOrdinarySessions(ctx context.Context, at time.Time) (int64, error) {
	return t.revoker.RevokeOrdinarySessions(ctx, at)
}
func (t *SessionPolicyTx) AppendSessionPolicyEvidence(ctx context.Context, audit, outbox domain.ID, in app.SessionPolicySave, revoked int64, at time.Time, key, hash [32]byte) error {
	when := pgtype.Timestamptz{Time: at, Valid: true}
	version := in.Policy.Version + 1
	actor := uuid(in.Subject.ActorID)
	if e := t.q.AppendPlatformSessionPolicyAudit(ctx, q.AppendPlatformSessionPolicyAuditParams{ID: uuid(audit), ActorID: actor, At: when, CorrelationID: uuid(in.CorrelationID), Version: version, Revoked: revoked}); e != nil {
		return e
	}
	if e := t.q.AppendPlatformSessionPolicyOutbox(ctx, q.AppendPlatformSessionPolicyOutboxParams{ID: uuid(outbox), ActorID: actor, At: when, Version: version}); e != nil {
		return e
	}
	return t.q.InsertPlatformSessionPolicyCommand(ctx, q.InsertPlatformSessionPolicyCommandParams{ActorID: actor, KeyHash: key[:], RequestHash: hash[:], ResultVersion: version})
}
