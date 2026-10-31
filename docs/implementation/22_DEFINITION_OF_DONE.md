# Definition of done

## 1. Purpose
Prevent code existence, page count or optimistic status labels from substituting for accepted Bidora behavior.

## 2. Scope
Every future ticket, slice and release; documentation changes have their own honest validation scope.

## 3. Product specifications
[Master definition of complete](../product/00_MASTER_INDEX.md), [QA §§17–19](../product/15_QA_TEST_STRATEGY.md), [completion states](../product/19_TRACEABILITY_ACCEPTANCE_EVIDENCE.md).

## 4. Technical designs
[QA](../technical_design/24_QA_TEST_ARCHITECTURE.md), [traceability](../technical_design/29_REQUIREMENTS_TRACEABILITY.md), [risks](../technical_design/28_TECHNICAL_RISKS.md).

## 5. Prerequisites
Ticket ready: operation/actor, rule IDs, accepted decisions, data/API/page contract, failure cases and test plan.

## 6. Required decisions
No Pending Approval decision used as if approved. Policy default needs explicit source or signed decision.

## 7. Dependencies
Accepted prerequisite behavior, not placeholder functions or mocked production integrations.

## 8. Target artifacts
Changed owner-module files, migrations/contracts/client, tests, runbook and requirement/evidence record.

## 9. Completion sequence
Review scope → implement approved operation → verify migration/domain/API/policy → integrate real UI → run regression/accessibility → review observability/security/recovery → record evidence → UAT/sign-off as required. Mark DESIGNED, IMPLEMENTING, IMPLEMENTED_NOT_VERIFIED, VERIFIED_TECHNICALLY, UAT_PENDING, ACCEPTED, DEFERRED or BLOCKED truthfully.

## 10. Domain/security
Same transaction state/audit/outbox; denied cross-tenant/resource/field paths; immutable approved versions; permission revocation; no secrets. Critical controls cannot be disabled for tests.

## 11. Database/API/frontend
Compatible migrations and rollback/forward-fix; contract generation current; all loading/empty/error/forbidden/conflict states; no accepted mock workflow.

## 12. Tests
Executed applicable unit/integration/contracts/authz/isolation/E2E/a11y/migration/security checks. Relevant performance/restore and AI benchmarks when affected. Missing tools/checks explicitly disclosed.

## 13. Acceptance
Persona completes intended operation and alternate/failure recovery; no manual DB fix; exact input/output/evidence versions reconcile.

## 14. Done checklist
- Requirements and decisions linked.
- Reviewed bounded changes and immutable release migrations.
- Exact commands/results with failures/skips.
- Security, privacy and operational ownership addressed.
- UAT/release status distinguished from technical verification.
- Known exclusions and deferred work recorded.

## 15. Risks
False 'complete' claims from screenshots or stub endpoints. Zero application code means documentation checks cannot establish application quality.

## 16. Evidence
Report files, requirement/task IDs, schema/API versions, test command/result, artifacts, risks, flags/deployment and approvals.

## 17. Exclusions
Invented test outcomes, generic 'tests pass', legal/security guarantees without independent evidence, silent scope reduction.
