# TenderOS — High-Level Design (HLD)

## 1. Architecture objectives

The architecture must:

- Enforce strong tenant isolation through one private database per tenant.
- Support public and cross-tenant tender transactions without direct cross-tenant database access.
- Deliver complete procurement and Tenderpreneur workflows before advanced AI.
- Scale from an MVP modular monolith to distributed services without rewriting domain contracts.
- Provide cryptographically controlled sealed submissions and auditable opening.
- Keep AI provider-neutral, source-grounded, and outside final decision authority.
- Support Kenya first while permitting country packs and international deployment.

## 2. Architectural style

### 2.1 Core platform

Use a **DDD-aware modular monolith** implemented in Go/Fiber.

Reasons:

- Most business operations require strong transactions and consistent state.
- A small-to-medium engineering team can deliver and operate one coherent backend faster than many microservices.
- Bounded contexts, ports/adapters, module ownership, outbox events, and separate schemas/packages preserve future extraction options.
- AI, crawling, and resource-intensive document processing are naturally separate worker workloads and may deploy independently.

### 2.2 Hexagonal boundaries

Each module contains:

```text
Domain
  ├─ Aggregates, entities, value objects, policies, domain events
Application
  ├─ Commands, queries, use cases, DTOs, ports
Adapters
  ├─ HTTP, database, message, storage, email, payment, AI, source connectors
Infrastructure
  └─ Composition, configuration, observability, runtime
```

Domain and application layers must not import Fiber, PostgreSQL drivers, cloud SDKs, model SDKs, or frontend concerns.

## 3. System overview

```text
                               PUBLIC INTERNET
                                      │
                         CDN / WAF / Load Balancer
                                      │
                  ┌───────────────────┴───────────────────┐
                  │                                       │
          Next.js Web Application                 Public/API Clients
     Public + 3 authenticated portals                     │
                  │                                       │
                  └───────────────────┬───────────────────┘
                                      │ HTTPS
                              Go/Fiber Core API
                                      │
       ┌──────────────────────────────┼──────────────────────────────┐
       │                              │                              │
Control Plane Modules          Tenant Domain Modules          Exchange Modules
Tenant registry, IAM,          Procurement/Tenderpreneur      Catalogue, sealed
plans, billing, sources        workflows and documents        submissions, outcomes
       │                              │                              │
Control PostgreSQL             Tenant DB Router               Exchange PostgreSQL
                                      │
                     ┌────────────────┴─────────────────┐
                     │                                  │
             Procurement DBs                    Tenderpreneur DBs
             one DB per tenant                  one DB per tenant

       ┌─────────────────────────────────────────────────────────────┐
       │ Shared technical services                                  │
       │ S3/MinIO | NATS/Rabbit adapter | Redis | Email | Payments   │
       │ OpenTelemetry | Metrics/Logs/Traces | KMS/Secrets           │
       └─────────────────────────────────────────────────────────────┘
                                      │
                         Python AI & Search Services
                 extraction, OCR, RAG, deep search, ranking, evals
                                      │
                   Model / embedding / OCR / search adapters
```

## 4. Data ownership model

## 4.1 Platform control database

Owns only platform-wide metadata:

- Identities, authentication factors, sessions, and recovery state
- Tenant registry and database routing references
- Membership directory and high-level role grants
- Plans, subscriptions, entitlements, usage aggregates, invoices, and payment state
- Country packs, feature flags, source registry, integration metadata
- Provisioning and tenant-schema orchestration
- Platform audit and support-access sessions

Private tenant workflow data does not belong here.

## 4.2 Tenant databases

Every organization receives a private PostgreSQL database with a common versioned schema appropriate to its tenant type.

Advantages:

- Stronger isolation and simpler data export/restore per tenant
- Tenant-specific backup/retention and enterprise placement
- Reduced risk of accidental cross-tenant SQL access

Operational costs:

- More connection pools, migrations, backups, monitoring, and provisioning
- Need for database fleet orchestration and schema-version compatibility

Mitigations:

- Cluster assignment and tenant database registry
- Lazy bounded pools through PgBouncer and application pool management
- Migration controller with canaries and batch rollout
- Standard health, backup, and restore automation
- Dedicated clusters for large enterprise tenants

## 4.3 Exchange database

The exchange database is not a tenant data warehouse. It stores controlled transaction metadata and public projections:

- Public tender catalogue and publication manifests
- Tender invitations and participation registration
- Sealed submission metadata and encrypted-object references
- Submission acknowledgements, withdrawal/replacement lineage
- Opening release state
- Award/outcome exchange notifications
- Delivery receipts and cross-plane event state

It must not hold Tenderpreneur draft content or procurement internal evaluation notes.

## 4.4 Object storage

Use logical namespaces and independent access policies:

```text
platform-control/
tenants/{tenant-id}/private/
tenants/{tenant-id}/exports/
public-tenders/{publication-id}/{version}/
sealed-submissions/{tender-id}/{submission-id}/
platform-audit/
```

Object keys are opaque. Applications do not trust user-provided object paths. Downloads use short-lived signed URLs or streamed authorization.

## 5. Cross-tenant transaction design

## 5.1 Tender publication

```text
Procurement tenant approves Tender vN
→ tenant transaction writes authoritative version + outbox event
→ publication worker validates projection policy
→ public documents copied/referenced into public namespace
→ exchange database stores immutable publication manifest and hash
→ public catalogue/search index updates
→ procurement tenant records delivery acknowledgement
```

No public projection is created from an unapproved draft.

## 5.2 Tenderpreneur workspace creation

```text
Tenderpreneur selects platform tender
→ API reads public exchange projection
→ local tenant DB creates linked working snapshot
→ local requirements/tasks/responses remain private
→ later addenda arrive through exchange events and are applied through inbox
```

## 5.3 Sealed submission

```text
Tenderpreneur finalizes approved response
→ submission service revalidates publication and deadline
→ deterministic validation runs
→ manifest and content are hashed
→ data encryption key (DEK) is generated
→ submission objects are encrypted with DEK
→ DEK is encrypted through KMS/opening policy
→ exchange DB records sealed envelope metadata
→ receipt is signed/hashed and returned
→ procurement tenant receives metadata-only event
```

The procurement application has no decryption path before valid opening release.

## 5.4 Opening

```text
Authorized committee starts session
→ API verifies tender closed, opening time, quorum, roles, conflicts, and step-up
→ key-release service authorizes release
→ each envelope manifest/hash is verified
→ content becomes available to the evaluation workspace
→ immutable release/opening events and minutes are recorded
```

Opening key policy and evidence require a focused security design and independent review before production.

## 6. Core components

### 6.1 Web application

- Public marketing and tender catalogue
- Platform Admin route group
- Procurement Team route group
- Tenderpreneur route group
- Shared auth/onboarding/settings components
- Server-side authorization-aware data loading
- Client-side interactive tables, forms, uploads, editors, and real-time progress

### 6.2 Core API

Modules:

- IAM and context
- Tenancy and platform admin
- Subscription and usage
- Documents and knowledge
- Workflow/approvals/tasks/notifications
- Procurement planning, authoring, publishing, clarification
- Opportunity, bid workspace, proposal, readiness
- Submission, opening, evaluation, award
- Reporting and audit

### 6.3 Worker runtime

- Outbox dispatch and inbox consumption
- Notification delivery
- Usage aggregation and quota reconciliation
- Document scan/preview coordination
- Projection/index updates
- Scheduled reminders, expiry, deadline, retention, and cleanup jobs
- Tenant provisioning/migration tasks

### 6.4 AI/search service

- Document extraction and OCR coordination
- Structured tender extraction
- Retrieval and source mapping
- Deep Search query planning and result synthesis
- Organization-to-tender matching
- Proposal drafting and review
- Procurement drafting and evaluation assistance
- AI evaluation, cost, and quality telemetry

### 6.5 Source connector workers

- Official APIs/feeds
- Website fetchers permitted by policy
- Email/connector ingestion
- Document parsers
- Change detection and canonicalization

## 7. Messaging architecture

Use a broker behind an application port. The recommended baseline is NATS JetStream; RabbitMQ remains an acceptable adapter if operations experience favors it.

Rules:

- Database transaction writes domain state and outbox row.
- Dispatcher publishes versioned event.
- Consumer stores event ID in inbox before/with side effects.
- Retry with exponential backoff and bounded attempts.
- Dead-letter stream and operator replay.
- At-least-once delivery; handlers must be idempotent.
- Do not claim exactly-once end-to-end semantics.

## 8. Search architecture

### MVP

- PostgreSQL full-text search for tenant-private content
- `pgvector` for tenant-private semantic retrieval where enabled
- Public catalogue/search database and index isolated from tenant-private data
- Hybrid rank: structured filters + full text + vector similarity + business-fit features

### Scale stage

Adopt OpenSearch or another dedicated search engine when result volume, faceting, language analysis, source scale, or query load justifies it. The search port prevents domain coupling.

## 9. AI architecture

```text
User/API request
→ AI use-case authorization
→ create AI job with budget and input refs
→ ingestion/retrieval service resolves permitted sources
→ prompt/workflow orchestrator calls model/tools through adapters
→ structured output validation
→ citation and policy validation
→ persist result and metrics
→ user reviews/accepts/edits/rejects
```

Key controls:

- Provider-neutral ports
- Model allowlist by use case and data classification
- No provider training/data-retention assumption without verified contract/configuration
- Tenant-private retrieval namespace
- Prompt injection isolation and untrusted-document labeling
- Structured outputs with schema validation
- Deterministic fallback when AI is unavailable
- Offline and online evaluation datasets

## 10. Security boundaries

1. Browser/public boundary
2. API/session boundary
3. Platform privileged boundary
4. Tenant routing boundary
5. Tenant database and storage boundary
6. Exchange/sealed-bid boundary
7. AI/model-provider boundary
8. External-source connector boundary
9. Operations/backup boundary

Each boundary requires authentication, authorization, validation, logging, and data-minimization decisions.

## 11. Deployment topology

### Local development

Docker Compose services:

- `web`
- `api`
- `worker`
- `ai-api`
- `ai-worker`
- `postgres-control`
- `postgres-tenant-dev` or dynamically created tenant DBs
- `postgres-exchange`
- `minio`
- `nats`
- `redis`
- `mailpit`
- optional observability stack

### Initial production

- Managed or HA PostgreSQL clusters
- PgBouncer per cluster
- Replicated API, web, worker, and AI worker containers
- Managed S3-compatible object storage and KMS
- Broker cluster
- WAF/load balancer/CDN
- Central secrets manager
- Metrics, logs, traces, alerting, and SIEM integration

### Scale-out

- Multiple tenant database clusters with placement service
- Dedicated enterprise clusters
- Separate public search cluster
- Queue partitioning by workload/priority/region
- Regional storage and processing where legal/commercially required

## 12. Availability and resilience

- No single in-process scheduler owns critical work.
- Database transactions and outbox prevent lost domain events.
- Workers use leases/idempotency.
- External calls use timeouts, retries, circuit breakers, and bulkheads.
- AI/source failures degrade to manual workflows.
- Publication and submission operations use synchronous authoritative validation; derived projections may be eventually consistent.
- Backups and restore tests cover control, exchange, and tenant databases separately.

## 13. Architecture decisions to record

At implementation kickoff create ADRs for:

- ADR-001 Modular monolith baseline
- ADR-002 Database-per-tenant and control/exchange split
- ADR-003 Tenant routing and pool management
- ADR-004 Session and SSO architecture
- ADR-005 Submission sealing and key release
- ADR-006 Object storage and encryption
- ADR-007 Message broker choice
- ADR-008 Search engine progression
- ADR-009 AI provider and orchestration adapters
- ADR-010 Frontend template adoption
- ADR-011 Payment provider and billing authority
- ADR-012 Deployment target and infrastructure-as-code

