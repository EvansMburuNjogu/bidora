// Package smtptest performs a bounded synthetic SMTP test using externally
// resolved secrets. It persists and logs no secret or provider diagnostic.
package smtptest

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"html"
	"net"
	"net/textproto"
	"strings"
	"time"

	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	settingsapp "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"bidora/backend/internal/platformsettings/smtpsecret"
)

// SecretResolver owns secret retrieval and lifetime. Implementations should
// clear temporary material after use; callers must not retain the supplied bytes.
type SecretResolver = smtpsecret.Resolver

type RecipientDirectory interface {
	VerifiedEmail(context.Context, domain.ID) (email.Address, error)
}

type BrandingReader interface {
	Shared(context.Context) (settingsapp.Information, error)
}

type Tester struct {
	branding   BrandingReader
	secrets    SecretResolver
	recipients RecipientDirectory
}

func New(secrets SecretResolver, recipients RecipientDirectory, branding ...BrandingReader) (*Tester, error) {
	if secrets == nil || recipients == nil {
		return nil, errors.New("smtp_test_invalid")
	}
	out := &Tester{secrets: secrets, recipients: recipients}
	if len(branding) > 0 {
		out.branding = branding[0]
	}
	return out, nil
}

func (t *Tester) Test(ctx context.Context, configuration domain.Configuration, actorID domain.ID) (settingsapp.TestOutcome, error) {
	if ctx == nil || t == nil || t.secrets == nil || t.recipients == nil || actorID.IsZero() || configuration.Kind != domain.SMTP || configuration.SMTP == nil || !configuration.Valid() {
		return settingsapp.TestOutcome{Passed: false, ResultCode: "INVALID_RESPONSE"}, nil
	}
	recipient, err := t.recipients.VerifiedEmail(ctx, actorID)
	if err != nil || recipient.IsZero() {
		return settingsapp.TestOutcome{Passed: false, ResultCode: "INVALID_RESPONSE"}, nil
	}
	subject := "Bidora SMTP configuration test"
	textBody := "This synthetic message checks the configured SMTP transport. Acceptance is not proof of recipient delivery."
	htmlBody := "<p>" + html.EscapeString(textBody) + "</p>"
	if t.branding != nil {
		info, e := t.branding.Shared(ctx)
		if e != nil {
			return settingsapp.TestOutcome{ResultCode: "INVALID_RESPONSE"}, nil
		}
		subject = info.PlatformName + " SMTP configuration test"
		footer := info.PlatformName
		for _, v := range []*string{info.SupportEmail, info.Website, &info.ContactPhone} {
			if v != nil && *v != "" {
				footer += " · " + *v
			}
		}
		textBody += "\n\n" + footer
		htmlBody += "<p>" + html.EscapeString(footer) + "</p>"
	}
	cfg := configuration.SMTP
	outcome := settingsapp.TestOutcome{Passed: false, ResultCode: "AUTHENTICATION_FAILED"}
	err = t.secrets.WithSecret(ctx, cfg.CredentialSecretRef, func(rawCredentials []byte) error {
		credentials, parseErr := smtpsecret.ParseCredentials(rawCredentials)
		if parseErr != nil {
			return parseErr
		}
		outcome.ResultCode = "TLS_FAILED"
		return t.secrets.WithSecret(ctx, cfg.TrustRootsSecretRef, func(rawRoots []byte) error {
			roots, rootsErr := smtpsecret.ParseTrustRoots(rawRoots)
			if rootsErr != nil {
				return rootsErr
			}
			sender, parseErr := email.Parse(cfg.SenderAddress)
			if parseErr != nil {
				return parseErr
			}
			timeout := minimumDuration(time.Duration(cfg.ConnectTimeoutSeconds)*time.Second, time.Duration(cfg.CommandTimeoutSeconds)*time.Second, time.Duration(cfg.DeliveryTimeoutSeconds)*time.Second)
			reply := email.Address{}
			if cfg.ReplyToAddress != nil {
				var e error
				reply, e = email.Parse(*cfg.ReplyToAddress)
				if e != nil {
					return e
				}
			}
			delivery, createErr := deliverysmtp.New(deliverysmtp.Config{Host: cfg.Host, Port: uint16(cfg.Port), Username: credentials.Username, Password: credentials.Password, Sender: sender, SenderName: cfg.SenderName, ReplyTo: reply, MaxConcurrency: cfg.MaxConcurrency, TLS: &tls.Config{ServerName: cfg.Host, RootCAs: roots, MinVersion: tls.VersionTLS12}, Timeout: timeout})
			if createErr != nil {
				return createErr
			}
			deliveryCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.DeliveryTimeoutSeconds)*time.Second)
			defer cancel()
			deliverErr := delivery.DeliverRendered(deliveryCtx, recipient, subject, textBody, htmlBody)
			if deliverErr != nil {
				outcome.ResultCode = classify(deliverErr)
				return deliverErr
			}
			outcome = settingsapp.TestOutcome{Passed: true, ResultCode: "SMTP_STARTTLS_VERIFIED"}
			return nil
		})
	})
	// Expected secret/provider failures are represented only by the allowlisted
	// outcome code. The application records no diagnostic or secret material.
	if err != nil {
		return outcome, nil
	}
	return outcome, nil
}

func classify(err error) string {
	var unknownAuthority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var certificateInvalid x509.CertificateInvalidError
	var smtpError *textproto.Error
	var networkError net.Error
	text := strings.ToLower(err.Error())
	switch {
	case errors.As(err, &unknownAuthority), errors.As(err, &hostname), errors.As(err, &certificateInvalid), strings.Contains(text, "starttls"), strings.Contains(text, "tls"):
		return "TLS_FAILED"
	case errors.As(err, &smtpError) && smtpError.Code >= 500, strings.Contains(text, "auth"), strings.Contains(text, "535"):
		return "AUTHENTICATION_FAILED"
	case errors.As(err, &networkError), errors.Is(err, context.DeadlineExceeded):
		return "CONNECTIVITY_FAILED"
	default:
		return "PROVIDER_REJECTED"
	}
}
func minimumDuration(values ...time.Duration) time.Duration {
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}
