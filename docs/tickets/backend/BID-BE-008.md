# BID-BE-008 — Design governed requisition conversion and tender-authoring commands

## Metadata
- Ticket ID: BID-BE-008
- Epic: E-BE
- Type: Design
- Priority: P2
- Status: Blocked
- Complexity: M
- Dependencies: BID-DB-008, BID-ARC-010
- Blockers: Approved procurement workflow and incomplete command/page details
- Milestone: M8

## 1. Objective
Complete an independently implementable packet from approved requisition to validated tender candidate.
## 2. Business Context
Procurement officers need budget/method authorization and measurable published criteria, not generic tender CRUD.
## 3. Scope
Plan/request approval and conversion boundaries, lots/requirements/criteria/schedule authoring, review readiness, version invalidation and actor/department scope. Create granular runtime child tickets after packet approval.
## 4. Out of Scope
Actual full procurement module, publishing, evaluation, invented legal thresholds.
## 5. Technical Requirements
RULE-PREQ-001/002 and RULE-TEN-010–016; immutable approved input, reference-period uniqueness, criteria totals/scales, explicit timestamp semantics and source-specific configurable method rules.
## 6. Proposed Implementation Approach
Map procedure policy to commands, local transactions and queries; define contract/UI/error/test packet per conversion/validation operation; identify approval exceptions and evidence requirements.
## 7. Dependencies
[Future schema](../database/BID-DB-008.md), [coverage/owners](../architecture/BID-ARC-010.md).
## 8. Acceptance Criteria
- Given unapproved budget/request, then conversion design rejects it.
- Given invalid criterion totals/dates, then candidate cannot pass publication readiness.
- Given material edit, then approval applicability is explicitly invalidated.
## 9. Testing Requirements
Packet specifies valid/invalid state, concurrent author/reviewer and cross-department scope tests with actual requirement references.
## 10. Definition of Done
Approved operation packets and focused implementation follow-up list; no runtime completion claim.
## 11. References
[PR-PT-001/002/004](../../product/03_PRD.md), [modules](../../technical_design/09_BACKEND_MODULE_DESIGN.md), [backend plan](../../implementation/06_BACKEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Future legal/procurement policy cannot be guessed from Kenya-first positioning.
