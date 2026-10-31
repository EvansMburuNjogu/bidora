# BID-BE-007 — Register outcome corrections and permission-filtered bid history

## Metadata
- Ticket ID: BID-BE-007
- Epic: E-BE
- Type: Engineering
- Priority: P1
- Status: VERIFIED_TECHNICALLY
- Complexity: M
- Dependencies: BID-BE-006, BID-API-006
- Remaining gates: real MinIO/ClamAV deployment acceptance; independent security review; retention/deployment review; UAT and formal acceptance
- Milestone: M4

## 1. Objective
Retain reported outcomes and lessons without overwriting past facts or leaking private commercial data.
## 2. Business Context
Bid teams learn from wins, losses and disqualifications only when reasons and corrections remain trustworthy.
## 3. Scope
Outcome/lesson command, permitted transitions, evidence references, supersedes history and basic scoped portfolio queries.
## 4. Out of Scope
Predictive analytics, public competitor claims, full reporting warehouse.
## 5. Technical Requirements
Separate lifecycle from reported outcome; append-only correction chain; required reasons/evidence per approved policy; safe currency/value fields where authorized; filtered counts and stable cursor; audit/outbox with expected version.
## 6. Proposed Implementation Approach
Implement approved transition table; validate correction lineage and document references; add owner-scoped queries; expose reviewed API and deterministic metrics only.
## 7. Dependencies
[External evidence](BID-BE-006.md), [outcome contracts](../backend_contracts/BID-API-006.md).
## 8. Acceptance Criteria
- Given corrected outcome, then previous report and actor/time remain visible to authorized users.
- Given invalid lifecycle/outcome combination, then command rejects safely.
- Given unauthorized pricing/report scope, then counts/fields reveal no private content.
## 9. Testing Requirements
Transition/correction cycle, concurrent updates, idempotency, evidence ownership and report filtering tests.
## 10. Definition of Done
Outcome operation and history verified, documented formula/source for every displayed metric.
## 11. References
[PR-TP-014](../../product/03_PRD.md), [schema](../../technical_design/06_DATABASE_SCHEMA_DESIGN.md), [backend plan](../../implementation/06_BACKEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
### Read-only archive assessment boundary

`GET /v1/tenderpreneur/bids/{bid_id}/archive-preparation` exposes current workspace, requirement, open-review, pending-upload/job, package-manifest and unverified external-report counts without changing lifecycle state. It deliberately returns `archive_policy_configured=false`, `read_only_closure_supported=false`, `can_archive=false` and no archive command. Pending operations and already-archived state are explicit blockers. This archive assessment remains separate from the now-implemented BID-ADR-012 reported-outcome history and still does not implement retention deletion, legal hold, restoration or permission behavior after archival. Status: **IMPLEMENTED_NOT_VERIFIED**.

### Approved outcome policy

BID-ADR-012 is approved. Submitted bids may have append-only `HUMAN_REPORTED_EXTERNAL_OUTCOME` records using `PENDING`, `UNKNOWN`, `WON`, `LOST`, `DISQUALIFIED`, `CANCELLED` or `WITHDRAWN`. Current deny-first `tenderpreneur.bid.outcome.report`, reason, source class, non-future reported instant, expected sequence and idempotency are mandatory. Terminal outcomes require an exact immutable CLEAN PDF up to 25 MiB; proof is optional for PENDING/UNKNOWN. Corrections supersede only the current active record, preserve all history and reapply proof rules. All outcomes remain `REPORTED_UNVERIFIED` and cannot alter `SUBMITTED`, establish an authoritative buyer decision or contract award, execute withdrawal, enter analytics, archive, publish or be decided by AI. Migration `00024_reported_outcomes.sql` now implements dedicated `OUTCOME_PROOF` ownership and append-only reports in source without reusing requirement/submission proof under incorrect ownership semantics. PDF upload initiation/completion uses exact hash/size checks, private storage and fail-closed scanning. POST/list/current endpoints enforce the approved statuses, source/reason/time, expected sequence, current active supersession, terminal CLEAN-proof requirement, idempotency and `SUBMITTED` workspace precondition. Audit/outbox payloads remain content-minimized and all API results explicitly deny buyer authority, contract award, withdrawal execution and analytics acceptance. This source slice was initially `IMPLEMENTED_NOT_VERIFIED`; V9 now supplies technical verification. Current runtime expects Tenderpreneur schema 34.

## 12. Technical verification — 2026-10-23

V9 exercised production authorization, paid-write, application, pgx/sqlc persistence, document-processing and HTTP boundaries against isolated PostgreSQL 15.19/TLS. `backend/tests/integration/outcome_archive_journey_test.go` proves proof-optional `PENDING`/`UNKNOWN`, exact CLEAN PDF requirements for terminal reports, quarantine denial, source/server timestamp separation, immutable correction lineage, expected sequence, concurrent one-winner corrections, replay/body mismatch, bounded current/history reads, deny-first capability checks, actor-attributed outbox evidence and tenant isolation. It also proves pending-upload/job/open-review archive blockers, eligible `NO_BID` closure, archive races/replay, immutable workspace enforcement and continued authorized historical reads. `backend/tests/httpapi/outcome_archive_test.go` proves strict authenticated binding, private/no-store responses and mandatory unverified/non-authoritative/non-destructive response facts.

Verification exposed and corrected bounded capability reachability, outcome-proof scan validation, safe conflict mapping, outbox actor, SQL parameter typing and cache-control defects. sqlc v1.31.1 output and reviewed checksums were regenerated. Selected V3–V9 journeys passed together under `-race`; affected policy/HTTP/database/architecture/runtime-grant suites, full repository tests, vet, build, formatting, checksum JSON and OpenAPI generation checks passed.

Status is `VERIFIED_TECHNICALLY`, not deployed, UAT-approved or accepted. No authoritative buyer result, contract award, withdrawal execution, production analytics, deletion, legal-hold change or restoration is implemented or claimed.

## 13. Risks and Notes
Do not infer outcome from external silence or model recommendation.

### Option A reconciliation — 2026-10-18
The approved source contains dedicated CLEAN outcome-proof handling, append-only `REPORTED_UNVERIFIED` outcome/correction history and non-destructive archive/read-only closure. Unsupported analytics, buyer authority, retention deletion, legal-hold changes and restoration remain excluded. Functional source is complete; deferred storage, race, authorization and acceptance evidence follows `backend/docs/option-a-verification-order.md`.
