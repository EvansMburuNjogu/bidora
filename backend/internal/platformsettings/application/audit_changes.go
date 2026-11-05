package application

import "encoding/json"

// FieldChange records only explicitly selected, non-secret settings fields.
// An empty value is represented as absent, distinguishing additions/deletions.
type FieldChange struct {
	Field     string  `json:"field"`
	Operation string  `json:"operation"`
	Before    *string `json:"before"`
	After     *string `json:"after"`
}

func settingChange(field, before, after string) []FieldChange {
	if before == after {
		return nil
	}
	change := FieldChange{Field: field, Operation: "MODIFIED"}
	if before == "" {
		change.Operation = "ADDED"
	} else {
		v := before
		change.Before = &v
	}
	if after == "" {
		change.Operation = "DELETED"
	} else {
		v := after
		change.After = &v
	}
	return []FieldChange{change}
}

// Only caller-listed public configuration fields are compared; never marshal
// these inputs directly into an audit record.
func selectedFieldChanges(before, after any, fields ...string) []FieldChange {
	decode := func(value any) map[string]json.RawMessage {
		raw, _ := json.Marshal(value)
		var result map[string]json.RawMessage
		_ = json.Unmarshal(raw, &result)
		return result
	}
	old, next := decode(before), decode(after)
	text := func(raw json.RawMessage) string {
		if len(raw) == 0 || string(raw) == "null" {
			return ""
		}
		var v string
		if json.Unmarshal(raw, &v) == nil {
			return v
		}
		return string(raw)
	}
	var changes []FieldChange
	for _, field := range fields {
		changes = append(changes, settingChange(field, text(old[field]), text(next[field]))...)
	}
	return changes
}
func textValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
