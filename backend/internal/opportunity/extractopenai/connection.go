package extractopenai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// CheckConnection performs a tiny, explicitly requested billable structured-output
// test through the same allowlisted transport. Never called at startup/readiness.
func (e *Extractor) CheckConnection(ctx context.Context) error {
	if ctx == nil || e == nil || e.client == nil {
		return errors.New("openai_test_unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	payload := map[string]any{"model": e.model, "store": false, "max_output_tokens": 64, "input": "Return the JSON object with ok set to true.", "text": map[string]any{"format": map[string]any{"type": "json_schema", "name": "bidora_connection_test", "strict": true, "schema": map[string]any{"type": "object", "properties": map[string]any{"ok": map[string]any{"type": "boolean", "enum": []bool{true}}}, "required": []string{"ok"}, "additionalProperties": false}}}}
	body, err := json.Marshal(payload)
	if err != nil {
		return errors.New("openai_test_invalid")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("openai_test_invalid")
	}
	request.Header.Set("Authorization", "Bearer "+e.key)
	request.Header.Set("Content-Type", "application/json")
	response, err := e.client.Do(request)
	if err != nil {
		return errors.New("openai_test_unavailable")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || response.StatusCode != 200 || len(raw) > 65536 {
		return errors.New("openai_test_failed")
	}
	var result struct {
		Status string `json:"status"`
		Model  string `json:"model"`
		Output []struct {
			Type    string                        `json:"type"`
			Content []struct{ Type, Text string } `json:"content"`
		} `json:"output"`
	}
	if rejectDuplicateJSON(raw) != nil || json.Unmarshal(raw, &result) != nil || result.Status != "completed" || (result.Model != e.model && !strings.HasPrefix(result.Model, e.model+"-")) {
		return errors.New("openai_test_invalid_response")
	}
	texts := 0
	for _, message := range result.Output {
		for _, content := range message.Content {
			if content.Type == "refusal" {
				return errors.New("openai_test_refused")
			}
			if content.Type == "output_text" {
				var value struct {
					OK bool `json:"ok"`
				}
				if strictJSON([]byte(strings.TrimSpace(content.Text)), &value) != nil || !value.OK {
					return errors.New("openai_test_invalid_response")
				}
				texts++
			}
		}
	}
	if texts != 1 {
		return errors.New("openai_test_invalid_response")
	}
	return nil
}
func (e *Extractor) Close() {
	if e != nil && e.client != nil {
		e.client.CloseIdleConnections()
	}
}
