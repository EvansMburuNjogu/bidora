# Performance and scalability

Sources: NFR-PERF-001–006 in [SRD](../product/04_SRD.md), [operations SLOs](../product/18_DEVOPS_DEPLOYMENT_OPERATIONS.md). Decisions [003, 004, 013](27_ARCHITECTURE_DECISIONS.md). Execution: [release readiness](../implementation/19_RELEASE_READINESS.md).

## Baseline before optimization

Approve tenant count, simultaneous active tenants, users per tenant, records per bid, file/page/package sizes, job arrival rates, API mix and future deadline burst. No benchmark currently exists. Source p95 targets are not meaningful without this profile. Capture CPU/memory/connection/queue/error and tail latency, not average throughput alone.

## Isolation of expensive work

Keep scans, rendering, OCR, exports and AI out of request transactions. Separate job kinds/consumer concurrency so document/AI backlog cannot starve identity or future submissions. Per-tenant quotas and fair scheduling prevent noisy neighbors. Bound body size, upload size, parser resources, query duration and queue retries. Apply backpressure with explicit retryable errors, never drop authoritative events.

## Database fleet

Lazy bounded pools with idle eviction; cluster budget includes replicas/workers/migrators/admin reserve. Benchmark PgBouncer/pgx prepared statement behavior. Batch migrations and tenant outbox polling. Index real predicates and stable cursors; avoid OFFSET for large lists and cross-tenant fan-out reporting. Platform analytics receives minimized aggregates, never interactive joins across tenant DBs.

## Frontend/network

Use Server Components for initial layouts, client boundaries only for interaction. Lazy-load editors/viewers, paginate matrices, virtualize only with accessibility evidence. Prefer scoped multipart transfers and avoid buffering whole files/packages in API memory. Cache public projections separately; tenant caches require context/policy/version keys and revocation. No global `/me` cache.

## Growth triggers

Move public search to dedicated infrastructure only after measured PostgreSQL query/maintenance contention. Add dedicated tenant clusters when placement budget or contractual isolation demands it. Extract a service only for independently measured scaling/failure-domain need, not module count. Redis loss must not lose business state; cache misses must not trigger unbounded DB stampede.

## Verification

Load pool churn, large upload, package surge and outbox backlog recovery. Record p95/p99 and error rate before/after changes. Verify memory remains bounded and quotas cannot race beyond limits. Future sealed deadline load tests must include KMS/storage outages and authoritative acceptance correctness, not only requests per second.
