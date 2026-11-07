package notification_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	app "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/domain"
	"bidora/backend/internal/notification/message"
)

func TestCommunicationSnapshotEncryptsRedactsAndBindsNotification(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	codec, err := message.New(key, 1)
	if err != nil {
		t.Fatal(err)
	}
	token := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
	content := message.Content{Recipient: "recipient@example.test", Subject: "Recovery code 123456", Text: "prefix123456suffix. Code: " + token + "\nhttps://example.test/recovery_email/user#code=" + token + "\nChange your password."}
	encrypted, err := codec.Seal(domain.ID{1}, content)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{content.Recipient, token, "123456", "Change your password"} {
		if bytes.Contains(encrypted.Ciphertext, []byte(secret)) {
			t.Fatal("plaintext persisted")
		}
	}
	decoded, err := codec.Open(domain.ID{1}, encrypted)
	if err != nil || decoded.Recipient != content.Recipient || !strings.Contains(decoded.Text, "Change your password") {
		t.Fatal("snapshot roundtrip failed", err)
	}
	for _, secret := range []string{token, "123456", "https://"} {
		if strings.Contains(decoded.Text, secret) || strings.Contains(decoded.Subject, secret) {
			t.Fatal("secret in preview")
		}
	}
	if _, err = codec.Open(domain.ID{2}, encrypted); err == nil {
		t.Fatal("snapshot swapped across notifications")
	}
	other, _ := message.New(bytes.Repeat([]byte{8}, 32), 1)
	if _, err = other.Open(domain.ID{1}, encrypted); err == nil {
		t.Fatal("wrong key accepted")
	}
	encrypted.KeyVersion = 2
	if _, err = codec.Open(domain.ID{1}, encrypted); err == nil {
		t.Fatal("wrong version accepted")
	}
}
func TestCommunicationFiltersAreValidatedBeforeReading(t *testing.T) {
	at := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	f := &historyScopeFixture{}
	service, _ := app.NewPlatformHistoryService(f, &templateIDs{}, templateClock{at: at})
	subject := app.HistorySubject{UserID: domain.ID{1}, SessionID: domain.ID{2}, CorrelationID: domain.ID{3}, ContextGeneration: 1}
	for _, filter := range []app.HistoryFilters{{Category: "bad category"}, {Status: "DELIVERED"}, {From: &at, To: &at}} {
		if _, err := service.List(context.Background(), subject, 25, nil, filter); !errors.Is(err, app.ErrHistoryInvalid) || f.reads != 0 {
			t.Fatal("invalid filters reached query")
		}
	}
}
