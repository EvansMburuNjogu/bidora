package localbudget

import (
	"context"
	"fmt"
	"time"
)

// Storage is a bounded observation of explicitly selected artifact directories,
// not an atomic filesystem snapshot or proof all experiment roots were selected.
// AllocatedBytes uses native allocated-block metadata; LogicalBytes counts
// regular-file lengths. Hard-linked inodes count once across all selected roots.
type Storage struct {
	StartedAt      time.Time
	FinishedAt     time.Time
	Roots          uint64
	Entries        uint64
	AllocatedBytes uint64
	LogicalBytes   uint64
}

type storageError struct{ cause error }

func (*storageError) Error() string { return "native_storage_measurement_failed" }
func (e *storageError) Unwrap() error { return e.cause }
func (*storageError) Format(s fmt.State, verb rune) { fmt.Fprint(s, "native_storage_measurement_failed") }

// CollectStorage reads only directory entries and metadata. On failure it
// returns an empty result and a safe error with a retained diagnostic cause.
// Cancellation/deadline checks bound traversal work, not a kernel filesystem
// call that is blocked on a stalled device or network mount.
func CollectStorage(ctx context.Context, roots []string) (Storage, error) {
	if ctx == nil { return Storage{}, &storageError{fmt.Errorf("context required")} }
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	result, err := collectStorage(ctx, roots)
	if err != nil { return Storage{}, &storageError{err} }
	return result, nil
}
