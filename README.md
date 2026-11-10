# Bidora

Procurement and tender-management SaaS under active full-stack MVP development. The existing Go modular backend and Next.js frontend include authentication, tenant workflows and Platform Admin operations; some management/settings workflows remain unfinished. Source implementation is not production acceptance.

- [Optional development commands and configuration](docs/DEVELOPMENT.md)
- [Current development rules](AGENTS.md)
- [Single MVP delivery tracker](docs/tickets/PLATFORM_ADMIN_DELIVERY.md)
- [Frontend configuration](web/README.md)
- [Pilot runtime configuration](backend/docs/pilot-runtime-configuration.md)
- [Migrations and runtime grants](backend/migrations/README.md)
- [Current backend endpoint inventory](backend/api/openapi/endpoint-inventory.md) and [OpenAPI drafts](backend/api/openapi/drafts/)

Historical requirements/decisions and evidence remain available in `docs/`, `backend/docs/` and [the pre-reset README](docs/history/readme-before-mvp-reset.md). The [old ticket index](docs/tickets/00_INDEX.md) is historical navigation, not an active queue. Dated health-only/backend-only permissions do not restrict routine full-stack development today.

## Stack and local development

Use the versions declared in `backend/go.mod`, `web/package.json`, `web/package-lock.json` and `web/.node-version`. Backend: Go/Fiber, PostgreSQL/pgx/sqlc and Goose. Frontend: Next.js App Router, TypeScript, Tailwind and installed ReUI/shadcn components. Do not create a replacement backend or duplicate frontend root.

```bash
cd web
npm ci
# Set server-only environment variables from .env.example in an uncommitted local file.
npm run dev
```

`BIDORA_API_ORIGIN` must point to the intended Go runtime; `BIDORA_WEB_ORIGIN` must agree with its browser Origin configuration. See `web/README.md` for BFF address trust and Google callback configuration. Never expose provider keys or privileged backend configuration through `NEXT_PUBLIC_*`.

### Backend entry points are not interchangeable

- `backend/cmd/api`: dependency-free **health-only** smoke server, not the business API. Run with `BIDORA_ENV=local BIDORA_PROFILE=health-only go run ./cmd/api` from `backend/`.
- `backend/cmd/pilot-api`: full application runtime using explicit deployment configuration and dependency readiness checks. Build from `backend/` with `go build -o /tmp/bidora-pilot-api ./cmd/pilot-api`. Loading configuration/building does not provision or migrate services. See the runtime configuration reference above before launching.
- `backend/cmd/identity-api`: identity/Platform runtime for designated development or explicitly configured staging, promoted from the former hidden `.smtp001-runtime` entry point (removed, not duplicated). Build from `backend/` with `go build -o /tmp/bidora-identity-api ./cmd/identity-api`. Reuses existing application services without tenant document/broker dependencies; probes Redis and control schema before readiness. Development uses explicit TLS test dependencies and local key variables; `BIDORA_LOCAL_TRUSTED_PROXY_CIDRS` must identify trusted proxies (no hardcoded trust range). `BIDORA_LOCAL_HTTP_ADDR` optionally overrides the loopback default `127.0.0.1:8080`. Staging additionally requires its own HTTPS origins, TLS server name and listener configuration. This is not a production deployment profile.
- `backend/cmd/migrate` and `backend/cmd/grant-runtime`: explicit migration/privilege tools; not API startup steps and not permission to modify an unknown target.

Rebuild/restart identified Bidora-owned development services as needed; preserve unrelated host services. Do not start a duplicate API on the same port or point the frontend at the health-only executable expecting business routes. Use only disposable or expressly designated development databases for integration writes, never an assumed-safe existing database. Missing provider credentials do not block application implementation.

## Source checks

Read [AGENTS.md](AGENTS.md) first. Tests are temporarily opt-in: do not run or generate backend/frontend tests unless explicitly requested. Preserve existing test files; do not repair unrelated test failures during feature work. Use only necessary targeted syntax/static checks, not automatic full builds or verification suites. See [development commands and test re-enabling](docs/DEVELOPMENT.md).

`verify:auth` and `verify:auth:fast` now check contracts without tests. The former comprehensive pipeline remains available as `npm run verify:auth:with-tests` when explicitly requested. This does not establish live persistence, provider, tenant-isolation or release acceptance.
