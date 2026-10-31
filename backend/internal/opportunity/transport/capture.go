// Package transport binds capture input. It registers no HTTP route and grants
// no authentication, CSRF, membership, document access or persistence authority.
package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"

	"bidora/backend/internal/opportunity/application"
)

const CaptureBodyLimit = 32 * 1024

// DecodeCaptureJSON enforces the approved operation's bounded, strict body.
// Future handlers must also cap streaming input before buffering it here.
func DecodeCaptureJSON(body []byte) (application.CaptureCommand, error) {
	if len(body) > CaptureBodyLimit {
		return application.CaptureCommand{}, failure("REQUEST_TOO_LARGE", "body", "too_large", nil)
	}
	if !utf8.Valid(body) {
		return application.CaptureCommand{}, failure("INVALID_REQUEST", "body", "invalid_utf8", nil)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil {
		return application.CaptureCommand{}, failure("INVALID_REQUEST", "body", "invalid_json", err)
	}
	if token != json.Delim('{') {
		return application.CaptureCommand{}, failure("INVALID_REQUEST", "body", "object_required", nil)
	}
	seen := make(map[string]bool)
	var input application.CaptureInput
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return application.CaptureCommand{}, failure("INVALID_REQUEST", "body", "invalid_json", err)
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return application.CaptureCommand{}, failure("INVALID_REQUEST", "body", "duplicate_property", nil)
		}
		seen[key] = true
		switch key {
		case "title", "buyer_name", "source_uri", "source_document_version_id", "closes_at", "source_timezone", "source_date_text":
		default:
			return application.CaptureCommand{}, failure("INVALID_REQUEST", "body", "unknown_property", nil)
		}
		var raw json.RawMessage
		if err = decoder.Decode(&raw); err != nil {
			return application.CaptureCommand{}, failure("INVALID_REQUEST", "body", "invalid_json", err)
		}
		var value *string
		if err = json.Unmarshal(raw, &value); err != nil {
			return application.CaptureCommand{}, failure("VALIDATION_FAILED", key, "invalid_type", err)
		}
		if !validSurrogates(raw) {
			return application.CaptureCommand{}, failure("VALIDATION_FAILED", key, "invalid_unicode", nil)
		}
		switch key {
		case "title":
			if value == nil {
				return application.CaptureCommand{}, failure("VALIDATION_FAILED", key, "required", nil)
			}
			input.Title = *value
		case "buyer_name":
			input.BuyerName = value
		case "source_uri":
			input.SourceURI = value
		case "source_document_version_id":
			input.SourceDocumentVersionID = value
		case "closes_at":
			input.ClosesAt = value
		case "source_timezone":
			input.SourceTimezone = value
		case "source_date_text":
			input.SourceDateText = value
		}
	}
	if _, err = decoder.Token(); err != nil {
		return application.CaptureCommand{}, failure("INVALID_REQUEST", "body", "invalid_json", err)
	}
	if _, err = decoder.Token(); err != io.EOF {
		return application.CaptureCommand{}, failure("INVALID_REQUEST", "body", "trailing_json", err)
	}
	for _, key := range []string{"title", "buyer_name"} {
		if !seen[key] {
			return application.CaptureCommand{}, failure("VALIDATION_FAILED", key, "required", nil)
		}
	}
	return application.PrepareCapture(input)
}

// encoding/json replaces unpaired UTF-16 escapes with U+FFFD. Reject them
// instead of silently changing source wording. Called only after JSON parsing.
func validSurrogates(raw []byte) bool {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		value, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if value >= 0xDC00 && value <= 0xDFFF {
			return false
		}
		if value >= 0xD800 && value <= 0xDBFF {
			if i+7 > len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if err != nil || low < 0xDC00 || low > 0xDFFF {
				return false
			}
			i += 6
		}
	}
	return true
}

type DecodeError struct {
	code, field, rule string
	cause             error
}

func failure(code, field, rule string, cause error) error {
	return &DecodeError{code, field, rule, cause}
}
func (e *DecodeError) Error() string                 { return e.code }
func (e *DecodeError) Code() string                  { return e.code }
func (e *DecodeError) Field() string                 { return e.field }
func (e *DecodeError) Rule() string                  { return e.rule }
func (e *DecodeError) Unwrap() error                 { return e.cause }
func (e *DecodeError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.code) }
