# BID-BE-001 — Implement transaction-bound command execution and safe errors

## Metadata
- Ticket ID: BID-BE-001
- Epic: E-BE
- Type: Engineering
- Priority: P0
- Status: VERIFIED_TECHNICALLY
- Complexity: M
- Dependencies: BID-FND-001, BID-DB-003, BID-OBS-001
- Remaining gates: independent security review, UAT and acceptance
- Milestone: M2

## 1. Objective
Provide explicit UnitOfWork, audit/outbox and idempotency primitives for domain commands.
## 2. Business Context
Approval and proof writes must never succeed without their corresponding evidence history.
## 3. Scope
Composition-owned application ports, transaction-scoped repositories, version conflicts, scoped idempotency acquisition/replay and safe domain-error mapping.
## 4. Out of Scope
Generic CRUD framework, authorization bypass, direct cross-module repository imports.
## 5. Technical Requirements
No repository fallback from transaction to pool; state/audit/outbox commit together; same key/different hash conflicts; replay reauthorization; error envelopes follow BID-API-001; context cancellation propagates. External calls never hold SQL transaction.
## 6. Proposed Implementation Approach
Implement ports and composition; build test command proving rollback; add error mappings and generated DTO boundary; enforce import direction in checks.
## 7. Dependencies
[Runtime](../foundation/BID-FND-001.md), [persistence](../database/BID-DB-003.md), [logging](../observability/BID-OBS-001.md); inherited base API contract.
## 8. Acceptance Criteria
- Given audit insert fails, then mutation/outbox roll back.
- Given concurrent same idempotency key, then only one operation executes.
- Given stale version, then safe 409 is returned without SQL details.
## 9. Testing Requirements
Unit port tests, real transaction/race integration, failure mapping, cancellation and forbidden import checks.
## 10. Definition of Done
Reusable primitives tested through real persistence, documented ownership and negative cases.
## 11. References
[LLD transactions](../../product/09_LLD.md), [backend design](../../technical_design/08_BACKEND_ARCHITECTURE.md), [backend plan](../../implementation/06_BACKEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Technical reuse cannot become a global mutable business model.

## Option A reconciliation — 2026-10-18
The bounded Option A source uses explicit transaction runners and application ports to commit state, audit, outbox and idempotency evidence together across implemented commands. Expected versions, matching-replay hashes, safe errors, server-owned routing and cancellation-aware database boundaries are composed in `cmd/pilot-api`; runtime startup never migrates.

## V3 technical verification — 2026-10-22

Against PostgreSQL 15.19 over verified TLS, the tenant capture command proved atomic state/source/audit/outbox/idempotency commit, matching replay, conflicting-hash rejection, same-key concurrent convergence, stale assignment denial, second-tenant non-observation, and complete rollback when either audit or outbox insertion failed. Representative control-plane races proved exactly-one invitation consumption and same-key session revocation replay without duplicate durable effects. Centralized HTTP evidence proves expected-version conflicts return safe `409 WORKFLOW_CONFLICT`, unknown SQL-like diagnostics return redacted `503 WORKFLOW_UNAVAILABLE`, and capture idempotency/in-progress conflicts return safe 409 responses. PostgreSQL boundary tests prove caller cancellation and bounded shutdown propagation; architecture checks retain transaction/module separation. Focused suites passed under the race detector. This advances the ticket to `VERIFIED_TECHNICALLY`, not UAT or acceptance.
