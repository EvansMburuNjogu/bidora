package httpapi

import (
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type pesaPalSecretPreparer interface {
	PreparePesaPalSecrets(domain.ID, string, *string, *string) (string, []app.SealedIntegrationSecret, error)
}

func (h *PlatformIntegrationSettingsWriteHandler) createPesaPal(c fiber.Ctx, common settingMutation) error {
	var body struct {
		Enabled        bool    `json:"enabled"`
		Environment    string  `json:"environment"`
		Merchant       string  `json:"merchant_name"`
		NotificationID string  `json:"notification_id"`
		Key            *string `json:"consumer_key"`
		Secret         *string `json:"consumer_secret"`
		Reason         string  `json:"reason"`
		Retain         bool    `json:"retain_active_credentials"`
	}
	if !bindManagedSettings(c, &body, []string{"enabled", "environment", "merchant_name", "notification_id", "reason", "retain_active_credentials"}, []string{"consumer_key", "consumer_secret"}) {
		return nil
	}
	if e := h.writer.AuthorizeCreate(c.Context(), common.subject, domain.PesaPal); e != nil {
		return platformSettingFailure(c, e)
	}
	id := uuid.Nil
	if body.NotificationID != "" {
		parsed, e := uuid.Parse(body.NotificationID)
		if e != nil || parsed == uuid.Nil {
			return platformSettingFailure(c, app.ErrInvalid)
		}
		id = parsed
	}
	if body.Retain && (body.Key != nil || body.Secret != nil) || !body.Retain && body.Enabled && (body.Key == nil || body.Secret == nil) {
		return platformSettingFailure(c, app.ErrInvalid)
	}
	preparer, ok := h.secrets.(pesaPalSecretPreparer)
	if !ok {
		return platformSettingFailure(c, app.ErrUnavailable)
	}
	ref, secrets, e := preparer.PreparePesaPalSecrets(common.subject.ActorID, common.idempotency, body.Key, body.Secret)
	body.Key = nil
	body.Secret = nil
	if e != nil {
		return platformSettingFailure(c, app.ErrInvalid)
	}
	if !body.Retain && ref == "retain-active" {
		ref = "unconfigured"
	}
	configuration, _, e := domain.NewPesaPalConfiguration(domain.PesaPalConfiguration{Enabled: body.Enabled, Environment: body.Environment, MerchantName: body.Merchant, CredentialsSecretRef: ref, NotificationID: domain.ID(id)})
	if e != nil {
		return platformSettingFailure(c, app.ErrInvalid)
	}
	out, e := h.writer.Create(c.Context(), app.CreateInput{Subject: common.subject, Configuration: configuration, Secrets: secrets, ReusePesaPalCredentials: body.Retain, Reason: body.Reason, IdempotencyKey: common.idempotency, CorrelationID: common.correlation})
	if e != nil {
		return platformSettingFailure(c, e)
	}
	return platformJSON(c, 201, commandVersionJSON(out, "NOT_TESTED", ""), nil)
}
