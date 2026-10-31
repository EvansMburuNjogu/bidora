# Bidora web

Bidora’s web application uses Next.js App Router, React, strict TypeScript, Tailwind CSS v4, and ReUI/shadcn `base-nova` components.

## Current implementation

`WEB-AUTH-001` establishes the authentication surface:

- email/password sign-in and open registration;
- Google sign-in initiation;
- password recovery and reset;
- email verification;
- TOTP or one-time recovery-code login challenge;
- password visibility controls and explicit input guidance across every authentication form;
- a server-only authentication proxy that preserves backend cookies, redirects, generic failures, request limits, and `private, no-store` behavior;
- a same-origin Google callback bridge plus explicit completion and generic failure destinations;
- canonical session and membership contract projections;
- backend-authoritative portal resolution;
- a responsive Tenderpreneur shell with explicit, memory-only organization selection from `GET /v1/me/memberships`.

The UI intentionally does not grant tenant or platform access. Portal resolution, memberships, capabilities, tenant context, account recovery delivery, provider acceptance, and production route activation remain backend/deployment-owned gates.

## Configuration

Copy `.env.example` to an uncommitted environment file and set `BIDORA_API_ORIGIN` and `BIDORA_WEB_ORIGIN` to the explicit Go API and approved browser origins. Never expose either through a `NEXT_PUBLIC_*` variable. Production requires HTTPS; only explicit loopback HTTP is permitted in development.

When deployment enables the backend Google flow, register the exact public `${BIDORA_WEB_ORIGIN}/api/auth/google/callback` URI with Google and use it for the backend `BIDORA_GOOGLE_CALLBACK_URL`. Configure `BIDORA_GOOGLE_SUCCESS_REDIRECT` as `${BIDORA_WEB_ORIGIN}/auth/google/complete` and `BIDORA_GOOGLE_FAILURE_REDIRECT` as `${BIDORA_WEB_ORIGIN}/auth/google/failed`. Account linking uses `${BIDORA_WEB_ORIGIN}/api/auth/google-link/callback`, with fixed success/failure destinations `/account/security/google-linked` and `/account/security/google-link-failed`. These values do not activate Google routes by themselves: client credentials, trust roots, Redis/PostgreSQL composition, provider acceptance, and backend route activation remain deployment-owned.

Authentication network limits require an explicit BFF client-address mode. `direct-peer` is development-only and forwards no browser-supplied address. Production must set `BIDORA_BFF_CLIENT_IP_MODE=trusted-header`; the deployment ingress must prevent direct access to Next.js and strip/overwrite `X-Bidora-Client-IP` with exactly one validated IP. The BFF rejects missing, lists and malformed addresses, then forwards the value to a Go peer that must itself be in the explicit trusted-proxy CIDRs. Deployment evidence for both boundaries remains mandatory.

## Checks

```bash
npm ci
npm run contracts:check
npm run lint
npm run test:auth
npm run build
# Or run the complete source/BFF authentication sequence:
npm run verify:auth
npm audit --omit=dev
```

The application is not started by these commands. No frontend UAT, accessibility acceptance, provider acceptance, or deployment acceptance is implied by a successful build.
