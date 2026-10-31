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

