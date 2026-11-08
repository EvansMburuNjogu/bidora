// Package tenantemail records Tenderpreneur invitation delivery through exact-active SMTP.
package tenantemail

import (
	"context"
	"errors"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	"bidora/backend/internal/authorization/invitation"
	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
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
	templates notificationapp.TemplateRenderer
	active    ActiveSMTP
	recorder  *notificationdelivery.Recorder
}

func New(active ActiveSMTP, recorder *notificationdelivery.Recorder) (*Delivery, error) {
	if active == nil || recorder == nil {
		return nil, errors.New("tenant_notification_delivery_invalid")
	}
	return &Delivery{active: active, recorder: recorder}, nil
}
func (d *Delivery) EnableTemplates(renderer notificationapp.TemplateRenderer) error {
	if d == nil || renderer == nil {
		return errors.New("tenant_notification_delivery_invalid")
	}
	d.templates = renderer
	return nil
}
func (*Delivery) DeliverTenantInvitation(context.Context, string, invitation.Token, time.Time) error {
	return errors.New("tenant_notification_metadata_required")
}
func (*Delivery) DeliverTenantRoleInvitation(context.Context, string, authzapp.TenderpreneurRole, invitation.Token, time.Time) error {
	return errors.New("tenant_notification_metadata_required")
}

func (d *Delivery) DeliverTenantRoleInvitationWithEvidence(ctx context.Context, request authzapp.TenantInvitationDeliveryRequest) error {
	if ctx == nil || d == nil || d.active == nil || d.recorder == nil || request.InvitationID.IsZero() || request.ActorID.IsZero() || request.TenantID.IsZero() || request.CorrelationID.IsZero() || request.Token.IsZero() || request.ExpiresAt.IsZero() {
		return errors.New("tenant_notification_delivery_invalid")
	}
	recipient, err := email.Parse(request.Recipient)
	if err != nil {
		return errors.New("tenant_notification_delivery_invalid")
	}
	return d.active.WithActive(ctx, func(versionID settingsdomain.ID, smtp *deliverysmtp.Delivery) error {
		prepared, prepareErr := smtp.PrepareTenantRoleInvitation(recipient, string(request.Role), request.Token.RevealForDelivery(), request.ExpiresAt)
		if prepareErr != nil {
			return prepareErr
		}
		version := int64(1)
		fingerprint := prepared.BytesForFingerprint()
		if d.templates != nil {
			v, e := d.templates.Render(ctx, "tenant.invitation", map[string]string{"token": request.Token.RevealForDelivery(), "role": string(request.Role), "expires_at": request.ExpiresAt.UTC().Format(time.RFC3339)})
			if e != nil {
				return e
			}
			version = v.Version
			fingerprint = v.ContentForFingerprint()
			prepared, e = smtp.PrepareRendered(recipient, v.Subject, v.Text, v.HTML)
			if e != nil {
				return e
			}
		}
		return d.recorder.Send(ctx, notificationdelivery.Request{
			TenantID: notificationID(request.TenantID), ActorID: notificationID(request.ActorID), SourceID: notificationID(request.InvitationID), CorrelationID: notificationID(request.CorrelationID), SMTPVersionID: notificationSettingsID(versionID),
			Category: "TENANT_INVITATION", TemplateKey: "tenant.invitation", SourceType: "TENANT_INVITATION", IdempotencyKey: uuid.UUID(request.InvitationID).String(), TemplateVersion: version, AttemptNumber: 1,
			Subject: prepared.RecordSubject(), Text: prepared.RecordText(), RecipientCanonical: recipient.Lookup(), RenderedContent: fingerprint,
		}, func(sendCtx context.Context) error { return smtp.DeliverPrepared(sendCtx, prepared) })
	})
}
func notificationID(value authzapp.ID) notificationdomain.ID { return notificationdomain.ID(value) }
func notificationSettingsID(value settingsdomain.ID) notificationdomain.ID {
	return notificationdomain.ID(value)
}
