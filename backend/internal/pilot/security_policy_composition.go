package pilot

import (
	"bidora/backend/internal/identity/securitypolicy"
	persistence "bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/postgres"
	"context"
)

func currentSecurityPolicy(ctx context.Context, db postgres.DBTX) (securitypolicy.Policy, error) {
	owner, e := persistence.NewSessionPolicyTx(db)
	if e != nil {
		return securitypolicy.Policy{}, e
	}
	p, e := owner.ReadSessionPolicy(ctx, false)
	return securitypolicy.Policy{Controls: p.Security, IdleSeconds: p.IdleSeconds, AbsoluteSeconds: p.AbsoluteSeconds}, e
}
func securityPolicyResolver(pool *postgres.Pool) func(context.Context) (securitypolicy.Policy, error) {
	return func(ctx context.Context) (p securitypolicy.Policy, e error) {
		e = pool.WithinReadOnlyTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
			var err error
			p, err = currentSecurityPolicy(c, db)
			return err
		})
		return
	}
}
