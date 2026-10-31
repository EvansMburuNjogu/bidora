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

