# BID-TEN-002 — Route authorized requests through bounded tenant PostgreSQL leases

## Metadata
- Ticket ID: BID-TEN-002
- Epic: E-TEN
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-TEN-001, BID-AUTH-002
- Blockers: Approved connection and revocation budgets
- Milestone: M2

## 1. Objective
Resolve tenant DB solely from validated context and trusted registry.
## 2. Business Context
Accidental pool reuse or stale placement can expose one bidder's confidential records to another.
## 3. Scope
Registry/schema checks, secret adapter, generation-aware pool cache, lazy bounded leases, idle eviction, drain/rotation and failure-closed behavior.
## 4. Out of Scope
Default DB fallback, client-supplied locators, cross-tenant SQL.
## 5. Technical Requirements
Bound per-process/per-cluster connections including workers/migrators; active leases cannot be evicted; validate schema interval and tenant state; pgx/PgBouncer compatibility tested; no sensitive routing details in errors.
## 6. Proposed Implementation Approach
Define AuthorizedTenant lease port; implement router/cache; enforce generation drain; add observability and concurrency fault tests using provisioned tenant databases.
## 7. Dependencies
[Provisioning](BID-TEN-001.md), [context authority](../authentication/BID-AUTH-002.md).
## 8. Acceptance Criteria
- Given missing/suspended/stale tenant, then acquisition fails without fallback.
- Given rotated assignment, then new requests never use old pool generation.
- Given churn beyond pool cap, then capacity remains bounded and active transactions are not interrupted by idle eviction.
## 9. Testing Requirements
Identical-ID tenants, wrong credential, control outage, schema mismatch, rotation/drain, concurrent lease and pool budget tests.
## 10. Definition of Done
Router contract and isolation/resource evidence accepted before business data exposure.
## 11. References
[SR-TEN-003–008](../../product/04_SRD.md), [tenancy design](../../technical_design/04_MULTI_TENANCY_DESIGN.md), [tenancy plan](../../implementation/09_TENANT_PROVISIONING.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Session-local DB state must not survive across tenant connection reuse.
