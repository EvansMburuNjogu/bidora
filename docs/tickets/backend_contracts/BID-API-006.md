# BID-API-006 — Specify package, external proof and outcome history contracts

## Metadata
- Ticket ID: BID-API-006
- Epic: E-API
- Type: Design
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-API-004, BID-API-005, BID-ARC-003
- Blockers: Evidence exception/outcome policy and format approval
- Milestone: M4

## 1. Objective
Define retry-safe package and external submission recording without implying external delivery.
## 2. Business Context
A bidder needs a verifiable approved package and historical proof for later dispute/outcome analysis.
## 3. Scope
Request/status/download package; proof registration/confirmation; mark-submitted; outcome registration/correction/history; permitted audit timeline.
## 4. Out of Scope
Automatic external portal submission, sealed platform acceptance and invented receipt timestamps.
## 5. Technical Requirements
Candidate/profile/fingerprint identity; whole-package classification; server recorded_at versus user reported_submitted_at; proof document version and same-bid package linkage; stable idempotency; append-only corrections; 202 versus success distinction.
## 6. Proposed Implementation Approach
Specify command/read cards and examples; define post-commit response-loss behavior; align Job DTO; review status labels with product owner; include restrictive negative examples.
## 7. Dependencies
[Documents/jobs](BID-API-004.md), [approval](BID-API-005.md), [outcome/proof policy](../architecture/BID-ARC-003.md).
## 8. Acceptance Criteria
- Given evidence from another bid, then contract rejects linkage.
- Given late capture, then both times remain distinguishable.
- Given repeated accepted command, then response references original fact rather than a second submission.
## 9. Testing Requirements
Example validation for missing proof, stale readiness, forbidden package, duplicate key/body mismatch and correction chain.
## 10. Definition of Done
Signed schemas and semantics, safe audit/history reads and compatibility evidence.
## 11. References
[PR-TP-013/014](../../product/03_PRD.md), [audit design](../../technical_design/20_AUDIT_LOGGING_EVIDENCE.md), [contracts plan](../../implementation/07_BACKEND_CONTRACTS_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
No evidence-free transition unless explicitly authorized policy is designed and tested.
