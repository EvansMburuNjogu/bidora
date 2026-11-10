package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/email"
	sql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// PrepareInvitationEnrollment locks an identity independently of the current
// browser. Assigned, disabled, externally linked and MFA-protected accounts are
// never eligible for invitation-based credential enrollment/replacement.
func PrepareInvitationEnrollment(ctx context.Context, db postgres.DBTX, newID app.ID, address email.Address, first, last string, at time.Time) (app.PlatformIdentityCandidate, error) {
	if ctx == nil || db == nil || newID.IsZero() || address.IsZero() || !app.ValidPersonalName(first) || !app.ValidPersonalName(last) {
		return app.PlatformIdentityCandidate{}, app.ErrPlatformIdentityIneligible
	}
	var id pgtype.UUID
	var epoch int64
	var eligible bool
	err := db.QueryRow(ctx, `SELECT u.id,u.auth_epoch,
 u.account_type IS NULL AND u.disabled_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM identity.external_identities e WHERE e.user_id=u.id)
 AND NOT EXISTS(SELECT 1 FROM identity.mfa_factors f WHERE f.user_id=u.id AND f.state='ACTIVE')
 FROM identity.users u WHERE u.email_lookup=$1 FOR UPDATE OF u`, address.Lookup()).Scan(&id, &epoch, &eligible)
	if errors.Is(err, pgx.ErrNoRows) {
		if err = sql.New(db).InsertPasswordUser(ctx, sql.InsertPasswordUserParams{ID: identityID(newID), Email: address.Display(), EmailLookup: address.Lookup(), FirstName: &first, LastName: &last, CreatedAt: identityTime(at)}); err != nil {
			return app.PlatformIdentityCandidate{}, err
		}
		return app.PlatformIdentityCandidate{UserID: newID, AuthEpoch: 1, AffiliationKey: address.Lookup()}, nil
	}
	if err != nil {
		return app.PlatformIdentityCandidate{}, err
	}
	if !eligible || !id.Valid || epoch < 1 {
		return app.PlatformIdentityCandidate{}, app.ErrPlatformIdentityIneligible
	}
	return app.PlatformIdentityCandidate{UserID: app.ID(id.Bytes), AuthEpoch: epoch, AffiliationKey: address.Lookup()}, nil
}

// SetInvitationEnrollmentPassword is called only inside the validated offer's
// existing transaction. The strong mailed capability is the one-time mailbox
// enrollment proof; it is not an authenticated or MFA-assured browser session.
func SetInvitationEnrollmentPassword(ctx context.Context, db postgres.DBTX, user app.ID, epoch int64, phc string, audit, correlation app.ID, at time.Time) error {
	if ctx == nil || db == nil || user.IsZero() || audit.IsZero() || correlation.IsZero() || !strings.HasPrefix(phc, "$argon2id$v=19$") {
		return errors.New("invitation_enrollment_invalid")
	}
	affected, err := db.Exec(ctx, `UPDATE identity.users SET email_verified_at=coalesce(email_verified_at,$2),updated_at=$2 WHERE id=$1 AND auth_epoch=$3 AND account_type IS NULL AND disabled_at IS NULL`, identityID(user), at, epoch)
	if err != nil {
		return err
	}
	if affected.RowsAffected() != 1 {
		return app.ErrPlatformIdentityIneligible
	}
	affected, err = db.Exec(ctx, `INSERT INTO identity.authentication_identities(user_id,password_phc,credential_version,password_changed_at,created_at,updated_at)
 VALUES($1,$2,1,$3,$3,$3) ON CONFLICT(user_id) DO UPDATE SET password_phc=excluded.password_phc,
 credential_version=identity.authentication_identities.credential_version+1,password_changed_at=excluded.password_changed_at,updated_at=excluded.updated_at
 WHERE identity.authentication_identities.credential_version<9223372036854775807`, identityID(user), phc, at)
	if err != nil {
		return err
	}
	if affected.RowsAffected() != 1 {
		return errors.New("invitation_enrollment_invalid")
	}
	q := sql.New(db)
	if _, err = q.InvalidateEmailVerificationChallenges(ctx, sql.InvalidateEmailVerificationChallengesParams{InvalidatedAt: identityTime(at), UserID: identityID(user)}); err != nil {
		return err
	}
	if err = q.InvalidateActivePasswordRecoveryChallenges(ctx, sql.InvalidateActivePasswordRecoveryChallengesParams{At: identityTime(at), UserID: identityID(user)}); err != nil {
		return err
	}
	_, err = db.Exec(ctx, `INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
 VALUES($1,NULL,$2,'InvitationPasswordSet','user',$2,$3,'SUCCEEDED',$4,'{"proof":"PLATFORM_INVITATION","prior_challenges_invalidated":true}'::jsonb,1,$3)`, identityID(audit), identityID(user), at, identityID(correlation))
	return err
}

func (r *PasswordReader) InvitationPasswordEnrollmentAllowed(ctx context.Context, lookup string) (allowed bool, err error) {
	if ctx == nil || r == nil || r.runner == nil {
		return false, errors.New("invitation_identity_unavailable")
	}
	err = r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		return db.QueryRow(txCtx, `SELECT NOT EXISTS(SELECT 1 FROM identity.users u WHERE u.email_lookup=$1 AND (
 u.account_type IS NOT NULL OR u.disabled_at IS NOT NULL
 OR EXISTS(SELECT 1 FROM identity.external_identities e WHERE e.user_id=u.id)
 OR EXISTS(SELECT 1 FROM identity.mfa_factors f WHERE f.user_id=u.id AND f.state='ACTIVE')))`, lookup).Scan(&allowed)
	})
	return allowed, err
}
