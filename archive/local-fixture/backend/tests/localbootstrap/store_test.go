//go:build darwin || linux

package localbootstrap_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"bidora/backend/internal/localbootstrap"
)

func storeLocation(t *testing.T) (string, string) {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir()) // macOS /var is a symlink.
	if err != nil {
		t.Fatal(err)
	}
	id, err := localbootstrap.NewExperimentID()
	if err != nil || len(id) != 32 {
		t.Fatal("identity generation failed")
	}
	return filepath.Join(parent, "fixture"), id
}

func createStore(t *testing.T) (*localbootstrap.Store, string, string) {
	t.Helper()
	path, id := storeLocation(t)
	s, err := localbootstrap.CreateStore(context.Background(), path, id)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s, path, id
}

func newCredential(t *testing.T) localbootstrap.Credential {
	t.Helper()
	c, err := localbootstrap.GenerateCredential()
	if err != nil {
		t.Fatal("credential generation failed")
	}
	return c
}

func TestStorePrivateCredentialsAndExclusiveOwnership(t *testing.T) {
	s, path, id := createStore(t)
	ctx := context.Background()
	if other, err := localbootstrap.CreateStore(ctx, path, id); err == nil || other != nil {
		t.Fatal("existing root adopted")
	}
	if other, err := localbootstrap.OpenStore(ctx, path, id); !errors.Is(err, localbootstrap.ErrStoreBusy) || other != nil {
		t.Fatal("second operator acquired store")
	}
	roles := []string{"bidora_local_bootstrap", "bidora_local_control_migrator", "bidora_local_control_runtime", "bidora_local_tenant_a_migrator", "bidora_local_tenant_a_runtime", "bidora_local_tenant_b_migrator", "bidora_local_tenant_b_runtime"}
	for _, role := range roles {
		c := newCredential(t)
		if err := s.WriteCredential(ctx, role, c); err != nil {
			t.Fatal(err)
		}
		data, err := s.ReadCredential(ctx, role)
		if err != nil || !bytes.Equal(data, c.Password()) {
			t.Fatal("credential did not round trip")
		}
		if err := s.WriteCredential(ctx, role, newCredential(t)); !errors.Is(err, localbootstrap.ErrStoreConflict) {
			t.Fatal("existing credential overwritten")
		}
		data[0] ^= 1
		again, err := s.ReadCredential(ctx, role)
		if err != nil || !bytes.Equal(again, c.Password()) {
			t.Fatal("read copy changed stored credential")
		}
		info, err := os.Stat(filepath.Join(path, "credential-"+role))
		if err != nil || info.Mode().Perm() != 0600 || info.Size() != 44 {
			t.Fatal("credential file permissions/shape unsafe")
		}
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("root is not private")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal("close is not idempotent")
	}
	if _, err := s.ReadCredential(ctx, roles[0]); !errors.Is(err, localbootstrap.ErrStoreClosed) {
		t.Fatal("closed store reused")
	}
	reopened, err := localbootstrap.OpenStore(ctx, path, id)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.ReadCredential(ctx, roles[6]); err != nil {
		t.Fatal("persisted credential lost")
	}
}

func TestStoreRejectsCredentialReuseAndUnsafeRoles(t *testing.T) {
	s, path, _ := createStore(t)
	ctx := context.Background()
	c := newCredential(t)
	if err := s.WriteCredential(ctx, "bidora_local_control_runtime", c); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteCredential(ctx, "bidora_local_tenant_a_runtime", c); !errors.Is(err, localbootstrap.ErrStoreConflict) {
		t.Fatal("cross-role credential reuse accepted")
	}
	for _, role := range []string{"", "../secret-canary", "postgres", "bidora_local_control_owner", "bidora_local_provisioner"} {
		if err := s.WriteCredential(ctx, role, c); err == nil || strings.Contains(err.Error(), "secret-canary") {
			t.Fatal("unsafe role accepted or disclosed")
		}
		if data, err := s.ReadCredential(ctx, role); err == nil || data != nil {
			t.Fatal("unsafe read accepted")
		}
	}
	if err := s.WriteCredential(ctx, "bidora_local_bootstrap", localbootstrap.Credential{}); err == nil {
		t.Fatal("empty credential persisted")
	}
	if _, err := os.Stat(filepath.Join(path, "credential-bidora_local_bootstrap")); !os.IsNotExist(err) {
		t.Fatal("invalid operation had filesystem effects")
	}
}

func TestStoredCredentialRestorationDoesNotRotateOrAdvance(t *testing.T) {
	s, path, id := createStore(t)
	ctx := context.Background()
	original := newCredential(t)
	role := "bidora_local_control_runtime"
	if err := s.WriteCredential(ctx, role, original); err != nil {
		t.Fatal(err)
	}
	if err := s.BeginStep(ctx, 0, "roles"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := localbootstrap.OpenStore(ctx, path, id)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	stored, err := reopened.ReadCredential(ctx, role)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := localbootstrap.RestoreCredential(stored)
	if err != nil || !bytes.Equal(restored.Password(), original.Password()) {
		t.Fatal("resume changed password")
	}
	verifySCRAM(t, restored)
	if _, err := restored.ActivationSQL(role); err != nil {
		t.Fatal(err)
	}
	after, err := reopened.ReadCredential(ctx, role)
	if err != nil || !bytes.Equal(stored, after) {
		t.Fatal("restoration changed private file")
	}
	cp, exists, err := reopened.Checkpoint(ctx)
	if err != nil || !exists || cp.Phase != "intent" || cp.Revision != 1 {
		t.Fatal("restoration implicitly advanced progress")
	}
	if err := reopened.BeginStep(ctx, 1, "control.create_database"); !errors.Is(err, localbootstrap.ErrStorePending) {
		t.Fatal("restoration bypassed reconciliation")
	}
}

func TestStoreCheckpointOrderAndCrashReconciliation(t *testing.T) {
	s, path, id := createStore(t)
	ctx := context.Background()
	if _, exists, err := s.Checkpoint(ctx); err != nil || exists {
		t.Fatal("new store has a checkpoint")
	}
	if err := s.CompleteStep(ctx, 0, "roles"); !errors.Is(err, localbootstrap.ErrStoreConflict) {
		t.Fatal("completion without intent accepted")
	}
	if err := s.BeginStep(ctx, 0, "tenant_a.create_database"); !errors.Is(err, localbootstrap.ErrStoreConflict) {
		t.Fatal("out-of-order step accepted")
	}
	if err := s.BeginStep(ctx, 0, "roles"); err != nil {
		t.Fatal(err)
	}
	if err := s.BeginStep(ctx, 1, "roles"); !errors.Is(err, localbootstrap.ErrStorePending) {
		t.Fatal("ambiguous intent retried")
	}
	if err := s.CompleteStep(ctx, 0, "roles"); !errors.Is(err, localbootstrap.ErrStoreConflict) {
		t.Fatal("stale revision accepted")
	}
	if err := s.CompleteStep(ctx, 1, "control.grants"); !errors.Is(err, localbootstrap.ErrStoreConflict) {
		t.Fatal("wrong completion accepted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := localbootstrap.OpenStore(ctx, path, id)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cp, exists, err := s.Checkpoint(ctx)
	if err != nil || !exists || cp.Phase != "intent" || cp.Revision != 1 || cp.StepID != "roles" || cp.Identity != id {
		t.Fatal("pending intent did not survive reopen")
	}
	if err := s.BeginStep(ctx, 1, "control.create_database"); !errors.Is(err, localbootstrap.ErrStorePending) {
		t.Fatal("reopen silently skipped unresolved work")
	}
	plan, err := localbootstrap.BuildPlan(fixtureConfig())
	if err != nil {
		t.Fatal(err)
	}
	for i, step := range plan.Steps {
		if i > 0 {
			if err := s.BeginStep(ctx, uint64(2*i), step.ID); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.CompleteStep(ctx, uint64(2*i+1), step.ID); err != nil {
			t.Fatal(err)
		}
	}
	cp, exists, err = s.Checkpoint(ctx)
	if err != nil || !exists || cp.Revision != 32 || cp.Phase != "complete" {
		t.Fatal("final state wrong")
	}
	if err := s.BeginStep(ctx, 32, "roles"); !errors.Is(err, localbootstrap.ErrStoreConflict) {
		t.Fatal("completed bootstrap restarted automatically")
	}
}

func TestStoreRefusesIncompleteAndCorruptRecords(t *testing.T) {
	for _, name := range []string{"pending temp", "marker without checkpoint", "deleted checkpoint", "deleted marker", "invalid revision", "wrong identity", "unknown JSON field", "duplicate JSON field", "truncated JSON", "extra JSON", "unknown file", "missing manifest", "missing lock"} {
		t.Run(name, func(t *testing.T) {
			s, path, id := createStore(t)
			ctx := context.Background()
			if err := s.BeginStep(ctx, 0, "roles"); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			checkpoint := filepath.Join(path, "checkpoint.json")
			var changeErr error
			switch name {
			case "pending temp":
				changeErr = os.WriteFile(filepath.Join(path, ".checkpoint-interrupted.pending"), []byte("partial"), 0600)
			case "marker without checkpoint", "deleted checkpoint":
				changeErr = os.Remove(checkpoint)
			case "deleted marker":
				changeErr = os.Remove(filepath.Join(path, "bootstrap.started"))
			case "unknown file":
				changeErr = os.WriteFile(filepath.Join(path, "unrelated"), nil, 0600)
			case "missing manifest":
				changeErr = os.Remove(filepath.Join(path, "manifest.json"))
			case "missing lock":
				changeErr = os.Remove(filepath.Join(path, "operation.lock"))
			default:
				data, err := os.ReadFile(checkpoint)
				if err != nil {
					t.Fatal(err)
				}
				text := string(data)
				switch name {
				case "invalid revision":
					text = strings.Replace(text, `"revision":1`, `"revision":999`, 1)
				case "wrong identity":
					text = strings.Replace(text, id, strings.Repeat("f", 32), 1)
				case "unknown JSON field":
					text = strings.Replace(text, "{", `{"unexpected":true,`, 1)
				case "duplicate JSON field":
					text = strings.Replace(text, "{", `{"format_version":1,`, 1)
				case "truncated JSON":
					text = "{"
				case "extra JSON":
					text += "{}"
				}
				changeErr = os.WriteFile(checkpoint, []byte(text), 0600)
			}
			if changeErr != nil {
				t.Fatal(changeErr)
			}
			if opened, err := localbootstrap.OpenStore(ctx, path, id); err == nil || opened != nil {
				t.Fatal("corrupt/partial store adopted")
			}
			if name == "pending temp" {
				if _, err := os.Stat(filepath.Join(path, ".checkpoint-interrupted.pending")); err != nil {
					t.Fatal("interrupted evidence was removed")
				}
			}
		})
	}
}

func TestStoreRejectsLinksPermissionsAndSpecialFiles(t *testing.T) {
	for _, mode := range []string{"root symlink", "parent symlink", "root permissions", "file permissions", "file symlink", "hardlink", "directory", "fifo", "duplicate credential"} {
		t.Run(mode, func(t *testing.T) {
			s, path, id := createStore(t)
			ctx := context.Background()
			if err := s.WriteCredential(ctx, "bidora_local_control_runtime", newCredential(t)); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			cred := filepath.Join(path, "credential-bidora_local_control_runtime")
			candidate := path
			var err error
			switch mode {
			case "root symlink":
				candidate = path + "-link"
				err = os.Symlink(path, candidate)
			case "parent symlink":
				parent := filepath.Dir(path)
				link := filepath.Join(parent, "parent-link")
				err = os.Symlink(parent, link)
				candidate = filepath.Join(link, "fixture")
			case "root permissions":
				err = os.Chmod(path, 0755)
			case "file permissions":
				err = os.Chmod(cred, 0644)
			case "file symlink":
				outside := filepath.Join(filepath.Dir(path), "outside-secret-canary")
				if err = os.WriteFile(outside, []byte("secret-canary"), 0600); err == nil {
					err = os.Remove(cred)
				}
				if err == nil {
					err = os.Symlink(outside, cred)
				}
			case "hardlink":
				err = os.Link(cred, filepath.Join(filepath.Dir(path), "outside-link"))
			case "directory":
				if err = os.Remove(cred); err == nil {
					err = os.Mkdir(cred, 0700)
				}
			case "fifo":
				if err = os.Remove(cred); err == nil {
					err = unix.Mkfifo(cred, 0600)
				}
			case "duplicate credential":
				var data []byte
				data, err = os.ReadFile(cred)
				if err == nil {
					err = os.WriteFile(filepath.Join(path, "credential-bidora_local_tenant_a_runtime"), data, 0600)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			opened, err := localbootstrap.OpenStore(ctx, candidate, id)
			if err == nil || opened != nil || strings.Contains(err.Error(), "secret-canary") || strings.Contains(err.Error(), path) {
				t.Fatal("unsafe store accepted or path/secret disclosed")
			}
		})
	}
}

func TestStoreDetectsActiveOwnershipDrift(t *testing.T) {
	for _, kind := range []string{"manifest", "lock", "root move", "root mode", "checkpoint removed"} {
		t.Run(kind, func(t *testing.T) {
			s, path, _ := createStore(t)
			ctx := context.Background()
			if err := s.BeginStep(ctx, 0, "roles"); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "manifest":
				if err := os.WriteFile(filepath.Join(path, "manifest.json"), []byte("{}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "lock":
				if err := os.Remove(filepath.Join(path, "operation.lock")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "operation.lock"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "root move":
				if err := os.Rename(path, path+"-moved"); err != nil {
					t.Fatal(err)
				}
			case "root mode":
				if err := os.Chmod(path, 0755); err != nil {
					t.Fatal(err)
				}
			case "checkpoint removed":
				if err := os.Remove(filepath.Join(path, "checkpoint.json")); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.WriteCredential(ctx, "bidora_local_control_runtime", newCredential(t)); err == nil {
				t.Fatal("mutated store remained writable")
			}
		})
	}
}

func TestStoreCancellationInvalidInputsAndWrongIdentity(t *testing.T) {
	path, id := storeLocation(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if s, err := localbootstrap.CreateStore(ctx, path, id); !errors.Is(err, context.Canceled) || s != nil {
		t.Fatal("cancellation not preserved")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("cancelled create had side effects")
	}
	for _, pair := range [][2]string{{"relative", id}, {"/", id}, {path + "/", id}, {path + "/../other", id}, {path, ""}, {path, strings.Repeat("A", 32)}, {path, "secret-canary"}} {
		if s, err := localbootstrap.CreateStore(context.Background(), pair[0], pair[1]); err == nil || s != nil || strings.Contains(err.Error(), "secret-canary") {
			t.Fatal("invalid store input accepted or echoed")
		}
	}
	if s, err := localbootstrap.CreateStore(nil, path, id); err == nil || s != nil {
		t.Fatal("nil context accepted")
	}
	s, path, _ := createStore(t)
	if err := s.BeginStep(ctx, 0, "roles"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled step accepted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if opened, err := localbootstrap.OpenStore(context.Background(), path, id); !errors.Is(err, localbootstrap.ErrStoreConflict) || opened != nil {
		t.Fatal("wrong experiment identity accepted")
	}
	var zero localbootstrap.Store
	if _, _, err := zero.Checkpoint(context.Background()); !errors.Is(err, localbootstrap.ErrStoreClosed) {
		t.Fatal("zero store did not fail safely")
	}
}

// Child invocations use only temporary synthetic state, never a database.
func TestStoreProcessHelper(t *testing.T) {
	mode := os.Getenv("BIDORA_STORE_TEST_CHILD")
	if mode == "" {
		return
	}
	s, err := localbootstrap.OpenStore(context.Background(), os.Getenv("BIDORA_STORE_TEST_PATH"), os.Getenv("BIDORA_STORE_TEST_ID"))
	if errors.Is(err, localbootstrap.ErrStoreBusy) {
		os.Exit(23)
	}
	if err != nil {
		os.Exit(24)
	}
	if mode == "intent-exit" {
		if err := s.BeginStep(context.Background(), 0, "roles"); err != nil {
			os.Exit(25)
		}
		os.Exit(0) // Deliberately omit Close: the OS must release the lock.
	}
	if err := s.Close(); err != nil {
		os.Exit(26)
	}
	os.Exit(0)
}

func runStoreChild(t *testing.T, path, id, mode string, expected int) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-test.run=^TestStoreProcessHelper$")
	cmd.Env = []string{"PATH=/usr/bin:/bin", "BIDORA_STORE_TEST_CHILD=" + mode, "BIDORA_STORE_TEST_PATH=" + path, "BIDORA_STORE_TEST_ID=" + id}
	err = cmd.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal("store helper could not execute")
		}
		code = exit.ExitCode()
	}
	if ctx.Err() != nil || code != expected {
		t.Fatalf("store helper exit %d; want %d", code, expected)
	}
}

func TestStoreCrossProcessLockAndExitRecovery(t *testing.T) {
	s, path, id := createStore(t)
	runStoreChild(t, path, id, "open", 23)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	runStoreChild(t, path, id, "open", 0)
	runStoreChild(t, path, id, "intent-exit", 0)
	reopened, err := localbootstrap.OpenStore(context.Background(), path, id)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	cp, exists, err := reopened.Checkpoint(context.Background())
	if err != nil || !exists || cp.Phase != "intent" || cp.Revision != 1 {
		t.Fatal("child exit lost intent or retained lock")
	}
}

func TestStoreConcurrentIntentAndAtomicCheckpoint(t *testing.T) {
	s, path, _ := createStore(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- s.BeginStep(ctx, 0, "roles") }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, localbootstrap.ErrStoreConflict) {
			t.Fatal("unexpected concurrent result")
		}
	}
	if success != 1 {
		t.Fatal("concurrent intent did not serialize")
	}
	stop := make(chan struct{})
	readerErrors := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			data, err := os.ReadFile(filepath.Join(path, "checkpoint.json"))
			if err != nil || !json.Valid(data) {
				if err == nil {
					err = errors.New("partial checkpoint visible")
				}
				readerErrors <- err
				return
			}
		}
	}()
	plan, err := localbootstrap.BuildPlan(fixtureConfig())
	if err != nil {
		t.Fatal(err)
	}
	for i, step := range plan.Steps {
		if i > 0 {
			if err := s.BeginStep(ctx, uint64(2*i), step.ID); err != nil {
				t.Error(err)
				break
			}
		}
		if err := s.CompleteStep(ctx, uint64(2*i+1), step.ID); err != nil {
			t.Error(err)
			break
		}
	}
	close(stop)
	wg.Wait()
	select {
	case err := <-readerErrors:
		t.Fatal(err)
	default:
	}
}
