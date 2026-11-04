// Package deliverysmtp delivers email-verification tokens through explicit,
// authenticated STARTTLS SMTP. It performs no provider discovery or logging.
package deliverysmtp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"bidora/backend/internal/emailhtml"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/verification"
)

type Config struct {
	ActionOrigin   string // Explicit browser origin for token-free quick actions.
	SenderName     string
	ReplyTo        email.Address
	MaxConcurrency int32
	Host           string
	Port           uint16
	Username       string
	Password       string
	Sender         email.Address
	TLS            *tls.Config
	Timeout        time.Duration
}

func (Config) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED SMTP delivery config]") }

type Delivery struct{ config Config }

type Prepared struct {
	recipient email.Address
	message   string
}

func (Prepared) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED prepared email]") }
func (p Prepared) BytesForFingerprint() []byte { return []byte(p.message) }

func New(config Config) (*Delivery, error) {
	if !validConfig(config) {
		return nil, errors.New("smtp_delivery_config_invalid")
	}
	owned := config
	owned.TLS = config.TLS.Clone()
	if owned.TLS.MinVersion == 0 {
		owned.TLS.MinVersion = tls.VersionTLS12
	}
	if owned.MaxConcurrency == 0 {
		owned.MaxConcurrency = 1
	}
	return &Delivery{owned}, nil
}
func (d *Delivery) PrepareEmailVerification(recipient email.Address, token verification.Token, expiresAt time.Time) (Prepared, error) {
	if d == nil || !validConfig(d.config) {
		return Prepared{}, errors.New("email_delivery_unavailable")
	}
	if recipient.IsZero() || token.IsZero() || expiresAt.IsZero() {
		return Prepared{}, errors.New("email_delivery_invalid")
	}
	return d.prepareStyled(recipient, "Verify your email", "identity.email_verification", token.RevealForDelivery(), expiresAt, verificationMessage(d.config.Sender, recipient, token, expiresAt.UTC()))
}
func (d *Delivery) PreparePasswordRecovery(recipient email.Address, token verification.Token, expiresAt time.Time) (Prepared, error) {
	if d == nil || !validConfig(d.config) {
		return Prepared{}, errors.New("email_delivery_unavailable")
	}
	if recipient.IsZero() || token.IsZero() || expiresAt.IsZero() {
		return Prepared{}, errors.New("email_delivery_invalid")
	}
	return d.prepareStyled(recipient, "Reset your password", "identity.password_recovery", token.RevealForDelivery(), expiresAt, passwordRecoveryMessage(d.config.Sender, recipient, token, expiresAt.UTC()))
}
func (d *Delivery) prepareStyled(recipient email.Address, title, key, code string, expiry time.Time, legacy string) (Prepared, error) {
	_, text, ok := strings.Cut(legacy, "\r\n\r\n")
	if !ok {
		return Prepared{}, errors.New("email_delivery_invalid")
	}
	body := strings.ReplaceAll(text, code, "(shown below)")
	if action := emailhtml.QuickAction(d.config.ActionOrigin, key); action.URL != "" {
		text += "\n" + action.Label + ": " + action.URL
	}
	html := emailhtml.Render(emailhtml.Content{Brand: d.config.SenderName, Title: title, Body: body, Code: code, Expiry: expiry.UTC().Format(time.RFC3339), Origin: d.config.ActionOrigin, Key: key})
	message, err := renderedMessage(d.config.Sender, recipient, title, text, html)
	return Prepared{recipient: recipient, message: message}, err
}

func (d *Delivery) DeliverPrepared(ctx context.Context, prepared Prepared) error {
	if ctx == nil || prepared.recipient.IsZero() || prepared.message == "" {
		return errors.New("email_delivery_invalid")
	}
	return d.deliver(ctx, prepared.recipient, prepared.message)
}
func (d *Delivery) DeliverEmailVerification(ctx context.Context, recipient email.Address, token verification.Token, expiresAt time.Time) error {
	prepared, err := d.PrepareEmailVerification(recipient, token, expiresAt)
	if err != nil {
		return err
	}
	return d.DeliverPrepared(ctx, prepared)
}
func (d *Delivery) DeliverPasswordRecovery(ctx context.Context, recipient email.Address, token verification.Token, expiresAt time.Time) error {
	prepared, err := d.PreparePasswordRecovery(recipient, token, expiresAt)
	if err != nil {
		return err
	}
	return d.DeliverPrepared(ctx, prepared)
}
func (d *Delivery) EmailLoginAvailable(context.Context) bool {
	return d != nil && validConfig(d.config)
}
func (d *Delivery) DeliverLoginCode(ctx context.Context, recipient email.Address, code string, expiresAt time.Time) error {
	if d == nil || !validConfig(d.config) {
		return errors.New("email_delivery_unavailable")
	}
	if recipient.IsZero() || !validLoginCode(code) || expiresAt.IsZero() {
		return errors.New("email_delivery_invalid")
	}
	expiry := expiresAt.UTC().Format("2 Jan 2006, 15:04 UTC")
	text := "Use this six-digit code to finish signing in to Bidora:\n\n" + code + "\n\nExpires " + expiry + ". Use it once and never share it.\nIf you did not attempt to sign in, ignore this message."
	action := emailhtml.QuickAction(d.config.ActionOrigin, "identity.login_code")
	if action.URL != "" {
		text += "\n\n" + action.Label + ": " + action.URL
	}
	html := emailhtml.Render(emailhtml.Content{Brand: d.config.SenderName, Title: "Verify your sign-in", Body: "Enter this code to continue signing in. It works once.", Code: code, Expiry: expiry, Origin: d.config.ActionOrigin, Key: "identity.login_code"})
	message, err := renderedMessage(d.config.Sender, recipient, "Your Bidora sign-in code", text, html)
	if err != nil {
		return err
	}
	return d.deliver(ctx, recipient, message)
}
func (d *Delivery) DeliverTenantInvitation(ctx context.Context, recipient email.Address, rawToken string, expiresAt time.Time) error {
	return d.DeliverTenantRoleInvitation(ctx, recipient, "CONTRIBUTOR", rawToken, expiresAt)
}

func (d *Delivery) PrepareTenantRoleInvitation(recipient email.Address, role, rawToken string, expiresAt time.Time) (Prepared, error) {
	if d == nil || !validConfig(d.config) {
		return Prepared{}, errors.New("email_delivery_unavailable")
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(rawToken)
	if recipient.IsZero() || !validTenantRole(role) || err != nil || len(decoded) != 32 || expiresAt.IsZero() {
		return Prepared{}, errors.New("email_delivery_invalid")
	}
	return d.prepareStyled(recipient, "You are invited to a workspace", "tenant.invitation", rawToken, expiresAt, invitationMessage(d.config.Sender, recipient, role, rawToken, expiresAt.UTC()))
}
func (d *Delivery) DeliverTenantRoleInvitation(ctx context.Context, recipient email.Address, role, rawToken string, expiresAt time.Time) error {
	prepared, err := d.PrepareTenantRoleInvitation(recipient, role, rawToken, expiresAt)
	if err != nil {
		return err
	}
	return d.DeliverPrepared(ctx, prepared)
}

// DeliverRendered delivers a pre-rendered provider-neutral notification. The
// caller owns template selection; this adapter only validates and serializes a
// multipart message before sending it through the configured SMTP transport.
func (d *Delivery) PrepareRendered(recipient email.Address, subject, text, html string) (Prepared, error) {
	if d == nil || !validConfig(d.config) {
		return Prepared{}, errors.New("email_delivery_unavailable")
	}
	message, err := renderedMessage(d.config.Sender, recipient, subject, text, html)
	if recipient.IsZero() || err != nil {
		return Prepared{}, errors.New("email_delivery_invalid")
	}
	return Prepared{recipient: recipient, message: message}, nil
}
func (d *Delivery) DeliverRendered(ctx context.Context, recipient email.Address, subject, text, html string) error {
	prepared, err := d.PrepareRendered(recipient, subject, text, html)
	if err != nil {
		return err
	}
	return d.DeliverPrepared(ctx, prepared)
}

func (d *Delivery) deliver(ctx context.Context, recipient email.Address, message string) error {
	if ctx == nil || recipient.IsZero() || message == "" {
		return errors.New("email_delivery_invalid")
	}
	if d == nil || !validConfig(d.config) {
		return errors.New("email_delivery_unavailable")
	}
	if !acquireSMTP(d.config.MaxConcurrency) {
		return safe(errors.New("smtp_concurrency_exhausted"))
	}
	defer releaseSMTP()
	var headerErr error
	message, headerErr = d.policyHeaders(message)
	if headerErr != nil {
		return headerErr
	}
	address := net.JoinHostPort(d.config.Host, strconv.Itoa(int(d.config.Port)))
	dialer := net.Dialer{Timeout: d.config.Timeout}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return safe(err)
	}
	defer connection.Close()
	deadline := time.Now().Add(d.config.Timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err = connection.SetDeadline(deadline); err != nil {
		return safe(err)
	}
	stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stop()
	client, err := smtp.NewClient(connection, d.config.TLS.ServerName)
	if err != nil {
		return safe(err)
	}
	defer client.Close()
	if supported, _ := client.Extension("STARTTLS"); !supported {
		return safe(errors.New("STARTTLS unavailable"))
	}
	if err = client.StartTLS(d.config.TLS.Clone()); err != nil {
		return safe(err)
	}
	ok, mechanisms := client.Extension("AUTH")
	if !ok || !containsPlain(mechanisms) {
		return safe(errors.New("SMTP AUTH PLAIN unavailable"))
	}
	if err = client.Auth(smtp.PlainAuth("", d.config.Username, d.config.Password, d.config.TLS.ServerName)); err != nil {
		return safe(err)
	}
	if err = client.Mail(d.config.Sender.Display()); err != nil {
		return safe(err)
	}
	if err = client.Rcpt(recipient.Display()); err != nil {
		return safe(err)
	}
	writer, err := client.Data()
	if err != nil {
		return safe(err)
	}
	if _, err = io.WriteString(writer, message); err != nil {
		_ = writer.Close()
		return safe(err)
	}
	if err = writer.Close(); err != nil {
		return safe(err)
	}
	if err = client.Quit(); err != nil {
		return safe(err)
	}
	return nil
}
func validConfig(config Config) bool {
	if config.MaxConcurrency < 0 || config.MaxConcurrency > 100 || !utf8.ValidString(config.SenderName) || len(config.SenderName) > 200 || strings.IndexFunc(config.SenderName, unicode.IsControl) >= 0 {
		return false
	}
	return validHost(config.Host) && config.Port > 0 && config.Username != "" && len(config.Username) <= 256 && strings.TrimSpace(config.Username) == config.Username && config.Password != "" && len(config.Password) <= 1024 && !strings.ContainsRune(config.Password, 0) && !config.Sender.IsZero() && config.TLS != nil && !config.TLS.InsecureSkipVerify && config.TLS.ServerName != "" && validHost(config.TLS.ServerName) && config.TLS.RootCAs != nil && len(config.TLS.RootCAs.Subjects()) > 0 && (config.TLS.MinVersion == 0 || config.TLS.MinVersion >= tls.VersionTLS12) && (config.TLS.MaxVersion == 0 || config.TLS.MaxVersion >= tls.VersionTLS12) && (config.TLS.MinVersion == 0 || config.TLS.MaxVersion == 0 || config.TLS.MinVersion <= config.TLS.MaxVersion) && config.Timeout >= time.Millisecond && config.Timeout <= 30*time.Second
}

var smtpConcurrency struct {
	sync.Mutex
	active int32
}

func acquireSMTP(limit int32) bool {
	smtpConcurrency.Lock()
	defer smtpConcurrency.Unlock()
	if limit < 1 || smtpConcurrency.active >= limit {
		return false
	}
	smtpConcurrency.active++
	return true
}
func releaseSMTP() { smtpConcurrency.Lock(); smtpConcurrency.active--; smtpConcurrency.Unlock() }
func (d *Delivery) policyHeaders(message string) (string, error) {
	prefix := "From: " + d.config.Sender.Display() + "\r\n"
	if !strings.HasPrefix(message, prefix) {
		return "", errors.New("email_delivery_invalid")
	}
	from := d.config.Sender.Display()
	if d.config.SenderName != "" {
		from = (&mail.Address{Name: d.config.SenderName, Address: d.config.Sender.Display()}).String()
	}
	headers := "From: " + from + "\r\n"
	if !d.config.ReplyTo.IsZero() {
		headers += "Reply-To: " + d.config.ReplyTo.Display() + "\r\n"
	}
	return headers + strings.TrimPrefix(message, prefix), nil
}

func validHost(value string) bool {
	if ip := net.ParseIP(value); ip != nil {
		return !ip.IsUnspecified() && !ip.IsMulticast()
	}
	if len(value) == 0 || len(value) > 253 {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range []byte(label) {
			if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' {
				continue
			}
			return false
		}
	}
	return true
}
func containsPlain(mechanisms string) bool {
	for _, mechanism := range strings.Fields(mechanisms) {
		if strings.EqualFold(mechanism, "PLAIN") {
			return true
		}
	}
	return false
}
func validTenantRole(role string) bool {
	switch role {
	case "ORGANIZATION_ADMIN", "BID_MANAGER", "CONTRIBUTOR", "APPROVER":
		return true
	}
	return false
}
func invitationMessage(sender, recipient email.Address, role, rawToken string, expiresAt time.Time) string {
	createdAt := expiresAt.Add(-72 * time.Hour)
	return "From: " + sender.Display() + "\r\nTo: " + recipient.Display() + "\r\nSubject: You are invited to a Bidora workspace\r\nDate: " + createdAt.Format(time.RFC1123Z) + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 7bit\r\n\r\nUse this invitation token to join the Bidora workspace with the fixed role " + role + ":\r\n\r\n" + rawToken + "\r\n\r\nThis invitation expires at " + expiresAt.Format(time.RFC3339) + ".\r\nIf you did not expect this invitation, ignore this message.\r\n"
}
func validLoginCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for i := range code {
		if code[i] < '0' || code[i] > '9' {
			return false
		}
	}
	return true
}
func loginCodeMessage(sender, recipient email.Address, code string, expiresAt time.Time) string {
	createdAt := expiresAt.Add(-5 * time.Minute)
	return "From: " + sender.Display() + "\r\nTo: " + recipient.Display() + "\r\nSubject: Your Bidora sign-in code\r\nDate: " + createdAt.Format(time.RFC1123Z) + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 7bit\r\n\r\nUse this verification code to finish signing in to Bidora:\r\n\r\n" + code + "\r\n\r\nThis code expires at " + expiresAt.Format(time.RFC3339) + " and can be used once.\r\nIf you did not attempt to sign in, ignore this message.\r\n"
}
func passwordRecoveryMessage(sender, recipient email.Address, token verification.Token, expiresAt time.Time) string {
	createdAt := expiresAt.Add(-verification.Lifetime)
	return "From: " + sender.Display() + "\r\nTo: " + recipient.Display() + "\r\nSubject: Reset your Bidora password\r\nDate: " + createdAt.Format(time.RFC1123Z) + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 7bit\r\n\r\nUse this password recovery token to reset your Bidora password:\r\n\r\n" + token.RevealForDelivery() + "\r\n\r\nThis token expires at " + expiresAt.Format(time.RFC3339) + ".\r\nIf you did not request this, ignore this message.\r\n"
}
func verificationMessage(sender, recipient email.Address, token verification.Token, expiresAt time.Time) string {
	createdAt := expiresAt.Add(-verification.Lifetime)
	return "From: " + sender.Display() + "\r\nTo: " + recipient.Display() + "\r\nSubject: Verify your Bidora email address\r\nDate: " + createdAt.Format(time.RFC1123Z) + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 7bit\r\n\r\nUse this verification token to verify your Bidora email address:\r\n\r\n" + token.RevealForDelivery() + "\r\n\r\nThis token expires at " + expiresAt.Format(time.RFC3339) + ".\r\nIf you did not request this, ignore this message.\r\n"
}

func renderedMessage(sender, recipient email.Address, subject, textBody, htmlBody string) (string, error) {
	if sender.IsZero() || recipient.IsZero() || strings.TrimSpace(subject) == "" || strings.ContainsAny(subject, "\r\n") || textBody == "" || htmlBody == "" {
		return "", errors.New("email_delivery_invalid")
	}
	var body bytes.Buffer
	parts := multipart.NewWriter(&body)
	writePart := func(contentType, value string) error {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Type", contentType+"; charset=UTF-8")
		header.Set("Content-Transfer-Encoding", "quoted-printable")
		part, err := parts.CreatePart(header)
		if err != nil {
			return err
		}
		encoded := quotedprintable.NewWriter(part)
		if _, err = io.WriteString(encoded, value); err != nil {
			_ = encoded.Close()
			return err
		}
		return encoded.Close()
	}
	if err := writePart("text/plain", textBody); err != nil {
		return "", err
	}
	if err := writePart("text/html", htmlBody); err != nil {
		return "", err
	}
	if err := parts.Close(); err != nil {
		return "", err
	}
	// RFC 2047 encoding and folding keeps managed Unicode subjects valid without
	// allowing content to introduce headers.
	encodedSubject := strings.ReplaceAll(mime.QEncoding.Encode("UTF-8", subject), "?= =?", "?=\r\n =?")
	message := "From: " + sender.Display() + "\r\nTo: " + recipient.Display() + "\r\nSubject: " + encodedSubject + "\r\nDate: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=\"" + parts.Boundary() + "\"\r\n\r\n" + body.String()
	return message, nil
}

type deliveryError struct{ cause error }

func (e *deliveryError) Error() string                 { return "email_delivery_unavailable" }
func (e *deliveryError) Unwrap() error                 { return e.cause }
func (e *deliveryError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
func safe(cause error) error                           { return &deliveryError{cause} }
