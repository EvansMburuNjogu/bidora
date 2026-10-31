# Phase 1 Tenderpreneur and Platform Admin gap audit

Status: **APPROVED PRODUCT DIRECTION — IMPLEMENTATION BLOCKED BY RECORDED GATES**  
Source packet: [Phase 1 portal functional design](../../backend/docs/phase-1-portal-functional-design.md)

## 1. Audit outcome

The existing backlog covers much of the external-tender transaction path but does not cover the newly selected commercial and platform-control scope end to end. The largest gaps are shared portal typing/routing, platform authority, typed global settings, organization knowledge for AI, plans/subscriptions, PesaPal, controlled support, retention execution, actionable dashboards and corresponding web/acceptance work.

This audit adds bounded child tickets and strengthens existing tickets instead of treating BID-ARC-010 as implementation authority. All new tickets remain Blocked pending their stated policies, contracts and explicit implementation authorization.

## 2. Reconciled active backend count — 2026-10-20, refreshed after source/policy expansion

Execution reset: the requesting user froze all 13 Phase 1 functional umbrellas and all controlled-support policy work. They remain preserved below as blocked inventory but are not active execution work. Only the unfinished Option A V1 verification packet is active; V2–V10 must not start automatically.

The fixed inventory is still **25 open backend packets**, but that number is not a burn-down measure. A functional umbrella leaves the count only when its whole ticket reaches source-complete status; implemented child slices do not close it. This section therefore reports one mutually exclusive primary state for every packet and separately exposes source maturity inside blocked umbrellas.

### 2.1 Mutually exclusive packet count

| Primary state | Count | Packets |
|---|---:|---|
| Source-complete, awaiting verification | **0** | None of the 13 functional umbrellas has its complete ticket scope in source |
| Source-incomplete and currently actionable | **0** | Every source-incomplete functional umbrella still has an explicit policy, authority, provider, legal/security, dependency or activation gate recorded in its ticket |
| Frozen blocked functional umbrella | **13** | BID-API-010, BID-API-011, BID-API-012, BID-AUTH-003, BID-AUTH-004, BID-TEN-004, BID-INT-004, BID-BE-014, BID-BE-015, BID-BE-016, BID-BE-017, BID-AI-005, BID-SEC-004 |
| Option A verification | **10** | V1–V2 `VERIFIED_TECHNICALLY`; deployment-specific proxy CIDRs deferred to deployment acceptance; V3–V10 frozen |
| Phase 1 acceptance | **2** | BID-QA-006 and BID-REL-005 |
| **Total** | **25** | No packet is double-counted |

`Blocked` does not mean untouched. All 13 functional umbrellas are source-incomplete at whole-ticket level, but several contain substantial `IMPLEMENTED_NOT_VERIFIED` child slices. The count remains 25 because no umbrella has yet crossed its complete source boundary, not because no application work has landed.

### 2.2 Functional umbrella maturity and exact unblock condition

| # | Ticket | Primary state | Source delivered inside the umbrella | What prevents whole-ticket source completion |
|---:|---|---|---|---|
| 1 | BID-API-010 | Blocked | Portal routing; Platform Administration lifecycle contracts; typed SMTP/OpenAI settings; approved countries policy; selective privacy and controlled-support policy | Countries provenance/manifest and implementation authority; incomplete privacy/legal matrices; pending controlled-support SS/E/R decisions and exact capabilities; retention policy |
| 2 | BID-API-011 | Blocked | C1–C9 provider-neutral invoice, payment, reconciliation, settlement, adjustment, reversal, reads, listings, queues and unmounted HTTP source | RR1–RR15 implementation is explicitly unauthorized; provider refund/cash/entitlement disposition, rendering and exact PesaPal/finance/tax evidence remain |
| 3 | BID-API-012 | Blocked | Organization profile/letterhead and exact branding-binding contract foundations | Knowledge classification, reviewer grants and complete permission-aware Tender Assistant operation contracts are unresolved |
| 4 | BID-AUTH-003 | Blocked | Account typing and Platform Administration invitation, maker/checker, role, identity-state, session-revocation, governance-read and unmounted HTTP source | Deployment bootstrap executable/values, provider wiring, migration execution and runtime/isolation evidence remain gated |
| 5 | BID-AUTH-004 | Blocked | T1–T6, D1–D6 and no-break-glass policy approved; SS1–SS6/E1–E5/R1–R5 ballot presented | Session/evidence/fencing choices and exact tenant capability identifiers remain unapproved; user explicitly directed no implementation |
| 6 | BID-TEN-004 | Blocked | Catalogue, limits, Trial, subscription lifecycle, AI reservation/accounting and paid-write/reversal-hold enforcement source | Complete paid entitlement/licence lifecycle, reversal recovery and provider-linked restoration remain gated; source is unmounted/unverified |
| 7 | BID-INT-004 | Blocked | Provider-neutral query-before-retry and reconciliation ports/persistence | Exact contracted PesaPal API/version, sandbox, authenticity, KES methods, status/refund/reversal behavior and production configuration are absent |
| 8 | BID-BE-014 | Blocked | Typed immutable SMTP/OpenAI settings persistence and application source | Secret-manager/KMS resolution, countries artifact/evidence, PesaPal settings, privacy/retention settings and provider/runtime boundaries remain gated |
| 9 | BID-BE-015 | Blocked | Company profile and immutable letterhead foundations | Exact knowledge catalogue/classification, reviewer policy, secure asset limits and BID-API-012 contract remain unresolved |
| 10 | BID-BE-016 | Blocked | BID-BE-016A unmounted permission-filtered Tenderpreneur and Platform Billing aggregation | Broader settings/support/security dashboard sections, actions, routes and owner operations depend on blocked source above |
| 11 | BID-BE-017 | Blocked | Exact immutable letterhead provenance and proposal/package binding | CLEAN asset contract plus hostile-asset and deterministic PDF/DOCX rendering/security review remain unresolved |
| 12 | BID-AI-005 | Blocked | Measurement, entitlement acknowledgement/accounting, cited Option A AI foundations and worker boundaries | Knowledge/retrieval contracts, approved corpus/quality/language thresholds, final parser isolation and provider deployment acceptance remain |
| 13 | BID-SEC-004 | Blocked | Selective privacy governance and draft retention/legal-hold authority packet | Lawful-basis/deadline/effect matrices, R1–R13 approval, legal/privacy/security/restore evidence and destructive authority remain absent |

### 2.3 Burn-down rule from this point

- Do not report bare “25 remaining” as progress.
- Report `0 source-complete / 0 actionable source-incomplete / 13 frozen-blocked / 10 verification (V1 active only) / 2 acceptance` until an owning packet changes category.
- Child slices may be reported as delivered evidence, but they do not decrement the 25-packet inventory.
- A blocked umbrella moves to **source-incomplete and actionable** only when its next source step has all required policy/authority inputs.
- It moves to **source-complete, awaiting verification** only when its entire approved backend source scope, migrations/contracts/generated artifacts and separately mountable composition exist; runtime activation and functional proof remain later gates.
- Verification and acceptance packets decrement only after their stated evidence and accountable disposition actually complete.

Existing dependency tickets such as BID-AUTH-002, BID-TEN-003, BID-EVT-001/002, BID-DOC-002, BID-INT-001 and BID-AI-001–004 are not added again. Their remaining Phase 1 work is included under the owning umbrellas above; their Option A evidence is included in V1–V10.

Excluded from 25: BID-FE-009–011 because web edits are not authorized; BID-BE-008–013 and future procurement/exchange work; an undefined Procurement Portal; production deployment/provider procurement; and separately human-owned UAT/sign-off activities beyond BID-REL-005. If web implementation is authorized, the inventory becomes **28** before any frontend-specific verification splits.

## 3. Existing coverage retained

- BID-AUTH-001: shared authentication, sessions, recovery and MFA.
- BID-AUTH-002: tenant membership, context and the simplified Tenderpreneur role policy.
- BID-BE-001–007: the implemented-not-verified external-tender path.
- BID-DOC-001–003: private storage, scanning and immutable packages.
- BID-INT-001: SMTP delivery and notification processing.
- BID-TEN-003: entitlement reservations and suspension behavior.
- BID-AI-001–004: AI privacy, retrieval, parsing and cited suggestions.
- BID-FE-001–008: existing Tenderpreneur workflow surfaces.

## 4. Gap and ownership matrix

| Phase 1 requirement | Existing coverage | Gap | Owning ticket |
|---|---|---|---|
| One login, exclusive portal type and redirect | AUTH-001 plus implemented-not-verified account typing/routing source | Migration/runtime verification and frontend routing remain | API-010, AUTH-003, FE-001 |
| Four simple Tenderpreneur roles | AUTH-002 | Runtime grants and complete role-management evidence remain | AUTH-002 strengthened |
| Platform roles and administrator lifecycle | Fixed catalogue, persistence, application commands, reads and unmounted HTTP source | Deployment bootstrap executable/values, provider wiring, migration/runtime/isolation evidence remain | API-010, AUTH-003 |
| Typed versioned platform settings | Implemented-not-verified SMTP/OpenAI draft/test/activate/rollback source | Secret resolution, provider adapters, routes/runtime and other setting types remain | API-010, BE-014 |
| Global SMTP administration | INT-001 delivery plus unmounted typed SMTP setting lifecycle | Secret/provider/runtime activation and template administration remain | INT-001, BE-014, FE-010 |
| Countries/reference data | Approved P1–P9 minimal Kenya policy | Exact standards provenance/licensing, Product metadata, deployment compatibility, reviewed manifest digest and implementation authority remain | BE-014, FE-010 |
| Plans and monthly Bid allowance | Catalogue, Trial, lifecycle, limits, AI reservations and financial source through C9 | Complete licence/recovery/provider-linked lifecycle, runtime composition and verification remain | API-011, TEN-003, TEN-004 |
| PesaPal | Selected provider plus approved C1–C15, provider-neutral financial schema and unmounted query-before-retry ports | Exact contracted API/authenticity evidence, provider adapter and reconciliation runtime absent | INT-003, INT-004 |
| Tenderpreneur billing/payment UX | None | No usable plan/invoice/payment journey | FE-009 |
| Organization profile and approved knowledge | Versioned profile/letterhead foundation | No approved structured reusable knowledge authority or reviewer/classification contract | API-012, BE-015 |
| Company-first onboarding and letterhead | Immutable profile/letterhead and proposal/package binding source | CLEAN branding assets, deterministic rendering/security evidence and UX remain | API-012, BE-015, BE-017, FE-009 |
| Every eligible document searchable/vectorized | AI-002 future retrieval | No complete clean-document event, local model/index lifecycle or privacy purge | DOC-002, AI-002, SEC-004 |
| Privacy access/export/deletion | Security principles only | No request workflow or cross-plane execution | API-010, BE-014, SEC-004, FE-009/011 |
| Powerful plain-language Tender Assistant | AI-001–004 partial | No unified operations, tender-language UX or addendum/preflight orchestration | API-012, AI-001, AI-004, AI-005 |
| Controlled support access | T1–T6/D1–D6/B1 selectively approved; SS/E/R ballot presented | Exact capabilities and session/evidence/fencing approval remain; no OpenAPI, persistence or runtime | API-010, AUTH-004 |
| Retention and legal hold | Selective privacy governance plus drafted BID-SEC-004R R1–R13 | Lawful matrices, R1–R13 approval, durations, restore evidence and destructive authority remain blocked | API-010, SEC-004 |
| Actionable dashboards | Unmounted permission-filtered Tenderpreneur and Platform Billing aggregation | Broader blocked-source sections, actions, routes/runtime and verification remain | BE-016, FE-009, FE-011 |
| Platform Admin UI | None | No settings/users/commercial/operations console | FE-010, FE-011 |
| Complete Phase 1 evidence | QA-002 external journey | No commercial/platform/AI/support/retention acceptance suite | QA-006 |

## 5. New child tickets

- BID-API-010–012: operation contracts for portal/platform, commercial and Tender Assistant/profile scope.
- BID-AUTH-003–004: platform authority and controlled support access.
- BID-TEN-004: plans, subscriptions and monthly Bid allowance.
- BID-INT-004: PesaPal runtime reconciliation.
- BID-BE-014–017: platform settings, organization knowledge, dashboards and exact branding exports.
- BID-AI-005: natural-language Tender Assistant orchestration.
- BID-SEC-004: retention/legal-hold execution.
- BID-FE-009–011: Tenderpreneur commercial/AI portal and two bounded Platform Admin surfaces.
- BID-QA-006: complete Phase 1 technical verification.
- BID-REL-005: provider/deployment review, persona UAT and formal Phase 1 acceptance.

## 6. Decision status and remaining blockers

1. Per-plan storage/document/AI limits, reservations and paid-write/reversal-hold denial are source-implemented-not-verified; complete licence/recovery enforcement remains open.
2. C1–C15 and RR1–RR15 policy are approved; Commercial source through C9 is implemented-not-verified. RR implementation is unauthorized, and tax-profile activation, PesaPal, refund execution, recovery and rendering remain finance/provider/security owned.
3. PesaPal API version/environment, merchant onboarding, authenticity mechanism, supported KES methods/status mapping and refund/reversal evidence remain unverified.
4. Production secret-manager/KMS choice remains open; control tables may store references only.
5. P1–P6 and the six-role lifecycle policy are approved; broad Platform Administration application/persistence/HTTP source is implemented-not-verified, while the deployment bootstrap executable/values, provider wiring and runtime evidence remain open.
6. Countries P1–P9 policy is approved without implementation/release authority; provenance/licensing, Product metadata, deployment compatibility and exact manifest digest remain absent.
7. Privacy D1–D12 is selectively approved/incomplete; lawful-basis/deadline and export/correction/restriction/destructive matrices remain absent.
8. Controlled support T1–T6/D1–D6/B1 is selectively approved. SS1–SS6/E1–E5/R1–R5, exact capabilities and every implementation step remain pending.
9. Retention/legal-hold R1–R13 remains drafted pending approval; exact periods, lawful bases, legal evidence, deletion/anonymization and restore execution remain blocked.
10. AI evaluation corpus, quality thresholds, language policy, knowledge/retrieval contract and provider acceptance remain open.
11. pgvector and local multilingual embedding model version/digest/licence/security/resource benchmark remain open.
12. Explicit permission to edit `web/` has not been granted; the repository remains backend-only.
13. Procurement Portal roles and complete functional journeys remain undefined.

## 7. Required delivery order

1. Complete the pending controlled-support SS/E/R and exact-capability decisions before any AUTH-004 source authorization.
2. Obtain privacy/legal matrices and review retention/legal-hold R1–R13; keep destructive execution unavailable.
3. Obtain countries provenance/licensing/Product/deployment/manifest evidence before any BE-014 country implementation or release.
4. Obtain exact PesaPal and accountable tax/provider evidence before INT-004 effects; keep RR1–RR15 unimplemented until separately authorized.
5. Freeze API-012 knowledge/assistant contracts, then complete BE-015/BE-017 and AI-005 one operation at a time.
6. Finish the remaining authorized AUTH-003/TEN-004/BE-014/BE-016 source only when each recorded gate is satisfied; do not activate runtime implicitly.
7. Implement FE-009–011 only after reviewed generated contracts and explicit web authorization.
8. Execute the 10 Option A verification packets and QA-006 only in their authorized verification phase.
9. Use REL-005 for provider/deployment review, persona UAT and formal human release disposition.

## 8. Audit conclusion

Phase 1 product direction, revised pricing/licences, company-first onboarding, letterhead and local-vector approach are approved, but the tickets are not Ready until their specialist/provider/contract gates close. No listed function should be called complete until its child ticket reaches the required evidence state. The frozen Option A source remains unchanged; these tickets define additional scope rather than retroactively claiming it was implemented.
