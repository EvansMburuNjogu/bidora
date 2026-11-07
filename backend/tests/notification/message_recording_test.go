package notification_test

import (
	"context"
	"errors"
	"testing"

	"bidora/backend/internal/notification/delivery"
	"bidora/backend/internal/notification/domain"
	"bidora/backend/internal/notification/message"
)

type snapshotEvidence struct {
	recoveryEvidence
	snapshot message.Encrypted
	fail     bool
}

func (e *snapshotEvidence) RecordMessage(_ context.Context, _ domain.ID, v message.Encrypted) error {
	if !e.prepared {
		panic("message before intent")
	}
	if e.fail {
		return errors.New("synthetic recording error")
	}
	e.snapshot = v
	return nil
}
func TestMessageIsDurableBeforeSMTPAndFailureStopsSend(t *testing.T) {
	e := &snapshotEvidence{recoveryEvidence: recoveryEvidence{fresh: true}, fail: true}
	key := make([]byte, 32)
	recorder, _ := delivery.NewRecorder(e, delivery.Config{FingerprintKey: key, FingerprintKeyVersion: 1})
	req := delivery.Request{RecipientCanonical: "recipient@example.test", RenderedContent: []byte("message"), Subject: "Sign in", Text: "Your code is 123456", AttemptNumber: 1}
	calls := 0
	send := func(context.Context) error {
		calls++
		if len(e.snapshot.Ciphertext) == 0 {
			t.Fatal("sent before snapshot")
		}
		return nil
	}
	if err := recorder.Send(context.Background(), req, send); err == nil || calls != 0 {
		t.Fatal("recording failure allowed send")
	}
	e.fail = false
	if err := recorder.Send(context.Background(), req, send); err != nil || calls != 1 {
		t.Fatal("recorded message not sent", err)
	}
	codec, _ := message.New(key, 1)
	decoded, err := codec.Open(domain.ID{1}, e.snapshot)
	if err != nil || decoded.Text != "Your code is [code redacted]" {
		t.Fatal("secret in stored preview", err)
	}
}
