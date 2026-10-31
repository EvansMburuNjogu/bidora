# BID-API-010C — Countries and reference-data decision packet

Status: **P1–P9 APPROVED POLICY — IMPLEMENTATION AND ACTIVE RELEASE NOT AUTHORIZED**  
Scope: Phase 1 country/reference catalogue authority, immutable versioning, activation and read behavior  
Persona: Operations Administrator maintaining safe choices for tenant onboarding and organization profiles  
Not implementation authority: policy approval authorizes no schema, migration, query, generated artifact, OpenAPI, manifest registration/import, handler, route, seed, worker, runtime activation, test or frontend change.

## 1. Outcome and boundaries

This packet closes only the policy needed to implement the countries/reference-data portion of BID-API-010 and BID-BE-014.

Included:

- the authoritative source and provenance model for country, currency, timezone and locale identifiers;
- the bounded Phase 1 Kenya pack and neutral classification lists;
- immutable versions, scheduling, activation, rollback and historical binding;
- Platform Administration and authenticated consumer API behavior;
- validation, concurrency, idempotency, audit and safe failure rules;
- update cadence and emergency correction behavior.

Excluded:

- procurement-law rules, statutory tender deadlines, tax computation or identifier validation;
- legal conclusions about organization form;
- holiday-driven deadline calculation or automatic date movement;
- tenant provisioning, tenant movement or database selection;
- plans, pricing, invoices, PesaPal, SMTP/OpenAI settings, privacy, retention and support access;
- arbitrary administrator-authored code lists or uploaded reference files;
- frontend work and production/deployment values.

A country pack supplies validated choices and display metadata. It never grants identity, tenant membership, capability, entitlement, placement or access to a tenant database.

## 2. Decision P1 — source authority and provenance

**Approved policy:** Bidora owns a reviewed, source-controlled reference manifest. Production code never discovers or refreshes country data from the network.

Each manifest release records:

- a Bidora schema version and immutable release UUID;
- an operator-visible semantic label;
- the exact upstream standard/version used for each identifier family;
- source title, publisher, canonical source URI, publication or retrieval date, licence/terms note and SHA-256 of the reviewed input artifact where redistribution permits retention;
- a canonical SHA-256 of the normalized Bidora manifest;
- reviewer identity and review timestamp.

Identifier authority is:

| Field | Authority and Phase 1 rule |
|---|---|
| Country | ISO 3166-1 alpha-2 identifier; exact source allowlist, not regex-only acceptance |
| Currency | ISO 4217 alphabetic identifier and minor-unit metadata; exact source allowlist |
| Timezone | IANA Time Zone Database region identifier from the deployment's reviewed pinned tzdb release |
| Locale | Canonical BCP 47 language tag; Phase 1 accepts only explicitly listed tags |
| Holiday | Dated Kenya government gazette or other named official public authority citation, manually reviewed |
| Bidora taxonomy | Source-defined stable Bidora codes, explicitly labelled as product classification rather than law |

A standards identifier is not accepted merely because its shape is valid. Unknown, withdrawn, deprecated or unreviewed identifiers fail closed for new writes.

No third-party text, logo or dataset is copied beyond what the reviewed licence permits. The implementation records provenance metadata rather than treating a package-library default as authority.

Accountable legal/licensing evidence is recorded outside the operational command and immutably bound to the manifest before Operations may act. `platform.reference.manage` does not grant licensing-attestation authority. Platform operations may select only a separately reviewed, server-known immutable manifest digest; callers cannot author pack content or provenance.

## 3. Decision P2 — bounded Phase 1 Kenya pack

**Approved policy:** Phase 1 supports one active market pack:

- country: `KE`;
- display name: `Kenya` in `en-KE`;
- default and only Phase 1 transaction currency: `KES`, with two minor units;
- default and only Phase 1 timezone: `Africa/Nairobi`;
- default and only Phase 1 locale: `en-KE`;
- address profile: `KE_FREEFORM_POSTAL_V1`, which preserves bounded postal and physical address text without claiming postal-address verification;
- calendar: Gregorian.

These are defaults for newly created records, never silent replacements for explicit historical facts. Existing timestamps retain UTC instant, source timezone and source wording where the operation requires them. Money remains exact KES minor units in Commercial operations.

The pack may expose the following **neutral Bidora organization classifications**, which are not legal-form validation:

- `SOLE_PROPRIETOR` — Sole proprietor;
- `PARTNERSHIP` — Partnership;
- `PRIVATE_COMPANY` — Private company;
- `PUBLIC_COMPANY` — Public company;
- `COOPERATIVE` — Cooperative;
- `NON_PROFIT` — Non-profit organization;
- `PUBLIC_SECTOR` — Public-sector organization;
- `OTHER` — Other.

The initial pack contains **no tender classifications**. Codes such as `GOODS`, `WORKS`, `CONSULTANCY_SERVICES` and `NON_CONSULTANCY_SERVICES` remain unavailable until accountable Product and procurement/legal review approves exact neutral meanings and presentation that cannot be mistaken for statutory classification or procedure advice.

Phase 1 does not accept custom category codes. A later reviewed catalogue version may add an approved code, but cannot reuse or change the meaning of an existing code. Display-label corrections create a new immutable version. The initial pack uses reviewed Bidora-owned English display metadata and `KE_FREEFORM_POSTAL_V1`; it contains no CLDR-derived content.

## 4. Decision P3 — public holidays are informational only

**Approved policy:** The initial Kenya pack declares holiday coverage `NONE` and contains no holiday entries. A later release may add holidays only after defining an exact coverage interval; each entry then requires an exact local calendar date, bounded display name and official-source citation. Tentative or uncited holidays cannot enter an activatable version.

Holiday data is informational in Phase 1. It must not:

- calculate or move a tender deadline;
- determine whether a submission was timely;
- change a contractual or statutory period;
- trigger a workflow transition;
- authorize AI to infer a different date.

An ad-hoc gazetted holiday is added through a new version. Historical versions remain unchanged. Tender deadlines continue to preserve source evidence and require authorized human verification under the existing opportunity rules.

## 5. Decision P4 — immutable release and activation model

**Approved policy:** One immutable **reference release** contains one or more complete country packs. Phase 1 permits only the Kenya pack, but the format must not hardcode Kenya into domain interfaces.

Release states are:

- `DRAFT`: immutable content has been created but not proposed for use;
- `SCHEDULED`: exact version is approved for activation at an explicit future UTC instant;
- `ACTIVE`: the sole version offered for new bindings;
- `SUPERSEDED`: formerly active and retained for historical resolution;
- `WITHDRAWN`: never active, or cancelled before activation because review failed.

Content is immutable after creation. Correction means creating a new complete release. There is at most one `ACTIVE` release and at most one future `SCHEDULED` release. Scheduling another version atomically withdraws the prior unactivated schedule.

Activation and rollback are serialized against a singleton catalogue generation. Rollback activates an exact historically active, still deployment-compatible release as a new activation event; it never edits history or deletes intervening releases. A failed or stale activation leaves the current active release unchanged.

The activation worker, if later authorized, uses database time and an exact-version fenced claim. Runtime startup never activates a release and never runs a migration.

## 6. Decision P5 — authority and assurance

**Approved policy:**

- Active-reference and history reads require a verified, enabled `PLATFORM_ADMIN`, current auth epoch, exact platform context generation, current AAL2 and `platform.reference.read`.
- Create, withdraw, schedule, cancel schedule, activate-now and rollback require the same identity conditions plus `platform.reference.manage` and TOTP AAL2 obtained no more than 15 minutes earlier.
- The Platform Owner role is not a wildcard. Account type, role labels, request IDs and release IDs never authorize.
- No tenant/cell/database selector is accepted on Platform Administration reference routes.

An Operations Administrator with `platform.reference.manage` may create or withdraw a draft that selects an exact server-known reviewed manifest digest. Schedule, activate-now and rollback require a distinct effective Platform Owner with `platform.governance.approve`; both actors use TOTP AAL2 no older than 15 minutes and bind the exact immutable digest. Approval expires after 30 minutes, execution rechecks both actors and deployment compatibility, and emergency activate-now has no bypass. If a future country pack contains procurement-law, tax, retention or statutory-deadline rules, it requires a separately approved legal/product contract; those rule types cannot be introduced through this schema.

## 7. Decision P6 — consumer binding and historical behavior

**Approved policy:** Every durable record whose interpretation depends on reference data stores both its stable code and the immutable reference release UUID used at validation. The release UUID is provenance, not authority.

For Phase 1 this applies prospectively to:

- new tenant registry country/timezone/locale bindings;
- organization profile country, organization classification, timezone and locale once those fields use the catalogue;
- Commercial currency metadata where the governing Commercial contract requires it.

A new release affects only subsequent creates or explicit authorized updates. It does not rewrite existing tenants, profiles, money, timestamps, tender classifications or audit evidence. Historical reads resolve against the bound immutable version, including after a code is no longer selectable.

Withdrawal or supersession stops new selection but does not make existing records unreadable. If the active release is unavailable or inconsistent, dependent creates/updates fail closed; reads of already-bound historical records continue from immutable stored data.

Values created before the reference catalogue retain their exact stored code/value with explicit `LEGACY_PRE_REFERENCE_CATALOGUE` provenance. They are never silently assigned the first release UUID. A later evidenced migration may bind only records whose historical validation is proven.

Tenant placement remains server-owned. Country and release identifiers cannot select a cell, cluster, database or secret.

## 8. Decision P7 — update cadence and emergency corrections

**Approved policy:**

- Review upstream country, currency, locale and timezone metadata at least quarterly.
- Review the next calendar year's Kenya holiday sources before publishing that year's holiday data.
- Process official ad-hoc holiday notices and correctness/security issues as an emergency new release without waiting for the quarterly cycle.
- Never auto-import or auto-activate upstream changes.
- Record review outcomes even when no release change is required.

An overdue review does not automatically deactivate an otherwise valid active release. It must be surfaced as an operational health condition; scheduling a successor still requires current review evidence. A known correctness/security problem uses the governed emergency supersession or withdrawal path, never silent mutation.

A timezone database update is a deployment compatibility event as well as reference review. A release cannot activate unless every serving deployment can resolve every listed timezone under its reviewed tzdb. Updating tzdb must not reinterpret already stored UTC instants.

## 9. Decision P8 — HTTP operation contract

**Approved policy:** Any future OpenAPI requires separate implementation authorization and remains bounded to these operations.

### Authenticated consumer read

`GET /v1/reference/countries`

- Requires an authenticated, verified, non-disabled current session and exact approved Origin.
- Accepts no tenant, context, account-type or database selector and no request body.
- Returns only the current active release ID/version, activation timestamp and selectable country packs with codes, display labels, currency/minor units, timezones, locales, address profile, approved neutral classifications, explicit holiday coverage and any cited holidays.
- Does not return reviewer identities, internal source artefact paths or deployment details.
- Uses `Cache-Control: private, no-store` for the browser-authenticated MVP.

### Platform Administration

- `GET /v1/platform/reference/releases` — bounded stable history and status filters.
- `GET /v1/platform/reference/releases/{release_id}` — exact immutable version and safe provenance.
- `POST /v1/platform/reference/releases` — create one immutable draft by selecting an exact server-known reviewed manifest digest; it never accepts pack content or provenance authored by the caller.
- `POST /v1/platform/reference/releases/{release_id}/schedule` — exact future UTC activation or explicit activate-now request.
- `POST /v1/platform/reference/releases/{release_id}/withdraw` — withdraw a never-active draft/schedule.
- `POST /v1/platform/reference/releases/{release_id}/rollback` — activate an exact formerly active compatible release.

Every mutation requires strict JSON, UUID `Idempotency-Key`, quoted `If-Match` catalogue/release version as applicable, exact Origin/CSRF, a bounded reason and server-issued trace metadata. Replays reauthorize. Same key with a different canonical request conflicts. Concurrent schedule, activation, cancellation and rollback serialize on exact versions.

Proposed validation limits:

- release-creation JSON contains only the bounded server-known manifest selector, reason and command metadata—not reference content;
- at most 25 country packs per registered release format, while Phase 1 policy permits exactly one (`KE`);
- at most 25 currencies, 100 timezones, 25 locales, 100 organization classifications, 250 tender classifications and 500 holidays per country pack;
- codes and identifiers are ASCII and source-allowlisted; display labels are valid UTF-8, trimmed and bounded to 200 bytes;
- reason is 1–2,000 UTF-8 bytes; provenance note fields are individually bounded to 2,000 bytes.

The large format bounds are compatibility ceilings, not permission to enable additional countries or codes. Phase 1 creation rejects anything outside P2.

## 10. Decision P9 — failures, evidence and privacy

**Approved policy:** Stable safe failures distinguish:

- `reference_request_invalid`;
- `reference_not_authorized`;
- `reference_release_not_found`;
- `reference_version_conflict`;
- `reference_idempotency_conflict`;
- `reference_state_conflict`;
- `reference_source_unapproved`;
- `reference_identifier_unapproved`;
- `reference_schedule_conflict`;
- `reference_deployment_incompatible`;
- `reference_unavailable`.

No failure returns SQL, filesystem locations, database locators, stack traces or raw upstream documents.

Each committed mutation atomically records the release/action IDs, actor ID, previous/next catalogue generation, reason, policy/schema version, safe result and timestamps in control-plane audit/outbox evidence. It excludes tenant-private content and does not claim that an upstream source was legally approved merely because its identifier parsed.

Reference releases are platform-control data. They contain no user email, tenant-private content, credential, secret reference or personal identifier. Reviewer actor IDs remain restricted audit facts rather than consumer-read fields.

## 11. Persistence and implementation consequences after policy approval

P1–P9 policy approval does **not** authorize source implementation. A later separately explicit implementation authorization could bound a BID-BE-014 increment to:

- additive control-plane tables for immutable releases/packs/items/provenance, catalogue state, activation history and idempotent commands;
- source-defined validation/domain/application ports and pgx/sqlc persistence;
- an OpenAPI draft for the operations in P8;
- compatibility updates and generated sqlc artifacts;
- prospective reference-version linkage only where an already-approved operation needs it.

No schema, migration, query, generated artifact, OpenAPI, seed, manifest registration/import mechanism, handler, route, worker, runtime mounting, frontend, migration execution, existing-record change or remote-data import is authorized. Those remain separate design, execution and verification gates.

The initial Kenya manifest must name the exact reviewed ISO 3166-1, ISO 4217/SIX, IANA tzdb and IANA Language Subtag Registry sources and pass licence review before an activatable release can be separately approved. It contains no CLDR-derived content, tender classifications or holidays, so no CLDR/gazette source is required unless a later release adds those contents. It must also bind Product-reviewed organization classifications/address metadata and deployment-owned tzdb compatibility evidence. Until the exact artifact and digest are separately approved, no active production catalogue exists.

## 12. Deferred verification and acceptance

Later verification must cover:

- clean and upgrade migrations, runtime grants and downgrade/recovery behavior;
- exact identifier allowlists and malformed/duplicate/cross-field rejection;
- only-one-active and only-one-scheduled races in real PostgreSQL;
- stale generation, idempotent replay, activation crash and rollback fencing;
- historic resolution after supersession/withdrawal;
- tzdb deployment incompatibility and dependency failure;
- tenant/cell selector rejection and proof that catalogue values grant no authority;
- audit/outbox atomicity and absence of private/secret data;
- OpenAPI schema/examples and generated-client drift;
- product/security review, Operations Administrator UAT and formal release acceptance.

## 13. Approval record

The requesting user approved amended P1–P9 as policy only:

- **P1:** reviewed source-controlled manifest; no runtime network discovery; explicit provenance and external legal/licensing attestation.
- **P2:** minimal Kenya-only `KE`/`KES`/`Africa/Nairobi`/`en-KE` pack with Bidora-owned organization/address metadata and no CLDR, tender classifications or holidays.
- **P3:** holiday coverage starts as `NONE`; any later cited holidays remain informational and never alter deadlines.
- **P4:** immutable complete releases with one active/one scheduled version and fenced rollback.
- **P5:** Operations Administrator draft maker plus distinct Platform Owner checker for schedule/activate/rollback, recent TOTP and no emergency bypass.
- **P6:** stable code plus immutable release binding; explicit legacy provenance, no retroactive rewrite or routing authority.
- **P7:** quarterly/manual review and governed emergency releases; no automatic import or overdue-review deactivation.
- **P8:** authenticated consumer read plus strict versioned Platform operations selecting only server-known reviewed manifest digests.
- **P9:** stable safe failures and minimal atomic audit/outbox evidence.

The user explicitly kept implementation and every active Kenya release blocked until exact provenance, licensing, deployment compatibility and manifest-artifact/digest evidence is separately approved. Policy approval is not source, deployment or release acceptance.

## 14. Provenance, licensing and authority review

`countries-reference-data-review.md` records approval of A1–A4 and C1–C4. The amendments bind external legal/licensing evidence, Operations-maker/Platform-Owner-checker activation, deployment-owned tzdb compatibility, server-known manifest digests, explicit `NONE` holiday coverage, legacy provenance and a minimal no-CLDR/no-holiday/no-tender-classification Kenya pack.

P1–P9 policy is approved, but no implementation is authorized. Exact ISO 3166-1, ISO 4217/SIX, IANA tzdb and IANA Language Subtag Registry evidence remains absent, as do Product-reviewed organization/address metadata, deployment compatibility evidence and the exact approved artifact digest. The initial active Kenya release therefore remains unavailable. No schema, migration, seed, OpenAPI, generated code, handler, route, import mechanism, worker, runtime or frontend change follows from these decisions.
