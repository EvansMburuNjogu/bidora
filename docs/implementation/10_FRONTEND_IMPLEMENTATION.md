# Frontend implementation

## 1. Purpose
Deliver usable operations backed by approved Go contracts, not mock dashboards.

## 2. Scope
Auth/onboarding/team, opportunity/bid, vault/matrix, responses/review, package/evidence and outcome screens.

## 3. Product specifications
[Frontend design](../product/12_FRONTEND_UX_DESIGN.md), UX-001–008, PR-TP-004–014; [template adoption](../product/20_FRONTEND_TEMPLATE_ADOPTION.md).

## 4. Technical designs
[Screens](../technical_design/12_FRONTEND_ARCHITECTURE.md), [components](../technical_design/13_FRONTEND_COMPONENT_DESIGN.md), [state](../technical_design/14_STATE_MANAGEMENT_DATA_FLOW.md).

## 5. Prerequisites
Approved frontend root, contract/client and page contract; template audited or product-owned baseline approved.

## 6. Required decisions
BID-ADR-005 path, 016 browser/proxy, 011 review policy and 017 upload/package controls.

## 7. Dependencies
Each accepted screen requires its real backend operation, permissions, migrations and error states. Component prototypes are not accepted business features.

## 8. Target files
Future `<frontend-root>/src/app`, feature folders, design primitives, API client/contracts and component/Playwright tests.

## 9. Sequence
Establish tokens/shell; request-scoped `/me`; central API/error handling; context-aware queries; one complete form/list/detail operation; upload/matrix/editor; immutable review; job/evidence/outcome; accessibility and mobile validation per slice.

## 10. Domain/security
No private shared SSR cache; never fetch pricing then hide it. Clear/cancel tenant state on switch; server determines readiness/actions. No offline queue for approvals/submissions.

## 11. Database/API/frontend
No DB imports in Next.js. Form validation mirrors contract shape, not business authority. 409 preserves draft and offers compare; 202 polls durable status; unavailable data never becomes fake success.

## 12. Tests
Keyboard/screen-reader and axe, 200% zoom, mobile upload/evidence, slow network, 401/403/404/409/429, old-context response, unsafe rich text and permission-filtered initial payload.

## 13. Acceptance
Target persona completes operation without raw ID entry, mock data or manual DB changes; every error has usable recovery.

## 14. Done
Real API integration, accessible states, visual review and multi-role E2E evidence.

## 15. Risks
Template defines wrong workflow; generic card grids hide blockers; autosave overwrites concurrent edits. Product page contracts and expected versions prevent this.

## 16. Evidence
Route/capability/API map, component reports, Playwright traces, accessibility findings and screenshots.

## 17. Exclusions
Duplicate app roots, placeholder procurement pages, frontend-only authorization and secret/browser telemetry leaks.
