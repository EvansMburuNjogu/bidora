# Observability implementation

## 1. Purpose
Make tenant operations, asynchronous failures and evidence integrity diagnosable without exposing confidential content.

## 2. Scope
Structured logs, OpenTelemetry traces, Prometheus-compatible metrics, audit and operational alerts/runbooks.

## 3. Product specifications
[Operations §§11–17](../product/18_DEVOPS_DEPLOYMENT_OPERATIONS.md), BR-050, SEC-008, SRD §14.

## 4. Technical designs
[Operations](../technical_design/23_OBSERVABILITY_OPERATIONS.md), [audit/evidence](../technical_design/20_AUDIT_LOGGING_EVIDENCE.md).

## 5. Prerequisites
Operation/error vocabulary, classification policy and owner for each alert.

## 6. Required decisions
BID-ADR-013 backend/retention/SLO/incident ownership. Numerical alert thresholds require actual load baseline.

## 7. Dependencies
Request IDs/safe logs in foundation; command audit in every business slice; worker correlation before asynchronous acceptance.

## 8. Target files
Future telemetry adapters, owner-module metrics/audit calls, dashboards, alert rules and runbooks.

## 9. Sequence
Define safe field allowlist; add trace propagation; instrument DB pool wait and job/outbox age; append business audit inside transaction; add bounded-label metrics; alert on user-impacting backlog/failure; exercise triage and containment.

## 10. Domain/security
No raw bid text/pricing, tokens, secrets, document bytes or signed URLs. Tenant IDs may be restricted log fields, not unbounded metric labels. Audit is not sampled diagnostic output.

## 11. Database/API/frontend
Audit failure aborts material mutation; API returns trace ID; UI reports error code/trace without telemetry leakage. 202 does not increment completed-operation metric.

## 12. Tests
Inject DB/broker/scanner failure; verify trace joins and alert; seed synthetic secrets to test redaction; deny audit updates; verify business success counter only after authoritative commit.

## 13. Acceptance
Operator identifies failed tenant/job and safe recovery action from telemetry without private content; alert delivered to named owner.

## 14. Done
Dashboard/runbook reviewed, redaction tests and real alert drill evidence recorded.

## 15. Risks
High-cardinality metrics, sensitive traces, silent sampling of critical audit, decorative dashboards. Separate audit and monitor oldest job/error budgets.

## 16. Evidence
Redacted trace, metric query, alert delivery timestamp, incident exercise and audit transaction test.

## 17. Exclusions
Logging complete payloads for convenience, pretending monitoring exists before deployment, public tenant operational details.
