package identityemail

import (
	"context"
	"errors"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	notificationdelivery "bidora/backend/internal/notification/delivery"
	settingsdomain "bidora/backend/internal/platformsettings/domain"
	"github.com/google/uuid"
	"time"
)

func (d *Delivery) EmailLoginAvailable(ctx context.Context) bool {
	return d != nil && d.active != nil && d.active.WithActive(ctx, func(settingsdomain.ID, *deliverysmtp.Delivery) error { return nil }) == nil
}

func (*Delivery) DeliverLoginCode(context.Context, email.Address, string, time.Time) error {
	return errors.New("identity_notification_metadata_required")
}

func (d *Delivery) DeliverLoginCodeWithEvidence(ctx context.Context, request identityapp.EmailLoginDeliveryRequest) error {
	if ctx == nil || d == nil || d.active == nil || d.recorder == nil || d.templates == nil ||
		request.UserID.IsZero() || request.TransactionID.IsZero() || request.Recipient.IsZero() || request.ExpiresAt.IsZero() || len(request.Code) != 6 {
		return errors.New("identity_notification_delivery_invalid")
	}
	for _, c := range request.Code {
		if c < '0' || c > '9' {
			return errors.New("identity_notification_delivery_invalid")
		}
	}
	return d.active.WithActive(ctx, func(version settingsdomain.ID, smtp *deliverysmtp.Delivery) error {
		v, err := d.templates.Render(ctx, "identity.login_code", map[string]string{"code": request.Code, "expires_at": request.ExpiresAt.UTC().Format(time.RFC3339)})
		if err != nil {
			return err
		}
		prepared, err := smtp.PrepareRendered(request.Recipient, v.Subject, v.Text, v.HTML)
		if err != nil {
			return err
		}
		return d.recorder.Send(ctx, notificationdelivery.Request{
			ActorID: notificationID(request.UserID), RecipientUserID: notificationID(request.UserID),
			SourceID: notificationID(request.TransactionID), CorrelationID: notificationID(request.TransactionID), SMTPVersionID: notificationSettingsID(version),
			Category: "LOGIN_CODE", TemplateKey: "identity.login_code", SourceType: "EMAIL_LOGIN_TRANSACTION",
			IdempotencyKey: uuid.UUID(request.TransactionID).String(), TemplateVersion: v.Version, AttemptNumber: 1,
			Subject: prepared.RecordSubject(), Text: prepared.RecordText(), RecipientCanonical: request.Recipient.Lookup(), RenderedContent: v.ContentForFingerprint(),
		}, func(c context.Context) error { return smtp.DeliverPrepared(c, prepared) })
	})
}
