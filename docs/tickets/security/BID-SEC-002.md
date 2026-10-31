# BID-SEC-002 — Verify tenant isolation before exposing bid data

## Metadata
- Ticket ID: BID-SEC-002
- Epic: E-SEC
- Type: Testing
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-QA-001, BID-TEN-002, BID-DOC-001
- Blockers: Runnable identity/router/storage boundary
- Milestone: M2

## 1. Objective
Establish an executable negative isolation gate reused by every private-data feature.
## 2. Business Context
No bidder should discover another organization's rows, names, files, jobs or permissions.
## 3. Scope
Same-shaped records across two tenants, registry/pool/context/cache/storage/job-policy paths, scoped test endpoints/ports without public business exposure.
## 4. Out of Scope
Declaring later domain APIs secure before they exist, weakening filters to simplify fixtures.
## 5. Technical Requirements
Assert denied content and metadata/counts; forged selector/foreign ID; unavailable registry fail closed; credential rotation; stale membership and context race; storage ACL scope; no default tenant.
## 6. Proposed Implementation Approach
Build adversarial matrix on real provisioned tenants; use QA harness and router/application ports; integrate gate in CI; require each later ticket to add its endpoints/exports to this suite.
## 7. Dependencies
[Harness](../qa/BID-QA-001.md), [router](../tenancy/BID-TEN-002.md), [storage adapter](../documents/BID-DOC-001.md).
## 8. Acceptance Criteria
- Given tenant A with B identifiers, then no B existence/content is returned.
- Given context switch during delayed request, then no result/write crosses context.
- Given control/secret outage, then routing fails without fallback.
## 9. Testing Requirements
Real DB/S3/HTTP-policy integration, pool churn and revoked authority cases; retain attack matrix/results.
## 10. Definition of Done
Baseline gate passes and is mandatory before business handler exposure; scope limitations explicit.
## 11. References
[QA isolation §4](../../product/15_QA_TEST_STRATEGY.md), [tenancy design](../../technical_design/04_MULTI_TENANCY_DESIGN.md), [security plan](../../implementation/15_SECURITY_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Database-per-tenant is only one layer; each new query/job/export must extend regression coverage.
