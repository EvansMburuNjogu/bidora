# Bidora web

Bidora’s web application uses Next.js App Router, React, strict TypeScript, Tailwind CSS v4, and ReUI/shadcn `base-nova` components.

## Everyday development

Read the root [AGENTS.md](../AGENTS.md) first; use [docs/DEVELOPMENT.md](../docs/DEVELOPMENT.md) only for relevant runtime/command details. Preserve [generated Next.js instructions](AGENTS.md).

Authentication pages use server-only proxies that preserve backend cookies, safe redirects, generic failures, request limits and `private, no-store` behavior. Session/membership contracts and backend-authoritative portal resolution connect the portal UI to the Go runtime; see the active delivery tracker for current authentication behavior rather than historical feature evidence.

The UI intentionally does not grant tenant or platform access. Portal resolution, memberships, capabilities, tenant context, account recovery delivery, provider acceptance, and production route activation remain backend/deployment-owned gates.

## Configuration

Copy `.env.example` to an uncommitted environment file and set `BIDORA_API_ORIGIN` and `BIDORA_WEB_ORIGIN` to the explicit Go API and approved browser origins. Never expose either through a `NEXT_PUBLIC_*` variable. Production requires HTTPS; only explicit loopback HTTP is permitted in development.

When deployment enables the backend Google flow, register the exact public `${BIDORA_WEB_ORIGIN}/api/auth/google/callback` URI with Google and use it for the backend `BIDORA_GOOGLE_CALLBACK_URL`. Configure `BIDORA_GOOGLE_SUCCESS_REDIRECT` as `${BIDORA_WEB_ORIGIN}/auth/google/complete` and `BIDORA_GOOGLE_FAILURE_REDIRECT` as `${BIDORA_WEB_ORIGIN}/auth/google/failed`. Account linking uses `${BIDORA_WEB_ORIGIN}/api/auth/google-link/callback`, with fixed success/failure destinations `/account/security/google-linked` and `/account/security/google-link-failed`. These values do not activate Google routes by themselves: client credentials, trust roots, Redis/PostgreSQL composition, provider acceptance, and backend route activation remain deployment-owned.

Authentication network limits require an explicit BFF client-address mode. `direct-peer` is development-only and forwards no browser-supplied address. Production must set `BIDORA_BFF_CLIENT_IP_MODE=trusted-header`; the deployment ingress must prevent direct access to Next.js and strip/overwrite `X-Bidora-Client-IP` with exactly one validated IP. The BFF rejects missing, lists and malformed addresses, then forwards the value to a Go peer that must itself be in the explicit trusted-proxy CIDRs. Deployment evidence for both boundaries remains mandatory.

## Development validation — tests opt-in

Do not automatically run or generate unit, integration, BFF or browser tests, or repair unrelated tests. Preserve existing test files. Use affected-file lint, `npm run typecheck` or `npm run contracts:check` only when necessary; these do not execute suites. `npm ci` is setup/dependency work, not routine validation.

- `npm run verify:auth` → `verify:auth:fast` → contract freshness only.
- `npm run verify:auth:full` → contracts, lint, TypeScript and production build, **without tests**. This is not a default development step.
- `npm run verify:auth:with-tests` preserves the original full pipeline, including auth tests, for an explicit testing request.
- Existing `test:auth*`, `test:platform` and browser scripts remain available manually; run them only when explicitly requested.

See [re-enabling automation](../docs/DEVELOPMENT.md#verification-scripts-and-re-enabling-tests). No tests were deleted and no runtime security controls are disabled. Static checks/builds alone do not prove browser, provider, accessibility, persistence or release acceptance.
