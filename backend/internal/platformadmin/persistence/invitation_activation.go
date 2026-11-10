package persistence

import (
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/email"
	identity "bidora/backend/internal/identity/persistence"
	app "bidora/backend/internal/platformadmin/application"
	"context"
	"errors"
	"time"
)

func (tx *invitationTx) ActivationReplay(ctx context.Context, token, key, request [32]byte) (found bool, err error) {
	err = tx.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_administration.administrator_invitations i
 JOIN platform_administration.commands c ON c.actor_user_id=i.accepted_by AND c.target_id=i.id
 WHERE i.token_hash=$1 AND i.state='ACCEPTED' AND c.operation='INVITATION_ACCEPT' AND c.key_hash=$2 AND c.request_hash=$3 AND c.response_status=204)`, token[:], key[:], request[:]).Scan(&found)
	return found, err
}
func (tx *invitationTx) PrepareInvitationIdentity(ctx context.Context, record app.InvitationRecord, newID app.ID, at time.Time) (app.BootstrapIdentity, error) {
	address, err := email.Parse(record.EmailDisplay)
	if err != nil || address.Lookup() != record.EmailLookup {
		return app.BootstrapIdentity{}, app.ErrInvitationInvalid
	}
	candidate, err := identity.PrepareInvitationEnrollment(ctx, tx.db, identityapp.ID(newID), address, record.FirstName, record.LastName, at)
	if errors.Is(err, identityapp.ErrPlatformIdentityIneligible) {
		return app.BootstrapIdentity{}, app.ErrInvitationInvalid
	}
	if err != nil {
		return app.BootstrapIdentity{}, err
	}
	// Account type is not sufficient: inconsistent/historical platform profiles or
	// assignments are also protected from this unauthenticated enrollment path.
	var protected bool
	err = tx.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_administration.administrator_profiles WHERE user_id=$1)
 OR EXISTS(SELECT 1 FROM platform_administration.role_assignments WHERE user_id=$1)`, platformID(app.ID(candidate.UserID))).Scan(&protected)
	if err != nil {
		return app.BootstrapIdentity{}, err
	}
	if protected {
		return app.BootstrapIdentity{}, app.ErrInvitationInvalid
	}
	return app.BootstrapIdentity{UserID: app.ID(candidate.UserID), AuthEpoch: candidate.AuthEpoch, AffiliationKey: candidate.AffiliationKey}, nil
}
func (tx *invitationTx) SetInvitationPassword(ctx context.Context, user app.BootstrapIdentity, phc string, audit, correlation app.ID, at time.Time) error {
	return identity.SetInvitationEnrollmentPassword(ctx, tx.db, identityapp.ID(user.UserID), user.AuthEpoch, phc, identityapp.ID(audit), identityapp.ID(correlation), at)
}
