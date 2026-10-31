# BID-AUTH-004 — Issue controlled tenant support access

## Metadata
- Ticket ID: BID-AUTH-004
- Epic: E-AUTH
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-AUTH-003, BID-API-010, BID-SEC-001
- Blockers: Support scope catalogue, tenant approval rule, maximum duration and emergency policy
- Milestone: M7

## 1. Objective
Allow support work only through tenant-approved, scoped, expiring and fully audited sessions.
## 2. Business Context
Customers need effective support without granting Platform Administrators standing access to private bids.
## 3. Scope
Cases, access request, tenant approval/rejection, TOTP step-up, scoped session issue, per-action recheck/audit, revoke/expire and tenant-visible summary.
## 4. Out of Scope
Implicit super-admin access, sealed-bid access, support self-approval and indefinite grants.
## 5. Technical Requirements
Exact tenant/resource/action scope; separate requester/approver; current tenant authority; hash-only opaque support session; bounded expiry; revocation freshness; no cross-tenant fallback.
## 6. Proposed Implementation Approach
Implement control-plane request/grant state, support-session middleware and owner-module authorization ports; keep tenant effects in their owning databases.
## 7. Dependencies
[Platform authority](BID-AUTH-003.md), [contracts](../backend_contracts/BID-API-010.md), [threat model](../security/BID-SEC-001.md).
## 8. Acceptance Criteria
- Given expired/revoked access, then every new operation is denied.
- Given out-of-scope record/action, then existence is concealed.
- Given support action, then tenant-visible immutable evidence is produced.
## 9. Testing Requirements
Cross-tenant probes, scope matrix, expiry/revocation races, self-approval denial and audit rollback tests.
## 10. Definition of Done
Real two-tenant support journey passes with no standing private access.
## 11. References
[J-17](../../product/06_USER_JOURNEYS_AND_WORKFLOWS.md), [security](../../product/14_SECURITY_PRIVACY_COMPLIANCE.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Emergency access remains denied until separately approved.

## 13. Phase 1 decision packet — DRAFTED_PENDING_APPROVAL

`backend/docs/controlled-support-access-decision-packet.md` and `backend/docs/controlled-support-access-review.md` now record selective policy approval only. T1–T6, D1 option 1, D2, D3 option 1, D4–D6 and B1 approve narrow effective-Organization-Administrator case/access semantics, mandatory tenant adoption of platform-recorded cases, one current TOTP approver who may only narrow/reject, system-known conflict checks, continuing approver authority, immutable closure, restricted minimized case text, tenant-nominated exact targets, aggregate-only membership diagnostics, versioned owner DTO/codes, five-scope/twenty-target ceilings, tenant approval/evidence for control-origin diagnostics and no Phase 1 break glass. Exact capability identifiers plus SS1–SS6, E1–E5 and R1–R5 remain pending. `backend/docs/controlled-support-session-evidence-fencing-decision.md` presents that separate ballot, corrects `__Host-` to require `Path=/`, and recommends a 15-second single-use control permit, ten-second owner deadline and final control commit serialized with revocation. Tenant SQL cannot atomically observe control revocation, and these recommendations are not yet approved. The proposed maximum remains 60 minutes with ten-minute idle expiry and no renewal, but session/evidence/revocation behavior is not executable. Document bytes, pricing, tender prose/evidence, AI content, downloads, exports, asynchronous jobs, mutations, impersonation and generic data access remain excluded. The user explicitly directed no implementation; no capability, schema, OpenAPI, migration, route, session, tenant access/evidence or runtime is authorized.
