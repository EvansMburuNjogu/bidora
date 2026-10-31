package extractopenai

import (
	"bidora/backend/internal/opportunity/application"
	"context"
	"errors"
)

// ConfigurationReader is composed from the settings owner. Resolve returns an
// execution-scoped concurrency release; no SQL transaction spans provider I/O.
type ConfigurationReader interface {
	ConfiguredModel(context.Context) (string, error)
	Resolve(context.Context) (Config, func(), error)
}

func NewManaged(reader ConfigurationReader) (*Extractor, error) {
	if reader == nil {
		return nil, errors.New("managed_openai_invalid")
	}
	return &Extractor{managed: reader}, nil
}
func (e *Extractor) ConfiguredModel(ctx context.Context) (string, error) {
	if e.managed != nil {
		return e.managed.ConfiguredModel(ctx)
	}
	return e.Model(), nil
}
func (e *Extractor) resolved(ctx context.Context) (*Extractor, func(), error) {
	cfg, release, err := e.managed.Resolve(ctx)
	if err != nil {
		return nil, nil, application.NewProviderExecutionError(application.ProviderConfirmedNoEffect, errors.New("managed_openai_unavailable"))
	}
	expected := application.ExpectedProviderModel(ctx)
	if expected != "" && cfg.Model != expected {
		release()
		return nil, nil, application.NewProviderExecutionError(application.ProviderConfirmedNoEffect, errors.New("managed_openai_model_changed"))
	}
	instance, err := New(cfg)
	if err != nil {
		release()
		return nil, nil, application.NewProviderExecutionError(application.ProviderConfirmedNoEffect, errors.New("managed_openai_invalid"))
	}
	return instance, func() { instance.client.CloseIdleConnections(); release() }, nil
}
