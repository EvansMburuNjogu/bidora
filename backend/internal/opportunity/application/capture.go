// Package application prepares opportunity commands. It grants no authority and
// does not persist data; authorized use-case execution remains a separate gate.
package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"bidora/backend/internal/opportunity/domain"
)

// CaptureInput preserves nullable wire values. Required property presence is
// enforced by the transport decoder. Formatting redacts; do not log JSON/input.
type CaptureInput struct {
	Title                   string  `json:"title"`
	BuyerName               *string `json:"buyer_name"`
	SourceURI               *string `json:"source_uri"`
	SourceDocumentVersionID *string `json:"source_document_version_id"`
	ClosesAt                *string `json:"closes_at"`
	SourceTimezone          *string `json:"source_timezone"`
	SourceDateText          *string `json:"source_date_text"`
}

func (CaptureInput) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED capture input]") }

// CaptureCommand is immutable prepared input, NOT an authorized/executed command.
// The canonical request is required for the future immutable source record;
// domain string getters alone cannot preserve null versus empty string.
type CaptureCommand struct {
	canonical string
	facts     domain.CapturedFacts
	record    CaptureRecord
}

func (CaptureCommand) Format(s fmt.State, verb rune)         { fmt.Fprint(s, "[REDACTED capture command]") }
func (c CaptureCommand) Facts() (domain.CapturedFacts, bool) { return c.facts, c.canonical != "" }

// CaptureRecord is the reviewed persistence shape. Returned pointers are owned
// copies so callers cannot mutate the command. It remains sensitive input.
type CaptureRecord struct {
	Title                                         string
	BuyerName, SourceURI, SourceDocumentVersionID *string
	ClosesAt                                      *time.Time
	SourceTimezone, SourceDateText                *string
}

func (CaptureRecord) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED capture record]") }
func (c CaptureCommand) Record() (CaptureRecord, bool) {
	if c.canonical == "" {
		return CaptureRecord{}, false
	}
	r := c.record
	r.BuyerName = copyString(r.BuyerName)
	r.SourceURI = copyString(r.SourceURI)
	r.SourceDocumentVersionID = copyString(r.SourceDocumentVersionID)
	r.SourceTimezone = copyString(r.SourceTimezone)
	r.SourceDateText = copyString(r.SourceDateText)
	if r.ClosesAt != nil {
		v := *r.ClosesAt
		r.ClosesAt = &v
	}
	return r, true
}

// CanonicalJSON returns a sensitive, owned copy for persistence, never logging.
func (c CaptureCommand) CanonicalJSON() ([]byte, bool) { return []byte(c.canonical), c.canonical != "" }

// Fingerprint is versioned and operation-separated, not a bearer credential or
// tenant deduplication key. Persistence must additionally scope by actor/tenant.
func (c CaptureCommand) Fingerprint() ([32]byte, bool) {
	if c.canonical == "" {
		return [32]byte{}, false
	}
	return sha256.Sum256([]byte("bidora.capture.v1\x00" + c.canonical)), true
}

func PrepareCapture(input CaptureInput) (CaptureCommand, error) {
	input.BuyerName = copyString(input.BuyerName)
	input.SourceURI = copyString(input.SourceURI)
	input.SourceDocumentVersionID = copyString(input.SourceDocumentVersionID)
	input.ClosesAt = copyString(input.ClosesAt)
	input.SourceTimezone = copyString(input.SourceTimezone)
	input.SourceDateText = copyString(input.SourceDateText)
	if strings.TrimSpace(input.Title) == "" {
		return CaptureCommand{}, invalid("title", "required", nil)
	}
	for _, field := range []struct {
		name  string
		value *string
		limit int
	}{
		{"title", &input.Title, 300}, {"buyer_name", input.BuyerName, 300},
		{"source_uri", input.SourceURI, 2048}, {"source_document_version_id", input.SourceDocumentVersionID, 36},
		{"closes_at", input.ClosesAt, 35}, {"source_timezone", input.SourceTimezone, 128},
		{"source_date_text", input.SourceDateText, 4096},
	} {
		if field.value == nil {
			continue
		}
		if !utf8.ValidString(*field.value) {
			return CaptureCommand{}, invalid(field.name, "invalid_utf8", nil)
		}
		if utf8.RuneCountInString(*field.value) > field.limit {
			return CaptureCommand{}, invalid(field.name, "too_long", nil)
		}
	}
	if input.SourceURI != nil {
		u, err := url.Parse(*input.SourceURI)
		if err != nil {
			return CaptureCommand{}, invalid("source_uri", "invalid_uri", err)
		}
		if (!strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https")) || u.Hostname() == "" || u.User != nil || u.Opaque != "" {
			return CaptureCommand{}, invalid("source_uri", "invalid_uri", nil)
		}
	}
	if input.SourceDocumentVersionID != nil && !uuidSyntax(*input.SourceDocumentVersionID) {
		return CaptureCommand{}, invalid("source_document_version_id", "invalid_uuid", nil)
	}
	var deadline *domain.Deadline
	if input.ClosesAt != nil {
		if input.SourceTimezone == nil || strings.TrimSpace(*input.SourceTimezone) == "" {
			return CaptureCommand{}, invalid("source_timezone", "required", nil)
		}
		if input.SourceDateText == nil || strings.TrimSpace(*input.SourceDateText) == "" {
			return CaptureCommand{}, invalid("source_date_text", "required", nil)
		}
		d, err := domain.DeadlineFromInstant(*input.ClosesAt, *input.SourceTimezone, *input.SourceDateText)
		if err != nil {
			return CaptureCommand{}, invalid("closes_at", "invalid_deadline", err)
		}
		deadline = &d
	}
	facts, err := domain.CaptureFacts(domain.CapturedFactsInput{
		Title: input.Title, BuyerName: text(input.BuyerName), SourceURI: text(input.SourceURI),
		SourceDocumentVersionID: text(input.SourceDocumentVersionID), SourceDateText: text(input.SourceDateText),
		SourceTimezone: text(input.SourceTimezone), InterpretedDeadline: deadline,
	})
	if err != nil {
		return CaptureCommand{}, invalid("closes_at", "invalid_deadline", err)
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return CaptureCommand{}, invalid("body", "encoding_failed", err)
	}
	var closesAt *time.Time
	if deadline != nil {
		instant, _ := deadline.Instant()
		instant = instant.UTC()
		closesAt = &instant
	}
	return CaptureCommand{canonical: string(encoded), facts: facts, record: CaptureRecord{
		Title: input.Title, BuyerName: copyString(input.BuyerName), SourceURI: copyString(input.SourceURI),
		SourceDocumentVersionID: copyString(input.SourceDocumentVersionID), ClosesAt: closesAt,
		SourceTimezone: copyString(input.SourceTimezone), SourceDateText: copyString(input.SourceDateText),
	}}, nil
}

func text(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func copyString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
func uuidSyntax(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	decoded, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil && len(decoded) == 16
}

// ValidationError exposes only fixed field/rule identifiers. Unwrapped causes
// are sensitive diagnostics and must never be logged or returned unfiltered.
type ValidationError struct {
	field, rule string
	cause       error
}

func invalid(field, rule string, cause error) error      { return &ValidationError{field, rule, cause} }
func (e *ValidationError) Error() string                 { return "VALIDATION_FAILED" }
func (e *ValidationError) Field() string                 { return e.field }
func (e *ValidationError) Rule() string                  { return e.rule }
func (e *ValidationError) Unwrap() error                 { return e.cause }
func (e *ValidationError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
