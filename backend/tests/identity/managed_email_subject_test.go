package identity_test

import (
	"bufio"
	"bytes"
	"mime"
	"net/mail"
	"strings"
	"testing"

	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
)

func TestManagedUnicodeSubjectUsesSafeMIMEEncoding(t *testing.T) {
	fixture := newSMTPFixture(t, true)
	sender, err := deliverysmtp.New(smtpConfig(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	recipient, err := email.Parse("recipient@example.test")
	if err != nil {
		t.Fatal(err)
	}
	subject := "Platform " + strings.Repeat("é", 220)
	prepared, err := sender.PrepareRendered(recipient, subject, "Plain content", "<p>HTML content</p>")
	if err != nil {
		t.Fatal(err)
	}
	raw := prepared.BytesForFingerprint()
	message, err := mail.ReadMessage(bufio.NewReader(bytes.NewReader(raw)))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
	if err != nil || decoded != subject {
		t.Fatal("Unicode subject not preserved", err)
	}
	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 998 {
			t.Fatal("SMTP header/body line exceeds protocol limit")
		}
	}
	if _, err = sender.PrepareRendered(recipient, "Subject\r\nBcc: attacker@example.test", "Plain", "<p>HTML</p>"); err == nil {
		t.Fatal("subject header injection accepted")
	}
}
