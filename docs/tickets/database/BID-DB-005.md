# BID-DB-005 — Persist immutable document versions and upload/retention state

## Metadata
- Ticket ID: BID-DB-005
- Epic: E-DB
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-DB-003, BID-API-004, BID-ARC-007
- Blockers: Approved format and retention schema
- Milestone: M2

## 1. Objective
Bind evidence metadata to immutable bytes and auditable verification.
## 2. Business Context
An approved certificate or tender document cannot be replaced behind an existing evidence reference.
## 3. Scope
Document roots/versions, upload sessions, scan states, verification findings, issue/expiry and retention holds; typed queries.
## 4. Out of Scope
Object transfer, scanner implementation, deletion bypassing holds.
## 5. Technical Requirements
Unique document/revision and object/storage-version; positive size and hash length; available requires clean; nullable issue/expiry validation; append-only verification corrections; authorized hold target validation.
## 6. Proposed Implementation Approach
Separate immutable byte identity from scan lifecycle; implement conditional upload finalization and version queries; apply restricted grants; test orphan metadata recovery state.
## 7. Dependencies
[Common](BID-DB-003.md), [document contracts](../backend_contracts/BID-API-004.md), [retention approval](../architecture/BID-ARC-007.md).
## 8. Acceptance Criteria
- Given uploaded version already bound, then another object cannot overwrite its identity.
- Given expiry before issue date, then persistence rejects it.
- Given active hold, then removal path cannot erase referenced evidence.
## 9. Testing Requirements
FK/check/unique, concurrent complete, immutable identity enforcement, retention/verification correction and upgrade cases.
## 10. Definition of Done
Reviewed schema/grants and negative persistence evidence ready for storage orchestration.
## 11. References
[RULE-DOC](../../product/05_DOMAIN_MODEL_AND_BUSINESS_RULES.md), [storage](../../technical_design/15_DOCUMENT_STORAGE_DESIGN.md), [document implementation](../../implementation/11_DOCUMENT_STORAGE_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Database metadata cannot prove object contents were scanned; storage tests remain necessary.
