# BID-DB-003 — Persist tenant common records and atomic audit/event infrastructure

## Metadata
- Ticket ID: BID-DB-003
- Epic: E-DB
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-DB-001, BID-API-007
- Blockers: Approved event/transaction schema
- Milestone: M2

## 1. Objective
Provide tenant template and plane-local audit/outbox/inbox/idempotency/job tables.
## 2. Business Context
Every material bid operation needs durable history and retry-safe asynchronous effects.
## 3. Scope
Organization singleton, user assignment projection, immutable audit, event payload/delivery state, deduplication, job leases and control-plane equivalents.
## 4. Out of Scope
Broker consumers, global cross-tenant audit joins, raw private content in event payload.
## 5. Technical Requirements
Audit runtime INSERT/SELECT only; unique consumer/event and actor/operation/key; immutable event IDs/payloads; partial pending indexes; shared-plane tenant attribution; logical actor references preserve historical attribution.
## 6. Proposed Implementation Approach
Add common migrations and owner-scoped sqlc queries; separate immutable facts from operational delivery fields; verify all stores can participate in a single owner DB transaction.
## 7. Dependencies
[Migrations](BID-DB-001.md), [event contracts](../backend_contracts/BID-API-007.md).
## 8. Acceptance Criteria
- Given audit UPDATE/DELETE by runtime, then PostgreSQL denies it.
- Given duplicate inbox key, then second effect is prevented transactionally.
- Given aborted domain transaction, then audit and outbox rows also roll back.
## 9. Testing Requirements
Constraint/grant tests, pending-index plans, concurrent lease acquisition, idempotency hash conflict and migration upgrade.
## 10. Definition of Done
Typed queries and migration evidence demonstrate atomic persistence without broker assumptions.
## 11. References
[SR-EVT-001–005](../../product/04_SRD.md), [BR-050](../../product/02_BRD.md), [schema](../../technical_design/06_DATABASE_SCHEMA_DESIGN.md), [events plan](../../implementation/12_EVENTS_WORKERS_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Inbox retention cannot silently permit replay of old irreversible external effects.

### Bounded manual-capture foundation — 2026-10-18

Under the user's database-foundation execution request, implemented only tenant-local audit, outbox and capture-idempotency structures needed for approved unverified manual capture. Audit mutation is trigger-rejected; outbox fact fields are immutable while delivery fields may progress; idempotency hashes are fixed-width and have no automatic TTL. Goose SQL, checksums, sqlc query generation and centralized static regression tests exist. [Evidence](../../../backend/docs/database-foundation.md).

A subsequent authorized schema continuation adds a singleton organization profile and non-authoritative local user profiles with logical control identity/membership references and monotonic projection versions. These tables support ownership/display only; they contain no grants and cannot authorize from `membership_state`.

A further provider-neutral control migration adds append-only control audit and immutable outbox facts, with tenant/actor fields retained only as provenance. It deliberately omits control idempotency until operation principal scopes/retention are approved.

A further common migration adds immutable payload-free inbox receipts and generated `ClaimInboxEvent` (`ON CONFLICT DO NOTHING`). No automatic expiry is defined; retention approval is required before cleanup. Receipt and database effects must share one transaction, and external effects still need operation state/reconciliation.

This does **not** implement jobs, roles/grants, runtime role grants, broker dispatch or complete control-plane analogues, so the parent remains Blocked. Goose/sqlc parser/generator checks and full Go tests passed; no PostgreSQL migration, privilege, rollback, transaction or integration test ran.
