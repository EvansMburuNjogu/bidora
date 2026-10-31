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

