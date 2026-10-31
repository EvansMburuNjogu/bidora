package localbootstrap

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
)

// Diagnostics discards raw subprocess output without storing, parsing or
// forwarding it. It is safe for concurrent stdout/stderr writes. The zero value
// is ready for use; counters saturate instead of wrapping. Do not copy after use.
// A future runner must still bound process lifetime and pipe-drain time.
type Diagnostics struct {
	mu     sync.Mutex
	bytes  uint64
	writes uint64
}

// DiagnosticCounts contains only safe metadata, never message contents.
type DiagnosticCounts struct {
	Bytes  uint64
	Writes uint64
}

func (d *Diagnostics) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	count := uint64(len(p))
	if count > math.MaxUint64-d.bytes {
		d.bytes = math.MaxUint64
	} else {
		d.bytes += count
	}
	if d.writes < math.MaxUint64 {
		d.writes++
	}
	return len(p), nil
}

func (d *Diagnostics) Format(state fmt.State, verb rune) {
	counts := d.Snapshot()
	fmt.Fprintf(state, "discarded_bytes=%d writes=%d", counts.Bytes, counts.Writes)
}

func (d *Diagnostics) MarshalJSON() ([]byte, error) { return json.Marshal(d.Snapshot()) }

func (d *Diagnostics) Snapshot() DiagnosticCounts {
	d.mu.Lock()
	defer d.mu.Unlock()
	return DiagnosticCounts{Bytes: d.bytes, Writes: d.writes}
}
