// Package identityemail composes identity email delivery through exact-active
// SMTP resolution and the durable notification evidence recorder.
package identityemail

import (
	"context"
	"errors"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/verification"
	notificationapp "bidora/backend/internal/notification/application"
	notificationdelivery "bidora/backend/internal/notification/delivery"
	notificationdomain "bidora/backend/internal/notification/domain"
	settingsdomain "bidora/backend/internal/platformsettings/domain"
	"github.com/google/uuid"
)

type ActiveSMTP interface {
	WithActive(context.Context, func(settingsdomain.ID, *deliverysmtp.Delivery) error) error
}

type Delivery struct {
	active    ActiveSMTP
	recorder  *notificationdelivery.Recorder
	templates notificationapp.TemplateRenderer
}

func New(active ActiveSMTP, recorder *notificationdelivery.Recorder) (*Delivery, error) {
	if active == nil || recorder == nil {
		return nil, errors.New("identity_notification_delivery_invalid")
	}
	return &Delivery{active: active, recorder: recorder}, nil
}

func (d *Delivery) EnableTemplates(renderer notificationapp.TemplateRenderer) error {
	if d == nil || renderer == nil {
		return errors.New("identity_notification_delivery_invalid")
	}
	d.templates = renderer
	return nil
}

// Legacy calls deliberately fail: authoritative source/correlation metadata is
// mandatory for ledger-backed delivery.
func (*Delivery) DeliverEmailVerification(context.Context, email.Address, verification.Token, time.Time) error {
	return errors.New("identity_notification_metadata_required")
}
func (*Delivery) DeliverPasswordRecovery(context.Context, email.Address, verification.Token, time.Time) error {
	return errors.New("identity_notification_metadata_required")
}

func (d *Delivery) DeliverEmailVerificationWithEvidence(ctx context.Context, request identityapp.EmailVerificationDeliveryRequest) error {
	if ctx == nil || d == nil || d.active == nil || d.recorder == nil || request.ChallengeID.IsZero() || request.UserID.IsZero() || request.CorrelationID.IsZero() || request.Recipient.IsZero() || request.Token.IsZero() || request.ExpiresAt.IsZero() {
		return errors.New("identity_notification_delivery_invalid")
	}
	return d.active.WithActive(ctx, func(versionID settingsdomain.ID, smtp *deliverysmtp.Delivery) error {
		version := int64(1)
		prepared, err := smtp.PrepareEmailVerification(request.Recipient, request.Token, request.ExpiresAt)
		fingerprint := prepared.BytesForFingerprint()
		if d.templates != nil {
			var v notificationapp.RenderedTemplate
			v, err = d.templates.Render(ctx, "identity.email_verification", map[string]string{"token": request.Token.RevealForDelivery(), "expires_at": request.ExpiresAt.UTC().Format(time.RFC3339)})
			if err == nil {
				version = v.Version
				fingerprint = v.ContentForFingerprint()
				prepared, err = smtp.PrepareRendered(request.Recipient, v.Subject, v.Text, v.HTML)
			}
		}
		if err != nil {
			return err
		}
		return d.recorder.Send(ctx, notificationdelivery.Request{
			ActorID: notificationID(request.ActorID), RecipientUserID: notificationID(request.UserID), SourceID: notificationID(request.ChallengeID), CorrelationID: notificationID(request.CorrelationID), SMTPVersionID: notificationSettingsID(versionID),
			Category: "EMAIL_VERIFICATION", TemplateKey: "identity.email_verification", SourceType: "EMAIL_VERIFICATION_CHALLENGE", IdempotencyKey: uuid.UUID(request.ChallengeID).String(), TemplateVersion: version, AttemptNumber: 1,
			RecipientCanonical: request.Recipient.Lookup(), RenderedContent: fingerprint,
		}, func(sendCtx context.Context) error { return smtp.DeliverPrepared(sendCtx, prepared) })
	})
}

func (d *Delivery) DeliverPasswordRecoveryWithEvidence(ctx context.Context, request identityapp.PasswordRecoveryDeliveryRequest) error {
	if ctx == nil || d == nil || d.active == nil || d.recorder == nil || request.ChallengeID.IsZero() || request.UserID.IsZero() || request.CorrelationID.IsZero() || request.Recipient.IsZero() || request.Token.IsZero() || request.ExpiresAt.IsZero() {
		return errors.New("identity_notification_delivery_invalid")
	}
	return d.active.WithActive(ctx, func(versionID settingsdomain.ID, smtp *deliverysmtp.Delivery) error {
		version := int64(1)
		prepared, err := smtp.PreparePasswordRecovery(request.Recipient, request.Token, request.ExpiresAt)
		fingerprint := prepared.BytesForFingerprint()
		if d.templates != nil {
			var v notificationapp.RenderedTemplate
			v, err = d.templates.Render(ctx, "identity.password_recovery", map[string]string{"token": request.Token.RevealForDelivery(), "user_id": uuid.UUID(request.UserID).String(), "expires_at": request.ExpiresAt.UTC().Format(time.RFC3339)})
			if err == nil {
				version = v.Version
				fingerprint = v.ContentForFingerprint()
				prepared, err = smtp.PrepareRendered(request.Recipient, v.Subject, v.Text, v.HTML)
			}
		}
		if err != nil {
			return err
		}
		return d.recorder.Send(ctx, notificationdelivery.Request{
			RecipientUserID: notificationID(request.UserID), SourceID: notificationID(request.ChallengeID), CorrelationID: notificationID(request.CorrelationID), SMTPVersionID: notificationSettingsID(versionID),
			Category: "PASSWORD_RECOVERY", TemplateKey: "identity.password_recovery", SourceType: "PASSWORD_RECOVERY_CHALLENGE", IdempotencyKey: uuid.UUID(request.ChallengeID).String(), TemplateVersion: version, AttemptNumber: 1,
			RecipientCanonical: request.Recipient.Lookup(), RenderedContent: fingerprint,
		}, func(sendCtx context.Context) error { return smtp.DeliverPrepared(sendCtx, prepared) })
	})
}

func notificationID(value identityapp.ID) notificationdomain.ID { return notificationdomain.ID(value) }
func notificationSettingsID(value settingsdomain.ID) notificationdomain.ID {
	return notificationdomain.ID(value)
}
