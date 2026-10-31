# TenderOS — System Requirements Document (SRD)

## 1. Purpose

This document translates the approved business and product requirements into system requirements. It defines system boundaries, deployable components, data ownership, interfaces, non-functional requirements, security controls, and operational constraints.

## 2. System context

TenderOS consists of:

- A public website and tender catalogue
- A Next.js web application exposing three authenticated portals
- A Go/Fiber core API and background workers
- A Python AI/search service and asynchronous workers
- A platform control-plane database
- One private PostgreSQL database per tenant
- A platform exchange database for public tender projections and cross-tenant transaction metadata
- S3-compatible object storage with tenant and exchange isolation
- Search/vector indexes separated by public and tenant-private scope
- A message broker used through transactional outbox/inbox adapters
- Email, payment, identity, AI-model, OCR, and source-connector integrations
- Observability, security monitoring, backup, and deployment services

## 3. Logical planes

### 3.1 Control plane

Owns platform-wide configuration and metadata:

- Identities and authentication state
- Tenant registry and tenant database routing metadata
- Membership directory and high-level entitlements
- Plans, subscriptions, usage, billing status, and feature flags
- Country packs and platform reference data
- Integration/source registry
- Provisioning, migration, backup, and support-access state
- Platform audit and operational metadata

The control plane does not own private procurement documents, private bids, prices, evaluation content, or tenant knowledge.

### 3.2 Tenant data plane

Each tenant receives a private PostgreSQL database and isolated storage namespace. It owns:

- Organization profile, teams, roles, and tenant configuration
- Procurement or Tenderpreneur domain records
- Documents and knowledge metadata
- Tasks, workflows, approvals, notifications, and tenant audit
- Private embeddings or search indexes
- AI jobs and outputs scoped to the tenant

### 3.3 Exchange plane

Supports interactions between tenants without direct cross-database joins:

- Public tender catalogue projections
- Publication versions and addenda manifests
- Participation/registration metadata
- Sealed submission envelope metadata
- Encrypted submission object references
- Acknowledgements and opening release state
- Outcome/notification exchange records
- Cross-tenant event delivery and receipts

The exchange plane stores the minimum data needed for the transaction. Procurement and Tenderpreneur private working records remain inside their tenant databases.

## 4. Deployable components

### SR-DEP-001 Web application

- Next.js App Router and TypeScript
- Public pages and authenticated portal route groups
- Server and client components selected by data sensitivity and interaction needs
- Shared design system implemented with shadcn/ui and Tailwind CSS
- No direct database access
- All business writes through versioned backend APIs

### SR-DEP-002 Core API

- Go and Fiber
- Modular monolith with bounded contexts and hexagonal ports/adapters
- REST/JSON APIs documented using OpenAPI 3.1
- Synchronous business commands/queries and asynchronous event publication
- Central authentication, context resolution, authorization, validation, idempotency, audit, and error handling

### SR-DEP-003 Core worker

- Go worker process using the same domain modules
- Outbox dispatch, inbox consumption, notifications, document post-processing, scheduled jobs, retries, usage aggregation, and tenant maintenance
- Horizontally scalable with distributed leases and idempotent handlers

### SR-DEP-004 AI/search API

- Python and FastAPI
- Provider-neutral model, embedding, OCR, and search ports
- LangChain/LangGraph permitted for orchestration; domain decisions remain explicit application code
- Structured output schemas and source references
- No direct authority to change procurement or submission states

### SR-DEP-005 AI/search workers

- Document extraction, OCR, classification, chunking, embeddings, retrieval, deep research, deduplication, ranking, and AI evaluation jobs
- Asynchronous job state and cancellation
- Per-tenant budgets, concurrency, and rate limits

### SR-DEP-006 Source connectors

- Adapter per source/API/feed/site
- Scheduled retrieval with rate limits, terms metadata, retry/backoff, parser version, provenance, and health monitoring
- Prefer official API/feed integrations over scraping

## 5. Identity and session requirements

- **SR-IAM-001:** Authentication shall use verified identities and secure session issuance.
- **SR-IAM-002:** Passwords, where supported, shall use a current memory-hard password hash and configurable policy.
- **SR-IAM-003:** MFA shall be available and enforceable by role, tenant, and risk policy.
- **SR-IAM-004:** OIDC/SAML enterprise SSO shall be supported through an adapter in later enterprise releases.
- **SR-IAM-005:** The system shall expose a stable current-user bootstrap contract containing identity, session expiry, memberships, active context, capabilities, preferences, and required actions.
- **SR-IAM-006:** Tenant context shall be selected from verified memberships and bound to the session or request token.
- **SR-IAM-007:** Logout, revocation, password change, role removal, suspension, and risk events shall invalidate affected sessions.
- **SR-IAM-008:** Step-up authentication shall be required for configurable high-risk actions such as support access, bid opening, final submission, award, billing change, key rotation, or export.

## 6. Tenant provisioning and routing requirements

- **SR-TEN-001:** Tenant creation shall be an asynchronous, observable workflow with states `REQUESTED`, `PROVISIONING`, `MIGRATING`, `SEEDING`, `ACTIVE`, `FAILED`, `SUSPENDED`, and `DECOMMISSIONED`.
- **SR-TEN-002:** Provisioning shall create or allocate a database, database roles, secrets, storage namespace, default settings, initial migration, and administrator membership.
- **SR-TEN-003:** The tenant router shall resolve the database from authenticated context and control-plane registry data.
- **SR-TEN-004:** Untrusted route, query, cookie, or header values shall never grant tenant access.
- **SR-TEN-005:** Connection pools shall be bounded per database/cluster and managed to prevent exhaustion as tenant count grows.
- **SR-TEN-006:** Schema versions shall be tracked for every tenant database.
- **SR-TEN-007:** Migrations shall support canary rollout, pause, retry, forward-fix, and compatibility windows.
- **SR-TEN-008:** Enterprise tenants may be assigned dedicated clusters without changing application contracts.

## 7. Functional system requirements

### 7.1 Platform administration

- **SR-PA-001:** Provide tenant, plan, entitlement, billing, usage, support-access, feature-flag, source-registry, and platform-operation APIs.
- **SR-PA-002:** Platform listings shall use control-plane metadata only unless audited support access has been granted.
- **SR-PA-003:** Privileged actions shall require strong authorization, reason capture, and audit evidence.

### 7.2 Procurement operations

- **SR-PROC-001:** Store procurement plans, requisitions, approvals, tenders, publication versions, criteria, lots, forms, clarifications, addenda, submissions, opening records, evaluations, recommendations, awards, and handovers in the procurement tenant database.
- **SR-PROC-002:** Publish an approved, immutable public projection to the exchange plane through an outbox event.
- **SR-PROC-003:** Amendments shall create a new publication version rather than modifying previously published facts in place.
- **SR-PROC-004:** Evaluation criteria used for scoring shall be frozen from the authoritative published version unless a governed amendment occurs before closing.
- **SR-PROC-005:** Opening and evaluation operations shall enforce tender state, time, committee/quorum, assignment, conflict declarations, and step-up authentication.

### 7.3 Tenderpreneur operations

- **SR-TP-001:** Store profiles, compliance records, external tenders, bid workspaces, requirements, content, pricing, approvals, packages, submission evidence, and outcomes in the Tenderpreneur tenant database.
- **SR-TP-002:** Importing a platform tender shall create a local working projection linked to the authoritative exchange-plane publication identifier and version.
- **SR-TP-003:** External tender facts shall retain source, extraction method, confidence, verification status, and last-checked time.
- **SR-TP-004:** Final submission readiness shall combine deterministic rules with optional AI findings; AI alone cannot mark a bid ready.

### 7.4 Submission exchange

- **SR-SUB-001:** Submission creation shall be idempotent and bound to a tender publication, bidder tenant, authorized submitter, and final response version.
- **SR-SUB-002:** The service shall validate eligibility to submit, closing status, required declarations, schema, package integrity, and configured limits.
- **SR-SUB-003:** Submission content shall be encrypted before or during storage and placed in the sealed exchange namespace.
- **SR-SUB-004:** The procurement tenant shall receive acknowledgement metadata but no content access before opening release.
- **SR-SUB-005:** Opening release shall require all configured conditions and produce immutable evidence of key/content release.
- **SR-SUB-006:** Withdrawal/replacement shall preserve prior envelope metadata and follow configured rules.

### 7.5 Search and AI

- **SR-AI-001:** AI jobs shall accept structured input references rather than unrestricted raw database access.
- **SR-AI-002:** Retrieval shall apply tenant and record authorization before documents/chunks reach the model.
- **SR-AI-003:** Outputs shall use versioned schemas, citations, confidence/verification state, model metadata, and safety status.
- **SR-AI-004:** Prompt, tool, retriever, model, and output-schema versions shall be recorded for reproducibility.
- **SR-AI-005:** Deep Search shall maintain source, canonical URL, first/last seen, fetched time, content hash, parser version, confidence, and authoritative-source indicator.
- **SR-AI-006:** Search result ranking shall be explainable at a product level through match factors, not hidden as an unexplained score.

## 8. API requirements

- **API-001:** All business APIs shall be versioned under `/v1` until a new incompatible major contract is introduced.
- **API-002:** Use consistent `{ "data": ..., "meta": ... }` success and `{ "error": { ... } }` failure envelopes.
- **API-003:** Use stable machine-readable error codes, trace identifiers, field errors, and retryability indicators.
- **API-004:** Create, publish, submit, open, award, and payment operations shall support idempotency keys.
- **API-005:** Mutable resources shall use optimistic concurrency through a version field and/or ETag/If-Match.
- **API-006:** List APIs shall support cursor pagination, safe filters, stable sort, and maximum page sizes.
- **API-007:** APIs shall return effective capabilities or action affordances where useful, but backend authorization remains authoritative.
- **API-008:** OpenAPI contracts shall be linted, tested, and used to generate or validate clients/types.

## 9. Eventing and consistency requirements

- **SR-EVT-001:** Domain changes and outbound events shall commit atomically through a transactional outbox.
- **SR-EVT-002:** Consumers shall record processed message identifiers in an inbox and remain idempotent.
- **SR-EVT-003:** Events shall include event ID, type, version, occurred time, tenant, actor, aggregate, correlation, causation, and payload classification.
- **SR-EVT-004:** Cross-plane events shall contain only the minimum permitted data; private details shall be referenced through controlled services.
- **SR-EVT-005:** Failed events shall use bounded retries, dead-letter handling, alerts, and replay tools.

## 10. Data and storage requirements

- **DATA-001:** PostgreSQL shall be the source of truth for transactional data.
- **DATA-002:** Object storage shall hold immutable or versioned file objects; database records shall hold metadata and authorized references.
- **DATA-003:** Every mutable business table shall include a stable identifier, lifecycle status, created/updated timestamps, actor identifiers, and version where concurrency matters.
- **DATA-004:** Money shall use decimal/numeric values with explicit ISO currency.
- **DATA-005:** Date/time values shall be stored in UTC with the source timezone retained where legally/business relevant.
- **DATA-006:** No cross-tenant SQL query or foreign key is permitted.
- **DATA-007:** Search indexes and embeddings are derived data and must be rebuildable from authorized source records.
- **DATA-008:** Soft deletion shall not replace explicit retention, archival, legal hold, anonymization, and erasure workflows.

## 11. Security requirements

- **SEC-001:** Enforce TLS for all network traffic and encryption at rest for databases, backups, object storage, and sealed submissions.
- **SEC-002:** Use least-privileged service identities and separate database roles for migration, application read/write, reporting, and backup.
- **SEC-003:** Apply authentication, context, authorization, validation, rate limit, idempotency, transaction, audit, and response-security middleware in a controlled order.
- **SEC-004:** Production CORS shall use explicit origins; wildcard origins with credentials are prohibited.
- **SEC-005:** Cookie-authenticated state-changing requests shall have CSRF protection and SameSite controls.
- **SEC-006:** Files shall be scanned, type-validated, size-limited, checksummed, and served through authorized short-lived URLs.
- **SEC-007:** AI ingestion shall defend against prompt injection, malicious documents, data exfiltration, unauthorized tool use, and cross-tenant retrieval.
- **SEC-008:** Logs shall not contain passwords, tokens, sensitive document content, raw financial bids, or excessive personal data.
- **SEC-009:** Security tests shall include cross-tenant access, privilege escalation, IDOR/BOLA, sealed-bid access, injection, SSRF, file attacks, and replay.

## 12. Performance and scalability requirements

- **NFR-PERF-001:** Stateless API and worker instances shall scale horizontally.
- **NFR-PERF-002:** Tenant database pools shall be lazy, bounded, observable, and evictable.
- **NFR-PERF-003:** Long-running extraction, indexing, research, report generation, and package operations shall use asynchronous jobs.
- **NFR-PERF-004:** Public catalogue and Deep Search reads may use caches/indexes; transactional state shall be validated against authoritative records before sensitive actions.
- **NFR-PERF-005:** Large files shall use multipart/direct uploads with server-issued scoped credentials where supported.
- **NFR-PERF-006:** Load tests shall cover tenant fan-out, closing-time submission spikes, bid opening, search bursts, and AI queues.

## 13. Reliability and recovery requirements

- **OPS-001:** Every service shall expose liveness, readiness, and dependency health.
- **OPS-002:** Backups shall be encrypted, monitored, retained according to policy, and restored in scheduled tests.
- **OPS-003:** Tenant-level restore to a new database shall be supported without overwriting current production data until approved.
- **OPS-004:** The platform shall define RPO/RTO by environment and plan.
- **OPS-005:** Critical event processing shall survive restarts and avoid silent loss through outbox/inbox persistence.
- **OPS-006:** Deployments shall support rollback or forward-fix and database/application compatibility windows.

## 14. Observability requirements

- Structured logs with trace/correlation IDs
- Distributed traces across web, API, worker, AI, storage, message, and external calls
- Service, endpoint, queue, tenant-cluster, AI, search-source, billing, and database metrics
- Business operation metrics that exclude sensitive content
- Alerts with runbooks and ownership
- Audit logs separate from diagnostic logs

## 15. Internationalization requirements

- Locale-aware formatting and translation keys
- Timezone-aware deadlines and explicit closing-time display
- ISO currency and configurable tax labels/rules
- Country-specific terminology, procurement methods, document types, and validation rules through country packs
- English first; architecture prepared for Kiswahili and additional languages
- Search and AI language detection with original-language preservation

## 16. Environments

- Local developer environment through Docker Compose
- Automated test environment with ephemeral databases/containers
- Shared development environment
- QA/integration environment
- UAT/pilot environment with sanitized or synthetic data
- Production environment
- Optional disaster-recovery environment

Production credentials, data, and AI indexes must never be reused in lower environments.

## 17. System acceptance

System acceptance requires:

- Contract, unit, integration, migration, authorization, E2E, security, performance, backup/restore, and AI evaluation gates passing
- No cross-tenant access in automated isolation suites
- Complete operational evidence for at least one external-tender and one platform-tender journey
- Observability and incident runbooks functioning in the target environment
- Approved architecture, privacy, and security review

