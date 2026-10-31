# Release milestones and evidence gates

Milestones are proposed evidence targets, not dates or delivery promises. M0–M8 are local backlog IDs, **not** source R0–R8. All are currently unachieved. Latest target assignment is in the [master](02_MASTER_BACKLOG.md); prerequisites can be delivered earlier. A milestone cannot waive the [Ready/Done standards](06_TICKET_STANDARDS.md).

| Milestone | Outcome / entry | Exit evidence and accountable review roles |
|---|---|---|
| M0 — Approved scoped design | Existing docs inspected; product/security/architecture reviewers assigned | ARC-001–008 and SEC-001 accepted as applicable; root/tool/identity/domain/file/provider/operations gaps resolved; scope deviations explicitly approved; no runtime claim |
| M1 — Runnable foundation | Authorized bounded implementation; M0 decisions affecting foundations closed | FND-001–003, API-001, OBS-001, OPS-001; one real Go→Next.js smoke flow, clean config/start, honest failure UI, CI negative checks and safe logs; engineering review |
| M2 — Trusted workspace | Identity/control/document/job contracts and schema accepted | DB/BE/audit/event/internal-storage/auth/tenancy dependencies, API-008, QA-001, SEC-002, FE-001; verified user provisions/selects two isolated tenants; retry/revocation/CSRF/pool/quota tests; security/engineering review |
| M3 — First external business value | M2 gates; opportunity and exposed document policies/contracts accepted | BE-002/FE-002 and DOC-002/FE-003 with owner schemas/contracts; real verified tender/bid decision and clean exact-version evidence; foreign access and stale/ambiguous facts denied; product/QA review |
| M4 — Accepted external MVP | M3; approved review/proof/outcome/package policies | Full BE-003–007, DOC-003, FE-004–008 chain; real staging, monitoring, QA E2E/a11y/load, independent assessment and measured restore; REL-001 signed evidence; product/procurement/QA/security/operations review |
| M5 — Controlled pilot | M4 acceptance; pilot data terms/cohort/support authorized | REL-002 actual pilot onboarding and representative usage, incident/support exercise, feedback defect disposition and user/ops acceptance |
| M6 — Production readiness/promotion | M5 feedback accepted; final operational/security sign-offs | REL-003 same tested artifact/schema compatibility, backup/alerts/on-call, canary and recovery plan, bounded post-release verification and truthful limitations |
| M7 — Phase 1 Tenderpreneur and Platform Admin increment | Selected Phase 1 scope; commercial/security/provider decisions; explicit backend and web authorization | ARC-010 plus API-010–012, AUTH-003/004, TEN-004, INT-004, BE-014–017, AI-002/005, SEC-004, FE-009–011 and QA-006 evidence; REL-005 records provider/deployment review, persona UAT and formal sign-off for shared routing, functional settings, plans/PesaPal, powerful human-controlled AI, support, retention controls and dashboards |
| M8 — Future closed-loop procurement | ARC-009 temporal/key-release and future governance approval | API-009/DB-008/BE-008–013 accepted designs **plus created and verified runtime/UI children**; QA-005 actual closed-loop/security/load/restore/UAT and REL-004 capability-specific approval |

## M4 external-MVP acceptance checklist

- Authorized team can onboard/select workspace and manage permitted membership without cross-tenant disclosure.
- Bid manager records source/version/deadline with unknown/ambiguous facts explicit, verifies facts and records human Bid/No-Bid.
- Contributors upload clean exact-version documents, assign requirements and link evidence; expired/missing mandatory evidence blocks readiness unless approved policy explicitly permits a governed exception.
- Technical/financial revisions use correct access and optimistic concurrency. Review is bound to immutable candidate; material edits invalidate current applicability, never erase historical approval.
- Package job reconciles manifest/bytes/storage state; retry is safe. External proof references exact package, with reported time separate from capture time; Bidora does not claim delivery it did not perform.
- Outcome corrections preserve previous facts. UI handles real loading/empty/error/forbidden/conflict states, keyboard and mobile essentials.
- No manual DB intervention or production mocks needed. Actual two-tenant, negative authorization, race/replay, scan/package outage, accessibility, load, security and restore artifacts exist.
- Exact release scope and excluded AI/marketplace/commercial capabilities are human-approved through ARC-001; original source acceptance remains preserved for later delivery.

## M5/M6 operational gates

Require approved providers/regions/data-processing/retention terms, live secrets outside repositories, least-privileged identities, current backup health and timed restore, artifact/schema provenance, compatibility-safe migration/rollback or forward-fix, verified alert transport and named incident/support ownership. Confidentiality failures cannot be waived. Failed/skipped required checks block acceptance. Approval records must include owner/date/evidence; this document supplies none.

## Future capability gates

Encryption at rest alone does not satisfy sealing. Require exact deadline acceptance oracle, retry/replacement/withdrawal rules, independent key-release review, quorum/step-up, cross-plane recovery and pre-opening denial. Evaluation uses published criteria and governed human scoring/award. AI requires authorized corpus, source terms, citation/unsupported-output evaluation, tender-language and uncertainty checks, bounded cost, injection tests and human disposition. PesaPal requires contracted API/authenticity review, sandbox reconciliation and no callback-only entitlement. Retention requires approved lawful periods/holds and restore-aware execution. Runtime child tickets are mandatory where current documents provide only design packets.

## Relationship to product roadmap

[Product R0](../product/16_PRODUCT_ROADMAP.md) relates to M0, R1 to M1/M2, proposed scoped R2 to M3–M6, R3 and selected R7 capabilities to M7, and R4–R6 to M8. R8 enterprise/intelligence/country expansion is explicitly allocated by ARC-010 but not exhaustively implementation-ticketed yet. Scope alignment, rather than matching numbers, determines acceptance.

References: [source acceptance evidence](../product/19_TRACEABILITY_ACCEPTANCE_EVIDENCE.md), [implementation readiness](../implementation/19_RELEASE_READINESS.md), [design limitations](../technical_design/QUALITY_REPORT.md).
