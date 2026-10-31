# BID-QA-002 — Verify complete external-tender journey and negative regressions

## Metadata
- Ticket ID: BID-QA-002
- Epic: E-QA
- Type: Testing
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-FE-008, BID-OPS-002, BID-QA-001
- Blockers: Complete runnable staging candidate
- Milestone: M4

## 1. Objective
Prove the scoped external lifecycle across UI/API/databases/storage with multiple real roles.
## 2. Business Context
A set of working endpoints is insufficient if a bid manager cannot finish preparation, approval and evidence capture.
## 3. Scope
Register/provision → verified tender → upload → matrix/evidence → technical/finance response → review → package → proof → outcome; regression of all slice tests.
## 4. Out of Scope
Marketplace, AI accuracy, mocked production dependencies.
## 5. Technical Requirements
Playwright uses real test mail/API/tenant DBs/storage/broker/scanner; verify audit/source/candidate/package/proof hashes; separate technical/finance/approver actors; failure/retry and context revocation paths.
## 6. Proposed Implementation Approach
Build persona fixtures; execute normal journey; inject stale approval, expired evidence, scanner/package outage and lost response; verify immutable history and denied foreign metadata; publish trace/report.
## 7. Dependencies
[Complete UI](../frontend/BID-FE-008.md), [staging](../devops/BID-OPS-002.md), [harness](BID-QA-001.md).
## 8. Acceptance Criteria
- Given valid workflow, then exact approved package is linked to confirmed proof and outcome.
- Given stale approval or missing proof, then transition blocks with usable feedback.
- Given repeated committed command, then no duplicate submission/audit fact results.
## 9. Testing Requirements
Multi-role E2E, API conformance, tenant/financial negative suite and failure/recovery traces.
## 10. Definition of Done
All scoped cases pass with reproducible artifacts and declared limitations, no manual database repair.
## 11. References
[J-01/J-06/J-07/J-08/J-10 (J-09 AI excluded)](../../product/06_USER_JOURNEYS_AND_WORKFLOWS.md), [QA architecture](../../technical_design/24_QA_TEST_ARCHITECTURE.md), [QA plan](../../implementation/16_QA_TEST_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Skipped cases remain unverified; screenshot alone does not prove authorization.
