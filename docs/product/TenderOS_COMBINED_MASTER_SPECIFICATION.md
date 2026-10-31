# TenderOS — Combined Master Specification

This consolidated document mirrors the separate specification files in this package. The separate files remain the preferred sources for implementation and version control.



---

<!-- SOURCE FILE: 00_MASTER_INDEX.md -->

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


---

<!-- SOURCE FILE: 01_PRODUCT_DEFINITION.md -->

# TenderOS — Product Definition

## 1. Product statement

TenderOS is a multi-tenant, AI-assisted procurement and tender operating system connecting Procurement Teams and Tenderpreneurs through the complete tender lifecycle.

It supports both tenders published inside the platform and tenders discovered or imported from external sources. It is designed for Kenyan organizations first and must be configurable for additional countries, procurement regimes, currencies, languages, and business practices.

## 2. Problem being solved

Tendering is commonly fragmented across websites, email, shared drives, spreadsheets, messaging applications, PDFs, and disconnected procurement portals. This creates recurring problems:

- Procurement documents are incomplete, contradictory, or difficult to evaluate consistently.
- Suppliers discover opportunities late or cannot determine whether they qualify.
- Bid teams repeatedly recreate the same company content and compliance evidence.
- Mandatory documents, signatures, addenda, deadlines, and submission instructions are missed.
- Procurement teams struggle to maintain sealed bids, evaluation independence, traceability, and consistent reports.
- Management cannot reliably see pipeline value, procurement cycle time, disqualification reasons, win rates, or recurring risks.
- AI tools may generate convincing but unsupported content unless they are constrained by verified records and human approval.

## 3. Product vision

TenderOS should become the trusted operating layer through which organizations:

- Create fair, clear, controlled, and auditable tender processes.
- Find opportunities suited to their actual capabilities.
- Turn unstructured tender documents into structured work.
- Prepare compliant, evidence-backed submissions.
- Protect sealed bids and procurement confidentiality.
- Evaluate consistently while keeping decisions human-controlled.
- Build reusable institutional knowledge from every procurement and bid.

## 4. Three portals

### 4.1 Platform Admin Portal

Used by the SaaS operator to manage:

- Tenants and workspaces
- Plans, entitlements, usage, billing, and support
- Public source registry and integrations
- Feature flags and platform configuration
- Platform security, health, audit, abuse controls, and operations
- Country packs and global reference data

Platform administrators do not automatically receive access to private tenant documents, proposals, prices, evaluations, or submissions. Support access must be explicit, time-bound, least-privileged, and audited.

### 4.2 Procurement Team Portal

Used by buying organizations to:

- Plan procurement activities
- Create requisitions
- Draft tender documents and requirements
- Configure lots, eligibility, mandatory, technical, and financial criteria
- Run reviews and approvals
- Publish public, restricted, or invited tenders
- Manage clarifications, meetings, site visits, addenda, and deadline changes
- Receive sealed submissions
- Open bids through an authorized committee
- Conduct preliminary, technical, and financial evaluation
- Approve recommendations and issue award outcomes
- Hand over awarded procurements to contract management
- Analyze procurement performance

### 4.3 Tenderpreneur Portal

Used by suppliers, contractors, consultants, SMEs, companies, and bid teams to:

- Build a reusable business and compliance profile
- Search for tenders using Deep Search
- Save searches and watch buyers, categories, regions, and requirements
- Import any tender through file, URL, email, or manual entry
- Run AI tender analysis and Bid/No-Bid assessments
- Create compliance matrices and assign work
- Prepare technical and financial responses
- Reuse only approved company knowledge
- Run readiness and pre-submission audits
- Submit directly to platform-hosted tenders
- Receive guidance for external submissions
- Capture immutable submission evidence
- Track outcomes and learn from wins, losses, and disqualifications

## 5. Product operating models

### 5.1 Open Tender Model

A Tenderpreneur can receive value without the buyer being a TenderOS tenant.

```text
Discover or import external tender
→ verify extracted facts
→ Bid/No-Bid
→ create bid workspace
→ satisfy compliance matrix
→ prepare and approve response
→ audit final package
→ submit through external channel
→ store evidence
→ record outcome
```

### 5.2 Closed Platform Tender Model

Both buyer and bidder transact through TenderOS.

```text
Procurement plan/requisition
→ author and approve tender
→ publish
→ Tenderpreneurs discover and respond
→ sealed submission
→ controlled opening
→ preliminary/technical/financial evaluation
→ approval and award
→ outcome notification and contract handover
```

## 6. Primary users

### Platform-side

- Platform Super Administrator
- Platform Operations Administrator
- Platform Support Officer
- Billing Administrator
- Security/Audit Administrator

### Procurement tenant

- Procurement Organization Administrator
- Requester
- Department Head/Budget Owner
- Procurement Officer
- Tender Author
- Reviewer
- Approver
- Opening Committee Member
- Preliminary Evaluator
- Technical Evaluator
- Financial Evaluator
- Evaluation Chair/Secretary
- Legal/Compliance Reviewer
- Finance Reviewer
- Auditor/Observer
- Executive/Read-only User

### Tenderpreneur tenant

- Tenderpreneur Organization Administrator
- Bid/Tender Manager
- Business Development User
- Technical Contributor
- Finance Contributor
- Legal/Compliance Contributor
- Document Controller
- Reviewer
- Approver
- Executive
- Read-only User

## 7. Core value propositions

### Procurement organizations

- Higher-quality tender documents
- Controlled reviews and approvals
- Secure bid receipt and opening
- Consistent, evidence-linked evaluation
- Reduced cycle time and duplicated administration
- Stronger auditability and management reporting
- AI assistance without surrendering human accountability

### Tenderpreneurs

- One workspace for tenders from any source
- Better opportunity matching
- Faster understanding of long tender documents
- Reusable compliance vault and company knowledge
- Lower disqualification risk
- Structured collaboration and approvals
- Submission evidence and performance intelligence

### Platform operator

- Two-sided SaaS revenue model
- Subscription, usage, enterprise, and integration revenue
- Defensible data and workflow network effects
- Country-specific expansion through configurable packs

## 8. Product principles

1. **Operations before screens:** Build complete user operations, not page shells.
2. **Evidence before claims:** Every compliance, score, and AI conclusion must identify its supporting source.
3. **Human accountability:** AI assists; authorized users decide and approve.
4. **Tenant isolation by design:** Each tenant has a private database and private storage boundary.
5. **No premature bid access:** Sealed submissions remain inaccessible until authorized opening conditions are met.
6. **Backend is the business authority:** The frontend never invents permissions, statuses, calculations, or workflow rules.
7. **Explicit state machines:** Tender, bid, submission, evaluation, approval, billing, and AI-job transitions are controlled.
8. **Interoperability:** External tenders remain first-class, not secondary records.
9. **Internationalization:** Country rules and terminology are configurable rather than embedded in core logic.
10. **Traceability:** Requirements, code, tests, decisions, and release evidence are linked.

## 9. Product boundaries

### In scope

- Multi-tenant SaaS and subscription operations
- Procurement planning through award and contract handover
- Tender discovery and deep search
- External tender import and management
- Bid preparation and collaboration
- Direct platform submission and external submission assistance
- AI-assisted authoring, extraction, analysis, comparison, and auditing
- Documents, workflows, approvals, notifications, search, reporting, and audit
- Public tender catalogue
- APIs and supported integrations

### Not in initial scope

- Full contract lifecycle management after handover
- Purchase orders, goods receipt, inventory, or accounts payable
- Autonomous award or disqualification
- Automatic external portal submission without an official integration and explicit authorization
- Legal certification that a procurement complies with every law
- Unrestricted web scraping that violates source terms, access controls, or law
- Marketplace escrow or supplier financing

## 10. Success measures

Initial product success should be measured using:

- Time from requisition approval to tender publication
- Percentage of tenders passing pre-publication quality review without rework
- Average bid preparation time
- Percentage of platform bids submitted before the internal deadline buffer
- Mandatory-requirement completion rate before submission
- Reduction in preventable disqualifications
- Procurement evaluation cycle time
- Percentage of AI outputs with valid source references
- Tenant activation rate and time to first completed operation
- Monthly active Procurement Teams and Tenderpreneurs
- Tender import, search-to-workspace, submission, and award conversion rates
- Subscription retention and expansion

## 11. Product positioning

**TenderOS is not merely an AI writing tool or tender-listing website.**

It is:

> An AI-assisted procurement and tender operating system that enables buyers to run controlled tender processes and enables Tenderpreneurs to discover, prepare, submit, and learn from bids from any source.


---

<!-- SOURCE FILE: 02_BRD.md -->

# TenderOS — Business Requirements Document (BRD)

## 1. Executive summary

TenderOS will provide a two-sided SaaS platform for Procurement Teams and Tenderpreneurs, governed through a Platform Admin Portal. It will support internal platform tenders and externally sourced tenders. Its commercial and operational advantage is the combination of structured procurement workflows, tender intelligence, compliance management, sealed submission controls, evaluation support, and source-grounded AI.

The first market is Kenya. Public procurement features must be reviewed against applicable Kenyan procurement law, regulations, organizational policies, and record-retention obligations before production use. The core system must remain configurable for private-sector, NGO, donor, and international procurement procedures.

## 2. Business objectives

| ID | Objective | Target outcome |
|---|---|---|
| BO-01 | Create a viable two-sided SaaS product | Procurement and Tenderpreneur tenants can subscribe and operate independently or transact together |
| BO-02 | Reduce preventable tender disqualification | Mandatory gaps, expired documents, missing forms, and submission conflicts are detected before submission |
| BO-03 | Improve procurement quality | Tender requirements and evaluation criteria are structured, reviewable, measurable, and approved before publication |
| BO-04 | Protect integrity and confidentiality | Bid content remains sealed until authorized opening and every sensitive action is auditable |
| BO-05 | Reduce operational cycle time | Repeatable workflows, reusable evidence, automation, and AI reduce manual processing |
| BO-06 | Create institutional knowledge | Past procurements, bids, evidence, evaluations, outcomes, and lessons remain searchable and reusable according to permissions |
| BO-07 | Enable external-tender workflows | Tenderpreneurs can use the product even when the procuring entity is not a platform tenant |
| BO-08 | Create trustworthy AI differentiation | AI outputs are grounded, source-linked, permission-aware, measured, and human-approved |
| BO-09 | Support regional expansion | Currency, language, timezone, tax, terminology, source connectors, and compliance rules are configurable |
| BO-10 | Operate securely at scale | Tenant isolation, database lifecycle, storage, observability, backup, and support processes are production-grade |

## 3. Stakeholders

| Stakeholder | Interest |
|---|---|
| Platform owner | Revenue, reliability, growth, security, support, and product governance |
| Procurement leadership | Control, transparency, cycle time, policy compliance, and value for money |
| Procurement officers | Efficient authoring, publishing, clarifications, evaluation, and reporting |
| Evaluation committees | Consistent criteria, evidence access, independent scoring, and auditability |
| Tenderpreneur leadership | Pipeline visibility, win rate, readiness, cost control, and reusable knowledge |
| Bid teams | Clear assignments, collaboration, compliant content, deadlines, and approvals |
| Finance and legal teams | Controlled access to sensitive pricing, contractual risks, and compliance evidence |
| Auditors and regulators | Reliable records, chronology, access controls, and immutable evidence |
| Data subjects | Lawful, fair, secure, and transparent processing of personal data |
| Integration partners | Stable APIs, secure credentials, webhooks, and supportable contracts |

## 4. Business capabilities

### BC-01 SaaS tenant lifecycle

Register, verify, create workspace, select plan, pay or trial, invite users, configure roles, enforce limits, suspend/reactivate, export, and close tenant.

### BC-02 Platform governance

Manage tenants, plans, entitlements, usage, billing, source connectors, feature flags, support access, announcements, security, and platform audit.

### BC-03 Procurement lifecycle

Plan, requisition, approve, author, review, publish, clarify, amend, receive, open, evaluate, recommend, approve, award, notify, and hand over.

### BC-04 Tenderpreneur lifecycle

Discover/import, qualify, Bid/No-Bid, plan response, assign work, satisfy compliance, prepare proposal and pricing, review, approve, submit, evidence, and analyze outcome.

### BC-05 Tender intelligence and Deep Search

Search supported sources, normalize results, deduplicate, rank by fit, monitor changes, retain provenance, and convert opportunities to workspaces.

### BC-06 Document and knowledge governance

Store, version, classify, validate, expire, approve, search, cite, reuse, and audit documents and knowledge.

### BC-07 Secure submission exchange

Validate submission, lock final version, encrypt and seal, acknowledge receipt, enforce closing conditions, authorize opening, and retain tamper-evident evidence.

### BC-08 AI assistance

Extract, classify, draft, review, compare, summarize, identify gaps, estimate fit, and produce source-linked reports under human control.

### BC-09 Analytics and reporting

Measure pipeline, cycle time, participation, evaluation, win/loss, disqualification, usage, cost, revenue, and AI quality.

## 5. Business requirements

### SaaS and tenancy

- **BR-001:** The platform shall support Procurement and Tenderpreneur tenant types within the same SaaS control plane.
- **BR-002:** Every tenant shall receive a private database and private storage namespace.
- **BR-003:** A user may belong to more than one tenant and must explicitly select an active workspace.
- **BR-004:** Subscription plans shall control features, users, storage, active records, search, AI usage, and integration quotas.
- **BR-005:** Tenant suspension shall prevent business operations without destroying records required for billing, audit, recovery, or lawful retention.
- **BR-006:** Platform support access to tenant data shall require explicit authorization, scope, reason, expiry, and audit.

### Procurement

- **BR-010:** Procurement teams shall create procurement plans and requisitions before tender authoring when the configured process requires them.
- **BR-011:** A tender shall define its procurement method, scope, lots, dates, requirements, criteria, scoring, submission rules, and approvals.
- **BR-012:** A tender shall not be published until all configured reviews and approvals are complete.
- **BR-013:** Published amendments and addenda shall be versioned and communicated to affected participants.
- **BR-014:** The system shall prevent procurement users from reading sealed submission content before authorized opening.
- **BR-015:** Evaluators shall score only assigned criteria and shall reference evidence from the submission.
- **BR-016:** Final evaluation consolidation, recommendation, approval, and award shall remain controlled by authorized humans.
- **BR-017:** The system shall produce an auditable procurement record from initiation through contract handover.

### Tenderpreneur

- **BR-020:** Tenderpreneurs shall import tenders from files, URLs, email attachments, copied text, and manual entry.
- **BR-021:** Imported tenders shall be marked as external and retain source and verification status.
- **BR-022:** Tenderpreneurs shall maintain reusable organization, compliance, staff, experience, certification, and reference evidence.
- **BR-023:** Each requirement shall be assigned a status, owner, due date, evidence, verification state, and source reference.
- **BR-024:** A mandatory unresolved requirement shall block readiness unless an authorized, reasoned override is permitted by policy.
- **BR-025:** Tenderpreneurs shall prepare technical and financial content with separate permissions and approvals.
- **BR-026:** The exact submitted version and evidence shall be retained for platform and external submissions.

### Deep Search

- **BR-030:** Deep Search shall search configured lawful sources and clearly identify source provenance and freshness.
- **BR-031:** Search shall combine filters, full-text retrieval, semantic matching, organization fit, and configurable ranking.
- **BR-032:** Duplicate notices shall be consolidated while preserving every discovered source.
- **BR-033:** Tenderpreneurs shall save searches, watch entities/categories, receive change alerts, and create workspaces from results.
- **BR-034:** The platform shall not represent unverified scraped or inferred information as authoritative tender fact.

### AI

- **BR-040:** AI outputs shall distinguish extracted facts, external research, organizational facts, and model interpretation.
- **BR-041:** AI shall cite source document locations or records for material findings.
- **BR-042:** AI shall respect tenant, record, and field-level permissions.
- **BR-043:** AI shall not automatically publish, submit, open, disqualify, score conclusively, approve, or award.
- **BR-044:** AI usage, latency, cost, model, prompt/version, output schema, and user decision shall be measurable.
- **BR-045:** Procurement AI shall support tender drafting, requirement review, criteria design, evaluation evidence location, price analysis, and report drafting.
- **BR-046:** Tenderpreneur AI shall support extraction, fit assessment, compliance matching, response drafting, evidence matching, and pre-submission audit.

### Governance and reporting

- **BR-050:** Every material state change shall have actor, time, tenant, record, previous state, new state, reason, and correlation identifier.
- **BR-051:** Sensitive pricing and evaluation content shall be restricted independently of general tender access.
- **BR-052:** Business reports shall be tenant-scoped and permission-filtered.
- **BR-053:** Records shall support configurable retention, legal hold, export, and deletion workflows.
- **BR-054:** The platform shall support country packs for terminology, required fields, currency, timezone, language, and configurable policy rules.

## 6. Commercial model

### Tenderpreneur plans

Possible entitlement dimensions:

- Users and teams
- Active and archived bid workspaces
- Storage
- Deep Search frequency and source coverage
- Saved searches and alerts
- AI pages/tokens/jobs
- Proposal generation and submission audits
- External submission evidence
- Integrations and API access
- Support level

### Procurement plans

Possible entitlement dimensions:

- Procurement users
- Active procurements and published tenders
- Supplier invitations and submissions
- Storage and retention
- Evaluation committee size
- AI drafting/evaluation usage
- Workflow customization
- SSO, APIs, audit exports, dedicated tenant infrastructure, and SLA

### Revenue channels

- Monthly and annual subscription
- Usage-based AI and search capacity
- Enterprise onboarding, migration, training, and support
- Country/source connector packs
- Custom integrations
- Dedicated hosting or database cluster
- Verified supplier/compliance services, subject to policy and legal review

## 7. Business KPIs

- Activation: tenant reaches first complete operation
- Search-to-workspace conversion
- Imported tender-to-Bid decision conversion
- Bid completion and submission rate
- Preventable disqualification rate
- Procurement publication and evaluation cycle time
- Average number of bidders per published tender
- Renewal, retention, expansion, and revenue per tenant
- AI grounded-answer rate and human acceptance/edit rate
- Support incident rate and mean time to resolution
- Platform uptime and backup restore success

## 8. Constraints and assumptions

- The platform cannot guarantee external source availability or external portal submission.
- Public and private procurement rules vary; workflows must be configurable.
- Legal and procurement professionals must validate country-specific rules before production adoption.
- Database-per-tenant increases isolation but requires disciplined provisioning, pooling, migration, monitoring, backup, and cost controls.
- AI quality depends on source quality, extraction accuracy, language, document scans, and user verification.
- A downloaded frontend template will be used as a design input after license, quality, accessibility, and component review.

## 9. Principal business risks

| Risk | Business response |
|---|---|
| Cold-start marketplace | External tender import and Deep Search give Tenderpreneurs independent value |
| Unreliable source crawling | Prefer official APIs/feeds, preserve provenance, monitor freshness, and permit manual import |
| AI hallucination or bias | Structured outputs, citations, confidence, evaluation datasets, human approval, and audit |
| Cross-tenant exposure | Private database per tenant, explicit context, service authorization, and isolation tests |
| Premature bid access | Cryptographic sealing, independent key release, opening quorum, and immutable logs |
| Procurement law variation | Country packs, configurable workflows, legal review, and no blanket compliance claims |
| High AI/search cost | Quotas, budgets, caching, model routing, asynchronous jobs, and usage analytics |
| Complex implementation | Modular-monolith baseline, dependency-ordered delivery, feature flags, and staged releases |

## 10. Business acceptance

The business MVP is accepted when:

1. A Procurement tenant can author, approve, publish, clarify, receive a sealed bid, open it, evaluate it, approve a recommendation, and issue an outcome.
2. A Tenderpreneur tenant can discover or import an external tender, analyze it, prepare a compliant bid, approve it, generate a package, record external submission evidence, and track the outcome.
3. A Tenderpreneur can respond directly to a platform-hosted tender and receive a verifiable acknowledgement.
4. Tenant isolation, authorization, sealed-bid controls, audit, backup, and restore tests pass.
5. AI outputs are source-grounded, reviewable, and never treated as final procurement decisions.


---

<!-- SOURCE FILE: 03_PRD.md -->

# TenderOS — Product Requirements Document (PRD)

## 1. Product goal

Deliver a multi-tenant SaaS platform with three portals—Platform Admin, Procurement Team, and Tenderpreneur—that supports the complete procurement and bid lifecycle for platform-hosted and externally sourced tenders.

## 2. Product outcomes

- Procurement teams can complete controlled tender operations without spreadsheets or disconnected email chains.
- Tenderpreneurs can manage any tender from discovery/import through submission evidence and outcome.
- AI improves speed and quality while preserving source traceability and human authority.
- The product remains useful before marketplace network effects emerge.
- Every accepted workflow is secure, auditable, and complete from database through user interface.

## 3. Portal requirements

## 3.1 Platform Admin Portal

### PR-PA-001 Tenant administration

The portal shall allow authorized administrators to:

- Search and view tenants by type, plan, status, region, risk, and usage
- Create, verify, suspend, reactivate, and close tenants
- View provisioning and migration status
- Manage enterprise database placement
- Initiate controlled support access
- Export permitted tenant metadata and platform audit records

### PR-PA-002 Plans, billing, and entitlements

- Create and version plans
- Configure feature entitlements and quotas
- Manage trials, grace periods, invoices, payments, credits, and cancellations
- View usage by tenant, feature, model, search source, storage, and API
- Enforce limits through backend policy, not frontend visibility alone

### PR-PA-003 Platform operations

- View service health, queue status, search-source health, AI provider health, migration status, backup status, and incidents
- Manage feature flags and maintenance windows
- Publish announcements
- Configure system email templates, country packs, and reference data

### PR-PA-004 Source and integration registry

- Register official APIs, feeds, websites, and connectors
- Record source owner, region, terms, access method, schedule, rate limits, parser version, confidence, and status
- Pause or disable non-compliant or unreliable sources

### PR-PA-005 Security and support governance

- Review platform-level security events
- Manage privileged roles
- Approve time-bound support access
- View every privileged access session and action
- Configure retention and legal-hold defaults

## 3.2 Procurement Team Portal

### PR-PT-001 Procurement planning

Users shall create annual/periodic plans and procurement requests containing:

- Department/requester
- Business need
- Category and procurement method
- Estimated value and currency
- Budget source and confirmation
- Required dates
- Procurement owner
- Approvals and attachments

### PR-PT-002 Tender authoring

Users shall define:

- Tender identity, scope, description, lots, locations, and schedule
- Eligibility and mandatory requirements
- Technical criteria, scoring, thresholds, and evidence
- Financial response structure and arithmetic rules
- Bid security, validity, site visits, pre-bid meetings, and clarification windows
- Submission channels, file rules, declarations, forms, and terms
- Opening and evaluation governance

### PR-PT-003 AI authoring and quality review

AI shall optionally:

- Draft sections from approved requisitions, templates, and organizational standards
- Identify ambiguity, contradiction, duplication, restrictive wording, non-measurable criteria, mismatched dates, and missing acceptance criteria
- Recommend criteria and weightings for human review
- Produce a readiness report with citations

### PR-PT-004 Review and approval

- Configurable serial and parallel reviews
- Version-specific approval
- Comments, changes requested, rejection, resubmission, delegation, and expiry
- Publication blocked until required approvals are complete

### PR-PT-005 Publication and marketplace

- Public, restricted, and invitation-only publication
- Public tender page with authoritative version and documents
- Supplier registration or invitation controls
- Publication schedule, visibility, categories, and notifications
- Withdrawal, cancellation, extension, and re-publication rules

### PR-PT-006 Clarifications and addenda

- Receive questions without exposing bidder-private information
- Group duplicates and prepare responses
- Review and approve responses where required
- Publish clarifications and addenda
- Identify changed requirements and notify affected Tenderpreneurs
- Require acknowledgement of material addenda when configured

### PR-PT-007 Submission receipt and sealing

- Receive structured answers, forms, and documents
- Validate deadline and required fields
- Issue immutable acknowledgement and reference
- Encrypt and seal content
- Prevent procurement-side content access before authorized opening
- Support withdrawal and replacement only according to tender rules

### PR-PT-008 Bid opening

- Configure opening committee and quorum
- Capture declarations and attendance
- Release bid access only when time, status, quorum, and permissions pass
- Record opening events, bidder list, declared prices where applicable, and minutes

### PR-PT-009 Evaluation

- Preliminary compliance review
- Technical and financial evaluation
- Criteria assignment and evaluator independence
- Evidence-linked scores and comments
- Clarification requests according to policy
- Arithmetic correction workflow
- Consolidation, moderation, ranking, and exception management
- Conflict-of-interest declaration and recusal

### PR-PT-010 AI evaluation assistance

AI may:

- Locate evidence and summarize responses
- Compare bids against published criteria
- Flag unsupported claims, contradictions, anomalies, and pricing outliers
- Draft evaluation narratives and reports

AI must not set the final evaluator score or award decision.

### PR-PT-011 Recommendation, approval, and award

- Create evaluation report and recommendation
- Run required reviews/approvals
- Record decision and reasons
- Notify successful and unsuccessful bidders
- Publish award information where configured
- Hand over approved record to contract management or export

### PR-PT-012 Procurement analytics

- Plan execution, cycle time, tender status, participation, evaluation time, awards, failures, savings, and risks
- Role-filtered dashboards and downloadable reports

## 3.3 Tenderpreneur Portal

### PR-TP-001 Organization onboarding and profile

Capture:

- Legal/business identity
- Contacts, addresses, owners/directors where required
- Sectors, products, services, regions, capacities, and preferred tender values
- Certifications, licences, memberships, financial profile, staff, experience, projects, and references
- Notification and search preferences

### PR-TP-002 Compliance and knowledge vault

- Document upload, classification, owner, issue/expiry, version, status, verification, and access
- Structured records for staff, certifications, projects, references, manufacturer authorizations, policies, and standard responses
- Expiry and missing-data alerts
- Approval before content becomes reusable by AI

### PR-TP-003 Deep Search

Tenderpreneurs shall:

- Search supported sources by natural language and filters
- Receive fit, qualification, risk, freshness, and source-confidence signals
- Save searches and watch buyers/categories/keywords
- Receive new tender, addendum, deadline, cancellation, and source-change alerts
- Consolidate duplicate notices
- Open authoritative source and create a workspace

### PR-TP-004 External tender import

Support:

- PDF, DOCX, spreadsheet, image/scan, ZIP, URL, email attachment, copied text, and manual entry
- Malware scan, checksum, extraction, OCR where needed, classification, and provenance
- User verification and correction of extracted fields
- External-source status and freshness tracking

### PR-TP-005 AI tender analysis

Extract and source-reference:

- Identity, dates, contacts, lots, eligibility, mandatory requirements, technical/financial requirements, criteria, scoring, forms, securities, validity, meetings, clarifications, submission instructions, and risk clauses
- Confidence and verification state for each extracted item

### PR-TP-006 Bid/No-Bid

- Configurable weighted assessment
- Eligibility, strategic fit, experience, resources, compliance readiness, value, margin, risk, competition, and preparation effort
- Recommendation, risk summary, missing capabilities, reviewers, and decision history
- Human decision required

### PR-TP-007 Tender workspace

Provide:

- Overview and source
- Requirements/compliance matrix
- Team, tasks, dependencies, deadlines, comments, and activity
- Documents and evidence
- Technical proposal and structured responses
- Financial response with restricted access
- Clarifications/addenda
- Reviews, approvals, readiness, submission, evidence, and outcome

### PR-TP-008 Compliance matrix

Each requirement shall include:

- Exact requirement and source location
- Category, mandatory flag, scoring impact, and due date
- Owner and collaborators
- Status: Not Started, In Progress, Satisfied, Partial, Missing, Not Applicable, Clarification Required, Waived
- Evidence and version
- Reviewer and verification outcome
- Risks, notes, and dependencies

### PR-TP-009 Proposal and response management

- Structured sections and free-form approved content
- Reusable answer library with provenance
- Versioning, comments, review, approval, and comparison
- Export to required document formats and package structure
- Financial response separated by permission and submission-envelope rules

### PR-TP-010 AI proposal assistant

- Draft from verified tenant knowledge and tender requirements
- Suggest evidence and reusable content
- Identify unanswered or unsupported statements
- Review clarity, consistency, compliance, and page/word limits
- Never invent company experience, certifications, staff qualifications, prices, or approvals

### PR-TP-011 Submission readiness and audit

- Deterministic readiness score based on configured gates
- AI-assisted final comparison against the authoritative tender version
- Blocking issues, risks, warnings, and recommendations
- Checks for forms, attachments, versions, expiry, signatures, file names, sizes, folder/envelope rules, dates, and approvals
- Authorized override with reason and audit where policy allows

### PR-TP-012 Platform submission

- Submit responses against procurement-defined requirements
- Final review, declarations, authorization, idempotent submission, locking, sealing, acknowledgement, and replacement/withdrawal rules

### PR-TP-013 External submission assistance

- Identify external channel and instructions
- Map final files to portal/email/physical requirements
- Generate final package and checklist
- Capture internal approval
- Record proof: time, user, channel, reference, receipt, screenshot, email, and exact file versions
- Never mark submitted without user-confirmed evidence

### PR-TP-014 Outcome and learning

- Record won, lost, disqualified, cancelled, withdrawn, pending, or unknown
- Capture reasons, feedback, competitor/public award data, contract value, and lessons
- Update analytics and reusable knowledge only after review

## 4. Shared product requirements

### PR-SH-001 Identity and context

- Registration, verification, login, logout, recovery, MFA, session management, SSO options
- `/me` bootstrap containing identity, memberships, active context, capabilities, preferences, and session expiry
- Explicit context selection for users with multiple memberships

### PR-SH-002 Authorization

- Role-based permissions plus attribute/resource policies
- Portal, tenant, department, team, tender, field, and workflow-step restrictions
- Server-side enforcement on every operation

### PR-SH-003 Documents

- Secure upload/download, previews, versioning, metadata, checksum, malware status, classification, expiry, retention, legal hold, and audit
- Signed URLs with short expiry and tenant scope

### PR-SH-004 Workflow and approvals

- Configurable templates
- Sequential/parallel steps
- Delegation, escalation, due dates, reminders, rejection, change request, rework, cancellation, and audit

### PR-SH-005 Tasks and collaboration

- Task owner, collaborators, priority, due date, dependency, status, comments, attachments, and reminders
- No permission leakage through mentions, notifications, or search

### PR-SH-006 Notifications

- In-app and email initially
- SMS/WhatsApp adapters later
- User preferences, quiet hours, digest, retries, delivery status, and template version

### PR-SH-007 Search

- Tenant-private search, public tender search, and Deep Search are distinct security domains
- Results filtered before ranking/display
- Search access shall not reveal existence of unauthorized private records

### PR-SH-008 Audit and evidence

- Append-only audit events for material actions
- Evidence package for publication, submission, opening, evaluation, approvals, awards, support access, and AI outputs

### PR-SH-009 Billing and usage

- Plans, entitlements, quotas, metering, invoices, receipts, payment status, grace periods, and suspension
- Usage displayed to authorized tenant administrators

### PR-SH-010 Internationalization

- Locale, timezone, currency, number/date formats, language, terminology, country, procurement method, and compliance-pack configuration
- Kenya English, Africa/Nairobi, and KES as initial defaults, not hardcoded universal values

## 5. Experience requirements

- **UX-001:** Every page shall represent a real user operation and expose only relevant business information.
- **UX-002:** Forms shall use correct controls—selects, searchable comboboxes, file uploaders, date/time pickers, structured editors, and linked records—not raw database identifiers.
- **UX-003:** View/detail pages shall provide contextual navigation and a reliable back path.
- **UX-004:** Lists shall support appropriate search, filters, sort, pagination, saved views, and bulk actions.
- **UX-005:** The UI shall implement loading, empty, validation, conflict, permission, deadline, network, and server-error states.
- **UX-006:** Destructive, publication, submission, opening, disqualification, and award actions shall use explicit confirmation and show consequences.
- **UX-007:** Accessibility target is WCAG 2.2 AA for production workflows.
- **UX-008:** The downloaded template may supply visual assets and components, but product workflows, data contracts, and accessibility requirements take precedence.

## 6. Non-functional product targets

- **NFR-001 Availability:** Initial production target 99.9% monthly availability excluding published maintenance.
- **NFR-002 API latency:** p95 non-AI reads under 300 ms and writes under 500 ms at agreed baseline load, excluding file transfer and third-party dependencies.
- **NFR-003 Search latency:** p95 interactive search under 2 seconds for indexed data at baseline scale.
- **NFR-004 AI jobs:** Long-running extraction/research shall be asynchronous, resumable, observable, and cancellable where safe.
- **NFR-005 Scalability:** The platform shall scale application workers horizontally and distribute tenant databases across clusters.
- **NFR-006 Recoverability:** Restore procedures shall be tested; target RPO/RTO shall be defined per plan and environment.
- **NFR-007 Privacy:** Personal and confidential data shall be minimized, access-controlled, retained purposefully, and exportable/deletable where legally permitted.
- **NFR-008 Portability:** Business logic shall not depend directly on a single AI, storage, email, payment, or message-broker provider.

## 7. MVP scope

### MVP-A — Independent Tenderpreneur value

- SaaS onboarding, tenant provisioning, identity, roles, billing foundation
- Tenderpreneur profile and compliance vault
- External tender import
- Structured tender workspace
- Deterministic compliance matrix
- Proposal collaboration
- Submission readiness and external submission evidence
- Initial AI extraction and grounded analysis
- Basic Deep Search for approved sources

### MVP-B — Procurement and closed-loop tendering

- Procurement planning/requisition
- Tender authoring, review, approval, and publication
- Public catalogue and clarifications
- Direct structured submissions and sealed bid vault
- Opening, preliminary, technical, and financial evaluation
- Recommendation, approval, award, and notifications

### MVP-C — AI and intelligence hardening

- Procurement AI drafting and quality review
- Tenderpreneur AI proposal/compliance assistant
- AI bid comparison and report drafting
- Advanced search, source monitoring, deduplication, alerts, analytics, and evaluation datasets

## 8. Product acceptance criteria

The MVP release shall not be accepted unless:

- The three portals have separate navigation, capabilities, and authorization boundaries.
- Tenant database provisioning, migration, backup, restore, suspension, and deletion workflows are tested.
- A Tenderpreneur completes an external tender journey end to end.
- A Procurement Team and Tenderpreneur complete a platform tender journey end to end.
- A procurement user cannot access sealed bid content before valid opening.
- Unauthorized cross-tenant access tests fail closed at API, database router, object storage, search, and AI layers.
- AI material outputs include citations and verification state.
- The frontend uses real backend contracts and contains no accepted workflow dependent on mock data.


---

<!-- SOURCE FILE: 04_SRD.md -->

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


---

<!-- SOURCE FILE: 05_DOMAIN_MODEL_AND_BUSINESS_RULES.md -->

# TenderOS — Domain Model and Business Rules

## 1. Domain design approach

TenderOS uses Domain-Driven Design to separate business concepts and prevent one large generic CRUD model. The initial implementation is a modular monolith in Go. Each bounded context owns its domain model, application services, persistence ports, API adapters, events, and tests.

Cross-context communication occurs through explicit application interfaces and versioned events. Modules may share technical libraries, but they must not share mutable domain entities or bypass each other's application layer.

## 2. Bounded contexts

| Context | Responsibility |
|---|---|
| Identity & Access | Identities, sessions, memberships, roles, permissions, step-up, and active context |
| Tenant Lifecycle | Tenant provisioning, database assignment, schema state, suspension, and decommissioning |
| Subscription & Entitlements | Plans, subscriptions, quotas, usage, invoices, and feature enforcement |
| Organization Profile | Tenant identity, departments, teams, sectors, capabilities, and preferences |
| Document & Knowledge | Files, versions, metadata, verification, expiry, knowledge records, and access |
| Workflow & Approval | Workflow templates, instances, steps, decisions, escalation, and delegation |
| Procurement Planning | Plans, requisitions, budget confirmation, method selection, and approvals |
| Tender Authoring | Tender draft, lots, requirements, criteria, forms, schedule, and review |
| Publication & Clarification | Publication versions, catalogue projection, invitations, questions, responses, and addenda |
| Opportunity & Discovery | Public catalogue, external source records, search, matching, watchlists, and alerts |
| Bid Workspace | Imported tender, Bid/No-Bid, requirements, tasks, responses, pricing, and approvals |
| Submission Exchange | Final response version, envelopes, sealing, acknowledgement, withdrawal, replacement, and opening release |
| Evaluation | Preliminary, technical, financial, panel consolidation, moderation, and reports |
| Award & Handover | Recommendation, approval, outcome, notice, publication, and contract handover |
| AI Intelligence | Ingestion, extraction, retrieval, generation, comparison, evaluation, and model governance |
| Notification | Templates, preferences, dispatch, retries, receipts, and digests |
| Audit & Evidence | Append-only audit events, evidence packages, hashes, and exports |
| Reporting & Analytics | Read models, metrics, dashboards, and permitted exports |

## 3. Core aggregates

### 3.1 Tenant aggregate

**Root:** `Tenant`

Key properties:

- Tenant ID
- Tenant type: `PROCUREMENT`, `TENDERPRENEUR`, or approved hybrid type
- Legal/display name
- Country and default locale/timezone/currency
- Lifecycle status
- Database locator reference
- Storage namespace reference
- Subscription reference
- Provisioning/schema version

Invariants:

- **RULE-TEN-001:** A tenant cannot become `ACTIVE` until database, roles, storage, initial migration, seed, and administrator membership are complete.
- **RULE-TEN-002:** A suspended tenant cannot perform business writes except approved remediation, export, payment, or closure operations.
- **RULE-TEN-003:** Database credentials are never exposed to clients or stored in tenant business records.

### 3.2 Membership aggregate

**Root:** `Membership`

- Identity
- Tenant
- Status
- Roles
- Team/department assignments
- Effective dates
- Invitation and acceptance details

Invariants:

- **RULE-IAM-001:** A request has no tenant authority until a valid membership and active context are resolved.
- **RULE-IAM-002:** Removing the last required tenant administrator requires transfer or governed override.
- **RULE-IAM-003:** Permission grants are additive only within policy; explicit denies and scope restrictions take precedence.

### 3.3 Procurement Request aggregate

**Root:** `ProcurementRequest`

- Requester/department
- Need and scope summary
- Category and method
- Budget/currency
- target dates
- approvals
- status

State machine:

```text
DRAFT → SUBMITTED → UNDER_REVIEW → APPROVED → CONVERTED_TO_TENDER
  └──────────────→ CHANGES_REQUESTED → SUBMITTED
  └──────────────→ REJECTED
  └──────────────→ CANCELLED
```

Rules:

- **RULE-PREQ-001:** Conversion requires required approvals and budget confirmation when configured.
- **RULE-PREQ-002:** Material changes after approval create a new version and may require reapproval.

### 3.4 Tender aggregate

**Root:** `Tender`

Entities/value objects:

- Tender identity/reference
- Lot
- Schedule
- Requirement
- Evaluation criterion
- Score scale/weight
- Submission schema
- Tender form/template
- Security/validity terms
- Review and approval references

State machine:

```text
DRAFT
→ INTERNAL_REVIEW
→ CHANGES_REQUESTED ↺
→ APPROVED
→ SCHEDULED
→ OPEN
→ CLOSED
→ OPENING
→ EVALUATION
→ RECOMMENDATION
→ AWARD_APPROVAL
→ AWARDED
→ HANDED_OVER

Alternative terminal states: CANCELLED, WITHDRAWN, FAILED, NO_AWARD
```

Invariants:

- **RULE-TEN-010:** Reference numbers must be unique within a procurement tenant and configured period.
- **RULE-TEN-011:** Publication requires at least one lot or scope, a closing time, submission rules, and all required approvals.
- **RULE-TEN-012:** Criteria weights must satisfy the configured total and thresholds before publication.
- **RULE-TEN-013:** A published version is immutable. Changes produce an addendum and new authoritative version.
- **RULE-TEN-014:** Closing time is stored in UTC plus declared source timezone.
- **RULE-TEN-015:** Evaluation may use only criteria from the authoritative version acknowledged by bidders.
- **RULE-TEN-016:** Cancellation, extension, and material amendment require reason, authority, notification, and audit.

### 3.5 Publication aggregate

**Root:** `TenderPublication`

- Tender/version
- Visibility
- Audience/invitations
- Publication schedule
- Public documents and manifest
- Category/location/search metadata
- Status

Rules:

- **RULE-PUB-001:** Only approved tender versions can be published.
- **RULE-PUB-002:** A public projection contains no internal notes, budgets, private approvals, or evaluator assignments.
- **RULE-PUB-003:** The authoritative procurement tenant version and exchange projection hashes must match.

### 3.6 Clarification aggregate

**Root:** `ClarificationCase`

- Tender/publication version
- Questioner identity or permitted anonymous/public representation
- Question
- Internal draft response
- Review/approval
- Published answer
- Related addendum

Rules:

- **RULE-CLR-001:** A bidder's private identity/details must not be disclosed in a public clarification unless policy permits and the bidder is informed.
- **RULE-CLR-002:** A response that changes requirements, dates, criteria, or submission rules must create or reference an addendum.

### 3.7 Tenderpreneur Opportunity aggregate

**Root:** `Opportunity`

- Source type: platform or external
- Source/provenance
- Extracted and verified facts
- Follow/watch status
- Fit assessment
- Workspace conversion reference

Rules:

- **RULE-OPP-001:** External extracted fields are not authoritative until user-verified or confirmed from an authoritative source.
- **RULE-OPP-002:** Duplicate source records may map to one canonical opportunity while preserving each source.

### 3.8 Bid Workspace aggregate

**Root:** `BidWorkspace`

Entities/value objects:

- Tender snapshot/version
- Bid/No-Bid assessment
- Requirement response
- Task
- Content section
- Financial response reference
- Approval/readiness state
- Submission plan

State machine:

```text
IDENTIFIED → ASSESSING → NO_BID
                    └→ BID_APPROVED → PREPARING → INTERNAL_REVIEW
                       → CHANGES_REQUESTED ↺
                       → APPROVED_FOR_SUBMISSION → READY → SUBMITTED
                       → OUTCOME_PENDING → WON | LOST | DISQUALIFIED | CANCELLED
```

Rules:

- **RULE-BID-001:** One active workspace per tenant and authoritative tender/lot unless a deliberate variant is authorized.
- **RULE-BID-002:** A human with permission records the Bid/No-Bid decision.
- **RULE-BID-003:** Requirement status `SATISFIED` requires evidence or an approved response appropriate to the requirement type.
- **RULE-BID-004:** Financial content is accessible only to roles granted commercial permission.
- **RULE-BID-005:** The approved-for-submission version is immutable; changes create a new candidate version and invalidate dependent approvals.

### 3.9 Document aggregate

**Root:** `DocumentRecord`

- Logical document
- File versions
- Category/type
- Owner
- Issue/expiry dates
- Verification status
- Access classification
- Retention/legal hold
- Hash, malware state, and storage reference

Rules:

- **RULE-DOC-001:** Files are unavailable to business workflows until malware/type validation passes.
- **RULE-DOC-002:** A version used in an approved submission cannot be physically replaced.
- **RULE-DOC-003:** Expired evidence cannot satisfy a requirement whose validity rule rejects it.
- **RULE-DOC-004:** Search/AI access follows the document's current authorization and classification.

### 3.10 Submission aggregate

**Root:** `Submission`

- Publication and lot
- Bidder tenant
- Final response version/hash
- Envelope(s)
- Submitter and authorization
- Receipt and timestamps
- Seal state
- Withdrawal/replacement chain
- Opening release state

State machine:

```text
DRAFT_PACKAGE → VALIDATING → READY_TO_SUBMIT → SEALED_SUBMITTED
              → REJECTED_VALIDATION
SEALED_SUBMITTED → WITHDRAWN | REPLACED | RELEASED_FOR_OPENING
RELEASED_FOR_OPENING → OPENED
```

Invariants:

- **RULE-SUB-001:** Final submission requires valid tender state, time, permissions, declarations, and idempotency key.
- **RULE-SUB-002:** Receipt time is server-controlled and immutable.
- **RULE-SUB-003:** Procurement users cannot receive decryption capability before opening release.
- **RULE-SUB-004:** Every replacement links to the prior sealed envelope; history is not erased.
- **RULE-SUB-005:** Content hash, manifest, encryption metadata, and acknowledgement must be evidence-linked.

### 3.11 Opening aggregate

**Root:** `BidOpeningSession`

- Tender/lot
- Scheduled/actual time
- Committee membership
- Declarations
- Quorum
- Submission release records
- Minutes

Rules:

- **RULE-OPEN-001:** Opening cannot start before closing except in explicitly supported procedures reviewed by governance.
- **RULE-OPEN-002:** Required committee quorum and step-up authentication must pass.
- **RULE-OPEN-003:** A committee member with unresolved conflict may not access affected submissions.
- **RULE-OPEN-004:** Release events and minutes are immutable after approved closure; corrections use addenda.

### 3.12 Evaluation aggregate

**Root:** `EvaluationCase`

- Stage
- Criteria snapshot
- Evaluator assignments
- Individual scores/findings
- Evidence references
- Clarifications
- Consolidated result
- Moderation and report

Rules:

- **RULE-EVAL-001:** Each score must use the published criterion and permitted scale.
- **RULE-EVAL-002:** Mandatory fail/disqualification requires a rule, evidence, reason, reviewer/authority, and audit.
- **RULE-EVAL-003:** Individual evaluator scores remain independent until the configured consolidation point.
- **RULE-EVAL-004:** AI suggestions are not final scores and must be visibly labeled.
- **RULE-EVAL-005:** Changes after score lock require governed unlock and reason.

### 3.13 Award aggregate

**Root:** `AwardDecision`

- Recommended bidder(s)
- Lot/value/currency
- Recommendation rationale
- Reviews/approvals
- Outcome notices
- Publication and handover

Rules:

- **RULE-AWD-001:** Award cannot precede complete configured evaluation and approvals.
- **RULE-AWD-002:** The final decision and rationale are human-authored/approved even when AI drafts text.
- **RULE-AWD-003:** Outcome notification content is versioned and delivery is tracked.

### 3.14 AI Job aggregate

**Root:** `AIJob`

- Tenant/user/use case
- Input references and authorization snapshot
- Model/provider/config versions
- Budget and priority
- Status/progress
- Structured result/citations
- Safety/quality state
- User disposition

State machine:

```text
QUEUED → RUNNING → SUCCEEDED
               └→ PARTIAL
               └→ FAILED_RETRYABLE → QUEUED
               └→ FAILED_FINAL
QUEUED/RUNNING → CANCEL_REQUESTED → CANCELLED
```

Rules:

- **RULE-AI-001:** A job may access only records authorized at execution time.
- **RULE-AI-002:** Material AI claims require citations or are labeled unsupported.
- **RULE-AI-003:** AI output cannot directly execute high-risk domain transitions.
- **RULE-AI-004:** Prompt/tool output from tender documents is untrusted data and cannot override system policy.

## 4. Domain events

Representative events:

- `TenantProvisioningRequested`
- `TenantActivated`
- `MembershipGranted`
- `ProcurementRequestApproved`
- `TenderApproved`
- `TenderPublished`
- `TenderAddendumPublished`
- `ClarificationPublished`
- `OpportunityDiscovered`
- `ExternalTenderImported`
- `BidDecisionRecorded`
- `RequirementSatisfied`
- `BidApprovedForSubmission`
- `SubmissionSealed`
- `SubmissionAcknowledged`
- `OpeningAuthorized`
- `SubmissionReleasedForOpening`
- `EvaluationStageCompleted`
- `AwardApproved`
- `OutcomePublished`
- `DocumentExpiring`
- `AIJobCompleted`

Events are facts, use past tense, and are versioned. Consumers must not infer permissions from event presence alone.

## 5. Shared value objects

- `Money(amount, currency)`
- `DateTimeWithZone(instant, zone, original_text)`
- `Percentage`
- `Score`
- `Address`
- `Contact`
- `DocumentRef`
- `SourceCitation(document_id, version, page/section/line, excerpt_hash)`
- `RecordVersion`
- `TenantContext`
- `Actor`
- `Correlation`
- `FileHash`
- `ExternalSource(source_id, url, fetched_at, confidence)`

## 6. Cross-context consistency

- Use synchronous application calls only within the same transaction boundary and bounded context where necessary.
- Use outbox events and projections for cross-context or cross-plane changes.
- Public catalogue, analytics, and search are projections; they must expose freshness and rebuild status.
- A failed projection must not roll back the authoritative tenant transaction; it must alert and retry.
- High-risk operations revalidate authoritative state immediately before execution.


---

<!-- SOURCE FILE: 06_USER_JOURNEYS_AND_WORKFLOWS.md -->

# TenderOS — User Journeys and Business Workflows

## 1. Journey design principles

- Journeys are defined by an actor's intended outcome, not by a collection of screens.
- Each step identifies responsible actor, input, business rule, system response, output, and failure path.
- Backend state and authorization control the journey. The frontend presents permitted actions and does not infer workflow completion.
- Every journey must support resume, audit, notification, and exception handling where appropriate.

## 2. Journey J-01 — Create and activate a Tenderpreneur tenant

**Primary actor:** Tenderpreneur founder/administrator  
**Outcome:** An active workspace ready to manage a tender.

```text
Register identity
→ verify email
→ create Tenderpreneur organization
→ select plan/trial
→ platform provisions private database and storage
→ complete organization profile
→ invite users and assign roles
→ upload initial compliance records
→ configure tender preferences
→ create/import first opportunity
```

### Rules and exceptions

- Duplicate legal identity or workspace names trigger review, not silent rejection.
- Provisioning failures remain visible with a retry/support path.
- The tenant is not considered activated until the first meaningful operation is complete.
- Plan limits and trial expiry are shown before they block an action.

### Evidence

- Identity verification
- Subscription state
- Provisioning events and schema version
- Administrator membership
- Onboarding completion state

## 3. Journey J-02 — Create and activate a Procurement tenant

**Primary actor:** Procurement organization administrator  
**Outcome:** An operational procurement workspace.

```text
Register and verify
→ create Procurement organization
→ select plan/enterprise onboarding
→ provision private database/storage
→ configure departments, methods, thresholds, approval templates, and committees
→ invite procurement users
→ configure tender templates and country pack
→ create first procurement request
```

High-risk configurations—approval thresholds, opening quorum, retention, SSO, and support access—require explicit administrator review.

## 4. Journey J-03 — Procurement request to approved tender draft

**Actors:** Requester, budget owner, procurement officer, reviewer, approver

```text
Requester records business need and attachments
→ department/budget owner confirms need and budget
→ procurement officer selects method and creates tender draft
→ tender author defines scope, lots, schedule, requirements, criteria, and submission schema
→ AI optionally reviews quality and risk
→ reviewers request changes or recommend approval
→ authorized approver approves a specific version
```

### Failure/exception paths

- Budget not confirmed → return to requester.
- Criteria weights invalid → prevent review submission.
- AI flags issue → advisory; human must resolve or record disposition.
- Material edit after approval → invalidate approval and create new version.

## 5. Journey J-04 — Publish, clarify, amend, and close a tender

**Actors:** Procurement officer, approver, Tenderpreneur/public visitor

```text
Schedule or publish approved version
→ create public catalogue projection
→ notify invited/matching Tenderpreneurs
→ receive clarification questions
→ draft/review/publish responses
→ create addendum when material change occurs
→ notify and capture acknowledgement where configured
→ close automatically at authoritative server time
```

### Critical rules

- Public content is generated from an approved immutable version.
- Closing time is unambiguous and timezone-aware.
- Publication failure is visible and retryable.
- A material addendum may extend deadline and invalidate Tenderpreneur readiness until acknowledged/reviewed.

## 6. Journey J-05 — Discover a tender through Deep Search

**Primary actor:** Tenderpreneur business development user

```text
Enter natural-language query or use saved search
→ platform searches configured sources and public catalogue
→ normalize and deduplicate results
→ rank by text relevance, business fit, qualification indicators, freshness, and source confidence
→ show explanations and likely gaps
→ user opens authoritative source
→ save/follow/ignore/share or create workspace
```

### Example query

> Find software-development tenders in Kenya closing within 30 days where our ERP, CRM, mobile, and cloud experience is relevant.

### Rules

- Search results state when the source was last checked.
- Inferred values are labeled.
- A result never claims the organization qualifies without analysis and human verification.
- Source access rules and rate limits are respected.

## 7. Journey J-06 — Import an external tender

**Primary actor:** Tenderpreneur bid manager

```text
Choose file, URL, email attachment, text, or manual entry
→ scan and validate content
→ extract/OCR documents
→ create provenance and hashes
→ AI extracts structured facts and requirements
→ user verifies title, buyer, dates, requirements, criteria, and submission instructions
→ system creates external tender workspace
```

### Exception paths

- Unsupported/encrypted file → request supported copy or password through secure flow.
- Low-confidence OCR → show page images and require verification.
- URL inaccessible or changed → preserve original attempt and allow file upload/manual capture.
- Duplicate opportunity → offer link/merge rather than create silent duplicate.

## 8. Journey J-07 — Bid/No-Bid decision

**Actors:** Bid manager, technical, finance, legal, management

```text
AI and system prepare eligibility/readiness summary
→ contributors assess strategic fit, capacity, risk, price potential, effort, and competition
→ missing evidence/capabilities are listed
→ authorized decision maker records BID, NO_BID, or DEFER
→ rationale and conditions are stored
```

AI can recommend; it cannot decide. A `BID` decision may generate the response plan and tasks.

## 9. Journey J-08 — Prepare a compliant bid

**Actors:** Bid manager, technical contributor, finance, legal, document controller, reviewers

```text
Create requirements matrix from verified tender facts
→ assign requirements/tasks and dependencies
→ match approved vault evidence
→ prepare technical responses
→ prepare restricted financial response
→ manage questions/addenda
→ review content and evidence
→ resolve gaps and conflicts
→ obtain technical, commercial, legal, and final approvals
```

### Operational controls

- Requirement source text and location remain visible.
- Reused content shows origin and approval state.
- Expired or superseded evidence is rejected or warned according to rule.
- Changes to approved content invalidate affected approvals.
- Team members see only permitted financial or confidential information.

## 10. Journey J-09 — AI-assisted proposal work

```text
User selects requirement/section
→ system retrieves authorized tender text and approved tenant knowledge
→ model produces structured draft with citations and unsupported-claim warnings
→ user edits and accepts/rejects
→ review and approval proceed through normal workflow
```

The UI must label generated text and show sources. User acceptance does not automatically verify a claim; verification status is separate.

## 11. Journey J-10 — External tender submission

**Actors:** Bid manager, final approver, submitter

```text
Run deterministic readiness gate
→ run AI pre-submission audit
→ resolve blockers or record authorized override
→ approve final immutable candidate
→ generate folder/envelope/package and checklist
→ submit through external portal/email/physical channel
→ record server/user time, method, reference, receipt, screenshot/email, and exact versions
→ mark submission evidence verified
```

A user cannot mark the tender `SUBMITTED` without evidence or an explicit policy-approved exception.

## 12. Journey J-11 — Platform tender submission

```text
Tenderpreneur opens platform tender workspace
→ maps responses to procurement-defined schema
→ system validates requirements, declarations, limits, and deadline
→ authorized submitter reviews final manifest and performs step-up authentication
→ system creates encrypted sealed envelope
→ authoritative server records receipt time and reference
→ Tenderpreneur receives acknowledgement
→ Procurement tenant receives submission metadata only
```

Replacement or withdrawal follows tender-specific rules and preserves history.

## 13. Journey J-12 — Bid opening

**Actors:** Opening chair/secretary and committee members

```text
Tender closes
→ system verifies opening schedule, committee, quorum, declarations, and step-up authentication
→ authorized chair starts opening session
→ sealed envelopes are released for permitted access
→ submission manifest/hash is verified
→ opening facts/minutes are recorded
→ session is closed and evidence locked
```

If quorum, time, key service, or integrity verification fails, opening stops and records the failure; no partial workaround is hidden.

## 14. Journey J-13 — Preliminary, technical, and financial evaluation

```text
Assign evaluators and criteria
→ collect conflict declarations
→ preliminary evaluators assess mandatory criteria with evidence
→ permitted bids advance
→ technical evaluators score assigned criteria independently
→ financial evaluators perform configured checks/comparisons
→ clarifications or arithmetic corrections follow governed workflow
→ scores lock
→ panel consolidates/moderates
→ report and recommendation are prepared
```

AI may locate evidence, summarize, compare, flag anomalies, and draft narrative. It cannot silently populate final scores.

## 15. Journey J-14 — Award and outcome

```text
Evaluation committee submits recommendation
→ legal/finance/management reviews as configured
→ authorized decision maker approves, rejects, or requests changes
→ successful and unsuccessful notices are generated and approved
→ outcomes are delivered/published according to policy
→ award and contract-handover package is created
→ Tenderpreneur records/receives outcome and lessons
```

## 16. Journey J-15 — Addendum impact on active bids

```text
Procurement publishes addendum
→ exchange plane emits new authoritative version
→ following/participating Tenderpreneurs are notified
→ local workspace identifies changed facts/requirements
→ affected requirement statuses and readiness are invalidated
→ bid manager reviews and acknowledges
→ tasks, content, pricing, package, and approvals are updated
```

## 17. Journey J-16 — Tenant administrator changes roles

```text
Administrator selects user
→ sees current roles and effective capabilities
→ changes role/team/scope
→ backend evaluates separation-of-duty and last-admin rules
→ sessions/capabilities are refreshed or revoked
→ audit and notification are recorded
```

The UI must not allow assignment of a role that the administrator is not permitted to grant.

## 18. Journey J-17 — Platform support access

```text
Tenant requests or approves support access
→ scope, reason, records/features, duration, and approver are captured
→ platform privileged user performs step-up authentication
→ temporary access token is issued
→ every viewed/changed record is audited
→ access expires or tenant revokes it
→ support session summary is available to tenant administrator
```

## 19. Journey J-18 — Subscription limit reached

```text
User attempts limited action
→ entitlement service evaluates current usage and reservation
→ operation is allowed, warned, queued for approval, or blocked
→ UI explains limit and actionable options
→ authorized admin upgrades or frees capacity
→ operation is retried idempotently
```

## 20. Workflow design checklist

Every workflow specification and implementation ticket must state:

- Trigger and initiating actor
- Preconditions and required capabilities
- Authoritative inputs
- State transitions and transaction boundary
- Validation and business rules
- Approval/separation-of-duty requirements
- Notifications and events
- Audit/evidence
- Retry/idempotency/concurrency behavior
- Expected frontend states
- Failure and recovery paths
- Tests and acceptance evidence


---

<!-- SOURCE FILE: 07_MODULE_DEPENDENCIES.md -->

# TenderOS — Module Catalogue and Dependency Order

## 1. Purpose

This document prevents downstream modules from being built on missing foundations. A module may be designed early, but production implementation must follow its dependency gates.

## 2. Dependency graph

```text
M00 Product Governance & Contracts
  └─ M01 Repository, CI/CD & Environments
      ├─ M02 Platform Control Plane
      │   ├─ M03 Tenant Provisioning & Database Routing
      │   │   └─ M04 Identity, Membership, Context & Authorization
      │   │       ├─ M05 Subscription, Entitlements & Usage
      │   │       ├─ M06 Audit & Evidence
      │   │       ├─ M07 Document & Object Storage
      │   │       ├─ M08 Workflow, Approval, Tasks & Notifications
      │   │       └─ M09 Search & Read-Model Foundation
      │   │
      │   ├─ Procurement branch
      │   │   └─ M10 Procurement Planning & Requisitions
      │   │       └─ M11 Tender Authoring, Criteria & Review
      │   │           └─ M12 Publication, Catalogue, Clarifications & Addenda
      │   │               └─ M17 Submission Exchange & Bid Sealing
      │   │                   └─ M18 Bid Opening
      │   │                       └─ M19 Evaluation
      │   │                           └─ M20 Award & Handover
      │   │
      │   └─ Tenderpreneur branch
      │       └─ M13 Organization Profile & Compliance Vault
      │           ├─ M14 External Tender Import & Ingestion
      │           ├─ M15 Deep Search, Matching & Watchlists
      │           └─ M16 Bid Workspace, Compliance, Proposal & Readiness
      │               └─ M17 Submission Exchange & Bid Sealing
      │
      └─ Intelligence branch
          └─ M21 AI Platform & Evaluation Foundation
              ├─ M22 Tenderpreneur AI
              ├─ M23 Procurement AI
              └─ M24 Analytics, Intelligence & Reporting

M25 Internationalization, Security Hardening & Enterprise Integrations
spans and validates all modules; it is introduced early and completed progressively.
```

## 3. Module specifications

## M00 — Product governance and contracts

**Purpose:** Establish requirements, terminology, business rules, traceability, OpenAPI/event conventions, decision process, and acceptance gates.

**Outputs:** Approved specs, requirement IDs, decision log, contract templates, evidence register.

**Exit gate:** No ambiguous portal or tenancy model; MVP scope and precedence accepted.

## M01 — Repository, CI/CD, environments, and developer platform

**Dependencies:** M00

**Outputs:** Monorepo, lint/test/build pipelines, local Docker Compose, secret handling, code owners, branch protections, image builds, testcontainers, observability skeleton.

**Exit gate:** One command starts local dependencies; sample API/web/worker/AI health checks pass in CI.

## M02 — Platform control plane

**Dependencies:** M01

**Owns:** Tenant registry, plans, country packs, source registry, feature flags, platform users/roles, provisioning jobs, service metadata.

**Exit gate:** Platform Admin can view/control non-sensitive platform metadata with full audit.

## M03 — Tenant provisioning and database routing

**Dependencies:** M02

**Owns:** Database allocation, credentials, schema version, migration orchestration, storage namespace, tenant lifecycle, connection routing.

**Exit gate:** Automated create, migrate, suspend, restore-to-new, and decommission tests pass for multiple tenants.

## M04 — Identity, membership, context, and authorization

**Dependencies:** M02, M03

**Owns:** Identity, sessions, invitations, memberships, roles, policies, active context, MFA/step-up foundation.

**Exit gate:** `/v1/me`, context selection, capability evaluation, revocation, 401/403 semantics, and cross-tenant tests pass.

## M05 — Subscription, entitlements, usage, and billing

**Dependencies:** M02, M04

**Owns:** Plans, subscriptions, limits, usage meters, reservations, invoices, payment/webhook state.

**Exit gate:** Backend blocks over-limit operations consistently and handles webhook replay idempotently.

## M06 — Audit and evidence

**Dependencies:** M03, M04

**Owns:** Append-only audit, correlation, evidence manifests, exports, support-access logs.

**Exit gate:** Material operations produce complete, queryable evidence without sensitive diagnostic leakage.

## M07 — Document and object storage

**Dependencies:** M03, M04, M06

**Owns:** Upload, scan, checksum, metadata, version, access, expiry, preview, retention, signed download.

**Exit gate:** Tenant isolation, malware quarantine, version immutability, large upload, and signed URL tests pass.

## M08 — Workflow, approval, tasks, and notifications

**Dependencies:** M04, M05, M06, M07

**Owns:** Workflow templates/instances, approvals, task dependencies, comments, notification preferences and dispatch.

**Exit gate:** Serial/parallel approval, rework, delegation, expiry, idempotent notification, and permission tests pass.

## M09 — Search and read-model foundation

**Dependencies:** M03, M04, M06

**Owns:** Tenant-private search contracts, public catalogue index contracts, rebuild/version/freshness, reporting projections.

**Exit gate:** Search never reveals unauthorized record existence; indexes rebuild from source data.

## M10 — Procurement planning and requisitions

**Dependencies:** M04–M08

**Owns:** Plans, requests, budgets, methods, approvals, conversion.

**Exit gate:** Requester-to-approved-requisition journey passes UAT.

## M11 — Tender authoring, criteria, and review

**Dependencies:** M10, M07, M08

**Owns:** Tender drafts, lots, schedule, requirements, criteria, forms, submission schema, review/approval.

**Exit gate:** An approved, internally consistent version can be produced; invalid criteria/dates cannot pass.

## M12 — Publication, catalogue, clarifications, and addenda

**Dependencies:** M09, M11

**Owns:** Publication version, public projection, invitation, question/response, addendum, deadline changes.

**Exit gate:** Public page matches approved hash/version; addendum impact and notifications work end to end.

## M13 — Tenderpreneur profile and compliance vault

**Dependencies:** M04–M08

**Owns:** Company capabilities, document types, staff, certifications, projects, references, knowledge approval.

**Exit gate:** Authorized users can maintain reusable, versioned, expiry-aware evidence.

## M14 — External tender import and ingestion

**Dependencies:** M07, M09, M13; AI extraction may be added after M21

**Owns:** File/URL/email/text/manual import, provenance, basic deterministic metadata capture, duplicate detection, verification.

**Exit gate:** An external tender workspace can be created without AI and remains fully usable.

## M15 — Deep Search, matching, and watchlists

**Dependencies:** M02 source registry, M09, M13; advanced semantic ranking depends on M21

**Owns:** Source fetch, canonicalization, deduplication, search filters, match factors, watchlists, alerts, workspace conversion.

**Exit gate:** Approved sources produce fresh, traceable, deduplicated results with one-click import.

## M16 — Bid workspace, compliance, proposal, and readiness

**Dependencies:** M08, M13, M14 and/or M12

**Owns:** Bid decision, requirements, tasks, response content, pricing, reviews, approvals, readiness, package/evidence.

**Exit gate:** External tender journey completes end to end without mocks.

## M17 — Submission exchange and bid sealing

**Dependencies:** M11, M12, M16, M06, M07

**Owns:** Submission schema validation, final version, envelope, encryption/sealing, receipt, withdrawal, replacement, opening release.

**Exit gate:** Procurement users cannot access content before authorized release; spike and replay tests pass.

## M18 — Bid opening

**Dependencies:** M17, M08

**Owns:** Committee, quorum, declarations, release, integrity verification, minutes.

**Exit gate:** Authorized opening succeeds and all invalid opening attempts fail closed with evidence.

## M19 — Evaluation

**Dependencies:** M18, M11, M08

**Owns:** Preliminary, technical, financial, evaluator assignment, score lock, moderation, clarification, report.

**Exit gate:** Published criteria drive evidence-linked, permission-scoped evaluation through consolidated result.

## M20 — Award and handover

**Dependencies:** M19, M08

**Owns:** Recommendation, approval, outcome notices, award publication, contract handover.

**Exit gate:** Award journey completes with approved notices and immutable evidence.

## M21 — AI platform and evaluation foundation

**Dependencies:** M03, M04, M06, M07, M09

**Owns:** AI jobs, provider adapters, retrieval authorization, structured outputs, citations, prompt/model registry, cost/usage, evaluation datasets.

**Exit gate:** A grounded extraction job is reproducible, tenant-safe, measured, and evaluated.

## M22 — Tenderpreneur AI

**Dependencies:** M13–M16, M21

**Owns:** Tender extraction, Bid/No-Bid assistance, compliance matching, proposal drafting, submission audit.

**Exit gate:** Offline benchmark and UAT meet agreed citation/accuracy thresholds; no high-risk automatic transition.

## M23 — Procurement AI

**Dependencies:** M10–M12, M18–M21

**Owns:** Tender drafting/review, criteria assistance, evidence location, bid comparison, price analysis, report drafting.

**Exit gate:** Evaluators can inspect sources and accept/edit/reject suggestions; final scoring remains manual.

## M24 — Analytics, intelligence, and reporting

**Dependencies:** M06, M09 and relevant domain modules

**Owns:** Tenant dashboards, platform metrics, pipeline, cycle time, win/loss, disqualification, participation, usage, AI quality.

**Exit gate:** Every metric has a defined source, formula, freshness, permission, and reconciliation test.

## M25 — Internationalization, security hardening, and enterprise integrations

**Dependencies:** Cross-cutting

**Owns:** Country packs, translations, SSO, APIs/webhooks, retention, legal hold, advanced security, dedicated infrastructure, data export/import.

**Exit gate:** Country/enterprise release checklist passes without modifying core domain invariants.

## 4. Dependency enforcement rules

1. A downstream frontend page cannot be accepted before its backend contract and domain rules exist.
2. A backend use case cannot be accepted before database constraints/migrations and authorization policy exist.
3. AI may enhance M14–M20 only after deterministic workflows work without AI.
4. Analytics may not become a substitute for operational capabilities.
5. Cross-tenant exchange must use M17; direct tenant database reads are prohibited.
6. Each module must publish explicit APIs/events rather than allowing table access from another module.
7. Every wave must include migration, API, UI, tests, evidence, and operational runbook changes where relevant.


---

<!-- SOURCE FILE: 08_HLD.md -->

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


---

<!-- SOURCE FILE: 09_LLD.md -->

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


---

<!-- SOURCE FILE: 10_DATABASE_DESIGN.md -->

# TenderOS — Database and Data Architecture

## 1. Database topology

TenderOS uses three database categories:

1. **Control database:** one platform database for identities, tenant registry, subscriptions, source registry, and platform operations.
2. **Tenant databases:** one private PostgreSQL database per Procurement or Tenderpreneur tenant.
3. **Exchange database:** one logical platform database for public tender projections, sealed-submission metadata, and cross-tenant exchange state.

Large enterprise tenants may receive dedicated clusters. The logical contract remains database-per-tenant.

## 2. Data isolation rules

- Tenant business tables exist only in the tenant's database.
- No cross-tenant foreign key, SQL join, or database link is permitted.
- Database selection is derived from authenticated context and the tenant registry.
- Each database has tenant-specific application credentials or securely isolated role mapping.
- Object storage, search indexes, embeddings, backups, and exports use the same isolation principle.
- The exchange database holds only approved public data or minimum cross-tenant transaction metadata.

## 3. Identifier and common-column standards

Recommended identifiers: application-generated UUIDv7 or equivalent time-sortable UUID.

Common columns:

```sql
id                uuid primary key,
status            text not null,
version           bigint not null default 1,
created_at        timestamptz not null,
created_by        uuid,
updated_at        timestamptz not null,
updated_by        uuid,
deleted_at        timestamptz,
deleted_by        uuid
```

Not every append-only table needs `updated_*` or soft deletion. Domain lifecycle fields must be explicit.

Standards:

- Money: `numeric(20,4)` plus ISO currency code.
- Percent/score: constrained numeric, not float.
- Time: `timestamptz`; retain source timezone/name separately for deadlines.
- JSONB: only for flexible metadata, event payloads, or versioned configuration—not as a substitute for core relational design.
- Enumerations: lookup/check constraints or versioned code tables; avoid database enum types when frequent evolution is expected.
- Text search: generated tsvector/index where appropriate.

## 4. Control database schemas and tables

### 4.1 `identity`

- `users`
- `user_emails`
- `password_credentials`
- `external_identities`
- `mfa_methods`
- `sessions`
- `session_events`
- `recovery_challenges`
- `security_risk_events`

### 4.2 `tenancy`

- `tenants`
- `tenant_types`
- `tenant_database_assignments`
- `database_clusters`
- `tenant_storage_assignments`
- `tenant_schema_versions`
- `tenant_provisioning_jobs`
- `tenant_lifecycle_events`
- `memberships`
- `membership_role_grants`
- `invitations`

`tenant_database_assignments` contains a secret reference, never plaintext credentials.

### 4.3 `subscription`

- `plans`
- `plan_versions`
- `features`
- `plan_entitlements`
- `subscriptions`
- `subscription_events`
- `usage_meters`
- `usage_ledger`
- `usage_reservations`
- `invoices`
- `invoice_lines`
- `payments`
- `payment_webhook_events`

Usage ledger is append-oriented and aggregated to current usage projections.

### 4.4 `platform`

- `feature_flags`
- `country_packs`
- `reference_data_versions`
- `source_registry`
- `source_connectors`
- `connector_runs`
- `integration_credentials` (secret references)
- `platform_announcements`
- `support_access_requests`
- `support_access_sessions`
- `platform_audit_events`

## 5. Common tenant schema

Every tenant database contains common modules.

### 5.1 Organization and access projection

- `organization_profile`
- `departments`
- `teams`
- `tenant_user_profiles`
- `role_definitions`
- `role_assignments`
- `permission_overrides`
- `organization_settings`

The control plane is authoritative for identity/membership; tenant DB retains a local projection needed for business assignments and audit.

### 5.2 Document and knowledge

- `documents`
- `document_versions`
- `document_files`
- `document_access_policies`
- `document_verifications`
- `document_expiry_rules`
- `knowledge_items`
- `knowledge_item_versions`
- `knowledge_approvals`
- `document_usage_links`
- `retention_holds`

Important uniqueness/constraints:

- File checksum + logical document/version relationship
- One current approved version per governed knowledge item
- Expiry date after issue date where both exist
- Malware state must be `CLEAN` before `AVAILABLE`

### 5.3 Workflow and collaboration

- `workflow_templates`
- `workflow_template_versions`
- `workflow_steps`
- `workflow_instances`
- `workflow_step_instances`
- `approval_decisions`
- `delegations`
- `tasks`
- `task_dependencies`
- `comments`
- `mentions`
- `notification_preferences`
- `notification_outbox`
- `notification_deliveries`

### 5.4 Audit and eventing

- `audit_events`
- `evidence_manifests`
- `outbox_events`
- `inbox_events`
- `idempotency_records`
- `scheduled_jobs`

Audit events are append-only. Update/delete permissions are denied to the application role.

### 5.5 AI and search

- `ai_jobs`
- `ai_job_inputs`
- `ai_job_outputs`
- `ai_citations`
- `ai_user_dispositions`
- `ai_usage_records`
- `document_chunks`
- `chunk_embeddings` or pgvector column
- `search_index_state`

Private chunks and embeddings remain in the tenant database or a tenant-isolated index namespace.

## 6. Procurement tenant schema

### 6.1 Planning

- `procurement_plans`
- `procurement_plan_items`
- `procurement_requests`
- `procurement_request_versions`
- `budget_confirmations`
- `method_selections`

### 6.2 Tender authoring

- `tenders`
- `tender_versions`
- `tender_lots`
- `tender_schedules`
- `tender_requirements`
- `evaluation_criteria`
- `criterion_scales`
- `submission_fields`
- `tender_forms`
- `tender_documents`
- `tender_review_findings`

Recommended constraints:

- Unique `(tender_reference, procurement_period)`
- Requirement/criterion sequence unique within version/lot/section
- Criterion weight and score ranges constrained
- Closing after opening/publication dates
- Published version reference immutable

### 6.3 Publication and clarification

- `publication_intents`
- `publication_deliveries`
- `tender_invitations`
- `clarification_questions`
- `clarification_responses`
- `addenda`
- `addendum_changes`
- `participant_acknowledgements`

### 6.4 Submission projection and opening

Procurement tenant stores metadata projections before opening and content references after release:

- `submission_receipts`
- `opening_sessions`
- `opening_committee_members`
- `conflict_declarations`
- `opening_release_records`
- `opening_minutes`

### 6.5 Evaluation

- `evaluation_cases`
- `evaluation_stages`
- `evaluator_assignments`
- `preliminary_findings`
- `criterion_scores`
- `evaluation_evidence_refs`
- `evaluation_clarifications`
- `arithmetic_corrections`
- `score_locks`
- `moderation_records`
- `evaluation_results`
- `evaluation_reports`

Use uniqueness on evaluator + submission + criterion + attempt/version.

### 6.6 Award

- `award_recommendations`
- `award_decisions`
- `award_approvals`
- `outcome_notices`
- `award_publications`
- `contract_handovers`

## 7. Tenderpreneur tenant schema

### 7.1 Organization capability profile

- `business_capabilities`
- `products_services`
- `certifications`
- `licences`
- `staff_profiles`
- `staff_qualifications`
- `past_projects`
- `project_references`
- `manufacturer_authorizations`
- `financial_profile_records`
- `search_preferences`

Sensitive fields use classification and field-level authorization.

### 7.2 Opportunities and external sources

- `opportunities`
- `opportunity_sources`
- `external_tender_imports`
- `extracted_tender_facts`
- `fact_verifications`
- `opportunity_matches`
- `saved_searches`
- `watchlists`
- `watchlist_targets`
- `opportunity_alerts`

### 7.3 Bid workspace

- `bid_workspaces`
- `bid_workspace_versions`
- `bid_decisions`
- `bid_assessments`
- `bid_assessment_factors`
- `bid_requirements`
- `requirement_assignments`
- `requirement_evidence`
- `response_sections`
- `response_section_versions`
- `financial_responses`
- `financial_response_lines`
- `bid_reviews`
- `bid_readiness_results`
- `bid_risks`
- `submission_packages`
- `external_submission_evidence`
- `bid_outcomes`
- `lessons_learned`

Constraints:

- Active workspace uniqueness by tenant + authoritative tender/lot unless variant flag
- Financial tables restricted through application policy and separate query adapters
- Approved response version immutable
- Evidence references include exact document version

## 8. Exchange database schema

### 8.1 Public catalogue

- `public_tenders`
- `public_tender_versions`
- `public_lots`
- `public_requirements`
- `public_criteria`
- `public_documents`
- `public_addenda`
- `public_clarifications`
- `public_source_links`

Public projection stores content approved for publication and source hashes.

### 8.2 Participation

- `tender_participation_registrations`
- `tender_followers`
- `invitation_deliveries`
- `addendum_acknowledgements`

### 8.3 Sealed submissions

- `submissions`
- `submission_versions`
- `submission_envelopes`
- `submission_manifest_entries`
- `submission_receipts`
- `submission_withdrawals`
- `submission_replacements`
- `opening_release_authorizations`
- `opening_release_events`

Do not store plaintext private submission content in ordinary columns.

### 8.4 Outcome exchange

- `award_outcomes`
- `outcome_delivery_receipts`
- `exchange_outbox_events`
- `exchange_inbox_events`

## 9. Deep Search platform data

Deep Search public-source data may use the exchange/search database or a dedicated search datastore.

Tables/index entities:

- `source_documents`
- `source_fetches`
- `canonical_opportunities`
- `opportunity_source_links`
- `source_content_versions`
- `source_change_events`
- `search_chunks`
- `search_embeddings`
- `deduplication_candidates`
- `source_health_metrics`

Required provenance:

- Source registry ID
- URL/identifier
- First and last seen
- Fetched/published/closing timestamps
- HTTP/content metadata
- Content hash and parser version
- Authoritative-source flag and confidence
- Original document reference

## 10. Indexing strategy

Examples:

- B-tree on state, date, owner, foreign keys, and frequently filtered columns
- Partial indexes for active/open/pending records
- GIN on `tsvector` search columns and selected JSONB metadata
- Vector index only after measuring dataset/query pattern
- Composite indexes aligned with tenant-database query patterns; tenant ID is not repeated in every tenant table unless needed for audit/import portability
- Exchange/control tables include tenant IDs and indexes because they are multi-tenant platform stores

Every index must be justified by a query or constraint; unnecessary indexes increase write and migration cost.

## 11. Migration strategy

### Migration sets

- Control
- Exchange
- Common tenant
- Procurement tenant extension
- Tenderpreneur tenant extension

### Rules

1. Migrations are immutable after release.
2. Each migration has forward SQL, verification query, data/backfill plan, compatibility impact, and recovery approach.
3. Prefer expand-and-contract for zero/low-downtime changes.
4. Application release must remain compatible with current and next schema version during rollout.
5. Tenant migrations run in canary batches and record per-tenant state.
6. Failed tenants are isolated and retried; rollout can pause.
7. Destructive changes require backup verification and explicit approval.

## 12. Provisioning workflow

```text
Create tenant registry record
→ choose cluster based on region/capacity/plan
→ create database and least-privileged roles
→ store credentials in secrets manager
→ apply common + tenant-type migrations
→ seed country/reference/workflow defaults
→ create object-storage namespace/policy
→ verify read/write/backup/health
→ mark tenant ACTIVE
```

Provisioning is idempotent and resumable.

## 13. Backup and restore

- Point-in-time recovery for production database clusters where supported
- Encrypted full/base backups and WAL/archive retention
- Tenant-level logical backup/export for portability and legal requests
- Object storage versioning and retention
- Control/exchange/tenant backup schedules coordinated but independently restorable
- Restore tests into isolated environments
- Evidence of backup age, success, checksum, and restore drill

Suggested starting targets, subject to plan/business approval:

- Control/exchange production RPO: 15 minutes or better
- Standard tenant RPO: 1 hour or better
- Critical enterprise tenant RPO: 15 minutes or contract-specific
- RTO differentiated by plan and incident severity

## 14. Retention and deletion

Retention policies are record-type and country/tenant configurable.

Lifecycle:

```text
ACTIVE → ARCHIVED → RETENTION_LOCKED/LEGAL_HOLD → ELIGIBLE_FOR_DELETION → DELETED/ANONYMIZED
```

Deletion must cover:

- Transactional rows
- Object versions
- Search indexes/embeddings
- Derived caches
- Exports
- Backups according to documented expiry

Audit/evidence that must be legally retained is separated from unnecessary personal content.

## 15. Data quality controls

- Reference codes and country-pack validation
- Required fields by state transition
- Referential and uniqueness constraints
- Source/fact verification status
- Currency and timezone explicitness
- Document issue/expiry validation
- Score/weight reconciliation
- Hash reconciliation for public projections and submissions
- Periodic orphan, stale projection, and index-rebuild checks

## 16. Reporting strategy

Operational queries use tenant database read models. Platform-wide analytics use privacy-minimized events/aggregates delivered to an analytics store; they do not query all tenant databases interactively.

Reports must define:

- Source tables/events
- Formula
- Timezone/currency treatment
- Freshness
- Permission/classification
- Reconciliation test


---

<!-- SOURCE FILE: 11_BACKEND_API_DESIGN.md -->

# TenderOS — Backend and API Design

## 1. Backend responsibility

The Go backend is the authoritative implementation of:

- Business state and transitions
- Tenant context and authorization
- Validation and calculations
- Approval, publication, submission, opening, evaluation, and award controls
- Plan entitlement and usage enforcement
- Audit and evidence generation
- API and event contracts

The frontend must not reproduce or override these rules. Python AI services return advisory structured results and cannot directly execute high-risk transitions.

## 2. API style

- REST/JSON for primary synchronous APIs
- OpenAPI 3.1 as the contract authority
- Server-sent events or polling for progress updates initially; WebSocket only where justified
- Webhooks for approved external integrations
- Versioned domain events for asynchronous communication

Base paths:

```text
/v1/auth/*
/v1/me
/v1/context/*
/v1/platform/*
/v1/procurement/*
/v1/tenderpreneur/*
/v1/exchange/*
/v1/shared/*
/v1/ai/*
/v1/search/*
/v1/webhooks/*
```

## 3. Response contracts

### Success

```json
{
  "data": {
    "id": "019...",
    "status": "DRAFT"
  },
  "meta": {
    "trace_id": "019...",
    "version": 3
  }
}
```

### List

```json
{
  "data": [
    { "id": "019...", "title": "Supply and Implementation..." }
  ],
  "meta": {
    "next_cursor": "opaque-token",
    "has_more": true,
    "trace_id": "019..."
  }
}
```

### Error

```json
{
  "error": {
    "code": "SUBMISSION_DEADLINE_PASSED",
    "message": "This tender is no longer accepting submissions.",
    "details": {
      "closed_at": "2026-10-30T08:00:00Z"
    },
    "field_errors": [],
    "trace_id": "019...",
    "retryable": false
  }
}
```

## 4. API contract standards

- JSON property names use `snake_case` consistently.
- IDs are opaque UUID strings; clients do not derive meaning from them.
- Dates use ISO 8601/RFC 3339 with offset/UTC; deadline DTOs include display timezone.
- Currency uses `{ amount: "1000.00", currency: "KES" }`, not floating-point numbers.
- Statuses are stable string codes with display labels localized by the frontend.
- Optional and nullable are different and documented.
- Every resource DTO includes permitted `actions` or effective capability hints where useful.
- Sensitive fields are omitted, not returned as masked placeholders unless the product requires a visible masked value.

## 5. Headers and request controls

Supported headers:

- `Authorization` for API/OAuth clients where applicable
- Secure HTTP-only session cookie for web sessions
- `Idempotency-Key` for qualifying writes
- `If-Match` with ETag or expected version for mutable resources
- `X-Request-ID` accepted only as a client correlation suggestion; server validates/generates canonical trace ID
- `Accept-Language`

A tenant identifier header may be accepted only as a routing hint after matching it to the authenticated active context. It never grants access.

## 6. Authentication and current context APIs

```text
POST   /v1/auth/register
POST   /v1/auth/verify-email
POST   /v1/auth/login
POST   /v1/auth/logout
POST   /v1/auth/refresh-or-extend
POST   /v1/auth/recovery/start
POST   /v1/auth/recovery/complete
POST   /v1/auth/mfa/challenge
POST   /v1/auth/mfa/verify
POST   /v1/auth/step-up
GET    /v1/me
GET    /v1/me/memberships
POST   /v1/context/select
DELETE /v1/context
GET    /v1/me/effective-capabilities
```

`GET /v1/me` must be sufficient to bootstrap the web application:

```json
{
  "data": {
    "identity": { "id": "...", "name": "...", "email": "..." },
    "session": {
      "expires_at": "...",
      "assurance": "MFA",
      "step_up_until": null
    },
    "active_context": {
      "tenant_id": "...",
      "tenant_type": "TENDERPRENEUR",
      "tenant_name": "...",
      "membership_id": "..."
    },
    "memberships": [],
    "capabilities": [],
    "preferences": { "locale": "en-KE", "timezone": "Africa/Nairobi" },
    "required_actions": []
  }
}
```

## 7. Platform Admin APIs

### Tenant and provisioning

```text
GET    /v1/platform/tenants
POST   /v1/platform/tenants
GET    /v1/platform/tenants/{tenant_id}
POST   /v1/platform/tenants/{tenant_id}/suspend
POST   /v1/platform/tenants/{tenant_id}/reactivate
POST   /v1/platform/tenants/{tenant_id}/decommission
GET    /v1/platform/tenants/{tenant_id}/provisioning
POST   /v1/platform/tenants/{tenant_id}/migrations/retry
POST   /v1/platform/tenants/{tenant_id}/restore-jobs
```

### Plans, usage, and billing

```text
GET/POST /v1/platform/plans
GET/PATCH /v1/platform/plans/{plan_id}
POST      /v1/platform/plans/{plan_id}/versions
GET       /v1/platform/subscriptions
GET       /v1/platform/usage
GET       /v1/platform/invoices
```

### Sources and operations

```text
GET/POST /v1/platform/sources
GET/PATCH /v1/platform/sources/{source_id}
POST      /v1/platform/sources/{source_id}/test
POST      /v1/platform/sources/{source_id}/pause
GET       /v1/platform/connector-runs
GET       /v1/platform/service-health
GET       /v1/platform/migration-status
GET       /v1/platform/backup-status
```

### Support access

```text
POST /v1/platform/support-access/requests
POST /v1/platform/support-access/{id}/activate
POST /v1/platform/support-access/{id}/revoke
GET  /v1/platform/support-access/{id}/audit
```

## 8. Shared tenant APIs

### Organization and users

```text
GET/PATCH /v1/shared/organization
GET/POST  /v1/shared/departments
GET/POST  /v1/shared/teams
GET       /v1/shared/users
POST      /v1/shared/invitations
PATCH     /v1/shared/memberships/{id}
DELETE    /v1/shared/memberships/{id}
GET       /v1/shared/roles
POST      /v1/shared/role-assignments
```

### Documents and knowledge

```text
POST   /v1/shared/uploads/initiate
POST   /v1/shared/uploads/{upload_id}/complete
GET    /v1/shared/documents
POST   /v1/shared/documents
GET    /v1/shared/documents/{id}
POST   /v1/shared/documents/{id}/versions
POST   /v1/shared/documents/{id}/verify
POST   /v1/shared/documents/{id}/archive
GET    /v1/shared/documents/{id}/download-url
GET    /v1/shared/knowledge
POST   /v1/shared/knowledge
POST   /v1/shared/knowledge/{id}/approve
```

### Tasks, workflows, approvals, notifications

```text
GET/POST /v1/shared/tasks
GET/PATCH /v1/shared/tasks/{id}
POST      /v1/shared/tasks/{id}/comments
GET       /v1/shared/approvals/pending
POST      /v1/shared/approvals/{step_id}/decisions
GET/POST /v1/shared/workflow-templates
GET       /v1/shared/notifications
POST      /v1/shared/notifications/{id}/read
GET/PATCH /v1/shared/notification-preferences
```

## 9. Procurement APIs

### Planning and requisitions

```text
GET/POST /v1/procurement/plans
GET/POST /v1/procurement/requests
GET/PATCH /v1/procurement/requests/{id}
POST      /v1/procurement/requests/{id}/submit
POST      /v1/procurement/requests/{id}/approve
POST      /v1/procurement/requests/{id}/convert-to-tender
```

### Tender authoring

```text
GET/POST /v1/procurement/tenders
GET/PATCH /v1/procurement/tenders/{id}
POST      /v1/procurement/tenders/{id}/versions
GET/POST /v1/procurement/tenders/{id}/lots
GET/POST /v1/procurement/tenders/{id}/requirements
GET/POST /v1/procurement/tenders/{id}/criteria
GET/POST /v1/procurement/tenders/{id}/submission-schema
POST      /v1/procurement/tenders/{id}/validate
POST      /v1/procurement/tenders/{id}/submit-for-review
POST      /v1/procurement/tenders/{id}/publish
POST      /v1/procurement/tenders/{id}/cancel
```

### Clarifications and addenda

```text
GET       /v1/procurement/tenders/{id}/clarifications
POST      /v1/procurement/clarifications/{id}/draft-response
POST      /v1/procurement/clarifications/{id}/publish
GET/POST /v1/procurement/tenders/{id}/addenda
POST      /v1/procurement/addenda/{id}/publish
```

### Submissions and opening

```text
GET  /v1/procurement/tenders/{id}/submission-receipts
POST /v1/procurement/tenders/{id}/opening-sessions
POST /v1/procurement/opening-sessions/{id}/declarations
POST /v1/procurement/opening-sessions/{id}/start
POST /v1/procurement/opening-sessions/{id}/release
POST /v1/procurement/opening-sessions/{id}/close
GET  /v1/procurement/opening-sessions/{id}/minutes
```

Before opening, receipt DTOs expose permitted metadata only.

### Evaluation and award

```text
POST      /v1/procurement/tenders/{id}/evaluation-cases
GET/POST /v1/procurement/evaluations/{id}/assignments
GET/POST /v1/procurement/evaluations/{id}/preliminary-findings
GET/POST /v1/procurement/evaluations/{id}/scores
POST      /v1/procurement/evaluations/{id}/lock-stage
POST      /v1/procurement/evaluations/{id}/moderate
POST      /v1/procurement/evaluations/{id}/complete
GET/POST /v1/procurement/evaluations/{id}/recommendations
POST      /v1/procurement/recommendations/{id}/submit
POST      /v1/procurement/awards/{id}/approve
POST      /v1/procurement/awards/{id}/notify
POST      /v1/procurement/awards/{id}/handover
```

## 10. Tenderpreneur APIs

### Profile and compliance vault

```text
GET/PATCH /v1/tenderpreneur/profile
GET/POST  /v1/tenderpreneur/capabilities
GET/POST  /v1/tenderpreneur/certifications
GET/POST  /v1/tenderpreneur/staff
GET/POST  /v1/tenderpreneur/projects
GET/POST  /v1/tenderpreneur/references
GET       /v1/tenderpreneur/compliance-readiness
```

### Opportunities and import

```text
GET       /v1/tenderpreneur/opportunities
POST      /v1/tenderpreneur/opportunities/import
POST      /v1/tenderpreneur/opportunities/import-url
POST      /v1/tenderpreneur/opportunities/manual
GET       /v1/tenderpreneur/opportunities/{id}
POST      /v1/tenderpreneur/opportunities/{id}/verify-facts
POST      /v1/tenderpreneur/opportunities/{id}/create-workspace
POST      /v1/tenderpreneur/opportunities/{id}/follow
POST      /v1/tenderpreneur/opportunities/{id}/ignore
```

### Bid workspaces

```text
GET/POST /v1/tenderpreneur/bids
GET/PATCH /v1/tenderpreneur/bids/{id}
GET/POST /v1/tenderpreneur/bids/{id}/assessments
POST      /v1/tenderpreneur/bids/{id}/decision
GET/POST /v1/tenderpreneur/bids/{id}/requirements
PATCH     /v1/tenderpreneur/bids/{id}/requirements/{requirement_id}
POST      /v1/tenderpreneur/bids/{id}/requirements/{requirement_id}/evidence
GET/POST /v1/tenderpreneur/bids/{id}/response-sections
GET/POST /v1/tenderpreneur/bids/{id}/financial-response
POST      /v1/tenderpreneur/bids/{id}/submit-for-review
POST      /v1/tenderpreneur/bids/{id}/approve-for-submission
GET       /v1/tenderpreneur/bids/{id}/readiness
POST      /v1/tenderpreneur/bids/{id}/packages
```

### External submission evidence and outcome

```text
POST /v1/tenderpreneur/bids/{id}/external-submission-evidence
POST /v1/tenderpreneur/bids/{id}/mark-submitted
POST /v1/tenderpreneur/bids/{id}/outcome
POST /v1/tenderpreneur/bids/{id}/lessons
```

## 11. Public catalogue and Deep Search APIs

```text
GET  /v1/search/tenders
POST /v1/search/tenders/query
GET  /v1/search/tenders/{public_tender_id}
POST /v1/search/saved-searches
GET  /v1/search/saved-searches
POST /v1/search/watchlists
GET  /v1/search/watchlists
POST /v1/search/results/{id}/create-workspace
GET  /v1/search/sources/{source_id}/status
```

Search result DTO includes:

- Canonical identity and source links
- Title, buyer, location, dates, category, method, value if verified
- Freshness and last checked
- Match score and factors
- Qualification indicators and likely gaps
- Source confidence and authoritative-source flag
- Actions permitted for the user

## 12. Exchange and submission APIs

Tenderpreneur-facing:

```text
POST /v1/exchange/tenders/{publication_id}/participate
POST /v1/exchange/tenders/{publication_id}/submissions/validate
POST /v1/exchange/tenders/{publication_id}/submissions
GET  /v1/exchange/submissions/{submission_id}/receipt
POST /v1/exchange/submissions/{submission_id}/withdraw
POST /v1/exchange/submissions/{submission_id}/replace
```

Submission creation requires:

- `Idempotency-Key`
- Expected publication version
- Final bid version
- Manifest
- Required declarations
- Step-up proof/reference

## 13. AI APIs

```text
POST /v1/ai/jobs/tender-extraction
POST /v1/ai/jobs/bid-assessment
POST /v1/ai/jobs/compliance-match
POST /v1/ai/jobs/response-draft
POST /v1/ai/jobs/submission-audit
POST /v1/ai/jobs/tender-quality-review
POST /v1/ai/jobs/bid-comparison
POST /v1/ai/jobs/evaluation-report
POST /v1/ai/jobs/deep-research
GET  /v1/ai/jobs/{job_id}
POST /v1/ai/jobs/{job_id}/cancel
POST /v1/ai/jobs/{job_id}/disposition
```

The core API creates jobs after authorization. The Python service does not trust client-supplied tenant IDs or raw object paths.

## 14. Webhooks

Inbound examples:

- Payment status
- External identity/SSO lifecycle
- Email ingestion
- Approved source/API notification

Outbound examples:

- Tender published/updated
- Submission acknowledged
- Award outcome
- Tenant usage threshold

Webhook rules:

- HMAC/signature verification
- Timestamp/replay window
- Event ID deduplication
- Store raw envelope securely for troubleshooting with retention limits
- Asynchronous processing
- Retry and dead-letter handling

## 15. CORS, CSRF, cookies, and browser access

- Prefer same-site deployment such as `app.example.com` and `api.example.com` with explicit origin policy.
- Credentials require explicit allowlisted origins.
- Do not use `*` origin with cookies or authorization credentials.
- Session cookies: Secure, HttpOnly, appropriate SameSite, bounded lifetime, rotation.
- State-changing cookie-authenticated requests require CSRF protection.
- Preflight responses expose only required methods/headers.
- Browser cannot directly invoke internal worker, AI, database, KMS, or broker endpoints.

## 16. File API design

Upload sequence:

```text
POST initiate → returns upload ID and scoped multipart instructions
client uploads → object remains quarantined
POST complete → verifies parts/hash and queues scan
GET document → shows PROCESSING/QUARANTINED/CLEAN/REJECTED
```

Limits are entitlement- and use-case-aware. File content type is server-detected, not trusted from filename/header.

## 17. Authorization matrix examples

| Operation | Required capability | Additional policy |
|---|---|---|
| Publish tender | `procurement.tender.publish` | Approved version, correct tenant, step-up if configured |
| View financial bid | `evaluation.financial.view` | Opening release, assigned stage, no conflict |
| Submit bid | `tenderpreneur.submission.execute` | Approved final version, deadline open, step-up |
| View pricing | `tenderpreneur.pricing.view` | Workspace scope and classification |
| Suspend tenant | `platform.tenants.suspend` | Platform role, reason, step-up, separation of duty |
| Generate AI draft | Relevant business capability + `ai.use` | Inputs authorized, quota available, use case allowed |

## 18. Contract-first workflow

1. Define operation, actor, rule, and acceptance criterion.
2. Update OpenAPI schema and examples.
3. Review authorization and error matrix.
4. Generate/validate server/client types.
5. Implement database migration/repository.
6. Implement domain/application behavior.
7. Implement API adapter.
8. Add contract/integration tests.
9. Implement frontend using generated/validated client.
10. Record evidence.


---

<!-- SOURCE FILE: 12_FRONTEND_UX_DESIGN.md -->

# TenderOS — Frontend, UX, Navigation, and Visual Design

## 1. Frontend objective

The frontend must enable target users to complete real procurement and tender operations. It must not expose backend implementation details, database identifiers, internal service status, or technical readiness labels as if they were user outcomes.

The downloaded template is a visual and component input. It is not the product source of truth. Routes, information architecture, permissions, fields, workflows, and data contracts are defined by the specifications and backend contracts.

## 2. Frontend stack

- Next.js App Router
- TypeScript with strict mode
- shadcn/ui primitives owned in the repository
- Tailwind CSS with design tokens
- Accessible headless components where required
- Generated or schema-validated API client
- React Hook Form plus schema validation for complex forms
- Server-state query/cache library for interactive client workflows where useful
- Playwright for end-to-end tests
- Testing Library/Vitest or equivalent for component behavior
- Axe-based accessibility checks

## 3. Route architecture

```text
/
├── (public)/
│   ├── page.tsx                         # marketing home
│   ├── tenders/
│   │   ├── page.tsx                     # public catalogue
│   │   └── [publicTenderId]/page.tsx    # authoritative public tender
│   ├── pricing/page.tsx
│   ├── resources/page.tsx
│   ├── contact/page.tsx
│   └── legal/*
├── (auth)/
│   ├── login/page.tsx
│   ├── register/page.tsx
│   ├── verify-email/page.tsx
│   ├── recovery/*
│   └── mfa/*
├── onboarding/
│   ├── workspace-type/page.tsx
│   ├── organization/page.tsx
│   ├── plan/page.tsx
│   ├── provisioning/page.tsx
│   ├── users/page.tsx
│   └── complete/page.tsx
├── platform/
│   ├── page.tsx
│   ├── tenants/*
│   ├── plans/*
│   ├── subscriptions/*
│   ├── usage/*
│   ├── sources/*
│   ├── operations/*
│   ├── support-access/*
│   ├── security/*
│   ├── audit/*
│   └── settings/*
├── procurement/
│   ├── page.tsx
│   ├── plans/*
│   ├── requests/*
│   ├── tenders/*
│   ├── publications/*
│   ├── clarifications/*
│   ├── openings/*
│   ├── evaluations/*
│   ├── awards/*
│   ├── reports/*
│   ├── documents/*
│   ├── users/*
│   └── settings/*
└── tenderpreneur/
    ├── page.tsx
    ├── discover/*
    ├── saved-searches/*
    ├── watchlists/*
    ├── opportunities/*
    ├── bids/*
    ├── compliance-vault/*
    ├── knowledge/*
    ├── submissions/*
    ├── outcomes/*
    ├── reports/*
    ├── users/*
    └── settings/*
```

Routes are permission-aware. Unauthorized routes fail safely; hiding navigation alone is not authorization.

## 4. Portal navigation

## 4.1 Platform Admin

Primary navigation:

- Overview
- Tenants
- Plans & Entitlements
- Subscriptions & Billing
- Usage
- Tender Sources
- Integrations
- Platform Operations
- Support Access
- Security
- Audit
- Country Packs
- Settings

## 4.2 Procurement Team

Primary navigation:

- Overview
- Procurement Plans
- Requests
- Tenders
- Publications
- Clarifications & Addenda
- Bid Openings
- Evaluations
- Awards & Handovers
- Documents & Templates
- Reports
- Team & Roles
- Organization Settings

Role-specific navigation removes irrelevant items. Evaluators may land on assigned evaluations rather than the full procurement dashboard.

## 4.3 Tenderpreneur

Primary navigation:

- Overview
- Discover Tenders
- Saved Searches
- Watchlists
- Opportunities
- Bid Workspaces
- Compliance Vault
- Company Knowledge
- Submissions
- Outcomes
- Reports
- Team & Roles
- Organization Settings

## 5. Page contract standard

Before implementing a page, define:

```text
Page ID and route
Primary actor and outcome
Entry points and back path
Required capability
Backend query/command contracts
Displayed fields and source
Permitted actions and state conditions
Form controls and validation
Loading, empty, error, forbidden, stale, and conflict states
Responsive behavior
Accessibility requirements
Telemetry and audit implications
Acceptance tests
```

No page is accepted without this contract.

## 6. Key Platform Admin pages

### Tenant list

- Filters: type, status, plan, country, cluster, provisioning state, risk
- Columns: tenant, type, plan, status, users, usage summary, schema version, last active
- Actions: view; permitted suspend/reactivate; support access request
- Never show private tender titles, documents, prices, or evaluation details by default

### Tenant detail

Tabs:

- Summary
- Subscription & Usage
- Provisioning & Database
- Integrations
- Security Events
- Support Access
- Platform Audit

### Platform operations

- Service health
- Queue/dead-letter status
- Tenant migrations
- Backup freshness and restore drills
- Source connector health
- AI provider and cost health
- Active incidents and maintenance

## 7. Key Procurement pages

### Procurement dashboard

Show actionable work:

- Requests awaiting action
- Tenders blocked from publication
- Clarifications nearing deadline
- Tenders closing soon
- Opening sessions requiring committee action
- Evaluations assigned/overdue
- Recommendations awaiting approval
- Cycle-time and risk indicators

Avoid a dashboard composed only of decorative cards. Use prioritized work queues, timeline, and concise metrics.

### Tender authoring workspace

Persistent sections:

1. Overview and procurement source
2. Lots and locations
3. Schedule and meetings
4. Eligibility and mandatory requirements
5. Technical criteria
6. Financial response
7. Forms and declarations
8. Submission instructions
9. Terms, security, and validity
10. Documents
11. Review findings
12. Approval and publication

Features:

- Completion/quality indicators tied to deterministic validation
- Inline source/template guidance
- Version and unsaved/stale state
- Review comments anchored to section/field
- AI suggestions visibly separated from approved content

### Evaluation workspace

- Assignment and conflict declaration
- Bid list permitted for stage
- Criterion panel with exact published wording and scale
- Submission evidence viewer beside score/comment area
- Save draft, submit score, lock state
- AI evidence suggestions labeled advisory
- No ability to view prohibited financial envelope during technical stage

## 8. Key Tenderpreneur pages

### Discover Tenders

- Natural-language search bar
- Structured filters
- Source/freshness indicators
- Match and qualification explanation
- Likely gaps and deadline urgency
- Original source link
- Save/follow/ignore/share/create workspace
- Duplicate-source expansion

### External import wizard

Steps:

1. Choose source type
2. Upload/URL/email/manual capture
3. Scan and extraction progress
4. Verify tender facts
5. Review extracted requirements
6. Create workspace and assign owner

Low-confidence items are highlighted and must be verified.

### Bid workspace

Navigation:

- Overview
- Bid/No-Bid
- Requirements
- Tasks
- Technical Response
- Financial Response
- Documents
- Clarifications & Addenda
- Reviews & Approvals
- Readiness
- Submission
- Activity

Overview emphasizes next action, blockers, deadline, readiness, responsible team, and authoritative tender version.

### Compliance matrix

Use a table optimized for work, not generic cards:

- Requirement
- Source
- Type/mandatory/score
- Status
- Owner
- Evidence
- Verification
- Due date
- Risk

Support grouping, bulk assignment, saved filters, keyboard navigation, inline status change where safe, and side-panel details.

### Submission workspace

- Authoritative deadline/timezone
- Current tender/addendum version
- Final response/package version
- Deterministic blockers
- AI audit findings
- Required declarations
- Approval state
- External-channel mapping or platform submit action
- Receipt/evidence after submission

## 9. Forms and input controls

Rules:

- Use searchable selects for related records; never ask a user to type a database ID.
- Use file upload components, not URL text fields, when a file is required.
- Use currency inputs with currency selector/locked tenant default.
- Use date and time controls that show timezone explicitly.
- Use repeatable structured rows for criteria, requirements, staff, and pricing.
- Validate progressively and on submission; preserve user input after server error.
- Explain why a field is required and where it will appear.
- Support autosave only where concurrency/version behavior is safe and visible.

## 10. Visual design system

### Design principles

- Professional, trustworthy, and operational rather than overly decorative
- High information clarity for tables, matrices, documents, and deadlines
- Limited use of cards; use hierarchy, whitespace, sections, tables, timelines, and split panes
- Consistent status semantics with icon + text, not color alone
- Dense and comfortable table modes where useful
- Clear separation of authoritative facts, user input, and AI suggestions

### Tokens

Extract and normalize from the downloaded template:

- Typography scale and font fallbacks
- Spacing and layout grid
- Radius, borders, elevation
- Semantic colors: neutral, information, success, warning, danger, AI/advisory
- Focus, hover, selected, disabled, and high-contrast states
- Table density and form control sizes

Do not retain inaccessible or inconsistent template tokens merely for visual fidelity.

## 11. Component catalogue

Shared components should include:

- Application shell and responsive navigation
- Workspace switcher
- Breadcrumb/back navigation
- Page header with status and actions
- Data table, filters, saved views, pagination, and bulk actions
- Detail description list
- Status badge
- Stepper/workflow timeline
- Approval panel
- Task list and dependency indicator
- Document uploader/viewer/version history
- Requirement/evidence matrix
- Criterion scoring panel
- Money and deadline display
- Notification center
- Audit/activity timeline
- Empty/error/forbidden/conflict states
- AI result panel with citations, confidence, warnings, and disposition controls

## 12. Frontend state management

State categories:

- **Server state:** API query/cache with keys including active tenant and resource version.
- **URL state:** filters, sort, page, selected tab where shareable.
- **Form state:** local form library and schema validation.
- **Session/context:** bootstrap from `/v1/me`, refreshed after role/context/session changes.
- **Ephemeral UI state:** dialogs, panels, selections.

Do not create one global store containing all business data. Clear tenant-scoped caches immediately on context change or logout.

## 13. Error and conflict behavior

- Validation errors appear at field and summary level.
- 401 initiates safe reauthentication without losing permitted draft where possible.
- 403 explains lack of permission without leaking private data.
- 404 handles missing or concealed records.
- 409 version conflict shows who/when changed where available and offers refresh/compare.
- 429 explains quota/rate limit and retry/upgrade options.
- Long-running jobs show progress, source, cancel/retry, and partial/failure state.
- Network errors allow safe retry; non-idempotent operations rely on idempotency key.

## 14. AI interaction design

Every AI panel shall show:

- Use case and scope
- Generated/advisory label
- Sources/citations
- Confidence or verification status where meaningful
- Unsupported claims or missing evidence
- Model job time/version in audit detail, not necessarily main UI
- Accept, edit, reject, regenerate, and report issue actions as permitted

AI text must never visually resemble approved organizational fact until reviewed.

## 15. Responsive and accessibility requirements

- Desktop-first for dense authoring/evaluation, fully usable on tablet
- Mobile supports dashboards, alerts, task updates, document capture, search, approvals, and essential bid actions; highly complex matrices may use optimized stacked/detail patterns
- Keyboard-operable navigation and dialogs
- Visible focus
- Accessible names and descriptions
- Screen-reader announcements for async job/status changes
- Sufficient contrast and non-color status cues
- Logical heading and landmark structure
- Tables include accessible headers and alternative presentation where necessary
- Respect reduced motion

## 16. Frontend security

- Never place secrets or privileged service credentials in browser bundles.
- Treat all server responses and uploaded content as untrusted for rendering.
- Sanitize rich text and document previews.
- Do not store access tokens in localStorage when secure cookie/session architecture is used.
- Clear sensitive cache on logout/context switch.
- Avoid logging financial bids, personal details, documents, or AI prompts to browser analytics.

## 17. Template adoption requirement

Before implementation, execute `20_FRONTEND_TEMPLATE_ADOPTION.md`. The template must be inventoried, licensed, scanned, tokenized, and mapped to the route/page/component catalogue. Placeholder pages and mock data are removed, not renamed and treated as finished functionality.


---

<!-- SOURCE FILE: 13_AI_DEEP_SEARCH_SPEC.md -->

# TenderOS — AI and Deep Search Specification

## 1. Purpose

AI and Deep Search are product accelerators built on top of reliable procurement and bid workflows. They must increase speed, understanding, and evidence quality without becoming an unreviewable decision maker.

## 2. AI principles

1. Source-grounded by default.
2. Tenant- and permission-aware retrieval.
3. Structured output before prose where a business object is required.
4. Human review for all material decisions and generated content.
5. Deterministic controls remain authoritative for deadlines, permissions, calculations, state, and mandatory gates.
6. Model/provider portability through adapters.
7. Quality, cost, latency, and user disposition are measured.
8. Tender documents and web content are untrusted input and cannot override system policy.

## 3. AI use cases

## 3.1 Tenderpreneur AI

- Tender extraction and document map
- Requirement and evaluation-criteria extraction
- Bid/No-Bid assistance
- Company capability and evidence matching
- Compliance gap analysis
- Proposal section drafting and rewriting
- Evidence recommendation
- Contradiction and unsupported-claim detection
- Addendum impact analysis
- Submission audit
- Win/loss and lessons analysis

## 3.2 Procurement AI

- Requisition-to-tender draft assistance
- Specification and requirement quality review
- Evaluation criteria and evidence recommendation
- Date/weight/threshold consistency checks
- Restrictive or ambiguous wording alerts
- Clarification clustering and draft response
- Submission evidence location and summarization
- Side-by-side bid comparison
- Pricing outlier and historical comparison support
- Draft evaluation and award reports

## 3.3 Platform AI

- Source connector anomaly summaries
- Support-ticket triage using non-sensitive metadata
- Usage/cost anomaly detection
- Security/risk alert summarization

Platform AI does not receive private tenant content unless explicit support access and policy permit it.

## 4. AI architecture

```text
Core Go API
  ├─ authorizes use case and inputs
  ├─ checks quota/budget
  ├─ creates AI job
  └─ supplies signed internal references
        │
        ▼
Python AI API / Worker
  ├─ loads authorized source content through service ports
  ├─ parses/OCRs/chunks/classifies
  ├─ retrieves relevant evidence
  ├─ executes versioned workflow
  ├─ validates structured output and citations
  └─ stores result, quality, latency, and cost metadata
        │
        ▼
Model / Embedding / OCR / Search adapters
```

The Python service must not accept a tenant ID from the browser and then query data independently. The core API issues a short-lived, job-scoped authorization token/reference.

## 5. Technology baseline

- Python and FastAPI
- Pydantic for versioned input/output schemas
- LangChain and/or LangGraph for orchestration where they reduce implementation risk
- Direct provider SDK adapters where simpler
- OpenAI as an initial model provider option, with a provider interface for alternatives
- PostgreSQL full-text search and pgvector initially
- Optional OpenSearch at scale
- OCR adapter supporting scanned PDFs/images
- Object storage for original and derived artifacts
- OpenTelemetry and model/job telemetry

LangChain is an orchestration implementation detail, not the domain model or source of business truth.

## 6. Document ingestion pipeline

```text
Upload/import
→ quarantine
→ malware/type/size validation
→ checksum and provenance
→ text extraction
→ OCR for image-only pages
→ page/section structure detection
→ language detection
→ document classification
→ chunking with page/section coordinates
→ embedding/indexing in correct security namespace
→ quality report
→ human verification where required
```

### Ingestion states

- `UPLOADED`
- `QUARANTINED`
- `REJECTED_SECURITY`
- `EXTRACTING`
- `OCR_REQUIRED`
- `INDEXING`
- `READY`
- `READY_WITH_WARNINGS`
- `FAILED_RETRYABLE`
- `FAILED_FINAL`

### Quality metadata

- Pages expected/processed
- Text coverage
- OCR confidence
- Tables/forms detected
- Missing/encrypted/corrupt pages
- Language(s)
- Parser/OCR versions
- Citation resolution quality

## 7. Source citation model

Material outputs use citations such as:

```json
{
  "claim_id": "REQ-17",
  "claim": "A bid security of KES 500,000 is required.",
  "source": {
    "document_id": "...",
    "document_version": 2,
    "page": 18,
    "section": "Tender Data Sheet",
    "bounding_box": [0.10, 0.22, 0.91, 0.31],
    "text_hash": "sha256:..."
  },
  "confidence": 0.96,
  "verification_status": "USER_VERIFIED"
}
```

Page/section references are preferred; line coordinates may be used for extracted text. Citation links must continue to resolve to the exact source version.

## 8. Structured tender extraction

Output schema includes:

- Tender identity and buyer
- Publication/closing/opening/clarification/site-visit dates and timezone
- Lots and locations
- Eligibility and mandatory requirements
- Technical requirements and criteria
- Financial response and pricing instructions
- Score scales, weights, thresholds, pass/fail rules
- Required forms/declarations
- Bid security and validity
- Personnel, experience, financial capacity, certifications
- Submission channel, envelopes, file formats/sizes/names
- Contract terms and high-risk clauses
- Addendum references
- Unknown/ambiguous/conflicting items

Each field includes source, confidence, and verification status. The model must return `unknown` rather than inventing absent values.

## 9. Bid/No-Bid assistance

The system combines deterministic and AI-derived factors:

```text
Eligibility gate
+ mandatory readiness
+ capability match
+ relevant experience
+ certification/licence match
+ staff availability
+ contract value and capacity
+ margin/commercial attractiveness
+ delivery/legal risk
+ competition and strategic fit
+ preparation effort and time remaining
```

Weights are tenant-configurable. AI may summarize evidence and suggest scores; the final assessment and decision are recorded by users.

## 10. Compliance matching

Matching compares tender requirements with:

- Approved organization profile
- Current verified document versions
- Staff and qualifications
- Past projects and references
- Certifications/licences
- Manufacturer authorizations
- Approved standard responses

Output statuses:

- `SATISFIED`
- `LIKELY_SATISFIED_REVIEW_REQUIRED`
- `PARTIAL`
- `MISSING`
- `EXPIRING_OR_INVALID`
- `CONFLICTING`
- `NOT_APPLICABLE_CANDIDATE`

AI does not directly set final requirement status; a permitted user verifies it.

## 11. Proposal generation

Generation inputs:

- Exact tender requirement and evaluation intent
- Approved organization facts and knowledge
- Selected evidence
- User instructions and required tone/format
- Page/word limits
- Prohibited unsupported claims

Output:

- Draft response
- Source references for organization claims
- Open questions/placeholders
- Unsupported-claim warnings
- Suggested evidence
- Compliance coverage notes

Generated content remains draft and requires normal review/approval.

## 12. Procurement tender quality review

Checks include:

- Missing scope/deliverable/acceptance detail
- Ambiguous or non-measurable wording
- Contradictory dates, sections, values, or requirements
- Duplicate requirements
- Criteria not linked to evidence
- Weights/thresholds not reconciling
- Potentially restrictive requirements
- Unreasonable timelines
- Requirements inconsistent with stated procurement method/category
- Missing submission/opening/evaluation rules

The report distinguishes deterministic validation from AI interpretation.

## 13. Evaluation assistance

Permitted AI behavior:

- Locate and quote/summarize relevant submission evidence within copyright/policy limits
- Map evidence to criterion
- Compare submissions using the same published criterion
- Flag missing, contradictory, or unclear evidence
- Identify pricing outliers and arithmetic review candidates
- Draft narrative from human scores/findings

Prohibited behavior:

- Final scoring without human evaluator entry/approval
- Introducing unpublished criteria
- Recommending award based on protected or irrelevant attributes
- Revealing one bidder's confidential content to another
- Accessing financial envelopes during prohibited stages
- Automatically disqualifying or awarding

## 14. Deep Search module

## 14.1 Source types

- Official procurement APIs and feeds
- Government and public-body tender portals
- County, parastatal, university, hospital, and agency websites
- NGO, donor, bank, insurance, and corporate procurement pages
- Approved tender marketplaces/aggregators
- Email feeds and organization subscriptions
- Platform-hosted tender catalogue

Every source is registered with ownership, access method, terms/policy notes, country, categories, frequency, rate limit, parser, authoritative status, and health.

## 14.2 Retrieval pipeline

```text
Schedule/API trigger
→ fetch with rate limit and conditional request
→ preserve raw response/document and metadata
→ parse and normalize
→ extract tender facts
→ canonicalize buyer/category/location/date
→ deduplicate against existing opportunities
→ assign source confidence
→ index full text/metadata/vector
→ detect changes
→ notify affected saved searches/watchlists/workspaces
```

## 14.3 Search query processing

A query may contain natural language and explicit filters.

The search planner extracts:

- Keywords/concepts
- Category/products/services
- Buyer/entity
- Geography
- Value/currency
- publication/closing window
- procurement method
- required certification/experience
- exclusions
- source types

Structured filters remain authoritative; semantic expansion must be explainable and user-adjustable.

## 14.4 Ranking

Suggested normalized score:

```text
0.30 semantic/full-text relevance
0.25 product/service capability fit
0.15 experience/reference fit
0.10 certification/licence fit
0.08 geography eligibility
0.05 value/capacity fit
0.04 deadline feasibility
0.03 source confidence/freshness
```

Weights are configurable and should not imply qualification certainty.

Result explanations include positive and negative factors:

- “Strong match: software implementation and CRM experience.”
- “Potential gap: requires ISO 27001; no current verified certificate found.”
- “Source last checked 2 hours ago; buyer website is authoritative.”

## 14.5 Deduplication

Use deterministic and probabilistic signals:

- Exact reference number + buyer
- Canonical URL/document hash
- Title/buyer/date similarity
- Document similarity
- Lot/source relationship

A canonical opportunity retains all source links and change histories. Human merge/split tools handle uncertain cases.

## 14.6 Change detection

Monitor:

- Closing-date extension
- Addendum/new document
- Clarification publication
- Requirement/document change
- Cancellation/withdrawal
- Source removal or access failure

Workspaces linked to a changed tender receive an impact-analysis job. Readiness may be invalidated until review.

## 15. Deep research

“Research this tender” may gather public information about:

- Procuring entity and official source
- Related notices and addenda
- Similar historical tenders and public awards
- Publicly available organizational context
- Relevant market or price references where lawful and reliable

Output sections are labeled:

- Tender document facts
- Official external facts
- Other public-source findings
- AI interpretation
- Unknown/unverified

## 16. Prompt-injection and tool safety

- Treat all tender/web/document text as untrusted content.
- System policy and tool permissions cannot be modified by document instructions.
- Retrieval content is delimited and labeled.
- Tools use allowlisted operations and job-scoped credentials.
- No arbitrary URL fetch from model output; source-fetch service applies SSRF controls and source policy.
- No arbitrary SQL, shell, file path, email, or submission action from model output.
- Sensitive data egress filters and classification policies apply before model calls.

## 17. AI quality evaluation

Maintain versioned benchmark datasets for:

- Tender field extraction
- Requirement recall/precision
- Criteria/weight/date accuracy
- Citation correctness and coverage
- Compliance matching
- Unsupported-claim rate
- Addendum impact detection
- Evaluation evidence retrieval
- Search relevance and deduplication
- Bias and protected-attribute leakage

Release gates are use-case-specific. Example initial targets to validate and refine:

- Mandatory-requirement recall ≥ 95% on approved benchmark set
- Material fact citation coverage ≥ 98%
- Citation correctness ≥ 95%
- Unsupported material organization claim rate < 1%
- No cross-tenant retrieval in adversarial test suite

These are engineering targets, not public guarantees until measured in production-representative data.

## 18. Cost and latency controls

- Per-plan AI quotas and monetary budgets
- Input page/token limits and user confirmation for large jobs
- Model routing by complexity and data classification
- Cache deterministic extraction/artifacts by content hash and workflow version
- Batch embeddings and incremental reindexing
- Cancel stale/superseded jobs
- Track cost by tenant, use case, document, model, and release
- Queue priorities for interactive, deadline-critical, and batch work

## 19. AI audit record

Record:

- Tenant, user, use case, and authorized input references
- Prompt/workflow/model/tool/schema versions
- Provider and region/configuration where relevant
- Start/end, latency, retries, token/usage/cost
- Structured output, citations, validation, and safety findings
- User accepted/edited/rejected disposition
- Downstream record/version created from accepted output

Raw sensitive prompts/responses require controlled retention and access; do not log them indiscriminately.


---

<!-- SOURCE FILE: 14_SECURITY_PRIVACY_COMPLIANCE.md -->

# TenderOS — Security, Privacy, and Compliance Specification

## 1. Security objectives

TenderOS handles confidential procurement plans, supplier records, technical proposals, pricing, evaluation data, identities, and personal information. Security is a product requirement, not a deployment afterthought.

Objectives:

- Prevent cross-tenant disclosure or modification.
- Protect sealed submissions from premature access or tampering.
- Enforce least privilege and separation of duty.
- Preserve trustworthy audit and evidence.
- Protect personal and confidential data throughout its lifecycle.
- Resist malicious files, prompt injection, account takeover, API abuse, and insider misuse.
- Recover from failure, compromise, or data loss with tested procedures.

## 2. Governance baseline

The product must be configured and legally reviewed for each deployment context. For Kenya, the governance baseline should consider, at minimum:

- The Constitution's principles for public contracting where applicable
- Public Procurement and Asset Disposal Act, 2015 and applicable regulations/policies
- Kenya Data Protection Act, 2019 and applicable regulations/guidance
- Electronic transactions, evidence, records-retention, tax, sector, donor, and organizational requirements as applicable

TenderOS must not market itself as automatically guaranteeing legal compliance. It provides configurable controls and evidence; the customer remains responsible for approved procurement policy and lawful use.

## 3. Security roles and accountability

- Platform security owner
- Product/data protection owner
- Infrastructure/operations owner
- Tenant organization administrator
- Procurement security/governance owner
- Incident commander
- Security reviewer for sealed-submission design
- Data controller/processor contacts defined contractually

Privileged access is reviewed regularly. No shared administrator accounts are permitted.

## 4. Threat model

Priority threats:

1. Cross-tenant database, storage, search, or AI leakage
2. Broken object-level authorization/IDOR
3. Premature or unauthorized access to sealed bids
4. Submission tampering, replay, replacement, or timestamp dispute
5. Account takeover and session theft
6. Privilege escalation and separation-of-duty bypass
7. Malicious uploads, archives, macros, scripts, or polyglot files
8. Injection: SQL, command, template, HTML, spreadsheet, email, and log injection
9. SSRF through URL import or AI tools
10. Prompt injection and retrieval poisoning
11. Unauthorized model-provider data disclosure
12. Insider misuse by platform or tenant administrators
13. Source-connector abuse, scraping violations, or poisoned external data
14. DDoS, deadline-time submission spikes, and resource exhaustion
15. Secret exposure and supply-chain compromise
16. Audit deletion or manipulation
17. Backup exposure or failed recovery
18. Privacy overcollection and excessive retention

A formal threat model must be updated for each major release and before the sealed-submission production launch.

## 5. Authentication

Requirements:

- Verified email/identity before privileged workspace access
- Memory-hard password hashing when password login is enabled
- MFA support; mandatory for platform administrators and configurable high-risk tenant roles
- Session rotation after login, privilege change, MFA, password reset, and step-up
- Short bounded idle and absolute lifetimes according to risk
- Device/session list and revocation
- Login throttling, credential-stuffing detection, and risk alerts
- Recovery with single-use, expiring challenges and anti-enumeration responses
- Enterprise OIDC/SAML through approved adapters

Sensitive actions requiring step-up may include:

- Platform support access
- Tenant suspension/decommission
- Final bid submission
- Bid opening/key release
- Evaluation unlock
- Award approval
- Billing or bank/payment detail changes
- Bulk export
- Secret/key rotation

## 6. Authorization

Use default-deny RBAC + ABAC.

Authorization dimensions:

- Platform versus tenant boundary
- Tenant type
- Membership status
- Role and explicit grant/deny
- Department/team
- Resource ownership/assignment
- Tender, workflow, and evaluation stage
- Data classification
- Separation-of-duty policy
- Conflict-of-interest status
- Session assurance

Rules:

- Every API operation authorizes server-side.
- List/search queries are permission-filtered before results and counts are returned.
- Authorization failures do not reveal private record existence.
- Role administration enforces grantable-role boundaries and last-admin rules.
- Permission changes revoke or refresh affected sessions.
- AI and export jobs repeat authorization at execution time.

## 7. Tenant isolation

### Control requirements

- One private database per tenant
- Tenant DB chosen from authenticated active context
- Separate database roles/credentials and storage namespace
- No client-controlled database locator
- No cross-tenant joins or shared tenant tables for private business data
- Tenant-scoped cache keys and job queues
- Private search/embedding namespaces
- Tenant-aware logs and metrics without content exposure
- Automated isolation tests at API, repository, database router, object storage, search, events, exports, and AI layers

### Failure behavior

When tenant context is missing, stale, suspended, mismatched, or unavailable, fail closed. Never fall back to a default tenant database.

## 8. Database security

- TLS database connections
- Least-privileged roles: migrator, application read/write, read-only/reporting, backup
- Secrets in secrets manager, not environment files committed to source or control tables
- Network restrictions and private endpoints
- Bounded statement and transaction timeouts
- Parameterized SQL only
- Migration review and signed/reproducible artifacts
- Audit for privilege and schema changes
- Encryption at rest and encrypted backups
- Restore into isolated target before production cutover

Database-per-tenant does not eliminate application authorization; both are required.

## 9. Object and document security

- Quarantine all uploads
- Detect type from content; reject disallowed/ambiguous formats
- Size, archive depth, page, and decompression limits
- Malware scanning and optional content disarm/reconstruction
- Strip or block active content where appropriate
- Checksum every object/version
- Store using opaque server-generated keys
- Encrypt at rest; use tenant/exchange-specific keys or key context
- Short-lived signed URLs with exact object/action scope
- Prevent public bucket access except intentionally published tender artifacts
- Sanitize HTML/rich text/previews
- Protect against spreadsheet formula injection in exports
- Immutable version links for approved submissions/evidence

## 10. Sealed-bid security

Sealed submissions require controls beyond ordinary permissions.

### Required properties

- Confidentiality before opening
- Integrity and tamper evidence
- Authoritative server timestamp and acknowledgement
- Non-repudiation evidence appropriate to the legal/business context
- Controlled withdrawal/replacement history
- Key separation and release policy
- Opening quorum/role/time/conflict controls
- Complete release/opening audit

### Design requirements

- Encrypt submission package with a unique data-encryption key.
- Encrypt/wrap the data key using KMS/HSM-backed keys and opening policy.
- The ordinary procurement API runtime must not possess a standing pre-opening decryption path.
- Opening release requires authoritative tender state and approved committee conditions.
- Hash manifest and every file/structured response.
- Sign or otherwise protect receipt and opening evidence.
- Monitor and alert on attempted premature access or key operations.
- Subject cryptographic design, KMS policy, and incident procedures to independent review and penetration testing.

## 11. API and browser security

- TLS and HSTS
- Explicit production CORS allowlist
- Secure, HttpOnly, appropriate SameSite cookies
- CSRF controls for cookie-authenticated writes
- CSP, frame-ancestors, referrer policy, MIME sniffing protection, and permissions policy
- Input validation, canonicalization, and output encoding
- Rate limits by IP, identity, tenant, operation, and risk
- Request body/time limits
- Idempotency and replay protection for critical writes
- SSRF-safe URL fetcher with DNS/IP validation, redirect limits, protocol allowlist, egress policy, and size/time limits
- Safe file names and `Content-Disposition`
- No sensitive information in URLs

## 12. AI and model security

- Separate system policy from untrusted content.
- Treat documents/web pages as data, never instructions.
- Use allowlisted tools with job-scoped credentials.
- Authorize retrieval before model input.
- Apply data-classification rules to provider/model selection.
- Configure provider data retention/training controls contractually and technically.
- Redact/minimize personal and confidential data where not needed.
- Detect prompt injection, data-exfiltration attempts, and malicious links.
- No arbitrary code, shell, SQL, email, network, publication, submission, or award tools.
- Validate structured output and citations.
- Log model/job metadata without indiscriminate raw sensitive content.
- Red-team cross-tenant retrieval, prompt injection, tool abuse, and poisoned knowledge.

## 13. Privacy engineering

### Privacy principles

- Purpose limitation
- Data minimization
- Lawful and transparent processing
- Accuracy and user correction
- Access control and confidentiality
- Retention limitation
- Data-subject rights support
- Processor/subprocessor governance
- Privacy by design/default

### Data inventory and classification

Classify at least:

- Public tender content
- Internal business content
- Confidential procurement content
- Sealed bid content
- Commercially sensitive pricing
- Personal data
- Special/sensitive personal data where present
- Authentication secrets
- Audit/security data

### Required capabilities

- Privacy notice and consent/acknowledgement where appropriate
- Controller/processor configuration and contractual records
- Data export/access request workflow
- Correction and deletion/anonymization workflow subject to lawful retention
- Legal hold and retention schedule
- Breach assessment and notification workflow
- Subprocessor registry
- Cross-border processing and hosting assessment
- Data Protection Impact Assessment for high-risk features, especially AI, profiling, sealed bids, and large-scale supplier data

## 14. Audit and evidence integrity

Audit records include:

- Actor and effective role/context
- Time and authoritative server clock
- Action and target
- Previous/new state or change summary
- Reason where required
- IP/device/session metadata proportionate to need
- Correlation and causation IDs
- Outcome and error code
- Support-access or step-up reference

Audit requirements:

- Append-only application behavior
- Restricted database permissions
- Tamper-evident export/manifest for critical procedures
- Separate retention from diagnostic logs
- Search/export permissions
- Clock synchronization monitoring
- No secret or unnecessary document content in audit payloads

## 15. Logging and monitoring

- Structured logs with classification-aware redaction
- No passwords, access/refresh tokens, MFA secrets, full bid content, raw documents, or unnecessary personal data
- Detection for repeated authorization failures, tenant mismatch, premature bid access, unusual downloads/exports, role changes, connector abuse, AI exfiltration patterns, and payment abuse
- Central alerting and defined severity/ownership
- Security event retention aligned with investigation needs

## 16. Secure SDLC

Required pipeline controls:

- Code review and protected branches
- Static analysis and linting
- Dependency and vulnerability scanning
- Secret scanning
- Container/image scanning
- Infrastructure-as-code scanning
- SBOM generation
- Signed build artifacts/images where feasible
- License review
- Unit/integration/authorization/security tests
- DAST against staging
- Independent VAPT before major production milestones
- Dependency update policy and emergency patch process

No dependency is introduced solely because an AI agent generated it. New dependencies require purpose, owner, license, maintenance, and security review.

## 17. Incident response

Incident classes:

- Account compromise
- Cross-tenant exposure
- Sealed-bid confidentiality/integrity incident
- Malicious upload
- AI/data exfiltration
- Source-connector compromise
- Service outage/data loss
- Payment/billing incident
- Privacy breach

Runbook lifecycle:

```text
Detect → Triage → Contain → Preserve evidence → Eradicate
→ Recover → Notify/escalate as required → Review → Remediate
```

Critical actions such as tenant isolation, token revocation, connector pause, model-provider disablement, and key rotation must be operable without a full application release.

## 18. Business continuity and disaster recovery

- Document dependencies and failure modes
- Multi-zone/high-availability production design as justified
- Encrypted backups and point-in-time recovery
- Restore drills for control, exchange, and sampled tenant databases
- Object storage version recovery
- Message/outbox reconciliation after restore
- KMS/key recovery and escrow policy
- Defined RPO/RTO and crisis communications
- Manual fallback for AI/search outages
- Deadline-sensitive incident procedure for submissions and opening

## 19. Security acceptance gates

Before production:

- Threat model approved
- Tenant isolation suite passes
- Authorization matrix tests pass
- Sealed-bid design independently reviewed
- VAPT critical/high findings remediated or formally accepted
- Backup and restore drill passes
- Incident runbooks exercised
- Data inventory, retention, privacy notice, and processor agreements reviewed
- AI red-team and provider configuration reviewed
- Support access, export, and privileged role controls tested

## 20. Reference baseline

Authoritative legal and regulatory interpretation must come from qualified counsel and official Kenyan sources, including Kenya Law, the Public Procurement Regulatory Authority/National Treasury, and the Office of the Data Protection Commissioner. This specification intentionally avoids hardcoding legal section interpretations into product logic without review.


---

<!-- SOURCE FILE: 15_QA_TEST_STRATEGY.md -->

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


---

<!-- SOURCE FILE: 16_PRODUCT_ROADMAP.md -->

# TenderOS — Product Roadmap

## 1. Roadmap intent

This roadmap sequences product outcomes rather than isolated features. It prioritizes:

1. Trustworthy SaaS and tenant foundations
2. Independent value for Tenderpreneurs using external tenders
3. Procurement authoring and publication
4. Secure closed-loop submission, evaluation, and award
5. AI acceleration after deterministic workflows exist
6. Scale, internationalization, and enterprise readiness

Dates should be set after team capacity, template quality, integration availability, legal review, and pilot partners are confirmed. Release exit criteria are stronger than calendar dates.

## 2. Product horizons

```text
H0 Define and de-risk
H1 Establish SaaS foundation
H2 Launch Tenderpreneur OS for external tenders
H3 Add Deep Search and grounded AI analysis
H4 Launch Procurement OS and public catalogue
H5 Connect secure submission, opening, evaluation, and award
H6 Add advanced AI and intelligence
H7 Scale, internationalize, and expand integrations
```

## 3. Release R0 — Product and architecture baseline

### Outcome

The product, three portals, operating models, tenancy, exchange plane, legal assumptions, data ownership, and delivery gates are unambiguous.

### Capabilities

- Approved BRD, PRD, SRD, HLD, LLD, data/API/frontend designs
- Personas, journeys, business rules, module dependencies
- Frontend template audit plan
- Threat model and privacy/data inventory baseline
- Architecture decisions and proof-of-concept backlog
- Initial pilot-customer discovery plan

### Exit criteria

- No unresolved contradiction about portal ownership or tenant model
- Database-per-tenant and sealed-submission approach accepted for implementation
- MVP boundaries and metrics approved

## 4. Release R1 — SaaS control plane and trusted workspace foundation

### Outcome

A user can register, create a Procurement or Tenderpreneur workspace, receive a private database, invite users, and access a permission-controlled portal.

### Capabilities

- Authentication, verification, session, recovery, MFA foundation
- `/me`, memberships, context selection, capabilities
- Tenant provisioning and private database routing
- Platform Admin tenant and plan management
- Subscription/entitlement foundation
- Organization profile, teams, roles, invitations
- Audit, events, notifications, tasks, approvals, document storage
- Local/CI/staging environments and observability

### Exit criteria

- Multiple tenants provisioned automatically
- Cross-tenant isolation suite passes
- Context switching and session revocation pass
- Backup/restore proof for sample tenant

## 5. Release R2 — Tenderpreneur OS: external tender execution MVP

### Outcome

A Tenderpreneur can manage any external tender from import through a recorded submission.

### Capabilities

- Tenderpreneur onboarding and company capability profile
- Compliance/document vault with versions and expiry
- External tender upload, URL, text, email/manual import
- User-verified tender facts
- Bid/No-Bid workflow
- Requirements/compliance matrix
- Team tasks and dependencies
- Technical and restricted financial response workspaces
- Review and approval workflows
- Deterministic readiness and package generation
- External submission instructions and evidence register
- Outcome and lessons capture

### Exit criteria

A pilot Tenderpreneur completes an external tender journey without spreadsheets, mock APIs, or manual database intervention.

### Commercial value

TenderOS can begin selling to Tenderpreneurs before procurement organizations publish through the platform.

## 6. Release R3 — Deep Search and AI tender understanding

### Outcome

Tenderpreneurs can discover suitable opportunities and understand long tender documents faster, with visible source provenance.

### Capabilities

- Source registry and initial official/approved connectors
- Public tender search and normalization
- Deduplication, freshness, and authoritative-source indicators
- Natural-language and structured search
- Saved searches, watchlists, and alerts
- Business-profile matching and explainable fit factors
- AI ingestion, OCR, extraction, citations, and verification
- AI Bid/No-Bid and compliance suggestions
- Addendum/change monitoring and workspace impact

### Exit criteria

- Search relevance, deduplication, and source freshness meet pilot targets
- Mandatory-requirement extraction/citation benchmarks pass
- Users can move from search result to verified bid workspace

## 7. Release R4 — Procurement OS: planning, authoring, and publication

### Outcome

A Procurement Team can create and publish a controlled tender inside TenderOS.

### Capabilities

- Procurement plans and requisitions
- Budget/method/approval configuration
- Tender authoring: lots, requirements, criteria, forms, schedule, submission rules
- Deterministic quality validation
- Review, change request, and approval
- Public/restricted/invited publication
- Public catalogue pages and supplier participation
- Clarifications, meetings, addenda, acknowledgement, and closing

### Exit criteria

A pilot Procurement Team publishes an approved tender whose public version reconciles exactly with the authorized tenant version.

## 8. Release R5 — Secure marketplace transaction

### Outcome

Tenderpreneurs can submit sealed bids to platform-hosted tenders and Procurement Teams can open them under controlled conditions.

### Capabilities

- Structured platform response forms
- Final submission validation and step-up
- Encrypted sealed envelope and signed/tamper-evident receipt
- Deadline enforcement
- Withdrawal and replacement
- Metadata-only procurement receipt view
- Opening committee, quorum, declarations, key release, integrity verification, and minutes
- Deadline-load resilience and incident runbooks

### Exit criteria

- Independent security review completed
- No procurement-side pre-opening access path
- Submission/opening load, replay, tamper, and recovery tests pass
- Pilot parties complete submission and opening with evidence

## 9. Release R6 — Evaluation, award, and handover

### Outcome

The platform supports the complete closed tender lifecycle.

### Capabilities

- Preliminary compliance evaluation
- Technical and financial evaluation
- Evaluator assignment and separation of duty
- Evidence-linked scores/findings
- Clarifications and arithmetic correction
- Locking, consolidation, moderation, and reports
- Recommendation, reviews, approval, award, notices, publication, and handover
- Procurement and Tenderpreneur outcome dashboards

### Exit criteria

A real or controlled pilot tender progresses from publication to approved outcome with complete audit evidence.

## 10. Release R7 — Advanced AI for Procurement Teams and Tenderpreneurs

### Outcome

AI measurably improves cycle time and quality across both sides while remaining reviewable and safe.

### Capabilities

- Procurement tender drafting and requirement-quality review
- Criteria/evidence suggestions
- Clarification clustering and draft responses
- Evaluation evidence locator and bid comparison
- Price/outlier intelligence
- Draft evaluation and award reports
- Tenderpreneur proposal drafting and evidence matching
- Submission audit and contradiction detection
- AI evaluation dashboards, model routing, cost controls, feedback loops

### Exit criteria

- Approved AI benchmarks pass
- Material AI claims are source-linked
- Human decisions and edits are visible
- No high-risk transition is autonomous
- Pilot shows measurable time/quality benefit

## 11. Release R8 — Intelligence, enterprise, and regional scale

### Outcome

TenderOS supports larger customers, integrations, multiple countries, and defensible intelligence.

### Capabilities

- Advanced analytics and benchmark reporting
- Historical tender/award and buyer intelligence where lawful
- SSO, SCIM or directory provisioning, APIs, webhooks
- ERP, contract management, e-signature, email, payment, and official procurement-source integrations
- Dedicated tenant clusters and plan-specific RPO/RTO
- Country packs, localization, currency/tax/timezone extensions
- Kiswahili and other languages
- Data migration/import tools
- Advanced retention, legal hold, and audit exports

### Exit criteria

- Enterprise security and operational review passes
- First additional country/country pack implemented without changing core domain rules
- Scale targets and tenant fleet operations demonstrated

## 12. Cross-release workstreams

These do not wait until the final release:

- Security and privacy engineering
- Accessibility and responsive design
- Observability and cost management
- Tenant migration/backup/restore automation
- Documentation and support content
- AI and search evaluation datasets
- Customer research and UAT
- Legal/procurement policy review
- Data quality and country-pack governance

## 13. Product metrics by horizon

### Foundation

- Tenant provisioning success/time
- Cross-tenant isolation defects
- Onboarding completion
- Time to first operation

### Tenderpreneur MVP

- External imports
- Bid decision completion
- Requirements completion
- Submission evidence completion
- Preventable blocker detection

### Search/AI

- Search-to-workspace conversion
- Relevant result precision
- Extraction/citation quality
- AI acceptance/edit/rejection
- AI cost per completed operation

### Procurement/marketplace

- Tender publication cycle time
- Bidders per tender
- Successful sealed submissions
- Opening/evaluation cycle time
- Audit exceptions

### Commercial

- Trial-to-paid conversion
- Monthly active tenants
- Retention and expansion
- Gross margin after AI/search/infrastructure cost
- Support burden per tenant

## 14. Roadmap governance

At each release boundary:

1. Reassess market and pilot evidence.
2. Review open legal/security decisions.
3. Reconcile requirements and architecture.
4. Confirm module dependencies and capacity.
5. Freeze release outcomes, not all later details.
6. Define measurable exit criteria.
7. Do not move incomplete critical workflows into a later release under a “ready” label.


---

<!-- SOURCE FILE: 17_DELIVERY_PLAN_WAVES_SPRINTS.md -->

# TenderOS — Product Delivery Plan: Waves and Sprints

## 1. Delivery model

- Default sprint length: **2 weeks**
- A wave contains one or more sprints and ends with an integrated, demonstrable operational outcome.
- Sequence is dependency-driven, not date-driven.
- Within every module/wave, delivery follows:

```text
Business operation and contract
→ Database/migrations/seed/reference data
→ Backend domain/application/API/security/events
→ Frontend routes/pages/forms/navigation/states
→ Integration/QA/security/performance/UAT evidence
```

The plan contains 17 waves and 33 indicative sprints. Calendar duration depends on team size, parallel work, template quality, infrastructure choice, and pilot feedback. A smaller team should preserve the sequence and reduce concurrent scope rather than skip quality gates.

## 2. Sprint operating rules

### Before a sprint

A ticket must have:

- Requirement IDs and persona outcome
- Dependencies
- Business rules and state transitions
- Data model/migration impact
- API/event contract
- Authorization matrix entry
- Page/route contract
- Acceptance tests and evidence

### During a sprint

- Database work merges before dependent backend completion.
- Backend contract and business logic pass integration tests before frontend is accepted.
- Frontend integrates real APIs; mocks are only temporary test fixtures.
- Reviewers assess product correctness, not only code style.
- Incomplete work is carried with explicit state; it is not marked done because a screen renders.

### Sprint review

Demonstrate the user operation in the target environment with real data and permissions. Capture evidence.

## 3. Wave overview

| Wave | Sprints | Outcome |
|---|---:|---|
| W00 | S00–S01 | Product, architecture, template, and delivery baseline |
| W01 | S02 | Repository, CI/CD, local platform, observability skeleton |
| W02 | S03–S04 | Control plane and private tenant database provisioning |
| W03 | S05–S06 | Authentication, memberships, context, and authorization |
| W04 | S07–S08 | Documents, audit, workflow, tasks, notifications, billing foundation |
| W05 | S09–S10 | Tenderpreneur onboarding and compliance vault |
| W06 | S11–S12 | External tender import and deterministic opportunity workspace |
| W07 | S13–S14 | Bid workspace, compliance, proposal, readiness, external submission evidence |
| W08 | S15–S16 | Deep Search baseline, saved searches, watchlists, and alerts |
| W09 | S17–S18 | AI platform, OCR/extraction, citations, Bid/No-Bid assistance |
| W10 | S19–S20 | Procurement planning, requisition, and tender authoring |
| W11 | S21–S22 | Review, approval, publication, catalogue, clarifications, addenda |
| W12 | S23–S24 | Platform submissions, sealing, receipts, withdrawal/replacement |
| W13 | S25–S26 | Bid opening and integrity evidence |
| W14 | S27–S29 | Preliminary, technical, financial evaluation and award |
| W15 | S30–S31 | Procurement AI, advanced Tenderpreneur AI, and intelligence |
| W16 | S32 | Internationalization, enterprise hardening, pilot and launch gate |

## 4. W00 — Product and design baseline

### S00 — Product model and requirements

**Database/design**

- Define control, tenant, exchange, search, and object-storage ownership.
- Define tenant types and country-pack model.

**Backend/API design**

- Define bounded contexts, state machines, API standards, error envelope, events, authorization vocabulary.

**Frontend/design**

- Define route groups, portal navigation, personas, page-contract template, visual principles.

**QA/evidence**

- Requirement IDs, traceability template, risk register, initial UAT scenarios.

**Exit:** BRD/PRD/SRD accepted.

### S01 — Architecture spikes and template audit

- Prove dynamic tenant database routing and bounded pool management.
- Prove outbox/inbox and exchange projection.
- Prototype sealed-envelope/key-release approach without production claims.
- Materialize/audit downloaded frontend template: license, dependencies, components, responsiveness, accessibility, design tokens.
- Select deployment, message, storage, identity, and payment adapters through ADRs.

**Exit:** High-risk architecture decisions have evidence or explicit follow-up.

## 5. W01 — Engineering platform

### S02 — Repository and environments

**Database**

- Local control, exchange, and sample tenant databases.
- Migration runners and schema-version tables.

**Backend**

- Go modules, API/worker skeleton, health, config, structured errors/logging.
- Python AI API/worker skeleton.

**Frontend**

- Next.js application, template-derived design tokens/primitives, public/auth shells.

**Operations/QA**

- Docker Compose, CI lint/test/build/scan, testcontainers, OpenTelemetry skeleton, seed data.

**Exit:** Clean checkout starts and tests all deployables with one documented workflow.

## 6. W02 — Control plane and tenant provisioning

### S03 — Control database and Platform Admin tenant registry

**Database first**

- Control schemas: tenant registry, clusters, provisioning jobs, plans, feature flags, platform audit.
- Seed tenant types and initial country pack.

**Backend**

- Tenant registry/admin APIs, provisioning command model, cluster selection, audit.

**Frontend**

- Platform Admin shell, tenant list/detail, provisioning status, platform settings baseline.

**QA**

- Admin authorization, error states, schema/migration tests.

### S04 — Private database provisioning and routing

**Database**

- Common tenant schema baseline and database roles.
- Exchange schema baseline.

**Backend**

- Provision database/roles/secrets/storage, migrate/seed, activate/suspend.
- Tenant DB router, pool limits/eviction, health, schema compatibility.

**Frontend**

- Onboarding provisioning progress, failure/retry/support path.

**QA**

- Multi-tenant create/read/write isolation, failed provisioning resume, restore-to-new proof.

**Wave exit:** Two Procurement and two Tenderpreneur tenants provision and remain isolated.

## 7. W03 — Identity, memberships, context, and authorization

### S05 — Authentication and session lifecycle

**Database**

- Identities, emails, credentials, sessions, recovery, MFA state.

**Backend**

- Register, verify, login, logout, recovery, rotation, revocation, stable 401 semantics.

**Frontend**

- Auth routes/forms, verification, recovery, session-expiry handling.

**QA**

- Enumeration resistance, rotation, revocation, rate limiting, browser security.

### S06 — Memberships, context, roles, capabilities

**Database**

- Memberships, invitations, role grants, tenant user projection.

**Backend**

- `/v1/me`, membership enumeration, context selection, effective capabilities, 403 semantics, authorization middleware.

**Frontend**

- Workspace switcher, invitation acceptance, team/role pages, route/nav filtering.

**QA**

- Full role/context matrix, cross-tenant IDs, cache clearing, last-admin rule.

**Wave exit:** Authenticated context is production-ready for downstream modules.

## 8. W04 — Shared operating foundation

### S07 — Documents, audit, and eventing

**Database**

- Documents/versions/files, audit, outbox/inbox, idempotency.

**Backend**

- Upload lifecycle, object storage adapter, scan state, download authorization, audit and event dispatch.

**Frontend**

- File uploader, document list/detail/versions, processing/quarantine/error states, activity timeline.

**QA**

- Malware fixtures, type/size limits, signed URLs, object isolation, event replay.

### S08 — Workflows, tasks, notifications, subscription foundation

**Database**

- Workflow templates/instances, approvals, tasks/dependencies, notification outbox, plans/subscriptions/usage.

**Backend**

- Serial/parallel approvals, task operations, notifications, entitlement checks, usage ledger.

**Frontend**

- Pending approvals, task boards/lists, comments, notifications, plan/usage pages.

**QA**

- Delegation/rework/expiry, notification retry, quota race/idempotency.

**Wave exit:** All later modules can reuse governed documents, approvals, tasks, audit, and entitlements.

## 9. W05 — Tenderpreneur onboarding and compliance vault

### S09 — Business profile and capability records

**Database**

- Products/services, sectors, regions, certifications, staff, projects, references.

**Backend**

- CRUD through domain use cases, validation, classifications, permissions.

**Frontend**

- Onboarding wizard and profile pages with correct structured controls.

**QA**

- Permission, validation, completeness, responsive/accessibility tests.

### S10 — Compliance vault and expiry readiness

**Database**

- Document categories, verification, expiry rules, knowledge versions/approvals, usage links.

**Backend**

- Vault operations, version/verification/expiry, reusable knowledge approval, readiness query.

**Frontend**

- Vault list/detail/upload, expiry calendar/alerts, knowledge approval.

**QA**

- Expired/superseded evidence, financial-document restriction, AI-access eligibility.

**Wave exit:** Tenderpreneur maintains trustworthy reusable evidence.

## 10. W06 — External tender import and deterministic workspace

### S11 — Import pipeline without AI dependency

**Database**

- Opportunities, source/provenance, imports, fact fields, verification.

**Backend**

- File, URL, text, manual import; safe fetch; duplicate candidate; basic metadata capture.

**Frontend**

- Import wizard, upload/fetch progress, manual facts, duplicate resolution.

**QA**

- SSRF, inaccessible URLs, encrypted/corrupt files, duplicate and provenance tests.

### S12 — Opportunity verification and workspace creation

**Database**

- Opportunity version, tender snapshot, workspace linkage.

**Backend**

- Fact verification, create workspace, deadlines/source changes, activity.

**Frontend**

- Opportunity detail, verify facts, source viewer, create/assign workspace.

**QA**

- Low-confidence/manual corrections and immutable source/version links.

**Wave exit:** Any tender can become a governed workspace even with AI disabled.

## 11. W07 — Bid execution MVP

### S13 — Bid/No-Bid, requirements, tasks, and evidence

**Database**

- Bid assessments/decisions, requirements, assignments, evidence, risks.

**Backend**

- Weighted assessment, human decision, requirement transitions, evidence validation, dependencies.

**Frontend**

- Bid/No-Bid page, compliance matrix, side-panel detail, bulk assignment, task views.

**QA**

- Mandatory gate, evidence version, role and concurrency tests.

### S14 — Proposal, pricing, approvals, readiness, external evidence

**Database**

- Response versions, financial lines, reviews, readiness, packages, external evidence/outcomes.

**Backend**

- Technical/financial response, restricted pricing, approvals, deterministic readiness, package manifest, submitted-evidence rule.

**Frontend**

- Response editor, pricing workspace, review/approval, readiness and submission pages, evidence capture.

**QA/UAT**

- Full external tender journey; no mock data; version invalidation and access controls.

**Wave exit / MVP-1:** A Tenderpreneur can execute an external tender end to end.

## 12. W08 — Deep Search baseline

### S15 — Source registry connectors and public search index

**Database**

- Source documents/fetches/canonical opportunities/source links/change events.

**Backend/search workers**

- Initial approved connectors, scheduler, conditional fetch, parse, normalize, deduplicate, index.

**Frontend**

- Discover page, filters, source/freshness display, original source.

**QA**

- Connector fixtures, rate limit, parsing, authoritative-source and dedup tests.

### S16 — Matching, saved searches, watchlists, and alerts

**Database**

- Search preferences, saved searches, watch targets, result/match factors, alerts.

**Backend**

- Hybrid search baseline, explainable match factors, saved execution, change alerts, workspace conversion.

**Frontend**

- Natural-language search, saved searches, watchlists, match/gap explanation.

**QA/UAT**

- Relevance benchmark, source freshness, alert correctness, search-to-workspace journey.

**Wave exit / MVP-2:** Tenderpreneurs discover and import traceable opportunities.

## 13. W09 — AI foundation and Tenderpreneur analysis

### S17 — AI jobs, ingestion, OCR, retrieval, citations

**Database**

- AI jobs/inputs/outputs/citations/usage, chunks/embeddings, prompt/model registry.

**Backend/AI**

- Job authorization, provider adapters, extraction/OCR, chunking, retrieval, structured output/citation validation.

**Frontend**

- AI job progress, result/source viewer, warnings, accept/edit/reject.

**QA**

- Benchmark dataset, prompt-injection, cross-tenant retrieval, provider outage, cost metrics.

### S18 — Tender extraction, Bid/No-Bid and compliance assistance

**Backend/AI**

- Structured tender extraction, requirement proposals, fit and evidence matching, addendum impact.

**Frontend**

- Verify extracted facts/requirements, AI assessment, suggested evidence, source navigation.

**QA/UAT**

- Mandatory recall, citation correctness, unsupported-claim, human review behavior.

**Wave exit:** AI accelerates but does not own the external tender workflow.

## 14. W10 — Procurement planning and authoring

### S19 — Plans, requests, budgets, methods, approvals

**Database**

- Procurement plans/items, requests/versions, budget confirmations, method selections.

**Backend**

- Request lifecycle, approval, conversion.

**Frontend**

- Plan/request lists, forms, approval pages, actionable dashboard queues.

**QA/UAT**

- Requester-to-approved-request journey and separation of duty.

### S20 — Tender draft, lots, requirements, criteria, schema

**Database**

- Tender/versions/lots/schedules/requirements/criteria/forms/submission fields.

**Backend**

- Domain invariants, validation, versioning, review readiness.

**Frontend**

- Tender authoring workspace with all sections, correct controls, review findings.

**QA**

- Date/weight/threshold and version/concurrency tests.

**Wave exit:** Procurement Team produces a valid tender candidate.

## 15. W11 — Approval, publication, clarification, and addenda

### S21 — Tender review, approval, and publication

**Database**

- Review findings, publication intents/deliveries, public exchange projections.

**Backend**

- Version-specific review/approval, publish outbox, projection/hash reconciliation, catalogue APIs.

**Frontend**

- Review/approval workspace, publication settings, public tender page.

**QA**

- Approval invalidation, publication failure/retry, public/private data leakage tests.

### S22 — Clarifications, addenda, invitations, and closing

**Database**

- Questions/responses/addenda/change records/acknowledgements/invitations.

**Backend**

- Clarification privacy, addendum version, impact events, deadline extension, auto-close.

**Frontend**

- Procurement clarification workspace, Tenderpreneur/public questions, addendum acknowledgement.

**QA/UAT**

- Material addendum changes and readiness invalidation.

**Wave exit / MVP-3:** Procurement Teams can publish and manage live tenders.

## 16. W12 — Direct submissions and sealed exchange

### S23 — Submission schema, final validation, encryption, receipt

**Database**

- Exchange submissions/versions/envelopes/manifests/receipts.

**Backend**

- Final validation, lock response, encrypt/seal, server timestamp, acknowledgement, metadata event.

**Frontend**

- Platform submission page, declaration, step-up, progress, receipt.

**QA/security**

- Idempotency, deadline boundary, content hash, tamper, no pre-opening content.

### S24 — Withdrawal, replacement, closing spike, incident controls

**Database/backend**

- Lineage, withdrawal/replacement policy, reconciliation, deadline reservation if needed.

**Frontend**

- Replace/withdraw with consequences and history.

**QA/performance**

- Concurrent closing spike, retries, interruption, queue/storage/KMS failure, evidence.

**Wave exit:** Sealed submission is independently security-reviewed for pilot.

## 17. W13 — Bid opening

### S25 — Committee, declarations, quorum, and release

**Database**

- Opening session/members/declarations/release records.

**Backend**

- Time/state/quorum/conflict/step-up/key-release policies.

**Frontend**

- Opening preparation and controlled start/release experience.

**QA/security**

- Invalid opening attempts, partial failure, double release, conflict cases.

### S26 — Integrity verification and minutes

**Database/backend**

- Manifest verification, opening facts, minutes, closure/evidence package.

**Frontend**

- Submission list after release, integrity states, minutes and sign-off.

**QA/UAT**

- Full submission-to-opening pilot scenario.

## 18. W14 — Evaluation and award

### S27 — Preliminary evaluation

- Database findings/evidence/clarifications.
- Backend pass/fail rules, disqualification evidence, stage progression.
- Frontend compliance matrix and submission viewer.
- QA on mandatory criteria, permissions, and audit.

### S28 — Technical and financial evaluation

- Database assignments/scores/arithmetic corrections/locks.
- Backend independent scoring, stage restrictions, consolidation.
- Frontend criterion scoring split view, restricted financial workspace.
- QA on unpublished criteria, score ranges, conflict and locking.

### S29 — Moderation, recommendation, award, notices, handover

- Database results/reports/recommendations/award/notices/handover.
- Backend approvals, outcome events, report export.
- Frontend panel results, recommendation/award workflow, Tenderpreneur outcome.
- UAT complete platform tender lifecycle.

**Wave exit / MVP-4:** Closed-loop procurement lifecycle works end to end.

## 19. W15 — Advanced AI and intelligence

### S30 — Tenderpreneur advanced AI

- Proposal drafting with verified knowledge
- Contradiction/unsupported claim review
- Submission audit
- Win/loss learning
- AI benchmark and cost gates

### S31 — Procurement AI and analytics

- Tender draft/quality review
- Criteria assistance
- Evidence locator/bid comparison
- Price/outlier support
- Draft evaluation reports
- Procurement, Tenderpreneur, and platform analytics/read models

**Exit:** Human authority is preserved; benchmarks and pilot value pass.

## 20. W16 — Internationalization, hardening, pilot, launch

### S32 — Release hardening

**Database/operations**

- Migration fleet rehearsal, backup/restore, retention/legal hold, scale tests.

**Backend/security**

- Country-pack validation, SSO/API readiness as scoped, VAPT remediation, threat model update.

**Frontend**

- Localization, accessibility remediation, browser/device coverage, help/onboarding content.

**QA/UAT**

- Full regression, performance, DR exercise, pilot sign-off, release evidence.

**Exit:** Production launch gate passed.

## 21. Ticket hierarchy

```text
Release
  → Wave
    → Epic/Module
      → User journey slice
        → Database ticket
        → Backend/API ticket
        → Frontend ticket
        → QA/evidence ticket
```

Do not create frontend-only “complete page” tickets without their operational contracts and dependencies.

## 22. Developer execution packet

Every implementation packet should include:

- Ticket ID/title and requirement IDs
- User/business outcome
- Current code/spec evidence
- Dependencies and blocked-by list
- Domain model and rules
- Migration/schema changes and seed data
- API/event changes and examples
- Authorization matrix
- UI route/page/form/states
- Test cases and evidence path
- Non-goals
- Rollout/feature flag/migration plan
- Completion checklist

## 23. Release branch and review gates

- Trunk-based or short-lived branches
- Mandatory reviews for domain, migration, security, API, and UI changes
- Contract-breaking changes blocked by CI unless versioned/approved
- Feature flags for incomplete or risky functionality
- No production flag activation before UAT/security/operations evidence
- Release notes map to requirement and evidence IDs


---

<!-- SOURCE FILE: 18_DEVOPS_DEPLOYMENT_OPERATIONS.md -->

# TenderOS — DevOps, Deployment, and Operations

## 1. Operational objectives

- Reproducible local, CI, test, staging, and production environments
- Safe database-per-tenant provisioning and migration
- Observable, reversible, and secure deployments
- Reliable asynchronous processing and external integration handling
- Tested backup, restore, and incident procedures
- Cost visibility for infrastructure, AI, search, storage, and tenant database fleets

## 2. Environment model

| Environment | Purpose | Data policy |
|---|---|---|
| Local | Developer implementation and debugging | Synthetic only |
| CI | Automated build/test/scan | Ephemeral synthetic fixtures |
| Development | Shared integration | Synthetic; no production secrets |
| QA | System, security, performance, AI evaluation | Versioned test datasets |
| UAT/Pilot | Persona validation | Synthetic or explicitly approved sanitized pilot data |
| Production | Customer operations | Controlled live data |
| DR | Recovery capability | Encrypted replicated/restorable production data under restricted access |

Environment configuration is code or secret-manager configuration; no undocumented manual server changes.

## 3. Local Docker Compose

Recommended services:

```yaml
services:
  web:
  api:
  worker:
  ai-api:
  ai-worker:
  postgres-control:
  postgres-exchange:
  postgres-tenant-cluster:
  pgbouncer:
  nats:
  redis:
  minio:
  mailpit:
  clamav:
  otel-collector:
  prometheus:
  grafana:
```

Compose profiles may keep the full observability/AI stack optional for normal development. Migrations and seed commands must be explicit and repeatable.

## 4. Container standards

- Minimal pinned base images
- Multi-stage builds
- Non-root runtime user
- Read-only root filesystem where practical
- Explicit health checks
- Resource requests/limits in orchestrated environments
- No build tools or source secrets in runtime image
- Image vulnerability scan and SBOM
- Immutable image tags using commit/release digest
- Signed images/artifacts where feasible
- Graceful shutdown and worker lease release

## 5. CI pipeline

Suggested stages:

```text
1. Validate repository/format
2. Lint and type check
3. Architecture/import boundary checks
4. Unit tests
5. OpenAPI/event compatibility checks
6. Build binaries/web/AI packages
7. Database migration tests
8. Integration tests with containers
9. Frontend component tests
10. E2E smoke tests
11. Security/dependency/secret/license scans
12. Container builds and scans
13. SBOM and provenance
14. Publish candidate artifacts
```

Pull requests fail when required gates fail. Flaky tests are defects, not routine retries.

## 6. CD pipeline

```text
Approved artifact
→ deploy infrastructure/config changes
→ apply compatible control/exchange migrations
→ canary tenant migrations
→ deploy API/worker/AI/web candidate
→ run smoke/contract checks
→ expand tenant migration batches
→ monitor SLO/error/business metrics
→ activate feature flag
→ complete release evidence
```

Database and application compatibility windows permit safe rolling deployment.

## 7. Infrastructure as code

Manage through Terraform, OpenTofu, Pulumi, or the selected approved tool:

- Networks and security groups
- Compute/orchestration
- Database clusters and PgBouncer
- Object storage and lifecycle policies
- Broker/cache
- KMS/secrets
- DNS/TLS/CDN/WAF
- Monitoring and alerting
- Backup policies
- IAM/service identities

Manual emergency changes are documented and reconciled back into code.

## 8. Secrets management

Secrets include:

- Database credentials
- KMS keys/references
- AI provider keys
- Payment/email/source connector credentials
- OAuth/OIDC client secrets
- Webhook signing secrets

Rules:

- Store in a dedicated secrets manager
- Short-lived/workload identity preferred
- Separate by environment
- Least-privileged access
- Rotation schedule and incident rotation runbook
- Never commit, echo, include in build artifacts, or expose to browser
- Audit reads/changes for high-risk secrets

## 9. Tenant provisioning operations

Provisioning worker steps:

1. Validate tenant and plan request.
2. Reserve tenant slug/database name and storage namespace.
3. Select region/cluster using capacity and plan rules.
4. Create database and roles.
5. Register secret references.
6. Apply common and tenant-type migrations.
7. Seed country pack, workflow, roles, and reference data.
8. Create object-storage policies.
9. Run isolation and health checks.
10. Mark active and notify administrator.

Every step records state and can retry safely. Operators can inspect failure without direct database editing.

## 10. Tenant migration fleet

### Strategy

- Track schema version per tenant.
- Canary on internal/test tenants.
- Migrate small batches with concurrency limits.
- Pause automatically on error threshold or performance degradation.
- Support retries and forward fixes.
- Keep API compatible with old/new schema during rollout.
- Display fleet status in Platform Admin.

### Prohibited practice

Do not run an unbounded loop across all tenant databases during a production release without batching, observability, and stop controls.

## 11. Observability

### Logs

- JSON structured logs
- Trace, request, tenant, actor pseudonymous ID, operation, and result code
- Redaction/classification
- Separate audit logs from diagnostic logs

### Metrics

Infrastructure:

- CPU, memory, storage, network, container restarts
- Database connections, locks, replication, WAL, query latency
- Queue depth, consumer lag, dead letters
- Object storage request/error/volume

Application:

- API latency/error by route and tenant cluster
- Tenant provisioning/migration state
- Publication/submission/opening/evaluation operations
- Authentication and authorization failures
- Search latency/freshness/source health
- AI queue, latency, tokens/cost/quality
- Email/payment/webhook delivery

### Traces

Trace user request through API, tenant router, database, event, worker, AI/search, and external dependency. Sampling must preserve error and critical-operation traces while controlling cost.

## 12. Service-level objectives

Initial targets subject to business approval:

- Monthly availability: 99.9% for core production APIs/web
- Critical submission availability monitored separately around deadlines
- p95 non-AI API read < 300 ms at baseline load
- p95 non-AI write < 500 ms excluding file/third-party work
- Search p95 < 2 seconds for indexed queries
- Critical queue oldest-job alert before user-facing breach
- Backup freshness and restore-drill targets by plan

SLOs include error budgets and an agreed response when consumed.

## 13. Scheduled jobs

Examples:

- Tender open/close transitions
- Deadline reminders
- Document expiry alerts
- Saved search execution
- Source fetch/change detection
- Usage aggregation
- Retention/deletion
- Subscription renewal/grace/suspension
- Projection reconciliation
- Tenant migration/backup verification

Jobs use persistent schedules, distributed leases, idempotency, and missed-run recovery. Server timezone assumptions are prohibited.

## 14. Backups and disaster recovery

### Backup coverage

- Control database
- Exchange database
- Every tenant database
- Object storage and version metadata
- Search/rebuild metadata
- KMS/key-recovery configuration
- Infrastructure/configuration repository

### Restore workflow

```text
Declare restore target and incident/change record
→ restore into isolated environment/database
→ verify integrity/schema/events/object references
→ reconcile outbox/inbox and search indexes
→ run security and business smoke tests
→ approve cutover or export
→ preserve original evidence
```

Never restore blindly over active production.

### Exercises

- Monthly sampled tenant restore
- Quarterly control/exchange restore exercise
- Periodic full DR tabletop/failover rehearsal
- Sealed-submission recovery scenario before marketplace pilot

## 15. Deployment safety

- Feature flags separate deployment from exposure.
- Kill switches for AI provider, source connector, submissions, opening, email, and payment where appropriate.
- Health and smoke checks before traffic expansion.
- Automated rollback for application-only regressions.
- Database rollback is not assumed; use expand/contract and forward fix.
- Deadline-critical operations may require release freeze windows.

## 16. Incident management

Severity:

- SEV-0: Confirmed cross-tenant/sealed-bid/security catastrophe or widespread irreversible loss
- SEV-1: Major outage or critical workflow unavailable
- SEV-2: Significant degradation or isolated high-impact issue
- SEV-3: Limited impact

Runbooks include:

- Account/session compromise
- Tenant database unavailable
- Cross-tenant access alert
- Submission deadline outage
- KMS/opening failure
- Broker backlog
- AI provider incident
- Source connector abuse/failure
- Payment webhook incident
- Backup/restore failure

Post-incident review produces corrective tickets, owner, due date, and evidence.

## 17. Capacity and cost management

Track cost by:

- Tenant/database cluster
- Object storage and egress
- Search/index
- Broker/cache
- AI model/embedding/OCR use case
- Source connector
- Email/SMS/WhatsApp
- Backup and logs/traces

Use quotas, batching, model routing, cache, storage lifecycle, pool limits, and dedicated enterprise pricing to preserve gross margin.

## 18. Operational readiness checklist

Before activating a module:

- Dashboard and alerts exist
- On-call owner and runbook assigned
- Rate/size/quota limits configured
- Dependency timeouts/retries/circuit breakers tested
- Audit and business metrics verified
- Backup/restore implications covered
- Feature flag and rollback/forward-fix path ready
- Support documentation and known errors available
- UAT and security evidence linked


---

<!-- SOURCE FILE: 19_TRACEABILITY_ACCEPTANCE_EVIDENCE.md -->

# TenderOS — Requirements Traceability, Acceptance, and Evidence

## 1. Purpose

This document defines how business intent remains traceable through product requirements, system design, implementation, tests, UAT, and release decisions.

## 2. Traceability chain

```text
Business Objective
→ Business Requirement
→ Product Requirement
→ System Requirement / Business Rule
→ Module and Architecture Component
→ Database Migration / API / Event / Page
→ Automated Test / UAT Scenario
→ Evidence Artifact
→ Release and Sign-off
```

No requirement is considered delivered because code exists. It is delivered when the traceability chain and acceptance evidence are complete.

## 3. Identifier conventions

| Prefix | Meaning | Example |
|---|---|---|
| `BO` | Business objective | `BO-02` |
| `BR` | Business requirement | `BR-024` |
| `PR` | Product requirement | `PR-TP-011` |
| `SR` | System requirement | `SR-SUB-003` |
| `RULE` | Domain invariant | `RULE-SUB-003` |
| `DATA` | Data requirement | `DATA-006` |
| `API` | API requirement | `API-004` |
| `UX` | Experience requirement | `UX-005` |
| `SEC` | Security requirement | `SEC-007` |
| `AI` | AI requirement | `AI-EXTRACT-001` |
| `SEARCH` | Deep Search requirement | `SEARCH-DEDUP-001` |
| `QA` | Quality requirement | `QA-ISO-001` |
| `OPS` | Operational requirement | `OPS-003` |
| `ADR` | Architecture decision | `ADR-005` |
| `EVID` | Evidence item | `EVID-W12-004` |

## 4. Traceability matrix baseline

| Business | Product/System | Module | Primary implementation | Acceptance evidence |
|---|---|---|---|---|
| BR-001–006 | PR-SH-001/002/009; SR-TEN/IAM | M02–M05 | Control DB, tenant router, `/me`, context, entitlements | Provisioning, context, isolation, billing tests |
| BR-010 | PR-PT-001 | M10 | Plans/requests DB, APIs, pages | Requester-to-approval UAT |
| BR-011 | PR-PT-002 | M11 | Tender schema, authoring API/workspace | Valid/invalid tender test set |
| BR-012 | PR-PT-004 | M08/M11 | Workflow/approval and publish guard | Approval version and block tests |
| BR-013 | PR-PT-006 | M12 | Addendum versions/events/pages | Addendum impact E2E |
| BR-014 | PR-PT-007/008; SR-SUB | M17/M18 | Encryption, KMS, exchange, opening | Independent review and attack tests |
| BR-015/016 | PR-PT-009–011 | M19/M20 | Evaluation/award DB, APIs, pages | Evaluation-to-award UAT |
| BR-020/021 | PR-TP-004/005 | M14/M21 | Import, provenance, extraction | File/URL import and verification tests |
| BR-022 | PR-TP-001/002 | M13 | Profile/vault DB, API, pages | Expiry/version/access tests |
| BR-023/024 | PR-TP-008/011 | M16 | Requirement/evidence/readiness | Mandatory blocker UAT |
| BR-025 | PR-TP-009 | M16 | Proposal/pricing controls | Financial access matrix |
| BR-026 | PR-TP-012/013 | M16/M17 | Package, receipt/evidence | Exact-version reconciliation |
| BR-030–034 | PR-TP-003; SR-AI-005/006 | M15 | Connectors, index, search, alerts | Relevance/freshness/dedup benchmarks |
| BR-040–046 | PR-PT-003/010; PR-TP-005/010/011 | M21–M23 | AI jobs, retrieval, citations, UI | AI benchmark and human review UAT |
| BR-050–054 | PR-SH-008/010; SR-EVT/DATA | M06/M24/M25 | Audit, country packs, reports | Audit export and locale/country tests |

The project tracker expands this baseline to ticket level.

## 5. Acceptance criterion format

Use Given/When/Then for observable behavior and explicit non-functional thresholds where applicable.

Example:

```text
Requirement: RULE-SUB-003
Given a tender has not reached an authorized opening state
And the user is a procurement administrator
When the user requests a submission document or decryption release
Then the API denies access
And no object URL or key operation is issued
And the attempt is recorded as a security audit event
```

## 6. Evidence types

- Automated test report
- API contract test
- Migration verification
- Security/authorization test
- Performance report
- AI benchmark report
- Accessibility report
- Browser/device report
- Screenshot or recording of UAT flow
- Audit/evidence manifest
- Backup/restore report
- Architecture/security review
- Approved decision record
- Pilot sign-off

Screenshots alone do not prove backend rules or authorization.

## 7. Evidence register schema

Each evidence item records:

```yaml
id: EVID-W07-014
requirements:
  - BR-024
  - PR-TP-011
  - RULE-BID-003
release: R2
wave: W07
build: git-sha-or-release
environment: uat
executed_at: 2026-xx-xxTxx:xx:xxZ
executed_by: user-or-ci
artifact_paths:
  - reports/e2e/readiness.html
  - recordings/external-submission.webm
result: PASS
limitations: []
approvals:
  - role: Product Owner
  - role: QA Lead
```

## 8. Completion states

Use truthful states:

- `NOT_STARTED`
- `DESIGNED`
- `IMPLEMENTING`
- `IMPLEMENTED_NOT_VERIFIED`
- `VERIFIED_TECHNICALLY`
- `UAT_PENDING`
- `ACCEPTED`
- `DEFERRED`
- `BLOCKED`
- `REJECTED`

Do not use `COMPLETE` for work that lacks required verification or UAT.

## 9. Review and sign-off matrix

| Change | Required reviewers/sign-off |
|---|---|
| Business scope/rule | Product owner + domain/procurement owner |
| Tenant/data ownership | Architecture + security + data owner |
| Database migration | Backend/data reviewer + operations |
| API breaking change | API owner + frontend/integration owners |
| Authorization/security | Security reviewer + module owner |
| Sealed submission/opening | Security + procurement governance + architecture |
| AI use case/prompt/model | Product + AI owner + security/privacy where sensitive |
| Frontend workflow | Product/UX + accessibility + target persona UAT |
| Production release | Product + engineering + QA + operations + security as risk requires |

## 10. Change control

When a requirement changes:

1. Create decision/change record.
2. State reason, affected users, risks, and effective release.
3. Update BRD/PRD/SRD/business rules as needed.
4. Identify affected modules, data, APIs, pages, tests, prompts, reports, and documentation.
5. Re-estimate waves/tickets.
6. Preserve superseded requirement/version history.
7. Re-run impacted acceptance evidence.

Silent specification drift is prohibited.

## 11. Release acceptance package

Every release package contains:

- Scope and requirement list
- Known exclusions and deferred items
- Architecture/decision changes
- Migration and deployment plan
- Automated test summary
- Security and privacy review
- Performance and recovery evidence
- AI evaluation report where applicable
- UAT scripts/results/signatures
- Open defects and accepted risks
- Rollback/forward-fix and runbooks
- Release notes and support guidance

## 12. Product-level acceptance scenarios

### A. External tender

A Tenderpreneur discovers/imports, verifies, assesses, prepares, approves, audits, packages, submits externally, records evidence, and records outcome.

### B. Platform tender

A Procurement Team plans/authors/approves/publishes; Tenderpreneur prepares/submits; system seals; committee opens; evaluators score; approval awards; both sides receive outcome.

### C. Security isolation

Two tenants with similarly shaped records cannot discover or access each other's database rows, objects, search results, AI citations, jobs, exports, or exchange-private data.

### D. AI governance

AI produces source-linked assistance; user can inspect, edit, reject; unsupported content is labeled; final decision remains human and auditable.

### E. Recovery

A sampled tenant is restored to an isolated target, reconciled with objects/events, and passes critical workflow checks.


---

<!-- SOURCE FILE: 20_FRONTEND_TEMPLATE_ADOPTION.md -->

# TenderOS — Frontend Template Adoption and Reverse-Engineering Guide

## 1. Purpose

A downloaded frontend template will accelerate visual implementation, but it must be treated as an input to a governed product design process. It must not define business workflows, roles, routes, fields, or backend behavior.

The template has not been provided with this specification pack. The steps below become mandatory when it is added to the workspace.

## 2. Required inputs

- Original template archive/repository
- License and purchase/download evidence
- Template documentation
- Framework/package manifest and lockfile
- Asset/font/icon licenses
- Screenshots or source demo URL if available
- Product brand name/logo/colors if decided

Do not distribute proprietary template or font files outside their license terms.

## 3. Stage A — Safety and license audit

Before running the template:

- Verify source and checksum where available
- Review license for commercial SaaS use, redistribution, seat/project limits, and attribution
- Scan archive and dependencies
- Inspect scripts, postinstall hooks, remote assets, analytics, trackers, and embedded credentials
- Identify abandoned, vulnerable, or duplicate packages
- Confirm framework version compatibility
- Quarantine unknown binaries

Output: `docs/design/template-audit.md` and accept/reject decision.

## 4. Stage B — Structural inventory

Inventory:

- Routes/pages
- Layouts and navigation variants
- Components and component variants
- Forms and validation
- Tables/charts
- Dialogs/drawers/toasts
- Icons/images/illustrations
- Typography and fonts
- Theme/dark mode
- Responsive breakpoints
- State management and API assumptions
- Mock-data sources
- Third-party plugins

Classify each item:

- Reuse as-is
- Reuse after accessibility/quality correction
- Restyle/rebuild
- Remove
- License-restricted

## 5. Stage C — Extract a design system

Convert template styling into product-owned tokens:

```text
Color
Typography
Spacing
Radius
Borders
Elevation
Layout widths
Breakpoints
Motion
Focus
Component density
Status semantics
```

Map tokens into Tailwind CSS theme variables and CSS custom properties. Avoid scattered hardcoded values.

Create/normalize shadcn-based primitives under repository ownership. Template-specific wrappers must not obscure accessible behavior.

## 6. Stage D — Product route mapping

Map template layouts to actual route groups:

- Public website/catalogue
- Authentication/onboarding
- Platform Admin
- Procurement Team
- Tenderpreneur

For every template page:

- Identify whether an approved TenderOS operation needs it.
- Map it to a route/page contract.
- Remove pages with no product purpose.
- Do not rename a generic CRM/e-commerce page and call it procurement functionality.

## 7. Stage E — Component mapping

Example mapping table:

| Product component | Template candidate | Decision | Required changes |
|---|---|---|---|
| App shell | Sidebar layout A | Adapt | Portal-aware nav, workspace switcher, responsive/a11y |
| Compliance matrix | Generic data table | Rebuild/extend | Grouping, evidence, sticky columns, saved filters |
| Tender authoring | Settings form | Partial reuse | Section navigation, version, review comments, validation |
| Evaluation workspace | None | New | Split evidence/scoring pane, restricted envelopes |
| AI result panel | Chat panel | Rebuild | Sources, confidence, warnings, disposition |
| Document viewer | File preview | Adapt | Version, permissions, page citations, secure URLs |

## 8. Stage F — Remove template assumptions

Remove or replace:

- Hardcoded demo users/roles
- Mock API calls and fake counts
- LocalStorage as business source of truth
- Placeholder charts without defined metrics
- Generic status values
- Frontend-only authorization
- Raw IDs in forms
- URL inputs where file upload is required
- Unnecessary card grids
- Broken mobile navigation
- Inaccessible custom components
- Remote fonts/assets that violate policy or performance goals
- Template analytics/telemetry

## 9. Stage G — Integrate contracts

- Generate or validate TypeScript types from OpenAPI.
- Implement centralized API client and error mapping.
- Bootstrap identity/context from `/v1/me`.
- Build portal navigation from effective capabilities and product route map.
- Use real backend pagination/filter/sort.
- Implement version conflict and idempotency behavior.
- Clear tenant-scoped data on context change.

No accepted route may depend on static mock JSON.

## 10. Stage H — Visual and accessibility validation

Validate:

- Keyboard navigation
- Focus visibility/order
- Screen-reader labels and live regions
- Contrast
- Table/form semantics
- Dialog/drawer trapping and restore focus
- Error summary/field association
- Responsive desktop/tablet/mobile behavior
- Reduced motion
- 200% zoom/reflow
- Long names, currencies, languages, and dates
- Empty/loading/error/forbidden/conflict states

## 11. Stage I — Product screenshot baseline

Capture approved screenshots for:

- Public tender page
- Platform Admin tenant list/detail
- Procurement dashboard and tender authoring
- Clarification and evaluation workspaces
- Tenderpreneur discovery, import, compliance matrix, proposal, readiness, submission
- Mobile essential flows

Use visual regression tests for stable components, not to lock every dynamic data pixel.

## 12. Acceptance criteria

The template adoption is complete when:

- License/security audit is approved.
- Product-owned tokens and component inventory exist.
- Portal route/navigation map is implemented.
- Critical components meet accessibility requirements.
- Mock pages/data are removed from accepted workflows.
- No template component bypasses backend authorization or product rules.
- Performance budget is met.
- Visual design is consistent across all three portals.
- Product/UX owner accepts representative workflows.


---

<!-- SOURCE FILE: 21_TECHNOLOGY_DECISIONS.md -->

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


---

<!-- SOURCE FILE: 22_GLOSSARY_ASSUMPTIONS_DECISIONS.md -->

# TenderOS — Glossary, Assumptions, and Decision Register

## 1. Glossary

| Term | Definition |
|---|---|
| TenderOS | Working name for the complete SaaS product |
| Platform Admin | SaaS operator portal and privileged platform role family |
| Procurement Team | Buying organization and its procurement users |
| Tenderpreneur | Supplier, contractor, consultant, company, or bid team pursuing tenders |
| Tenant | An isolated organization workspace with its own database and storage boundary |
| Control plane | Platform-wide identities, tenant registry, subscriptions, operations, and configuration |
| Tenant data plane | Private organization database and storage containing business records |
| Exchange plane | Controlled public/cross-tenant projections and sealed transaction metadata |
| Platform tender | Tender authored and published by a Procurement tenant in TenderOS |
| External tender | Tender sourced outside TenderOS and imported by a Tenderpreneur |
| Public catalogue | Searchable approved platform tender projections and lawful source records |
| Deep Search | Multi-source, provenance-aware tender discovery, matching, monitoring, and research |
| Opportunity | A discovered or imported tender candidate before/while becoming a bid workspace |
| Bid workspace | Private Tenderpreneur work area for assessment, requirements, response, approvals, and submission |
| Compliance matrix | Requirement-by-requirement status, owner, evidence, verification, and risk record |
| Compliance vault | Reusable company documents, facts, staff, projects, certifications, and knowledge |
| Publication version | Immutable approved version of a tender exposed to participants |
| Addendum | Authorized published change to a tender or its instructions |
| Submission package | Final approved response version and manifest prepared for submission |
| Sealed submission | Encrypted platform submission unavailable to procurement users before authorized opening |
| Opening session | Governed process releasing sealed bids to authorized committee/evaluation users |
| Preliminary evaluation | Mandatory/eligibility/completeness stage |
| Technical evaluation | Scored or pass/fail assessment of technical criteria |
| Financial evaluation | Price and commercial assessment according to published rules |
| Human in the loop | Required authorized human review/decision before material action |
| Source citation | Exact link from AI/extracted claim to source document/version/location |
| Outbox | Transactional table recording events to publish after domain commit |
| Inbox | Consumer record preventing duplicate event side effects |
| Country pack | Configurable terminology, locale, currency, methods, document types, and policy defaults |
| Evidence register | Traceable proof that a requirement or release gate was verified |

## 2. Baseline assumptions

- The first commercial market is Kenya.
- Both private and public/regulated procurement contexts may use the product, subject to configuration and legal review.
- Tenderpreneurs need independent value from external tenders before a large buyer network exists.
- Each tenant must have its own PostgreSQL database.
- A user may belong to multiple tenants.
- Platform Admin is separate from tenant organization administration.
- The downloaded frontend template will be available before frontend implementation and has a valid commercial license.
- Go is the only primary transactional backend language; Python is limited to AI/search/document processing services.
- PostgreSQL is the transactional source of truth.
- Direct external portal submission is only enabled through official supported integration and human authorization.
- AI is advisory and source-grounded.
- Mobile web responsiveness is required; a native mobile app is not initial scope.
- Contract management after award begins as a handover/export, not a full CLM module.

## 3. Open product decisions

| ID | Decision needed | Default until decided |
|---|---|---|
| DEC-P-001 | Final brand/product name | Use TenderOS as working name |
| DEC-P-002 | Can one tenant be both buyer and bidder? | Separate tenant type/workspace; hybrid only after policy design |
| DEC-P-003 | Tenderpreneur verification/badging model | No public “verified” badge until verification governance exists |
| DEC-P-004 | Free trial and plan pricing | Entitlement engine supports; commercial values deferred |
| DEC-P-005 | Whether public visitors can submit clarification questions | Require authenticated participation initially |
| DEC-P-006 | Native mobile application | Responsive web only for MVP |

## 4. Open architecture decisions

| ADR | Decision | Required evidence |
|---|---|---|
| ADR-001 | Final monorepo and module layout | Build/test proof and team agreement |
| ADR-002 | Database cluster provider/topology | Cost, HA, backup, PgBouncer, region |
| ADR-003 | Tenant database credentials/pool model | Isolation, connection scale, rotation test |
| ADR-004 | Identity provider vs application-owned auth details | Security, SSO, MFA, cost, migration |
| ADR-005 | Sealed bid cryptography and key release | Threat model, KMS prototype, independent review |
| ADR-006 | Message broker | Operations skill, durability, load, local parity |
| ADR-007 | Search store and scale trigger | Relevance/load benchmark |
| ADR-008 | AI provider/model routing | Quality, cost, privacy, region, fallback |
| ADR-009 | OCR/parser stack | Benchmark on real tender corpus |
| ADR-010 | Payment provider(s) | Kenya support, recurring billing, webhooks, reconciliation |
| ADR-011 | Production orchestration | Team capability, scale, reliability, cost |
| ADR-012 | Analytics store | Volume, privacy, reporting latency |

## 5. Open governance decisions

- Procurement workflow templates and separation-of-duty defaults
- Opening quorum and emergency procedure defaults
- Retention periods by record type and tenant type
- Whether/when bid prices may appear in opening minutes/public data
- Supplier/Tenderpreneur identity disclosure rules for clarifications
- Country-specific disqualification and appeal/review workflows
- Platform support access approval model
- AI provider data residency/retention contract
- Source crawling terms and connector approval process
- Record/e-signature requirements for legally sensitive procedures

## 6. Decision record template

```markdown
# ADR/Decision <ID>: <Title>

- Status: Proposed | Accepted | Superseded | Rejected
- Date:
- Owners:
- Related requirements:

## Context

## Options considered

## Decision

## Consequences

## Security/privacy/compliance impact

## Migration/rollout

## Validation evidence

## Supersedes / superseded by
```

## 7. Assumption validation rule

An assumption becomes a decision or requirement when implementation depends on it. The responsible ticket must either:

- Validate it with evidence,
- Record an accepted decision, or
- Mark the work blocked/deferred.

Agents and developers must not silently convert assumptions into permanent architecture.


---

<!-- SOURCE FILE: 23_AGENTS.md -->

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
