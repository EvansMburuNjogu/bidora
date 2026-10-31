# BID-API-003 — Specify verified opportunity, bid decision and compliance contracts

## Metadata
- Ticket ID: BID-API-003
- Epic: E-API
- Type: Design
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-API-001, BID-ARC-001, BID-ARC-003
- Blockers: External release and transition policy approval
- Milestone: M3

## 1. Objective
Freeze source-verification, workspace and requirement operations before bid handlers.
## 2. Business Context
Imported facts must not silently become authoritative requirements or satisfied compliance.
## 3. Scope
Manual opportunity capture/detail/list, verify-facts, create-workspace, human decision, bid detail/dashboard, requirements/assignment/status/evidence and readiness queries.
## 4. Out of Scope
Automatic URL fetch, AI extraction, arbitrary state PATCH.
## 5. Technical Requirements
Exact source version/location; timezone/original date text; unknown facts; duplicate/variant policy; owner membership; exactly one evidence target; version preconditions; filtered counts/pricing. Complete missing read contracts.
## 6. Proposed Implementation Approach
Map approved transitions to endpoint schemas; specify invalid SATISFIED/expired evidence/stale source errors; review transaction version owner and cursor behavior.
## 7. Dependencies
[Base](BID-API-001.md), [scope](../architecture/BID-ARC-001.md), [domain policy](../architecture/BID-ARC-003.md).
## 8. Acceptance Criteria
- Given unverified required facts, then workspace conversion response specifies the blocker.
- Given unsupported SATISFIED, then validation cannot be bypassed by DTO status.
- Given foreign bid/evidence, then responses conceal private existence.
## 9. Testing Requirements
Schema examples for dates, duplicate retries, cross-bid evidence, cursor integrity, stale revision and restricted fields.
## 10. Definition of Done
Approved schemas, examples and capability/error matrix for every scoped action/read.
## 11. References
[PR-TP-004/006–008/011](../../product/03_PRD.md), [API](../../technical_design/10_API_CONTRACT_DESIGN.md), [contracts plan](../../implementation/07_BACKEND_CONTRACTS_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
External deadline display is not Bidora-controlled acceptance timing.

### Manual capture operation — review draft prepared

Prepared one bounded POST contract in [OpenAPI 0.1.0-draft](../../../backend/api/openapi/drafts/manual-capture.json) with [semantic/security/retry rules and review choices](../../../backend/docs/manual-capture-contract.md). Includes unverified creation only, preserved unknown facts/provenance, strict request shapes, proposed limits, context/capability/CSRF checks, scoped idempotency, atomic audit/outbox and a proposed ID-only event schema. UTC wire instants and nullable raw input are explicitly distinguished from the existing domain representation.

On **2026-10-18** the requesting user replied **“approve and proceed”** to approval of capture rules, request limits and no-TTL idempotency retention. Those choices are now **Approved**, not the full endpoint/security/event/persistence contract. Metadata records the partial scope; parent stays Blocked. Do not ask for the same narrow approval again.

Implemented strict capture body decoding, semantic validation, UTC-to-source-zone mapping and immutable canonical input/fingerprints in the opportunity transport/application/domain packages. Both OpenAPI examples pass the real decoder; full Go race tests, vet/build and module verification passed. Combined opportunity coverage **96.1%**; 30,117 bounded fuzz executions passed. Linux cross-compilation only; fresh security scan found no reachable/imported findings, with the existing unused-openpgp module advisory retained. See linked evidence for commands and limits.

JSON/14-reference structural checks passed. The bounded manual-capture operation is now implemented through strict authenticated HTTP, transactional PostgreSQL persistence, server-owned tenant routing and live two-tenant isolation tests. Evidence covers replay/conflict/concurrency, rollback, active DENY precedence, stale membership context, assignment-generation change, CSRF/Origin rejection, Redis-loss separation and post-logout denial. The production pilot factory registers this approved operation, while deployment remains unperformed.

The parent remains **Blocked** because BID-API-003 also includes list/detail, fact verification, workspace conversion, bid decisions, requirements, evidence and readiness contracts. Those contracts, document-source linkage and broader event/publication behavior are not approved or implemented; the completed manual-capture subset must not be treated as the full ticket.
