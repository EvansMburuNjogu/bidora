# BID-OPS-001 — Enforce reproducible CI, contract and supply-chain gates

## Metadata
- Ticket ID: BID-OPS-001
- Epic: E-OPS
- Type: DevOps
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-FND-002, BID-API-001, BID-ARC-007
- Blockers: Approved CI host and credential policy
- Milestone: M1

## 1. Objective
Make reviewed changes reproducibly build/test/scan before merging or publishing artifacts.
## 2. Business Context
An unnoticed contract or migration regression could break bidder approvals or expose tenant data.
## 3. Scope
Provider-independent check targets, format/types/static analysis, architecture imports, tests, schema compatibility/generation drift, secrets/dependencies/licenses, immutable image/SBOM pipeline as runtimes exist.
## 4. Out of Scope
Pretending nonexistent suites passed, deployment secrets in untrusted PRs, selected cloud by inference.
## 5. Technical Requirements
Pinned tool versions/digests; fail gates explicitly; collect failed/skipped reports; no hidden CI-only logic; grow migration/E2E checks with features; artifact provenance includes commit/config/schema compatibility.
## 6. Proposed Implementation Approach
Implement clean-run targets and pipeline; seed deliberate type/contract/secret violations; prove merge/publish gate failure; document generated-client update and flaky-test handling.
## 7. Dependencies
[Runtime pair](../foundation/BID-FND-002.md), [schema conventions](../backend_contracts/BID-API-001.md), [CI policy](../architecture/BID-ARC-007.md).
## 8. Acceptance Criteria
- Given breaking contract or secret fixture, then pipeline fails before artifact promotion.
- Given clean checkout, then local and CI checks match.
- Given absent test layer, then report says not implemented/skipped rather than passed.
## 9. Testing Requirements
Pipeline negative fixtures, reproducible generation/build, credential-isolation and report retention checks.
## 10. Definition of Done
Reviewed CI and evidence artifacts with reproducible commands and no hidden overrides.
## 11. References
[Operations §5](../../product/18_DEVOPS_DEPLOYMENT_OPERATIONS.md), [CI design](../../technical_design/22_DEVOPS_CICD_DESIGN.md), [DevOps plan](../../implementation/17_DEVOPS_DEPLOYMENT.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Retries must not erase flaky-test defects or failed security scans.
