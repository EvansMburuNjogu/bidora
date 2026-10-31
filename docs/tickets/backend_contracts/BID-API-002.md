# BID-API-002 — Complete identity, membership and workspace lifecycle contracts

## Metadata
- Ticket ID: BID-API-002
- Epic: E-API
- Type: Design
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-API-001, BID-ARC-005, BID-ARC-008
- Blockers: Accepted identity, quota and browser policies
- Milestone: M2

## 1. Objective
Specify all contracts needed for verified tenant onboarding and revocable membership.
## 2. Business Context
Users must safely create and select the correct organization before editing a bid.
## 3. Scope
Selected auth approach, recovery/MFA/step-up/session list-revoke, /me, workspace create/status, context select, invitation acceptance, roles, entitlement/suspension metadata.
## 4. Out of Scope
Building both password and OIDC flows, private tenant admin access.
## 5. Technical Requirements
Close missing session/invitation cards; define assurance/expiry, generic anti-enumeration responses, context generation, last-admin errors, idempotent creation and 202 jobs. Never accept DB locators or platform-role grants from tenant input.
## 6. Proposed Implementation Approach
Review command/state matrix; write OpenAPI with approved limits and sample forbidden/retry responses; align job schema with API-004; obtain security/product sign-off.
## 7. Dependencies
[Base](BID-API-001.md), [identity policy](../architecture/BID-ARC-005.md), [limits](../architecture/BID-ARC-008.md).
## 8. Acceptance Criteria
- Given invitation reuse, then contract specifies rejection without extra membership.
- Given stale context, then write routing fails explicitly.
- Given provisioning retry, then original operation identity is retained.
## 9. Testing Requirements
Examples for enumeration, expired factor, revoked session, last-admin conflict, tenant mismatch and quota failure.
## 10. Definition of Done
Complete approved request/response/error/auth/audit cards and schema validation report.
## 11. References
[API auth/context](../../product/11_BACKEND_API_DESIGN.md), [identity](../../technical_design/11_AUTHENTICATION_AUTHORIZATION.md), [contracts plan](../../implementation/07_BACKEND_CONTRACTS_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
A workspace selector remains a hint, never authorization.
