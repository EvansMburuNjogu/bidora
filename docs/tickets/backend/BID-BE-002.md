# BID-BE-002 — Verify external tender facts and create a human-owned bid workspace

## Metadata
- Ticket ID: BID-BE-002
- Epic: E-BE
- Type: Engineering
- Priority: P0
- Status: VERIFIED_TECHNICALLY
- Complexity: M
- Dependencies: BID-DB-004, BID-API-003, BID-SEC-002
- Remaining gates: independent security review, deployment acceptance, UAT and formal acceptance
- Milestone: M3

## 1. Objective
Implement manual capture, immutable fact verification, workspace conversion and human Bid/No-Bid decision.
## 2. Business Context
Teams need a usable bid record even without AI or marketplace buyers.
## 3. Scope
Opportunity/bid application commands, authorized list/detail, duplicate candidates, owner assignment, exact source snapshot, audit/outbox.
## 4. Out of Scope
URL fetching, AI verification, arbitrary state updates or full profile catalogue.
## 5. Technical Requirements
Unknown/ambiguous facts remain unverified; preserve zone/source wording; active owner membership; one active authoritative workspace unless policy variant; version/idempotency guard; no private pricing in summary.
## 6. Proposed Implementation Approach
Implement domain verification/decision transitions; transaction-bound repositories; bind reviewed endpoints; return real blockers and action hints; test races and response loss.
## 7. Dependencies
[Persistence](../database/BID-DB-004.md), [API](../backend_contracts/BID-API-003.md), [isolation proof](../security/BID-SEC-002.md).
## 8. Acceptance Criteria
- Given verified source and authorized owner, then conversion creates one private workspace with source version.
- Given repeated request, then original workspace is returned without duplicate event.
- Given foreign source or unverified required fact, then command fails safely.
## 9. Testing Requirements
Domain/date/duplicate, real transaction/API policy, stale version and cross-tenant list/detail tests.
## 10. Definition of Done
Real operation and denied/retry behavior demonstrated with audit lineage.
## 11. References
[PR-TP-004/006/007](../../product/03_PRD.md), [modules](../../technical_design/09_BACKEND_MODULE_DESIGN.md), [backend plan](../../implementation/06_BACKEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Source URL is descriptive until a separately approved safe-fetch adapter exists.

### Application-development restart — independent deadline domain slice

Following the user's cleanup/application-development request, implemented only the source-defined deadline representation under `backend/internal/opportunity/domain/`, with external-package tests under `backend/tests/opportunity/`. It preserves source text, source-local timestamp and named zone, derives UTC, and rejects missing offsets, contradictory timezone offsets and DST gaps without guessing. Explicit offsets distinguish DST folds. Historical dates remain representable; no submission cutoff or future-only rule is invented.

This is an interpreted value, **not human verification, a source snapshot aggregate, an application command, or a persisted/API operation**. Zero values remain unknown. Parse errors do not make unknown facts authoritative. Full capture/verification, source-version links, owner/tenant policy, transactional audit/outbox and HTTP endpoints remain blocked by the existing dependencies. Parent status is unchanged.

Continuation adds immutable **unverified captured facts**: retain unknown title/buyer/date facts and descriptive source/document-version references without fetching, inference or duplicate merging. Optional deadline interpretation must match retained source wording/zone; capture owns a value copy. This is still domain-only, not a persisted source-version aggregate or an authorized capture command. There is no verification transition or workspace creation.

The domain-only continuation passed full active race tests/vet/build after an initial corrected test typo, with 98.1% domain coverage at that stage.

### Approved request implementation — 2026-10-18

The requesting user approved capture rules, limits and no-TTL idempotency retention under the [manual-capture contract](../../../backend/docs/manual-capture-contract.md). Implemented bounded strict JSON binding, semantic validation, UTC-to-source-zone mapping, owned canonical input and versioned fingerprints. Unknown/duplicate/client-authority fields, malformed Unicode, invalid references/URIs and date/size violations fail safely. Unknown facts remain unverified; original nullable data and non-null strings are retained. Canonicalization vectors are implemented/tested, not accepted persistence/replay proof.

Full active Go race tests, vet/build and module verification passed; the earlier domain/application/transport slice measured **96.1%**. After adding persistence and authenticated-control composition to the coverage scope, combined opportunity domain/application/integration/persistence/transport coverage is **86.0%**. Bounded fuzzing passed 30,117 executions; Linux cross-compilation passed, not Linux execution. Security scan: no reachable/imported findings; unchanged module-only unused-openpgp advisory. [Evidence and commands](../../../backend/docs/application-development.md).

Application transaction orchestration now exists behind explicit authorization, exact-document eligibility and tenant unit-of-work ports. It reauthorizes replay, constant-time compares request fingerprints, handles new/completed/pending/conflict/inconsistent claims and orders source/audit/outbox/idempotency completion in one callback. Every step-failure path is tested; no success result escapes callback failure.

A tenant-lease-bound pgx/sqlc unit-of-work adapter now maps the callback to generated idempotency, opportunity/source, audit and outbox operations. It rejects tenant/generation mismatch, reconstructs locked completed replay, checks affected rows and persists actor provenance without raw captured content. Scripted DB-interface tests verify ordering and failure propagation.

The bounded manual-capture slice now runs through the production pilot composition with real TLS PostgreSQL/Redis dependencies and a temporary Fiber listener. Live evidence covers authenticated session/CSRF binding, current membership and DENY precedence, server-owned assignment routing, tenant-local lease checks, atomic state/audit/outbox/idempotency commit and rollback, replay/conflict/concurrent convergence, assignment-change failure, Redis-loss separation and tenant-B non-observation. A runnable `cmd/pilot-api` exists but is not deployed; startup performs compatibility probes and never migrates.

The user subsequently approved the simple MVP policy: an authorized human confirms the exact current captured source version; title is required while buyer/deadline may remain unknown; verification and workspace creation use separate explicit capabilities; the owner must be a verified active tenant member; and only one active workspace per opportunity is permitted with no variants. Code generation now adds Tenderpreneur migration 00003, immutable VERIFIED source revisions, the single active `bid.workspaces` table, deny-first control authorization, server-routed tenant transactions, idempotency, audit/outbox writes, strict authenticated HTTP handlers and the draft contract `backend/api/openapi/drafts/opportunity-workspace.json`. sqlc artifacts were regenerated and affected production packages compile. Tests and live migration execution were deliberately deferred under the current code-first phase, so this is **IMPLEMENTED_NOT_VERIFIED**.

The human decision continuation now adds Tenderpreneur migration 00004 and `bid.decisions`: only `BID` or `NO_BID` is accepted, rationale is mandatory, history is append-only, and the workspace state/version advances under an expected-version lock. `POST /v1/tenderpreneur/bids/{bid_id}/decision` requires explicit deny-first `tenderpreneur.bid.decision.record`, session/Origin/CSRF/context binding and idempotency; it commits the decision, workspace state, `BidDecisionRecorded` audit and `bid.decision.recorded.v1` outbox atomically. AI/service actors have no decision path. Generated sqlc and affected production compilation succeeded; tests and migration execution remain deferred, so this is **IMPLEMENTED_NOT_VERIFIED**.

The read continuation adds explicitly authorized, server-routed `GET /v1/tenderpreneur/opportunities`, `GET /v1/tenderpreneur/opportunities/{id}`, `GET /v1/tenderpreneur/bids` and `GET /v1/tenderpreneur/bids/{id}` operations. Lists are bounded to 100 and use stable created-time/ID keyset cursors; bid filters are allowlisted to state and owner. Details expose the exact source snapshot and immutable human decision history without financial fields or database locators. Opportunity detail returns at most 20 exact case-insensitive title/buyer possible-duplicate IDs plus a truncation flag; these are hints only and never merge or reject records. Access requires separate deny-first `tenderpreneur.opportunity.read` or `tenderpreneur.bid.read` authority and server-owned assignment routing. Generated sqlc, JSON contracts and affected production compilation succeeded; tests remain deferred.

The **BID-BE-002 functional code-generation scope is complete and technically verified** through V4 in `backend/docs/option-a-verification-order.md`: capture, human verification, one active workspace, human Bid/No-Bid history, owner checks, idempotency, audit/outbox, bounded list/detail, duplicate visibility, authorization/routing races and cross-tenant concealment now have focused synthetic PostgreSQL/API evidence. The verification found and corrected missing mandatory outbox actor provenance in all three workflow events before the final race run passed. The parent ticket is `VERIFIED_TECHNICALLY`, not Done or accepted; independent security review, deployment acceptance, UAT and formal human acceptance remain. Requirement/evidence/readiness functionality belongs to BID-BE-003.
