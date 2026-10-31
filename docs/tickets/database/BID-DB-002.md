# BID-DB-002 — Persist control-plane identity, membership and tenant registry

## Metadata
- Ticket ID: BID-DB-002
- Epic: E-DB
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-DB-001, BID-API-002
- Blockers: Selected identity and control schema
- Milestone: M2

## 1. Objective
Create authoritative identity/session/membership and resumable registry persistence.
## 2. Business Context
Tenant authority must come from verified membership, not a client-provided identifier.
## 3. Scope
Selected identity tables, challenge/session uniqueness, membership/grants/invitations, tenants/clusters/assignments/jobs/schema versions, plan grants/usage reservations and typed queries.
## 4. Out of Scope
Tenant business data, implementing both auth alternatives, plaintext credentials.
## 5. Technical Requirements
Single-use challenge consumption; session hash/epoch; unique membership; one active assignment; secret references only; last-admin transaction locking; decimal usage and operation dedup; no cross-database FK.
## 6. Proposed Implementation Approach
Implement reviewed control migrations in bounded groups; add sqlc queries for conditional consume/version/lease/reservation; test role restrictions and FK/unique checks.
## 7. Dependencies
[Migration runner](BID-DB-001.md), [control contracts](../backend_contracts/BID-API-002.md).
## 8. Acceptance Criteria
- Given concurrent challenge consumes, then only one succeeds.
- Given duplicate provisioning request identity, then registry resources are not duplicated.
- Given runtime query results, then secret references remain internal and private bid tables are absent.
## 9. Testing Requirements
Constraints, lease/version concurrency, last-admin locking primitive, quota reservation race and upgrade tests.
## 10. Definition of Done
Reviewed migrations/queries, generated code reproducible, negative persistence evidence.
## 11. References
[SR-IAM/SR-TEN](../../product/04_SRD.md), [schema](../../technical_design/06_DATABASE_SCHEMA_DESIGN.md), [DB implementation](../../implementation/05_DATABASE_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Logical identity references in tenant DBs do not grant current access.

### Provider-neutral registry schema start — 2026-10-18

Implemented only control compatibility, Option A `TENDERPRENEUR` tenant registry, database clusters, one-active database assignments and per-tenant schema-version states. Typed sqlc routing requires ACTIVE tenant/cluster, matching assignment generation and compatible tenant-common/Tenderpreneur schema versions. It returns internal endpoint/secret references only for composition infrastructure. [Evidence and inventory](../../../backend/docs/database-foundation.md).

No identity credentials, sessions, memberships, grants, invitations, provisioning jobs, entitlements or actual database/role creation were added. A caller-supplied tenant UUID remains non-authoritative; current membership validation must precede registry routing. Goose/sqlc/static/Go checks pass, but no live control database, grants, concurrency or cross-tenant test ran. Parent remains Blocked.
