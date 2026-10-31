# BID-FND-004 — Local-only PostgreSQL foundation child

## Metadata
- Ticket ID: BID-FND-004
- Epic: E-FND
- Type: DevOps
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-FND-005
- Blockers: Broader platform/security and implemented monitor/recovery review, accepted FND-005 evidence, accountable technical/live-integration review and separate startup authorization remain open. The user approved the local exceptions and bootstrap contract implementation without starting PostgreSQL; disconnected preparation code is verified, not full integration. Isolated tools are 15.19; affected system 15.17 must not be used.
- Milestone: M1

### Current direction — local-fixture tooling retired and archived

The user requested cleanup and application development. All five local-tooling packages, their tests and local-check command are now in [the checksum-recorded archive](../../../archive/local-fixture/README.md), outside the active backend module. Do not continue or restore the collector/bootstrap runner by default. The partial collector remains unverified; excluding it from active checks is not completion. Host installations and existing profiles/databases are preserved, not run or deleted. Earlier source paths and results below are historical. Parent status/gates are unchanged.

The separately authorized portable connection-management increment is recorded with [FND-003](BID-FND-003.md#approved-portable-connection-management-increment) and [adapter evidence](../../../backend/docs/portable-postgres.md). This change does not mark FND-004 or its dependencies Done and does not authorize a PostgreSQL/web startup or production deployment.

## 1. Objective
Provide one explicitly selected local PostgreSQL profile and a synthetic integration harness without introducing business endpoints or changing web code.

## 2. Business Context
Backend development needs reproducible database behavior before claiming transaction, role or tenant-isolation guarantees. A local child avoids misrepresenting the full production/privacy baseline as accepted.

## 3. Scope
Child of BID-FND-003, with local prerequisite subsets from ARC-002/007. The requesting user approved packet 13 and the 2 CPU / 2 GiB RAM / 10 GiB storage budget on 2026-10-17. Implement only reviewed profile/configuration/bootstrap and test artifacts under `backend/`, preserving the existing health-only profile. Use synthetic control and two tenant databases with separate provisioner/migrator/runtime roles; no customer data.

## 4. Out of Scope
Web/frontend changes, production or shared remote databases, public provisioning/authentication/business APIs, business migrations, production retention/restore guarantees, Git initialization, NATS/Redis/mail/storage/scanner/AI services and closure of parent tickets.

## 5. Technical Requirements
Explicit local/test profile; invalid/missing settings and unavailable required dependencies fail closed. Bind exposed ports only to loopback. Commit no secrets. Apply reviewed CPU/memory limits and document disk monitoring: ordinary Compose volumes do not enforce a 10 GiB quota. Stop if the storage budget cannot be respected. Normal startup must preserve data and never migrate, seed or reset implicitly. Bootstrap and any cleanup need explicit target and confirmation; no destructive automated down/reset. Role/password/DSN handling and bootstrap SQL must be reviewed before implementation. The user subsequently selected native PostgreSQL for local development: Docker/Compose/Colima/QEMU are not prerequisites. Native resource limits require their own reviewed treatment; configuration validation is not OS enforcement. Keep provider credentials out of check environments.

## 6. Proposed Implementation Approach
Verify native tool versions/platform, licenses and security; review the isolated native cluster/configuration/role contract and local recovery instructions; establish resource-budget treatment without Docker; implement the bounded backend-local profile; test synthetic roles/connectivity/restart/failure cases; submit evidence for review. Use exact accepted versions/digests, never `latest`. Stop on unavailable tooling, unsafe defaults or unresolved findings rather than substituting products.

## 7. Dependencies
Depends on [FND-005](BID-FND-005.md), the Docker-free local resource validation child. Integrate only after its reviewed acceptance. No hard Done dependency is asserted on full ARC-002/007 or FND-003: their scope exceeds this approved local carve-out. Packet 13 supplies the explicit authorization to prepare this child; its local decision gates remain in Blockers until genuinely reviewed. This child does not satisfy or remove any original parent's dependencies and does not unblock identity/schema/business tickets. The controller must validate Ready before delegation.

## 8. Acceptance Criteria
- Given missing settings, a non-local environment or an unavailable required database, startup/readiness rejects the configuration safely.
- Given a normal stop/start, existing synthetic data survives and no migration/seed is performed.
- Given runtime credentials, DDL and access to unauthorized databases are denied according to the reviewed role contract.
- Given separately selected control and tenant test credentials, connectivity and synthetic sentinel checks demonstrate the intended database separation; this is not application authorization acceptance.
- Given the default health-only profile, no PostgreSQL connection or implicitly healthy business profile is introduced.
- Given local startup, only approved loopback ports are exposed and resource/secret settings match the reviewed contract.

## 9. Testing Requirements
Centralized external-package Go tests under `backend/tests/`; invalid config and missing/unavailable dependency cases; real PostgreSQL role/DDL/connectivity and repeat-start persistence checks; preserved health suite; go test -race, vet and build. Report environment/platform/digest, exact commands, all failed/skipped checks and sanitized evidence. Second-engineer clean-start review remains acceptance work, not something the implementing worker can self-certify.

## 10. Definition of Done
Reviewed exact pins, configuration/role contract and implementation; real required checks passed with evidence; backend-local startup/stop/recovery instructions; accepted human review. Worker returns review_required or blocked, never its own Done. Parent ARC/FND tickets remain open unless independently accepted in full.

## 11. References
[Approved scope and verification](../13_BACKEND_FOUNDATION_UNBLOCKING.md), [parent](BID-FND-003.md), [toolchain](../architecture/BID-ARC-002.md), [local baseline](../architecture/BID-ARC-007.md), [environment implementation](../../implementation/03_DEVELOPMENT_ENVIRONMENT.md), [worker controls](../../../.pi/README.md).

## 12. Risks and Notes
A running Docker CLI is not a running engine. Registry digest verification is not execution, image vulnerability scanning or signature validation. Disk availability on the host does not prove capacity inside the Docker VM. No business data or production secrets may enter this experiment. Preparation approval does not constitute independent engineering/security sign-off.

### Current direction: native local development and generated code

The user requested focusing on code, then explicitly instructed **“for local development, lets not use docker and proceed”**. This supersedes the Colima startup path below. No more Docker/Colima/QEMU setup or installs are authorized by the current task. Existing stopped profiles are left untouched.

Native `psql`, `postgres`, `pg_ctl` and `initdb` are available under `/usr/local/bin`; each reports **15.17 (Homebrew)**. This differs from the previously researched PostgreSQL 18.6 candidate and is an observed installation, not acceptance of its patch/security/feature baseline. No existing server was connected to, started, stopped, upgraded or initialized. Do not adopt an existing personal database or use its credentials.

Implemented the bounded, disconnected environment-loading preparation increment:
- `backend/internal/localprofile/load.go`: explicit caller-supplied environment reader, canonical positive ASCII integer parsing, safe errors, empty config on any failure, and validation through the existing resource contract.
- `backend/tests/localprofile/load_test.go`: successful min/max settings, all missing fields, malformed/oversized/overflowed numbers, zero/negative/over-budget values, invalid scope, canary secrecy, nil reader, ambient-environment independence and existing health startup separation.
- `backend/docs/local-profile-preparation.md`: exact input keys and native-local limits; explicitly not API startup wiring or OS resource enforcement.

Actual verification after formatting: `go test -count=1 -race ./...`, `go vet ./...`, `go build ./...` all passed from `backend/`. `go test -count=1 -coverpkg=./internal/localprofile ./tests/localprofile` passed with **100.0% statement coverage** of the validator and loader. These tests require no Docker/database. No dependencies or web code changed. This increment is technically verified, but the full native database integration ticket remains Blocked; the earlier review of FND-005 did not review this subsequent loader.

### Native configuration preflight code increment

Added `backend/internal/localprofile/postgres.go` (`LoadPostgres`), `backend/internal/localcheck/check.go`, and `backend/cmd/local-check/main.go`. The runnable command validates explicit resource settings and a required loopback IP/port without DNS, libpq defaults, database credentials, a default DB target or any connection. Invalid input produces a safe diagnostic and exit 1; success states explicitly that connectivity/roles/resource enforcement were NOT checked. Existing API startup is untouched.

External tests: `backend/tests/localprofile/postgres_test.go` and `backend/tests/localcheck/check_test.go`. Cover IPv4/IPv6 loopback and port limits; reject URLs, hostnames, wildcard/remote/mapped/scoped addresses, malformed/missing endpoints and ambient libpq fallback; verify empty results on errors, safe output, writer failures and exit status.

Verification: full `go test -count=1 -race ./...`, `go vet ./...` and `go build ./...` passed. Separate `-coverpkg` runs reported **100.0% statement coverage** for each of `internal/localprofile` and `internal/localcheck`. The first negative test exposed that Go's loopback classification accepts mapped IPv4-in-IPv6; an explicit `Is4In6` rejection was added, and the complete suite then passed. A compiled CLI smoke verified valid configuration exits 0, production/unsafe URL input exits 1, and a synthetic secret is not echoed. The temporary smoke binary was removed with its temporary directory.

No PostgreSQL connection/service operation, Docker command, dependency install, web edit or subagent was used for this code increment. Full database integration and human acceptance remain pending; no fixture readiness is claimed.

### Native toolchain inspector increment — backend-only continuation

The user reiterated **“please remember do not start web, continue”**. Added `backend/internal/localpostgres/tools.go` and external `backend/tests/localpostgres/tools_test.go`. Extended `internal/localcheck`, `cmd/local-check` and `tests/localcheck/command_test.go` with explicit `--tools` mode and rejection of unknown arguments. Default configuration-only behavior remains unchanged.

An explicitly trusted absolute `BIDORA_PG_BIN_DIR` and exact `BIDORA_PG_VERSION` are required. The inspector executes only each of `postgres`, `pg_ctl`, `initdb`, `psql` with `--version`, sequentially. It passes a minimal fixed environment, caps combined output at 4096 bytes per tool, preserves cancellation/wrapped diagnostic causes and emits safe errors without tool paths, raw output or secrets. Five-second per-tool and fifteen-second overall deadlines plus 250 ms pipe draining bound the check; this is not a sandbox or a descendant-process-tree kill guarantee.

Initial full race-test runs hit the provisional 3-second aggregate and then 2-second per-tool limits on this macOS host during concurrent fixture execution. Targeted tests passed but showed those limits were too tight for cold/concurrent processes. The final 5/15-second engineering bounds are documented; no negative assertion was removed. Parent-context deadline and cancellation tests continue to pass.

Final actual checks from `backend/`: `go test -count=1 -race ./...`, `go vet ./...`, `go build ./...` passed. `go test -count=1 -coverpkg=./internal/localpostgres ./tests/localpostgres` and the corresponding `internal/localcheck` run each reported **100.0% statement coverage**. Unit executables are synthetic fixtures, not database integration. A separate real `go run ./cmd/local-check --tools` with explicit `/usr/local/bin` and `15.17` matched all four native tools; output states security acceptance/connectivity/roles/resource enforcement were NOT checked. No Windows fixture skip occurred on this macOS run. Compiled CLI smoke checks also passed: configuration and real native-tool modes exit 0, intentional version mismatch exits 1, and an unknown argument exits 2 without echoing its synthetic canary. Temporary binaries were removed by their scoped temporary-directory cleanup. Changed documentation links were checked separately and passed.

No web files/services, database connections or cluster mutations, dependency installations, Docker operations or subagents were involved. The native cluster/version-security/role/resource review gates remain open; ticket status remains Blocked for full integration.

### Upstream native security review — 2026-10-17

Retrieved the official PostgreSQL versioning policy, PostgreSQL 15 security index and individual CVE-2026-6473, CVE-2026-6477 and CVE-2026-16239 advisories. Installed 15.17 is within their affected ranges; upstream lists 15.19 as the current PostgreSQL 15 release. Details, exact source links, publication dates and limits are in [native security evidence](../../../backend/docs/native-postgres-security.md).

Pause live integration on 15.17. An isolated native 15.19 installation is proposed, not authorized or security-approved; existing host binaries/services/clusters must remain untouched. Compatibility, provenance, linked dependencies and total storage/resource budget require review before any installation. The earlier successful tool check remains truthful version-parity evidence, not security acceptance.

Documentation-only increment: no code changes, application-test rerun, dependency installation, cluster operation, database query, Docker action or web startup. The no-web constraint remains in force. Parent/ticket acceptance status is unchanged.

### Authorized isolated PostgreSQL 15.19 installation — 2026-10-17

The user approved the proposed isolated installation. Built official 15.19 source under `~/.local/share/bidora/postgresql/15.19/`, installed into its `install/` subdirectory, preserving system binaries/links and all clusters. [Detailed installation evidence](../../../backend/docs/native-postgres-installation.md) records archive URL/hash/license, exact configure/build commands, failed initial configure flag and successful retry, host/toolchain, dynamic-library inventory, resource observations, artifacts and remaining gates.

Configure/build/install passed; explicit Go `local-check --tools` matched all four new 15.19 tools. System postgres/psql still report 15.17. Peak sampled build process-group RSS was 182,208 KiB with two make workers; post-install workspace was 264,264 KiB, about 600 MiB total when conservatively including earlier shared Colima cache/profile artifacts. Sampled guards are not hard aggregate OS quotas. The installed minimal build has no TLS/readline/optional compression/authentication dependencies; TLS use/exception needs explicit review before runtime.

No cluster was initialized, started, connected to or upgraded; no web action, Docker operation or subagent was used. Upstream `make check` was not run because it would start a test cluster. Application source and dependencies were unchanged; prior Go unit tests were not rerun, while the real tool-check command and binary inventory were executed. Installation is technically verified, not full FND-004 acceptance.

### Next native fixture plan — proposed for review

Prepared [native fixture implementation plan](../../../backend/docs/native-postgres-fixture-plan.md): an explicitly owned private target, fixed loopback endpoint, SCRAM/private per-role credentials, synthetic role/data scope, separate inspect/initialize/start/bootstrap/verify/stop operations, non-destructive conflict/retry behavior and required real integration cases.

Two explicit decisions block the next live increment: a fixture-only no-TLS exception for the installed minimal build, and monitored native resource budgets rather than hard OS quotas. The plan records alternatives, limitations and pending target/role/SQL/FND-005/reviewer gates; none is self-approved. Detailed bootstrap SQL is not yet prepared. Documentation-only work: no service/web start, cluster mutation, installation, production code change or application-test rerun.

### Explicit local exception approvals and bootstrap review draft

On 2026-10-17 the requesting user explicitly replied **“approvve both”** to the two enumerated choices: fixture-only loopback no-TLS with SCRAM, and monitored native resource budgets with disclosed possible overshoot. Recorded these scoped approvals in the fixture plan, canonical decision register and AGENTS.md; no other gate is marked approved.

Prepared [bootstrap review draft](../../../backend/docs/native-postgres-bootstrap-review.md): fixed role/database mapping, NOLOGIN-first credentials, privileged provisioner limitations, PostgreSQL-15-compatible role/database/sentinel SQL shapes, secret-safe process/server diagnostics, nontransactional CREATE DATABASE checkpoint handling and concrete negative tests. It is a non-executable document; SQL placeholders are not runnable migrations. Credential-verifier construction, logging capture, roles, checkpoint/monitor semantics and reviewer/FND-005 acceptance remain pending.

No application code/test change, secret generation, cluster start/query or web action occurred. Documentation links/structure are checked separately; application tests are not rerun for this documentation-only change.

### Explicit bootstrap implementation approval and first code slice

The requesting user explicitly replied **“I approve”** to implementing the bootstrap contract (roles, credentials, SQL and safeguards) **without starting PostgreSQL**. Recorded this as implementation authorization, not ticket acceptance or permission to run a cluster. No further repeat approval is required to continue this bounded code/test scope.

Implemented `backend/internal/localbootstrap/{plan,credential,diagnostics}.go` with external tests under `backend/tests/localbootstrap/`: freshly allocated fixed-role SQL plans, explicit nontransactional database-create and credential-activation steps, statement/lock timeout metadata, generated 32-byte-entropy ASCII passwords and PostgreSQL-format SCRAM-SHA-256 verifiers, restricted/redacted activation SQL and concurrent discard-only diagnostics. See [scope and evidence](../../../backend/docs/local-bootstrap-preparation.md).

Actual Go 1.27.2 checks: `go test -count=1 -race ./...`, `go vet ./...`, `go build ./...` and `go test -count=1 -coverpkg=./internal/localbootstrap ./tests/localbootstrap` all passed. New package coverage **95.3%**; no coverage assertion was weakened or entropy source overridden. Unit checks compare SCRAM derivation independently and exercise secret-safe formatting/output, role/SQL structure, scope rejection and concurrency. Existing health suites passed.

Not implemented/executed: private credential files, ownership/lock/checkpoint/monitor/runner/executor, PostgreSQL SQL parsing/authentication/privilege/persistence integration, cluster creation/start or web work. No dependency changes or business schema. FND-004 stays Blocked for full integration; this code slice is VERIFIED_TECHNICALLY only.

### Private metadata/credential store and checkpoint code increment

Continued the existing approved implementation scope without a repeat approval request. Added `backend/internal/localbootstrap/store_unix.go` and `backend/tests/localbootstrap/store_test.go`: explicit new/existing owned metadata roots, descriptor-relative no-symlink opens, UID/mode/link checks, distinct immutable private credential files, exclusive nonblocking advisory locks, canonical identity records and fsynced atomic fixed-step intent/completion checkpoints. Pending/corrupt/unknown state is retained and rejected, not silently repaired or replayed. The initial started marker detects a lost first checkpoint. This metadata store is not proof of ownership of a PostgreSQL data directory.

Promoted already pinned `golang.org/x/sys v0.49.0` to direct in `backend/go.mod`, without upgrade/download. [Store implementation/evidence](../../../backend/docs/local-bootstrap-store.md) records APIs, security limits, artifacts and remaining work. No production fixture root was created; storage tests used owned temporary directories and synthetic credentials.

Actual checks: `GOPROXY=off go test -count=1 -race ./...`, `go vet ./...`, `go build ./...` and localbootstrap coverage run all passed; package coverage **85.0%**. Real filesystem/process tests include symlinks/hard links/FIFO/modes, wrong/missing/corrupt ownership state, duplicate/truncated JSON, credential overwrite/reuse, stale/out-of-order checkpoints, cross-process locking and exit recovery, and concurrent atomic checkpoint observation. Linux/amd64 test cross-compilation passed; Linux runtime tests and power-loss/disk-full/fsync-failure/ACL/other-UID scenarios were not run.

The initial pinned `go run` govulncheck invocation with `GOPROXY=off` failed on module deprecation lookup. Building the already cached v1.9.0 scanner avoided downloads, but the first wrapper incorrectly selected host Go 1.27.1 and reported six standard-library findings fixed in 1.27.2. Corrected the wrapper to the verified project Go 1.27.2 binary/PATH and reran `govulncheck -show verbose ./...`: **0 reachable-symbol and 0 imported-package findings** across 17 modules plus go1.27.2. The known unused-openpgp module-only GO-2026-5932 remains. No finding suppression or tool/module installation was used; full evidence/limitations are in the store notes. Broader platform/security acceptance remains open.

No SQL, database server, native real fixture initialization, web process/file, Docker operation or subagent was involved. Native monitoring, process supervision, cluster identity/lifecycle and SQL execution/reconciliation remain next coding work under the existing no-start authorization; live execution and final acceptance remain gated.

### Resume-safe credentials and resource decision guard

Continued the existing implementation-only approval. Added `RestoreCredential` in `internal/localbootstrap/credential.go` to reconstruct a fresh-salt SCRAM verifier without rotating the existing private password or advancing checkpoints. Added `internal/localbudget/guard.go` with synchronized, sticky stop decisions for 1.5 GiB RSS / 8 GiB storage thresholds, >=180% CPU sustained for ten seconds, stale/missing/incomplete observations, PID/birth identity mismatch and clock/CPU regressions. The intended collection cadence is one second with a two-second maximum age/gap. The guard samples/stops nothing itself.

Expanded credential/store tests and added external `tests/localbudget/guard_test.go`. [Detailed evidence and limits](../../../backend/docs/native-resource-guard.md) distinguishes policy-unit verification from actual resource monitoring. Temporary-store resume tests confirm unchanged passwords/files and retained pending checkpoints; independent SCRAM calculations continue to pass.

Actual Go 1.27.2 checks: full `GOPROXY=off go test -count=1 -race ./...`, vet and build passed; coverage **85.2% localbootstrap**, **100.0% localbudget**. Both suites cross-compiled for Linux/amd64, not executed there. Cached govulncheck v1.9.0 with verified Go 1.27.2 scanned final code: **0 reachable/imported findings**; existing unused-openpgp module advisory GO-2026-5932 remains. No dependencies or tool versions changed.

No real fixture/password rotation, PostgreSQL/web startup, process sampling/signalling, business SQL, Docker action or subagent was used. Native collector/storage accounting and controlled supervision/initialization/execution/reconciliation remain next coding work. Full integration and human acceptance remain pending; ticket is not Done.

### Historical isolated Colima setup — superseded by native-local direction

The user approved creating a separate Bidora Colima profile. Created `~/.colima/bidora/colima.yaml`: 2 CPUs, 2 GiB RAM, 4 GiB root + 4 GiB data disk; 2 GiB of the approved total storage budget reserved for cache/metadata overhead. Disabled home mounts, SSH agent forwarding, SSH config modification, Kubernetes and automatic Docker-context activation. No dependency installation was authorized or performed.

Executed `colima start bidora --cpus 2 --memory 2 --disk 4 --root-disk 4 --mount none --activate=false --ssh-config=false --ssh-agent=false --binfmt=false --template=false`. Colima obtained its VM image, created local profile/Lima metadata and a sparse data disk, then failed with `qemu-system-x86_64: executable file not found in $PATH`. The installed Android SDK supplies `qemu-img` only; it is not a replacement for the missing VM executable. Homebrew lists Colima 0.10.3 and Lima 2.1.4, but no QEMU installation.

Post-failure checks: `bidora` and original `default` remain Stopped. Default remains 4 CPUs / 6 GiB / 20 GiB, and active Docker context remains `desktop-linux`. New profile and VM metadata occupy 12 KiB and 16 KiB; the new data disk occupies 196 KiB and `qemu-img info` confirms 4 GiB virtual capacity. Shared Colima cache measured 349,904 KiB total (not asserted to be entirely new). No root disk or Docker/PostgreSQL service was demonstrated running. Guest disk settings plus measured overhead are a monitored budget, not a hard host-filesystem quota.

Next prerequisite: select/approve/install a QEMU build compatible with macOS 12.7.6 x86_64, then retry this same bounded profile and verify effective resources and storage. Do not raise disk sizes, start the old profile, silently switch VM implementations or initialize Git. Broader role/security/storage acceptance remains open.

### Manual prerequisite work — 2026-10-17

The user requested proceeding to this next ticket without further subagent setup. Prepared [local PostgreSQL configuration/role contract](../../../backend/docs/local-postgres-contract.md): separate synthetic control/two-tenant targets, owner/migrator/runtime privileges, safe credentials and bootstrap lifecycle, storage-budget gap and concrete integration cases. This is a proposal for review, not executable SQL or an approved production policy.

Actual read-only verification:
- Docker server query at 21:25 UTC still failed: configured daemon unreachable. Compose reports 5.3.1.
- Version-tagged upstream license files for pgx v5.11.0, Goose v3.29.0, sqlc v1.31.1 and PostgreSQL 18.6 returned HTTP 200. Direct license terms and byte checksums are in the contract.
- Exact Go module/version OSV queries returned no advisory IDs for the three proposed tools/libraries. Transitive resolution, image scanning, legal disposition and runtime compatibility are not thereby accepted.
- FND-005's completed source-only reviewer report was inspected and linked in that ticket; no automatic Done status was inferred.

Further runtime diagnosis found Colima 0.10.3 and Lima installed. The active Docker context is `desktop-linux`, pointing at Docker Desktop's unreachable socket; no Docker Desktop application was found under `/Applications` or `~/Applications`. `colima list` reports an existing stopped default profile configured with 4 CPUs, 6 GiB memory and 20 GiB data disk. Its allocation exceeds the approved Bidora budget, so it was not started, resized, deleted or adopted. Colima's CLI exposes a separate root-disk allocation (20 GiB default), so `--disk 10` alone would not establish a 10 GiB total storage budget. A separate reviewed Bidora profile and explicit root/data/cache budget treatment are required before using Colima instead. No global Docker context was changed.

No new dependencies, containers, database connections, credentials, migrations, application code or web changes were introduced. No application tests were rerun for this documentation-only increment. Docker/role/integration tests remain unexecuted, and the 10 GiB storage budget needs an explicit enforcement or reviewed monitoring mechanism. This ticket remains Blocked; preparation advances its evidence without manufacturing runtime acceptance.
