# BID-FE-009 — Deliver the simplified AI-first Tenderpreneur portal

## Metadata
- Ticket ID: BID-FE-009
- Epic: E-FE
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-FE-008, BID-API-011, BID-API-012, BID-TEN-004, BID-AI-005, BID-BE-016, BID-BE-017
- Blockers: Explicit web authorization, accepted page contracts, prices and AI language/accessibility review
- Milestone: M7

## 1. Objective
Let the four Tenderpreneur roles complete onboarding, AI-assisted tender work, billing and support through understandable real-data screens.
## 2. Business Context
The selling point is a simple journey from tender upload to cited assistance and safe submission readiness.
## 3. Scope
Tenderpreneur redirect/dashboard; guided company details/document/letterhead setup and Ready-for-AI states; four-role team controls; approved plan/licence/allowance/invoice/PesaPal views; natural-language Tender Assistant surfaces; privacy access/export/correction/deletion requests; support request/approval; links into existing bid workflow pages.
## 4. Out of Scope
Custom roles, Procurement portal, autonomous AI actions and frontend authorization.
## 5. Technical Requirements
Generated clients; private no-store SSR; tender-language/citation UX; loading/empty/error/pending-payment/provider-failure states; stale context cancellation; keyboard/mobile/WCAG 2.2 AA.
## 6. Proposed Implementation Approach
Extend the existing `web/` root only after authorization; deliver journey increments; retain server action denials and safe input on retry.
## 7. Dependencies
[Existing journey](BID-FE-008.md), [commercial contracts](../backend_contracts/BID-API-011.md), [AI contracts](../backend_contracts/BID-API-012.md), [assistant](../ai_search/BID-AI-005.md).
## 8. Acceptance Criteria
- Given an uploaded tender, then a user can understand AI findings and open exact sources without technical jargon.
- Given exhausted Bid/licence allowance, then an Admin can pay/upgrade and safely resume.
- Given clean company documents, then users see processing/review/readiness without treating AI extraction as approved evidence.
- Given privacy request, then the user sees verified scope, retained-lawful categories and honest checkpoint status.
- Given role loss/context switch, then stale private results do not render.
## 9. Testing Requirements
Playwright six-journey path, payment pending/failure, multilingual/citation UX, role negatives, accessibility and mobile tests.
## 10. Definition of Done
Real backend Tenderpreneur journey passes UAT and accessibility evidence with no mocked business completion.
## 11. References
[Phase 1 design](../../../backend/docs/phase-1-portal-functional-design.md), [onboarding/context](../../../backend/docs/phase-1-tenderpreneur-onboarding-context.md), [privacy design](../../../backend/docs/phase-1-data-privacy-retention-deletion.md), [frontend design](../../technical_design/12_FRONTEND_ARCHITECTURE.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
The current repository boundary forbids web edits until explicitly changed.
