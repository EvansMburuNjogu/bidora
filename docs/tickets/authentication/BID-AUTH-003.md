# BID-AUTH-003 — Enforce portal type and fixed platform authority

## Metadata
- Ticket ID: BID-AUTH-003
- Epic: E-AUTH
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-AUTH-001, BID-API-010, BID-DB-002, BID-SEC-001
- Blockers: Control-schema/runtime implementation authorization, deployment-owned initial-owner UUID/bootstrap credential and verification evidence
- Milestone: M7

## 1. Objective
Route one authenticated identity to its authorized Phase 1 portal and govern fixed Platform Administrator roles.
## 2. Business Context
Shared login must not confuse navigation with authority or permit self-created platform administration.
## 3. Scope
Exclusive portal type, canonical `/v1/me` portal result, one-time owner bootstrap, six fixed platform roles, grant/revoke/list, final-owner protection, TOTP AAL2 and session/auth-epoch effects.
## 4. Out of Scope
Custom role designer, mixed portal identities, tenant-private support access and Procurement business functions.
## 5. Technical Requirements
Capabilities not labels authorize; active DENY wins; bootstrap is non-public and single-use; expected versions/idempotency; privilege changes revoke stale sessions; state/audit/outbox commit atomically.
## 6. Proposed Implementation Approach
Add reviewed control migration/queries; implement policy/application ports; expose only API-010 operations; compose shared login redirect data.
## 7. Dependencies
[Identity](BID-AUTH-001.md), [contracts](../backend_contracts/BID-API-010.md), [control DB](../database/BID-DB-002.md), [security](../security/BID-SEC-001.md).
## 8. Acceptance Criteria
- Given forged portal type, then no platform or tenant authority is established.
- Given concurrent final-owner removals, then an effective owner remains.
- Given privilege revocation, then old sessions cannot retain the role.
## 9. Testing Requirements
Bootstrap replay, self-elevation, grant-boundary, TOTP, stale session, concurrency and portal-redirect tests.
## 10. Definition of Done
Shared portal resolution and administrator lifecycle pass real PostgreSQL/session/security evidence.
## 11. References
[Phase 1 design](../../../backend/docs/phase-1-portal-functional-design.md), [identity design](../../technical_design/11_AUTHENTICATION_AUTHORIZATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Portal type remains routing metadata and never substitutes for current membership or capability.

### Bounded account-type slice — IMPLEMENTED_NOT_VERIFIED

The approved BID-AUTH-003A subset persists one nullable/immutable routing type on a verified, enabled identity. Existing identities with historical Tenderpreneur membership are backfilled to `TENDERPRENEUR`; identities without membership remain `UNASSIGNED`. A user may atomically self-select only `TENDERPRENEUR` or `PROCUREMENT` once, with hash-only idempotency evidence, minimal audit and outbox facts. `GET /v1/me` resolves only allowlisted paths and returns session assurance/expiry metadata; it grants no tenant context or authority. `PLATFORM_ADMIN` remains deliberately denied at the HTTP routing boundary until the approved owner/assignment/capability lifecycle receives application and handler composition. Control migration `00014`, sqlc, application/persistence, HTTP and pilot composition are source-complete but not migration-, race-, API- or acceptance-verified. The parent ticket remains Blocked for platform authority.

The requesting user approved P1–P6 in `backend/docs/platform-admin-authority-decision-packet.md`. The fixed capability catalogue, one-time deployment bootstrap, administrator invitation/activation, privilege/session effects, final-owner concurrency and highest-risk maker/checker boundaries are now accepted policy. Their lifecycle API is designed in `backend/api/openapi/drafts/platform-administrator-lifecycle.json`. The requesting user subsequently approved that lifecycle contract and authorized its bounded control-plane persistence. Control migration `00015_platform_administration.sql`, module-owned sqlc queries/generated artifacts, the source-defined fixed-role domain, atomic first-owner bootstrap committer and current-authority reader are `IMPLEMENTED_NOT_VERIFIED`. BID-ADR-024 approved cell-ready CBA plus Hexagonal DDD, and BID-ARC-011 corrected the Platform Admin bootstrap/authority transaction and owner-query boundaries. The bounded BID-AUTH-003C invitation issue/cancel/accept, BID-AUTH-003D owner-invitation disposition and BID-AUTH-003E fixed-role change and BID-AUTH-003F owner-role disposition application slices are now `IMPLEMENTED_NOT_VERIFIED`: it requires current effective-owner recent TOTP for issue/cancel, exact verified UNASSIGNED identity/recent TOTP/no-tenant-ties for acceptance, and commits fixed-role, auth-epoch/session, idempotency, audit and outbox effects through the abstract control-plane unit of work. Exact pending owner invitations can now be approved/activated or rejected by a distinct eligible current checker after current maker, two-owner continuity, recipient separation, version, expiry and recent-TOTP rechecks. Provider-neutral minimal HTML/text invitation and reminder templates exist, but reminder cadence, SMTP/runtime composition and delivery evidence remain deferred. Non-owner fixed-role changes now execute with auth-epoch/session invalidation; Platform Owner role changes create exact pending proposals. Identity disable/reactivation, cross-account session revocation, HTTP lifecycle orchestration, the deployment bootstrap executable, migration execution and verification remain not started.
