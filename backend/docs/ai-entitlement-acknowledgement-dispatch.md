# AI entitlement acknowledgement dispatch

Status: **IMPLEMENTED_NOT_VERIFIED — UNMOUNTED**

Date: 2026-10-20

Tickets: BID-EVT-003, BID-AI-003, BID-TEN-004E

## Outcome

The source now contains a server-owned control-to-tenant dispatcher for fixed Commercial AI reservation acknowledgements. It claims durable control outbox events, validates their immutable envelope, resolves the current compatible tenant assignment at the event's exact generation and invokes the tenant acknowledgement application documented in [document-entitlement-acknowledgement.md](document-entitlement-acknowledgement.md).

Tenant IDs, assignment generations, registry records and event payloads remain routing/applicability facts—not authorization.

## Dispatch sequence

For one due acknowledgement at a time, the dispatcher:

1. verifies control source compatibility is exactly version 21 and application generation 1 remains inside the declared read/write intervals;
2. claims one unpublished `commercial.ai_usage.reserved.v1` or `commercial.ai_usage.denied.v1` control outbox row with `FOR UPDATE SKIP LOCKED`;
3. increments its attempt and advances `next_attempt_at` to an explicit finite claim lease;
4. strictly decodes the bounded internal JSON envelope and cross-checks event type, aggregate, tenant, IDs, versions, disposition and stable denial/usage facts;
5. resolves only the current active Tenderpreneur assignment matching the event generation and requiring tenant-common compatibility at least 6 plus Tenderpreneur compatibility at least 29;
6. selects an endpoint from a server-built assignment catalog—never from caller/event database locators;
7. invokes the generation-bound tenant acknowledgement application;
8. marks the control event published only after tenant commit succeeds.

The control claim transaction is closed before tenant access. There is no distributed SQL transaction.

## Ambiguous outcomes and concurrency

The claim uses an explicit lease and attempt number as a fencing version. Publish/defer updates require that exact attempt. If the process stops after tenant commit but before control publication, the event becomes claimable again after the lease; tenant inbox/checkpoint/job replay makes the repeated effect idempotent.

Concurrent workers use `SKIP LOCKED`. A worker that outlives its lease cannot publish, defer or hold over a newer claim. Temporary registry, current-assignment, endpoint or tenant-application failures use the fixed nine-claim schedule.

A stale assignment generation is never sent to the current replacement cell. Approved [stale-cell and poison-event recovery](ai-entitlement-dispatch-recovery-decision.md) places it on an immutable `STALE_ASSIGNMENT` hold while leaving the original event unpublished. Future movement-gated reconciliation remains separately blocked; the source does not invent forwarding between old and new tenant databases.

Malformed immutable events become immediate `POISON_EVENT` holds. Generation-ahead integrity conflicts are also held immediately. Claim nine for a transient or ambiguous failure creates `DELIVERY_EXHAUSTED`. Context cancellation leaves the current claim lease to expire instead of deliberately consuming a retry transition.

## Accepted event surface

Only these fixed internal events are accepted:

- `commercial.ai_usage.reserved.v1`, aggregate `commercial_ai_usage_reservation`, version 1;
- `commercial.ai_usage.denied.v1`, aggregate `ai_usage_checkpoint`, version 1.

Payloads are capped at 4 KiB and may contain identifiers, usage kind, counts, state and stable denial codes. They contain no document text, object locator, prompt, model output or credential.

## Source changes

- `internal/entitlementdispatch/application/dispatcher.go`
- `internal/entitlementdispatch/application/runner.go`
- `internal/entitlementdispatch/worker/worker.go`
- `internal/controlplane/persistence/ai_acknowledgement_dispatch_store.go`
- `internal/opportunity/integration/entitlement_acknowledgement_routing.go`
- specialized typed tenant assignment resolver/reader in `internal/tenancy/`
- `queries/controlplane/ai_acknowledgement_dispatch.sql`
- `migrations/control/00021_ai_acknowledgement_dispatch_holds.sql`
- specialized compatibility-aware registry query in `queries/control/registry.sql`
- generated sqlc artifacts and the reviewed checksum manifest

Control migration 21 adds immutable fenced hold evidence and excludes held events from claim, defer and publish operations. Tenant-common 6 and Tenderpreneur 29 still provide the destination acknowledgement facts.

## Explicit exclusions

- runtime startup and deployment composition; the approved [bounded service-loop policy](ai-entitlement-dispatch-service-loop-decision.md) is source-implemented but unmounted;
- NATS/broker publication and subscription;
- cell-movement reconciliation and all hold resolution/replay operations;
- provider invocation;
- runtime composition of the separate settlement/release coordinator documented in [document-ai-accounting.md](document-ai-accounting.md);
- migrations or database execution;
- test implementation/execution.

The dispatcher is not mounted in `pilot-api` and does not alter the frozen Option A runtime compatibility floor. Its dedicated single-loop policy, fixed timing, previous-tenant preference, startup-probe port and bounded observer surface are source-implemented but have no deployed process, health endpoint or metrics adapter.
