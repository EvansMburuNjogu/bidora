# BID-API-010 — Define shared portal and platform-administration contracts

## Metadata
- Ticket ID: BID-API-010
- Epic: E-API
- Type: Design
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-API-001, BID-ARC-005, BID-ARC-010
- Blockers: Remaining typed settings/privacy contracts, support-access policy and retention/legal-hold policy
- Milestone: M7

## 1. Objective
Freeze contracts for portal discovery/routing, platform administrators, typed settings, countries, support access and retention-policy administration.
## 2. Business Context
One login must route each user safely while Platform Administrators perform real governed operations.
## 3. Scope
`/v1/me` portal result, platform bootstrap/grant/revoke/list, setting draft/test/activate/rollback, country/reference versions, privacy-notice acknowledgement, data access/export/correction/deletion/restriction requests, support request/approval/session/revoke and retention/legal-hold lifecycle cards.
## 4. Out of Scope
Plan/payment operations, Tender Assistant operations, private support bypass and destructive retention without approved policy.
## 5. Technical Requirements
Exact capabilities, TOTP assurance, final-owner guard, requester identity/tenant authority, request/hold/policy versions, partial approval/denial, idempotency, safe errors, secret-reference-only fields, audit/outbox effects and no unauthorized tenant-private payloads.
## 6. Proposed Implementation Approach
Build actor/state matrices; split OpenAPI groups by operation; review schemas, errors and examples; generate compatibility fixtures before handlers.
## 7. Dependencies
[API conventions](BID-API-001.md), [identity decision](../architecture/BID-ARC-005.md), [Phase 1 packet](../architecture/BID-ARC-010.md).
## 8. Acceptance Criteria
- Given a portal type without current authority, then the contract cannot imply access.
- Given final-owner removal, then a stable conflict is defined.
- Given a secret setting, then no read response can contain its value.
- Given deletion request with retained legal/hold classes, then the contract represents partial completion without claiming full erasure.
## 9. Testing Requirements
Schema validation, negative examples, stale-version/idempotency cases and generated-client drift checks.
## 10. Definition of Done
Reviewed operation cards and schemas with security/product approval; no runtime claim.
## 11. References
[Phase 1 design](../../../backend/docs/phase-1-portal-functional-design.md), [privacy design](../../../backend/docs/phase-1-data-privacy-retention-deletion.md), [gap audit](../14_PHASE_1_PORTAL_GAP_AUDIT.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Do not create one generic settings endpoint that bypasses typed validation.

### Shared portal-routing contract slice — APPROVED / IMPLEMENTED_NOT_VERIFIED

The requesting user explicitly approved `backend/api/openapi/drafts/me-portal-routing.json` as the implementation contract. It defines strict `GET /v1/me` routing and one-time `POST /v1/me/account-type` selection. A verified `UNASSIGNED` account may self-select only `TENDERPRENEUR` or `PROCUREMENT`; `PLATFORM_ADMIN` remains bootstrap/invitation-only. Routing metadata grants no membership, capability, active context, subscription or database authority. The bounded non-platform runtime slice now has control migration `00014`, sqlc queries, an Identity domain value, transactional application/persistence services, strict Fiber handlers and pilot composition. Unknown persisted state and `PLATFORM_ADMIN` fail closed because platform assignment/role implementation remains blocked. Migration execution, race/API tests and acceptance remain deferred. The broader BID-API-010 settings, support, privacy and platform-administration contracts remain Blocked.

### Typed SMTP/OpenAI settings contract — DRAFTED_NOT_APPROVED

`backend/api/openapi/drafts/platform-integration-settings.json` and `backend/docs/platform-integration-settings-contract.md` now define the first remaining settings slice: immutable typed SMTP/OpenAI versions, bounded synthetic tests, redacted reads, exact activation/rollback proposals and the approved 30-minute maker/checker execution boundary. SMTP remains authenticated STARTTLS-only; OpenAI remains fixed to the Responses API with `store:false`; secret references are write-only and no generic setting keys are accepted. This is source contract work only. Product/security approval, secret-manager namespace policy, BE-014 persistence, provider adapters, handlers, runtime mounting and verification remain open. PesaPal, countries/reference data, privacy, support and retention contracts remain outside this bounded draft.

### Platform authority policy and lifecycle contract — APPROVED / PERSISTENCE_IMPLEMENTED_NOT_VERIFIED

The requesting user explicitly approved P1–P6 in `backend/docs/platform-admin-authority-decision-packet.md`: six source-defined fixed-role bundles, deployment-only first-owner bootstrap eligibility, no-self-grant boundaries, auth-epoch/session effects, final-effective-owner serialization and exact maker/checker policy. `backend/api/openapi/drafts/platform-administrator-lifecycle.json` now defines the non-HTTP bootstrap contract and strict Platform Admin role catalogue, invitations, activation, directory/history, role and identity changes, approval/rejection and administrator-session revocation operations. It also extends the `/v1/me` draft with an optional platform context generation selector needed by those APIs. The contract grants no tenant-private access. The requesting user subsequently approved it and authorized the bounded control-plane persistence now represented by migration `00015`, module-owned sqlc artifacts and bootstrap/authority adapters. HTTP handlers, application lifecycle orchestration, bootstrap executable, deployment and web work remain separately gated; the broader settings, support and privacy portions of BID-API-010 remain Blocked.

### Countries/reference-data policy — APPROVED_POLICY_IMPLEMENTATION_BLOCKED

`backend/docs/countries-reference-data-decision-packet.md` records approved P1–P9 policy for a reviewed source-controlled manifest and minimal Kenya Phase 1 pack (`KE`/`KES`/`Africa/Nairobi`/`en-KE`). The requesting user approved review decisions A1–A4/C1–C4: external licensing attestation; Operations Administrator maker plus distinct Platform Owner activation checker; deployment-owned tzdb compatibility; server-known manifest selection rather than raw content; explicit `NONE` holiday coverage; `LEGACY_PRE_REFERENCE_CATALOGUE` history; and no initial CLDR, tender classifications or holidays. Tender classifications remain omitted pending procurement/legal review. Country metadata never grants authority or selects a cell/database; updates never rewrite historical records; holidays never alter tender deadlines.

The user approved amended P1–P9 as policy only and explicitly kept implementation and every active Kenya release blocked. Exact ISO 3166-1, ISO 4217/SIX, IANA tzdb and IANA Language Subtag Registry evidence plus Product/address review, deployment compatibility evidence and the reviewed manifest artifact/digest remains absent. No OpenAPI, schema, seed, handler, route, import mechanism or runtime activation is authorized. Privacy, support and retention portions remain separately blocked.

### Privacy notice/request lifecycle — DRAFTED_PENDING_APPROVAL

`backend/docs/privacy-request-contract-decision-packet.md` proposes D1–D12 for immutable notices, exact individual/organization request types, fresh requester verification, immutable category scoping, partial dispositions, owner-module correction/restriction, private short-lived exports, non-destructive lifecycle/checkpoints and stable safe evidence. Selective policy approval now covers notice-versus-consent, external exceptional-requester/objection channels, a narrow unimplemented catalogue-v2 `PRIVACY_ADMINISTRATOR`, an unimplemented Organization Administrator privacy-request capability, checked notice governance, affiliation denial, checked sensitive export release and deletion intake limited to scoping/`LEGAL_OR_HOLD_REVIEW`.

D1–D12 remain incomplete and unimplemented. Lawful-basis/purpose and jurisdiction/deadline matrices plus every export, correction, restriction and destructive-effect matrix remain pending accountable privacy/legal evidence. Export generation/release, correction/restriction effects, deletion/anonymization approval, organization closure and every destructive transition remain unavailable. No OpenAPI, role/capability change, schema, migration, route, request, worker or effect is authorized. Countries P1–P9 policy is approved but implementation/activation evidence remains blocked; support and retention execution remain separately blocked.

### Controlled support-access policy — DRAFTED_PENDING_APPROVAL

`backend/docs/controlled-support-access-decision-packet.md` and `backend/docs/controlled-support-access-review.md` record selective policy approval only. T1–T6, D1 option 1, D2, D3 option 1, D4–D6 and B1 approve narrow tenant case/access semantics, tenant adoption of platform-recorded cases, one current TOTP approver, system-known conflict checks, continuing approver authority, immutable closure, restricted minimized case text, tenant-nominated targets, aggregate membership diagnostics, versioned owner DTO/codes, five-scope/twenty-target limits, tenant approval/evidence for control-origin diagnostics and no break glass. Exact capability identifiers and SS1–SS6, E1–E5 and R1–R5 remain pending. `backend/docs/controlled-support-session-evidence-fencing-decision.md` presents the separate ballot, corrects `__Host-` to require `Path=/`, and recommends a 15-second single-use control permit, ten-second owner deadline and final control commit serialized with revocation. The recommendation is not approved, and tenant SQL cannot be treated as atomically observing control revocation. Private content/downloads, mutations, asynchronous work, impersonation and broad search/export remain excluded. The user directed no implementation; no OpenAPI, capability, schema, migration, route, session issuance, tenant access/evidence or runtime is authorized. Countries P1–P9 policy is approved but implementation/activation evidence remains blocked; privacy D1–D12 remain selectively approved/incomplete and retention execution remains separately blocked.

### Retention/legal-hold authority — DRAFTED_PENDING_APPROVAL

`backend/docs/retention-legal-hold-authority-decision-packet.md` proposes R1–R13 for fixed record classes, immutable complete policy versions without default durations, external accountable legal-validation evidence, narrow Security/Audit capability additions, typed protective holds, immediate destructive fencing while holds propagate, maker/checker release, content-free previews, future finite checkpoints and restore reconciliation. Policy activation, hold-release completion, deletion, anonymization and backup expiry remain explicitly unavailable until qualified legal/privacy, security, restore and separate implementation gates clear. No OpenAPI, schema, migration, route, hold, duration, worker or destructive effect is authorized until the requesting user and accountable owners approve or amend R1–R13. Countries P1–P9 policy is approved but implementation/activation evidence remains blocked; privacy D1–D12 and support S1–S13 remain pending.
