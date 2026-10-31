# BID-TEN-003 — Enforce quota reservations and governed tenant suspension

## Metadata
- Ticket ID: BID-TEN-003
- Epic: E-TEN
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-AUTH-002, BID-DB-002, BID-API-002
- Blockers: BID-ADR-015 limits, grace and suspension exceptions
- Milestone: M2

## 1. Objective
Provide race-safe entitlement checks before workspace, file and job creation.
## 2. Business Context
Tenant limits must be understandable and cannot destroy retained bid evidence when a subscription changes.
## 3. Scope
Operator-assigned versioned entitlements, reserve/settle/release ports, usage query, suspend/reactivate commands and approved remediation exceptions. Phase 1 meters successful new Bid-workspace creation and tenant licences; an active invitation reserves licence capacity and acceptance settles it.
## 4. Out of Scope
Plan/subscription/invoice lifecycle, PesaPal transport, invented prices or unrestricted suspension bypass; those are owned by BID-TEN-004 and BID-INT-004.
## 5. Technical Requirements
Control-plane operation IDs; atomic reservation; invitation-expiry/cancellation and membership-revocation release; one person counts once per tenant regardless of compatible roles; failed operation reconciliation; audit reason/assurance; no private tenant content in usage; writes denied during suspension except explicitly approved paths.
## 6. Proposed Implementation Approach
Implement entitlement port and concurrency tests; wire registry actions through policy; provide safe errors/usage DTO; define worker settlement/reconciliation contract.
## 7. Dependencies
[Policy](../authentication/BID-AUTH-002.md), [control persistence](../database/BID-DB-002.md), [contracts](../backend_contracts/BID-API-002.md).
## 8. Acceptance Criteria
- Given simultaneous final-capacity requests, then at most approved capacity is reserved.
- Given failed Bid-workspace creation, then its monthly allowance reservation is released exactly once.
- Given NO_BID, DEFER or opportunity analysis without workspace creation, then no Bid allowance is consumed.
- Given concurrent final-licence invitations, then at most approved capacity is reserved.
- Given invitation expiry/cancellation, then licence capacity releases exactly once.
- Given suspended tenant, then prohibited writes fail while retained records remain intact.
## 9. Testing Requirements
Quota races, repeated settle/release, stale policy, privileged action denial and safe usage response tests.
## 10. Definition of Done
Audited limit/lifecycle ports with reconciliation and operator runbook evidence.
## 11. References
[BR-004/005](../../product/02_BRD.md), [Phase 1 plans](../../../backend/docs/phase-1-portal-functional-design.md), [subscriptions](BID-TEN-004.md), [modules](../../technical_design/09_BACKEND_MODULE_DESIGN.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Source suspension/export rights are policy decisions; fail closed without silently denying lawful remediation.
