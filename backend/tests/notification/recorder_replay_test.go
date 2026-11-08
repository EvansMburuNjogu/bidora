package notification_test

import (
	"context"
	"errors"
	"testing"

	app "bidora/backend/internal/notification/application"
	delivery "bidora/backend/internal/notification/delivery"
	domain "bidora/backend/internal/notification/domain"
)

type recorderEvidence struct {
	fresh             bool
	started, finished int
	disposition       domain.Disposition
}

func (e *recorderEvidence) Prepare(context.Context, app.PrepareInput) (domain.Notification, error) {
	return domain.Notification{ID: domain.ID{1}}, nil
}
func (e *recorderEvidence) StartAttemptForDelivery(context.Context, app.StartAttemptInput) (domain.Attempt, bool, error) {
	e.started++
	return domain.Attempt{ID: domain.ID{2}}, e.fresh, nil
}
func (e *recorderEvidence) FinishAttempt(_ context.Context, v app.FinishAttemptInput) (domain.Result, error) {
	e.finished++
	e.disposition = v.Disposition
	return domain.Result{}, nil
}
func TestRecorderCannotSendOnReplayedAttemptOrClaimAmbiguousDelivery(t *testing.T) {
	evidence := &recorderEvidence{}
	recorder, err := delivery.NewRecorder(evidence, delivery.Config{FingerprintKey: make([]byte, 32), FingerprintKeyVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	request := delivery.Request{RecipientCanonical: "recipient@example.invalid", RenderedContent: []byte("synthetic"), SMTPVersionID: domain.ID{3}, AttemptNumber: 1}
	calls := 0
	send := func(context.Context) error { calls++; return nil }
	if err = recorder.Send(context.Background(), request, send); !errors.Is(err, app.ErrConflict) || calls != 0 || evidence.finished != 0 {
		t.Fatalf("replay caused external send: %v %d", err, calls)
	}
	evidence.fresh = true
	if err = recorder.Send(context.Background(), request, send); err != nil || calls != 1 || evidence.finished != 1 || evidence.disposition != domain.SMTPAccepted {
		t.Fatalf("new send not truthfully recorded: %v", err)
	}
	failure := errors.New("synthetic transport uncertainty")
	if err = recorder.Send(context.Background(), request, func(context.Context) error { return failure }); !errors.Is(err, failure) || evidence.disposition != domain.Ambiguous {
		t.Fatalf("transport failure claimed delivery: %v", err)
	}
}
