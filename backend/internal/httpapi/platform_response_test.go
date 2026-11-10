package httpapi

import (
	"encoding/json"
	"testing"
)

func TestPlatformResponseCursorContract(t *testing.T) {
	next := "next-page"
	for _, cursor := range []*string{nil, &next} {
		encoded, err := json.Marshal(platformResponse{Data: map[string]any{"administrators": []any{}}, Meta: platformMeta{TraceID: "test-trace", NextCursor: cursor}})
		if err != nil {
			t.Fatal(err)
		}
		var response struct {
			Meta map[string]json.RawMessage `json:"meta"`
		}
		if err := json.Unmarshal(encoded, &response); err != nil {
			t.Fatal(err)
		}
		value, exists := response.Meta["next_cursor"]
		if !exists {
			t.Fatal("last-page cursor must be present as null, not omitted")
		}
		if cursor == nil && string(value) != "null" {
			t.Fatal("last-page cursor must be null")
		}
		if cursor != nil && string(value) != `"next-page"` {
			t.Fatal("next-page cursor changed")
		}
	}
}
