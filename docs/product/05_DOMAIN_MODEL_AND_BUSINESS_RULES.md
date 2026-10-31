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

