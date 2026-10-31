# Vertical slices roadmap

## 1. Purpose
Order verifiable outcomes rather than layers of unused scaffolding.

## 2. Scope
S0–S9 proposed external MVP; later releases add public procurement, exchange and AI only after their gates. These S identifiers are local planning IDs, not source sprint IDs.

## 3. Product specifications
[Delivery W00–W07](../product/17_DELIVERY_PLAN_WAVES_SPRINTS.md), [journeys J-01/J-06–J-10](../product/06_USER_JOURNEYS_AND_WORKFLOWS.md), PR-TP-004–014.

## 4. Technical designs
[Modules](../technical_design/09_BACKEND_MODULE_DESIGN.md), [schema](../technical_design/06_DATABASE_SCHEMA_DESIGN.md), [API](../technical_design/10_API_CONTRACT_DESIGN.md), [screens](../technical_design/12_FRONTEND_ARCHITECTURE.md).

## 5. Prerequisites
ADR-006 release scope, ADR-005 root and owner-approved task packets. Each slice must resolve its policy blockers before dependent handlers.

## 6. Required decisions
007 identity; 011 approval/exception; 012 outcomes; 013 deployment/recovery; 015 quotas; 017 file/package policy. Sealed 009/010 is not an external-MVP prerequisite.

## 7. Dependencies
S0 → S1 → S2 → S3; S4 depends S2 and joins S3 at S5; S5 → S6 → S7 → S8 → S9. Documents/events may be developed alongside opportunity UI only after shared authz foundations.

## 8. Target components
Each slice owns contract, migration, application, UI and tests. Paths are specified by governing workstreams rather than duplicate per-slice skeleton directories.

## 9. Implementation sequence and engineering scope

| Slice / objective / complexity | Database and backend contracts | Frontend / security / tests | Dependencies / completion evidence |
|---|---|---|---|
| S0 reproducible health path / M | Config and health contracts; ephemeral migration smoke only | Shell calling real API; no mock auth; config/health/build/Playwright smoke | Approved paths/tools; clean checkout and honest API failure evidence |
| S1 trusted identity / L | Control users/challenges/sessions/factors; auth and /me | Auth/recovery/MFA; CSRF/rotation/throttling; replay and enumeration tests | S0, ADR-007; verified human session and revoked cookie denied |
| S2 isolated workspace/team / L | Registry/assignments/jobs/memberships/grants, tenant common; workspaces/context/invitations | Provisioning/team/switcher; role grant boundary and no default DB; crash/race/last-admin tests | S1, quota/storage policy; two isolated tenants, checkpoint retry and sampled restore |
| S3 external opportunity/bid / M | Opportunities/source versions/bids/decisions; manual, verify, create-workspace, decision | Source/deadline capture and bid overview; human decision; zone/source/duplicate/conflict tests | S2; real verified opportunity becomes assigned private bid without AI |
| S4 clean versioned documents / L | Upload/version/verification/jobs; initiate/complete/download/verify | Uploader/vault/quarantine; immutable scan target and classification; malformed/oversize/overwrite/outage tests | S2/events, ADR-017; only clean exact versions available |
| S5 compliance matrix / M–L | Requirements/evidence/readiness; requirement/status/link/readiness APIs | Matrix/source/evidence panel; restricted evidence and SATISFIED guards; expired/foreign/stale tests | S3+S4; mandatory blockers and exact evidence demonstrated |
| S6 response/pricing / L | Response revisions/financial lines; technical and financial APIs | Versioned editors and conflict resolution; independent pricing capability; decimal/XSS/leakage/race tests | S5; contributors collaborate without price leakage |
| S7 immutable review / L | Candidate/decisions/applicability; submit-for-review and step decisions | Reviewer snapshot/approval panel; assigned step/current authority; edit/approve/revoke tests | S6, ADR-011; changes invalidate applicability without erasing decisions |
| S8 package/external proof / L | Packages/jobs/evidence/confirmations; package, download, record/confirm/mark-submitted | Readiness/progress/proof timeline; package-wide classification, no false external delivery; retry/storage-split/late-capture tests | S7, ADR-017; exact approved package and confirmed proof linked once |
| S9 outcome/pilot / M feature + L release | Outcome history and authorized portfolio queries; outcome/correction | Outcome/history, no unsupported metrics; retention and reporting scope; full E2E, restore/load/accessibility | S8, ADR-012/013; persona UAT and complete release package |

Future sequence: approved-source search → grounded extraction → procurement authoring/publication → reviewed sealed acceptance/opening → evaluation/award → advanced AI/enterprise. The precise ordering of search versus procurement may change through product decision, never by silently weakening existing rules.

## 10. Domain/security
Every slice includes permission-denied, stale version and retry behavior. No final state can be set through unrestricted PATCH or inferred from a completed UI step.

## 11. Database/API/frontend
Contract-first per slice; reviewed schema before persistence; real API before UI acceptance. Lightweight component development may proceed with fixtures, but accepted workflow cannot depend on them.

## 12. Tests
Use [QA workstream](16_QA_TEST_IMPLEMENTATION.md); table cases are minimum, not a replacement for authorization matrix and transaction tests.

## 13. Acceptance
Every row's user outcome runs against real dependencies and has linked evidence; no manual database intervention.

## 14. Done
[Definition of done](22_DEFINITION_OF_DONE.md) applies independently to each slice; don't mark whole M08/M13 complete for a bounded pilot subset.

## 15. Risks
Large L slices need ticket subdivision by complete operation, not frontend-only pages. Capacity/calendar must be estimated by actual team, not copied from 33 indicative source sprints.

## 16. Evidence
One operation demonstration plus requirement/test mapping, migration and contract versions, failure-case results and UAT status per slice.

## 17. Exclusions
No scheduled delivery promise, sealed bidding in first MVP, autonomous AI or full-module completion from partial capabilities.
