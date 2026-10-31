# BID-BE-010 — Design stage-restricted evaluation, score locks and moderation

## Metadata
- Ticket ID: BID-BE-010
- Epic: E-BE
- Type: Design
- Priority: P2
- Status: Blocked
- Complexity: M
- Dependencies: BID-DB-008, BID-API-009, BID-BE-013
- Blockers: Approved evaluation/recusal/consolidation procedure
- Milestone: M8

## 1. Objective
Specify accountable evaluation commands against published criteria and released submissions.
## 2. Business Context
Independent evaluators must not see prohibited pricing or influence each other's scores prematurely.
## 3. Scope
Assignment/conflict declaration, preliminary findings/disqualification, technical/financial stage access, evidence-linked scores, locks/unlock reasons, moderation and recommendation input; split runtime packets by stage.
## 4. Out of Scope
Autonomous AI scores, unpublished criteria, actual complete evaluation implementation.
## 5. Technical Requirements
RULE-EVAL-001–005; criterion scale/version identity; independent score visibility; human rationale; governed arithmetic corrections; current assignment/assurance; append-only unlock/change evidence and concurrency semantics.
## 6. Proposed Implementation Approach
Build actor/stage matrix; map approved data model/contract to commands; define score lock races and consolidated calculation fixtures; specify evaluator workspace tests.
## 7. Dependencies
[Schema](../database/BID-DB-008.md), [contracts](../backend_contracts/BID-API-009.md), [opening packet](BID-BE-013.md).
## 8. Acceptance Criteria
- Given technical stage only, then financial envelope access is denied.
- Given score lock, then edits require approved unlock/reason.
- Given absent published criterion, then no score can be entered.
## 9. Testing Requirements
Stage/assignment/recusal matrix, score bounds/concurrency, independence, audit and publication-version fixtures.
## 10. Definition of Done
Reviewed stage packets and implementation child-ticket criteria with deterministic test oracles.
## 11. References
[PR-PT-009/010](../../product/03_PRD.md), [modules](../../technical_design/09_BACKEND_MODULE_DESIGN.md), [backend plan](../../implementation/06_BACKEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
AI evidence suggestions remain distinct from human final findings.
