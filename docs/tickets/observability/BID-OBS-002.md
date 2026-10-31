# BID-OBS-002 — Monitor tenant/job health and exercise actionable incident alerts

## Metadata
- Ticket ID: BID-OBS-002
- Epic: E-OBS
- Type: DevOps
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-OBS-001, BID-EVT-002, BID-TEN-002, BID-DOC-002
- Blockers: Approved SLO/alert ownership and telemetry backend
- Milestone: M4

## 1. Objective
Connect user-impacting failures to safe operational recovery before pilot.
## 2. Business Context
Stuck scans/packages or tenant DB outages must not remain invisible while users face bid deadlines.
## 3. Scope
Bounded-label latency/error/pool/queue/job/scan/provisioning metrics, end-to-end traces, dashboards, oldest-job/backup/auth alerts and runbooks.
## 4. Out of Scope
Per-user/tenant metric-cardinality explosion, fake availability claims and private content telemetry.
## 5. Technical Requirements
Correlation through outbox/worker; distinguish accepted versus completed operations; explicit alert owner/escalation, safe scoped retry and containment; redaction and audit/diagnostic separation.
## 6. Proposed Implementation Approach
Instrument owners' ports; provision reviewed local/staging telemetry definitions; create symptom→diagnosis→containment→verification runbooks; inject outages and verify delivery.
## 7. Dependencies
[Logging](BID-OBS-001.md), [jobs](../events_workers/BID-EVT-002.md), [router](../tenancy/BID-TEN-002.md), [documents](../documents/BID-DOC-002.md).
## 8. Acceptance Criteria
- Given scanner/broker/DB failure, then user state and alert agree and named owner receives it.
- Given queued job, then completion metric remains unchanged until durable success.
- Given replay action, then scope/reason/actor are audited.
## 9. Testing Requirements
Outage injection, alert transport, cardinality/redaction, trace linkage and runbook exercise by a second engineer.
## 10. Definition of Done
Actionable monitoring and incident evidence, not only dashboard screenshots.
## 11. References
[Operations §11](../../product/18_DEVOPS_DEPLOYMENT_OPERATIONS.md), [operations design](../../technical_design/23_OBSERVABILITY_OPERATIONS.md), [observability plan](../../implementation/18_OBSERVABILITY_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
BID-OBS-002A adds unmounted, pull-based health/readiness/low-cardinality metrics adapters for document measurement, entitlement acknowledgement dispatch and completion reconciliation as `IMPLEMENTED_NOT_VERIFIED`. They require explicit probe deadlines and staleness thresholds, retain no dynamic identifiers or causes, and do not claim durable business evidence. Telemetry export, routes, runtime composition, SLO/alert policy and outage evidence remain blocked.

Thresholds must be based on approved baseline and workload, not arbitrary defaults.
