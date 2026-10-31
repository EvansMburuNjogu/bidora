# Database implementation plan

## 1. Purpose
Build reliable owner-scoped persistence before dependent business acceptance.

## 2. Scope
Control/tenant schema slices, SQL migrations, sqlc, transaction binding, fleet compatibility and recovery. Exchange schema is later.

## 3. Product specifications
[Database specification](../product/10_DATABASE_DESIGN.md), DATA-001–008, SR-TEN-006/007, RULE-BID-003–005.

## 4. Technical designs
[Architecture](../technical_design/05_DATABASE_ARCHITECTURE.md), [schema](../technical_design/06_DATABASE_SCHEMA_DESIGN.md), [migrations](../technical_design/07_DATABASE_MIGRATIONS.md).

## 5. Prerequisites
Accepted table/command contract for the slice; representative synthetic data; selected PostgreSQL version.

## 6. Required decisions
BID-ADR-008 tool; 011 policy constraints; 012 outcomes; 017 package fingerprint/canonicalization. Do not generate all tables while those rules remain open.

## 7. Dependencies
Identity schema first; registry/membership then tenant common/typed extensions. Domain packages may be designed in parallel, but accepted queries require actual constraints.

## 8. Target files
Future backend/migrations/{control,tenant_common,tenant_tenderpreneur}, owner SQL queries, sqlc configuration, repository adapters and integration tests.

## 9. Sequence
Specify keys/nullability/checks and query patterns; review migration recovery; implement clean/upgrade SQL; generate sqlc; bind repositories to explicit TxScope; test constraints and atomic command/audit/outbox; benchmark real queries; rehearse canary rollout and restore.

## 10. Domain/security
No cross-database FK/join. Runtime role cannot DDL or mutate audit. Exact document/candidate references remain immutable; soft delete cannot erase approved proof. Logical actor IDs do not confer authority.

## 11. Database/API/frontend
Map sqlc rows to domain then DTOs; no nullable/decimal ambiguity leaking into forms. Return stable version/conflict codes rather than SQL errors.

## 12. Tests
Real PostgreSQL clean/upgrade, foreign/unique/check failures, rollback on audit failure, concurrent version writes, last-admin serialization, package/bid mismatch, migration crash/retry and schema incompatibility.

## 13. Acceptance
Invalid states fail at appropriate layers; old/new compatible binaries work during expand-contract; failed tenant migration is isolated.

## 14. Done
Reviewed migration, verification query, rollout/recovery notes, generated code reproducibility and evidence.

## 15. Risks
Generic JSON snapshots hiding relationships; restrict JSON to typed manifests/config. Unbounded fleet migration or pool creation prohibited.

## 16. Evidence
Schema versions/checksums, constraint cases, EXPLAIN plans, lock duration, restore and rollback/forward-fix reports.

## 17. Exclusions
Executable migrations in this task; unapproved future procurement/payment/AI schema; destructive automatic down migrations.
