# Backend architecture

Sources: [LLD](../product/09_LLD.md), [API responsibility](../product/11_BACKEND_API_DESIGN.md); API-004/005, SR-EVT-001. Decisions [002, 003, 008, 018, 024](27_ARCHITECTURE_DECISIONS.md). Execution: [backend workstream](../implementation/06_BACKEND_IMPLEMENTATION.md).

## Dependencies and composition

Fiber is an HTTP adapter only. Domain methods accept values and return explicit domain errors/events; application handlers accept standard context, validated actor/tenant and commands. PostgreSQL adapters translate sqlc rows to domain objects. Composition roots in cmd/api and cmd/worker supply ports; no global mutable registry or repository singleton.

Illustrative interfaces (design examples, not source files):

```go
type UnitOfWork interface {
    WithinTenant(ctx context.Context, tenant AuthorizedTenant,
        fn func(TxScope) error) error
}
type BidCommands interface {
    Finalize(ctx context.Context, cmd FinalizeCandidate) (CandidateDTO, error)
    Approve(ctx context.Context, cmd ApproveCandidate) (DecisionDTO, error)
}
```

TxScope binds owner repositories, audit and outbox to the same transaction. Domain/application interfaces do not expose `pgxpool.Pool` or Fiber context. Pool details remain infrastructure concerns, refining the illustrative LLD router interface without changing its behavior. New Phase 1 use cases expose business operations through application-owned abstract units of work; an outer pgx adapter may compose transaction-scoped owner adapters, but SQL for an owned table remains in that owner module.

## Cell-ready deployment boundary

BID-ADR-024 combines Hexagonal DDD with a cell-ready deployment model. Shared identity, Platform Administration, membership, entitlement and placement remain in the global control plane. Tenant-private work resolves a server-owned tenant-to-cell assignment and generation only after authorization, then stays within that cell and the organization's private database. A cell identifier is placement metadata, never client authority or a database selector. Cross-cell moves use durable checkpoints/reconciliation rather than distributed transactions. The future exchange plane remains separately governed. See [`backend/docs/cell-ready-hexagonal-architecture.md`](../../backend/docs/cell-ready-hexagonal-architecture.md); this logical boundary does not authorize cell infrastructure or production topology.

## Request pipeline

Request ID → recovery/safe errors → limits/security headers → preflight → session → CSRF → abuse limit → validated context → registry/schema lease → input binding → capability/resource policy → idempotency acquisition → use case/transaction → safe DTO/metrics. Cheap edge abuse controls also precede expensive authentication. Request-supplied IDs are validated; no raw header is trusted as tenant authority.

## Approve command transaction

Load candidate and bid version using the transaction repository; authorize actor, scope, assurance and approved review policy; validate candidate still current; recheck evidence/source readiness; insert append-only decision; update applicability/aggregate version; append audit/outbox. Concurrent changes yield 409, not last-write-wins. Missing policy yields a blocked operation, not a default approver role.

## Error taxonomy

Validation maps to 422, malformed syntax to 400, absent identity 401, forbidden operation 403, concealed inaccessible record 404, stale/idempotency/state conflict 409, throttling 429, dependency unavailable 503. Internal errors retain wrapped causes in redacted telemetry while returning stable codes. Never map all errors to 500 or leak SQL/schema names.

## Startup/shutdown

Validate config and required secret references; connect bounded dependencies; verify schema compatibility; expose readiness only after required initialization. Runtime startup never creates databases or auto-migrates. On shutdown stop accepting writes/jobs, drain bounded requests, release worker leases and DB pools, flush telemetry within deadline. Interrupted jobs remain durable and retryable.

## Tests

Domain tests have no network. Application tests fake ports to verify transaction/audit/outbox behavior; integration tests use real PostgreSQL and broker. Assert rollback on audit/outbox failure, stale update conflicts, revoked worker authority and bounded cancellation. See [module contracts](09_BACKEND_MODULE_DESIGN.md).
