# BID-SEC-004 — Execute retention and legal holds safely

## Metadata
- Ticket ID: BID-SEC-004
- Epic: E-SEC
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-API-010, BID-AUTH-003, BID-OPS-003
- Blockers: Legal validation of proposed periods/lawful bases, hold authority, partial-denial language, anonymization/deletion and restore policy
- Milestone: M7

## 1. Objective
Apply versioned retention policies without deleting held, ineligible or unverifiably scoped records.
## 2. Business Context
Platform settings must result in lawful controlled action rather than decorative policy values.
## 3. Scope
Privacy notice/acknowledgement, access/export/correction/deletion/restriction request lifecycle, policy versions/effective dates, record classes, legal hold apply/release, affected-count preview, partial approval/denial, leased cross-plane execution, append-only evidence, retries and restore/reconciliation implications.
## 4. Out of Scope
Invented retention periods, silent hard delete, archive restoration and tenant-admin bypass of legal authority.
## 5. Technical Requirements
Fail closed on unknown policy/hold; verified requester/tenant authority/capability/TOTP; immutable request/hold history; tenant isolation; bounded checkpoints across DB/object/FTS/vector/cache/jobs/providers; immediate tombstone/revocation; backup limitations and retained lawful classes disclosed.
## 6. Proposed Implementation Approach
Legally validate the proposed data-class schedule; implement request/scoping/preview separately from execution; process owner-module commands with checkpoints; verify derived-index purge and test restore into a new target with deletion/hold reconciliation.
## 7. Dependencies
[Contracts](../backend_contracts/BID-API-010.md), [platform authority](../authentication/BID-AUTH-003.md), [restore](../devops/BID-OPS-003.md).
## 8. Acceptance Criteria
- Given active hold, then eligible-looking records are not deleted/anonymized.
- Given partial external effect, then checkpoint retry converges without widening scope.
- Given restored backup, then current holds/policies/deletion tombstones are reconciled before cutover.
- Given a lawful retained class, then request completion reports partial retention without exposing protected evidence.
- Given approved source deletion, then authorization, caches, jobs and FTS/vector retrieval deny immediately and physical purge converges.
## 9. Testing Requirements
Hold/execution races, crash injection, cross-tenant scope, object/search reconciliation and restore tests.
## 10. Definition of Done
Approved policy classes execute against synthetic data with independent privacy/security and recovery evidence.
## 11. References
[Privacy/retention design](../../../backend/docs/phase-1-data-privacy-retention-deletion.md), [Security/privacy](../../product/14_SECURITY_PRIVACY_COMPLIANCE.md), [DR](../../technical_design/26_DISASTER_RECOVERY.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Configuration may ship before destructive execution, but must clearly report execution unavailable rather than imply deletion occurred.

## 13. Retention/legal-hold authority packet — DRAFTED_PENDING_APPROVAL

`backend/docs/retention-legal-hold-authority-decision-packet.md` proposes R1–R13: fixed record classes, complete immutable policies with no default/approved durations, external accountable legal-validation evidence, narrow Security/Audit capability additions, typed holds, immediate destructive fencing during propagation, exact maker/checker release, content-free previews, future finite execution checkpoints and mandatory restore reconciliation. It explicitly keeps policy activation, hold-release completion and every destructive effect unavailable until legal/privacy, backup/restore, security and explicit implementation gates clear. No schema, OpenAPI, migration, route, worker, duration, hold, deletion or runtime is authorized until the requesting user and accountable owners approve or amend R1–R13.

`backend/docs/privacy-request-contract-review.md` records selective policy approval for privacy role/request governance while preserving the execution block. Exact lawful-basis/category/owner matrices remain absent. The approved X1 boundary limits deletion requests to receipt, identity verification, immutable scoping and `LEGAL_OR_HOLD_REVIEW`; they cannot become `APPROVED_FOR_DELETION` or `APPROVED_FOR_ANONYMIZATION`. Catalogue, export, correction/restriction and destructive implementation remain unauthorized, and this review adds no verification evidence.
