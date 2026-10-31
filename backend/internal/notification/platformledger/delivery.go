// Package platformledger records Platform invitation and security email delivery.
package platformledger

import (
	"context"
	"errors"
	"time"

	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	notificationapp "bidora/backend/internal/notification/application"
	notificationdelivery "bidora/backend/internal/notification/delivery"
	notificationdomain "bidora/backend/internal/notification/domain"
	"bidora/backend/internal/notification/emailtemplate"
	platformapp "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	platforminvite "bidora/backend/internal/platformadmin/invitation"
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
		return nil, errors.New("platform_notification_delivery_invalid")
	}
	return &Delivery{active: active, recorder: recorder}, nil
}
func (d *Delivery) EnableTemplates(renderer notificationapp.TemplateRenderer) error {
	if d == nil || renderer == nil {
		return errors.New("platform_notification_delivery_invalid")
	}
	d.templates = renderer
	return nil
}
func (*Delivery) DeliverPlatformInvitation(context.Context, email.Address, platforminvite.Token, domain.Role, time.Time) error {
	return errors.New("platform_notification_metadata_required")
}

func (d *Delivery) DeliverPlatformInvitationWithEvidence(ctx context.Context, request platformapp.PlatformInvitationDeliveryRequest) error {
	if ctx == nil || d == nil || d.active == nil || d.recorder == nil || request.InvitationID.IsZero() || request.ActorID.IsZero() || request.CorrelationID.IsZero() || request.Recipient.IsZero() || request.Token.IsZero() || request.ExpiresAt.IsZero() {
		return errors.New("platform_notification_delivery_invalid")
	}
	message, err := emailtemplate.RenderPlatformInvitation(emailtemplate.PlatformInvitationInput{Kind: emailtemplate.PlatformInvitationIssued, Role: request.Role, Token: request.Token.RevealForDelivery(), ExpiresAt: request.ExpiresAt.UTC()})
	if err != nil {
		return err
	}
	return d.send(ctx, sendRequest{SourceID: request.InvitationID, ActorID: request.ActorID, CorrelationID: request.CorrelationID, Recipient: request.Recipient, Category: "PLATFORM_INVITATION", SourceType: "PLATFORM_INVITATION", TemplateKey: "platform.invitation.issued", Values: map[string]string{"role": string(request.Role), "token": request.Token.RevealForDelivery(), "expires_at": request.ExpiresAt.UTC().Format(time.RFC3339)}, Message: message})
}

func (d *Delivery) DeliverPlatformSessionRevocation(ctx context.Context, request platformapp.PlatformSecurityDeliveryRequest) error {
	if !validSecurity(ctx, d, request) {
		return errors.New("platform_notification_delivery_invalid")
	}
	message, err := emailtemplate.RenderPlatformSessionRevocation(emailtemplate.PlatformSessionRevocationInput{OccurredAt: request.OccurredAt.UTC()})
	if err != nil {
		return err
	}
	return d.send(ctx, sendRequest{SourceID: request.SourceID, ActorID: request.ActorID, RecipientUserID: request.RecipientUserID, CorrelationID: request.CorrelationID, Recipient: request.Recipient, Category: "PLATFORM_SESSION_REVOCATION", SourceType: "PLATFORM_SESSION_REVOCATION", TemplateKey: "platform.session_revocation", Values: map[string]string{"occurred_at": request.OccurredAt.UTC().Format(time.RFC3339)}, Message: message})
}
func (d *Delivery) DeliverPlatformIdentityStateChange(ctx context.Context, request platformapp.PlatformSecurityDeliveryRequest, disabled bool) error {
	if !validSecurity(ctx, d, request) {
		return errors.New("platform_notification_delivery_invalid")
	}
	message, err := emailtemplate.RenderPlatformIdentityState(emailtemplate.PlatformIdentityStateInput{Disabled: disabled, OccurredAt: request.OccurredAt.UTC()})
	if err != nil {
		return err
	}
	return d.send(ctx, sendRequest{SourceID: request.SourceID, ActorID: request.ActorID, RecipientUserID: request.RecipientUserID, CorrelationID: request.CorrelationID, Recipient: request.Recipient, Category: "PLATFORM_IDENTITY_STATE_CHANGE", SourceType: "PLATFORM_IDENTITY_STATE_CHANGE", TemplateKey: "platform.identity_state", Values: map[string]string{"action": map[bool]string{true: "disabled", false: "reactivated"}[disabled], "occurred_at": request.OccurredAt.UTC().Format(time.RFC3339)}, Message: message})
}
func (d *Delivery) DeliverPlatformRoleChange(ctx context.Context, request platformapp.PlatformSecurityDeliveryRequest, role domain.Role, assigned bool) error {
	if !validSecurity(ctx, d, request) {
		return errors.New("platform_notification_delivery_invalid")
	}
	message, err := emailtemplate.RenderPlatformRoleChange(emailtemplate.PlatformRoleChangeInput{Role: role, Assigned: assigned, OccurredAt: request.OccurredAt.UTC()})
	if err != nil {
		return err
	}
	return d.send(ctx, sendRequest{SourceID: request.SourceID, ActorID: request.ActorID, RecipientUserID: request.RecipientUserID, CorrelationID: request.CorrelationID, Recipient: request.Recipient, Category: "PLATFORM_ROLE_CHANGE", SourceType: "PLATFORM_ROLE_CHANGE", TemplateKey: "platform.role_change", Values: map[string]string{"role": string(role), "action": map[bool]string{true: "assigned", false: "revoked"}[assigned], "occurred_at": request.OccurredAt.UTC().Format(time.RFC3339)}, Message: message})
}

type sendRequest struct {
	SourceID, ActorID, RecipientUserID, CorrelationID platformapp.ID
	Recipient                                         email.Address
	Category, SourceType, TemplateKey                 string
	Message                                           emailtemplate.Message
	Values                                            map[string]string
}

func (d *Delivery) send(ctx context.Context, request sendRequest) error {
	version := int64(1)
	if d.templates != nil {
		v, e := d.templates.Render(ctx, request.TemplateKey, request.Values)
		if e != nil {
			return e
		}
		version = v.Version
		request.Message = emailtemplate.Message{Subject: v.Subject, Text: v.Text, HTML: v.HTML}
	}
	return d.active.WithActive(ctx, func(versionID settingsdomain.ID, smtp *deliverysmtp.Delivery) error {
		prepared, err := smtp.PrepareRendered(request.Recipient, request.Message.Subject, request.Message.Text, request.Message.HTML)
		if err != nil {
			return err
		}
		fingerprint := prepared.BytesForFingerprint()
		if d.templates != nil {
			fingerprint = (notificationapp.RenderedTemplate{Subject: request.Message.Subject, Text: request.Message.Text, HTML: request.Message.HTML}).ContentForFingerprint()
		}
		return d.recorder.Send(ctx, notificationdelivery.Request{
			ActorID: notificationID(request.ActorID), RecipientUserID: notificationID(request.RecipientUserID), SourceID: notificationID(request.SourceID), CorrelationID: notificationID(request.CorrelationID), SMTPVersionID: notificationSettingsID(versionID),
			Category: request.Category, TemplateKey: request.TemplateKey, SourceType: request.SourceType, IdempotencyKey: uuid.UUID(request.SourceID).String(), TemplateVersion: version, AttemptNumber: 1,
			RecipientCanonical: request.Recipient.Lookup(), RenderedContent: fingerprint,
		}, func(sendCtx context.Context) error { return smtp.DeliverPrepared(sendCtx, prepared) })
	})
}
func validSecurity(ctx context.Context, d *Delivery, request platformapp.PlatformSecurityDeliveryRequest) bool {
	return ctx != nil && d != nil && d.active != nil && d.recorder != nil && !request.SourceID.IsZero() && !request.ActorID.IsZero() && !request.RecipientUserID.IsZero() && !request.CorrelationID.IsZero() && !request.Recipient.IsZero() && !request.OccurredAt.IsZero()
}
func notificationID(value platformapp.ID) notificationdomain.ID { return notificationdomain.ID(value) }
func notificationSettingsID(value settingsdomain.ID) notificationdomain.ID {
	return notificationdomain.ID(value)
}
