# BID-REL-001 — Validate external-MVP acceptance and assemble release evidence

## Metadata
- Ticket ID: BID-REL-001
- Epic: E-REL
- Type: Testing
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-ARC-001, BID-QA-002, BID-QA-003, BID-QA-004, BID-SEC-003, BID-OPS-003
- Blockers: Signed scope and all required technical/security/recovery evidence
- Milestone: M4

## 1. Objective
Decide whether the scoped external-tender product is accepted for controlled pilot.
## 2. Business Context
Documentation and healthy containers cannot replace a bid team's complete verified journey.
## 3. Scope
Requirement-to-build/test mapping, persona UAT, exclusions, known defects, migration/rollback plan, privacy/security, accessibility, capacity/recovery and documentation sign-off.
## 4. Out of Scope
Full marketplace or AI acceptance, production deployment authorization.
## 5. Technical Requirements
Trace exact source/candidate/package/proof versions; no accepted mock workflow; honest pass/fail/skipped; owners sign relevant gates; no cross-tenant confidentiality waiver; source scope reduction only approved through ARC-001.
## 6. Proposed Implementation Approach
Freeze candidate; reconcile task evidence; run target persona UAT; triage gaps; obtain approvals or retain Blocked; publish immutable release package.
## 7. Dependencies
[Scope](../architecture/BID-ARC-001.md), [E2E](../qa/BID-QA-002.md), [a11y](../qa/BID-QA-003.md), [load](../qa/BID-QA-004.md), [security](../security/BID-SEC-003.md), [recovery](../devops/BID-OPS-003.md).
## 8. Acceptance Criteria
- Given valid scoped journey, then users complete it without manual DB intervention.
- Given missing required evidence, then release is not accepted.
- Given deferred feature, then release notes clearly avoid claiming it implemented.
## 9. Testing Requirements
Actual referenced reports plus product/procurement persona UAT and release checklist review.
## 10. Definition of Done
Signed acceptance package with precise limitations and pilot authorization decision.
## 11. References
[Traceability acceptance](../../product/19_TRACEABILITY_ACCEPTANCE_EVIDENCE.md), [QA design](../../technical_design/24_QA_TEST_ARCHITECTURE.md), [release plan](../../implementation/19_RELEASE_READINESS.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Milestones are evidence gates, not fixed dates or completed task claims.

## 13. Current bounded evidence snapshot — 2026-10-18

The backend pilot slice has live synthetic evidence for verified-TLS control/two-tenant PostgreSQL, Redis attempt controls, login, CSRF bootstrap, membership-authorized manual capture, idempotent replay, tenant isolation, logout, dependency bootstrap failure/recovery and graceful listener drain. Full race tests, vet, build, module verification and application vulnerability scans pass. `cmd/pilot-api` is runnable only with explicit deployment-owned configuration and has not been deployed.

Release acceptance remains **Blocked**. There is no frontend/UAT journey, delivery provider, registration/invitation policy, recovery/MFA, complete `/v1/me`, document scanning/storage, fact verification, workspace/bid workflow, accessibility/load/restore evidence, independent assessment, production configuration or accountable release sign-off.
