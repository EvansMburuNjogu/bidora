# BID-ARC-007 — Approve deployment, recovery, privacy and QA baselines

## Metadata
- Ticket ID: BID-ARC-007
- Epic: E-ARC
- Type: Design
- Priority: P1
- Status: Proposed
- Complexity: M
- Dependencies: None
- Blockers: BID-ADR-013; operations/privacy/QA approval
- Milestone: M0

## 1. Objective
Turn proposed operational targets into measurable release gates without inventing a cloud provider.
## 2. Business Context
Lost bid evidence or an untested restore is unacceptable even when API requests succeed.
## 3. Scope
Local/staging/production boundaries, CI host, region/provider selection, secrets/KMS custody, retention/holds, backup/RPO/RTO, workload baseline, alert ownership and test evidence matrix.
## 4. Out of Scope
Provisioning infrastructure, claiming 99.9% availability, legal certification.
## 5. Technical Requirements
Specify tenant/user/file/package/load profiles; define dependency health and error budgets; differentiate live and synthetic environments. Define permitted erasure versus retained evidence, backup expiry and current authorization after restore. Validate source dependency graph and required review roles.
## 6. Proposed Implementation Approach
Review topology alternatives/cost; agree privacy/retention obligations; approve workload/recovery targets and QA gates; document deployment/rollback authority and incident escalation.
## 7. Dependencies
None. Release-specific amendments follow scope approval, but baseline review can proceed in parallel.
## 8. Acceptance Criteria
- Given a performance target, then workload/window/error threshold and measurement method are specified.
- Given tenant restore, then object/event reconciliation and authorization checks are required before cutover.
- Given unknown provider, then it remains pending rather than defaulting to a named cloud.
## 9. Testing Requirements
Tabletop migration/restore/outage scenarios and test-layer/source requirement coverage review.
## 10. Definition of Done
Approved operational/QA matrix, named role owners, measurable gates and documented unresolved procurement/legal constraints.
## 11. References
[Operations](../../product/18_DEVOPS_DEPLOYMENT_OPERATIONS.md), [QA](../../technical_design/24_QA_TEST_ARCHITECTURE.md), [DR](../../technical_design/26_DISASTER_RECOVERY.md), [release plan](../../implementation/19_RELEASE_READINESS.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Retention and recovery decisions affect schema/storage; do not defer them until customer data exists.

### Initial local-environment proposal — 2026-10-17

Prepared as the next design step after user approval of the repository/tool bundle. **Proposed, not an approved operational baseline.** No cloud provider, region, SLO or retention period is selected here.

| Boundary | Proposed rule | Verification owner / evidence |
|---|---|---|
| Local data | Synthetic fixtures only; never production credentials, customer bids or copied tenant backups | Engineering/security: fixture and secret scans |
| Network | Bind exposed development ports to loopback; databases/broker/storage internal unless a documented local tool needs access; no public tunnel by default | Engineering: inspected published ports and unauthorized connection tests |
| Startup | Explicit profile selection; preserve volumes; no automatic migration or seed on normal start | FND-003: repeated startup and restart proof |
| Reset | Separate development-only operation requiring explicit target and confirmation; reject non-local configuration | FND-003: wrong-environment and cancellation tests |
| Credentials | Distinct provisioner/migrator/runtime roles; ignored local secret files or references; committed examples contain names, not usable secrets | DB-001/security: runtime DDL denial and repository secret checks |
| Configuration | Required fields validated before serving; refuse invalid ports/URLs, missing secrets and production use of development settings | FND-003: table-driven invalid configuration tests |
| Profiles | Minimal Go/web/PostgreSQL foundation first; add NATS/Redis/mail for identity/jobs and storage/scanner for documents; AI disabled by default | Foundation owners: absence of optional services does not break supported operations |
| Health | Liveness reports process health; readiness reflects required configured dependencies; no healthy status fabricated for unavailable services | API-001/FND-001: actual dependency-outage cases |
| QA | Unit fakes allowed; accepted integration/E2E uses actual scoped dependencies and two independent tenant DBs where applicable | QA-001: reproducible synthetic fixtures and negative isolation evidence |
| Telemetry | Request/correlation identifiers and safe classifications only; no tokens, DSNs, signed URLs, bid bodies or pricing | OBS-001: synthetic secret/content redaction cases |

Before FND-003 can become Ready, approve the local configuration schema, service/image pins and digests, Docker/Compose prerequisites, resource budgets and profile-specific health behavior. Do not treat this proposal or a Docker CLI version as proof that the daemon, Compose or Testcontainers works.

The full ARC-007 ticket also requires CI host, production/provider/region/legal/privacy ownership, workload/SLO and RPO/RTO definitions, retention/holds, backup/reconciliation and incident authority. Those remain unresolved. If only the local subset is approved first, create a reviewed local-baseline child ticket and explicitly amend FND-003's dependency; do not silently treat the entire operations ticket as complete.

### Backend-only prerequisite inspection — 2026-10-17

[Packet 13](../13_BACKEND_FOUNDATION_UNBLOCKING.md) records Docker CLI 29.6.2 and Compose 5.3.1, but the Docker daemon was unreachable at the configured local socket. No container was started or image pulled. The user subsequently approved packet 13's local-only plan with 2 CPUs, 2 GiB RAM and a 10 GiB storage budget. [FND-004](../foundation/BID-FND-004.md) records the bounded child, with runtime, exact-pin/security and configuration-role review still blocking execution. Production, privacy, recovery and owner gates remain open. This ticket remains Proposed.
