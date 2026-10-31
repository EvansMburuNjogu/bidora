// Package platformemail adapts Platform Administration notification requests to
// an explicitly supplied email sender. It selects no SMTP/provider configuration.
package platformemail

import (
	"context"
	"errors"
	"html"
	"strings"
	"time"

	"bidora/backend/internal/identity/email"
	notificationapp "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/emailtemplate"
	"bidora/backend/internal/platformadmin/domain"
	platforminvite "bidora/backend/internal/platformadmin/invitation"
)

type Sender interface {
	Send(context.Context, email.Address, emailtemplate.Message) error
}

type PlatformIdentity struct{ Name, SupportEmail, Website, Phone string }
type IdentityReader interface {
	PlatformIdentity(context.Context) (PlatformIdentity, error)
}
type Delivery struct {
	sender    Sender
	identity  IdentityReader
	templates notificationapp.TemplateRenderer
}

func (d *Delivery) EnableTemplates(renderer notificationapp.TemplateRenderer) error {
	if d == nil || renderer == nil {
		return errors.New("platform_email_delivery_invalid")
	}
	d.templates = renderer
	return nil
}
func (d *Delivery) configured(ctx context.Context, to email.Address, key string, values map[string]string, fallback emailtemplate.Message) error {
	if d.templates != nil {
		v, e := d.templates.Render(ctx, key, values)
		if e != nil {
			return e
		}
		fallback = emailtemplate.Message{Subject: v.Subject, Text: v.Text, HTML: v.HTML}
	}
	return d.send(ctx, to, fallback)
}

func NewWithIdentity(sender Sender, identity IdentityReader) (*Delivery, error) {
	if sender == nil || identity == nil {
		return nil, errors.New("platform_email_delivery_invalid")
	}
	return &Delivery{sender: sender, identity: identity}, nil
}
func (d *Delivery) send(ctx context.Context, recipient email.Address, message emailtemplate.Message) error {
	if d.identity != nil && d.templates == nil {
		identity, err := d.identity.PlatformIdentity(ctx)
		if err != nil {
			return err
		}
		// Append configured facts without rewriting invitation tokens, protected
		// message content or the separately governed SMTP sender identity.
		contact := identity.Name
		for _, value := range []string{identity.SupportEmail, identity.Website, identity.Phone} {
			if value != "" {
				contact += " · " + value
			}
		}
		message.Text += "\n" + contact + "\n"
		message.HTML = strings.Replace(message.HTML, "</body>", "<p>"+html.EscapeString(contact)+"</p></body>", 1)
	}
	return d.sender.Send(ctx, recipient, message)
}

func New(sender Sender) (*Delivery, error) {
	if sender == nil {
		return nil, errors.New("platform_email_delivery_invalid")
	}
	return &Delivery{sender: sender}, nil
}

func (d *Delivery) DeliverPlatformInvitation(ctx context.Context, recipient email.Address, token platforminvite.Token, role domain.Role, expiresAt time.Time) error {
	if ctx == nil || d == nil || d.sender == nil || recipient.IsZero() || token.IsZero() || expiresAt.IsZero() {
		return errors.New("platform_email_delivery_invalid")
	}
	message, err := emailtemplate.RenderPlatformInvitation(emailtemplate.PlatformInvitationInput{
		Kind: emailtemplate.PlatformInvitationIssued, Role: role,
		Token: token.RevealForDelivery(), ExpiresAt: expiresAt.UTC(),
	})
	if err != nil {
		return err
	}
	return d.configured(ctx, recipient, "platform.invitation.issued", map[string]string{"role": string(role), "token": token.RevealForDelivery(), "expires_at": expiresAt.UTC().Format(time.RFC3339)}, message)
}

func (d *Delivery) DeliverPlatformSessionRevocation(ctx context.Context, recipient email.Address, occurredAt time.Time) error {
	if ctx == nil || d == nil || d.sender == nil || recipient.IsZero() || occurredAt.IsZero() {
		return errors.New("platform_email_delivery_invalid")
	}
	message, err := emailtemplate.RenderPlatformSessionRevocation(emailtemplate.PlatformSessionRevocationInput{OccurredAt: occurredAt.UTC()})
	if err != nil {
		return err
	}
	return d.configured(ctx, recipient, "platform.session_revocation", map[string]string{"occurred_at": occurredAt.UTC().Format(time.RFC3339)}, message)
}

func (d *Delivery) DeliverPlatformIdentityStateChange(ctx context.Context, recipient email.Address, disabled bool, occurredAt time.Time) error {
	if ctx == nil || d == nil || d.sender == nil || recipient.IsZero() || occurredAt.IsZero() {
		return errors.New("platform_email_delivery_invalid")
	}
	message, err := emailtemplate.RenderPlatformIdentityState(emailtemplate.PlatformIdentityStateInput{Disabled: disabled, OccurredAt: occurredAt.UTC()})
	if err != nil {
		return err
	}
	action := "reactivated"
	if disabled {
		action = "disabled"
	}
	return d.configured(ctx, recipient, "platform.identity_state", map[string]string{"action": action, "occurred_at": occurredAt.UTC().Format(time.RFC3339)}, message)
}

func (d *Delivery) DeliverPlatformRoleChange(ctx context.Context, recipient email.Address, role domain.Role, assigned bool, occurredAt time.Time) error {
	if ctx == nil || d == nil || d.sender == nil || recipient.IsZero() || occurredAt.IsZero() {
		return errors.New("platform_email_delivery_invalid")
	}
	message, err := emailtemplate.RenderPlatformRoleChange(emailtemplate.PlatformRoleChangeInput{Role: role, Assigned: assigned, OccurredAt: occurredAt.UTC()})
	if err != nil {
		return err
	}
	action := "revoked"
	if assigned {
		action = "assigned"
	}
	return d.configured(ctx, recipient, "platform.role_change", map[string]string{"role": string(role), "action": action, "occurred_at": occurredAt.UTC().Format(time.RFC3339)}, message)
}

// DeliverPlatformInvitationReminder deliberately contains no invitation token:
// only its hash is retained after issuance. Reminder cadence and scheduling are
// separate policy; callers must recheck that the invitation is still active.
func (d *Delivery) DeliverPlatformInvitationReminder(ctx context.Context, recipient email.Address, role domain.Role, expiresAt time.Time) error {
	if ctx == nil || d == nil || d.sender == nil || recipient.IsZero() || expiresAt.IsZero() {
		return errors.New("platform_email_delivery_invalid")
	}
	message, err := emailtemplate.RenderPlatformInvitation(emailtemplate.PlatformInvitationInput{
		Kind: emailtemplate.PlatformInvitationReminder, Role: role, ExpiresAt: expiresAt.UTC(),
	})
	if err != nil {
		return err
	}
	return d.configured(ctx, recipient, "platform.invitation.reminder", map[string]string{"role": string(role), "expires_at": expiresAt.UTC().Format(time.RFC3339)}, message)
}
