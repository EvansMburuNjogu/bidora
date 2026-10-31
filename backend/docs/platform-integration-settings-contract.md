# Platform integration settings contract

Status: **APPROVED FOR BOUNDED SOURCE IMPLEMENTATION — IMPLEMENTED_NOT_VERIFIED**  
Ticket: BID-API-010, first remaining settings slice  
OpenAPI: `api/openapi/drafts/platform-integration-settings.json`

## Outcome

Define a typed, non-generic Platform Administration boundary for immutable SMTP and OpenAI configuration versions. Authorized administrators may create a version, run a bounded synthetic test, propose activation or rollback, and complete exact-version maker/checker disposition. The requesting user authorized the bounded schema/domain/application/pgx persistence slice. PA-F09 now mounts only the redacted active-version, version-history and exact-change reads. No provider adapter, secret-resolution implementation or configuration mutation route is authorized by this read slice.

PesaPal, countries/reference data, privacy requests, support access and retention/legal-hold operations remain separate contracts because their governing policies are not complete.

## Types and fixed invariants

Only `SMTP` and `OPENAI` are accepted. Callers cannot invent keys, endpoints, provider types, environment names or arbitrary JSON.

- SMTP requires authenticated STARTTLS with verified trust roots. Plaintext SMTP, opportunistic TLS and ambient mail configuration are invalid. Credential and trust-root references are write-only.
- OpenAI uses only `https://api.openai.com/v1/responses`, explicit model and secret/trust references, and `store:false`. A caller cannot supply another endpoint or enable remote response storage.
- Configuration values are immutable after version creation. Test attempts, governance state and activation history append around that immutable configuration.
- Secret references are resolved only through a deployment-configured allowlisted namespace. They are never returned, logged, placed in audit/outbox payloads or used as proof of authority.
- SMTP host connectivity is subject to deployment egress policy and SSRF-safe resolution. Loopback, link-local, metadata and unapproved private destinations fail closed.
- Reads return only redacted booleans showing that required references were configured. Provider diagnostic bodies, credential identifiers and resolved secret versions remain private.

## Authorization matrix

All operations require a verified, enabled `PLATFORM_ADMIN` identity, active TOTP, current auth epoch, exact platform context generation and AAL2. Capabilities are rechecked on every request; path values and account type are never authority.

| Operation | Maker capability | Additional assurance |
|---|---|---|
| Read active/history | `platform.integration.read` | Current AAL2 |
| Create SMTP version | `platform.integration.smtp.manage` | Current AAL2 |
| Test SMTP version | `platform.integration.smtp.test` | Current AAL2 |
| Propose SMTP activation | `platform.integration.smtp.activate` | TOTP AAL2 no older than 15 minutes |
| Create OpenAI version | `platform.integration.ai.manage` | Current AAL2 |
| Test OpenAI version | `platform.integration.ai.test` | Current AAL2 |
| Propose OpenAI activation | `platform.integration.ai.activate` | TOTP AAL2 no older than 15 minutes |
| Propose rollback | `platform.integration.rollback` plus the type-specific activation capability | TOTP AAL2 no older than 15 minutes |
| Approve/reject exact change | distinct effective Platform Owner with `platform.governance.approve` | TOTP AAL2 no older than 15 minutes |

The maker, checker and target exact version are immutable proposal facts. Maker and checker must differ. Approval expires after 30 minutes. Execution rechecks both identities, capabilities, context, assurance, exact version, test applicability, current active version and deployment compatibility inside the command transaction.

## Lifecycle

1. Create an immutable `DRAFT` configuration version using UUID idempotency.
2. Test outside the SQL transaction with bounded deadlines and no tenant data.
3. Persist only an allowlisted redacted result and its non-secret applicability facts.
4. A successful test yields `TEST_PASSED`; failure yields `TEST_FAILED` and leaves the current active version unchanged.
5. An eligible maker proposes `ACTIVATE` or `ROLLBACK` for the exact tested version. The version becomes `PENDING_APPROVAL`; it is not effective.
6. A distinct eligible checker approves and executes or rejects the exact proposal.
7. Successful execution atomically makes the target `ACTIVE`, marks the prior active version `SUPERSEDED`, records immutable decision/audit/outbox facts and preserves all historical versions.
8. Failure or expiry leaves the prior active version effective. A rollback is activation of a historical compatible version; it never rewrites history.

Any configuration change creates a new version and invalidates previous test applicability. Rotation or incompatibility of a referenced deployment secret/trust object also invalidates applicability without revealing its value.

## Synthetic tests

- SMTP sends only a marked synthetic message to the acting administrator's verified identity after TLS, trust and authentication checks. The request cannot choose a recipient.
- OpenAI sends a fixed public synthetic prompt requiring a bounded structured response. It sends no tenant, identity or production content and enforces `store:false`.
- Tests do not hold SQL transactions during network calls. Ambiguous provider outcomes are recorded as failed/unavailable, never as passed.
- Responses expose only stable result codes. Raw provider bodies, SMTP transcripts and secret-bearing errors are excluded.

## Concurrency, replay and evidence

Mutations require an `Idempotency-Key`; version-targeting operations also require quoted `If-Match`. Matching replay reauthorizes and returns the original safe result. Different request fingerprints conflict. Only one active version per type may commit. Concurrent approval, rejection, expiry, activation and rollback serialize on the exact change and setting-type state.

Every committed mutation records minimal audit and outbox evidence. Evidence may include setting type, version/change IDs, action, actor IDs, policy version, stable result code and timestamps. It excludes host credentials, secret references, resolved versions, sender test recipient, model input/output and provider diagnostics.

## Stable outcomes

The contract defines safe invalid/session/context/forbidden/not-found/version/idempotency/state/test/checker/expiry/dependency errors. A provider failure is not returned verbatim. Reads and writes are private/no-store. No response grants tenant access, changes a subscription, sends production tender content or activates another setting type.

## Remaining gates

- Production secret manager/KMS and reference-namespace policy.
- Provider adapters, bounded timeout/retry behavior and egress configuration.
- Reconciliation policy for a provider test left durably `PENDING` after an ambiguous process interruption.
- Real PostgreSQL maker/checker, concurrency, rollback and audit/outbox evidence.
- Mutation-route composition; the redacted read routes are source-composed only.
- Migration execution, web authorization, UAT and release acceptance.
