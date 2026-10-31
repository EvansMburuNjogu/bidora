# BID-AUTH-004 / BID-API-010S — Controlled tenant support-access decision packet

Status: **SELECTIVE POLICY APPROVAL — SESSION/EVIDENCE/FENCING DECISIONS PENDING; IMPLEMENTATION NOT AUTHORIZED**  
Scope: Phase 1 support cases, tenant-approved read-only diagnostic access and expiring support sessions  
Personas: Support Administrator diagnosing a tenant-reported problem; effective Organization Administrator approving minimum necessary access  
Not implementation authority: no schema, migration, handler, route, tenant-data access, session issuance, worker, deployment or frontend is approved by this draft alone.

## 1. Outcome and boundaries

This packet frames the recorded blockers for support scope, tenant approval, maximum duration and emergency behavior. The selective approval in section 17 closes only T1–T6, D1–D6 and B1 policy choices; session, evidence and fencing remain open.

Included:

- bounded support cases that contain no tender/document body or attachment;
- exact read-only diagnostic scope catalogue;
- Support Administrator request and tenant Organization Administrator approval/rejection;
- expiring support sessions bound to the requesting Platform Admin browser session;
- per-action authorization, tenant routing, immutable tenant-visible evidence and immediate revocation;
- safe lifecycle, concurrency, idempotency and HTTP behavior.

Excluded:

- standing tenant access or implicit Platform Owner access;
- impersonating a tenant user or minting tenant membership;
- write/mutation actions in tenant business modules;
- document bytes, pricing, proposal/response text, evidence content, review comments or AI prompt/output access;
- bulk search, export, download, screenshots or copying content into a support case;
- sealed-bid, procurement evaluation/opening/award or exchange data;
- privacy-request disposition, retention execution, hold release, tenant deletion or membership administration;
- emergency/break-glass access;
- frontend and production/deployment activation.

A support session is a constrained additional authorization fact. It never changes account type, membership, role, tenant placement or the underlying resource's workflow state.

## 2. Decision S1 — support cases

**Recommended policy:** Every access request belongs to one immutable-identity support case.

A case contains:

- case UUID and server-generated human reference;
- exact tenant UUID as a control-plane subject, never a database locator;
- source `TENANT_OPENED` or `PLATFORM_RECORDED_TENANT_CONTACT`;
- category from `ACCOUNT_ACCESS`, `MEMBERSHIP_DISPLAY`, `ORGANIZATION_PROFILE`, `OPPORTUNITY_WORKSPACE`, `DOCUMENT_PROCESSING`, `AI_PROCESSING`, `SUBSCRIPTION_USAGE`, `OTHER_DIAGNOSTIC`;
- bounded subject (1–200 UTF-8 bytes), safe structured symptom codes and an explicitly shared content-minimized summary (1–2,000 UTF-8 bytes);
- requester/recorder identity, timestamps, state and optimistic version.

Case states are `WAITING_FOR_TENANT_CONFIRMATION`, `OPEN`, `ACCESS_PENDING`, `IN_PROGRESS`, `WAITING_FOR_TENANT`, `RESOLVED` and `CLOSED`. State history is append-only.

Cases accept no attachment, URL, document content, pricing, credentials, tokens, secrets, personal identity evidence or arbitrary structured payload. The minimized summary is treated as tenant-confidential text under restricted case-participant access and an approved retention class, excluded from general audit/outbox/notifications and never treated as proof that secret detection is complete.

A Platform Administrator may record a case only after tenant contact; the source label must not imply the tenant used Bidora when they did not. The case remains `WAITING_FOR_TENANT_CONFIRMATION` until an effective Organization Administrator adopts that exact case. Recording a case creates no tenant access, approval or proof that the contact was authorized, and no access request may be created before adoption.

A tenant creator/effective Organization Administrator or assigned Support Administrator may propose resolution. Either side may close a case; closure invalidates pending/active access. A closed case is immutable and cannot reopen. Continuing work requires a linked new case, and closure never erases tenant-visible history.

## 3. Decision S2 — Phase 1 scope catalogue

**Recommended policy:** Phase 1 support access is read-only and exposes purpose-built diagnostic DTOs, not owner repositories or generic table access.

| Scope code | Required exact target | Permitted diagnostic fields | Explicit exclusions |
|---|---|---|---|
| `TENANT_CONFIGURATION_READ` | Tenant | tenant state, account type, country/reference codes, active schema compatibility and placement generation as redacted operational labels | DSN, cluster/database names, credentials, secret references |
| `MEMBERSHIP_SUMMARY_READ` | Tenant | bounded aggregate counts by membership state and capability-policy health | exact membership status, email, profile, grants, mutation |
| `ORGANIZATION_PROFILE_DIAGNOSTIC_READ` | Tenant singleton | profile existence/version, completeness field keys, letterhead active-version metadata/hash | field values, addresses, identifiers, contact values, branding bytes |
| `BID_WORKSPACE_DIAGNOSTIC_READ` | Exact bid UUID | existence, lifecycle state/version, blocker codes, counts and processing timestamps | title/buyer, pricing, responses, proposal prose, evidence, comments, document content |
| `DOCUMENT_PROCESSING_DIAGNOSTIC_READ` | Exact document-version UUID | media type, byte count, checksum-match boolean, scan/measurement/processing states, stable failure code, attempt counts/timestamps | filename when personal, object key, signed URL, bytes, extracted text, provider diagnostic body |
| `AI_JOB_DIAGNOSTIC_READ` | Exact AI-run UUID | operation kind, state, measured units/credits, reservation/checkpoint state, stable failure code, attempt/timing metadata | prompts, excerpts, suggestions, citations containing text, provider response/diagnostics |
| `SUBSCRIPTION_USAGE_DIAGNOSTIC_READ` | Tenant | plan key/term state, allowance totals and aggregate consumed/reserved counts | invoices, payment references, personal billing data, provider payloads |

A request contains one to five scope entries and at most twenty exact resource targets total. Duplicate/equivalent scopes are rejected canonically, and combined responses require classification review so individually safe fields cannot reconstruct excluded content or personal profiles. Tenant-wide scope is permitted only for the three explicitly tenant-level DTOs above. Wildcards, prefixes, arbitrary resource types, arbitrary action names, query filters and database/object locators are invalid.

Exact private-resource targets must be nominated by the tenant through ordinary authority or supplied as a server-issued opaque diagnostic reference. Support gets no title, filename, email, UUID-probing, list or search fallback for target discovery.

Every DTO, safe stable failure code and corruption behavior is module-owned and bound to an immutable schema/policy version. Unknown fields/codes fail closed. A material field/classification change creates a new policy version and requires fresh approval for new sessions. Raw provider, scanner, database and security diagnostics are never stable support codes.

Tenant approval and tenant-visible evidence remain mandatory for subscription/configuration DTOs whose source is control-plane data. The later evidence contract must not pretend that one tenant SQL transaction atomically covers a control read.

No support mutation is approved in Phase 1. Retrying a job, editing a profile, changing membership, releasing credits, altering a subscription or downloading content remains an ordinary owner-module operation performed by an independently authorized tenant/platform actor. A later write scope requires a new policy, separation-of-duty analysis and operation-specific evidence.

## 4. Decision S3 — support requester authority

**Recommended policy:** A support access request requires:

- verified, enabled `PLATFORM_ADMIN` identity and current auth epoch;
- exact platform context generation and current ACTIVE TOTP factor;
- TOTP AAL2 obtained no more than 15 minutes earlier;
- current `platform.support.access.request` from an ACTIVE `SUPPORT_ADMINISTRATOR` assignment;
- an OPEN/WAITING tenant-linked case assigned to that administrator, tenant-opened or explicitly adopted by an effective Organization Administrator;
- exact current ACTIVE tenant and server-owned assignment generation;
- UUID idempotency, quoted case version and a 1–2,000-byte purpose explaining why every requested scope is necessary.

The requester cannot approve the request. Platform Owner, role labels, account type, case assignment, tenant ID and routing metadata do not independently authorize access.

The request binds the exact Support Administrator identity, platform profile/context/auth epoch, case, tenant, assignment generation, scope set, purpose, requested duration and policy version. Material changes create a new request; an approver cannot broaden scope or change purpose.

## 5. Decision S4 — tenant approval authority

**Recommended policy:** Approval or rejection requires an **effective support approver**:

- verified and non-disabled identity;
- current `ACTIVE` membership in the exact `ACTIVE` Tenderpreneur tenant;
- matching membership context generation;
- explicit `tenderpreneur.organization.support_access.approve` ALLOW with same-capability DENY precedence;
- current TOTP factor and AAL2 no more than 15 minutes old;
- exact request version, Origin/CSRF and UUID idempotency.

This capability is source-defined authority, not a role label. Narrow case create/read/close and support-access approve/read/history semantics belong only to the future fixed effective Organization Administrator bundle and remain protected by final-effective-administrator/continuity rules. Broad `.manage` authority is rejected; exact final capability identifiers remain a contract decision before API/schema work.

One effective Organization Administrator with recent TOTP is sufficient for this read-only catalogue. The approver may reduce the requested duration or remove scopes/targets, but cannot add or alter them. Approval must retain at least one scope and include a 1–2,000-byte reason. Rejection includes a safe reason and issues no session eligibility. Future private-content, export, download or mutation scope requires a new maker/checker policy.

The approver cannot be the platform requester or act through support access. Enforce only authoritative system-known conflicts: the actor must have the correct current account type and no affected-tenant membership, pending invitation or support-session conflict. Bidora does not claim unimplemented cross-identity same-person detection. Platform administrators cannot approve on a tenant's behalf. A pending invitation, inactive member, role label or support case participant does not count as approval authority.

Every later action must recheck the exact original approving actor's current verified/enabled membership and effective approval capability. Authority loss invalidates the grant; another administrator cannot inherit it and must approve a new request.

## 6. Decision S5 — durations and activation window

**Recommended policy:** Exact Phase 1 bounds are:

- pending access request expiry: 24 hours after creation;
- approved-but-not-activated window: 15 minutes after approval;
- requested/granted active duration: 15, 30 or 60 minutes only;
- maximum absolute support-session lifetime: 60 minutes from activation and never beyond the approved grant expiry;
- idle timeout: 10 minutes;
- no renewal, extension or refresh.

A continuing investigation requires a new request and new tenant approval. Reapproval never resurrects an expired/revoked session or resets its evidence.

All comparisons use database time. Expiry is authoritative even if a cleanup worker has not run. Clock or dependency uncertainty denies access.

## 7. Decision S6 — support-session issuance and browser binding

**Recommended policy:** After tenant approval, the exact named Support Administrator activates the grant while their ordinary Platform Admin browser session remains current and TOTP AAL2 is no older than 15 minutes.

Activation:

- rechecks the support identity, assignment/capabilities, platform context/auth epoch, case/request/grant, tenant state, tenant assignment generation and approval authority snapshot;
- creates 32 random bytes from a cryptographic generator;
- stores only a domain-separated SHA-256 token hash plus session UUID;
- binds the support session to the current ordinary Platform Admin session UUID, user/auth epoch, platform context generation, tenant/assignment generation, exact approved scope and expiry;
- returns the raw token once in `__Host-bidora_support`, Secure, HttpOnly, SameSite=Strict, Path `/`, no Domain;
- rotates nothing into a tenant user session and creates no membership/capability grant.

Every support operation requires both the current ordinary Platform Admin session and the support cookie. An `X-Bidora-Support-Session-ID` UUID selects the support context but is not a bearer credential or authority. The operation takes no tenant context header because the exact tenant is server-derived from the support session.

The cookie is cleared after explicit end/revoke detection. Failure to clear it does not preserve server authority.

Only one ACTIVE support session per approved request and one ACTIVE support session per Support Administrator are allowed in Phase 1. Concurrent activation permits one winner.

## 8. Decision S7 — per-action authorization and routing

**Recommended policy:** Every diagnostic request, including repeated reads, rechecks before tenant access:

1. ordinary Platform Admin session, current auth epoch and platform context;
2. ACTIVE Support Administrator assignment and both `platform.support.session.use` plus the operation's diagnostic support policy;
3. exact support-token hash/session binding, idle and absolute expiry, and non-revoked request/grant/session;
4. current tenant ACTIVE state and exact current assignment generation;
5. current effective tenant approver authority and approval-not-revoked generation;
6. exact scope code/resource target/action match;
7. owner-module resource existence/classification through the assignment-bound tenant port.

The router resolves only the session's current exact server-owned assignment. A stale or moved assignment fails closed. An old-generation support request/session is never forwarded or automatically recreated in a replacement cell.

Out-of-scope or absent resources return concealed not-found behavior. There is no default tenant, broad list/search or repository fallback. Permission checks occur before DTO serialization, and DTOs are allowlisted per S2.

Support access is not passed to AI/model providers and cannot authorize downloads, exports, signed URLs or asynchronous jobs.

## 9. Decision S8 — tenant-visible action evidence

**Recommended policy:** No tenant-private diagnostic DTO may be returned unless its exact access event commits in that tenant database first.

The owner-module transaction:

- locks/rechecks a tenant-local projection of the exact support grant/session generation;
- records immutable event UUID, support case/request/session IDs, platform support actor ID, tenant ID, scope code, target UUID when applicable, action `READ_DIAGNOSTIC`, server timestamp, purpose hash/policy version and stable result;
- obtains the allowlisted diagnostic DTO in the same bounded transaction;
- commits before bytes are returned.

If evidence insert or commit is ambiguous, no DTO is returned. Repeating the read creates a new action event; reads are not hidden by idempotent replay because each disclosure is independently evidenced.

Tenant administrators with `tenderpreneur.organization.support_access.read` may list bounded request/grant/session summaries and action metadata. They see actor display identity, case reference, scope codes/targets, purpose, approval, start/end times and action timestamps/result codes, but no hidden platform security detail, raw token/hash, provider diagnostics or additional tenant content.

A minimal control-plane action receipt is delivered through outbox/inbox reconciliation after tenant commit. It cannot replace the tenant-local disclosure evidence or make an ambiguous tenant commit appear successful.

## 10. Decision S9 — revocation and invalidation

**Recommended policy:** A support session ends immediately in authorization terms when any of these occur:

- tenant approver or another current effective support approver revokes the grant;
- the Support Administrator explicitly ends their own session;
- a Platform Support or Security/Audit Administrator with `platform.support.session.revoke` revokes it;
- idle/absolute expiry;
- support identity disablement, auth-epoch change, role/capability loss, TOTP loss or platform-context advancement;
- tenant suspension/closure, assignment-generation change or incompatibility;
- approving membership/capability loss or approval-generation revocation;
- case closure;
- integrity conflict.

Tenant revocation requires current exact context, `tenderpreneur.organization.support_access.approve`, recent TOTP, reason, expected grant version and idempotency. Platform revocation requires current platform context, recent TOTP, reason, expected session version and idempotency. The actor cannot use the support session itself as revocation authority.

Revocation is checked from authoritative state on every action; caches cannot extend access. In-flight operations lock/recheck before disclosure. Revocation that commits before the tenant disclosure transaction's final check prevents disclosure. A disclosure already committed and returned cannot be recalled, which is why content/download scopes are excluded.

History is append-only. Revocation/expiry never deletes approval or action evidence.

## 11. Decision S10 — request/session lifecycle and replay

**Recommended policy:**

Access-request states:

- `PENDING_TENANT_APPROVAL`;
- `APPROVED_PENDING_ACTIVATION`;
- `REJECTED`;
- `CANCELLED`;
- `EXPIRED`;
- `ACTIVATED`;
- `ENDED`;
- `REVOKED`.

Support-session states:

- `ACTIVE`;
- `ENDED`;
- `REVOKED`;
- `EXPIRED`;
- `INTEGRITY_HOLD`.

Transitions use optimistic versions and immutable events. Request creation, approval/rejection, cancellation, activation, explicit end and revocation require operation-scoped UUID idempotency and canonical request hashes. Matching replay reauthorizes and returns the original safe result; conflicting reuse fails.

The requester may cancel only before activation. Cancellation cannot erase tenant approval. Ending or revoking an ACTIVE session atomically marks it unusable; cleanup is not required for denial.

Unknown, generation-ahead or internally inconsistent session facts enter `INTEGRITY_HOLD` and deny access. There is no automatic repair that widens scope or rewrites approval.

## 12. Decision S11 — emergency policy

**Approved policy (B1; implementation not authorized):** Phase 1 has no emergency or break-glass tenant-private access.

An incident may use approved platform operations such as disabling an identity, revoking sessions, pausing an integration or suspending a tenant, but those actions do not reveal tenant-private data. If no effective tenant approver is available, private support access is denied.

Future emergency access requires a separate threat model, accountable authority, two-person approval, narrower duration, customer notification, independent review and explicit non-waivable exclusions. This packet supplies no placeholder bypass, environment flag, database credential or operational workaround.

## 13. Decision S12 — HTTP contract groups

**Recommended policy:** OpenAPI may be drafted after S1–S13 approval.

### Tenant case and approval operations

- `POST /v1/tenderpreneur/support/cases` — authorized tenant actor opens a case.
- `GET /v1/tenderpreneur/support/cases` and `GET /v1/tenderpreneur/support/cases/{case_id}` — bounded own-tenant summaries.
- `GET /v1/tenderpreneur/support/access-requests` — tenant-visible pending/history list.
- `POST /v1/tenderpreneur/support/access-requests/{request_id}/approve` — reduce/approve exact scope and duration.
- `POST /v1/tenderpreneur/support/access-requests/{request_id}/reject`.
- `POST /v1/tenderpreneur/support/access-grants/{grant_id}/revoke`.
- `GET /v1/tenderpreneur/support/sessions/{session_id}/actions` — bounded tenant-visible action evidence.

Tenant case create/read/close and access approve/read/history require the approved narrow effective-Organization-Administrator semantics from S4. Exact final capability identifiers are intentionally pending; no broad `.manage` capability is approved.

### Platform Support Administration

- `GET /v1/platform/support/cases` and `GET /v1/platform/support/cases/{case_id}` — assigned/safe case views.
- `POST /v1/platform/support/cases/{case_id}/access-requests` — exact minimum scope request.
- `POST /v1/platform/support/access-requests/{request_id}/cancel`.
- `POST /v1/platform/support/access-grants/{grant_id}/activate` — issue bound support cookie.
- `POST /v1/platform/support/sessions/{session_id}/end`.
- `POST /v1/platform/support/sessions/{session_id}/revoke` — independently authorized revocation.
- `GET /v1/platform/support/sessions` — bounded safe active/history list.

### Diagnostic reads

Exact paths are owner-module operations under `/v1/support-diagnostics/...`; each path corresponds to one S2 DTO and accepts only the support-session selector plus exact resource UUID where required. There is no generic resource path, query language or proxy endpoint.

All writes use exact Origin/CSRF, strict JSON, UUID idempotency, quoted `If-Match` where mutable state exists and bounded reasons. Reads use exact Origin and private/no-store responses. Platform routes accept no tenant/cell/database selector. Tenant routes use ordinary membership context only for tenant actors, never for Support Administrators.

## 14. Decision S13 — safe failures, notifications and evidence

**Recommended policy:** Stable failures include:

- `support_request_invalid`;
- `support_not_authorized`;
- `support_case_not_found`;
- `support_scope_not_allowed`;
- `support_request_version_conflict`;
- `support_idempotency_conflict`;
- `support_state_conflict`;
- `support_approval_required`;
- `support_session_expired`;
- `support_session_revoked`;
- `support_context_stale`;
- `support_resource_not_found`;
- `support_integrity_hold`;
- `support_dependency_unavailable`.

Unauthorized/out-of-scope resources are concealed. Responses and logs exclude DSNs, database/object locators, raw tokens/hashes, private content, secret references and provider diagnostics.

Every state mutation commits minimal audit/outbox evidence in its authoritative plane. Cross-plane events carry opaque IDs, versions, scope codes and generations, never tenant content or bearer authority. Notifications are generic and contain no support token, private target label or case description. Notification failure does not change grant/session authority.

## 15. Implementation consequences after approval

Approval would authorize contract/schema planning for:

- control-plane support cases, immutable access requests/scopes, approvals, grants, hash-only bound support sessions, state events and command evidence;
- tenant-local support-grant projection and immutable disclosure-action evidence;
- source-defined tenant support case/read/approve capabilities;
- exact owner-module diagnostic DTO ports;
- strict OpenAPI drafts from S12;
- cross-plane projection/checkpoint contracts that fail closed on stale generations.

Approval would not authorize migrations, role/grant deployment, tenant data access, route mounting, actual support-cookie issuance, notifications, worker execution, frontend or production activation.

## 16. Deferred verification and acceptance

Later evidence must cover:

- two real tenant databases and foreign-ID/cross-tenant probes;
- scope matrix and DTO field-denial tests;
- self/platform approval bypass and forged tenant selector denial;
- 24-hour request, 15-minute activation, 10-minute idle and 60-minute absolute expiry boundaries;
- token theft without bound Platform session and Platform session rotation/revocation;
- tenant movement, stale assignment and incompatible schema denial;
- tenant approver capability loss and revocation/disclosure races;
- audit insert/commit ambiguity with zero returned DTO bytes;
- case close/session end and concurrent activation single winner;
- absence of content, locators, secrets and credentials in response/audit/outbox/logs;
- independent security/privacy review, tenant UAT and formal acceptance.

## 17. Decision status

T1–T6, D1 option 1, D2, D3 option 1, D4–D6 and B1 from `controlled-support-access-review.md` are approved as policy only. They establish tenant adoption/approval/closure, minimized case text, tenant-nominated exact targets, aggregate-only membership diagnostics, versioned module-owned DTOs/codes, bounded scope combinations, evidence requirements for control-origin diagnostics and no Phase 1 emergency access.

SS1–SS6, E1–E5 and R1–R5 remain pending a separate session/evidence/fencing decision. Therefore S5–S10 and the dependent HTTP/evidence portions of S12–S13 remain recommendations rather than an executable contract. Exact tenant capability identifiers also remain to be frozen. The requesting user explicitly directed that no implementation begin.

The bounded S1–S13 subjects are:

- **S1:** bounded content-minimized support cases.
- **S2:** exact read-only diagnostic scope catalogue; no private content/download/mutation.
- **S3:** assigned Support Administrator with recent TOTP requests exact scope.
- **S4:** capability-derived tenant approver with recent TOTP may only narrow/reject.
- **S5:** request expires in 24 hours; activation in 15 minutes; 15/30/60-minute grants; 10-minute idle; no renewal.
- **S6:** 32-byte hash-only support token bound to the ordinary Platform session; one active session.
- **S7:** every action rechecks platform, tenant, approval, assignment and exact resource scope.
- **S8:** tenant-local evidence commits before any diagnostic DTO is disclosed.
- **S9:** immediate authoritative revocation/invalidation with no cache grace.
- **S10:** immutable lifecycle, optimistic versions and reauthorized idempotent replay.
- **S11:** no Phase 1 emergency/break-glass access.
- **S12:** separate tenant, platform and owner-specific diagnostic APIs; no generic proxy.
- **S13:** concealed safe failures and content-minimal cross-plane evidence.

## 18. Tenant-approval, scope, session, evidence and revocation review

`controlled-support-access-review.md` records the selective policy approval. T1–T6, D1 option 1, D2, D3 option 1, D4–D6 and B1 are approved exactly as summarized in section 17, without implementation authority. Remaining decisions are exact tenant capability identifiers plus SS1–SS6, E1–E5 and R1–R5: cookie path, session concurrency/touch/token handling, truthful disclosure evidence, evidence visibility/retention/denials, authoritative revokers and cross-plane revocation fencing.

The existing statement that a control-plane revocation automatically wins a race inside a tenant SQL transaction is not approved: separate databases cannot provide that atomic observation. `controlled-support-session-evidence-fencing-decision.md` presents the separate SS1–SS6, E1–E5 and R1–R5 ballot. It corrects the cookie rule (`__Host-` requires `Path=/`) and recommends a single-use 15-second control permit, ten-second owner preparation deadline and final control commit serialized with revocation so evidence can distinguish `READ_PREPARED`, `RELEASE_AUTHORIZED` and `RELEASE_DENIED` without claiming browser receipt. The ballot remains unapproved. Until the pending decisions and a later implementation request are approved, no case, capability, access request, support cookie/session, diagnostic DTO, tenant read/evidence, route, worker or runtime access is authorized.
