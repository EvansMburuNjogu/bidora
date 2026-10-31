# AI entitlement acknowledgement stale-cell and poison-event recovery

Status: **OPTION 1 APPROVED — SOURCE IMPLEMENTED_NOT_VERIFIED — DISPATCHER UNMOUNTED**

Date: 2026-10-20

Tickets: BID-EVT-003, BID-EVT-001, BID-AI-003, BID-TEN-004E

## Approved decision

On 2026-10-20 the requesting user proceeded with recommended Option 1 for how Bidora must stop retrying a Commercial AI-entitlement acknowledgement when either:

1. the tenant has moved beyond the assignment generation captured by the immutable event; or
2. the immutable control outbox event is invalid or cannot be delivered after a finite retry budget.

The decision must preserve the authoritative Commercial outcome while preventing accidental delivery to a replacement cell, infinite retries and unaudited operator mutation. It does not authorize a tenant move workflow, worker startup, migration execution, provider execution or an operator replay API.

## Invariants shared by every acceptable option

- The original control outbox event is immutable and remains authoritative evidence of the Commercial `RESERVED` or `DENIED` outcome.
- Tenant ID, assignment generation, cell, database and event metadata never grant authority.
- An event for generation N is never sent directly to generation N+1.
- A stale event is never rewritten in place, marked successfully published, or silently discarded.
- A malformed event is never repaired by editing its payload.
- A control transaction and tenant transaction are never presented as one atomic transaction.
- Successful tenant application remains inbox/idempotency protected, and control publication occurs only after tenant commit.
- Held-event evidence contains identifiers, hashes, bounded reason codes and timestamps only. It contains no document text, object key, prompt, model output, secret reference, database locator or provider diagnostic.
- Held records have no automatic deletion until the broader retention/replay policy is approved.
- Metrics use bounded reason labels and never tenant IDs, cell IDs, event IDs or payload data as labels.

## Option 1 — durable typed holds with movement-gated reconciliation (recommended)

### Stale assignment handling

The assignment resolver returns a typed routing assessment rather than collapsing every failure into `tenant_assignment_unavailable`:

- `EXACT_CURRENT_ASSIGNMENT`: the active compatible assignment has the event generation; dispatch may continue.
- `STALE_ASSIGNMENT`: the registry generation is greater than the event generation; place the event on a durable `STALE_ASSIGNMENT` hold immediately.
- `GENERATION_AHEAD_OF_REGISTRY`: the event generation is greater than the registry generation; place it on an integrity hold immediately.
- `CURRENT_ASSIGNMENT_TEMPORARILY_UNAVAILABLE`: the generation still matches but the endpoint, cell, schema compatibility or tenant availability check is temporarily unavailable; use bounded transient retries.
- `REGISTRY_UNAVAILABLE`: no trustworthy classification can be made; use bounded transient retries.

A `STALE_ASSIGNMENT` hold does not forward the original event to the replacement cell. A future separately approved tenant-movement reconciler may resolve it only after durable evidence proves source freeze, exact checkpoint/job copy or restore, destination validation, authorization reconciliation and approved cutover. That reconciler must append a new reconciliation command/event referencing the original event and old/new generations. It must not mutate the original event or pretend that the old-generation event was delivered.

Until that movement contract exists, the tenant checkpoint and tender-analysis job remain pending and provider execution remains impossible.

### Poison and retry handling

Classify failures by semantics, never merely by message text:

- deterministic envelope/schema/aggregate/payload mismatch: immediate `POISON_EVENT` hold;
- event generation ahead of the trusted registry: immediate `INTEGRITY_CONFLICT` hold;
- stale assignment: immediate `STALE_ASSIGNMENT` hold;
- temporary control registry, endpoint, compatible-cell or tenant transaction unavailability: retry;
- ambiguous tenant commit: retry the same immutable event because tenant inbox/effect application is idempotent;
- exhausted temporary failure budget: `DELIVERY_EXHAUSTED` hold.

Recommended pilot retry budget is **nine total claims**. After failed claims 1–8, schedule the next claim after respectively:

`30 seconds, 2 minutes, 10 minutes, 30 minutes, 2 hours, 6 hours, 12 hours, 24 hours`.

Failure of claim 9 creates `DELIVERY_EXHAUSTED`. There is no unlimited retry and no attempt-counter reset. The event retains one stable ID throughout.

### Durable representation

Control migration `00021_ai_acknowledgement_dispatch_holds.sql` adds an acknowledgement-dispatch hold ledger owned by control eventing. Each event may receive at most one current hold fact containing:

- original event ID and immutable envelope SHA-256;
- hold category and bounded reason code;
- event assignment generation and observed current generation when safely available;
- fenced dispatch attempt;
- detected timestamp;
- unresolved status.

The claim query excludes held event IDs. Hold creation and fencing of the exact claimed attempt commit in one control transaction. The original outbox row remains unpublished.

A later resolution must append immutable resolution evidence; it must not delete the hold. No resolution handler, operator capability or retention expiry is authorized by this packet.

### Operational behavior

- Alert immediately for `POISON_EVENT` and `INTEGRITY_CONFLICT`.
- Alert immediately for `STALE_ASSIGNMENT` because movement reconciliation is required.
- Alert on `DELIVERY_EXHAUSTED` and on bounded backlog age/size thresholds selected during deployment review.
- Logs expose only trace/event IDs and bounded reason codes.
- Readiness does not report healthy merely because an event was held; held counts are operational degradation evidence, not process liveness failure by themselves.

## Option 2 — leave stale and invalid events on finite periodic retry

All failures consume the same nine-claim schedule and then enter one generic hold.

This is simpler, but it delays recognition of immutable corruption and cell-movement work, loses useful reason separation and repeatedly performs work that cannot succeed. It is not recommended.

## Option 3 — automatically retarget stale events to the current assignment

The dispatcher rewrites or reinterprets the old generation and sends the acknowledgement to the replacement cell.

This is rejected. It bypasses the required movement checkpoint, cannot prove the destination contains the exact pending tenant facts and would turn routing metadata into implicit cross-cell authority.

## Operator replay boundary

Option 1 does **not** authorize arbitrary replay or payload correction.

A future replay/reconciliation operation requires all of the following before implementation:

- an operation-specific Platform Administration capability;
- current TOTP/AAL2 and an explicit reason;
- exact event/hold version binding and idempotency;
- maker/checker policy for stale-cell reconciliation or source replacement;
- proof that the authoritative producer outcome still matches;
- destination generation and compatibility revalidation;
- immutable audit/outbox evidence;
- final-owner and support-access rules where applicable.

A poison event caused by a producer defect may only be superseded by a new source-owned event created from authoritative Commercial facts after a separately reviewed repair use case. Operators never edit JSON directly.

## Implemented source boundary

1. Typed assignment assessments distinguish exact, stale, generation-ahead and temporarily unavailable placement without exposing database locators to the dispatcher application.
2. Control migration `00021_ai_acknowledgement_dispatch_holds.sql` adds immutable holds and advances source compatibility to 21. Resolution evidence remains deferred because no resolution operation is approved.
3. The outbox port provides an exact-attempt-fenced `Hold`; retry scheduling is the approved fixed sequence rather than deployment configuration.
4. Claim, defer and publish queries exclude held events while leaving their original outbox rows unpublished.
5. Dispatcher policy separates deterministic poison, stale assignment, integrity conflict, transient/ambiguous failure and exhausted delivery.
6. The dispatcher and all recovery loops remain unmounted.
7. During the later verification phase, PostgreSQL race tests remain required for stale generation, hold fencing, claim exhaustion, poison events, ambiguous tenant commit and duplicate processing.

## Explicit exclusions

- tenant move, evacuation, failover or restore implementation;
- forwarding an old-generation event to a new cell;
- worker/service-loop mounting or startup;
- NATS publication/subscription;
- operator replay or hold-release HTTP endpoints;
- production alert thresholds, paging vendor or retention period;
- provider invocation, measurement execution or credit settlement changes;
- migration execution, test execution, deployment or web work.

## Approved choice

Option 1 was approved, including:

- immediate typed holds for stale assignment, generation integrity conflict and deterministic poison events;
- nine total claims for transient/ambiguous failures with the exact eight-delay schedule above;
- terminal `DELIVERY_EXHAUSTED` after claim 9;
- original outbox rows remaining unpublished and immutable;
- no automatic forwarding, payload editing, attempt reset, deletion or operator replay;
- future stale-cell resolution only through a separately approved movement-gated reconciliation operation.
