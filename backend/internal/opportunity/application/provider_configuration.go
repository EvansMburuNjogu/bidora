package application

import (
	"context"
	"errors"
)

// Optional owner port keeps legacy/test adapters intact. Real managed providers
// must resolve current enabled configuration before creating a measured job.
type ConfiguredModelReader interface {
	ConfiguredModel(context.Context) (string, error)
}

func ResolveConfiguredModel(ctx context.Context, model interface{ Model() string }) (string, error) {
	if reader, ok := model.(ConfiguredModelReader); ok {
		return reader.ConfiguredModel(ctx)
	}
	value := model.Model()
	if value == "" {
		return "", errors.New("provider_not_configured")
	}
	return value, nil
}

type expectedProviderModelKey struct{}

// A consistency fence only, never authority: providers must still recheck active
// global settings and existing workers still validate tenant permissions/credits.
func WithProviderModel(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, expectedProviderModelKey{}, model)
}
func ExpectedProviderModel(ctx context.Context) string {
	value, _ := ctx.Value(expectedProviderModelKey{}).(string)
	return value
}
