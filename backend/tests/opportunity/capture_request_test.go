package opportunity_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/opportunity/application"
	"bidora/backend/internal/opportunity/transport"
)

func decode(t *testing.T, body string) application.CaptureCommand {
	t.Helper()
	c, err := transport.DecodeCaptureJSON([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCaptureCanonicalVectorAndOwnedCopies(t *testing.T) {
	c := decode(t, `{"title":" Synthetic ","buyer_name":null}`)
	const expected = `{"title":" Synthetic ","buyer_name":null,"source_uri":null,"source_document_version_id":null,"closes_at":null,"source_timezone":null,"source_date_text":null}`
	canonical, ok := c.CanonicalJSON()
	if !ok || string(canonical) != expected {
		t.Fatal("canonical v1 vector drifted")
	}
	want := sha256.Sum256([]byte("bidora.capture.v1\x00" + expected))
	got, ok := c.Fingerprint()
	if !ok || got != want {
		t.Fatal("fingerprint v1 vector drifted")
	}
	other := decode(t, ` {"source_date_text":null,"buyer_name":null,"title":" Synthetic ","source_uri":null} `)
	hash, _ := other.Fingerprint()
	if hash != got {
		t.Fatal("ordering/null/omission affected fingerprint")
	}
	canonical[0] = 'x'
	again, _ := c.CanonicalJSON()
	if string(again) != expected {
		t.Fatal("caller mutated canonical request")
	}
	for _, body := range []string{`{"title":"Synthetic","buyer_name":null}`, `{"title":" Synthetic ","buyer_name":""}`} {
		changed := decode(t, body)
		h, _ := changed.Fingerprint()
		if h == got {
			t.Fatal("distinct source input collapsed")
		}
	}
	record, ok := c.Record()
	if !ok || record.Title != " Synthetic " || record.BuyerName != nil || record.ClosesAt != nil {
		t.Fatal("persistence record lost null/source values")
	}
	record.Title = "mutated"
	againRecord, _ := c.Record()
	if againRecord.Title != " Synthetic " {
		t.Fatal("caller mutated persistence record")
	}
	facts, ok := c.Facts()
	if !ok || facts.Title() != " Synthetic " || facts.Verified() {
		t.Fatal("invalid prepared facts")
	}
	var zero application.CaptureCommand
	if _, ok := zero.CanonicalJSON(); ok {
		t.Fatal("zero command accepted")
	}
	if _, ok := zero.Fingerprint(); ok {
		t.Fatal("zero command fingerprinted")
	}
	if _, ok := zero.Facts(); ok {
		t.Fatal("zero command acquired facts")
	}
	if _, ok := zero.Record(); ok {
		t.Fatal("zero command acquired persistence record")
	}
}

func TestCaptureWireInstantMapsToSourceZone(t *testing.T) {
	c := decode(t, `{"title":"Synthetic","buyer_name":null,"closes_at":"2026-10-17T12:00:00Z","source_timezone":"Africa/Nairobi","source_date_text":"17 October 2026 at 15:00 EAT"}`)
	facts, _ := c.Facts()
	deadline, known := facts.Deadline()
	instant, valid := deadline.Instant()
	if !known || !valid || instant.Format(time.RFC3339) != "2026-10-17T12:00:00Z" || deadline.SourceTimestamp() != "2026-10-17T15:00:00+03:00" || facts.Verified() {
		t.Fatal("wire instant/source-zone mapping failed")
	}
	raw, _ := c.CanonicalJSON()
	if !bytes.Contains(raw, []byte(`"closes_at":"2026-10-17T12:00:00Z"`)) {
		t.Fatal("original wire instant rewritten")
	}
	other := decode(t, `{"title":"Synthetic","buyer_name":null,"closes_at":"2026-10-17T15:00:00+03:00","source_timezone":"Africa/Nairobi","source_date_text":"17 October 2026 at 15:00 EAT"}`)
	a, _ := c.Fingerprint()
	b, _ := other.Fingerprint()
	if a == b {
		t.Fatal("different supplied timestamp strings collapsed")
	}
	unknown := decode(t, `{"title":"Synthetic","buyer_name":null,"source_date_text":"next Friday","source_timezone":"CST"}`)
	f, _ := unknown.Facts()
	if _, known := f.Deadline(); known || f.SourceDateText() != "next Friday" || f.SourceTimezone() != "CST" {
		t.Fatal("unknown source date was guessed or discarded")
	}
}

func TestCaptureStrictBindingAndSafeErrors(t *testing.T) {
	cases := []struct{ body, code string }{
		{`null`, "INVALID_REQUEST"}, {`[]`, "INVALID_REQUEST"}, {``, "INVALID_REQUEST"}, {`{"title":`, "INVALID_REQUEST"},
		{`{"title":"x","buyer_name":null} {}`, "INVALID_REQUEST"},
		{`{"title":"x","buyer_name":null,"verified":true}`, "INVALID_REQUEST"},
		{`{"title":"x","buyer_name":null,"tenant_id":"secret-canary"}`, "INVALID_REQUEST"},
		{`{"title":"x","buyer_name":null,"secret-canary":null}`, "INVALID_REQUEST"},
		{`{"title":"x","buyer_name":null,"title":"y"}`, "INVALID_REQUEST"},
		{`{"title":"x","buyer_name":null,"\u0074itle":"y"}`, "INVALID_REQUEST"},
		{`{"title":"x"}`, "VALIDATION_FAILED"}, {`{"buyer_name":null}`, "VALIDATION_FAILED"},
		{`{"title":null,"buyer_name":null}`, "VALIDATION_FAILED"}, {`{"title":"x","buyer_name":12}`, "VALIDATION_FAILED"},
		{`{"title":"x","buyer_name":null,"source_date_text":{}}`, "VALIDATION_FAILED"},
		{`{"title":"\ud800","buyer_name":null}`, "VALIDATION_FAILED"},
		{`{"title":"\udc00","buyer_name":null}`, "VALIDATION_FAILED"},
		{`{"title":"\ud800\u0041","buyer_name":null}`, "VALIDATION_FAILED"},
	}
	for _, tc := range cases {
		command, err := transport.DecodeCaptureJSON([]byte(tc.body))
		if err == nil || err.Error() != tc.code || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
			t.Fatalf("unexpected safe binding result: %v", err)
		}
		if _, ok := command.Facts(); ok {
			t.Fatal("failed decode returned partial command")
		}
		var typed *transport.DecodeError
		if !errors.As(err, &typed) || typed.Code() != tc.code || typed.Field() == "secret-canary" || typed.Rule() == "" {
			t.Fatal("missing safe error metadata")
		}
	}
	if _, err := transport.DecodeCaptureJSON([]byte{'{', '"', 0xff, '"', ':', '1', '}'}); err == nil {
		t.Fatal("invalid UTF8 accepted")
	}
	unicode := decode(t, `{"title":"\ud83d\ude00","buyer_name":null,"source_date_text":"literal \\uD800"}`)
	facts, _ := unicode.Facts()
	if facts.Title() != "😀" || facts.SourceDateText() != `literal \uD800` {
		t.Fatal("valid escaped source text changed")
	}
}

func TestCaptureBodyAndCharacterBounds(t *testing.T) {
	base := []byte(`{"title":"x","buyer_name":null}`)
	exact := append(base, bytes.Repeat([]byte{' '}, transport.CaptureBodyLimit-len(base))...)
	if _, err := transport.DecodeCaptureJSON(exact); err != nil {
		t.Fatal("exact body limit rejected", err)
	}
	if _, err := transport.DecodeCaptureJSON(append(exact, ' ')); err == nil || err.Error() != "REQUEST_TOO_LARGE" {
		t.Fatal("oversize body accepted")
	}
	for _, n := range []int{300, 301} {
		body, err := json.Marshal(map[string]any{"title": strings.Repeat("😀", n), "buyer_name": nil})
		if err != nil {
			t.Fatal(err)
		}
		_, err = transport.DecodeCaptureJSON(body)
		if (err == nil) != (n == 300) {
			t.Fatal("character bound used bytes or accepted overflow")
		}
	}
}

func TestPrepareCaptureValidationAndDefensiveCopy(t *testing.T) {
	ptr := func(s string) *string { return &s }
	for _, tc := range []struct {
		input application.CaptureInput
		field string
	}{
		{application.CaptureInput{Title: " \n"}, "title"},
		{application.CaptureInput{Title: "x", BuyerName: ptr(strings.Repeat("b", 301))}, "buyer_name"},
		{application.CaptureInput{Title: "x", SourceURI: ptr("https://secret-canary@example.invalid")}, "source_uri"},
		{application.CaptureInput{Title: "x", SourceURI: ptr("javascript:secret-canary")}, "source_uri"},
		{application.CaptureInput{Title: "x", SourceURI: ptr("http:opaque")}, "source_uri"},
		{application.CaptureInput{Title: "x", SourceURI: ptr("https://")}, "source_uri"},
		{application.CaptureInput{Title: "x", SourceURI: ptr("https://bad host/secret-canary")}, "source_uri"},
		{application.CaptureInput{Title: "x", SourceDocumentVersionID: ptr("not-a-uuid")}, "source_document_version_id"},
		{application.CaptureInput{Title: "x", SourceDateText: ptr(string([]byte{0xff}))}, "source_date_text"},
		{application.CaptureInput{Title: "x", ClosesAt: ptr("2026-10-17T12:00:00Z")}, "source_timezone"},
		{application.CaptureInput{Title: "x", ClosesAt: ptr("2026-10-17T12:00:00Z"), SourceTimezone: ptr("UTC")}, "source_date_text"},
		{application.CaptureInput{Title: "x", ClosesAt: ptr("2026-10-17T12:00:00-00:00"), SourceTimezone: ptr("UTC"), SourceDateText: ptr("source")}, "closes_at"},
		{application.CaptureInput{Title: "x", ClosesAt: ptr("2026-10-17T12:00:00Z"), SourceTimezone: ptr("Local"), SourceDateText: ptr("source")}, "closes_at"},
	} {
		command, err := application.PrepareCapture(tc.input)
		var typed *application.ValidationError
		if !errors.As(err, &typed) || typed.Field() != tc.field || typed.Rule() == "" || err.Error() != "VALIDATION_FAILED" || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
			t.Fatal("unsafe or missing validation", err)
		}
		if _, ok := command.Facts(); ok {
			t.Fatal("invalid command retained partial facts")
		}
	}
	buyer := "Original"
	input := application.CaptureInput{Title: "x", BuyerName: &buyer, SourceURI: ptr("https://example.invalid/notice"), SourceDocumentVersionID: ptr("019c0000-0000-7000-8000-000000000001")}
	command, err := application.PrepareCapture(input)
	if err != nil {
		t.Fatal(err)
	}
	buyer = "Changed"
	facts, _ := command.Facts()
	if facts.BuyerName() != "Original" {
		t.Fatal("input pointer alias retained")
	}
	for _, format := range []string{"%v", "%+v", "%#v"} {
		if strings.Contains(fmt.Sprintf(format, command), "Original") || strings.Contains(fmt.Sprintf(format, input), "Changed") {
			t.Fatal("command/input formatting leaked")
		}
	}
}

func TestCaptureAllFieldBoundsAndWrappedDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		field string
		limit int
		set   func(*application.CaptureInput, string)
	}{
		{"buyer_name", 300, func(i *application.CaptureInput, s string) { i.BuyerName = &s }},
		{"source_uri", 2048, func(i *application.CaptureInput, s string) {
			uri := "https://example.invalid/" + s[len("https://example.invalid/"):]
			i.SourceURI = &uri
		}},
		{"source_timezone", 128, func(i *application.CaptureInput, s string) { i.SourceTimezone = &s }},
		{"source_date_text", 4096, func(i *application.CaptureInput, s string) { i.SourceDateText = &s }},
	} {
		for _, n := range []int{tc.limit, tc.limit + 1} {
			input := application.CaptureInput{Title: "x"}
			tc.set(&input, strings.Repeat("a", n))
			_, err := application.PrepareCapture(input)
			if n == tc.limit {
				if err != nil {
					t.Fatal("exact bound rejected", tc.field, err)
				}
			} else {
				var typed *application.ValidationError
				if !errors.As(err, &typed) || typed.Field() != tc.field || typed.Rule() != "too_long" {
					t.Fatal("bound not enforced", tc.field, err)
				}
			}
		}
	}
	_, err := transport.DecodeCaptureJSON([]byte(`{"title":"x","buyer_name":42}`))
	var typeError *json.UnmarshalTypeError
	if !errors.As(err, &typeError) {
		t.Fatal("transport diagnostic cause discarded")
	}
	_, err = transport.DecodeCaptureJSON([]byte(`{"title":"x","buyer_name":null,"closes_at":"2026-02-30T12:00:00Z","source_timezone":"UTC","source_date_text":"source"}`))
	var parseError *time.ParseError
	if !errors.As(err, &parseError) {
		t.Fatal("application/domain diagnostic cause discarded")
	}
}

func TestApprovedCaptureExamplesDecode(t *testing.T) {
	data, err := os.ReadFile("../../api/openapi/drafts/manual-capture.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		BodyMax int `json:"x-request-body-max-bytes"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.BodyMax != transport.CaptureBodyLimit {
		t.Fatal("approved body limit drifted")
	}
	for _, key := range []string{"paths", "/v1/tenderpreneur/opportunities/manual", "post", "requestBody", "content", "application/json", "examples"} {
		var node map[string]json.RawMessage
		if err := json.Unmarshal(data, &node); err != nil {
			t.Fatal(err)
		}
		var ok bool
		data, ok = node[key]
		if !ok {
			t.Fatal("contract node missing", key)
		}
	}
	var examples map[string]struct{ Value json.RawMessage }
	if err := json.Unmarshal(data, &examples); err != nil {
		t.Fatal(err)
	}
	if len(examples) != 2 {
		t.Fatal("review example changes explicitly")
	}
	for name, example := range examples {
		t.Run(name, func(t *testing.T) {
			command, err := transport.DecodeCaptureJSON(example.Value)
			if err != nil {
				t.Fatal(err)
			}
			facts, ok := command.Facts()
			if !ok || facts.Verified() {
				t.Fatal("example acquired authority")
			}
		})
	}
}

func FuzzCaptureBody(f *testing.F) {
	for _, seed := range []string{`{"title":"x","buyer_name":null}`, `{"title":"\ud800","buyer_name":null}`, `{"title":"\ud83d\ude00","buyer_name":null}`, `null`, ``} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		command, err := transport.DecodeCaptureJSON(body)
		if err != nil {
			if _, ok := command.CanonicalJSON(); ok {
				t.Fatal("partial command on failure")
			}
			return
		}
		facts, ok := command.Facts()
		if !ok || facts.Verified() {
			t.Fatal("successful parse created authority")
		}
		canonical, _ := command.CanonicalJSON()
		if !json.Valid(canonical) {
			t.Fatal("invalid canonical JSON")
		}
	})
}
