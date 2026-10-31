# BID-FE-008 — Capture reported outcomes and show authorized correction history

## Metadata
- Ticket ID: BID-FE-008
- Epic: E-FE
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: S
- Dependencies: BID-FE-007, BID-BE-007, BID-API-006
- Blockers: Approved outcome policy
- Milestone: M4

## 1. Objective
Complete bid lifecycle tracking with honest outcome and lessons history.
## 2. Business Context
Teams need reliable learning records without overwriting prior reports or treating guesses as awards.
## 3. Scope
Outcome form, conditional reason/evidence fields, correction/supersedes timeline and basic permitted portfolio summary.
## 4. Out of Scope
Predictive win probability, unverified competitor facts and full BI dashboards.
## 5. Technical Requirements
API-006 enums/validation; current bid version; append-only history; reported-versus-verified labels; restricted financial fields omitted; accessible status semantics and conflict recovery.
## 6. Proposed Implementation Approach
Bind approved outcome fields/actions; show prior facts and corrections; refresh authorized portfolio queries; test invalid transitions and stale tabs.
## 7. Dependencies
[Submission UI](BID-FE-007.md), [outcome backend](../backend/BID-BE-007.md), [contracts](../backend_contracts/BID-API-006.md).
## 8. Acceptance Criteria
- Given correction, then original report remains inspectable with actor/time.
- Given invalid outcome transition, then form shows server error without changing history.
- Given unauthorized pricing, then summary reveals no hidden commercial values.
## 9. Testing Requirements
Component form/error tests, full-journey Playwright completion, correction race and keyboard/mobile checks.
## 10. Definition of Done
Outcome/history slice completes actual external journey with permission filtering and evidence.
## 11. References
[PR-TP-014](../../product/03_PRD.md), [screens](../../technical_design/12_FRONTEND_ARCHITECTURE.md), [frontend plan](../../implementation/10_FRONTEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
No inferred outcome from lack of response or AI output.
