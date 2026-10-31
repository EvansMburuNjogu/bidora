package opportunity_test

import (
	"fmt"
	"testing"

	"bidora/backend/internal/opportunity/domain"
)

func TestCapturePreservesUnknownFactsAndRawSource(t *testing.T) {
	input := domain.CapturedFactsInput{
		Title: "  Synthetic tender\n", SourceURI: "https://example.invalid/notice?edition=2",
		SourceDocumentVersionID: "synthetic-exact-version-reference",
		SourceDateText:          " Next Friday at 3pm (zone not stated)\n",
	}
	facts, err := domain.CaptureFacts(input)
	if err != nil {
		t.Fatal(err)
	}
	if facts.Title() != input.Title || facts.BuyerName() != "" || facts.SourceURI() != input.SourceURI || facts.SourceDocumentVersionID() != input.SourceDocumentVersionID || facts.SourceDateText() != input.SourceDateText || facts.SourceTimezone() != "" {
		t.Fatal("capture changed or invented source facts")
	}
	if _, known := facts.Deadline(); known || facts.Verified() {
		t.Fatal("ambiguous source became authoritative")
	}
	input.Title = "changed"
	if facts.Title() == input.Title {
		t.Fatal("caller mutation altered captured facts")
	}
	for _, format := range []string{"%v", "%+v", "%#v"} {
		if fmt.Sprintf(format, facts) != "[REDACTED captured facts]" || fmt.Sprintf(format, input) != "[REDACTED captured facts input]" {
			t.Fatal("formatting leaked source facts")
		}
	}
}

func TestInterpretedCaptureStillUnverifiedAndOwnsDeadline(t *testing.T) {
	deadline, err := domain.ParseDeadline("2026-10-17T15:00:00+03:00", "Africa/Nairobi", "17 October at 15:00 EAT")
	if err != nil {
		t.Fatal(err)
	}
	input := domain.CapturedFactsInput{Title: "Synthetic tender", BuyerName: "Synthetic buyer", SourceDateText: deadline.SourceText(), SourceTimezone: deadline.SourceZone(), InterpretedDeadline: &deadline}
	facts, err := domain.CaptureFacts(input)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := deadline.Instant()
	deadline = domain.Deadline{}
	got, known := facts.Deadline()
	instant, valid := got.Instant()
	if !known || !valid || !instant.Equal(want) || facts.Verified() || facts.BuyerName() != input.BuyerName {
		t.Fatal("deadline aliasing or automatic verification")
	}
	got = domain.Deadline{}
	again, known := facts.Deadline()
	if _, valid = again.Instant(); !known || !valid {
		t.Fatal("returned value mutated capture")
	}
}

func TestCaptureRejectsMismatchedOrZeroInterpretation(t *testing.T) {
	d, err := domain.ParseDeadline("2026-10-17T15:00:00+03:00", "Africa/Nairobi", "original wording")
	if err != nil {
		t.Fatal(err)
	}
	zero := domain.Deadline{}
	cases := []struct {
		input domain.CapturedFactsInput
		code  string
	}{
		{domain.CapturedFactsInput{InterpretedDeadline: &zero}, "capture_deadline_unknown"},
		{domain.CapturedFactsInput{InterpretedDeadline: &d, SourceDateText: "different wording", SourceTimezone: d.SourceZone()}, "capture_deadline_source_mismatch"},
		{domain.CapturedFactsInput{InterpretedDeadline: &d, SourceDateText: d.SourceText(), SourceTimezone: "UTC"}, "capture_deadline_source_mismatch"},
	}
	for _, c := range cases {
		facts, err := domain.CaptureFacts(c.input)
		if err == nil || err.Error() != c.code {
			t.Fatal("missing source consistency guard", err)
		}
		if _, known := facts.Deadline(); known || facts.Verified() || facts.SourceDateText() != "" {
			t.Fatal("failed capture returned partial facts")
		}
	}
}

func TestIncompleteAndDuplicateCapturesDoNotInventPolicy(t *testing.T) {
	var zero domain.CapturedFacts
	if _, known := zero.Deadline(); known || zero.Verified() {
		t.Fatal("zero value acquired authority")
	}
	if _, err := domain.CaptureFacts(domain.CapturedFactsInput{}); err != nil {
		t.Fatal("unknown facts must remain representable")
	}
	first, err := domain.CaptureFacts(domain.CapturedFactsInput{Title: "Same title", SourceURI: "source A"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := domain.CaptureFacts(domain.CapturedFactsInput{Title: "Same title", SourceURI: "source B"})
	if err != nil {
		t.Fatal(err)
	}
	if first.SourceURI() != "source A" || second.SourceURI() != "source B" {
		t.Fatal("independent sources silently merged")
	}
}
