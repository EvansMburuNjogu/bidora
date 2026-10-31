# BID-API-010C — P1–P9 countries/reference-data review

Status: **P1–P9 APPROVED POLICY — IMPLEMENTATION/ACTIVE RELEASE EVIDENCE GATES OPEN**  
Reviewed packet: `countries-reference-data-decision-packet.md`  
Scope: provenance, licensing and authority review only  
Implementation authority: none

## 1. Review conclusion

P1–P9 are directionally consistent with Bidora's Kenya-first, versioned-reference, server-owned-routing and fail-closed architecture. The packet correctly prevents country/reference metadata from becoming authority, keeps holidays informational, preserves historical bindings and prohibits runtime network discovery or automatic import.

A1–A4 and C1–C4 resolved the identified authority and contract ambiguities: external licensing attestation, Operations-maker/Platform-Owner-checker activation, deployment-owned compatibility, server-known manifest selection, no initial holiday/CLDR/tender-classification content and explicit legacy provenance. The requesting user then approved amended P1–P9 as policy only. Exact source/licence and artifact evidence remains unresolved, so implementation and the initial activatable Kenya release remain unavailable.

P3's “holidays never alter tender deadlines” rule, P4's immutable/fenced lifecycle, the no-retroactive-rewrite and no-routing-authority parts of P6, and P9's safe evidence/privacy rules are approved policy. They do not establish implementation, provenance, deployment or release acceptance.

## 2. Provenance and licensing evidence still required

### ISO 3166-1 country identifiers and names

Before `KE` or a country display name is represented as reviewed upstream data, the accountable owner must record:

- the exact ISO 3166-1 edition/snapshot or other explicitly accepted authoritative artifact;
- publisher, canonical URI, retrieval/publication date and SHA-256 where retention is permitted;
- terms permitting Bidora's intended internal use and any redistribution through source, binaries or API responses;
- whether only the identifier `KE` is sourced from the standard while the localized label `Kenya` is Bidora/CLDR-owned display metadata.

A citation to “ISO 3166” without an exact artifact and terms review is insufficient. Shape validation does not establish authority or a redistribution right.

### ISO 4217 currency and minor units

The review must identify the exact ISO 4217/SIX-published list/version used to validate `KES` and its two minor units, its artifact hash or restricted evidence reference, and the applicable use/redistribution terms. Commercial's existing source-defined KES invariant remains authoritative for its own operations; a reference release may provide validation provenance but must not silently replace or weaken the Commercial contract.

### IANA timezone data

The initial release must identify an exact IANA tzdb release, source archive/hash and licence notice. Deployment owners must choose and evidence one compatible resolution mechanism—such as a pinned application tzdb or a verified operating-system tzdb—and prove every serving runtime resolves `Africa/Nairobi` under the reviewed release. “The deployment's tzdb” is not a reproducible version decision.

A later tzdb update must not reinterpret stored UTC instants. Activation compatibility is a deployment gate, not something an Operations Administrator can attest from a browser.

### BCP 47 locale and localized display metadata

BCP 47 defines tag structure and canonicalization; it is not by itself a complete locale dataset. The packet must distinguish:

- an exact dated/hash-recorded IANA Language Subtag Registry snapshot for validating/canonicalizing `en-KE`; and
- any Unicode CLDR release used for localized country names, date/number formats or display labels, including its exact version, artifact hash and Unicode licence notice.

If no CLDR content is used, `Kenya` and other display labels must be explicitly identified as reviewed Bidora-owned product text rather than attributed to BCP 47.

### Kenya holidays and official notices

The initial coverage period and source inventory are undefined. Before holiday entries can activate, the accountable reviewer must define:

- whether an initial release may contain no holidays, or the exact required year range;
- the official authority and exact Act/gazette/notice citation for every date, including issue/notice/page where applicable;
- how observed/substitute days, one-off gazetted holidays, same-date collisions and later corrections are represented;
- whether the source artifact may be retained or redistributed, and otherwise the restricted evidence location/hash and minimal public citation;
- the behavior when an official URI disappears.

The recommended safe publication boundary is factual date, bounded display label and citation only. Gazette text or images must not be copied into general reference data unless licence/legal review permits it. Holiday incompleteness must be explicit and must never move or judge a tender deadline.

### Bidora-owned taxonomy and address profile

The organization/tender classification labels, Gregorian-calendar selection and `KE_FREEFORM_POSTAL_V1` are product-owned rules, not ISO/IANA facts. They need:

- accountable Product approval and semantic definitions that prohibit legal-form/procurement-procedure inference;
- procurement/legal review that the tender labels are safely presented as neutral product classifications;
- exact address fields and byte limits plus privacy/security review;
- a Bidora policy/version and reviewer record, rather than an invented third-party citation.

## 3. Reviewed authority alternatives

### A1 — source/licence attestation owner

`OPERATIONS_ADMINISTRATOR` has approved `platform.reference.read/manage`, but that role does not confer legal/licensing authority. Select who may attest that an upstream artifact and its terms are acceptable:

1. **Recommended:** accountable legal/licensing reviewer evidence is recorded outside the operational command and bound by immutable reference/hash; Operations may act only after that evidence is present.
2. Give an Operations Administrator unilateral attestation authority. This is not recommended and would expand the approved role's meaning.
3. Add a new platform role/capability. This requires a separate role-catalogue decision and is not justified for the bounded pack unless an accountable in-product workflow is required.

No route should expose a checkbox that turns an operator assertion into legal approval.

### A2 — release maker/checker

At review time, P5 recommended that one Operations Administrator could create, schedule, activate-now and roll back a release. Rollback and history do not prevent a malicious or mistaken release from immediately changing all new onboarding/profile choices.

Choose one:

1. **Recommended:** Operations Administrator with `platform.reference.manage` creates/withdraws a draft and proposes schedule/activation/rollback; a distinct effective Platform Owner with `platform.governance.approve` and fresh TOTP checks the exact immutable digest. Approval expires after 30 minutes and execution rechecks both actors and deployment compatibility. Emergency correction uses the same rule with activate-now; there is no bypass.
2. Single Operations Administrator authors and activates. This is simpler but accepts the single-actor blast radius and makes the manifest “reviewer” potentially the author.
3. Distinct Operations maker/checker. This needs a new checking capability or an explicit capability interpretation and role-catalogue amendment.

Option 1 can use existing approved capability names but expands the approved maker/checker operation list, so it still needs explicit approval.

### A3 — product/procurement review of Bidora taxonomies

Decide whether the P2 organization and tender lists are accepted exactly as non-legal Bidora classifications. Product must own their meanings; a qualified procurement/legal reviewer must confirm that labels such as `CONSULTANCY_SERVICES` and `NON_CONSULTANCY_SERVICES` will not be presented as statutory classification or procedure advice. If that review is unavailable, omit tender classifications from the initial pack rather than implying legal validation.

### A4 — deployment compatibility authority

Select the deployment owner responsible for attesting the exact tzdb version and runtime compatibility before activation. An Operations release actor cannot supply this proof merely by selecting `Africa/Nairobi`. Activation should consume a server-produced compatibility result bound to release digest and deployment set; clients must not submit a compatibility boolean.

## 4. Reviewed contract alternatives

### C1 — source-controlled manifest versus raw HTTP creation

At review time, P1 prohibited arbitrary administrator-authored/uploaded reference data while P8 proposed a 256 KiB `POST /v1/platform/reference/releases` containing a complete release. These boundaries conflict unless the server can prove the submitted content exactly matches a reviewed artifact.

Choose one:

1. **Recommended:** a separately reviewed deployment/import boundary registers an exact immutable manifest digest; the Platform operation may only create/propose a release by selecting that server-known digest. Browser/API callers cannot author pack content or provenance.
2. Permit strict JSON content creation only when every identifier, label, holiday and provenance item matches a pre-registered source/evidence registry. This is more complex and still requires a trusted registry owner.
3. Permit arbitrary bounded content with operator attestation. This contradicts P1 and the packet's exclusions and should be rejected.

The exact import/deployment mechanism remains a later design; this review does not authorize building one.

### C2 — holiday coverage and stale-review behavior

Decide whether an active release:

- may contain zero holidays with explicit `coverage: NONE`;
- must contain a complete named year range, such as current/next year; or
- may contain partial coverage with exact from/to dates and completeness state.

Also decide what happens when quarterly or annual review is overdue. **Recommended:** do not automatically deactivate an otherwise valid release, because that could halt onboarding without proving the data wrong; mark review overdue in operations health and block scheduling a successor until review evidence is current. A known correctness/security problem may require an emergency superseding release or explicit withdrawal under the approved authority path.

### C3 — pre-catalogue records and prospective binding

Existing tenant/profile rows may predate immutable reference release IDs. Decide how reads represent them without false provenance:

1. **Recommended:** retain their exact stored code/value and mark provenance explicitly as `LEGACY_PRE_REFERENCE_CATALOGUE`; do not backfill a release UUID unless exact historical validation evidence exists.
2. Perform a separately authorized, evidenced migration that binds only rows proven to match a reviewed release.
3. Silently bind all existing values to the first release. This would rewrite provenance and should be rejected.

New writes may be required to bind the active release only after an active reviewed catalogue exists. Until then, dependent new catalogue-based operations fail closed; existing historical reads remain available.

### C4 — initial release contents

Approval must say whether the initial Kenya release includes:

- only `KE`/`KES`/`Africa/Nairobi`/`en-KE` plus product-owned classifications/address profile;
- a defined holiday coverage set; and
- localized/format data from CLDR or only Bidora-owned English labels.

Without this exact content boundary, “approve P1–P9” does not identify the artifact that may eventually activate.

## 5. Decisions that can be accepted without source artifacts

The following policy rules can be approved independently of the eventual seed evidence:

- no runtime network discovery, auto-import or auto-activation;
- exact allowlists rather than regex-only acceptance;
- Kenya-only Phase 1 market identifiers, subject to source verification before activation;
- holidays are informational and never alter deadlines or workflow;
- immutable complete releases, one active/one scheduled release and fenced rollback;
- stable code plus immutable release provenance for new records;
- no historical rewrite and no routing/authorization meaning;
- strict bounded operations, idempotency, optimistic concurrency and safe errors;
- minimal control audit/outbox evidence with no private or secret data.

Approval of those rules would still not create an activatable release.

## 6. Approval record and remaining sequence

The requesting user approved these review decisions:

- **A1 option 1:** accountable legal/licensing evidence is recorded outside the operational command and immutably bound before Operations may act;
- **A2 option 1:** Operations Administrator maker plus distinct effective Platform Owner checker for schedule, activation and rollback; emergency activate-now has no bypass;
- **A3 amended:** Bidora organization classifications remain product-owned; tender classifications are omitted until procurement/legal review approves their safe neutral use;
- **A4:** deployment-owned, server-produced tzdb compatibility evidence bound to the release/deployment set;
- **C1 option 1:** Platform operations may select only a separately reviewed, server-known immutable manifest digest and cannot author pack content;
- **C2 amended:** the initial pack explicitly declares holiday coverage `NONE`; review overdue does not automatically deactivate an otherwise valid release;
- **C3 option 1:** existing values retain exact stored facts with `LEGACY_PRE_REFERENCE_CATALOGUE` provenance and are never silently bound to the first release;
- **C4 amended:** the initial content boundary is the minimal `KE`/`KES`/`Africa/Nairobi`/`en-KE` pack with reviewed Bidora-owned English display metadata/address profile, no CLDR-derived content, no tender classifications and no holidays.

These choices amend P1–P9, which the requesting user subsequently approved as policy only. The user explicitly directed **do not implement yet** and kept every active Kenya release blocked. Exact ISO 3166-1, ISO 4217/SIX, IANA tzdb and IANA Language Subtag Registry provenance/licensing evidence, Product review of organization classifications/address metadata, the deployment compatibility evidence contract and the exact reviewed manifest digest remain required before an activatable release can be separately approved.

Remaining sequence:

1. Obtain and record the exact standards/tzdb/locale licence and provenance evidence; holiday/CLDR evidence is unnecessary while those contents remain absent.
2. Separately approve the exact initial Kenya manifest artifact and digest plus deployment compatibility evidence.
3. Only after a new explicit request consider a separately bounded source implementation authorization.

No schema, migration, seed, OpenAPI, generated code, handler, route, import tool, activation worker, runtime configuration, provider call, test or frontend change was made or authorized by this review.
