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

