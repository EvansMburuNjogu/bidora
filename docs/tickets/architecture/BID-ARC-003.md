# BID-ARC-003 — Approve bid transitions, review policy and external-evidence semantics

## Metadata
- Ticket ID: BID-ARC-003
- Epic: E-ARC
- Type: Design
- Priority: P0
- Status: Proposed
- Complexity: M
- Dependencies: None
- Blockers: BID-ADR-011, BID-ADR-012; product/procurement approval
- Milestone: M0

## 1. Objective
Produce an authoritative actor/state/policy matrix for the external bid lifecycle.
## 2. Business Context
A stale approval or unsupported compliance claim can cause preventable disqualification.
## 3. Scope
Bid/No-Bid/defer mapping; requirement status/waiver rules; financial arithmetic; material-edit invalidation; reviewer separation; final approval; proof confirmation and exceptions; outcome correction history.
## 4. Out of Scope
Inventing legal procurement defaults, platform sealed deadline policy, implementation.
## 5. Technical Requirements
Resolve PR-TP-013 evidence requirement versus journey exception, and PR-TP-014 outcome values versus domain state machine. Define reported external time versus server capture time, approved policy versions and terminal transition behavior. Require immutable decision history and exact evidence versions.
## 6. Proposed Implementation Approach
Enumerate commands and forbidden transitions; review real synthetic tenders; document self-approval/delegation/expiry/override alternatives; obtain signed policy fixtures usable by schema and API tests.
## 7. Dependencies
None. Implementation contracts and constraints must wait for accepted policies, not just this ticket's draft.
## 8. Acceptance Criteria
- Given material edit after approval, then the matrix requires new applicable approval without deleting old decisions.
- Given late proof capture, then the rule does not equate capture time to external acceptance.
- Given unresolved waiver authority, then affected commands remain blocked.
## 9. Testing Requirements
Table-driven scenario review covering negative/race/exception cases; examples include expired evidence and outcome correction.
## 10. Definition of Done
Policy/version matrix signed, source conflicts resolved explicitly, machine-testable fixtures specified.
## 11. References
[Domain RULE-BID/DOC](../../product/05_DOMAIN_MODEL_AND_BUSINESS_RULES.md), [PRD](../../product/03_PRD.md), [domain design](../../technical_design/03_DOMAIN_ARCHITECTURE.md), [backend plan](../../implementation/06_BACKEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
A no-waiver proposal is not an approved removal of policy-supported exceptions.
