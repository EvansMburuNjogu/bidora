# BID-QA-006 — Verify complete Phase 1 portal journeys

## Metadata
- Ticket ID: BID-QA-006
- Epic: E-QA
- Type: Testing
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-FE-009, BID-FE-011, BID-QA-001, BID-SEC-003, BID-OPS-003
- Blockers: Runnable Phase 1 candidate, provider sandboxes, approved corpus and UAT owners
- Milestone: M7

## 1. Objective
Verify shared routing, Tenderpreneur AI/commercial journeys and Platform Admin operations using real dependencies and adversarial cases.
## 2. Business Context
Phase 1 is not complete until both customer and operator journeys recover safely from dependency, concurrency and authorization failures.
## 3. Scope
All account types, four Tenderpreneur roles, six platform roles, company setup/letterhead, every-eligible-document FTS/vector lifecycle, AI journeys, approved plan/licence/allowance prices, PesaPal, SMTP/settings, dashboards, privacy requests, support and approved retention behavior.
## 4. Out of Scope
Procurement lifecycle, production provider acceptance and unapproved destructive retention.
## 5. Technical Requirements
Synthetic data; two tenants; real PostgreSQL/Redis/storage/scanner/mail/provider fixtures; race/fault/restart/restore; privacy/log review; accessibility and mobile coverage.
## 6. Proposed Implementation Approach
Create traceability matrix from the Phase 1 design and child acceptance criteria; execute packets in dependency order; stop on blocking failure.
## 7. Dependencies
[Tenderpreneur portal](../frontend/BID-FE-009.md), [platform portal](../frontend/BID-FE-011.md), [harness](BID-QA-001.md), [security](../security/BID-SEC-003.md), [restore](../devops/BID-OPS-003.md).
## 8. Acceptance Criteria
- Given each persona, then permitted journeys succeed and forbidden/cross-tenant actions fail without leaks.
- Given duplicate/crash/outage, then billing, AI and support recover without duplicate authority/effects.
- Given evidence completion, then results identify exact build/schema/contracts and remaining provider/UAT gates.
## 9. Testing Requirements
DB/API/browser/race/security/accessibility/load/restore/AI multilingual retrieval corpus, tombstone/purge, historical branding, privacy request/hold and sandbox payment suites.
## 10. Definition of Done
Executed report has no unresolved release-blocking defect and is signed by technical/security reviewers; UAT remains separate.
## 11. References
[Phase 1 design](../../../backend/docs/phase-1-portal-functional-design.md), [onboarding/context](../../../backend/docs/phase-1-tenderpreneur-onboarding-context.md), [privacy design](../../../backend/docs/phase-1-data-privacy-retention-deletion.md), [gap audit](../14_PHASE_1_PORTAL_GAP_AUDIT.md), [AGENTS](../../../AGENTS.md).
## 12. V10 release-gate assessment — BLOCKED 2026-10-23

The first full live backend integration run executed all 35 PostgreSQL/Redis tests under `-race`. Isolated V3–V9 business journeys and most identity tests passed, but six production-pilot compositions remain blocked at `pilot_deployment_open_failed` because their provider-free fixtures predate mandatory document-processing and TLS NATS JetStream configuration. No authorized compatible JetStream endpoint/credentials exist. The run stopped before a single composed account-to-archive journey, load/resource measurement, privacy/log inspection, current-schema restore, provider acceptance, browser/accessibility coverage or independent security review. Repository-wide non-live tests, vet and build pass, but do not satisfy this ticket.

Status remains **Blocked**. BID-FE-009/BID-FE-011, BID-SEC-003, BID-OPS-003, real dependency fixtures, approved UAT owners and signed technical/security review remain unresolved.

## 13. Risks and Notes
A test plan or mocked happy path is not Phase 1 verification.
