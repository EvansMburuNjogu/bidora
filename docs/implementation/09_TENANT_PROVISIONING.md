# Tenant provisioning implementation

## 1. Purpose
Create a usable private workspace without partial activation or cross-tenant resource adoption.

## 2. Scope
Registry, asynchronous checkpoints, database/roles/secrets/storage, schema compatibility, initial membership, context and suspension.

## 3. Product specifications
SR-TEN-001–008, RULE-TEN-001–003, J-01; [database provisioning](../product/10_DATABASE_DESIGN.md).

## 4. Technical designs
[Tenancy](../technical_design/04_MULTI_TENANCY_DESIGN.md), [migrations](../technical_design/07_DATABASE_MIGRATIONS.md), [recovery](../technical_design/26_DISASTER_RECOVERY.md).

## 5. Prerequisites
Verified identity, authorized self-service creation contract, provisioner identity and migration test suite.

## 6. Required decisions
BID-ADR-003 isolation preserved; 013 secret/cluster placement; 015 quotas/plan and suspension exceptions; 006 supported pilot tenant type.

## 7. Dependencies
Identity/audit/events first; storage policy and admin membership are activation requirements, not optional cleanup.

## 8. Target files
Future platform/tenancy, provisioner worker, registry queries, migration runner, onboarding progress and operator metadata view.

## 9. Sequence
Reserve request/idempotency; create registry/job/outbox; lease job; allocate owned DB/roles; register secret refs; migrate/seed; storage namespace; admin member; verify isolated access/health; mark ACTIVE. Record each checkpoint and safe retry status. Add rotate/drain assignment generation and restore-to-new path.

## 10. Domain/security
API has no create-database privilege. Existing resource name is adopted only after ownership verification. Unknown registry/schema/context fails closed. Platform metadata UI cannot show private titles/pricing.

## 11. Database/API/frontend
Control state plus external resource saga, not distributed SQL transaction. 202 job and durable status drive UI. Tenant common/type schema version recorded independently.

## 12. Tests
Crash after each checkpoint, duplicate request, resource collision, secret failure, schema incompatibility, pool eviction, wrong credentials, suspension and restore reassignment.

## 13. Acceptance
Two tenants provision and remain isolated; retry creates no duplicate DB/member; no ACTIVE tenant missing prerequisite.

## 14. Done
Operational retry/runbook, pool budget, isolation suite and sampled restore evidence accepted.

## 15. Risks
Unbounded pools or migration loops; leaked orphan DBs; stale registry. Use bounded leases, ownership reconciliation and generation drain.

## 16. Evidence
Checkpoint history, sanitized placement/version report, negative isolation and timed restore tests.

## 17. Exclusions
Shared private tables, direct cross-tenant SQL, silent cleanup deletion, hybrid tenants before policy approval.
