# Engineering standards

## 1. Purpose
Keep Bidora domain ownership and evidence reliable across human and agent contributions.

## 2. Scope
Go, SQL, TypeScript, later Python, contracts, migrations and documentation changes.

## 3. Product specifications
[LLD coding standards](../product/09_LLD.md), [agent guide](../product/23_AGENTS.md), API-003–005, BR-050.

## 4. Technical designs
[Repository](../technical_design/02_REPOSITORY_ARCHITECTURE.md), [backend](../technical_design/08_BACKEND_ARCHITECTURE.md), [API](../technical_design/10_API_CONTRACT_DESIGN.md).

## 5. Prerequisites
Reviewed task with operation, actors, rule IDs, failure cases and allowed paths.

## 6. Required decisions
BID-ADR-005 approves `web/` and `backend/`; BID-ADR-008 approves Goose and npm with one web lockfile. Pin actual compatible versions during authorized kickoff using the [toolchain packet](../tickets/10_FOUNDATION_DECISION_PACKET.md); tool selection is not runtime verification.

## 7. Dependencies
Contract review before handlers; migration verification before dependent repository acceptance; server rule tests before frontend acceptance.

## 8. Target files
Owner module domain/application/adapters, its SQL queries, API schema, feature UI and tests. Avoid unrelated formatting/refactors.

## 9. Sequence
Write command/transition tests; specify contract/errors; author reviewed migration/query; implement domain/application transaction; adapter; UI; integration/security/E2E evidence. Keep changes small enough to review rollback and authorization implications.

## 10. Domain/security
No domain imports of Fiber/pgx/provider SDKs. No shared mutable entities or direct other-module repositories. Parameters for SQL values; allowlisted identifiers. Context cancellation/timeouts mandatory. Never swallow errors or log sensitive input.

## 11. Database/API/frontend
DB rows are not DTOs. Money strings/decimal; UTC plus zone; expected versions for writes; strict TypeScript; generated contracts not hand-edited; frontend does not compute authoritative readiness.

## 12. Tests
Go formatter/vet/selected static analysis, strict TS/lint, schema compatibility, Testcontainers and relevant Playwright. Later Python adds Pydantic validation, Ruff/mypy/pytest as selected.

## 13. Acceptance
Forbidden import checks and contract drift checks fail on deliberate violations; command errors map deterministically.

## 14. Done
Code reviewed, generated output reproducible, tests run, evidence linked, deviations approved.

## 15. Risks
Generic repositories hide SQL constraints; giant shared packages bypass ownership; avoid both. Dependency additions require purpose/license/maintenance/security review.

## 16. Evidence
Commit, changed files, rule IDs, contract/migration versions, exact test output and known limitations.

## 17. Exclusions
No 'cleanup' that changes procurement policy, no unapproved ORM/framework, no test bypass to produce green CI.
