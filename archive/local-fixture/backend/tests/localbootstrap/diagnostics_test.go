package localbootstrap_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"bidora/backend/internal/localbootstrap"
)

func TestDiagnosticsDiscardRawOutput(t *testing.T) {
	var sink localbootstrap.Diagnostics
	if n, err := sink.Write(nil); n != 0 || err != nil || sink.Snapshot() != (localbootstrap.DiagnosticCounts{}) {
		t.Fatal("empty write changed diagnostic metadata")
	}
	parts := []string{"secret-", "canary\x00\nSCRAM-SHA-256$4096:private", strings.Repeat("oversized-secret-canary", 100000)}
	var count uint64
	for _, part := range parts {
		n, err := sink.Write([]byte(part))
		if n != len(part) || err != nil {
			t.Fatal("discard did not consume output")
		}
		count += uint64(len(part))
	}
	if got := sink.Snapshot(); got.Bytes != count || got.Writes != uint64(len(parts)) {
		t.Fatal("incorrect safe diagnostic counts")
	}
	for _, value := range []any{&sink, sink.Snapshot()} {
		formatted := fmt.Sprintf("%+v", value)
		encoded, err := json.Marshal(value)
		if err != nil || strings.Contains(formatted+string(encoded), "canary") || strings.Contains(formatted+string(encoded), "SCRAM") {
			t.Fatal("diagnostics retained raw output")
		}
	}
}

func TestDiagnosticsConcurrentStreams(t *testing.T) {
	var sink localbootstrap.Diagnostics
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if _, err := sink.Write([]byte("secret-canary")); err != nil {
					t.Error("discard failed")
				}
				_ = sink.Snapshot()
				_ = fmt.Sprintf("%#v", &sink)
				if _, err := json.Marshal(&sink); err != nil {
					t.Error("safe diagnostic snapshot failed")
				}
			}
		}()
	}
	wg.Wait()
	if got := sink.Snapshot(); got.Writes != 800 || got.Bytes != 800*uint64(len("secret-canary")) {
		t.Fatal("concurrent counters lost updates")
	}
}
