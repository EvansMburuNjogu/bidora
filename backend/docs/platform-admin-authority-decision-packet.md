# BID-API-010P — Platform Admin authority and bootstrap decision packet

Status: **APPROVED POLICY/CONTRACT — BOOTSTRAP/AUTHORITY/INVITATION GOVERNANCE SOURCE IMPLEMENTED_NOT_VERIFIED**  
Scope: fixed Platform Admin authority, first-owner bootstrap and administrator lifecycle  
Persona: Platform Owner governing Bidora's platform administrators  
Not implementation authority: no schema, migration, handler, route, deployment command or web change is approved by this packet alone.

## 1. Outcome and boundaries

This packet closes only the policy needed before BID-AUTH-003 can implement Platform Admin assignments and before BID-API-010 can freeze administrator-lifecycle operations.

Included:

- nomination and one-time bootstrap rules for the first Platform Owner;
- six fixed roles and their source-defined capability bundles;
- role assignment, invitation, revocation, disable/reactivate and session effects;
- final-effective-owner protection;
- TOTP/AAL2 requirements;
- proposed maker/checker controls for the highest-risk platform changes.

Excluded:

- actual owner user ID, email or deployment credential;
- tenant membership or access to tenant-private data;
- support-session resource scopes and duration;
- settings schemas and provider secret values;
- retention periods, legal-hold release or destructive execution;
- refund thresholds, tenant decommission policy and production provider acceptance;
- Procurement authority and all frontend work.

Account type remains routing metadata. Every operation must authorize from current identity and platform-assignment facts; `PLATFORM_ADMIN` never grants authority by itself.

## 2. Proposed fixed role model

### 2.1 Assignment model

**Approved policy P1:** A Platform Admin may hold multiple fixed roles. Effective capabilities are the union of current ACTIVE assignments from the immutable versioned role catalogue. Phase 1 has no custom roles, user-authored capability grants or role editor. If a future explicit DENY mechanism exists, current DENY wins.

Each assignment has its own identity, fixed role key, state, version, grantor, activation/revocation evidence and context generation. Role labels are provenance; the backend authorizes the source-defined capabilities below.

### 2.2 Capability catalogue v1

| Fixed role | Exact capability bundle |
|---|---|
| `PLATFORM_OWNER` | `platform.admin.read`, `platform.admin.invite`, `platform.admin.role.assign`, `platform.admin.role.revoke`, `platform.admin.identity.disable`, `platform.admin.identity.reactivate`, `platform.admin.session.revoke`, `platform.audit.read`, `platform.audit.export`, `platform.governance.approve` |
| `OPERATIONS_ADMINISTRATOR` | `platform.tenant.read`, `platform.tenant.create`, `platform.tenant.provision.retry`, `platform.tenant.suspend`, `platform.tenant.reactivate`, `platform.migration.read`, `platform.migration.manage`, `platform.reference.read`, `platform.reference.manage`, `platform.maintenance.read`, `platform.maintenance.manage`, `platform.announcement.manage`, `platform.operations.health.read` |
| `BILLING_ADMINISTRATOR` | `platform.plan.read`, `platform.plan.manage`, `platform.plan.publish`, `platform.subscription.read`, `platform.subscription.manage`, `platform.invoice.read`, `platform.payment.read`, `platform.payment.reconcile`, `platform.billing.adjustment.request`, `platform.billing.export` |
| `INTEGRATION_ADMINISTRATOR` | `platform.integration.read`, `platform.integration.smtp.manage`, `platform.integration.smtp.test`, `platform.integration.smtp.activate`, `platform.integration.ai.manage`, `platform.integration.ai.test`, `platform.integration.ai.activate`, `platform.integration.pesapal.manage`, `platform.integration.pesapal.test`, `platform.integration.pesapal.activate`, `platform.integration.pause`, `platform.integration.rollback`, `platform.integration.failure.read` |
| `SUPPORT_ADMINISTRATOR` | `platform.support.case.read`, `platform.support.case.manage`, `platform.support.access.request`, `platform.support.session.use`, `platform.support.session.revoke` |
| `SECURITY_AUDIT_ADMINISTRATOR` | `platform.security.event.read`, `platform.audit.read`, `platform.audit.export`, `platform.admin.read`, `platform.admin.identity.disable`, `platform.admin.session.revoke`, `platform.governance.review`, `platform.support.session.read`, `platform.support.session.revoke`, `platform.retention.policy.read`, `platform.retention.policy.draft` |

Bundle limits:

- **2026-10-30 requesting-user revision of this line only:** the fixed `PLATFORM_OWNER` bundle includes the union of all existing fixed Platform Admin capability bundles, including billing, operations, integration, support and audit capabilities. This is not a wildcard over future/unmounted operations, tenant membership, tenant-private data, support-session consent or retention execution. Financial writes retain their billing-role maker and distinct-owner checker guards; a sole owner cannot approve their own change. The nominated bootstrap identity, fixed roles, MFA, continuity, session and audit rules below remain unchanged.
- `SUPPORT_ADMINISTRATOR` has no standing tenant access. `platform.support.session.use` is inert without a separate approved, tenant-authorized, unexpired support session.
- `SECURITY_AUDIT_ADMINISTRATOR` may draft but cannot activate retention policy, release a legal hold or run destructive processing under this packet.
- Billing adjustments are requests only. Refund execution remains blocked until exact financial authority and thresholds are approved.
- Tenant decommission, destructive retention, legal-hold release, MFA bypass/reset and secret-value read capabilities do not exist in catalogue v1.
- Secret settings use references only; no role can read credential values through Bidora.

## 3. First Platform Owner bootstrap

### 3.1 Recommended bootstrap mechanism

**Approved policy P2:** Use a one-time, deployment-operated backend command with a dedicated bootstrap database credential that is unavailable to the API runtime. Do not expose a public or ordinary authenticated bootstrap endpoint, inject identity data through a migration, or permit ambient administrator discovery.

The deployment operator supplies the nominated user's immutable UUID through a protected deployment input. The identifier is not committed to the repository or written to general logs. The command transaction must lock the singleton bootstrap state and target identity, then atomically:

1. prove bootstrap has never succeeded and no Platform Owner assignment history exists;
2. prove the target user exists, is verified and not disabled;
3. prove `account_type` is `UNASSIGNED`;
4. prove the user has no tenant membership and no active tenant invitation;
5. prove the user has an ACTIVE TOTP factor;
6. set the immutable account type to `PLATFORM_ADMIN`;
7. create one ACTIVE `PLATFORM_OWNER` assignment and context generation 1;
8. mark bootstrap consumed permanently;
9. advance the user's `auth_epoch`, revoke existing sessions, and append minimal audit/outbox evidence.

The target signs in again and completes TOTP to obtain AAL2. Bootstrap returns no session, recovery code, TOTP secret or personal profile data.

Failures reveal no user details in ordinary output. A committed bootstrap can never be rerun or transferred; correction requires a separately approved owner lifecycle, not resetting the marker.

### 3.2 Nomination boundary

The requesting user approves the policy, not a real identity value. The accountable deployment owner must nominate the immutable verified user ID out of band before execution and record human authorization in deployment evidence. Email matching alone cannot nominate or link the owner.

## 4. Administrator invitation and activation

**Approved policy P3:** Only an effective Platform Owner with recent TOTP AAL2 may invite or mutate Platform Admin roles.

Invitation flow:

1. Owner selects an email and one fixed role.
2. Server stores only the approved invitation facts and a hash of a 32-byte single-use token.
3. Invitation is bounded to 72 hours, one active platform invitation per normalized email, with resend invalidating the prior token.
4. Recipient authenticates or registers, verifies the exact invited email, and must remain `UNASSIGNED`, enabled, without tenant membership or active tenant invitation.
5. Recipient enrolls and confirms TOTP before acceptance.
6. Acceptance atomically consumes the invitation, sets `PLATFORM_ADMIN`, activates the selected role, advances `auth_epoch`, revokes prior sessions and records audit/outbox evidence.
7. Recipient signs in again with TOTP AAL2.

A platform invitation cannot link by matching email alone, convert an established Tenderpreneur/Procurement account, create tenant membership, or grant support access.

After a user is already a Platform Admin, an effective Owner may add another fixed role without email invitation, subject to the grant rules below.

## 5. Grant, revoke and identity-state boundaries

**Approved policy P4:** Apply all of these rules:

- Every role mutation requires expected assignment/user/context versions, UUID idempotency, exact Origin/CSRF and TOTP AAL2 assured no more than 15 minutes earlier.
- The actor cannot grant a role to themselves. Checked voluntary relinquishment may be permitted only for a non-final owner under the maker/checker rule below.
- The maker and checker must be distinct. The assignment subject cannot act as checker and may act as maker only for that checked voluntary relinquishment.
- The role catalogue is source-defined; owners cannot edit bundle contents.
- Grant/revoke, disable/reactivate and privilege-sensitive invitation acceptance atomically advance the subject's platform context generation and `auth_epoch`, revoke all subject sessions, and append audit/outbox evidence.
- An administrator must sign in again and satisfy TOTP after every privilege or enabled-state change.
- Revocation is historical and append-only; reactivation creates a new assignment version rather than erasing history.
- Disable denies all Platform Admin operations and routing even if assignments remain recorded.
- API requests recheck verified/non-disabled identity, immutable account type, ACTIVE assignment, capability, context generation, current auth epoch, active TOTP and required assurance. Request IDs, role labels, account type and session age never independently authorize.
- No Platform Admin role can select a tenant database or bypass tenant membership/support-session authorization.

## 6. Final-effective-owner invariant

An **effective Platform Owner** is a verified, non-disabled `PLATFORM_ADMIN` identity with an ACTIVE `PLATFORM_OWNER` assignment and ACTIVE TOTP factor.

**Approved policy P5:** Serialize every owner revoke, owner disable and any future factor-removal operation against the effective-owner set. A transaction must leave at least one effective owner after commit. Concurrent attempts that could remove the final owner permit at most one safe result; the unsafe operation returns a stable conflict. Session state and a pending invitation do not count as continuity.

Operational target: maintain at least two effective owners, but enforce a hard minimum of one. A warning is not authority and does not relax the hard invariant.

## 7. Highest-risk maker/checker policy

### 7.1 Recommended rule

**Approved policy P6:** Require an independent eligible checker under the exact owner/security boundaries below for:

1. assigning or revoking `PLATFORM_OWNER` once at least two effective owners exist;
2. disabling or reactivating a Platform Owner once at least two effective owners exist;
3. activating or rotating a production SMTP, OpenAI or PesaPal endpoint/credential-reference version;
4. publishing a platform plan price/allowance version;
5. applying an approved billing adjustment or refund once the financial policy defines executable limits;
6. scheduling tenant decommission, releasing a legal hold, activating destructive retention or authorizing destructive execution once those policies are separately approved.

The maker and checker must be distinct and use TOTP AAL2 no older than 15 minutes. For integration, commercial, decommission and retention changes, the maker needs the operation-specific capability and the checker must be an effective Platform Owner with `platform.governance.approve`. For Platform Owner role or identity lifecycle, the maker must hold the applicable `platform.admin.role.assign`, `platform.admin.role.revoke`, `platform.admin.identity.disable` or `platform.admin.identity.reactivate` capability; the checker may instead be a distinct effective Security and Audit Administrator with `platform.governance.review`, but at least one participant must be an effective Platform Owner. The affected administrator cannot check the action; they may make only their own checked voluntary non-final-owner relinquishment. Both participants bind the exact immutable proposed version. Material edits invalidate prior approval. Approval expires after 30 minutes if execution has not committed. Execution rechecks both participants, exact version, current policy and all domain guards atomically.

### 7.2 Sole-owner bootstrap exception

The initial owner may invite exactly the first additional Platform Owner without a second owner because no checker exists. This continuity-increasing exception requires recent TOTP AAL2, a distinct recipient, the normal verified/TOTP invitation flow and immutable audit evidence. While fewer than two effective owners exist, all other maker/checker operations above fail closed rather than falling back to one-person approval.

After two effective owners exist, the exception closes permanently; later temporary loss of an owner does not reopen it. Restoring continuity then requires the remaining owner plus a separately approved break-glass recovery policy. No break-glass or MFA bypass is authorized in Phase 1 by this packet.

## 8. Authorized API contract groups

The requesting user approved P1–P6 and authorized contract drafting, but not implementation, for:

- one deployment-only bootstrap command contract, not an HTTP route;
- `POST /v1/platform/administrators/invitations`;
- invitation acceptance through an authenticated platform-specific operation;
- `GET /v1/platform/administrators` and safe assignment history;
- fixed-role assignment and revocation commands;
- identity disable/reactivate and session revocation commands;
- maker/checker proposal, approval and execution primitives shared by later typed settings.

Responses must omit email where not required, all token/hash/factor/secret values, tenant-private data and database locators. Pagination is bounded and deterministic. Every mutation has explicit idempotency, expected version, audit, outbox, assurance and session-effect semantics.

## 9. Deferred implementation evidence

Even after policy approval, source remains incomplete until separately authorized migrations, contracts, application code and centralized tests exist. Required later evidence includes:

- clean and upgrade migrations plus runtime grants;
- zero/one/two-owner concurrency and final-owner races in real PostgreSQL;
- bootstrap single-winner and permanent replay denial;
- invitation resend, expiry, cross-account and competing-acceptance races;
- self-grant, stale context, stale assurance and forged-role denial;
- exact-version maker/checker invalidation and expiry;
- auth-epoch/session revocation after every privilege change;
- proof that no role gains tenant-private access;
- API/browser security, independent security review, UAT and formal acceptance.

## 10. Decision record

The requesting user explicitly approved P1–P6 and requested the Platform Admin lifecycle API contract. The approved policies are:

- **P1:** multiple fixed roles; union of source-defined bundles; no custom grants.
- **P2:** deployment-only one-time bootstrap using immutable user UUID and dedicated credential.
- **P3:** verified-email, no-tenant-ties, active-TOTP invitation activation policy.
- **P4:** owner-only, no-self-grant, recent-TOTP role lifecycle with auth-epoch/session invalidation.
- **P5:** hard minimum one effective owner; operational target two.
- **P6:** exact maker/checker list, 30-minute approval expiry and continuity-only first-second-owner exception.

The requesting user subsequently approved `api/openapi/drafts/platform-administrator-lifecycle.json` and authorized its bounded control-plane persistence slice. Control migration `00015_platform_administration.sql`, module-owned sqlc queries/generated artifacts, the source-defined role domain and pgx bootstrap/authority adapters are now `IMPLEMENTED_NOT_VERIFIED`. The migration has not run. Actual initial-owner UUID, deployment credential, production secret manager, HTTP lifecycle orchestration, maker/checker execution, bootstrap executable and deployment execution remain separately gated. BID-AUTH-003C subsequently implemented application-owned invitation issue/cancel/accept sequencing, owner-module transaction adapters and provider-neutral minimal HTML/text invitation/reminder templates. BID-AUTH-003D subsequently added exact owner-invitation approval/rejection, current maker/checker/continuity rechecks and approval-time token delivery preparation. BID-AUTH-003E then added direct non-owner fixed-role assignment/revocation and exact pending Platform Owner role proposals with session invalidation and continuity guards; BID-AUTH-003F subsequently implemented exact owner-role approval/rejection execution with current continuity, version, session and affected-subject rechecks; live race evidence remains deferred. SMTP/runtime composition, reminder cadence, migration execution and live race/rollback evidence remain deferred.

## Privacy catalogue-v2 amendment — approved policy, not implemented

The requesting user approved the narrow `PRIVACY_ADMINISTRATOR` role and capability bundle defined by D4 in `privacy-request-contract-decision-packet.md`, together with Privacy Administrator maker/distinct Platform Owner checker notice governance, affected-requester/tenant-affiliation denial and checked organization/high-sensitivity export release. This is policy for a future catalogue version 2 only. Catalogue v1, migration `00015`, generated/source role bundles and runtime authority remain unchanged. No privacy role assignment, capability, disposition, export, route or worker exists. L1/L3 and all export/correction/restriction/destructive-effect matrices remain gates.
