# Ticket standards and execution gates

## 1. Identity and structure

Individual files live at `docs/tickets/<category>/BID-<PREFIX>-NNN.md`. IDs are immutable, unique and never recycled. Epics use E-*; milestones M0–M8 are local planning identifiers, not product release IDs or original BID-T task numbers.

Required metadata: Ticket ID, Epic, Type, Priority, Status, Complexity, Dependencies, Blockers, Milestone. Proposed owner roles are in epic/decision context; assign accountable people before Ready without inventing assignments.

Required sections, in order:
1. Objective — one measurable outcome.
2. Business Context — actor/value/procurement significance.
3. Scope — concrete records, operations and interfaces.
4. Out of Scope — prevent accidental platform-sized work.
5. Technical Requirements — exact policy/transaction/isolation constraints.
6. Proposed Implementation Approach — bounded sequence and recovery.
7. Dependencies — links and approval gates, consistent with metadata.
8. Acceptance Criteria — observable positive/negative outcomes, preferably Given/When/Then.
9. Testing Requirements — actual relevant layers and failure cases.
10. Definition of Done — evidence, docs and acceptance, not just merged code.
11. References — exact relevant source/design/implementation paths and source IDs/sections where known.
12. Risks and Notes — uncertainty, failure/recovery and deferred work.

## 2. Priority, size and type

- P0: integrity/foundation or prerequisite blocking scoped delivery; not automatically Ready.
- P1: required scoped release completion/verification; some P1 decisions block P0 implementation legitimately.
- P2: later capability, not permission to omit its source acceptance permanently.
- S: bounded small interface/operation; M: one coherent operation or adapter across layers; L: material multi-operation/uncertain packet. Sizes are not time estimates.
- Types distinguish Engineering, Design, Documentation, Testing, Security and DevOps. Some design work uses Documentation type; acceptance must remain artifact-level. L implementation work must split before Ready if it cannot be independently reviewed/recovered. In particular AUTH-001 must elaborate the selected identity adapter and independently testable session/MFA/recovery increments after ARC-005.

## 3. Status lifecycle

`Proposed → Ready → In Progress → In Review → Done`, with `Blocked` possible whenever prerequisites/decisions/evidence are missing. Reopen Done if required acceptance is invalidated; preserve prior evidence. Cancelled/deferred work keeps its ID and successor rationale.

Current full-ticket statuses: 8 Proposed, 89 Blocked, 7 IMPLEMENTED_NOT_VERIFIED and 1 In Review. FND-004 retains its runtime/technical blockers; FND-005 is the explicitly authorized Docker-free preparation split, manually implemented and technically verified pending user acceptance. A separately authorized [bounded health increment](12_HEALTH_SLICE_EVIDENCE.md) has runtime evidence; full foundation and business ticket gates remain open. Proposed discovery tasks are candidates for owner review, **not** automatically Ready. Documentation authorization does not authorize the application work described in tickets.

## 4. Definition of Ready

All are required for the selected bounded task:
- Explicit authorization for permitted files and work type; accountable product/technical reviewer assigned.
- Applicable decision statuses accepted with owner/date/evidence; no unresolved domain/legal/privacy rule affecting operation.
- Hard dependency tickets accepted at their required evidence level. Design-only dependency completion never substitutes for missing runtime prerequisites.
- Approved scope, source requirement mapping, actor/capability/state/assurance matrix and policy on ambiguity.
- Reviewed per-operation API/event/page contracts before dependent handlers/UI; reviewed persistence fields/constraints/migrations before data code.
- Named acceptance oracle, negative/concurrency/recovery cases and representative synthetic fixtures.
- Compatible rollout, feature exposure, telemetry and recovery plan appropriate to impact.
- Complexity is bounded; required future child tickets exist and are linked. No unknown IDs or circular dependencies.

A decision approval unblocks only affected work. Sealing is not required for manual external workflow; conversely external scope approval cannot authorize opening/awards.

## 5. Contract-first implementation rules

Follow approved contract → reviewed schema/domain → server policy/transaction → real typed frontend → integrated evidence. Schema and API reviews may iterate together before acceptance; do not encode that collaboration as a dependency cycle.

API cards cover request/response, decimal/time formats, permissions, validation, errors, concurrency, idempotency and side effects. Event envelopes are provenance, not credentials. Clients are generated/validated from accepted schema and compatibility-gated; UI cannot invent DTOs.

Go module owners expose application ports, not repositories. One owning DB transaction commits state/audit/outbox; durable checkpoints reconcile storage/broker/control/tenant operations. No SQL transaction during scanner/renderer/email/model/KMS calls. Inbox and effect-level keys handle at-least-once delivery; do not assert exactly-once external effects.

## 6. Mandatory domain/security checks

- Server-authorized tenant routing only; no fallback DB, cross-tenant join, private public/control data or unverified selector.
- Check current authority at execution and result/download; tenant/context-scoped cache/SSR; revoke stale sessions and suppress stale UI results.
- Separate technical and pricing views; absence from payload, not CSS hiding.
- Clean exact immutable document version; quarantine on scan uncertainty; no unsafe preview or object overwrite.
- Source/version, evidence, response candidate, approval and package links immutable; edits invalidate applicability without rewriting history.
- Mandatory requirements cannot become satisfied through unsupported override; human Bid/No-Bid/review/submission/outcome decisions remain explicit.
- Reported external submission time differs from server capture time; no unsupported external acceptance claim.
- AI remains advisory, cited, budgeted and permission-scoped; no approve/submit/open/award tools.
- Future sealed time/KMS/key-release decisions and independent review are hard gates.

## 7. Definition of Done and evidence record

For Engineering/DevOps: approved scope implemented and reviewed; applicable unit/DB/API/policy/isolation/concurrency/recovery/UI tests actually run; contract/generated-code/migration compatibility checks pass; accessibility/security/load/restore evidence as required; updated authorized docs/runbooks; no known blocking defects. Merge or passing lint alone is insufficient.

For Design/Documentation: reviewed artifact covers declared decisions/schemas/contracts/failure matrices with owner/date/evidence and remaining gates. This is design completion only, never runtime completion.

For Testing/Security: executed scope/report, environment/build/schema/contract versions, actual results and limitations; findings assigned and required remediation verified. A test plan alone does not satisfy a runtime testing ticket.

Evidence record: ID, source/ADR versions, reviewer, commit/artifact/schema/contracts, changed files, exact commands and pass/fail/skipped output, sanitized artifacts, recovery/rollout status, residual risks and UAT/release disposition. Never fabricate results or use private production data in fixtures/screenshots.

## 8. Backlog maintenance

Update individual metadata and master row together. Revalidate unique IDs, links, epics/milestones, all prerequisites, graph acyclicity and release ordering. Add explicit dependencies when designs produce runtime children. Keep cancelled/deferred source coverage visible. Documentation checks are separate from application tests.

References: [AGENTS](../../AGENTS.md), [engineering standards](../implementation/02_ENGINEERING_STANDARDS.md), [definition of done](../implementation/22_DEFINITION_OF_DONE.md), [QA Ready/Done §§17–18](../product/15_QA_TEST_STRATEGY.md), [decision registry](../technical_design/27_ARCHITECTURE_DECISIONS.md).
