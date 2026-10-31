# BID-FE-006 — Present immutable candidate review and guarded human decisions

## Metadata
- Ticket ID: BID-FE-006
- Epic: E-FE
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-FE-005, BID-BE-005, BID-API-005
- Blockers: Accepted reviewer/assurance policy
- Milestone: M4

## 1. Objective
Make the exact candidate and approval consequences clear to reviewers.
## 2. Business Context
Approvers must distinguish the snapshot they reviewed from newer draft content.
## 3. Scope
Submit-for-review, pending reviews, candidate snapshot/diff, approve/reject/request changes, history and stale/expired authority states.
## 4. Out of Scope
Automatic approval, hidden policy overrides or UI-only permission decisions.
## 5. Technical Requirements
API-005 candidate/history/decision contracts; immutable version label, source/evidence links and allowed financial view; confirmation/step-up where required; no optimistic approved state; 409 re-review path.
## 6. Proposed Implementation Approach
Compose approval panel and version summary; query current applicability; submit exact candidate/version; display append-only decisions; integrate safe auth refresh without losing permitted comment.
## 7. Dependencies
[Response UI](BID-FE-005.md), [review backend](../backend/BID-BE-005.md), [contracts](../backend_contracts/BID-API-005.md).
## 8. Acceptance Criteria
- Given candidate changes while open, then approval rejection prompts explicit new review.
- Given revoked reviewer, then action fails safely and cached actions refresh.
- Given accepted decision, then UI shows actual immutable decision identity, not local state alone.
## 9. Testing Requirements
Two-user edit/approve browser race, assurance expiry, forbidden pricing, keyboard dialog and history correction tests.
## 10. Definition of Done
Review slice demonstrated with real immutable evidence and negative UX cases.
## 11. References
[PR-SH-004](../../product/03_PRD.md), [screens](../../technical_design/12_FRONTEND_ARCHITECTURE.md), [frontend plan](../../implementation/10_FRONTEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
A stale read must never lead users to believe their latest edits were approved.
