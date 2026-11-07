package notification_test

import (
	"context"
	"errors"
	"testing"

	app "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/delivery"
	"bidora/backend/internal/notification/domain"
)

type recoveryEvidence struct {
	prepared           bool
	fresh              bool
	finishContextError error
	disposition        domain.Disposition
	finishCalled       bool
}

func (e *recoveryEvidence) Prepare(context.Context, app.PrepareInput) (domain.Notification, error) {
	e.prepared = true
	return domain.Notification{ID: domain.ID{1}}, nil
}
func (e *recoveryEvidence) StartAttemptForDelivery(context.Context, app.StartAttemptInput) (domain.Attempt, bool, error) {
	return domain.Attempt{ID: domain.ID{2}}, e.fresh, nil
}
func (e *recoveryEvidence) FinishAttempt(ctx context.Context, in app.FinishAttemptInput) (domain.Result, error) {
	e.finishCalled = true
	e.finishContextError = ctx.Err()
	e.disposition = in.Disposition
	if _, ok := ctx.Deadline(); !ok {
		return domain.Result{}, errors.New("unbounded evidence commit")
	}
	return domain.Result{}, ctx.Err()
}
func TestNotificationRecordsBeforeSendingAndFinishesAfterDisconnect(t *testing.T) {
	e := &recoveryEvidence{fresh: true}
	recorder, err := delivery.NewRecorder(e, delivery.Config{FingerprintKey: make([]byte, 32), FingerprintKeyVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := delivery.Request{RecipientCanonical: "synthetic@example.invalid", RenderedContent: []byte("synthetic"), AttemptNumber: 1}
	calls := 0
	if err = recorder.Send(ctx, request, func(context.Context) error {
		if !e.prepared {
			t.Fatal("sent before recording")
		}
		calls++
		cancel()
		return nil
	}); err != nil || !e.finishCalled || e.finishContextError != nil || e.disposition != domain.SMTPAccepted {
		t.Fatalf("lost handoff after disconnect: %v", err)
	}
	e.fresh = false
	if err = recorder.Send(context.Background(), request, func(context.Context) error { calls++; return nil }); !errors.Is(err, app.ErrConflict) || calls != 1 {
		t.Fatal("replay sent twice")
	}
	e.fresh = true
	failed := errors.New("synthetic failure")
	if err = recorder.Send(context.Background(), request, func(context.Context) error { return failed }); !errors.Is(err, failed) || e.disposition != domain.Ambiguous {
		t.Fatal("failed transport claimed delivery")
	}
}
