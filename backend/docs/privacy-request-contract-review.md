# BID-API-010D — D1–D12 privacy request contract review

Status: **SELECTIVE POLICY APPROVAL RECORDED — LAWFUL-BASIS AND EFFECT MATRICES OPEN**  
Reviewed packet: `privacy-request-contract-decision-packet.md`  
Scope: lawful basis, authority, export, correction, restriction and destructive-execution review only  
Implementation authority: none

## 1. Review conclusion

D1–D12 correctly separate request receipt from entitlement and completion, prohibit cross-tenant targeting, preserve owner-module invariants, require current reauthorization, represent partial outcomes and keep destructive execution disabled. The proposed immutable scope/checkpoint model is consistent with Bidora's control-plane/tenant-plane separation and no-distributed-transaction rule.

The packet is not ready for approval as one executable contract. It does not contain an approved purpose/lawful-basis matrix, jurisdiction and deadline applicability rules, complete requester channels, exact data-category ownership, export formats/lifecycle, correction field matrix, restriction effect/release matrix or destructive owner-effect inventory. It also relies on two capabilities that do not exist in an approved catalogue—`PRIVACY_ADMINISTRATOR` and `tenderpreneur.organization.privacy.manage`—and does not fully define notice activation or bulk-export segregation of duties.

Destructive execution, deletion/anonymization approval, tenant closure, export generation/release and restriction effects must remain unavailable. Intake/status cannot be described as a rights-compliance process until accountable privacy/legal owners approve the applicable policy and operational channel.

## 2. Remaining lawful-basis decisions

### L1 — controller/processor and purpose matrix

A qualified privacy/legal owner must approve a versioned matrix for every supported purpose and data category. At minimum it must identify:

- whether Bidora acts as controller, processor on a tenant's instructions, or an independent controller;
- the exact processing purpose and necessary data categories;
- the jurisdiction/contract profile and approved lawful-basis reference;
- whether processing is mandatory or optional and the consequence of withholding data;
- applicable objection/restriction/portability/correction/deletion handling;
- approved retention-policy class and external legal-review evidence;
- processors/subprocessors, processing regions and approved transfer mechanism where applicable.

No default lawful basis may be inferred from account creation, notice acknowledgement, a Kenya country code, subscription payment, tenant membership or product use. “Contract,” “legal obligation,” “legitimate interests” and “consent” must not appear as selectable operator labels without approved definitions and evidence.

### L2 — consent and notice acknowledgement

D1 correctly says acknowledgement is not automatically consent. Decide whether any Phase 1 purpose actually relies on consent. **Recommended:** keep notice acknowledgement and consent as different evidence types; do not introduce a generic consent toggle. If a purpose later requires consent, define purpose-specific grant, withdrawal, effect timing and proof in a separate approved policy.

Notice activation also needs external legal/privacy validation bound to the exact body hash. A `PRIVACY_ADMINISTRATOR` should not be able to author and activate legal text alone.

### L3 — jurisdiction, rights applicability and deadlines

The applicable jurisdiction cannot be selected solely from tenant country, locale, timezone, IP address or the minimal Kenya reference pack. Account residence, data-subject location, controller establishment, contract and processing context may differ.

Select an accountable policy source that determines:

- which rights/request types apply to an exact requester/context;
- statutory acknowledgement/completion/extension rules;
- permitted identity-evidence requests;
- denial/partial-response wording and escalation/complaint channel;
- extension and pause rules;
- when a request is manifestly unfounded, excessive or duplicate, if applicable.

Until approved, `due_at` remains absent/null and no UI, email or operator may promise a deadline.

### L4 — requester channels outside an authenticated account

D2/D3 exclude lost-account, former-user, representative, guardian, estate and regulator requests. Account recovery does not cover every legally valid requester.

Choose one:

1. **Recommended:** authenticated self-service is only one Phase 1 intake channel; an externally governed privacy channel handles unsupported requester types with separate identity/authority verification and may create a restricted case reference without granting account access.
2. Declare authenticated-only requests to be the complete rights channel. This requires explicit legal approval and may be unacceptable.
3. Add all exceptional requester types to the application now. This is not recommended before evidence/representation policy exists.

The external channel, reviewer and identity-evidence retention rules remain deployment/legal decisions, not support-session authority.

### L5 — objection request type

The approved product direction lists objection “where applicable,” but D2 omits it. Decide whether Phase 1:

- adds `ACCOUNT_OBJECTION` only for purposes identified as objection-eligible by L1; or
- explicitly marks objection as handled through the external privacy channel until an in-product effect matrix is approved.

It must not be silently treated as generic restriction.

## 3. Remaining authority decisions

### A1 — Platform privacy role catalogue v2

D4 requires an explicit decision:

1. **Recommended:** approve the narrow fixed `PRIVACY_ADMINISTRATOR` role and listed capabilities in catalogue v2, with no tenant access, hold/retention/destructive authority, credential access or support-session use.
2. Add privacy disposition to `SECURITY_AUDIT_ADMINISTRATOR`. This combines audit oversight with case disposition and is not recommended.
3. Treat Platform Owner as implicit privacy authority. This violates the approved non-wildcard rule and must be rejected.

Role creation alone is not legal qualification. Deployment governance must separately identify accountable privacy/legal reviewers.

### A2 — Tenderpreneur organization privacy capability

`tenderpreneur.organization.privacy.manage` does not exist in the approved tenant capability catalogue. Decide whether to add it to the fixed Organization Administrator bundle or create a narrower fixed privacy representative role.

**Recommended:** add the capability only to the effective Organization Administrator bundle, with deny precedence, current context, recent TOTP and continuity checks. It authorizes request creation/status/cancellation only; it grants no platform disposition, other-user account request, hold release or direct deletion.

### A3 — notice governance

Select the maker/checker boundary for notice scheduling, activation and rollback.

**Recommended:** Privacy Administrator maker with current `platform.privacy.notice.manage`; distinct effective Platform Owner checker with `platform.governance.approve`; both bind exact immutable content hash, locale/audience, purpose matrix and external legal/privacy validation evidence using fresh TOTP. Approval expires after 30 minutes. Withdrawal of a never-active draft may remain single-actor; active history is immutable.

### A4 — request disposition and affiliation conflicts

D4 excludes tenant membership and active support context but does not mention pending tenant invitation or other current tenant affiliation. **Recommended:** Privacy Administrator disposition fails closed for any active membership, pending invitation, organization role, support-session context or requester identity overlap involving the affected tenant/request.

Access, correction and restriction disposition may use one eligible Privacy Administrator only after L1 and the exact owner-effect matrix are approved. Destructive disposition retains the distinct Platform Owner checker. Decide separately under A5 whether export release also requires a checker.

### A5 — export segregation of duties

A generated export is a high-impact disclosure. Choose one:

1. **Recommended:** Privacy Administrator scopes and approves; a distinct effective Platform Owner checks organization exports and any account export containing tenant-controlled or high-sensitivity categories before release. The requester then independently reauthenticates for download.
2. One Privacy Administrator scopes, approves and releases every export. This has a larger exfiltration blast radius.
3. Tenant Organization Administrator checks organization exports. This may be appropriate for tenant-controlled data but cannot replace platform privacy/legal review or authorize another person's restricted data without an exact policy.

No actor may approve/release their own request or an export for an affiliated tenant.

## 4. Remaining export decisions

### E1 — access, portability and organization-export category matrix

Define exact categories for each operation. The matrix must distinguish:

- Bidora-controlled account/profile/security/privacy-request data;
- tenant-controlled authored/contributed data;
- organization business records;
- another person's personal data embedded in tenant records;
- immutable financial, audit, approval, submission, security and dispute evidence;
- AI source/derived/provider metadata;
- nonportable derived/inferred/internal diagnostic data.

An individual account export cannot silently traverse every tenant database and disclose tenant-owned records. The controller/processor matrix must say whether such data is supplied by Bidora, referred to the tenant controller or represented only as a safe category inventory. Organization export cannot automatically disclose other users' private account/security data.

### E2 — export format and boundedness

Approve exact machine-readable formats, character encoding, archive profile, manifest canonicalization and ceilings for records, files, expanded bytes, object count and generation time. **Recommended initial boundary:** UTF-8 JSON plus referenced original authorized files in a deterministic ZIP profile; CSV only where a source-defined tabular schema exists and formula-safe escaping is specified. No executable content, symbolic links, nested archives or password-derived archive encryption.

Limits must fail safely or produce an explicitly complete paged/multipart manifest; silent truncation is prohibited. Exact ceilings remain a product/security/capacity decision.

### E3 — export object protection and download

Decide:

- encryption/KMS and private-object versioning policy;
- whether download is single-use or repeatable during the release window;
- maximum downloads and transfer lifetime;
- download audit content;
- behavior after requester authority, membership, restriction or hold changes;
- deletion/reconciliation after expiry.

The proposed 24-hour availability is not yet approved. **Recommended:** approve at most 24 hours, require fresh reauthentication at each download and revoke immediately on authority/scope change. The API returns a same-origin authorized stream or short-lived opaque redirect only under an approved object-delivery policy—never a raw object key.

### E4 — export-artifact retention prerequisite

Generating an export creates another sensitive copy. Export release must remain disabled until an approved `EXPORT_ARTIFACT` lifecycle proves logical denial at expiry and convergent deletion from object storage, caches and restores under R1–R13/BID-OPS-003. “Expires in 24 hours” cannot mean only that the link stops working while the archive is retained indefinitely.

### E5 — export completion and external delivery

Define `GENERATED`, `RELEASE_APPROVED`, `AVAILABLE`, `EXPIRED`, `PURGED` and failure/ambiguity semantics separately. `COMPLETED` for the privacy request must not imply the archive was downloaded, and a download must not imply physical purge. Email is notification only and never carries an archive/token.

## 5. Remaining correction decisions

### C1 — eligible field and owner-command matrix

D7 needs an explicit allowlist mapping each correctable fact to its owner operation, validation, evidence and side effects. At minimum decide treatment for:

- display email/canonical email and re-verification/session effects;
- account profile/display name;
- organization legal/display/address/tax facts;
- membership attribution;
- authored tender facts, requirements and responses;
- reported outcomes and other immutable evidence;
- financial/tax documents;
- security/audit records.

**Recommended:** self-service owner operations remain preferred; privacy workflow coordinates only where no safe owner operation exists. Financial, audit, approval, submission and security evidence is never overwritten—only source-defined append-only correction/supplement or safe annotation is permitted.

### C2 — requester-supplied corrected values and supporting evidence

Corrected values and supporting documents are private content and cannot enter general control audit/outbox facts. Decide their authoritative storage owner, classification, malware scanning where files are accepted, access scope and retention. **Recommended:** the control request stores only category/field keys and restricted evidence references/hashes; actual proposed values live in the responsible owner plane or a separately approved restricted privacy-evidence store.

### C3 — correction decision authority

A Privacy Administrator may determine workflow disposition but must not decide domain truth outside approved evidence rules. Define who verifies identity facts, organization facts and tenant-authored content. Tenant Organization Administrators cannot rewrite another member's account facts; Platform Privacy Administrators cannot edit tender or financial records directly.

### C4 — downstream invalidation

Every correction owner must define effects on identity lookup/verification, auth epoch/sessions, tenant context, search/vector indexes, AI suggestions, exports, notices and historical displays. Completion requires the owner command and required invalidation checkpoints, not merely approval of corrected text.

## 6. Remaining restriction decisions

### R1 — exact purpose/category effect matrix

`APPROVED_FOR_RESTRICTION` is too abstract to execute. Define allowlisted restriction kinds and effects, for example:

- nonessential AI provider processing;
- search/vector retrieval of eligible personal content;
- optional notification/marketing purpose, if any;
- support disclosure;
- optional analytics.

For each, define immediate deny point, physical cleanup checkpoint, allowed mandatory processing and requester-visible consequence. Security, authentication, fraud prevention, immutable audit, financial/legal retention and essential service operation cannot be switched off through a generic restriction without an approved lawful-basis rule.

### R2 — subject versus tenant-controlled content

Decide whether restriction attaches to an identity, exact records, category/purpose, tenant or combination. Restricting one contributor's personal processing must not silently hide organization-owned bid records from authorized coworkers or corrupt immutable attribution. Pseudonymous attribution and purpose-specific denial may be required instead.

### R3 — activation, review and release

D7 defines applying restriction but not lifting it. Define:

- requester withdrawal versus controller/legal decision;
- who approves release;
- expected versions and maker/checker for disputed restrictions;
- duration/review-at behavior;
- stale scope/authority/hold invalidation;
- cross-plane release checkpoints.

Restriction remains effective until every required target acknowledges release; it never auto-expires merely because a timer passed.

### R4 — conflict with holds and evidence

Restriction may narrow access but cannot destroy held evidence. Legal/security holds take precedence over physical purge while still permitting stricter retrieval denial where lawful. Exact conflict outcomes must use the approved retention/hold generations; a privacy request cannot release or weaken a hold.

## 7. Remaining destructive-execution decisions

### X1 — intake versus destructive disposition gate

D9 says deletion requests may be accepted, scoped and dispositioned before destructive policy is approved. That is too broad: without an approved lawful-basis/retention matrix, Bidora cannot safely label categories `APPROVED_FOR_DELETION` or `APPROVED_FOR_ANONYMIZATION`.

**Recommended:** before all destructive gates clear, allow receipt, identity verification, immutable scoping and `LEGAL_OR_HOLD_REVIEW` only. Category outcomes may be retained/review-required, but destructive approval and transition to `APPROVED`/`EXECUTING` remain unavailable with `execution_available:false`.

### X2 — deletion versus anonymization matrix

For every owner type and R1 record class, approve whether the effect is deletion, irreversible anonymization, pseudonymization/restriction or retention. Define what attributes remain, reidentification controls, linked-record effects and proof. Hashing an email or replacing a user ID is not automatically anonymization.

### X3 — owner-effect and finite-target inventory

Approve a complete inventory for control PostgreSQL, each exact tenant cell/database, MinIO object versions/derived files, FTS/vector indexes, Redis/cache state, queued/running jobs, notifications/support attachments, export artifacts, OpenAI/provider-held data where contractually possible and backup/restore reconciliation. Unknown owners/targets fail closed; no unbounded fleet scan is allowed.

### X4 — individual account deletion and membership continuity

Define the cross-tenant process for an identity with memberships, assignments, authored work and final-administrator obligations. Existing offboarding transfers only approved active ownership and preserves history. Account deletion must not bypass final-admin, invitation/reactivation, assignment, immutable-attribution or another tenant's controller obligations.

### X5 — organization closure is a separate business transition

`ORGANIZATION_CLOSURE_DELETION` combines closure, commercial/legal settlement, recovery window and privacy destruction. Before approval define:

- closure authority and final-administrator confirmation;
- active bids/jobs/support sessions/payments/refunds/reversal holds and disputes;
- export availability;
- writes permitted during closure;
- recovery/cancellation boundary;
- exact tenant-cell freeze and routing behavior;
- retention/hold and eventual destruction effects.

The proposed 90-day recovery window is unapproved and has no default. Privacy authority alone cannot close a commercial tenant.

### X6 — retention/hold and restore prerequisites

Destructive execution remains blocked until R1–R13, exact durations/lawful bases, hold application/release, BID-OPS-003 backup expiry/restore reconciliation, owner-module effects, security review and real race/crash evidence are approved. No documentation-only decision can mark these gates passed.

### X7 — completion semantics

`COMPLETED` requires every effect terminal and verified, including provider/object/index/cache/job checkpoints according to approved policy. `PARTIALLY_COMPLETED` requires every remaining category to have a terminal retained/denied explanation. `AMBIGUOUS`, `RETRYABLE`, unavailable target or restore reconciliation can never count as completion.

## 8. Decisions that can be accepted independently

The following D1–D12 principles can be approved without enabling exports or destruction:

- notices are immutable and acknowledgement is not consent;
- request target derives from authenticated identity/current tenant authority;
- no cross-tenant selector or support-session bypass;
- append-only request/scope/disposition history;
- category-level partial outcomes and no invented deadline;
- corrections use owner commands and immutable evidence is never overwritten;
- AI cannot decide lawful basis, approval or hold release;
- destructive execution reports unavailable until every gate clears;
- idempotency, optimistic concurrency and fenced checkpoints;
- safe errors, generic notifications and content-minimal audit/outbox evidence.

These principles do not prove a lawful operational rights channel or authorize a route.

## 9. Selective approval record and remaining sequence

The requesting user approved these policy choices:

- **L2:** notice acknowledgement and consent remain different evidence types; no generic consent toggle is introduced;
- **L4 option 1:** authenticated self-service is one channel, while unsupported requester types use an externally governed privacy channel with separate identity/authority verification;
- **L5 amended:** objection remains in the external privacy channel until an exact objection effect matrix is approved;
- **A1 option 1:** approve the narrow fixed `PRIVACY_ADMINISTRATOR` role/capability bundle in catalogue v2, without tenant, hold, retention, destructive, credential or support-session authority;
- **A2:** add `tenderpreneur.organization.privacy.manage` only to the effective Organization Administrator bundle for request creation/status/cancellation, with deny precedence, current context, recent TOTP and continuity checks;
- **A3:** Privacy Administrator notice maker plus distinct effective Platform Owner checker, exact content/purpose/legal-evidence binding, fresh TOTP and 30-minute approval expiry;
- **A4:** disposition fails closed for requester overlap, membership, pending invitation, organization-role or support-session affiliation; ordinary non-export/non-destructive disposition still depends on the missing lawful-basis/effect matrices;
- **A5 option 1:** Privacy Administrator maker plus distinct effective Platform Owner checker for organization exports and account exports containing tenant-controlled or high-sensitivity categories, followed by independent requester reauthentication;
- **X1:** before destructive gates clear, deletion requests may reach receipt, identity verification, immutable scoping and `LEGAL_OR_HOLD_REVIEW` only; they cannot receive destructive approval or enter execution.

The user explicitly directed **do not implement yet**. Catalogue-v2 role/capability policy approval does not authorize changing the catalogue source or database. L1 controller/processor/purpose/lawful-basis and L3 jurisdiction/deadline rules remain pending accountable privacy/legal evidence. E1–E5, C1–C4, R1–R4 and X2–X7 remain pending; therefore export generation/release, correction effects, restriction effects, deletion/anonymization approval, organization closure and all destructive execution remain unavailable.

Remaining sequence:

1. Obtain accountable privacy/legal approval for L1/L3 and the exact data inventory/classification.
2. Approve exact E1–E5, C1–C4 and R1–R4 matrices.
3. Resolve X2–X7 only with approved retention/legal-hold, backup/restore and owner-effect evidence.
4. Review amended D1–D12 for any separately approvable non-effect subset.
5. Only after a new explicit request consider bounded non-destructive contract/source implementation.

No role catalogue, capability, notice, schema, migration, OpenAPI, route, request, export, object, correction, restriction, deletion, worker, runtime, test or frontend change was made or authorized by this review.
