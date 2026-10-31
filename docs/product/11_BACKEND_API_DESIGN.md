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

