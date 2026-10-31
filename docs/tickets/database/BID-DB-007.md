# BID-DB-007 — Persist package fingerprints, external proof and outcome corrections

## Metadata
- Ticket ID: BID-DB-007
- Epic: E-DB
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-DB-006, BID-API-006
- Blockers: Approved package/outcome policies
- Milestone: M4

## 1. Objective
Maintain a verifiable candidate-to-package-to-proof chain and append-only outcome history.
## 2. Business Context
Bid teams need to show what was submitted, when they reported it and what later happened.
## 3. Scope
Packages/jobs/fingerprint constraints, external proof, confirmations, reported/server timestamps, outcome supersedes chain and authorized history queries.
## 4. Out of Scope
External delivery automation or overwriting accepted evidence.
## 5. Technical Requirements
Unique candidate/fingerprint; success requires complete immutable object/hash fields; same-bid package/proof linkage; clean proof guard; correction chain acyclic; permanent domain dedup beyond short-lived idempotency expiry.
## 6. Proposed Implementation Approach
Add reviewed migrations/sqlc queries; serialize completion/confirmation; test package metadata reconciliation and immutable successful artifact fields; index bid/history cursors.
## 7. Dependencies
[Candidate schema](BID-DB-006.md), [proof/outcome contracts](../backend_contracts/BID-API-006.md).
## 8. Acceptance Criteria
- Given duplicate generation fingerprint, then only one package identity exists.
- Given package from another bid, then evidence linkage is rejected.
- Given corrected outcome, then prior facts remain queryable and cycles are rejected.
## 9. Testing Requirements
Constraint/race/hash fields, timestamp provenance, correction history, immutable artifact and upgrade tests.
## 10. Definition of Done
Verified persistence and query boundaries with negative evidence and recovery notes.
## 11. References
[BR-026](../../product/02_BRD.md), [PR-TP-014](../../product/03_PRD.md), [schema](../../technical_design/06_DATABASE_SCHEMA_DESIGN.md), [DB plan](../../implementation/05_DATABASE_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Recording proof does not prove actual external acceptance.
