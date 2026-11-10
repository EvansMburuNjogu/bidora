# Bidora technical design index

Status: historical design baseline, not a description of current source or implementation permissions. The application now exists in `backend/` and `web/`; use [current development instructions](../../AGENTS.md), [the single delivery tracker](../tickets/PLATFORM_ADMIN_DELIVERY.md) and [runtime/API references](../../README.md) for active work. Approved stack/principles and historical decisions remain preserved below.

## Reading order and authority

Read decisions → system/domain/tenancy → schema/transactions → API/identity → frontend/storage/events → security/operations/QA → implementation. Lower-level design cannot silently override an approved product rule. [Root AGENTS](../../AGENTS.md) defines escalation and future execution workflow. [Implementation index](../implementation/00_INDEX.md) maps workstreams and tasks.

| Document | Purpose / dependency |
|---|---|
| [01 System architecture](01_SYSTEM_ARCHITECTURE.md) | Planes, deployables and external workflow; depends decisions |
| [02 Repository architecture](02_REPOSITORY_ARCHITECTURE.md) | Ownership/imports and unresolved frontend path |
| [03 Domain architecture](03_DOMAIN_ARCHITECTURE.md) | Commands, immutable versions and state transitions |
| [04 Multi-tenancy](04_MULTI_TENANCY_DESIGN.md) | Context, provisioning, pools and revocation |
| [05 Database architecture](05_DATABASE_ARCHITECTURE.md) | Ownership, representation and transactions |
| [06 Schema design](06_DATABASE_SCHEMA_DESIGN.md) | Core table catalogue, relationships and proposed DDL |
| [07 Migrations](07_DATABASE_MIGRATIONS.md) | Compatible fleet rollout and recovery |
| [08 Backend architecture](08_BACKEND_ARCHITECTURE.md) | Ports, pipeline, errors and lifecycle |
| [09 Module design](09_BACKEND_MODULE_DESIGN.md) | Owner commands, events, transactions and tests |
| [10 API contracts](10_API_CONTRACT_DESIGN.md) | External-MVP endpoint cards, DTOs and compatibility |
| [11 Identity/authorization](11_AUTHENTICATION_AUTHORIZATION.md) | Sessions, policy, context and assurance |
| [12 Frontend architecture](12_FRONTEND_ARCHITECTURE.md) | Route/page contracts and accessibility |
| [13 Components](13_FRONTEND_COMPONENT_DESIGN.md) | Matrix, evidence, approval and job composition |
| [14 State/data flow](14_STATE_MANAGEMENT_DATA_FLOW.md) | Cache scope, context races and mutation recovery |
| [15 Documents/storage](15_DOCUMENT_STORAGE_DESIGN.md) | Immutable upload/scan/evidence/package lifecycle |
| [16 Integrations](16_INTEGRATION_ARCHITECTURE.md) | Provider ports, security and ambiguous failure recovery |
| [17 Events/messaging](17_EVENT_MESSAGING_DESIGN.md) | Envelope, outbox/inbox and durable workers |
| [18 AI/search](18_AI_SEARCH_ARCHITECTURE.md) | Later grounded, authorized advisory processing |
| [19 Security](19_SECURITY_ARCHITECTURE.md) | Threat/control/negative evidence matrix |
| [20 Audit/evidence](20_AUDIT_LOGGING_EVIDENCE.md) | Business history versus diagnostics and acceptance proof |
| [21 Deployment](21_DEPLOYMENT_ARCHITECTURE.md) | Local/staging/production boundaries, no cloud assumed |
| [22 CI/CD](22_DEVOPS_CICD_DESIGN.md) | Reproducible checks, promotion and migration gates |
| [23 Observability](23_OBSERVABILITY_OPERATIONS.md) | Signals, SLOs, runbooks and safe operations |
| [24 QA](24_QA_TEST_ARCHITECTURE.md) | Behavior-based test layers and release evidence |
| [25 Performance](25_PERFORMANCE_SCALABILITY.md) | Workload/pool/job/frontend budgets and scale triggers |
| [26 Disaster recovery](26_DISASTER_RECOVERY.md) | Restore-to-new, cross-plane reconciliation and drills |
| [27 Decisions](27_ARCHITECTURE_DECISIONS.md) | Canonical unique IDs, statuses, legacy mapping and owners |
| [28 Risks](28_TECHNICAL_RISKS.md) | Blocking policies, security/operational risks and closure |
| [29 Traceability](29_REQUIREMENTS_TRACEABILITY.md) | All 24 source documents to design/tasks/evidence |
| [Quality report](QUALITY_REPORT.md) | Actual documentation validation, limitations and remaining gates |

## Interpretation

`<frontend-root>` is a documentation variable pending BID-ADR-005, not a folder to create. Observed empty root is `web/`; user describes `frontend/`. Proposed numerical examples are not commercial/security policy defaults. Diagrams describe main flows and do not grant authority for unspecified transitions. SQL/Go/JSON blocks are design examples only.

External-MVP schemas/contracts are more detailed than later marketplace/AI/billing extensions. Those future designs intentionally expose approval and schema-completion gates rather than inventing procurement/cryptographic policy. See quality report before claiming readiness for implementation.
