# Local Go ↔ Next.js health slice — execution evidence

Status: **VERIFIED_TECHNICALLY for the bounded local health-only slice; broader tickets and release acceptance remain open**.

## Authorization and scope

The user authorized proceeding after the assistant explicitly requested permission to install pinned dependencies, implement/validate the health contract, build a smoke page and run tests. The user subsequently required centralized Go tests under `backend/tests/`; that convention is implemented and recorded in root AGENTS.

This is a narrow implementation increment across BID-API-001, BID-FND-001 and BID-FND-002, not completion of all their original prerequisites. It deliberately excludes database/service profiles in FND-003, customer data, authentication, tenancy, business APIs, production infrastructure and Git initialization. Remaining ARC-002/006/007 and security/business decision work is not silently approved or bypassed for those excluded surfaces.

The explicit profile `BIDORA_PROFILE=health-only` and environment `local`/`test` are mandatory. No missing database configuration is interpreted as business readiness. A later database-backed profile must fail closed on missing/unavailable required dependencies.

## Implemented files and behavior

- [OpenAPI health contract](../../backend/api/openapi/health.json): `/health/live` and `/health/ready`, safe envelopes/headers, strict schema and body-size failure.
- [Go process](../../backend/cmd/api/main.go): validated local config, loopback listener, explicit ready state, SIGINT/SIGTERM drain and bounded shutdown.
- [HTTP adapter](../../backend/internal/httpapi/app.go): no-store responses, server-generated UUID correlation, safe 400/404/405/413/500/503, no raw provider/internal errors.
- [Centralized Go tests](../../backend/tests/httpapi/app_test.go) and [config suite](../../backend/tests/config/config_test.go): external packages import production APIs, no exported test-only internals.
- [Next.js page](../../web/src/app/page.tsx) and [health panel](../../web/src/components/health-panel.tsx): real readiness request, accessible status/retry, no fabricated healthy state or business dashboards.
- [Server route](../../web/src/app/api/health/route.ts): fixed health path against an operator-configured loopback origin; no cookie/header forwarding or user-selected upstream URL.
- [Bounded client](../../web/src/lib/health-client.ts): validates schema shape, HTTP status, media type, cache header and correlation; bounds response size/time and never forwards raw upstream messages.
- [Generated types](../../web/src/contracts/health.generated.ts) and [drift check](../../web/scripts/check-contract-drift.mjs): generated from OpenAPI, not hand-edited.
- [Browser test](../../web/tests/e2e/health.spec.mjs): real Go process plus production-mode local Next server; no mocked success.
- [README](../../README.md): reproducible start/check commands, profile limits and known gaps.

Native semantic elements and Tailwind CSS are used for this smoke page. No downloaded template, shadcn installer or domain UI design system is claimed adopted. Same-origin server routing is scoped to public local health, not approval of future cookie/CSRF topology.

## Verified versions

| Tool | Actual selected/tested version |
|---|---|
| Go toolchain | 1.27.2, downloaded through module toolchain selection |
| Fiber | 3.6.1 |
| Host Node / npm | 24.16.0 / 11.13.0 |
| Next.js / React / React DOM | 16.4.2 / 19.3.1 / 19.3.1 |
| TypeScript | 5.9.3 — tested conservative baseline rather than unverified 7.0.2 candidate |
| Tailwind / PostCSS adapter | 4.3.3 / 4.3.3 |
| OpenAPI lint / generation | Redocly CLI 2.64.1 / openapi-typescript 7.13.0 |
| JSON Schema validation | Ajv 8.20.0 with ajv-formats 3.0.1 |
| Browser driver | Playwright 1.64.1, installed Google Chrome channel on macOS 12.7.6 |
| Go vulnerability analysis | govulncheck 1.9.0 |

Exact direct dependencies and transitive resolutions are in `backend/go.mod`, `backend/go.sum`, `web/package.json` and `web/package-lock.json`. The researched Node 24.22.0/npm 11.19.1 pair was not installed; upgrade validation remains open. No claim that the local host pins are production patch-policy approval.

## Executed checks and actual results

Commands are relative to their stated directory. Results were observed during this implementation session, not inferred from source files.

| Directory / command | Result |
|---|---|
| `backend`: `go test -race ./...` and final `go test -count=1 -race ./...` | Pass: centralized configuration and HTTP suites; actual oversized-request listener case; final rerun bypassed test cache |
| `backend`: `go vet ./...` | Pass |
| `web`: final `npm ci`, then drift/unit/contract/type/build/browser suite | Clean lockfile install succeeded; all listed checks passed again |
| `web`: `npm run contract:lint` | Valid OpenAPI, **one warning**: license URL/identifier absent; not suppressed or fabricated |
| `web`: `npm run contract:generate` | Generated TypeScript schema types |
| `web`: `npm run contract:check` | Pass: regenerated bytes including standard generator header match committed output |
| `web`: `npm test` | 6 tests passed: runtime guards, safe network/error handling, loopback config, correlation/status/cache/media validation and response cap |
| `web`: `npm run test:contract` | 2 tests passed: positive/negative JSON Schema fixtures and actual Go HTTP/status/header/error/signal-exit checks |
| `web`: `npm run typecheck` | Pass |
| `web`: `BIDORA_API_ORIGIN=http://127.0.0.1:18765 npm run build` | Pass: production-mode local Next build |
| `web`: `PLAYWRIGHT_CHANNEL=chrome npm run test:e2e` | 1 browser scenario passed: real readiness, keyboard retry, mobile width, stopped Go outage and restarted Go recovery |
| Client static artifact inspection | Canary origin `http://127.0.0.1:18765` absent from all 11 `.next/static` files; narrowly proves this server configuration value was not bundled |
| `web`: `npm audit --audit-level=low` | 0 reported vulnerabilities at execution time |
| `backend`: `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -show verbose ./...` | No reachable symbol/imported-package vulnerabilities; **one module-only advisory**, described below |

Final filesystem checks: all 98 ticket Markdown files and root README/AGENTS local links resolve; no `*_test.go` files remain under `backend/internal/` or `backend/cmd/`; no `.git/` or duplicate `frontend/` exists. All 25 product-source files match the original SHA256 baseline (`shasum -a 256 -c /tmp/bidora-product-before.sha256`).

### Failures encountered and resolved or retained

- Fiber's in-memory `app.Test` surfaced oversize input as a transport error. Kept the 413 assertion but moved that case to a real HTTP listener in the centralized Go suite; actual wire tests now pass. The body limit was not weakened.
- The first generated-drift checker omitted openapi-typescript's standard comment header, producing a false mismatch. It now uses the generator's exported `COMMENT_HEADER`; no generated code was manually changed to hide drift.
- Initial Next build warned about a parent/home lockfile. Explicit Turbopack root now limits the build to `web/`.
- `npx playwright install chromium` failed: the pinned browser download does not support macOS 12.7.6. The installer also removed an unused cached browser revision as part of its normal cache maintenance. No success is claimed for bundled Chromium installation. Browser verification used the existing Chrome channel explicitly; a supported host is still needed for the standard bundled-browser workflow.
- License metadata warning remains. Legal ownership/license terms were not invented to get a warning-free lint report.

## Security disposition and limitations

Govulncheck reports **GO-2026-5932**, unmaintained/unsafe `golang.org/x/crypto/openpgp`, in required module `golang.org/x/crypto@v0.58.0`; no fixed version is listed. Neither the affected package nor its symbols are used by this code according to the scan. This is a disclosed module-only finding, not a blanket security waiver. Do not add OpenPGP usage; reassess dependency changes and before release.

No independent security assessment, production certification, tenant isolation suite, database migration/restore, load/chaos or full WCAG/manual assistive-technology review ran. Keyboard/mobile checks are a small smoke subset. The canary scan is not a comprehensive secret scanner.

Local engineering limits are documented in README: body/response caps and short bounded request/shutdown deadlines. They do not establish production SLO/RPO/RTO or approved business retry policies.

## Ticket and next-step disposition

- BID-API-001: health subset implemented and schema/live validated; general business pagination/concurrency/idempotency/error-policy approval and generator governance remain open.
- BID-FND-001: local Go health/config/drain verified; broader foundation/CI/service prerequisites and independent acceptance remain open.
- BID-FND-002: real Next→Go smoke verified; full design-system adoption and broader UX/identity/deployment policy remain open.
- BID-FND-003: database/Compose/secrets/service profile implementation was **not performed**; do not mark Done based on this dependency-free profile.
- Architecture and master tickets retain their current workflow statuses. This evidence records a bounded verified increment without blanket-unblocking full tickets.

Next bounded work: review this result and resolve the local PostgreSQL/configuration baseline and remaining identity/authorization policy before introducing data or private business endpoints. Keep centralized Go test placement and extend real contract/isolation tests as surfaces are added. No live server is intentionally left running by the test suite, and Git remains uninitialized.
