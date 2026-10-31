# Epics and capability coverage

Epics organize ownership, not waterfall phases. Deliver the cross-layer [vertical slices](04_DELIVERY_ROADMAP.md), not all backend first. All epics are proposed/blocked plans; no named team/person is assigned without approval. Ticket ranges below are inclusive and enumerate every file in the [master](02_MASTER_BACKLOG.md).

| Epic | Business goal / ticket coverage | Principal prerequisite | Completion evidence |
|---|---|---|---|
| E-ARC — Architecture and decisions | Resolve real policy/design gaps; BID-ARC-001–010 | Accountable product/security/architecture owners | Accepted decision packets, exact source conflicts, bounded future child plans |
| E-FND — Runnable foundation | One Go API and one Next.js app; BID-FND-001–003 | Approved root/tool/config/API decisions | Clean-start real API UI and honest outage behavior |
| E-API — Contracts | Stable operation/event contracts before consumers; BID-API-001–012 | Actor/state/policy decisions | Reviewed schemas, examples, conformance/compatibility and generated-client evidence |
| E-DB — Persistence | Private planes and immutable records; BID-DB-001–008 | Reviewed contracts/schema/role policy | Clean/upgrade/constraint/role/transaction tests; future schema packet separately accepted |
| E-BE — Procurement/bid domain | Govern verified bid lifecycle and platform/profile/read-model/branding operations; BID-BE-001–017 | Contracts, owner persistence, authorization/isolation | State/version/race/audit evidence; future procurement tickets 008–013 are design packets, not handlers |
| E-AUTH — Identity and policy | Verified identities, scoped memberships, platform authority and controlled support; BID-AUTH-001–004 | Identity decision, control schema, notifications, threat model | Session/recovery/MFA/CSRF/revocation/last-owner/support-scope negative tests |
| E-TEN — Isolated tenancy | Provision/recover/route and enforce subscriptions without fallback; BID-TEN-001–004 | Identity/audit/jobs/storage/quota/commercial policies | Crash-per-step, two-tenant routing and allowance/subscription race evidence |
| E-EVT — Events and jobs | Durable, visible at-least-once processing; BID-EVT-001–002 | Transactional outbox/inbox, event/job contracts | Publish/commit/ack fault tests, scoped replay, no exactly-once external claim |
| E-DOC — Documents and packages | Trusted immutable evidence and approved outputs; BID-DOC-001–003 | Storage/scan/format/retention and candidate policy | Scan/access/hash/package fingerprint/recovery tests |
| E-FE — User operations | Tenderpreneur and Platform Admin users complete real authorized workflows; BID-FE-001–011 | Accepted page/API contracts, explicit web authorization and real backend slices | Full state handling, context races, role-filtered payloads, accessible real-data UI |
| E-INT — Integrations | Safe notification/source/commercial boundaries; BID-INT-001–004 | Provider/terms/idempotency decisions | Ambiguous-effect reconciliation; source provenance; PesaPal design and runtime evidence |
| E-AI — Advisory search and AI | Authorized cited natural-language assistance, not decisions; BID-AI-001–005 | Approved contracts/provider/corpus/knowledge policy | Privacy/isolation/citation/language/injection/cost evaluation and human disposition |
| E-SEC — Security | Prevent and independently assess confidentiality/retention failures; BID-SEC-001–004 | Approved threat/privacy/retention scope and runnable surfaces | Isolation, assessment, remediation and legal-hold-safe retention evidence |
| E-QA — Verification | Reproducible journey and negative acceptance; BID-QA-001–006 | Actual runtime/contracts and approved budgets | Real dependencies, persona E2E, accessibility/load and Phase 1/future acceptance |
| E-OPS — Delivery/recovery | Reproducible builds and recoverable environments; BID-OPS-001–003 | Host/secrets/compatibility/RPO/RTO policy | CI negative checks, staging promotion, measured restore/reconciliation |
| E-OBS — Operations insight | Diagnose safely and respond to user impact; BID-OBS-001–002 | Telemetry/privacy/SLO decisions | Redaction/correlation, alert delivery and exercised runbooks |
| E-REL — Acceptance/release | Promote only evidenced, human-approved capabilities; BID-REL-001–005 | UAT/security/QA/ops/provider evidence | Signed scoped acceptance, Phase 1 disposition, pilot disposition and production authorization |

## Product capability coverage and explicit limits

References refer to the actual [PRD](../product/03_PRD.md), [SRD](../product/04_SRD.md), [domain rules](../product/05_DOMAIN_MODEL_AND_BUSINESS_RULES.md) and [journeys](../product/06_USER_JOURNEYS_AND_WORKFLOWS.md). Coverage means planned ownership, **not full satisfaction**.

| Source capability | Concrete ticket coverage | Scope boundary / remaining gate |
|---|---|---|
| PR-TP-001; PR-SH-001/002; J-01/J-16 | AUTH-001–003, TEN-001–004, BE-015/017, FE-001/009 (all IDs prefixed BID-) | Exclusive Phase 1 portal typing and simple roles proposed; field classification and commercial decisions remain gates |
| PR-TP-002; PR-SH-003 | API-004/012, DB-005, DOC-001/002, BE-015, FE-003/009 | Exact-version clean vault plus proposed approved-knowledge lifecycle; field/approval policy remains blocked |
| PR-TP-004/006/007; J-06/J-07 | API-003, DB-004, BE-002, FE-002 | Manual verified facts/source/deadline; automated extraction remains later |
| PR-TP-008/011; J-08 | BE-003, FE-004, DOC-002 | Deterministic readiness, mandatory gaps/evidence; unsupported waiver blocks rather than guessed |
| PR-TP-009; BR-025 | API-005, DB-006, BE-004, FE-005 | Versioned technical/financial preparation; full reusable content library is later ARC-010 |
| PR-SH-004; RULE-BID-004/005 | BE-005, FE-006 | Immutable candidate review; configurable workflow designer/delegation later ARC-010 |
| PR-TP-013; J-10; BR-026 | API-006, DB-007, DOC-003, BE-006, FE-007 | Approved package and confirmed external proof; no automatic portal delivery claim |
| PR-TP-014 | BE-007, FE-008 | Reported outcomes/corrections; predictive analytics is later |
| PR-TP-003/005/010/011; SR-AI-001–006 | API-012, BE-015/017, AI-001–005, DOC-002, FE-009 | Phase 1 Tender Assistant now has explicit child ownership; corpus, budget, provider and knowledge approval gates remain |
| PR-PT-001/002/004 | BE-008, API-009, DB-008 | Requisition/conversion/authoring design; runtime children required |
| PR-PT-005/006; J-04/J-15 | BE-009, API-009, DB-008 | Publication/version/addendum design; impact acknowledgements and public UI children required |
| PR-PT-007/008; PR-TP-012; J-11/J-12 | ARC-009, BE-012/013, QA-005 | Sealed acceptance/opening design; deadline/KMS review and runtime children required |
| PR-PT-009/011; J-13/J-14 | BE-010/011, QA-005, REL-004 | Evaluation/award/handover design; criteria/committee policies and runtime/UI children required |
| PR-PA-001/003; SR-TEN | API-010, AUTH-003, TEN-001–004, BE-014/016, EVT-002, OBS-002, FE-010/011 | Functional platform control children exist; policy/contracts/web authorization and runtime evidence remain blocked |
| PR-PA-002; PR-SH-009; J-18 | API-011, TEN-003/004, INT-003/004, FE-009/011 | PesaPal selected; prices/tax/provider contract and sandbox/runtime evidence remain blocked |
| PR-PA-004 | INT-002, AI-001 | One approved source; source-registry UI/additional connectors later ARC-010 |
| PR-PA-005; J-17; SR-IAM-004 | API-010, AUTH-003/004, SEC-001/003/004, FE-010/011 | Support and retention children exist; no standing private access or destructive retention is approved |
| PR-SH-005/006/007/008/010 | BE-003/014/016, INT-001, AI-002, DB-003/BE-001, ARC-006/010 | Core assignments/notices/audit plus Phase 1 settings/dashboard ownership; detailed policy and acceptance remain |
| PR-PT-012; later analytics/enterprise/privacy | ARC-010, SEC-001/003, OPS-003 | Basic scoped outcome view only; BI, erasure/legal-hold orchestration and enterprise features require explicit packets |

## No feature disappears through deferral

ARC-001 must reconcile proposed manual external scope with broader BRD/PRD MVP acceptance. ARC-010 owns exhaustive future packet allocation; its Done is a reviewed coverage register, **not** completed future software. Before choosing a future capability, replace the umbrella gap with approved field-level schema/API/page contracts and bounded runtime/test tickets. Keep this matrix and release exclusions current. Existing product source requirements remain unchanged.

## Cross-cutting ownership

Every command owner owns authorization, versions, idempotency, transaction/audit/outbox and negative tests. Every document/job/export owner extends isolation regression. Every frontend ticket consumes contracts and tests loading/empty/error/forbidden/conflict/mobile states. QA/security/ops epics verify independent gates; they do not absorb implementation obligations or authorize bypasses.
