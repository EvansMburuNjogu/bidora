package opportunity_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"bidora/backend/internal/opportunity/domain"
)

func TestDeadlinePreservesSourceAndUTC(t *testing.T) {
	text := " Closing: 17 October 2026 at 15:00 EAT\n"
	d, err := domain.ParseDeadline("2026-10-17T15:00:00+03:00", "Africa/Nairobi", text)
	if err != nil {
		t.Fatal(err)
	}
	instant, ok := d.Instant()
	if !ok || instant.Format(time.RFC3339) != "2026-10-17T12:00:00Z" || instant.Location() != time.UTC {
		t.Fatal("UTC instant lost")
	}
	if d.SourceText() != text || d.SourceZone() != "Africa/Nairobi" || d.SourceTimestamp() != "2026-10-17T15:00:00+03:00" {
		t.Fatal("source evidence altered")
	}
	for _, format := range []string{"%v", "%+v", "%#v"} {
		if fmt.Sprintf(format, d) != "[REDACTED external deadline]" {
			t.Fatal("private source formatted")
		}
	}
}

func TestRejectsAmbiguityAndContradiction(t *testing.T) {
	cases := []struct{ timestamp, zone, text, code string }{
		{"2026-10-17T15:00:00", "Africa/Nairobi", "source", "deadline_time_invalid"},
		{"17/10/26 3pm", "Africa/Nairobi", "source", "deadline_time_invalid"},
		{"2026-10-17T15:00:00-00:00", "UTC", "source", "deadline_time_invalid"},
		{"2026-10-17T15:00:00+24:00", "UTC", "source", "deadline_time_invalid"},
		{"2026-10-17T15:00:00+03:60", "Africa/Nairobi", "source", "deadline_time_invalid"},
		{"2026-10-17T15:00:00.1234567891Z", "UTC", "source", "deadline_time_invalid"},
		{"2026-02-30T15:00:00Z", "UTC", "source", "deadline_time_invalid"},
		{"0000-01-01T00:00:00Z", "UTC", "source", "deadline_time_invalid"},
		{"2026-10-17T15:00:00Z", "", "source", "deadline_zone_required"},
		{"2026-10-17T15:00:00Z", "Local", "source", "deadline_zone_required"},
		{"2026-10-17T15:00:00Z", "EST", "source", "deadline_zone_required"},
		{"2026-10-17T15:00:00Z", "Nowhere/secret-canary", "source", "deadline_zone_invalid"},
		{"2026-10-17T15:00:00+02:00", "Africa/Nairobi", "source", "deadline_zone_offset_mismatch"},
		{"2026-03-08T02:30:00-05:00", "America/New_York", "source", "deadline_zone_offset_mismatch"},
		{"2026-03-08T02:30:00-04:00", "America/New_York", "source", "deadline_zone_offset_mismatch"},
		{"2026-10-17T15:00:00Z", "UTC", " \n ", "deadline_source_text_required"},
	}
	for _, c := range cases {
		t.Run(c.code+"/"+c.timestamp, func(t *testing.T) {
			d, err := domain.ParseDeadline(c.timestamp, c.zone, c.text)
			if err == nil || err.Error() != c.code || fmt.Sprintf("%#v", err) != c.code {
				t.Fatal("expected safe validation error", err)
			}
			if _, ok := d.Instant(); ok || d.SourceText() != "" {
				t.Fatal("failed parse returned partial value")
			}
		})
	}
}

func TestDSTFoldRequiresExplicitOffset(t *testing.T) {
	first, err := domain.ParseDeadline("2026-11-01T01:30:00-04:00", "America/New_York", "source")
	if err != nil {
		t.Fatal(err)
	}
	second, err := domain.ParseDeadline("2026-11-01T01:30:00-05:00", "America/New_York", "source")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := first.Instant()
	b, _ := second.Instant()
	if b.Sub(a) != time.Hour {
		t.Fatal("ambiguous wall-clock instants collapsed")
	}
}

func TestZeroHistoricalAndDiagnosticBehavior(t *testing.T) {
	var zero domain.Deadline
	if _, ok := zero.Instant(); ok {
		t.Fatal("zero value became a known deadline")
	}
	for _, timestamp := range []string{"2000-01-01T00:00:00Z", "0001-01-01T00:00:00Z", "2026-10-17T12:00:00.123456789Z"} {
		d, err := domain.ParseDeadline(timestamp, "UTC", "source")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := d.Instant(); !ok {
			t.Fatal("explicit instant lost")
		}
	}
	_, err := domain.ParseDeadline("2026-02-30T15:00:00Z", "UTC", "source")
	var cause *time.ParseError
	if !errors.As(err, &cause) {
		t.Fatal("diagnostic cause discarded")
	}
}
