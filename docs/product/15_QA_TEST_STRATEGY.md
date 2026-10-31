# TenderOS — Quality Assurance and Test Strategy

## 1. Quality objective

Quality is demonstrated by executable evidence that real personas can complete approved operations safely and correctly. Passing builds, rendered pages, endpoint existence, or high test counts do not by themselves prove product completion.

## 2. Quality principles

1. Test business invariants before UI details.
2. Test every authorization boundary positively and negatively.
3. Test tenant isolation at every data path.
4. Test state transitions and failure/recovery paths.
5. Contract-test frontend/backend and service/event boundaries.
6. Use production-representative data shapes without production secrets or personal data.
7. Make AI quality measurable with benchmark datasets.
8. Record evidence by requirement and release gate.
9. Do not waive critical defects through optimistic status labels.

## 3. Test levels

### 3.1 Static quality

- Go/Python/TypeScript formatting and linting
- Type checking
- OpenAPI linting and breaking-change detection
- SQL/migration linting
- Dependency, license, secret, container, and IaC scans
- Architecture boundary tests preventing forbidden imports

### 3.2 Unit tests

Target:

- Domain invariants and state machines
- Policies and calculations
- Authorization rules
- Readiness and score formulas
- Date/time/deadline behavior
- Deduplication and ranking functions
- Mapping/validation/error behavior
- AI schema/citation validators

Unit tests must not depend on network or shared databases.

### 3.3 Database and repository integration tests

Use ephemeral PostgreSQL containers.

Test:

- Migrations from clean and supported previous versions
- Constraints, uniqueness, foreign keys, indexes, and transaction behavior
- Optimistic concurrency
- Outbox/inbox atomicity and idempotency
- Tenant DB routing
- Control/exchange/tenant separation
- Restore/import compatibility where feasible

### 3.4 API integration tests

Run the real API with test dependencies.

Test:

- Success/error envelopes
- Authentication/session/context
- 401 versus 403 versus concealed 404 semantics
- Resource authorization and state conditions
- Idempotency and replay
- Pagination/filter/sort
- CORS/CSRF
- File upload lifecycle
- Quota/entitlement enforcement
- OpenAPI contract conformance

### 3.5 Event and worker tests

- Outbox publishing after commit only
- Inbox duplicate handling
- Retry/backoff and dead-letter
- Event schema compatibility
- Projection rebuild
- Notification delivery state
- Addendum/change propagation
- Cross-plane exchange acknowledgement

### 3.6 Component tests

Frontend component tests cover:

- Forms and validation
- Tables/filters/pagination
- Permission-aware actions
- Loading/empty/error/conflict states
- Upload and document version UI
- AI citations and disposition controls
- Accessibility behavior

### 3.7 End-to-end tests

Use Playwright against a deployed test environment with real API, databases, object storage, broker, and safe AI stubs/approved test provider.

Critical journeys:

1. Tenant registration and provisioning
2. Membership invitation/context selection
3. Procurement request to approved tender
4. Publication, clarification, addendum, and closing
5. Tenderpreneur external import to submission evidence
6. Deep Search result to bid workspace
7. Platform tender participation and sealed submission
8. Bid withdrawal/replacement
9. Authorized bid opening
10. Preliminary/technical/financial evaluation
11. Recommendation, award, and notifications
12. Subscription limit and upgrade path
13. Support access and expiry
14. Tenant context switch without data leakage

### 3.8 UAT

UAT is persona-based and uses realistic scenarios.

Participants:

- Procurement requester/officer/approver/evaluator
- Tenderpreneur bid manager/technical/finance/legal/approver
- Platform admin/support
- Security/audit observer

Each UAT script includes preconditions, data, actions, expected outcomes, exceptions, evidence, and sign-off.

## 4. Tenant isolation test suite

Run automated tests for two or more active tenants of each type.

Attempt cross-tenant access through:

- Direct object IDs
- List/search filters and counts
- Tenant hints/headers
- Context switch race
- Cached frontend/server data
- Document URLs and object keys
- Export/report IDs
- Task/comment/notification references
- Event replay
- AI job and citation IDs
- Search/vector retrieval
- Submission receipt and opening IDs
- Support-access expiry
- Backup/export paths

Expected result is fail closed with no existence or metadata leak.

## 5. Authorization matrix testing

Generate tests from a maintained matrix:

```text
persona × capability × resource state × assignment/scope × session assurance
```

Examples:

- Technical evaluator can score assigned technical criteria after opening, but cannot see restricted financial content.
- Procurement officer can draft but cannot approve own publication where separation-of-duty applies.
- Tenderpreneur technical contributor can edit technical response but cannot view pricing.
- Platform support user cannot access tenant data without active support grant.

Both API and E2E layers test material controls.

## 6. State-machine testing

For each aggregate:

- Every valid transition
- Every invalid transition
- Concurrent transition conflicts
- Approval invalidation after material change
- Deadline boundary at exact server time
- Cancellation/withdrawal terminal behavior
- Recovery after dependency failure
- Audit/event emission

Property-based tests are encouraged for score/weight/deadline and state invariants.

## 7. Submission and opening testing

### Submission

- Before, at, and after closing-time boundaries
- Idempotent duplicate requests
- Interrupted multipart upload
- Package hash mismatch
- Missing/extra required envelope
- File-size/type limit
- Replacement and withdrawal policy
- Server timestamp and receipt verification
- High-concurrency deadline spike

### Opening

- Too early
- Wrong role
- Missing quorum
- Missing/failed conflict declaration
- Step-up expired
- KMS/key release unavailable
- Tampered manifest/object
- Partial release failure
- Retry without double release
- Immutable evidence and minutes

Independent security testing is mandatory.

## 8. AI quality tests

### Offline benchmark suites

- Scanned and native PDFs
- Tables, forms, annexes, multiple lots, and addenda
- English and future supported languages
- Contradictory/ambiguous tender documents
- Prompt-injection content
- Organization knowledge with near matches and expired evidence

Metrics:

- Field precision/recall
- Mandatory-requirement recall
- Citation correctness and coverage
- Unsupported-claim rate
- Compliance-match precision/recall
- Search nDCG/precision@k/recall@k
- Deduplication precision/recall
- Latency and cost
- Human acceptance/edit/rejection rate

### AI safety tests

- Cross-tenant retrieval attempts
- Hidden prompt injection
- Malicious links/SSRF attempts
- Tool escalation
- Sensitive data exfiltration
- Model refusal/failure handling
- Provider outage/fallback
- Invalid structured output

AI model or prompt changes require benchmark regression before release.

## 9. Deep Search testing

- Source connector contract tests using recorded fixtures
- Rate limit/backoff
- Conditional fetch and freshness
- HTML/PDF/JSON/feed parsing
- Canonicalization and buyer/category mapping
- Duplicate merge/split
- Deadline and timezone extraction
- Addendum/change detection
- Removed/cancelled source behavior
- Authoritative-source preference
- Saved search and alert correctness
- Search access and tenant preference isolation

Live-source tests are monitored separately because external sites are unstable.

## 10. Performance testing

Scenarios:

- Concurrent tenant users and database pool churn
- Public catalogue/search traffic
- Tenant-private search
- Large document upload/download
- Deadline-time submission surge
- Bid opening and content release
- Evaluation panel concurrent reads/writes
- AI ingestion backlog and model throttling
- Source connector batch runs
- Platform-wide usage aggregation

Measure:

- p50/p95/p99 latency
- Error and timeout rate
- Throughput
- DB connections/locks/slow queries
- Queue depth and job age
- Memory/CPU/storage/network
- AI tokens/cost/latency
- Cache hit and search freshness

Performance budgets are release gates for critical operations.

## 11. Reliability and recovery testing

- API/worker restart during transaction/job
- Broker unavailable/recovered
- AI provider timeout
- object storage latency/unavailability
- tenant DB unavailable
- control-plane cache stale
- outbox backlog replay
- duplicate events
- backup restore
- point-in-time recovery exercise
- regional/service dependency failure tabletop

The system must fail clearly and avoid silent data loss or false completion.

## 12. Security testing

- SAST, dependency, secret, container, and IaC scanning
- DAST
- API authorization and BOLA/IDOR
- SQL/HTML/template/log/spreadsheet injection
- CORS/CSRF/session fixation
- File upload and archive bombs
- SSRF and redirect/DNS rebinding defenses
- Rate-limit and abuse controls
- Privilege escalation
- Support-access abuse
- Prompt injection and AI data leakage
- VAPT before pilot/full launch

## 13. Accessibility testing

- Automated axe scans on key pages
- Keyboard-only operation
- Screen-reader checks for critical workflows
- Focus order and dialog trapping
- Labels, descriptions, errors, and async announcements
- Contrast and non-color cues
- Responsive zoom and text reflow
- Reduced motion

## 14. Browser/device coverage

Define supported versions at release time. Baseline:

- Current and previous major Chrome/Edge/Firefox
- Current and previous major Safari
- Responsive desktop/tablet/mobile viewport coverage
- Procurement authoring/evaluation optimized for desktop but not blocked on tablet

## 15. Test data

- Synthetic organizations, users, roles, tenders, documents, submissions, evaluations, and invoices
- No copied production personal/confidential data
- Seed scenarios versioned in repository
- Deterministic fixtures for contracts and AI benchmarks
- Data factory supports multiple tenants and country packs
- Test documents include safe scans, tables, annexes, and malformed cases

## 16. Defect severity

- **S0 Critical:** Security breach, cross-tenant leak, sealed-bid compromise, irreversible corruption, unauthorized award/submission
- **S1 High:** Critical journey blocked, incorrect permission/state, wrong evaluation/calculation, data loss with recovery
- **S2 Medium:** Major workaround needed, partial workflow failure, material accessibility/performance issue
- **S3 Low:** Minor usability, visual, copy, or non-blocking issue

No open S0/S1 defect enters production without formal executive/security acceptance, and sealed-bid or cross-tenant defects are not waivable for launch.

## 17. Definition of ready

A ticket is ready when it has:

- Requirement IDs and business outcome
- Actor, preconditions, workflow/state impact
- Data/migration design
- API/event contract
- Authorization and security notes
- Frontend/page contract
- Acceptance criteria and test plan
- Dependencies resolved
- Design assets/template mapping where relevant

## 18. Definition of done

A ticket is done when:

- Migration and repository tests pass
- Domain/API behavior and errors pass
- Authorization and tenant tests pass
- Frontend operation and all states work
- Accessibility checks pass
- Observability/audit exists
- Documentation/contracts updated
- Evidence linked to requirement
- Code reviewed and deployed to target test environment
- Persona can complete the operation without mock data

## 19. Release quality gates

- All planned requirements trace to tests/evidence
- CI/CD and dependency/security gates green
- No open release-blocking defects
- Migration rehearsal and rollback/forward-fix plan approved
- Performance target met
- Backup/restore and incident checks passed
- UAT signed by required personas
- AI benchmark passed for changed workflows
- Security/privacy review completed
- Release notes and operational runbooks approved

