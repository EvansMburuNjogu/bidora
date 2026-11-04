package identity_test

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
)

func TestLoginEmailDeliversHTMLAndPlainTextWithSafeAction(t *testing.T) {
	fixture := newSMTPFixture(t, true)
	config := smtpConfig(t, fixture)
	config.ActionOrigin = "https://sandbox.bidora.africa"
	sender, err := deliverysmtp.New(config)
	if err != nil {
		t.Fatal(err)
	}
	recipient, _ := email.Parse("recipient@example.test")
	if err = sender.DeliverLoginCode(context.Background(), recipient, "123456", time.Now().Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	select {
	case raw := <-fixture.message:
		msg, err := mail.ReadMessage(strings.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		kind, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
		if err != nil || kind != "multipart/alternative" {
			t.Fatal("missing multipart alternative")
		}
		reader := multipart.NewReader(msg.Body, params["boundary"])
		for _, want := range []string{"text/plain", "text/html"} {
			part, err := reader.NextRawPart()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(part.Header.Get("Content-Type"), want) {
				t.Fatal("incorrect alternative order")
			}
			body, err := io.ReadAll(quotedprintable.NewReader(part))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), "123456") || !strings.Contains(string(body), "https://sandbox.bidora.africa/auth/login-code") {
				t.Fatal("code/action missing")
			}
			if strings.Contains(string(body), "?code=") || strings.Contains(string(body), "?token=") {
				t.Fatal("secret in action URL")
			}
		}
	case <-time.After(time.Second):
		t.Fatal("synthetic SMTP did not capture message")
	}
}
