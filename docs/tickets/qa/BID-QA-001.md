# BID-QA-001 — Establish synthetic multi-tenant integration and migration harness

## Metadata
- Ticket ID: BID-QA-001
- Epic: E-QA
- Type: Testing
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-TEN-002, BID-DB-003, BID-FND-003
- Blockers: Runnable selected dependencies and approved fixture policy
- Milestone: M2

## 1. Objective
Provide reproducible real-dependency fixtures and test evidence before private feature exposure.
## 2. Business Context
Isolation and migration defects often hide behind one-tenant mocked test setups.
## 3. Scope
Testcontainers PostgreSQL/NATS/storage as needed, two-tenant identity/role factories, deterministic clock/IDs, clean/upgrade migrations, transaction/failure helpers and report artifacts.
## 4. Out of Scope
Copied production data, production mock API acceptance, all later feature tests in one ticket.
## 5. Technical Requirements
Use actual migrations/roles; independent tenant DBs and storage policy; seed permitted members safely; reset isolated test environments only; collect versions and failed/skipped results; API conformance helper from approved schemas.
## 6. Proposed Implementation Approach
Create fixtures and lifecycle cleanup; exercise clean/previous schema; validate harness intentionally catches wrong tenant/grant/constraint; publish reusable test interfaces without weakening production policy.
## 7. Dependencies
[Router](../tenancy/BID-TEN-002.md), [common schema](../database/BID-DB-003.md), [local environment](../foundation/BID-FND-003.md).
## 8. Acceptance Criteria
- Given fresh isolated test run, then reproducible tenant fixtures and schema versions are created.
- Given deliberately wrong grant or migration, then test fails.
- Given failed case, then logs/artifacts remain available without secrets.
## 9. Testing Requirements
Harness self-tests, cleanup failure, migration checksum/upgrade and deterministic clock validation.
## 10. Definition of Done
Documented reusable integration harness with actual CI reports; later feature suites remain their owners' responsibility.
## 11. References
[QA levels](../../product/15_QA_TEST_STRATEGY.md), [QA design](../../technical_design/24_QA_TEST_ARCHITECTURE.md), [QA plan](../../implementation/16_QA_TEST_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Test fixtures must not expose production-only administrative bypasses.
