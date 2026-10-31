# Implementation strategy

## 1. Purpose
Turn specifications into accepted user operations, not a large speculative scaffold. This is instruction for future authorized work; no application implementation is authorized now.

## 2. Scope
Approved release scope (BID-ADR-006, Option A): External Tender Pilot Increment — identity → isolated workspace → manually verified external opportunity/file attachments → documents → matrix → response → human approval → immutable package → external evidence → reported outcome. This is not acceptance of the full BRD/PRD MVP or all R2 capabilities. AI/search, payment automation and marketplace remain later; detailed exclusions and remaining review gates are in the [approved scope packet](../tickets/09_RELEASE_SCOPE_REVIEW.md). Application implementation is not yet authorized.

## 3. Product specifications
[Roadmap R2](../product/16_PRODUCT_ROADMAP.md), [delivery W00–W07](../product/17_DELIVERY_PLAN_WAVES_SPRINTS.md), BR-020–026, PR-TP-004–014. Broader BRD/PRD acceptance is not silently superseded.

## 4. Technical designs
[System](../technical_design/01_SYSTEM_ARCHITECTURE.md), [decisions](../technical_design/27_ARCHITECTURE_DECISIONS.md), [traceability](../technical_design/29_REQUIREMENTS_TRACEABILITY.md).

## 5. Prerequisites
Approved bounded task, accountable product/security/engineering owners and access to actual specifications.

## 6. Required decisions
BID-ADR-006 release scope, BID-ADR-005 `web/` frontend root and BID-ADR-008 Goose/npm tooling are approved. Exact version validation remains outstanding. Identity/policy/provider decisions and outstanding scope-review evidence still gate their dependent slices, not unrelated documentation.

## 7. Dependencies
Identity/policy/audit precede exposed provisioning. Secure documents precede evidence; immutable candidates precede approvals/packages. Contracts are approved before corresponding handlers.

## 8. Target components and files
Future runtime paths follow approved repository design; ticket/evidence artifacts link the source requirement and slice. Never create both web and frontend apps.

## 9. Sequence
Phase 0 align decisions; Phase 1 foundation; Phase 2 database/constraints; Phase 3 backend including identity prerequisites; Phase 4 contract-governance tooling; Phase 5 frontend; Phase 6 documents/integrations; Phase 7 later AI/search; Phase 8 continuous QA/security plus release hardening; Phase 9 deployment/pilot. These are workstreams, not waterfall permission to defer API/security design. Each vertical slice crosses the needed phases before acceptance. BID-ADR-006 permits the scoped increment to precede Phase 7; shipping still requires actual QA/security/recovery/UAT evidence and human release authorization.

## 10. Domain/security
Preserve tenant isolation, human decisions and immutable evidence. No unapproved waiver, auto-submission or default support access.

## 11. Database/API/frontend
For each operation approve schema/transition, contract and page state model together. A rendered screen without real permitted command is not delivered.

## 12. Tests
Use [QA plan](16_QA_TEST_IMPLEMENTATION.md) per slice, including denied/race/failure paths, not only success E2E.

## 13. Acceptance
One user can complete the scoped operation against real dependencies, with traceable evidence and no manual DB repair.

## 14. Done
Use [definition of done](22_DEFINITION_OF_DONE.md); unverified work is IMPLEMENTED_NOT_VERIFIED, not complete.

## 15. Risks
Scope creep, invented defaults and infrastructure overload. Gate decisions and introduce services only when their slice needs them.

## 16. Evidence
Requirement → design → task → test/report → UAT/release. Record exact commands, results and skipped checks.

## 17. Exclusions
No framework generators, packages, executable SQL or infrastructure changes in this documentation task; no full-platform completion claim.
