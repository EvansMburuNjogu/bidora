# Vertical-slice delivery roadmap

## Scope and sequencing

This is proposed delivery sequencing, not calendar commitments or authorization. The user-operable external workflow is the first proposed release; [ARC-001](architecture/BID-ARC-001.md) must reconcile its deliberate manual/non-AI scope with broader BRD/PRD acceptance. Do not silently remove source requirements. Existing source releases R0–R8 and implementation slices S0–S9 retain their meanings; the local increments below elaborate them without claiming they are complete.

## Vertical increments

All ticket suffixes below use the BID- prefix; exact links and hard prerequisites are in the [master](02_MASTER_BACKLOG.md).

| Increment / actor outcome | Contract → persistence/server → UI | Required demonstration and failures |
|---|---|---|
| V0 — Operator can start real application pair | ARC-002/006/007; API-001 → FND-003/001, OBS-001 → FND-002; OPS-001 | Fresh reproducible startup; browser reflects actual Go health and unavailable API, not static success |
| V1 — Verified user creates/selects private workspace | API-002/004/007/008 → DB-001/002/003/005, BE-001, EVT-001/002, INT-001, AUTH-001/002, DOC-001, TEN-003/001/002 → FE-001; SEC-001/002, QA-001 | Real verification/onboarding and two independent tenants; retry failed checkpoint; no forged tenant, stale membership or cross-context result |
| V2 — Bid manager records verified tender and human Bid/No-Bid | API-003 → DB-004/BE-002 → FE-002 | Source/version/deadline/owner visible; duplicate/stale/unknown facts handled; another tenant cannot infer record |
| V3 — Contributor stores and links trusted evidence | API-004 → DB-005/DOC-002 → FE-003 | Actual upload/scan/version/download; quarantine, bad type/hash, scanner outage and revoked download denied |
| V4 — Bid team resolves mandatory requirements | API-003 → BE-003 → FE-004 | Exact-version evidence and owner/due linkage; missing/expired evidence blocks readiness; keyboard matrix usable |
| V5 — Technical and finance contributors prepare revisions | API-005 → DB-006/BE-004 → FE-005 | Explicit revisions/decimal currency, sanitized text; conflict does not overwrite; technical-only actor never receives pricing |
| V6 — Authorized reviewer decides on exact candidate | API-005 → BE-005 → FE-006 | Immutable snapshot and decision history; edit/revoke/approve race rejects stale authority/candidate |
| V7 — Bid manager generates package and records external proof | API-006 → DB-007/DOC-003/BE-006 → FE-007 | Manifest/hash/candidate links; recover storage-success/DB-failure; capture clean proof and separate reported/server time; lost response retry does not duplicate |
| V8 — Bid manager records outcome and correction | API-006 → BE-007 → FE-008 | Valid state transition, correction preserves prior report, permitted portfolio only; full real external journey |
| V9 — Owners accept and release scoped product | OBS-002/OPS-002/003; QA-002/003/004; SEC-003 → REL-001/002/003 | Staging/UAT, independent assessment, measured restore/load, accessible mobile flow, pilot findings and human production sign-off |
| V10 — Later search/advisory AI | ARC-010/AI-001 → INT-002/AI-002/003/004 | Authorized corpus/retrieval, exact citations, bounded cost, human accept/edit/reject; manual workflow survives AI outage |
| V11 — Later procurement marketplace | ARC-009/010 → API-009/DB-008/BE-008–013 → approved runtime children → QA-005/REL-004 | Publication through acceptance/opening/evaluation/award with independent time/crypto/security review; design packets alone cannot pass |

V2 and V3 may proceed in parallel once V1 and their own policies/contracts hold. V4 combines them. Testing belongs inside every increment; final QA is additional acceptance, not deferred first testing. No generic dashboards or fake forms count as delivered slices.

## Mapping original implementation tasks

The [existing BID-T000–017 plan](../implementation/21_IMPLEMENTATION_BACKLOG.md) is unchanged and not a completion ledger. This is its elaboration into individually owned tickets, including corrected prerequisite ordering for identity, internal jobs and provisioning.

| Original task | New ticket coverage (BID- prefix) |
|---|---|
| BID-T000 alignment | ARC-001–008, SEC-001 |
| BID-T001 runnable foundation | API-001, FND-001–003, OBS-001, OPS-001 |
| BID-T002 DB/TxScope | DB-001–003, BE-001 |
| BID-T003 identity | API-002, AUTH-001, INT-001 prerequisites |
| BID-T004 membership/audit/events | AUTH-002, DB-003, BE-001, API-007 |
| BID-T005 provisioning/router | TEN-001–003, DOC-001, QA-001, SEC-002, FE-001 |
| BID-T006 jobs/notifications | EVT-001/002, INT-001, API-004/007, OBS-002 |
| BID-T007 external tender/bid | API-003, DB-004, BE-002, FE-002 |
| BID-T008 secure vault | API-004, DB-005, DOC-001/002, FE-003 |
| BID-T009 requirements/readiness | BE-003, FE-004 |
| BID-T010 responses/pricing | API-005, DB-006, BE-004, FE-005 |
| BID-T011 immutable review | BE-005, FE-006 |
| BID-T012 package | API-006, DB-007, DOC-003, FE-007 |
| BID-T013 external proof | BE-006, FE-007 |
| BID-T014 outcomes | BE-007, FE-008 |
| BID-T015 release evidence | QA-002/003/004, SEC-003, OPS-002/003, OBS-002, REL-001/002/003 |
| BID-T016 later search/AI | ARC-010, INT-002, AI-001–004, REL-004 capability split gate |
| BID-T017 future closed loop | ARC-009/010, API-009, DB-008, BE-008–013, QA-005, REL-004 |

API-008 supports all browser consumers; INT-003 explicitly preserves commercial expansion. These cross-cutting tickets are not additional claims of source scope approval.

## Decision-to-ticket unblock register

Canonical IDs below refer to the [decision registry](../technical_design/27_ARCHITECTURE_DECISIONS.md). Legacy unqualified ADR numbers are not used as authority.

| Decision area | Resolution ticket | Affected work |
|---|---|---|
| BID-ADR-005 frontend root; BID-ADR-008 tooling/migrations | ARC-002 | Foundation, CI and migrations; observed web/ vs described frontend/ remains unresolved |
| BID-ADR-006 external scope | ARC-001 | Business acceptance and release exclusions |
| BID-ADR-007 identity; BID-ADR-016 browser/security | ARC-005/006 | Identity, page/contracts, cookie/CSRF and deployment topology |
| BID-ADR-011 review/exception; BID-ADR-012 proof/outcomes | ARC-003/004 | Readiness, candidates/approval, external proof and state history |
| BID-ADR-013 production/recovery/privacy | ARC-007 | Local profiles through staging, retention, restore and release |
| BID-ADR-014 providers; BID-ADR-015 quotas; BID-ADR-017 files/package | ARC-008 with ARC-007/AI-001 | Mail/storage/scan/render, reservations, external-source/AI egress and package reproducibility |
| BID-ADR-009/010 sealing/time | ARC-009 | Future platform submission/opening only |
| BID-ADR-018 sequencing | All relevant graph edges | Identity/policy/audit before exposed provisioning/admin; no circular bootstrap shortcut |

Approved decisions BID-ADR-001–004 establish naming, core stack, plane/isolation and messaging/storage direction. Their acceptance is not approval of unresolved provider, identity or procurement policy choices. Each resolution must record owner/date/evidence in a separately authorized change; this ticket-generation task does not edit source decisions.

## Future coverage limit

ARC-010 allocates advanced profiles, enterprise SSO/support, collaboration/delegation, country packs, reporting, privacy/erasure and payment expansion packets. Future procurement tickets are design work because field-level contracts and policies are incomplete. Creating arbitrary implementation detail now would conceal real blockers. Before implementing any selected future capability, create reviewed granular runtime/UI/test children and update graph, coverage and release gate; no blanket future-platform ticket may become Ready.
