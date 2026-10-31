# Private fixture storage and checkpoint preparation — BID-FND-004

> Historical evidence only. Tooling source/tests are now in [the retired archive](../../archive/local-fixture/README.md). Do not execute or resume these instructions by default.

**VERIFIED_TECHNICALLY for disconnected filesystem helpers and tests; no live bootstrap execution.** This continues the user's approved bootstrap implementation without starting PostgreSQL. No real fixture root, database or web service was created/started by this increment. Filesystem tests use owned temporary directories and synthetic credentials only.

## Implemented boundary

`backend/internal/localbootstrap/store_unix.go` provides the following on **Darwin and Linux** (build-constrained; no insecure fallback on other platforms):

- `NewExperimentID`: 16 random bytes encoded as 32 lowercase hex characters. This is a target identity, not an authorization credential.
- `CreateStore(ctx, path, identity)`: create exactly one new private leaf beneath an existing parent; refuse any existing target rather than adopt or repair it.
- `OpenStore`: open only an existing store with matching format, identity, PostgreSQL 15.19 marker and canonical target path. Missing files are not created by open.
- `WriteCredential` / `ReadCredential`: explicit access for the bootstrap administrator and six fixed migrator/runtime roles only. Owners/provisioner/unknown/path-like names fail. Each persisted password is 43 ASCII base64url characters plus a newline, representing 32 random bytes. Empty/noncanonical files fail. Existing credentials are never overwritten, and reuse of the same password across roles is rejected.
- `BeginStep` / `CompleteStep` / `Checkpoint`: fixed SQL-plan ordering, expected revisions, explicit intent/completion and refusal to skip or automatically retry unresolved work.
- `Close`: release the exclusive operation lock without deleting metadata or credentials. Repeated close is safe.

No new command invokes this API yet. The current `local-check` remains non-mutating. These helpers do not validate a PostgreSQL data directory or start/connect to a server.

## Filesystem safeguards

All path components are opened relative to directory descriptors with `O_NOFOLLOW`; paths must be explicit, clean, absolute UTF-8 paths. There is no relative/CWD fallback, symlink normalization/adoption or implicit parent-directory creation. The new root must belong to the effective user and have mode **0700**. Regular metadata/credential files must belong to that user, have mode **0600**, and have exactly one hard link. Symlinks, hard links, special files, unexpected entries and unsafe modes fail closed.

Reads are size-bounded and special-file opens are nonblocking. Mutations recheck root identity at the selected path, root permissions, lock-file inode, manifest and checkpoint state. A renamed/replaced root or replaced/unlinked lock cannot silently redirect an active handle. File creation is exclusive; no truncate/rewrite of existing credential files is provided.

The operation lock uses an exclusive nonblocking advisory `flock` held for the Store lifetime. A second cooperating process fails with `fixture_store_busy`; there is no hidden wait or retry loop. Store methods also serialize goroutines and coordinate close with active operations.

**Limits:** this is not a hostile-host sandbox. A malicious same-user/root process can modify private files or bypass advisory locking. Extended ACLs, mount/bind-mount behavior, network filesystems and host compromise are not certified by mode/UID checks. Review the actual fixture parent's ACL/mount environment before live use. Context is checked at operation boundaries; it cannot forcibly interrupt a blocked kernel filesystem operation. Native process CPU/RAM/storage monitoring is still separate work.

The existing `golang.org/x/sys v0.49.0` pin was promoted from indirect to direct for descriptor-relative Unix operations. No version upgrade, new module pin or package download was performed.

## Record layout and crash behavior

The private metadata directory currently accepts only:

- `manifest.json`: canonical format version 1, experiment identity, PostgreSQL version and canonical root path.
- `operation.lock`: empty private lock file; locked inode must remain the named inode.
- Seven possible `credential-<fixed-role>` files.
- `bootstrap.started`: fsynced identity marker created before the first checkpoint. This distinguishes a missing/deleted first checkpoint from a never-started plan.
- `checkpoint.json`: canonical format/identity/revision/step/phase record.

A reserved `.checkpoint-<random-id>.pending` file is written exclusively, synced and closed, then renamed atomically to `checkpoint.json`, followed by directory sync. A leftover pending artifact is uncertainty evidence and blocks reopening/progress. It is never automatically deleted. A started marker without a checkpoint also blocks progress; a checkpoint without its marker is invalid. Unknown/duplicate JSON keys, trailing data, truncation, alternate serialization, mismatched identities and invalid step/revision combinations are rejected rather than normalized.

There are 16 fixed SQL-plan steps. Each has an intent revision followed by a completion revision, ending at revision 32. Begin requires the next approved step and current expected revision. Pending intent blocks another begin. Complete requires the same pending step and revision. A caller must verify/reconcile actual SQL effects before marking completion; the store cannot prove a database operation happened. No SQL is executed by these methods, and there is no force/skip/reset API.

An interrupted or failed write may retain partial artifacts. Sync/rename uncertainty fails safely; a write-uncertain handle rejects further operations until closed and reviewed. No automatic rollback, deletion or replay is attempted. The checkpoint is the **current progress record**, not an immutable business audit history. Filesystem sync and process-exit recovery were tested, not power-loss durability or storage-controller guarantees.

The future cluster initialization lifecycle needs its own reviewed state handling; these checkpoints cover the fixed SQL plan only. Do not treat a valid Store as permission to initialize or start PostgreSQL.

## Actual verification

Executed on Go **1.27.2 / darwin-amd64**:

```sh
cd backend
GOPROXY=off go test -count=1 -race ./...
GOPROXY=off go vet ./...
GOPROXY=off go build ./...
GOPROXY=off go test -count=1 -coverpkg=./internal/localbootstrap ./tests/localbootstrap
```

All passed. Combined `internal/localbootstrap` coverage is **85.0%**, not 100%; many defensive OS I/O failure branches remain unforced. Tests do not simulate disk-full, failed fsync, real power loss, another UID, extended ACLs or network filesystems.

`tests/localbootstrap/store_test.go` exercises actual temporary filesystem objects: permissions; no adoption/overwrite/reuse; symlinked parents/root/files; hard links; FIFO/directory rejection; missing/conflicting/truncated/duplicate/unknown records; retained partial artifacts; active ownership/lock drift; cancellation; zero/closed handles; competing goroutines; separate-process lock exclusion/release; process exit without Close after a synced intent; persistence on reopen; and atomic replacement observed by a concurrent reader. Subprocess tests receive a minimal explicit environment without inherited provider/DB credentials. They are not PostgreSQL processes.

Linux/amd64 test **cross-compilation passed** with `CGO_ENABLED=0`, `GOPROXY=off`, and a temporary output binary removed afterward. Linux runtime tests were not executed.

Attempted vulnerability rescan:

```sh
GOPROXY=off go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

That invocation **did not run**: Go refused the tool module's deprecation lookup with `GOPROXY=off`.

A subsequent build from the **already cached** `golang.org/x/vuln@v1.9.0` source avoided module lookup/downloads. The first wrapper incorrectly forced the host-local **Go 1.27.1** instead of the project's auto-selected **1.27.2**. That scan exited 3 with six standard-library findings fixed in 1.27.2 (GO-2026-6617/6613/6611/6608/6607/6603); it is not reported as a passing project scan.

The corrected invocation resolved `GOROOT` from `backend/`, verified its `bin/go version` as **go1.27.2 darwin/amd64**, and used that compiler plus its `bin/` first in the scanner's PATH, `GOTOOLCHAIN=local`, `GOPROXY=off` and `GOFLAGS=-mod=readonly`. It built the cached scanner into an owned temporary directory and ran `govulncheck -show verbose ./...` from `backend/`. Temporary scanner binaries were removed afterward; no module/tool installation or finding suppression was used. The vulnerability database was fetched normally over the network.

**Final actual result:** 17 modules plus the **go1.27.2** standard library scanned; **0 reachable-symbol findings, 0 imported-package findings**. Existing module-only **GO-2026-5932** remains in `golang.org/x/crypto@v0.58.0` for unused `openpgp`, with no fixed version. This is not a blanket vulnerability-free or platform-security certification. After the final duplicate-credential reopening guard was added, Linux cross-compilation and the correctly pinned scanner were rerun against final code and passed with the same no-reachable/no-imported findings and one module-only advisory.

## Next bounded work

Resume-safe SCRAM reconstruction and the resource decision policy are now [implemented and unit-verified](native-resource-guard.md). Continue the already authorized code/test scope with actual native resource sampling, complete storage accounting, controlled process/logging supervision, initialization identity checks and SQL execution/reconciliation. Keep those disconnected from startup until separately authorized. Real SCRAM/privilege/isolation/persistence tests and human/FND-005 acceptance remain pending. Web must remain untouched and unstarted.
