# BID-API-010D — Privacy notice and request-lifecycle decision packet

Status: **SELECTIVE POLICY APPROVAL — D1–D12 FULL CONTRACT AND IMPLEMENTATION REMAIN BLOCKED**  
Scope: Phase 1 privacy notices and access/export/correction/deletion/restriction request contracts  
Personas: an authenticated person exercising rights over their account, and an effective Organization Administrator acting for their Tenderpreneur organization  
Not implementation authority: no destructive processing, retention period, legal-hold mutation, schema, migration, handler, worker, route or frontend is approved by this draft alone.

## 1. Outcome and boundaries

This packet defines safe intake, authorization, scoping, disposition, status and evidence behavior for privacy requests without claiming that deletion or export has occurred.

Included:

- immutable privacy notices and acknowledgement evidence;
- individual account access, portable export, correction, restriction and deletion/anonymization requests;
- organization export and closure/deletion requests by an effective Organization Administrator;
- requester verification, current-authority rechecks and cross-plane scope snapshots;
- partial approval/denial and retained-category explanations;
- asynchronous export and deletion checkpoint semantics;
- cancellation, concurrency, idempotency, audit and safe errors;
- a proposed accountable Platform Privacy Administrator role.

Excluded:

- exact statutory response periods, lawful bases and retention durations;
- applying or releasing legal holds;
- deciding whether a particular record must legally be retained;
- deleting held, disputed, financial, security, audit or submission evidence without approved policy;
- silently changing authoritative tenant data through a generic correction endpoint;
- third-party, guardian, estate, regulator or unauthenticated lost-account requests;
- support-session initiation or use;
- backup destruction or restore cutover;
- frontend and deployment work.

A submitted request is an acknowledgement of receipt, not proof of entitlement, approval, export completion, erasure or legal compliance.

## 2. Decision D1 — immutable privacy notices

**Recommended policy:** Privacy notices are immutable, versioned control-plane records with:

- notice UUID and monotonically increasing version;
- locale and audience (`ACCOUNT_USER` or `ORGANIZATION_ADMINISTRATOR`);
- rendered title/body hash and an immutable object/reference to reviewed content;
- identified purposes, data categories, processor/controller statement, subprocessors, AI processing disclosure, rights channel and effective UTC instant;
- product/privacy/legal reviewer evidence;
- `DRAFT`, `SCHEDULED`, `ACTIVE`, `SUPERSEDED` or `WITHDRAWN` state.

Only one notice per audience/locale may be active. Activation and rollback preserve history and never change the text of an existing version.

Acknowledgement records the user ID, exact notice UUID/version, server timestamp, interaction type and policy version. It contains no IP, user-agent, inferred location or copied notice text. Notice acknowledgement and consent are different evidence types. No generic consent toggle exists. If a later purpose relies on consent, its purpose-specific grant, withdrawal, effect timing and proof require a separate approved lawful-basis policy.

Notice scheduling, activation and rollback require a Privacy Administrator maker and distinct effective Platform Owner checker. Both bind the exact immutable content hash, locale/audience, purpose matrix and external legal/privacy validation evidence with fresh TOTP; approval expires after 30 minutes and execution rechecks both. Withdrawal of a never-active draft may remain single-actor. A changed notice may create a `NOTICE_ACKNOWLEDGEMENT_REQUIRED` action, but cannot silently suspend access to mandatory account/privacy operations. Whether ordinary product use must pause is separately determined per notice version by the approved legal/product policy.

## 3. Decision D2 — supported requester operations

**Recommended policy:** Phase 1 accepts these exact request types:

### Individual account scope

- `ACCOUNT_ACCESS` — a human-readable inventory of the requester's account data categories and processing purposes;
- `ACCOUNT_PORTABLE_EXPORT` — a machine-readable export of portable data the requester is entitled to receive;
- `ACCOUNT_CORRECTION` — a request to correct identified account/profile facts;
- `ACCOUNT_RESTRICTION` — a request to restrict eligible processing;
- `ACCOUNT_DELETION` — account deletion/anonymization subject to membership continuity, lawful retention and holds.

The target identity is always derived from the authenticated session. A request cannot supply another user ID or email.

### Tenderpreneur organization scope

- `ORGANIZATION_EXPORT` — export organization-controlled data within the actor's current authority and applicable third-party restrictions;
- `ORGANIZATION_CLOSURE_DELETION` — closure followed by approved recovery/retention/deletion checkpoints.

The target tenant is derived from current membership authorization and server-owned routing. Headers select context only and never grant authority or identify a database.

Phase 1 does not accept arbitrary request-type strings, free-form target IDs or requests spanning multiple tenants.

Authenticated self-service is only one intake channel. Lost-account, former-user, representative, guardian, estate and regulator requests use an externally governed privacy channel with separate identity/authority verification; that channel grants no account or tenant access and requires separately approved evidence-retention rules. Objection also remains in that external channel until an exact objection-eligible purpose/effect matrix is approved; it is not silently treated as generic restriction.

## 4. Decision D3 — requester authentication and tenant authority

**Recommended policy:** All request creation requires:

- a verified, non-disabled identity and current browser session/auth epoch;
- exact approved HTTPS Origin and session-bound CSRF;
- fresh reauthentication completed no more than 15 minutes earlier using the account's current approved authentication method;
- a UUID idempotency key;
- an exact active privacy-notice acknowledgement where policy marks acknowledgement required.

Individual requests need no tenant context and confer no tenant authority. Membership in a tenant does not permit requesting another person's account export or deletion.

Organization requests additionally require:

- an `ACTIVE` Tenderpreneur tenant and exact active membership/context generation;
- current deny-first `tenderpreneur.organization.privacy.manage` authority, approved as policy only for the fixed effective Organization Administrator bundle and limited to request creation/status/cancellation;
- current TOTP AAL2 no more than 15 minutes old;
- the approved continuity definition: a verified, non-disabled `ACTIVE` member whose authority survives deny precedence;
- final-administrator and active-work continuity checks before closure can advance.

`PLATFORM_ADMIN`, account type, role labels, support access, tenant IDs and routing generations never substitute for these checks. A support session cannot create, approve, cancel or widen a privacy request.

If the requester loses relevant identity or organization authority, pending processing pauses for reassessment. Previously collected evidence remains immutable, but no new tenant-private data is read under stale authority.

## 5. Decision D4 — accountable platform privacy authority

The approved Platform Administration catalogue currently has six fixed roles and no capability that permits privacy-request disposition. Platform Owner is not a wildcard, Security and Audit Administrator can only draft retention policy, and Support Administrator has no standing tenant access.

**Approved policy amendment, implementation not authorized:** Add a seventh fixed role in catalogue version 2:

`PRIVACY_ADMINISTRATOR`

with only:

- `platform.privacy.notice.read`;
- `platform.privacy.notice.manage`;
- `platform.privacy.request.read`;
- `platform.privacy.request.scope`;
- `platform.privacy.request.disposition`;
- `platform.privacy.export.release`.

Limits:

- The role grants no standing tenant database access, support-session use, legal-hold apply/release, retention-policy activation, destructive execution, billing access or identity credential access.
- Scope discovery and execution occur through owner-module ports and server-owned assignments, not arbitrary SQL or database selection.
- Disposition fails closed when the actor is the requester or has an affected-tenant membership, pending invitation, organization role or active support-session context.
- `ACCOUNT_DELETION` and `ORGANIZATION_CLOSURE_DELETION` approval requires a distinct effective Platform Owner with `platform.governance.approve`, recent TOTP AAL2 and exact immutable scope/policy versions. Maker and checker differ; neither may be the requester or have an affected-tenant affiliation.
- Access, correction and restriction disposition may require one eligible Privacy Administrator with recent TOTP AAL2 only after their exact lawful-basis/effect matrices are approved.
- Organization export and any account export containing tenant-controlled or high-sensitivity categories require a Privacy Administrator maker plus distinct effective Platform Owner checker, followed by independent requester reauthentication. Export remains unavailable until E1–E5 are approved.

This is an approved policy amendment to the six-role catalogue, but the user has not authorized catalogue implementation. No platform disposition endpoint or processing worker may be activated; even intake/status implementation remains separately gated by the unresolved L1/L3 and applicable data/effect decisions.

## 6. Decision D5 — lifecycle and immutable scope

**Recommended policy:** Request states are:

- `RECEIVED`;
- `IDENTITY_VERIFICATION_REQUIRED`;
- `SCOPING`;
- `LEGAL_OR_HOLD_REVIEW`;
- `PENDING_DISPOSITION`;
- `APPROVED`;
- `EXECUTING`;
- `VERIFYING`;
- `COMPLETED`;
- `PARTIALLY_COMPLETED`;
- `DENIED`;
- `CANCELLED`;
- `FAILED_REVIEW_REQUIRED`.

A request maintains append-only state-transition evidence. Mutable projection versioning is optimistic and cannot erase history.

Scoping produces an immutable scope version containing only bounded category/system counts and opaque checkpoint identifiers. It does not copy raw documents, pricing, credentials, email bodies or provider payloads into the control database. Every rescope creates a new version and invalidates an earlier disposition.

A disposition binds:

- exact request and scope versions;
- requester identity/tenant authority facts;
- active privacy notice and approved legal/retention policy versions;
- discovered legal-hold generation;
- approved, denied and retained category sets;
- safe reason codes and requester-facing explanation;
- maker/checker evidence where D4 requires it.

Any change in scope, requester authority, hold generation, retention policy, tenant status or assignment generation before execution invalidates or pauses the disposition. Execution never widens scope automatically.

## 7. Decision D6 — disposition and partial outcomes

**Recommended policy:** Every scoped category receives one of:

- `APPROVED_FOR_ACCESS`;
- `APPROVED_FOR_EXPORT`;
- `APPROVED_FOR_CORRECTION_WORKFLOW`;
- `APPROVED_FOR_RESTRICTION`;
- `APPROVED_FOR_ANONYMIZATION`;
- `APPROVED_FOR_DELETION`;
- `RETAINED_LEGAL_OR_REGULATORY`;
- `RETAINED_ACTIVE_HOLD`;
- `RETAINED_SECURITY_OR_DISPUTE`;
- `OUTSIDE_REQUESTER_AUTHORITY`;
- `NOT_HELD_BY_BIDORA`;
- `REVIEW_REQUIRED`.

A request may finish `PARTIALLY_COMPLETED` only when every category has a terminal verified outcome and at least one eligible category could not be completed. It must explain retained categories at a safe class level without exposing another person, legal strategy, security investigation or tenant-private record.

`DENIED` means no requested effect was approved. `COMPLETED` must not be used while an external provider, object, index, cache, job or database checkpoint remains ambiguous or unverified.

No endpoint promises a universal statutory completion date. Responses return an approved jurisdiction-policy due date only when one exists; otherwise it is `null` and the UI must not invent one.

## 8. Decision D7 — correction and restriction semantics

**Recommended policy:** Privacy APIs coordinate; they do not bypass owner-module invariants.

- `ACCOUNT_CORRECTION` identifies a bounded field/category and proposed corrected value or supporting note. Approved corrections invoke the existing Identity/Organization owner command with its ordinary validation, expected version, audit and downstream invalidation rules.
- Immutable audit, approval, submission and financial evidence is corrected by append-only superseding evidence where policy permits, never overwritten.
- A correction request may be completed by directing the requester to an existing self-service authoritative operation; completion records that operation's evidence ID rather than duplicating the write.
- Restriction is category/purpose-specific. It does not mean account deletion and cannot disable mandatory security, fraud, audit or legal processing without approved policy.
- Once an approved restriction checkpoint commits, retrieval, AI indexing/provider use and new nonessential processing for that category fail closed immediately. Physical cleanup follows durable checkpoints where required.

AI cannot approve a request, determine a lawful basis, release a hold or decide that processing is necessary.

## 9. Decision D8 — export behavior

**Recommended policy:** Exports are asynchronous, classification-aware and tenant-isolated.

An export:

- reauthorizes requester and exact scope at generation and download;
- uses owner-module readers through server-owned routing;
- contains a versioned manifest with included categories, source record IDs/versions where disclosable, file hashes, generation time and omissions at safe category level;
- excludes passwords, PHC strings, session/CSRF/token hashes, MFA secrets/recovery codes, secret references, signed URLs, internal database/object locators, another person's non-disclosable data and raw provider diagnostics;
- applies spreadsheet-formula and archive/path traversal protections;
- is stored as an immutable encrypted private object with an opaque key;
- has no public bucket access and is never attached to email.

After successful generation, the requester receives a short-lived authorized download operation, not an object key. Recommended Phase 1 availability is 24 hours from release, after which retrieval is denied and physical object deletion is reconciled. Every download uses fresh current authorization and reauthentication no older than 15 minutes. A release or download failure never exposes a partial archive.

`ACCOUNT_ACCESS` may return a bounded human-readable inventory directly only when it contains no bulk private content; otherwise it uses the same export workflow.

## 10. Decision D9 — deletion/anonymization and organization closure

**Recommended policy:** Destructive execution remains disabled until the legal/privacy owner approves exact policy classes and BID-SEC-004 verification gates pass.

Before that gate, the system may only receive a deletion request, verify identity, create immutable scope and enter `LEGAL_OR_HOLD_REVIEW`. It exposes `execution_available:false`; it cannot assign `APPROVED_FOR_DELETION` or `APPROVED_FOR_ANONYMIZATION`, transition the request to `APPROVED`/`EXECUTING`, or claim completion.

When separately authorized, execution is a durable cross-plane operation:

1. atomically recheck exact disposition, requester authority, policy and hold generation;
2. deny new retrieval/processing for approved categories through revocation/tombstone checkpoints;
3. revoke affected sessions and stale/cancel affected queued jobs;
4. process identity/control data, the exact tenant database, private objects, FTS/vector entries, caches, notifications/support attachments and permitted provider deletion effects;
5. verify every checkpoint independently;
6. reconcile deletion tombstones against restores before any restored target can serve traffic;
7. append completion evidence without retaining deleted content.

There is no distributed SQL transaction. Retries are idempotent and effect-specific. Ambiguous effects remain nonterminal for reconciliation. Active holds and ineligible classes are never deleted to make a request appear complete.

Organization closure additionally requires the approved final-administrator and ownership-continuity policy, no automatic successor selection, and an approved closure/recovery-window policy. This packet does not approve the proposed 90-day closure window.

## 11. Decision D10 — cancellation, concurrency and replay

**Recommended policy:**

- A requester may cancel while a request is `RECEIVED`, `IDENTITY_VERIFICATION_REQUIRED`, `SCOPING`, `LEGAL_OR_HOLD_REVIEW` or `PENDING_DISPOSITION`.
- After approval, cancellation becomes a new reviewed request to stop only effects that have not committed. It cannot reverse immutable audit, a completed disclosure, anonymization or deletion.
- One open request per requester/scope/type is allowed. A materially identical submission replays or returns the existing request; a conflicting idempotency reuse fails.
- Request creation, cancellation, disposition, export release and destructive approval use operation-scoped UUID idempotency and canonical request hashes.
- State changes require quoted `If-Match`; stale versions conflict.
- Worker claims use database time, non-renewable UUID leases, bounded retries and exact-attempt fencing. Cancellation does not reclaim an active lease early; expiry recovery owns retry.
- A second request cannot widen, reset or erase an earlier request's evidence.

Exact worker lease durations and retry schedules require an implementation-specific decision before workers are mounted; they are not invented by this contract.

## 12. Decision D11 — HTTP contract groups

**Recommended policy:** OpenAPI may be drafted after D1–D12 approval for these operations.

### Notices

- `GET /v1/privacy/notices/current` — current notices applicable to the authenticated user.
- `POST /v1/privacy/notices/{notice_id}/acknowledge` — exact-version acknowledgement.

### Requester operations

- `POST /v1/privacy/requests` — create one individual request; target identity comes from session.
- `POST /v1/tenderpreneur/privacy/requests` — create one organization request; target tenant comes from authorized context.
- `GET /v1/privacy/requests` — current requester's bounded list.
- `GET /v1/privacy/requests/{request_id}` — safe status, category-level disposition and checkpoint summary.
- `POST /v1/privacy/requests/{request_id}/cancel` — bounded cancellation.
- `POST /v1/privacy/requests/{request_id}/export-release` — obtain a short-lived authorized download response after reauthorization; no raw object key.

### Platform Privacy Administration

- `GET /v1/platform/privacy/requests` and `GET /v1/platform/privacy/requests/{request_id}` — safe queue/detail with no raw tenant content.
- `POST /v1/platform/privacy/requests/{request_id}/scope` — create exact immutable scope through owner ports.
- `POST /v1/platform/privacy/requests/{request_id}/dispositions` — approve, partially approve or deny exact categories.
- `POST /v1/platform/privacy/requests/{request_id}/approve-destructive` — distinct Platform Owner checker for exact destructive disposition.
- `POST /v1/platform/privacy/requests/{request_id}/release-export` — confirm exact completed export is eligible for requester release.

All writes require strict JSON, exact Origin/CSRF, UUID idempotency, quoted `If-Match` where state exists and bounded bodies. Reads are private/no-store with stable bounded pagination. Platform routes accept no tenant, cell or database selector.

Request bodies permit only enumerated request type, bounded category/field keys and an optional 1–2,000-byte reason/clarification. They reject arbitrary SQL-like filters, object keys, database names, storage paths, URLs and provider instructions.

## 13. Decision D12 — safe errors, notifications and evidence

**Recommended policy:** Stable failures include:

- `privacy_request_invalid`;
- `privacy_request_not_authorized`;
- `privacy_request_not_found`;
- `privacy_request_version_conflict`;
- `privacy_request_idempotency_conflict`;
- `privacy_request_state_conflict`;
- `privacy_notice_acknowledgement_required`;
- `privacy_request_verification_required`;
- `privacy_request_scope_stale`;
- `privacy_request_hold_review_required`;
- `privacy_request_execution_unavailable`;
- `privacy_request_dependency_unavailable`.

Unauthorized reads conceal existence. Error responses exclude requested content, legal-hold details, internal policy analysis, provider diagnostics and locators.

Committed mutations atomically record minimal audit/outbox evidence: request/scope/disposition IDs and versions, actor/requester IDs, action, category keys, policy/hold generations, stable outcome, correlation and timestamps. Free-form requester text, corrected values, exported content and legal advice are excluded from general audit/outbox payloads.

Notifications are generic status notices delivered through approved authenticated channels. Email contains no export attachment, download token, private category detail, legal-hold detail or deletion evidence. A notification failure does not roll back the authoritative request state and must not be reported as request failure.

## 14. Implementation consequences after approval

Selective policy approval does not authorize contract or source implementation. A later separately bounded non-destructive proposal could consider immutable notices/acknowledgements and request receipt/status/scoping only after L1/L3 and the applicable authority/data matrices are approved.

No Platform role/capability catalogue change, schema, migration, query, OpenAPI, request route, notice activation, scope worker, export, correction/restriction effect, destructive SQL/object/index/provider effect, legal-hold mutation, active retention schedule, external notification, frontend or production activation is authorized.

## 15. Deferred verification and acceptance

Later evidence must cover:

- requester/target substitution and cross-tenant denial;
- platform self-disposition and support-session bypass denial;
- stale authority, notice, scope, hold, policy and assignment races;
- exact maker/checker separation for destructive approval;
- partial category disposition and safe retained-class wording;
- export manifest correctness, formula/archive safety, expiration and repeated authorization;
- crash/ambiguous-effect checkpoint recovery without widening scope;
- database/object/FTS/vector/cache/job/provider convergence;
- legal-hold and deletion races;
- restore reconciliation before cutover;
- audit/outbox atomicity and private-data absence;
- privacy/legal/security review, synthetic UAT and formal release acceptance.

## 16. Requested decision

Approve or amend this bounded set:

- **D1:** immutable notices; acknowledgement is not automatically consent.
- **D2:** exact individual and organization request types; no arbitrary targets.
- **D3:** current session plus fresh reauthentication; tenant privacy capability and recent TOTP for organization scope.
- **D4:** add fixed `PRIVACY_ADMINISTRATOR` in role catalogue v2 with narrow capabilities and destructive maker/checker.
- **D5:** append-only lifecycle and immutable category scope/disposition versions.
- **D6:** explicit partial outcomes and no invented statutory deadline.
- **D7:** correction/restriction orchestrate owner modules and cannot rewrite immutable evidence.
- **D8:** reauthorized private exports available for 24 hours with safe manifests.
- **D9:** destructive execution remains disabled pending legal policy and BID-SEC-004 gates.
- **D10:** bounded cancellation, one open request/type/scope and exact replay/concurrency rules.
- **D11:** requester and Platform Privacy Administration API groups.
- **D12:** stable safe errors, generic notifications and content-minimal evidence.

## 17. Lawful-basis, authority and effect review

`privacy-request-contract-review.md` records selective approval of L2, L4 option 1, external-channel objection, A1 option 1, A2–A4, A5 option 1 and X1. These approve policy boundaries for notice-versus-consent, exceptional requester/objection channels, narrow Platform and tenant capabilities, notice governance, affiliation denial, checked sensitive export release and intake-only deletion scoping.

D1–D12 as a complete executable contract remain pending. L1 controller/processor/purpose/lawful-basis and L3 jurisdiction/deadline policy remain unresolved, as do E1–E5 exports, C1–C4 corrections, R1–R4 restrictions and X2–X7 destructive effects. Export generation/release, correction effects, restriction effects, deletion/anonymization approval, organization closure and every destructive transition remain unavailable. The user explicitly directed not to implement yet. No role catalogue, capability, schema, migration, OpenAPI, route, request, export, worker, runtime or frontend change follows from these approvals.
