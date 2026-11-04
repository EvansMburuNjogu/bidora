package pilot

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strconv"
	"time"

	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/platformsettings/smtpsecret"
)

// PlatformLoginEmailConfig is explicit deployment configuration for login OTP
// only. Secret references resolve encrypted envelopes using deployment key
// material. It neither activates managed SMTP nor becomes an invitation,
// recovery, or ordinary-user delivery fallback.
type PlatformLoginEmailConfig struct {
	Host                                string
	Port                                uint16
	Sender                              string
	CredentialReference, TrustReference string
}

func (PlatformLoginEmailConfig) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED platform login email configuration]")
}
func LoadPlatformLoginEmailConfig(getenv func(string) string) (*PlatformLoginEmailConfig, error) {
	raw := getenv("BIDORA_PLATFORM_EMAIL_OTP")
	if raw == "" || raw == "false" {
		return nil, nil
	}
	if raw != "true" {
		return nil, errors.New("platform_login_email_config_invalid")
	}
	p, err := strconv.ParseUint(getenv("BIDORA_PLATFORM_LOGIN_SMTP_PORT"), 10, 16)
	if err != nil || p == 0 {
		return nil, errors.New("platform_login_email_config_invalid")
	}
	c := &PlatformLoginEmailConfig{Host: getenv("BIDORA_PLATFORM_LOGIN_SMTP_HOST"), Port: uint16(p), Sender: getenv("BIDORA_PLATFORM_LOGIN_SMTP_SENDER"), CredentialReference: getenv("BIDORA_PLATFORM_LOGIN_SMTP_CREDENTIAL_REF"), TrustReference: getenv("BIDORA_PLATFORM_LOGIN_SMTP_TRUST_REF")}
	if c.Host == "" || c.CredentialReference == "" || c.TrustReference == "" {
		return nil, errors.New("platform_login_email_config_invalid")
	}
	if _, err = email.Parse(c.Sender); err != nil {
		return nil, errors.New("platform_login_email_config_invalid")
	}
	return c, nil
}

type platformLoginMail struct {
	origin string
	config PlatformLoginEmailConfig
	vault  smtpsecret.Resolver
}

func (m *platformLoginMail) DeliverLoginCode(ctx context.Context, to email.Address, code string, expires time.Time) error {
	if ctx == nil || m == nil || m.vault == nil {
		return errors.New("platform_login_email_unavailable")
	}
	sender, err := email.Parse(m.config.Sender)
	if err != nil {
		return errors.New("platform_login_email_unavailable")
	}
	return m.vault.WithSecret(ctx, m.config.CredentialReference, func(credentials []byte) error {
		c, err := smtpsecret.ParseCredentials(credentials)
		if err != nil {
			return err
		}
		return m.vault.WithSecret(ctx, m.config.TrustReference, func(pem []byte) error {
			roots, err := smtpsecret.ParseTrustRoots(pem)
			if err != nil {
				return err
			}
			transport, err := deliverysmtp.New(deliverysmtp.Config{ActionOrigin: m.origin, Host: m.config.Host, Port: m.config.Port, Username: c.Username, Password: c.Password, Sender: sender, SenderName: "Bidora", TLS: &tls.Config{ServerName: m.config.Host, RootCAs: roots, MinVersion: tls.VersionTLS12}, Timeout: 20 * time.Second, MaxConcurrency: 2})
			if err != nil {
				return err
			}
			return transport.DeliverLoginCode(ctx, to, code, expires)
		})
	})
}
