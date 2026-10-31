// Package domain owns content-minimized notification delivery evidence.
package domain

import (
	"errors"
	"regexp"
	"time"
)

type ID [16]byte

func (id ID) IsZero() bool { return id == ID{} }

type Disposition string

const (
	SMTPAccepted Disposition = "SMTP_ACCEPTED"
	Failed       Disposition = "FAILED"
	Ambiguous    Disposition = "AMBIGUOUS"
)

var ErrInvalid = errors.New("notification_evidence_invalid")
var upperName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
var templateName = regexp.MustCompile(`^[a-z][a-z0-9_.-]*$`)

var failureCodes = map[string]bool{
	"CONNECTIVITY_FAILED": true, "AUTHENTICATION_FAILED": true, "TLS_FAILED": true,
	"PROVIDER_REJECTED": true, "TIMEOUT": true, "PROCESS_INTERRUPTED": true,
	"PROVIDER_OUTCOME_UNKNOWN": true, "INTERNAL_ERROR": true,
}

type Notification struct {
	ID, TenantID, ActorID, RecipientUserID, SourceID, CorrelationID ID
	Category, TemplateKey, SourceType                               string
	TemplateVersion                                                 int64
	RecipientFingerprint, ContentFingerprint, IdempotencyKeyHash    [32]byte
	FingerprintKeyVersion                                           int32
	CreatedAt                                                       time.Time
}

type Attempt struct {
	ID, NotificationID, SMTPVersionID, CorrelationID ID
	Number                                           int32
	StartedAt                                        time.Time
}

type Result struct {
	ID, AttemptID, CorrelationID ID
	Disposition                  Disposition
	FailureCode                  string
	FinishedAt                   time.Time
}

func (n Notification) Valid() bool {
	return !n.ID.IsZero() && !n.SourceID.IsZero() && !n.CorrelationID.IsZero() &&
		len(n.Category) <= 80 && upperName.MatchString(n.Category) &&
		len(n.SourceType) <= 80 && upperName.MatchString(n.SourceType) &&
		len(n.TemplateKey) <= 100 && templateName.MatchString(n.TemplateKey) &&
		n.TemplateVersion > 0 && n.FingerprintKeyVersion > 0 &&
		n.RecipientFingerprint != [32]byte{} && n.ContentFingerprint != [32]byte{} &&
		n.IdempotencyKeyHash != [32]byte{} && !n.CreatedAt.IsZero()
}

func (a Attempt) Valid() bool {
	return !a.ID.IsZero() && !a.NotificationID.IsZero() && !a.SMTPVersionID.IsZero() &&
		!a.CorrelationID.IsZero() && a.Number >= 1 && a.Number <= 100 && !a.StartedAt.IsZero()
}

func (r Result) Valid() bool {
	if r.ID.IsZero() || r.AttemptID.IsZero() || r.CorrelationID.IsZero() || r.FinishedAt.IsZero() {
		return false
	}
	if r.Disposition == SMTPAccepted {
		return r.FailureCode == ""
	}
	return (r.Disposition == Failed || r.Disposition == Ambiguous) && failureCodes[r.FailureCode]
}
