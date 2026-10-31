# TenderOS — Technology Decisions and Recommended Stack

## 1. Decision principles

Technology choices must support:

- Strong domain rules and transactions
- Database-per-tenant operations
- Secure cross-tenant exchange
- High-quality document and AI workloads
- Typed contracts and automated testing
- Containerized local and production operations
- Provider portability and controlled complexity

Use current stable, supported versions at implementation kickoff and pin them in lockfiles, module files, container digests, and release documentation. Do not follow release-candidate versions in production without an approved ADR.

## 2. Core stack

| Layer | Decision | Rationale |
|---|---|---|
| Web | Next.js App Router + TypeScript | Mature full-stack rendering/navigation model, typed frontend, route groups for three portals |
| UI | shadcn/ui + Tailwind CSS | Product-owned components, accessible primitives, adaptable downloaded template/design tokens |
| Core backend | Go + Fiber v3 | Strong concurrency/performance, simple deployment, explicit architecture, current Fiber middleware ecosystem |
| Transactional data | PostgreSQL | Strong relational integrity, transactions, full-text search, extensions, mature operations |
| SQL access | pgx + sqlc | Native PostgreSQL features, type-safe SQL, avoids domain leakage from active-record models |
| Migrations | SQL-first migration tool such as golang-migrate/Goose selected by ADR | Reviewable and reproducible schema changes |
| AI/search service | Python + FastAPI + Pydantic | Strong document/ML ecosystem and typed service contracts |
| AI orchestration | LangChain/LangGraph selectively | Useful workflow/tool/retrieval orchestration while domain decisions remain explicit |
| Initial AI provider | OpenAI through an adapter | Strong structured-output and model capabilities; adapter prevents provider lock-in |
| Object storage | S3-compatible; MinIO locally | Standard object API, multipart upload, lifecycle/versioning, private deployment option |
| Messaging | NATS JetStream recommended, broker port permits RabbitMQ | Durable asynchronous jobs/events with simple operations; outbox/inbox remains authoritative |
| Cache/ephemeral coordination | Redis | Rate limit, short-lived cache, distributed leases where appropriate; never source of truth |
| Search | PostgreSQL FTS + pgvector initially; OpenSearch later | Lower MVP complexity with clear scale path |
| Containers | Docker + Docker Compose | Reproducible multi-container local/integration environment |
| Observability | OpenTelemetry + Prometheus/Grafana + log/trace backend | Vendor-neutral traces/metrics and operational visibility |
| E2E testing | Playwright | Browser-level workflow and multi-role testing |
| Backend integration | Testcontainers for Go/Python | Real dependency behavior in isolated tests |

## 3. Current baseline notes

As of the specification date:

- Fiber's current documentation covers v3 and states its Go version prerequisite; pin the supported Go/Fiber pair selected at kickoff.
- PostgreSQL 18 is the current supported documentation line, while later release candidates must not be treated as production baseline.
- Next.js App Router is the current route/application model in official documentation.
- Tailwind CSS 4.x and current shadcn/ui documentation should be reconciled with the downloaded template before adoption.
- Docker Compose remains the standard local multi-container definition tool.

Exact versions belong in `backend/go.mod`, `web/package.json`, Python lockfiles, container files, and an ADR—not only in this document.

## 4. Backend details

### Go/Fiber

Use Fiber only at the HTTP adapter boundary. Domain/application code uses standard `context.Context` and project types.

Required middleware/capabilities:

- Recovery and secure error handler
- Request/trace IDs
- Authentication/session
- CSRF and CORS
- Rate limiting
- Body/file limits
- Compression where measured
- OpenTelemetry
- Secure headers

Fiber context values must not escape their request lifecycle without copying/conversion.

### Database access

Use:

- `pgxpool` for cluster/database connections
- `sqlc` for typed queries
- Explicit repositories and transaction manager
- PostgreSQL advisory locks only for carefully defined operational coordination, not general business locks
- PgBouncer for fleet connection management

Avoid:

- Cross-tenant SQL
- Dynamic table/database names from user input
- Generic repository abstractions that hide necessary SQL behavior
- ORM models as API/domain models

### Validation

- Request schema validation at adapter boundary
- Business rules in domain/application layer
- Database constraints as final integrity defense
- Shared error codes and field error structure

## 5. Frontend details

### Next.js

- App Router route groups for public/auth/platform/procurement/tenderpreneur
- Server Components for secure initial data/layout where appropriate
- Client Components only for interactive behavior
- No direct database or privileged internal-service calls
- API client generated/validated from OpenAPI
- Same-origin/proxy deployment preferred for secure session handling

### UI and styling

- shadcn/ui components copied into and owned by project
- Tailwind theme variables derived from template audit
- Radix/headless primitives as underlying dependencies where shadcn uses them
- Icon set selected once and used consistently
- No bundled proprietary font redistribution outside license

### State/forms

Recommended:

- React Hook Form
- Zod or generated schema validation aligned with OpenAPI
- TanStack Query for client-side server state where interactive caching is needed
- TanStack Table for rich operational tables

Do not duplicate server business rules in Zod; use it for client input guidance and contract shape.

## 6. AI and document processing details

### Python runtime

- FastAPI service
- Pydantic schemas
- Async job workers
- Provider interfaces for LLM, embeddings, OCR, parsing, reranking, and web/source retrieval

### Document parsing

Use adapters so the implementation can combine:

- Native PDF/DOCX/XLSX text extraction
- Page-aware PDF parsing
- OCR for scanned pages
- Table/form extraction
- Image rendering for difficult pages

No single parser should be assumed to handle every tender document. Preserve original page coordinates and quality metadata.

### LangChain/LangGraph

Use for:

- Retrieval chains
- Multi-step extraction/research workflows
- Tool orchestration with explicit allowlists
- Checkpointed long-running AI workflows where justified

Do not use for:

- Core procurement state machines
- Authorization
- Final scoring/award logic
- Database transaction management
- Hiding unreviewed prompt chains from tests/version control

### OpenAI/provider integration

Implement `LLMPort`, `EmbeddingPort`, and optional `RerankerPort`. Record provider/model/config per job. Model selection is policy-controlled by use case, classification, cost, and evaluation result.

## 7. Search and Deep Search

### MVP

- PostgreSQL metadata/full-text catalogue
- pgvector for semantic match
- Source connector workers
- Hybrid ranking in application/search layer
- Redis only for short-lived query/result cache

### Scale trigger for OpenSearch

Consider dedicated search when:

- Catalogue reaches volume where PostgreSQL search/maintenance is inefficient
- Complex facets/language analyzers dominate
- Query concurrency or ranking experiments require independent scaling
- Public search availability needs independent failure domain

Migration is through the search port and rebuildable source projections.

## 8. Messaging

Recommended NATS JetStream topology:

- `platform.*`
- `tenant.lifecycle.*`
- `document.*`
- `procurement.*`
- `tenderpreneur.*`
- `exchange.submission.*`
- `ai.jobs.*`
- `notification.*`

Use durable consumers, explicit acknowledgements, retry streams, and dead-letter handling. Payloads are versioned and minimal.

If the operating team has stronger RabbitMQ expertise, an ADR may select RabbitMQ while retaining the same outbox/inbox and message-port contracts.

## 9. Identity decision

Baseline:

- Application-owned membership, role, authorization, and tenant context
- Secure session service in Go
- OIDC adapters for Google/Microsoft/enterprise identity
- Evaluate Keycloak, ZITADEL, or a managed IdP for authentication/MFA/SSO through an ADR

Even with an external identity provider, TenderOS owns business authorization and resource policies.

## 10. Payments and communications

Use adapters for:

- Card/bank/mobile-money payment provider(s)
- Email provider
- SMS/WhatsApp provider in later releases

Payment webhooks are signed, idempotent, and reconciled. The payment provider is not the source of product entitlement until backend reconciliation succeeds.

## 11. Observability and error monitoring

- OpenTelemetry SDK/collector in Go, Next.js, and Python
- Prometheus-compatible metrics
- Grafana dashboards
- Loki/compatible logs and Tempo/compatible traces, or selected managed equivalents
- Sentry or equivalent optional for application exception diagnostics
- Security events sent to SIEM/alerting destination

Observability providers are adapters; correlation IDs remain consistent.

## 12. CI/CD tooling

The repository can run on Bitbucket/Jenkins where that is the operating organization's standard. Pipelines must still be reproducible through scripts/Make targets rather than hidden only in Jenkins UI configuration.

Recommended tools/capabilities:

- Go test/vet/staticcheck/golangci-lint as selected
- TypeScript/ESLint/Next build
- Python Ruff/mypy/pytest as selected
- OpenAPI lint/breaking-change tool
- Testcontainers
- Playwright
- Trivy/Grype and dependency scanners
- Gitleaks/secret scanner
- SBOM generation
- Docker BuildKit/buildx

## 13. Production orchestration

Start with the simplest reliable target supported by the operations team:

- Managed container platform or virtual machines with Docker for early controlled deployment
- Kubernetes when horizontal scaling, multi-service deployment, tenancy fleet operations, and enterprise requirements justify its overhead

Do not adopt Kubernetes merely because the product is multi-tenant. The database fleet, KMS, backups, and security matter more than orchestrator branding.

## 14. Technologies intentionally excluded from the core baseline

- Separate microservice per module
- Multiple backend languages for ordinary business modules
- NoSQL as primary transactional store
- Client-side direct database access
- Blockchain for bid integrity without a validated legal/business need
- Autonomous procurement agents with award authority
- One shared tenant database using only `tenant_id`, because the approved requirement is a private database per tenant

## 15. Reference links

Official references to verify during implementation:

- Go documentation: https://go.dev/doc/
- Fiber documentation: https://docs.gofiber.io/
- PostgreSQL documentation: https://www.postgresql.org/docs/current/
- Next.js App Router: https://nextjs.org/docs/app
- shadcn/ui: https://ui.shadcn.com/docs
- Tailwind CSS: https://tailwindcss.com/docs
- Docker Compose: https://docs.docker.com/compose/
- LangChain documentation: https://docs.langchain.com/
- OpenAI developer documentation: https://platform.openai.com/docs/

