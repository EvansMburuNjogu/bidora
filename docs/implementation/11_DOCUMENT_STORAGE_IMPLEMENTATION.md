# Documents and storage implementation

## 1. Purpose
Turn uploaded files into authorized immutable evidence with honest processing states.

## 2. Scope
Upload/quarantine/scan, document versions, verification/expiry, authorized download and approved-candidate package generation.

## 3. Product specifications
PR-SH-003, PR-TP-002/013, RULE-DOC-001–004; [security §9](../product/14_SECURITY_PRIVACY_COMPLIANCE.md).

## 4. Technical designs
[Storage](../technical_design/15_DOCUMENT_STORAGE_DESIGN.md), [schema](../technical_design/06_DATABASE_SCHEMA_DESIGN.md), [API](../technical_design/10_API_CONTRACT_DESIGN.md).

## 5. Prerequisites
Tenant routing, quotas, private object policies, scanner sandbox, document/job contracts and worker delivery.

## 6. Required decisions
BID-ADR-017 approved format/size/page/archive policy, renderer/manifest canonicalization; 013 retention and storage provider.

## 7. Dependencies
Identity/tenancy/events before uploads; clean document before evidence; approved candidate before package.

## 8. Target files
Future shared/document adapters/domain, package worker, S3/scan ports, tenant migrations and upload/vault/package UI.

## 9. Sequence
Reserve quota/upload identity; issue scoped transfer; finalize immutable version; validate size/type/hash; enqueue scan; persist clean/rejected; authorize download; add version verification/expiry; generate immutable package with fingerprint; reconcile orphan/ambiguous failures.

## 10. Domain/security
No preview/business use before CLEAN. Bind scanned bytes to served storage version. Signed URL capability is bounded but not instantly revocable; sensitive streaming policy requires review. No raw keys supplied by user.

## 11. Database/API/frontend
DB metadata and S3 are not atomic; durable job/checkpoints bridge them. UI shows quarantined/failed/retry state. Evidence points to exact version, not latest document.

## 12. Tests
Malicious/corrupt/misdeclared file, object overwrite after scan, interrupted multipart, duplicate complete, expired upload, scanner outage, unauthorized version, package object-success/DB-failure and hash mismatch.

## 13. Acceptance
Unscanned/rejected bytes never exposed; package contains only exact authorized approved inputs; retries recover without duplicate success records.

## 14. Done
Storage/scan contracts, failure tests, retention implications, alerts and user workflow evidence accepted.

## 15. Risks
TOCTOU scan bypass, zip bombs, leaked pricing exports, lifecycle deletion of retained evidence. Enforce version binding/resource limits/classification/holds.

## 16. Evidence
Synthetic scan corpus, checksum/manifests, retry traces, access-denial results and recovery report.

## 17. Exclusions
Unapproved OCR/AI ingestion, arbitrary preview execution, public buckets for private evidence, speculative format support.
