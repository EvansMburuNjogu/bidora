package httpapi

import (
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"github.com/gofiber/fiber/v3"
)

type openAISecretPreparer interface {
	PrepareOpenAISecrets(domain.ID, string, *string, *string) (string, string, []app.SealedIntegrationSecret, error)
}
type createOpenAIRequest struct {
	Enabled     bool    `json:"enabled"`
	Model       string  `json:"model"`
	APIKey      *string `json:"api_key"`
	TrustRoots  *string `json:"trust_roots_pem"`
	Timeout     int32   `json:"timeout_seconds"`
	Concurrency int32   `json:"max_concurrency"`
	Reason      string  `json:"reason"`
}

func (h *PlatformIntegrationSettingsWriteHandler) createOpenAI(c fiber.Ctx, common settingMutation) error {
	var body createOpenAIRequest
	if !bindManagedSettings(c, &body, []string{"enabled", "model", "timeout_seconds", "max_concurrency", "reason"}, []string{"api_key", "trust_roots_pem"}) {
		return nil
	}
	if !app.SupportedOpenAIModel(body.Model) || body.Timeout < 10 || body.Timeout > 900 || body.Concurrency < 1 || body.Concurrency > 100 {
		return fail(c, 400, "INVALID_REQUEST", "Check the model and execution limits.", false)
	}
	if e := h.writer.AuthorizeCreate(c.Context(), common.subject, domain.OpenAI); e != nil {
		return platformSettingFailure(c, e)
	}
	preparer, ok := h.secrets.(openAISecretPreparer)
	if !ok {
		return platformSettingFailure(c, app.ErrUnavailable)
	}
	keyRef, rootRef, secrets, e := preparer.PrepareOpenAISecrets(common.subject.ActorID, common.idempotency, body.APIKey, body.TrustRoots)
	body.APIKey = nil
	body.TrustRoots = nil
	if e != nil {
		return fail(c, 400, "INVALID_REQUEST", "Check the replacement key and CA certificates.", false)
	}
	configuration, _, e := domain.NewConfiguration(domain.OpenAI, nil, &domain.OpenAIConfiguration{Disabled: !body.Enabled, Model: body.Model, APIKeySecretRef: keyRef, TrustRootsSecretRef: rootRef, TimeoutSeconds: body.Timeout, MaxConcurrency: body.Concurrency})
	if e != nil {
		return platformSettingFailure(c, app.ErrInvalid)
	}
	result, e := h.writer.Create(c.Context(), app.CreateInput{Subject: common.subject, Configuration: configuration, Secrets: secrets, ReuseOpenAIKey: keyRef == "retain-active", ReuseOpenAIRoots: rootRef == "retain-active", Reason: body.Reason, IdempotencyKey: common.idempotency, CorrelationID: common.correlation})
	if e != nil {
		return platformSettingFailure(c, e)
	}
	return platformJSON(c, 201, commandVersionJSON(result, "NOT_TESTED", ""), nil)
}
func (h *PlatformIntegrationSettingsWriteHandler) models(c fiber.Ctx) error {
	subject, ok := platformSettingsSubject(c)
	if !ok {
		return nil
	}
	if !settingsEmptyRead(c) {
		return nil
	}
	_, e := h.reader.Versions(c.Context(), subject, domain.OpenAI, nil, 1)
	if e != nil {
		return platformSettingFailure(c, e)
	}
	return platformJSON(c, 200, app.SupportedOpenAIModels(), nil)
}
