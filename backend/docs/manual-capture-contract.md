# BID-API-003 — manual tender capture contract

**Request scope APPROVED; overall endpoint BLOCKED on remaining gates.** On **2026-10-18**, the requesting user replied **“approve and proceed”** to the explicit request to approve capture rules, request limits and idempotency retention. This approval covers those choices and their input implementation, not identity/provider policy, event publication, executable database schema or release acceptance. One operation for a Tenderpreneur bid preparer: record external facts without declaring them verified. Machine-readable draft: [OpenAPI 0.1.0-draft](../api/openapi/drafts/manual-capture.json). Existing health contract/routes are unchanged.

Sources: RULE-OPP-001/002, DATA-005, [BID-BE-002](../../docs/tickets/backend/BID-BE-002.md), [API design](../../docs/technical_design/10_API_CONTRACT_DESIGN.md), [event design](../../docs/technical_design/17_EVENT_MESSAGING_DESIGN.md). Approved architecture: BID-ADR-002/003/006/008. This does not approve pending identity, verification, procurement, legal/data-retention or deployment policy.

## Approved request choices

`POST /v1/tenderpreneur/opportunities/manual`

- Explicit nonblank title; buyer name is required but nullable for unknown. Empty supplied strings are retained, not silently normalized into known facts.
- Source HTTP(S) URI and exact document-version reference are optional descriptive provenance. Either, both or neither may be supplied **for unverified capture only**. Missing provenance must remain visible and cannot qualify for later verification/workspace conversion. No URL fetch, redirect following or duplicate merge.
- Raw date wording and raw timezone may be retained without a known instant. If `closes_at` is supplied, require original wording and recognized IANA region/UTC. Strict RFC3339, explicit offset, at most nanosecond precision; reject unknown `-00:00` offset and out-of-range dates rather than guessing/truncating. Normalize the interpreted instant to UTC.
- Capture produces an opportunity and an immutable **unverified** source version. This source-version representation at capture time is a proposed persistence contract, not an existing schema. No owner assignment, Bid/No-Bid, verification, workspace, approval or submission transition.
- Approved request limits: 32 KiB encoded request body, title/buyer 300 characters each, URI 2,048, timezone 128 and original date text 4,096. These are new endpoint proposals, not changes to the health endpoint's 4,096-byte limit. Strict unknown-property rejection; reject client-supplied IDs, tenant/database locators, verification flags and server timestamps.
- Source URI validation must require an absolute HTTP(S) URI with a host and reject credentials/userinfo, opaque URIs and non-HTTP(S) schemes; treat it as untrusted text, not HTML or a fetch target. Document references are opaque IDs, never object keys. Attachments are outside this request; no upload/scanner provisioning is included.

## Domain mapping — important timezone distinction

The wire `closes_at` is an **instant**, as in the existing API design. `2026-10-17T12:00:00Z` with `Africa/Nairobi` means 15:00 locally and is valid. The existing `ParseDeadline` constructor expects a **source-local timestamp** with matching offset. The application input adapter parses the explicit wire instant, resolves the named zone, renders that instant in the source zone, then constructs the deadline with unchanged source wording. It must not pass the UTC wire text directly into that source-local constructor or parse natural-language text to manufacture a date. This mapping is now implemented by `DeadlineFromInstant` and `application.PrepareCapture`; no HTTP handler is registered.

`CapturedFacts` can preserve incomplete raw values; that does not make it a complete API command or an approved source. Its string fields do not distinguish null from an explicit empty string: the immutable source record must therefore also retain the canonical validated request, rather than reconstructing raw input solely from domain getters. Wire requirements are stricter approved request rules. A successful parse/capture never becomes `VERIFIED`.

## Authority and transaction

Proposed transport: opaque `bidora_session` cookie, `X-CSRF-Token`, and `X-Workspace-ID` selector. These names/browser-security conventions still need approval under BID-API-001/002 and ADR-007/016; the draft does not choose the entire identity/MFA/recovery policy.

Before new work **and before replay**, resolve the approved `__Host-bidora_session`, exact HTTPS Origin and double-submit CSRF binding (`__Host-bidora_csrf` equals `X-CSRF-Token` and hash-matches the session), then resolve verified human identity, active membership/current context and capture capability. `X-Bidora-Tenant-ID` plus positive `X-Bidora-Context-Generation` are selectors only. A UUID/header is never authority or a DB locator. Mismatched current context fails without looking up an arbitrary tenant database. Resolve the DB using server registry/context only. The confirmed control-plane read requires verified/non-disabled identity, ACTIVE membership and Tenderpreneur tenant, exact context generation, positive assignment generation, explicit capture ALLOW and no active DENY. Every command must still reauthorize, then enforce the tenant-local lease; do not claim cross-plane atomic authorization.

For a supplied document version, call the document owner's application interface to check current access and required CLEAN/AVAILABLE eligibility for that exact version. Missing and foreign references are concealed consistently; do not substitute a newer version or inspect bytes before eligibility. Do not scan/fetch/render inside the SQL transaction.

Within one tenant transaction: acquire the scoped idempotency record, insert opportunity/version 1 and its immutable unverified source version, append audit, insert outbox event and save the replay result. All succeed or roll back together. No business success before commit. No cross-tenant joins, transaction-to-pool fallback, implicit provisioning or migration. No `If-Match` is needed for a new aggregate; mutations require their own reviewed contracts.

Proposed audit action: `ExternalTenderImported`, with server actor/context, target/source-version IDs, version, timestamp and correlation. Do not put raw source wording/URL, tokens, document bytes or pricing in logs/audit payloads. Proposed event type: `opportunity.external_captured`, schema version 1, minimum ID-only payload; draft schema is `ExternalTenderImportedV1Draft` in the OpenAPI components. It is not a published/accepted event. Event actor/tenant IDs are provenance, not consumer authority.

## Retry and concurrency

Require proposed UUID `Idempotency-Key`, scoped by validated tenant, actor and operation. Compare canonical decoded input: key ordering is irrelevant; missing optional nullable fields and explicit null normalize equivalently; original non-null strings remain byte-preserved. Never trim/change source text for hashing. A different supplied value with the same key conflicts, even if it represents an equivalent instant. Canonicalization implementation and vectors require review before persistence.

Concurrent same-key requests produce one committed opportunity/audit/outbox result. Replays return the same committed IDs/version/time with a new request trace, only after reauthorization and current reference/access checks. Different body conflicts; in-progress requests return a retry hint rather than claiming success. Distinct keys do not deduplicate by title or silently merge source records. A lost/ambiguous commit response is retried using the same key, never automatically a new key.

**Approved retention choice:** keep the idempotency record while its opportunity/history exists, with no automatic TTL expiry. Deletion or tombstoning must follow separately approved retention/erasure policy, not silently permit an earlier committed command to be repeated. This does not authorize real-data retention or deployment and is not implemented by request fingerprinting.

## Errors and response

201 returns only opportunity/source-version IDs, aggregate version 1, `UNVERIFIED`, server UTC creation time and request trace. No raw storage/DB locators, pricing or claim of workspace creation. All responses private/no-store. Safe errors contain stable codes, field paths, generic messages, trace ID and retryability; never echo rejected source values.

| HTTP | Proposed cases |
|---|---|
| 400 | Malformed JSON, unknown properties, invalid/missing required headers |
| 401 | Missing/expired/revoked authentication |
| 403 | Identity/assurance, capability or CSRF check denied |
| 404 | Missing or inaccessible exact source-document reference |
| 409 | CONTEXT_STALE, IDEMPOTENCY_CONFLICT, OPERATION_IN_PROGRESS, ineligible document state |
| 413 / 415 | Proposed request-byte limit exceeded / unsupported media type |
| 422 | Invalid field values, unsupported URI, invalid interpreted date/zone or missing supporting date wording |
| 429 | Approved abuse/quota limits reached; retry hint where applicable |
| 500 | Unexpected internal failure, redacted; not automatically retryable |
| 503 | Dependency/commit outcome unavailable; bounded retry with the same key only where safe |

This operation creates no publicly accessible detail URL. List/detail and source verification remain separate pending contracts; no response `Location` points to an invented implemented route.

## Required verification before handler acceptance

1. Valid unknown-date and explicitly interpreted-date capture; original wording preserved, UTC conversion correct, no auto-verification.
2. Foreign/inactive context, stale context, absent capability/assurance, CSRF and foreign/missing/quarantined document denial; no private existence leaks.
3. Invalid/extra fields, forbidden URI userinfo/schemes, absent timezone/text, unknown offsets, excessive precision and boundary sizes.
4. Same-key concurrency, replay after lost response, different-body conflict, revoked actor/reference on replay; no repeated audit/outbox effects.
5. Real PostgreSQL rollback on source/audit/outbox/idempotency failure and two-tenant isolation. No mock repository as integration proof.
6. Error/cache headers, API schema validation and preservation of the existing health contract.

## Request implementation — 2026-10-18

- `internal/opportunity/transport/capture.go`: strict bounded JSON decoding, exact snake_case properties, duplicate-key rejection (including escaped aliases), required-property/type checks and rejection of invalid UTF-8/unpaired UTF-16 escapes rather than silently changing source wording. Future HTTP code must enforce the byte cap before buffering as well. No route is registered.
- `internal/opportunity/application/capture.go`: approved character/URI/UUID/date validation, nullable input preservation, immutable prepared command and domain mapping. References are syntax-checked only: no existence/access/scan verification is claimed. Input preparation grants no actor/context authority and commits nothing.
- `internal/opportunity/domain/deadline.go`: adds `DeadlineFromInstant`, sharing strict timestamp validation with the existing source-local constructor; original source wording stays unchanged.
- Canonicalization **v1**: fixed field order `title`, `buyer_name`, `source_uri`, `source_document_version_id`, `closes_at`, `source_timezone`, `source_date_text`; explicit null for missing optional nullable fields; Go JSON string encoding with no trimming or Unicode normalization. Empty string differs from null. Original non-null timestamp strings remain in the canonical request even when their normalized instants agree. Fingerprint is SHA-256 over UTF-8 `bidora.capture.v1` + NUL byte + canonical JSON. This is a versioned implementation/vector proposal for persistence review, not a published replay protocol. Future changes require compatibility/version handling, not silently replacing old fingerprints.
- Canonical JSON and domain facts are owned copies. `CanonicalJSON` is a sensitive persistence accessor; never log or return it wholesale. Formatter output and errors are redacted; wrapped diagnostic causes remain available for trusted inspection. Fingerprints are neither authority nor globally scoped deduplication keys.
- `tests/opportunity/capture_request_test.go`: both contract examples through production decoding; fixed canonical vector; null/empty/omission distinctions; source-zone mapping; copy isolation; approved limits; malformed/duplicate/extra fields; URI/UUID failures; safe errors/causes; Unicode preservation and fuzz seeds.

Go 1.27.2 / darwin amd64 checks from `backend/` passed:

```sh
GOPROXY=off go test -count=1 -race ./...
GOPROXY=off go vet ./...
GOPROXY=off go build ./...
GOPROXY=off go test -count=1 -coverpkg=./internal/opportunity/domain,./internal/opportunity/application,./internal/opportunity/transport -coverprofile=/tmp/bidora-capture-coverage.out ./tests/opportunity
go mod verify
GOMAXPROCS=2 GOPROXY=off go test ./tests/opportunity -run '^$' -fuzz '^FuzzCaptureBody$' -fuzztime=3s -parallel=2
```

Combined opportunity domain/application/transport statement coverage **96.1%**; coverage artifact `/tmp/bidora-capture-coverage.out`. Fuzz run passed with **30,117 executions**, two workers; this is bounded testing, not exhaustive security proof. Linux/amd64 active backend build and opportunity test cross-compilation passed with `CGO_ENABLED=0`; Linux runtime was not tested. Cached govulncheck v1.9.0 built/run using verified Go 1.27.2 scanned 13 root packages/22 modules: zero reachable/imported findings; existing module-only GO-2026-5932 (unused openpgp) remains. No dependencies added or versions changed.

## Application transaction orchestration — 2026-10-18

`internal/opportunity/application/capture_execute.go` now implements the reviewed use-case sequence through ports, without Fiber or pgx imports:

1. reject malformed correlation/idempotency/zero command cheaply;
2. reauthorize current actor/tenant/context through `CaptureAuthorizer` on every request and replay;
3. recheck any exact source-document reference before opening the transaction;
4. enter one tenant `CaptureUnitOfWork`, claim/lock scoped idempotency and constant-time compare request hash;
5. return a valid completed replay, reject different-body/pending/inconsistent states, or insert opportunity/source, audit, outbox and completion in order;
6. return success only after the unit of work reports commit success.

UUID idempotency-key spellings are lowercased before key hashing, so equivalent upper/lowercase UUID text cannot create two operation scopes. The request fingerprint still preserves approved non-null body strings exactly. Generated IDs, UTC server time and assignment generation are explicit ports/values; IDs and event provenance confer no authority. Document checks and authorization occur before SQL transaction work; persistence adapters must not perform network calls inside it.

Centralized external-package tests cover new execution, replay reauthorization/document recheck, body conflict, pending/inconsistent state, every transaction-step failure, missing dependencies/input, ID failure, safe wrapped diagnostics and UUID-case canonicalization. Full active race tests, vet/build and module verification passed. Combined opportunity domain/application/transport coverage is **96.3%**. Linux/amd64 build/test cross-compilation passed, not runtime. Fresh vulnerability scan: zero reachable/imported findings; unchanged module-only unused-openpgp advisory. That 96.3% measurement predates and excludes the persistence adapter below. No route, authorization implementation or live transaction exists, so atomic rollback remains pending real PostgreSQL evidence.

## pgx persistence adapter — 2026-10-18

`internal/opportunity/persistence/capture_store.go` now binds the use-case unit of work to an expected tenant ID and assignment generation, then executes generated sqlc operations through `postgres.Pool.WithinTransaction`. The pool supplies a caller/lifecycle/operation-timeout-derived transaction context, owns begin/rollback/commit, and reports ambiguous commit failure without returning success. The adapter implements insert-or-lock idempotency, persisted replay reconstruction, nullable source mapping, affected-row conflict checks, audit evidence and outbox insertion.

Outbox actor provenance was found missing from the original pre-release table. It was corrected with additive `tenant_common/00005_outbox_actor_provenance.sql`, not by rewriting `00001`; common compatibility is now version 5. The not-valid check enforces non-null actors for new rows while preserving an explicit reconciliation gate for hypothetical pre-v5 rows. Actor IDs remain provenance only. Audit/outbox JSON contains IDs/state only, not title, buyer, URI, source date text or document content.

Scripted external-package tests execute the application use case through generated SQL, assert query order, replay locking, tenant/generation lease rejection, actor persistence, evidence-content exclusion, affected-row conflicts and no commit after callback failure. Full race tests, vet/build/module verification, Goose validation, sqlc zero-drift, Linux/amd64 cross-compilation and vulnerability scanning passed. A route-independent integration coordinator now derives the actor only from a validated session and maps the confirmed control grant into the capture authority; caller tenant/context values are still authorization inputs, never authority. Unregistered Fiber middleware enforces session and browser policy before touching idle expiry or exposing the request-local principal. An unregistered Fiber capture handler now binds the approved tenant/context/idempotency headers and strict JSON, derives actor identity from that principal and correlation from the server request ID, rejects queries/media ambiguity, and emits only the reviewed envelope fields and safe error mappings. Combined opportunity domain/application/integration/persistence/transport coverage is **86.0%**. The checksum manifest now covers 24 reviewed inputs, including the subsequently selected identity/session and membership-authorization schema/query inputs. The later live integration executes verified-TLS PostgreSQL begin/commit/rollback, constraints and triggers across two tenant databases, plus a temporary Fiber login/capture/logout flow with real Redis/session/membership adapters. Capture routing now re-resolves the exact active compatible control assignment and selects only an exact server-registered store tuple after authorization; request headers never select a DSN. Production deployment and `cmd/api` route activation are still not claimed.

## Review gates / actual evidence

Capture semantics, limits and no-TTL idempotency retention are approved as recorded above. Cookie/CSRF/context binding and the narrow capture capability policy are approved and implemented through an unregistered boundary. Still review exact document eligibility/error mapping, event schema and live persistence/isolation evidence before route activation; source-verification/workspace/duplicate-resolution policies are not inferred from capture approval.

Preparation checks passed: JSON parsing, all **14 internal references** resolved, exactly one POST operation, required headers/security and response statuses, both example key sets, UNVERIFIED-only creation result and ID-only event payload. These are structural checks only. Full OpenAPI lint, JSON Schema example validation and runtime/DB tests are **not claimed**; Python jsonschema is unavailable and no dependency was installed. No frontend, service startup, migration, generated client or runtime API change. Keep BID-API-003 and BID-BE-002 Blocked until their actual gates clear.
