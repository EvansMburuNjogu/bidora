package integration_test

import (
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
)

type pilotSMTPFixture struct {
	listener  net.Listener
	roots     *x509.CertPool
	serverTLS *tls.Config
	done      chan error
	message   chan string
	recipient string
	expected  int
}

func newPilotSMTPFixture(t *testing.T, recipient string) *pilotSMTPFixture {
	return newPilotSMTPFixtureMessages(t, recipient, 1)
}

func newPilotSMTPFixtureMessages(t *testing.T, recipient string, expected int) *pilotSMTPFixture {
	t.Helper()
	if expected < 1 {
		t.Fatal("SMTP fixture message count invalid")
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(19), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	certificate := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})) {
		t.Fatal("SMTP fixture trust root rejected")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	fixture := &pilotSMTPFixture{listener: listener, roots: roots, serverTLS: &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}, done: make(chan error, 1), message: make(chan string, expected), recipient: recipient, expected: expected}
	go fixture.serve()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case err := <-fixture.done:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("pilot SMTP fixture did not stop")
		}
	})
	return fixture
}

func (f *pilotSMTPFixture) serve() {
	for index := 0; index < f.expected; index++ {
		connection, err := f.listener.Accept()
		if err != nil {
			f.done <- err
			return
		}
		if err = f.serveConnection(connection); err != nil {
			_ = connection.Close()
			f.done <- err
			return
		}
	}
	f.done <- nil
}

func (f *pilotSMTPFixture) serveConnection(connection net.Conn) error {
	defer connection.Close()
	protocol := textproto.NewConn(connection)
	defer protocol.Close()
	if err := protocol.PrintfLine("220 localhost test SMTP"); err != nil {
		return err
	}
	if _, err := expectPilotSMTP(protocol, "EHLO "); err != nil {
		return err
	}
	if err := protocol.PrintfLine("250-localhost\r\n250-STARTTLS\r\n250 AUTH PLAIN"); err != nil {
		return err
	}
	if _, err := expectPilotSMTP(protocol, "STARTTLS"); err != nil {
		return err
	}
	if err := protocol.PrintfLine("220 begin TLS"); err != nil {
		return err
	}
	tlsConnection := tls.Server(connection, f.serverTLS)
	if err := tlsConnection.Handshake(); err != nil {
		return err
	}
	protocol = textproto.NewConn(tlsConnection)
	defer protocol.Close()
	if _, err := expectPilotSMTP(protocol, "EHLO "); err != nil {
		return err
	}
	if err := protocol.PrintfLine("250-localhost\r\n250 AUTH PLAIN"); err != nil {
		return err
	}
	line, err := expectPilotSMTP(protocol, "AUTH PLAIN ")
	if err != nil {
		return err
	}
	credentials, decodeErr := base64.StdEncoding.DecodeString(strings.TrimPrefix(line, "AUTH PLAIN "))
	if decodeErr != nil || string(credentials) != "\x00smtp-user\x00smtp-password" {
		return errors.New("SMTP credentials drifted")
	}
	if err = protocol.PrintfLine("235 authenticated"); err != nil {
		return err
	}
	for _, command := range []struct{ prefix, response string }{{"MAIL FROM:<noreply@example.invalid>", "250 accepted"}, {"RCPT TO:<" + f.recipient + ">", "250 accepted"}, {"DATA", "354 send data"}} {
		if _, err = expectPilotSMTP(protocol, command.prefix); err != nil {
			return err
		}
		if err = protocol.PrintfLine("%s", command.response); err != nil {
			return err
		}
	}
	body, err := protocol.ReadDotBytes()
	if err != nil {
		return err
	}
	f.message <- string(body)
	if err = protocol.PrintfLine("250 queued"); err != nil {
		return err
	}
	if _, err = expectPilotSMTP(protocol, "QUIT"); err != nil {
		return err
	}
	return protocol.PrintfLine("221 bye")
}

func expectPilotSMTP(protocol *textproto.Conn, prefix string) (string, error) {
	line, err := protocol.ReadLine()
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("unexpected SMTP command")
	}
	return line, nil
}
