# BID-BE-003 — Enforce requirement evidence, assignments and deterministic readiness

## Metadata
- Ticket ID: BID-BE-003
- Epic: E-BE
- Type: Engineering
- Priority: P0
- Status: VERIFIED_TECHNICALLY
- Complexity: M
- Dependencies: BID-BE-002, BID-DOC-002, BID-API-003
- Remaining gates: real MinIO/ClamAV deployment acceptance; independent security review; UAT and formal acceptance
- Milestone: M4

## 1. Objective
Make compliance states reflect exact valid evidence rather than unchecked labels.
## 2. Business Context
Unresolved mandatory requirements and expired evidence are preventable bid failure causes.
## 3. Scope
Create/assign/edit requirement, attach clean document version, review status, deterministic blockers and permission-filtered matrix/readiness queries. Response evidence is enabled with BE-004.
## 4. Out of Scope
AI automatically setting SATISFIED, unapproved waiver, generic workflow designer.
## 5. Technical Requirements
RULE-BID-003 and RULE-DOC-003; source wording/location; owner membership; exact evidence version; version conflict; classification before counts/snippets; material change invalidates candidate applicability through owner port and synchronous guards.
## 6. Proposed Implementation Approach
Implement transition policy and evidence-validation port; compute blocker list from authoritative versions/expiry; expose reviewed APIs; add race and negative tests.
## 7. Dependencies
[Bid lifecycle](BID-BE-002.md), [clean documents](../documents/BID-DOC-002.md), [contracts](../backend_contracts/BID-API-003.md).
## 8. Acceptance Criteria
- Given mandatory unresolved requirement, then readiness is false with safe blocker.
- Given unsupported SATISFIED or expired evidence, then transition is denied.
- Given simultaneous edits, then stale version conflicts instead of overwriting evidence.
## 9. Testing Requirements
Status matrix, expiry/time tests, cross-bid/cross-tenant linkage, hidden financial evidence and transactional invalidation tests.
## 10. Definition of Done
Audited commands and accurate filtered readiness with negative evidence.
## 11. References
[BR-023/024](../../product/02_BRD.md), [domain](../../technical_design/03_DOMAIN_ARCHITECTURE.md), [backend plan](../../implementation/06_BACKEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
### Explicit evidence expiry continuation

Tenant migration `00015_document_expiry.sql` adds optional immutable expiry metadata to bid-evidence upload sessions and document versions. The upload boundary accepts only an explicit future RFC3339 instant; AI never derives or changes it. Completion fails if the evidence expires before binding, and satisfaction plus technical-response evidence linking require an exact `CLEAN`, currently unexpired version. Requirement, technical gap and proposal approval-readiness calculations treat a historically `SATISFIED` requirement as currently blocked when all linked evidence has expired, reporting `REQUIREMENT_EVIDENCE_EXPIRED`/`EVIDENCE_EXPIRED` with time-specific counts. Evidence reads expose the stored instant and observed expired flag. AI drafting fails closed if any snapshotted provider input expires before processing. Renewal, reminders, retention deletion and legal hold remain excluded. Status: **VERIFIED_TECHNICALLY** through V5 PostgreSQL expiry-boundary and exact-version evidence; deployment/UAT/acceptance remain pending.

### Immutable requirement correction history

Tenant migration `00016_requirement_corrections.sql` adds `bid.requirement_versions`, backfills revision 1 without changing source identities and makes every newly created manual or AI-accepted requirement create its initial immutable content revision atomically. `PATCH /v1/tenderpreneur/bids/{bid_id}/requirements/{requirement_id}` permits authorized corrections only while the requirement is `OPEN`, requiring complete corrected content, a reason, optimistic requirement/workspace versions and idempotency. Source opportunity/document identities cannot be changed. Each correction advances requirement/workspace versions with audit/outbox evidence and is exposed through bounded `/history`. Technical responses now snapshot the requirement version they address; corrections mark older responses `TECHNICAL_RESPONSE_OUTDATED`, exclude them from new proposal snapshots and block approval readiness until rewritten. Satisfied requirements, evidence history, deletion, waiver and `NOT_APPLICABLE` are not mutated. Status: **VERIFIED_TECHNICALLY** through V5 immutable-history, concurrent/stale-correction and outdated-response evidence; deployment/UAT/acceptance remain pending.

## 12. Risks and Notes
A readiness result is time-specific, not permanent submission authorization.

### V5 technical verification — 2026-10-23

Synthetic PostgreSQL 15.19/TLS evidence traversed the production authorization, application, persistence and processing boundaries. It proved mandatory readiness blockers; active-owner assignment; exact CLEAN same-bid evidence; cross-bid and cross-tenant denial; exclusive expiry boundaries; unsupported repeat satisfaction denial; immutable human corrections; one-winner concurrent edits and stale conflicts; outdated technical-response invalidation; immutable cited AI proposal acceptance; audit/outbox/idempotency lineage; PDF/DOCX/size/checksum/object-version binding; and quarantine with CLEAN, REJECTED and fail-closed scan behavior. The production ClamAV protocol adapter passed a synthetic Unix-socket INSTREAM test. No external provider or model was contacted.

Verification exposed and corrected missing mandatory outbox `actor_id` values across this journey, an overly narrow shared read guard that made reviewed 500/1000-row queries unreachable, and missing explicit private/no-store cache controls on requirement responses. Successful authenticated HTTP contracts now cover document initiation, requirement creation/readiness and evidence satisfaction. sqlc output and reviewed checksums were regenerated. Focused live and centralized suites passed under `-race`; affected vet/build/OpenAPI/integrity checks passed. This is technical verification, not UAT, deployment acceptance or formal acceptance. Real MinIO bucket/versioning/private-policy and real ClamAV deployment acceptance remain pending.

### Option A reconciliation — 2026-10-18
The approved bounded source includes manual and human-accepted cited requirements, assignment, exact CLEAN evidence satisfaction, expiry-aware deterministic readiness and immutable correction history. Waiver and `NOT_APPLICABLE` remain intentionally unsupported. Functional source is complete; migration, transaction/race, cross-tenant, scan-state and acceptance evidence follows `backend/docs/option-a-verification-order.md`.

### Code-first human-owned requirement slice

The current functionality-first phase adds Tenderpreneur migration 00005 and human-owned `bid.requirements`. An authorized human creates each requirement against the bid workspace's exact verified source version and source location; every new requirement starts `OPEN`. Optional creation-time ownership and later assignment both recheck a verified ACTIVE control-plane member. Requirement and workspace expected versions prevent silent overwrite, while creation/assignment commit tenant-local audit and outbox evidence with idempotent replay records. The composed routes are `POST/GET /v1/tenderpreneur/bids/{bid_id}/requirements`, `PATCH /v1/tenderpreneur/bids/{bid_id}/requirements/{requirement_id}/assignment`, and `GET /v1/tenderpreneur/bids/{bid_id}/readiness`.

Readiness is deterministic and reports only source-defined blockers currently available: workspace decision not `BID`, unresolved mandatory requirements, unassigned mandatory requirements and overdue unresolved requirements. AI proposals cannot write authoritative requirements directly, mark them satisfied, or override blockers; future document analysis must enter through human acceptance with exact citations. The connected AI-review continuation now lists at most 500 immutable proposals for one exact document version. Human acceptance requires `tenderpreneur.bid.requirement.manage`, a CLEAN document, an unchanged suggestion, and a matching verified-source workspace already in `BID`; it atomically records `ACCEPTED`, creates a separate authoritative `OPEN` requirement linked to the exact document version, advances the workspace version, and writes audit/outbox/idempotency evidence. Humans may edit the authoritative text/category/mandatory flag without altering the original AI proposal or citation. Rejection requires a nonblank reason and preserves history. The evidence continuation now adds bid-owned private PDF/DOCX uploads under the same checksum-, exact-size- and MinIO-version-bound policy. Completion queues malware scanning but never tender extraction. An authorized human can move an `OPEN` requirement to `SATISFIED` only by atomically linking an exact `CLEAN` `BID_EVIDENCE` version belonging to the same `BID` workspace, with nonblank evidence location and justification plus expected requirement/workspace versions. The evidence row, requirement/workspace advancement, audit, outbox and idempotency completion commit together; bare status updates, tender-source documents, quarantined/rejected files, AI actions, waivers and unsupported attestations have no path. Readiness stops counting satisfied mandatory requirements as open, unassigned or overdue. Document expiry, response evidence and candidate invalidation were initially scheduled for later continuation. At implementation time, sqlc generation, JSON parsing, production compilation and compile-only test discovery succeeded while tests and migration execution were deferred. V5 has since supplied the technical evidence recorded above. Status: **VERIFIED_TECHNICALLY**; deployment/UAT/acceptance remain pending.
