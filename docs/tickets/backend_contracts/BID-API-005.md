# BID-API-005 — Specify response revision, candidate review and approval contracts

## Metadata
- Ticket ID: BID-API-005
- Epic: E-API
- Type: Design
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-API-003, BID-ARC-004
- Blockers: Approved workflow/arithmetic and candidate schema
- Milestone: M4

## 1. Objective
Make immutable candidate and approval semantics explicit to backend and frontend.
## 2. Business Context
Technical contributors, finance and approvers must work on known versions without pricing leakage.
## 3. Scope
Technical/financial revision commands and read/history queries, finalize/submit-for-review, candidate detail, pending approvals, decision and current applicability.
## 4. Out of Scope
Generic approval builder, automatic approval, final-score/award APIs.
## 5. Technical Requirements
Separate pricing capabilities and decimal strings; candidate manifest version; expected aggregate version; assigned reviewer/assurance; immutable decisions; invalidation after material edit. Define comparison payload that omits unauthorized financial fields.
## 6. Proposed Implementation Approach
Map policy matrix to schemas; resolve which aggregate ETag guards each action; specify APPROVAL_STALE and safe change-request/rejection flows; review sample concurrent edit timeline.
## 7. Dependencies
[Bid contracts](BID-API-003.md), [schema/backend review](../architecture/BID-ARC-004.md).
## 8. Acceptance Criteria
- Given finalized candidate, then contract identifies exact input versions but does not claim approval.
- Given stale candidate, then approve returns conflict and no success DTO.
- Given technical-only user, then pricing fields are absent from read/history schemas in that context.
## 9. Testing Requirements
Examples for malformed decimal, stale version, denied pricing, expired assurance and invalid review step.
## 10. Definition of Done
Approved OpenAPI/action matrix and example validation before handlers/client generation.
## 11. References
[PR-TP-009/011](../../product/03_PRD.md), [API design](../../technical_design/10_API_CONTRACT_DESIGN.md), [contract plan](../../implementation/07_BACKEND_CONTRACTS_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Candidate/decision IDs are not authority; current membership and policy remain required.
