// Package domain contains opportunity values, not HTTP, persistence or authority.
package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata" // Portable fallback when the host has no timezone database.
)

// Deadline preserves an explicit interpretation of an external source time.
// Construction is NOT verification of a fact, readiness, or authority to submit.
// Fields are private; corrections create a new value, not a mutation.
type Deadline struct {
	instant         time.Time
	sourceTimestamp string
	sourceZone      string
	sourceText      string
}

// ParseDeadline accepts an interpreted SOURCE-LOCAL RFC3339 timestamp with an
// explicit offset, its named IANA region (or UTC), and unchanged source wording.
// It never guesses a missing timezone/offset, parses natural-language dates,
// imposes a future-only deadline, or defines platform submission cutoff rules.
func ParseDeadline(sourceTimestamp, sourceZone, sourceText string) (Deadline, error) {
	if strings.TrimSpace(sourceText) == "" {
		return Deadline{}, invalid("deadline_source_text_required", nil)
	}
	parsed, err := parseInstant(sourceTimestamp)
	if err != nil {
		return Deadline{}, err
	}
	if sourceZone != "UTC" && (!strings.Contains(sourceZone, "/") || strings.HasPrefix(sourceZone, "/")) {
		return Deadline{}, invalid("deadline_zone_required", nil)
	}
	zone, err := time.LoadLocation(sourceZone)
	if err != nil {
		return Deadline{}, invalid("deadline_zone_invalid", err)
	}
	_, offset := parsed.Zone()
	_, zoneOffset := parsed.In(zone).Zone()
	if offset != zoneOffset {
		return Deadline{}, invalid("deadline_zone_offset_mismatch", nil)
	}
	return Deadline{parsed.UTC(), sourceTimestamp, sourceZone, sourceText}, nil
}

// Instant returns false for the zero value; absence must never become a deadline.
func (d Deadline) Instant() (time.Time, bool)  { return d.instant, d.sourceTimestamp != "" }
func (d Deadline) SourceTimestamp() string     { return d.sourceTimestamp }
func (d Deadline) SourceZone() string          { return d.sourceZone }
func (d Deadline) SourceText() string          { return d.sourceText }
func (Deadline) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED external deadline]") }

// DeadlineFromInstant maps an explicit wire instant to its declared source zone.
// Unlike ParseDeadline, the input need not already be expressed in that zone.
// The caller must retain the original wire input separately for provenance and
// idempotency. Conversion is not verification of source wording or authority.
func DeadlineFromInstant(timestamp, sourceZone, sourceText string) (Deadline, error) {
	parsed, err := parseInstant(timestamp)
	if err != nil {
		return Deadline{}, err
	}
	if sourceZone != "UTC" && (!strings.Contains(sourceZone, "/") || strings.HasPrefix(sourceZone, "/")) {
		return Deadline{}, invalid("deadline_zone_required", nil)
	}
	zone, err := time.LoadLocation(sourceZone)
	if err != nil {
		return Deadline{}, invalid("deadline_zone_invalid", err)
	}
	return ParseDeadline(parsed.In(zone).Format(time.RFC3339Nano), sourceZone, sourceText)
}

func parseInstant(timestamp string) (time.Time, error) {
	// Reject permissive offsets and precision truncation before time.Parse.
	const pattern = `^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`
	if len(timestamp) > 35 || strings.HasSuffix(timestamp, "-00:00") {
		return time.Time{}, invalid("deadline_time_invalid", nil)
	}
	matched, err := regexp.MatchString(pattern, timestamp)
	if err != nil {
		return time.Time{}, invalid("deadline_time_invalid", err)
	}
	if !matched {
		return time.Time{}, invalid("deadline_time_invalid", nil)
	}
	parsed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return time.Time{}, invalid("deadline_time_invalid", err)
	}
	if parsed.UTC().Year() < 1 || parsed.UTC().Year() > 9999 {
		return time.Time{}, invalid("deadline_time_invalid", nil)
	}
	return parsed, nil
}

type validationError struct {
	code  string
	cause error
}

func invalid(code string, cause error) error             { return &validationError{code, cause} }
func (e *validationError) Error() string                 { return e.code }
func (e *validationError) Unwrap() error                 { return e.cause }
func (e *validationError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.code) }
