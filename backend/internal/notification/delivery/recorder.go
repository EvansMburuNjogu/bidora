// Package delivery records content-minimized evidence around an SMTP call.
package delivery

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"time"

	app "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/domain"
	"bidora/backend/internal/notification/message"
)

type EvidenceService interface {
	Prepare(context.Context, app.PrepareInput) (domain.Notification, error)
	StartAttemptForDelivery(context.Context, app.StartAttemptInput) (domain.Attempt, bool, error)
	FinishAttempt(context.Context, app.FinishAttemptInput) (domain.Result, error)
}

type Config struct {
	FingerprintKey        []byte
	FingerprintKeyVersion int32
}

type Recorder struct {
	service EvidenceService
	key     []byte
	version int32
}

func NewRecorder(service EvidenceService, config Config) (*Recorder, error) {
	if service == nil || len(config.FingerprintKey) < 32 || config.FingerprintKeyVersion < 1 {
		return nil, errors.New("notification_delivery_recorder_invalid")
	}
	key := make([]byte, len(config.FingerprintKey))
	copy(key, config.FingerprintKey)
	return &Recorder{service: service, key: key, version: config.FingerprintKeyVersion}, nil
}

type Request struct {
	TenantID, ActorID, RecipientUserID, SourceID, CorrelationID, SMTPVersionID domain.ID
	Category, TemplateKey, SourceType, IdempotencyKey                          string
	TemplateVersion                                                            int64
	AttemptNumber                                                              int32
	RecipientCanonical                                                         string
	RenderedContent                                                            []byte
	Subject, Text                                                              string
}

// Send persists the logical notification and attempt start before invoking send.
// It stores only keyed fingerprints. Any transport error is recorded as an
// ambiguous provider outcome because SMTP acceptance cannot safely be inferred.
func (r *Recorder) Send(ctx context.Context, request Request, send func(context.Context) error) error {
	if ctx == nil || r == nil || r.service == nil || len(r.key) < 32 || r.version < 1 ||
		request.RecipientCanonical == "" || len(request.RenderedContent) == 0 || send == nil ||
		request.AttemptNumber < 1 || request.AttemptNumber > 100 {
		return errors.New("notification_delivery_request_invalid")
	}
	recipientFingerprint := fingerprint(r.key, "recipient", []byte(request.RecipientCanonical))
	contentFingerprint := fingerprint(r.key, "content", request.RenderedContent)
	notification, err := r.service.Prepare(ctx, app.PrepareInput{
		TenantID: request.TenantID, ActorID: request.ActorID, RecipientUserID: request.RecipientUserID,
		SourceID: request.SourceID, CorrelationID: request.CorrelationID, Category: request.Category,
		TemplateKey: request.TemplateKey, SourceType: request.SourceType, IdempotencyKey: request.IdempotencyKey,
		TemplateVersion: request.TemplateVersion, RecipientFingerprint: recipientFingerprint,
		ContentFingerprint: contentFingerprint, FingerprintKeyVersion: r.version,
	})
	if err != nil {
		return err
	}
	if writer, ok := r.service.(interface {
		RecordMessage(context.Context, domain.ID, message.Encrypted) error
	}); ok {
		codec, e := message.New(r.key, r.version)
		if e != nil {
			return e
		}
		snapshot, e := codec.Seal(notification.ID, message.Content{Recipient: request.RecipientCanonical, Subject: request.Subject, Text: request.Text})
		if e != nil {
			return e
		}
		if e = writer.RecordMessage(ctx, notification.ID, snapshot); e != nil {
			return e
		}
	}
	attempt, fresh, err := r.service.StartAttemptForDelivery(ctx, app.StartAttemptInput{NotificationID: notification.ID, SMTPVersionID: request.SMTPVersionID, CorrelationID: request.CorrelationID, Number: request.AttemptNumber})
	if err != nil {
		return err
	}
	if !fresh {
		return app.ErrConflict // Replayed evidence never authorizes another SMTP send.
	}
	sendErr := send(ctx)
	finish := app.FinishAttemptInput{AttemptID: attempt.ID, CorrelationID: request.CorrelationID, Disposition: domain.SMTPAccepted}
	if sendErr != nil {
		finish.Disposition = domain.Ambiguous
		finish.FailureCode = "PROVIDER_OUTCOME_UNKNOWN"
	}
	// Preserve the known handoff outcome even if the browser disconnects during
	// SMTP. Keep audit context, but bound this independent evidence commit.
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, evidenceErr := r.service.FinishAttempt(finishCtx, finish)
	if evidenceErr != nil {
		return errors.Join(sendErr, evidenceErr)
	}
	return sendErr
}

func fingerprint(key []byte, purpose string, value []byte) (result [32]byte) {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("bidora-notification-v1\x00" + purpose + "\x00"))
	_, _ = mac.Write(value)
	copy(result[:], mac.Sum(nil))
	return result
}
