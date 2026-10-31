# TenderOS — Low-Level Design (LLD)

## 1. Repository layout

Recommended monorepo:

```text
/
├── AGENTS.md
├── README.md
├── Makefile
├── docker-compose.yml
├── backend/
│   ├── cmd/
│   │   ├── api/
│   │   ├── worker/
│   │   └── migrate/
│   ├── internal/
│   │   ├── platform/
│   │   │   ├── identity/
│   │   │   ├── tenancy/
│   │   │   ├── subscription/
│   │   │   ├── source_registry/
│   │   │   └── support_access/
│   │   ├── shared/
│   │   │   ├── authz/
│   │   │   ├── audit/
│   │   │   ├── document/
│   │   │   ├── workflow/
│   │   │   ├── notification/
│   │   │   ├── eventing/
│   │   │   └── observability/
│   │   ├── procurement/
│   │   │   ├── planning/
│   │   │   ├── tender/
│   │   │   ├── publication/
│   │   │   ├── opening/
│   │   │   ├── evaluation/
│   │   │   └── award/
│   │   ├── tenderpreneur/
│   │   │   ├── profile/
│   │   │   ├── opportunity/
│   │   │   ├── bid/
│   │   │   ├── proposal/
│   │   │   └── readiness/
│   │   ├── exchange/
│   │   │   └── submission/
│   │   └── reporting/
│   ├── migrations/
│   │   ├── control/
│   │   ├── tenant_common/
│   │   ├── tenant_procurement/
│   │   ├── tenant_tenderpreneur/
│   │   └── exchange/
│   ├── api/openapi/
│   └── tests/
├── web/
│   ├── src/app/
│   ├── src/features/
│   ├── src/components/
│   ├── src/lib/
│   ├── src/contracts/
│   └── tests/
├── ai/
│   ├── app/api/
│   ├── app/domain/
│   ├── app/use_cases/
│   ├── app/adapters/
│   ├── app/workflows/
│   ├── app/evals/
│   └── tests/
├── infrastructure/
│   ├── docker/
│   ├── terraform-or-equivalent/
│   ├── monitoring/
│   └── runbooks/
└── docs/
    ├── specifications/
    ├── decisions/
    ├── tickets/
    ├── evidence/
    └── releases/
```

Root directories must remain limited. New deployables or major roots require an ADR.

## 2. Backend module structure

Example module:

```text
internal/procurement/tender/
├── domain/
│   ├── tender.go
│   ├── requirement.go
│   ├── criterion.go
│   ├── policy.go
│   ├── events.go
│   └── errors.go
├── application/
│   ├── commands/
│   ├── queries/
│   ├── ports.go
│   ├── dto.go
│   └── service.go
├── adapters/
│   ├── postgres/
│   ├── http/
│   ├── messaging/
│   └── document/
└── module.go
```

Rules:

- Domain packages do not import application/adapters.
- Application packages depend on domain and declared ports.
- Adapters implement ports.
- Module composition wires dependencies in `module.go` or root composition.
- Another module cannot import an adapter or repository directly.

## 3. Command/query pattern

A command changes state and runs inside an explicit transaction. A query returns an authorization-filtered read model.

Example command contract:

```go
type PublishTenderCommand struct {
    TenantID       uuid.UUID
    TenderID       uuid.UUID
    ExpectedVersion int64
    Actor           auth.Actor
    IdempotencyKey  string
}

type PublishTenderResult struct {
    TenderID          uuid.UUID
    PublicationID     uuid.UUID
    PublicationVersion int
    State             string
}
```

Application handler responsibilities:

1. Validate syntax-level command requirements.
2. Load tenant-scoped aggregate through repository port.
3. Authorize action using actor, resource, state, and policy.
4. Execute domain method.
5. Persist aggregate and outbox events atomically.
6. Record audit facts.
7. Return stable DTO.

## 4. Transaction handling

Use an application transaction manager port:

```go
type TxManager interface {
    WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
```

Repositories obtain transaction-bound `pgx` handles from context or explicit unit-of-work object. Do not begin nested implicit transactions inside repositories.

Every state-changing use case must define:

- Isolation expectation
- Concurrency/version check
- Idempotency behavior
- Outbox events
- Audit record
- Retry safety

## 5. Database access

Recommended baseline:

- `pgx` for PostgreSQL connectivity
- `sqlc` for typed query generation
- Explicit SQL migrations
- Repository adapters translating between rows and domain objects

Do not expose generated database models as API DTOs or domain entities.

## 6. Tenant request lifecycle

Middleware order:

```text
1. Request ID / trace context
2. Panic recovery and secure error boundary
3. Security headers
4. Body/size limits
5. CORS and preflight handling
6. Authentication/session validation
7. CSRF validation where applicable
8. Rate limit / abuse controls
9. Active context resolution
10. Tenant registry and database resolution
11. Route-level capability/resource authorization
12. Input binding and validation
13. Idempotency acquisition for qualifying writes
14. Handler/use case
15. Audit completion and metrics
16. Standard response envelope
```

Context object passed to use cases includes:

- Request/correlation ID
- Identity and actor
- Active tenant and tenant type
- Effective capabilities/scopes
- Session assurance/step-up state
- Tenant database handle provider
- Locale/timezone

## 7. Tenant database router

Responsibilities:

- Accept only authenticated tenant context.
- Load database locator and status from a signed/cacheable control-plane registry record.
- Reject inactive, migrating-incompatible, or unavailable tenants.
- Acquire a bounded connection from the correct cluster/database.
- Apply statement timeout, application name, trace metadata, and transaction settings.
- Never cache raw secrets in logs or frontend-visible objects.

Suggested interfaces:

```go
type TenantDBRouter interface {
    Acquire(ctx context.Context, tenant auth.TenantContext) (TenantDB, error)
}

type TenantDB interface {
    Pool() *pgxpool.Pool
    SchemaVersion() int
    CloseLease()
}
```

Pool cache must enforce maximum active tenant pools, idle expiry, health checks, and per-cluster connection budget.

## 8. Authorization design

Use layered RBAC + ABAC/capability checks.

Inputs:

- Tenant type and membership state
- Roles and direct grants/denies
- Department/team scope
- Resource ownership/assignment
- Tender/workflow state
- Data classification
- Separation-of-duty rules
- Session assurance

Example capability names:

```text
platform.tenants.read
platform.tenants.suspend
procurement.tender.create
procurement.tender.publish
procurement.submission.open
evaluation.technical.score
evaluation.financial.view
tenderpreneur.bid.manage
tenderpreneur.pricing.view
tenderpreneur.submission.execute
document.confidential.download
```

HTTP handlers never authorize by role name alone. They call an authorization service with action, resource, and context.

## 9. Error design

Domain/application errors map to stable API codes:

```json
{
  "error": {
    "code": "TENDER_PUBLICATION_BLOCKED",
    "message": "The tender cannot be published until blocking issues are resolved.",
    "details": {
      "blocking_issue_ids": ["..."],
      "current_state": "INTERNAL_REVIEW"
    },
    "field_errors": [],
    "trace_id": "01J...",
    "retryable": false
  }
}
```

Categories:

- Validation: 400/422
- Unauthenticated: 401
- Authenticated but forbidden: 403
- Not found or intentionally concealed: 404
- State/version/idempotency conflict: 409
- Rate/quota: 429
- Dependency unavailable: 502/503
- Unexpected: 500 with safe message

## 10. Idempotency design

Qualifying endpoints require `Idempotency-Key`.

The idempotency record stores:

- Tenant and actor
- Endpoint/operation
- Key and request hash
- Status: processing/completed/failed-retryable
- Response code/body reference
- Expiry

Same key + different request hash returns conflict. Database uniqueness prevents duplicate processing.

## 11. Optimistic concurrency

Mutable aggregates include `version BIGINT`.

Updates use:

```sql
UPDATE tenders
SET ..., version = version + 1
WHERE id = $1 AND version = $2;
```

Zero rows means stale version and returns a conflict with current summary. Frontend must offer refresh/compare, not silently overwrite.

## 12. Outbox/inbox implementation

### Outbox table

- `event_id`
- `tenant_id` or plane
- `aggregate_type/id`
- `event_type/version`
- `payload JSONB`
- `occurred_at`
- `correlation_id/causation_id`
- `classification`
- `published_at`
- `attempts/last_error/next_attempt_at`

### Inbox table

- `consumer`
- `event_id`
- `received_at`
- `processed_at`
- `status`
- `attempts/last_error`

Consumer transaction checks/creates inbox record and applies side effects atomically where possible.

## 13. Submission sealing design

Components:

- `SubmissionAssembler`
- `SubmissionValidator`
- `ManifestHasher`
- `EnvelopeEncryptor`
- `KeyManagementPort`
- `SubmissionRepository`
- `ReceiptSigner`
- `OpeningPolicy`
- `ReleaseService`

Envelope manifest includes:

- Publication and version
- Bidder tenant
- Submission and replacement lineage
- Files/structured response entries with size, media type, checksum, classification, and envelope
- Final response version
- Submission timestamp and receipt reference

The exact cryptographic scheme and key-release process must be approved through a dedicated security ADR and external review.

## 14. Frontend structure

```text
web/src/
├── app/
│   ├── (public)/
│   ├── (auth)/
│   ├── platform/
│   ├── procurement/
│   └── tenderpreneur/
├── features/
│   ├── auth/
│   ├── platform-admin/
│   ├── procurement/
│   ├── tenderpreneur/
│   ├── documents/
│   ├── workflow/
│   ├── search/
│   └── ai/
├── components/
│   ├── ui/
│   ├── layout/
│   ├── data-display/
│   ├── forms/
│   └── feedback/
├── contracts/       # generated/validated API types
├── lib/api/
├── lib/auth/
├── lib/i18n/
└── lib/telemetry/
```

Rules:

- Feature folders own page-specific components and hooks.
- `components/ui` contains design-system primitives, not domain logic.
- API calls use typed clients and centralized error mapping.
- Server state uses a query/cache library where beneficial; form state remains local and schema-validated.
- Permission checks improve UX only; protected APIs enforce authority.

## 15. AI service structure

```text
ai/app/
├── domain/
│   ├── jobs.py
│   ├── citations.py
│   ├── results.py
│   └── policies.py
├── use_cases/
│   ├── extract_tender.py
│   ├── match_compliance.py
│   ├── draft_response.py
│   ├── review_tender.py
│   ├── compare_bids.py
│   └── deep_search.py
├── ports/
│   ├── llm.py
│   ├── embeddings.py
│   ├── retrieval.py
│   ├── ocr.py
│   ├── source_fetch.py
│   └── job_store.py
├── adapters/
├── workflows/
├── prompts/
├── schemas/
└── evals/
```

Prompts are versioned assets with tests. Provider SDK objects do not leak into domain output schemas.

## 16. Key sequence — publish tender

```text
Web → POST /v1/procurement/tenders/{id}/publish
API → authenticate/context/authorize/idempotency
Application → load approved Tender
Domain → validate publication invariants and emit TenderPublished
Repository → persist publication intent + outbox in tenant DB
API → return publication pending/complete state
Worker → consume outbox, build public projection
Exchange DB/Object Store → commit manifest and files
Worker → publish catalogue event and tenant acknowledgement
Web → observe status through query/SSE/polling
```

## 17. Key sequence — external import

```text
Web → request upload URL
API → authorize and create quarantined document record
Browser → multipart upload to object store
Worker → malware/type/hash checks
AI worker → extract/OCR/chunk and propose facts
API → expose proposed facts with confidence/source refs
User → verify/correct
API → create external opportunity and bid workspace in tenant DB
```

## 18. Key sequence — submit sealed bid

```text
Web → request final readiness
API → deterministic validation + current publication check
Web → user confirms declarations and step-up
Web → POST submission with idempotency key
API → lock approved response version
Assembler → create manifest
Encryptor/KMS → encrypt objects and envelope key
Exchange repository → store sealed metadata
Receipt signer → create receipt
Outbox → notify procurement metadata-only
API → return acknowledgement
```

## 19. Key sequence — AI response drafting

```text
Web → request draft for requirement
Core API → authorize and create AI job with references/budget
AI worker → resolve permitted tender and tenant knowledge chunks
Retriever → return source-linked context
LLM adapter → generate structured draft
Validator → schema, citation, unsupported claim, safety checks
Job store → persist output and metrics
Web → display generated label, sources, warnings, accept/edit/reject
Core API → save accepted text as unverified draft until normal review
```

## 20. Coding standards

- Context-aware functions and timeouts
- No global mutable state
- Structured logging only
- Explicit errors, no swallowed failures
- Interfaces owned by the consumer package
- Small domain methods with invariant tests
- Migrations reviewed with data/backfill/rollback plan
- Generated code committed or reproducibly generated in CI
- No business rule implemented exclusively in UI, SQL trigger, or AI prompt

