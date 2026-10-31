# BID-AUTH-002 — Enforce membership scopes, invitations and stale-context rejection

## Metadata
- Ticket ID: BID-AUTH-002
- Epic: E-AUTH
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-AUTH-001, BID-API-002
- Blockers: Accepted capability/role matrix
- Milestone: M2

## 1. Objective
Implement authoritative tenant membership and capability checks independent of UI roles.
## 2. Business Context
A finance contributor in one organization must not gain pricing or approval authority in another.
## 3. Scope
Invitation accept, scoped grants/denies, last-admin guard, current capabilities, context select/generation, revocation and application authorization port. Phase 1 uses four fixed Tenderpreneur roles: Organization Admin, Bid Manager, Contributor and Approver.
## 4. Out of Scope
Automatic private support access, custom role designer, platform roles and database routing.
## 5. Technical Requirements
Control membership authority; grantable-role limits; explicit deny/scope precedence; serialized last-admin updates; assurance and resource policy inputs; session refresh/revoke on changes. Role labels group capabilities only. Maker/approver and submission reporter/verifier separation are enforced per operation even when one user holds multiple compatible roles. No local projection grants authority.
## 6. Proposed Implementation Approach
Implement policy evaluator and command guards; wire /me/context; bind validated actor/context to use cases; document worker execution revalidation port.
## 7. Dependencies
[Identity](BID-AUTH-001.md), [contracts](../backend_contracts/BID-API-002.md).
## 8. Acceptance Criteria
- Given two last-admin removals, then required administrator remains.
- Given forged workspace selector, then no tenant authority is established.
- Given stale tab context, then command rejects rather than silently targeting a newly selected tenant.
## 9. Testing Requirements
Capability matrix, invitation replay, escalation, revocation freshness, concurrency and context-race integration tests.
## 10. Definition of Done
Server-side policy port and endpoints satisfy negative tests with audited changes.
## 11. References
[RULE-IAM](../../product/05_DOMAIN_MODEL_AND_BUSINESS_RULES.md), [identity](../../technical_design/11_AUTHENTICATION_AUTHORIZATION.md), [auth plan](../../implementation/08_AUTHENTICATION_AUTHORIZATION.md), [AGENTS](../../../AGENTS.md).
### Member activity and ownership continuity preparation

`GET /v1/tenderpreneur/organization/ownership-continuity` now requires deny-first current-context `tenant.memberships.manage`, reads at most 200 current active/verified/non-disabled control-plane members, and reconciles them with at most 500 tenant-local ownership/activity identities under the exact assignment generation. It reports unarchived bid ownership, open requirement/review assignments, overdue reviews and last content-free collaboration activity by internal IDs only. Work owned by identities absent from the current active membership snapshot is flagged as orphaned. It returns no emails or role labels and always keeps `can_revoke_without_review=false`, `automatic_transfer_supported=false` and `last_active_admin_policy_configured=false`. It does not broaden the existing fixed-contributor revocation command or implement last-admin rules. Status: **IMPLEMENTED_NOT_VERIFIED**.

BID-ADR-021 is approved and implemented in source as **IMPLEMENTED_NOT_VERIFIED**. Control migration `00011` records resumable offboarding operations/events and the fail-closed `OFFBOARDING` state; Tenderpreneur migration `00026` fences assignment races and records immutable bounded ownership-transfer evidence. The composed pilot routes begin/read/retarget/cancel operations with current manager authority, a fresh 15-minute session, expected generations, reason and idempotency. The serialized last-administrator guard, explicit eligible successor, 500-record bound and non-reversing cancellation are enforced without automatic succession, restoration, platform override or cross-tenant transfer. Live cross-plane race/crash and migration verification remains deferred.

## 12. Risks and Notes
Test fixtures may precreate registry records; do not expose provisioning before this boundary is accepted. The simplified role catalogue does not authorize financial-bid functionality or custom role creation.

### Approved narrow capture-contributor invitation slice — 2026-10-18

The requesting user approved an invitation policy restricted to an active verified inviter with explicit `tenant.memberships.manage` authority, one fixed `TENDERPRENEUR_CAPTURE_CONTRIBUTOR` provenance role granting only `tenderpreneur.opportunity.capture_external`, 32-byte SHA-256-at-rest tokens with 72-hour expiry, verified matching-email authenticated acceptance, single use and one active invitation per tenant/email. This slice cannot grant or remove administrator authority, so last-admin mutation remains blocked.

Control migration 9, generated invitation queries, the application service and pgx committer now implement atomic acceptance: token/user/tenant eligibility is locked and rechecked, one ACTIVE membership and fixed ALLOW grant are created, the invitation is consumed, context generation advances, and `InvitationAccepted` audit plus `membership.granted.v1` outbox evidence commit together. Live PostgreSQL evidence using the migrator role verifies fixed authority, generic replay denial and one winner under concurrent consumption. Authorized issuance now locks and rechecks explicit `tenant.memberships.manage` ALLOW with active DENY precedence, invalidates a prior active tenant/email invite, commits a fresh hash plus `InvitationIssued` audit and `invitation.issued.v1` outbox, then invokes a provider-neutral delivery port. Live evidence proves distinct resend tokens, one active invitation, and no delivery after DENY. The vertical slice is now composed in `cmd/pilot-api`: authenticated issue and acceptance handlers use exact Origin/session-bound CSRF, strict 4 KiB JSON, tenant/context selectors only for deny-first issuance, fixed safe failures and no new acceptance cookies. The existing authenticated-STARTTLS adapter sends a minimal token/expiry invitation only after commit. Local runtime-role grants were narrowed to invitation SELECT/INSERT/UPDATE, membership INSERT/UPDATE and grant INSERT; the complete production deployment path then proved manager login → issue → SMTP delivery → verified matching-email invitee login → acceptance → context generation 2 → authorized tenant-local capture through `pilot.Open`. No caller-selected authority or administrator capability is introduced. The draft contract is `backend/api/openapi/drafts/invitations.json`. `GET /v1/me/memberships` is now composed as the bounded membership-aware bootstrap read: it returns only ACTIVE memberships in ACTIVE tenants, current context generations, and explicit effective ALLOW capabilities after same-capability active DENY precedence; it exposes no role authority or database references. The live invitation journey verifies capture appears after acceptance, then disappears from the directory after a DENY bumps generation and the corresponding capture is forbidden. Authenticated password-backed users can now call `POST /v1/auth/password` with exact Origin/session CSRF, account email, fresh current-password proof and an approved-profile new password. Credential version and auth epoch advance atomically, all prior sessions are revoked, one replacement session is linked to the current session, and `PasswordChanged` plus replacement-login audits are persisted. Live evidence verifies failed proof causes no mutation, successful Argon2id replacement, old-session rejection and replacement-session continuity. Recovery, passwordless Google accounts, MFA and session administration remain excluded. The bounded `DELETE /v1/shared/invitations/{invitation_id}` operation now cancels only active, unexpired, unconsumed fixed-role invitations after exact Origin/session CSRF and deny-first current-context manager authorization. Invalidation, `InvitationCancelled` audit and `invitation.cancelled.v1` outbox version 2 commit atomically; replay and inaccessible targets are concealed as not found. Live evidence covers success, persisted evidence, replay concealment and non-manager denial without preventing later acceptance of the untouched invitation. The manager-facing `GET /v1/shared/invitations` directory now returns only active, unexpired fixed-role invitations after the same deny-first current-context authorization. It uses bounded newest-first keyset pagination and excludes raw tokens, hashes, lookup forms and creator IDs. Live evidence covers authorized listing, unauthorized denial, cursor continuation and immediate disappearance after consumption. The manager-facing `GET /v1/shared/memberships` directory now makes the narrow lifecycle usable: after deny-first current-context authorization it returns only active fixed capture contributors, with membership ID, preserved display email and current generation; administrator and arbitrary-role memberships are excluded from the result. Live evidence verifies the authorized manager result and a contributor's `403` denial. The bounded `DELETE /v1/shared/memberships/{membership_id}` operation now lets a deny-first authorized membership manager revoke only the fixed capture-contributor membership: state, grant, generation, `MembershipRevoked` audit and `membership.revoked.v1` outbox commit atomically. Targets with any other active grant are structurally ineligible, so this slice cannot mutate administrators or invoke unresolved last-admin policy. Live evidence confirms the revoked membership disappears and its existing browser session immediately loses capture authority. The complete `/v1/me` identity/preferences/active-context DTO, context-selection mutations, and broader role administration remain blocked. External SMTP/deployment acceptance and production role provisioning remain pending. The parent ticket remains Blocked because broader grant/revoke, `/me`, context selection and last-admin policy are unresolved.
