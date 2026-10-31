# BID-FE-007 — Guide package generation and external proof confirmation

## Metadata
- Ticket ID: BID-FE-007
- Epic: E-FE
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-FE-006, BID-BE-006, BID-API-006
- Blockers: Accepted evidence/format policy
- Milestone: M4

## 1. Objective
Let users generate approved files, perform external submission themselves and record verifiable proof.
## 2. Business Context
The interface must not confuse package creation or proof upload with delivery to an external buyer.
## 3. Scope
Readiness/checklist, package job/progress/download, proof upload/registration, confirmation and mark-submitted history.
## 4. Out of Scope
Automatic external transmission, sealed platform receipt or optimistic submitted state.
## 5. Technical Requirements
API-006 plus document API-004; exact candidate/package version; safe job polling and stable idempotency key on network retry; separate reported and capture time labels; restricted package disclosure; accessible progress and consequences.
## 6. Proposed Implementation Approach
Compose readiness→approval→package→proof panels; preserve operation identity on ambiguous response; require clean proof and permitted verifier; refresh authoritative result/history.
## 7. Dependencies
[Review UI](BID-FE-006.md), [proof backend](../backend/BID-BE-006.md), [contracts](../backend_contracts/BID-API-006.md).
## 8. Acceptance Criteria
- Given package job failure, then UI never announces successful submission.
- Given lost mark-submitted response, then retry returns original evidence result.
- Given missing proof/stale readiness, then blockers remain actionable and server-owned.
## 9. Testing Requirements
Playwright package-to-proof, job outage/retry, duplicate clicks, expired permission, mobile capture and screen-reader announcements.
## 10. Definition of Done
Actual external tracking slice demonstrated without implying unsupported delivery guarantees.
## 11. References
[PR-TP-013](../../product/03_PRD.md), [evidence flow](../../technical_design/20_AUDIT_LOGGING_EVIDENCE.md), [frontend plan](../../implementation/10_FRONTEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
User-entered external time/reference must be visibly distinguished from system evidence.
