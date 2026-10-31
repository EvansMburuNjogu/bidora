# TenderOS — DevOps, Deployment, and Operations

## 1. Operational objectives

- Reproducible local, CI, test, staging, and production environments
- Safe database-per-tenant provisioning and migration
- Observable, reversible, and secure deployments
- Reliable asynchronous processing and external integration handling
- Tested backup, restore, and incident procedures
- Cost visibility for infrastructure, AI, search, storage, and tenant database fleets

## 2. Environment model

| Environment | Purpose | Data policy |
|---|---|---|
| Local | Developer implementation and debugging | Synthetic only |
| CI | Automated build/test/scan | Ephemeral synthetic fixtures |
| Development | Shared integration | Synthetic; no production secrets |
| QA | System, security, performance, AI evaluation | Versioned test datasets |
| UAT/Pilot | Persona validation | Synthetic or explicitly approved sanitized pilot data |
| Production | Customer operations | Controlled live data |
| DR | Recovery capability | Encrypted replicated/restorable production data under restricted access |

Environment configuration is code or secret-manager configuration; no undocumented manual server changes.

## 3. Local Docker Compose

Recommended services:

```yaml
services:
  web:
  api:
  worker:
  ai-api:
  ai-worker:
  postgres-control:
  postgres-exchange:
  postgres-tenant-cluster:
  pgbouncer:
  nats:
  redis:
  minio:
  mailpit:
  clamav:
  otel-collector:
  prometheus:
  grafana:
```

Compose profiles may keep the full observability/AI stack optional for normal development. Migrations and seed commands must be explicit and repeatable.

## 4. Container standards

- Minimal pinned base images
- Multi-stage builds
- Non-root runtime user
- Read-only root filesystem where practical
- Explicit health checks
- Resource requests/limits in orchestrated environments
- No build tools or source secrets in runtime image
- Image vulnerability scan and SBOM
- Immutable image tags using commit/release digest
- Signed images/artifacts where feasible
- Graceful shutdown and worker lease release

## 5. CI pipeline

Suggested stages:

```text
1. Validate repository/format
2. Lint and type check
3. Architecture/import boundary checks
4. Unit tests
5. OpenAPI/event compatibility checks
6. Build binaries/web/AI packages
7. Database migration tests
8. Integration tests with containers
9. Frontend component tests
10. E2E smoke tests
11. Security/dependency/secret/license scans
12. Container builds and scans
13. SBOM and provenance
14. Publish candidate artifacts
```

Pull requests fail when required gates fail. Flaky tests are defects, not routine retries.

## 6. CD pipeline

```text
Approved artifact
→ deploy infrastructure/config changes
→ apply compatible control/exchange migrations
→ canary tenant migrations
→ deploy API/worker/AI/web candidate
→ run smoke/contract checks
→ expand tenant migration batches
→ monitor SLO/error/business metrics
→ activate feature flag
→ complete release evidence
```

Database and application compatibility windows permit safe rolling deployment.

## 7. Infrastructure as code

Manage through Terraform, OpenTofu, Pulumi, or the selected approved tool:

- Networks and security groups
- Compute/orchestration
- Database clusters and PgBouncer
- Object storage and lifecycle policies
- Broker/cache
- KMS/secrets
- DNS/TLS/CDN/WAF
- Monitoring and alerting
- Backup policies
- IAM/service identities

Manual emergency changes are documented and reconciled back into code.

## 8. Secrets management

Secrets include:

- Database credentials
- KMS keys/references
- AI provider keys
- Payment/email/source connector credentials
- OAuth/OIDC client secrets
- Webhook signing secrets

Rules:

- Store in a dedicated secrets manager
- Short-lived/workload identity preferred
- Separate by environment
- Least-privileged access
- Rotation schedule and incident rotation runbook
- Never commit, echo, include in build artifacts, or expose to browser
- Audit reads/changes for high-risk secrets

## 9. Tenant provisioning operations

Provisioning worker steps:

1. Validate tenant and plan request.
2. Reserve tenant slug/database name and storage namespace.
3. Select region/cluster using capacity and plan rules.
4. Create database and roles.
5. Register secret references.
6. Apply common and tenant-type migrations.
7. Seed country pack, workflow, roles, and reference data.
8. Create object-storage policies.
9. Run isolation and health checks.
10. Mark active and notify administrator.

Every step records state and can retry safely. Operators can inspect failure without direct database editing.

## 10. Tenant migration fleet

### Strategy

- Track schema version per tenant.
- Canary on internal/test tenants.
- Migrate small batches with concurrency limits.
- Pause automatically on error threshold or performance degradation.
- Support retries and forward fixes.
- Keep API compatible with old/new schema during rollout.
- Display fleet status in Platform Admin.

### Prohibited practice

Do not run an unbounded loop across all tenant databases during a production release without batching, observability, and stop controls.

## 11. Observability

### Logs

- JSON structured logs
- Trace, request, tenant, actor pseudonymous ID, operation, and result code
- Redaction/classification
- Separate audit logs from diagnostic logs

### Metrics

Infrastructure:

- CPU, memory, storage, network, container restarts
- Database connections, locks, replication, WAL, query latency
- Queue depth, consumer lag, dead letters
- Object storage request/error/volume

Application:

- API latency/error by route and tenant cluster
- Tenant provisioning/migration state
- Publication/submission/opening/evaluation operations
- Authentication and authorization failures
- Search latency/freshness/source health
- AI queue, latency, tokens/cost/quality
- Email/payment/webhook delivery

### Traces

Trace user request through API, tenant router, database, event, worker, AI/search, and external dependency. Sampling must preserve error and critical-operation traces while controlling cost.

## 12. Service-level objectives

Initial targets subject to business approval:

- Monthly availability: 99.9% for core production APIs/web
- Critical submission availability monitored separately around deadlines
- p95 non-AI API read < 300 ms at baseline load
- p95 non-AI write < 500 ms excluding file/third-party work
- Search p95 < 2 seconds for indexed queries
- Critical queue oldest-job alert before user-facing breach
- Backup freshness and restore-drill targets by plan

SLOs include error budgets and an agreed response when consumed.

## 13. Scheduled jobs

Examples:

- Tender open/close transitions
- Deadline reminders
- Document expiry alerts
- Saved search execution
- Source fetch/change detection
- Usage aggregation
- Retention/deletion
- Subscription renewal/grace/suspension
- Projection reconciliation
- Tenant migration/backup verification

Jobs use persistent schedules, distributed leases, idempotency, and missed-run recovery. Server timezone assumptions are prohibited.

## 14. Backups and disaster recovery

### Backup coverage

- Control database
- Exchange database
- Every tenant database
- Object storage and version metadata
- Search/rebuild metadata
- KMS/key-recovery configuration
- Infrastructure/configuration repository

### Restore workflow

```text
Declare restore target and incident/change record
→ restore into isolated environment/database
→ verify integrity/schema/events/object references
→ reconcile outbox/inbox and search indexes
→ run security and business smoke tests
→ approve cutover or export
→ preserve original evidence
```

Never restore blindly over active production.

### Exercises

- Monthly sampled tenant restore
- Quarterly control/exchange restore exercise
- Periodic full DR tabletop/failover rehearsal
- Sealed-submission recovery scenario before marketplace pilot

## 15. Deployment safety

- Feature flags separate deployment from exposure.
- Kill switches for AI provider, source connector, submissions, opening, email, and payment where appropriate.
- Health and smoke checks before traffic expansion.
- Automated rollback for application-only regressions.
- Database rollback is not assumed; use expand/contract and forward fix.
- Deadline-critical operations may require release freeze windows.

## 16. Incident management

Severity:

- SEV-0: Confirmed cross-tenant/sealed-bid/security catastrophe or widespread irreversible loss
- SEV-1: Major outage or critical workflow unavailable
- SEV-2: Significant degradation or isolated high-impact issue
- SEV-3: Limited impact

Runbooks include:

- Account/session compromise
- Tenant database unavailable
- Cross-tenant access alert
- Submission deadline outage
- KMS/opening failure
- Broker backlog
- AI provider incident
- Source connector abuse/failure
- Payment webhook incident
- Backup/restore failure

Post-incident review produces corrective tickets, owner, due date, and evidence.

## 17. Capacity and cost management

Track cost by:

- Tenant/database cluster
- Object storage and egress
- Search/index
- Broker/cache
- AI model/embedding/OCR use case
- Source connector
- Email/SMS/WhatsApp
- Backup and logs/traces

Use quotas, batching, model routing, cache, storage lifecycle, pool limits, and dedicated enterprise pricing to preserve gross margin.

## 18. Operational readiness checklist

Before activating a module:

- Dashboard and alerts exist
- On-call owner and runbook assigned
- Rate/size/quota limits configured
- Dependency timeouts/retries/circuit breakers tested
- Audit and business metrics verified
- Backup/restore implications covered
- Feature flag and rollback/forward-fix path ready
- Support documentation and known errors available
- UAT and security evidence linked

