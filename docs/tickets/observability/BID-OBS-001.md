# BID-OBS-001 — Add classified structured logs, correlation and redaction tests

## Metadata
- Ticket ID: BID-OBS-001
- Epic: E-OBS
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: S
- Dependencies: BID-FND-001, BID-ARC-007
- Blockers: Approved safe telemetry fields/retention
- Milestone: M1

## 1. Objective
Make requests diagnosable before sensitive commands exist, without logging confidential content.
## 2. Business Context
Operators need to investigate failed bid actions while preserving privacy and evidence integrity.
## 3. Scope
Structured logger, validated request/trace ID, safe error fields, OpenTelemetry context interface and synthetic secret redaction assertions.
## 4. Out of Scope
Business audit persistence owned by DB/BE tickets, production dashboard claims, raw payload logging.
## 5. Technical Requirements
Bounded operation/result fields; no tokens, DSNs, signed URLs, pricing or document bodies; preserve wrapped diagnostic errors safely; trace IDs returned per API-001; configurable sampling cannot sample away business audit.
## 6. Proposed Implementation Approach
Define field allowlist; wire HTTP/error composition; add request-to-transaction correlation port; inject synthetic sensitive values and inspect outputs; document future worker propagation.
## 7. Dependencies
[Runtime](../foundation/BID-FND-001.md), [observability policy](../architecture/BID-ARC-007.md).
## 8. Acceptance Criteria
- Given invalid request, then safe error includes correlatable ID but no internal locator.
- Given synthetic secret in provider error, then serialized logs redact it.
- Given malformed client trace header, then validated server ID replaces it.
## 9. Testing Requirements
Logger/error serialization unit tests, HTTP correlation and secret-leak regression fixtures.
## 10. Definition of Done
Actual redaction/correlation evidence and documented safe field convention.
## 11. References
[SEC-008](../../product/04_SRD.md), [observability](../../technical_design/23_OBSERVABILITY_OPERATIONS.md), [observability plan](../../implementation/18_OBSERVABILITY_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Diagnostic logs are not immutable business audit evidence.
