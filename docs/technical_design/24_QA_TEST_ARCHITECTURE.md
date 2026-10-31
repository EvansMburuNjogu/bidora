# QA and test architecture

Sources: [QA strategy](../product/15_QA_TEST_STRATEGY.md), [evidence specification](../product/19_TRACEABILITY_ACCEPTANCE_EVIDENCE.md); decisions [006–012, 017](27_ARCHITECTURE_DECISIONS.md). Execution: [test implementation](../implementation/16_QA_TEST_IMPLEMENTATION.md).

## Test layers and gates

| Category / purpose | Scope / tooling | Stage / pass criteria / evidence |
|---|---|---|
| Domain invariants | Go table/property tests for bid states, decimal/date rules, approval applicability | Every PR; valid transitions succeed and all disallowed transitions fail; rule-to-case report |
| Persistence and migrations | Testcontainers PostgreSQL, sqlc repositories, roles/constraints | PR/candidate; clean and supported upgrade, rollback atomicity, denied audit mutation; migration versions/logs |
| API contracts | OpenAPI validation against real Go API and generated clients | PR; examples conform, breaking changes blocked, stable errors; conformance report |
| Identity/authorization | Session, CSRF, MFA/recovery, grant scope, revoked roles | Every relevant PR; positive and negative matrix passes; no enumeration/field leakage |
| Tenant isolation | Two tenants with similar data; API/router/storage/cache/jobs/exports/search | Every data-access change; zero unauthorized rows, counts, names, URLs or citations; adversarial case results |
| Workers/events | Real NATS + PostgreSQL, fault injection around commit/ack | PR/candidate; repeats cannot duplicate domain side effects; outbox recovery and dead-letter evidence |
| Upload/security | MinIO, scanner fixture/corpus, sandbox limits | Document slice/release; no unscanned bytes exposed, immutable version binding; scan/upload reports |
| Components | Testing Library plus selected Vitest-equivalent runner | PR; forms, conflict and permission states; component report |
| Accessibility | axe plus keyboard/screen-reader manual checks | PR smoke and release; WCAG 2.2 AA issues resolved for accepted workflows; manual evidence not replaced by axe |
| End to end | Playwright with real API/DB/storage/broker and test mail | Slice/candidate; user completes actual journey, no accepted mock workflow; trace/video/report |
| Performance | Approved load tool, representative fixtures | Release; agreed latency/error/throughput/resource budgets; load profile and raw results |
| Recovery | Isolated restore, object versions, outbox reconciliation | Pilot/release; RPO/RTO and business integrity verified; timed restore report |
| Security regression | SAST/dependency/secret/container scans, targeted DAST and independent assessment | PR/release; no release-blocking findings; scanner version/scope and remediation record |
| AI evaluation | Versioned corpus and adversarial fixtures, later | Model/prompt/index change; approved quality thresholds and zero cross-tenant retrieval; benchmark/version manifest |

## Critical scenarios

1. User A from tenant A requests tenant B bid ID, document version, job, count and package. All conceal private existence consistently.
2. Workspace switch while prior request is delayed: old response is discarded; old write context is rejected rather than applied to new tenant.
3. Two administrators remove the final admin concurrently: at least one operation is rejected and organization retains required admin.
4. Concurrent response edit and approval: stale candidate cannot become currently approved.
5. Evidence expires or source changes after approval: readiness and package-use gate block; historical approval remains immutable.
6. Scanner crashes or file bytes change after initial upload: no CLEAN artifact is served from unverified bytes.
7. Package object write succeeds but metadata commit fails: retry reconciles artifact and never reports false completion.
8. Network drops after MarkSubmitted commit: idempotent retry returns original result, with one submission fact.
9. Late external evidence capture preserves reported time separately; it cannot prove on-time external acceptance.
10. Future sealed submit immediately before/at/after cutoff: expected behavior comes from approved ADR-009, not invented test assumptions. Early opening, missing quorum, expired step-up and tampered ciphertext always fail under applicable policy.
11. Permission revoked between job enqueue and execution/download: job/result access denied or safely suspended.
12. Restore tenant to new DB: object hashes, approval versions and dedup state reconcile without repeated notifications or submission effects.

## Evidence and fixtures

Synthetic data only; deterministic clocks/IDs where practical. Store requirement IDs, commit/build, environment, tool versions, executed timestamp, case results, limitations and artifact paths. 'Not run' is not 'passed'. Screenshots prove visual state only. Proposed test IDs in implementation docs are engineering IDs, not fabricated source requirements.
