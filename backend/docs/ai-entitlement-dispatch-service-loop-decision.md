# AI entitlement acknowledgement dispatcher service-loop policy

> Historical decision: the unmounted worker boundary was removed in BE-SIM-01. This is not an active service or startup instruction.

Status: **OPTION 1 APPROVED — SOURCE IMPLEMENTED_NOT_VERIFIED — LOOP UNMOUNTED**

Date: 2026-10-20

Tickets: BID-EVT-003, BID-EVT-001, BID-AI-003, BID-TEN-004E

## Approved decision

On 2026-10-20 the requesting user proceeded with recommended Option 1 for the bounded process-loop behavior that repeatedly invokes the implemented AI-entitlement acknowledgement dispatcher without creating a tight retry loop, consuming an expired lease, starving another tenant or coupling API readiness to asynchronous delivery.

This decision concerns only control-to-tenant `RESERVED`/`DENIED` acknowledgement dispatch. It does not cover measurement workers, OpenAI execution, settlement/release reconciliation, NATS, tenant movement, hold resolution or operator replay.

## Invariants shared by every acceptable option

- The loop receives only explicitly composed control and tenant adapters; it discovers no DSN, credential, cell or tenant from an event.
- Each dispatch attempt remains one control claim followed by at most one server-selected tenant transaction and one fenced control disposition.
- No SQL transaction spans control and tenant databases.
- Cancellation never publishes, defers or holds with an expired lease. An interrupted attempt becomes reclaimable through lease expiry and tenant replay remains idempotent.
- The loop never resets the approved nine-claim event budget or changes its retry schedule.
- `POISON_EVENT`, `STALE_ASSIGNMENT`, `INTEGRITY_CONFLICT` and `DELIVERY_EXHAUSTED` are durable event outcomes, not process crashes.
- The worker process emits no tenant-private content, payload JSON, database locator, credential, object key or provider diagnostic.
- Metrics use bounded operation/result labels; tenant/event IDs may appear only as redacted trace fields, never metric labels.
- Runtime startup never runs migrations.

## Option 1 — dedicated single-loop pilot process with bounded cancellation (recommended)

### Process boundary

Implement a reusable application runner and a separately mountable worker composition boundary. Do not add it to `pilot-api` or its HTTP readiness lifecycle. The pilot deployment runs **one dispatcher goroutine in one worker replica**. PostgreSQL fencing remains correct if more processes are accidentally present, but increasing intentional concurrency requires a later measured capacity decision.

The source may provide a command/composition constructor, but it remains unmounted and cannot open services from ambient configuration in this increment.

### Attempt and lease limits

- Claim lease: **2 minutes**, non-renewable.
- Per-attempt context deadline: **90 seconds**.
- Shutdown unwind allowance after cancellation: **10 seconds**.
- No heartbeat, lease extension or early lease release.

The 30-second difference between attempt deadline and lease expiry gives the application time to unwind while ensuring an overrun cannot make a stale control transition. PostgreSQL remains the final lease fence.

On shutdown, the loop stops claiming, cancels the active attempt and waits up to ten seconds for local goroutines to return. Cancellation does not consume an event retry transition. If local cleanup does not finish, process termination is safe: the event remains unavailable only until its two-minute lease expires.

### Polling and process-level backoff

Interpret dispatcher results as follows:

- `processed=false, err=nil`: the control query succeeded but no event was due; wait **1 second**.
- `processed=true`: an event was published, durably deferred, durably held, or left for lease recovery after an ambiguous/local failure; continue serving other due work subject to the burst yield below.
- `processed=false, err!=nil`: no trustworthy claim was obtained; use process-level backoff of **1, 2, 4, 8, 16, then 30 seconds**, capped at 30 seconds.
- Any successful claim query, including a no-work result, resets process-level backoff.

After **25 processed claims**, wait **100 milliseconds** before claiming again. This bounds a continuously non-empty loop without delaying ordinary low-volume delivery. Event-level retry timestamps remain authoritative and are not replaced by process-level backoff.

All waits are cancellation-aware and use timers rather than uninterruptible sleeps.

### Pilot fairness

The claim operation receives the previously served tenant ID as a non-authoritative scheduling hint. Among due events, SQL first prefers a tenant different from the immediately previous tenant, then retains stable `next_attempt_at, id` order. If no other tenant is due, the same tenant may proceed.

The runner retains only the last tenant ID in process memory and clears it on restart. It is never logged, exposed or persisted as authority. Combined with one pilot loop/replica, this prevents an unbounded consecutive run for one tenant whenever another tenant has due work. It does not claim weighted global fairness or authorize future multi-replica scaling.

Implementing this requires the dispatcher result to return a scheduling outcome containing `processed` and the claimed tenant ID without exposing that value outside the worker boundary.

### Error and lifecycle behavior

- A durable hold is an expected processed result and does not terminate the loop.
- A durably deferred transient failure is an expected processed result and does not terminate the loop.
- Context cancellation returns cleanly after bounded unwind.
- Invalid constructor/configuration fails before the loop starts.
- A panic is not swallowed; deployment supervision may restart the process and the lease recovers the event.
- Control compatibility 21, tenant-common 6 and Tenderpreneur 29 are startup prerequisites.
- Runtime dependency loss enters bounded process backoff rather than opening a fallback route or busy-looping.

### Health and observability

Because this is a dedicated asynchronous worker, API health is unchanged.

The later worker boundary exposes separate liveness/readiness semantics:

- liveness means the process loop is running and not deadlocked;
- readiness requires successful startup probes for control compatibility 21 and the explicitly registered tenant endpoint catalogue;
- durable holds and retrying events degrade delivery metrics but do not by themselves make the process unready;
- incompatible schema/configuration fails startup rather than reporting ready;
- runtime control-query failures increment a bounded failure signal and oldest-due/backlog age, but do not erase readiness evidence or fabricate successful delivery.

Required bounded signals are claim result, publication, defer reason, hold category, global claim failure, oldest due age, unpublished due count, hold count by category and loop shutdown outcome. Exact alert thresholds and paging ownership remain deployment review items.

## Option 2 — mount the dispatcher beside the pilot HTTP server

Use the existing `pilot-api` process and tie the loop to API startup/shutdown.

This reduces executables but couples control-to-cell delivery, tenant endpoint breadth and HTTP availability. A worker failure could incorrectly drain user-facing API readiness, and the API process would require broader cross-cell credentials. This conflicts with cell-ready failure isolation and is not recommended.

## Option 3 — unbounded drain loop with configurable timing

Continuously call `ProcessNext`, using deployment-selected lease, timeout and sleep values.

This is rejected. It can busy-loop, silently weaken lease safety, create configuration drift and starve other tenant work. Source-defined security and recovery limits must not depend on undocumented deployment defaults.

## Implemented source boundary

1. `internal/entitlementdispatch/application/runner.go` owns the runner policy without pgx, sqlc or process-signal imports.
2. Dispatch attempts return only `processed` and the claimed tenant ID as a scheduling hint.
3. The claim port/query accepts an optional previous-tenant hint used only for ordering.
4. The two-minute lease, 90-second attempt deadline, polling/backoff, cancellation unwind and burst-yield constants are source-defined with no fallback defaults.
5. `internal/entitlementdispatch/worker/worker.go` provides a separately mountable startup-probed boundary without changing `pilot-api`, existing document workers or API readiness.
6. No command/process is mounted or started; the boundary loads no ambient configuration or credentials.
7. A mandatory bounded observer port exposes loop/claim/disposition outcomes without tenant/event labels or payloads. Concrete metrics and health endpoints remain deployment composition work.
8. During the later verification phase, fake-timer unit tests and PostgreSQL tests remain required for cancellation, lease expiry, no-work polling, global-failure backoff, hold/defer continuation, tenant alternation, concurrent claim fencing and shutdown.

## Explicit exclusions

- service startup, deployment, migrations or database execution;
- changes to `pilot-api` or its existing document worker;
- more than one intentional pilot dispatcher replica;
- NATS subjects, consumers or broker credentials;
- settlement/release completion reconciliation;
- tenant movement or hold resolution/replay;
- alert thresholds, paging provider and production SLO acceptance;
- test implementation/execution in the current functionality-first phase;
- frontend work.

## Approved choice

Option 1 was approved, including:

- a dedicated separately mountable worker boundary rather than `pilot-api` coupling;
- one pilot loop/replica;
- two-minute non-renewable claim lease and 90-second attempt deadline;
- ten-second cancellation unwind;
- one-second idle polling;
- process-level `1, 2, 4, 8, 16, 30` second capped backoff only when no claim was obtained;
- a 100-millisecond yield after 25 processed claims;
- previous-tenant scheduling preference with stable due-time/ID ordering and no authority meaning;
- no heartbeat, early release, fallback routing, runtime migration or automatic process mounting.
