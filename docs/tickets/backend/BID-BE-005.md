# BID-BE-005 — Finalize immutable review candidates and enforce human approval

## Metadata
- Ticket ID: BID-BE-005
- Epic: E-BE
- Type: Engineering
- Priority: P0
- Status: VERIFIED_TECHNICALLY
- Complexity: M
- Dependencies: BID-BE-004, BID-API-005
- Remaining gates: deployment evidence, independent security review, UAT and formal acceptance
- Milestone: M4

## 1. Objective
Approve only exact current candidates through assigned human decisions.
## 2. Business Context
A review must not authorize a response edited after the approver inspected it.
## 3. Scope
Finalize candidate, submit review, request changes/reject/approve, pending reviews/history, applicability invalidation and readiness integration.
## 4. Out of Scope
Universal workflow builder, invented self-approval/delegation defaults, AI approval.
## 5. Technical Requirements
Snapshot source/requirements/evidence/technical/pricing/policy versions; hash approved canonical manifest; current actor/scope/assurance checks; same-transaction decision/audit/outbox/version update; synchronous stale checks despite delayed invalidation events.
## 6. Proposed Implementation Approach
Implement candidate assembly and review policy port; lock/version authoritative bid; append decisions; invalidate applicability on material change; expose approved read/command contracts.
## 7. Dependencies
[Responses](BID-BE-004.md), [approval API](../backend_contracts/BID-API-005.md).
## 8. Acceptance Criteria
- Given concurrent edit and approve, then stale candidate cannot become currently approved.
- Given revoked reviewer or failed separation rule, then no decision commits.
- Given correction/new review, then prior decision remains unchanged and traceable.
## 9. Testing Requirements
Transaction races, expired evidence/source, role/assurance, repeated decision and audit rollback tests.
## 10. Definition of Done
Human review flow with immutable input/decision evidence and all negative cases.
## 11. References
[PR-SH-004/PR-TP-011](../../product/03_PRD.md), [domain](../../technical_design/03_DOMAIN_ARCHITECTURE.md), [backend plan](../../implementation/06_BACKEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
### Collaboration preparation — review threads

Migration `00017_bid_review_threads.sql` and the review-thread API add append-only human discussion against a bid, requirement or exact proposal revision. Response editors can create threads, append comments and resolve with an immutable note using idempotency and expected thread versions; bid readers can read bounded threads/comments. Every mutation reauthorizes, validates target ownership in the tenant database and commits content-free audit/outbox provenance. Comment text never enters readiness, proposal exports or AI provider input. Threads cannot be edited, deleted, reopened or treated as votes/approval decisions. This is `IMPLEMENTED_NOT_VERIFIED` collaboration preparation, not implementation or authorization of BID-BE-005 approval decisions.

### Review ownership and due dates

Migration `00018_review_assignments.sql` adds current assignee/due-date projections and immutable assignment history. `PATCH /v1/tenderpreneur/bids/{bid_id}/review-threads/{thread_id}/assignment` requires response-edit authority, validates the assignee as a current active same-tenant user with `tenderpreneur.bid.read`, requires a reason and expected thread version, and optionally accepts an explicit future RFC3339 due instant. Reassignment appends history, advances the thread version and commits content-free audit/outbox facts; bounded history is readable separately. Approval readiness reports `OPEN_REVIEWS_UNASSIGNED` and `REVIEWS_OVERDUE` counts/blockers in addition to unresolved threads. Notifications, escalation and workload policy remain excluded. Status: **IMPLEMENTED_NOT_VERIFIED**.

### Review dashboard and reviewer workloads

Read-only collaboration views now expose a bid dashboard (`GET /v1/tenderpreneur/bids/{bid_id}/review-dashboard`), bounded tenant-local reviewer summaries (`GET /v1/tenderpreneur/review-workloads`) and at most 200 open items for one currently eligible reviewer (`GET /v1/tenderpreneur/review-workloads/{reviewer_id}`). Dashboard access requires bid-read authority; tenant workload access requires response-edit authority; reviewer detail additionally revalidates the selected user's active same-tenant membership and bid-read authority. Counts use one explicit UTC observation instant, exclude archived workspaces and fail rather than truncate above 100 reviewers or 200 detail items. Views expose internal IDs and review metadata only—no email, role labels, scoring, ranking or cross-tenant data. Status: **IMPLEMENTED_NOT_VERIFIED**.

### Internal proposal review decisions and approval-policy preparation

Migration `00019_internal_review_decisions.sql` adds immutable per-reviewer `REVIEW_COMPLETE` and `CHANGES_REQUESTED` history for the exact current proposal revision. Recording requires response-edit authority, expected previous reviewer sequence, idempotency and a rationale; proposal replacement and stale sequences fail closed. Bid readers may inspect bounded history. A separate preparation read reports latest-reviewer facts and open-thread counts, but deliberately returns `approval_policy_configured=false`, `eligible_for_approval=false` and `APPROVAL_POLICY_NOT_CONFIGURED`. These decisions are collaboration evidence, not approval votes, quorum, signatures, locking or submission authorization. Status: **IMPLEMENTED_NOT_VERIFIED**.

### Bid collaboration activity timeline

`GET /v1/tenderpreneur/bids/{bid_id}/activity` now provides a bid-read-authorized, tenant-routed, keyset-paginated stream of review-thread creation, comment/resolution facts, reviewer assignments and internal review decisions. Pages are limited to 100 and use immutable occurrence-time/UUID cursors. The projection intentionally excludes comment bodies, rationale, proposal prose, emails and role labels; it returns event type, actor, target, version, bounded detail codes, related internal IDs and due instants only. This is an operational collaboration history, not the complete administrative/security audit log or retention solution. Status: **IMPLEMENTED_NOT_VERIFIED**.

### Exact-package final approval

BID-ADR-011 is approved and implemented in source as `IMPLEMENTED_NOT_VERIFIED`. Tenderpreneur migration `00022_proposal_approvals.sql` adds one immutable approval per exact package manifest. `POST /v1/tenderpreneur/bids/{bid_id}/proposal/package-preparations/{manifest_id}/approval` requires current deny-first `tenderpreneur.bid.proposal.approve`, a session created within 15 minutes, exact expected proposal/workspace versions, an approver distinct from the current proposal-revision creator, and that approver's latest `REVIEW_COMPLETE` on the exact revision. Any latest `CHANGES_REQUESTED`, open/unassigned/overdue review, deterministic readiness blocker, stale manifest/version or idempotency conflict fails closed. Approval commits decision, audit and outbox evidence atomically and reports `signed=false` and `submitted=false`; no waiver or delegation exists. A bid-reader-authorized GET returns the immutable decision. Signing remains blocked; human-confirmed external submission is implemented separately under BID-BE-006. Current runtime expects Tenderpreneur schema 24.

## 12. Risks and Notes
Finalization is not approval; approval does not eliminate later readiness rechecks.

### Option A reconciliation — 2026-10-18
The source now contains review threads, immutable comments/assignments/internal decisions, activity reads, deterministic package manifests and approved BID-ADR-011 two-person exact-package approval with recent TOTP AAL2. Signing and waiver remain excluded. Functional source is complete; deferred race, migration, authorization and acceptance evidence follows `backend/docs/option-a-verification-order.md`.

### V7 technical verification — 2026-10-23

The approved Option A review and exact-package approval boundary is **VERIFIED_TECHNICALLY**. Live PostgreSQL evidence covers immutable discussion/assignment/decision history, bounded dashboards/workloads/activity, concurrent one-winner updates, exact manifest replay, maker/approver separation, current personal `REVIEW_COMPLETE`, stale-assurance denial, active-DENY precedence, concurrent approval, proposal-edit races, historical approval immutability and tenant isolation. Authenticated HTTP evidence verifies request binding, recent TOTP AAL2 principal propagation and private/no-store responses. Existing live MFA composition and Redis limiter evidence passed under `-race`.

Verification corrected the missing production authorization allowlist entries for review/approval, mandatory outbox actor provenance, raw readiness calculation, empty blocker-array encoding and private cache controls. Selected V3–V7 journeys and affected suites passed under `-race`; full tests, vet, build, formatting, checksum and OpenAPI checks passed. Approval remains unsigned/unsubmitted and requires downstream current-applicability checks. Deployment acceptance, independent security review, UAT and formal acceptance remain pending.
