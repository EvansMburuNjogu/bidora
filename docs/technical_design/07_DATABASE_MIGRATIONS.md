# Database migrations

Sources: [database §11](../product/10_DATABASE_DESIGN.md), [operations §§9–10](../product/18_DEVOPS_DEPLOYMENT_OPERATIONS.md); SR-TEN-006/007. Decision [BID-ADR-008](27_ARCHITECTURE_DECISIONS.md). Execution: [database plan](../implementation/05_DATABASE_IMPLEMENTATION.md).

## Executable bounded capture sets — 2026-10-18

The user authorized migration execution artifacts for the approved unverified manual-capture slice. `backend/migrations/tenant_common/00001_command_evidence.sql` and `tenant_tenderpreneur/00001_unverified_opportunity_capture.sql` are authored, checksummed, Goose v3.29.0 validated and parsed by sqlc v1.31.1. They were **not run against PostgreSQL**. See [database foundation](../../backend/docs/database-foundation.md) and [set runbook](../../backend/migrations/README.md).

The control set has its own control database and `goose_control_registry_version`; its first migration creates registry/routing and compatibility metadata; its second adds control audit/outbox and advances control schema to version 2; after email/password selection, its third additively introduces password identity records and advances control schema to version 3; its fourth adds the confirmed email identity constraints and advances control schema to version 4; its fifth adds hashed single-use email-verification challenges and advances control schema to version 5; its sixth adds opaque browser-session/CSRF hashes and advances control schema to version 6; its seventh adds current memberships and explicit capability effects and advances control schema to version 7. The two tenant sets require distinct Goose ledger tables (`goose_tenant_common_version` and `goose_tenderpreneur_version`) despite both starting at version 1. Common precedes the tenant-type extension. Tenant version-2 migrations add `bidora_meta.schema_compatibility`: common and Tenderpreneur rows expose schema version plus bounded binary read/write generation intervals. The current capture binary generation is 1 and requires tenant-common schema version >=5 and Tenderpreneur schema version >=2, plus compatible reader/writer intervals. Missing/duplicate/unknown/incompatible results fail closed. This metadata does not select a tenant, authorize a command or replace Goose history. Runtime startup never migrates. Up SQL has bounded lock/statement timeouts and no destructive operation; Down is only for owned empty development databases. Deployment-specific role grants, live migration/upgrade/restore and isolation evidence remain mandatory before acceptance.

## Sets and ownership

Separate ordered migration sets: control, tenant_common, tenant_tenderpreneur, tenant_procurement, exchange. Tenant type selects extensions. A release records a compatibility interval per set, not one misleading global schema version. Migrations are immutable after release and checksummed. Goose is approved as the SQL-first migration tool under BID-ADR-008; exact version acceptance and runtime migration proof remain separate gates. The rules do not depend on its CLI.

## Authoring contract

Each future migration includes intent, owner, SQL, preconditions, verification queries, lock/statement timeout, expected row volume, data backfill plan, application compatibility, recovery approach and evidence. No destructive 'down' is assumed safe. Never generate migrations by diffing production into unreviewed SQL.

Expand columns/tables first; deploy code tolerant of old/new shapes; run checkpointed backfill; verify; switch reads/writes; contract only after old binaries and rollback window expire. Large indexes use a reviewed nontransactional concurrent-index step where PostgreSQL requires it. This exception must not disable transactional safety for unrelated schema changes.

## Fleet algorithm

Acquire per-database migration lease/advisory lock. Verify registry identity and current checksum. Run selected compatible migration. Execute verification queries. Record result in database and control fleet state. Canary on internal tenants, then bounded batches. Pause on threshold breach. Failed tenants are marked unavailable for incompatible commands, never silently routed elsewhere. Registry write failure after DB success is reconciled by inspecting the database migration ledger.

## Failure and recovery

- Transactional SQL failure: rollback, record safe error, retain current version.
- Interrupted backfill: resume from durable cursor; repeated updates must be idempotent.
- Index creation failure: inspect invalid index and retry under documented cleanup.
- Deployment rollback: only binaries compatible with current schema may be promoted.
- Destructive loss: restore into a new DB and reconcile objects/events; never auto-run reverse SQL over live data.

## Tests and acceptance

Test fresh bootstrap, oldest supported schema upgrade, interrupted migration, duplicate runner, concurrent writes, compatibility checks and restore. Run Testcontainers on pinned PostgreSQL version. Evidence includes before/after schema versions, checksum, duration, lock waits, verification results and rollback/forward-fix rehearsal. No production execution is authorized by this document.
