# Observability and operations

Sources: [operations §§11–17](../product/18_DEVOPS_DEPLOYMENT_OPERATIONS.md), [SRD §14](../product/04_SRD.md), SEC-008. Decisions [013](27_ARCHITECTURE_DECISIONS.md). Execution: [observability](../implementation/18_OBSERVABILITY_IMPLEMENTATION.md).

## Signals

Structured logs include trace/request ID, operation, safe result code, deployment version and redacted actor/tenant references where needed. Traces link HTTP → transaction → outbox → worker → provider using correlation/causation. Do not propagate session tokens or document content. Audit remains separate from sampled diagnostic logs.

Metrics use bounded labels (route template, operation, cluster, outcome), not tenant/user/object IDs. Track p50/p95/p99 latency, DB pool wait/churn, locks, schema compatibility failures, outbox oldest age, consumer lag/dead letters, job retries, scanner backlog, package failure, provisioning duration, storage errors, authentication abuse and quota failures. Business measures: first verified opportunity, approval invalidations, package completion, evidence confirmation; never fake win-rate metrics without definitions.

## SLO design

Source starting targets: 99.9% monthly core availability, p95 reads <300ms, writes <500ms excluding transfer/provider work, indexed search <2s. These remain proposed until baseline workload and measurement window are approved. Track user-perceived package/provisioning completion separately; API 202 latency is not job success. Define error budgets and alert ownership before deployment.

## Runbook contract

Every alert identifies symptom, impact, dashboard/query, safe diagnostic steps, containment action, escalation owner, recovery verification and evidence record. Required initial runbooks: tenant DB unavailable, control DB unavailable, authentication abuse, provisioning stuck, scanner unavailable, package failure, outbox backlog, backup stale, unauthorized access suspicion. Later add deadline/KMS incident procedures.

## Safe operational actions

Pause connector/model or job kind, revoke session, suspend tenant under approved policy, retry a scoped idempotent job, drain a bad deployment. No routine manual SQL to make a workflow look complete. Replay and support access require reason, scope, expiry and audit. Restore goes to isolated target before cutover.

## Verification

Inject DB timeout, broker outage and scanner failure; confirm user state, metrics, trace and actionable alert agree. Check logs for synthetic secrets/private pricing to ensure redaction. Test alert delivery and runbook execution by someone other than its author. Record actual drills, not merely dashboard screenshots.
