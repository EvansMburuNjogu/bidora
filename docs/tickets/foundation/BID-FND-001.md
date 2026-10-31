# BID-FND-001 — Establish reviewed repository and runnable Go health boundary

## Metadata
- Ticket ID: BID-FND-001
- Epic: E-FND
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-ARC-002, BID-API-001, BID-FND-003
- Blockers: Approved layout/toolchain and explicit implementation authorization
- Milestone: M1

## 1. Objective
Create a reproducible Go/Fiber process with safe startup, health and shutdown behavior.
## 2. Business Context
Every later bid operation needs predictable deployment and diagnostics rather than ad hoc boot scripts.
## 3. Scope
Authorized Git conventions, Go composition root, pinned modules, health handlers, request IDs, safe errors, README and build/check targets.
## 4. Out of Scope
Identity bypass, business tables, mock tender dashboard, remote repository creation without approval.
## 5. Technical Requirements
Implement approved live/ready contract; validate config before serving; Fiber confined to adapter; readiness checks required dependencies without exposing locators. Drain requests and close pools on bounded shutdown.
## 6. Proposed Implementation Approach
Initialize authorized repository files; pin compatible tooling; wire config/logger/HTTP adapters; add health contracts and tests; document clean-start workflow.
## 7. Dependencies
[Conventions](../architecture/BID-ARC-002.md), [contracts](../backend_contracts/BID-API-001.md), [environment](BID-FND-003.md).
## 8. Acceptance Criteria
- Given clean checkout, when documented build/start runs, then health returns approved safe shape.
- Given invalid secret/config reference, then startup fails without printing its value.
- Given shutdown, then new traffic stops and in-flight work drains within configured bound.
## 9. Testing Requirements
Unit config/error tests, HTTP conformance, signal lifecycle and secret-log assertions; record exact commands.
## 10. Definition of Done
Reviewed runnable foundation, pinned dependencies and reproducible evidence; not business-feature completion.
## 11. References
[M01](../../product/07_MODULE_DEPENDENCIES.md), [backend](../../technical_design/08_BACKEND_ARCHITECTURE.md), [foundation plan](../../implementation/04_FOUNDATION_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Ticket generation does not authorize executing any setup command.

### Authorized health-only increment
The user subsequently authorized dependency installation and the real Go/web health slice. Loopback-only configuration, Go/Fiber health/error handlers, safe correlation and bounded shutdown now have centralized race-tested suites and live HTTP/process-exit evidence. See [execution report](../12_HEALTH_SLICE_EVIDENCE.md). No database, business migrations, full service profiles, Git or CI infrastructure was created. Full ticket acceptance and remaining prerequisites stay open; this is not a blanket Done claim.
