# BID-BE-004 — Version technical responses and restrict financial schedules

## Metadata
- Ticket ID: BID-BE-004
- Epic: E-BE
- Type: Engineering
- Priority: P0
- Status: VERIFIED_TECHNICALLY
- Complexity: M
- Dependencies: BID-BE-003, BID-DB-006, BID-API-005
- Remaining gates: real OpenAI/provider privacy and availability acceptance, deployment evidence, independent security review, UAT and formal acceptance. Financial schedules remain excluded from Option A.
- Milestone: M4

## 1. Objective
Allow collaborative response revision without leaking pricing or overwriting another contributor.
## 2. Business Context
Technical and finance staff need separate authority while producing a coherent final response.
## 3. Scope
Response revision/read/history, financial line revisions/approved arithmetic, response evidence links and material-change applicability invalidation.
## 4. Out of Scope
Rich office suite, automatic pricing, final approval.
## 5. Technical Requirements
Immutable revisions; expected bid/section version; decimal-string inputs; independent pricing view/edit; allowed content formats; safe output encoding; transactionally mark affected candidate applicability stale with audit/outbox.
## 6. Proposed Implementation Approach
Implement response and pricing policies/repositories; validate numeric/format limits; map safe DTOs; integrate evidence target validation; test concurrent revisions and filtered history.
## 7. Dependencies
[Compliance](BID-BE-003.md), [response persistence](../database/BID-DB-006.md), [contracts](../backend_contracts/BID-API-005.md).
## 8. Acceptance Criteria
- Given technical-only user, then financial content is absent from every read/history/error path.
- Given stale save, then no content is lost through silent retry.
- Given approved input changes, then old candidate cannot remain currently applicable.
## 9. Testing Requirements
Decimal/rounding fixtures, revision races, HTML/formula injection, permission matrix and immutable history tests.
## 10. Definition of Done
Versioned response APIs with pricing denial and invalidation evidence.
## 11. References
[BR-025](../../product/02_BRD.md), [RULE-BID-004/005](../../product/05_DOMAIN_MODEL_AND_BUSINESS_RULES.md), [modules](../../technical_design/09_BACKEND_MODULE_DESIGN.md), [backend plan](../../implementation/06_BACKEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Do not duplicate authoritative arithmetic in frontend code.

### Option A reconciliation — 2026-10-18
The approved pilot deliberately narrows this parent ticket to technical responses, exact evidence links, cited advisory drafting, immutable proposal revisions, human application and deterministic PDF/DOCX export. Financial schedules, arithmetic and pricing capabilities were not approved for Option A and are not silently treated as implemented. The bounded functional source is complete; verification follows `backend/docs/option-a-verification-order.md`.

### Plain-text technical response slice

The current functionality-first increment deliberately excludes all financial schedules and approval candidates. Tenant migration 00010 adds one technical response section per requirement, immutable response revisions and exact links to previously validated requirement-evidence records. Authorized humans with `tenderpreneur.bid.response.edit` save plain UTF-8 text bounded to 20,000 bytes within the existing request ceiling with a mandatory change note, at most 20 same-requirement evidence links, expected section/workspace versions and idempotency. Each revision advances the workspace and commits minimal audit/outbox provenance without response content. Current and bounded history reads require current bid-read authority and contain no financial fields.

`GET /v1/tenderpreneur/bids/{bid_id}/gap-review` computes time-specific blockers from authoritative state: missing mandatory technical response, unsatisfied mandatory requirement, unassigned open requirement, overdue open requirement and missing BID decision. It does not invoke AI, approve readiness, or create a waiver. Pricing arithmetic/capabilities, response approval and candidate applicability remain separate continuations. Formatting, sqlc generation, JSON parsing, production compilation and compile-only discovery succeeded; migrations and tests are deferred. Status: **IMPLEMENTED_NOT_VERIFIED**.

### Grounded AI response assistance

Migration `00011_ai_response_drafts.sql`, the strict OpenAI adapter and pilot routes queue and expose advisory per-requirement response drafts. Each run snapshots workspace/response versions and one exact `CLEAN` tender source plus at most four exact `CLEAN` same-requirement evidence versions, capped at 50 MiB combined. Processing reauthorizes the requester, cancels stale snapshots, requires at least one bounded exact-quote citation and stores structured advisory gaps separately. AI output remains `PROPOSED_UNVERIFIED` and cannot change the human response, requirement status, deterministic readiness, approval or submission state. Explicit human acceptance/editing remains a separate operation. Status: **IMPLEMENTED_NOT_VERIFIED**.

### Editable professional proposal workspace

Migration `00012_proposal_document_workspace.sql` and the proposal APIs add a human-editable, versioned proposal document over authoritative technical responses. A revision snapshots the server-owned organization legal name, bounded human-managed letterhead contact lines, recipient, title, introduction, closing and every included immutable response revision. Requirement text/category/mandatory values are copied into the revision snapshot so later requirement edits cannot rewrite historical proposal content. Saves require response-edit authority, expected proposal/workspace versions and idempotency; they advance the workspace and commit content-free audit/outbox provenance. The current document and bounded revision history are readable through bid-read authority. Logo/signature assets, financial schedules, approval and submission remain excluded from this slice. Status: **IMPLEMENTED_NOT_VERIFIED**.

### Deterministic PDF and DOCX exports

`GET /v1/tenderpreneur/bids/{bid_id}/proposal/versions/{proposal_version_id}/export/{format}` now reauthorizes bid-read access and renders one exact immutable proposal revision as `pdf` or `docx`. The standard-library renderer uses only snapshotted company/letterhead/requirement/response content, fixed document structure, fixed ZIP metadata and no wall-clock values, random IDs, mutable templates or external renderer. Every successful response is private/no-store and includes SHA-256, Digest and ETag metadata. Source is bounded to 5 MiB, output to 8 MiB, and hostile XML/control characters fail closed. DOCX preserves valid Unicode; the dependency-free PDF profile supports Windows-1252 and rejects unsupported characters with 422 rather than corrupting them. Export does not approve, sign, persist or submit the proposal. Status: **IMPLEMENTED_NOT_VERIFIED**.

### Proposal-level AI writing assistance

Migration `00013_ai_proposal_writing.sql` and `POST/GET /v1/tenderpreneur/bids/{bid_id}/proposal/versions/{proposal_version_id}/ai-writing-suggestions` add durable advisory introduction/closing generation. Proposal response snapshots now retain their exact tender source-version identifiers. Each run is bound to one current proposal/workspace version and at most five distinct exact `CLEAN` tender sources (50 MiB combined), reauthorizes `tenderpreneur.bid.response.edit`, cancels stale queued work and requires strict bounded OpenAI output with at least one exact citation. The model receives no company profile, letterhead, current proposal prose, pricing or response text and is instructed not to invent company-specific claims. Results remain `PROPOSED_UNVERIFIED`; no proposal revision is changed until a human edits and saves it separately. Status: **IMPLEMENTED_NOT_VERIFIED**.

### Human application and approval readiness

Migration `00014_proposal_ai_application.sql` and `POST /v1/tenderpreneur/bids/{bid_id}/proposal/ai-writing-suggestions/{suggestion_id}/apply` let a response editor apply human-reviewed introduction/closing text from one current proposal suggestion. The operation requires edited final text, a review note, expected suggestion/proposal/workspace versions and idempotency. It preserves the original AI output, marks the suggestion `APPLIED`, links it to a new immutable human-owned proposal revision, refreshes exact response/source snapshots, advances the workspace and commits separate proposal/review audit and outbox facts atomically. Proposal revisions now record their resulting workspace version.

`GET /v1/tenderpreneur/bids/{bid_id}/proposal/approval-readiness` reports deterministic blockers for missing or outdated proposals, non-BID state, unsatisfied mandatory requirements, unassigned/overdue open requirements and missing mandatory responses. `ready_for_approval_review` is only preparation evidence: no approval capability, state transition, signature, candidate lock or submission is implemented. Status: **IMPLEMENTED_NOT_VERIFIED**.

### V6 technical verification — 2026-10-23

The approved non-financial Option A boundary is **VERIFIED_TECHNICALLY**. A live synthetic PostgreSQL 15.19/TLS journey exercised production authorization, paid-write checks, application orchestration, tenant persistence, entitlement acknowledgements and advisory workers for immutable technical-response revisions, exact evidence, concurrent one-winner updates, cited unverified response/proposal suggestions, stale-before-provider cancellation, ambiguous-provider failure recording, proposal snapshots/history, human-edited suggestion application, readiness, deterministic PDF/DOCX output and tenant isolation. Authenticated HTTP tests cover response save, proposal save, advisory request, human application and exact-version export with private/no-store handling.

Verification corrected missing proposal/advisory outbox actors and added migrations 33–34 to make entitlement run states reachable and allow repeated immutable advisory runs while retaining singleton scan/extraction jobs. Tenderpreneur compatibility/routing/runtime-grant floors are now 34. Selected V3–V6 journeys and affected suites passed under `-race`; full tests, vet, build, Goose validation, formatting, checksums and canonical OpenAPI generation passed. No real OpenAI call occurred; `store:false` and strict cited schemas were source-reviewed only, so provider/deployment acceptance, security review, UAT and formal acceptance remain pending. Financial schedules remain excluded rather than implicitly completed.
