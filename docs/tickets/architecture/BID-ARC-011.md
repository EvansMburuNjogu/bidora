# BID-ARC-011 — Adopt cell-ready CBA and enforce Hexagonal DDD boundaries

## Metadata
- Ticket ID: BID-ARC-011
- Epic: E-ARC
- Type: Engineering
- Priority: P0
- Status: IMPLEMENTED_NOT_VERIFIED
- Complexity: M
- Dependencies: BID-ARC-010
- Blockers: Deferred architecture regression tests and real PostgreSQL transaction evidence
- Milestone: M7

## 1. Objective
Keep new Phase 1 backend modules aligned with Hexagonal DDD inside a cell-ready deployment model before additional application or HTTP behavior is layered onto them.
## 2. Business Context
Platform administration controls privileged identities and settings. Incorrect module ownership or persistence-owned policy makes security changes harder to reason about and verify.
## 3. Scope
Approved cell-ready control/cell/exchange boundaries; application-owned abstract unit of work; Identity- and Authorization-owned control queries; Platform Administration-owned governance persistence; transaction-scoped adapters; module facade; import-boundary verification specification.
## 4. Out of Scope
Repository-wide Option A rewrite, changing approved business policy, HTTP handlers, bootstrap execution, database startup, web work or production deployment.
## 5. Technical Requirements
Domain/application remain infrastructure-free; no Platform Administration SQL may directly query Identity or Tenancy tables; pgx/sqlc remain adapter-only; one control transaction may compose owner adapters without exposing DBTX to application; audit/outbox remain atomic.
## 6. Proposed Implementation Approach
Apply BID-ADR-024 and R1–R5 from `backend/docs/hexagonal-ddd-audit.md` to BID-AUTH-003B as a strangler pattern. Preserve migration 00015 facts while splitting queries and moving bootstrap sequencing to an application service. Keep Platform Administration in the global control plane; do not add cell infrastructure.
## 7. Dependencies
[Phase 1 architecture](BID-ARC-010.md), [BID-ADR-024 packet](../../../backend/docs/cell-ready-hexagonal-architecture.md), [audit](../../../backend/docs/hexagonal-ddd-audit.md), [Platform Admin policy](../../../backend/docs/platform-admin-authority-decision-packet.md).
## 8. Acceptance Criteria
- Platform Admin application imports no pgx/sqlc/Fiber/provider package.
- Platform Admin-owned query sources do not reference `identity.*` or `tenancy.*` tables.
- Bootstrap business sequencing is visible in the application service inside an abstract unit of work.
- Identity and Authorization adapters own their respective eligibility/session effects.
- Platform Administration remains global control-plane work and accepts no tenant/cell selector.
- No Option A behavior, cell infrastructure or route changes.
## 9. Testing Requirements
Later centralized architecture import tests plus real PostgreSQL bootstrap single-winner, rollback, owner-continuity and audit/outbox evidence. Current code-first phase permits formatting, generation and compile-only checks only.
## 10. Definition of Done
Refactored source compiles and generated artifacts are reproducible; technical verification, UAT and acceptance remain separate.
## 11. References
[Backend architecture](../../technical_design/08_BACKEND_ARCHITECTURE.md), [module contracts](../../technical_design/09_BACKEND_MODULE_DESIGN.md), [repository architecture](../../technical_design/02_REPOSITORY_ARCHITECTURE.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Command-specific atomic persistence ports remain in frozen Option A code. This ticket establishes the stricter boundary for new Phase 1 modules without falsely claiming a repository-wide conversion.
