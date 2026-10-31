# Bidora — repository instructions for coding agents

## Active anti-diversion rules — application development first

The user explicitly requested cleanup, application development and rules against further diversion. These rules supersede historical directions to continue local-fixture tooling below.

1. **One bounded application outcome at a time.** Before coding, name its ticket, persona/operation, intended deliverable and exclusions briefly. Prefer working application/domain code and tests over more setup documentation.
2. **No substitute tooling projects.** Do not create or expand local bootstrap runners, resource collectors, custom credential stores, preflight frameworks, container/VM setup, agent controllers or equivalent renamed infrastructure unless the user separately requests that specific work. A blocked feature is not permission to invent a tooling task.
3. **Support work must directly unblock the current feature.** Use the existing Go/pgx/PostgreSQL stack and standard libraries. If a prerequisite grows beyond the bounded task, stop and present the concrete blocker and smallest options; do not spawn a chain of foundation tickets or silently extend scope.
4. **Preserve approval boundaries.** General permission to develop does not select identity, procurement or production policy. Implement independent source-defined domain invariants where authorized; keep dependent handlers, schemas and state transitions blocked until their contracts/decisions clear. Ask only for the decision actually needed next, without repeatedly requesting already granted permission. Previously approved decisions may be explicitly revisited or superseded by the requesting user; record the new scope and consequences, preserve unrelated approvals and safeguards, and never infer a broader policy change from one revised decision.
5. **Retired tooling stays retired.** `archive/local-fixture/` preserves original local packages, tests and module snapshots. Do not import, execute, restore, finish or expand them without explicit task-specific authorization. Older paths/instructions and review evidence are historical, not current tasks. `backend/tests/architecture/` guards this separation.
6. **Backend only, manual execution.** Do not edit/start web, launch agents, initialize Git, provision servers, start PostgreSQL or modify existing host installations/databases. Any change to these boundaries requires explicit scoped authorization. Preserve existing agent configuration/ledgers.
7. **Prove application progress honestly.** Centralized tests under `backend/tests/`; distinguish domain tests from real database/API/isolation evidence. Archived tests are excluded from active checks, not counted as completed work. Never mark a parent ticket Done from a partial domain slice or configuration tests.
8. **Keep documentation proportional.** Update the active task/evidence and necessary rules, not a new operations framework. Report actual behavior delivered, commands/results and remaining blockers. No unrelated cleanup or speculative feature expansion.
9. **Code-first vertical slices (latest user direction).** Complete one end-to-end operation at a time, prioritizing production code, migrations, contracts and required generated artifacts. During this development phase, defer new test implementation and test-suite execution to the later verification phase. Run only formatting and affected compilation/generation checks needed to avoid accumulating syntactically unusable code. Preserve existing tests without weakening or deleting them. Report each slice as `IMPLEMENTED_NOT_VERIFIED`, never as tested, complete, accepted or release-ready.
10. **Functionality-first MVP sequencing (user-directed).** Complete the approved Option A MVP application functionality before the later verification/release-hardening phase. Test implementation/execution, deployment evidence, independent security review, UAT and formal acceptance are deferred for now, not waived or passed. Continue designing source-defined security, authorization, isolation, replay and rollback invariants into production code even though their proof is deferred. Report functional implementation separately, do not deploy, and do not mark broad parent tickets Done until their deferred gates are eventually satisfied.
11. **Human-readable progress prompts.** For each active ticket/slice, explain in plain language why the functionality is being developed, avoid leading with internal process detail, and end each report or blocker with the exact recommended command the user can enter next.

Current task/evidence: `backend/docs/application-development.md`. Architecture and security rules below still apply.

### Included MVP tender-document AI flow

The requesting user explicitly included this functional sequence in the MVP: users upload tender documents, AI analysis starts, AI proposes tender facts/requirements with exact source citations, and humans verify or edit those proposals before they become authoritative. This supersedes the earlier scheduling of all AI work as future scope only for this bounded tender-analysis/drafting assistance. It does not authorize AI decisions, automatic `SATISFIED`, uncited claims, arbitrary external-source scraping, procurement evaluation, award, or a production model-provider choice. The user subsequently approved the MVP upload policy: PDF/DOCX only, 25 MiB maximum, private S3-compatible MinIO, ten-minute upload instructions, mandatory SHA-256/byte-size verification, fail-closed quarantine scanning, and analysis queued after completion but unable to process before `CLEAN`. MinIO deployment credentials, bucket creation/policy changes and provider acceptance remain deployment-owned and deferred. The user then selected OpenAI API-key authentication for MVP tender analysis. The current deployment approach is an explicit environment-supplied key and model; never commit, persist, log or return the key. A platform-admin secret configuration is not authorized until encrypted storage/KMS, access, rotation and audit policy are approved. Only exact immutable `CLEAN` document versions may be sent to the fixed OpenAI API, with remote response storage disabled where supported; outputs remain cited `PROPOSED_UNVERIFIED` suggestions requiring human acceptance. Deployment/provider privacy and acceptance evidence remains deferred.

### Approved Option A table inventory and schema start

On 2026-10-18 the user requested all needed tables and to start schemas. Scope this to BID-ADR-006 Option A: maintain the reviewed inventory at `backend/schema/option-a-tables.json`, and implement only entries whose governing policy is approved. A catalog entry is not permission to implement blocked policy. The provider-neutral control registry/routing schema is authorized and now exists; it must not create databases/roles, choose password versus OIDC, or treat a tenant ID as membership authority. Document/workflow/outcome tables remain blocked by the gates recorded per entry. Procurement/exchange/AI schemas are future-release exclusions. Evidence: `backend/docs/database-foundation.md`.

### Approved bounded database-foundation execution

On 2026-10-18 the user requested completion of database-design execution and backend foundations. This authorizes executable migration/query/generated-code work only for the approved unverified manual-capture slice and its tenant-local audit/outbox/idempotency foundation. It does not authorize inventing pending identity, document, bid/workflow, procurement/exchange or production schemas; server/database provisioning; database startup; weakening TLS; or enabling a route without authorization/isolation evidence. Implemented files and remaining gates are recorded in `backend/docs/database-foundation.md`. Continue application transaction orchestration only against reviewed ports; do not call generated repositories directly from HTTP handlers or treat parser checks as PostgreSQL integration proof.

### Selected Go-owned password identity approach

On 2026-10-18 the requesting user selected option 1, email/password, over managed OIDC. This authorizes the bounded control `identity.users` and `identity.authentication_identities` schema using Argon2id PHC strings. The user subsequently approved the exact pilot password profile: Argon2id v19, 64 MiB, 3 iterations, parallelism 2, 16-byte random salt, 32-byte key, and 12–128 valid UTF-8 bytes with no trimming, normalization or composition rules; configuration remains explicit with no fallback defaults. Generic failures with dummy-hash work, verified-email-only login and successful-login rehash on profile change are also approved requirements. The user then confirmed the pilot email identity policy: ASCII dot-atom mailbox form, maximum 254 bytes/local part 64, preserved display spelling, full-address lowercase lookup, no whitespace/display names/comments or provider-specific dot/plus rewriting, and authentication denied generically while unverified. The user then approved email verification tokens: 32 random bytes, SHA-256 at rest, 30-minute expiry, `EMAIL_VERIFY` purpose, one active challenge per user, resend invalidation, atomic single-use verification, generic invalid/expired/consumed results, no raw-token logging/storage and no automatic metadata deletion pending retention policy. The user subsequently approved browser sessions: 32-byte opaque session tokens stored as SHA-256, `__Host-bidora_session` with Secure/HttpOnly/SameSite=Lax/Path=/ and no Domain, 30-minute idle/12-hour absolute lifetime, rotation after authentication/assurance changes, revocation on logout/password/auth-epoch changes, and separate session-bound 32-byte CSRF values stored hashed and supplied memory-only through authenticated bootstrap with validated Origin plus `X-CSRF-Token` on unsafe methods. The user then approved login attempt controls: required fail-closed limiter; identity 5 failures/15 minutes then 15-minute deny; network 30/minute and 100/15 minutes then 15-minute deny; success clears identity failures but not network counters; malformed/unknown identities consume limits; HMAC-SHA-256 scope keys from an explicit secret; Redis may hold ephemeral counters; trusted-proxy client IP must be explicit; denials remain generic. This does not approve a Redis/provider deployment, trusted-proxy values, delivery provider, registration/invitation entry policy, recovery, MFA implementation, memberships, capabilities or exposed authentication routes. Do not implement OIDC in parallel or repeatedly request the provider choice. Remaining identity policies retain their gates in BID-ADR-007 and BID-AUTH-001.

## Approved pilot login HTTP contract

On 2026-10-18 the user approved `POST /v1/auth/login`: strict 4 KiB JSON with only required `email`/`password` strings; exact approved HTTPS Origin without pre-session CSRF; trusted-proxy client-IP resolution and privacy-safe network attempt scope; independent new sessions without revoking existing sessions; 204 with both approved cookies and no body; fixed generic 400/401/403/413/415/429/503 responses, with `Retry-After: 900` for attempt denial. This authorizes an unregistered handler/contract only. Do not activate the route before live PostgreSQL/Redis and deployment proxy evidence.

## Approved browser session/CSRF/context binding

On 2026-10-18 the user approved the bounded browser contract: `__Host-bidora_session` is Secure/HttpOnly/SameSite=Lax/Path `/`; `__Host-bidora_csrf` is Secure/non-HttpOnly/SameSite=Lax/Path `/`; unsafe requests require an exact approved HTTPS Origin, equal CSRF header/cookie values and a session-hash match. Login requires approved Origin without pre-session CSRF; logout revokes then clears both cookies with generic 204 behavior. Capture uses `X-Bidora-Tenant-ID` and positive `X-Bidora-Context-Generation` as selectors only. Do not treat either header as authority or a database locator. This approves contract/primitives, not route activation before live PostgreSQL/Redis/isolation evidence.

## Authorized Google SSO addition — linking policy pending

On 2026-10-18 the user requested Google signup/sign-in SSO in addition to Go-owned passwords, superseding the earlier exclusion of parallel OIDC for this bounded addition. Do not implement account linking until the exact takeover-prevention policy is approved: matching email alone must never silently link credentials, Google email must be verified, and existing password/invited identities require an explicit approved linking flow. Client IDs/secrets, callback origins, nonce/state/PKCE, hosted-domain restrictions and production values remain deployment-specific.

## Selected verification delivery transport

On 2026-10-18 the user selected provider-neutral SMTP for pilot verification email. The adapter must require explicit authenticated STARTTLS with verified trust roots, bounded timeouts and a validated sender; it must not discover ambient mail settings or log recipient/token/credential values. This selects the transport, not a vendor, deployment values, retry/queue policy, sender-domain/DKIM policy or an exposed registration/verification route.

## Approved pilot account entry modes

On 2026-10-18 the user selected both invitation-based entry and open self-registration. Self-registration creates only an unverified identity and password credential; it grants no tenant membership, capability, active context or platform authority. Invitation acceptance remains separately blocked until inviter capability, role assignment, expiry/replay and last-admin rules are approved. The user subsequently approved the pilot password-registration boundary: strict 4 KiB `POST /v1/auth/register`, exact HTTPS Origin, generic 202 for new/existing identities, no membership authority, Redis fail-closed identity 3/hour and network 10/hour limits with 15-minute denial, HMAC scopes, 429 plus `Retry-After: 900`, and SMTP issuance with ambiguous delivery still returning generic 202. No CAPTCHA is required for this increment. Route activation still requires SMTP deployment values and live composed acceptance.

## Approved CSRF-only session bootstrap

On 2026-10-18 the user selected option 1 for `GET /v1/auth/session/bootstrap`, separate from the future canonical `/v1/me`: authenticated current session, exact approved HTTPS Origin, matching CSRF cookie/session hash, no body/query, and a response containing only the session-bound `csrf_token` plus trace metadata. Validation precedes idle touch. This does not approve or partially define `/v1/me`, memberships, capabilities, preferences, active context, required actions or frontend storage.

## Approved narrow capture membership policy

The user confirmed the Option A capture authorization policy on 2026-10-18: only verified/non-disabled users with current `ACTIVE` membership in an `ACTIVE` Tenderpreneur tenant, matching browser context generation and explicit `tenderpreneur.opportunity.capture_external` ALLOW may proceed; any active DENY wins. Role labels alone, platform-admin status, tenant IDs and tenant projections never authorize. Capability changes bump membership context generation and every request must recheck membership, capability, tenant state and routing assignment generation. This authorizes the bounded membership/capability schema and read-side capture authorization only, not role catalogs, grant-management/invitation handlers, last-admin policy or route activation.

## Approved manual-capture request scope

On 2026-10-18 the requesting user replied “approve and proceed” to the manual-capture draft's capture rules, request limits and idempotency retention choice. These choices are approved for the bounded BID-API-003/BID-BE-002 input implementation; do not repeatedly request this same approval. See `backend/docs/manual-capture-contract.md` and `backend/api/openapi/drafts/manual-capture.json`. Implement strict capture binding/validation, source-preserving domain mapping and versioned canonical input/fingerprints. Fingerprints alone do not implement transactional idempotency or grant authority. Identity/MFA/provider and cookie/CSRF/context security, event publication, canonicalization review before persistence, database schema/isolation and runtime acceptance remain separately gated. Do not register an unauthenticated route, fabricate storage or start a database to bypass these gates.

## A. Project identity and current authorization

Bidora is a procurement and tender-management SaaS for platform administrators, procurement organizations and Tenderpreneur bid teams. It supports external tender preparation/evidence and, in later releases, platform publication, sealed submission, opening, evaluation and award. AI assists with grounded analysis; authorized humans decide, approve, submit, score and award.

The user authorized a bounded **local Go ↔ Next.js health slice**, including pinned dependencies, health contracts, a smoke page and tests. That slice now exists; see [README](README.md) and [execution evidence](docs/tickets/12_HEALTH_SLICE_EVIDENCE.md). This does not authorize business features, database/service provisioning, executable business migrations, production infrastructure or Git initialization. Further implementation requires explicit scoped authorization and applicable decision gates. Existing `docs/product/` sources must not be edited without explicit authorization.

BID-ADR-005 is Approved: use existing `web/` as the sole frontend root and `backend/` for Go. The earlier `frontend/` description is resolved; do not create a duplicate app. Existing `<frontend-root>` design placeholders resolve to `web/`. BID-ADR-008 approves Goose migrations and npm with a single `web/package-lock.json`; exact version pins still require compatibility/security verification. Tool/path approval alone does not authorize scaffolding or installation.

## Backend-only delegation boundary

The user subsequently requested a Pi subagent to execute backend tickets on their behalf, with no further web work and an AI usage ceiling. Project setup is in [.pi/README.md](.pi/README.md). The approved Option A backend is the target; future marketplace/AI scope remains separately gated. The worker may execute only eligible Ready backend packets after `/backend-run` confirmation, never invent decisions or mark its own approval gates complete. Preserve `web/`; do not create `frontend/` or hand off to frontend on completion. Stop on blockers, required review, failed checks, budget limits or final backend completion. Persistent default AI reservation is US$5, 2,000,000 tokens and 20 requests; never automatically raise or reset it. These controls apply to delegated workers, not a provider billing guarantee. Controller settings and usage ledgers are not worker-editable.

### Approved local-only foundation increment

On 2026-10-17 the requesting user explicitly approved [packet 13](docs/tickets/13_BACKEND_FOUNDATION_UNBLOCKING.md), including **2 CPUs, 2 GiB RAM and a 10 GiB storage budget**. This authorizes preparing the local child tickets, verifying candidates and, after their technical review gates clear, implementing the bounded PostgreSQL-only backend profile and synthetic harness in BID-FND-004. This is a scoped exception to the earlier health-only limit, not approval of production, identity, procurement policies or unrestricted provisioning. Preserve `web/`, existing health behavior, all original parent gates and the AI budget. Candidate metadata/digests are not installation/security/runtime acceptance. No business schema or real data is authorized.

The user subsequently approved configuring a separate local `bidora` Colima profile without changing the existing default profile or active Docker context. Its requested allocation is 2 CPUs / 2 GiB RAM, 4 GiB root + 4 GiB data disk, with 2 GiB overhead headroom within the 10 GiB storage budget. Startup failed because `qemu-system-x86_64` is missing; both profiles remain stopped. Installing a compatible QEMU build is a new host-tooling step requiring approval. No database/bootstrap/business implementation or resource-limit increase is inferred from VM setup permission.

The user then explicitly requested: “please split and start the sub agents, i want them running”. [FND-005](docs/tickets/foundation/BID-FND-005.md) is the authorized Docker-free preparation child: a disconnected standard-library resource validator, centralized tests and backend-local usage notes. It does not need Docker or unchecked DB tool pins, and it must not enable a database-backed runtime or alter existing health behavior. The user has authorized launch; a separate interactive confirmation is not needed for this requested controller invocation. Retain sequential workers, protected-file guards and all persistent usage ceilings; submit results for user review rather than self-approving.

### Current execution mode: manual

The user subsequently instructed: “okay, lets exclude the sub agent, lets proceed with tickets manually”. Execute scoped tickets directly in the foreground; do not launch or retry subagents unless the user reauthorizes delegation. Keep existing controller configuration/usage evidence intact rather than deleting it. FND-005 has been manually implemented and verified; user review is pending. This change in execution mode does not waive any backend-only, testing, approval or infrastructure boundary. Worker budget counters do not measure the foreground conversation.

### Package reviewer: completed one review; implementation remains manual

The user requested configuration of one backend review agent after installing `pi-subagents`. [Review setup](.pi/REVIEW_AGENT.md) defines project agent `bidora-backend-review`, pinned to GPT-6 Astra with read-only tools, no nested delegation and a three-minute default deadline. Package-native user-level launch/concurrency caps are one. This is configuration authorization only: manual implementation remains the default and no review was launched. The package does not inherit the earlier controller's token/request/dollar reservation caps; do not silently waive them. Obtain explicit acceptance of the disclosed review-run controls or establish suitable enforceable controls before launch. A reviewer cannot approve its own findings as user acceptance or alter ticket state.

The user has now explicitly authorized **one read-only review of BID-FND-005** and accepted the **one-agent, three-minute controls without hard token or dollar caps for this review**. This narrow exception applies only to that package reviewer run, not implementation workers, repeat reviews or future spending. It does not reset or change the old controller ledger. Launch only through the native pi-subagents interface; retain read-only tools, no nested delegation and the configured model/deadline. Record the actual run result separately; authorization alone is not proof of launch.

That authorized review completed as run `b57a481e-3d18-4a14-9e07-82d038321a00` with `no_blocking_findings`; its source-only report did not rerun tests or grant user acceptance. The user then requested proceeding to the next ticket. Continue FND-004 preparation manually; do not resume/retry agents or infer additional spend authorization. Current FND-004 configuration/role proposal and remaining runtime/storage/security gates are in `backend/docs/local-postgres-contract.md`.

### Approved isolated TLS integration endpoints

On 2026-10-18, after confirming no existing endpoints were available, the user requested creation of test endpoints. A Bidora-owned loopback-only synthetic PostgreSQL 15.19 TLS cluster and Redis 8.6.1 TLS-only instance were created under `~/.local/share/bidora/integration/`, within the existing 2 CPU / 2 GiB / 10 GiB budget, without Docker, system links or existing database changes. Random credentials are held only in mode-0600 `test-endpoints.env`; never print or commit them. PostgreSQL uses verified TLS/SCRAM, separated control/tenant migrator/runtime roles and control plus two tenant databases. Redis disables plaintext/default-user access and limits the `bidora` user to `bidora:*` keys. Services are local synthetic integration dependencies only, not production or deployment approval. Initial live migration execution exposed and corrected pre-release Goose dollar-quote annotations and broken cross-statement `IF NOT FOUND` compatibility guards; reviewed migrations now execute to control 7, tenant-common 5 and Tenderpreneur 2. Keep web stopped and do not alter other host services.

### Current local-development direction: no Docker

The user reiterated: **do not start web; continue backend-only**. Do not launch Next.js/frontend services during this work. Keep `web/` untouched.

The user instructed: “for local development, lets not use docker and proceed”, after requesting a focus on generating code. Use native PostgreSQL for the local foundation; Docker/Compose/Colima/QEMU are no longer prerequisites for this increment. Do not install QEMU or restart the stopped profiles. Preserve existing host databases and all web code. Native PostgreSQL tools report 15.17 (Homebrew). Subsequent upstream review confirmed that version falls within security-advisory affected ranges; do not use it for live Bidora integration. The user subsequently approved an isolated native 15.19 installation. Source build and explicit tool-version checks passed at `~/.local/share/bidora/postgresql/15.19/install`; system links/tools remain untouched. See `backend/docs/native-postgres-installation.md` and `backend/docs/native-postgres-security.md`. This is a minimal build without TLS or optional third-party libraries. The user subsequently explicitly approved both local choices (“approvve both”): no TLS only for the owned loopback-only synthetic fixture with SCRAM authentication, and monitored native budgets with possible brief overshoot. Resource ceilings remain 2 CPUs / 2 GiB RAM / 10 GiB storage. This does not waive target/credential/role/bootstrap SQL/logging review, detailed monitor tests, FND-005 acceptance or broader security gates. See `backend/docs/native-postgres-fixture-plan.md` and its bootstrap review draft. No cluster has been initialized or started. Do not adopt, upgrade or alter an existing cluster. FND-004's disconnected environment loader and centralized tests are now technically verified. Keep the approved resource ceilings, explicit isolated-cluster/role review and production/business gates; do not claim native resource enforcement or database readiness from configuration tests.

### Approved bootstrap code implementation — no database startup

The user explicitly approved implementing `backend/docs/native-postgres-bootstrap-review.md` (roles, credentials, bootstrap SQL and safeguards) **without starting PostgreSQL**. Continue this backend-only code/test scope without repeatedly requesting the same implementation approval. `backend/internal/localbootstrap/` now provides fixed SQL plans, in-memory SCRAM credentials, discard-only diagnostics, and a Darwin/Linux private metadata/credential store with descriptor-relative ownership checks, exclusive operation locks and ordered atomic SQL-plan checkpoints. Centralized tests passed on macOS; Linux storage tests were cross-compiled only. See `backend/docs/local-bootstrap-store.md` for limits, including ACL/mount assumptions and the vulnerability-scan toolchain correction. The final pinned Go 1.27.2 scan found no reachable/imported vulnerabilities; the existing unused openpgp module advisory remains. Resume-safe SCRAM reconstruction and `backend/internal/localbudget/` resource decision logic are now unit-verified; see `backend/docs/native-resource-guard.md`. No native OS collector, complete storage accounting, process supervisor, PostgreSQL initialization/runner or SQL executor/reconciliation is complete; no SQL or cluster startup is authorized by this implementation-only approval. Keep web untouched and do not start it. Preserve outstanding FND-005, technical/security, live integration and human acceptance gates; do not self-mark tickets Done.

### Current focus: portable Go/PostgreSQL backend, not local-fixture expansion

The user paused local-fixture work and requested a focus on Go/PostgreSQL backend code suitable for server deployment, then said “proceed”. Prioritize portable backend/data infrastructure. Do not continue expanding Colima/native fixture tooling or require those helper tickets as substitutes for server-compatible application work. The unfinished collector is now preserved under `archive/local-fixture/backend/internal/localbudget/storage*.go`, outside the active backend module. It remains behaviorally unverified; historical compilation is not completion. Original local package/test paths below refer to archived evidence, not code to resume.

The authorized connection-management increment now lives in `backend/internal/postgres/`: pgx v5.11.0, explicit configuration/trust roots, verified TLS/SCRAM, bounded lazy pools/Ping and cancellation-aware shutdown, with centralized tests. See `backend/docs/portable-postgres.md`. It does not import local-fixture helpers and is not wired into the health-only API. Continue only scoped backend work; business schemas/operations need applicable contracts, and real PostgreSQL startup/integration, server provisioning and production deployment are not authorized by this code increment. No web edits or startup. Existing parent gates, manual execution and protected agent evidence remain intact.

## B. Documentation hierarchy

1. Applicable approved requirements/legal obligations and accepted architecture/product decisions. A technical ADR cannot override business/legal requirements without appropriate approval.
2. [Product master](docs/product/00_MASTER_INDEX.md), BRD, PRD, SRD and domain rules in their documented precedence.
3. [Technical designs](docs/technical_design/00_INDEX.md).
4. [Implementation workstreams](docs/implementation/00_INDEX.md), approved task packet and dependency gates.
5. Existing implementation and actually executed tests as evidence of current behavior, not silent requirement authority.

Read the [canonical decision register](docs/technical_design/27_ARCHITECTURE_DECISIONS.md). Only Approved entries are accepted direction; Proposed/Pending Approval is not permission. Legacy ADR numbers have different meanings across source files: qualify them by document and consult the alias mapping. Use Bidora in new artifacts; legacy TenderOS sources remain unchanged until authorized.

Conflict procedure: identify exact source/rule and affected operation; inspect related decisions; document alternatives, impact and owner; pause dependent implementation; request approval; continue only unrelated authorized work. Do not choose the easiest rule or infer approval from silence. Record accepted decision with owner/date/evidence and update cross-references within permitted scope.

## C. Architecture rules

BID-ADR-024 is approved: combine Hexagonal DDD inside modules with a cell-ready deployment model. Shared identity, Platform Administration, membership, entitlement and placement remain global control-plane concerns; tenant-private work resolves a server-owned tenant-to-cell assignment/generation and stays in that cell and organization database; future exchange remains separately governed. A cell ID is placement metadata, never authority or a caller-selected database locator. Cross-cell moves require durable checkpoint/reconciliation and separate authorization. This decision does not authorize cell infrastructure, production topology or an Option A rewrite. New Phase 1 use cases use application-owned abstract units of work and transaction-scoped owner adapters; see `backend/docs/cell-ready-hexagonal-architecture.md`.

- Core transactions live in a Go/Fiber modular monolith; Fiber stays at HTTP boundary. PostgreSQL/pgx/sqlc are the approved data stack.
- Each organization has a private database. Control owns identity/registry/membership/entitlement; tenant owns private bids/pricing/evidence; exchange owns approved public projections and minimum sealed metadata.
- Never join tenant databases or place private drafts/pricing in control/public exchange stores.
- Domain/application depend on ports, not Fiber, SQL drivers, provider SDKs or frontend types. Owner modules expose application interfaces; never call another module's repository or tables directly.
- Commands have explicit transaction, version, idempotency, authorization, audit and event semantics. A transaction commits state/audit/outbox in one database.
- Cross-plane operations use durable checkpoints/reconciliation, not pretend nested atomic transactions.
- NATS JetStream is at-least-once; consumers require inbox and effect-specific idempotency. Redis is ephemeral, never business truth.
- Approval, publication, submission, opening, scoring and award are not unrestricted CRUD status updates.

## D. Database rules

Read [schema](docs/technical_design/06_DATABASE_SCHEMA_DESIGN.md) and [migrations](docs/technical_design/07_DATABASE_MIGRATIONS.md) for the scoped operation. Those SQL examples are proposed designs, not executed migrations.

- Select DB from validated membership/context and server registry only; no fallback tenant. A header/UUID is never authorization.
- Separate provisioner/migrator/runtime/backup credentials; API cannot create databases or change roles.
- Use parameters for SQL values and allowlisted identifiers/sorts. Map rows to domain/DTO types.
- Money is decimal plus currency; timestamps are UTC instants with source timezone/text where relevant. Do not silently guess ambiguous external deadlines.
- Mutable aggregates use expected versions; approved candidates, response/evidence bytes and decisions are immutable. Corrections append history.
- Preserve source-version/evidence/package links. Soft delete is not legal retention or erasure.
- Migrations are immutable after release, checksummed, verified and compatible through expand-contract. Fleet rollout is bounded/canary with pause/retry. No destructive auto-down or unbounded tenant loop.
- Restore into a new target; reconcile current authorization, objects and external effects before approved cutover.

## E. Backend rules

Read [backend architecture](docs/technical_design/08_BACKEND_ARCHITECTURE.md) and [module contracts](docs/technical_design/09_BACKEND_MODULE_DESIGN.md).

Handlers bind/validate and call use cases. Use cases authorize actor/resource/state/classification and own transaction orchestration. Domain methods enforce invariants; DB constraints defend integrity. Do not hold SQL transactions during scanning, rendering, email, model or KMS calls. Context cancellation and bounded external deadlines are required.

Use stable safe error codes and correct unauthenticated/forbidden/concealed-not-found/conflict behavior. Preserve wrapped diagnostic causes without leaking SQL, secrets or private content. No global mutable state or swallowed errors. Job execution and result download revalidate current permissions. A 202 response is pending work, not business success.

## F. Frontend rules

Read [screens](docs/technical_design/12_FRONTEND_ARCHITECTURE.md), [components](docs/technical_design/13_FRONTEND_COMPONENT_DESIGN.md), [state](docs/technical_design/14_STATE_MANAGEMENT_DATA_FLOW.md).

Next.js App Router/strict TypeScript/Tailwind/shadcn are approved. Server Components load authorized initial views through Go APIs; no direct DB or privileged internal-service access. Client components handle interaction; design primitives contain no domain policy. Template assets/scripts must be licensed and audited before execution.

Query keys include tenant/context; private SSR requests are no-store/request-scoped. Switch/logout cancels pending requests and clears sensitive state; reject stale generation responses. Never load pricing then hide with CSS. Do not store session tokens or private bid drafts in localStorage. No offline approval/submission queue.

Every accepted page has real API data, proper linked-record controls, explicit timezones/currency, loading/empty/validation/forbidden/not-found/conflict/network/server-error states and safe retry. Preserve permitted input on failure; never silently overwrite stale revisions. WCAG 2.2 AA, keyboard, focus, screen-reader and mobile essential-flow checks are required. Hiding actions does not authorize APIs.

## G. API and event contracts

Read [API cards](docs/technical_design/10_API_CONTRACT_DESIGN.md) and [events](docs/technical_design/17_EVENT_MESSAGING_DESIGN.md). Approve per-operation OpenAPI/event contracts before handlers or dependent UI. Phase numbering does not postpone contract design.

Define request/response, auth/capability, validation, state guards, errors, concurrency/idempotency, audit and negative tests. JSON snake_case, decimal strings, ISO timestamps, stable cursor sort and bounded filters. Do not expose raw object keys/DB locators. Generated clients/types must match reviewed schemas; no hand-edited generated files. Breaking changes require versioned/approved migration plan. Event actor/tenant fields are provenance, not bearer credentials.

## H. Security and procurement integrity

- Default deny; explicit scopes and assurance. Platform admin has no automatic private-tenant access; support grant must be approved, scoped, expiring and audited.
- Secure session cookies, CSRF, rotation/revocation and MFA policies; never invent approved identity/provider choices.
- Clean exact object version before evidence/preview use; quarantine scanner failures. Sandbox parsers and enforce approved resource/type limits.
- SSRF-safe external fetch, sanitized rich text/previews, formula-safe exports, parameterized SQL and minimum egress.
- No secrets, signed URLs, raw documents, pricing or unnecessary personal data in logs/telemetry/errors/screenshots.
- Mandatory unresolved requirements block readiness unless an explicitly approved exception policy applies. Undefined override/self-approval policy is a blocker.
- Approved response changes invalidate approval applicability; preserve historical decisions.
- External submission records distinguish reported action time from server evidence capture; do not claim Bidora transmitted or external portal accepted a bid without supported proof/integration.
- Future sealed-bid acceptance/time/key-release rules are blocked pending BID-ADR-009/010 and independent review. Encryption at rest alone is not bid sealing.
- AI text is advisory/untrusted; authorized retrieval and citations required; model cannot approve, submit, open, score conclusively or award.

## I. Testing and evidence rules

Go test files must be centralized under `backend/tests/`, grouped by component or test layer, and retain Go's `*_test.go` naming convention. Use external test packages importing the production APIs; do not place tests beside production code or export private implementation details solely for tests. `cd backend && go test ./...` must discover the centralized suites.

Every affected change includes unit/domain, DB integration, API contract, positive/negative policy and relevant UI tests. Data changes include cross-tenant rows/counts/objects/jobs/exports/caches; approval changes include edit/revoke races and stale evidence; worker changes include duplicate/out-of-order/crash/ambiguous effects. Test migrations from clean and supported previous schemas, not only fresh bootstrap.

Use real dependencies for accepted E2E via Testcontainers/Playwright as applicable; unit fakes are permitted, production mocks are not completion. Include accessibility, scan/security, performance, restore or AI evaluations when relevant. Synthetic fixtures only. No claim that tests passed without actual execution; disclose skipped/failed/unavailable checks and scope. Never weaken security or assertions to make CI green. See [QA](docs/technical_design/24_QA_TEST_ARCHITECTURE.md).

## J. Execution workflow

1. Identify requested persona/business outcome and permitted change scope.
2. Read applicable product requirements/domain rules.
3. Read governing technical designs and current decision statuses.
4. Read implementation workstream, [backlog](docs/implementation/21_IMPLEMENTATION_BACKLOG.md) and [dependency matrix](docs/implementation/23_DEPENDENCY_MATRIX.md).
5. Identify missing prerequisite approvals/contracts/schema/evidence; block only affected work.
6. Prepare a bounded change/test/recovery plan; implement only explicitly authorized scope.
7. Follow contract → reviewed schema/domain → backend policy/transaction → frontend → integrated verification.
8. Execute applicable checks and inspect actual output; review data isolation, privacy and retry implications.
9. Update authorized documentation/evidence with truthful completion status.
10. Report files, actual test results, unresolved risks and deviations.

## K. Prohibited behaviors

No invented procurement rules or default approvals; no authorization bypass or default tenant; no private cross-tenant joins; no direct cross-module persistence; no constraint weakening; no unapproved infrastructure/provider/framework; no arbitrary source scraping or model tools; no generic status CRUD for governed transitions; no unrelated edits/deletions; no secret exposure; no fake test/UAT/production-readiness claims. Do not change `docs/product/` just to make implementation conform to an easier design.

## L. Task completion report

Use this format for future work:

- **Scope/status:** task ID, persona operation, DESIGNED / IMPLEMENTED_NOT_VERIFIED / VERIFIED_TECHNICALLY / UAT_PENDING / ACCEPTED / BLOCKED.
- **Requirements/decisions:** actual source IDs/paths, accepted ADRs and unresolved blockers.
- **Files changed:** paths and purpose; schema/API/event versions and generated artifacts.
- **Behavior:** state transitions, authorization, audit/evidence and retry/concurrency effects.
- **Verification:** exact commands, actual pass/fail/skipped results, environment/tool versions and artifact paths.
- **Risks/deviations:** privacy/security, compatibility, untested assumptions and remaining work.
- **Release:** feature flag/deployment state, migration recovery, required UAT/sign-offs; do not equate merge with acceptance.

For documentation-only tasks report file/link/structure/integrity checks separately from application tests. Consult [documentation quality report](docs/technical_design/QUALITY_REPORT.md) for current limitations.
