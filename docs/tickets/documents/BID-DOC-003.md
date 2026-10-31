# BID-DOC-003 — Generate immutable approved-candidate packages with recoverable jobs

## Metadata
- Ticket ID: BID-DOC-003
- Epic: E-DOC
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-BE-005, BID-DB-007, BID-DOC-002, BID-API-006
- Blockers: Accepted renderer/profile/canonical fingerprint
- Milestone: M4

## 1. Objective
Generate a verifiable package from exact approved inputs, never live mutable drafts.
## 2. Business Context
The team must be able to prove the submitted bytes correspond to its approved response.
## 3. Scope
Readiness recheck, package request/job, deterministic manifest/fingerprint, safe renderer, immutable artifact/hash, reconciliation and classified download.
## 4. Out of Scope
External sending, sealed encryption, unapproved export formats.
## 5. Technical Requirements
Candidate/renderer/profile identity; all-content authorization; exact source/evidence versions; sandbox rendering and safe filenames; storage success/DB failure recoverable; success only after verified object and metadata commit.
## 6. Proposed Implementation Approach
Implement package application/worker ports; snapshot permitted immutable references; stream output; persist checksums; reconcile existing fingerprint; reauthorize status/download and audit.
## 7. Dependencies
[Approval](../backend/BID-BE-005.md), [package schema](../database/BID-DB-007.md), [documents](BID-DOC-002.md), [API](../backend_contracts/BID-API-006.md).
## 8. Acceptance Criteria
- Given identical operation retry, then one package identity and exact manifest result exist.
- Given stale approval or expired evidence, then generation/use blocks explicitly.
- Given technical-only actor, then package containing pricing cannot be downloaded.
## 9. Testing Requirements
Hash vectors, renderer malicious input, worker crash/storage split failure, duplicate job and permission revocation tests.
## 10. Definition of Done
Artifact lineage/recovery demonstrated with real storage and safe API state.
## 11. References
[BR-026](../../product/02_BRD.md), [storage](../../technical_design/15_DOCUMENT_STORAGE_DESIGN.md), [document plan](../../implementation/11_DOCUMENT_STORAGE_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Successful generation does not guarantee external portal acceptance or unchanged current readiness.
