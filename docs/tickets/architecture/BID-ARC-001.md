# BID-ARC-001 — Approve external-tender release boundaries and source alignment

## Metadata
- Ticket ID: BID-ARC-001
- Epic: E-ARC
- Type: Design
- Priority: P0
- Status: Proposed
- Complexity: M
- Dependencies: None
- Blockers: Outstanding reviewer evidence; user approved Option A and BID-ADR-006 is synchronized
- Milestone: M0

## 1. Objective
Obtain an explicit release acceptance matrix distinguishing external MVP from full marketplace acceptance.
## 2. Business Context
Bid teams can gain independent value before buyers publish tenders, but current BRD/PRD acceptance requires the full platform.
## 3. Scope
Map BRD §10, PRD §§7–8, roadmap R2 and W07. Record manual/file import subset, AI/search/payments exclusions, retained future requirements and approval owners. Reconcile legacy TenderOS naming and document-qualified ADR aliases without editing protected sources in this task.
## 4. Out of Scope
Application implementation, silent renaming of source specifications, declaring the MVP approved.
## 5. Technical Requirements
Define M0–M8 gates, source precedence and decision status. New ticket IDs must map to BID-T000–017 without pretending those planning IDs were executed tickets.
## 6. Proposed Implementation Approach
Inventory contradictions; propose scope alternatives and business consequences; obtain product/procurement sign-off; prepare an authorized source-update change request separately.
## 7. Dependencies
None. Approval evidence must name owner/date and affected requirements; an agent recommendation is insufficient.
## 8. Acceptance Criteria
- Given broader PRD acceptance, when external MVP is approved, then excluded capabilities remain assigned to future milestones rather than deleted.
- Given no sign-off, then dependent implementation remains Blocked.
- Given legacy ADR collisions, then each alias is qualified by source file.
## 9. Testing Requirements
Documentation traceability and link checks; reviewer walks external and closed-loop acceptance scenarios.
## 10. Definition of Done
Signed scope matrix, explicit exclusions, preserved source requirements and decision evidence; no runtime completion claim.
## 11. References
[BRD](../../product/02_BRD.md), [PRD](../../product/03_PRD.md), [decisions](../../technical_design/27_ARCHITECTURE_DECISIONS.md), [strategy](../../implementation/01_IMPLEMENTATION_STRATEGY.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Do not use prioritization instructions alone as formal acceptance of all proposed feature reductions.

### Execution record — 2026-10-17
Prepared [release scope decision packet](../09_RELEASE_SCOPE_REVIEW.md): source acceptance matrix, three alternatives, recommended manual external increment, explicit exclusions/future gates, qualified ADR aliases and reviewer walkthroughs. The requesting user subsequently approved Option A explicitly: **“I approve option A”** (recorded 2026-10-17). Status remains **Proposed** pending reviewer evidence. The user's subsequent instruction to proceed authorized synchronization: BID-ADR-006 is now Approved and the implementation strategy reflects Option A. The user's formal product-owner title/name was not supplied. Product sources and application directories remain unchanged; scope approval is not implementation authorization.
