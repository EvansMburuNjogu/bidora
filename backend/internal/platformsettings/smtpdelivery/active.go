// Package smtpdelivery resolves the exact active immutable SMTP version and its
// external secrets for one callback-scoped delivery operation.
package smtpdelivery

import (
	"context"
	"crypto/tls"
	"errors"
	"time"

	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/verification"
	"bidora/backend/internal/platformsettings/domain"
	"bidora/backend/internal/platformsettings/smtpsecret"
)

type ActiveStore interface {
	ActiveSMTP(context.Context) (domain.Version, error)
}

type Active struct {
	store   ActiveStore
	secrets smtpsecret.Resolver
}

func New(store ActiveStore, secrets smtpsecret.Resolver) (*Active, error) {
	if store == nil || secrets == nil {
		return nil, errors.New("active_smtp_delivery_invalid")
	}
	return &Active{store: store, secrets: secrets}, nil
}

func (a *Active) EmailLoginAvailable(ctx context.Context) bool {
	return a.WithActive(ctx, func(domain.ID, *deliverysmtp.Delivery) error { return nil }) == nil
}
func (a *Active) DeliverLoginCode(ctx context.Context, recipient email.Address, code string, expiresAt time.Time) error {
	return a.WithActive(ctx, func(_ domain.ID, delivery *deliverysmtp.Delivery) error {
		return delivery.DeliverLoginCode(ctx, recipient, code, expiresAt)
	})
}
func (a *Active) DeliverEmailVerification(ctx context.Context, recipient email.Address, token verification.Token, expiresAt time.Time) error {
	return a.WithActive(ctx, func(_ domain.ID, delivery *deliverysmtp.Delivery) error {
		return delivery.DeliverEmailVerification(ctx, recipient, token, expiresAt)
	})
}
func (a *Active) DeliverPasswordRecovery(ctx context.Context, recipient email.Address, token verification.Token, expiresAt time.Time) error {
	return a.WithActive(ctx, func(_ domain.ID, delivery *deliverysmtp.Delivery) error {
		return delivery.DeliverPasswordRecovery(ctx, recipient, token, expiresAt)
	})
}
func (a *Active) DeliverTenantRoleInvitation(ctx context.Context, recipient email.Address, role, token string, expiresAt time.Time) error {
	return a.WithActive(ctx, func(_ domain.ID, delivery *deliverysmtp.Delivery) error {
		return delivery.DeliverTenantRoleInvitation(ctx, recipient, role, token, expiresAt)
	})
}
func (a *Active) DeliverRendered(ctx context.Context, recipient email.Address, subject, text, html string) error {
	return a.WithActive(ctx, func(_ domain.ID, delivery *deliverysmtp.Delivery) error {
		return delivery.DeliverRendered(ctx, recipient, subject, text, html)
	})
}
func (a *Active) WithActive(ctx context.Context, use func(domain.ID, *deliverysmtp.Delivery) error) error {
	if ctx == nil || a == nil || a.store == nil || a.secrets == nil || use == nil {
		return errors.New("active_smtp_delivery_unavailable")
	}
	version, err := a.store.ActiveSMTP(ctx)
	if err != nil || version.ID.IsZero() || version.State != domain.Active || version.Kind != domain.SMTP || version.Configuration.SMTP == nil {
		return errors.New("active_smtp_delivery_unavailable")
	}
	cfg := version.Configuration.SMTP
	return a.secrets.WithSecret(ctx, cfg.CredentialSecretRef, func(rawCredentials []byte) error {
		credentials, parseErr := smtpsecret.ParseCredentials(rawCredentials)
		if parseErr != nil {
			return errors.New("active_smtp_delivery_unavailable")
		}
		return a.secrets.WithSecret(ctx, cfg.TrustRootsSecretRef, func(rawRoots []byte) error {
			roots, rootsErr := smtpsecret.ParseTrustRoots(rawRoots)
			if rootsErr != nil {
				return errors.New("active_smtp_delivery_unavailable")
			}
			sender, senderErr := email.Parse(cfg.SenderAddress)
			if senderErr != nil {
				return errors.New("active_smtp_delivery_unavailable")
			}
			reply := email.Address{}
			if cfg.ReplyToAddress != nil {
				var e error
				reply, e = email.Parse(*cfg.ReplyToAddress)
				if e != nil {
					return e
				}
			}
			delivery, deliveryErr := deliverysmtp.New(deliverysmtp.Config{Host: cfg.Host, Port: uint16(cfg.Port), Username: credentials.Username, Password: credentials.Password, Sender: sender, SenderName: cfg.SenderName, ReplyTo: reply, MaxConcurrency: cfg.MaxConcurrency, TLS: &tls.Config{ServerName: cfg.Host, RootCAs: roots, MinVersion: tls.VersionTLS12}, ConnectTimeout: time.Duration(cfg.ConnectTimeoutSeconds) * time.Second, CommandTimeout: time.Duration(cfg.CommandTimeoutSeconds) * time.Second, DeliveryTimeout: time.Duration(cfg.DeliveryTimeoutSeconds) * time.Second})
			if deliveryErr != nil {
				return errors.New("active_smtp_delivery_unavailable")
			}
			return use(version.ID, delivery)
		})
	})
}
