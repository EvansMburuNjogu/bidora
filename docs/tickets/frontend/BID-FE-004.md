# BID-FE-004 — Deliver accessible compliance matrix with exact evidence linking

## Metadata
- Ticket ID: BID-FE-004
- Epic: E-FE
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-FE-002, BID-FE-003, BID-BE-003, BID-API-003
- Blockers: Approved compliance status/waiver policy
- Milestone: M4

## 1. Objective
Let bid teams assign, inspect and resolve requirements using authoritative evidence.
## 2. Business Context
Mandatory gaps should be actionable, not hidden behind a generic completion percentage.
## 3. Scope
Matrix filters/sort, owner/due fields, source/evidence side panel, exact-version picker, status review and deterministic blocker summary.
## 4. Out of Scope
Client-calculated authoritative readiness, automatic satisfied status and unsupported bulk overrides.
## 5. Technical Requirements
API-003 typed queries/commands; semantic headers/keyboard access; mobile detail alternative; classified evidence omitted; safe stale/conflict handling; no optimistic SATISFIED before server acceptance.
## 6. Proposed Implementation Approach
Build bounded paginated table; connect source/evidence panel; submit expected versions; invalidate readiness after accepted changes; test denied transitions and slow network.
## 7. Dependencies
[Bid UI](BID-FE-002.md), [vault](BID-FE-003.md), [domain](../backend/BID-BE-003.md), [contracts](../backend_contracts/BID-API-003.md).
## 8. Acceptance Criteria
- Given mandatory missing evidence, then blocker is visible and cannot be dismissed client-side.
- Given expired/forbidden version, then linking fails with safe recoverable feedback.
- Given concurrent edit, then UI preserves draft and offers refresh/compare.
## 9. Testing Requirements
Component/Playwright assignment-link-review, keyboard/screen-reader matrix, mobile and cross-tenant metadata tests.
## 10. Definition of Done
Requirement/evidence slice completed with real policy and accessibility evidence.
## 11. References
[PR-TP-008](../../product/03_PRD.md), [components](../../technical_design/13_FRONTEND_COMPONENT_DESIGN.md), [frontend plan](../../implementation/10_FRONTEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Virtualization requires accessibility proof, not only render-speed improvement.
