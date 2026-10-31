# BID-ARC-002 — Approve repository, toolchain and agent execution conventions

## Metadata
- Ticket ID: BID-ARC-002
- Epic: E-ARC
- Type: Design
- Priority: P0
- Status: Proposed
- Complexity: S
- Dependencies: None
- Blockers: Exact version/convention acceptance and engineering compatibility/security review; repository/tool bundle approved
- Milestone: M0

## 1. Objective
Select one frontend root and a reproducible supported toolchain before scaffolding.
## 2. Business Context
Duplicate applications and incompatible migrations would slow every bid workflow and confuse coding agents.
## 3. Scope
Resolve observed empty web versus described frontend; retain docs/product; select migration tooling, compatible Go/Fiber/Node/PostgreSQL versions, package manager, import boundaries, branch/review conventions and root AGENTS interpretation.
## 4. Out of Scope
Installing tools, initializing Git, modifying AGENTS or source designs during ticket generation.
## 5. Technical Requirements
Record exact version-selection evidence and future pin locations; decide generated-file policy. Preserve modular-monolith ownership. Review all AGENTS paths and conflict-escalation rules; propose corrections through separately authorized changes.
## 6. Proposed Implementation Approach
Compare layout alternatives, verify official compatibility requirements, choose tooling with a small future proof plan, review conventions with frontend/backend/operations owners.
## 7. Dependencies
None. Accept BID-ADR-005/008 or an explicit replacement; never create both web and frontend.
## 8. Acceptance Criteria
- Given conflicting paths, when decision is signed, then all future task targets resolve to exactly one frontend root.
- Given an unsupported version combination, then it is rejected with cited compatibility evidence.
- Given agent instructions conflict, then escalation is recorded rather than weakened.
## 9. Testing Requirements
Path/reference inventory and dependency-direction review; runnable proof belongs to foundation tickets.
## 10. Definition of Done
Approved layout/tool register and conventions, AGENTS review findings and change authorization boundaries recorded.
## 11. References
[LLD](../../product/09_LLD.md), [repository](../../technical_design/02_REPOSITORY_ARCHITECTURE.md), [standards](../../implementation/02_ENGINEERING_STANDARDS.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Version recommendations are not installation evidence. Existing directories remain unchanged in this task.

### Execution record — 2026-10-17
Prepared [repository/toolchain decision packet](../10_FOUNDATION_DECISION_PACKET.md), recommending existing `web/`/`backend/`, Goose and npm. Recorded actual host inventory, upstream Go/module/npm metadata, official PostgreSQL/Next requirements, exact candidate versions and future pin locations. Official Node release metadata failed DNS resolution; support/patch verification remains open. Reviewed AGENTS and module/generated-file/review conventions. No installs, scaffolding, Git initialization or runtime proof executed. Subsequent user response **“apporve and proceed”** approves `web/`, `backend/`, Goose and npm; BID-ADR-005/008 and root AGENTS are synchronized. Node release metadata succeeded on retry: revised candidates are Node 24.22.0 and bundled npm 11.19.1, without installation. Status remains Proposed pending exact version/convention and compatibility/security review. ARC-007 now contains an initial local-environment proposal as the next prerequisite-design step.

Subsequent user authorization enabled the bounded health implementation. [Actual verified versions and checks](../12_HEALTH_SLICE_EVIDENCE.md) now record Go 1.27.2/Fiber 3.6.1 and the tested local Node 24.16.0/npm 11.13.0/Next 16.4.2/TypeScript 5.9.3 baseline. This supersedes candidate-only claims for that slice; broader version/security/convention and owner review remain open.

### Backend-only blocker review — 2026-10-17

[Packet 13](../13_BACKEND_FOUNDATION_UNBLOCKING.md) separates the approved roots/tools and verified health baseline from unresolved database/tool candidates. Fresh Go race tests and vet passed. The packet proposes explicit local-only child tickets rather than incorrectly closing this full ticket or requiring completed migration-runtime proof before a scoped foundation task can start. Candidate acceptance and reviewer evidence remain pending; status is unchanged.
