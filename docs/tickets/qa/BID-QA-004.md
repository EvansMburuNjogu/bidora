# BID-QA-004 — Measure tenant-pool, package and concurrency limits against approved load

## Metadata
- Ticket ID: BID-QA-004
- Epic: E-QA
- Type: Testing
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-OPS-002, BID-OBS-002, BID-DOC-003, BID-ARC-007
- Blockers: Approved workload/SLO/resource budgets
- Milestone: M4

## 1. Objective
Establish measured external-MVP capacity without sacrificing correctness under contention.
## 2. Business Context
Large file/package work must not exhaust tenant connections or make human approval unreliable.
## 3. Scope
Tenant churn/pool eviction, mixed reads/writes, upload/package surge, outbox backlog recovery, quota races, response-edit/approve and job cancellation concurrency.
## 4. Out of Scope
Invented tenant scale guarantees, future sealed deadline-load certification.
## 5. Technical Requirements
Document workload/cardinality/file sizes/replicas; measure p50/p95/p99, error/timeout, DB locks/pools, memory/CPU and queue age; assert versions/idempotency and fairness under load.
## 6. Proposed Implementation Approach
Seed realistic synthetic data; baseline; run repeatable stress and dependency outages; diagnose bottlenecks; create targeted fixes and rerun same profile.
## 7. Dependencies
[Staging](../devops/BID-OPS-002.md), [metrics](../observability/BID-OBS-002.md), [packages](../documents/BID-DOC-003.md), [budgets](../architecture/BID-ARC-007.md).
## 8. Acceptance Criteria
- Given approved load, then budgets pass with no cross-tenant or duplicate transition failure.
- Given dependency slowdown, then queues/backpressure stay bounded and recover visibly.
- Given pool cap, then active leases are not incorrectly evicted.
## 9. Testing Requirements
Repeatable load scripts/reports, correctness assertions, EXPLAIN/lock evidence and regression comparison.
## 10. Definition of Done
Measured profile/results and resolved blockers; limitations accompany capacity claim.
## 11. References
[NFR-PERF](../../product/04_SRD.md), [performance](../../technical_design/25_PERFORMANCE_SCALABILITY.md), [QA plan](../../implementation/16_QA_TEST_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Fast 202 responses do not measure completed package latency.
