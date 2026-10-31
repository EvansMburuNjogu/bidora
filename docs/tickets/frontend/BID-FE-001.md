# BID-FE-001 — Complete authenticated onboarding, workspace switching and role-aware shell

## Metadata
- Ticket ID: BID-FE-001
- Epic: E-FE
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-API-008, BID-TEN-002, BID-SEC-002
- Blockers: Approved page/identity contracts and real backend
- Milestone: M2

## 1. Objective
Let verified users authenticate once, resolve their exclusive Phase 1 portal type, reach the correct portal and manage permitted context/team actions through real APIs.
## 2. Business Context
All bid activity must clearly belong to the selected organization.
## 3. Scope
Shared auth/verification/recovery/MFA pages, canonical `/me`, server-directed Platform Admin/Procurement/Tenderpreneur redirect, onboarding progress, authorized tenant context, role-aware navigation and invitation/team controls.
## 4. Out of Scope
Frontend-only authorization, duplicate app roots, mixed portal identity switching, private support access UI and Procurement business workflow.
## 5. Technical Requirements
Consume API-002 via generated client; private SSR no-store; cancel/clear tenant state on switch/logout; ignore stale context responses; accessible field errors and generic account messages; safe 202/failed provisioning states.
## 6. Proposed Implementation Approach
Compose approved primitives; implement forms and context-aware queries; link jobs/status; show only permitted actions while retaining server denials; run multi-tab tests.
## 7. Dependencies
[Client](../backend_contracts/BID-API-008.md), [tenant routing](../tenancy/BID-TEN-002.md), [isolation](../security/BID-SEC-002.md); [consumed API](../backend_contracts/BID-API-002.md).
## 8. Acceptance Criteria
- Given authenticated user type and current authority, then redirect reaches only the permitted portal.
- Given forged portal URL or type, then server denial is preserved without private navigation/data.
- Given slow prior-tenant request, then switch never displays its result.
- Given expired session/MFA, then safe reauthentication preserves only permitted input.
- Given failed provisioning, then retry reflects durable state, not fake ACTIVE.
## 9. Testing Requirements
Playwright onboarding/auth/team/context race; component errors, keyboard/focus and secret bundle checks.
## 10. Definition of Done
Real authenticated tenant onboarding demonstration with negative/a11y evidence.
## 11. References
[PR-SH-001/002](../../product/03_PRD.md), [frontend](../../technical_design/12_FRONTEND_ARCHITECTURE.md), [frontend plan](../../implementation/10_FRONTEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Route groups and hidden navigation are not server authorization.
