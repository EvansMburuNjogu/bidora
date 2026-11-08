package identity_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/verification"
)

type smtpFixture struct {
	listener  net.Listener
	roots     *x509.CertPool
	serverTLS *tls.Config
	done      chan error
	message   chan string
}

func newSMTPFixture(t *testing.T, startTLS bool, bannerDelay ...time.Duration) *smtpFixture {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	certificate := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	fixture := &smtpFixture{listener: listener, roots: roots, serverTLS: &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}, done: make(chan error, 1), message: make(chan string, 1)}
	go func() {
		if len(bannerDelay) > 0 {
			time.Sleep(bannerDelay[0])
		}
		fixture.serve(startTLS)
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case err := <-fixture.done:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("SMTP fixture did not stop")
		}
	})
	return fixture
}
func (f *smtpFixture) serve(startTLS bool) {
	connection, err := f.listener.Accept()
	if err != nil {
		f.done <- err
		return
	}
	defer connection.Close()
	protocol := textproto.NewConn(connection)
	defer protocol.Close()
	if err = protocol.PrintfLine("220 localhost test SMTP"); err != nil {
		f.done <- err
		return
	}
	if _, err = expectSMTP(protocol, "EHLO "); err != nil {
		f.done <- err
		return
	}
	if startTLS {
		err = protocol.PrintfLine("250-localhost\r\n250-STARTTLS\r\n250 AUTH PLAIN")
	} else {
		err = protocol.PrintfLine("250 localhost")
	}
	if err != nil {
		f.done <- err
		return
	}
	if !startTLS {
		f.done <- nil
		return
	}
	if _, err = expectSMTP(protocol, "STARTTLS"); err != nil {
		f.done <- err
		return
	}
	if err = protocol.PrintfLine("220 begin TLS"); err != nil {
		f.done <- err
		return
	}
	tlsConnection := tls.Server(connection, f.serverTLS)
	if err = tlsConnection.Handshake(); err != nil {
		f.done <- err
		return
	}
	protocol = textproto.NewConn(tlsConnection)
	defer protocol.Close()
	if _, err = expectSMTP(protocol, "EHLO "); err != nil {
		f.done <- err
		return
	}
	if err = protocol.PrintfLine("250-localhost\r\n250 AUTH PLAIN"); err != nil {
		f.done <- err
		return
	}
	line, err := expectSMTP(protocol, "AUTH PLAIN ")
	if err != nil {
		f.done <- err
		return
	}
	encoded := strings.TrimPrefix(line, "AUTH PLAIN ")
	credentials, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || string(credentials) != "\x00smtp-user\x00smtp-password" {
		f.done <- errors.New("SMTP credentials drifted")
		return
	}
	if err = protocol.PrintfLine("235 authenticated"); err != nil {
		f.done <- err
		return
	}
	for _, prefix := range []string{"MAIL FROM:<sender@example.test>", "RCPT TO:<recipient@example.test>", "DATA"} {
		if _, err = expectSMTP(protocol, prefix); err != nil {
			f.done <- err
			return
		}
		code := "250 accepted"
		if prefix == "DATA" {
			code = "354 send data"
		}
		if err = protocol.PrintfLine("%s", code); err != nil {
			f.done <- err
			return
		}
	}
	body, err := protocol.ReadDotBytes()
	if err != nil {
		f.done <- err
		return
	}
	f.message <- string(body)
	if err = protocol.PrintfLine("250 queued"); err != nil {
		f.done <- err
		return
	}
	if _, err = expectSMTP(protocol, "QUIT"); err != nil {
		f.done <- err
		return
	}
	err = protocol.PrintfLine("221 bye")
	f.done <- err
}
func expectSMTP(protocol *textproto.Conn, prefix string) (string, error) {
	line, err := protocol.ReadLine()
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("unexpected SMTP command")
	}
	return line, nil
}
func smtpConfig(t *testing.T, fixture *smtpFixture) deliverysmtp.Config {
	t.Helper()
	sender, err := email.Parse("sender@example.test")
	if err != nil {
		t.Fatal(err)
	}
	return deliverysmtp.Config{Host: "127.0.0.1", Port: uint16(fixture.listener.Addr().(*net.TCPAddr).Port), Username: "smtp-user", Password: "smtp-password", Sender: sender, TLS: &tls.Config{ServerName: "localhost", RootCAs: fixture.roots, MinVersion: tls.VersionTLS12}, Timeout: time.Second}
}
func TestSMTPDeliveryRequiresVerifiedSTARTTLSAndSendsMinimalToken(t *testing.T) {
	fixture := newSMTPFixture(t, true)
	delivery, err := deliverysmtp.New(smtpConfig(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	recipient, _ := email.Parse("recipient@example.test")
	token, err := verification.GenerateWithRandom(tokenReader(9))
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().UTC().Add(verification.Lifetime).Truncate(time.Second)
	if err = delivery.DeliverEmailVerification(context.Background(), recipient, token, expires); err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-fixture.message:
		if !strings.Contains(message, token.RevealForDelivery()) || !strings.Contains(message, "To: recipient@example.test") || strings.Contains(strings.ToLower(message), "bcc:") {
			t.Fatal("SMTP message drifted")
		}
	case <-time.After(time.Second):
		t.Fatal("SMTP message missing")
	}
}
func TestSMTPDeliveryFailsClosedWithoutSTARTTLSAndRedactsFailure(t *testing.T) {
	fixture := newSMTPFixture(t, false)
	config := smtpConfig(t, fixture)
	delivery, err := deliverysmtp.New(config)
	if err != nil {
		t.Fatal(err)
	}
	recipient, _ := email.Parse("recipient@example.test")
	token, _ := verification.GenerateWithRandom(tokenReader(3))
	err = delivery.DeliverEmailVerification(context.Background(), recipient, token, time.Now().Add(verification.Lifetime))
	if err == nil || err.Error() != "email_delivery_unavailable" || strings.Contains(fmt.Sprintf("%#v", err), config.Password) || strings.Contains(fmt.Sprintf("%#v", err), config.Host) {
		t.Fatal("STARTTLS failure unsafe", err)
	}
}
func TestSMTPDeliveryConfigurationAndCancellationAreBounded(t *testing.T) {
	fixture := newSMTPFixture(t, true)
	valid := smtpConfig(t, fixture)
	for _, mutate := range []func(*deliverysmtp.Config){func(c *deliverysmtp.Config) { c.Host = "" }, func(c *deliverysmtp.Config) { c.Port = 0 }, func(c *deliverysmtp.Config) { c.Username = "" }, func(c *deliverysmtp.Config) { c.Password = "" }, func(c *deliverysmtp.Config) { c.TLS = nil }, func(c *deliverysmtp.Config) { c.TLS = &tls.Config{ServerName: "localhost", InsecureSkipVerify: true} }, func(c *deliverysmtp.Config) { c.Timeout = 0 }, func(c *deliverysmtp.Config) { c.Timeout = 31 * time.Second }} {
		config := valid
		config.TLS = valid.TLS.Clone()
		mutate(&config)
		if delivery, err := deliverysmtp.New(config); err == nil || delivery != nil {
			t.Fatal("unsafe SMTP config accepted")
		}
	}
	if formatted := fmt.Sprintf("%#v", valid); strings.Contains(formatted, "smtp-password") || strings.Contains(formatted, "sender@example") {
		t.Fatal("SMTP config formatting leaked")
	}
	delivery, _ := deliverysmtp.New(valid)
	recipient, _ := email.Parse("recipient@example.test")
	token, _ := verification.GenerateWithRandom(tokenReader(2))
	if err := delivery.DeliverEmailVerification(nil, recipient, token, time.Now()); err == nil || err.Error() != "email_delivery_invalid" {
		t.Fatal("nil delivery context accepted")
	}
}
