# BID-FND-002 — Bootstrap one Next.js application with real API smoke view

## Metadata
- Ticket ID: BID-FND-002
- Epic: E-FND
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-FND-001, BID-ARC-006
- Blockers: Broader design-system approval and full prerequisite acceptance; web root is approved
- Milestone: M1

## 1. Objective
Prove the approved Next.js application talks to the real Go API.
## 2. Business Context
A working network/error boundary prevents later bid pages from relying on static mock success.
## 3. Scope
Pinned Next.js/TypeScript/Tailwind/shadcn baseline, accessible shell, same-origin routing and health smoke view.
## 4. Out of Scope
Tenant portals, fake users, decorative statistics and a second frontend root.
## 5. Technical Requirements
Consume the health schema from BID-API-001 through a centralized client. Keep backend configuration server-only; separate Server/Client Components; safe unavailable/starting/healthy states.
## 6. Proposed Implementation Approach
Create only approved root; establish tokens/layout; wire health query and safe errors; add build/type/component and browser smoke commands.
## 7. Dependencies
[Go runtime](BID-FND-001.md), [UX approval](../architecture/BID-ARC-006.md); inherited [health contract](../backend_contracts/BID-API-001.md).
## 8. Acceptance Criteria
- Given live API, then browser displays actual response.
- Given API unavailable, then honest retry/error state appears rather than fake success.
- Given production build, then no server credential appears in browser assets.
## 9. Testing Requirements
Strict type/build, component states, Playwright real API success/failure and keyboard focus smoke.
## 10. Definition of Done
Clean-start browser/API proof and safe failure evidence; no claimed authentication capability.
## 11. References
[Frontend stack](../../product/12_FRONTEND_UX_DESIGN.md), [frontend design](../../technical_design/12_FRONTEND_ARCHITECTURE.md), [foundation](../../implementation/04_FOUNDATION_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Audit template scripts/licenses before use; no template is assumed available.

### Authorized health-only increment
A real Next.js/TypeScript/Tailwind health page now calls the Go API through a fixed local server route. Generated contract types, bounded validation, safe failure states, build/type checks and a real Chrome readiness→outage→recovery test pass. See [execution evidence](../12_HEALTH_SLICE_EVIDENCE.md). No template/shadcn/domain UI adoption or authenticated browser-policy acceptance is claimed; those broader gates remain open.
