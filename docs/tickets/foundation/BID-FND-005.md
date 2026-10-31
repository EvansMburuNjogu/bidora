# BID-FND-005 — Docker-free local resource configuration validation

## Metadata
- Ticket ID: BID-FND-005
- Epic: E-FND
- Type: Engineering
- Priority: P0
- Status: In Review
- Complexity: S
- Dependencies: None
- Blockers: None
- Milestone: M1

### Cleanup disposition — historical review only

The user requested cleanup and application development. The validator and tests are now preserved in [the retired local-fixture archive](../../../archive/local-fixture/README.md), not the active backend module. Earlier file paths, results and review findings below are historical. This does not convert pending user acceptance to Done or authorize restoring/expanding the tooling. Existing controller settings and usage evidence remain intact.

## 1. Objective
Implement and verify a small pure-Go local resource configuration validator without Docker, new dependencies or service access.

## 2. Business Context
The user approved a local PostgreSQL foundation with 2 CPUs, 2 GiB RAM and a 10 GiB storage budget, then explicitly requested splitting Docker-free work and starting subagents. Validate the approved limits before later service integration without pretending a validator enforces container or disk quotas.

## 3. Scope
Create `backend/internal/localprofile/` with a production-facing configuration type and validation API for future local-profile integration. Add external-package tests only under `backend/tests/localprofile/` and a short `backend/docs/local-profile-preparation.md` explaining use, limits and deferred integration. Use only the Go standard library and existing pinned toolchain. Do not wire this helper into HTTP startup/readiness yet. Existing health-only behavior must remain unchanged.

## 4. Out of Scope
Docker, Compose, PostgreSQL connections, dependency installs/upgrades, image/tool pin acceptance, SQL/bootstrap/migrations, secrets, business/auth APIs, application startup changes, filesystem capacity probing, quota enforcement, web/frontend, product/ticket/controller edits and production approval.

## 5. Technical Requirements
Engineering contract implementing the user's approved local-only limits and explicit split/start instruction, narrowly scoped to disconnected preparation:
- A caller-supplied configuration has environment, profile, CPU count, memory MiB and storage GiB fields.
- Environment must be exactly `local` or `test`; profile must be exactly `postgres-local`.
- CPU count must be an integer from 1 through 2; memory MiB an integer from 1 through 2048; storage GiB an integer from 1 through 10. No implicit defaults or silent clamping. Integer Go fields make fractional values unrepresentable at this API boundary; this task introduces no JSON/environment parser.
- A pure validation method/function returns an error for any invalid field. Do not echo arbitrary input in errors. Avoid global mutable state, I/O, goroutines and network calls.
- Validation is not allocation, readiness, DB authorization, minimum viable PostgreSQL sizing or enforcement of a disk quota. Values below the approved ceilings can pass this helper without being approved operating sizes; a future reviewed runtime profile must establish viability and enforcement.
- Do not export implementation internals solely for tests; the exported configuration/validation API is the intended future production integration boundary.

## 6. Proposed Implementation Approach
Read this packet, root instructions, packet 13 and the existing health configuration/tests. Implement the isolated helper, table-driven negative/boundary tests and usage documentation. Run format, then test/vet/build against the final fingerprint. Submit review_required with actual evidence; do not change ticket status or enable a database-backed service.

## 7. Dependencies
No Docker or broader architecture Done status is required for this pure, disconnected preparation helper. Existing verified Go 1.27.2 health baseline and the approved local resource budget are sufficient. FND-004 now explicitly depends on this child for its later integration, while retaining its own pins/security/role and Docker gates. Accountable scope/result reviewer: the requesting user, who authorized the local limits and instructed the split/start; no independent security-review role is inferred. The implementing agent must not accept its own result.

## 8. Acceptance Criteria
- `local` and `test` with `postgres-local` and 2/2048/10 validate successfully.
- Positive boundary values within the stated ceilings validate; zero, negative and over-limit values fail without clamping.
- Production, empty, whitespace/case-mutated or arbitrary environment/profile values fail.
- Errors do not contain arbitrary input values; a synthetic canary in an invalid field is not echoed.
- Validation does not mutate the input, perform I/O or alter existing health behavior.
- Documentation explicitly distinguishes configuration validation from runtime enforcement and deferred database acceptance.

## 9. Testing Requirements
External Go tests under `backend/tests/localprofile/`: valid environments/boundaries, every invalid-field class, multiple invalid fields, safe errors and unchanged input. Run all existing/new tests with `go test -count=1 -race ./...`, `go vet ./...` and `go build ./...` from `backend/` after the final edit (or equivalent fixed worker checks if delegation is later reauthorized). No fake DB test or Docker skip is needed: DB behavior is explicitly outside this task.

## 10. Definition of Done
Helper/tests/docs implemented, required checks passed, criterion evidence submitted and user review accepted. The worker can only return review_required or blocked. No full foundation/backend completion is claimed.

## 11. References
[Approved local scope](../13_BACKEND_FOUNDATION_UNBLOCKING.md), [integration sibling/parent](BID-FND-004.md), [health evidence](../12_HEALTH_SLICE_EVIDENCE.md), [existing configuration](../../../backend/internal/config/config.go), [existing tests](../../../backend/tests/config/config_test.go), [repository instructions](../../../AGENTS.md).

## 12. Risks and Notes
This is deliberately small disconnected preparation, not a PostgreSQL feature. Disk space, Docker allocation, tool licenses/security, real credentials/roles and integration remain FND-004 acceptance blockers. The user subsequently instructed manual execution without subagents. No worker was used for implementation; delegate-budget controls remain unchanged and do not meter the foreground conversation. Stop at the packet boundary rather than widening the task.

### Completed read-only review

Run `b57a481e-3d18-4a14-9e07-82d038321a00` by `bidora-backend-review` (GPT-6 Astra, low thinking) completed with `no_blocking_findings`. Its output artifact was read by the parent from the local Pi session artifacts. The seven-file review found the validator, tests, unchanged startup and documentation consistent with this packet. It did not rerun tests, establish repository-wide history or grant user acceptance. Status remains In Review; no repeat agent is authorized by this result. The user subsequently requested moving to the next ticket manually.

### Manual execution evidence

Status: **VERIFIED_TECHNICALLY / In Review**. User acceptance is pending; this is not full foundation or backend completion.

Files:
- `backend/internal/localprofile/config.go`: explicit configuration value and pure `Validate` method; no HTTP/startup wiring, I/O, defaults or clamping.
- `backend/tests/localprofile/config_test.go`: external-package boundary/negative/error-safety/input-preservation tests.
- `backend/docs/local-profile-preparation.md`: usage, precise limits and deferred runtime enforcement.

Actual checks executed after the final source edits:
- `gofmt -w backend/internal/localprofile/config.go backend/tests/localprofile/config_test.go` — completed.
- `cd backend && go test -count=1 -race ./...` — passed existing config/HTTP and new localprofile suites.
- `cd backend && go vet ./...` — passed.
- `cd backend && go build ./...` — passed.
- `cd backend && go test -count=1 -coverpkg=./internal/localprofile ./tests/localprofile` — passed; 100.0% statement coverage for the new package.

Both supported environments and positive resource boundaries pass. Empty/production/unknown/case/whitespace-mutated scope, zero/negative/over-limit/extreme resources and multiple invalid fields fail; errors are static, canaries are not echoed, and inputs stay unchanged. Source inspection confirms no I/O or runtime wiring. Coverage is not proof of PostgreSQL viability, quota enforcement or independent security acceptance. No Docker, service startup, new dependency, migration, product-source or web change was performed.
