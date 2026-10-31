package pilot

import (
	"bidora/backend/internal/httpapi"
	app "bidora/backend/internal/platformsettings/application"
	persistence "bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/postgres"
	"context"
)

func buildPlatformSessionPolicyHandler(pool *postgres.Pool) (*httpapi.PlatformSessionPolicyHandler, error) {
	store, e := persistence.NewSessionPolicyStore(pool)
	if e != nil {
		return nil, e
	}
	service, e := app.NewSessionPolicyService(store, platformSettingIDs{}, systemClock{})
	if e != nil {
		return nil, e
	}
	return httpapi.NewPlatformSessionPolicyHandler(service)
}

// Bind the settings owner's read port to the same transaction used for session
// validation/touch. Missing/invalid settings fail closed; no cached policy.
func currentSessionLimits(ctx context.Context, db postgres.DBTX) (int32, int32, error) {
	owner, e := persistence.NewSessionPolicyTx(db)
	if e != nil {
		return 0, 0, e
	}
	p, e := owner.ReadSessionPolicy(ctx, false)
	return p.IdleSeconds, p.AbsoluteSeconds, e
}
