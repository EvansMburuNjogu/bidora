# BID-DB-001 — Implement compatible migration sets and least-privilege database roles

## Metadata
- Ticket ID: BID-DB-001
- Epic: E-DB
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-ARC-004, BID-FND-003
- Blockers: BID-ADR-008 selection and schema approval
- Milestone: M2

## 1. Objective
Create explicit control/common/type migration execution with safe compatibility checks.
## 2. Business Context
A schema rollout must not corrupt tenant evidence or expose another organization's database.
## 3. Scope
Migration runner/ledger/checksums, role separation, clean/upgrade verification, per-database leases and bounded fleet interface.
## 4. Out of Scope
Application auto-migration, destructive automatic down SQL, all domain tables.
## 5. Technical Requirements
Separate runtime/migrator/provisioner/backup grants; immutable released migrations; expand-contract compatibility intervals; transactional versus concurrent-index exceptions reviewed. Registry reconciliation handles DB success/control update failure.
## 6. Proposed Implementation Approach
Implement selected SQL tool adapter; test role grants; add clean/previous-version fixtures; implement lock/checksum/verification reporting and stop-on-error canary contract.
## 7. Dependencies
[Schema approval](../architecture/BID-ARC-004.md), [local services](../foundation/BID-FND-003.md).
## 8. Acceptance Criteria
- Given duplicate runners, then only one owns migration lease.
- Given runtime role, then DDL is denied.
- Given failed migration, then prior compatible schema remains or recovery is explicit; no tenant fallback occurs.
## 9. Testing Requirements
Testcontainers clean/upgrade, checksum drift, crash/retry, invalid index recovery and incompatible binary cases.
## 10. Definition of Done
Reviewed migrations/roles, verification and recovery notes, executed integration evidence.
## 11. References
[Database §11](../../product/10_DATABASE_DESIGN.md), [migration design](../../technical_design/07_DATABASE_MIGRATIONS.md), [DB plan](../../implementation/05_DATABASE_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
A down migration is not guaranteed safe rollback.

### Bounded compatibility foundation — 2026-10-18

Added common and Tenderpreneur schema-compatibility migrations, distinct-set ordering, typed sqlc query and fail-closed application-generation validation. Control reports schema version 2; tenant-common reports version 3; Tenderpreneur reports version 2. All use read/write generation interval 1; missing, duplicate, unknown, old or incompatible rows fail safely. Compatibility metadata is not tenant authority or a replacement Goose ledger. [Evidence](../../../backend/docs/database-foundation.md).

Goose/sqlc checks and full active Go tests pass after one transient sum.golang.org connection reset was retried without disabling checksum verification. No live migration, lease, runtime grants, clean/upgrade/restore or duplicate-runner test occurred, so the parent remains Blocked.
