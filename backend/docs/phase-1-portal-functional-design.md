# Phase 1 portal functional design

Status: **APPROVED PRODUCT DIRECTION — IMPLEMENTATION/VERIFICATION NOT STARTED**  
Scope: shared authentication, Tenderpreneur Portal and Platform Admin Portal  
Related planning gate: BID-ARC-010  
Current Option A status: feature-frozen and unaffected by this proposal

## 1. Purpose

This document records the Phase 1 portal direction discussed with the requesting user. Phase 1 is intended to be a commercially useful release rather than a read-only administration shell. It combines a simple Tenderpreneur experience, powerful source-grounded AI, platform administration, plans, billing, PesaPal payments, notifications and controlled support access.

The requesting user approved this product direction and the later company-first onboarding, letterhead, local-vector context and revised commercial model. This approval does not itself execute migrations, API handlers, frontend work, provider deployment or destructive retention processing. Each functional area still requires a bounded contract, reviewed policy, schema, implementation authorization and evidence.

## 2. Confirmed product direction

The requesting user selected the following direction:

- All users authenticate through one shared login entry point.
- A Phase 1 user has one portal type: `PLATFORM_ADMIN`, `PROCUREMENT` or `TENDERPRENEUR`.
- The authenticated user is redirected to the portal associated with that type.
- Platform administration must be functional, not limited to read-only metadata.
- Platform administration includes global SMTP, countries/reference data, AI provider settings, retention settings, roles and permissions, PesaPal, plans, subscriptions, payments, notifications, support access, dashboards and users.
- Tenderpreneur plans are primarily differentiated by the number of Bid workspaces that may be started in a monthly billing period.
- PesaPal is the selected payment provider.
- AI is a primary Bidora selling point and should drive the Tenderpreneur experience.
- AI must speak in the tender's language and terminology and must not feel technical to ordinary users.
- Tenderpreneur roles and journeys must remain simple.

Approved commercial direction is recorded in section 7. Per-plan storage/AI processing limits, legally validated retention periods, production secret management, PesaPal deployment details and frontend execution authorization remain gates.

Detailed approved continuations:

- [Tenderpreneur organization onboarding and AI context](phase-1-tenderpreneur-onboarding-context.md)
- [Data privacy, retention and deletion policy design](phase-1-data-privacy-retention-deletion.md)

## 3. Identity, portal type and authority

### 3.1 Shared authentication

Every user signs in through the same authentication service and login page. Password and approved Google authentication use the same identity and session model. Email verification, login throttling, recovery, browser sessions, CSRF protection and MFA remain shared controls.

After authentication, the server resolves the user's current portal type and authority:

| Portal type | Redirect | Additional authority required |
|---|---|---|
| `PLATFORM_ADMIN` | `/platform` | Active platform role and mandatory TOTP AAL2 |
| `PROCUREMENT` | `/procurement` | Active membership in an active Procurement tenant |
| `TENDERPRENEUR` | `/tenderpreneur` | Active membership in an active Tenderpreneur tenant |

Portal type controls routing only. It is not sufficient authority for an operation. Every API operation still rechecks identity state, membership or platform assignment, explicit capability, resource state, context generation and required session assurance.

### 3.2 Phase 1 account-type restriction

A Phase 1 identity has one portal type. A single email identity does not switch between Platform Admin, Procurement and Tenderpreneur portals in this initial model. Changing an established type is not a normal profile edit and requires a separately governed migration policy.

Platform Admin identities cannot self-assign their type or role. They are established by the controlled initial-owner bootstrap or an authorized Platform Owner invitation.

### 3.3 Roles and capabilities

Roles are understandable labels that group explicit capabilities. Role names do not independently authorize commands. Active DENY effects, narrower resource scope, separation-of-duty rules and stale context always take precedence.

## 4. Simplified Tenderpreneur roles

Phase 1 uses four fixed Tenderpreneur roles. Users may hold more than one compatible role, but operation-level maker/approver and reporter/verifier separation always applies.

### 4.1 Organization Admin

Responsibilities:

- Manage the organization account and team.
- Maintain at least one effective administrator.
- Manage the subscription, notifications and support relationship.

Functions:

- Maintain the organization profile.
- Invite users and manage pending invitations.
- Assign or revoke the four fixed roles within the actor's grant boundary.
- Transfer work before offboarding a member.
- View the current plan, billing period and tender allowance.
- Select a plan and initiate PesaPal payment.
- View invoices, receipts, credits and payment history.
- Configure permitted organization notification preferences.
- Open support cases and approve scoped support-access requests.
- View the organization dashboard and administrative history.

An Organization Admin who works on bids must also receive the relevant Bid Manager, Contributor or Approver authority. Organization administration does not automatically grant access to every bid record.

### 4.2 Bid Manager

Responsibilities:

- Move a tender from capture through preparation, submission evidence and closure.
- Coordinate people, requirements, deadlines and package readiness.

Functions:

- Capture or import tender information.
- Upload tender documents.
- Review, edit, accept or reject AI-proposed tender facts.
- Coordinate and record Bid/No-Bid decisions.
- Create a Bid workspace when allowance is available.
- Create or accept proposed requirements and assign work.
- Monitor deadlines, gaps, readiness and review activity.
- Prepare proposal versions and immutable package candidates.
- Submit the exact package for approval.
- Report an external submission and upload proof.
- Record reported outcomes and request archive.

A Bid Manager cannot approve their own exact package or verify their own external-submission report.

### 4.3 Contributor

Responsibilities:

- Complete assigned tender work accurately and support it with valid evidence.

Functions:

- View assigned tenders and requirements.
- Upload documents and evidence.
- Draft and revise technical responses.
- Use source-grounded AI suggestions.
- Edit, accept or reject AI-proposed text.
- Link permitted evidence.
- Respond to comments and requested changes.
- Mark assigned work ready for review.

Technical, legal, compliance and document-control contributors use this one role in Phase 1. More specialized role separation may be introduced later if justified by an approved policy.

### 4.4 Approver

Responsibilities:

- Independently review an exact proposal package.
- Approve or reject that package.
- Independently verify external-submission evidence when eligible.

Functions:

- Review requirements, evidence and responses.
- Open comments or request changes.
- Complete personal review of an exact version.
- Approve or reject the exact immutable package after TOTP step-up.
- Verify or reject external-submission evidence after TOTP step-up.
- View immutable historical approval and verification evidence.

The backend rejects self-approval, stale-package approval and self-verification of a submission report regardless of role combinations.

### 4.5 Tenderpreneur responsibility matrix

| Function | Organization Admin | Bid Manager | Contributor | Approver |
|---|:---:|:---:|:---:|:---:|
| Organization and users | Yes | No | No | No |
| Plans, invoices and payments | Yes | No | No | No |
| Capture and verify a tender | Optional separate grant | Yes | Optional scoped grant | No |
| Create and coordinate Bid workspace | No | Yes | No | No |
| Upload evidence | No | Yes | Yes | Review only |
| Draft responses | No | Yes | Yes | Review only |
| Prepare package | No | Yes | Yes when assigned | No |
| Approve package | No | No | No | Yes |
| Report external submission | No | Yes | No | No |
| Verify external submission | No | No | No | Yes |
| Record outcome and archive | No | Yes | No | Review only |

## 5. AI product principles

### 5.1 Product promise

Bidora should turn an uploaded tender into a cited, actionable Bid workspace containing the tender summary, important dates, requirements, risks, assignments, evidence gaps, response assistance and final checks while authorized humans remain responsible for decisions and approvals.

AI is central to the experience, not a hidden optional text generator.

### 5.2 Natural language and terminology

The Tender Assistant must:

- Detect and use the tender's primary language by default.
- Preserve the buyer's terminology, defined terms, official names, reference numbers and abbreviations.
- Avoid replacing a precise procurement or legal term with a casual synonym.
- Use the organization's approved terminology in response drafts.
- Explain findings in clear, practical language.
- State what was found, why it matters, where it appears and what the user can do next.
- Keep original authoritative wording visible when translating.
- Clearly separate a translation from the authoritative source text.
- Admit uncertainty without displaying technical model jargon as the primary experience.

Examples of user-facing labels:

| Internal meaning | User-facing wording |
|---|---|
| Proposed unverified | AI found this — please check |
| Human verified | Confirmed by your team |
| Human corrected | Updated by your team |
| Rejected proposal | Not used |
| Superseded result | Outdated after a newer document |
| Low confidence | Please double-check this |
| Citation | See where this came from |
| Requirement matrix | Tender checklist |
| Readiness gate | Ready to submit? |
| Provider failure | We couldn't finish the review |
| Stale analysis | The tender changed — review this again |

Internal states remain precise and auditable even when the interface uses natural language.

### 5.3 Natural source references

Every material AI finding must retain an exact source reference presented naturally, for example:

> Found in: Section 4.2, Mandatory Requirements, page 17  
> “The bidder shall provide a valid tax compliance certificate.”

Users should be able to open the relevant page or section without seeing internal object IDs.

### 5.4 AI operation catalogue

Phase 1 AI should support the following bounded operations.

#### Tender understanding

- Summarize the tender in its own terminology.
- Identify buyer, tender reference, lots, dates and submission channel.
- Identify eligibility and mandatory requirements.
- Identify technical and financial requirements while respecting field permissions.
- Identify forms, declarations, securities, meetings and clarification dates.
- Identify evaluation criteria, weights and thresholds.
- Identify ambiguity, conflicting dates and important risks.
- Provide exact quotations and locations.

#### Tender checklist

- Propose one checklist item per requirement.
- Suggest category, importance, evidence, owner and due date.
- Identify dependencies and missing organization information.
- Preserve source wording and location.
- Require human acceptance, editing or rejection.

#### Bid/No-Bid assistance

- Compare requirements with permitted organization information.
- Identify eligibility gaps, strengths, missing evidence and deadline pressure.
- Explain likely effort and major risks.
- Suggest `BID`, `NO_BID` or `DEFER` with reasons.
- Leave the authoritative decision to an eligible human.

#### Evidence matching

- Suggest relevant approved certificates, licences, projects, staff records, policies and references.
- Explain why each item may help.
- Show exact supporting passages, versions and expiry concerns.
- Never represent expired, unverified or unauthorized material as valid proof.

#### Response assistance

- Draft responses to selected requirements using authorized sources.
- Improve clarity, structure and consistency.
- Shorten text to stated limits.
- Identify unsupported claims and missing details.
- Suggest stronger evidence.
- Preserve the tender's and organization's preferred language.

#### Work planning

- Propose tasks, owners, internal due dates and dependencies.
- Highlight critical-path and high-risk work.
- Require Bid Manager confirmation before assignments become authoritative.

#### Tender conversation

The user may ask natural questions such as:

- What could disqualify us?
- Which documents expire before submission?
- Where is bid validity stated?
- Which requirements have no evidence?
- What changed in the latest addendum?
- What must be completed this week?

Answers must be permission-aware and source-grounded.

#### Review and pre-submission assistance

- Check requirement coverage.
- Identify missing responses and evidence.
- Identify expiry, contradiction and unsupported-claim risks.
- Identify unresolved placeholders, limits and packaging issues.
- Compare the exact proposal package with submission instructions.
- Present findings as blockers, important risks, warnings or improvements.

Deterministic application rules, not the model, decide whether a known blocker prevents approval or submission.

#### Addendum impact

- Compare a new authoritative source version with the previous version.
- Identify changed dates, requirements, criteria and instructions.
- Show affected responses, evidence and approvals.
- Mark prior suggestions as outdated where applicable.
- Require human confirmation before authoritative workflow changes.

#### Outcome learning

- Summarize reported lessons.
- Identify recurring evidence or response gaps.
- Suggest reusable material for separate human approval.
- Distinguish reported outcomes from independently verified buyer decisions.

### 5.5 Non-negotiable AI controls

Powerful AI still requires:

- Exact immutable clean document versions.
- Permission-aware tenant-isolated retrieval.
- Source citations for material tender claims.
- Human acceptance before proposals become authoritative.
- Strict bounded outputs and resource limits.
- `store:false` where supported by the approved OpenAI API.
- No API keys, private content or unnecessary personal data in logs.
- No automatic Bid/No-Bid decision, satisfaction, approval, submission or outcome verification.
- No invented organization experience, qualifications, staff, certifications, prices or approvals.
- No arbitrary external-source scraping.
- Invalidation or re-review after source changes.
- Recorded model/configuration provenance.
- Visible, retryable provider failure rather than false success.

## 6. Simplified Tenderpreneur journeys

### TP-01 — Start the organization

```text
Register and verify email
→ select Tenderpreneur account type
→ create organization
→ become Organization Admin
→ select trial or paid plan
→ complete PesaPal payment when required
→ provisioning completes
→ enter Tenderpreneur dashboard
```

The dashboard shows plan, tender allowance, active bids, deadlines, assignments, payment state and required actions.

### TP-02 — Add the team

```text
Organization Admin selects one of four roles
→ sends invitation
→ user registers or logs in
→ accepts invitation
→ membership becomes active
→ user enters Tenderpreneur Portal
```

The final Organization Admin cannot be removed without an eligible successor and completed transfer process.

### TP-03 — Understand a tender

```text
Bid Manager captures opportunity
→ uploads supported tender documents
→ documents pass quarantine scanning
→ AI creates a plain-language tender overview and proposed checklist
→ Bid Manager reviews important facts and requirements
→ accepted or edited facts become authoritative
```

Capturing and analyzing an opportunity does not consume the monthly Bid-workspace allowance.

### TP-04 — Decide whether to bid

```text
AI compares the tender with permitted organization information
→ AI explains strengths, gaps, risks and likely effort
→ team discusses the recommendation
→ eligible human records BID, NO_BID or DEFER with rationale
```

For `BID`, the entitlement service reserves one monthly allowance before creating the workspace. `NO_BID` and `DEFER` do not consume an allowance.

### TP-05 — Plan and prepare

```text
AI proposes checklist items, tasks, owners and internal deadlines
→ Bid Manager confirms the work plan
→ Contributors add evidence and responses
→ AI suggests cited evidence and draft wording
→ humans edit and accept appropriate content
→ readiness updates from authoritative facts
```

AI does not mark requirements satisfied by itself.

### TP-06 — Review and approve

```text
AI runs a plain-language compliance and consistency review
→ team resolves blockers and important risks
→ Bid Manager creates an exact immutable package
→ eligible Approver reviews the exact version
→ Approver completes TOTP step-up
→ Approver approves or rejects the package
```

A content change creates a new version and invalidates the applicability of prior review or approval where affected.

### TP-07 — Submit externally

```text
Bid Manager downloads the approved package
→ submits through the external channel
→ records channel, reported time and reference
→ uploads receipt, screenshot or email proof
→ proof passes scanning
→ a different eligible Approver verifies or rejects the evidence after TOTP step-up
```

Bidora records evidence of the reported external action. It does not claim that Bidora transmitted the bid or that the external buyer accepted it without supporting evidence.

### TP-08 — Record outcome and close

```text
Bid Manager records a reported outcome
→ attaches required clean evidence
→ corrections append history
→ AI helps summarize lessons without overstating certainty
→ Bid Manager requests archive
→ the system checks open blockers
→ bid becomes read-only archived history
```

Archive is not deletion, legal-hold release or restoration policy.

### Supporting journey — Plan limit reached

```text
Bid Manager tries to create a Bid workspace
→ monthly allowance is exhausted
→ the system explains current usage and available options
→ Organization Admin selects an upgrade
→ PesaPal payment is reconciled
→ new entitlement becomes effective
→ the original operation is retried idempotently
```

### Supporting journey — Request support

```text
Tenant user opens a support case
→ Platform Support Administrator requests minimum necessary access if needed
→ Organization Admin approves scope and expiry
→ support user completes TOTP step-up
→ every support action is audited
→ access expires or is revoked
```

## 7. Approved Tenderpreneur commercial model

Prices are displayed before VAT; applicable VAT is itemized separately before PesaPal payment. Every paid plan includes the core Tender Assistant.

| Plan | Bid workspaces/month | Included licences | Monthly base price | Additional licence/month |
|---|---:|---:|---:|---:|
| Trial | 1 during a 14-day trial | 2 | Free | Not available |
| Basic | 3 | 3 | KES 4,500 | KES 750 |
| Growth | 10 | 8 | KES 12,500 | KES 650 |
| Professional | 30 | 20 | KES 29,500 | KES 500 |
| Enterprise | Contract limit | Contract limit | From KES 60,000 | Custom |

Approved annual prices provide approximately two months' savings: Basic KES 45,000, Growth KES 125,000 and Professional KES 295,000 before applicable VAT. Enterprise terms are contracted and versioned.

One licence is one active person in one organization regardless of compatible role count. Active accepted memberships and active invitations reserve capacity; cancelled/expired invitations and revoked memberships do not. Platform/support users do not consume tenant licences. Additional mid-period licences use a server-calculated prorated charge in KES minor units and become effective only after reconciled payment. Removing licences affects future renewal; there is no automatic prorated refund, though an authorized audited credit may be issued under policy.

Core AI remains available in every paid plan because it is the central value proposition. Plans differ through Bid allowance, included/additional licences, storage, processing volume and advanced analytics rather than removing essential AI assistance from Basic.

### 7.1 Approved allowance and licence rules

- An allowance is consumed only when a `BID` decision successfully creates a new Bid workspace.
- Capture, document analysis, `NO_BID` and `DEFER` do not consume it.
- Failed or rolled-back creation releases its reservation exactly once.
- Archiving or abandoning a workspace does not restore the allowance.
- Unused allowance does not roll over.
- The allowance resets at the tenant's billing-period boundary.
- A reconciled upgrade may increase the current period allowance.
- A downgrade applies to a future period and cannot invalidate retained historical bids.
- Exhausted allowance blocks new Bid workspaces but not authorized access to retained work.
- Temporary adjustments require authorized, reasoned and audited platform action.
- Invitation issue atomically reserves available licence capacity; expiry/cancellation releases it and acceptance settles it.
- Concurrent final-licence attempts permit at most the approved capacity.
- A downgrade never removes users automatically; the Organization Admin resolves excess members/invitations before it takes effect.
- The final effective Organization Admin remains protected regardless of licence changes.

## 8. Platform Admin roles

Phase 1 uses six fixed platform roles. All Platform Administrators require TOTP AAL2. Platform role status never grants implicit access to tenant-private records.

### 8.1 Platform Owner

Responsibilities and functions:

- Maintain platform governance and administrator continuity.
- Invite Platform Administrators.
- Assign and revoke fixed platform roles.
- Protect the final effective Platform Owner.
- Approve high-risk settings and lifecycle changes.
- Revoke privileged sessions.
- Review platform audit evidence.

### 8.2 Operations Administrator

Responsibilities and functions:

- Operate tenant lifecycle and platform services.
- Create and provision tenants through durable checkpoints.
- Inspect and retry eligible failed provisioning work.
- Coordinate migration waves.
- Suspend and reactivate tenants under approved policy.
- Manage countries, reference data, maintenance windows and announcements.
- Monitor service, queue, storage, scanner and database health.

### 8.3 Billing Administrator

Responsibilities and functions:

- Create and version plans and tender allowances.
- Publish or retire plans.
- Manage subscriptions, trials, credits and future changes.
- Review invoices and payments.
- Reconcile PesaPal transactions.
- Process approved refunds and corrections through append-only records.
- Review revenue, usage and payment dashboard information.

### 8.4 Integration Administrator

Responsibilities and functions:

- Configure global SMTP through secret references.
- Test and activate email-delivery versions.
- Manage global email templates.
- Configure approved OpenAI endpoint/model and limits.
- Configure PesaPal environment, callback and credential references.
- Test, activate, pause, rotate and roll back integration configurations.
- Review redacted provider failures.

### 8.5 Support Administrator

Responsibilities and functions:

- Triage and assign support cases.
- Request minimum necessary scoped tenant access.
- Use only tenant-approved, expiring support sessions.
- Record actions and resolution.
- End support access early when no longer required.

A Support Administrator cannot approve their own access request and has no standing tenant access.

### 8.6 Security and Audit Administrator

Responsibilities and functions:

- Review security events and privileged activity.
- Disable or reactivate compromised platform identities.
- Revoke sessions.
- Govern approved MFA recovery.
- Review role changes and support sessions.
- Create and schedule retention/legal-hold policies when authorized.
- Export permitted audit evidence.
- Coordinate secret-rotation and containment incidents.

Audit history cannot be rewritten, and active legal holds cannot be bypassed.

## 9. Platform administration functions

### 9.1 Dashboard

The functional dashboard should provide safe operational summaries and actions for:

- Active, onboarding, suspended and failed tenants.
- Subscriptions by plan.
- Paid, pending, failed, reversed and unreconciled payments.
- Tender allowance consumption.
- SMTP delivery health.
- OpenAI availability and aggregate consumption.
- PesaPal callback and reconciliation failures.
- Open support requests and active support sessions.
- Platform users, disabled users and MFA-required users.
- Schema compatibility, migration progress, storage, scanning and worker health.
- Recent privileged and security events.

Authorized administrators may retry eligible work, pause an integration, start reconciliation, open the affected record or publish an announcement. The dashboard must not expose private tender content.

### 9.2 Platform users, roles and permissions

Functions:

- Bootstrap the first verified Platform Owner through a controlled one-time deployment operation.
- Invite Platform Administrators.
- Assign and revoke fixed roles.
- Protect the final owner.
- Disable and reactivate platform accounts.
- Revoke sessions after privilege changes.
- Require TOTP enrollment.
- Search and paginate safe administrator records.
- Review redacted authentication and role-assignment history.

Custom platform roles are deferred from the initial implementation unless separately approved.

### 9.3 Global SMTP and email templates

Settings and functions:

- SMTP host and port.
- Authenticated STARTTLS requirement.
- Credential and trust-root secret references.
- Sender name/address and reply-to address.
- Connection, command and delivery timeouts.
- Rate, concurrency and retry policy.
- Delivery pause state.
- Versioned templates for verification, recovery, invitation, security, billing, support and tender notifications.
- Synthetic preview and test delivery.
- Activation and rollback of tested configuration versions.
- Redacted delivery results.

Secrets must not be committed, logged, returned after saving or stored as ordinary plaintext settings.

### 9.4 Countries and reference data

Settings and functions:

- Supported countries, currencies, timezones and locales.
- Default country and currency.
- Address formats and public holidays.
- Tender categories and organization types.
- Versioned country packs and reference data.
- Scheduled activation and rollback.
- Historical preservation of the version used by a transaction.

Proposed initial market defaults are Kenya, KES, `Africa/Nairobi` and `en-KE`; these require explicit approval.

### 9.5 AI provider settings

Settings and functions:

- Enable or pause AI globally or by operation.
- Fixed approved OpenAI endpoint and model version.
- API-key secret reference.
- `store:false` enforcement.
- Timeout, retry, request, token and cost limits.
- Allowed AI operations and data classifications.
- Per-plan processing allowances.
- Synthetic connectivity and structured-output testing.
- Configuration activation, scheduled change and rollback.
- Aggregate usage and redacted failure reporting.

A platform setting cannot authorize the model to approve, submit or invent unsupported claims.

### 9.6 Retention and legal holds

Settings and functions are intended for:

- Identity/security records.
- Sessions and challenges.
- Platform audit.
- Billing/payment records.
- Notifications and support records.
- Tender documents and archived bids.
- AI metadata permitted for retention.

The eventual workflow must support versioned policies, effective dates, legal holds, affected-count preview, authorized execution, retries and immutable deletion/anonymization evidence. Exact durations, lawful bases, minimum periods, restoration interaction and destructive execution are unresolved. Destructive retention jobs remain blocked until those policies receive legal/security approval.

### 9.7 Plans, subscriptions and entitlements

Functions:

- Create and version plans.
- Define future prices, currencies and Bid-workspace allowances.
- Define user, storage, document and AI processing limits.
- Publish, schedule and retire plans.
- Start or extend trials.
- Assign approved enterprise terms.
- Schedule upgrades and downgrades.
- Apply credits or temporary allowance adjustments.
- Preserve historical invoice/subscription terms.
- Reserve, settle and release usage atomically.
- Reconcile inconsistent reservations.

Limits are enforced by backend policy, not frontend visibility.

### 9.8 PesaPal, invoices and payments

Settings and functions:

- Sandbox/production endpoint selection.
- Consumer credential secret references.
- IPN callback registration and identifier.
- Supported currency configuration.
- Provider timeout and reconciliation schedule.
- Invoice and receipt numbering policy.
- Payment initiation.
- Browser return handling.
- IPN authenticity checks and deduplication.
- Authoritative status query after ambiguous responses.
- Out-of-order event reconciliation.
- Failed, reversed, cancelled and refunded payment handling.
- Activation of entitlement only after backend reconciliation.
- Append-only financial adjustments and permitted exports.

A provider callback alone never grants a paid entitlement.

### 9.9 Notifications and announcements

Functions:

- Version global templates.
- Configure event-to-email mappings.
- Define mandatory security, payment and account-status notices.
- Allow tenant users to manage optional preferences.
- Publish targeted platform announcements.
- Schedule and expire announcements.
- Pause nonessential delivery.
- Review redacted delivery outcomes.

### 9.10 Controlled support access

Functions:

- Open and assign a support case.
- Request a specific tenant, purpose, resource/action scope and expiry.
- Obtain tenant approval where required.
- Require support-user TOTP step-up.
- Issue an expiring support session.
- Audit every permitted view or mutation.
- Revoke access immediately.
- Expose a support-session summary to the tenant administrator.

Platform administrators have no automatic tenant-private access.

## 10. Platform Admin journeys

### PA-01 — Establish the first Platform Owner

```text
Deployment operator identifies an existing verified user by immutable ID
→ one-time bootstrap assigns Platform Owner
→ user enrolls TOTP
→ user signs in through shared login
→ server redirects to Platform Admin Portal
→ bootstrap evidence is permanently audited
```

### PA-02 — Add a Platform Administrator

```text
Platform Owner selects a fixed role
→ sends invitation
→ invited user verifies identity and enrolls TOTP
→ assignment becomes active
→ user enters Platform Admin Portal
```

### PA-03 — Configure SMTP

```text
Integration Administrator creates a configuration draft
→ selects credential and trust-root references
→ validates STARTTLS and sends a synthetic test
→ activates the tested version
→ delivery uses the new version
→ previous valid version remains available for rollback
```

### PA-04 — Configure AI

```text
Integration Administrator creates an OpenAI configuration version
→ selects secret reference, model and limits
→ runs a synthetic source-grounded test
→ activates configuration
→ monitors aggregate usage and failures
→ pauses or rolls back when necessary
```

### PA-05 — Publish a plan

```text
Billing Administrator creates plan version
→ defines price and monthly Bid allowance
→ validates effective date and entitlements
→ publishes plan
→ new subscriptions may select it
→ existing historical terms remain unchanged
```

### PA-06 — Reconcile PesaPal payment

```text
PesaPal sends IPN
→ Bidora authenticates and deduplicates it
→ billing service queries authoritative provider status
→ invoice/payment state is reconciled
→ subscription entitlement becomes effective once
→ receipt and audit evidence are recorded
```

### PA-07 — Provision or suspend a tenant

```text
Operations Administrator creates tenant with type, plan, country and placement
→ provisioner completes isolated database/storage checkpoints
→ migrations and health checks pass
→ initial administrator membership is created
→ tenant becomes ACTIVE
```

Suspension and reactivation require current state, reason, authorization and recent TOTP. Suspension preserves retained evidence and only permits explicitly approved remediation, payment, export or closure operations.

### PA-08 — Provide tenant support

```text
Tenant opens case
→ Support Administrator requests minimum necessary access
→ Tenant Organization Admin approves scope and expiry
→ support user completes TOTP step-up
→ temporary support session is issued
→ actions are audited
→ access expires or is revoked
```

### PA-09 — Change retention policy

```text
Security and Audit Administrator creates policy version
→ previews affected record classes
→ obtains required approval
→ schedules effective date
→ legal holds are rechecked
→ authorized worker processes eligible records
→ execution evidence is recorded
```

The execution steps remain blocked until exact retention and legal-hold policies are approved.

### PA-10 — Respond to an integration incident

```text
Dashboard reports SMTP, OpenAI or PesaPal failure
→ authorized administrator opens incident
→ pauses affected integration or operation
→ unsafe dependent work fails closed
→ provider/configuration recovers
→ reconciliation runs
→ administrator resumes service and closes incident evidence
```

## 11. Procurement boundary

Detailed Procurement roles and governed publication/evaluation/award journeys are not defined by this document. Phase 1 shared authentication may route a valid Procurement user to `/procurement`, but a useful Procurement Portal requires a separate approved design covering organization administration, planning, authoring, publication, sealed submission, opening, evaluation and award.

A placeholder redirect or empty dashboard is not completion of Procurement functionality.

## 12. Required implementation slices

Calling this collection “Phase 1” does not make it one safe coding task. Implement it through bounded vertical slices:

1. Shared account type, `/v1/me` and secure portal routing.
2. Fixed platform roles, TOTP enforcement, bootstrap and administrator lifecycle.
3. Versioned typed platform-settings foundation.
4. Global SMTP and notification templates.
5. Countries and reference data.
6. AI provider settings and the Tender Assistant operation contracts.
7. Tenderpreneur plan catalogue and atomic Bid-workspace allowance.
8. Subscriptions, invoices and PesaPal reconciliation.
9. Tenderpreneur dashboard and simplified team roles.
10. Support cases and controlled access.
11. Retention policy and legal-hold design, followed separately by approved execution.
12. Platform dashboard and operational actions.

Each slice requires its own authorization model, request/response contract, schema, audit/outbox semantics, idempotency, concurrency behavior, migration plan and verification evidence.

The completed planning audit is [Phase 1 Tenderpreneur and Platform Admin gap audit](../../docs/tickets/14_PHASE_1_PORTAL_GAP_AUDIT.md). The first BID-API-010 slice at `api/openapi/drafts/me-portal-routing.json` is approved, and its bounded non-platform BID-AUTH-003A backend is `IMPLEMENTED_NOT_VERIFIED`: canonical `/v1/me` routing plus one-time self-selection of Tenderpreneur or Procurement without granting authority. Platform assignment/role policy and lifecycle API are approved, with control persistence implemented-not-verified but application/HTTP composition still absent; the remaining BID-API-010 settings, support and privacy contracts remain blocked. The broader design maps to BID-API-010–012, BID-AUTH-003/004, BID-TEN-004, BID-INT-004, BID-BE-014–017, BID-AI-002/005, BID-SEC-004, BID-FE-009–011, BID-QA-006 and BID-REL-005. Ticket creation supplies traceability only; each unsliced area retains its recorded decisions and evidence gates.

## 13. Decisions required before implementation

The following remain unresolved or require specialist/deployment validation:

1. Per-plan storage and AI processing limits.
2. Exact PesaPal API environment/version, merchant onboarding, authenticity mechanism and supported payment methods.
3. Production secret manager/KMS selection; until approved, settings may reference only deployment-managed secrets.
4. Legal validation of retention periods, lawful bases, legal-hold authority, deletion/anonymization and backup interaction.
5. P1–P6 in `platform-admin-authority-decision-packet.md` and the lifecycle API are approved; control persistence is implemented-not-verified, while application/HTTP composition, migration execution and the deployment-owned first-owner UUID remain gated.
6. Exact local embedding model/version/digest after licence, security, quality and resource benchmarking.
7. Support-access maximum duration and approved resource/action scopes.
8. Explicit authorization to edit and implement the web portals; the current execution boundary remains backend-only.
9. Separate Procurement role and journey design.

## 14. Verification and acceptance expectations

Phase 1 cannot be called ready from schemas or UI alone. Acceptance eventually requires:

- Shared-login and correct redirect tests for all three account types.
- Negative authorization and stale-session tests.
- Mandatory Platform Admin MFA and privileged-action step-up tests.
- Last-admin/last-owner and incompatible-operation race tests.
- Cross-tenant database, storage, job, cache and AI isolation tests.
- SMTP STARTTLS/authentication and failure tests.
- OpenAI citation, permission, prompt-injection, stale-source and provider-failure evaluations.
- PesaPal sandbox signature/authenticity, duplicate, replay, out-of-order and ambiguous-payment tests.
- Concurrent quota reservation/settlement tests.
- Support-access scope, expiry, revocation and audit tests.
- Retention/legal-hold race and restore implications before destructive execution.
- Privacy-safe logs, errors, dashboards and exports.
- UAT by Platform Admin and Tenderpreneur personas.
- Provider/deployment acceptance and formal sign-off.

Until those gates pass, implementation remains separate from technical verification, UAT and acceptance.
