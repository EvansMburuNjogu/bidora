# Bidora

Procurement and tender SaaS under development. **Implemented: the local Go ↔ Next.js health slice, a portable PostgreSQL connection component, opportunity domain values and strict manual-capture request preparation.** There are no authenticated business APIs or database-backed workflows yet.

## Backend-only Pi delegation

Current work is **manual and backend-only**. One explicitly authorized read-only package review has completed; no further agent run is authorized. See [.pi/REVIEW_AGENT.md](.pi/REVIEW_AGENT.md) for that setup and its limits. The older `/backend-*` controller is retained separately in [.pi/README.md](.pi/README.md); its budget ledger does not meter package agents or the foreground conversation.

## Requirements

- Go **1.27.2** (the module's toolchain directive can download this version).
- Node **24.16.0** and npm **11.13.0**, the versions used for this local verification. Upgrading to the researched Node 24.22.0/npm 11.19.1 candidate pair remains a separate verification task; these local pins are not a production-support approval.
- No Docker, PostgreSQL, broker, credentials or external providers required for this explicitly dependency-free smoke profile.

Dependencies: Go/Fiber 3.6.1, Next.js 16.4.2, React 19.3.1, TypeScript 5.9.3, Tailwind 4.3.3. npm versions are exact and resolved by `web/package-lock.json`; Go uses `go.mod`/`go.sum`. No framework generator or Git initialization is required.

## Start locally

Terminal 1, from the repository root:

```bash
cd backend
go mod download
BIDORA_ENV=local BIDORA_PROFILE=health-only go run ./cmd/api
```

Terminal 2:

```bash
cd web
npm ci
BIDORA_API_ORIGIN=http://127.0.0.1:8080 npm run dev
```

Open **http://127.0.0.1:3000**. The page calls a fixed Next.js server route, which requests readiness from the real Go API. Click **Check again** after stopping/restarting Go to verify honest failure and recovery. Stop either process with Ctrl+C.

The Go process rejects missing profile/environment, production mode and non-loopback bind addresses. `BIDORA_HTTP_ADDR` defaults to `127.0.0.1:8080`; an alternate value must be a loopback IP and valid port. `.env.example` files describe configuration but Go does **not** automatically load them. Do not put the backend origin in a `NEXT_PUBLIC_` variable.

Readiness is explicitly **health-only**, not an implicit fallback when database configuration is missing. `/health/live` measures process responsiveness; `/health/ready` is unavailable before initialization and during drain. No database is connected or certified. Adding dependencies requires a new explicit profile and actual bounded dependency checks.

## Portable PostgreSQL backend component — current focus

`backend/internal/postgres/` provides a pinned pgx pool with explicit configuration, verified TLS, SCRAM authentication, bounded operations and graceful shutdown. It is **not yet wired into API startup** and has not passed real PostgreSQL integration tests. See [implementation, security boundaries and actual verification](backend/docs/portable-postgres.md).

Current development is backend-only; web remains untouched/unstarted. Production provisioning/deployment and business policy remain separately gated.

## Option A database schemas

The reviewed [Option A table inventory](backend/schema/option-a-tables.json) identifies 52 required control/tenant tables and each implementation gate. Executable schemas have started with provider-neutral control database routing, tenant-common command evidence/compatibility and Tenderpreneur unverified capture. Inventory is not blanket approval; blocked identity, document, workflow and outcome policy is not guessed.

## Manual-capture database foundation

Executable Goose SQL and sqlc/pgx query code now exist for tenant-local audit/outbox/idempotency, schema compatibility checks and immutable **unverified** opportunity capture. Tool parsing, generation, drift, full race tests, vet and builds pass, but the migrations have not run against PostgreSQL and no route is enabled. Document tables were not guessed: their format/scan/retention policies remain blocked, so non-null source-document capture remains disabled. See [database scope, evidence and remaining gates](backend/docs/database-foundation.md) and [migration runbook](backend/migrations/README.md).

## Application development — current task

Manual-capture request rules/limits are approved. The opportunity request pipeline now validates strict JSON, preserves unknown/raw facts, maps explicit UTC instants into source timezones, and produces owned canonical input/fingerprints. It does not verify facts, authorize users or commit a capture; the HTTP route remains unregistered pending identity and persistence/security gates. See [scope, tests and next blockers](backend/docs/application-development.md).

The five `local*` packages, their tests and `cmd/local-check` have been removed from the active backend and preserved byte-for-byte in [the historical archive](archive/local-fixture/README.md). Do not run old local-tooling instructions. [Anti-diversion rules](AGENTS.md#active-anti-diversion-rules--application-development-first) prioritize bounded application outcomes.

## Checks

Go tests are centralized in `backend/tests/`, not beside production code:

```bash
cd backend
go test -race ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -show verbose ./...
```

Frontend, schema and real HTTP contract checks:

```bash
cd web
npm ci
npm run contract:lint
npm run contract:check
npm test
npm run test:contract
npm run typecheck
npm run build
npm audit --audit-level=low
```

`test:contract` builds an isolated Go binary in an OS temporary directory, starts it on a free loopback port, validates real responses against the OpenAPI schemas, checks safe errors and graceful exit, then removes its own temporary files. No database or mock API is used.

Browser smoke on a supported Playwright host:

```bash
cd web
npx playwright install chromium
npm run build
npm run test:e2e
```

On this development machine, bundled Chromium installation is **unsupported on macOS 12.7.6**. The actual browser test instead passed against installed Google Chrome:

```bash
PLAYWRIGHT_CHANNEL=chrome npm run test:e2e
```

The browser suite owns a real Go process on port **18080** and production-mode Next.js on **3100**, exercises ready → stopped API → restarted API plus keyboard/mobile layout, and cleans up its processes. These ports must be free. `reuseExistingServer` is disabled so an unrelated server cannot produce false evidence. Production-mode Next.js here is still local testing, not deployment approval.

## Contract and code layout

- `backend/api/openapi/health.json`: machine-readable health contract.
- `backend/cmd/api/`: process startup and bounded shutdown.
- `backend/internal/postgres/`: portable, explicit TLS/SCRAM pgx connection management; independent of local fixture paths/tools.
- `backend/migrations/`, `backend/queries/`, `backend/sqlc.yaml`: bounded capture migration/query inputs and checksums; not database-executed.
- `backend/internal/opportunity/persistence/sqlc/`: generated pgx/v5 typed capture queries; never hand-edit.
- `backend/internal/opportunity/{domain,application,transport}/`: unverified capture facts, deadline mapping, strict request validation and canonical input/fingerprints; no registered route, SQL or verification/approval authority.
- `backend/tests/architecture/`: regression gate against restoring/importing retired tooling.
- `archive/local-fixture/`: historical experiments, excluded from active Go builds/tests.
- `backend/internal/{config,health,httpapi}/`: configuration, framework-independent state and HTTP adapter.
- `backend/tests/`: external-package Go tests, including real-listener oversized-input coverage.
- `web/src/contracts/health.generated.ts`: generated types; update with `npm run contract:generate`, never by hand.
- `web/src/lib/health-client.ts`: bounded response validation and safe upstream handling.
- `web/src/app/api/health/`: fixed, server-only loopback upstream request; no cookie forwarding or user-selected target.
- `web/tests/`: unit, schema/live contract and real-browser tests.

Local implementation limits: 4096-byte Go request body and upstream response cap, 2-second upstream fetch timeout, 4-second browser timeout, Go read/write timeout 3 seconds, idle timeout 10 seconds, shutdown timeout 5 seconds. These are bounded local engineering settings, **not approved production SLOs**. Polling is manual; no automated retry of business operations exists.

## Known limitations

- OpenAPI lint currently emits one license URL/identifier metadata warning; no license grant or legal URL was invented to suppress it.
- Go vulnerability analysis reports no reachable vulnerable symbols/imported packages, but one module-only advisory for unused `golang.org/x/crypto/openpgp` (GO-2026-5932, no fixed version). Keep that package out of use and reassess dependency changes.
- Full accessibility audit, production hardening, CI hosting, identity, tenant isolation and recovery/load tests are not completed. Native semantic UI is used; shadcn/template adoption remains later UI work.
- The health route's same-origin arrangement applies only to this public local smoke slice. It does not approve the future authenticated cookie/CSRF deployment policy.
- Full foundation tickets remain partially implemented/blocked on broader prerequisites. See the [execution evidence](docs/tickets/12_HEALTH_SLICE_EVIDENCE.md), [backlog](docs/tickets/00_INDEX.md) and [AGENTS](AGENTS.md).
