// Package openairuntime connects governed active settings to the existing Responses adapter.
package openairuntime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"bidora/backend/internal/opportunity/extractopenai"
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"bidora/backend/internal/platformsettings/smtpsecret"
)

type ActiveReader interface {
	ActiveOpenAI(context.Context) (domain.Version, error)
}
type PolicyReader interface {
	CurrentPolicy(context.Context) (app.OperationalPolicy, error)
}

type Runtime struct {
	policies PolicyReader
	active   ActiveReader
	secrets  smtpsecret.Resolver
	running  atomic.Int32
}

func New(active ActiveReader, secrets smtpsecret.Resolver, policies PolicyReader) (*Runtime, error) {
	if active == nil || policies == nil {
		return nil, errors.New("managed_openai_invalid")
	}
	return &Runtime{active: active, secrets: secrets, policies: policies}, nil
}
func (r *Runtime) configuration(ctx context.Context) (*domain.OpenAIConfiguration, error) {
	if ctx == nil || r == nil || r.secrets == nil {
		return nil, errors.New("managed_openai_unavailable")
	}
	policy, e := r.policies.CurrentPolicy(ctx)
	if e != nil || !policy.AIEnabled || policy.UnderMaintenance(time.Now().UTC()) {
		return nil, errors.New("managed_openai_disabled_or_unavailable")
	}
	version, e := r.active.ActiveOpenAI(ctx)
	if e != nil || version.Kind != domain.OpenAI || version.Configuration.OpenAI == nil || !version.Configuration.Valid() || version.Configuration.OpenAI.Disabled || !app.SupportedOpenAIModel(version.Configuration.OpenAI.Model) {
		return nil, errors.New("managed_openai_disabled_or_unavailable")
	}
	return version.Configuration.OpenAI, nil
}
func (r *Runtime) ConfiguredModel(ctx context.Context) (string, error) {
	c, e := r.configuration(ctx)
	if e != nil {
		return "", e
	}
	return c.Model, nil
}
func (r *Runtime) Resolve(ctx context.Context) (out extractopenai.Config, release func(), err error) {
	c, e := r.configuration(ctx)
	if e != nil {
		return out, nil, e
	}
	for {
		active := r.running.Load()
		if active >= c.MaxConcurrency {
			return out, nil, errors.New("managed_openai_busy")
		}
		if r.running.CompareAndSwap(active, active+1) {
			break
		}
	}
	var once sync.Once
	done := func() { once.Do(func() { r.running.Add(-1) }) }
	err = r.secrets.WithSecret(ctx, c.APIKeySecretRef, func(key []byte) error {
		return r.secrets.WithSecret(ctx, c.TrustRootsSecretRef, func(pem []byte) error {
			roots, e := smtpsecret.ParseTrustRoots(pem)
			if e != nil {
				return e
			}
			out = extractopenai.Config{APIKey: string(key), Model: c.Model, RootCAs: roots, Timeout: time.Duration(c.TimeoutSeconds) * time.Second}
			return nil
		})
	})
	if err != nil {
		done()
		return extractopenai.Config{}, nil, errors.New("managed_openai_credentials_unavailable")
	}
	return out, done, nil
}
