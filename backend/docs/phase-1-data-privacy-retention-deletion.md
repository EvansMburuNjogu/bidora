# Phase 1 data privacy, retention and deletion policy design

Status: **APPROVED PRODUCT DIRECTION — LEGAL/SECURITY VALIDATION REQUIRED BEFORE DESTRUCTIVE EXECUTION**  
Scope: Tenderpreneur and Platform Admin Phase 1 records, AI-derived indexes and data-subject/tenant requests

## 1. Principles

Bidora applies purpose limitation, data minimization, accuracy, confidentiality, tenant isolation, transparent processing, retention limitation and support for data-subject rights. A commercial subscription does not authorize unnecessary collection, indefinite retention or AI training.

Legal counsel and the accountable privacy owner must validate jurisdiction-specific lawful bases, notices, statutory response periods, financial-record minimums, employment/personal-data treatment and cross-border/subprocessor terms before production acceptance.

## 2. Privacy roles and notices

Bidora must document when it acts as controller, processor or independent controller for platform security/billing records. Tenant contracts must identify customer responsibilities for uploaded personal data. Privacy notices cover:

- data categories and purposes;
- required versus optional fields;
- AI parsing, local embeddings and OpenAI generation egress;
- subprocessors and processing regions;
- retention categories;
- access/correction/export/deletion/restriction channels;
- legal-hold and statutory-retention exceptions;
- contact and complaint/escalation route;
- notice version and user acknowledgement.

Notice acceptance is versioned and cannot be inferred from continued use where explicit acknowledgement is required.

## 3. Data inventory and classification

At minimum classify:

- public tender material;
- tenant-internal business material;
- confidential proposal/evidence content;
- commercially sensitive pricing;
- personal and potentially sensitive staff data;
- authentication/MFA/recovery secrets;
- payment/invoice records and provider references;
- security/audit/support records;
- AI source chunks, embeddings, prompts/results and usage metadata;
- platform configuration and secret references.

Classification controls collection, role access, AI eligibility, export, retention and deletion.

## 4. Privacy request types

Support the following governed requests:

- access and portable export;
- correction;
- deletion or anonymization;
- restriction of processing;
- objection where applicable;
- tenant closure and organization-data deletion;
- legal-hold application/release by separately authorized actors.

A request never grants access to another person's or tenant's data.

## 5. Request workflow

```text
Request received
→ verify requester identity and authority
→ classify account/user/tenant scope
→ discover affected systems and legal holds
→ assess lawful/contractual retention exceptions
→ provide preview and obtain required confirmation
→ approve, partially approve or deny with safe reason
→ execute resumable checkpoints
→ verify database/object/search/vector/cache/provider effects
→ issue completion/export evidence
```

Suggested internal states:

- `RECEIVED`
- `IDENTITY_VERIFICATION_REQUIRED`
- `SCOPING`
- `LEGAL_OR_HOLD_REVIEW`
- `APPROVED`
- `EXECUTING`
- `VERIFYING`
- `COMPLETED`
- `PARTIALLY_COMPLETED`
- `DENIED`
- `CANCELLED`

Statutory deadlines must be configuration backed by approved jurisdiction policy. Product copy cannot promise an unsupported universal deadline.

## 6. Identity and authority checks

Account requests require current-session authentication and recent step-up; lost-account requests use the approved recovery and anti-enumeration process. Tenant-wide export/deletion requires the effective Organization Admin/Owner policy, recent TOTP and final-owner/continuity checks. Platform support cannot initiate tenant deletion through a support grant.

Before execution, invalidate stale approvals when requester authority, tenant status, legal hold, policy version or scope changes.

## 7. Deletion behavior

Deletion is a cross-plane operation, not one SQL statement. The operation uses durable checkpoints for:

- identity/profile fields;
- tenant database records;
- object-storage versions and derived files;
- full-text and vector indexes;
- caches and queued/running jobs;
- notifications/support attachments;
- external provider data where contract/API permits;
- backups and restored copies according to backup policy.

Immediate effects after approved execution include session revocation, denial of new retrieval, index tombstones, cache invalidation and cancellation/staling of affected jobs. Physical deletion/anonymization proceeds in bounded retryable batches.

## 8. Records that may require retained evidence

Deletion does not silently erase records subject to active legal hold or approved statutory/contractual minimums. Typical candidates for restricted retention include invoices/payment adjustments, security/audit facts, support-access evidence and proposal/submission records required for dispute defence. The exact legal periods remain validation gates.

Where full deletion is not lawful, minimize or pseudonymize identity fields, restrict access, stop unrelated processing and explain the retained category/reason to the requester where permitted. Audit records retain internal immutable subject references rather than unnecessary email/name content.

## 9. Backups and restore

Ordinary point deletions from immutable backups may be impractical and unsafe. The approved backup schedule must define expiry. Restored data remains isolated until deletion requests, holds, revocations and index tombstones recorded after the backup are replayed/reconciled. Restored stale data cannot return to service before this check.

## 10. AI and vector privacy

- Only eligible `CLEAN` exact document versions are parsed/indexed.
- Local open-source embeddings are preferred to minimize external egress.
- Vector/full-text records inherit source classification and retention.
- Revocation/deletion immediately denies retrieval and creates tombstones.
- Physical index removal is verified and rebuilds exclude deleted content.
- No cross-tenant vector cache or training corpus is permitted.
- OpenAI receives only authorized minimum excerpts for approved generation operations.
- Provider remote response storage remains disabled where supported.
- Bidora does not use tenant content to train shared models without a separate explicit lawful agreement and opt-in policy.

## 11. Recommended retention schedule for legal review

The following are product defaults proposed for legal/security validation, not activated statutory claims:

| Record class | Proposed default | Trigger / disposition |
|---|---:|---|
| Active tenant bid/profile/documents and approved AI outputs | Tenant life plus 90-day closure recovery window | Delete/anonymize after window unless hold or tenant-selected lawful longer schedule |
| Archived bid/proposal/submission evidence | 7 years | From archive/terminal event; tenant policy may require reviewed variation |
| Invoice/payment/credit/refund records | 7 years | From financial period/transaction; retain minimum fields required by law |
| Platform and privileged audit evidence | 7 years | From event; restricted access, content-minimized |
| Authentication/security events | 1 year | From event unless active investigation/hold |
| Expired/revoked browser sessions | 90 days | From terminal state; token hashes remain inaccessible and then deleted |
| Verification/recovery/MFA transaction metadata | 90 days | From expiry/consumption; factor state follows account lifecycle |
| Notification delivery metadata | 1 year | Message body minimized; transient rendered content no longer than operational need |
| Support cases and access-session evidence | 3 years | From closure, unless dispute/hold |
| Failed/transient AI job technical metadata | 90 days | Raw provider payload not retained as a diagnostic default |
| AI usage/cost metadata | 1 year | Content-free accounting/operations data |
| Full-text/vector derived entries | Same as source | Tombstone immediately on revoke/delete; physical purge through checkpoint |
| Operational logs | 90 days | Shorter where practical; no raw private content/secrets |
| Backups | Proposed 35-day rolling window | Encrypted; restored deletion/hold reconciliation required |

Country-specific law, customer contract and active legal hold can override a default only through versioned approved policy. Longer retention must have an identified purpose; shorter retention cannot violate required evidence obligations.

## 12. Tenant closure

Recommended flow:

```text
Organization Admin requests closure
→ recent TOTP and current authority checked
→ open payments, support access, active jobs and holds displayed
→ closure confirmation and recovery window recorded
→ business writes stop except approved export/payment/support actions
→ export remains available under policy
→ recovery window expires
→ deletion/anonymization checkpoints execute
→ completion evidence is issued
```

Closure never means immediate unaudited destruction. It also cannot be used to hide payment, security or legal evidence.

## 13. Individual user deletion

Removing a user from one tenant first transfers/fences owned work and revokes membership. Deleting the identity then revokes sessions and credentials, removes unnecessary profile/contact data, deletes private indexes/content owned solely by the individual where lawful, and pseudonymizes retained attribution. If the user is the final effective Organization Admin, transfer or tenant closure is required first.

## 14. Export

Exports are asynchronous, scoped, encrypted in transit/at rest, short-lived and reauthorize download. They include a manifest and explain omitted third-party/other-user/legal-restricted data. Raw credentials, hashes, provider secrets and another tenant's data are never exported.

## 15. Breach and subprocessor governance

Maintain a subprocessor register for OpenAI, PesaPal, SMTP and hosting/storage dependencies with purpose, region and contract status. Security incidents use an approved assessment/notification workflow and preserve necessary evidence under restricted hold. Exact notification duties and timelines require legal/incident-policy approval.

## 16. Acceptance gates

Before destructive processing is enabled:

- privacy/legal owner approves the classification and schedule;
- controller/processor roles and notices are accepted;
- request identity/authority and partial-denial language are reviewed;
- deletion effects cover PostgreSQL, objects, vectors/FTS, caches, jobs and providers;
- hold/deletion and restore races pass with synthetic data;
- exports pass isolation and privacy review;
- operational owners and escalation paths are named;
- UAT demonstrates request, status, partial retention explanation and completion evidence.

## 17. Ticket ownership

- BID-API-010: privacy request, export, hold and retention contracts.
- BID-SEC-004: request orchestration, hold, deletion/anonymization and reconciliation.
- BID-AUTH-004: no support-session bypass of privacy authority.
- BID-BE-014: versioned privacy notices and retention settings.
- BID-AI-002: vector/FTS tombstones and rebuild exclusion.
- BID-DOC-002: source/object lifecycle.
- BID-FE-009: tenant/user request and setup notice experience.
- BID-FE-011: authorized policy/hold/request operations.
- BID-OPS-003: backup expiry and restore reconciliation.
- BID-QA-006 and BID-REL-005: technical, legal, UAT and release evidence.
