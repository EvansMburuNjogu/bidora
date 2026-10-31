# BID-API-004 — Specify immutable upload, document and long-running job contracts

## Metadata
- Ticket ID: BID-API-004
- Epic: E-API
- Type: Design
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-API-001, BID-ARC-008
- Blockers: Format, size, URL lifetime and job-policy approval
- Milestone: M2

## 1. Objective
Define a quarantine-to-available contract with durable progress and authorized exact-version downloads.
## 2. Business Context
Tender evidence cannot be used before safety checks or silently replaced after approval.
## 3. Scope
Upload initiate/complete, document list/detail/version/verify/archive/download, shared job query/cancel/retry and safe processing states.
## 4. Out of Scope
Provider raw keys in browser contracts, OCR/model endpoints.
## 5. Technical Requirements
Server-owned upload identity; expected size/type/checksum hints; immutable storage version binding; scan failures remain unavailable; exact version selection; owner/role controls; 202 Location and nonfabricated progress. Define job cancellation safe points and reauthorization.
## 6. Proposed Implementation Approach
Translate approved lifecycle to OpenAPI; model lost-complete-response retry; document scanner outage and forbidden result semantics; align shared Job DTO with API-002/006.
## 7. Dependencies
[Base](BID-API-001.md), [file policies](../architecture/BID-ARC-008.md).
## 8. Acceptance Criteria
- Given unscanned document, then no contract permits byte access.
- Given complete retry, then response identifies the same durable upload/job.
- Given revoked job requester, then result references are not leaked.
## 9. Testing Requirements
Schema examples for invalid parts, expiry, type/size mismatch, quarantine, repeated completion and unauthorized version.
## 10. Definition of Done
Reviewed lifecycle/cards, examples and validation evidence; no runtime claims.
## 11. References
[File API](../../product/11_BACKEND_API_DESIGN.md), [storage design](../../technical_design/15_DOCUMENT_STORAGE_DESIGN.md), [document plan](../../implementation/11_DOCUMENT_STORAGE_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Signed URL expiry bounds exposure but cannot revoke already downloaded bytes.
