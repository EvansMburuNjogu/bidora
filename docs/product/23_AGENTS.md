# TenderOS — AI and Developer Agent Guide

## Purpose

Build TenderOS as a secure, auditable, multi-tenant procurement and tender operating system. Deliver real user operations for Platform Admins, Procurement Teams, and Tenderpreneurs. Do not optimize for generated code volume, page count, or superficial readiness.

## Authoritative sources

Use project sources in this order:

1. Approved decisions and legal/policy requirements
2. `docs/specifications/02_BRD.md`
3. `docs/specifications/03_PRD.md`
4. `docs/specifications/04_SRD.md`
5. `docs/specifications/05_DOMAIN_MODEL_AND_BUSINESS_RULES.md`
6. HLD, LLD, database, API, frontend, AI, security, and QA specifications
7. Product roadmap and delivery plan
8. Approved ticket/Developer Execution Packet
9. Current implementation and evidence

When sources conflict, stop the affected change, identify the conflict and impact, and propose a decision. Do not silently choose the easiest interpretation.

## Repository contract

The approved top-level roots are:

```text
backend/        Go/Fiber API, worker, migrations, contracts, backend tests
web/            Next.js application, design system, frontend tests
ai/             Python/FastAPI AI, search, parsing, evaluation tests
infrastructure/ Docker, IaC, monitoring, deployment, runbooks
docs/           Specifications, decisions, tickets, evidence, releases
```

Do not add a new top-level runtime, duplicate application, archive, generated dump, or tooling tree without explicit approval.

## Technology rules

- Core business backend: Go only.
- HTTP adapter: Fiber.
- Transactional database: PostgreSQL.
- Database access: explicit SQL through approved pgx/sqlc pattern.
- Frontend: Next.js App Router, TypeScript, shadcn/ui, Tailwind CSS.
- AI/search/document processing: Python/FastAPI; LangChain/LangGraph only where justified.
- Local/integration environment: Docker Compose.
- Business events: transactional outbox/inbox through approved broker adapter.

Do not introduce a new framework, ORM, database, broker, AI provider, or cloud-specific dependency without an ADR or ticket approval.

## Architectural rules

- The core backend is a DDD-aware modular monolith with hexagonal boundaries.
- Each module owns its domain, application, ports, adapters, tables, APIs, and tests.
- Modules do not read each other's tables directly.
- Domain code does not import Fiber, SQL drivers, model SDKs, storage SDKs, or frontend types.
- Cross-module and cross-plane communication uses explicit application interfaces or versioned events.
- No microservice extraction without measured operational or scaling need and an ADR.

## Multi-tenancy rules

- Every Procurement or Tenderpreneur tenant has its own private database.
- Tenant database selection comes from authenticated membership and active context.
- Headers, route parameters, query parameters, or form fields never establish tenant authority.
- Never fall back to a default tenant database.
- Never create cross-tenant joins, foreign keys, database links, search namespaces, caches, or AI retrieval.
- Platform Admin does not automatically have private tenant access.
- Support access must be scoped, approved, expiring, and audited.

Every data-access change must include negative cross-tenant tests.

## Control, tenant, and exchange planes

- Control plane: identities, tenant registry, plans, usage, billing, platform configuration.
- Tenant plane: private procurement or Tenderpreneur business data.
- Exchange plane: approved public tender projections and minimum sealed-submission/outcome metadata.

Do not copy private drafts, pricing, evaluation notes, or tenant knowledge into the control or public exchange plane.

## Operation-driven implementation

For every ticket, work in this order:

1. Understand the actor, operation, business outcome, and failure cases.
2. Confirm dependencies and authoritative requirements.
3. Define state changes, invariants, permissions, audit, and evidence.
4. Implement database migrations, constraints, queries, and seed/reference data.
5. Implement backend domain/application logic and tests.
6. Implement/adjust OpenAPI and event contracts.
7. Implement frontend route, navigation, page, form, all UI states, and accessibility.
8. Integrate with real backend data.
9. Run unit, integration, contract, authorization, E2E, and relevant security/performance tests.
10. Record evidence and update docs.

A rendered page, endpoint stub, repository method, status card, or mock workflow is not completion.

## Database rules

- Migrations are immutable after release.
- Every migration has verification and rollout/recovery notes.
- Use database constraints for integrity in addition to domain validation.
- Use transactions explicitly.
- Use optimistic concurrency for mutable aggregates.
- Store money as decimal plus currency, not float.
- Store timestamps in UTC and preserve source timezone for deadlines.
- JSONB does not replace core relational modeling.
- Do not expose database rows as domain or API models.
- Do not modify production data manually to make a workflow appear complete.

## Backend rules

- Backend is the source of business truth.
- Handlers remain thin; use cases own application flow; aggregates/policies own invariants.
- Authorization runs server-side for every operation and list/search result.
- Use stable error codes and correct 401/403/404/409/422/429 semantics.
- Critical creates/transitions support idempotency.
- Every material transition produces audit and required events.
- External calls have timeout, retry policy, circuit breaker/bulkhead where needed.
- Do not swallow errors or log secrets/private document content.

## API rules

- OpenAPI 3.1 contracts are versioned and reviewed before frontend dependency.
- Use standard success/error envelopes.
- Use cursor pagination for large lists.
- Use ETag/expected version for concurrency-sensitive resources.
- Never expose internal database locator, secret reference, raw object key, stack trace, or unauthorized field.
- Tenant headers are routing hints only after authenticated-context match.

## Frontend rules

- Use the approved route/page/navigation design.
- The downloaded template is a design input, not product authority.
- Do not preserve mock pages/data as accepted functionality.
- Do not ask users to type record IDs; use appropriate linked-record controls.
- Use real file uploaders for files, structured selectors for related data, and explicit timezone/currency controls.
- Implement loading, empty, validation, forbidden, not found, conflict, offline/network, and server-error states.
- Provide contextual back navigation.
- Do not fill the interface with decorative cards when tables, work queues, timelines, or matrices are more useful.
- Hide unauthorized actions for usability, but rely on backend authorization.
- Clear tenant-scoped state/cache on context switch.
- Meet accessibility requirements and test keyboard/responsive behavior.

## Procurement integrity rules

- Published tender versions are immutable.
- Material changes use addenda and versioning.
- Evaluation uses only published criteria.
- Procurement users cannot access sealed submission content before authorized opening.
- Opening requires time/state/quorum/role/conflict/step-up controls.
- Scores, disqualifications, recommendations, approvals, and awards remain human-controlled and auditable.
- AI suggestions must be visibly advisory.

## Tenderpreneur rules

- External tenders are first-class and retain provenance/verification state.
- Mandatory requirements, evidence versions, owners, and readiness must be explicit.
- Financial response access is independently restricted.
- Approved submission versions are immutable; changes invalidate affected approvals.
- External tender status cannot become submitted without evidence or approved exception.

## AI rules

- AI assists; it is not the source of truth.
- Retrieve only records the current job is authorized to access.
- Treat tender/web/document content as untrusted and defend against prompt injection.
- Material outputs include citations or are labeled unsupported.
- Store prompt/workflow/model/tool/schema versions and usage metrics.
- Do not invent company experience, staff, certificates, prices, requirements, scores, or approvals.
- Do not allow model output to execute SQL, shell, arbitrary URL fetch, email, publication, submission, opening, scoring, or award actions.
- AI changes require benchmark regression.

## Deep Search rules

- Use only approved lawful sources/connectors.
- Prefer official source/API/feed where available.
- Preserve source URL/ID, first/last seen, fetch time, content hash, parser version, confidence, and authoritative status.
- Consolidate duplicates without discarding source history.
- Show freshness and uncertainty.
- Do not present inferred data as verified fact.
- Source changes must propagate to linked workspaces and may invalidate readiness.

## Security rules

- Default deny and least privilege.
- Explicit CORS origins; no credentialed wildcard.
- CSRF protection for cookie-authenticated writes.
- Secure, HttpOnly session cookies; no long-lived access token in localStorage.
- Scan and quarantine uploads.
- Prevent SSRF in URL imports/connectors.
- No secrets in source, logs, errors, browser bundles, fixtures, or screenshots.
- Redact personal/confidential data from telemetry.
- Security controls are not disabled to make tests pass.

## Testing rules

Minimum for each material change:

- Unit tests for rules/state
- Database/repository integration tests
- API/contract tests
- Positive and negative authorization tests
- Cross-tenant tests when data is involved
- Frontend component/E2E tests for the operation
- Accessibility checks for user-facing changes
- Migration test for schema change
- AI evaluation for prompt/model/retrieval change

A test that asserts only HTTP 200 or page render is insufficient for business completion.

## Evidence and completion

Before claiming complete, provide:

- Files changed
- Requirement IDs satisfied
- Migration/schema version
- API/event contracts changed
- Tests executed and exact results
- UAT/evidence artifact path
- Known limitations and remaining risks
- Deployment/feature-flag status

Use truthful states such as `IMPLEMENTED_NOT_VERIFIED` or `UAT_PENDING` when appropriate. Never state “complete,” “production-ready,” “secure,” or “compliant” without the required evidence.

## Dependency discipline

Follow `07_MODULE_DEPENDENCIES.md` and `17_DELIVERY_PLAN_WAVES_SPRINTS.md`.

Do not build:

- Evaluation before opening and published criteria
- Submission before approved response and exchange controls
- AI automation before deterministic workflow
- Analytics as a substitute for operational records
- Frontend screens before required data/API/authorization contracts
- Downstream modules using placeholders for missing foundations

## Decision flow

When uncertain:

```text
Investigate → cite current evidence → recommend options
→ record decision/assumption → implement → validate → document evidence
```

Do not silently guess. Do not stop merely because a problem is difficult; make the safest evidence-based progress within the ticket scope and record unresolved decisions.

