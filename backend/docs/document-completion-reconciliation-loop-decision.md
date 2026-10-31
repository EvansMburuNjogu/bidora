# Document AI completion reconciliation service-loop policy

Status: **OPTION 1 APPROVED — SOURCE IMPLEMENTED_NOT_VERIFIED — WORKER UNMOUNTED**

Date: 2026-10-20

Tickets: BID-EVT-003, BID-AI-003, BID-TEN-004E

## Approved decision

On 2026-10-20 the requesting user proceeded with recommended Option 1 for how a cell worker safely reconciles durable tenant `SETTLE_PENDING` and `RELEASE_PENDING` requests with global Commercial accounting, then records the matching tenant acknowledgement.

The existing `DocumentCompletionCoordinator` demonstrates the replay-safe cross-plane sequence, but its current `FindPendingDocumentCompletion` read has no lease, attempt fence, retry schedule, hold state or multi-tenant service loop. It must remain unmounted until those boundaries are approved and implemented.

This packet does not activate measurement, OpenAI, provider processing, tenant movement, hold resolution, migrations or services.

## Invariants shared by every acceptable option

- The immutable tenant completion request is the reconciliation source. It is never rewritten, deleted or replaced to obtain a different accounting result.
- `SETTLE` always uses the exact reservation, execution ID and deterministic measured credit amount already committed by tenant processing.
- `RELEASE` always uses the exact reservation and execution ID and zero actual credits.
- A provider-ambiguous run that entered `SETTLE_PENDING` is never converted to release by this worker.
- Commercial completion is idempotent only for the exact reservation, operation, execution ID and actual credits. A conflicting terminal result fails closed.
- A crash or timeout after Commercial commit but before tenant acknowledgement replays the same immutable request; it never creates a second customer charge or releases settled credits.
- No SQL transaction spans a tenant database and control Commercial accounting.
- Tenant ID, assignment generation, cell and database location remain routing/applicability facts, never authority.
- A stale assignment is never forwarded automatically to a replacement cell.
- Runtime startup never runs migrations.
- Logs and metrics contain no document content, prompt, provider response, object key, database locator, credential, personal email or tenant/event ID label.

## Option 1 — fenced tenant reconciliation with cell-local round-robin scheduling (recommended)

### Durable tenant claim state

Add tenant-common migration `00008_ai_entitlement_completion_reconciliation.sql` without changing immutable request rows. It introduces:

1. one mutable reconciliation row per completion request containing state, attempt count, next-attempt time and the current UUID lease/expiry; and
2. immutable attempt/outcome evidence containing request ID, lease ID, attempt number, operation, bounded result code, timestamps and optional next-attempt time.

Allowed reconciliation states are:

- `PENDING` — eligible immediately;
- `LEASED` — one worker owns an unexpired attempt;
- `RETRY_WAIT` — transient or ambiguous failure, eligible only at `next_attempt_at`;
- `HELD` — no automatic claim or mutation;
- `ACKNOWLEDGED` — matching tenant completion acknowledgement committed.

The immutable completion acknowledgement remains the authoritative local terminal evidence. The mutable reconciliation row is worker coordination state only.

Migration must reject any pre-existing pending request that cannot be initialized consistently. Rollback must reject retained attempt, hold or acknowledged reconciliation evidence.

### Claim and fencing

- Claim lease: **2 minutes**, UUID-fenced and non-renewable.
- Per-attempt deadline: **90 seconds**.
- Cancellation unwind: **10 seconds**.
- Maximum: **nine total claims** per immutable request.
- Claim uses `FOR UPDATE SKIP LOCKED` inside one tenant database.
- Commercial invocation happens after the claim transaction closes.
- Retry, hold and local acknowledgement require the exact unexpired lease ID and attempt number.
- Context cancellation does not deliberately consume a retry; expiry recovery records an immutable `LEASE_EXPIRED` outcome.

Before any tenant database access, the worker re-resolves its catalogue entry and requires the exact current assignment; a stale catalogue entry is skipped without opening the old database. After a request is claimed, the worker rechecks that its snapshotted generation still equals the current assignment before Commercial invocation. After Commercial success/replay and before local acknowledgement, it rechecks assignment again. If generation changed after claim or Commercial commit, it does not write the old cell or release the lease early; the lease expires and the exact request remains movement-reconciliation work. A local `STALE_ASSIGNMENT` hold is permitted only when the stale request is present in the currently assigned database, such as after an approved copy/restore; stale placement never authorizes access to the former cell.

### Failure classification

Use typed outcomes rather than diagnostic text:

- exact Commercial success or exact terminal replay: acknowledge tenant state;
- unknown timeout/connection loss after Commercial dispatch: `AMBIGUOUS_INTERNAL_EFFECT`, retry the same request because Commercial execution is exactly fenced;
- dependency unavailable before dispatch: `TRANSIENT_UNAVAILABLE`, retry;
- local acknowledgement failure after Commercial success: `LOCAL_ACK_UNAVAILABLE`, retry the same request;
- Commercial terminal mismatch, changed execution ID, operation or credit amount: immediate `ACCOUNTING_CONFLICT` hold;
- malformed or inconsistent immutable tenant request/checkpoint: immediate `INVALID_REQUEST` hold;
- request generation older than the current assignment while that request is in the current assigned database: immediate `STALE_ASSIGNMENT` hold;
- event generation ahead of registry: immediate `GENERATION_INTEGRITY_CONFLICT` hold;
- transient/ambiguous failure on claim nine: `RECONCILIATION_EXHAUSTED` hold.

Unknown errors after a possible internal Commercial call are retried, not treated as confirmed failure. This differs from OpenAI accounting because Commercial completion is Bidora-owned, replay-safe and execution-fenced.

### Retry schedule

After failed claims 1–8, schedule the next claim after respectively:

`30 seconds, 2 minutes, 10 minutes, 30 minutes, 2 hours, 6 hours, 12 hours, 24 hours`.

Failure of claim nine creates `RECONCILIATION_EXHAUSTED`. Attempts never reset automatically. Held completion requests continue to block archive completion and remain visible operational debt.

### Cell-local scheduling

Run one completion-reconciliation loop per cell worker process. The process receives an explicit finite server-owned assignment catalogue; it does not discover tenant database locators from requests or caller input.

The loop maintains an in-memory stable round-robin cursor:

1. select one registered assignment;
2. revalidate its exact current generation through control placement and skip stale catalogue entries without old-cell access;
3. claim at most one due completion request in that currently assigned tenant database;
4. process at most that one request;
5. advance to the next registered assignment regardless of result.

A busy tenant therefore cannot consume consecutive turns while another registered tenant is eligible. One loop iteration opens at most one tenant transaction and one Commercial operation. The loop never scans every tenant in one SQL transaction and never creates one goroutine/pool per tenant.

After a full catalogue pass with no claimed request, wait **1 second**. A tenant endpoint failure receives in-memory process backoff of **1, 2, 4, 8, 16, then 30 seconds**, capped at 30 seconds, while other catalogue entries continue. Successful access to that tenant resets only its process backoff. Durable request retry timestamps remain authoritative.

The catalogue has no caller-visible selector and grants no authority. Production cell size and intentional worker concurrency remain deployment/capacity decisions; no process may mount until its explicit catalogue and connection budget are approved.

### Process boundary and shutdown

Use a dedicated separately mountable cell-worker boundary, not `pilot-api` and not the global acknowledgement-dispatch loop. It may share reviewed connection infrastructure but not mutable loop state.

On shutdown:

- stop selecting new tenants/requests;
- cancel the active attempt;
- wait at most ten seconds for local unwind;
- do not release or rewrite the lease early;
- let the two-minute lease expire if cleanup cannot finish.

A panic is not swallowed. Deployment supervision may restart the process and lease recovery preserves the request.

### Health and bounded signals

The source boundary requires startup-probe and observer ports with no no-op defaults.

Startup readiness requires:

- control placement compatibility for the acting binary;
- tenant-common compatibility at least 8 for every explicitly registered assignment;
- Commercial accounting adapter availability;
- a non-empty, internally unique assignment catalogue.

Required bounded signals are loop state, catalogue pass result, tenant endpoint availability, completion claim result, settlement/release disposition, lease expiry, retry reason, hold category, oldest due age and pending/held counts. Tenant, request, reservation, execution and database IDs are trace fields only and never metric labels.

A held request degrades accounting health but does not by itself fabricate process unavailability. API readiness remains independent.

## Option 2 — reuse the current unfenced coordinator in a polling loop

Sequentially call `FindPendingDocumentCompletion`, Commercial and local acknowledgement without a durable claim.

This is rejected. Concurrent workers can execute the same request simultaneously, there is no bounded retry/hold evidence, and a failing first row can starve later requests indefinitely.

## Option 3 — move completion accounting into the provider transaction

Keep the tenant SQL transaction open while settling/releasing Commercial credits.

This is rejected. It creates an invalid cross-database transaction assumption, extends locks across an external dependency and still cannot provide atomic commit across tenant and control databases.

## Option 4 — emit completion requests only to a broker

Treat NATS acknowledgement as completion truth and remove tenant polling.

This is rejected for the current slice. Broker delivery is not business truth, NATS runtime is not authorized, and durable tenant request/acknowledgement evidence must remain sufficient for restart recovery.

## Implemented source boundary

1. Tenant-common migration `00008_ai_entitlement_completion_reconciliation.sql` adds mutable lease coordination plus immutable attempt/outcome evidence and advances source compatibility to 8.
2. Pending reads are replaced in the active coordinator path by claim/recover/retry/hold/acknowledge ports fenced by UUID lease and attempt.
3. Typed Commercial completion effects separate confirmed exact replay, retryable/ambiguous outcomes and accounting conflicts.
4. Exact current placement is rechecked by the cell runner before tenant access and by the coordinator before Commercial access and tenant acknowledgement.
5. `document_completion_runner.go` provides a finite-catalogue round-robin runner with one request per tenant turn, per-tenant in-memory process backoff and fixed timing.
6. `completionworker/worker.go` provides a separately mountable startup-probed boundary; mandatory bounded observer ports have no no-op fallback.
7. No command/process is mounted or started, and `pilot-api` is unchanged.
8. During the later verification phase, fake-timer and PostgreSQL tests remain required for concurrent claims, expiry, ambiguous Commercial commit, terminal replay, conflicting execution/credits, stale movement, round-robin fairness and cancellation.

## Explicit exclusions

- worker mounting, service startup, deployment or migration execution;
- OpenAI/provider invocation and measurement processing;
- NATS publication/subscription;
- tenant movement, stale-request forwarding or hold resolution/replay;
- operator accounting adjustments, refunds or payment reconciliation;
- production cell size, replica count, alert thresholds or paging provider;
- test implementation/execution in the current functionality-first phase;
- frontend work.

## Approved choice

Option 1 was approved, including:

- tenant-common migration 8 with separate mutable reconciliation state and immutable attempt/outcome evidence;
- two-minute UUID-fenced non-renewable leases, 90-second attempt deadline and ten-second unwind;
- nine total claims with the exact existing eight-delay schedule;
- immediate holds for invalid requests, accounting conflicts, stale assignment and generation integrity conflicts;
- retry of exact requests after ambiguous internal Commercial effects or local acknowledgement failure;
- one-request-per-tenant round-robin turns from an explicit server-owned catalogue;
- one-second idle full-pass polling and per-tenant `1, 2, 4, 8, 16, 30` second process backoff;
- mandatory startup probes/observations and no automatic forwarding, hold release, attempt reset, runtime migration or process mounting.
