//go:build darwin || linux

package localbootstrap

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

// StoreCode is a safe error code; errors.Is also reaches wrapped OS causes.
type StoreCode string

func (c StoreCode) Error() string { return string(c) }

const (
	ErrStoreUnsafe    StoreCode = "fixture_store_unsafe"
	ErrStoreBusy      StoreCode = "fixture_store_busy"
	ErrStoreConflict  StoreCode = "fixture_store_conflict"
	ErrStorePending   StoreCode = "fixture_store_pending_reconciliation"
	ErrStoreUncertain StoreCode = "fixture_store_write_uncertain"
	ErrStoreClosed    StoreCode = "fixture_store_closed"
)

type storeError struct {
	code  StoreCode
	cause error
}

func (e *storeError) Error() string                  { return string(e.code) }
func (e *storeError) Unwrap() error                  { return e.cause }
func (e *storeError) Is(target error) bool           { return target == e.code }
func storeFailure(code StoreCode, cause error) error { return &storeError{code, cause} }

// Store owns an exclusive advisory lock until Close. Directory operations are
// anchored to a descriptor; every path component rejects symlinks. This protects
// against accidental adoption and cooperating writers, not a malicious host user.
// Never copy a Store. It does not create/inspect/start a PostgreSQL data directory.
type Store struct {
	mu        sync.Mutex
	root      *os.File
	lock      *os.File
	identity  string
	path      string
	closed    bool
	uncertain bool
}

type manifest struct {
	Format   int    `json:"format_version"`
	Identity string `json:"experiment_id"`
	Postgres string `json:"postgres_version"`
	Root     string `json:"root"`
}

// Checkpoint records intent/completion of a fixed SQL-plan step only. Completion
// is a caller assertion after reconciliation, not proof a SQL effect occurred.
type Checkpoint struct {
	Format   int    `json:"format_version"`
	Identity string `json:"experiment_id"`
	Revision uint64 `json:"revision"`
	StepID   string `json:"step_id"`
	Phase    string `json:"phase"`
}

// NewExperimentID supplies a random identity, not an authorization credential.
func NewExperimentID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", storeFailure(ErrStoreUnsafe, err)
	}
	return hex.EncodeToString(id[:]), nil
}

// CreateStore creates exactly one new 0700 leaf beneath an existing parent.
// Existing targets are never adopted or repaired. Failed partial creations are
// retained for inspection, not deleted or silently retried.
func CreateStore(ctx context.Context, path, identity string) (*Store, error) {
	return openStore(ctx, path, identity, true)
}

// OpenStore acquires an existing store's lock without creating missing files.
func OpenStore(ctx context.Context, path, identity string) (*Store, error) {
	return openStore(ctx, path, identity, false)
}

func openStore(ctx context.Context, path, identity string, create bool) (result *Store, resultErr error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if !validIdentity(identity) || !utf8.ValidString(path) || !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" || len(path) > 1024 || strings.IndexFunc(path, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
		return nil, ErrStoreUnsafe
	}
	parent, err := openDirectory(filepath.Dir(path))
	if err != nil {
		return nil, storeFailure(ErrStoreUnsafe, err)
	}
	defer func() {
		if closeErr := unix.Close(parent); closeErr != nil {
			if result != nil {
				closeErr = errors.Join(closeErr, result.Close())
				result = nil
			}
			resultErr = errors.Join(resultErr, storeFailure(ErrStoreUnsafe, closeErr))
		}
	}()
	if create {
		if err := unix.Mkdirat(parent, filepath.Base(path), 0700); err != nil {
			return nil, storeFailure(ErrStoreConflict, err)
		}
	}
	fd, err := unix.Openat(parent, filepath.Base(path), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, storeFailure(ErrStoreUnsafe, err)
	}
	s := &Store{root: os.NewFile(uintptr(fd), "fixture-store"), identity: identity, path: path}
	ok := false
	defer func() {
		if !ok {
			resultErr = errors.Join(resultErr, s.Close())
		}
	}()
	if err := validateFD(fd, true); err != nil {
		return nil, storeFailure(ErrStoreUnsafe, err)
	}
	flags := unix.O_RDWR | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
	if create {
		flags |= unix.O_CREAT | unix.O_EXCL
	}
	lockFD, err := unix.Openat(fd, "operation.lock", flags, 0600)
	if err != nil {
		return nil, storeFailure(ErrStoreUnsafe, err)
	}
	s.lock = os.NewFile(uintptr(lockFD), "fixture-lock")
	if err := validateFD(lockFD, false); err != nil {
		return nil, storeFailure(ErrStoreUnsafe, err)
	}
	var st unix.Stat_t
	if err := unix.Fstat(lockFD, &st); err != nil || st.Size != 0 {
		return nil, storeFailure(ErrStoreUnsafe, err)
	}
	if err := unix.Flock(lockFD, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, storeFailure(ErrStoreBusy, err)
		}
		return nil, storeFailure(ErrStoreUnsafe, err)
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	want := manifest{1, identity, "15.19", path}
	if create {
		data, err := encodeRecord(want)
		if err != nil {
			return nil, err
		}
		if err := s.writeNew("manifest.json", data); err != nil {
			return nil, err
		}
		if err := s.lock.Sync(); err != nil {
			return nil, storeFailure(ErrStoreUncertain, err)
		}
		if err := s.root.Sync(); err != nil {
			return nil, storeFailure(ErrStoreUncertain, err)
		}
		if err := unix.Fsync(parent); err != nil {
			return nil, storeFailure(ErrStoreUncertain, err)
		}
	} else {
		data, err := s.readPrivate("manifest.json", 8192)
		if err != nil {
			return nil, err
		}
		var got manifest
		if err := decodeRecord(data, &got); err != nil {
			return nil, err
		}
		if got != want {
			return nil, ErrStoreConflict
		}
	}
	if err := s.inspectEntries(); err != nil {
		return nil, err
	}
	if _, _, err := s.readCheckpoint(); err != nil {
		return nil, err
	}
	ok = true
	return s, nil
}

func openDirectory(path string) (int, error) {
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" {
			continue
		}
		next, err := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		closeErr := unix.Close(fd)
		if err != nil {
			return -1, err
		}
		if closeErr != nil {
			return -1, errors.Join(closeErr, unix.Close(next))
		}
		fd = next
	}
	return fd, nil
}

func validateFD(fd int, directory bool) error {
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return err
	}
	kind, mode := uint32(unix.S_IFREG), uint32(0600)
	if directory {
		kind, mode = unix.S_IFDIR, 0700
	}
	if uint32(st.Mode)&unix.S_IFMT != kind || uint32(st.Mode)&07777 != mode || st.Uid != uint32(os.Geteuid()) || (!directory && st.Nlink != 1) {
		return ErrStoreUnsafe
	}
	return nil
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return ErrStoreUnsafe
	}
	if err := ctx.Err(); err != nil {
		return storeFailure(ErrStoreUnsafe, err)
	}
	return nil
}

func validIdentity(id string) bool {
	if len(id) != 32 {
		return false
	}
	decoded, err := hex.DecodeString(id)
	return err == nil && hex.EncodeToString(decoded) == id
}

func encodeRecord(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, storeFailure(ErrStoreUnsafe, err)
	}
	return append(data, '\n'), nil
}

// Internal records are canonical: reject duplicate/unknown keys, alternate
// formatting, trailing data and truncated JSON rather than normalizing drift.
func decodeRecord(data []byte, value any) error {
	if err := json.Unmarshal(data, value); err != nil {
		return storeFailure(ErrStoreUnsafe, err)
	}
	canonical, err := encodeRecord(value)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, canonical) {
		return ErrStoreUnsafe
	}
	return nil
}

func (s *Store) ready(ctx context.Context) error {
	if s.closed || s.root == nil || s.lock == nil {
		return ErrStoreClosed
	}
	if s.uncertain {
		return ErrStoreUncertain
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := validateFD(int(s.root.Fd()), true); err != nil {
		return storeFailure(ErrStoreUnsafe, err)
	}
	if err := validateFD(int(s.lock.Fd()), false); err != nil {
		return storeFailure(ErrStoreUnsafe, err)
	}
	current, err := openDirectory(s.path)
	if err != nil {
		return storeFailure(ErrStoreUnsafe, err)
	}
	var actual, held unix.Stat_t
	statErr := unix.Fstat(current, &actual)
	closeErr := unix.Close(current)
	if statErr != nil || closeErr != nil {
		return storeFailure(ErrStoreUnsafe, errors.Join(statErr, closeErr))
	}
	if err := unix.Fstat(int(s.root.Fd()), &held); err != nil {
		return storeFailure(ErrStoreUnsafe, err)
	}
	if actual.Dev != held.Dev || actual.Ino != held.Ino {
		return ErrStoreConflict
	}
	if err := unix.Fstatat(int(s.root.Fd()), "operation.lock", &actual, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return storeFailure(ErrStoreUnsafe, err)
	}
	if err := unix.Fstat(int(s.lock.Fd()), &held); err != nil {
		return storeFailure(ErrStoreUnsafe, err)
	}
	if actual.Dev != held.Dev || actual.Ino != held.Ino {
		return ErrStoreConflict
	}
	if err := s.inspectEntries(); err != nil {
		return err
	}
	data, err := s.readPrivate("manifest.json", 8192)
	if err != nil {
		return err
	}
	var got manifest
	if err := decodeRecord(data, &got); err != nil {
		return err
	}
	if got != (manifest{1, s.identity, "15.19", s.path}) {
		return ErrStoreConflict
	}
	if _, _, err := s.readCheckpoint(); err != nil {
		return err
	}
	return checkContext(ctx)
}

func credentialName(role string) (string, error) {
	if role != "bidora_local_bootstrap" && !allowedCredentialRole(role) {
		return "", ErrStoreUnsafe
	}
	return "credential-" + role, nil
}

func (s *Store) inspectEntries() error {
	fd, err := unix.Openat(int(s.root.Fd()), ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return storeFailure(ErrStoreUnsafe, err)
	}
	f := os.NewFile(uintptr(fd), "fixture-entries")
	entries, readErr := f.ReadDir(32)
	closeErr := f.Close()
	if readErr != nil && readErr != io.EOF {
		return storeFailure(ErrStoreUnsafe, readErr)
	}
	if closeErr != nil {
		return storeFailure(ErrStoreUnsafe, closeErr)
	}
	seenCredentials := make(map[[32]byte]bool)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".checkpoint-") {
			return ErrStoreUncertain
		}
		limit := 0
		switch name {
		case "operation.lock":
			limit = 0
		case "manifest.json":
			limit = 8192
		case "checkpoint.json":
			limit = 2048
		case "bootstrap.started":
			limit = 33
		default:
			if expected, err := credentialName(strings.TrimPrefix(name, "credential-")); err != nil || expected != name {
				return ErrStoreUnsafe
			}
			limit = 44
		}
		data, err := s.readPrivate(name, limit)
		if err != nil {
			return err
		}
		if strings.HasPrefix(name, "credential-") {
			if !validPasswordFile(data) {
				return ErrStoreUnsafe
			}
			digest := sha256.Sum256(data)
			if seenCredentials[digest] {
				return ErrStoreConflict
			}
			seenCredentials[digest] = true
		}
	}
	if len(entries) > 11 {
		return ErrStoreUnsafe
	}
	return nil
}

func (s *Store) readPrivate(name string, limit int) ([]byte, error) {
	fd, err := unix.Openat(int(s.root.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, storeFailure(ErrStoreUnsafe, err)
	}
	f := os.NewFile(uintptr(fd), "fixture-private-file")
	if err := validateFD(fd, false); err != nil {
		return nil, storeFailure(ErrStoreUnsafe, errors.Join(err, f.Close()))
	}
	data, readErr := io.ReadAll(io.LimitReader(f, int64(limit+1)))
	closeErr := f.Close()
	if readErr != nil {
		return nil, storeFailure(ErrStoreUnsafe, readErr)
	}
	if closeErr != nil {
		return nil, storeFailure(ErrStoreUnsafe, closeErr)
	}
	if len(data) > limit {
		return nil, ErrStoreUnsafe
	}
	return data, nil
}

func (s *Store) writeNew(name string, data []byte) error {
	fd, err := unix.Openat(int(s.root.Fd()), name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return storeFailure(ErrStoreConflict, err)
	}
	f := os.NewFile(uintptr(fd), "fixture-new-file")
	if err := validateFD(fd, false); err != nil {
		s.uncertain = true
		return storeFailure(ErrStoreUnsafe, errors.Join(err, f.Close()))
	}
	n, writeErr := f.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		s.uncertain = true
		return storeFailure(ErrStoreUncertain, errors.Join(writeErr, closeErr))
	}
	return nil
}

func validPasswordFile(data []byte) bool {
	if len(data) != 44 || data[43] != '\n' {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(string(data[:43]))
	return err == nil && len(decoded) == 32 && base64.RawURLEncoding.EncodeToString(decoded) == string(data[:43])
}

// WriteCredential exclusively creates a private file. Existing credentials are
// never overwritten, rotated or repaired by retries.
func (s *Store) WriteCredential(ctx context.Context, role string, credential Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(ctx); err != nil {
		return err
	}
	name, err := credentialName(role)
	if err != nil {
		return err
	}
	data := append(credential.Password(), '\n')
	if !validPasswordFile(data) {
		return ErrStoreUnsafe
	}
	roles := []string{"bidora_local_bootstrap"}
	for _, target := range targets() {
		roles = append(roles, target.migrator, target.runtime)
	}
	for _, existingRole := range roles {
		if existingRole == role {
			continue
		}
		existing, err := s.readPrivate("credential-"+existingRole, 44)
		if errors.Is(err, unix.ENOENT) {
			continue
		}
		if err != nil {
			return err
		}
		if bytes.Equal(existing, data) {
			return ErrStoreConflict
		}
	}
	if err := s.writeNew(name, data); err != nil {
		return err
	}
	if err := s.root.Sync(); err != nil {
		s.uncertain = true
		return storeFailure(ErrStoreUncertain, err)
	}
	return nil
}

// ReadCredential returns sensitive bytes explicitly; never format/log them.
func (s *Store) ReadCredential(ctx context.Context, role string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	name, err := credentialName(role)
	if err != nil {
		return nil, err
	}
	data, err := s.readPrivate(name, 44)
	if err != nil {
		return nil, err
	}
	if !validPasswordFile(data) {
		return nil, ErrStoreUnsafe
	}
	return data[:43], nil
}

func orderedStepIDs() []string {
	ids := []string{"roles"}
	for _, t := range targets() {
		ids = append(ids, t.name+".create_database", t.name+".grants", t.migrator+".activate", t.runtime+".activate", t.name+".schema")
	}
	return ids
}

func (s *Store) readCheckpoint() (Checkpoint, bool, error) {
	marker, markerErr := s.readPrivate("bootstrap.started", 33)
	started := markerErr == nil
	if markerErr != nil && !errors.Is(markerErr, unix.ENOENT) {
		return Checkpoint{}, false, markerErr
	}
	if started && string(marker) != s.identity+"\n" {
		return Checkpoint{}, false, ErrStoreConflict
	}
	data, err := s.readPrivate("checkpoint.json", 2048)
	if errors.Is(err, unix.ENOENT) {
		if started {
			return Checkpoint{}, false, ErrStoreUncertain
		}
		return Checkpoint{}, false, nil
	}
	if err != nil {
		return Checkpoint{}, false, err
	}
	if !started {
		return Checkpoint{}, false, ErrStoreUnsafe
	}
	var cp Checkpoint
	if err := decodeRecord(data, &cp); err != nil {
		return Checkpoint{}, false, err
	}
	if cp.Format != 1 || cp.Identity != s.identity {
		return Checkpoint{}, false, ErrStoreConflict
	}
	for i, step := range orderedStepIDs() {
		if step == cp.StepID {
			if (cp.Phase == "intent" && cp.Revision == uint64(2*i+1)) || (cp.Phase == "complete" && cp.Revision == uint64(2*i+2)) {
				return cp, true, nil
			}
			break
		}
	}
	return Checkpoint{}, false, ErrStoreUnsafe
}

func (s *Store) Checkpoint(ctx context.Context) (Checkpoint, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(ctx); err != nil {
		return Checkpoint{}, false, err
	}
	return s.readCheckpoint()
}

// BeginStep records and fsyncs the next fixed intent. A pending intent blocks
// further progress until its effect is reconciled; it never triggers SQL retry.
func (s *Store) BeginStep(ctx context.Context, expectedRevision uint64, stepID string) error {
	return s.advance(ctx, expectedRevision, stepID, false)
}

// CompleteStep records a caller-verified effect. This method cannot establish
// that SQL executed, and must not be called merely because a request was sent.
func (s *Store) CompleteStep(ctx context.Context, expectedRevision uint64, stepID string) error {
	return s.advance(ctx, expectedRevision, stepID, true)
}

func (s *Store) advance(ctx context.Context, expectedRevision uint64, stepID string, complete bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(ctx); err != nil {
		return err
	}
	old, exists, err := s.readCheckpoint()
	if err != nil {
		return err
	}
	if old.Revision != expectedRevision {
		return ErrStoreConflict
	}
	ids := orderedStepIDs()
	if complete {
		if !exists || old.Phase != "intent" || old.StepID != stepID {
			return ErrStoreConflict
		}
	} else {
		if exists && old.Phase == "intent" {
			return ErrStorePending
		}
		index := int(old.Revision / 2)
		if index >= len(ids) || ids[index] != stepID {
			return ErrStoreConflict
		}
	}
	if !exists {
		if err := s.writeNew("bootstrap.started", []byte(s.identity+"\n")); err != nil {
			return err
		}
		if err := s.root.Sync(); err != nil {
			s.uncertain = true
			return storeFailure(ErrStoreUncertain, err)
		}
	}
	phase := "intent"
	if complete {
		phase = "complete"
	}
	cp := Checkpoint{1, s.identity, old.Revision + 1, stepID, phase}
	data, err := encodeRecord(cp)
	if err != nil {
		return err
	}
	id, err := NewExperimentID()
	if err != nil {
		return err
	}
	tmp := ".checkpoint-" + id + ".pending"
	if err := s.writeNew(tmp, data); err != nil {
		return err
	}
	if err := checkContext(ctx); err != nil {
		s.uncertain = true
		return storeFailure(ErrStoreUncertain, err)
	}
	if err := unix.Renameat(int(s.root.Fd()), tmp, int(s.root.Fd()), "checkpoint.json"); err != nil {
		s.uncertain = true
		return storeFailure(ErrStoreUncertain, err)
	}
	if err := s.root.Sync(); err != nil {
		s.uncertain = true
		return storeFailure(ErrStoreUncertain, err)
	}
	return nil
}

// Close releases the advisory operation lock. Partial/uncertain artifacts are
// preserved. Idempotent close never deletes credentials or the current checkpoint.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	var errs []error
	if s.lock != nil {
		errs = append(errs, s.lock.Close())
	}
	if s.root != nil {
		errs = append(errs, s.root.Close())
	}
	if err := errors.Join(errs...); err != nil {
		return storeFailure(ErrStoreUnsafe, err)
	}
	return nil
}
