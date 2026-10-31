# BID-ARC-004 — Review and close MVP database and backend design gaps

## Metadata
- Ticket ID: BID-ARC-004
- Epic: E-ARC
- Type: Design
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-ARC-003, BID-ARC-005
- Blockers: Schema/backend reviewer approval; unresolved identity and workflow policy
- Milestone: M0

## 1. Objective
Approve executable persistence and command boundaries from the proposed schema catalogue.
## 2. Business Context
Exact source-to-approval-to-package lineage is the core of trustworthy bid preparation.
## 3. Scope
Control/tenant ownership, identity-specific schema, mutable-root enumeration, role grants, FK ordering, null semantics, financial revisions, approval attempts, job references, retention and transaction scopes.
## 4. Out of Scope
Executing SQL, all future procurement/payment/AI tables, approving a generic JSON store.
## 5. Technical Requirements
Close concrete gaps: response evidence FK ordering, polymorphic job/hold validation, candidate manifest canonicalization handoff, one-active-workspace normalization, approval correction attempt uniqueness and idempotency expiry. Specify same-database module orchestration without direct repository coupling.
## 6. Proposed Implementation Approach
Walk each command through schema/port/transaction; list required columns/keys/checks/indexes and failure behavior; document revisions for separately authorized design changes; approve reviewer evidence.
## 7. Dependencies
[Policy](BID-ARC-003.md), [identity/security](BID-ARC-005.md). Accept schema and backend review before migration tickets become Ready.
## 8. Acceptance Criteria
- Given two concurrent revisions, then the design identifies the owning version and conflict transaction.
- Given cross-plane identity reference, then no impossible cross-database FK is proposed.
- Given immutable approval, then correction has explicit append-only semantics.
## 9. Testing Requirements
Schema review cases for FK/check/unique violations and audit/outbox rollback; no SQL execution claimed.
## 10. Definition of Done
Reviewed field-level MVP schema and command transaction matrix with no undocumented blocker hidden as default.
## 11. References
[Database](../../product/10_DATABASE_DESIGN.md), [schema](../../technical_design/06_DATABASE_SCHEMA_DESIGN.md), [modules](../../technical_design/09_BACKEND_MODULE_DESIGN.md), [database plan](../../implementation/05_DATABASE_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Documentation presence is not approved schema or tested migration evidence.
