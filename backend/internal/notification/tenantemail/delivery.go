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
	notificationdelivery "bidora/backend/internal/notification/delivery"
	notificationdomain "bidora/backend/internal/notification/domain"
	settingsdomain "bidora/backend/internal/platformsettings/domain"
	"github.com/google/uuid"
)

type ActiveSMTP interface {
	WithActive(context.Context, func(settingsdomain.ID, *deliverysmtp.Delivery) error) error
}
type Delivery struct {
	active   ActiveSMTP
	recorder *notificationdelivery.Recorder
}

func New(active ActiveSMTP, recorder *notificationdelivery.Recorder) (*Delivery, error) {
	if active == nil || recorder == nil {
		return nil, errors.New("tenant_notification_delivery_invalid")
	}
	return &Delivery{active: active, recorder: recorder}, nil
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
		return d.recorder.Send(ctx, notificationdelivery.Request{
			TenantID: notificationID(request.TenantID), ActorID: notificationID(request.ActorID), SourceID: notificationID(request.InvitationID), CorrelationID: notificationID(request.CorrelationID), SMTPVersionID: notificationSettingsID(versionID),
			Category: "TENANT_INVITATION", TemplateKey: "tenant.invitation", SourceType: "TENANT_INVITATION", IdempotencyKey: uuid.UUID(request.InvitationID).String(), TemplateVersion: 1, AttemptNumber: 1,
			RecipientCanonical: recipient.Lookup(), RenderedContent: prepared.BytesForFingerprint(),
		}, func(sendCtx context.Context) error { return smtp.DeliverPrepared(sendCtx, prepared) })
	})
}
func notificationID(value authzapp.ID) notificationdomain.ID { return notificationdomain.ID(value) }
func notificationSettingsID(value settingsdomain.ID) notificationdomain.ID {
	return notificationdomain.ID(value)
}
