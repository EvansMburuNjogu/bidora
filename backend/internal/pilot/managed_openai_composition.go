package pilot

import (
	"bidora/backend/internal/opportunity/extractopenai"
	"bidora/backend/internal/platformsettings/openairuntime"
	"bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"bidora/backend/internal/postgres"
)

func newManagedOpenAI(pool *postgres.Pool, config Config) (*extractopenai.Extractor, error) {
	store, e := persistence.NewActiveOpenAIStore(pool)
	if e != nil {
		return nil, e
	}
	var secrets smtpsecret.Resolver
	if len(config.SMTPVaultKey) == 32 && config.SMTPVaultKeyVersion > 0 {
		secrets, e = smtpsecret.NewVault(pool, config.SMTPVaultKey, config.SMTPVaultKeyVersion)
		if e != nil {
			return nil, e
		}
	}
	policies, e := persistence.NewRegionalPolicyStore(pool)
	if e != nil {
		return nil, e
	}
	runtime, e := openairuntime.New(store, secrets, policies)
	if e != nil {
		return nil, e
	}
	return extractopenai.NewManaged(runtime)
}
