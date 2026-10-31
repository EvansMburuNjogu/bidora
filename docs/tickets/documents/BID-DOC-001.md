# BID-DOC-001 — Implement private S3 namespace and immutable-object adapter

## Metadata
- Ticket ID: BID-DOC-001
- Epic: E-DOC
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-DB-005, BID-BE-001, BID-SEC-001
- Blockers: Approved storage/format/retention policy
- Milestone: M2

## 1. Objective
Provide least-privileged storage primitives for provisioning and later upload orchestration.
## 2. Business Context
Database isolation alone does not protect confidential tender attachments and package exports.
## 3. Scope
Provision private namespace/policy, opaque server keys, scoped multipart transfer, immutable version-bound read/copy/hash, expiration/hold hooks and provider failure mapping.
## 4. Out of Scope
Exposed upload endpoints before tenant isolation, scanner, public bucket or user-supplied keys.
## 5. Technical Requirements
S3-compatible port hides SDK; namespace plus provider policy enforce isolation; exact version/action scope; secret-managed service identity; reconcile ambiguous complete/copy by operation ID; no full-file API memory buffering.
## 6. Proposed Implementation Approach
Implement fake port and MinIO contract suite; test namespace ACLs/versioning; expose provisioning-only internal port; document quarantine and cleanup safety.
## 7. Dependencies
[Metadata](../database/BID-DB-005.md), [application boundary](../backend/BID-BE-001.md), [threat review](../security/BID-SEC-001.md). Public handlers await DOC-002.
## 8. Acceptance Criteria
- Given tenant A credentials, then B namespace/version access is denied.
- Given repeated complete operation, then same immutable object can be reconciled.
- Given retained reference, then lifecycle cleanup cannot delete it.
## 9. Testing Requirements
Provider integration ACL/versioning/multipart/timeout, credential rotation and bounded transfer tests.
## 10. Definition of Done
Storage port passes real-provider-compatible tests with no public private objects.
## 11. References
[Security §9](../../product/14_SECURITY_PRIVACY_COMPLIANCE.md), [storage design](../../technical_design/15_DOCUMENT_STORAGE_DESIGN.md), [document plan](../../implementation/11_DOCUMENT_STORAGE_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Path prefixes alone are not security; provider bucket policy remains a required deployment control.

### Approved MinIO MVP increment

The user approved private MinIO storage for PDF/DOCX tender sources up to 25 MiB, ten-minute transfer instructions, mandatory SHA-256/byte-size verification and fail-closed quarantine. `internal/opportunity/storageminio` now implements an S3-compatible MinIO adapter with explicit verified TLS roots/name, bounded transport timeouts, checksum-, content-type- and exact-size-bound presigned multipart POST policy fields, exact-object inspection and mandatory bucket versioning verification. The adapter never creates buckets, changes provider policy, enables versioning or exposes credentials. It stores and later reads the exact MinIO object version rather than trusting a mutable key. MinIO Go v7.4.1 is pinned. A real MinIO ACL/versioning integration run remains deferred. Status: **IMPLEMENTED_NOT_VERIFIED**.
