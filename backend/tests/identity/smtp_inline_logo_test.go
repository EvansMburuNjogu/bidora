package identity_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/emailhtml"
	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/verification"
)

func TestSMTPEmbedsExactBidoraLogoWithoutRemoteImageDependency(t *testing.T) {
	fixture := newSMTPFixture(t, true)
	delivery, err := deliverysmtp.New(smtpConfig(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	recipient, _ := email.Parse("recipient@example.test")
	token, _ := verification.GenerateWithRandom(tokenReader(5))
	if err = delivery.DeliverEmailVerification(context.Background(), recipient, token, time.Now().Add(verification.Lifetime)); err != nil {
		t.Fatal(err)
	}
	var raw string
	select {
	case raw = <-fixture.message:
	case <-time.After(time.Second):
		t.Fatal("missing local fixture message")
	}
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	media, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || media != "multipart/related" {
		t.Fatal("missing related MIME container")
	}
	related := multipart.NewReader(msg.Body, params["boundary"])
	alternative, err := related.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	media, params, err = mime.ParseMediaType(alternative.Header.Get("Content-Type"))
	if err != nil || media != "multipart/alternative" {
		t.Fatal("missing plain text alternative")
	}
	parts := multipart.NewReader(alternative, params["boundary"])
	plain, err := parts.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	plainText, _ := io.ReadAll(plain)
	if !bytes.Contains(plainText, []byte(token.RevealForDelivery())) {
		t.Fatal("plain text lost code")
	}
	html, err := parts.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	htmlBody, _ := io.ReadAll(html)
	if !bytes.Contains(htmlBody, []byte("cid:"+emailhtml.LogoContentID)) || bytes.Contains(htmlBody, []byte("data:image/")) {
		t.Fatal("HTML does not use inline logo")
	}
	image, err := related.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if image.Header.Get("Content-ID") != "<"+emailhtml.LogoContentID+">" || image.Header.Get("Content-Type") != "image/png" {
		t.Fatal("incorrect inline image metadata")
	}
	png, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, image))
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile("../../../web/public/bidora-aperture-192.png")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(png, original) {
		t.Fatal("email logo differs from the app logo")
	}
}
