package persistence

import (
	"bidora/backend/internal/platformsettings/domain"
	q "bidora/backend/internal/platformsettings/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
)

type ActiveOpenAIStore struct{ runner settingsReadRunner }

func NewActiveOpenAIStore(r settingsReadRunner) (*ActiveOpenAIStore, error) {
	if r == nil {
		return nil, errors.New("active_openai_invalid")
	}
	return &ActiveOpenAIStore{r}, nil
}
func (s *ActiveOpenAIStore) ActiveOpenAI(ctx context.Context) (out domain.Version, err error) {
	if s == nil || ctx == nil {
		return out, errors.New("active_openai_unavailable")
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		v, e := q.New(db).GetActiveOpenAIConfiguration(c)
		if e != nil {
			return e
		}
		configuration, hash, e := domain.NewConfiguration(domain.OpenAI, nil, &domain.OpenAIConfiguration{Disabled: !v.Enabled, Model: v.Model, APIKeySecretRef: v.ApiKeySecretRef, TrustRootsSecretRef: v.TrustRootsSecretRef, TimeoutSeconds: v.TimeoutSeconds, MaxConcurrency: v.MaxConcurrency})
		if e != nil || !v.ID.Valid || hash32(v.ConfigurationHash) != hash {
			return errors.New("active_openai_corrupt")
		}
		out = domain.Version{ID: id(v.ID), Kind: domain.OpenAI, State: domain.Active, Version: v.Version, Configuration: configuration, ConfigurationHash: hash}
		return nil
	})
	if err != nil {
		return domain.Version{}, errors.New("active_openai_unavailable")
	}
	return out, nil
}
