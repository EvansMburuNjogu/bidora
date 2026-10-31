# Authentication completion — backend and web

Status: **IN PROGRESS — NOT PRODUCTION READY**

Scope: password and Google identity, registration/verification, login/logout, recovery/change, opaque browser sessions, CSRF, MFA, account security, access-resolution foundations, Next.js BFF integration and authentication verification. Platform Admin and Tenderpreneur portal development are excluded.

## Architecture

The browser calls same-origin Next.js route handlers. Server-only BFF modules forward bounded requests to Go/Fiber using explicit API/web origins. Go handlers enforce strict request binding, Origin and session-bound CSRF, then call identity application services. PostgreSQL control tables are authoritative for users, credentials, verification/recovery challenges, sessions, MFA, external identities and audit evidence. Redis stores only ephemeral HMAC-scoped attempt/OIDC/MFA state. SMTP and Google are explicit external adapters. Session and CSRF values use Secure host-only cookies; CSRF and one-time MFA material remain memory-only in web code.

## Audit matrix

| Capability | State | Evidence / confirmed issue |
|---|---|---|
| Password registration | Partial | Strict Go handler/service/store and web form exist. First/last-name contract compiles; migration 29 and updated journey are not executed. |
| Email verification | Partial | Single-use hash-only 30-minute token consumption is technically verified. No resend HTTP/web recovery path. Real SMTP acceptance is pending. |
| Password login/logout | Complete technically | Generic credential failure, dummy work, Argon2id rehash, Redis limits, session issue/revoke and secure cookie handling have backend synthetic evidence. Deployment proxy/browser acceptance remains open. |
| Password change | Complete technically / partial web | Atomic credential/auth-epoch/session rotation exists and web form is wired. New-password confirmation and browser automation are absent. |
| Password recovery/reset | Complete technically / partial web | Backend issuance/reset/limiting and web pages exist. Real SMTP and full browser verification remain open; reset confirmation is absent. |
| Sessions and CSRF | Complete technically | Opaque tokens, hash-at-rest, idle/absolute expiry, Origin/CSRF, touch ordering and logout are verified in backend. Browser lifecycle regression coverage is absent. |
| Session inventory/revocation | Integration gap | Backend GET/revoke-one/revoke-all routes exist and are technically verified. No Next.js BFF or account-security UI exists. |
| MFA login/enrollment | Complete technically / partial web | Backend TOTP/recovery-code login and enrollment are verified; web supports login challenge and initial enrollment. |
| MFA lifecycle | Integration gap | Backend step-up, recovery-code regeneration and factor replacement exist. Web has no BFF/UI for them. |
| Google sign-in | Partial | Backend OIDC/state/PKCE/JWKS/session flow and frontend start/callback exist with synthetic evidence. External Google acceptance is pending. Identity-only runtime omits Google composition. |
| Google account linking | Integration gap | Backend handlers/routes exist in full pilot. Web has no BFF/UI. External provider acceptance is pending. |
| Post-login access resolution | Integration gap | `/v1/me` and account-type mutation exist. Web explicitly reports account-type selection unavailable. No portal work is required to fix selection. |
| Authorization foundation | Complete technically | Protected Go routes use current server-side session, membership/capability/context checks; tenant selectors are not authority. Portal business UI is out of scope. |
| Auditability | Partial | Security state changes append minimized database audit evidence. Deployment monitoring/alerts and provider-delivery evidence are not accepted. |
| Web security headers | Partial | Referrer, nosniff, frame denial and Permissions-Policy exist. Production CSP/HSTS/deployment validation remains open. |
| Automated verification | Complete technically | Non-Playwright BFF/source tests, complete race-enabled identity/HTTP suites and selected live PostgreSQL/Redis/local-JWKS/authenticated-STARTTLS authentication groups pass. Manual/provider/deployment acceptance remains separate. |

## Confirmed gaps

1. Authentication schema migration was ordered after unrelated organization schema work; corrected in AUTH-001 so personal names are migration 29, notification evidence is migration 30, and organization creation is migration 31.
2. Control compatibility checks were stale at schema 28 while source/runtime grants target 31.
3. Updated name-aware registration has no functional PostgreSQL/API/browser evidence.
4. Verification resend is absent, leaving users unable to recover from a lost or ambiguous verification email.
5. Account-type selection is implemented in Go but unavailable in web.
6. Session administration and advanced MFA/account-linking controls are backend-only.
7. BFF client-network provenance is not deployment-defined, risking shared network limits or trusted-proxy failure.
8. External SMTP/Google, production proxy/TLS, CSP/HSTS, monitoring, rollback and independent security acceptance remain unverified.

## Development tickets

### AUTH-001 — Stabilize authentication schema baseline

- **Priority:** High
- **Issue:** Personal-name registration depended on unrelated organization migration; compatibility checks targeted schema 28.
- **Implementation:** Make personal names control migration 29, notification evidence migration 30, and organization creation migration 31, align all compatibility/grant checks, generated artifacts and checksums.
- **Acceptance:** Goose/sqlc/checksum validation passes; schema 28→29 supports authentication without notification or organization tables; 29→30 and 30→31 remain later non-authentication steps.

### AUTH-002 — Verify registration and add verification resend

- **Priority:** High
- **Issue:** Name-aware registration is unverified and verification email cannot be resent.
- **Implementation:** Add focused name policy/HTTP/store tests, run migration 29 in an isolated database, verify registration persistence/duplicate privacy, then add a generic bounded resend contract using existing challenge invalidation and delivery ports.
- **Acceptance:** New password users persist exact approved names, legacy/provider null names remain valid, duplicate responses stay generic, resend invalidates the old token, limits fail closed, and web can request resend without enumeration.

### AUTH-003 — Complete post-login account setup

- **Priority:** High
- **Issue:** `/v1/me` returns `SELECT_ACCOUNT_TYPE`, but web has no selection action.
- **Implementation:** Add generated mutation contract, server-only BFF and deliberate no-default account-type UI. Keep portal implementation excluded.
- **Acceptance:** Verified authenticated user can select the approved account type once; replay is safe; CSRF/Origin/idempotency apply; no membership or portal authority is inferred.

### AUTH-004 — Complete account-security web controls

- **Priority:** High
- **Issue:** Session administration, MFA step-up/regeneration/replacement and Google linking are backend-only; password change/reset lack confirmation.
- **Implementation:** Add generated DTOs, bounded BFF routes and account-security UI using existing backend operations and memory-only secret handling.
- **Acceptance:** User can inspect/revoke sessions, revoke all, step up, regenerate codes, replace TOTP, link Google and confirm new passwords with safe stale-session/error behavior.

### AUTH-005 — Harden BFF authentication integration

- **Priority:** High
- **Issue:** Safe BFF errors drift from canonical envelopes and client-network provenance is undefined.
- **Implementation:** Align error/trace/cache headers and define a deployment-owned trusted ingress-to-BFF-to-Go client-IP contract without trusting arbitrary browser headers.
- **Acceptance:** Per-network limits use the approved client address; spoofed forwarding headers fail safely; all BFF failures match safe contracts and expose no secrets.

### AUTH-006 — Automated end-to-end authentication verification

- **Priority:** High
- **Issue:** No current web authentication tests and current contract changes lack functional evidence.
- **Implementation:** Add non-Playwright contract/component/BFF tests plus focused Go tests and synthetic web→BFF→Go→PostgreSQL/Redis/SMTP journeys.
- **Acceptance:** Registration/verification/login/MFA/recovery/change/session/logout/Google synthetic flows and negative security cases pass without weakening controls.

### AUTH-007 — Deployment and provider acceptance

- **Priority:** Medium
- **Issue:** Production SMTP, Google, proxy/TLS, CSP/HSTS, monitoring, recovery and rollback are unaccepted.
- **Implementation:** Validate explicit secrets/trust roots/callbacks/proxy CIDRs, sender/provider behavior, browser security policy, redacted telemetry, backup/restore and incident procedures.
- **Acceptance:** Independent evidence covers real providers and production topology; no secret or personal-data leakage; rollback/recovery and security sign-offs are recorded.

### AUTH-008A — Ordinary-login email verification codes

- **Priority:** High
- **Issue:** Ordinary password and Google login previously depended on an authenticator-app challenge, contrary to the requesting user's revised email-code direction.
- **Implementation:** Deliver a cryptographically uniform six-digit, five-minute, transaction-bound code through explicit authenticated STARTTLS; persist only a transaction-scoped HMAC-SHA-256 value; permit five attempts; atomically consume on success; replace the transaction on resend; fail closed without SMTP.
- **Acceptance:** Password and Google login both require the latest delivered code, old/resubmitted codes fail, raw codes never persist or log, SMTP failure issues no session, and email provenance remains AAL1 rather than privileged AAL2.

## Execution order

1. AUTH-001.
2. AUTH-002 and AUTH-005 backend work in parallel after schema baseline.
3. AUTH-003 and AUTH-004 web work after their contracts are stable.
4. AUTH-006 after each vertical slice, ending with the composed journey.
5. AUTH-007 last; production readiness cannot be claimed before it passes.

## Progress

- AUTH-001: **IMPLEMENTED_NOT_VERIFIED**. Authentication is now the independent control-29 migration target; source ordering, compatibility checks, checksums, focused identity/HTTP/database tests and migrator builds pass. Live PostgreSQL migration/application evidence remains pending.
- AUTH-002: **VERIFIED_TECHNICALLY / BROWSER_ACCEPTANCE_PENDING**. Control migration 29 was applied to the isolated TLS fixture. Race-enabled live PostgreSQL/Redis/in-process authenticated-STARTTLS evidence covers exact personal-name persistence, duplicate-name privacy, generic registration/resend, replacement-token invalidation, atomic consumption/replay denial, unknown-target concealment and fail-closed resend limits. The Next.js BFF/UI passes contract, lint, TypeScript and production-build checks; manual browser and external SMTP acceptance remain pending.
- AUTH-003: **VERIFIED_TECHNICALLY / MANUAL_BROWSER_ACCEPTANCE_PENDING**. The Next.js resolution screen requires an explicit, initially unselected Tenderpreneur choice, uses memory-only CSRF/idempotency state through a bounded server-side BFF, and refreshes canonical `/v1/me` after success or conflict. An isolated live HTTP journey traversed the running Next.js BFF and identity backend: login/session bootstrap, `UNASSIGNED` resolution, selection, matching replay, conflicting replay denial, and canonical `TENDERPRENEUR` plus `ESTABLISH_TENDERPRENEUR_MEMBERSHIP` resolution all passed. No capability, context or portal authority appeared. Automated Safari interaction remains unavailable because remote automation is disabled, so visual/keyboard/browser acceptance is still pending.
- AUTH-004: **VERIFIED_TECHNICALLY / PROVIDER_AND_MANUAL_UI_ACCEPTANCE_PENDING**. Session administration and MFA lifecycle passed live BFF and race-enabled backend evidence. Google linking is source-complete with race-enabled local-JWKS backend evidence but external-provider acceptance remains in AUTH-007. Registration, password reset and authenticated password change now require matching client-only confirmation values; confirmation is never included in backend payloads. Frontend lint, TypeScript, contract drift and production build pass; focused password application/HTTP suites pass.
- AUTH-005: **VERIFIED_TECHNICALLY / DEPLOYMENT_PROXY_ACCEPTANCE_PENDING**. Locally generated BFF failures now always use bounded JSON envelopes with trace IDs; authentication proxies reject oversized/unsupported input and non-JSON upstream errors, and Google redirects are allowlisted. Development direct-peer mode forwards no caller header. Production fails closed unless explicit trusted-header mode receives exactly one valid `X-Bidora-Client-IP`; deployment must prove ingress overwrite/no-bypass and Go trusted-proxy CIDRs. Runtime BFF checks passed generic credential, 415 and 413 envelopes. Production ingress evidence remains AUTH-007.
- AUTH-006: **VERIFIED_TECHNICALLY / MANUAL_AND_PROVIDER_ACCEPTANCE_PENDING**. Added non-Playwright Node tests that launch an isolated Next.js dev BFF and fake upstream, prove trusted client-address fail-closed behavior, exact forwarding, bounded trace errors, non-JSON diagnostic concealment and Google redirect allowlisting, then remove their process-scoped build output. Source checks prove browser storage remains unused and password confirmation remains client-only. `npm run verify:auth` passed 8 tests plus contract drift, ESLint, TypeScript and production build; the subsequent registration-route diagnostic added a ninth passing regression that maps an unexpectedly unmounted known backend route to safe `503 AUTH_UNAVAILABLE`. Race-enabled full identity/HTTP suites and 15 live PostgreSQL/Redis/local-JWKS/authenticated-STARTTLS authentication groups pass registration, verification/resend, login/logout/session touch, MFA lifecycle/limits, recovery/change, session revocation, Google signup/sign-in/linking and replay races. No external-provider, production-ingress, visual/UAT or formal acceptance is inferred.
- AUTH-008A: **IMPLEMENTED_NOT_VERIFIED / SMTP_DEPLOYMENT_BLOCKED**. Control migration 32, keyed email-code application/persistence logic, authenticated-STARTTLS delivery, browser-bound verify/resend routes, renamed secure cookie/BFF flow, `/auth/login-code` UI, generated sqlc/OpenAPI artifacts and compatibility floors are implemented. The approved resend limits are enforced fail-closed through HMAC-scoped Redis counters: one per 30 seconds, three per 15 minutes per identity and ten per 15 minutes per validated network, with generic 429/`Retry-After: 900`. The insufficient-Platform-assurance page no longer duplicates authenticator instructions and now offers explicit logout without pretending email OTP is AAL2. Formatting, backend build, OpenAPI validation, frontend contract drift, lint, TypeScript and production build pass. Migration execution, tests, SMTP/provider evidence and browser verification are deferred. Privileged TOTP-backed AAL2 policy is unchanged.
- AUTH-007: **PAUSED / BLOCKED ON DEPLOYMENT AND LEGAL INPUTS**. The running loopback identity API is healthy but was composed without SMTP/registration, so `/v1/auth/register` is intentionally unmounted and returned backend `404`. The BFF now maps an unexpected `404` for a known authentication operation to safe `503 AUTH_UNAVAILABLE`; it cannot make registration functional without an authenticated-STARTTLS SMTP deployment configuration and route recomposition. Application-owned legal routes `/legal/terms` and `/legal/privacy` are approved and reserved. Separate version fields were withdrawn; future atomic evidence is expected to identify the published policy URLs, user, acceptance timestamp and minimized audit provenance. The user requested generated policy content, so both routes now carry Bidora-specific drafts covering the service, accounts, tender/AI responsibility, acceptable use, user content, providers, privacy categories/purposes, security, cookies, retention, transfers and rights. They explicitly identify unresolved legal entity, jurisdiction, liability, retention, subprocessor, transfer, contact and effective-date inputs. Both remain noindex and visibly `Draft for legal review / Not effective and not open for acceptance`; registration and its Google alternative remain disabled. No consent evidence is recorded.
