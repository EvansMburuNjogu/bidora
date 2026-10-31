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

