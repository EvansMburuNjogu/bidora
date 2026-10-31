# BID-API-008 — Generate typed clients and enforce contract compatibility in CI

## Metadata
- Ticket ID: BID-API-008
- Epic: E-API
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-API-002, BID-FND-002
- Blockers: Approved generator/toolchain and reviewed schema baseline
- Milestone: M2

## 1. Objective
Keep frontend DTOs and backend binding aligned with approved contracts.
## 2. Business Context
Contract drift can lose user edits or misrepresent approval/submission state.
## 3. Scope
Reproducible Go/TypeScript generation or validation, centralized frontend error mapping, schema example checks and breaking-change detection. Add later approved APIs incrementally.
## 4. Out of Scope
Generating clients from unapproved future endpoints, domain models as DTOs, hand-editing generated output.
## 5. Technical Requirements
Pin generators; separate server/client types; preserve decimal strings/nullability; handle 401/403/concealed404/409/429/503; fail CI on drift or incompatible request/enum changes.
## 6. Proposed Implementation Approach
Generate initial auth/workspace baseline; wrap transport/context/CSRF; validate fixtures; inject deliberate schema changes to prove gate; document update workflow.
## 7. Dependencies
[Identity/workspace contracts](BID-API-002.md), [web foundation](../foundation/BID-FND-002.md). Each later frontend ticket additionally requires its specific API ticket.
## 8. Acceptance Criteria
- Given clean regeneration, then output is identical.
- Given breaking field change, then compatibility check fails.
- Given stale version response, then client exposes conflict rather than retrying a changed payload automatically.
## 9. Testing Requirements
Generator reproducibility, transport/error unit tests and approved response conformance fixtures.
## 10. Definition of Done
CI-checkable generation, typed transport and documented versioned update path.
## 11. References
[API-008](../../product/04_SRD.md), [API](../../technical_design/10_API_CONTRACT_DESIGN.md), [contracts plan](../../implementation/07_BACKEND_CONTRACTS_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Initial client generation does not imply future business schemas are approved.
