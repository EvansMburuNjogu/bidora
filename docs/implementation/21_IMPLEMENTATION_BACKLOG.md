# Ordered implementation backlog

## 1. Purpose
Provide bounded, dependency-aware engineering tasks ready for approval and elaboration.

## 2. Scope
Alignment through external MVP; identifiers BID-Txxx are new engineering task IDs, not source requirement IDs.

## 3. Product specifications
[Module dependencies](../product/07_MODULE_DEPENDENCIES.md), [delivery plan](../product/17_DELIVERY_PLAN_WAVES_SPRINTS.md), [evidence](../product/19_TRACEABILITY_ACCEPTANCE_EVIDENCE.md).

## 4. Technical designs
[Decision register](../technical_design/27_ARCHITECTURE_DECISIONS.md), [traceability](../technical_design/29_REQUIREMENTS_TRACEABILITY.md), [schema](../technical_design/06_DATABASE_SCHEMA_DESIGN.md).

## 5. Prerequisites
Explicit implementation authorization; approval of task packet and its decision blockers. Documentation generation does not satisfy this.

## 6. Required decisions
Task rows reference blockers; decision acceptance requires owner/date/evidence, not an agent inference.

## 7. Dependencies
Use [matrix](23_DEPENDENCY_MATRIX.md); identity/audit before exposed provisioning, contracts before handlers, approved evidence before packaging.

## 8. Target components
Paths follow [repository design](../technical_design/02_REPOSITORY_ARCHITECTURE.md). Each task lists only bounded owner components; no unrelated refactors.

## 9. Ordered tasks

| Task | Concrete change / target | Depends / decision | Definition of done / evidence |
|---|---|---|---|
| BID-T000 | Approve release/path/identity policy; record canonical decisions and scope mapping | Product/architecture/security | Accepted/deferred decisions, source conflicts mapped, blockers owned; no source rewriting without approval |
| BID-T001 | Git conventions and API/web health slice; root tooling, API composition, shell, CI | T000 path/tool approval | Clean-start/build/config/health tests; truthful unavailable API UI |
| BID-T002 | Control/tenant migration runner, sqlc/TxScope baseline, least-privilege roles | T001, ADR-008 | Clean/upgrade/rollback tests and denied runtime DDL |
| BID-T003 | Identity contract and verified session/recovery/MFA implementation | T002, ADR-007 | Browser verification/login/logout, replay/CSRF/enumeration/rotation tests |
| BID-T004 | Membership capability/context and atomic audit/outbox foundation | T003 | Grant-boundary/last-admin/context tests; mutation/audit/outbox rollback proof |
| BID-T005 | Resumable tenant provisioning and bounded router | T004, ADR-013/015 | Two tenant isolation, crash-per-checkpoint, retry and restore-to-new evidence |
| BID-T006 | NATS durable worker/inbox/job status and notification adapter | T004; integrate T005 tenancy | Publish/commit/ack fault injection; scoped job query and honest UI state |
| BID-T007 | Manual opportunity verification and human Bid/No-Bid | T005, approved decision policy | Source/timezone/duplicate/stale tests; real created bid E2E |
| BID-T008 | Secure uploads/versioned vault and scan pipeline | T005/T006, ADR-017 | Quarantine/malware/type/hash/overwrite/signed URL tests; real uploader/vault |
| BID-T009 | Requirement ownership/evidence and deterministic readiness | T007/T008 | Unsupported SATISFIED/expiry/foreign evidence denied; keyboard matrix E2E |
| BID-T010 | Technical revisions and restricted financial schedule | T009, approved arithmetic/format | Concurrency/decimal/XSS/price leakage tests; contributor workflow |
| BID-T011 | Immutable candidate and version-specific human approval | T010, ADR-011 | Approve/edit/revoke races; immutable decisions; changes-requested UI |
| BID-T012 | Retry-safe package generation and authorized download | T011/T006, ADR-017 fingerprint | Manifest/bytes reconcile, storage-success/DB-failure recovery, pricing restriction |
| BID-T013 | External evidence capture/confirmation and mark-submitted | T012/T008, ADR-011 exception | Exact package proof, separate reported/server time, duplicate/lost response tests |
| BID-T014 | Outcome correction history and scoped portfolio | T013, ADR-012 | Approved transitions/history and authorized reports; full external journey |
| BID-T015 | Pilot staging promotion, recovery, security, load and accessibility acceptance | T014, ADR-006/013/015 | Signed release package, measured recovery/load, no blocking defects |
| BID-T016 | Approved-source search and grounded AI discovery design/implementation | T015 or separately approved increment, ADR-014 | Corpus, source terms, privacy, citation and isolation evidence; not first MVP |
| BID-T017 | Closed-loop publication/submission/opening release-specific schemas/contracts | Authoring foundations, ADR-009/010 and governance | Complete future schemas/cards, independent crypto review and failure/deadline evidence before handlers |

## 10. Domain/security
Each packet contains actor/capability/state matrix, audit, idempotency and immutable references. Missing approval policy blocks the command, not just a UI button.

## 11. Database/API/frontend
Tasks are cross-layer outcomes. Split large tasks into smaller complete commands if needed; do not turn table rows into unrestricted CRUD tickets.

## 12. Tests
Attach unit, DB, contract, negative authorization and UI cases relevant to each row; regress previously accepted journey.

## 13. Acceptance
Demonstrate operation and failure recovery using real dependencies; satisfy task evidence column.

## 14. Done
Approved scope, reviewed files, executed checks, evidence and known limitations per [DoD](22_DEFINITION_OF_DONE.md).

## 15. Risks
T006 worker foundation must not delay all synchronous prototype work, but no queued feature ships before it; avoid circular prerequisites by separating identity from tenant membership.

## 16. Evidence
Record task ID, actual source requirement IDs, build/schema/contracts, commands/results and decision dependencies.

## 17. Exclusions
No task is executed by this document; no automatic Git initialization, dependency install or source-spec rename.
