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

