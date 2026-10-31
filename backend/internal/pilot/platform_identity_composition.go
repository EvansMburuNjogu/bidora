package pilot

import (
	"bidora/backend/internal/identity/email"
	notificationapp "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/platformemail"
	"bidora/backend/internal/notification/platformledger"
	platformapp "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	"bidora/backend/internal/platformadmin/invitation"
	app "bidora/backend/internal/platformsettings/application"
	persistence "bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"bidora/backend/internal/postgres"
	"context"
	"time"
)

type platformSettingsDelivery struct {
	base     *platformemail.Delivery
	recorded *platformledger.Delivery
}

func (d *platformSettingsDelivery) DeliverPlatformInvitation(ctx context.Context, to email.Address, token invitation.Token, role domain.Role, at time.Time) error {
	return d.base.DeliverPlatformInvitation(ctx, to, token, role, at)
}
func (d *platformSettingsDelivery) DeliverPlatformInvitationWithEvidence(ctx context.Context, r platformapp.PlatformInvitationDeliveryRequest) error {
	if d.recorded != nil {
		return d.recorded.DeliverPlatformInvitationWithEvidence(ctx, r)
	}
	return d.base.DeliverPlatformInvitation(ctx, r.Recipient, r.Token, r.Role, r.ExpiresAt)
}
func (d *platformSettingsDelivery) DeliverPlatformSessionRevocation(ctx context.Context, r platformapp.PlatformSecurityDeliveryRequest) error {
	if d.recorded != nil {
		return d.recorded.DeliverPlatformSessionRevocation(ctx, r)
	}
	return d.base.DeliverPlatformSessionRevocation(ctx, r.Recipient, r.OccurredAt)
}
func (d *platformSettingsDelivery) DeliverPlatformIdentityStateChange(ctx context.Context, r platformapp.PlatformSecurityDeliveryRequest, disabled bool) error {
	if d.recorded != nil {
		return d.recorded.DeliverPlatformIdentityStateChange(ctx, r, disabled)
	}
	return d.base.DeliverPlatformIdentityStateChange(ctx, r.Recipient, disabled, r.OccurredAt)
}
func (d *platformSettingsDelivery) DeliverPlatformRoleChange(ctx context.Context, r platformapp.PlatformSecurityDeliveryRequest, role domain.Role, assigned bool) error {
	if d.recorded != nil {
		return d.recorded.DeliverPlatformRoleChange(ctx, r, role, assigned)
	}
	return d.base.DeliverPlatformRoleChange(ctx, r.Recipient, role, assigned, r.OccurredAt)
}

type platformIdentityForEmail struct{ service *app.InformationService }

func (r platformIdentityForEmail) PlatformIdentity(ctx context.Context) (platformemail.PlatformIdentity, error) {
	value, e := r.service.Shared(ctx)
	if e != nil {
		return platformemail.PlatformIdentity{}, e
	}
	out := platformemail.PlatformIdentity{Name: value.PlatformName, Phone: value.ContactPhone}
	if value.SupportEmail != nil {
		out.SupportEmail = *value.SupportEmail
	}
	if value.Website != nil {
		out.Website = *value.Website
	}
	return out, nil
}
func newPlatformEmailDelivery(pool *postgres.Pool, delivery smtpDelivery, config Config, renderer notificationapp.TemplateRenderer) (platformapp.PlatformInvitationDelivery, error) {
	store, e := persistence.NewInformationStore(pool)
	if e != nil {
		return nil, e
	}
	service, e := app.NewInformationService(store, platformSettingIDs{}, systemClock{})
	if e != nil {
		return nil, e
	}
	base, e := platformemail.NewWithIdentity(platformSMTPSender{delivery: delivery}, platformIdentityForEmail{service})
	if e != nil {
		return nil, e
	}
	if e = base.EnableTemplates(renderer); e != nil {
		return nil, e
	}
	var recorded *platformledger.Delivery
	if len(config.NotificationFingerprintKey) == 32 && config.NotificationFingerprintKeyVersion > 0 {
		vault, e := smtpsecret.NewVault(pool, config.SMTPVaultKey, config.SMTPVaultKeyVersion)
		if e != nil {
			return nil, e
		}
		_, recorded, e = BuildInvitationNotificationDelivery(pool, vault, config.NotificationFingerprintKey, config.NotificationFingerprintKeyVersion)
		if e != nil {
			return nil, e
		}
		if e = recorded.EnableTemplates(renderer); e != nil {
			return nil, e
		}
	}
	return &platformSettingsDelivery{base, recorded}, nil
}
