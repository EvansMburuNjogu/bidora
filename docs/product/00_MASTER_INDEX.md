# TenderOS — Master Specification Index

**Working title:** TenderOS — AI Procurement & Tender Platform  
**Document set version:** 1.0  
**Primary market:** Kenya, with internationalization and country-pack support  
**Delivery model:** Multi-tenant SaaS with one private PostgreSQL database per tenant  
**Portals:** Platform Admin, Procurement Team, Tenderpreneur

## 1. Purpose of this pack

This specification pack defines the business, product, system, architecture, data, API, frontend, AI, security, quality, roadmap, and delivery requirements for TenderOS.

TenderOS supports two complementary operating models:

1. **Open Tender Model:** Tenderpreneurs discover or import tenders from any lawful external source, prepare compliant bids, submit through the external channel, and retain submission evidence.
2. **Closed Platform Tender Model:** Procurement teams create and publish tenders inside TenderOS; Tenderpreneurs respond through TenderOS; bids remain sealed until authorized opening; evaluation and award are completed within the platform.

## 2. Product vision

Create an AI-assisted procurement and tender operating system that enables:

- Procurement teams to plan, author, approve, publish, clarify, receive, open, evaluate, award, and learn from procurements.
- Tenderpreneurs to discover, import, qualify, prepare, review, submit, and track bids from any source.
- Platform administrators to govern tenants, plans, usage, billing, integrations, security, and platform operations.

AI may extract, draft, compare, summarize, recommend, and flag risk. It must not autonomously approve, disqualify, score conclusively, award, or submit on behalf of an authorized human.

## 3. Specification authority

When documents conflict, use the following precedence unless a signed decision record says otherwise:

1. Approved business decisions and legal obligations
2. `02_BRD.md`
3. `03_PRD.md`
4. `04_SRD.md`
5. `05_DOMAIN_MODEL_AND_BUSINESS_RULES.md`
6. `08_HLD.md`, `09_LLD.md`, and component design documents
7. `14_SECURITY_PRIVACY_COMPLIANCE.md`
8. `15_QA_TEST_STRATEGY.md`
9. `16_PRODUCT_ROADMAP.md` and `17_DELIVERY_PLAN_WAVES_SPRINTS.md`
10. Tickets, implementation notes, and code

Code is evidence of current implementation; it is not permitted to silently redefine approved product behavior.

## 4. Operation-driven delivery rule

Development is ordered by real operations, not by dashboards or isolated CRUD screens.

For every capability, implement in this sequence:

1. Confirm business operation and acceptance criteria.
2. Define domain rules, state transitions, permissions, and failure cases.
3. Design data ownership, schema, constraints, migrations, and seed/reference data.
4. Define API contracts and events.
5. Implement backend domain logic, authorization, idempotency, and audit.
6. Implement frontend routes, navigation, pages, forms, states, and accessibility.
7. Integrate the complete persona journey.
8. Verify with automated tests and evidence.
9. Run internal UAT using realistic data.
10. Mark complete only when the target user can finish the operation end to end.

A page showing a status, count, or placeholder is not functional completion.

## 5. Multi-tenancy rule

Each Procurement or Tenderpreneur organization is a tenant with its own private database. Tenant context is established from authenticated membership and a selected workspace. A client-supplied tenant header may assist routing only; it can never establish authorization.

Shared platform data is restricted to the control plane and exchange plane. Private tenant content must never be joined directly across tenant databases.

## 6. Document catalogue

| File | Purpose |
|---|---|
| `01_PRODUCT_DEFINITION.md` | Vision, product boundaries, users, value proposition, and success outcomes |
| `02_BRD.md` | Business requirements, stakeholders, objectives, commercial model, and business acceptance |
| `03_PRD.md` | Product capabilities, portal requirements, functional requirements, releases, and product acceptance |
| `04_SRD.md` | System requirements, non-functional requirements, integrations, environments, and technical constraints |
| `05_DOMAIN_MODEL_AND_BUSINESS_RULES.md` | Bounded contexts, aggregates, state machines, invariants, and domain events |
| `06_USER_JOURNEYS_AND_WORKFLOWS.md` | Persona journeys, end-to-end workflows, exceptions, and handoffs |
| `07_MODULE_DEPENDENCIES.md` | Dependency-ordered module catalogue and delivery prerequisites |
| `08_HLD.md` | High-level architecture, planes, services, data flows, deployment, and scalability |
| `09_LLD.md` | Code-level design, packages, request lifecycle, transactions, jobs, and sequence designs |
| `10_DATABASE_DESIGN.md` | Control-plane, tenant, exchange, search, audit, migration, backup, and retention design |
| `11_BACKEND_API_DESIGN.md` | API conventions, routes, contracts, errors, events, authorization, and CORS |
| `12_FRONTEND_UX_DESIGN.md` | Portal information architecture, routes, navigation, page contracts, visual and interaction design |
| `13_AI_DEEP_SEARCH_SPEC.md` | AI architecture, document ingestion, grounded generation, deep search, ranking, safety, and evaluation |
| `14_SECURITY_PRIVACY_COMPLIANCE.md` | Security model, tenant isolation, sealed bids, privacy, secure SDLC, incident response, and compliance |
| `15_QA_TEST_STRATEGY.md` | Test strategy, quality gates, tenant isolation, security, performance, accessibility, AI evaluation, and UAT |
| `16_PRODUCT_ROADMAP.md` | Product horizons, releases, outcomes, metrics, and exit criteria |
| `17_DELIVERY_PLAN_WAVES_SPRINTS.md` | Dependency-ordered waves and sprint-level database → backend → frontend → QA plan |
| `18_DEVOPS_DEPLOYMENT_OPERATIONS.md` | Environments, containers, CI/CD, provisioning, observability, backup, DR, and runbooks |
| `19_TRACEABILITY_ACCEPTANCE_EVIDENCE.md` | Requirement identifiers, traceability matrix, acceptance evidence, and sign-off rules |
| `20_FRONTEND_TEMPLATE_ADOPTION.md` | Controlled reverse engineering and adoption of the downloaded frontend template |
| `21_TECHNOLOGY_DECISIONS.md` | Chosen technology stack, rationale, constraints, and alternatives |
| `22_GLOSSARY_ASSUMPTIONS_DECISIONS.md` | Terminology, assumptions, unresolved decisions, and decision-log format |
| `23_AGENTS.md` | Repository and execution rules for human and AI implementation agents |

## 7. Requirement identifiers

Use stable identifiers throughout tickets, commits, tests, evidence, and release notes.

- `BR-*`: Business requirement
- `PR-*`: Product requirement
- `SR-*`: System requirement
- `RULE-*`: Business rule or invariant
- `SEC-*`: Security requirement
- `PRIV-*`: Privacy requirement
- `NFR-*`: Non-functional requirement
- `AI-*`: AI requirement
- `SEARCH-*`: Deep-search requirement
- `DATA-*`: Data/database requirement
- `API-*`: API contract requirement
- `UX-*`: Frontend/experience requirement
- `QA-*`: Quality requirement
- `OPS-*`: Operations/DevOps requirement

## 8. Baseline implementation approach

- **Architecture:** DDD-aware modular monolith for the core Go backend, with hexagonal boundaries and separately deployable AI/search workers.
- **Backend:** Go and Fiber.
- **Database:** PostgreSQL, one database per tenant, plus platform control and exchange databases.
- **Frontend:** Next.js App Router, TypeScript, shadcn/ui, and Tailwind CSS.
- **AI:** Python, FastAPI, LangChain/LangGraph where useful, provider adapters, structured outputs, source-grounded evidence, and evaluation datasets.
- **Storage:** S3-compatible object storage, with MinIO for local/private deployments.
- **Messaging:** Transactional outbox/inbox and a message broker adapter.
- **Containers:** Docker and Docker Compose for local development and integration testing; orchestrated production deployment as scale requires.
- **Observability:** OpenTelemetry, metrics, logs, traces, alerting, and immutable audit evidence.

Versions must be pinned in the repository at implementation kickoff. Upgrades require compatibility tests and an approved architecture decision.

## 9. Definition of complete

A module is complete only when:

- Its database migration and rollback strategy are approved and tested.
- Backend business rules, authorization, audit, and error behavior are implemented.
- OpenAPI contracts and events match implementation.
- Frontend operations are available to every intended role and hidden from unauthorized roles.
- Loading, empty, success, validation, conflict, forbidden, offline, and server-error states are implemented.
- Unit, integration, contract, authorization, end-to-end, and relevant performance/security tests pass.
- No mock or placeholder is used in the accepted production workflow.
- Evidence is recorded in the traceability register.
- The target persona completes the end-to-end journey in UAT.

