# QA and testing implementation

## 1. Purpose
Prove real Bidora journeys and failure recovery with reproducible evidence.

## 2. Scope
Unit/domain, DB, API, authorization, workers, components, accessibility, E2E, performance, migration, recovery and security.

## 3. Product specifications
[QA strategy](../product/15_QA_TEST_STRATEGY.md), [traceability/evidence](../product/19_TRACEABILITY_ACCEPTANCE_EVIDENCE.md).

## 4. Technical designs
[QA architecture](../technical_design/24_QA_TEST_ARCHITECTURE.md), [performance](../technical_design/25_PERFORMANCE_SCALABILITY.md), [recovery](../technical_design/26_DISASTER_RECOVERY.md).

## 5. Prerequisites
Requirement/operation matrix, synthetic fixtures, approved policy edge cases and pinned test dependencies.

## 6. Required decisions
BID-ADR-009 exact deadline behavior and 011/012 policy outcomes before asserting expected results; 013 load/recovery targets.

## 7. Dependencies
Tests begin with each slice. Release regression composes accepted slices; mocks may isolate unit tests but not accepted business E2E.

## 8. Target files
Future backend/tests, module tests, frontend component/Playwright tests, migration/scan fixtures and evidence artifacts.

## 9. Sequence
Define Given/When/Then cases; implement deterministic unit fixtures; real Testcontainers DB/broker/storage; API schema tests; component states; multi-role Playwright; fault injection; load/recovery; UAT with target personas.

## 10. Domain/security
Every permission has positive and negative cases. Test counts/snippets/filenames as well as content. Use synthetic identities/documents, never copied customer data.

## 11. Database/API/frontend
Assert command+audit+outbox atomicity, stable API errors and visible recovery UI together. Exact version/hash reconciliation connects approval to package to proof.

## 12. Tests
Baseline: cross-tenant request; context switch race; stale approval; expired evidence; corrupt upload; worker crash; duplicate evidence command; revoked approver; late external capture; restore with replay. Future deadline tests remain blocked until policy approved.

## 13. Acceptance
All applicable required cases pass in target environment; skipped checks are disclosed and cannot count as passed; no open release-blocking defect.

## 14. Done
Requirement-case mapping, deterministic rerun, actual reports and persona sign-off where required.

## 15. Risks
Flaky retries masking defects, 200-only assertions, screenshots mistaken for backend proof. Treat flakiness as defect and assert domain evidence.

## 16. Evidence
Build/commit, environment, tool versions, timestamps, pass/fail/skipped, artifact paths, limitations and reviewer.

## 17. Exclusions
Invented coverage percentages, fabricated source requirement IDs, legal compliance certification and application tests during documentation-only generation.
