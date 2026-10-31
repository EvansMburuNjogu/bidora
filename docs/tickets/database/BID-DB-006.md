# BID-DB-006 — Persist response revisions, immutable candidates and approval history

## Metadata
- Ticket ID: BID-DB-006
- Epic: E-DB
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-DB-004, BID-API-005
- Blockers: Approved financial/review policy
- Milestone: M4

## 1. Objective
Store exact technical/financial inputs and version-specific decisions without destructive updates.
## 2. Business Context
Reviewers must know which response they approved and retain accountability after edits.
## 3. Scope
Response sections/revisions, decimal financial schedules/lines, response evidence FK extension, review candidates, decision attempts, applicability and readiness history.
## 4. Out of Scope
Scoring/award schema, editable approved snapshots, generic workflow designer.
## 5. Technical Requirements
Unique revision keys; immutable candidate manifest/hash; approved correction-attempt identity; exactly one evidence target; restrictive FKs; current applicability separate from history; pricing accessed through separate owner queries.
## 6. Proposed Implementation Approach
Implement migrations in FK order; version writes against bid aggregate; validate snapshot refs; test correction and invalidation transaction; add query indexes for pending review.
## 7. Dependencies
[Bid persistence](BID-DB-004.md), [review contracts](../backend_contracts/BID-API-005.md).
## 8. Acceptance Criteria
- Given material edit, then old decision remains unchanged while applicability can be invalidated.
- Given concurrent candidate revisions, then unique/version checks prevent conflicting current candidates.
- Given evidence with two targets, then CHECK rejects it.
## 9. Testing Requirements
Immutable grants, decimal precision, exactly-one target, candidate FK/unique, edit/approve transaction races and migration upgrades.
## 10. Definition of Done
Reviewed tables/queries with lineage and concurrency tests; no policy silently inferred.
## 11. References
[RULE-BID-004/005](../../product/05_DOMAIN_MODEL_AND_BUSINESS_RULES.md), [schema](../../technical_design/06_DATABASE_SCHEMA_DESIGN.md), [DB plan](../../implementation/05_DATABASE_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Hash canonicalization must use approved profile; JSONB cannot hide undefined relationships.
