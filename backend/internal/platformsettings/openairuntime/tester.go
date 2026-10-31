package openairuntime

import (
	"bidora/backend/internal/opportunity/extractopenai"
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"context"
	"errors"
	"time"
)

type Tester struct {
	Secrets smtpsecret.Resolver
	SMTP    app.Tester
	PesaPal app.Tester
}

func (t *Tester) Test(ctx context.Context, c domain.Configuration, actor domain.ID) (app.TestOutcome, error) {
	if c.Kind == domain.PesaPal && t.PesaPal != nil {
		return t.PesaPal.Test(ctx, c, actor)
	}
	if c.Kind == domain.SMTP {
		return t.SMTP.Test(ctx, c, actor)
	}
	if c.Kind != domain.OpenAI || !c.Valid() || t.Secrets == nil || actor.IsZero() {
		return app.TestOutcome{}, errors.New("openai_test_invalid")
	}
	if c.OpenAI.Disabled {
		return app.TestOutcome{Passed: true, ResultCode: "OPENAI_DISABLED_CONFIG_VERIFIED"}, nil
	}
	outcome := app.TestOutcome{ResultCode: "PROVIDER_REJECTED"}
	e := t.Secrets.WithSecret(ctx, c.OpenAI.APIKeySecretRef, func(key []byte) error {
		return t.Secrets.WithSecret(ctx, c.OpenAI.TrustRootsSecretRef, func(pem []byte) error {
			roots, e := smtpsecret.ParseTrustRoots(pem)
			if e != nil {
				return e
			}
			provider, e := extractopenai.New(extractopenai.Config{APIKey: string(key), Model: c.OpenAI.Model, RootCAs: roots, Timeout: time.Duration(c.OpenAI.TimeoutSeconds) * time.Second})
			if e != nil {
				return e
			}
			defer provider.Close()
			if e = provider.CheckConnection(ctx); e != nil {
				return e
			}
			outcome = app.TestOutcome{Passed: true, ResultCode: "OPENAI_STRUCTURED_RESPONSE_VERIFIED"}
			return nil
		})
	})
	if e != nil {
		return outcome, nil
	}
	return outcome, nil
}
