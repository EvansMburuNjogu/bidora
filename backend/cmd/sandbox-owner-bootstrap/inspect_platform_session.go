package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	app "bidora/backend/internal/identity/application"
	identitystore "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/pilot"
	"bidora/backend/internal/platformadmin"
	platformstore "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

// Reads an existing unexpired session and invokes the production resolver. It
// never reads raw tokens/codes, creates sessions, changes assurance, or writes
// identity/role data. No cookie is fabricated for a browser or API request.
func inspectPlatformSession(ctx context.Context, pool *postgres.Pool, user app.ID) error {
	now := time.Now().UTC()
	p := app.SessionPrincipal{UserID: user}
	var sessionID uuid.UUID
	err := pool.WithinReadOnlyTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		return db.QueryRow(c, `SELECT s.id,s.auth_epoch,s.assurance_level,s.assurance_method,s.assured_at,s.idle_expires_at,s.absolute_expires_at FROM identity.sessions s JOIN identity.users u ON u.id=s.user_id AND u.auth_epoch=s.auth_epoch WHERE u.id=$1 AND u.account_type='PLATFORM_ADMIN' AND u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL AND s.revoked_at IS NULL AND s.idle_expires_at>$2 AND s.absolute_expires_at>$2 AND s.assurance_level='AAL2' AND s.assurance_method='EMAIL_OTP' ORDER BY s.created_at DESC LIMIT 1`, uuid.UUID(user), now).Scan(&sessionID, &p.AuthEpoch, &p.AssuranceLevel, &p.AssuranceMethod, &p.AssuredAt, &p.IdleExpiresAt, &p.AbsoluteExpiresAt)
	})
	if err != nil {
		return errors.New("platform_session_verification_no_live_verified_session")
	}
	p.SessionID = app.ID(sessionID)
	store, err := platformstore.NewStore(pool)
	if err != nil {
		return errors.New("platform_session_verification_unavailable")
	}
	module, err := platformadmin.NewModule(store, store)
	if err != nil {
		return errors.New("platform_session_verification_unavailable")
	}
	accountStore, err := identitystore.NewAccountTypeStore(pool)
	if err != nil {
		return errors.New("platform_session_verification_unavailable")
	}
	service, err := app.NewAccountTypeService(accountStore, accountStore, ids{}, clock{})
	if err != nil {
		return errors.New("platform_session_verification_unavailable")
	}
	mfa, err := identitystore.NewMFAStore(pool)
	if err != nil {
		return errors.New("platform_session_verification_unavailable")
	}
	if service.EnablePlatformAccess(pilot.NewPlatformPortalResolver(module, mfa)) != nil {
		return errors.New("platform_session_verification_unavailable")
	}
	for i := 0; i < 2; i++ {
		resolution, e := service.Resolve(ctx, p)
		if e != nil || resolution.Portal == nil || resolution.Portal.Path != "/platform" || resolution.PlatformContextGeneration == nil || len(resolution.Capabilities) == 0 {
			return errors.New("platform_session_verification_resolution_failed")
		}
		fmt.Printf("platform_session_verified pass=%d account_type=%s portal=%s assurance=%s:%s capabilities=%d context_generation=%d\n", i+1, resolution.AccountType, resolution.Portal.Path, p.AssuranceLevel, p.AssuranceMethod, len(resolution.Capabilities), *resolution.PlatformContextGeneration)
	}
	return nil
}
