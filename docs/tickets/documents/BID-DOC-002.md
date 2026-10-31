# BID-DOC-002 — Orchestrate quarantine, scanning, verification and authorized document access

## Metadata
- Ticket ID: BID-DOC-002
- Epic: E-DOC
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-DOC-001, BID-SEC-002, BID-EVT-002, BID-API-004
- Blockers: Approved scanner/resource policies
- Milestone: M3

## 1. Objective
Make only clean exact document versions available to bid workflows.
## 2. Business Context
Malicious files and silently replaced certificates must never become trusted compliance evidence.
## 3. Scope
Upload initiate/complete, quota settlement, exact byte binding, scan worker, version verification/archive, safe preview/download, vault queries and durable eligibility event for parsing/indexing every supported `CLEAN` content version.
## 4. Out of Scope
Parser/embedding implementation, arbitrary active-content preview, unapproved formats and treating indexing as human verification.
## 5. Technical Requirements
Server-detected type/hash/size; sandbox limits; scan timeout stays quarantined; immutability prevents scan/serve/index TOCTOU; one exact-version indexing eligibility event after `CLEAN`; human verification cannot override malware state; expiry/hold/classification checked; audit and jobs durable.
## 6. Proposed Implementation Approach
Wire reviewed contracts to document use cases; coordinate S3 completion with metadata/checkpoint reconciliation; scan exact version; persist clean/rejected; implement scoped read/verify actions.
## 7. Dependencies
[Storage](BID-DOC-001.md), [isolation gate](../security/BID-SEC-002.md), [jobs](../events_workers/BID-EVT-002.md), [contracts](../backend_contracts/BID-API-004.md).
## 8. Acceptance Criteria
- Given scanner outage or rejected file, then no preview/download/evidence use succeeds.
- Given bytes replaced at original key, then only bound scanned version can be served.
- Given repeated completion or foreign upload ID, then dedup succeeds safely or access is concealed.
- Given one eligible version becomes CLEAN, then one logical parse/index request is recoverable without marking knowledge approved.
## 9. Testing Requirements
Malformed/polyglot/oversize fixtures, interrupted multipart, overwrite race, revoked download and quota retry tests.
## 10. Definition of Done
Real clean/quarantine workflow and negative security/recovery evidence accepted.
## 11. References
[RULE-DOC](../../product/05_DOMAIN_MODEL_AND_BUSINESS_RULES.md), [onboarding/context](../../../backend/docs/phase-1-tenderpreneur-onboarding-context.md), [storage](../../technical_design/15_DOCUMENT_STORAGE_DESIGN.md), [document plan](../../implementation/11_DOCUMENT_STORAGE_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Signed URLs have approved residual TTL; sensitive streaming may be required.

### Upload initiation/completion slice

Tenderpreneur migration 00006 introduces tenant-local logical documents, owned upload sessions, immutable document versions and durable jobs. `POST /v1/tenderpreneur/opportunities/{opportunity_id}/documents/uploads` reserves an opaque server key after deny-first `tenderpreneur.opportunity.document.upload` authorization and returns checksum-, media-type- and exact-size-bound ten-minute MinIO multipart POST instructions. Completion is owner-bound and idempotent, reconciles exact media type, size, SHA-256 and nonempty MinIO version, then atomically activates metadata, records the immutable version as `QUARANTINED`, queues `MALWARE_SCAN`, records `TENDER_ANALYSIS` as `WAITING_FOR_CLEAN`, and commits audit/outbox evidence. The completion response links to owner-only, tenant-routed `GET /v1/jobs/{job_id}` status metadata. No route serves quarantined bytes or advances analysis. The continuation adds an exact-version ClamAV Unix-socket adapter and transactional `CLEAN`/`REJECTED` transitions. The worker streams at most the recorded byte size without a SQL transaction, verifies current requester authority before reading bytes, emits only safe result codes, releases analysis solely on `CLEAN`, and cancels it on malware. Scanner errors return the job to `QUEUED` and leave the document `QUARANTINED`; they never create a clean record. Live ClamAV execution, bounded retry/backoff policy, quota accounting, safe download, archival and retention remain pending. Formatting, sqlc generation, JSON parsing and affected production compilation succeeded; database/provider execution and tests are deferred. Status: **IMPLEMENTED_NOT_VERIFIED**.
