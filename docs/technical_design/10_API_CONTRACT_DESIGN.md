# API contract design

Status: proposed wire contracts, no OpenAPI artifacts or handlers created. Sources: [API specification](../product/11_BACKEND_API_DESIGN.md), [SRD](../product/04_SRD.md) API-001–008, PR-TP-004–014. Decisions [007, 009–012, 016–017](27_ARCHITECTURE_DECISIONS.md). Execution: [contracts workstream](../implementation/07_BACKEND_CONTRACTS_IMPLEMENTATION.md).

## Universal contract

Base `/v1`. JSON snake_case; opaque UUID strings; RFC3339 instants; IANA timezone separately; decimal money as strings. Unknown command properties are rejected in strict request schemas. Output additions may be compatible; enum additions require consumer compatibility review. Never expose object keys, DB locators, secrets, hashes of credentials or stack traces.

Success `{data,meta:{trace_id,version?}}`; errors `{error:{code,message,field_errors:[{path,code,message}],trace_id,retryable}}`. Lists use opaque cursor over stable sort `(created_at,id)` with allowlisted filters and bounded `limit` (proposed default 25, max 100; tune before contract approval). Cursor includes filter/sort binding and must be integrity-protected; it is not authorization. Errors: 400 syntax, 401 identity, 403 capability, 404 concealed resource, 409 version/state/idempotency/context conflict, 422 semantic validation, 429 rate/quota with Retry-After where known, 503 dependency unavailable.

All cookie-authenticated writes require CSRF. Context must be validated against active membership. `X-Workspace-ID` is a proposed selector and stale-context guard, not authority; mismatch returns CONTEXT_STALE. Authenticated APIs return private/no-store cache directives. Request ID is a validated correlation suggestion, not audit identity.

Mutable commands require `If-Match: "<version>"`; missing required precondition returns proposed 428, stale returns 409 VERSION_CONFLICT to match product convention. Critical creates/transitions require `Idempotency-Key`; acquisition is scoped by tenant/actor/operation and request hash. Same key/different body → 409; completed replay reauthorizes then returns the original safe result; in-progress returns 409 OPERATION_IN_PROGRESS with retry hint. Irreversible domain uniqueness remains after key expiry.

## Shared DTOs

- Identity: `{id,name,email,verified}`; Session: `{expires_at,assurance,step_up_until}`. `/me` adds active_context, memberships, capabilities, preferences, required_actions as source specification defines.
- Job: `{id,kind,status,progress?,result_id?,error_code?,retryable}`. Status QUEUED/RUNNING/SUCCEEDED/FAILED_RETRYABLE/FAILED_FINAL/CANCEL_REQUESTED/CANCELLED. Progress is nullable, never fabricated. 202 includes `Location` for proposed `GET /v1/shared/jobs/{id}`; job polling reauthorizes target and fields.
- Document: `{id,title,classification,version,processing_status,actions}`; bytes remain inaccessible until CLEAN and AVAILABLE.
- Bid: `{id,opportunity_id,source_version_id,state,version,owner_id,actions}`.
- Requirement: `{id,text,source:{document_version_id,location},mandatory,status,owner_id,due_at,evidence,verification_state,version}`.
- Candidate: `{id,bid_id,revision,manifest_hash,policy_version,approval_state,applicability}`; no pricing fields for unauthorized readers.
- Package: `{id,candidate_id,status,manifest_hash?,generated_at?,actions}`; authorization-protected artifact download is separate.
- Evidence: `{id,bid_id,package_id,proof_document_version_id,channel,external_reference?,reported_submitted_at,recorded_at,verification_state}`.

The tables below are endpoint cards using the universal contract above. Every row inherits error/auth/idempotency/concurrency rules; explicit differences override only technical defaults, not domain policy. All writes audit outcome and target; auth failures use a security event without leaking account existence. Request schemas list required fields unless marked `?`. `I` = required idempotency; `V` = expected version; `—` = read or challenge-specific replay control. All paths below are relative to `/v1`.

## Identity and tenant contracts

| Endpoint / method / purpose | Authentication / authorization | Request → response | Validation / invariants / errors | Replay / audit / acceptance |
|---|---|---|---|---|
| POST /auth/register | Public, throttled | `{email,password,name}` → 202 generic acknowledgement | Password policy and email syntax; never reveal existing identity; ADR-007 blocks final credentials contract | I recommended; RegistrationRequested; duplicate email timing/content enumeration tests |
| POST /auth/verify-email | Public single-use challenge | `{token}` → verification acknowledgement | Unexpired hashed challenge; CHALLENGE_INVALID; no automatic elevated session | Atomic consume; IdentityVerified; replay/expiry tests |
| POST /auth/login | Public, throttled | `{email,password}` → session or MFA challenge | Generic AUTH_INVALID; no workspace authority before membership; rotate session | Challenge replay control, no login idempotency cache; SessionCreated; fixation/throttling tests |
| POST /auth/mfa/verify | Pending challenge or session | `{challenge_id,code}` → session assurance | Factor ownership, retry budget and challenge expiry | Consume challenge; MFAConfirmed; replay/wrong-factor tests |
| POST /auth/recovery/start | Public, throttled | `{email}` → 202 generic acknowledgement | Enumeration-resistant; notification side effect not account disclosure | Dedup/time policy; RecoveryRequested; existing/unknown equivalent response |
| POST /auth/recovery/complete | Recovery challenge | `{token,new_password}` → acknowledgement | Single use, password policy, revoke affected sessions | Conditional consume; RecoveryCompleted; concurrent consume and revocation tests |
| POST /auth/logout | Current session + CSRF | `{}` → acknowledgement | Revokes server session and clears cookie; safe repeated logout | Idempotent action; SessionRevoked; old cookie denied |
| GET /me | Session | no body → bootstrap DTO | Return only authorized memberships; safe expired session 401 | —; access telemetry; context and role refresh tests |
| POST /workspaces (proposed) | Verified session; workspace creation entitlement | `{name,tenant_type,country,timezone,locale}` → 202 Job | Pilot TENDERPRENEUR only if ADR-006 accepted; no DB/role names accepted | I; TenantProvisioningRequested; duplicate/restart/partial failure tests |
| POST /context/select | Session + CSRF; active membership | `{tenant_id}` → updated bootstrap/context generation | Registry ACTIVE, compatible schema; TENANT_NOT_ACTIVE, CONTEXT_STALE | I; ContextSelected; forged ID/multi-tab race tests |
| POST /shared/invitations | Tenant user; invite capability and grantable roles | `{email,role_ids,expires_at?}` → invitation ID/status | Approved expiry, role scopes, quotas; cannot grant platform roles | I; MemberInvited; privilege escalation and duplicate tests |
| PATCH /shared/memberships/{id} | Tenant role administrator | `{role_ids,state}` → membership/version | Last-admin lock, explicit deny/scope rules; LAST_ADMIN | V+I; MembershipChanged; concurrent last-admin removal tests |
| GET /shared/jobs/{id} (proposed) | Requester or authorized operator scoped to target | no body → Job | Recheck membership/resource; conceal foreign job and result refs | —; access telemetry; revoked user/foreign job denial |

Session list/revoke and invitation acceptance need additional approved OpenAPI cards before their implementation; identity ADR decides OIDC versus password-specific contracts. They are not silently considered complete here.

## Tender preparation contracts

| Endpoint / method / purpose | Authentication / authorization | Request → response | Validation / invariants / errors | Replay / audit / acceptance |
|---|---|---|---|---|
| POST /tenderpreneur/opportunities/manual | Tenant session; opportunity manage | `{title,buyer_name,source_uri?,source_document_version_id?,closes_at?,source_timezone?,source_date_text?}` → 201 opportunity | Preserve unknown facts; no automatic URL fetch; invalid timezone 422 | I; ExternalTenderImported; manual capture without AI, invalid/ambiguous date tests |
| POST /tenderpreneur/opportunities/{id}/verify-facts | Opportunity verifier | `{facts,source_refs}` → immutable source version | Fact schema allowlist, document permission; dates require source/zone; SOURCE_UNVERIFIED if incomplete for next transition | V+I; OpportunityVerified; source version immutability tests |
| POST /tenderpreneur/opportunities/{id}/create-workspace | Bid manager | `{source_version_id,owner_id,lot_key?,variant_reason?}` → 201 Bid | Verified snapshot; active assigned member; RULE-BID-001 uniqueness | V+I; BidWorkspaceCreated; duplicate/retry/wrong source tests |
| GET /tenderpreneur/bids | Tenant read capability | filters `state,owner_id`, cursor, limit → Bid list | Allowed filters/sort; count must be authorization-filtered | —; access telemetry; pagination and foreign-tenant tests |
| GET /tenderpreneur/bids/{id} (explicit read contract) | Bid scope | no body → Bid/detail | Pricing omitted without independent capability | —; access telemetry; concealed 404 and field filtering |
| POST /tenderpreneur/bids/{id}/decision | Authorized human decision-maker | `{decision,rationale}` → decision/history version | ADR-011/012-approved transitions; nonblank reason | V+I; BidDecisionRecorded; AI/service actor cannot decide |
| POST /tenderpreneur/bids/{id}/requirements | Bid manager | `{text,source,mandatory,category,owner_id?,due_at?}` → 201 Requirement | Exact source version/location; owner scope; no automatic SATISFIED | V+I; RequirementCreated; invalid source/owner tests |
| PATCH /tenderpreneur/bids/{id}/requirements/{rid} | Requirement edit/review capability | `{status?,owner_id?,due_at?,text?}` → Requirement | RULE-BID-003; status transition validates evidence/response; material edits invalidate candidate applicability | V; RequirementChanged; unsupported SATISFIED and stale write tests |
| POST /tenderpreneur/bids/{id}/requirements/{rid}/evidence | Authorized contributor | `{document_version_id}` or `{response_version_id}` → link | Exactly one target; clean/current eligibility; expiry policy; target belongs to permitted tenant/bid | I+V; EvidenceLinked; expired/cross-bid/foreign object tests |
| POST /tenderpreneur/bids/{id}/response-sections | Technical contributor | `{section_id?,title,content,format,expected_revision?}` → response revision | Approved format, safe content bounds; creates immutable revision; never write pricing through technical route | I+V; ResponseRevised; XSS, concurrent edit and approval invalidation tests |
| POST /tenderpreneur/bids/{id}/financial-response | Pricing edit capability | `{currency,lines:[{line_key,description,quantity,unit_price,tax_amount}]}` → financial revision | Decimal strings; tender-approved arithmetic/rounding; no arbitrary float | I+V; FinancialResponseRevised; rounding and denied reader tests |
| POST /tenderpreneur/bids/{id}/submit-for-review | Bid manager | `{policy_version}` → Candidate | Freeze source, requirements, evidence, response/pricing versions; finalization is not approval | I+V; CandidateFinalized; edit/finalize race and manifest tests |
| POST /shared/approvals/{step_id}/decisions | Assigned approver; required assurance | `{candidate_id,decision,reason?}` → immutable decision | Candidate current, step policy, membership and evidence valid; APPROVAL_STALE/POLICY_UNRESOLVED | I+V on candidate/bid aggregate; ApprovalRecorded; self-approval, revocation and race tests |
| GET /tenderpreneur/bids/{id}/readiness | Bid read with classification-filtered findings | no body → `{candidate_id,ready,blockers,warnings,evaluated_at}` | Deterministic current checks; redact restricted evidence details | —; query telemetry; stale approval/expired evidence tests |
| POST /tenderpreneur/bids/{id}/packages | Package generation capability including all content classifications | `{candidate_id,format_profile}` → 202 Job | Approved candidate, current readiness, approved renderer profile | I+V; PackageRequested; duplicate job, storage failure and changed source tests |
| GET /tenderpreneur/bids/{id}/packages/{pid}/download (proposed) | Package content authorization | no body → streamed artifact or scoped URL | Recheck artifact readiness/retention/access; no raw key in DTO | —; PackageDownloaded; expired authority/sensitive pricing tests |
| POST /tenderpreneur/bids/{id}/external-submission-evidence | Submission recorder | `{package_id,proof_document_version_id,channel,external_reference?,reported_submitted_at}` → 201 Evidence | Exact bid/package; clean proof; server sets recorded_at; no claim of external delivery | I; ExternalEvidenceRecorded; wrong package/late capture tests |
| POST /tenderpreneur/bids/{id}/external-submission-evidence/{eid}/confirm (proposed) | Evidence verifier | `{verdict,reason?}` → confirmation | Approved verifier policy; cannot replace proof bytes | I+V; ExternalEvidenceConfirmed; unauthorized/self-confirm policy tests |
| POST /tenderpreneur/bids/{id}/mark-submitted | Submission execute capability | `{evidence_id,candidate_id}` → Bid | Confirmed proof and exact package; exception path disabled until policy approved | I+V; ExternalSubmissionRecorded; missing proof/duplicate/conflict tests |
| POST /tenderpreneur/bids/{id}/outcome | Outcome recorder | `{outcome,reason?,evidence_version_id?,supersedes_id?}` → outcome history | ADR-012 transition policy; corrections append history | I+V; BidOutcomeRecorded; invalid transition/correction chain tests |

## Upload contracts

| Endpoint | Authentication / authorization | Request → response; validations | Errors / replay / audit / tests |
|---|---|---|---|
| POST /shared/uploads/initiate | Tenant document-create capability, quota | `{filename,byte_size,declared_type,classification,document_id?}` → `{upload_id,instructions,expires_at}`; approved size/type, server-owned key | QUOTA_EXCEEDED, TYPE_NOT_ALLOWED; I; UploadInitiated; arbitrary path/oversize tests |
| POST /shared/uploads/{id}/complete | Initiating actor or explicitly authorized resumable role | `{parts:[{number,etag}],client_checksum?}` → 202 Job; verify exact immutable object version, size, server checksum, ownership | UPLOAD_EXPIRED, CHECKSUM_MISMATCH; I; UploadCompleted; duplicate finalize/overwrite tests |
| GET /shared/documents/{id} | Document read/classification policy | Document DTO and processing state, no bytes before CLEAN | Concealed 404; —; access telemetry; quarantine tests |
| GET /shared/documents/{id}/download-url | Document download plus version policy | query `version_id` → `{url,expires_at}` or authorized stream; exact version; short TTL | FILE_QUARANTINED, ACCESS_REVOKED; —; DocumentDownloaded; foreign version/expired URL tests |
| POST /shared/documents/{id}/verify | Human document verifier | `{version_id,verdict,reason?}` → verification ID; immutable findings | VERSION_CONFLICT; I+V; DocumentVerified; verdict does not bypass scan test |

## Realistic wire examples

Identifiers below are synthetic; values are examples, not default business policy.

```json
{
  "title": "Supply of network equipment",
  "buyer_name": "Example Training Institute",
  "source_uri": "https://example.org/procurement/notice-42",
  "closes_at": "2027-03-12T09:00:00Z",
  "source_timezone": "Africa/Nairobi",
  "source_date_text": "12 March 2027 at 12:00 EAT"
}
```

```json
{
  "data": {
    "id": "019c0000-0000-7000-8000-000000000001",
    "status": "QUEUED",
    "kind": "PACKAGE_GENERATION"
  },
  "meta": {"trace_id": "019c0000-0000-7000-8000-000000000002"}
}
```

```json
{
  "error": {
    "code": "APPROVAL_STALE",
    "message": "The response changed after this review candidate was created.",
    "field_errors": [],
    "trace_id": "019c0000-0000-7000-8000-000000000002",
    "retryable": false
  }
}
```

## Future contracts and compatibility

Existing product endpoints for publish, sealed submit, replace/withdraw, opening, scoring and awards remain future contracts. Do not derive handlers from endpoint names alone. ADR-009/010 and procedure policy must supply exact request schemas, authorization, failure/recovery and receipt semantics first. AI endpoints additionally require job budgets, scoped references and citation schema.

These cards become OpenAPI 3.1 under `backend/api/openapi/` only during an authorized implementation task. [Event schemas](17_EVENT_MESSAGING_DESIGN.md) become versioned AsyncAPI artifacts. CI must lint, validate examples, generate/validate Go and TypeScript DTOs, compare breaking changes and run real contract tests. Adding an operation requires its complete card and owner sign-off before handler/frontend implementation. This document does not claim complete machine-readable coverage for every future product endpoint.
