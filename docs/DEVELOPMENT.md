# Everyday development

[AGENTS.md](../AGENTS.md) is the central project/agent reference. Start there, then inspect only relevant code. This guide supplies optional command/configuration details, not a mandatory checklist.

## Temporary testing policy

Tests are **opt-in until explicitly requested**, including unit, integration, BFF and browser scripts. Do not automatically run or generate tests, repair unrelated tests, install test infrastructure, or invoke test-bearing scripts. Existing files under `backend/tests/`, `web/tests/` and colocated packages are preserved. This policy supersedes older risk-matrix/mandatory-test advice in historical documents.

Review the focused diff and use only necessary syntax/static validation. Do not automatically run full builds, project-wide checks or restart services. Runtime authentication, authorization, tenant isolation, CSRF/Origin, audit, idempotency, secret encryption and TLS remain mandatory application behavior. Paused tests are not release acceptance.

## Targeted commands (not a sequence)

| Location | Command | Purpose |
|---|---|---|
| `web/` | `npm run dev` | Start development frontend only if the intended service is not running |
| `web/` | `npx eslint <affected-file>` | Targeted static lint, when needed |
| `web/` | `npm run typecheck` | Next route type generation + TypeScript; no tests |
| `web/` | `npm run contracts:generate` / `contracts:check` | Generate/check frontend contract projection when relevant |
| `backend/` | `go build -o /tmp/bidora-identity-api ./cmd/identity-api` | Compile identity/Platform API when necessary |
| `backend/` | `go build -o /tmp/bidora-pilot-api ./cmd/pilot-api` | Compile full runtime when necessary |
| `backend/` | `python3 api/openapi/generate.py` / `--check` | Generate/check API contracts when relevant |

Use `npm ci` only for initial setup or dependency changes. Never hand-edit generated sqlc Go. SQL generation/migration/grant operations follow [the migration guide](../backend/migrations/README.md); confirm its commands against current source before use. Do not silently alter unrelated reviewed checksums.

## Runtime/configuration navigation

- `backend/cmd/api` is health-only. It does not expose business routes.
- `backend/cmd/identity-api` reuses identity/Platform services without full tenant document/broker dependencies. It requires the configured TLS PostgreSQL/Redis and local key inputs; inspect its source before launch.
- `backend/cmd/pilot-api` uses `backend/internal/pilot/config.go` and full dependency readiness. See [runtime configuration](../backend/docs/pilot-runtime-configuration.md) before launching.
- `cmd/migrate` and `cmd/grant-runtime` are explicit maintenance tools, never startup steps. Source builds do not authorize migrations, grants, destructive operations or production deployment.
- Frontend environment examples and proxy trust are in [web/.env.example](../web/.env.example) and [web/README.md](../web/README.md). Align `BIDORA_API_ORIGIN`, `BIDORA_WEB_ORIGIN` and backend allowed origins. Production requires HTTPS and trusted ingress address handling; never expose server secrets via `NEXT_PUBLIC_*`.
- Reuse identified Bidora-owned services; do not launch duplicate listeners or disturb unrelated services. Never assume an existing database is disposable.
- All system emails use Platform Settings' active encrypted SMTP configuration through existing notification consumers/shared transport. Do not restore obsolete per-feature SMTP credentials. Delivery evidence must not be replaced by raw sends or fake success.
- Provider configuration, source implementation, successful compilation and real provider acceptance are different facts. Do not activate paid providers or send real messages as a validation shortcut.

## Verification scripts and re-enabling tests

| Command from `web/` | Current behavior |
|---|---|
| `npm run verify:auth` | Alias to static fast verification |
| `npm run verify:auth:fast` | Contract freshness only; no tests/server/build |
| `npm run verify:auth:full` | Contract freshness, lint, TypeScript and build; no tests; not a routine default |
| `npm run verify:auth:with-tests` | Preserved original comprehensive test-bearing pipeline; explicit request only |
| Existing `npm run test:*` scripts | Preserved manual entry points; explicit request only |

On an explicit testing request, choose the smallest relevant preserved test command. Do not substitute unrelated passing suites or repair unrelated failures. Go tests remain runnable through explicit `go test` commands from `backend/`; no automatic backend test runner was found.

To restore automation, point `verify:auth` at `verify:auth:with-tests` (or restore the full alias and its test step), and restore `npm run test:auth:fast` after contract checking in `verify:auth:fast`. No checked-in CI workflow or active Git hook was found; inspect any externally managed CI/branch protections separately before claiming they were changed.

Historical test/check evidence in the delivery tracker is not a command checklist or proof that current code passed. Do not rerun known unrelated failures during feature development. Report unexecuted verification and actual blockers briefly; maintain release/security caution even while test gates are paused.
