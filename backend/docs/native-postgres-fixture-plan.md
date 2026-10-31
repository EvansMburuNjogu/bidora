# Native fixture implementation plan — BID-FND-004

> Retired plan. Source/tests are now in [the archive](../../archive/local-fixture/README.md); do not continue this tooling plan by default. Historical approvals below are preserved, not extended.

**PROPOSED — review required, not authorization to initialize/start a cluster.**

This is the next bounded backend increment after the [isolated 15.19 tool installation](native-postgres-installation.md). Existing health behavior, system PostgreSQL, stopped Colima profiles and all web files/services remain untouched. Only synthetic fixture data is permitted.

## 1. Decisions and remaining review

On **2026-10-17**, the requesting user explicitly replied **“approvve both”** to the two enumerated local-only choices. Items 1 and 2 below are therefore **Approved** for this bounded fixture only. No personal name/title is inferred. Other gates remain pending.

1. **Local transport exception:** permit PostgreSQL connections without TLS only to the separately owned synthetic fixture at `127.0.0.1:15432`, with SCRAM-SHA-256 authentication. No remote/non-loopback endpoint, real customer data, production credentials or application identity is permitted. The installed build cannot provide TLS. This exception would not apply to production or any other database. Alternative: prepare a TLS-enabled native build and review its TLS dependencies/certificates first.
2. **Native resource treatment:** retain the 2 CPU / 2 GiB RAM / 10 GiB budgets but accept conservative process/storage monitoring with automatic stop for this fixture, rather than claiming hard OS quotas. Sampling can miss peaks and cannot guarantee zero overshoot. Alternative: keep live execution blocked until suitable hard native limits are designed. Docker is not proposed as a fallback.
3. **Scope and technical review:** review the target/credential/role/lifecycle contract below and the existing [role matrix](local-postgres-contract.md#3-roleownership-matrix). FND-005 acceptance and the accountable reviewer remain outstanding; implementation cannot self-approve them. The [bootstrap SQL and credential/logging contract](native-postgres-bootstrap-review.md) was subsequently explicitly approved for implementation without starting PostgreSQL. Its [initial disconnected code slice](local-bootstrap-preparation.md) is technically verified; this is not live execution or final acceptance.

## 2. Proposed isolated target

| Setting | Proposed rule |
|---|---|
| Binaries | Explicit `~/.local/share/bidora/postgresql/15.19/install/bin`; require exact version parity, no PATH fallback |
| Fixture root | Separate `~/.local/share/bidora/fixtures/fnd-004/`; not inside the tool installation or any existing data directory |
| Ownership | New private directory mode 0700, private manifest with random experiment identity and canonical target; refuse nonempty/unowned targets, symlinks and identity mismatches |
| Listener | Exactly `127.0.0.1:15432`; reject an occupied port, never adopt the process on it |
| Unix sockets | Private fixture-local socket directory, never shared `/tmp`; restricted directory/socket permissions |
| Authentication | Explicit SCRAM-SHA-256 for both TCP and local sockets; no `trust` rule |
| Secrets | Random per-role fixture credentials; private regular files mode 0600 outside the repository, no symlink following, no inline CLI passwords/DSNs or inherited libpq defaults |
| Database names | Exactly `bidora_local_control`, `bidora_local_tenant_a`, `bidora_local_tenant_b`; no default/fallback or caller-supplied database names |
| API | No connection from `cmd/api`, no database readiness claim, no new HTTP/business endpoint |

A hostname, port or path alone is never ownership evidence. Bootstrap/start/stop must validate the explicit experiment identity and owned target. Ordinary read-only checks must not create a directory, generate credentials, initialize, migrate, seed or start anything.

## 3. Synthetic role and data boundary

Follow the existing proposed role matrix: separate privileged bootstrap/provisioning operations, distinct NOLOGIN owners and explicit migrators, and one restricted runtime role per database. Runtime roles are not superusers/owners and cannot create roles/databases/schemas, bypass RLS, replicate, inherit/assume owner or migrator roles, or connect to another target.

Revoke unintended PUBLIC CONNECT/TEMP and schema CREATE. Address default access to `postgres` and `template1` on this exclusively owned cluster. Never change these permissions on a personal/shared cluster. Provisioning privileges are powerful cluster-level authority, not a tenant-isolation boundary; keep them out of API/runtime checks.

The only fixture table would hold synthetic sentinel keys/values in a dedicated fixture schema. No business schema, customer data, identity records, audit-policy invention or executable business migrations are authorized. Explicitly grant only the sentinel DML needed by the harness. Successful SQL or sentinel separation must not be presented as application authorization evidence.

## 4. Proposed lifecycle and failure behavior

- **Inspect:** read-only configuration/tool/ownership checks; no automatic setup.
- **Initialize:** explicit target and confirmation, newly owned empty directory only; reject unrelated or partial/conflicting state. No automatic destructive repair.
- **Start:** only a validated owned initialized fixture; no implicit migration, role creation or seeding. Preserve data on normal restart.
- **Bootstrap:** separately confirmed, reviewed role/database/sentinel setup; create/reconcile only fixed targets and fail on unexpected ownership/grant drift. Do not DROP conflicting objects to make retry pass.
- **Verify:** connect using each selected identity, check positive behavior and negative privileges/database access, and verify persisted sentinels across explicit stop/start.
- **Stop:** signal only the validated owned PostgreSQL instance, using process identity and owned-data checks; do not match a process by port/name alone. Bound shutdown and report failures. No automatic directory deletion/reset.

Errors/logs must exclude credentials, connection strings, SQL containing secrets and raw subprocess output. Bootstrap SQL and PostgreSQL logging/error settings need joint review so safe application logging does not leave server-side credential leaks.

## 5. Approved monitored-budget approach — detailed mechanism pending

Use low-concurrency synthetic operations, disabled query-parallel workers and conservative connection/buffer/work-memory settings. These reduce demand but do not impose hard CPU/RAM quotas.

Proposed guard thresholds: stop/pause fixture work before 1.5 GiB sampled process-tree RSS, 8 GiB total experiment storage, or sustained 180% aggregate CPU (two cores equal 200%). Include child processes and include retained downloads/build/install artifacts, fixture data/WAL/logs and earlier experiment artifacts in storage accounting. Treat failed/stale measurement as a stop condition, not success. Bound/rotate logs, deadlines and retries. Never silently increase budgets.

The disconnected [resource decision guard](native-resource-guard.md) now tests a one-second intended cadence, two-second maximum age/gap, and ten-second sustained CPU threshold, with sticky stop decisions and PID/birth-identity matching. Actual OS collection, complete descendant/storage accounting, timer scheduling and controlled shutdown are still unimplemented; validate those before live execution. Thresholds leave headroom but cannot guarantee no transient overshoot. Do not claim the pure resource validator or PostgreSQL `work_mem` enforces a host quota.

## 6. Acceptance and recovery evidence

Required real-PostgreSQL cases: unavailable endpoint; invalid/missing credential; allowed sentinel operations; cross-database/admin-database denial; DDL/role escalation/temp creation denial; owner/migrator/runtime separation; conflicting-target refusal; partial-bootstrap reconciliation; explicit restart persistence; bounded timeout/cancellation; loopback exposure and safe logs; resource-monitor stop behavior.

Go tests stay in external packages under `backend/tests/`. Unit fakes are not database integration evidence. Never silently skip required PostgreSQL integration checks and report success. Preserve the health regression suites; do not start or test web as part of this increment.

No deletion is needed to recover from this planning step. Future setup failures must leave owned evidence for inspection, without altering other clusters. Technical verification does not complete user acceptance or parent tickets.

## Review record

- No-TLS local exception: **APPROVED**, requesting user, 2026-10-17, explicit “approvve both” response to the two choices.
- Monitored native resource treatment: **APPROVED**, same evidence; caps unchanged and possible transient overshoot disclosed. Sampling/stop implementation still requires review and tests.
- Target/credential/role/lifecycle and bootstrap SQL/logging contract: **APPROVED FOR IMPLEMENTATION**, requesting user, explicit “I approve” in response to the proposal to implement the draft without starting PostgreSQL. See [contract](native-postgres-bootstrap-review.md).
- Resulting code/integration/monitor/recovery acceptance: **PENDING**; initial disconnected unit-tested slice is not full bootstrap completion.
- FND-005 user acceptance / accountable reviewer: **PENDING**.

Approval of the first two decisions alone does not waive the remaining reviews. Record actual user/reviewer decisions and evidence; do not infer approval from a generic continuation request.
