# Foundation implementation plan

## 1. Purpose
Define BID-T001, the smallest verified engineering slice: a real web-to-Go health interaction.

## 2. Scope
Repository conventions, configuration, Go/Fiber API, Next.js shell, health/error contracts and CI smoke. This is not business-feature completion.

## 3. Product specifications
M01 in [module dependencies](../product/07_MODULE_DEPENDENCIES.md), W01/S02 in [delivery](../product/17_DELIVERY_PLAN_WAVES_SPRINTS.md), OPS-001.

## 4. Technical designs
[Repository](../technical_design/02_REPOSITORY_ARCHITECTURE.md), [backend](../technical_design/08_BACKEND_ARCHITECTURE.md), [CI](../technical_design/22_DEVOPS_CICD_DESIGN.md).

## 5. Prerequisites
Explicit application implementation approval, frontend root decision and toolchain compatibility selection.

## 6. Required decisions
BID-ADR-005 and 008; CI host under 013. Git initialization/remote creation must be requested, not inferred from documentation permission.

## 7. Dependencies
None beyond approved engineering baseline; no business schema or auth bypass needed.

## 8. Target files
Future root README/ignore/config/tooling; backend/cmd/api and config/telemetry adapters; `<frontend-root>/src/app` shell; test and CI files.

## 9. Sequence
Initialize approved Git conventions; pin dependencies; define `/health/live` and `/health/ready` contracts; implement config rejection/safe errors/graceful shutdown; build shell using actual API; add startup/build/test commands and CI. Liveness is process responsiveness; readiness uses only required initialized dependencies.

## 10. Domain/security
No private data in health responses. Do not introduce fake authenticated users, admin bypass flags or mock business endpoints to make the shell attractive.

## 11. Database/API/frontend
A migration runner smoke test may use ephemeral PostgreSQL; no business tables. Health schema is reviewed before handler. UI shows unavailable/starting/healthy accurately.

## 12. Tests
Config missing/invalid, safe error serialization, graceful shutdown, API health contract, Playwright real API success/failure, build and secret scan.

## 13. Acceptance
Clean checkout starts and builds; unavailable API produces honest failure; invalid required config exits clearly without secrets.

## 14. Done
Commands executed in CI and evidence recorded; named foundation-only status.

## 15. Risks
Scaffolding expands into all future modules; keep scope to runtime composition and smoke path.

## 16. Evidence
BID-T001 build/report links, tool versions, command output and file list; no claim of tenant isolation yet.

## 17. Exclusions
Identity, tenants, documents, AI, billing, procurement, sealed bidding and all application work during the current documentation task.
