# Release readiness

## 1. Purpose
Separate an implemented feature from a verified, operable and approved release.

## 2. Scope
External MVP pilot gate and later AI/marketplace gates; no production-readiness claim from documentation alone.

## 3. Product specifications
[QA release gates](../product/15_QA_TEST_STRATEGY.md), [traceability release package](../product/19_TRACEABILITY_ACCEPTANCE_EVIDENCE.md), [roadmap](../product/16_PRODUCT_ROADMAP.md).

## 4. Technical designs
[QA](../technical_design/24_QA_TEST_ARCHITECTURE.md), [risks](../technical_design/28_TECHNICAL_RISKS.md), [DR](../technical_design/26_DISASTER_RECOVERY.md).

## 5. Prerequisites
Accepted release scope, tested vertical slices, production-like staging, named product/QA/security/operations owners.

## 6. Required decisions
BID-ADR-006 scope, 011/012 workflow policy, 013 recovery/retention/host, 015 limits, 017 formats. Later sealed release additionally 009/010.

## 7. Dependencies
No release before full scoped user journey, isolation and restore evidence. AI is separately gated; no need to claim full-product acceptance for external pilot.

## 8. Target files
Future docs/releases package, evidence register, runbooks, deployment artifacts and reviewed feature flags.

## 9. Sequence
Freeze scope/contract versions; reconcile requirements; run regression/security/accessibility/load; rehearse migrations/restore; UAT target personas; review risks/privacy; approve artifact/flags; staged rollout; monitor; post-release verification.

## 10. Domain/security
No cross-tenant/sealed confidentiality defect can be waived. Support access/retention/privacy and exception policy must be explicit. Customer claims distinguish external tracking from actual submission service.

## 11. Database/API/frontend
Test exact approved version → package → proof; no mock routes; migrations compatible; private UI/API/export paths remain authorized after restore and revocation.

## 12. Tests
Critical external E2E with separate technical/finance/approver users; negative isolation; concurrent update; scanner/package interruption; restore-to-new; keyboard/mobile; measured load/error budgets.

## 13. Acceptance
All required evidence passes, no blocking defect, approved risks within allowed policy, deployable artifact and recovery/on-call ownership ready.

## 14. Done
Product, engineering, QA, security and operations sign-offs appropriate to release; truthful known exclusions and release notes.

## 15. Risks
Pressure to reinterpret 'MVP' after failures; use approved scope rather than reducing gates silently. Absence of evidence is not accepted risk.

## 16. Evidence
Build digest, schema versions, tests/scans, UAT, performance profile, recovery times, privacy review, rollback plan and approvals.

## 17. Exclusions
Full procurement/AI acceptance on external pilot; guarantees of legal compliance; production activation by this documentation task.
