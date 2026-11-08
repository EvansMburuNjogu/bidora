package notification_test

import (
	"context"
	"testing"
	"time"

	app "bidora/backend/internal/notification/application"
	domain "bidora/backend/internal/notification/domain"
)

type attemptTx struct {
	app.Tx
	existing           bool
	inserted, evidence int
}

func (t *attemptTx) GetNotification(context.Context, domain.ID) (domain.Notification, bool, error) {
	return domain.Notification{ID: domain.ID{1}}, true, nil
}
func (t *attemptTx) GetAttemptByNumber(context.Context, domain.ID, int32) (domain.Attempt, bool, error) {
	return domain.Attempt{ID: domain.ID{2}, SMTPVersionID: domain.ID{3}}, t.existing, nil
}
func (t *attemptTx) CanStartAttempt(context.Context, domain.ID, int32) (bool, error) {
	return true, nil
}
func (t *attemptTx) IsActiveSMTPVersion(context.Context, domain.ID) (bool, error) { return true, nil }
func (t *attemptTx) InsertAttempt(context.Context, domain.Attempt) error          { t.inserted++; return nil }
func (t *attemptTx) AppendEvidence(context.Context, app.Evidence) error           { t.evidence++; return nil }

type attemptUOW struct{ tx *attemptTx }

func (u attemptUOW) WithinNotification(ctx context.Context, fn func(context.Context, app.Tx) error) error {
	return fn(ctx, u.tx)
}

type attemptIDs struct{ n byte }

func (i *attemptIDs) NewID() (domain.ID, error) { i.n++; return domain.ID{i.n + 10}, nil }

type attemptClock struct{}

func (attemptClock) Now() time.Time { return time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC) }
func TestDeliveryAttemptFenceDistinguishesEvidenceReplayFromFreshSend(t *testing.T) {
	tx := &attemptTx{existing: true}
	svc, err := app.NewService(attemptUOW{tx}, &attemptIDs{}, attemptClock{})
	if err != nil {
		t.Fatal(err)
	}
	input := app.StartAttemptInput{NotificationID: domain.ID{1}, SMTPVersionID: domain.ID{3}, CorrelationID: domain.ID{4}, Number: 1}
	_, fresh, err := svc.StartAttemptForDelivery(context.Background(), input)
	if err != nil || fresh || tx.inserted != 0 || tx.evidence != 0 {
		t.Fatalf("replay marked fresh or mutated: %v", err)
	}
	tx.existing = false
	_, fresh, err = svc.StartAttemptForDelivery(context.Background(), input)
	if err != nil || !fresh || tx.inserted != 1 || tx.evidence != 1 {
		t.Fatalf("fresh attempt not evidenced: %v", err)
	}
}
