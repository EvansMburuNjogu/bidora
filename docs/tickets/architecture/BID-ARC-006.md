# BID-ARC-006 — Approve frontend page contracts and design-system adoption

## Metadata
- Ticket ID: BID-ARC-006
- Epic: E-ARC
- Type: Design
- Priority: P1
- Status: Proposed
- Complexity: M
- Dependencies: None
- Blockers: UX/product review; template unavailable or unaudited
- Milestone: M0

## 1. Objective
Approve accessible route/component/page boundaries for the external-tender journey.
## 2. Business Context
Users must see evidence gaps, stale approvals and submission status without confusing visual success with actual acceptance.
## 3. Scope
Auth/onboarding, workspace switcher, bid dashboard, upload/vault, compliance matrix, response/review and evidence/outcome screens; template license audit or product-owned primitives proposal.
## 4. Out of Scope
UI code, decorative mock dashboards, procurement portal placeholders.
## 5. Technical Requirements
For each screen identify capability, data fields, API dependency, action/state conditions, error/conflict/offline behavior, keyboard/mobile pattern and classification filtering. Resolve missing candidate/history/detail read contracts through API tickets.
## 6. Proposed Implementation Approach
Walk persona journey; map components; inspect available assets before execution; define tokens/semantic states; review matrix and evidence interactions with keyboard/mobile scenarios.
## 7. Dependencies
None for design review. Runtime root choice remains [BID-ARC-002](BID-ARC-002.md), not a prerequisite to discuss page behavior.
## 8. Acceptance Criteria
- Given restricted pricing, then no screen contract fetches forbidden fields merely to hide them.
- Given stale approval, then UI recovery requires authoritative refresh/new review.
- Given missing template license, then adoption is blocked or a separately approved owned baseline selected.
## 9. Testing Requirements
Documented WCAG 2.2 AA checks, screen-reader/keyboard flows, responsive essential actions and API-state mapping review.
## 10. Definition of Done
Approved page/component catalogue, asset decision and explicit API gaps with owners.
## 11. References
[Frontend](../../product/12_FRONTEND_UX_DESIGN.md), [template](../../product/20_FRONTEND_TEMPLATE_ADOPTION.md), [screens](../../technical_design/12_FRONTEND_ARCHITECTURE.md), [frontend plan](../../implementation/10_FRONTEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Screenshots do not establish backend authorization or workflow completeness.
