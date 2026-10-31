# BID-SEC-004R / BID-API-010R — Retention and legal-hold authority decision packet

Status: **DRAFTED — REQUESTING PRODUCT/PRIVACY/LEGAL/SECURITY APPROVAL**  
Scope: Phase 1 retention-policy lifecycle, protective legal holds, affected-count previews and destructive-execution fencing  
Persona: Security and Audit Administrator preserving records and proposing reviewed policy; distinct Platform Owner governing release/destructive authority  
Not implementation authority: no retention duration, lawful basis, hold release, deletion, anonymization, backup expiry, migration, handler, worker, route or runtime is approved by this draft alone.

## 1. Outcome and boundaries

This packet defines how Bidora may represent reviewed retention policy and protective legal holds without pretending that unresolved legal rules or destructive effects are approved.

Included:

- fixed record-class identifiers and policy schema;
- immutable policy versions and external legal/privacy validation evidence;
- protective hold application, cross-plane propagation and hold precedence;
- exact maker/checker boundaries for policy activation, hold release and destructive execution;
- content-free affected-count previews;
- durable destructive checkpoints for later implementation;
- restore reconciliation and safe status/error semantics.

Excluded:

- approving the proposed periods in `phase-1-data-privacy-retention-deletion.md`;
- choosing a lawful basis, jurisdictional deadline or customer-contract override;
- authorizing deletion/anonymization of any record class;
- deleting or modifying audit, financial, submission or legal evidence;
- backup schedule/RPO/RTO approval;
- legal advice or automated legal interpretation;
- tenant-private browsing by Platform Administrators;
- support-session, privacy-request or tenant-closure bypass;
- frontend, migration execution and production activation.

A configured duration is not a legal conclusion. A record becoming old is not sufficient authority to delete it.

## 2. Decision R1 — fixed record-class catalogue

**Recommended policy:** Retention and holds use source-defined record classes. Administrators cannot create arbitrary table names, SQL predicates, object prefixes or free-form classes.

Phase 1 class keys are:

- `IDENTITY_PROFILE` — account profile and non-secret identity attributes;
- `AUTHENTICATION_SECURITY` — credential/factor/challenge/recovery metadata and security events, never raw secrets;
- `BROWSER_SESSION` — expired/revoked session metadata and hashes;
- `PLATFORM_ADMINISTRATION` — role/invitation/governance history;
- `TENANT_MEMBERSHIP_ACCESS` — membership, grants, invitations and offboarding evidence;
- `PLATFORM_AUDIT_EVENT` — control-plane privileged/business audit;
- `TENANT_AUDIT_EVENT` — tenant-plane business/access audit;
- `SUPPORT_CASE_ACCESS` — support case, grant/session and disclosure evidence;
- `NOTIFICATION_METADATA` — minimized delivery metadata, not transient rendered secrets;
- `COMMERCIAL_FINANCIAL` — subscription, invoice, payment, credit/refund and accounting evidence;
- `TENDER_WORKSPACE` — opportunity/bid/requirements/responses/reviews and ordinary workspace history;
- `TENDER_DOCUMENT_OBJECT` — document metadata, immutable object versions and derived previews;
- `PROPOSAL_SUBMISSION_EVIDENCE` — exact packages, approvals, external proof, outcomes and dispute evidence;
- `AI_SOURCE_DERIVED` — source chunks, FTS/vector entries and cited derived records;
- `AI_TECHNICAL_USAGE` — content-minimized job/provider/accounting metadata;
- `OPERATIONAL_TELEMETRY` — classified application logs/metrics/traces;
- `EXPORT_ARTIFACT` — generated privacy/business exports and manifests;
- `BACKUP_RECOVERY_COPY` — encrypted database/object recovery copies and deletion tombstone reconciliation.

Every persisted owner-module type maps to exactly one primary class plus explicit linked classes where effects differ. Unknown/unmapped types fail closed for preview and execution. Classification changes create a new catalogue/policy version and cannot retroactively reinterpret completed evidence without approved migration/review.

## 3. Decision R2 — immutable policy schema, without approved durations

**Recommended policy:** A retention policy is an immutable complete version containing, for every R1 class:

- applicable plane and tenant type;
- jurisdiction/country and contract profile selectors from approved catalogues;
- lifecycle trigger from an allowlist, such as `CREATED_AT`, `TERMINAL_AT`, `REVOKED_AT`, `TENANT_CLOSED_AT` or `FINANCIAL_PERIOD_ENDED_AT`;
- positive duration expressed as whole calendar days or months under a versioned calendar rule;
- terminal disposition `DELETE`, `ANONYMIZE`, `RESTRICT_RETAIN` or `RETAIN_INDEFINITELY`;
- affected linked classes and required checkpoint kinds;
- approved legal-purpose/lawful-basis reference and legal-review evidence reference;
- backup-expiry interaction and restored-copy reconciliation requirement;
- schema/catalogue version and canonical content hash.

No duration defaults exist. The proposed schedule in the privacy design remains advisory input for legal review, not a seed or fallback. Missing, zero, malformed, contradictory or unvalidated entries make the complete policy ineligible for activation.

Tenant/customer variation may select only an independently approved immutable contract profile. A tenant cannot shorten a statutory/evidence minimum, extend retention without an identified purpose or upload arbitrary rules. Country, tenant and contract selectors are applicability facts, not database locators or authorization.

## 4. Decision R3 — policy lifecycle and external legal validation

**Recommended policy:** Policy states are:

- `DRAFT`;
- `PENDING_LEGAL_VALIDATION`;
- `VALIDATED`;
- `PENDING_APPROVAL`;
- `SCHEDULED`;
- `ACTIVE`;
- `SUPERSEDED`;
- `WITHDRAWN`.

Content is immutable after draft creation. Correction creates a new complete version.

A transition to `VALIDATED` requires immutable evidence from named accountable privacy/legal reviewers outside the ordinary Platform Admin role grant. The evidence records reviewer organization/role reference, review date, jurisdiction/contract scope, approved content hash and external decision reference. Bidora stores no privileged legal advice in general configuration/audit payloads. Possessing a Platform Admin role alone never constitutes legal validation.

The ordinary application cannot self-generate, infer or waive validation. Until a deployment's accountable legal/privacy governance channel and evidence format are approved, policies may remain `DRAFT` or `PENDING_LEGAL_VALIDATION` only.

Only one policy per exact applicability profile may be ACTIVE and at most one may be SCHEDULED. Activation or rollback never rewrites completed dispositions or historical policy bindings.

## 5. Decision R4 — authority and role-catalogue amendment

The current fixed `SECURITY_AUDIT_ADMINISTRATOR` role has only `platform.retention.policy.read` and `platform.retention.policy.draft`. Existing approved P6 already requires maker/checker for legal-hold release and destructive retention but no executable capability exists.

**Recommended catalogue-v2 amendment:** add these narrow capabilities to `SECURITY_AUDIT_ADMINISTRATOR`:

- `platform.retention.policy.propose`;
- `platform.retention.preview.run`;
- `platform.legal_hold.apply`;
- `platform.legal_hold.release.request`;
- `platform.retention.execution.request`.

Add read-only `platform.legal_hold.read` and `platform.retention.execution.read` to the same role. Do not add direct release, activation or deletion capability.

Authority rules:

- Read/draft: current Security and Audit Administrator, current AAL2.
- Propose policy activation/rollback: current capability and TOTP AAL2 no older than 15 minutes.
- Apply a protective hold: `platform.legal_hold.apply`, recent TOTP, exact scope and legal/investigation authority reference; no checker because application can only preserve, not disclose or destroy.
- Request hold release: `platform.legal_hold.release.request`, recent TOTP.
- Request destructive execution: `platform.retention.execution.request`, recent TOTP, exact active policy and preview.
- Approve policy activation/rollback, hold release or destructive execution: distinct effective Platform Owner with `platform.governance.approve`, TOTP AAL2 no older than 15 minutes.

Maker/checker bind the exact immutable policy/hold/scope/preview/generation facts. They differ; neither may be an affected privacy requester, tenant member or active support-session participant for the scoped tenant. Approval expires after 30 minutes. Execution rechecks both participants and every guard.

Platform Owner remains a checker, not a wildcard legal expert or tenant-data reader. Tenant administrators may view safe hold/retention status for their tenant and request review through a case, but cannot apply/release platform legal holds or execute deletion under this packet.

## 6. Decision R5 — exact legal-hold scope

**Recommended policy:** A hold contains:

- immutable hold UUID and externally governed matter/reference ID;
- reason category `LITIGATION`, `REGULATORY`, `SECURITY_INVESTIGATION`, `PAYMENT_DISPUTE`, `PROCUREMENT_DISPUTE` or `OTHER_REVIEWED`;
- one exact scope kind: platform-wide class set, tenant plus class set, identity plus class set, bid plus class set, or explicit bounded resource set;
- one or more R1 record classes;
- tenant/identity/bid/resource UUIDs only where the scope kind requires them;
- effective start, review-at date and optional externally determined end expectation;
- applying actor, authority-reference hash, policy/catalogue version and reason;
- immutable scope hash and monotonically increasing hold generation.

A hold accepts no SQL, wildcard object key, database name, arbitrary predicate, tender text, legal document or attachment. Explicit resource scope is capped at 500 identifiers; broader matters use a typed aggregate scope.

An open-ended hold requires a review-at date no more than 90 days after application and recurring reviewed renewal no more than every 90 days. Review does not auto-release. Missing review puts the hold into `REVIEW_OVERDUE` while continuing to block destructive effects.

Hold metadata is highly restricted. Tenant-facing views show only hold presence, affected safe record classes, applied/review timestamps and a safe reason category when disclosure is permitted; they do not expose legal strategy, investigation detail or another person's identity.

## 7. Decision R6 — cross-plane hold application and precedence

**Recommended policy:** Hold states are:

- `PROPAGATING`;
- `ACTIVE`;
- `REVIEW_OVERDUE`;
- `RELEASE_PENDING_APPROVAL`;
- `RELEASING`;
- `RELEASED`;
- `INTEGRITY_HOLD`.

Application first commits a control-plane hold at a new generation in `PROPAGATING`. From that commit onward, every retention/privacy/closure/destructive coordinator must treat the matching scope as held and refuse new destructive claims, even before tenant/object/index projections acknowledge it.

Server-owned workers propagate exact immutable hold facts to every affected authoritative plane through inbox/checkpoint transactions. A hold becomes `ACTIVE` only after every finite catalogued target acknowledges the exact generation. Unknown target, stale assignment, ambiguous effect or missing owner mapping leaves the hold nonterminal and destructive work blocked.

Hold precedence is absolute for matching scope:

1. `PROPAGATING`, `ACTIVE`, `REVIEW_OVERDUE`, `RELEASE_PENDING_APPROVAL`, `RELEASING` or `INTEGRITY_HOLD` blocks deletion, anonymization, overwrite, object expiry and index purge.
2. Restriction of access may still become stricter, but cannot destroy held evidence.
3. Archive state, subscription state, tenant closure, privacy approval, elapsed duration and support access cannot override a hold.
4. Hold does not grant content access or change ordinary authorization.

There is no distributed transaction. Every destructive batch later rechecks current control hold generation and exact local projection inside its authoritative transaction before effect.

## 8. Decision R7 — release is maker/checker and convergent

**Recommended policy:** Hold release is never a direct state toggle.

1. Eligible maker requests release with exact hold version/generation, external authority reference and reason.
2. Hold enters `RELEASE_PENDING_APPROVAL`; it continues to block every destructive effect.
3. A distinct eligible Platform Owner approves/rejects within 30 minutes after current authority and scope rechecks.
4. Approval moves to `RELEASING` and creates exact-generation release checkpoints for every affected plane.
5. Each plane atomically records immutable release projection evidence without deleting held records.
6. Only after every finite target acknowledges does control mark `RELEASED`.

A stale, ambiguous, moved-tenant or generation-ahead target enters reconciliation/integrity hold. The original hold remains effective. Old-generation release events are never forwarded automatically to a replacement cell.

Release makes records eligible for ordinary current-policy assessment; it does not itself delete, anonymize, resume a privacy request or execute accumulated overdue retention work.

## 9. Decision R8 — content-free affected-count preview

**Recommended policy:** An authorized preview evaluates one exact validated policy version or proposed execution against one finite server-owned target catalogue.

Preview output contains only:

- preview UUID, exact policy/catalogue/hold generations and observed timestamp;
- target plane/tenant opaque IDs where authorized;
- count by record class and proposed disposition;
- counts blocked by active/pending hold, unresolved mapping, restore reconciliation, privacy request or dependency uncertainty;
- estimated object bytes where owner modules can calculate safely;
- completeness state `COMPLETE`, `PARTIAL_UNAVAILABLE` or `INTEGRITY_CONFLICT`.

It contains no record IDs, filenames, email, tender title, pricing, document text, object/database locators or legal detail. Counts are authorization-filtered and bounded. A preview performs no mutation, tombstone, lock extension or provider call.

A preview is usable for approval for 30 minutes only and only while policy, hold, target-catalogue, assignment and schema generations remain exact. Partial or conflicting preview cannot support scheduling/approval.

## 10. Decision R9 — destructive execution remains disabled

**Recommended policy:** Source implementation before legal approval must expose `execution_available:false`. It may support draft policy, protective hold application/projection, release proposal without execution, and preview, but must not:

- transition a policy to SCHEDULED/ACTIVE;
- approve or complete a hold release;
- claim records are eligible for destruction;
- enqueue or claim deletion/anonymization work;
- shorten object-lock or backup expiry;
- emit a deletion-completed event.

Destructive activation requires all of:

- approved legal/privacy evidence for every R1 class/applicability profile;
- approved backup expiry and restore reconciliation under BID-OPS-003;
- approved owner-module deletion/anonymization semantics;
- validated active privacy notice and processor/controller obligations;
- independent security/privacy review;
- real cross-plane hold/race/crash/restore evidence;
- explicit requesting-user authorization for the exact source/runtime increment.

Documentation or elapsed time alone cannot open this gate.

## 11. Decision R10 — later destructive checkpoint contract

**Recommended policy for a future authorized implementation:** One execution binds exact policy version, preview, target catalogue, hold generation and cutoff instant. It never scans an unbounded fleet dynamically.

Execution lifecycle:

- `REQUESTED`;
- `PENDING_APPROVAL`;
- `APPROVED`;
- `EXECUTING`;
- `VERIFYING`;
- `COMPLETED`;
- `PARTIALLY_COMPLETED`;
- `CANCELLED`;
- `HELD`;
- `FAILED_REVIEW_REQUIRED`.

Each record/effect checkpoint uses owner-module idempotency and one of `RESTRICTED`, `ANONYMIZED`, `DELETED`, `RETAINED_BY_HOLD`, `RETAINED_BY_POLICY`, `NOT_FOUND_VERIFIED`, `RETRYABLE`, `AMBIGUOUS` or `CONFLICT`. Ambiguous effects never count as completed.

Before every batch/effect, the worker rechecks current policy, hold generation, tenant assignment, schema compatibility and authorization. Leases use database time, UUID attempt fencing and bounded retries. Exact lease/retry/capacity values require a separate implementation decision.

Immediate logical denial/tombstones and physical destruction are distinct checkpoints. A successful database deletion does not claim object/index/cache/provider/backup completion. `PARTIALLY_COMPLETED` requires every effect to be terminal and safely reports retained categories.

## 12. Decision R11 — backup and restore reconciliation

**Recommended policy:** Ordinary point deletion from immutable recovery copies is not claimed unless the approved backup platform safely supports it. Policy defines backup expiry separately and discloses that copies age out under that approved schedule.

Current hold generations, privacy/deletion tombstones, identity/session revocations and completed execution checkpoints are control/reconciliation inputs independent of the restored snapshot.

A restored target remains isolated and cannot serve traffic until it:

- reconciles current identity/membership authorization;
- imports/replays current holds and blocks destructive work;
- reapplies deletion/restriction tombstones and excludes affected search/vector rebuild inputs;
- reconciles object/provider/external effects;
- verifies exact schema and assignment generation;
- records review and approved cutover evidence.

A restore cannot resurrect released access, deleted content into serving paths or an old policy/hold generation. If reconciliation is incomplete, cutover fails closed.

## 13. Decision R12 — HTTP contract groups

**Recommended policy:** OpenAPI may be drafted after R1–R13 approval, while destructive operations remain explicitly unavailable under R9.

### Retention policy administration

- `GET /v1/platform/retention/record-classes` — fixed source catalogue.
- `GET /v1/platform/retention/policies` and `GET /v1/platform/retention/policies/{policy_id}` — bounded safe history/detail.
- `POST /v1/platform/retention/policies` — immutable complete draft.
- `POST /v1/platform/retention/policies/{policy_id}/submit-legal-validation` — bind external validation evidence; unavailable until governance channel approval.
- `POST /v1/platform/retention/policies/{policy_id}/propose-activation` — exact maker proposal; unavailable until R9 gates clear.
- `POST /v1/platform/retention/policy-changes/{change_id}/approve|reject` — exact checker disposition.
- `POST /v1/platform/retention/previews` — read-only content-free preview.

### Legal holds

- `GET /v1/platform/legal-holds` and `GET /v1/platform/legal-holds/{hold_id}`.
- `POST /v1/platform/legal-holds` — apply/propagate exact protective hold.
- `POST /v1/platform/legal-holds/{hold_id}/review` — record periodic review without release.
- `POST /v1/platform/legal-holds/{hold_id}/request-release`.
- `POST /v1/platform/legal-hold-releases/{change_id}/approve|reject` — contract may exist but approval/execution remains unavailable until R9 gates clear.

### Tenant visibility

- `GET /v1/tenderpreneur/retention/status` — authorized safe active-policy availability, hold presence and execution-unavailable status.
- `GET /v1/tenderpreneur/legal-holds` — only disclosure-permitted class/review summaries, never legal detail or mutation.

Platform routes accept no tenant/cell/database locator. Typed hold scope may contain an exact tenant UUID as governed subject, but routing remains server-owned. All writes require exact Origin/CSRF, UUID idempotency, quoted `If-Match`, recent TOTP and bounded reason/reference fields. Responses are private/no-store.

## 14. Decision R13 — errors, evidence and privacy

**Recommended policy:** Stable failures include:

- `retention_request_invalid`;
- `retention_not_authorized`;
- `retention_policy_not_found`;
- `retention_version_conflict`;
- `retention_idempotency_conflict`;
- `retention_state_conflict`;
- `retention_legal_validation_required`;
- `retention_preview_stale`;
- `retention_execution_unavailable`;
- `legal_hold_not_found`;
- `legal_hold_scope_invalid`;
- `legal_hold_release_blocked`;
- `legal_hold_integrity_hold`;
- `retention_dependency_unavailable`.

Errors do not disclose private records, legal analysis, investigation state, raw external authority references, database/object locators or internal diagnostics.

Committed mutations atomically append minimal audit/outbox evidence in their authoritative plane: policy/hold/change/execution IDs and versions, class keys, scope hash, actors, governance evidence references/hashes, generations, stable outcomes and timestamps. They exclude record content, legal advice, tender/payment details and credentials.

No report or UI may say “legally compliant,” “fully erased” or “hold released” unless the corresponding exact approved terminal evidence exists.

## 15. Implementation consequences after approval

Approval would authorize a bounded **non-destructive** source proposal for:

- the R1 catalogue and immutable draft policy versions;
- protective hold application/state/projection/checkpoint records;
- role-catalogue-v2 capabilities in R4;
- content-free preview contracts;
- strict OpenAPI drafts with unavailable destructive operations clearly marked;
- owner-module ports and finite target catalogues that never expose tenant content.

Approval would not authorize exact retention periods, policy activation, hold release completion, destructive execution, backup expiry, migrations, route mounting, workers, frontend or production use.

## 16. Deferred verification and acceptance

Later evidence must cover:

- role and maker/checker separation, self/tenant-conflict denial and 30-minute expiry;
- typed scope validation and arbitrary predicate/locator rejection;
- hold propagation before destructive claim and apply/delete races;
- moved tenant, stale generation, unavailable target and ambiguous projection handling;
- release convergence with hold remaining effective until all acknowledgements;
- preview completeness/generation invalidation and content absence;
- owner-module deletion/anonymization correctness per class;
- database/object/FTS/vector/cache/export/provider convergence;
- backup expiry and isolated restore reconciliation;
- audit/outbox atomicity, privacy and immutable history;
- qualified legal/privacy approval, independent security review, synthetic UAT and formal release acceptance.

## 17. Requested decision

Approve or amend this bounded set:

- **R1:** fixed source-defined record classes; unknown mappings fail closed.
- **R2:** complete immutable policy schema with no default or currently approved durations.
- **R3:** legal validation is external accountable evidence, not a Platform role assertion.
- **R4:** narrow Security/Audit catalogue-v2 capabilities plus distinct Platform Owner checker.
- **R5:** typed bounded hold scopes with 90-day maximum review interval, never auto-release.
- **R6:** propagating holds immediately block destruction and have absolute precedence.
- **R7:** release uses exact maker/checker and remains effective until every target acknowledges.
- **R8:** 30-minute content-free previews bound to exact policy/hold/target generations.
- **R9:** destructive execution stays unavailable pending legal, restore, security and explicit implementation gates.
- **R10:** future execution uses finite fenced checkpoints and never equates partial effects with completion.
- **R11:** restored targets reconcile current holds/tombstones/authority before cutover.
- **R12:** bounded Platform and tenant-visibility APIs with destructive operations unavailable.
- **R13:** stable safe errors, minimal evidence and no unsupported compliance claims.
