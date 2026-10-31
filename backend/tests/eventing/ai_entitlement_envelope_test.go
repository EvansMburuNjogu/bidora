package eventing_test

import (
	"encoding/json"
	"testing"

	"bidora/backend/internal/eventing"
)

func TestAIEntitlementEnvelopePreservesEventAndActorProvenance(t *testing.T) {
	raw, err := json.Marshal(eventing.Envelope{EventID: "event-1", ActorID: "actor-1"})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["event_id"]) != `"event-1"` || string(fields["actor_id"]) != `"actor-1"` {
		t.Fatal("AI entitlement envelope lost event or actor provenance", string(raw))
	}
}
